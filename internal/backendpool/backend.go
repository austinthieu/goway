// Package backendpool holds the shared state for the set of backend servers
// the gateway routes traffic to: their addresses and their live health/load
// status, as tracked concurrently by the health checker, the proxy's request
// path, and the load balancer.
package backendpool

import (
	"net/http/httputil"
	"net/url"
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

// NewPool builds a Pool of reverse proxies for the given backend URLs.
func NewPool(urls []string) (*Pool, error) {
	pool := &Pool{}
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		pool.Backends = append(pool.Backends, &Backend{
			URL:          u,
			Weight:       1,
			ReverseProxy: httputil.NewSingleHostReverseProxy(u),
		})
	}
	return pool, nil
}
