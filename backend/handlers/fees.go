package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
	"gorm.io/gorm"
)

type FeeHandler struct {
	feeService *services.FeeService
}

func NewFeeHandler(db *gorm.DB) *FeeHandler {
	return &FeeHandler{
		feeService: services.NewFeeService(db),
	}
}

type CreateFeeConfigRequest struct {
	AssetType   string `json:"asset_type" binding:"required"`
	BaseFeeBps  int16  `json:"base_fee_bps" binding:"required,min=0"`
	Description string `json:"description"`
}

type AddDiscountTierRequest struct {
	FeeConfigID      uint   `json:"fee_config_id" binding:"required"`
	TierName         string `json:"tier_name" binding:"required"`
	MinVolumeStroops int64  `json:"min_volume_stroops" binding:"required"`
	MaxVolumeStroops int64  `json:"max_volume_stroops"`
	DiscountBps      int16  `json:"discount_bps" binding:"required,min=0"`
}

type CalculateFeeRequest struct {
	AssetType string `json:"asset_type" binding:"required"`
	Amount    int64  `json:"amount" binding:"required,min=1"`
}

type CalculateFeeResponse struct {
	Amount             int64  `json:"amount"`
	FeeStroops         int64  `json:"fee_stroops"`
	AssetType          string `json:"asset_type"`
	BaseFeeBps         int16  `json:"base_fee_bps"`
	AppliedDiscountBps int16  `json:"applied_discount_bps"`
	NetFeeBps          int16  `json:"net_fee_bps"`
}

func (fh *FeeHandler) CreateFeeConfiguration(c *gin.Context) {
	var req CreateFeeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &models.FeeConfiguration{
		AssetType:   req.AssetType,
		BaseFeeBps:  req.BaseFeeBps,
		Description: req.Description,
		Active:      true,
	}

	if err := fh.feeService.CreateFeeConfiguration(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

func (fh *FeeHandler) UpdateFeeConfiguration(c *gin.Context) {
	id := c.Param("id")
	configID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fee configuration id"})
		return
	}

	var req CreateFeeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &models.FeeConfiguration{
		BaseFeeBps:  req.BaseFeeBps,
		Description: req.Description,
	}

	if err := fh.feeService.UpdateFeeConfiguration(uint(configID), config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

func (fh *FeeHandler) GetFeeConfiguration(c *gin.Context) {
	assetType := c.Query("asset_type")
	if assetType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_type is required"})
		return
	}

	config, err := fh.feeService.GetFeeConfiguration(assetType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fee configuration not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

func (fh *FeeHandler) ListFeeConfigurations(c *gin.Context) {
	configs, err := fh.feeService.GetAllFeeConfigurations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"configurations": configs})
}

func (fh *FeeHandler) AddDiscountTier(c *gin.Context) {
	var req AddDiscountTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tier := &models.FeeDiscountTier{
		FeeConfigID: req.FeeConfigID,
		TierName:    req.TierName,
		DiscountBps: req.DiscountBps,
	}

	if err := fh.feeService.AddDiscountTier(tier); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tier)
}

func (fh *FeeHandler) CalculateFee(c *gin.Context) {
	var req CalculateFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	feeStroops, err := fh.feeService.CalculateFee(req.AssetType, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	config, _ := fh.feeService.GetFeeConfiguration(req.AssetType)
	discountBps := int16(0)
	if config != nil {
		for _, tier := range config.DiscountTiers {
			discountBps = tier.DiscountBps
		}
	}

	netFeeBps := int16(0)
	if config != nil {
		netFeeBps = config.BaseFeeBps - discountBps
		if netFeeBps < 0 {
			netFeeBps = 0
		}
	}

	response := CalculateFeeResponse{
		Amount:             req.Amount,
		FeeStroops:         feeStroops,
		AssetType:          req.AssetType,
		BaseFeeBps:         config.BaseFeeBps,
		AppliedDiscountBps: discountBps,
		NetFeeBps:          netFeeBps,
	}

	c.JSON(http.StatusOK, response)
}

func (fh *FeeHandler) GenerateFeeReport(c *gin.Context) {
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_time format"})
			return
		}
	} else {
		startTime = time.Now().AddDate(0, -1, 0)
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_time format"})
			return
		}
	} else {
		endTime = time.Now()
	}

	report, err := fh.feeService.GenerateFeeReport(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (fh *FeeHandler) GetFeeReports(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	reports, err := fh.feeService.GetFeeReports(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}
