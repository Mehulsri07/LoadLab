package main

import (
	"fmt"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	c := LoadConfig(func(string) string { return "" }, t.Logf)
	assertEq(t, c.WorkCPU, 20*time.Millisecond)
	assertEq(t, c.WorkWait, 30*time.Millisecond)
	assertEq(t, c.WorkTimeout, 5*time.Second)
	assertEq(t, c.FaultLatency, time.Duration(0))
	assertEq(t, c.FaultErrorRate, 0.0)
	assertEq(t, c.Version, "dev")
	assertEq(t, c.Drain, 20*time.Second)
	assertEq(t, c.StatusTargets["prometheus"], "http://prometheus:9090/-/healthy")
	assertEq(t, c.StatusTargets["grafana"], "http://grafana:3000/api/health")
}

func TestLoadConfig_ClampsAndFallsBack(t *testing.T) {
	env := map[string]string{"WORK_CPU_MS": "-5", "WORK_DURATION_MS": "abc", "FAULT_ERROR_RATE": "1.5", "FAULT_LATENCY_MS": "999999"}
	var logged []string
	c := LoadConfig(func(k string) string { return env[k] }, func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) })
	assertEq(t, c.WorkCPU, 20*time.Millisecond)  // negative -> default
	assertEq(t, c.WorkWait, 30*time.Millisecond) // unparsable -> default
	assertEq(t, c.FaultErrorRate, 1.0)           // clamped to [0,1]
	assertEq(t, c.FaultLatency, 10*time.Second)  // clamped to 10 s
	if len(logged) != 4 {
		t.Fatalf("want 4 warnings, got %v", logged)
	}
}

func TestLoadConfig_StatusTargetsFromEnv(t *testing.T) {
	c := LoadConfig(func(k string) string {
		if k == "STATUS_TARGETS" {
			return "prometheus=http://p:9090/-/healthy,keda=http://k:8080/healthz"
		}
		return ""
	}, t.Logf)
	assertEq(t, len(c.StatusTargets), 2)
	assertEq(t, c.StatusTargets["keda"], "http://k:8080/healthz")
}
