// Package balancer selects which backend a given request should be routed
// to. Each strategy (round-robin, least-connections, weighted round-robin)
// implements the same Balancer interface, so the gateway can switch
// strategies via config without changing any call sites.
package balancer

import "github.com/austinthieu/goway/internal/backendpool"

// Balancer picks the next backend from pool. It must be safe for concurrent
// use by many goroutines handling simultaneous requests.
type Balancer interface {
	Next(pool []*backendpool.Backend) *backendpool.Backend
}

// filterEligible returns the subset of pool currently eligible for traffic.
// Shared by every strategy so "skip ineligible backends" logic lives in one
// place.
func filterEligible(pool []*backendpool.Backend) []*backendpool.Backend {
	eligible := make([]*backendpool.Backend, 0, len(pool))
	for _, b := range pool {
		if b.IsEligible() {
			eligible = append(eligible, b)
		}
	}
	return eligible
}
