package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	circuitState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Current state of the circuit breaker (0: Closed, 1: Half-Open, 2: Open)",
		},
		[]string{"path"},
	)

	circuitFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "circuit_breaker_failures_total",
			Help: "Total number of failures tracked by circuit breaker",
		},
		[]string{"path"},
	)
)

type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

type CircuitBreaker struct {
	mu               sync.RWMutex
	state            State
	failureCount     int
	lastFailure      time.Time
	failureThreshold int
	resetTimeout     time.Duration
	halfOpenTimeout  time.Duration
}

func NewCircuitBreaker(failureThreshold int, resetTimeout, halfOpenTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		halfOpenTimeout:  halfOpenTimeout,
	}
}

func (cb *CircuitBreaker) setState(state State, path string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = state
	circuitState.WithLabelValues(path).Set(float64(state))
}

func (cb *CircuitBreaker) recordFailure(path string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()
	circuitFailures.WithLabelValues(path).Inc()

	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		circuitState.WithLabelValues(path).Set(float64(StateOpen))
	}
}

func (cb *CircuitBreaker) resetFailureCount() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
}

func (cb *CircuitBreaker) shouldAllow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

func CircuitBreakerMiddleware(failureThreshold int, resetTimeout, halfOpenTimeout time.Duration) func(http.Handler) http.Handler {
	cb := NewCircuitBreaker(failureThreshold, resetTimeout, halfOpenTimeout)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if !cb.shouldAllow() {
				http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
				return
			}

			wrapped := wrapResponseWriter(w)
			next.ServeHTTP(wrapped, r)

			if wrapped.Status() >= 500 {
				cb.recordFailure(path)
			} else if cb.state == StateHalfOpen {
				cb.setState(StateClosed, path)
				cb.resetFailureCount()
			}

			if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
				cb.setState(StateHalfOpen, path)
			}
		})
	}
}
