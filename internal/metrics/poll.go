package metrics

import (
	"context"
	"time"

	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/circuitbreaker"
)

// PollBackends periodically snapshots each backend's active-health and
// circuit-breaker state into gauges, until ctx is canceled. These states
// don't change on every request the way Requests/RequestDuration do, so a
// poll loop is simpler than threading a Recorder into the health checker
// and breaker themselves.
func PollBackends(ctx context.Context, pool *backendpool.Pool, r *Recorder, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, b := range pool.Backends {
				label := b.URL.String()

				healthy := 0.0
				if b.ActiveHealthy() {
					healthy = 1.0
				}
				r.BackendHealthy.WithLabelValues(label).Set(healthy)

				r.BreakerState.WithLabelValues(label).Set(breakerStateValue(b.Breaker.State()))
			}
		}
	}
}

func breakerStateValue(s circuitbreaker.State) float64 {
	switch s {
	case circuitbreaker.Open:
		return 1
	case circuitbreaker.HalfOpen:
		return 2
	default:
		return 0
	}
}
