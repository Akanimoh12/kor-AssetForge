package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoMatchingWorkflow = errors.New("no matching approval workflow")
	ErrNotPending         = errors.New("approval request is not pending")
	ErrNotApprover        = errors.New("user is not eligible to approve this step")
)

type TransferApprovalInput struct {
	AssetID     uint
	FromAddress string
	ToAddress   string
	Amount      int64
}

type WorkflowService struct {
	db    *gorm.DB
	email EmailService
}

func NewWorkflowService(db *gorm.DB, email EmailService) *WorkflowService {
	return &WorkflowService{db: db, email: email}
}

func (s *WorkflowService) CreateWorkflow(ctx context.Context, workflow *models.ApprovalWorkflow) error {
	if workflow.Name == "" || workflow.TimeoutHours <= 0 || len(workflow.Steps) == 0 {
		return errors.New("workflow name, positive timeout, and at least one step are required")
	}
	for i := range workflow.Steps {
		step := &workflow.Steps[i]
		if step.StepOrder != i+1 || step.RequiredApprovals < 1 || (step.RequiredRole == "" && step.ApproverUserID == nil) {
			return errors.New("workflow steps must be ordered consecutively and define an approver role or user")
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Steps").Create(workflow).Error; err != nil {
			return err
		}
		for i := range workflow.Steps {
			workflow.Steps[i].WorkflowID = workflow.ID
		}
		return tx.Create(&workflow.Steps).Error
	})
}

func (s *WorkflowService) ListWorkflows(ctx context.Context) ([]models.ApprovalWorkflow, error) {
	var workflows []models.ApprovalWorkflow
	err := s.db.WithContext(ctx).Preload("Steps", func(db *gorm.DB) *gorm.DB { return db.Order("step_order") }).Order("name").Find(&workflows).Error
	return workflows, err
}

// CreateTransferRequest makes a transaction non-executable until the selected
// approval chain completes. A missing matching workflow is explicit so legacy
// transfer paths can still operate when no policy applies.
func (s *WorkflowService) CreateTransferRequest(ctx context.Context, requesterID uint, input TransferApprovalInput) (*models.ApprovalRequest, error) {
	var asset models.Asset
	if err := s.db.WithContext(ctx).First(&asset, input.AssetID).Error; err != nil {
		return nil, err
	}
	var workflow models.ApprovalWorkflow
	err := s.db.WithContext(ctx).Where("active = ? AND minimum_amount <= ? AND (asset_type = '' OR asset_type IS NULL OR asset_type = ?)", true, input.Amount, asset.AssetType).
		Order("minimum_amount DESC").First(&workflow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoMatchingWorkflow
	}
	if err != nil {
		return nil, err
	}

	request := &models.ApprovalRequest{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		transaction := models.Transaction{
			AssetID: input.AssetID, FromAddress: input.FromAddress, ToAddress: input.ToAddress,
			Amount: input.Amount, TxHash: "approval-" + uuid.NewString(), Status: "approval_pending",
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}
		*request = models.ApprovalRequest{
			WorkflowID: workflow.ID, TransactionID: transaction.ID, RequesterUserID: requesterID,
			AssetID: input.AssetID, FromAddress: input.FromAddress, ToAddress: input.ToAddress, Amount: input.Amount,
			Status: "pending", CurrentStep: 1, ExpiresAt: time.Now().UTC().Add(time.Duration(workflow.TimeoutHours) * time.Hour),
		}
		return tx.Create(request).Error
	})
	if err != nil {
		return nil, err
	}
	_ = s.notifyPendingApprovers(ctx, request.ID)
	return s.GetRequest(ctx, request.ID)
}

func (s *WorkflowService) GetRequest(ctx context.Context, id uint) (*models.ApprovalRequest, error) {
	var request models.ApprovalRequest
	err := s.db.WithContext(ctx).Preload("Workflow.Steps", func(db *gorm.DB) *gorm.DB { return db.Order("step_order") }).Preload("Transaction").First(&request, id).Error
	return &request, err
}

