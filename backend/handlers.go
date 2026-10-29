package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// Health tracks readiness for the Kubernetes probes.
type Health struct{ ready, draining atomic.Bool }

func (h *Health) SetReady(v bool) { h.ready.Store(v) }
func (h *Health) SetDraining()    { h.draining.Store(true) }

func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	switch {
	case h.draining.Load():
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]string{"status": "draining"})
	case !h.ready.Load():
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]string{"status": "not ready"})
	default:
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

var (
	cpuSink    atomic.Uint64 // keeps the compiler from eliminating mix; atomic because requests run concurrently
	calibrate  sync.Once
	itersPerNs float64
)

// mix runs n xorshift iterations: a fixed unit of CPU work.
func mix(n int64) {
	x := cpuSink.Load() | 1
	for range n {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	cpuSink.Store(x)
}

// calibrateCPU measures mix's speed once, for about 50ms.
// ponytail: calibrated on an idle core; if startup is contended, per-request cost is underestimated.
func calibrateCPU() {
	calibrate.Do(func() {
		var n int64
		t0 := time.Now()
		for time.Since(t0) < 50*time.Millisecond {
			mix(10_000)
			n += 10_000
		}
		itersPerNs = float64(n) / float64(time.Since(t0))
	})
}

// burnCPU does d worth of calibrated CPU work, checking for cancellation about every millisecond.
func burnCPU(ctx context.Context, d time.Duration) error {
	calibrateCPU()
	left := int64(float64(d) * itersPerNs)
	chunk := max(int64(itersPerNs*float64(time.Millisecond)), 1)
	for left > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		k := min(chunk, left)
		mix(k)
		left -= k
	}
	return nil
}

// Work costs real CPU then holds the request open, like a request awaiting a dependency.
func Work(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), cfg.WorkTimeout)
		defer cancel()
		err := burnCPU(ctx, cfg.WorkCPU)
		if err == nil {
			t := time.NewTimer(cfg.WorkWait)
			defer t.Stop()
			select {
			case <-t.C:
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			w.WriteHeader(http.StatusGatewayTimeout)
			writeJSON(w, map[string]string{"status": "timeout"})
			return
		}
		if err != nil { // client gone
			return
		}
		writeJSON(w, map[string]any{
			"status":  "ok",
			"cpu_ms":  cfg.WorkCPU.Milliseconds(),
			"wait_ms": cfg.WorkWait.Milliseconds(),
		})
	}
}

// Slow simulates a slow upstream dependency.
func Slow(delay time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := time.NewTimer(delay)
		defer t.Stop()
		select {
		case <-t.C:
		case <-r.Context().Done():
			return
		}
		writeJSON(w, map[string]any{"endpoint": "slow", "delay_ms": delay.Milliseconds()})
	}
}

// CPU counts primes below limit, stopping if the client goes away.
func CPU(limit int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count := 0
		for i := 2; i < limit; i++ {
			if i%10000 == 0 && r.Context().Err() != nil {
				return
			}
			prime := true
			for j, s := 2, int(math.Sqrt(float64(i))); j <= s; j++ {
				if i%j == 0 {
					prime = false
					break
				}
			}
			if prime {
				count++
			}
		}
		writeJSON(w, map[string]any{"endpoint": "cpu", "primes_found": count})
	}
}

// Memory allocates a large slice.
func Memory(w http.ResponseWriter, r *http.Request) {
	type obj struct {
		Index int     `json:"index"`
		Value float64 `json:"value"`
	}
	arr := make([]obj, 0, 1_000_000)
	for i := 0; i < 1_000_000; i++ {
		arr = append(arr, obj{Index: i, Value: float64(i) * 0.1})
	}
	writeJSON(w, map[string]any{"endpoint": "memory", "allocated": len(arr)})
}

func Info(cfg Config, start time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env := os.Getenv("APP_ENV")
		if env == "" {
			env = "development"
		}
		writeJSON(w, map[string]any{
			"app":            "LoadLab",
			"version":        cfg.Version,
			"go":             runtime.Version(),
			"uptime_seconds": int(time.Since(start).Seconds()),
			"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
			"environment":    env,
		})
	}
}

// Status probes each configured target concurrently.
func Status(cfg Config, client *http.Client) http.HandlerFunc {
	type result struct {
		Status string `json:"status"`
		Ms     int    `json:"ms"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var mu sync.Mutex
		var wg sync.WaitGroup
		out := make(map[string]result, len(cfg.StatusTargets))
		for name, url := range cfg.StatusTargets {
			wg.Add(1)
			go func() {
				defer wg.Done()
				t0 := time.Now()
				res := result{Status: "offline"}
				if resp, err := client.Get(url); err == nil {
					resp.Body.Close()
					if resp.StatusCode < 500 {
						res.Status = "online"
					}
				}
				res.Ms = int(time.Since(t0).Milliseconds())
				mu.Lock()
				out[name] = res
				mu.Unlock()
			}()
		}
		wg.Wait()
		writeJSON(w, out)
	}
}
