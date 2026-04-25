package middleware

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter implements a per-client-IP token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter. If rps <= 0, rate limiting is disabled.
func NewRateLimiter(rps float64) *RateLimiter {
	burst := int(rps)
	if burst < 1 {
		burst = 1
	}

	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter returns the rate limiter for a given IP, creating one if needed.
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limiter, exists := rl.limiters[ip]; exists {
		return limiter
	}

	limiter := rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[ip] = limiter
	return limiter
}

// Limit returns middleware that enforces per-IP rate limiting.
// If rateLimit <= 0, the middleware is a no-op pass-through.
func Limit(rateLimit float64) func(http.Handler) http.Handler {
	if rateLimit <= 0 {
		return func(next http.Handler) http.Handler {
			return next // disabled
		}
	}

	rl := NewRateLimiter(rateLimit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			limiter := rl.getLimiter(ip)
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded, try again later"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
