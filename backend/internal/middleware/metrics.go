package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by status code, method, and path",
		},
		[]string{"status_code", "method", "path"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	scrapingOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scraping_operations_total",
			Help: "Total number of scraping operations by type and status",
		},
		[]string{"type", "status"},
	)

	scrapingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "scraping_duration_seconds",
			Help:    "Scraping operation duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		},
		[]string{"type"},
	)
)

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := wrapResponseWriter(w)

		var path string
		route := mux.CurrentRoute(r)
		if route != nil {
			path, _ = route.GetPathTemplate()
		} else {
			path = r.URL.Path
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(wrapped.Status())

		httpRequestsTotal.WithLabelValues(statusCode, r.Method, path).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

func RecordScraping(operationType string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
	}

	scrapingOperationsTotal.WithLabelValues(operationType, status).Inc()
	scrapingDuration.WithLabelValues(operationType).Observe(duration)
}

func RecordFighterScraping(start time.Time, err error) {
	RecordScraping("fighter", start, err)
}

func RecordEventScraping(start time.Time, err error) {
	RecordScraping("event", start, err)
}

func RecordRosterScraping(start time.Time, err error) {
	RecordScraping("roster", start, err)
}
