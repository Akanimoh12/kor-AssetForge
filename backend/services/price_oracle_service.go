package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type PriceProvider string

const (
	ProviderChainlink PriceProvider = "chainlink"
	ProviderBand      PriceProvider = "band"
	ProviderCustom    PriceProvider = "custom"
)

type PriceFeed struct {
	AssetID   uint    `json:"asset_id"`
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
	Source    string  `json:"source"`
	Decimals  int     `json:"decimals"`
}

type OraclePriceService struct {
	mu        sync.RWMutex
	feeds     map[uint]*PriceFeed
	providers map[string]PriceProvider
	client    *http.Client
}

func NewOraclePriceService() *OraclePriceService {
	return &OraclePriceService{
		feeds: make(map[uint]*PriceFeed),
		providers: map[string]PriceProvider{
			"chainlink": ProviderChainlink,
			"band":      ProviderBand,
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *OraclePriceService) FetchPrice(symbol string) (*PriceFeed, error) {
	provider := os.Getenv("PRICE_ORACLE_PROVIDER")
	if provider == "" {
		provider = "chainlink"
	}

	switch PriceProvider(provider) {
	case ProviderChainlink:
		return s.fetchChainlink(symbol)
	case ProviderBand:
		return s.fetchBand(symbol)
	default:
		return s.fetchCustom(symbol)
	}
}

func (s *OraclePriceService) fetchChainlink(symbol string) (*PriceFeed, error) {
	url := fmt.Sprintf("https://min-api.cryptocompare.com/data/price?fsym=%s&tsyms=USD", symbol)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("oracle: chainlink fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oracle: failed to read response: %w", err)
	}

	var result map[string]float64
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("oracle: failed to parse response: %w", err)
	}

	price, ok := result["USD"]
	if !ok {
		return nil, fmt.Errorf("oracle: no USD price for %s", symbol)
	}

	feed := &PriceFeed{
		Price:     price,
		Timestamp: time.Now().Unix(),
		Source:    "chainlink",
		Decimals:  8,
	}

	return feed, nil
}

func (s *OraclePriceService) fetchBand(symbol string) (*PriceFeed, error) {
	url := fmt.Sprintf("https://api.bandprotocol.com/bandchain/price?symbol=%s", symbol)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("oracle: Band Protocol fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oracle: failed to read Band response: %w", err)
	}

	var result struct {
		Price     float64 `json:"price"`
		Timestamp int64   `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("oracle: failed to parse Band response: %w", err)
	}

	return &PriceFeed{
		Price:     result.Price,
		Timestamp: result.Timestamp,
		Source:    "band",
		Decimals:  9,
	}, nil
}

func (s *OraclePriceService) fetchCustom(symbol string) (*PriceFeed, error) {
	customURL := os.Getenv("CUSTOM_PRICE_ORACLE_URL")
	if customURL == "" {
		return nil, fmt.Errorf("oracle: CUSTOM_PRICE_ORACLE_URL not set")
	}

	url := fmt.Sprintf("%s?symbol=%s", customURL, symbol)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("oracle: custom fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oracle: failed to read custom response: %w", err)
	}

	var feed PriceFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("oracle: failed to parse custom response: %w", err)
	}

	feed.Source = "custom"
	return &feed, nil
}

func (s *OraclePriceService) UpdateFeed(assetID uint, feed *PriceFeed) {
	s.mu.Lock()
	defer s.mu.Unlock()
	feed.AssetID = assetID
	s.feeds[assetID] = feed
}

func (s *OraclePriceService) GetFeed(assetID uint) *PriceFeed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.feeds[assetID]
}

func (s *OraclePriceService) IsStale(assetID uint, maxAge time.Duration) bool {
	feed := s.GetFeed(assetID)
	if feed == nil {
		return true
	}
	return time.Since(time.Unix(feed.Timestamp, 0)) > maxAge
}
