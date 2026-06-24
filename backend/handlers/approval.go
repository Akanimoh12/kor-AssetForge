package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
	"gorm.io/gorm"
)

type ApprovalHandler struct{ workflow *services.WorkflowService }

func NewApprovalHandler(workflow *services.WorkflowService) *ApprovalHandler {
	return &ApprovalHandler{workflow: workflow}
}

type createWorkflowRequest struct {
	Name          string                `json:"name" binding:"required,max=255"`
	AssetType     string                `json:"asset_type"`
	MinimumAmount int64                 `json:"minimum_amount" binding:"gte=0"`
	TimeoutHours  int                   `json:"timeout_hours" binding:"required,min=1,max=720"`
	Active        *bool                 `json:"active"`
	Steps         []models.ApprovalStep `json:"steps" binding:"required,min=1,max=10"`
}

type createApprovalRequest struct {
	AssetID     uint   `json:"asset_id" binding:"required,gt=0"`
	FromAddress string `json:"from_address" binding:"required"`
	ToAddress   string `json:"to_address" binding:"required"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
}

type approvalDecisionRequest struct {
	Action  string `json:"action" binding:"required,oneof=approved rejected"`
	Comment string `json:"comment" binding:"max=2000"`
}

type approvalDelegationRequest struct {
	DelegateUserID uint   `json:"delegate_user_id" binding:"required,gt=0"`
	Comment        string `json:"comment" binding:"max=2000"`
}

func (h *ApprovalHandler) CreateWorkflow(c *gin.Context) {
	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	workflow := models.ApprovalWorkflow{Name: req.Name, AssetType: req.AssetType, MinimumAmount: req.MinimumAmount, TimeoutHours: req.TimeoutHours, Active: active, Steps: req.Steps}
	if err := h.workflow.CreateWorkflow(c.Request.Context(), &workflow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, workflow)
}

func (h *ApprovalHandler) ListWorkflows(c *gin.Context) {
	workflows, err := h.workflow.ListWorkflows(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load workflows"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": workflows})
}

func (h *ApprovalHandler) CreateRequest(c *gin.Context) {
	var req createApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	request, err := h.workflow.CreateTransferRequest(c.Request.Context(), userID(c), services.TransferApprovalInput{AssetID: req.AssetID, FromAddress: req.FromAddress, ToAddress: req.ToAddress, Amount: req.Amount})
	if errors.Is(err, services.ErrNoMatchingWorkflow) {
		c.JSON(http.StatusConflict, gin.H{"error": "no approval workflow applies to this transfer"})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create approval request"})
		return
	}
	c.JSON(http.StatusCreated, request)
}

func (h *ApprovalHandler) ListRequests(c *gin.Context) {
	requests, err := h.workflow.ListRequests(c.Request.Context(), userID(c), models.UserRole(userRole(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load approval requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": requests})
}

func (h *ApprovalHandler) GetRequest(c *gin.Context) {
	id, ok := approvalID(c)
	if !ok {
		return
	}
	request, err := h.workflow.GetRequest(c.Request.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "approval request not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load approval request"})
		return
	}
	if !h.workflow.CanViewRequest(c.Request.Context(), request, userID(c), models.UserRole(userRole(c))) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed to view this request"})
		return
	}
	c.JSON(http.StatusOK, request)
}

func (h *ApprovalHandler) Decide(c *gin.Context) {
	id, ok := approvalID(c)
	if !ok {
		return
	}
	var req approvalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	request, err := h.workflow.Decide(c.Request.Context(), id, userID(c), models.UserRole(userRole(c)), req.Action, req.Comment)
	if errors.Is(err, services.ErrNotApprover) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, services.ErrNotPending) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, request)
}

func (h *ApprovalHandler) Delegate(c *gin.Context) {
	id, ok := approvalID(c)
	if !ok {
		return
	}
	var req approvalDelegationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.workflow.Delegate(c.Request.Context(), id, userID(c), req.DelegateUserID, models.UserRole(userRole(c)), req.Comment); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, services.ErrNotApprover) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "approval delegation recorded"})
}

func (h *ApprovalHandler) ExpireTimedOut(c *gin.Context) {
	count, err := h.workflow.ExpireTimedOut(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to expire requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"expired": count, "checked_at": time.Now().UTC()})
}

func approvalID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid approval request id"})
		return 0, false
	}
	return uint(id), true
}
func userID(c *gin.Context) uint {
	if value, ok := c.Get("user_id"); ok {
		if id, ok := value.(uint); ok {
			return id
		}
	}
	return 0
}
func userRole(c *gin.Context) string {
	if value, ok := c.Get("role"); ok {
		if role, ok := value.(string); ok {
			return role
		}
	}
	return ""
}
