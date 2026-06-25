package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/services"
	"gorm.io/gorm"
)

// AnalyticsHandler handles analytics and reporting endpoints.
type AnalyticsHandler struct {
	DB               *gorm.DB
	ValuationTracker *services.ValuationTrackerService
}

// NewAnalyticsHandler creates an AnalyticsHandler.
func NewAnalyticsHandler(db *gorm.DB, vt *services.ValuationTrackerService) *AnalyticsHandler {
	return &AnalyticsHandler{DB: db, ValuationTracker: vt}
}

// PlatformSummary holds aggregated platform metrics.
type PlatformSummary struct {
	TotalUsers        int64     `json:"total_users"`
	TotalAssets       int64     `json:"total_assets"`
	TotalVolume       float64   `json:"total_volume_xlm"`
	ActiveListings    int64     `json:"active_listings"`
	ReportGeneratedAt time.Time `json:"report_generated_at"`
}

// GetSummary returns key platform metrics.
// GET /api/v1/analytics/summary
func (h *AnalyticsHandler) GetSummary(c *gin.Context) {
	var summary PlatformSummary
	summary.ReportGeneratedAt = time.Now().UTC()

	h.DB.Table("users").Where("deleted_at IS NULL").Count(&summary.TotalUsers)
	h.DB.Table("assets").Where("deleted_at IS NULL").Count(&summary.TotalAssets)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// GetUserGrowth returns a daily user-registration time series.
// GET /api/v1/analytics/user-growth?days=30
func (h *AnalyticsHandler) GetUserGrowth(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := time.ParseDuration(d + "h"); err == nil {
			days = int(parsed.Hours() / 24)
		}
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	type DailyCount struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	var rows []DailyCount
	h.DB.Raw(`
		SELECT DATE(created_at) AS date, COUNT(*) AS count
		FROM users
		WHERE created_at >= ? AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC`, since).Scan(&rows)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows, "days": days})
}

// RecordValuation stores a new valuation snapshot for an asset.
// POST /api/v1/assets/:id/valuations
func (h *AnalyticsHandler) RecordValuation(c *gin.Context) {
	assetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}

	var req struct {
		ValuationUSD float64 `json:"valuation_usd" binding:"required,gt=0"`
		Currency     string  `json:"currency" binding:"required"`
		Source       string  `json:"source" binding:"required"`
		Notes        string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record, err := h.ValuationTracker.RecordValuation(uint(assetID), req.ValuationUSD, req.Currency, req.Source, req.Notes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record valuation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": record})
}

// GetValuationHistory returns paginated valuation snapshots for an asset.
// GET /api/v1/assets/:id/valuations?page=1&limit=50
func (h *AnalyticsHandler) GetValuationHistory(c *gin.Context) {
	assetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	records, total, err := h.ValuationTracker.GetHistory(uint(assetID), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch valuation history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
		"meta": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetValuationTrend returns aggregated valuation trend data for charting.
// GET /api/v1/assets/:id/valuations/trend?from=2024-01-01&to=2024-12-31&granularity=daily
func (h *AnalyticsHandler) GetValuationTrend(c *gin.Context) {
	assetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().UTC().AddDate(0, -3, 0).Format("2006-01-02"))
	toStr := c.DefaultQuery("to", time.Now().UTC().Format("2006-01-02"))
	granularity := c.DefaultQuery("granularity", "daily")

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' date (use YYYY-MM-DD)"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' date (use YYYY-MM-DD)"})
		return
	}
	to = to.Add(24*time.Hour - time.Second) // include full end day

	if granularity != "daily" && granularity != "weekly" && granularity != "monthly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "granularity must be daily, weekly, or monthly"})
		return
	}

	trend, err := h.ValuationTracker.GetTrend(uint(assetID), from, to, granularity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute trend"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"data":        trend,
		"asset_id":    assetID,
		"from":        fromStr,
		"to":          toStr,
		"granularity": granularity,
	})
}

// GetLatestValuation returns the most recent valuation for an asset.
// GET /api/v1/assets/:id/valuations/latest
func (h *AnalyticsHandler) GetLatestValuation(c *gin.Context) {
	assetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id"})
		return
	}

	v, err := h.ValuationTracker.GetLatestValuation(c.Request.Context(), uint(assetID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no valuation found for this asset"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": v})
}
