package balancer

import (
	"sync/atomic"

	"github.com/athieu123/goway/internal/backendpool"
)

// RoundRobin cycles through the healthy backends in order. The counter is
// atomic since Next is called concurrently by every in-flight request's
// goroutine.
type RoundRobin struct {
	counter uint64
}

func (r *RoundRobin) Next(pool []*backendpool.Backend) *backendpool.Backend {
	healthy := filterHealthy(pool)
	if len(healthy) == 0 {
		return nil
	}
	n := atomic.AddUint64(&r.counter, 1)
	return healthy[n%uint64(len(healthy))]
}
