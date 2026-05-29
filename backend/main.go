package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/ulule/limiter/v3"
	"github.com/yourusername/kor-assetforge/config"
	_ "github.com/yourusername/kor-assetforge/docs"
	"github.com/yourusername/kor-assetforge/handlers"
	"github.com/yourusername/kor-assetforge/handlers/auth"
	"github.com/yourusername/kor-assetforge/middleware"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
	"github.com/yourusername/kor-assetforge/utils"
	"github.com/yourusername/kor-assetforge/validator"
	"golang.org/x/time/rate"
)

// @title kor-AssetForge API
// @version 0.1.0
// @description Decentralized marketplace for tokenizing and trading real-world assets on Stellar.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize database
	db, err := config.InitDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Stellar client
	stellarClient, err := config.InitStellarClient()
	if err != nil {
		log.Fatalf("Failed to initialize Stellar client: %v", err)
	}

	// Initialize Redis
	redisURL := os.Getenv("REDIS_URL")
	redisClient, err := utils.InitRedis(redisURL)
	if err != nil {
		log.Printf("Warning: Failed to initialize Redis, continuing without cache: %v", err)
		redisClient = nil
	} else {
		defer redisClient.Close()
	}

	// Initialize advanced cache manager (wraps Redis with L1 + metrics)
	cacheManager := utils.NewCacheManager(redisClient)

	// Warm common cache entries on startup
	go cacheManager.Warm(context.Background(), config.WarmCacheEntries(db))

	// Initialize Redis-backed rate limiter (optional)
	var rateLimiterMiddleware gin.HandlerFunc
	if redisClient != nil {
		rl, err := handlers.NewRateLimiter(redisClient, limiter.Rate{
			Period: time.Minute,
			Limit:  100,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize rate limiter: %v", err)
		} else {
			rateLimiterMiddleware = rl.Middleware()
		}
	}
	_ = rateLimiterMiddleware // available for use on individual routes if needed

	// Setup authentication
	authConfig := &auth.AuthConfig{
		JWTSecret:          getEnvOrDefault("JWT_SECRET", "your-super-secret-jwt-key-change-in-production"),
		JWTExpirationHours: getEnvIntOrDefault("JWT_EXPIRATION_HOURS", 24),
		RefreshTokenHours:  getEnvIntOrDefault("REFRESH_TOKEN_HOURS", 168),
		EmailTokenHours:    getEnvIntOrDefault("EMAIL_TOKEN_HOURS", 24),
		PasswordResetHours: getEnvIntOrDefault("PASSWORD_RESET_HOURS", 1),
		BcryptCost:         getEnvIntOrDefault("BCRYPT_COST", 12),
	}
	emailService := services.NewEmailServiceFromEnv()
	authHandler := auth.NewAuthHandler(db, authConfig, emailService)
	authMiddleware := auth.NewAuthMiddleware(authConfig.JWTSecret)
	authRateLimiter := auth.NewAuthRateLimiter(rate.Limit(5.0/60.0), 10)

	// Setup router
	router := gin.New()

	if err := validator.Init(); err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
	}

	// Use custom enhanced middleware
	router.Use(
		handlers.RequestLogger(),
		handlers.GlobalErrorHandler(),
		middleware.RequestSizeLimiter(2<<20),
		middleware.RequireJSON(),
		middleware.RateLimit(20, time.Minute),
		middleware.CSRFProtection(os.Getenv("CSRF_SECRET")),
	)

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(db, redisClient, stellarClient)
	router.GET("/health", healthHandler.LivenessCheck)
	router.GET("/health/ready", healthHandler.ReadinessCheck)
	router.GET("/health/live", healthHandler.LivenessCheck)

	// Metrics endpoint
	// @Summary Prometheus metrics
	// @Description Get service metrics in Prometheus format
	// @Tags monitoring
	// @Produce plain
	// @Success 200 {string} string "Prometheus metrics"
	// @Router /metrics [get]
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Cache metrics
	router.GET("/metrics/cache", middleware.CacheMetricsHandler(cacheManager))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (public)
		authGroup := v1.Group("/auth")
		authGroup.Use(authRateLimiter.GeneralAuthRateLimit())
		{
			authGroup.POST("/register", authRateLimiter.RegisterRateLimit(), authHandler.Register)
			authGroup.POST("/login", authRateLimiter.LoginRateLimit(), authHandler.Login)
			authGroup.POST("/refresh", authHandler.RefreshToken)
			authGroup.POST("/verify-email", authRateLimiter.EmailVerificationRateLimit(), authHandler.VerifyEmail)
			authGroup.POST("/forgot-password", authRateLimiter.PasswordResetRateLimit(), authHandler.ForgotPassword)
			authGroup.POST("/reset-password", authHandler.ResetPassword)
		}

		// Protected user routes
		protected := v1.Group("")
		protected.Use(authMiddleware.JWTAuth())
		{
			protected.GET("/profile", authHandler.GetProfile)
			protected.POST("/logout", authHandler.Logout)

			// 2FA routes
			protected.POST("/auth/2fa/setup", authHandler.Setup2FA)
			protected.POST("/auth/2fa/verify", authHandler.Verify2FA)
			protected.POST("/auth/2fa/disable", authHandler.Disable2FA)

			// Admin-only routes
			admin := protected.Group("")
			admin.Use(authMiddleware.RequireRole(models.RoleAdmin))
			{
				_ = admin // placeholder until admin routes are added
			}
		}

		// 2FA verification during login (unauthenticated)
		v1.POST("/auth/2fa/login", authHandler.LoginWith2FA)

		// Asset routes (with write-through cache invalidation)
		assetHandler := handlers.NewAssetHandler(db, stellarClient, redisClient, emailService)
		v1.POST("/assets/tokenize",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.TokenizeAsset)
		v1.POST("/assets",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.TokenizeAsset)
		v1.GET("/assets",
			middleware.HTTPCache(cacheManager, 5*time.Minute, "kor:asset", nil),
			assetHandler.ListAssets)
		v1.GET("/assets/:id",
			middleware.HTTPCache(cacheManager, 5*time.Minute, "kor:asset", nil),
			assetHandler.GetAsset)

		// NFT Metadata routes
		v1.POST("/assets/metadata",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.UpdateMetadata)
		v1.GET("/assets/:id/metadata",
			middleware.HTTPCache(cacheManager, 5*time.Minute, "kor:asset", nil),
			assetHandler.GetMetadata)
		v1.POST("/assets/metadata/immutable",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.MakeMetadataImmutable)

		// Oracle price feed routes (#104)
		v1.GET("/oracle/price",
			middleware.HTTPCache(cacheManager, 1*time.Minute, "kor:oracle", nil),
			assetHandler.GetOraclePrice)
		v1.GET("/assets/:id/oracle-price",
			middleware.HTTPCache(cacheManager, 1*time.Minute, "kor:oracle", nil),
			assetHandler.GetAssetOraclePrice)

		// Batch transaction routes (#106)
		v1.POST("/batch/execute",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.ExecuteBatch)
		v1.GET("/batch/:id",
			middleware.HTTPCache(cacheManager, 1*time.Minute, "kor:batch", nil),
			assetHandler.GetBatchStatus)
		v1.GET("/batches",
			assetHandler.ListBatchTransactions)

		// Marketplace routes
		v1.POST("/marketplace/list",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.ListAssetForSale)
		v1.POST("/marketplace/transfer",
			middleware.InvalidateOnWrite(cacheManager, "kor:asset:*"),
			assetHandler.TransferAsset)
		v1.GET("/transactions", assetHandler.ListTransactions)

		// Search routes (#57)
		searchBackend := services.NewESSearchBackend(os.Getenv("ELASTICSEARCH_URL"), db)
		searchHandler := handlers.NewSearchHandler(searchBackend)
		v1.GET("/search/assets", searchHandler.Search)
		v1.GET("/search/suggestions", searchHandler.Suggest)
		v1.GET("/search/analytics", searchHandler.SearchAnalytics)

		// KYC / AML routes (#55)
		kycHandler := handlers.NewKYCHandler(db, nil, emailService) // nil = mock provider
		v1.POST("/kyc/submit", kycHandler.SubmitKYC)
		v1.GET("/kyc/status", kycHandler.GetKYCStatus)
		v1.POST("/kyc/documents", kycHandler.UploadDocument)
		v1.POST("/kyc/aml/screen", kycHandler.ScreenAML)
		v1.POST("/kyc/accredited", kycHandler.VerifyAccreditedInvestor)
		v1.GET("/kyc/audit", kycHandler.GetAuditLog)
		v1.GET("/compliance/report", kycHandler.ComplianceReport)

		// Webhook routes
		webhookHandler := handlers.NewWebhookHandler(db)
		router.POST("/webhooks/stellar-events", webhookHandler.HandleStellarEvent)
		router.POST("/webhooks/kyc", kycHandler.HandleKYCWebhook)
	}

	// WebSocket routes (#54) — outside v1 group so the CSRF/JSON middleware
	// does not block the Upgrade handshake.
	wsHandler := handlers.NewWebSocketHandler()
	router.GET("/ws", wsHandler.HandleWS)
	router.GET("/ws/stats", wsHandler.HandleWSStats)

	// Pre-launch the hub so it's ready before the first connection.
	_ = handlers.GetHub()

	// Start server
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}
