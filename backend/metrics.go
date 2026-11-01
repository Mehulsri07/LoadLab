package main

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer, version string) *Metrics {
	f := promauto.With(reg)
	f.NewGaugeVec(prometheus.GaugeOpts{
		Name: "build_info",
		Help: "Build information; value is always 1",
	}, []string{"version"}).WithLabelValues(version).Set(1)
	return &Metrics{
		requests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"route", "status"}),
		duration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.075, 0.1, 0.15, 0.2, 0.25, 0.3, 0.4, 0.5, 0.75, 1, 2, 5},
		}, []string{"route"}),
		inFlight: f.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served",
		}),
	}
}

// Instrument records count, duration and in-flight gauge around h.
func (m *Metrics) Instrument(route string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Inc()
		defer m.inFlight.Dec()
		timer := prometheus.NewTimer(m.duration.WithLabelValues(route))
		defer timer.ObserveDuration()

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		h(rec, r)

		m.requests.WithLabelValues(route, strconv.Itoa(rec.statusCode)).Inc()
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
