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

// StakingHandler handles staking and reward distribution HTTP requests
type StakingHandler struct {
	db *gorm.DB
}

// NewStakingHandler creates a new StakingHandler
func NewStakingHandler(db *gorm.DB) *StakingHandler {
	return &StakingHandler{db: db}
}

type stakeRequest struct {
	UserID         uint   `json:"user_id" binding:"required,gt=0"`
	AssetID        uint   `json:"asset_id" binding:"required,gt=0"`
	StellarAddress string `json:"stellar_address" binding:"required"`
	Amount         int64  `json:"amount" binding:"required,gt=0"`
}

type unstakeRequest struct {
	StakeID uint  `json:"stake_id" binding:"required,gt=0"`
	Amount  int64 `json:"amount" binding:"required,gt=0"`
}

type claimRewardRequest struct {
	StakeID uint `json:"stake_id" binding:"required,gt=0"`
}

type distributeRewardsRequest struct {
	AssetID        uint  `json:"asset_id" binding:"required,gt=0"`
	APRBasisPoints int   `json:"apr_basis_points" binding:"required,gt=0,max=100000"`
}

// Stake creates or increases a stake position
// @Summary Stake tokens
// @Description Stake an amount of an asset to start earning rewards
// @Tags staking
// @Accept json
// @Produce json
// @Param body body stakeRequest true "Stake details"
// @Success 201 {object} models.StakePosition
// @Router /staking/stake [post]
func (h *StakingHandler) Stake(c *gin.Context) {
	var req stakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var asset models.Asset
	if err := h.db.First(&asset, req.AssetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	var user models.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Look for an existing active position to top up
	var position models.StakePosition
	err := h.db.Where("user_id = ? AND asset_id = ? AND active = true", req.UserID, req.AssetID).
		First(&position).Error

	if err == gorm.ErrRecordNotFound {
		// New position
		now := time.Now()
		position = models.StakePosition{
			UserID:         req.UserID,
			AssetID:        req.AssetID,
			StakedAmount:   req.Amount,
			StellarAddress: req.StellarAddress,
			StakedAt:       now,
			Active:         true,
		}
		if err := h.db.Create(&position).Error; err != nil {
			apperrors.AbortWithError(c, apperrors.Wrap(err, apperrors.CodeDatabaseError, "Failed to create stake position", http.StatusInternalServerError))
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	} else {
		// Top up existing position
		position.StakedAmount += req.Amount
		if err := h.db.Save(&position).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stake"})
			return
		}
	}

	c.JSON(http.StatusCreated, position)
}

// Unstake reduces a stake position
// @Summary Unstake tokens
// @Description Reduce or close a stake position
// @Tags staking
// @Accept json
// @Produce json
// @Param body body unstakeRequest true "Unstake details"
// @Success 200 {object} models.StakePosition
// @Router /staking/unstake [post]
func (h *StakingHandler) Unstake(c *gin.Context) {
	var req unstakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var position models.StakePosition
	if err := h.db.First(&position, req.StakeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stake position not found"})
		return
	}

	if !position.Active {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stake position is already closed"})
		return
	}

	if req.Amount > position.StakedAmount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot unstake more than staked amount"})
		return
	}

	position.StakedAmount -= req.Amount
	if position.StakedAmount == 0 {
		position.Active = false
	}

	if err := h.db.Save(&position).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unstake"})
		return
	}

	c.JSON(http.StatusOK, position)
}

// GetStakingDashboard returns staking positions for a user
// @Summary Staking dashboard
// @Description Get all stake positions and accumulated rewards for a user
// @Tags staking
// @Param user_id query int true "User ID"
// @Success 200 {array} models.StakePosition
// @Router /staking/dashboard [get]
func (h *StakingHandler) GetStakingDashboard(c *gin.Context) {
	type query struct {
		UserID uint `form:"user_id" binding:"required,gt=0"`
	}
	var q query
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var positions []models.StakePosition
	if err := h.db.Preload("Asset").
		Where("user_id = ?", q.UserID).
		Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch positions"})
		return
	}

	var totalStaked, totalAccrued, totalClaimed int64
	for _, p := range positions {
		if p.Active {
			totalStaked += p.StakedAmount
		}
		totalAccrued += p.AccruedRewards
		totalClaimed += p.ClaimedRewards
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":      positions,
		"total_staked":   totalStaked,
		"total_accrued":  totalAccrued,
		"total_claimed":  totalClaimed,
		"pending_rewards": totalAccrued - totalClaimed,
	})
}

