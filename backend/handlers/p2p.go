package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/utils"
	"gorm.io/gorm"
)

// P2PHandler handles P2P marketplace HTTP requests
type P2PHandler struct {
	db *gorm.DB
}

// NewP2PHandler creates a new P2PHandler
func NewP2PHandler(db *gorm.DB) *P2PHandler {
	return &P2PHandler{db: db}
}

type createOrderRequest struct {
	AssetID      uint              `json:"asset_id" binding:"required,gt=0"`
	OwnerAddress string            `json:"owner_address" binding:"required"`
	Side         models.OrderSide  `json:"side" binding:"required,oneof=buy sell"`
	Price        int64             `json:"price" binding:"required,gt=0"`
	Quantity     int64             `json:"quantity" binding:"required,gt=0"`
	ExpiresInSec int64             `json:"expires_in_seconds" binding:"omitempty,gt=0"`
}

// CreateOrder places a new buy or sell order in the P2P marketplace
// @Summary Create P2P order
// @Description Place a buy or sell order; matching runs immediately against existing orders
// @Tags p2p
// @Accept json
// @Produce json
// @Param order body createOrderRequest true "Order details"
// @Success 201 {object} models.P2POrder
// @Failure 400 {object} apperrors.ErrorResponse
// @Router /p2p/orders [post]
func (h *P2PHandler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure asset exists
	var asset models.Asset
	if err := h.db.First(&asset, req.AssetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	order := models.P2POrder{
		AssetID:      req.AssetID,
		OwnerAddress: req.OwnerAddress,
		Side:         req.Side,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       models.OrderStatusOpen,
	}
	if req.ExpiresInSec > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresInSec) * time.Second)
		order.ExpiresAt = &t
	}

	if err := h.db.Create(&order).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to create order", http.StatusInternalServerError))
		return
	}

	// Attempt automatic matching
	trades := h.matchOrder(&order)

	c.JSON(http.StatusCreated, gin.H{
		"order":  order,
		"trades": trades,
	})
}

// matchOrder performs price-time priority matching against opposing orders
func (h *P2PHandler) matchOrder(order *models.P2POrder) []models.P2PTrade {
	var trades []models.P2PTrade

	oppositeSide := models.OrderSideSell
	priceCondition := "price <= ?"
	priceOrder := "price ASC, created_at ASC"
	if order.Side == models.OrderSideSell {
		oppositeSide = models.OrderSideBuy
		priceCondition = "price >= ?"
		priceOrder = "price DESC, created_at ASC"
	}

	var counterOrders []models.P2POrder
	h.db.Where(
		"asset_id = ? AND side = ? AND status IN ? AND "+priceCondition,
		order.AssetID, oppositeSide,
		[]string{string(models.OrderStatusOpen), string(models.OrderStatusPartial)},
		order.Price,
	).Order(priceOrder).Find(&counterOrders)

	remaining := order.Quantity - order.FilledQuantity

	for _, counter := range counterOrders {
		if remaining <= 0 {
			break
		}

		counterRemaining := counter.Quantity - counter.FilledQuantity
		fillQty := remaining
		if counterRemaining < fillQty {
			fillQty = counterRemaining
		}

		tradePrice := counter.Price

		var buyerAddr, sellerAddr string
		var buyOrderID, sellOrderID uint
		if order.Side == models.OrderSideBuy {
			buyerAddr = order.OwnerAddress
			sellerAddr = counter.OwnerAddress
			buyOrderID = order.ID
			sellOrderID = counter.ID
		} else {
			buyerAddr = counter.OwnerAddress
			sellerAddr = order.OwnerAddress
			buyOrderID = counter.ID
			sellOrderID = order.ID
		}

		trade := models.P2PTrade{
			AssetID:       order.AssetID,
			BuyOrderID:    buyOrderID,
			SellOrderID:   sellOrderID,
			BuyerAddress:  buyerAddr,
			SellerAddress: sellerAddr,
			Price:         tradePrice,
			Quantity:      fillQty,
			TotalValue:    tradePrice * fillQty,
		}

		h.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&trade).Error; err != nil {
				return err
			}

			// Update taker order
			order.FilledQuantity += fillQty
			if order.FilledQuantity >= order.Quantity {
				order.Status = models.OrderStatusFilled
			} else {
				order.Status = models.OrderStatusPartial
			}
			if err := tx.Save(order).Error; err != nil {
				return err
			}

			// Update maker order
			counter.FilledQuantity += fillQty
			if counter.FilledQuantity >= counter.Quantity {
				counter.Status = models.OrderStatusFilled
			} else {
				counter.Status = models.OrderStatusPartial
			}
			if err := tx.Save(&counter).Error; err != nil {
				return err
			}

			// Record price discovery point
			pp := models.PricePoint{
				AssetID:   order.AssetID,
				Price:     tradePrice,
				Volume:    fillQty,
				High:      tradePrice,
				Low:       tradePrice,
				Open:      tradePrice,
				Close:     tradePrice,
				Timestamp: time.Now(),
			}
			return tx.Create(&pp).Error
		})

		trades = append(trades, trade)
		remaining -= fillQty
	}

	return trades
}

