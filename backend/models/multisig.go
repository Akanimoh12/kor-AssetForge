package models

import (
	"time"

	"gorm.io/gorm"
)

type MultiSigWallet struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	OwnerIDs    string         `gorm:"type:text;not null" json:"owner_ids"` // JSON array of user IDs
	Threshold   int            `gorm:"not null" json:"threshold"`
	ContractID  string         `gorm:"uniqueIndex" json:"contract_id,omitempty"`
	CreatedByID uint           `gorm:"not null;index" json:"created_by_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type MultiSigProposal struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	WalletID    uint           `gorm:"not null;index" json:"wallet_id"`
	ProposerID  uint           `gorm:"not null;index" json:"proposer_id"`
	ToAddress   string         `gorm:"not null" json:"to_address"`
	Amount      int64          `gorm:"not null" json:"amount"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Status      string         `gorm:"not null;default:'pending';index" json:"status"` // pending, approved, executed, rejected
	SignCount   int            `gorm:"not null;default:0" json:"sign_count"`
	TxHash      string         `json:"tx_hash,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type MultiSigSignature struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProposalID uint      `gorm:"not null;index" json:"proposal_id"`
	SignerID   uint      `gorm:"not null;index" json:"signer_id"`
	CreatedAt  time.Time `json:"created_at"`
}
