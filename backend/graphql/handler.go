// Package graphql wires a minimal GraphQL endpoint alongside the REST API.
// It exposes GET /graphql (playground) and POST /graphql (query execution).
// Cursor-based pagination follows the Relay Connection spec via the pagination_service.
package graphql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
)

// Handler provides the GraphQL HTTP handlers.
type Handler struct {
	DB *gorm.DB
}

// NewHandler creates a GraphQL Handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

// Playground serves a simple HTML page pointing users at the GraphQL endpoint.
// GET /graphql
func (h *Handler) Playground(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!DOCTYPE html>
<html><head><title>AssetForge GraphQL</title></head>
<body>
  <h1>AssetForge GraphQL</h1>
  <p>Send POST requests to <code>/graphql</code> with a JSON body:</p>
  <pre>{ "query": "{ assets(first: 10) { edges { node { id name } cursor } pageInfo { hasNextPage endCursor } totalCount } }" }</pre>
</body></html>`))
}

type graphqlRequest struct {
	Query     string                 `json:"query" binding:"required"`
	Variables map[string]interface{} `json:"variables"`
}

// Execute handles GraphQL query execution.
// POST /graphql
// Body: { "query": "...", "variables": {...} }
func (h *Handler) Execute(c *gin.Context) {
	var req graphqlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": []gin.H{{"message": err.Error()}}})
		return
	}

	query := strings.TrimSpace(req.Query)
	vars := req.Variables

	// Merge inline variables into vars map for simple queries
	if vars == nil {
		vars = make(map[string]interface{})
	}

	data, gqlErrors := h.resolve(query, vars)
	if len(gqlErrors) > 0 {
		c.JSON(http.StatusOK, gin.H{"data": data, "errors": gqlErrors})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) resolve(query string, vars map[string]interface{}) (interface{}, []gin.H) {
	q := strings.ToLower(strings.TrimSpace(query))
	// Remove opening brace if bare query shorthand
	q = strings.TrimPrefix(q, "{")

	switch {
	case strings.Contains(q, "assetssummary") || strings.Contains(q, "analyticssummary"):
		return h.resolveAnalytics()
	case strings.Contains(q, "assets(") || strings.Contains(q, "assets ("):
		return h.resolveAssets(query, vars)
	case strings.Contains(q, "asset(") || strings.Contains(q, "asset ("):
		return h.resolveAsset(vars)
	case strings.Contains(q, "userprofile"):
		return h.resolveUserProfile(vars)
	default:
		return nil, []gin.H{{"message": "unknown query field"}}
	}
}

// assetNode is the GraphQL representation of an asset
type assetNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	AssetType   string `json:"assetType"`
	Status      string `json:"status"`
	TotalSupply string `json:"totalSupply"`
	CreatedAt   string `json:"createdAt"`
}

type assetEdge struct {
	Node   assetNode `json:"node"`
	Cursor string    `json:"cursor"`
}

type assetConnection struct {
	Edges      []assetEdge       `json:"edges"`
	PageInfo   services.PageInfo `json:"pageInfo"`
	TotalCount int64             `json:"totalCount"`
}

func (h *Handler) resolveAssets(rawQuery string, vars map[string]interface{}) (interface{}, []gin.H) {
	// Extract pagination vars (from variables map or defaults)
	var first, last *int
	var after, before string
	var statusFilter string

	if v, ok := vars["first"]; ok {
		if n, ok := toInt(v); ok {
			first = &n
		}
	}
	if v, ok := vars["last"]; ok {
		if n, ok := toInt(v); ok {
			last = &n
		}
	}
	if v, ok := vars["after"]; ok {
		after, _ = v.(string)
	}
	if v, ok := vars["before"]; ok {
		before, _ = v.(string)
	}
	if v, ok := vars["status"]; ok {
		statusFilter, _ = v.(string)
	}

	// Simple inline arg parsing for queries without variables
	if first == nil && last == nil && !strings.Contains(rawQuery, "$") {
		n := 20
		first = &n
	}

	limit, afterID, beforeID, forward, err := services.NormalizePaginationArgs(first, last, after, before)
	if err != nil {
		return nil, []gin.H{{"message": err.Error()}}
	}

	var total int64
	q := h.DB.Model(&models.Asset{})
	if statusFilter != "" {
		q = q.Where("verified = ?", statusFilter == "active" || statusFilter == "verified")
	}
	q.Count(&total)

	// Fetch limit+1 to determine hasNextPage/hasPreviousPage
	fetchLimit := limit + 1
	assetQuery := h.DB.Model(&models.Asset{})
	if statusFilter != "" {
		assetQuery = assetQuery.Where("verified = ?", statusFilter == "active" || statusFilter == "verified")
	}

	if forward {
		if afterID > 0 {
			assetQuery = assetQuery.Where("id > ?", afterID)
		}
		if beforeID > 0 {
			assetQuery = assetQuery.Where("id < ?", beforeID)
		}
		assetQuery = assetQuery.Order("id asc").Limit(fetchLimit)
	} else {
		if beforeID > 0 {
			assetQuery = assetQuery.Where("id < ?", beforeID)
		}
		if afterID > 0 {
			assetQuery = assetQuery.Where("id > ?", afterID)
		}
		assetQuery = assetQuery.Order("id desc").Limit(fetchLimit)
	}

	var assets []models.Asset
	if err := assetQuery.Find(&assets).Error; err != nil {
		return nil, []gin.H{{"message": "failed to fetch assets"}}
	}

	hasMore := len(assets) > limit
	if hasMore {
		assets = assets[:limit]
	}

	// Reverse for backward pagination
	if !forward {
		for i, j := 0, len(assets)-1; i < j; i, j = i+1, j-1 {
			assets[i], assets[j] = assets[j], assets[i]
		}
	}

	edges := make([]assetEdge, 0, len(assets))
	for _, a := range assets {
		status := "inactive"
		if a.Verified {
			status = "active"
		}
		edges = append(edges, assetEdge{
			Node: assetNode{
				ID:          fmt.Sprintf("%d", a.ID),
				Name:        a.Name,
				Symbol:      a.Symbol,
				AssetType:   a.AssetType,
				Status:      status,
				TotalSupply: fmt.Sprintf("%d", a.TotalSupply),
				CreatedAt:   a.CreatedAt.Format(time.RFC3339),
			},
			Cursor: services.EncodeCursor(a.ID),
		})
	}

	var startID, endID uint
	hasNext, hasPrev := false, false
	if len(assets) > 0 {
		startID = assets[0].ID
		endID = assets[len(assets)-1].ID
		if forward {
			hasNext = hasMore
			hasPrev = afterID > 0
		} else {
			hasPrev = hasMore
			hasNext = beforeID > 0
		}
	}

	return gin.H{
		"assets": assetConnection{
			Edges:      edges,
			PageInfo:   services.BuildPageInfo(hasNext, hasPrev, startID, endID),
			TotalCount: total,
		},
	}, nil
}

func (h *Handler) resolveAsset(vars map[string]interface{}) (interface{}, []gin.H) {
	id, ok := vars["id"]
	if !ok {
		return nil, []gin.H{{"message": "id variable required"}}
	}
	var asset models.Asset
	if err := h.DB.First(&asset, id).Error; err != nil {
		return gin.H{"asset": nil}, nil
	}
	status := "inactive"
	if asset.Verified {
		status = "active"
	}
	return gin.H{"asset": assetNode{
		ID:          fmt.Sprintf("%d", asset.ID),
		Name:        asset.Name,
		Symbol:      asset.Symbol,
		AssetType:   asset.AssetType,
		Status:      status,
		TotalSupply: fmt.Sprintf("%d", asset.TotalSupply),
		CreatedAt:   asset.CreatedAt.Format(time.RFC3339),
	}}, nil
}

func (h *Handler) resolveUserProfile(vars map[string]interface{}) (interface{}, []gin.H) {
	id, ok := vars["id"]
	if !ok {
		return nil, []gin.H{{"message": "id variable required"}}
	}
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		return gin.H{"userProfile": nil}, nil
	}
	return gin.H{"userProfile": gin.H{
		"id":             fmt.Sprintf("%d", user.ID),
		"username":       user.Username,
		"stellarAddress": user.StellarAddress,
		"createdAt":      user.CreatedAt.Format(time.RFC3339),
	}}, nil
}

func (h *Handler) resolveAnalytics() (interface{}, []gin.H) {
	var totalUsers, totalAssets, activeListings int64
	h.DB.Model(&models.User{}).Count(&totalUsers)
	h.DB.Model(&models.Asset{}).Count(&totalAssets)
	h.DB.Model(&models.Listing{}).Where("active = ?", true).Count(&activeListings)
	return gin.H{"analyticsSummary": gin.H{
		"totalUsers":        totalUsers,
		"totalAssets":       totalAssets,
		"activeListings":    activeListings,
		"reportGeneratedAt": time.Now().Format(time.RFC3339),
	}}, nil
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}