// CancelOrder cancels an open or partial order
// @Summary Cancel P2P order
// @Description Cancel an open or partially filled order
// @Tags p2p
// @Param id path int true "Order ID"
// @Success 200 {object} models.P2POrder
// @Router /p2p/orders/{id}/cancel [put]
func (h *P2PHandler) CancelOrder(c *gin.Context) {
	var uri struct {
		ID uint `uri:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var order models.P2POrder
	if err := h.db.First(&order, uri.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.Status == models.OrderStatusFilled || order.Status == models.OrderStatusCancelled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled"})
		return
	}

	order.Status = models.OrderStatusCancelled
	if err := h.db.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}

	c.JSON(http.StatusOK, order)
}

// ListOrders returns open orders for an asset (order book)
// @Summary Order book
// @Description Get open buy and sell orders for an asset
// @Tags p2p
// @Param asset_id query int true "Asset ID"
// @Param side query string false "Filter by side: buy or sell"
// @Success 200 {object} map[string]interface{}
// @Router /p2p/orders [get]
func (h *P2PHandler) ListOrders(c *gin.Context) {
	type query struct {
		AssetID uint   `form:"asset_id" binding:"required,gt=0"`
		Side    string `form:"side" binding:"omitempty,oneof=buy sell"`
		Page    int    `form:"page,default=1"`
		Limit   int    `form:"limit,default=20"`
	}

	var q query
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}

	db := h.db.Model(&models.P2POrder{}).
		Where("asset_id = ? AND status IN ?", q.AssetID,
			[]string{string(models.OrderStatusOpen), string(models.OrderStatusPartial)})
	if q.Side != "" {
		db = db.Where("side = ?", q.Side)
	}

	var orders []models.P2POrder
	var total int64
	paginationRes, err := utils.Paginate(db, c, q.Page, q.Limit, &total, &orders)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}
	c.JSON(http.StatusOK, paginationRes)
}

// GetTradeHistory returns executed trades for an asset or address
// @Summary Trade history
// @Description Paginated list of executed P2P trades
// @Tags p2p
// @Param asset_id query int false "Filter by asset ID"
// @Param address query string false "Filter by buyer or seller address"
// @Success 200 {object} utils.Pagination
// @Router /p2p/trades [get]
func (h *P2PHandler) GetTradeHistory(c *gin.Context) {
	type query struct {
		AssetID uint   `form:"asset_id"`
		Address string `form:"address"`
		Page    int    `form:"page,default=1"`
		Limit   int    `form:"limit,default=20"`
	}

	var q query
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}

	db := h.db.Model(&models.P2PTrade{}).Order("created_at DESC")
	if q.AssetID != 0 {
		db = db.Where("asset_id = ?", q.AssetID)
	}
	if q.Address != "" {
		db = db.Where("buyer_address = ? OR seller_address = ?", q.Address, q.Address)
	}

	var trades []models.P2PTrade
	var total int64
	paginationRes, err := utils.Paginate(db, c, q.Page, q.Limit, &total, &trades)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trades"})
		return
	}
	c.JSON(http.StatusOK, paginationRes)
}

// GetPriceChart returns OHLCV price history for an asset
// @Summary Price chart
// @Description Get OHLCV price data points for an asset
// @Tags p2p
// @Param asset_id query int true "Asset ID"
// @Param limit query int false "Number of data points"
// @Success 200 {array} models.PricePoint
// @Router /p2p/prices [get]
func (h *P2PHandler) GetPriceChart(c *gin.Context) {
	assetIDStr := c.Query("asset_id")
	if assetIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_id is required"})
		return
	}

	limit := 100
	var points []models.PricePoint
	if err := h.db.
		Where("asset_id = ?", assetIDStr).
		Order("timestamp DESC").
		Limit(limit).
		Find(&points).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch price data"})
		return
	}

	c.JSON(http.StatusOK, points)
}
