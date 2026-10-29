package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := LoadConfig(os.Getenv, log.Printf)
	m := NewMetrics(prometheus.DefaultRegisterer, cfg.Version) // temporary wiring until Task 4
	var health Health
	mux := http.NewServeMux()

	mux.HandleFunc("/health", m.Instrument("/health", healthHandler))
	mux.HandleFunc("/livez", health.Live)
	mux.HandleFunc("/readyz", health.Ready)
	mux.HandleFunc("/work", m.Instrument("/work", Work(cfg)))
	mux.HandleFunc("/slow", m.Instrument("/slow", Slow(3*time.Second)))
	mux.HandleFunc("/cpu", m.Instrument("/cpu", CPU(1_000_000)))
	mux.HandleFunc("/memory", m.Instrument("/memory", Memory))
	mux.HandleFunc("/info", m.Instrument("/info", Info(cfg, time.Now())))
	mux.HandleFunc("/status", m.Instrument("/status", Status(cfg, &http.Client{Timeout: 3 * time.Second})))
	mux.Handle("/metrics", promhttp.Handler())

	health.SetReady(true)
	fmt.Println("LoadLab backend running on port 3000")
	if err := http.ListenAndServe(":3000", mux); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
