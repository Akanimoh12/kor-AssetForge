package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret          string
	JWTExpirationHours int
	RefreshTokenHours  int
	EmailTokenHours    int
	PasswordResetHours int
	BcryptCost         int
}

// AuthHandler handles authentication operations
type AuthHandler struct {
	db     *gorm.DB
	config *AuthConfig
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, config *AuthConfig) *AuthHandler {
	return &AuthHandler{db: db, config: config}
}

// RegisterRequest represents user registration request
type RegisterRequest struct {
	StellarAddress string `json:"stellar_address" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Username       string `json:"username" binding:"required,min=3,max=50"`
	Password       string `json:"password" binding:"required,min=8"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// TokenResponse represents JWT token response
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	User         UserInfo  `json:"user"`
}

// UserInfo represents user information in responses
type UserInfo struct {
	ID             uint       `json:"id"`
	StellarAddress string     `json:"stellar_address"`
	Email          string     `json:"email"`
	Username       string     `json:"username"`
	Role           string     `json:"role"`
	EmailVerified  bool       `json:"email_verified"`
	KYCVerified    bool       `json:"kyc_verified"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
}

// RefreshTokenRequest represents token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// VerifyEmailRequest represents email verification request
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// ForgotPasswordRequest represents forgot password request
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents password reset request
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var existing models.User
	if err := h.db.Where("email = ? OR username = ? OR stellar_address = ?",
		req.Email, req.Username, req.StellarAddress).First(&existing).Error; err == nil {
		apperrors.AbortWithError(c, apperrors.NewConflictError("User already exists with this email, username, or Stellar address"))
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.config.BcryptCost)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to hash password"))
		return
	}

	emailToken := generateSecureToken()
	user := models.User{
		StellarAddress:    req.StellarAddress,
		Email:             req.Email,
		Username:          req.Username,
		PasswordHash:      string(hashedPassword),
		Role:              models.RoleUser,
		EmailVerified:     false,
		EmailToken:        emailToken,
		EmailTokenExpires: time.Now().Add(time.Hour * time.Duration(h.config.EmailTokenHours)),
	}

	if err := h.db.Create(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to create user"))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully. Please check your email for verification.",
		"user_id": user.ID,
	})
}

// Login handles user authentication
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid email or password"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid email or password"))
		return
	}

	if !user.EmailVerified {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("Please verify your email before logging in"))
		return
	}

	now := time.Now()
	user.LastLoginAt = &now
	h.db.Save(&user)

	accessToken, refreshToken, err := h.generateTokens(&user)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to generate tokens"))
		return
	}

	session := models.UserSession{
		UserID:       user.ID,
		SessionToken: generateSecureToken(),
		IPAddress:    c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		ExpiresAt:    time.Now().Add(time.Hour * time.Duration(h.config.RefreshTokenHours)),
	}
	if err := h.db.Create(&session).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to create session"))
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.config.JWTExpirationHours * 3600),
		User:         toUserInfo(&user),
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid refresh token"))
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid token claims"))
		return
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid token claims"))
		return
	}

	var user models.User
	if err := h.db.First(&user, uint(userIDFloat)).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not found"))
		return
	}

	accessToken, refreshToken, err := h.generateTokens(&user)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to generate tokens"))
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.config.JWTExpirationHours * 3600),
		User:         toUserInfo(&user),
	})
}

// VerifyEmail handles email verification
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var user models.User
	if err := h.db.Where("email_token = ? AND email_token_expires > ?", req.Token, time.Now()).First(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Invalid or expired verification token"))
		return
	}

	user.EmailVerified = true
	user.EmailToken = ""
	user.EmailTokenExpires = time.Time{}
	if err := h.db.Save(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to verify email"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

// ForgotPassword handles forgot password requests
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal whether the email exists
		c.JSON(http.StatusOK, gin.H{"message": "If an account with this email exists, a password reset link has been sent."})
		return
	}

	user.PasswordResetToken = generateSecureToken()
	user.PasswordResetExpires = time.Now().Add(time.Hour * time.Duration(h.config.PasswordResetHours))
	if err := h.db.Save(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to generate reset token"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account with this email exists, a password reset link has been sent."})
}

// ResetPassword handles password reset
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var user models.User
	if err := h.db.Where("password_reset_token = ? AND password_reset_expires > ?", req.Token, time.Now()).First(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Invalid or expired reset token"))
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.config.BcryptCost)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to hash password"))
		return
	}

	user.PasswordHash = string(hashedPassword)
	user.PasswordResetToken = ""
	user.PasswordResetExpires = time.Time{}
	if err := h.db.Save(&user).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to reset password"))
		return
	}

	// Invalidate all sessions for security
	h.db.Where("user_id = ?", user.ID).Delete(&models.UserSession{})

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	if err := h.db.Where("user_id = ?", userID).Delete(&models.UserSession{}).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to logout"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetProfile returns the current user's profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("User not found"))
		return
	}

	c.JSON(http.StatusOK, toUserInfo(&user))
}

func (h *AuthHandler) generateTokens(user *models.User) (string, string, error) {
	accessClaims := jwt.MapClaims{
		"user_id":  user.ID,
		"email":    user.Email,
		"username": user.Username,
		"role":     string(user.Role),
		"exp":      time.Now().Add(time.Hour * time.Duration(h.config.JWTExpirationHours)).Unix(),
		"iat":      time.Now().Unix(),
		"type":     "access",
		"jti":      uuid.New().String(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(h.config.JWTSecret))
	if err != nil {
		return "", "", err
	}

	refreshClaims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * time.Duration(h.config.RefreshTokenHours)).Unix(),
		"iat":     time.Now().Unix(),
		"type":    "refresh",
		"jti":     uuid.New().String(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(h.config.JWTSecret))
	if err != nil {
		return "", "", err
	}

	return accessStr, refreshStr, nil
}

func generateSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func toUserInfo(u *models.User) UserInfo {
	return UserInfo{
		ID:             u.ID,
		StellarAddress: u.StellarAddress,
		Email:          u.Email,
		Username:       u.Username,
		Role:           string(u.Role),
		EmailVerified:  u.EmailVerified,
		KYCVerified:    u.KYCVerified,
		LastLoginAt:    u.LastLoginAt,
	}
}
