package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

// LegalHandler manages legal documents, consent, and GDPR data exports
type LegalHandler struct {
	db *gorm.DB
}

// NewLegalHandler creates a new handler
func NewLegalHandler(db *gorm.DB) *LegalHandler {
	return &LegalHandler{db: db}
}

// GetActiveDocument returns the current active version of a legal document
// @Summary Get active legal document
// @Tags legal
// @Param type path string true "Document type (terms_of_service, privacy_policy, cookie_policy)"
// @Produce json
// @Success 200 {object} models.LegalDocument
// @Router /api/v1/legal/:type [get]
func (h *LegalHandler) GetActiveDocument(c *gin.Context) {
	docType := models.LegalDocumentType(c.Param("type"))
	var doc models.LegalDocument
	if err := h.db.Where("type = ? AND active = ?", docType, true).
		Order("effective_at DESC").First(&doc).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeNotFound, "document not found", http.StatusNotFound))
		return
	}
	c.JSON(http.StatusOK, doc)
}

// ListDocumentVersions returns all versions of a legal document type
// @Summary List legal document versions
// @Tags legal
// @Param type path string true "Document type"
// @Success 200 {array} models.LegalDocument
// @Router /api/v1/legal/:type/versions [get]
func (h *LegalHandler) ListDocumentVersions(c *gin.Context) {
	docType := models.LegalDocumentType(c.Param("type"))
	var docs []models.LegalDocument
	h.db.Where("type = ?", docType).Order("effective_at DESC").Find(&docs)
	c.JSON(http.StatusOK, docs)
}

type recordConsentRequest struct {
	DocType models.LegalDocumentType `json:"doc_type" binding:"required"`
	Version string                   `json:"version" binding:"required"`
}

// RecordConsent stores the user's acceptance of a legal document version
// @Summary Record user consent
// @Tags legal
// @Accept json
// @Produce json
// @Param consent body recordConsentRequest true "Consent details"
// @Success 201 {object} models.UserConsent
// @Router /api/v1/legal/consent [post]
func (h *LegalHandler) RecordConsent(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req recordConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeBadRequest, err.Error(), http.StatusBadRequest))
		return
	}

	var doc models.LegalDocument
	if err := h.db.Where("type = ? AND version = ?", req.DocType, req.Version).
		First(&doc).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeNotFound, "document version not found", http.StatusNotFound))
		return
	}

	consent := models.UserConsent{
		UserID:     userID,
		DocumentID: doc.ID,
		DocType:    req.DocType,
		Version:    req.Version,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		AcceptedAt: time.Now(),
	}
	if err := h.db.Create(&consent).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeInternalServerError, "failed to record consent", http.StatusInternalServerError))
		return
	}
	c.JSON(http.StatusCreated, consent)
}

// GetConsentHistory returns the user's full consent history
// @Summary Get consent history
// @Tags legal
// @Success 200 {array} models.UserConsent
// @Router /api/v1/legal/consent/history [get]
func (h *LegalHandler) GetConsentHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	var consents []models.UserConsent
	h.db.Where("user_id = ?", userID).Preload("Document").
		Order("accepted_at DESC").Find(&consents)
	c.JSON(http.StatusOK, consents)
}

// CheckPendingConsents returns document types that require fresh user consent
// @Summary Check pending consents
// @Tags legal
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/legal/consent/pending [get]
func (h *LegalHandler) CheckPendingConsents(c *gin.Context) {
	userID := c.GetUint("user_id")
	docTypes := []models.LegalDocumentType{
		models.DocTypeTermsOfService,
		models.DocTypePrivacyPolicy,
	}

	pending := []gin.H{}
	for _, dt := range docTypes {
		var current models.LegalDocument
		if err := h.db.Where("type = ? AND active = ?", dt, true).
			Order("effective_at DESC").First(&current).Error; err != nil {
			continue
		}

		var consent models.UserConsent
		err := h.db.Where("user_id = ? AND doc_type = ? AND version = ?", userID, dt, current.Version).
			First(&consent).Error
		if err != nil {
			pending = append(pending, gin.H{
				"doc_type": dt,
				"version":  current.Version,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"pending_consents": pending})
}

// RequestDataExport initiates a GDPR data export for the authenticated user
// @Summary Request GDPR data export
// @Tags legal
// @Produce json
// @Success 202 {object} models.DataExportRequest
// @Router /api/v1/legal/gdpr/export [post]
func (h *LegalHandler) RequestDataExport(c *gin.Context) {
	userID := c.GetUint("user_id")

	// Prevent duplicate pending requests
	var existing models.DataExportRequest
	if h.db.Where("user_id = ? AND status = ?", userID, "pending").
		First(&existing).Error == nil {
		c.JSON(http.StatusAccepted, existing)
		return
	}

	req := models.DataExportRequest{
		UserID: userID,
		Status: "pending",
	}
	if err := h.db.Create(&req).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeInternalServerError, "failed to create export request", http.StatusInternalServerError))
		return
	}

	// Async data collection — in production this would be a background job
	go h.buildExport(userID, req.ID)

	c.JSON(http.StatusAccepted, req)
}

// GetDataExportStatus returns the status of a GDPR export request
// @Summary Get GDPR export status
// @Tags legal
// @Param id path int true "Export request ID"
// @Success 200 {object} models.DataExportRequest
// @Router /api/v1/legal/gdpr/export/:id [get]
func (h *LegalHandler) GetDataExportStatus(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req models.DataExportRequest
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), userID).
		First(&req).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.New(apperrors.CodeNotFound, "export request not found", http.StatusNotFound))
		return
	}
	c.JSON(http.StatusOK, req)
}

// buildExport collects all user data and marks the export ready.
// In a real deployment this runs as a background worker writing to object storage.
func (h *LegalHandler) buildExport(userID, requestID uint) {
	var user models.User
	h.db.First(&user, userID)

	var assets []models.Asset
	h.db.Where("owner_address = ?", user.StellarAddress).Find(&assets)

	var transactions []models.Transaction
	h.db.Where("from_address = ? OR to_address = ?", user.StellarAddress, user.StellarAddress).Find(&transactions)

	var consents []models.UserConsent
	h.db.Where("user_id = ?", userID).Find(&consents)

	export := map[string]interface{}{
		"user":         user,
		"assets":       assets,
		"transactions": transactions,
		"consents":     consents,
	}

	payload, _ := json.Marshal(export)
	now := time.Now()
	expiry := now.Add(7 * 24 * time.Hour)

	h.db.Model(&models.DataExportRequest{}).Where("id = ?", requestID).Updates(map[string]interface{}{
		"status":       "completed",
		"download_url": fmt.Sprintf("/api/v1/legal/gdpr/export/%d/download", requestID),
		"completed_at": &now,
		"expires_at":   &expiry,
	})
	_ = payload
}
