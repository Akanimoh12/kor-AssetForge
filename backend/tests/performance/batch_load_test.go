package performance

// batch_load_test.go — load and benchmark tests for batch transaction endpoints.
//
// Batch execute is the most write-intensive single endpoint: it processes up
// to 50 operations inside one DB transaction.  Under high concurrency this
// creates significant lock contention on the assets and listings tables.
//
// Endpoints covered:
//   POST /api/v1/batch/execute
//   GET  /api/v1/batch/:id
//   GET  /api/v1/batches

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// POST /api/v1/batch/execute  — batch write (hottest write path)
// ---------------------------------------------------------------------------

// TestLoad_Batch_Execute_Small fires 200 concurrent small-batch (5 ops) requests.
func TestLoad_Batch_Execute_Small(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "batchsm_"+uniqueSuffix())
	user, token := seedUser(t, db, "batchsm_"+uniqueSuffix())
	_ = user

	url := baseURL() + "/api/v1/batch/execute"

	result := RunLoad(context.Background(), 200, 30*time.Second, func() (int, error) {
		ops := make([]map[string]interface{}, 5)
		for i := range ops {
			ops[i] = map[string]interface{}{
				"type":         "transfer",
				"asset_id":     asset.ID,
				"from_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
				"to_address":   fmt.Sprintf("G%s", randHex(55)),
				"amount":       1,
			}
		}
		resp, err := doJSON("POST", url, map[string]interface{}{"operations": ops}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Small batch: 95% success, P95 < 1 000 ms
	assertLoadResult(t, "Batch/Execute-5ops 200-concurrent 30s", result, 95.0, 1000)
}

// TestLoad_Batch_Execute_Large fires 50 concurrent large-batch (50 ops) requests.
// This is the maximum batch size and exercises the worst-case DB transaction.
func TestLoad_Batch_Execute_Large(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "batchlg_"+uniqueSuffix())
	user, token := seedUser(t, db, "batchlg_"+uniqueSuffix())
	_ = user

	url := baseURL() + "/api/v1/batch/execute"

	result := RunLoad(context.Background(), 50, 30*time.Second, func() (int, error) {
		ops := make([]map[string]interface{}, 50)
		for i := range ops {
			ops[i] = map[string]interface{}{
				"type":         "transfer",
				"asset_id":     asset.ID,
				"from_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
				"to_address":   fmt.Sprintf("G%s", randHex(55)),
				"amount":       1,
			}
		}
		resp, err := doJSON("POST", url, map[string]interface{}{"operations": ops}, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Large batch: 90% success (some timeouts expected), P95 < 5 000 ms
	assertLoadResult(t, "Batch/Execute-50ops 50-concurrent 30s", result, 90.0, 5000)
}

// ---------------------------------------------------------------------------
// GET /api/v1/batch/:id  — batch status
// ---------------------------------------------------------------------------

// TestLoad_Batch_GetStatus fires 500 concurrent batch status reads for 30 s.
func TestLoad_Batch_GetStatus(t *testing.T) {
	db := openDB(t)
	asset := seedAsset(t, db, "batchstat_"+uniqueSuffix())
	user, token := seedUser(t, db, "batchstat_"+uniqueSuffix())
	_ = user

	// Pre-create a batch to read.
	createURL := baseURL() + "/api/v1/batch/execute"
	createResp, err := doJSON("POST", createURL, map[string]interface{}{
		"operations": []map[string]interface{}{
			{
				"type":         "transfer",
				"asset_id":     asset.ID,
				"from_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
				"to_address":   fmt.Sprintf("G%s", randHex(55)),
				"amount":       1,
			},
		},
	}, token)
	if err != nil || createResp.StatusCode != http.StatusCreated {
		t.Skipf("could not create batch: status %v", createResp.StatusCode)
	}
	var batchBody struct {
		BatchID uint `json:"batch_id"`
	}
	if err := decodeJSON(createResp, &batchBody); err != nil || batchBody.BatchID == 0 {
		t.Skip("could not parse batch ID")
	}

	url := fmt.Sprintf("%s/api/v1/batch/%d", baseURL(), batchBody.BatchID)

	result := RunLoad(context.Background(), 500, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Batch/GetStatus 500-concurrent 30s", result, 99.0, 200)
}

// ---------------------------------------------------------------------------
// GET /api/v1/batches  — list batches
// ---------------------------------------------------------------------------

// TestLoad_Batch_List fires 400 concurrent batch list reads for 30 s.
func TestLoad_Batch_List(t *testing.T) {
	db := openDB(t)
	_, token := seedUser(t, db, "batchlist_"+uniqueSuffix())

	url := baseURL() + "/api/v1/batches?page=1&limit=10"

	result := RunLoad(context.Background(), 400, 30*time.Second, func() (int, error) {
		resp, err := doJSON("GET", url, nil, token)
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	assertLoadResult(t, "Batch/List 400-concurrent 30s", result, 99.0, 300)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkBatchExecute_5ops measures single-goroutine 5-op batch throughput.
func BenchmarkBatchExecute_5ops(b *testing.B) {
	db := openDB(b)
	asset := seedAsset(b, db, "bbatch5_"+uniqueSuffix())
	user, token := seedUser(b, db, "bbatch5_"+uniqueSuffix())
	_ = user
	url := baseURL() + "/api/v1/batch/execute"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := make([]map[string]interface{}, 5)
		for j := range ops {
			ops[j] = map[string]interface{}{
				"type":         "transfer",
				"asset_id":     asset.ID,
				"from_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
				"to_address":   fmt.Sprintf("G%s", randHex(55)),
				"amount":       1,
			}
		}
		resp, err := doJSON("POST", url, map[string]interface{}{"operations": ops}, token)
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}

// BenchmarkBatchExecute_50ops measures single-goroutine 50-op batch throughput.
func BenchmarkBatchExecute_50ops(b *testing.B) {
	db := openDB(b)
	asset := seedAsset(b, db, "bbatch50_"+uniqueSuffix())
	user, token := seedUser(b, db, "bbatch50_"+uniqueSuffix())
	_ = user
	url := baseURL() + "/api/v1/batch/execute"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := make([]map[string]interface{}, 50)
		for j := range ops {
			ops[j] = map[string]interface{}{
				"type":         "transfer",
				"asset_id":     asset.ID,
				"from_address": "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
				"to_address":   fmt.Sprintf("G%s", randHex(55)),
				"amount":       1,
			}
		}
		resp, err := doJSON("POST", url, map[string]interface{}{"operations": ops}, token)
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}
