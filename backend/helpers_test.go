package main

import (
	"slices"
	"testing"

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
