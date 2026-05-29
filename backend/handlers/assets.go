package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/yourusername/kor-assetforge/apperrors"
	"github.com/yourusername/kor-assetforge/models"
	"github.com/yourusername/kor-assetforge/services"
	"github.com/yourusername/kor-assetforge/utils"
	"github.com/yourusername/kor-assetforge/validator"
	"gorm.io/gorm"
)

type AssetHandler struct {
	db            *gorm.DB
	stellarClient *utils.StellarClient
	redisClient   *redis.Client
	emailService  services.EmailService
}

func NewAssetHandler(db *gorm.DB, stellarClient *utils.StellarClient, redisClient *redis.Client, emailService services.EmailService) *AssetHandler {
	return &AssetHandler{
		db:            db,
		stellarClient: stellarClient,
		redisClient:   redisClient,
		emailService:  emailService,
	}
}

// TokenizeAsset handles formal asset tokenization with Soroban integration
// @Summary Tokenize a new asset
// @Description Create a new fractionalized asset on the Stellar network
// @Tags assets
// @Accept json
// @Produce json
// @Param asset body validator.TokenizeAssetRequest true "Asset creation details"
// @Success 201 {object} models.Asset
// @Failure 400 {object} apperrors.ErrorResponse
// @Failure 500 {object} apperrors.ErrorResponse
// @Router /assets/tokenize [post]
// @Router /assets [post]
func (h *AssetHandler) TokenizeAsset(c *gin.Context) {
	var req validator.TokenizeAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeBadRequest, "Invalid request payload", http.StatusBadRequest))
		return
	}

	validator.SanitizeStruct(&req)
	req.Symbol = strings.ToUpper(req.Symbol)

	if err := validator.ValidateStruct(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadataJSON, _ := json.Marshal(req.Metadata)

	asset := models.Asset{
		Name:         req.Name,
		Symbol:       req.Symbol,
		Description:  req.Description,
		AssetType:    req.AssetType,
		TotalSupply:  req.TotalSupply,
		Fractions:    req.Fractions,
		OwnerAddress: req.IssuerAccount,
		Metadata:     string(metadataJSON),
		Verified:     false,
	}

	if err := h.db.Create(&asset).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to create asset record", http.StatusInternalServerError))
		return
	}

	if h.redisClient != nil {
		ctx := context.Background()
		if err := h.redisClient.Del(ctx, "kor:asset:list:page1").Err(); err != nil {
			log.Printf("Warning: failed to invalidate cache for list: %v", err)
		}
	}

	params := []interface{}{req.Name, req.Symbol, req.TotalSupply, req.IssuerAccount}
	contractID := "CXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

	txHash, err := h.stellarClient.InvokeContract(contractID, "mint", params)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{
			"message": "Asset created in database but contract invocation failed",
			"asset":   asset,
		})
		return
	}

	h.db.Model(&asset).Update("verified", true)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Asset tokenized successfully",
		"asset":   asset,
		"tx_hash": txHash,
	})
}

// ListAssets returns all assets with pagination
// @Summary List all assets
// @Description Get a paginated list of all tokenized assets
// @Tags assets
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Page size" default(10)
// @Success 200 {object} utils.Pagination
// @Failure 500 {object} apperrors.ErrorResponse
// @Router /assets [get]
func (h *AssetHandler) ListAssets(c *gin.Context) {
	cacheKey := "kor:asset:list:page1"

	// Try fetching from Redis first
	if h.redisClient != nil {
		ctx := context.Background()
		cachedData, err := h.redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			log.Printf("Cache hit for %s", cacheKey)
			c.Data(http.StatusOK, "application/json", []byte(cachedData))
			return
		} else if err != redis.Nil {
			log.Printf("Redis error on Get %s: %v", cacheKey, err)
		}
	}

	var query validator.PaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page := query.Page
	if page == 0 {
		page = 1
	}
	limit := query.Limit
	if limit == 0 {
		limit = 10
	}

	var assets []models.Asset
	var total int64
	if err := utils.Paginate(h.db, page, limit, &total, &assets); err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to fetch assets", http.StatusInternalServerError))
		return
	}

	// Save to Redis (simplified: only cache page 1 default view for now to match upstream)
	if h.redisClient != nil && page == 1 {
		if jsonData, err := json.Marshal(paginationRes); err == nil {
			ctx := context.Background()
			if err := h.redisClient.Set(ctx, cacheKey, jsonData, 5*time.Minute).Err(); err != nil {
				log.Printf("Warning: failed to cache list: %v", err)
			}
		}
	}

	c.JSON(http.StatusOK, paginationRes)
}

