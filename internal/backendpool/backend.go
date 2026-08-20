// Package backendpool holds the shared state for the set of backend servers
// the gateway routes traffic to: their addresses and their live health/load
// status, as tracked concurrently by the health checker, the circuit
// breaker, the proxy's request path, and the load balancer.
package backendpool

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"

	"github.com/austinthieu/goway/internal/circuitbreaker"
	"github.com/austinthieu/goway/internal/config"
)

// Backend represents a single upstream server the gateway can route to.
type Backend struct {
	URL          *url.URL
	Weight       int
	ReverseProxy *httputil.ReverseProxy
	Breaker      *circuitbreaker.Breaker

	// healthy is written only by the active health checker (see
	// internal/healthcheck) and read on every request by the balancer, so
	// it's an atomic.Bool rather than a mutex-guarded field: the health
	// check loop never blocks a request in flight.
	healthy atomic.Bool

	// activeConns is incremented/decremented around every proxied request
	// by internal/proxy, and read by the LeastConnections strategy on every
	// selection — a hot path on both ends, hence atomic rather than a mutex.
	activeConns int64
}

// Pool is the ordered set of backends the gateway currently knows about.
type Pool struct {
	Backends []*Backend
}

// NewPool builds a Pool of reverse proxies for the given backend configs.
// Backends start out healthy and Closed; the health checker and circuit
// breaker will mark any down as failures are observed.
func NewPool(backends []config.BackendConfig) (*Pool, error) {
	pool := &Pool{}
	for _, b := range backends {
		u, err := url.Parse(b.URL)
		if err != nil {
			return nil, err
		}

		backend := &Backend{
			URL:     u,
			Weight:  b.Weight,
			Breaker: circuitbreaker.New(circuitbreaker.DefaultOptions()),
		}
		backend.healthy.Store(true)

		rp := httputil.NewSingleHostReverseProxy(u)
		rp.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode >= 500 {
				backend.Breaker.RecordFailure()
			} else {
				backend.Breaker.RecordSuccess()
			}
			return nil
		}
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			backend.Breaker.RecordFailure()
			w.WriteHeader(http.StatusBadGateway)
		}
		backend.ReverseProxy = rp

		pool.Backends = append(pool.Backends, backend)
	}
	return pool, nil
}

// IsHealthy reports whether this backend is currently eligible for traffic:
// confirmed healthy by the active checker AND not tripped open by the
// circuit breaker.
func (b *Backend) IsHealthy() bool {
	return b.healthy.Load() && b.Breaker.Allow()
}

// SetHealthy updates the backend's active-health-check state.
func (b *Backend) SetHealthy(healthy bool) {
	b.healthy.Store(healthy)
}

// IncConn records the start of a proxied request to this backend.
func (b *Backend) IncConn() {
	atomic.AddInt64(&b.activeConns, 1)
}

// DecConn records the end of a proxied request to this backend.
func (b *Backend) DecConn() {
	atomic.AddInt64(&b.activeConns, -1)
}

// ActiveConnections returns the number of in-flight requests to this backend.
func (b *Backend) ActiveConnections() int64 {
	return atomic.LoadInt64(&b.activeConns)
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
