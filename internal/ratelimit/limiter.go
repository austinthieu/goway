// Package ratelimit applies a per-Client token bucket rate limit to
// incoming requests before they reach the balancer.
package ratelimit

import (
	"net/http"
	"strings"
	"sync"

	"github.com/austinthieu/gobalance/pkg/tokenbucket"
)

// Options configures the shared default token bucket applied to every
// client.
type Options struct {
	Capacity   float64
	RefillRate float64
}

// Limiter tracks one token bucket per client identity. Buckets are lazily
// created on first use and share Capacity/RefillRate from Options.
//
// buckets is a plain map guarded by a mutex: map access itself isn't
// goroutine-safe in Go even though each individual *tokenbucket.Bucket has
// its own internal lock — two levels of locking, one per client and one for
// the map that holds them.
type Limiter struct {
	opts    Options
	mu      sync.Mutex
	buckets map[string]*tokenbucket.Bucket
}

func New(opts Options) *Limiter {
	return &Limiter{opts: opts, buckets: make(map[string]*tokenbucket.Bucket)}
}

// Allow reports whether a request from clientID may proceed.
func (l *Limiter) Allow(clientID string) bool {
	l.mu.Lock()
	b, ok := l.buckets[clientID]
	if !ok {
		b = tokenbucket.New(l.opts.Capacity, l.opts.RefillRate)
		l.buckets[clientID] = b
	}
	l.mu.Unlock()

	return b.Allow()
}

// ClientID identifies the caller for rate-limiting purposes: the
// X-API-Key header if present, otherwise the request's source IP.
func ClientID(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return "key:" + key
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	return "ip:" + host
}
