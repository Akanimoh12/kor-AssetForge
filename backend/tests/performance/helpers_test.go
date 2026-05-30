// Package performance contains load and performance tests for the kor-AssetForge API.
//
// These tests exercise the system at 10x expected load and are designed to run
// against a live server (or an in-process test server) with a real database.
//
// # Running
//
//	# Start the server first, then:
//	go test ./tests/performance/... -v -timeout 10m
//
//	# Against a custom target:
//	PERF_BASE_URL=http://staging:8080 go test ./tests/performance/... -v -timeout 10m
//
//	# Run only benchmarks (no load tests):
//	go test ./tests/performance/... -bench=. -benchtime=10s -run=^$
//
// # Environment variables
//
//	PERF_BASE_URL   – base URL of the server under test (default: http://localhost:8080)
//	DATABASE_URL    – Postgres DSN used to seed test data
//	JWT_SECRET      – must match the server's JWT_SECRET (default: your-super-secret-jwt-key-change-in-production)
package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/yourusername/kor-assetforge/models"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

func baseURL() string {
	if u := os.Getenv("PERF_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "your-super-secret-jwt-key-change-in-production"
}

// ---------------------------------------------------------------------------
// Database helpers
// ---------------------------------------------------------------------------

func openDB(t testing.TB) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=password dbname=assetforge port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("performance test database not available: %v", err)
	}
	return db
}

// seedUser creates a verified user in the DB and returns a signed JWT for it.
func seedUser(t testing.TB, db *gorm.DB, suffix string) (models.User, string) {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte("Password123!"), 4) // cost 4 for speed
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	addr := fmt.Sprintf("G%s%s", suffix, randHex(46-len(suffix)))
	if len(addr) > 56 {
		addr = addr[:56]
	}
	user := models.User{
		StellarAddress: addr,
		Email:          fmt.Sprintf("perf_%s@example.com", suffix),
		Username:       fmt.Sprintf("perf_%s", suffix),
		PasswordHash:   string(hashed),
		Role:           models.RoleUser,
		EmailVerified:  true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&user) })
	return user, mintToken(t, user)
}

// seedAsset creates an asset in the DB and returns it.
func seedAsset(t testing.TB, db *gorm.DB, suffix string) models.Asset {
	t.Helper()
	sym := fmt.Sprintf("P%s", suffix)
	if len(sym) > 12 {
		sym = sym[:12]
	}
	asset := models.Asset{
		Name:         fmt.Sprintf("Perf Asset %s", suffix),
		Symbol:       sym,
		Description:  "Performance test asset",
		AssetType:    "real_estate",
		TotalSupply:  1_000_000,
		Fractions:    1_000_000,
		OwnerAddress: "GAHJJJKMOKYE4RVPZEWZTKH5FVI4PA3VL7GK2LFNUBSGBV3QLBDNLQQ",
		Verified:     true,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&asset) })
	return asset
}

// mintToken creates a signed JWT access token for the given user.
func mintToken(t testing.TB, user models.User) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"email":    user.Email,
		"username": user.Username,
		"role":     string(user.Role),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
		"type":     "access",
		"jti":      uuid.New().String(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(jwtSecret()))
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return signed
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 500,
		IdleConnTimeout:     90 * time.Second,
	},
}

func doJSON(method, url string, body interface{}, token string) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return httpClient.Do(req)
}

