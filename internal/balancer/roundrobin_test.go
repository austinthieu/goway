package balancer

import (
	"sync"
	"testing"
)

func TestRoundRobinDistributesEvenly(t *testing.T) {
	pool := newTestPool(t, 1, 1, 1)
	rr := &RoundRobin{}

	counts := map[string]int{}
	const n = 300
	for i := 0; i < n; i++ {
		b := rr.Next(pool)
		if b == nil {
			t.Fatal("expected a backend, got nil")
		}
		counts[b.URL.String()]++
	}

	for url, c := range counts {
		if c != n/len(pool) {
			t.Errorf("backend %s: expected exactly %d selections, got %d", url, n/len(pool), c)
		}
	}
}

func TestRoundRobinSkipsIneligibleBackends(t *testing.T) {
	pool := newTestPool(t, 1, 1, 1)
	pool[1].SetHealthy(false)
	rr := &RoundRobin{}

	for i := 0; i < 30; i++ {
		b := rr.Next(pool)
		if b == pool[1] {
			t.Fatal("expected round-robin to never select the ineligible backend")
		}
	}
}

func TestRoundRobinReturnsNilWhenNoneEligible(t *testing.T) {
	pool := newTestPool(t, 1, 1)
	for _, b := range pool {
		b.SetHealthy(false)
	}
	rr := &RoundRobin{}

	if got := rr.Next(pool); got != nil {
		t.Fatalf("expected nil when no backends are eligible, got %v", got)
	}
}

func TestRoundRobinConcurrentSelectionIsSafe(t *testing.T) {
	pool := newTestPool(t, 1, 1, 1, 1)
	rr := &RoundRobin{}

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b := rr.Next(pool); b == nil {
				t.Error("expected a non-nil backend under concurrent selection")
			}
		}()
	}
	wg.Wait()
}
