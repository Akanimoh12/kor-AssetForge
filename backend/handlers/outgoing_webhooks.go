package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

// OutgoingWebhookHandler manages webhook subscriptions and delivery logs
type OutgoingWebhookHandler struct {
	db *gorm.DB
}

// NewOutgoingWebhookHandler creates a new handler
func NewOutgoingWebhookHandler(db *gorm.DB) *OutgoingWebhookHandler {
	return &OutgoingWebhookHandler{db: db}
}

type createSubscriptionRequest struct {
	URL         string   `json:"url" binding:"required,url"`
	Events      []string `json:"events" binding:"required,min=1"`
	Description string   `json:"description" binding:"omitempty,max=255"`
}

type updateSubscriptionRequest struct {
	URL         string   `json:"url" binding:"omitempty,url"`
	Events      []string `json:"events" binding:"omitempty,min=1"`
	Description string   `json:"description" binding:"omitempty,max=255"`
	Active      *bool    `json:"active"`
}

// CreateSubscription registers a new outgoing webhook endpoint
// @Summary Register a webhook subscription
// @Description Register an HTTPS endpoint to receive outgoing webhook events
// @Tags webhooks
// @Accept json
// @Produce json
// @Param subscription body createSubscriptionRequest true "Subscription details"
// @Success 201 {object} models.WebhookSubscription
// @Failure 400 {object} apperrors.ErrorResponse
// @Router /api/v1/webhooks/subscriptions [post]
func (h *OutgoingWebhookHandler) CreateSubscription(c *gin.Context) {
	var req createSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeBadRequest, err.Error(), http.StatusBadRequest))
		return
	}

	userID := c.GetUint("user_id")

	secret, err := generateSecret()
	if err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeInternalServerError, "failed to generate secret", http.StatusInternalServerError))
		return
	}

	sub := models.WebhookSubscription{
		UserID:      userID,
		URL:         req.URL,
		Secret:      secret,
		Events:      strings.Join(req.Events, ","),
		Description: req.Description,
		Active:      true,
	}

	if err := h.db.Create(&sub).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeInternalServerError, "failed to create subscription", http.StatusInternalServerError))
		return
	}

	// Return the secret only on creation
	resp := gin.H{
		"id":          sub.ID,
		"url":         sub.URL,
		"events":      req.Events,
		"description": sub.Description,
		"active":      sub.Active,
		"secret":      secret,
		"created_at":  sub.CreatedAt,
	}
	c.JSON(http.StatusCreated, resp)
}

// ListSubscriptions returns all webhook subscriptions for the authenticated user
// @Summary List webhook subscriptions
// @Tags webhooks
// @Produce json
// @Success 200 {array} models.WebhookSubscription
// @Router /api/v1/webhooks/subscriptions [get]
func (h *OutgoingWebhookHandler) ListSubscriptions(c *gin.Context) {
	userID := c.GetUint("user_id")
	var subs []models.WebhookSubscription
	h.db.Where("user_id = ?", userID).Find(&subs)
	c.JSON(http.StatusOK, subs)
}

// UpdateSubscription updates a webhook subscription
// @Summary Update a webhook subscription
// @Tags webhooks
// @Accept json
// @Produce json
// @Param id path int true "Subscription ID"
// @Param body body updateSubscriptionRequest true "Fields to update"
// @Success 200 {object} models.WebhookSubscription
// @Router /api/v1/webhooks/subscriptions/:id [put]
func (h *OutgoingWebhookHandler) UpdateSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	var sub models.WebhookSubscription
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&sub).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeNotFound, "subscription not found", http.StatusNotFound))
		return
	}

	var req updateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeBadRequest, err.Error(), http.StatusBadRequest))
		return
	}

	if req.URL != "" {
		sub.URL = req.URL
	}
	if len(req.Events) > 0 {
		sub.Events = strings.Join(req.Events, ",")
	}
	if req.Description != "" {
		sub.Description = req.Description
	}
	if req.Active != nil {
		sub.Active = *req.Active
	}

	h.db.Save(&sub)
	c.JSON(http.StatusOK, sub)
}

// DeleteSubscription removes a webhook subscription
// @Summary Delete a webhook subscription
// @Tags webhooks
// @Param id path int true "Subscription ID"
// @Success 204
// @Router /api/v1/webhooks/subscriptions/:id [delete]
func (h *OutgoingWebhookHandler) DeleteSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), userID).
		Delete(&models.WebhookSubscription{}).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeNotFound, "subscription not found", http.StatusNotFound))
		return
	}
	c.Status(http.StatusNoContent)
}

// GetDeliveryLogs returns delivery logs for a subscription
// @Summary Get webhook delivery logs
// @Tags webhooks
// @Param id path int true "Subscription ID"
// @Success 200 {array} models.WebhookDeliveryLog
// @Router /api/v1/webhooks/subscriptions/:id/logs [get]
func (h *OutgoingWebhookHandler) GetDeliveryLogs(c *gin.Context) {
	userID := c.GetUint("user_id")
	var sub models.WebhookSubscription
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&sub).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeNotFound, "subscription not found", http.StatusNotFound))
		return
	}

	var logs []models.WebhookDeliveryLog
	h.db.Where("subscription_id = ?", sub.ID).Order("created_at DESC").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
