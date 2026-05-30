package performance

// assets_load_test.go — load and benchmark tests for asset and marketplace endpoints.
//
// Endpoints covered:
//   GET  /api/v1/assets            (cached list)
//   GET  /api/v1/assets/:id        (cached detail)
//   POST /api/v1/assets/tokenize   (write + cache invalidation)
//   POST /api/v1/marketplace/list  (write)
//   POST /api/v1/marketplace/transfer (write + email notification)
//   GET  /api/v1/transactions      (paginated read)

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// GET /api/v1/assets  — cached list
// ---------------------------------------------------------------------------

// TestLoad_Assets_List fires 1 000 concurrent GET /assets requests for 30 s.
// The response is served from Redis cache after the first hit, so latency
// should be very low.
func TestLoad_Assets_List(t *testing.T) {
	db := openDB(t)
	// Seed a few assets so the list is non-empty.
	for i := 0; i < 5; i++ {
		seedAsset(t, db, fmt.Sprintf("list%d_%s", i, uniqueSuffix()))
	}

	url := baseURL() + "/api/v1/assets"

	result := RunLoad(context.Background(), 1000, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Cached reads: 99% success, P95 < 100 ms
	assertLoadResult(t, "Assets/List 1000-concurrent 30s", result, 99.0, 100)
}

// ---------------------------------------------------------------------------
// GET /api/v1/assets/:id  — cached detail
// ---------------------------------------------------------------------------

// TestLoad_Assets_GetByID fires 1 000 concurrent GET /assets/:id requests.
func TestLoad_Assets_GetByID(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "detail_"+uniqueSuffix())

	url := fmt.Sprintf("%s/api/v1/assets/%d", baseURL(), asset.ID)

	result := RunLoad(context.Background(), 1000, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Cached reads: 99% success, P95 < 100 ms
	assertLoadResult(t, "Assets/GetByID 1000-concurrent 30s", result, 99.0, 100)
}

// ---------------------------------------------------------------------------
// POST /api/v1/assets/tokenize  — write path
// ---------------------------------------------------------------------------

// TestLoad_Assets_Tokenize fires 200 concurrent tokenize requests for 20 s.
// Each request creates a unique asset to avoid symbol conflicts.
func TestLoad_Assets_Tokenize(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() {
		db.Exec("DELETE FROM assets WHERE name LIKE 'PerfTok%'")
	})

	_, token := seedUser(t, db, "tok_"+uniqueSuffix())
	url := baseURL() + "/api/v1/assets/tokenize"

	result := RunLoad(context.Background(), 200, 20*time.Second, func() (int, error) {
		sym := fmt.Sprintf("PT%s", randHex(4))
		resp, err := doJSON("POST", url, map[string]interface{}{
			"issuer_account": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"name":           fmt.Sprintf("PerfTok %s", sym),
			"symbol":         sym,
			"asset_type":     "real_estate",
			"total_supply":   1000000,
			"fractions":      1000000,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Write path: 95% success, P95 < 500 ms
	assertLoadResult(t, "Assets/Tokenize 200-concurrent 20s", result, 95.0, 500)
}

// ---------------------------------------------------------------------------
// POST /api/v1/marketplace/list  — create listing
// ---------------------------------------------------------------------------

// TestLoad_Marketplace_List fires 300 concurrent listing creation requests.
func TestLoad_Marketplace_List(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "mktlist_"+uniqueSuffix())
	_, token := seedUser(t, db, "mktlist_"+uniqueSuffix())

	url := baseURL() + "/api/v1/marketplace/list"

	result := RunLoad(context.Background(), 300, 20*time.Second, func() (int, error) {
		resp, err := doJSON("POST", url, map[string]interface{}{
			"asset_id":       asset.ID,
			"seller_addr":    "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"amount":         100,
			"price_per_unit": 5000000,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Write path: 95% success, P95 < 500 ms
	assertLoadResult(t, "Marketplace/List 300-concurrent 20s", result, 95.0, 500)
}

// ---------------------------------------------------------------------------
// POST /api/v1/marketplace/transfer  — asset transfer
// ---------------------------------------------------------------------------

// TestLoad_Marketplace_Transfer fires 200 concurrent transfer requests.
func TestLoad_Marketplace_Transfer(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "transfer_"+uniqueSuffix())
	_, token := seedUser(t, db, "transfer_"+uniqueSuffix())

	url := baseURL() + "/api/v1/marketplace/transfer"

	result := RunLoad(context.Background(), 200, 20*time.Second, func() (int, error) {
		resp, err := doJSON("POST", url, map[string]interface{}{
			"asset_id":     asset.ID,
			"from_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"to_address":   "GBVNQGYOZQMXKSLGDSFWQ6TYU4KVWLTJJFC7MGXUA74P7UJVSGZ1234",
			"amount":       10,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Write + email notification: 95% success, P95 < 800 ms
	assertLoadResult(t, "Marketplace/Transfer 200-concurrent 20s", result, 95.0, 800)
}

// ---------------------------------------------------------------------------
// GET /api/v1/transactions  — paginated read
// ---------------------------------------------------------------------------

// TestLoad_Transactions_List fires 500 concurrent paginated transaction reads.
func TestLoad_Transactions_List(t *testing.T) {
	url := baseURL() + "/api/v1/transactions?page=1&limit=20"

	result := RunLoad(context.Background(), 500, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Paginated DB read: 99% success, P95 < 300 ms
	assertLoadResult(t, "Transactions/List 500-concurrent 30s", result, 99.0, 300)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkGetAssets measures single-goroutine asset list throughput.
func BenchmarkGetAssets(b *testing.B) {
	url := baseURL() + "/api/v1/assets"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			b.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status %d", resp.StatusCode)
		}
		drainClose(resp)
	}
}

// BenchmarkGetAssetByID measures single-goroutine asset detail throughput.
func BenchmarkGetAssetByID(b *testing.B) {
	db := openDB(b)
	asset := seedAsset(b, db, "bench_detail_"+uniqueSuffix())
	url := fmt.Sprintf("%s/api/v1/assets/%d", baseURL(), asset.ID)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}

// BenchmarkTokenizeAsset measures single-goroutine tokenization throughput.
func BenchmarkTokenizeAsset(b *testing.B) {
	db := openDB(b)
	_, token := seedUser(b, db, "bench_tok_"+uniqueSuffix())
	b.Cleanup(func() {
		db.Exec("DELETE FROM assets WHERE name LIKE 'BenchTok%'")
	})
	url := baseURL() + "/api/v1/assets/tokenize"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sym := fmt.Sprintf("BT%s", randHex(4))
		resp, err := doJSON("POST", url, map[string]interface{}{
			"issuer_account": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"name":           fmt.Sprintf("BenchTok %s", sym),
			"symbol":         sym,
			"asset_type":     "real_estate",
			"total_supply":   1000000,
			"fractions":      1000000,
		}, token)
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}
