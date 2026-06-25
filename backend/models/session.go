package models

import (
	"time"

	"gorm.io/gorm"
)

// DeviceSession is an enhanced session model with device fingerprinting and geolocation.
// It extends UserSession with additional tracking fields.
type DeviceSession struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"not null;index" json:"user_id"`
	SessionToken      string         `gorm:"uniqueIndex;not null" json:"-"`
	IPAddress         string         `json:"ip_address"`
	UserAgent         string         `json:"user_agent"`
	DeviceFingerprint string         `gorm:"index" json:"device_fingerprint,omitempty"`
	DeviceType        string         `json:"device_type,omitempty"`   // mobile, desktop, tablet
	Browser           string         `json:"browser,omitempty"`
	OS                string         `json:"os,omitempty"`
	CountryCode       string         `json:"country_code,omitempty"`
	City              string         `json:"city,omitempty"`
	Timezone          string         `json:"timezone,omitempty"`
	LastActiveAt      time.Time      `gorm:"index" json:"last_active_at"`
	ExpiresAt         time.Time      `gorm:"index" json:"expires_at"`
	IsRevoked         bool           `gorm:"default:false;index" json:"is_revoked"`
	RevokedAt         *time.Time     `json:"revoked_at,omitempty"`
	RevokedBy         *uint          `json:"revoked_by,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// SessionSummary is a safe public view of a DeviceSession (no token exposed).
type SessionSummary struct {
	ID                uint      `json:"id"`
	DeviceFingerprint string    `json:"device_fingerprint,omitempty"`
	DeviceType        string    `json:"device_type,omitempty"`
	Browser           string    `json:"browser,omitempty"`
	OS                string    `json:"os,omitempty"`
	IPAddress         string    `json:"ip_address"`
	CountryCode       string    `json:"country_code,omitempty"`
	City              string    `json:"city,omitempty"`
	LastActiveAt      time.Time `json:"last_active_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	IsRevoked         bool      `json:"is_revoked"`
	CreatedAt         time.Time `json:"created_at"`
	IsCurrent         bool      `json:"is_current"`
}
