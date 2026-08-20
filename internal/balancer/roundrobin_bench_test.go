package balancer

import "testing"

func BenchmarkRoundRobinNext(b *testing.B) {
	pool := newTestPool(b, 1, 1, 1)
	rr := &RoundRobin{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr.Next(pool)
	}
}
