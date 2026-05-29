package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/utils"
	"gorm.io/gorm"
)

// DisputeHandler handles dispute-related HTTP requests
type DisputeHandler struct {
	db *gorm.DB
}

// NewDisputeHandler creates a new DisputeHandler
func NewDisputeHandler(db *gorm.DB) *DisputeHandler {
	return &DisputeHandler{db: db}
}

type fileDisputeRequest struct {
	TransactionID  uint   `json:"transaction_id" binding:"required,gt=0"`
	FiledByAddress string `json:"filed_by_address" binding:"required"`
	RespondentAddr string `json:"respondent_address" binding:"required"`
	Reason         string `json:"reason" binding:"required,min=10,max=2000"`
	Evidence       string `json:"evidence" binding:"omitempty,max=5000"`
}

type resolveDisputeRequest struct {
	Resolution models.DisputeResolution `json:"resolution" binding:"required,oneof=buyer_favor seller_favor split"`
	AdminNotes string                   `json:"admin_notes" binding:"omitempty,max=2000"`
}

// FileDispute submits a new dispute for a transaction
// @Summary File a dispute
// @Description Submit a dispute for a completed transaction; funds are moved to escrow
// @Tags disputes
// @Accept json
// @Produce json
// @Param dispute body fileDisputeRequest true "Dispute details"
// @Success 201 {object} models.Dispute
// @Failure 400 {object} apperrors.ErrorResponse
// @Failure 404 {object} apperrors.ErrorResponse
// @Router /disputes [post]
func (h *DisputeHandler) FileDispute(c *gin.Context) {
	var req fileDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure the transaction exists
	var tx models.Transaction
	if err := h.db.First(&tx, req.TransactionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Only allow filing against confirmed transactions
	if tx.Status != "confirmed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only dispute confirmed transactions"})
		return
	}

	// Check no existing open dispute for this transaction
	var existing models.Dispute
	if err := h.db.Where("transaction_id = ? AND status NOT IN ?", req.TransactionID,
		[]string{string(models.DisputeStatusResolved), string(models.DisputeStatusRejected)}).
		First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An active dispute already exists for this transaction"})
		return
	}

	dispute := models.Dispute{
		TransactionID:  req.TransactionID,
		FiledByAddress: req.FiledByAddress,
		RespondentAddr: req.RespondentAddr,
		Reason:         req.Reason,
		Evidence:       req.Evidence,
		Status:         models.DisputeStatusOpen,
		EscrowAmount:   tx.Amount,
	}

	if err := h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := dbTx.Create(&dispute).Error; err != nil {
			return err
		}
		escrow := models.DisputeEscrow{
			DisputeID: dispute.ID,
			AssetID:   tx.AssetID,
			Amount:    tx.Amount,
			HeldFrom:  time.Now(),
		}
		return dbTx.Create(&escrow).Error
	}); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to create dispute", http.StatusInternalServerError))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Dispute filed successfully. Funds held in escrow.",
		"dispute": dispute,
	})
}

