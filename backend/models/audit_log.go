package models

import (
	"time"

	"gorm.io/gorm"
)

// AuditLog is an immutable compliance record for an asset operation.
// BeforeState and AfterState contain JSON (optionally gzipped+base64 after
// archival compression) and StateEncoding identifies how to read them.
type AuditLog struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        *uint          `gorm:"index" json:"user_id,omitempty"`
	Action        string         `gorm:"not null;index" json:"action"`
	Resource      string         `gorm:"not null;index" json:"resource"`
	ResourceID    string         `gorm:"not null;index" json:"resource_id"`
	Method        string         `gorm:"not null" json:"method"`
	Path          string         `gorm:"not null" json:"path"`
	Status        int            `gorm:"not null" json:"status"`
	IPAddress     string         `gorm:"type:inet;not null" json:"ip_address"`
	BeforeState   string         `gorm:"type:text" json:"before_state,omitempty"`
	AfterState    string         `gorm:"type:text" json:"after_state,omitempty"`
	Details       string         `gorm:"type:text" json:"details,omitempty"`
	StateEncoding string         `gorm:"not null;default:'json'" json:"state_encoding"`
	ExpiresAt     time.Time      `gorm:"not null;index" json:"expires_at"`
	CreatedAt     time.Time      `gorm:"index" json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
