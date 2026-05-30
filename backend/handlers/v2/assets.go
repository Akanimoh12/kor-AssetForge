// Package v2 contains API v2 handlers. v2 extends v1 with richer response
// envelopes and additional filtering capabilities.
package v2

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

// AssetsHandler provides v2 asset endpoints
type AssetsHandler struct {
	db *gorm.DB
}

// NewAssetsHandler creates a new v2 assets handler
func NewAssetsHandler(db *gorm.DB) *AssetsHandler {
	return &AssetsHandler{db: db}
}

type assetListResponse struct {
	Data       []models.Asset `json:"data"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalCount int64          `json:"total_count"`
	APIVersion string         `json:"api_version"`
}

// ListAssets returns a paginated, filterable list of assets (v2)
// @Summary List assets (v2)
// @Description Returns a paginated list of assets with richer metadata
// @Tags assets
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Items per page (default 20, max 100)"
// @Param asset_type query string false "Filter by asset type"
// @Param min_price query number false "Minimum price filter"
// @Param max_price query number false "Maximum price filter"
// @Success 200 {object} assetListResponse
// @Router /api/v2/assets [get]
func (h *AssetsHandler) ListAssets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := h.db.Model(&models.Asset{})

	if assetType := c.Query("asset_type"); assetType != "" {
		query = query.Where("asset_type = ?", assetType)
	}
	if minPrice := c.Query("min_price"); minPrice != "" {
		if v, err := strconv.ParseFloat(minPrice, 64); err == nil {
			query = query.Where("price_per_token >= ?", v)
		}
	}
	if maxPrice := c.Query("max_price"); maxPrice != "" {
		if v, err := strconv.ParseFloat(maxPrice, 64); err == nil {
			query = query.Where("price_per_token <= ?", v)
		}
	}

	var total int64
	query.Count(&total)

	var assets []models.Asset
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&assets)

	c.JSON(http.StatusOK, assetListResponse{
		Data:       assets,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		APIVersion: "v2",
	})
}

// GetAsset returns a single asset with v2 envelope
// @Summary Get asset (v2)
// @Tags assets
// @Produce json
// @Param id path int true "Asset ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v2/assets/:id [get]
func (h *AssetsHandler) GetAsset(c *gin.Context) {
	var asset models.Asset
	if err := h.db.First(&asset, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":        asset,
		"api_version": "v2",
	})
}