// GetDispute returns details for a single dispute
// @Summary Get dispute
// @Description Retrieve a dispute and its escrow by ID
// @Tags disputes
// @Produce json
// @Param id path int true "Dispute ID"
// @Success 200 {object} models.Dispute
// @Failure 404 {object} apperrors.ErrorResponse
// @Router /disputes/{id} [get]
func (h *DisputeHandler) GetDispute(c *gin.Context) {
	var uri struct {
		ID uint `uri:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dispute ID"})
		return
	}

	var dispute models.Dispute
	if err := h.db.Preload("Transaction").First(&dispute, uri.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dispute not found"})
		return
	}

	var escrow models.DisputeEscrow
	h.db.Where("dispute_id = ?", dispute.ID).First(&escrow)

	c.JSON(http.StatusOK, gin.H{"dispute": dispute, "escrow": escrow})
}

// ListDisputes returns disputes with optional status filter
// @Summary List disputes
// @Description Paginated list of disputes, optionally filtered by status or address
// @Tags disputes
// @Produce json
// @Param status query string false "Filter by status"
// @Param address query string false "Filter by filed_by_address or respondent_address"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} utils.Pagination
// @Router /disputes [get]
func (h *DisputeHandler) ListDisputes(c *gin.Context) {
	type query struct {
		Status  string `form:"status"`
		Address string `form:"address"`
		Page    int    `form:"page,default=1"`
		Limit   int    `form:"limit,default=10"`
	}

	var q query
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 10
	}

	db := h.db.Model(&models.Dispute{})
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.Address != "" {
		db = db.Where("filed_by_address = ? OR respondent_addr = ?", q.Address, q.Address)
	}

	var disputes []models.Dispute
	var total int64
	paginationRes, err := utils.Paginate(db, c, q.Page, q.Limit, &total, &disputes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch disputes"})
		return
	}
	c.JSON(http.StatusOK, paginationRes)
}

// AdminReviewDispute allows an admin to update dispute status during review
// @Summary Admin: update dispute status
// @Description Mark a dispute as under_review
// @Tags disputes
// @Param id path int true "Dispute ID"
// @Success 200 {object} models.Dispute
// @Router /admin/disputes/{id}/review [put]
func (h *DisputeHandler) AdminReviewDispute(c *gin.Context) {
	var uri struct {
		ID uint `uri:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dispute ID"})
		return
	}

	var dispute models.Dispute
	if err := h.db.First(&dispute, uri.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dispute not found"})
		return
	}

	if dispute.Status != models.DisputeStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dispute is not in open state"})
		return
	}

	dispute.Status = models.DisputeStatusUnderReview
	if err := h.db.Save(&dispute).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update dispute"})
		return
	}

	c.JSON(http.StatusOK, dispute)
}

// AdminResolveDispute resolves a dispute and releases escrow
// @Summary Admin: resolve dispute
// @Description Resolve a dispute with a decision and release escrowed funds
// @Tags disputes
// @Accept json
// @Produce json
// @Param id path int true "Dispute ID"
// @Param body body resolveDisputeRequest true "Resolution"
// @Success 200 {object} models.Dispute
// @Router /admin/disputes/{id}/resolve [put]
func (h *DisputeHandler) AdminResolveDispute(c *gin.Context) {
	var uri struct {
		ID uint `uri:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dispute ID"})
		return
	}

	var req resolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dispute models.Dispute
	if err := h.db.First(&dispute, uri.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dispute not found"})
		return
	}

	if dispute.Status == models.DisputeStatusResolved || dispute.Status == models.DisputeStatusRejected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dispute is already closed"})
		return
	}

	now := time.Now()
	// Determine escrow release recipient based on resolution
	var releaseToAddr string
	switch req.Resolution {
	case models.ResolutionBuyerFavor:
		releaseToAddr = dispute.FiledByAddress
	case models.ResolutionSellerFavor:
		releaseToAddr = dispute.RespondentAddr
	case models.ResolutionSplit:
		releaseToAddr = "split"
	}

	if err := h.db.Transaction(func(dbTx *gorm.DB) error {
		dispute.Status = models.DisputeStatusResolved
		dispute.Resolution = req.Resolution
		dispute.AdminNotes = req.AdminNotes
		dispute.EscrowReleased = true
		dispute.ResolvedAt = &now
		if err := dbTx.Save(&dispute).Error; err != nil {
			return err
		}

		return dbTx.Model(&models.DisputeEscrow{}).
			Where("dispute_id = ?", dispute.ID).
			Updates(map[string]interface{}{
				"released_at": now,
				"released_to": releaseToAddr,
				"updated_at":  now,
			}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve dispute"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Dispute resolved and escrow released",
		"dispute":      dispute,
		"released_to":  releaseToAddr,
	})
}

// GetDisputeHistory returns resolved disputes for an address
// @Summary Dispute resolution history
// @Description Fetch resolved disputes for a given address
// @Tags disputes
// @Produce json
// @Param address query string true "Stellar address"
// @Success 200 {array} models.Dispute
// @Router /disputes/history [get]
func (h *DisputeHandler) GetDisputeHistory(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address query parameter required"})
		return
	}

	var disputes []models.Dispute
	if err := h.db.
		Where("(filed_by_address = ? OR respondent_addr = ?) AND status IN ?",
			address, address,
			[]string{string(models.DisputeStatusResolved), string(models.DisputeStatusRejected)}).
		Order("resolved_at DESC").
		Find(&disputes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dispute history"})
		return
	}

	c.JSON(http.StatusOK, disputes)
}
