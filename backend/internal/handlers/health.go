package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	uptime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_uptime_seconds",
			Help: "The uptime of the application in seconds",
		},
	)

	goroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_goroutines",
			Help: "Number of goroutines currently running",
		},
	)
)

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	GoVersion string `json:"go_version"`
	Memory    struct {
		Alloc      uint64 `json:"alloc"`
		TotalAlloc uint64 `json:"total_alloc"`
		Sys        uint64 `json:"sys"`
		NumGC      uint32 `json:"num_gc"`
	} `json:"memory"`
}

var startTime = time.Now()

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	currentTime := time.Now()
	uptimeDuration := currentTime.Sub(startTime)

	uptime.Set(uptimeDuration.Seconds())
	goroutines.Set(float64(runtime.NumGoroutine()))

	response := HealthResponse{
		Status:    "ok",
		Timestamp: currentTime.UTC().Format(time.RFC3339),
		Version:   "1.0.0",
		Uptime:    uptimeDuration.String(),
		GoVersion: runtime.Version(),
	}

	response.Memory.Alloc = memStats.Alloc
	response.Memory.TotalAlloc = memStats.TotalAlloc
	response.Memory.Sys = memStats.Sys
	response.Memory.NumGC = memStats.NumGC

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
