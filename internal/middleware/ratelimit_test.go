package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowsBurst(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, 3, time.Second, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if !rl.allow("ip-1") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
}

func TestRateLimiter_BlocksAfterBurst(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, 3, time.Second, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		rl.allow("ip-1")
	}
	if rl.allow("ip-1") {
		t.Fatal("expected 4th request to be blocked")
	}
}

func TestRateLimiter_RefillsAtConfiguredRate(t *testing.T) {
	now := time.Unix(0, 0)
	// rate=2 per interval: a 3s gap must add 6 tokens.
	rl := newRateLimiter(2, 10, time.Second, func() time.Time { return now })

	rl.allow("ip-1") // consume 1 (9 left)
	now = now.Add(3 * time.Second)

	// Refilled by 6 -> capped at 10, then 10 requests drain the bucket.
	allowed := 0
	for rl.allow("ip-1") {
		allowed++
	}
	if allowed != 10 {
		t.Fatalf("expected 10 requests to be allowed after refill, got %d", allowed)
	}
}

func TestRateLimiter_CapsAtBurst(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, 3, time.Second, func() time.Time { return now })

	rl.allow("ip-1") // consume 1, 2 left
	now = now.Add(10 * time.Second)
	rl.allow("ip-1") // refill caps at 3, consume 1 -> 2 left

	// Exactly 2 more should pass, the third must be blocked.
	if !rl.allow("ip-1") {
		t.Fatal("expected request to be allowed")
	}
	if !rl.allow("ip-1") {
		t.Fatal("expected second request to be allowed")
	}
	if rl.allow("ip-1") {
		t.Fatal("expected token bucket to cap at burst")
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, 1, time.Second, func() time.Time { return now })

	if !rl.allow("key-a") {
		t.Fatal("expected key-a to be allowed")
	}
	if rl.allow("key-a") {
		t.Fatal("expected key-a to be blocked after burst")
	}
	if !rl.allow("key-b") {
		t.Fatal("expected key-b to be allowed independently")
	}
}

func TestRateLimiter_MiddlewareResponds429(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, 1, time.Second, func() time.Time { return now })
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on first request, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second request, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429")
	}
}

func TestRateLimiter_MiddlewareKeysByAPIKey(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, 1, time.Second, func() time.Time { return now })
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "iaas_shared")
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-API-Key", "iaas_shared")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req2)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for shared api key, got %d", rec.Code)
	}
}
