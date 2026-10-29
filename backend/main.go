package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	cfg := LoadConfig(os.Getenv, log.Printf)
	m := NewMetrics(prometheus.DefaultRegisterer, cfg.Version) // temporary wiring until Task 4
	var health Health

	go func() { // internal-only admin listener; not exposed by the Service
		if err := http.ListenAndServe(":9091", AdminMux(NewLeaker(time.Tick(time.Second)))); err != nil {
			fmt.Printf("Admin server failed: %v\n", err)
		}
	}()

	health.SetReady(true)
	fmt.Println("LoadLab backend running on port 3000")
	if err := http.ListenAndServe(":3000", PublicMux(cfg, m, &health, time.Now())); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
