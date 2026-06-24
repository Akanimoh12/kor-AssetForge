package handlers

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/kor-assetforge/middleware"
)

type AuditHandler struct{ service *middleware.AuditService }

func NewAuditHandler(service *middleware.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

func (h *AuditHandler) List(c *gin.Context) {
	filter, ok := h.bindFilter(c)
	if !ok {
		return
	}
	logs, total, err := h.service.Search(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to query audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total, "page": page(filter.Page), "limit": limit(filter.Limit)})
}

func (h *AuditHandler) Export(c *gin.Context) {
	filter, ok := h.bindFilter(c)
	if !ok {
		return
	}
	filter.Page, filter.Limit = 1, 10000
	logs, _, err := h.service.Search(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to export audit logs"})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=asset-audit-"+time.Now().UTC().Format("20060102")+".csv")
	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	_ = w.Write([]string{"id", "created_at", "user_id", "action", "asset_id", "ip_address", "status", "before_state", "after_state"})
	for _, log := range logs {
		user := ""
		if log.UserID != nil {
			user = strconv.FormatUint(uint64(*log.UserID), 10)
		}
		_ = w.Write([]string{strconv.FormatUint(uint64(log.ID), 10), log.CreatedAt.Format(time.RFC3339), user, log.Action, log.ResourceID, log.IPAddress, strconv.Itoa(log.Status), middleware.DecodeAuditState(log.BeforeState, log.StateEncoding), middleware.DecodeAuditState(log.AfterState, log.StateEncoding)})
	}
}

func (h *AuditHandler) bindFilter(c *gin.Context) (middleware.AuditQuery, bool) {
	var filter middleware.AuditQuery
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return filter, false
	}
	return filter, true
}
func page(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
func limit(value int) int {
	if value < 1 || value > 10000 {
		return 50
	}
	return value
}
