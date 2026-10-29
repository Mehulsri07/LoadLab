package main

import (
	"testing"
	"time"
)

func TestLeaker_GrowsPerTickAndStopReleases(t *testing.T) {
	tick := make(chan time.Time)
	l := NewLeaker(tick)
	l.Start(60) // 60 MB/min = 1 MB per 1 s tick
	for i := 0; i < 3; i++ {
		tick <- time.Now()
	}
	waitFor(t, func() bool { return l.HeldMB() == 3 })
	l.Stop()
	assertEq(t, l.HeldMB(), 0)
}

func TestAdminLeak_ClampsAndValidates(t *testing.T) {
	l := NewLeaker(make(chan time.Time))
	mux := AdminMux(l)
	assertEq(t, post(mux, "/admin/leak?mb_per_min=100000").Code, 202)
	assertEq(t, l.rate(), 200) // clamped
	assertEq(t, post(mux, "/admin/leak").Code, 400)
	assertEq(t, post(mux, "/admin/leak?mb_per_min=-3").Code, 400)
	assertEq(t, get(mux, "/admin/leak").Code, 405)
	assertEq(t, post(mux, "/admin/leak/stop").Code, 202)
}

func TestFaultMiddleware_OffByDefault(t *testing.T) {
	h := FaultMiddleware(Config{}, func() float64 { return 0 }, ok200())
	t0 := time.Now()
	rec := serve(h, "/work")
	assertEq(t, rec.Code, 200)
	if time.Since(t0) > 20*time.Millisecond {
		t.Fatal("added latency with faults off")
	}
}

func TestFaultMiddleware_AddsLatencyAndErrors(t *testing.T) {
	cfg := Config{FaultLatency: 50 * time.Millisecond, FaultErrorRate: 0.1}
	t0 := time.Now()
	assertEq(t, serve(FaultMiddleware(cfg, func() float64 { return 0.05 }, ok200()), "/work").Code, 500) // 0.05 < 0.1 → error
	if time.Since(t0) < 50*time.Millisecond {
		t.Fatal("latency not applied")
	}
	assertEq(t, serve(FaultMiddleware(cfg, func() float64 { return 0.5 }, ok200()), "/work").Code, 200)
}

func TestFaultMiddleware_SparesProbesAndMetrics(t *testing.T) {
	cfg := Config{FaultErrorRate: 1}
	for _, p := range []string{"/livez", "/readyz", "/metrics"} {
		assertEq(t, serve(FaultMiddleware(cfg, func() float64 { return 0 }, ok200()), p).Code, 200)
	}
}
