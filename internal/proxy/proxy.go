// Package proxy wires the backend pool, load balancer, and rate limiter
// into an http.Handler that routes each incoming request to a chosen
// backend.
package proxy

import (
	"net/http"

	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/balancer"
	"github.com/austinthieu/goway/internal/ratelimit"
)

// Gateway routes incoming HTTP requests to a backend chosen by its balancer,
// after checking the caller's rate limit.
type Gateway struct {
	Pool     *backendpool.Pool
	Balancer balancer.Balancer
	Limiter  *ratelimit.Limiter
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.Limiter != nil && !g.Limiter.Allow(ratelimit.ClientID(r)) {
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

	backend.ReverseProxy.ServeHTTP(w, r)
}
