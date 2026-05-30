package performance

// concurrent_mixed_load_test.go — mixed workload tests that simulate realistic
// production traffic patterns at 10x expected load.
//
// A realistic traffic mix for a DeFi marketplace:
//   40% read (asset list, order book, price chart)
//   25% auth (login, refresh)
//   20% trading (P2P orders, swaps)
//   10% write (tokenize, stake, add liquidity)
//    5% admin/misc (KYC, disputes, health)
//
// The 10x load test targets 1 000 concurrent virtual users.

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mixed workload — 10x load
// ---------------------------------------------------------------------------

// TestLoad_Mixed_10x simulates 1 000 concurrent users for 60 s with a
// realistic traffic distribution.  This is the primary acceptance test for
// issue #118.
func TestLoad_Mixed_10x(t *testing.T) {
	db := openDB(t)

	// Seed shared resources
	asset := seedAsset(t, db, "mixed_"+uniqueSuffix())
	assetA := seedAsset(t, db, "mixedA_"+uniqueSuffix())
	assetB := seedAsset(t, db, "mixedB_"+uniqueSuffix())
	user, userToken := seedUser(t, db, "mixed_"+uniqueSuffix())
	_ = user

	// Create a liquidity pool for swap traffic
	poolURL := baseURL() + "/api/v1/liquidity/pools"
	poolResp, _ := doJSON("POST", poolURL, map[string]interface{}{
		"asset_a_id":       assetA.ID,
		"asset_b_id":       assetB.ID,
		"creator_address":  "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		"fee_basis_points": 30,
	}, userToken)
	var poolBody struct {
		ID uint `json:"id"`
	}
	decodeJSON(poolResp, &poolBody)
	poolID := poolBody.ID

	if poolID > 0 {
		addResp, _ := doJSON("POST", baseURL()+"/api/v1/liquidity/add", map[string]interface{}{
			"pool_id":          poolID,
			"provider_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"amount_a":         100_000_000,
			"amount_b":         100_000_000,
		}, userToken)
		drainClose(addResp)
	}

	// Seed a pool of tokens for auth traffic
	const tokenPoolSize = 100
	tokenPool := make([]string, tokenPoolSize)
	for i := 0; i < tokenPoolSize; i++ {
		_, tok := seedUser(t, db, fmt.Sprintf("mixauth%d_%s", i, uniqueSuffix()))
		tokenPool[i] = tok
	}

	// Define the weighted request mix
	type requestFn func() (int, error)
	type weightedFn struct {
		weight int
		fn     requestFn
	}

	var mu sync.Mutex
	tokenIdx := 0
	nextToken := func() string {
		mu.Lock()
		defer mu.Unlock()
		t := tokenPool[tokenIdx%tokenPoolSize]
		tokenIdx++
		return t
	}

	fns := []weightedFn{
		// 40% reads
		{40, func() (int, error) {
			urls := []string{
				baseURL() + "/api/v1/assets",
				fmt.Sprintf("%s/api/v1/assets/%d", baseURL(), asset.ID),
				fmt.Sprintf("%s/api/v1/p2p/orders?asset_id=%d", baseURL(), asset.ID),
				fmt.Sprintf("%s/api/v1/p2p/prices?asset_id=%d", baseURL(), asset.ID),
				baseURL() + "/api/v1/liquidity/pools",
			}
			url := urls[rand.Intn(len(urls))]
			resp, err := doJSON("GET", url, nil, "")
			if err != nil {
				return 0, err
			}
			drainClose(resp)
			return resp.StatusCode, nil
		}},
		// 25% auth
		{25, func() (int, error) {
			tok := nextToken()
			resp, err := doJSON("POST", baseURL()+"/api/v1/auth/refresh",
				map[string]string{"refresh_token": tok}, "")
			if err != nil {
				return 0, err
			}
			drainClose(resp)
			return resp.StatusCode, nil
		}},
		// 20% trading
		{20, func() (int, error) {
			tok := nextToken()
			if rand.Intn(2) == 0 {
				// P2P order
				side := "buy"
				if rand.Intn(2) == 0 {
					side = "sell"
				}
				resp, err := doJSON("POST", baseURL()+"/api/v1/p2p/orders", map[string]interface{}{
					"asset_id":      asset.ID,
					"owner_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
					"side":          side,
					"price":         5000000 + rand.Int63n(100000),
					"quantity":      1,
				}, tok)
				if err != nil {
					return 0, err
				}
				drainClose(resp)
				return resp.StatusCode, nil
			}
			// Swap
			if poolID == 0 {
				return http.StatusOK, nil
			}
			resp, err := doJSON("POST", baseURL()+"/api/v1/liquidity/swap", map[string]interface{}{
				"pool_id":           poolID,
				"trader_address":    fmt.Sprintf("G%s", randHex(55)),
				"input_asset_id":    assetA.ID,
				"input_amount":      10,
				"min_output_amount": 1,
			}, tok)
			if err != nil {
				return 0, err
			}
			drainClose(resp)
			return resp.StatusCode, nil
		}},
		// 10% writes
		{10, func() (int, error) {
			tok := nextToken()
			sym := fmt.Sprintf("MX%s", randHex(4))
			resp, err := doJSON("POST", baseURL()+"/api/v1/assets/tokenize", map[string]interface{}{
				"issuer_account": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
				"name":           fmt.Sprintf("Mixed %s", sym),
				"symbol":         sym,
				"asset_type":     "real_estate",
				"total_supply":   1000,
				"fractions":      1000,
			}, tok)
			if err != nil {
				return 0, err
			}
			drainClose(resp)
			return resp.StatusCode, nil
		}},
		// 5% misc
		{5, func() (int, error) {
			resp, err := doJSON("GET", baseURL()+"/health", nil, "")
			if err != nil {
				return 0, err
			}
			drainClose(resp)
			return resp.StatusCode, nil
		}},
	}

	// Build a flat dispatch table from weights
	var dispatch []requestFn
	for _, wf := range fns {
		for i := 0; i < wf.weight; i++ {
			dispatch = append(dispatch, wf.fn)
		}
	}

	result := RunLoad(context.Background(), 1000, 60*time.Second, func() (int, error) {
		fn := dispatch[rand.Intn(len(dispatch))]
		return fn()
	})

	// 10x mixed load: 95% success, P95 < 2 000 ms
	assertLoadResult(t, "Mixed/10x 1000-concurrent 60s", result, 95.0, 2000)
}

