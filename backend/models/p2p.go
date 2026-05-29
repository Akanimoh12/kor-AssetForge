package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderSide indicates buy or sell
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// OrderStatus tracks lifecycle of a P2P order
type OrderStatus string

const (
	OrderStatusOpen      OrderStatus = "open"
	OrderStatusPartial   OrderStatus = "partial"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// P2POrder represents a buy or sell order in the peer-to-peer marketplace
type P2POrder struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	AssetID        uint           `gorm:"not null;index" json:"asset_id"`
	Asset          Asset          `gorm:"foreignKey:AssetID" json:"asset,omitempty"`
	OwnerAddress   string         `gorm:"not null;index" json:"owner_address"`
	Side           OrderSide      `gorm:"not null" json:"side"`
	Price          int64          `gorm:"not null" json:"price"` // price per unit in stroops
	Quantity       int64          `gorm:"not null" json:"quantity"`
	FilledQuantity int64          `gorm:"default:0" json:"filled_quantity"`
	Status         OrderStatus    `gorm:"default:'open'" json:"status"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// P2PTrade records an executed match between two orders
type P2PTrade struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AssetID      uint      `gorm:"not null;index" json:"asset_id"`
	BuyOrderID   uint      `gorm:"not null;index" json:"buy_order_id"`
	SellOrderID  uint      `gorm:"not null;index" json:"sell_order_id"`
	BuyerAddress string    `gorm:"not null" json:"buyer_address"`
	SellerAddress string   `gorm:"not null" json:"seller_address"`
	Price        int64     `gorm:"not null" json:"price"`
	Quantity     int64     `gorm:"not null" json:"quantity"`
	TotalValue   int64     `gorm:"not null" json:"total_value"` // price * quantity
	TxHash       string    `gorm:"index" json:"tx_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// PricePoint stores a historical price data point for charting
type PricePoint struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AssetID   uint      `gorm:"not null;index" json:"asset_id"`
	Price     int64     `gorm:"not null" json:"price"`
	Volume    int64     `gorm:"not null" json:"volume"`
	High      int64     `gorm:"not null" json:"high"`
	Low       int64     `gorm:"not null" json:"low"`
	Open      int64     `gorm:"not null" json:"open"`
	Close     int64     `gorm:"not null" json:"close"`
	Timestamp time.Time `gorm:"not null;index" json:"timestamp"`
}