func mustDo(t testing.TB, method, url string, body interface{}, token string) *http.Response {
	t.Helper()
	resp, err := doJSON(method, url, body, token)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func drainClose(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// ---------------------------------------------------------------------------
// Load runner
// ---------------------------------------------------------------------------

// LoadResult holds aggregate metrics from a load run.
type LoadResult struct {
	Total      int64
	Successes  int64
	Failures   int64
	TotalNs    int64 // sum of response times in nanoseconds
	MinNs      int64
	MaxNs      int64
	P50Ns      int64
	P95Ns      int64
	P99Ns      int64
	ErrorCodes map[int]int64
	mu         sync.Mutex
	latencies  []int64
}

func (r *LoadResult) record(statusCode int, latency time.Duration) {
	ns := latency.Nanoseconds()
	atomic.AddInt64(&r.Total, 1)
	atomic.AddInt64(&r.TotalNs, ns)
	if statusCode >= 200 && statusCode < 300 {
		atomic.AddInt64(&r.Successes, 1)
	} else {
		atomic.AddInt64(&r.Failures, 1)
	}
	r.mu.Lock()
	r.latencies = append(r.latencies, ns)
	if r.ErrorCodes == nil {
		r.ErrorCodes = make(map[int]int64)
	}
	if statusCode < 200 || statusCode >= 300 {
		r.ErrorCodes[statusCode]++
	}
	r.mu.Unlock()
}

func (r *LoadResult) compute() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.latencies) == 0 {
		return
	}
	// Simple insertion sort — acceptable for ≤ 100k samples
	lats := make([]int64, len(r.latencies))
	copy(lats, r.latencies)
	for i := 1; i < len(lats); i++ {
		key := lats[i]
		j := i - 1
		for j >= 0 && lats[j] > key {
			lats[j+1] = lats[j]
			j--
		}
		lats[j+1] = key
	}
	r.MinNs = lats[0]
	r.MaxNs = lats[len(lats)-1]
	r.P50Ns = lats[int(float64(len(lats))*0.50)]
	r.P95Ns = lats[int(float64(len(lats))*0.95)]
	r.P99Ns = lats[int(float64(len(lats))*0.99)]
}

func (r *LoadResult) AvgMs() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.TotalNs) / float64(r.Total) / 1e6
}

func (r *LoadResult) SuccessRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Successes) / float64(r.Total) * 100
}

// RunLoad fires `concurrency` goroutines each executing `fn` for `duration`.
// Returns aggregated metrics.
func RunLoad(ctx context.Context, concurrency int, duration time.Duration, fn func() (int, error)) *LoadResult {
	result := &LoadResult{}
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				start := time.Now()
				code, _ := fn()
				result.record(code, time.Since(start))
			}
		}()
	}
	wg.Wait()
	result.compute()
	return result
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

// assertLoadResult fails the test if the result does not meet the thresholds.
func assertLoadResult(t *testing.T, name string, r *LoadResult, minSuccessRate float64, maxP95Ms float64) {
	t.Helper()
	r.compute()
	t.Logf("=== %s ===", name)
	t.Logf("  Total requests : %d", r.Total)
	t.Logf("  Success rate   : %.1f%%", r.SuccessRate())
	t.Logf("  Avg latency    : %.1f ms", r.AvgMs())
	t.Logf("  P50 latency    : %.1f ms", float64(r.P50Ns)/1e6)
	t.Logf("  P95 latency    : %.1f ms", float64(r.P95Ns)/1e6)
	t.Logf("  P99 latency    : %.1f ms", float64(r.P99Ns)/1e6)
	t.Logf("  Min / Max      : %.1f ms / %.1f ms", float64(r.MinNs)/1e6, float64(r.MaxNs)/1e6)
	if len(r.ErrorCodes) > 0 {
		t.Logf("  Error codes    : %v", r.ErrorCodes)
	}

	if r.SuccessRate() < minSuccessRate {
		t.Errorf("%s: success rate %.1f%% < required %.1f%%", name, r.SuccessRate(), minSuccessRate)
	}
	if maxP95Ms > 0 && float64(r.P95Ns)/1e6 > maxP95Ms {
		t.Errorf("%s: P95 latency %.1f ms > threshold %.1f ms", name, float64(r.P95Ns)/1e6, maxP95Ms)
	}
}

// ---------------------------------------------------------------------------
// Misc utilities
// ---------------------------------------------------------------------------

func randHex(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
