package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
)

// AuthMiddleware handles JWT authentication and authorization
type AuthMiddleware struct {
	jwtSecret string
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{jwtSecret: jwtSecret}
}

// JWTAuth middleware validates JWT access tokens
func (m *AuthMiddleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Authorization header required"))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid authorization header format"))
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, apperrors.NewUnauthorizedError("Invalid signing method")
			}
			return []byte(m.jwtSecret), nil
		})
		if err != nil || !token.Valid {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid token"))
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid token claims"))
			return
		}

		if tokenType, _ := claims["type"].(string); tokenType != "access" {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid token type"))
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("Invalid user ID in token"))
			return
		}

		c.Set("user_id", uint(userIDFloat))
		c.Set("email", claims["email"])
		c.Set("username", claims["username"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

// RequireRole middleware checks if user has the required role level
func (m *AuthMiddleware) RequireRole(requiredRole models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User role not found"))
			return
		}

		if !hasRequiredRole(models.UserRole(role.(string)), requiredRole) {
			apperrors.AbortWithError(c, apperrors.NewForbiddenError("Insufficient permissions"))
			return
		}

		c.Next()
	}
}

// RequireRoles middleware accepts any of the given roles
func (m *AuthMiddleware) RequireRoles(requiredRoles ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User role not found"))
			return
		}

		userRole := models.UserRole(role.(string))
		for _, r := range requiredRoles {
			if hasRequiredRole(userRole, r) {
				c.Next()
				return
			}
		}

		apperrors.AbortWithError(c, apperrors.NewForbiddenError("Insufficient permissions"))
	}
}

// OptionalAuth sets user context if a valid token is present, but does not require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, nil
			}
			return []byte(m.jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.Next()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Next()
			return
		}

		if tokenType, _ := claims["type"].(string); tokenType != "access" {
			c.Next()
			return
		}

		if userIDFloat, ok := claims["user_id"].(float64); ok {
			c.Set("user_id", uint(userIDFloat))
			c.Set("email", claims["email"])
			c.Set("username", claims["username"])
			c.Set("role", claims["role"])
			c.Set("authenticated", true)
		}

		c.Next()
	}
}

// roleLevel maps roles to numeric levels for hierarchy checks
var roleLevel = map[models.UserRole]int{
	models.RoleUser:      1,
	models.RoleModerator: 2,
	models.RoleAdmin:     3,
}

func hasRequiredRole(userRole, requiredRole models.UserRole) bool {
	return roleLevel[userRole] >= roleLevel[requiredRole]
}
