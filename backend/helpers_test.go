package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func assertEq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func findMetric(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) *dto.Metric {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			ok := true
			for k, v := range labels {
				found := false
				for _, lp := range m.GetLabel() {
					found = found || (lp.GetName() == k && lp.GetValue() == v)
				}
				ok = ok && found
			}
			if ok {
				return m
			}
		}
	}
	t.Fatalf("metric %s %v not found", name, labels)
	return nil
}

func assertBuckets(t *testing.T, reg *prometheus.Registry, name string, want []float64) {
	t.Helper()
	var got []float64
	for _, b := range findMetric(t, reg, name, nil).GetHistogram().GetBucket() {
		got = append(got, b.GetUpperBound())
	}
	if !slices.Equal(got, want) {
		t.Errorf("buckets %v, want %v", got, want)
	}
}

func gaugeValue(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	return findMetric(t, reg, name, labels).GetGauge().GetValue()
}

func code(h http.HandlerFunc) int {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	return rec.Code
}

func serve(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// serveCancelled sends a request whose context is already cancelled, so slow handlers return at once.
func serveCancelled(h http.Handler, path string) *httptest.ResponseRecorder {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil).WithContext(ctx))
	return rec
}

func get(h http.Handler, path string) *httptest.ResponseRecorder { return serve(h, path) }

func post(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", path, nil))
	return rec
}

func ok200() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal("condition not met within 2s")
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

// waitListening waits until Run has marked h ready, then returns srv's base URL.
// Run stores the bound address into srv.Addr before the atomic SetReady, so the
// read below happens-after the write.
func waitListening(t *testing.T, h *Health, srv *http.Server) string {
	t.Helper()
	waitFor(t, func() bool { return code(h.Ready) == http.StatusOK })
	return "http://" + srv.Addr
}
