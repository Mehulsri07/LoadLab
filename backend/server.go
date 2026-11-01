package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// NewPublicServer bounds how long a client may take to send headers and body,
// so slow or stuck clients cannot pin connections.
func NewPublicServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// Run binds both servers, marks the pod ready and serves until ctx is done or a
// server fails. It then fails readiness and drains in-flight requests for up to
// cfg.Drain. Each server's Addr is rewritten to the bound address before
// SetReady(true), so a reader that has seen readiness sees the real port.
func Run(ctx context.Context, cfg Config, health *Health, public, admin *http.Server, logf func(string, ...any)) error {
	servers := []*http.Server{public, admin}
	lns := make([]net.Listener, 0, len(servers))
	for _, s := range servers {
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			for _, l := range lns {
				l.Close()
			}
			return err
		}
		s.Addr = ln.Addr().String()
		lns = append(lns, ln)
	}
	health.SetReady(true)

	serveErr := make(chan error, len(servers))
	for i, s := range servers {
		go func() { serveErr <- s.Serve(lns[i]) }()
	}
	logf("listening: public=%s admin=%s", public.Addr, admin.Addr)

	var err error
	select {
	case <-ctx.Done():
	case err = <-serveErr: // Serve only returns early on failure; it is never ErrServerClosed here
		logf("serve failed: %v", err)
	}

	health.SetDraining()
	logf("draining (up to %v)", cfg.Drain)
	sctx, cancel := context.WithTimeout(context.Background(), cfg.Drain)
	defer cancel()
	shutErr := make(chan error, len(servers))
	for _, s := range servers {
		go func() { shutErr <- s.Shutdown(sctx) }()
	}
	for range servers {
		if e := <-shutErr; err == nil && !errors.Is(e, http.ErrServerClosed) {
			err = e
		}
	}
	logf("stopped")
	return err
}
