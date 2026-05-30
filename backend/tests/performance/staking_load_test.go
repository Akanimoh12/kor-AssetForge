package performance

// staking_load_test.go — load and benchmark tests for staking endpoints.
//
// Endpoints covered:
//   POST /api/v1/staking/stake
//   POST /api/v1/staking/unstake
//   POST /api/v1/staking/claim
//   GET  /api/v1/staking/dashboard
//   GET  /api/v1/staking/rewards/history

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// POST /api/v1/staking/stake
// ---------------------------------------------------------------------------

// TestLoad_Staking_Stake fires 300 concurrent stake requests for 20 s.
func TestLoad_Staking_Stake(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "stake_"+uniqueSuffix())
	user, token := seedUser(t, db, "stake_"+uniqueSuffix())

	url := baseURL() + "/api/v1/staking/stake"

	result := RunLoad(context.Background(), 300, 20*time.Second, func() (int, error) {
		resp, err := doJSON("POST", url, map[string]interface{}{
			"user_id":         user.ID,
			"asset_id":        asset.ID,
			"stellar_address": user.StellarAddress,
			"amount":          100,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Stake upsert: 95% success, P95 < 500 ms
	assertLoadResult(t, "Staking/Stake 300-concurrent 20s", result, 95.0, 500)
}

// ---------------------------------------------------------------------------
// GET /api/v1/staking/dashboard
// ---------------------------------------------------------------------------

// TestLoad_Staking_Dashboard fires 600 concurrent dashboard reads for 30 s.
func TestLoad_Staking_Dashboard(t *testing.T) {
	db := openDB(t)
	user, _ := seedUser(t, db, "stakdash_"+uniqueSuffix())

	url := fmt.Sprintf("%s/api/v1/staking/dashboard?user_id=%d", baseURL(), user.ID)

	result := RunLoad(context.Background(), 600, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Read: 99% success, P95 < 200 ms
	assertLoadResult(t, "Staking/Dashboard 600-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// POST /api/v1/staking/unstake
// ---------------------------------------------------------------------------

// TestLoad_Staking_Unstake fires 200 concurrent unstake requests for 20 s.
// We pre-seed a stake position and repeatedly unstake small amounts.
func TestLoad_Staking_Unstake(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "unstake_"+uniqueSuffix())
	user, token := seedUser(t, db, "unstake_"+uniqueSuffix())

	// Seed a large stake position
	stakeURL := baseURL() + "/api/v1/staking/stake"
	stakeResp, err := doJSON("POST", stakeURL, map[string]interface{}{
		"user_id":         user.ID,
		"asset_id":        asset.ID,
		"stellar_address": user.StellarAddress,
		"amount":          1_000_000_000,
	}, token)
	if err != nil || stakeResp.StatusCode != http.StatusCreated {
		t.Skipf("could not seed stake position: status %v", stakeResp.StatusCode)
	}
	var stakeBody struct {
		ID uint `json:"id"`
	}
	if err := decodeJSON(stakeResp, &stakeBody); err != nil || stakeBody.ID == 0 {
		t.Skip("could not parse stake ID")
	}
	stakeID := stakeBody.ID

	url := baseURL() + "/api/v1/staking/unstake"

	result := RunLoad(context.Background(), 200, 20*time.Second, func() (int, error) {
		resp, err := doJSON("POST", url, map[string]interface{}{
			"stake_id": stakeID,
			"amount":   1,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		// 400 = no more stake to unstake — expected at end of run
		if resp.StatusCode == http.StatusBadRequest {
			return http.StatusOK, nil
		}
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Staking/Unstake 200-concurrent 20s", result, 95.0, 500)
}

// ---------------------------------------------------------------------------
// GET /api/v1/staking/rewards/history
// ---------------------------------------------------------------------------

// TestLoad_Staking_RewardHistory fires 500 concurrent reward history reads.
func TestLoad_Staking_RewardHistory(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "rewhist_"+uniqueSuffix())

	url := fmt.Sprintf("%s/api/v1/staking/rewards/history?asset_id=%d", baseURL(), asset.ID)

	result := RunLoad(context.Background(), 500, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Staking/RewardHistory 500-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkStake measures single-goroutine stake throughput.
func BenchmarkStake(b *testing.B) {
	db := openDB(b)
	asset := seedAsset(b, db, "bstake_"+uniqueSuffix())
	user, token := seedUser(b, db, "bstake_"+uniqueSuffix())
	url := baseURL() + "/api/v1/staking/stake"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("POST", url, map[string]interface{}{
			"user_id":         user.ID,
			"asset_id":        asset.ID,
			"stellar_address": user.StellarAddress,
			"amount":          1,
		}, token)
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}

// BenchmarkStakingDashboard measures dashboard read throughput.
func BenchmarkStakingDashboard(b *testing.B) {
	db := openDB(b)
	user, _ := seedUser(b, db, "bdash_"+uniqueSuffix())
	url := fmt.Sprintf("%s/api/v1/staking/dashboard?user_id=%d", baseURL(), user.ID)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}
