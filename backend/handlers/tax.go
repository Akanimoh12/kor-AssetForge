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

type TaxHandler struct {
	taxService *services.TaxService
}

func NewTaxHandler(db *gorm.DB) *TaxHandler {
	return &TaxHandler{
		taxService: services.NewTaxService(db),
	}
}

type RecordTaxEventRequest struct {
	UserID               uint      `json:"user_id" binding:"required"`
	TransactionID        uint      `json:"transaction_id"`
	AssetID              uint      `json:"asset_id" binding:"required"`
	TransactionType      string    `json:"transaction_type" binding:"required"`
	Quantity             int64     `json:"quantity" binding:"required"`
	CostBasisStroops     int64     `json:"cost_basis_stroops" binding:"required"`
	SalePriceStroops     int64     `json:"sale_price_stroops"`
	TransactionDate      time.Time `json:"transaction_date" binding:"required"`
}

type GenerateTaxReportRequest struct {
	UserID  uint `json:"user_id" binding:"required"`
	TaxYear int  `json:"tax_year" binding:"required,min=2000"`
}

type RecordWithholdingRequest struct {
	UserID                   uint      `json:"user_id" binding:"required"`
	TransactionID            uint      `json:"transaction_id"`
	WithholdingAmountStroops int64     `json:"withholding_amount_stroops" binding:"required"`
	WithholdingRate          float64   `json:"withholding_rate" binding:"required"`
	WithholdingDate          time.Time `json:"withholding_date"`
}

type ExportTaxReportRequest struct {
	UserID   uint   `json:"user_id" binding:"required"`
	ReportID uint   `json:"report_id" binding:"required"`
	Format   string `json:"format" binding:"required,oneof=pdf csv json"`
}

func (th *TaxHandler) RecordTaxEvent(c *gin.Context) {
	var req RecordTaxEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := &models.TaxRecord{
		UserID:               req.UserID,
		TransactionID:        req.TransactionID,
		AssetID:              req.AssetID,
		TransactionType:      req.TransactionType,
		Quantity:             req.Quantity,
		CostBasisStroops:     req.CostBasisStroops,
		SalePriceStroops:     req.SalePriceStroops,
		TransactionDate:      req.TransactionDate,
		CreatedAt:            time.Now(),
	}

	capitalGain := th.taxService.CalculateCapitalGain(
		req.CostBasisStroops,
		req.SalePriceStroops,
		req.Quantity,
	)
	record.CapitalGainLossStroops = capitalGain

	if err := th.taxService.RecordTaxEvent(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

func (th *TaxHandler) GenerateTaxReport(c *gin.Context) {
	var req GenerateTaxReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := th.taxService.GenerateTaxReport(req.UserID, req.TaxYear)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

func (th *TaxHandler) GetTaxReport(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	taxYear, err := strconv.Atoi(c.Query("tax_year"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tax_year"})
		return
	}

	report, err := th.taxService.GetTaxReport(uint(userID), taxYear)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tax report not found"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (th *TaxHandler) ListTaxReports(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	reports, err := th.taxService.ListTaxReports(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

func (th *TaxHandler) Generate1099Form(c *gin.Context) {
	reportID, err := strconv.ParseUint(c.Query("report_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report_id"})
		return
	}

	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	form, err := th.taxService.Generate1099Form(uint(reportID), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, form)
}

func (th *TaxHandler) Get1099Forms(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	forms, err := th.taxService.Get1099Forms(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"forms": forms})
}

func (th *TaxHandler) RecordWithholding(c *gin.Context) {
	var req RecordWithholdingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	withholding := &models.TaxWithholding{
		UserID:                   req.UserID,
		TransactionID:            req.TransactionID,
		WithholdingAmountStroops: req.WithholdingAmountStroops,
		WithholdingRate:          req.WithholdingRate,
		WithholdingDate:          req.WithholdingDate,
		Status:                   "completed",
	}

	if err := th.taxService.RecordWithholding(withholding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, withholding)
}

func (th *TaxHandler) GetWithholdings(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	withholdings, err := th.taxService.GetWithholdings(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"withholdings": withholdings})
}

func (th *TaxHandler) ExportTaxReport(c *gin.Context) {
	var req ExportTaxReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	export, err := th.taxService.ExportTaxReport(req.ReportID, req.UserID, req.Format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, export)
}

func (th *TaxHandler) GetTaxExports(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	exports, err := th.taxService.GetTaxExports(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exports": exports})
}

func (th *TaxHandler) GetTaxSummary(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate time.Time

	if startDateStr != "" {
		parsedStart, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format"})
			return
		}
		startDate = parsedStart
	} else {
		startDate = time.Now().AddDate(-1, 0, 0)
	}

	if endDateStr != "" {
		parsedEnd, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format"})
			return
		}
		endDate = parsedEnd
	} else {
		endDate = time.Now()
	}

	summary, err := th.taxService.GetTaxSummary(uint(userID), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}