func (s *WorkflowService) ListRequests(ctx context.Context, userID uint, role models.UserRole) ([]models.ApprovalRequest, error) {
	var requests []models.ApprovalRequest
	q := s.db.WithContext(ctx).Preload("Workflow.Steps", func(db *gorm.DB) *gorm.DB { return db.Order("step_order") }).Order("created_at DESC")
	if role != models.RoleAdmin && role != models.RoleModerator {
		q = q.Where(`requester_user_id = ? OR EXISTS (
			SELECT 1 FROM approval_steps
			WHERE approval_steps.workflow_id = approval_requests.workflow_id
			AND approval_steps.step_order = approval_requests.current_step
			AND (approval_steps.required_role = ? OR approval_steps.approver_user_id = ?)
		) OR EXISTS (
			SELECT 1 FROM approval_actions
			WHERE approval_actions.approval_request_id = approval_requests.id
			AND approval_actions.step_order = approval_requests.current_step
			AND approval_actions.approver_user_id = ? AND approval_actions.action = 'delegated'
		)`, userID, role, userID, userID)
	}
	err := q.Find(&requests).Error
	return requests, err
}

func (s *WorkflowService) CanViewRequest(ctx context.Context, request *models.ApprovalRequest, userID uint, role models.UserRole) bool {
	if request.RequesterUserID == userID || role == models.RoleAdmin || role == models.RoleModerator {
		return true
	}
	step, err := currentStep(request)
	if err == nil && eligibleForStep(step, userID, role) {
		return true
	}
	var count int64
	s.db.WithContext(ctx).Model(&models.ApprovalAction{}).Where("approval_request_id = ? AND step_order = ? AND approver_user_id = ? AND action = ?", request.ID, request.CurrentStep, userID, "delegated").Count(&count)
	return count > 0
}

func (s *WorkflowService) Decide(ctx context.Context, requestID, approverID uint, role models.UserRole, action, comment string) (*models.ApprovalRequest, error) {
	if action != "approved" && action != "rejected" {
		return nil, errors.New("action must be approved or rejected")
	}
	var result *models.ApprovalRequest
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ApprovalRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Workflow.Steps").First(&request, requestID).Error; err != nil {
			return err
		}
		if request.Status != "pending" {
			return ErrNotPending
		}
		if !time.Now().UTC().Before(request.ExpiresAt) {
			return s.expireInTransaction(tx, &request, "approval timeout")
		}
		step, err := currentStep(&request)
		if err != nil {
			return err
		}
		delegated := false
		if !eligibleForStep(step, approverID, role) {
			var count int64
			if err := tx.Model(&models.ApprovalAction{}).Where("approval_request_id = ? AND step_order = ? AND approver_user_id = ? AND action = ?", request.ID, step.StepOrder, approverID, "delegated").Count(&count).Error; err != nil {
				return err
			}
			delegated = count > 0
			if !delegated {
				return ErrNotApprover
			}
		}
		var duplicate int64
		if err := tx.Model(&models.ApprovalAction{}).Where("approval_request_id = ? AND step_order = ? AND approver_user_id = ? AND action = ?", request.ID, step.StepOrder, approverID, "approved").Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return errors.New("approver has already approved this step")
		}
		approvalAction := &models.ApprovalAction{ApprovalRequestID: request.ID, StepOrder: step.StepOrder, ApproverUserID: approverID, Action: action, Comment: comment}
		if delegated {
			var delegation models.ApprovalAction
			if err := tx.Where("approval_request_id = ? AND step_order = ? AND approver_user_id = ? AND action = ?", request.ID, step.StepOrder, approverID, "delegated").Order("created_at DESC").First(&delegation).Error; err != nil {
				return err
			}
			approvalAction.DelegatedFromID = delegation.DelegatedFromID
		}
		if err := tx.Create(approvalAction).Error; err != nil {
			return err
		}
		if action == "rejected" {
			now := time.Now().UTC()
			request.Status, request.CompletedAt = "rejected", &now
			if err := tx.Model(&models.Transaction{}).Where("id = ?", request.TransactionID).Update("status", "rejected").Error; err != nil {
				return err
			}
			if err := tx.Save(&request).Error; err != nil {
				return err
			}
			result = &request
			return nil
		}
		var approvals int64
		if err := tx.Model(&models.ApprovalAction{}).Where("approval_request_id = ? AND step_order = ? AND action = ?", request.ID, step.StepOrder, "approved").Count(&approvals).Error; err != nil {
			return err
		}
		if approvals >= int64(step.RequiredApprovals) {
			request.CurrentStep++
			if request.CurrentStep > len(request.Workflow.Steps) {
				now := time.Now().UTC()
				request.Status, request.CompletedAt = "approved", &now
				if err := tx.Model(&models.Transaction{}).Where("id = ?", request.TransactionID).Update("status", "approved").Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Save(&request).Error; err != nil {
			return err
		}
		result = &request
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Status == "pending" {
		_ = s.notifyPendingApprovers(ctx, result.ID)
	}
	return s.GetRequest(ctx, result.ID)
}

func (s *WorkflowService) Delegate(ctx context.Context, requestID, fromID, toID uint, role models.UserRole, comment string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ApprovalRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Workflow.Steps").First(&request, requestID).Error; err != nil {
			return err
		}
		if request.Status != "pending" {
			return ErrNotPending
		}
		step, err := currentStep(&request)
		if err != nil {
			return err
		}
		if !step.AllowDelegation || !eligibleForStep(step, fromID, role) {
			return ErrNotApprover
		}
		return tx.Create(&models.ApprovalAction{ApprovalRequestID: request.ID, StepOrder: step.StepOrder, ApproverUserID: toID, DelegatedFromID: &fromID, Action: "delegated", Comment: comment}).Error
	})
}