// ClaimRewards allows a staker to claim their accrued rewards
// @Summary Claim staking rewards
// @Description Claim all accrued rewards for a stake position
// @Tags staking
// @Accept json
// @Produce json
// @Param body body claimRewardRequest true "Claim details"
// @Success 200 {object} models.RewardClaim
// @Router /staking/claim [post]
func (h *StakingHandler) ClaimRewards(c *gin.Context) {
	var req claimRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var position models.StakePosition
	if err := h.db.First(&position, req.StakeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stake position not found"})
		return
	}

	pendingRewards := position.AccruedRewards - position.ClaimedRewards
	if pendingRewards <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No rewards to claim"})
		return
	}

	var claim models.RewardClaim
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		claim = models.RewardClaim{
			StakeID:   position.ID,
			UserID:    position.UserID,
			AssetID:   position.AssetID,
			Amount:    pendingRewards,
			ClaimedAt: time.Now(),
		}
		if err := tx.Create(&claim).Error; err != nil {
			return err
		}
		position.ClaimedRewards += pendingRewards
		return tx.Save(&position).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process claim"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Rewards claimed successfully",
		"claim":   claim,
		"amount":  pendingRewards,
	})
}

// DistributeRewards runs an automated reward distribution for all stakers of an asset
// @Summary Distribute staking rewards (admin)
// @Description Calculate and credit rewards to all active stakers for an asset
// @Tags staking
// @Accept json
// @Produce json
// @Param body body distributeRewardsRequest true "Distribution parameters"
// @Success 200 {object} models.RewardDistribution
// @Router /admin/staking/distribute [post]
func (h *StakingHandler) DistributeRewards(c *gin.Context) {
	var req distributeRewardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var positions []models.StakePosition
	if err := h.db.Where("asset_id = ? AND active = true", req.AssetID).Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stakers"})
		return
	}

	if len(positions) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No active stakers to distribute to"})
		return
	}

	periodEnd := time.Now()
	var periodStart time.Time
	var totalDistributed int64

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		for i := range positions {
			p := &positions[i]

			// Calculate duration staked since last reward (or staked_at)
			since := p.StakedAt
			if p.LastRewardAt != nil {
				since = *p.LastRewardAt
			}
			if i == 0 {
				periodStart = since
			}

			durationSecs := int64(periodEnd.Sub(since).Seconds())
			if durationSecs <= 0 {
				continue
			}

			// reward = staked * apr_bps / 10000 * duration_secs / 31536000
			reward := p.StakedAmount * int64(req.APRBasisPoints) / 10000 * durationSecs / 31_536_000
			if reward <= 0 {
				continue
			}

			now := periodEnd
			p.AccruedRewards += reward
			p.LastRewardAt = &now
			totalDistributed += reward

			if err := tx.Save(p).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to distribute rewards"})
		return
	}

	distribution := models.RewardDistribution{
		AssetID:          req.AssetID,
		TotalDistributed: totalDistributed,
		StakerCount:      len(positions),
		APRBasisPoints:   req.APRBasisPoints,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
	}
	h.db.Create(&distribution)

	c.JSON(http.StatusOK, gin.H{
		"message":      "Rewards distributed successfully",
		"distribution": distribution,
		"stakers":      len(positions),
		"total":        totalDistributed,
	})
}

// GetRewardHistory returns reward distribution history for an asset
// @Summary Reward history
// @Description List reward distribution events for an asset
// @Tags staking
// @Param asset_id query int true "Asset ID"
// @Success 200 {object} utils.Pagination
// @Router /staking/rewards/history [get]
func (h *StakingHandler) GetRewardHistory(c *gin.Context) {
	type query struct {
		AssetID uint `form:"asset_id" binding:"required,gt=0"`
		Page    int  `form:"page,default=1"`
		Limit   int  `form:"limit,default=20"`
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

	db := h.db.Model(&models.RewardDistribution{}).
		Where("asset_id = ?", q.AssetID).Order("created_at DESC")

	var dists []models.RewardDistribution
	var total int64
	paginationRes, err := utils.Paginate(db, c, q.Page, q.Limit, &total, &dists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reward history"})
		return
	}
	c.JSON(http.StatusOK, paginationRes)
}
