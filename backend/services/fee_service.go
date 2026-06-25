package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

type FeeService struct {
	db *gorm.DB
}

func NewFeeService(db *gorm.DB) *FeeService {
	return &FeeService{db: db}
}

func (fs *FeeService) CreateFeeConfiguration(config *models.FeeConfiguration) error {
	if config.AssetType == "" {
		return errors.New("asset type is required")
	}
	return fs.db.Create(config).Error
}

func (fs *FeeService) UpdateFeeConfiguration(id uint, config *models.FeeConfiguration) error {
	return fs.db.Model(&models.FeeConfiguration{}).Where("id = ?", id).Updates(config).Error
}

func (fs *FeeService) GetFeeConfiguration(assetType string) (*models.FeeConfiguration, error) {
	var config models.FeeConfiguration
	if err := fs.db.Where("asset_type = ? AND active = ?", assetType, true).
		Preload("DiscountTiers").
		First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (fs *FeeService) GetAllFeeConfigurations() ([]models.FeeConfiguration, error) {
	var configs []models.FeeConfiguration
	if err := fs.db.Where("active = ?", true).
		Preload("DiscountTiers").
		Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

func (fs *FeeService) AddDiscountTier(tier *models.FeeDiscountTier) error {
	if tier.FeeConfigID == 0 {
		return errors.New("fee config id is required")
	}
	return fs.db.Create(tier).Error
}

func (fs *FeeService) CalculateFee(assetType string, amount int64) (int64, error) {
	config, err := fs.GetFeeConfiguration(assetType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("no fee configuration found for asset type: %s", assetType)
		}
		return 0, err
	}

	baseFeeBps := int64(config.BaseFeeBps)
	discountBps := int64(0)

	for _, tier := range config.DiscountTiers {
		min := tier.MinVolumeStroops
		max := tier.MaxVolumeStroops

		if amount >= min && (max <= 0 || amount <= max) {
			discountBps = int64(tier.DiscountBps)
			break
		}
	}

	netFeeBps := baseFeeBps - discountBps
	if netFeeBps < 0 {
		netFeeBps = 0
	}

	feeStroops := (amount * netFeeBps) / 10000

	return feeStroops, nil
}

func (fs *FeeService) RecordFeeTransaction(tx *models.FeeTransaction) error {
	if tx.UserAddress == "" || tx.AssetType == "" || tx.AssetID == 0 {
		return errors.New("missing required fields for fee transaction")
	}
	return fs.db.Create(tx).Error
}

func (fs *FeeService) GenerateFeeReport(startTime, endTime time.Time) (*models.FeeReport, error) {
	var totalVolume int64
	var totalFees int64
	var count int64

	err := fs.db.Model(&models.FeeTransaction{}).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Select("SUM(transaction_amount) as total_volume, SUM(total_fee_stroops) as total_fees, COUNT(*) as count").
		Row().
		Scan(&totalVolume, &totalFees, &count)

	if err != nil {
		return nil, err
	}

	report := &models.FeeReport{
		PeriodStart:       startTime,
		PeriodEnd:         endTime,
		TotalVolume:       totalVolume,
		TotalFeeCollected: totalFees,
		TransactionCount:  count,
		GeneratedAt:       time.Now(),
	}

	if err := fs.db.Create(report).Error; err != nil {
		return nil, err
	}

	return report, nil
}

func (fs *FeeService) GetFeeReports(limit int) ([]models.FeeReport, error) {
	var reports []models.FeeReport
	if err := fs.db.Order("generated_at DESC").Limit(limit).Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}
