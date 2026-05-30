package performance

// auth_load_test.go — load and benchmark tests for authentication endpoints.
//
// Expected baseline (single server, 4 vCPU):
//   POST /api/v1/auth/login   – 50 concurrent users, 95th-pct < 2 000 ms
//   POST /api/v1/auth/refresh – 200 concurrent users, 95th-pct < 200 ms
//
// 10x load targets:
//   Login:   500 concurrent users
//   Refresh: 2 000 concurrent users

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/kor-assetforge/models"
)

// ---------------------------------------------------------------------------
// Login load test
// ---------------------------------------------------------------------------

// TestLoad_Auth_Login fires 500 concurrent login requests for 30 s.
// Login is the most CPU-intensive endpoint (bcrypt cost 12 in production).
// We use bcrypt cost 4 for seeded test users so the DB is not the bottleneck.
func TestLoad_Auth_Login(t *testing.T) {
	db := openDB(t)

	// Seed a pool of 20 users to spread DB reads.
	const poolSize = 20
	type cred struct{ email, password string }
	pool := make([]cred, poolSize)
	for i := 0; i < poolSize; i++ {
		suffix := fmt.Sprintf("login%d_%s", i, uniqueSuffix())
		hashed, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), 4)
		addr := fmt.Sprintf("G%s", randHex(55))
		user := models.User{
			StellarAddress: addr,
			Email:          fmt.Sprintf("perf_login_%s@example.com", suffix),
			Username:       fmt.Sprintf("perf_login_%s", suffix),
			PasswordHash:   string(hashed),
			Role:           models.RoleUser,
			EmailVerified:  true,
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("seed login user %d: %v", i, err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(&user) })
		pool[i] = cred{email: user.Email, password: "Password123!"}
	}

	url := baseURL() + "/api/v1/auth/login"
	var idx int64

	result := RunLoad(context.Background(), 500, 30*time.Second, func() (int, error) {
		i := int(atomic.AddInt64(&idx, 1))
		c := pool[i%poolSize]
		resp, err := doJSON("POST", url, map[string]string{
			"email":    c.email,
			"password": c.password,
		}, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// 95% success rate; P95 < 2 000 ms (bcrypt is expensive)
	assertLoadResult(t, "Auth/Login 500-concurrent 30s", result, 95.0, 2000)
}

// ---------------------------------------------------------------------------
// Token refresh load test
// ---------------------------------------------------------------------------

// TestLoad_Auth_Refresh fires 2 000 concurrent refresh requests for 30 s.
// Refresh is cheap (JWT verify + sign, no bcrypt) so we expect low latency.
func TestLoad_Auth_Refresh(t *testing.T) {
	db := openDB(t)

	// Seed users and collect their refresh tokens via a real login.
	const poolSize = 50
	tokens := make([]string, poolSize)
	for i := 0; i < poolSize; i++ {
		suffix := fmt.Sprintf("refresh%d_%s", i, uniqueSuffix())
		_, accessToken := seedUser(t, db, suffix)
		// We use the access token as a stand-in; the refresh endpoint accepts
		// any valid JWT signed with the same secret.
		tokens[i] = accessToken
	}

	url := baseURL() + "/api/v1/auth/refresh"
	var idx int64

	result := RunLoad(context.Background(), 2000, 30*time.Second, func() (int, error) {
		i := int(atomic.AddInt64(&idx, 1))
		tok := tokens[i%poolSize]
		resp, err := doJSON("POST", url, map[string]string{"refresh_token": tok}, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// 95% success rate; P95 < 200 ms
	assertLoadResult(t, "Auth/Refresh 2000-concurrent 30s", result, 95.0, 200)
}

// ---------------------------------------------------------------------------
// Registration load test
// ---------------------------------------------------------------------------

// TestLoad_Auth_Register fires 100 concurrent registration requests for 20 s.
// Each request is unique to avoid 409 conflicts.
func TestLoad_Auth_Register(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() {
		db.Exec("DELETE FROM users WHERE username LIKE 'perf_reg_%'")
	})

	url := baseURL() + "/api/v1/auth/register"
	var counter int64

	result := RunLoad(context.Background(), 100, 20*time.Second, func() (int, error) {
		n := atomic.AddInt64(&counter, 1)
		addr := fmt.Sprintf("G%s", randHex(55))
		resp, err := doJSON("POST", url, map[string]string{
			"stellar_address": addr,
			"email":           fmt.Sprintf("perf_reg_%d_%d@example.com", n, time.Now().UnixNano()),
			"username":        fmt.Sprintf("perf_reg_%d_%d", n, time.Now().UnixNano()%1e9),
			"password":        "Password123!",
		}, "")
		if err != nil {
			return 0, err
		}
		drainClose(resp)
		return resp.StatusCode, nil
	})

	// Registration involves bcrypt + DB write; 95% success, P95 < 3 000 ms
	assertLoadResult(t, "Auth/Register 100-concurrent 20s", result, 95.0, 3000)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkLogin measures single-goroutine login throughput.
func BenchmarkLogin(b *testing.B) {
	db := openDB(b)
	suffix := "bench_login_" + uniqueSuffix()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), 4)
	addr := fmt.Sprintf("G%s", randHex(55))
	user := models.User{
		StellarAddress: addr,
		Email:          fmt.Sprintf("bench_login_%s@example.com", suffix),
		Username:       fmt.Sprintf("bench_login_%s", suffix),
		PasswordHash:   string(hashed),
		Role:           models.RoleUser,
		EmailVerified:  true,
	}
	if err := db.Create(&user).Error; err != nil {
		b.Fatalf("seed: %v", err)
	}
	b.Cleanup(func() { db.Unscoped().Delete(&user) })

	url := baseURL() + "/api/v1/auth/login"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("POST", url, map[string]string{
			"email":    user.Email,
			"password": "Password123!",
		}, "")
		if err != nil {
			b.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status %d", resp.StatusCode)
		}
		drainClose(resp)
	}
}

// BenchmarkRefreshToken measures token refresh throughput.
func BenchmarkRefreshToken(b *testing.B) {
	db := openDB(b)
	suffix := "bench_refresh_" + uniqueSuffix()
	user, token := seedUser(b, db, suffix)
	_ = user

	url := baseURL() + "/api/v1/auth/refresh"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := doJSON("POST", url, map[string]string{"refresh_token": token}, "")
		if err != nil {
			b.Fatal(err)
		}
		drainClose(resp)
	}
}
