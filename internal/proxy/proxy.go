// Package proxy wires the backend pool, load balancer, rate limiter, and
// metrics into an http.Handler that routes each incoming request to a
// chosen backend.
package proxy

import (
	"net/http"
	"time"

	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/balancer"
	"github.com/austinthieu/goway/internal/metrics"
	"github.com/austinthieu/goway/internal/ratelimit"
)

// Gateway routes incoming HTTP requests to a backend chosen by its balancer,
// after checking the caller's rate limit.
type Gateway struct {
	Pool     *backendpool.Pool
	Balancer balancer.Balancer
	Limiter  *ratelimit.Limiter
	Metrics  *metrics.Recorder
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.Limiter != nil && !g.Limiter.Allow(ratelimit.ClientID(r)) {
		if g.Metrics != nil {
			g.Metrics.RateLimitRejections.Inc()
		}
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	backend := g.Balancer.Next(g.Pool.Backends)
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
