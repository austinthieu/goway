package balancer

import (
	"testing"

	"github.com/austinthieu/goway/internal/backendpool"
)

func TestWeightedRoundRobinRespectsWeightRatio(t *testing.T) {
	pool := newTestPool(t, 5, 3, 2) // weights sum to 10
	wrr := &WeightedRoundRobin{}

	const n = 1000
	counts := map[string]int{}
	for i := 0; i < n; i++ {
		b := wrr.Next(pool)
		counts[b.URL.String()]++
	}

	want := map[string]int{
		pool[0].URL.String(): 500,
		pool[1].URL.String(): 300,
		pool[2].URL.String(): 200,
	}
	for url, w := range want {
		got := counts[url]
		if got != w {
			t.Errorf("backend %s: expected exactly %d selections (weight ratio), got %d", url, w, got)
		}
	}
}

func TestWeightedRoundRobinInterleavesRatherThanClumping(t *testing.T) {
	// weight 5:1 — smooth WRR should not pick the heavy backend 5 times in
	// a row; it should interleave the light backend in between.
	pool := newTestPool(t, 5, 1)
	wrr := &WeightedRoundRobin{}

	consecutive := 0
	maxConsecutive := 0
	var last *backendpool.Backend
	for i := 0; i < 12; i++ {
		b := wrr.Next(pool)
		if last != nil && b == last {
			consecutive++
			if consecutive > maxConsecutive {
				maxConsecutive = consecutive
			}
		} else {
			consecutive = 0
		}
		last = b
	}

	if maxConsecutive >= 5 {
		t.Fatalf("expected smooth interleaving, but the heavy backend ran %d times consecutively", maxConsecutive+1)
	}
}
