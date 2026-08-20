package tokenbucket

import "testing"

func BenchmarkAllow(b *testing.B) {
	bucket := New(1e9, 1e9) // effectively never empties, isolates Allow's own cost

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucket.Allow()
	}
}
