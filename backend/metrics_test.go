package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_BucketsMatchSLO(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, "v1")
	m.Instrument("/work", func(w http.ResponseWriter, r *http.Request) {})(httptest.NewRecorder(), httptest.NewRequest("GET", "/work", nil))
	want := []float64{0.01, 0.025, 0.05, 0.075, 0.1, 0.15, 0.2, 0.25, 0.3, 0.4, 0.5, 0.75, 1, 2, 5}
	assertBuckets(t, reg, "http_request_duration_seconds", want)
}

func TestMetrics_InstrumentCountsStatusAndInFlight(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, "v1")
	seen := -1.0
	h := m.Instrument("/work", func(w http.ResponseWriter, r *http.Request) {
		seen = testutil.ToFloat64(m.inFlight)
		w.WriteHeader(http.StatusTeapot)
	})
	h(httptest.NewRecorder(), httptest.NewRequest("GET", "/work", nil))
	assertEq(t, seen, 1.0)                           // gauge raised during the request
	assertEq(t, testutil.ToFloat64(m.inFlight), 0.0) // and lowered after
	assertEq(t, testutil.ToFloat64(m.requests.WithLabelValues("/work", "418")), 1.0)
	assertEq(t, gaugeValue(t, reg, "build_info", map[string]string{"version": "v1"}), 1.0)
}
