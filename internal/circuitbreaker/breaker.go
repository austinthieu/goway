// Package circuitbreaker implements a per-backend circuit breaker that
// reacts to failures observed on live proxied traffic, independent of and
// faster than the active health checker (see internal/healthcheck).
package circuitbreaker

import (
	"sync"
	"time"
)

// State is one of Closed, Open, or HalfOpen.
type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

// Options configures the breaker's trip/recovery behavior.
type Options struct {
	FailureThreshold int           // consecutive failures to trip Closed -> Open
	OpenDuration     time.Duration // how long Open lasts before trying HalfOpen
	HalfOpenTrials   int           // number of trial calls allowed per HalfOpen cycle
}

// DefaultOptions returns the project's chosen defaults: 5 consecutive
// failures trips the breaker, it stays Open for 10s, and HalfOpen allows a
// single trial call to decide recovery.
func DefaultOptions() Options {
	return Options{FailureThreshold: 5, OpenDuration: 10 * time.Second, HalfOpenTrials: 1}
}

// Breaker is a single backend's circuit breaker. State transitions involve
// several related fields together (state, failure count, timers), so this
// is guarded by a mutex rather than atomics.
type Breaker struct {
	mu       sync.Mutex
	opts     Options
	state    State
	failures int
	openedAt time.Time
	trials   int
}

func New(opts Options) *Breaker {
	return &Breaker{opts: opts, state: Closed}
}

// Allow is a side-effect-free eligibility check, safe to call any number of
// times per request (e.g. once per backend while a Balancer filters
// candidates). The one exception isn't really a side effect from a caller's
// perspective: once OpenDuration has elapsed it flips Open -> HalfOpen,
// which is idempotent under concurrent callers. It does NOT consume a
// HalfOpen trial slot — call Reserve for that, exactly once, right before
// actually dispatching a request.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return true
	case Open:
		if time.Since(b.openedAt) < b.opts.OpenDuration {
			return false
		}
		b.state = HalfOpen
		b.trials = 0
		return true
	case HalfOpen:
		return b.trials < b.opts.HalfOpenTrials
	}
	return false
}

// Reserve claims one trial slot for an actual dispatch attempt. Call this
// exactly once per request, immediately before proxying, after the
// balancer has already chosen this backend via Allow-based filtering.
// Closed always succeeds; Open always fails; HalfOpen succeeds for at most
// HalfOpenTrials concurrent callers per cycle, so a request that loses the
// race gets a clean failure instead of silently proceeding.
func (b *Breaker) Reserve() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return true
	case Open:
		return false
	case HalfOpen:
		if b.trials >= b.opts.HalfOpenTrials {
			return false
		}
		b.trials++
		return true
	}
	return false
}

// RecordSuccess reports a successful call. In HalfOpen this recovers the
// breaker to Closed; in Closed it resets the failure counter.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case HalfOpen:
		b.state = Closed
		b.failures = 0
	case Closed:
		b.failures = 0
	}
}

// RecordFailure reports a Failure (connection error or backend 5xx). In
// HalfOpen this immediately re-trips to Open. In Closed it trips to Open
// once FailureThreshold consecutive failures have been recorded.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case HalfOpen:
		b.trip()
	case Closed:
		b.failures++
		if b.failures >= b.opts.FailureThreshold {
			b.trip()
		}
	}
}

func (b *Breaker) trip() {
	b.state = Open
	b.openedAt = time.Now()
	b.failures = 0
	b.trials = 0
}

// State returns the breaker's current state, for metrics/logging.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
