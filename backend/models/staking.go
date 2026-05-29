package models

import (
	"time"

	"gorm.io/gorm"
)

// StakePosition tracks a user's staked tokens for a given asset
type StakePosition struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"not null;index" json:"user_id"`
	User              User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	AssetID           uint           `gorm:"not null;index" json:"asset_id"`
	Asset             Asset          `gorm:"foreignKey:AssetID" json:"asset,omitempty"`
	StakedAmount      int64          `gorm:"not null" json:"staked_amount"`
	AccruedRewards    int64          `gorm:"default:0" json:"accrued_rewards"`
	ClaimedRewards    int64          `gorm:"default:0" json:"claimed_rewards"`
	StakedAt          time.Time      `gorm:"not null" json:"staked_at"`
	LastRewardAt      *time.Time     `json:"last_reward_at,omitempty"`
	StellarAddress    string         `gorm:"not null" json:"stellar_address"`
	Active            bool           `gorm:"default:true" json:"active"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// RewardDistribution records a batch reward distribution event
type RewardDistribution struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	AssetID          uint      `gorm:"not null;index" json:"asset_id"`
	Asset            Asset     `gorm:"foreignKey:AssetID" json:"asset,omitempty"`
	TotalDistributed int64     `gorm:"not null" json:"total_distributed"`
	StakerCount      int       `gorm:"not null" json:"staker_count"`
	APRBasisPoints   int       `gorm:"not null" json:"apr_basis_points"` // APR in bps (e.g. 500 = 5%)
	PeriodStart      time.Time `gorm:"not null" json:"period_start"`
	PeriodEnd        time.Time `gorm:"not null" json:"period_end"`
	TxHash           string    `gorm:"index" json:"tx_hash,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// RewardClaim records an individual reward claim by a staker
type RewardClaim struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	StakeID     uint      `gorm:"not null;index" json:"stake_id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	AssetID     uint      `gorm:"not null;index" json:"asset_id"`
	Amount      int64     `gorm:"not null" json:"amount"`
	TxHash      string    `gorm:"index" json:"tx_hash,omitempty"`
	ClaimedAt   time.Time `gorm:"not null" json:"claimed_at"`
}
