package balancer

import (
	"sync"

	"github.com/austinthieu/goway/internal/backendpool"
)

// WeightedRoundRobin is the smooth/interleaved weighted round-robin
// algorithm (as used by Nginx): each selection bumps every healthy
// backend's current weight by its static weight, picks the backend with
// the highest current weight, then deducts the total weight from the
// winner. This spreads high-weight backends evenly through the sequence
// instead of clumping them, unlike a flattened weight-list approach.
//
// Next mutates several related fields together (the per-backend running
// weight and the selection itself), so this needs a mutex rather than
// atomics — the deliberate contrast case to RoundRobin's atomic counter.
type WeightedRoundRobin struct {
	mu             sync.Mutex
	currentWeights map[*backendpool.Backend]int
}

func (w *WeightedRoundRobin) Next(pool []*backendpool.Backend) *backendpool.Backend {
	eligible := filterEligible(pool)
	if len(eligible) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentWeights == nil {
		w.currentWeights = make(map[*backendpool.Backend]int)
	}

	total := 0
	var selected *backendpool.Backend
	for _, b := range eligible {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		total += weight

		w.currentWeights[b] += weight
		if selected == nil || w.currentWeights[b] > w.currentWeights[selected] {
			selected = b
		}
	}

	w.currentWeights[selected] -= total
	return selected
}
