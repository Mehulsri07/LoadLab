package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPublicServer_Timeouts(t *testing.T) {
	s := NewPublicServer(":0", http.NewServeMux())
	assertEq(t, s.ReadHeaderTimeout, 5*time.Second)
	assertEq(t, s.ReadTimeout, 10*time.Second)
	assertEq(t, s.WriteTimeout, 15*time.Second)
	assertEq(t, s.IdleTimeout, 60*time.Second)
	assertEq(t, s.MaxHeaderBytes, 64<<10)
}

func TestPublicServer_ClosesSlowHeaderClient(t *testing.T) {
	s := NewPublicServer("127.0.0.1:0", http.NewServeMux())
	s.ReadHeaderTimeout = 200 * time.Millisecond // shortened for the test
	ln := listen(t)
	go s.Serve(ln)
	defer s.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _ = c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")) // never finishes the headers
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	t0 := time.Now()
	_, err = c.Read(make([]byte, 1))
	if err == nil || time.Since(t0) > time.Second {
		t.Fatalf("slow client not cut off: err=%v after %v", err, time.Since(t0))
	}
}

func TestRun_DrainsInFlightRequest(t *testing.T) {
	cfg := Config{WorkWait: 300 * time.Millisecond, WorkTimeout: 2 * time.Second, Drain: 2 * time.Second}
	var h Health
	pub := NewPublicServer("127.0.0.1:0", PublicMux(cfg, NewMetrics(prometheus.NewRegistry(), "t"), &h, time.Now()))
	adm := &http.Server{Addr: "127.0.0.1:0", Handler: AdminMux(NewLeaker(make(chan time.Time)))}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, &h, pub, adm, t.Logf) }()
	base := waitListening(t, &h, pub) // readiness true, then Addr holds the bound address
	resp := make(chan int, 1)
	go func() {
		r, err := http.Get(base + "/work")
		if err != nil {
			resp <- -1
			return
		}
		r.Body.Close()
		resp <- r.StatusCode
	}()
	time.Sleep(50 * time.Millisecond)
	stop()                   // SIGTERM equivalent, mid-request
	assertEq(t, <-resp, 200) // in-flight request finished
	assertEq(t, <-done, nil)
	assertEq(t, code(h.Ready), 503) // readiness went false
}
