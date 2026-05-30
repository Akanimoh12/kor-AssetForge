package performance

// p2p_load_test.go — load and benchmark tests for the P2P secondary marketplace.
//
// P2P order creation is the most complex write path: it creates an order then
// immediately runs synchronous price-time priority matching inside a DB
// transaction.  This is a known bottleneck under high concurrency.
//
// Endpoints covered:
//   POST /api/v1/p2p/orders          (create + match)
//   GET  /api/v1/p2p/orders          (order book read)
//   PUT  /api/v1/p2p/orders/:id/cancel
//   GET  /api/v1/p2p/trades          (trade history)
//   GET  /api/v1/p2p/prices          (OHLCV chart)

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// POST /api/v1/p2p/orders  — create order with matching
// ---------------------------------------------------------------------------

// TestLoad_P2P_CreateOrder fires 300 concurrent order creation requests for 30 s.
// Half are buy orders, half are sell orders, so matching runs on every request.
func TestLoad_P2P_CreateOrder(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "p2p_"+uniqueSuffix())
	_, token := seedUser(t, db, "p2p_"+uniqueSuffix())

	url := baseURL() + "/api/v1/p2p/orders"

	result := RunLoad(context.Background(), 300, 30*time.Second, func() (int, error) {
		side := "buy"
		if time.Now().UnixNano()%2 == 0 {
			side = "sell"
		}
		resp, err := doJSON("POST", url, map[string]interface{}{
			"asset_id":         asset.ID,
			"owner_address":    "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"side":             side,
			"price":            5000000 + (time.Now().UnixNano() % 100000),
			"quantity":         10,
			"expires_in_seconds": 3600,
		}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Order creation + matching: 95% success, P95 < 1 000 ms
	assertLoadResult(t, "P2P/CreateOrder 300-concurrent 30s", result, 95.0, 1000)
}

// ---------------------------------------------------------------------------
// GET /api/v1/p2p/orders  — order book read
// ---------------------------------------------------------------------------

// TestLoad_P2P_ListOrders fires 800 concurrent order book reads for 30 s.
func TestLoad_P2P_ListOrders(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "p2plist_"+uniqueSuffix())

	url := fmt.Sprintf("%s/api/v1/p2p/orders?asset_id=%d&limit=20", baseURL(), asset.ID)

	result := RunLoad(context.Background(), 800, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Read-heavy: 99% success, P95 < 200 ms
	assertLoadResult(t, "P2P/ListOrders 800-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// PUT /api/v1/p2p/orders/:id/cancel
// ---------------------------------------------------------------------------

// TestLoad_P2P_CancelOrder fires 200 concurrent cancel requests for 20 s.
// We pre-seed a pool of open orders and cancel them in rotation.
func TestLoad_P2P_CancelOrder(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "p2pcancel_"+uniqueSuffix())
	_, token := seedUser(t, db, "p2pcancel_"+uniqueSuffix())

	// Pre-seed 500 open orders.
	createURL := baseURL() + "/api/v1/p2p/orders"
	orderIDs := make([]uint, 0, 500)
	for i := 0; i < 500; i++ {
		resp, err := doJSON("POST", createURL, map[string]interface{}{
			"asset_id":      asset.ID,
			"owner_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"side":          "buy",
			"price":         9999999,
			"quantity":      1,
		}, token)
		if err != nil || resp.StatusCode != http.StatusCreated {
			drainClose(resp)
			continue
		}
		var body struct {
			Order struct {
				ID uint `json:"id"`
			} `json:"order"`
		}
		if err := decodeJSON(resp, &body); err == nil && body.Order.ID > 0 {
			orderIDs = append(orderIDs, body.Order.ID)
		}
	}
	if len(orderIDs) == 0 {
		t.Skip("could not seed orders for cancel test")
	}

	var idx int64
	result := RunLoad(context.Background(), 200, 20*time.Second, func() (int, error) {
		i := int(atomic.AddInt64(&idx, 1))
		id := orderIDs[i%len(orderIDs)]
		url := fmt.Sprintf("%s/api/v1/p2p/orders/%d/cancel", baseURL(), id)
		resp, err := doJSON("PUT", url, nil, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		// 400 is expected once an order is already cancelled — count as success
		if resp.StatusCode == http.StatusBadRequest {
			return http.StatusOK, nil
		}
		return resp.StatusCode, nil
	})

	// Cancel: 95% success, P95 < 500 ms
	assertLoadResult(t, "P2P/CancelOrder 200-concurrent 20s", result, 95.0, 500)
}

// ---------------------------------------------------------------------------
// GET /api/v1/p2p/trades  — trade history
// ---------------------------------------------------------------------------

// TestLoad_P2P_TradeHistory fires 600 concurrent trade history reads for 30 s.
func TestLoad_P2P_TradeHistory(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "p2ptrades_"+uniqueSuffix())

	url := fmt.Sprintf("%s/api/v1/p2p/trades?asset_id=%d&limit=20", baseURL(), asset.ID)

	result := RunLoad(context.Background(), 600, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Read: 99% success, P95 < 200 ms
	assertLoadResult(t, "P2P/TradeHistory 600-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// GET /api/v1/p2p/prices  — OHLCV chart
// ---------------------------------------------------------------------------

// TestLoad_P2P_PriceChart fires 500 concurrent price chart reads for 30 s.
func TestLoad_P2P_PriceChart(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "p2pprice_"+uniqueSuffix())

	url := fmt.Sprintf("%s/api/v1/p2p/prices?asset_id=%d", baseURL(), asset.ID)

	result := RunLoad(context.Background(), 500, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Read: 99% success, P95 < 200 ms
	assertLoadResult(t, "P2P/PriceChart 500-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkP2PCreateOrder measures single-goroutine order creation throughput.
func BenchmarkP2PCreateOrder(b *testing.B) {
	db := openDB(b)
	asset := seedAsset(b, db, "bench_p2p_"+uniqueSuffix())
	_, token := seedUser(b, db, "bench_p2p_"+uniqueSuffix())
	url := baseURL() + "/api/v1/p2p/orders"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		side := "buy"
		if i%2 == 0 {
			side = "sell"
		}
		resp, err := doJSON("POST", url, map[string]interface{}{
			"asset_id":      asset.ID,
			"owner_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
			"side":          side,
			"price":         5000000,
			"quantity":      1,
		}, token)
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}

// BenchmarkP2PListOrders measures order book read throughput.
func BenchmarkP2PListOrders(b *testing.B) {
	db := openDB(b)
	asset := seedAsset(b, db, "bench_p2plist_"+uniqueSuffix())
	url := fmt.Sprintf("%s/api/v1/p2p/orders?asset_id=%d", baseURL(), asset.ID)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("GET", url, nil, "")
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}
