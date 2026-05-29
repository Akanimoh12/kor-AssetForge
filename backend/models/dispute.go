package models

import (
	"time"

	"gorm.io/gorm"
)

// DisputeStatus represents the lifecycle state of a dispute
type DisputeStatus string

const (
	DisputeStatusOpen       DisputeStatus = "open"
	DisputeStatusUnderReview DisputeStatus = "under_review"
	DisputeStatusResolved   DisputeStatus = "resolved"
	DisputeStatusRejected   DisputeStatus = "rejected"
)

// DisputeResolution represents the outcome of a resolved dispute
type DisputeResolution string

const (
	ResolutionBuyerFavor  DisputeResolution = "buyer_favor"
	ResolutionSellerFavor DisputeResolution = "seller_favor"
	ResolutionSplit       DisputeResolution = "split"
)

// Dispute represents a filed transaction dispute
type Dispute struct {
	ID              uint              `gorm:"primaryKey" json:"id"`
	TransactionID   uint              `gorm:"not null;index" json:"transaction_id"`
	Transaction     Transaction       `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	FiledByAddress  string            `gorm:"not null" json:"filed_by_address"`
	RespondentAddr  string            `gorm:"not null" json:"respondent_address"`
	Reason          string            `gorm:"type:text;not null" json:"reason"`
	Evidence        string            `gorm:"type:text" json:"evidence"`
	Status          DisputeStatus     `gorm:"default:'open'" json:"status"`
	Resolution      DisputeResolution `json:"resolution,omitempty"`
	AdminNotes      string            `gorm:"type:text" json:"admin_notes,omitempty"`
	ReviewedBy      uint              `gorm:"index" json:"reviewed_by,omitempty"`
	EscrowAmount    int64             `gorm:"default:0" json:"escrow_amount"`
	EscrowReleased  bool              `gorm:"default:false" json:"escrow_released"`
	ResolvedAt      *time.Time        `json:"resolved_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	DeletedAt       gorm.DeletedAt    `gorm:"index" json:"-"`
}

// DisputeEscrow tracks funds held in escrow during a dispute
type DisputeEscrow struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	DisputeID  uint           `gorm:"not null;uniqueIndex" json:"dispute_id"`
	Dispute    Dispute        `gorm:"foreignKey:DisputeID" json:"dispute,omitempty"`
	AssetID    uint           `gorm:"not null" json:"asset_id"`
	Amount     int64          `gorm:"not null" json:"amount"`
	HeldFrom   time.Time      `gorm:"not null" json:"held_from"`
	ReleasedAt *time.Time     `json:"released_at,omitempty"`
	ReleasedTo string         `json:"released_to,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
