package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/yourusername/kor-assetforge/models"
)

// SessionValidator validates that the device session is still active and not revoked.
// It reads the X-Session-Token header and looks up the DeviceSession.
// Also updates LastActiveAt on valid sessions.
func SessionValidator(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Session-Token")
		if token == "" {
			c.Next()
			return
		}

		var session models.DeviceSession
		if err := db.Where("session_token = ? AND is_revoked = false AND expires_at > ?", token, time.Now()).
			First(&session).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Session is invalid or has been revoked",
				"code":    "session_invalid",
			})
			return
		}

		// Update last active timestamp (non-blocking — ignore error).
		db.Model(&session).Update("last_active_at", time.Now())

		// Set session context for handlers.
		c.Set("device_session_id", session.ID)
		c.Set("session_ip", session.IPAddress)

		c.Next()
	}
}

// DeviceFingerprint extracts a simple device fingerprint from request headers.
func DeviceFingerprint(c *gin.Context) string {
	ua := c.GetHeader("User-Agent")
	accept := c.GetHeader("Accept-Language")
	encoding := c.GetHeader("Accept-Encoding")
	return fmt.Sprintf("%s|%s|%s", ua, accept, encoding)
}

// ParseDeviceInfo extracts browser, OS, and device type from User-Agent.
func ParseDeviceInfo(userAgent string) (browser, os, deviceType string) {
	ua := strings.ToLower(userAgent)

	switch {
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone"):
		deviceType = "mobile"
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		deviceType = "tablet"
	default:
		deviceType = "desktop"
	}

	switch {
	case strings.Contains(ua, "chrome"):
		browser = "Chrome"
	case strings.Contains(ua, "firefox"):
		browser = "Firefox"
	case strings.Contains(ua, "safari"):
		browser = "Safari"
	case strings.Contains(ua, "edge"):
		browser = "Edge"
	default:
		browser = "Unknown"
	}

	switch {
	case strings.Contains(ua, "windows"):
		os = "Windows"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macos"):
		os = "macOS"
	case strings.Contains(ua, "linux"):
		os = "Linux"
	case strings.Contains(ua, "android"):
		os = "Android"
	case strings.Contains(ua, "ios") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		os = "iOS"
	default:
		os = "Unknown"
	}

	return
}
