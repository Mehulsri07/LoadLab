package main

import (
	"math"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WorkCPU, WorkWait, WorkTimeout time.Duration
	FaultLatency                   time.Duration
	FaultErrorRate                 float64
	Version                        string
	StatusTargets                  map[string]string
	Drain                          time.Duration
}

// LoadConfig reads settings via getenv. Unset values use the default silently;
// unparsable or below-range values fall back to the default and values above
// the range are clamped, each with exactly one warning through logf.
func LoadConfig(getenv func(string) string, logf func(string, ...any)) Config {
	num := func(key string, def, lo, hi float64) float64 {
		s := getenv(key)
		if s == "" {
			return def
		}
		v, err := strconv.ParseFloat(s, 64)
		switch {
		case err != nil || math.IsNaN(v) || v < lo:
			logf("config: %s=%q invalid, using default %v", key, s, def)
			return def
		case v > hi:
			logf("config: %s=%q above max, clamped to %v", key, s, hi)
			return hi
		}
		return v
	}
	ms := func(key string, def, lo, hi float64) time.Duration {
		return time.Duration(num(key, def, lo, hi) * float64(time.Millisecond))
	}

	c := Config{
		WorkCPU:        ms("WORK_CPU_MS", 20, 0, 1000),
		WorkWait:       ms("WORK_DURATION_MS", 30, 0, 10000),
		WorkTimeout:    ms("WORK_TIMEOUT_MS", 5000, 100, 30000),
		FaultLatency:   ms("FAULT_LATENCY_MS", 0, 0, 10000),
		FaultErrorRate: num("FAULT_ERROR_RATE", 0, 0, 1),
		Version:        getenv("APP_VERSION"),
		Drain:          time.Duration(num("SHUTDOWN_DRAIN_S", 20, 1, 25) * float64(time.Second)),
		StatusTargets: map[string]string{
			"prometheus": "http://prometheus:9090/-/healthy",
			"grafana":    "http://grafana:3000/api/health",
		},
	}
	if c.Version == "" {
		c.Version = buildVersion
	}
	if s := getenv("STATUS_TARGETS"); s != "" {
		c.StatusTargets = map[string]string{}
		for _, pair := range strings.Split(s, ",") {
			if name, url, ok := strings.Cut(pair, "="); ok && name != "" && url != "" {
				c.StatusTargets[strings.TrimSpace(name)] = strings.TrimSpace(url)
			} else {
				logf("config: STATUS_TARGETS entry %q ignored, want name=url", pair)
			}
		}
	}
	return c
}
