package models

import (
	"time"

	"gorm.io/gorm"
)

// LegalDocumentType enumerates versioned legal documents
type LegalDocumentType string

const (
	DocTypeTermsOfService LegalDocumentType = "terms_of_service"
	DocTypePrivacyPolicy  LegalDocumentType = "privacy_policy"
	DocTypeCookiePolicy   LegalDocumentType = "cookie_policy"
)

// LegalDocument stores versioned legal documents
type LegalDocument struct {
	ID          uint              `gorm:"primaryKey" json:"id"`
	Type        LegalDocumentType `gorm:"not null;index" json:"type"`
	Version     string            `gorm:"not null" json:"version"`
	Content     string            `gorm:"type:text;not null" json:"content"`
	EffectiveAt time.Time         `gorm:"not null" json:"effective_at"`
	Active      bool              `gorm:"default:false" json:"active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// UserConsent records a user's acceptance of a legal document version
type UserConsent struct {
	ID         uint              `gorm:"primaryKey" json:"id"`
	UserID     uint              `gorm:"not null;index" json:"user_id"`
	DocumentID uint              `gorm:"not null;index" json:"document_id"`
	Document   LegalDocument     `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
	DocType    LegalDocumentType `gorm:"not null" json:"doc_type"`
	Version    string            `gorm:"not null" json:"version"`
	IPAddress  string            `json:"ip_address"`
	UserAgent  string            `json:"user_agent"`
	AcceptedAt time.Time         `gorm:"not null" json:"accepted_at"`
	CreatedAt  time.Time         `json:"created_at"`
	DeletedAt  gorm.DeletedAt    `gorm:"index" json:"-"`
}

// DataExportRequest tracks GDPR data export requests
type DataExportRequest struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	Status      string         `gorm:"default:'pending'" json:"status"`
	DownloadURL string         `json:"download_url,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
