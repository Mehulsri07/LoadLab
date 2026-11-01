package main

import (
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PublicMux serves the app routes on the public port. It has no admin routes.
func PublicMux(cfg Config, m *Metrics, h *Health, start time.Time) http.Handler {
	routes := map[string]http.HandlerFunc{
		"/work":    Work(cfg),
		"/slow":    Slow(3 * time.Second),
		"/cpu":     CPU(1_000_000),
		"/memory":  Memory,
		"/info":    Info(cfg, start),
		"/status":  Status(cfg, &http.Client{Timeout: 3 * time.Second}),
		"/health":  healthHandler, // Compose frontend; body kept as before
		"/livez":   h.Live,
		"/readyz":  h.Ready,
		"/metrics": promhttp.Handler().ServeHTTP,
	}
	mux := http.NewServeMux()
	for route, fn := range routes {
		mux.HandleFunc(route, m.Instrument(route, FaultMiddleware(cfg, rand.Float64, fn).ServeHTTP))
	}
	return mux
}

// AdminMux is the internal-only listener: the memory-leak switch.
func AdminMux(l *Leaker) http.Handler {
	pod, _ := os.Hostname()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/leak", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.URL.Query().Get("mb_per_min"))
		if err != nil || n < 1 {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "mb_per_min must be an integer >= 1"})
			return
		}
		n = min(n, 200)
		l.Start(n)
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]any{"mb_per_min": n, "pod": pod})
	})
	mux.HandleFunc("POST /admin/leak/stop", func(w http.ResponseWriter, r *http.Request) {
		l.Stop()
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]string{"status": "stopped", "pod": pod})
	})
	return mux
}