// ListTransactions returns all transactions with pagination
// @Summary List transactions
// @Description Get a paginated list of all asset transactions
// @Tags marketplace
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Page size" default(10)
// @Param asset_id query int false "Filter by asset ID"
// @Success 200 {object} utils.Pagination
// @Failure 400 {object} apperrors.ErrorResponse
// @Failure 500 {object} apperrors.ErrorResponse
// @Router /transactions [get]
func (h *AssetHandler) ListTransactions(c *gin.Context) {
	var queryParams validator.TransactionQuery
	if err := c.ShouldBindQuery(&queryParams); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page := queryParams.Page
	if page == 0 {
		page = 1
	}
	limit := queryParams.Limit
	if limit == 0 {
		limit = 10
	}

	var transactions []models.Transaction
	var total int64
	query := h.db.Model(&models.Transaction{}).Order("created_at desc")
	if queryParams.AssetID != 0 {
		query = query.Where("asset_id = ?", queryParams.AssetID)
	}

	paginationRes, err := utils.Paginate(query, c, page, limit, &total, &transactions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}

	c.JSON(http.StatusOK, paginationRes)
}

// GetAsset returns a specific asset
// @Summary Get asset details
// @Description Get detailed information about a specific asset by its ID
// @Tags assets
// @Accept json
// @Produce json
// @Param id path int true "Asset ID"
// @Success 200 {object} models.Asset
// @Failure 404 {object} apperrors.ErrorResponse
// @Router /assets/{id} [get]
func (h *AssetHandler) GetAsset(c *gin.Context) {
	var uri validator.AssetIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	cacheKey := fmt.Sprintf("kor:asset:detail:%d", uri.ID)

	// Try fetching from Redis first
	if h.redisClient != nil {
		ctx := context.Background()
		cachedData, err := h.redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			log.Printf("Cache hit for %s", cacheKey)
			c.Data(http.StatusOK, "application/json", []byte(cachedData))
			return
		} else if err != redis.Nil {
			log.Printf("Redis error on Get %s: %v", cacheKey, err)
		}
	}

	var asset models.Asset
	if err := h.db.First(&asset, uri.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	// Save to Redis
	if h.redisClient != nil {
		if jsonData, err := json.Marshal(asset); err == nil {
			ctx := context.Background()
			if err := h.redisClient.Set(ctx, cacheKey, jsonData, 5*time.Minute).Err(); err != nil {
				log.Printf("Warning: failed to cache detail for %d: %v", uri.ID, err)
			}
		}
	}

	c.JSON(http.StatusOK, asset)
}

