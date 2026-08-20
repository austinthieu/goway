// Package balancer selects which backend a given request should be routed
// to. Each strategy (round-robin, least-connections, weighted round-robin)
// implements the same Balancer interface, so the gateway can switch
// strategies via config without changing any call sites.
package balancer

import "github.com/athieu123/goway/internal/backendpool"

// Balancer picks the next backend from pool. It must be safe for concurrent
// use by many goroutines handling simultaneous requests.
type Balancer interface {
	Next(pool []*backendpool.Backend) *backendpool.Backend
}
