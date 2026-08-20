package balancer

import "github.com/austinthieu/goway/internal/backendpool"

// LeastConnections picks the healthy backend with the fewest active
// connections. Ties are broken by first-found-in-pool-order.
type LeastConnections struct{}

func (l *LeastConnections) Next(pool []*backendpool.Backend) *backendpool.Backend {
	healthy := filterHealthy(pool)
	if len(healthy) == 0 {
		return nil
	}

	best := healthy[0]
	bestConns := best.ActiveConnections()
	for _, b := range healthy[1:] {
		if c := b.ActiveConnections(); c < bestConns {
			best, bestConns = b, c
		}
	}
	return best
}
