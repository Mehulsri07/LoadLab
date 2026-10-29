package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// buildVersion is set at link time with -X main.buildVersion=...; APP_VERSION overrides it.
var buildVersion = "dev"

func main() {
	cfg := LoadConfig(os.Getenv, log.Printf)
	var health Health
	public := NewPublicServer(":3000", PublicMux(cfg, NewMetrics(prometheus.DefaultRegisterer, cfg.Version), &health, time.Now()))
	admin := &http.Server{ // internal-only admin listener; not exposed by the Service
		Addr:              ":9091",
		Handler:           AdminMux(NewLeaker(time.Tick(time.Second))),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := Run(ctx, cfg, &health, public, admin, log.Printf)
	stop()
	if err != nil {
		log.Printf("server: %v", err)
		os.Exit(1)
	}
}
