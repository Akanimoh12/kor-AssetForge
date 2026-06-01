package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
)

type MarketMakerHandler struct {
	mmService *services.MarketMakerService
}

func NewMarketMakerHandler(db *gorm.DB) *MarketMakerHandler {
	return &MarketMakerHandler{
		mmService: services.NewMarketMakerService(db),
	}
}

type CreateBotRequest struct {
	Name                     string `json:"name" binding:"required"`
	ManagedAssetID           uint   `json:"managed_asset_id" binding:"required"`
	OperatorAddress          string `json:"operator_address" binding:"required"`
	MinSpreadBps             int16  `json:"min_spread_bps"`
	MaxPositionStroops       int64  `json:"max_position_stroops" binding:"required"`
	InventoryTargetStroops   int64  `json:"inventory_target_stroops" binding:"required"`
}

type CreateOrderRequest struct {
	BotID       uint   `json:"bot_id" binding:"required"`
	OrderType   string `json:"order_type" binding:"required,oneof=buy sell"`
	AssetID     uint   `json:"asset_id" binding:"required"`
	PriceStroops int64 `json:"price_stroops" binding:"required"`
	AmountUnits int64  `json:"amount_units" binding:"required"`
}

type RecordTradeRequest struct {
	BotID                uint   `json:"bot_id" binding:"required"`
	OrderID              uint   `json:"order_id" binding:"required"`
	CounterpartyAddress  string `json:"counterparty_address" binding:"required"`
	Side                 string `json:"side" binding:"required,oneof=buy sell"`
	PriceStroops         int64  `json:"price_stroops" binding:"required"`
	AmountUnits          int64  `json:"amount_units" binding:"required"`
	FeeStroops           int64  `json:"fee_stroops" binding:"required"`
}

type CalculatePriceRequest struct {
	MarketPrice     int64   `json:"market_price" binding:"required"`
	Volatility      float64 `json:"volatility" binding:"required"`
	TargetSpreadBps int16   `json:"target_spread_bps" binding:"required"`
}

func (mmh *MarketMakerHandler) CreateBot(c *gin.Context) {
	var req CreateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bot := &models.MarketMakerBot{
		Name:                    req.Name,
		ManagedAssetID:          req.ManagedAssetID,
		OperatorAddress:         req.OperatorAddress,
		MinSpreadBps:            req.MinSpreadBps,
		MaxPositionStroops:      req.MaxPositionStroops,
		InventoryTargetStroops:  req.InventoryTargetStroops,
		Status:                  "inactive",
	}

	if err := mmh.mmService.CreateBot(bot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bot)
}

func (mmh *MarketMakerHandler) GetBot(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	bot, err := mmh.mmService.GetBot(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
		return
	}

	c.JSON(http.StatusOK, bot)
}

func (mmh *MarketMakerHandler) GetBotsByStatus(c *gin.Context) {
	status := c.Query("status")
	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status parameter required"})
		return
	}

	bots, err := mmh.mmService.GetBotsByStatus(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bots": bots})
}

func (mmh *MarketMakerHandler) UpdateBotStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, ok := req["status"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status field required"})
		return
	}

	if err := mmh.mmService.UpdateBotStatus(uint(id), status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bot status updated", "bot_id": id, "status": status})
}

func (mmh *MarketMakerHandler) CalculatePricingSpread(c *gin.Context) {
	var req CalculatePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bidPrice, askPrice := mmh.mmService.CalculatePricingSpread(req.MarketPrice, req.Volatility, req.TargetSpreadBps)

	c.JSON(http.StatusOK, gin.H{
		"market_price":  req.MarketPrice,
		"bid_price":     bidPrice,
		"ask_price":     askPrice,
		"spread_stroops": askPrice - bidPrice,
		"volatility":    req.Volatility,
	})
}

func (mmh *MarketMakerHandler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order := &models.MarketMakerOrder{
		BotID:       req.BotID,
		OrderType:   req.OrderType,
		AssetID:     req.AssetID,
		PriceStroops: req.PriceStroops,
		AmountUnits: req.AmountUnits,
		Status:      "active",
	}

	if err := mmh.mmService.CreateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

func (mmh *MarketMakerHandler) GetActiveOrders(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("bot_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	orders, err := mmh.mmService.GetActiveOrders(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

func (mmh *MarketMakerHandler) CancelOrder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("order_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	if err := mmh.mmService.CancelOrder(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order cancelled", "order_id": id})
}

func (mmh *MarketMakerHandler) RecordTrade(c *gin.Context) {
	var req RecordTradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trade := &models.MarketMakerTrade{
		BotID:               req.BotID,
		OrderID:             req.OrderID,
		CounterpartyAddress: req.CounterpartyAddress,
		Side:                req.Side,
		PriceStroops:        req.PriceStroops,
		AmountUnits:         req.AmountUnits,
		FeeStroops:          req.FeeStroops,
		CreatedAt:           time.Now(),
	}

	if err := mmh.mmService.RecordTrade(trade); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, trade)
}

func (mmh *MarketMakerHandler) GetInventory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("bot_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	inv, err := mmh.mmService.GetInventory(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"inventory": inv})
}

func (mmh *MarketMakerHandler) GetHealthChecks(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("bot_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	checks, err := mmh.mmService.GetHealthChecks(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"health_checks": checks})
}

func (mmh *MarketMakerHandler) GetBotStats(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("bot_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	stats, err := mmh.mmService.GetBotStats(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (mmh *MarketMakerHandler) CheckPositionLimit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("bot_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	ok, err := mmh.mmService.CheckPositionLimit(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bot_id": id, "within_limit": ok})
}
