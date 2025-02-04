package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ratelimitTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimit_requests_total",
			Help: "Total number of requests handled by rate limiter",
		},
		[]string{"status"},
	)

	ratelimitCurrentRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ratelimit_current_requests",
			Help: "Current number of requests in the window",
		},
	)
)

type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	mu       sync.RWMutex
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) cleanup(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if times, exists := rl.requests[ip]; exists {
		now := time.Now()
		cutoff := 0
		for i, t := range times {
			if now.Sub(t) <= rl.window {
				cutoff = i
				break
			}
		}
		if cutoff > 0 {
			rl.requests[ip] = times[cutoff:]
		}
	}
}

func (rl *RateLimiter) isAllowed(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.requests[ip]

	validTimes := []time.Time{}
	for _, t := range times {
		if now.Sub(t) <= rl.window {
			validTimes = append(validTimes, t)
		}
	}

	rl.requests[ip] = validTimes
	ratelimitCurrentRequests.Set(float64(len(validTimes)))

	if len(validTimes) >= rl.limit {
		ratelimitTotal.WithLabelValues("blocked").Inc()
		return false
	}

	rl.requests[ip] = append(validTimes, now)
	ratelimitTotal.WithLabelValues("allowed").Inc()
	return true
}

func (rl *RateLimiter) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		rl.cleanup(ip)

		if !rl.isAllowed(ip) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RateLimitMiddleware(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(requestsPerMinute, time.Minute)
	return limiter.RateLimit
}
