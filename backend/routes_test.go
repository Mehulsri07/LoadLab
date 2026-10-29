package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPublicMux_HasNoAdminRoutes(t *testing.T) {
	mux := PublicMux(Config{}, NewMetrics(prometheus.NewRegistry(), "t"), &Health{}, time.Now())
	assertEq(t, post(mux, "/admin/leak?mb_per_min=10").Code, 404)
	for _, p := range []string{"/work", "/slow", "/cpu", "/memory", "/info", "/status", "/health", "/livez", "/readyz", "/metrics"} {
		if serveCancelled(mux, p).Code == 404 {
			t.Errorf("%s missing", p)
		}
	}
}
