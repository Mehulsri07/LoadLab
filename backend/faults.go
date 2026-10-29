package main

import (
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

const mb = 1 << 20

// Leaker grows resident memory at a set rate until stopped.
type Leaker struct {
	mu     sync.Mutex
	mbMin  int
	chunks [][]byte
	held   int // bytes
}

func NewLeaker(tick <-chan time.Time) *Leaker {
	l := &Leaker{}
	go func() {
		for range tick {
			l.grow()
		}
	}()
	return l
}

func (l *Leaker) grow() {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := l.mbMin * mb / 60
	if n == 0 {
		return
	}
	b := make([]byte, n)
	for i := 0; i < len(b); i += 4096 { // touch every page so it is really resident
		b[i] = 1
	}
	l.chunks = append(l.chunks, b)
	l.held += n
}

func (l *Leaker) Start(mbPerMin int) {
	l.mu.Lock()
	l.mbMin = mbPerMin
	l.mu.Unlock()
}

func (l *Leaker) Stop() {
	l.mu.Lock()
	l.mbMin, l.chunks, l.held = 0, nil, 0
	l.mu.Unlock()
	debug.FreeOSMemory()
}

func (l *Leaker) HeldMB() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held / mb
}

func (l *Leaker) rate() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.mbMin
}

// FaultMiddleware injects latency and errors, sparing probes and metrics.
func FaultMiddleware(cfg Config, rnd func() float64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/livez", "/readyz", "/health", "/metrics":
			next.ServeHTTP(w, r)
			return
		}
		if cfg.FaultLatency > 0 {
			t := time.NewTimer(cfg.FaultLatency)
			select {
			case <-t.C:
			case <-r.Context().Done():
				t.Stop()
				return
			}
		}
		if rnd() < cfg.FaultErrorRate {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]string{"status": "injected fault"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
