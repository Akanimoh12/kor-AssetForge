package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
	"gorm.io/gorm"
)

type BatchHandler struct {
	db        *gorm.DB
	processor *services.BatchProcessor
}

func NewBatchHandler(db *gorm.DB, processor *services.BatchProcessor) *BatchHandler {
	return &BatchHandler{db: db, processor: processor}
}

// SubmitBatch enqueues a batch of operations for async processing
// POST /batch/submit
func (h *BatchHandler) SubmitBatch(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	var req struct {
		Operations []models.BatchOperation `json:"operations" binding:"required"`
		Priority   int                     `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid batch request", err))
		return
	}

	if len(req.Operations) == 0 {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Batch must contain at least one operation"))
		return
	}
	if len(req.Operations) > models.MaxBatchSize {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError(fmt.Sprintf("Batch cannot exceed %d operations", models.MaxBatchSize)))
		return
	}

	opsJSON, _ := json.Marshal(req.Operations)
	batch := models.BatchTransaction{
		UserID:          userID.(uint),
		Operations:      string(opsJSON),
		Status:          models.BatchStatusQueued,
		TotalOperations: len(req.Operations),
	}
	if err := h.db.Create(&batch).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to create batch"))
		return
	}

	queueEntry := models.BatchQueueEntry{
		BatchID:     batch.ID,
		Priority:    req.Priority,
		MaxAttempts: 3,
		ScheduledAt: time.Now(),
	}
	h.db.Create(&queueEntry)

	if err := h.processor.Enqueue(batch.ID); err != nil {
		// Queue full — batch remains in "queued" state and will be picked up
		// by a future worker or retry mechanism
	}

	c.JSON(http.StatusAccepted, gin.H{
		"batch_id":         batch.ID,
		"status":           batch.Status,
		"total_operations": batch.TotalOperations,
		"message":          "Batch queued for processing",
	})
}

// GetBatchResult returns the current status and results of a batch
// GET /batch/results/:id
func (h *BatchHandler) GetBatchResult(c *gin.Context) {
	batchID := c.Param("id")
	if batchID == "" {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Batch ID is required"))
		return
	}

	var batch models.BatchTransaction
	if err := h.db.First(&batch, batchID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Batch not found"))
		return
	}

	userID, _ := c.Get("user_id")
	if batch.UserID != userID.(uint) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("Access denied"))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// ListBatches lists all batches for the authenticated user with optional status filter
// GET /batch/list
func (h *BatchHandler) ListBatches(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	status := c.Query("status")
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	var batches []models.BatchTransaction
	var total int64
	q := h.db.Model(&models.BatchTransaction{}).Where("user_id = ?", userID).Order("created_at desc")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	if err := q.Offset((page - 1) * limit).Limit(limit).Find(&batches).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to fetch batches"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"batches": batches,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// CancelBatch cancels a queued batch that hasn't started processing
// DELETE /batch/:id
func (h *BatchHandler) CancelBatch(c *gin.Context) {
	batchID := c.Param("id")
	userID, _ := c.Get("user_id")

	var batch models.BatchTransaction
	if err := h.db.First(&batch, batchID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Batch not found"))
		return
	}
	if batch.UserID != userID.(uint) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("Access denied"))
		return
	}
	if batch.Status != models.BatchStatusQueued {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Only queued batches can be cancelled"))
		return
	}

	h.db.Model(&batch).Update("status", models.BatchStatusRolledBack)
	h.db.Where("batch_id = ?", batch.ID).Delete(&models.BatchQueueEntry{})

	c.JSON(http.StatusOK, gin.H{"message": "Batch cancelled"})
}
