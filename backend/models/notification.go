package models

import (
	"time"

	"gorm.io/gorm"
)

// NotificationType categorises notifications
type NotificationType string

const (
	NotifTypeAssetMinted     NotificationType = "asset_minted"
	NotifTypeAssetTransferred NotificationType = "asset_transferred"
	NotifTypeDisputeFiled    NotificationType = "dispute_filed"
	NotifTypeDisputeResolved NotificationType = "dispute_resolved"
	NotifTypeStakeReward     NotificationType = "stake_reward"
	NotifTypeP2POrderFilled  NotificationType = "p2p_order_filled"
	NotifTypeKYCApproved     NotificationType = "kyc_approved"
	NotifTypeKYCRejected     NotificationType = "kyc_rejected"
	NotifTypeSystem          NotificationType = "system"
)

// NotificationChannel defines delivery channels
type NotificationChannel string

const (
	ChannelInApp NotificationChannel = "in_app"
	ChannelEmail NotificationChannel = "email"
	ChannelPush  NotificationChannel = "push"
)

// Notification is a single in-app notification record
type Notification struct {
	ID         uint             `gorm:"primaryKey" json:"id"`
	UserID     uint             `gorm:"not null;index" json:"user_id"`
	Type       NotificationType `gorm:"not null" json:"type"`
	Title      string           `gorm:"not null" json:"title"`
	Body       string           `gorm:"type:text;not null" json:"body"`
	ResourceID *uint            `json:"resource_id,omitempty"`
	ResourceType string         `json:"resource_type,omitempty"`
	Read       bool             `gorm:"default:false" json:"read"`
	ReadAt     *time.Time       `json:"read_at,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	DeletedAt  gorm.DeletedAt   `gorm:"index" json:"-"`
}

// NotificationPreference stores per-user channel preferences per notification type
type NotificationPreference struct {
	ID               uint             `gorm:"primaryKey" json:"id"`
	UserID           uint             `gorm:"not null;uniqueIndex:idx_user_notif_type" json:"user_id"`
	NotificationType NotificationType `gorm:"not null;uniqueIndex:idx_user_notif_type" json:"notification_type"`
	InApp            bool             `gorm:"default:true" json:"in_app"`
	Email            bool             `gorm:"default:true" json:"email"`
	Push             bool             `gorm:"default:false" json:"push"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}
