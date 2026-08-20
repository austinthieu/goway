// Package proxy wires the backend pool, load balancer, rate limiter, and
// metrics into an http.Handler that routes each incoming request to a
// chosen backend. It also owns config reload: Reload builds an entirely new
// snapshot off to the side and swaps it in atomically, so in-flight
// requests finish against the snapshot they started with while new
// requests immediately see the reloaded config.
package proxy

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/balancer"
	"github.com/austinthieu/goway/internal/config"
	"github.com/austinthieu/goway/internal/healthcheck"
	"github.com/austinthieu/goway/internal/metrics"
	"github.com/austinthieu/goway/internal/ratelimit"
)

// snapshot is everything a request needs, built together and swapped in as
// one atomic unit so a request never sees a pool from one config alongside
// a balancer from another.
type snapshot struct {
	pool     *backendpool.Pool
	balancer balancer.Balancer
	limiter  *ratelimit.Limiter
	cancel   context.CancelFunc // stops this snapshot's health-check/metrics goroutines
}

// Gateway routes incoming HTTP requests to a backend chosen by its balancer,
// after checking the caller's rate limit.
type Gateway struct {
	Metrics *metrics.Recorder

	current  atomic.Pointer[snapshot]
	reloadMu sync.Mutex // serializes concurrent Reload calls
}

// Reload builds a new pool/balancer/limiter from cfg, starts its
// health-check and metrics-poll goroutines under a fresh cancelable
// context derived from ctx, then atomically swaps it in as the active
// snapshot. The previous snapshot's background goroutines are canceled
// only after the swap, so there's never a moment with zero snapshots.
func (g *Gateway) Reload(ctx context.Context, cfg *config.Config) error {
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()

	pool, err := backendpool.NewPool(cfg.Backends)
	if err != nil {
		return err
	}
	bal, err := balancer.New(cfg.Strategy)
	if err != nil {
		return err
	}
	limiter := ratelimit.New(ratelimit.Options{
		Capacity:   cfg.RateLimit.Capacity,
		RefillRate: cfg.RateLimit.RefillRate,
	})

	snapCtx, cancel := context.WithCancel(ctx)
	go healthcheck.Run(snapCtx, pool, healthcheck.DefaultOptions())
	if g.Metrics != nil {
		go metrics.PollBackends(snapCtx, pool, g.Metrics, time.Second)
	}

	newSnap := &snapshot{pool: pool, balancer: bal, limiter: limiter, cancel: cancel}
	old := g.current.Swap(newSnap)
	if old != nil {
		old.cancel()
	}
	return nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snap := g.current.Load()

	if snap.limiter != nil && !snap.limiter.Allow(ratelimit.ClientID(r)) {
		if g.Metrics != nil {
			g.Metrics.RateLimitRejections.Inc()
		}
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	backend := snap.balancer.Next(snap.pool.Backends)
	if backend == nil {
		http.Error(w, "no backends available", http.StatusServiceUnavailable)
		return
	}

	// The balancer chose this backend from Allow()-filtered candidates;
	// Reserve claims the actual trial slot (matters only in HalfOpen, where
	// concurrent requests can race for a single slot).
	if !backend.Breaker.Reserve() {
		http.Error(w, "backend temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	backend.IncConn()
	defer backend.DecConn()

	if g.Metrics == nil {
		backend.ReverseProxy.ServeHTTP(w, r)
		return
	}

	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	backend.ReverseProxy.ServeHTTP(rec, r)
	g.Metrics.ObserveRequest(backend.URL.String(), rec.statusCode, time.Since(start))
}

// statusRecorder captures the status code a ReverseProxy writes, since
// httputil.ReverseProxy writes directly to the ResponseWriter with no
// built-in way to observe what it sent.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.statusCode = code
	s.ResponseWriter.WriteHeader(code)
}
