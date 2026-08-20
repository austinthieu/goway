package balancer

import (
	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/config"
)

// testingTB is the subset of *testing.T / *testing.B that newTestPool
// needs, so it can be shared between unit tests and benchmarks.
type testingTB interface {
	Helper()
	Fatalf(format string, args ...interface{})
}

// newTestPool builds n backends, each pointed at a distinct dummy URL (no
// server needs to actually be listening — these tests only exercise
// selection logic, never dispatch a real request), starting Eligible.
func newTestPool(t testingTB, weights ...int) []*backendpool.Backend {
	t.Helper()

	var backends []config.BackendConfig
	for i, w := range weights {
		backends = append(backends, config.BackendConfig{
			URL:    "http://backend" + string(rune('a'+i)) + ".invalid",
			Weight: w,
		})
	}

	pool, err := backendpool.NewPool(backends)
	if err != nil {
		t.Fatalf("failed to build test pool: %v", err)
	}
	return pool.Backends
}
