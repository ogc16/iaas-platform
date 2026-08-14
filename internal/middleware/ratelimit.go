package middleware

import (
	"fmt"
	"net/http"
	"strconv"
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
	now      func() time.Time
}

func NewRateLimiter(rate, burst int, interval time.Duration) *RateLimiter {
	return newRateLimiter(rate, burst, interval, time.Now)
}

func newRateLimiter(rate, burst int, interval time.Duration, now func() time.Time) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
		interval: interval,
		now:      now,
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

// allow checks whether a request is permitted and returns the number of
// remaining tokens after consumption. When denied it returns 0 remaining
// and the number of seconds until the next token is available.
func (rl *RateLimiter) allow(key string) (allowed bool, remaining int, retryAfter int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	v, exists := rl.visitors[key]
	if !exists {
		rl.visitors[key] = &visitor{tokens: rl.burst - 1, lastSeen: now}
		return true, rl.burst - 1, 0
	}

	// Refill tokens at `rate` tokens per `interval`.
	elapsed := now.Sub(v.lastSeen)
	v.lastSeen = now
	v.tokens += int(elapsed/rl.interval) * rl.rate
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}

	if v.tokens <= 0 {
		// Approximate retry delay: one token arrives after one interval / rate.
		retry := int(rl.interval.Seconds()) / rl.rate
		if retry < 1 {
			retry = 1
		}
		return false, 0, retry
	}

	v.tokens--
	return true, v.tokens, 0
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			key = apiKey
		}

		allowed, remaining, retryAfter := rl.allow(key)

		// Emit standard rate-limit headers on every response so clients can
		// self-throttle before hitting the limit.
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.burst))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(retryAfter))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after":%d}`, retryAfter)
			return
		}

		next.ServeHTTP(w, r)
	})
}