// ListAssetForSale creates a marketplace listing
// @Summary List asset for sale
// @Description Create a new marketplace listing for a tokenized asset
// @Tags marketplace
// @Accept json
// @Produce json
// @Param listing body validator.ListAssetSaleRequest true "Listing details"
// @Success 201 {object} models.Listing
// @Failure 400 {object} apperrors.ErrorResponse
// @Failure 404 {object} apperrors.ErrorResponse
// @Failure 500 {object} apperrors.ErrorResponse
// @Router /marketplace/list [post]
func (h *AssetHandler) ListAssetForSale(c *gin.Context) {
	var req validator.ListAssetSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validator.SanitizeStruct(&req)
	if err := validator.ValidateStruct(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure asset exists before creating a listing
	var asset models.Asset
	if err := h.db.First(&asset, req.AssetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	listingID := "listing_1"
	listing := models.Listing{
		AssetID:      req.AssetID,
		SellerAddr:   req.SellerAddr,
		Amount:       req.Amount,
		PricePerUnit: req.PricePerUnit,
		Active:       true,
		ListingID:    listingID,
	}

	if err := h.db.Create(&listing).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to create listing", http.StatusInternalServerError))
		return
	}

	if h.redisClient != nil {
		ctx := context.Background()
		detailKey := fmt.Sprintf("kor:asset:detail:%d", req.AssetID)
		if err := h.redisClient.Del(ctx, detailKey).Err(); err != nil {
			log.Printf("Warning: failed to invalidate cache for asset %d: %v", req.AssetID, err)
		}
	}

	c.JSON(http.StatusCreated, listing)
}

// TransferAsset handles asset transfers
// @Summary Transfer asset ownership
// @Description Transfer asset tokens from one address to another
// @Tags marketplace
// @Accept json
// @Produce json
// @Param transfer body validator.TransferAssetRequest true "Transfer details"
// @Success 200 {object} models.Transaction
// @Failure 400 {object} apperrors.ErrorResponse
// @Failure 404 {object} apperrors.ErrorResponse
// @Failure 500 {object} apperrors.ErrorResponse
// @Router /marketplace/transfer [post]
func (h *AssetHandler) TransferAsset(c *gin.Context) {
	var req validator.TransferAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validator.SanitizeStruct(&req)
	if err := validator.ValidateStruct(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure asset exists before recording the transfer
	var asset models.Asset
	if err := h.db.First(&asset, req.AssetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	txHash := "tx_hash_placeholder"
	transaction := models.Transaction{
		AssetID:     req.AssetID,
		FromAddress: req.FromAddress,
		ToAddress:   req.ToAddress,
		Amount:      req.Amount,
		TxHash:      txHash,
		Status:      "pending",
	}

	if err := h.db.Create(&transaction).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to record transaction", http.StatusInternalServerError))
		return
	}

	if h.emailService != nil {
		if err := h.notifyTransactionParticipants(&transaction); err != nil {
			log.Printf("failed to queue transaction confirmation email: %v", err)
		}
	}

	if h.redisClient != nil {
		ctx := context.Background()
		detailKey := fmt.Sprintf("kor:asset:detail:%d", req.AssetID)
		if err := h.redisClient.Del(ctx, detailKey).Err(); err != nil {
			log.Printf("Warning: failed to invalidate cache for asset %d: %v", req.AssetID, err)
		}
	}

	c.JSON(http.StatusOK, transaction)
}

// UpdateMetadata updates the NFT metadata URI and hash for an asset
func (h *AssetHandler) UpdateMetadata(c *gin.Context) {
	var req validator.UpdateMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var asset models.Asset
	if err := h.db.First(&asset, req.AssetID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Asset not found"))
		return
	}

	if asset.IsImmutable {
		apperrors.AbortWithError(c, apperrors.NewForbiddenError("Metadata is immutable after minting"))
		return
	}

	asset.MetadataURI = req.MetadataURI
	asset.MetadataHash = req.MetadataHash
	if err := h.db.Save(&asset).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to update metadata"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Metadata updated successfully",
		"metadata_uri":  asset.MetadataURI,
		"metadata_hash": asset.MetadataHash,
	})
}

// GetMetadata returns the metadata for an asset
func (h *AssetHandler) GetMetadata(c *gin.Context) {
	var uri validator.AssetIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	var asset models.Asset
	if err := h.db.First(&asset, uri.ID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Asset not found"))
		return
	}

	result := models.NFTMetadata{
		Name:        asset.Name,
		Description: asset.Description,
		Image:       asset.ImageURL,
		ExternalURL: asset.MetadataURI,
		Attributes:  nil,
	}

	if asset.Metadata != "" {
		var attrs []models.NFTAttribute
		var props map[string]interface{}
		if err := json.Unmarshal([]byte(asset.Metadata), &props); err == nil {
			for k, v := range props {
				attrs = append(attrs, models.NFTAttribute{
					TraitType: k,
					Value:     v,
				})
			}
		}
		result.Attributes = attrs
		result.Properties = props
	}

	c.JSON(http.StatusOK, gin.H{
		"asset":         asset,
		"nft_metadata":  result,
		"metadata_uri":  asset.MetadataURI,
		"metadata_hash": asset.MetadataHash,
		"is_immutable":  asset.IsImmutable,
		"ipfs_cid":      asset.IPFSCID,
	})
}

