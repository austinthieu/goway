// Package tokenbucket implements a standalone token bucket rate limiter.
// It has no dependency on the rest of goway, so it's usable outside this
// project as-is.
package tokenbucket

import (
	"sync"
	"time"
)

// Bucket is a single token bucket: it holds up to Capacity tokens and
// refills continuously at RefillRate tokens per second.
type Bucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
	now        func() time.Time
}

// New creates a Bucket that starts full.
func New(capacity, refillRate float64) *Bucket {
	return &Bucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
		now:        time.Now,
	}
}

// Allow consumes one token if available and reports whether it did.
func (b *Bucket) Allow() bool {
	return b.AllowN(1)
}

// AllowN consumes n tokens if available and reports whether it did.
func (b *Bucket) AllowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}

// refill must be called with mu held.
func (b *Bucket) refill() {
	now := b.now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now

	b.tokens += elapsed * b.refillRate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
}
