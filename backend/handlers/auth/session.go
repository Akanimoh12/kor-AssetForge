package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
)

// SessionHandler handles device session management endpoints.
type SessionHandler struct {
	db *gorm.DB
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{db: db}
}

// ListSessions returns all active DeviceSessions for the current user.
func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	var sessions []models.DeviceSession
	if err := h.db.Where("user_id = ? AND is_revoked = false AND expires_at > ?", userID, time.Now()).
		Order("last_active_at desc").
		Find(&sessions).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to fetch sessions"))
		return
	}

	// Identify current session from X-Session-Token header.
	currentToken := c.GetHeader("X-Session-Token")

	summaries := make([]models.SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		summary := models.SessionSummary{
			ID:                s.ID,
			DeviceFingerprint: s.DeviceFingerprint,
			DeviceType:        s.DeviceType,
			Browser:           s.Browser,
			OS:                s.OS,
			IPAddress:         s.IPAddress,
			CountryCode:       s.CountryCode,
			City:              s.City,
			LastActiveAt:      s.LastActiveAt,
			ExpiresAt:         s.ExpiresAt,
			IsRevoked:         s.IsRevoked,
			CreatedAt:         s.CreatedAt,
			IsCurrent:         currentToken != "" && s.SessionToken == currentToken,
		}
		summaries = append(summaries, summary)
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": summaries,
		"count":    len(summaries),
	})
}

// RevokeSession revokes a specific session by ID.
func (h *SessionHandler) RevokeSession(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Session ID is required"))
		return
	}

	var session models.DeviceSession
	if err := h.db.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Session not found"))
		return
	}

	if session.IsRevoked {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Session is already revoked"))
		return
	}

	now := time.Now()
	uid := userID.(uint)
	if err := h.db.Model(&session).Updates(map[string]interface{}{
		"is_revoked": true,
		"revoked_at": &now,
		"revoked_by": &uid,
	}).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to revoke session"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session revoked successfully"})
}

// RevokeAllSessions revokes all active sessions for the user (logout all devices).
func (h *SessionHandler) RevokeAllSessions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	now := time.Now()
	uid := userID.(uint)
	if err := h.db.Model(&models.DeviceSession{}).
		Where("user_id = ? AND is_revoked = false", userID).
		Updates(map[string]interface{}{
			"is_revoked": true,
			"revoked_at": &now,
			"revoked_by": &uid,
		}).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to revoke sessions"))
		return
	}

	// Also revoke legacy UserSession records.
	h.db.Where("user_id = ?", userID).Delete(&models.UserSession{})

	c.JSON(http.StatusOK, gin.H{"message": "All sessions revoked successfully"})
}

// GetCurrentSession returns info about the current session.
func (h *SessionHandler) GetCurrentSession(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	token := c.GetHeader("X-Session-Token")

	var session models.DeviceSession
	q := h.db.Where("user_id = ? AND is_revoked = false", userID)
	if token != "" {
		q = q.Where("session_token = ?", token)
	}
	if err := q.Order("last_active_at desc").First(&session).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"session":         nil,
			"timeout_warning": false,
		})
		return
	}

	timeoutWarning := time.Until(session.ExpiresAt) < 10*time.Minute

	c.JSON(http.StatusOK, gin.H{
		"session": models.SessionSummary{
			ID:           session.ID,
			DeviceType:   session.DeviceType,
			Browser:      session.Browser,
			OS:           session.OS,
			IPAddress:    session.IPAddress,
			CountryCode:  session.CountryCode,
			City:         session.City,
			LastActiveAt: session.LastActiveAt,
			ExpiresAt:    session.ExpiresAt,
			IsRevoked:    session.IsRevoked,
			CreatedAt:    session.CreatedAt,
			IsCurrent:    true,
		},
		"timeout_warning":    timeoutWarning,
		"expires_in_seconds": int(time.Until(session.ExpiresAt).Seconds()),
	})
}