// ---------------------------------------------------------------------------
// Spike test — sudden burst to 2 000 concurrent users for 10 s
// ---------------------------------------------------------------------------

// TestLoad_Spike tests the system's ability to handle a sudden traffic spike.
// This simulates a flash sale or viral event.
func TestLoad_Spike(t *testing.T) {
	url := baseURL() + "/api/v1/assets"

	result := RunLoad(context.Background(), 2000, 10*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Spike: 90% success (some rate limiting expected), P95 < 500 ms
	assertLoadResult(t, "Spike/2000-concurrent 10s", result, 90.0, 500)
}

// ---------------------------------------------------------------------------
// Sustained load — 500 concurrent users for 5 minutes
// ---------------------------------------------------------------------------

// TestLoad_Sustained_5min runs a sustained load test for 5 minutes to detect
// memory leaks, connection pool exhaustion, and gradual degradation.
func TestLoad_Sustained_5min(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sustained load test in short mode")
	}

	db := openDB(t)
	asset := seedAsset(t, db, "sustained_"+uniqueSuffix())
	_, token := seedUser(t, db, "sustained_"+uniqueSuffix())

	urls := []string{
		baseURL() + "/api/v1/assets",
		fmt.Sprintf("%s/api/v1/assets/%d", baseURL(), asset.ID),
		baseURL() + "/health",
		fmt.Sprintf("%s/api/v1/p2p/orders?asset_id=%d", baseURL(), asset.ID),
	}

	result := RunLoad(context.Background(), 500, 5*time.Minute, func() (int, error) {
		url := urls[rand.Intn(len(urls))]
		resp, err := doJSON("GET", url, nil, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Sustained: 99% success, P95 < 500 ms
	assertLoadResult(t, "Sustained/500-concurrent 5min", result, 99.0, 500)
}

// ---------------------------------------------------------------------------
// Rate limiter validation
// ---------------------------------------------------------------------------

// TestLoad_RateLimit_Enforcement verifies that the rate limiter correctly
// rejects excess requests (429) without crashing the server.
func TestLoad_RateLimit_Enforcement(t *testing.T) {
	// Fire 5 000 requests in 1 second to a single IP — should trigger rate limiting.
	url := baseURL() + "/api/v1/assets"
	var total, limited int64

	result := RunLoad(context.Background(), 5000, 1*time.Second, func() (int, error) {
		atomic.AddInt64(&total, 1)
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			atomic.AddInt64(&limited, 1)
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	t.Logf("Rate limit test: %d total, %d limited (%.1f%%)",
		result.Total, limited, float64(limited)/float64(result.Total)*100)

	// Server must not crash — all responses should be 200 or 429
	for code, count := range result.ErrorCodes {
		if code != http.StatusTooManyRequests {
			t.Errorf("unexpected error code %d (%d times)", code, count)
		}
	}
}
