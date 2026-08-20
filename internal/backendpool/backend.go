// Package backendpool holds the shared state for the set of backend servers
// the gateway routes traffic to: their addresses and their live health/load
// status, as tracked concurrently by the health checker, the proxy's request
// path, and the load balancer.
package backendpool

import (
	"net/http/httputil"
	"net/url"
	"sync/atomic"

	"github.com/athieu123/goway/internal/config"
)

// Backend represents a single upstream server the gateway can route to.
type Backend struct {
	URL          *url.URL
	Weight       int
	ReverseProxy *httputil.ReverseProxy

	// healthy is read on every request (by the balancer) and written
	// concurrently by both the active health-check loop and the passive
	// failure-counting request path, so it's an atomic.Bool rather than a
	// mutex-guarded field: writers never block readers on the hot path.
	healthy atomic.Bool
}

// Pool is the ordered set of backends the gateway currently knows about.
type Pool struct {
	Backends []*Backend
}

// NewPool builds a Pool of reverse proxies for the given backend configs.
// Backends start out healthy; the health checker will mark any down.
func NewPool(backends []config.BackendConfig) (*Pool, error) {
	pool := &Pool{}
	for _, b := range backends {
		u, err := url.Parse(b.URL)
		if err != nil {
			return nil, err
		}
		backend := &Backend{
			URL:          u,
			Weight:       b.Weight,
			ReverseProxy: httputil.NewSingleHostReverseProxy(u),
		}
		backend.healthy.Store(true)
		pool.Backends = append(pool.Backends, backend)
	}
	return pool, nil
}

// IsHealthy reports whether this backend is currently eligible for traffic.
func (b *Backend) IsHealthy() bool {
	return b.healthy.Load()
}

// SetHealthy updates the backend's health state.
func (b *Backend) SetHealthy(healthy bool) {
	b.healthy.Store(healthy)
}

// Healthy returns the subset of pool that are currently eligible for traffic.
func (p *Pool) Healthy() []*Backend {
	healthy := make([]*Backend, 0, len(p.Backends))
	for _, b := range p.Backends {
		if b.IsHealthy() {
			healthy = append(healthy, b)
		}
	}
	return healthy
}
