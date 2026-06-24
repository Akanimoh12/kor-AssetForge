package models

import (
	"time"

	"gorm.io/gorm"
)

type ApprovalWorkflow struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"not null;uniqueIndex" json:"name"`
	AssetType     string         `gorm:"index" json:"asset_type,omitempty"`
	MinimumAmount int64          `gorm:"default:0" json:"minimum_amount"`
	TimeoutHours  int            `gorm:"not null;default:72" json:"timeout_hours"`
	Active        bool           `gorm:"not null;default:true;index" json:"active"`
	Steps         []ApprovalStep `json:"steps"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApprovalStep is a link in a workflow approval chain. A step can name a
// specific user or any user with RequiredRole; RequiredApprovals supports
// parallel approvals at the same step.
type ApprovalStep struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	WorkflowID        uint      `gorm:"not null;index" json:"workflow_id"`
	StepOrder         int       `gorm:"not null" json:"step_order"`
	RequiredRole      UserRole  `gorm:"type:varchar(32)" json:"required_role,omitempty"`
	ApproverUserID    *uint     `gorm:"index" json:"approver_user_id,omitempty"`
	RequiredApprovals int       `gorm:"not null;default:1" json:"required_approvals"`
	AllowDelegation   bool      `gorm:"not null;default:false" json:"allow_delegation"`
	CreatedAt         time.Time `json:"created_at"`
}

type ApprovalRequest struct {
	ID              uint             `gorm:"primaryKey" json:"id"`
	WorkflowID      uint             `gorm:"not null;index" json:"workflow_id"`
	Workflow        ApprovalWorkflow `json:"workflow,omitempty"`
	TransactionID   uint             `gorm:"not null;uniqueIndex" json:"transaction_id"`
	Transaction     Transaction      `json:"transaction,omitempty"`
	RequesterUserID uint             `gorm:"not null;index" json:"requester_user_id"`
	AssetID         uint             `gorm:"not null;index" json:"asset_id"`
	FromAddress     string           `gorm:"not null" json:"from_address"`
	ToAddress       string           `gorm:"not null" json:"to_address"`
	Amount          int64            `gorm:"not null" json:"amount"`
	Status          string           `gorm:"not null;default:'pending';index" json:"status"` // pending, approved, rejected, expired
	CurrentStep     int              `gorm:"not null;default:1" json:"current_step"`
	ExpiresAt       time.Time        `gorm:"not null;index" json:"expires_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"-"`
}

type ApprovalAction struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	ApprovalRequestID uint      `gorm:"not null;index" json:"approval_request_id"`
	StepOrder         int       `gorm:"not null" json:"step_order"`
	ApproverUserID    uint      `gorm:"not null;index" json:"approver_user_id"`
	DelegatedFromID   *uint     `gorm:"index" json:"delegated_from_id,omitempty"`
	Action            string    `gorm:"not null" json:"action"` // approved, rejected, delegated, expired
	Comment           string    `gorm:"type:text" json:"comment,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
