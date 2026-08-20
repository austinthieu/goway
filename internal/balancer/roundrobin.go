package balancer

import (
	"sync/atomic"

	"github.com/athieu123/goway/internal/backendpool"
)

// RoundRobin cycles through the pool in order. The counter is atomic since
// Next is called concurrently by every in-flight request's goroutine.
type RoundRobin struct {
	counter uint64
}

func (r *RoundRobin) Next(pool []*backendpool.Backend) *backendpool.Backend {
	if len(pool) == 0 {
		return nil
	}
	n := atomic.AddUint64(&r.counter, 1)
	return pool[n%uint64(len(pool))]
}
