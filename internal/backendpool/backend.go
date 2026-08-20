// Package backendpool holds the shared state for the set of backend servers
// the gateway routes traffic to: their addresses and their live health/load
// status, as tracked concurrently by the health checker, the proxy's request
// path, and the load balancer.
package backendpool

import (
	"net/http/httputil"
	"net/url"

	"github.com/athieu123/goway/internal/config"
)

// Backend represents a single upstream server the gateway can route to.
type Backend struct {
	URL          *url.URL
	Weight       int
	ReverseProxy *httputil.ReverseProxy
}

// Pool is the ordered set of backends the gateway currently knows about.
type Pool struct {
	Backends []*Backend
}

// NewPool builds a Pool of reverse proxies for the given backend configs.
func NewPool(backends []config.BackendConfig) (*Pool, error) {
	pool := &Pool{}
	for _, b := range backends {
		u, err := url.Parse(b.URL)
		if err != nil {
			return nil, err
		}
		pool.Backends = append(pool.Backends, &Backend{
			URL:          u,
			Weight:       b.Weight,
			ReverseProxy: httputil.NewSingleHostReverseProxy(u),
		})
	}
	return pool, nil
}