// MakeMetadataImmutable marks asset metadata as immutable
func (h *AssetHandler) MakeMetadataImmutable(c *gin.Context) {
	var req validator.MakeImmutableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid request data", err))
		return
	}

	var asset models.Asset
	if err := h.db.First(&asset, req.AssetID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Asset not found"))
		return
	}

	if asset.MetadataURI == "" {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Set metadata URI before making immutable"))
		return
	}

	asset.IsImmutable = true
	if err := h.db.Save(&asset).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to make metadata immutable"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Metadata is now immutable",
		"is_immutable": true,
	})
}

// GetOraclePrice returns the current price from the oracle for an asset symbol
func (h *AssetHandler) GetOraclePrice(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Symbol query parameter is required"))
		return
	}

	oracleService := services.NewOraclePriceService()
	feed, err := oracleService.FetchPrice(symbol)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewExternalServiceError("Failed to fetch price from oracle", err))
		return
	}

	c.JSON(http.StatusOK, feed)
}

// GetAssetOraclePrice returns the current oracle price for a specific asset
func (h *AssetHandler) GetAssetOraclePrice(c *gin.Context) {
	var uri validator.AssetIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	var asset models.Asset
	if err := h.db.First(&asset, uri.ID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Asset not found"))
		return
	}

	oracleService := services.NewOraclePriceService()
	feed, err := oracleService.FetchPrice(asset.Symbol)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewExternalServiceError("Failed to fetch price from oracle", err))
		return
	}

	oracleService.UpdateFeed(asset.ID, feed)
	staleDuration := 1 * time.Hour
	isStale := oracleService.IsStale(asset.ID, staleDuration)

	c.JSON(http.StatusOK, gin.H{
		"asset_id":  asset.ID,
		"symbol":    asset.Symbol,
		"price":     feed.Price,
		"source":    feed.Source,
		"decimals":  feed.Decimals,
		"timestamp": feed.Timestamp,
		"is_stale":  isStale,
	})
}

