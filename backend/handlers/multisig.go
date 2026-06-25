package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

type MultiSigHandler struct {
	db *gorm.DB
}

func NewMultiSigHandler(db *gorm.DB) *MultiSigHandler {
	return &MultiSigHandler{db: db}
}

type createWalletRequest struct {
	Name      string `json:"name" binding:"required,max=255"`
	OwnerIDs  []uint `json:"owner_ids" binding:"required,min=1"`
	Threshold int    `json:"threshold" binding:"required,gt=0"`
}

type createProposalRequest struct {
	ToAddress   string `json:"to_address" binding:"required"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	Description string `json:"description" binding:"omitempty,max=2000"`
}

func (h *MultiSigHandler) CreateWallet(c *gin.Context) {
	var req createWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError(err.Error(), err))
		return
	}

	if req.Threshold <= 0 || req.Threshold > len(req.OwnerIDs) {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("threshold must be > 0 and <= number of owners"))
		return
	}

	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	callerIsOwner := false
	for _, id := range req.OwnerIDs {
		if id == callerID {
			callerIsOwner = true
			break
		}
	}
	if !callerIsOwner {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("caller must be in owner_ids"))
		return
	}

	for _, ownerID := range req.OwnerIDs {
		var user models.User
		if err := h.db.First(&user, ownerID).Error; err != nil {
			apperrors.AbortWithError(c, apperrors.NewBadRequestError(fmt.Sprintf("user %d not found", ownerID)))
			return
		}
	}

	ownerIDsJSON, err := json.Marshal(req.OwnerIDs)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to serialize owner_ids"))
		return
	}

	wallet := models.MultiSigWallet{
		Name:        req.Name,
		OwnerIDs:    string(ownerIDsJSON),
		Threshold:   req.Threshold,
		CreatedByID: callerID,
	}

	if err := h.db.Create(&wallet).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to create wallet"))
		return
	}

	c.JSON(http.StatusCreated, wallet)
}

func (h *MultiSigHandler) GetWallet(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	var wallet models.MultiSigWallet
	if err := h.db.First(&wallet, id).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("wallet not found"))
		return
	}

	if !isOwnerOf(wallet.OwnerIDs, callerID) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("not an owner of this wallet"))
		return
	}

	c.JSON(http.StatusOK, wallet)
}

func (h *MultiSigHandler) ListWallets(c *gin.Context) {
	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	pattern := fmt.Sprintf("%%%d%%", callerID)
	var wallets []models.MultiSigWallet
	if err := h.db.Where("owner_ids LIKE ? AND deleted_at IS NULL", pattern).Find(&wallets).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to list wallets"))
		return
	}

	// Filter to only wallets where the caller is actually an owner (not just a substring match)
	filtered := make([]models.MultiSigWallet, 0, len(wallets))
	for _, w := range wallets {
		if isOwnerOf(w.OwnerIDs, callerID) {
			filtered = append(filtered, w)
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": filtered})
}

func (h *MultiSigHandler) CreateProposal(c *gin.Context) {
	walletID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	var wallet models.MultiSigWallet
	if err := h.db.First(&wallet, walletID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("wallet not found"))
		return
	}

	if !isOwnerOf(wallet.OwnerIDs, callerID) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("not an owner of this wallet"))
		return
	}

	var req createProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError(err.Error(), err))
		return
	}

	proposal := models.MultiSigProposal{
		WalletID:    walletID,
		ProposerID:  callerID,
		ToAddress:   req.ToAddress,
		Amount:      req.Amount,
		Description: req.Description,
		Status:      "pending",
		SignCount:   0,
	}

	if err := h.db.Create(&proposal).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to create proposal"))
		return
	}

	c.JSON(http.StatusCreated, proposal)
}

func (h *MultiSigHandler) ListProposals(c *gin.Context) {
	walletID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	var wallet models.MultiSigWallet
	if err := h.db.First(&wallet, walletID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("wallet not found"))
		return
	}

	if !isOwnerOf(wallet.OwnerIDs, callerID) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("not an owner of this wallet"))
		return
	}

	var proposals []models.MultiSigProposal
	if err := h.db.Where("wallet_id = ? AND deleted_at IS NULL", walletID).Find(&proposals).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to list proposals"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": proposals})
}

func (h *MultiSigHandler) SignProposal(c *gin.Context) {
	proposalID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	var proposal models.MultiSigProposal
	if err := h.db.First(&proposal, proposalID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("proposal not found"))
		return
	}

	var wallet models.MultiSigWallet
	if err := h.db.First(&wallet, proposal.WalletID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("wallet not found"))
		return
	}

	if !isOwnerOf(wallet.OwnerIDs, callerID) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("not an owner of this wallet"))
		return
	}

	var existing models.MultiSigSignature
	if err := h.db.Where("proposal_id = ? AND signer_id = ?", proposalID, callerID).First(&existing).Error; err == nil {
		apperrors.AbortWithError(c, apperrors.NewConflictError("already signed this proposal"))
		return
	}

	sig := models.MultiSigSignature{
		ProposalID: proposalID,
		SignerID:   callerID,
	}
	if err := h.db.Create(&sig).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to record signature"))
		return
	}

	proposal.SignCount++
	if proposal.SignCount >= wallet.Threshold {
		proposal.Status = "approved"
	}

	if err := h.db.Save(&proposal).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to update proposal"))
		return
	}

	c.JSON(http.StatusOK, proposal)
}

func (h *MultiSigHandler) ExecuteProposal(c *gin.Context) {
	proposalID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	callerID := userID(c)
	if callerID == 0 {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("authentication required"))
		return
	}

	var proposal models.MultiSigProposal
	if err := h.db.First(&proposal, proposalID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("proposal not found"))
		return
	}

	var wallet models.MultiSigWallet
	if err := h.db.First(&wallet, proposal.WalletID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("wallet not found"))
		return
	}

	if !isOwnerOf(wallet.OwnerIDs, callerID) {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("not an owner of this wallet"))
		return
	}

	if proposal.Status != "approved" {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("proposal must be approved before execution"))
		return
	}

	proposal.Status = "executed"
	if err := h.db.Save(&proposal).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("failed to execute proposal"))
		return
	}

	c.JSON(http.StatusOK, proposal)
}

func isOwnerOf(ownerIDsJSON string, userID uint) bool {
	var ids []uint
	if err := json.Unmarshal([]byte(ownerIDsJSON), &ids); err != nil {
		return false
	}
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

func parseUintParam(c *gin.Context, param string) (uint, bool) {
	val, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || val == 0 {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError(fmt.Sprintf("invalid %s", param)))
		return 0, false
	}
	return uint(val), true
}
