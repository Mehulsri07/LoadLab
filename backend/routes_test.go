package main

import (
	"encoding/json"
	"os"
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

func TestPublicMux_InjectedFaultsReachMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	mux := PublicMux(Config{FaultErrorRate: 1}, NewMetrics(reg, "t"), &Health{}, time.Now())
	assertEq(t, get(mux, "/info").Code, 500)
	assertEq(t, get(mux, "/livez").Code, 200)
	got := findMetric(t, reg, "http_requests_total", map[string]string{"route": "/info", "status": "500"})
	assertEq(t, got.GetCounter().GetValue(), 1.0)
}

func TestAdminMux_ResponsesNamePod(t *testing.T) {
	want, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	mux := AdminMux(NewLeaker(make(chan time.Time)))
	for _, p := range []string{"/admin/leak?mb_per_min=1", "/admin/leak/stop"} {
		rec := post(mux, p)
		assertEq(t, rec.Code, 202)
		var body struct {
			Pod string `json:"pod"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		assertEq(t, body.Pod, want)
	}
}