// ExecuteBatch processes a batch of transactions atomically
func (h *AssetHandler) ExecuteBatch(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	var req validator.BatchTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.AbortWithError(c, apperrors.NewValidationError("Invalid batch request", err))
		return
	}

	if len(req.Operations) == 0 {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Batch must contain at least one operation"))
		return
	}

	if len(req.Operations) > 50 {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Batch cannot exceed 50 operations"))
		return
	}

	operationsJSON, _ := json.Marshal(req.Operations)

	batch := models.BatchTransaction{
		UserID:          userID.(uint),
		Operations:      string(operationsJSON),
		Status:          "pending",
		TotalOperations: len(req.Operations),
	}

	if err := h.db.Create(&batch).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewInternalError("Failed to create batch"))
		return
	}

	var completedOps int
	var failedOps int
	var lastError string

	tx := h.db.Begin()

	for i, op := range req.Operations {
		switch op.Type {
		case "transfer":
			var asset models.Asset
			if err := tx.First(&asset, op.AssetID).Error; err != nil {
				failedOps++
				lastError = fmt.Sprintf("operation %d: asset not found", i)
				continue
			}

			transaction := models.Transaction{
				AssetID:     op.AssetID,
				FromAddress: op.FromAddress,
				ToAddress:   op.ToAddress,
				Amount:      op.Amount,
				TxHash:      fmt.Sprintf("batch_%d_op_%d", batch.ID, i),
				Status:      "confirmed",
			}
			if err := tx.Create(&transaction).Error; err != nil {
				failedOps++
				lastError = fmt.Sprintf("operation %d: %v", i, err)
				continue
			}
			completedOps++

		case "list":
			listing := models.Listing{
				AssetID:      op.AssetID,
				SellerAddr:   op.FromAddress,
				Amount:       op.Amount,
				PricePerUnit: 0,
				Active:       true,
				ListingID:    fmt.Sprintf("batch_listing_%d_%d", batch.ID, i),
			}
			if op.ExtraParams != nil {
				if price, ok := op.ExtraParams["price_per_unit"].(float64); ok {
					listing.PricePerUnit = int64(price)
				}
			}
			if err := tx.Create(&listing).Error; err != nil {
				failedOps++
				lastError = fmt.Sprintf("operation %d: %v", i, err)
				continue
			}
			completedOps++

		case "cancel_listing":
			var listingID string
			if op.ExtraParams != nil {
				if id, ok := op.ExtraParams["listing_id"].(string); ok {
					listingID = id
				}
			}
			if listingID == "" {
				failedOps++
				lastError = fmt.Sprintf("operation %d: listing_id required", i)
				continue
			}
			if err := tx.Model(&models.Listing{}).Where("listing_id = ?", listingID).Update("active", false).Error; err != nil {
				failedOps++
				lastError = fmt.Sprintf("operation %d: %v", i, err)
				continue
			}
			completedOps++

		default:
			failedOps++
			lastError = fmt.Sprintf("operation %d: unsupported type %s", i, op.Type)
		}
	}

	batch.CompletedCount = completedOps
	batch.FailedCount = failedOps
	batch.ErrorDetails = lastError

	if failedOps > 0 && completedOps == 0 {
		tx.Rollback()
		batch.Status = "failed"
		h.db.Save(&batch)
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("All batch operations failed: "+lastError))
		return
	}

	if failedOps > 0 {
		batch.Status = "completed_with_errors"
	} else {
		batch.Status = "completed"
	}

	tx.Commit()

	batch.TxHash = fmt.Sprintf("batch_tx_%d", batch.ID)
	h.db.Save(&batch)

	c.JSON(http.StatusCreated, gin.H{
		"message":          "Batch processed",
		"batch_id":         batch.ID,
		"status":           batch.Status,
		"total_operations": batch.TotalOperations,
		"completed":        completedOps,
		"failed":           failedOps,
		"tx_hash":          batch.TxHash,
	})
}

// GetBatchStatus returns the status of a batch transaction
func (h *AssetHandler) GetBatchStatus(c *gin.Context) {
	batchID := c.Param("id")
	if batchID == "" {
		apperrors.AbortWithError(c, apperrors.NewBadRequestError("Batch ID is required"))
		return
	}

	var batch models.BatchTransaction
	if err := h.db.First(&batch, batchID).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.NewNotFoundError("Batch not found"))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// ListBatchTransactions returns batch transactions for the authenticated user
func (h *AssetHandler) ListBatchTransactions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		apperrors.AbortWithError(c, apperrors.NewUnauthorizedError("User not authenticated"))
		return
	}

	var query validator.BatchStatusQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page := query.Page
	if page == 0 {
		page = 1
	}
	limit := query.Limit
	if limit == 0 {
		limit = 10
	}

	var batches []models.BatchTransaction
	var total int64
	dbQuery := h.db.Model(&models.BatchTransaction{}).Where("user_id = ?", userID).Order("created_at desc")
	if query.Status != "" {
		dbQuery = dbQuery.Where("status = ?", query.Status)
	}

	paginationRes, err := utils.Paginate(dbQuery, c, page, limit, &total, &batches)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.NewDatabaseError("Failed to fetch batches", err))
		return
	}

	c.JSON(http.StatusOK, paginationRes)
}

func (h *AssetHandler) notifyTransactionParticipants(transaction *models.Transaction) error {
	participants := map[string]struct{}{}
	for _, addr := range []string{transaction.FromAddress, transaction.ToAddress} {
		participants[addr] = struct{}{}
	}

	for addr := range participants {
		var user models.User
		if err := h.db.Select("email", "username").Where("stellar_address = ?", addr).First(&user).Error; err != nil {
			continue
		}
		if err := h.emailService.SendTransactionConfirmation(user.Email, user.Username, transaction.TxHash, transaction.Amount, transaction.AssetID, transaction.FromAddress, transaction.ToAddress); err != nil {
			return err
		}
	}
	return nil
}
