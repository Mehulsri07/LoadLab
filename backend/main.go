package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var startTime = time.Now()

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

var probeClient = &http.Client{Timeout: 3 * time.Second}

// Service health status — pings Prometheus and Grafana from inside Docker network
func statusHandler(w http.ResponseWriter, r *http.Request) {
	type result struct {
		Status string `json:"status"`
		Ms     int    `json:"ms"`
	}

	probe := func(url string) result {
		t0 := time.Now()
		resp, err := probeClient.Get(url)
		ms := int(time.Since(t0).Milliseconds())
		if err != nil {
			return result{Status: "offline", Ms: ms}
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return result{Status: "offline", Ms: ms}
		}
		return result{Status: "online", Ms: ms}
	}

	writeJSON(w, map[string]any{
		"prometheus": probe("http://prometheus:9090/-/healthy"),
		"grafana":    probe("http://grafana:3000/api/health"),
	})
}

// --- Start ---

func main() {
	m := NewMetrics(prometheus.DefaultRegisterer, LoadConfig(os.Getenv, log.Printf).Version) // temporary wiring until Task 4
	instrumentHandler := m.Instrument
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
