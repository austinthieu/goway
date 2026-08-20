// Package proxy wires the backend pool and load balancer into an
// http.Handler that routes each incoming request to a chosen backend.
package proxy

import (
	"net/http"

	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/balancer"
)

// Gateway routes incoming HTTP requests to a backend chosen by its balancer.
type Gateway struct {
	Pool     *backendpool.Pool
	Balancer balancer.Balancer
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := g.Balancer.Next(g.Pool.Backends)
	if backend == nil {
		http.Error(w, "no backends available", http.StatusServiceUnavailable)
		return
	}
	backend.ReverseProxy.ServeHTTP(w, r)
}
