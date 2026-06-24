package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var startTime = time.Now()

// --- Metrics ---

var requestCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	},
	[]string{"route", "status"},
)

var requestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: []float64{0.05, 0.1, 0.3, 0.5, 1, 2, 5, 10},
	},
	[]string{"route"},
)

func init() {
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(requestDuration)
}

// --- Middleware: track duration for all routes ---

func instrumentHandler(route string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(requestDuration.WithLabelValues(route))
		defer timer.ObserveDuration()

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		handler(rec, r)

		requestCounter.WithLabelValues(route, strconv.Itoa(rec.statusCode)).Inc()
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Routes ---

// Basic health check — fast, always succeeds
func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// Simulates a slow upstream dependency (3s delay)
func slowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(3 * time.Second)
	writeJSON(w, map[string]any{
		"endpoint": "slow",
		"delay_ms": 3000,
	})
}

// CPU-intensive: counts primes up to 1,000,000
func cpuHandler(w http.ResponseWriter, r *http.Request) {
	count := 0
	for i := 2; i < 1_000_000; i++ {
		prime := true
		limit := int(math.Sqrt(float64(i)))
		for j := 2; j <= limit; j++ {
			if i%j == 0 {
				prime = false
				break
			}
		}
		if prime {
			count++
		}
	}
	writeJSON(w, map[string]any{
		"endpoint":     "cpu",
		"primes_found": count,
	})
}

// Memory-intensive: allocates a large slice
func memoryHandler(w http.ResponseWriter, r *http.Request) {
	type obj struct {
		Index int     `json:"index"`
		Value float64 `json:"value"`
	}
	arr := make([]obj, 0, 1_000_000)
	for i := 0; i < 1_000_000; i++ {
		arr = append(arr, obj{Index: i, Value: float64(i) * 0.1})
	}
	writeJSON(w, map[string]any{
		"endpoint":  "memory",
		"allocated": len(arr),
	})
}

// Server info
func infoHandler(w http.ResponseWriter, r *http.Request) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	writeJSON(w, map[string]any{
		"app":            "LoadLab",
		"go":             runtime.Version(),
		"uptime_seconds": int(time.Since(startTime).Seconds()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"environment":    env,
	})
}

// Service health status — pings Prometheus and Grafana from inside Docker network
func statusHandler(w http.ResponseWriter, r *http.Request) {
	type result struct {
		Status string `json:"status"`
		Ms     int    `json:"ms"`
	}

	probe := func(url string) result {
		t0 := time.Now()
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(url)
		ms := int(time.Since(t0).Milliseconds())
		if err != nil || resp.StatusCode >= 500 {
			return result{Status: "offline", Ms: ms}
		}
		return result{Status: "online", Ms: ms}
	}

	promResult := probe("http://prometheus:9090/-/healthy")
	grafResult := probe("http://grafana:3000/api/health")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"prometheus": promResult,
		"grafana":    grafResult,
	})
}

// --- Start ---

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", instrumentHandler("/health", healthHandler))
	mux.HandleFunc("/slow", instrumentHandler("/slow", slowHandler))
	mux.HandleFunc("/cpu", instrumentHandler("/cpu", cpuHandler))
	mux.HandleFunc("/memory", instrumentHandler("/memory", memoryHandler))
	mux.HandleFunc("/info", instrumentHandler("/info", infoHandler))
	mux.HandleFunc("/status", instrumentHandler("/status", statusHandler))

	// Prometheus metrics scrape endpoint
	mux.Handle("/metrics", promhttp.Handler())

	fmt.Println("LoadLab backend running on port 3000")
	if err := http.ListenAndServe(":3000", mux); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