func (s *WorkflowService) ExpireTimedOut(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Model(&models.ApprovalRequest{}).Where("status = ? AND expires_at <= ?", "pending", time.Now().UTC()).Updates(map[string]interface{}{"status": "expired", "completed_at": time.Now().UTC()})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		s.db.WithContext(ctx).Model(&models.Transaction{}).Where("status = ?", "approval_pending").Where("id IN (SELECT transaction_id FROM approval_requests WHERE status = ?)", "expired").Update("status", "expired")
	}
	return result.RowsAffected, nil
}

func currentStep(request *models.ApprovalRequest) (*models.ApprovalStep, error) {
	for i := range request.Workflow.Steps {
		if request.Workflow.Steps[i].StepOrder == request.CurrentStep {
			return &request.Workflow.Steps[i], nil
		}
	}
	return nil, fmt.Errorf("workflow %d has no step %d", request.WorkflowID, request.CurrentStep)
}

func eligibleForStep(step *models.ApprovalStep, userID uint, role models.UserRole) bool {
	if step.ApproverUserID != nil && *step.ApproverUserID == userID {
		return true
	}
	return step.RequiredRole != "" && step.RequiredRole == role
}

func (s *WorkflowService) expireInTransaction(tx *gorm.DB, request *models.ApprovalRequest, comment string) error {
	now := time.Now().UTC()
	request.Status, request.CompletedAt = "expired", &now
	if err := tx.Create(&models.ApprovalAction{ApprovalRequestID: request.ID, StepOrder: request.CurrentStep, ApproverUserID: request.RequesterUserID, Action: "expired", Comment: comment}).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.Transaction{}).Where("id = ?", request.TransactionID).Update("status", "expired").Error; err != nil {
		return err
	}
	return tx.Save(request).Error
}

func (s *WorkflowService) notifyPendingApprovers(ctx context.Context, requestID uint) error {
	if s.email == nil {
		return nil
	}
	request, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	step, err := currentStep(request)
	if err != nil {
		return err
	}
	q := s.db.WithContext(ctx).Model(&models.User{})
	if step.ApproverUserID != nil {
		q = q.Where("id = ?", *step.ApproverUserID)
	} else {
		q = q.Where("role = ?", step.RequiredRole)
	}
	var users []models.User
	if err := q.Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		if user.Email != "" {
			_ = s.email.SendApprovalPendingEmail(user.Email, user.Username, request.ID, request.ExpiresAt)
		}
	}
	return nil
}
