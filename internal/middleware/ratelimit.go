package middleware

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	tokens   int
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int
	burst    int
	interval time.Duration
}

func NewRateLimiter(rate, burst int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
		interval: interval,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(10 * time.Minute)
		rl.mu.Lock()
		for k, v := range rl.visitors {
			if time.Since(v.lastSeen) > 30*time.Minute {
				delete(rl.visitors, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	now := time.Now()

	if !exists {
		rl.visitors[key] = &visitor{tokens: rl.burst - 1, lastSeen: now}
		return true
	}

	elapsed := now.Sub(v.lastSeen)
	v.lastSeen = now
	v.tokens += int(elapsed / rl.interval)
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}

	if v.tokens <= 0 {
		return false
	}

	v.tokens--
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			key = apiKey
		}

		if !rl.allow(key) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
