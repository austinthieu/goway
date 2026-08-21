// Package balancer selects which backend a given request should be routed
// to. Each strategy (round-robin, least-connections, weighted round-robin)
// implements the same Balancer interface, so the gateway can switch
// strategies via config without changing any call sites.
package balancer

import "github.com/austinthieu/gobalance/internal/backendpool"

// Balancer picks the next backend from pool. It must be safe for concurrent
// use by many goroutines handling simultaneous requests.
type Balancer interface {
	Next(pool []*backendpool.Backend) *backendpool.Backend
}

// New builds the Balancer named by strategy.
func New(strategy string) (Balancer, error) {
	switch strategy {
	case "round_robin", "":
		return &RoundRobin{}, nil
	case "least_conn":
		return &LeastConnections{}, nil
	case "weighted":
		return &WeightedRoundRobin{}, nil
	default:
		return nil, unknownStrategyError(strategy)
	}
}

type unknownStrategyError string

func (e unknownStrategyError) Error() string {
	return "unknown load balancing strategy: " + string(e)
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
