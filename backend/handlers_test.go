package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestWork_SpendsCPUThenWaits(t *testing.T) {
	cfg := Config{WorkCPU: 30 * time.Millisecond, WorkWait: 20 * time.Millisecond, WorkTimeout: time.Second}
	rec := httptest.NewRecorder()
	t0 := time.Now()
	Work(cfg)(rec, httptest.NewRequest("GET", "/work", nil))
	assertEq(t, rec.Code, 200)
	if d := time.Since(t0); d < 45*time.Millisecond || d > 250*time.Millisecond { // calibrated CPU cost jitters a few %
		t.Fatalf("took %v", d)
	}
}

func TestWork_TimeoutReturns504(t *testing.T) {
	cfg := Config{WorkCPU: 0, WorkWait: 500 * time.Millisecond, WorkTimeout: 50 * time.Millisecond}
	rec := httptest.NewRecorder()
	Work(cfg)(rec, httptest.NewRequest("GET", "/work", nil))
	assertEq(t, rec.Code, http.StatusGatewayTimeout)
}

func TestBurnCPU_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	t0 := time.Now()
	err := burnCPU(ctx, 5*time.Second)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(t0) > 100*time.Millisecond {
		t.Fatalf("err=%v after %v", err, time.Since(t0))
	}
}

func TestBurnCPU_FixedWorkUnderContention(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	var wg sync.WaitGroup
	t0 := time.Now()
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := burnCPU(context.Background(), 30*time.Millisecond); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// Two 30ms CPU jobs sharing one P need ~60ms of wall time.
	if d := time.Since(t0); d < 50*time.Millisecond {
		t.Fatalf("two 30ms burns on one P finished in %v; want >= 50ms", d)
	}
}

func TestSlowAndCPU_StopWhenClientGoes(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{"slow": Slow(3 * time.Second), "cpu": CPU(1 << 40)} {
		ctx, cancel := context.WithCancel(context.Background())
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		t0 := time.Now()
		h(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx))
		if time.Since(t0) > 200*time.Millisecond {
			t.Fatalf("%s kept running %v after cancel", name, time.Since(t0))
		}
	}
}

func TestReady_FollowsLifecycle(t *testing.T) {
	var h Health
	assertEq(t, code(h.Ready), 503)
	h.SetReady(true)
	assertEq(t, code(h.Ready), 200)
	h.SetDraining()
	assertEq(t, code(h.Ready), 503)
	assertEq(t, code(h.Live), 200)
}

func TestStatus_UsesConfiguredTargets(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer up.Close()
	cfg := Config{StatusTargets: map[string]string{"prometheus": up.URL, "ghost": "http://127.0.0.1:1"}}
	rec := httptest.NewRecorder()
	Status(cfg, &http.Client{Timeout: time.Second})(rec, httptest.NewRequest("GET", "/status", nil))
	var got map[string]struct{ Status string }
	_ = json.NewDecoder(rec.Body).Decode(&got)
	assertEq(t, got["prometheus"].Status, "online")
	assertEq(t, got["ghost"].Status, "offline")
}
