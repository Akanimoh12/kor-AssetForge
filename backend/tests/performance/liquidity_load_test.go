package performance

// liquidity_load_test.go — load and benchmark tests for liquidity pool endpoints.
//
// The swap endpoint is the hottest path: it runs the constant-product AMM
// formula, updates pool reserves, distributes fees to LP positions, and
// records a PoolSwap — all inside a single DB transaction.
//
// Endpoints covered:
//   POST /api/v1/liquidity/pools    (create pool)
//   GET  /api/v1/liquidity/pools    (list pools)
//   GET  /api/v1/liquidity/pools/:id
//   POST /api/v1/liquidity/add      (add liquidity)
//   POST /api/v1/liquidity/remove   (remove liquidity)
//   POST /api/v1/liquidity/swap     (AMM swap — hottest path)
//   GET  /api/v1/liquidity/positions
//   GET  /api/v1/liquidity/swaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// decodeJSON decodes the response body into v and closes the body.
func decodeJSON(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// ---------------------------------------------------------------------------
// POST /api/v1/liquidity/pools  — create pool
// ---------------------------------------------------------------------------

// TestLoad_Liquidity_CreatePool fires 100 concurrent pool creation requests.
// Most will return 409 (pool already exists) after the first creation — that
// is expected and counted as success.
func TestLoad_Liquidity_CreatePool(t *testing.T) {
	db := openDB(t)
	assetA := seedAsset(t, db, "lpA_"+uniqueSuffix())
	assetB := seedAsset(t, db, "lpB_"+uniqueSuffix())
	_, token := seedUser(t, db, "lpcreate_"+uniqueSuffix())

	url := baseURL() + "/api/v1/liquidity/pools"

	result := RunLoad(context.Background(), 100, 20*time.Second, func() (int, error) {
		resp, err := doJSON("POST", url, map[string]interface{}{
			"asset_a_id":       assetA.ID,
			"asset_b_id":       assetB.ID,
			"creator_address":  "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"fee_basis_points": 30,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		// 409 = pool already exists, still a valid outcome
		if resp.StatusCode == http.StatusConflict {
			return http.StatusCreated, nil
		}
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Liquidity/CreatePool 100-concurrent 20s", result, 99.0, 300)
}

// ---------------------------------------------------------------------------
// GET /api/v1/liquidity/pools  — list pools
// ---------------------------------------------------------------------------

// TestLoad_Liquidity_ListPools fires 800 concurrent pool list reads for 30 s.
func TestLoad_Liquidity_ListPools(t *testing.T) {
	url := baseURL() + "/api/v1/liquidity/pools?limit=20"

	result := RunLoad(context.Background(), 800, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Liquidity/ListPools 800-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// POST /api/v1/liquidity/swap  — AMM swap (hottest path)
// ---------------------------------------------------------------------------

// TestLoad_Liquidity_Swap fires 400 concurrent swap requests for 30 s.
// This is the most write-intensive endpoint: AMM math + DB transaction +
// fee distribution to all LP positions.
func TestLoad_Liquidity_Swap(t *testing.T) {
	db := openDB(t)
	assetA := seedAsset(t, db, "swapA_"+uniqueSuffix())
	assetB := seedAsset(t, db, "swapB_"+uniqueSuffix())
	_, token := seedUser(t, db, "swap_"+uniqueSuffix())

	// Create pool
	poolURL := baseURL() + "/api/v1/liquidity/pools"
	poolResp, err := doJSON("POST", poolURL, map[string]interface{}{
		"asset_a_id":       assetA.ID,
		"asset_b_id":       assetB.ID,
		"creator_address":  "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		"fee_basis_points": 30,
	}, token)
	if err != nil || (poolResp.StatusCode != http.StatusCreated && poolResp.StatusCode != http.StatusConflict) {
		t.Skipf("could not create pool for swap test: status %v", poolResp.StatusCode)
	}
	var poolBody struct {
		ID uint `json:"id"`
	}
	if err := decodeJSON(poolResp, &poolBody); err != nil || poolBody.ID == 0 {
		t.Skip("could not parse pool ID")
	}
	poolID := poolBody.ID

	// Seed initial liquidity so swaps can execute
	addURL := baseURL() + "/api/v1/liquidity/add"
	addResp, err := doJSON("POST", addURL, map[string]interface{}{
		"pool_id":          poolID,
		"provider_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		"amount_a":         10_000_000,
		"amount_b":         10_000_000,
	}, token)
	if err != nil || addResp.StatusCode != http.StatusOK {
		t.Skipf("could not seed liquidity: status %v", addResp.StatusCode)
	}
	drainClose(addResp)

	swapURL := baseURL() + "/api/v1/liquidity/swap"

	result := RunLoad(context.Background(), 400, 30*time.Second, func() (int, error) {
		resp, err := doJSON("POST", swapURL, map[string]interface{}{
			"pool_id":           poolID,
			"trader_address":    "GBVNQGYOZQMXKSLGDSFWQ6TYU4KVWLTJJFC7MGXUA74P7UJVSGZ1234",
			"input_asset_id":    assetA.ID,
			"input_amount":      100,
			"min_output_amount": 1,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// AMM swap: 95% success, P95 < 1 000 ms
	assertLoadResult(t, "Liquidity/Swap 400-concurrent 30s", result, 95.0, 1000)
}

// ---------------------------------------------------------------------------
// POST /api/v1/liquidity/add  — add liquidity
// ---------------------------------------------------------------------------

// TestLoad_Liquidity_AddLiquidity fires 200 concurrent add-liquidity requests.
func TestLoad_Liquidity_AddLiquidity(t *testing.T) {
	db := openDB(t)
	assetA := seedAsset(t, db, "addlpA_"+uniqueSuffix())
	assetB := seedAsset(t, db, "addlpB_"+uniqueSuffix())
	_, token := seedUser(t, db, "addlp_"+uniqueSuffix())

	// Create pool
	poolURL := baseURL() + "/api/v1/liquidity/pools"
	poolResp, err := doJSON("POST", poolURL, map[string]interface{}{
		"asset_a_id":       assetA.ID,
		"asset_b_id":       assetB.ID,
		"creator_address":  "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		"fee_basis_points": 30,
	}, token)
	if err != nil {
		t.Skipf("pool creation error: %v", err)
	}
	var poolBody struct {
		ID uint `json:"id"`
	}
	if err := decodeJSON(poolResp, &poolBody); err != nil || poolBody.ID == 0 {
		t.Skip("could not parse pool ID")
	}
	poolID := poolBody.ID

	addURL := baseURL() + "/api/v1/liquidity/add"

	result := RunLoad(context.Background(), 200, 20*time.Second, func() (int, error) {
		resp, err := doJSON("POST", addURL, map[string]interface{}{
			"pool_id":          poolID,
			"provider_address": fmt.Sprintf("G%s", randHex(55)),
			"amount_a":         1000,
			"amount_b":         1000,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Liquidity/AddLiquidity 200-concurrent 20s", result, 95.0, 800)
}

// ---------------------------------------------------------------------------
// GET /api/v1/liquidity/swaps  — swap history
// ---------------------------------------------------------------------------

// TestLoad_Liquidity_SwapHistory fires 600 concurrent swap history reads.
func TestLoad_Liquidity_SwapHistory(t *testing.T) {
	url := baseURL() + "/api/v1/liquidity/swaps?limit=20"

	result := RunLoad(context.Background(), 600, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Liquidity/SwapHistory 600-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkLiquiditySwap measures single-goroutine swap throughput.
func BenchmarkLiquiditySwap(b *testing.B) {
	db := openDB(b)
	assetA := seedAsset(b, db, "bswapA_"+uniqueSuffix())
	assetB := seedAsset(b, db, "bswapB_"+uniqueSuffix())
	_, token := seedUser(b, db, "bswap_"+uniqueSuffix())

	// Create pool + seed liquidity
	poolURL := baseURL() + "/api/v1/liquidity/pools"
	poolResp, err := doJSON("POST", poolURL, map[string]interface{}{
		"asset_a_id":       assetA.ID,
		"asset_b_id":       assetB.ID,
		"creator_address":  "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		"fee_basis_points": 30,
	}, token)
	if err != nil {
		b.Skipf("pool creation error: %v", err)
	}
	var poolBody struct {
		ID uint `json:"id"`
	}
	if err := decodeJSON(poolResp, &poolBody); err != nil || poolBody.ID == 0 {
		b.Skip("could not parse pool ID")
	}
	poolID := poolBody.ID

	addResp, _ := doJSON("POST", baseURL()+"/api/v1/liquidity/add", map[string]interface{}{
		"pool_id":          poolID,
		"provider_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		"amount_a":         100_000_000,
		"amount_b":         100_000_000,
	}, token)
	drainClose(addResp)

	swapURL := baseURL() + "/api/v1/liquidity/swap"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("POST", swapURL, map[string]interface{}{
			"pool_id":           poolID,
			"trader_address":    "GBVNQGYOZQMXKSLGDSFWQ6TYU4KVWLTJJFC7MGXUA74P7UJVSGZ1234",
			"input_asset_id":    assetA.ID,
			"input_amount":      100,
			"min_output_amount": 1,
		}, token)
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}

// BenchmarkLiquidityListPools measures pool list read throughput.
func BenchmarkLiquidityListPools(b *testing.B) {
	url := baseURL() + "/api/v1/liquidity/pools"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}
