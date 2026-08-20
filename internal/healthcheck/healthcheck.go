// Package healthcheck actively probes each backend's /healthz endpoint on a
// timer and marks it healthy or unhealthy accordingly.
package healthcheck

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/athieu123/goway/internal/backendpool"
)

// Options configures the active health-check loop.
type Options struct {
	Interval time.Duration
	Timeout  time.Duration
}

// DefaultOptions returns sane defaults for interval/timeout.
func DefaultOptions() Options {
	return Options{Interval: 2 * time.Second, Timeout: 1 * time.Second}
}

// Run starts one goroutine per backend that periodically probes /healthz
// until ctx is canceled. It blocks until ctx is done.
func Run(ctx context.Context, pool *backendpool.Pool, opts Options) {
	for _, backend := range pool.Backends {
		go probeLoop(ctx, backend, opts)
	}
	<-ctx.Done()
}

func probeLoop(ctx context.Context, backend *backendpool.Backend, opts Options) {
	client := &http.Client{Timeout: opts.Timeout}
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	healthzURL := backend.URL.String() + "/healthz"

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			healthy := probe(client, healthzURL)
			if healthy != backend.IsHealthy() {
				log.Printf("healthcheck: backend %s healthy=%v", backend.URL, healthy)
			}
			backend.SetHealthy(healthy)
		}
	}
}

func probe(client *http.Client, url string) bool {
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
