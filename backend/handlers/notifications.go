package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

// NotificationHandler manages in-app notifications and preferences
type NotificationHandler struct {
	db *gorm.DB
}

// NewNotificationHandler creates a new handler
func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// ListNotifications returns notifications for the authenticated user
// @Summary List notifications
// @Tags notifications
// @Produce json
// @Param unread_only query bool false "Return only unread notifications"
// @Success 200 {array} models.Notification
// @Router /api/v1/notifications [get]
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	userID := c.GetUint("user_id")
	query := h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(50)
	if c.Query("unread_only") == "true" {
		query = query.Where("read = ?", false)
	}
	var notifs []models.Notification
	query.Find(&notifs)
	c.JSON(http.StatusOK, notifs)
}

// MarkRead marks one or all notifications as read
// @Summary Mark notifications as read
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path int false "Notification ID (omit to mark all read)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/notifications/:id/read [put]
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := c.Param("id")

	query := h.db.Model(&models.Notification{}).Where("user_id = ?", userID)
	if id != "" && id != "all" {
		query = query.Where("id = ?", id)
	}

	result := query.Updates(map[string]interface{}{"read": true})
	c.JSON(http.StatusOK, gin.H{"updated": result.RowsAffected})
}

// MarkAllRead marks every unread notification for the user as read
// @Summary Mark all notifications as read
// @Tags notifications
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/notifications/read-all [put]
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := c.GetUint("user_id")
	result := h.db.Model(&models.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Updates(map[string]interface{}{"read": true})
	c.JSON(http.StatusOK, gin.H{"updated": result.RowsAffected})
}

// GetPreferences returns notification channel preferences
// @Summary Get notification preferences
// @Tags notifications
// @Success 200 {array} models.NotificationPreference
// @Router /api/v1/notifications/preferences [get]
func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	userID := c.GetUint("user_id")
	var prefs []models.NotificationPreference
	h.db.Where("user_id = ?", userID).Find(&prefs)
	c.JSON(http.StatusOK, prefs)
}

type updatePreferenceRequest struct {
	NotificationType models.NotificationType `json:"notification_type" binding:"required"`
	InApp            *bool                   `json:"in_app"`
	Email            *bool                   `json:"email"`
	Push             *bool                   `json:"push"`
}

// UpdatePreference creates or updates a notification preference
// @Summary Update notification preference
// @Tags notifications
// @Accept json
// @Produce json
// @Param preference body updatePreferenceRequest true "Preference settings"
// @Success 200 {object} models.NotificationPreference
// @Router /api/v1/notifications/preferences [put]
func (h *NotificationHandler) UpdatePreference(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req updatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeBadRequest, err.Error(), http.StatusBadRequest))
		return
	}

	var pref models.NotificationPreference
	h.db.Where("user_id = ? AND notification_type = ?", userID, req.NotificationType).FirstOrCreate(&pref, models.NotificationPreference{
		UserID:           userID,
		NotificationType: req.NotificationType,
		InApp:            true,
		Email:            true,
		Push:             false,
	})

	if req.InApp != nil {
		pref.InApp = *req.InApp
	}
	if req.Email != nil {
		pref.Email = *req.Email
	}
	if req.Push != nil {
		pref.Push = *req.Push
	}

	h.db.Save(&pref)
	c.JSON(http.StatusOK, pref)
}

// UnreadCount returns the count of unread notifications
// @Summary Get unread notification count
// @Tags notifications
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/notifications/unread-count [get]
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	userID := c.GetUint("user_id")
	var count int64
	h.db.Model(&models.Notification{}).Where("user_id = ? AND read = ?", userID, false).Count(&count)
	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}
