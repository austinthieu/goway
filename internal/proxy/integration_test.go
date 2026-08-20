package proxy_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/austinthieu/goway/internal/config"
	"github.com/austinthieu/goway/internal/proxy"
)

// fakeBackend is an httptest.Server whose /healthz always succeeds and
// whose / reports its own name, so the test can tell which backend served
// a given request.
func fakeBackend(t *testing.T, name string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, name)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestGateway(t *testing.T, ctx context.Context, backendURLs []string) *proxy.Gateway {
	t.Helper()
	cfg := &config.Config{
		Strategy: "round_robin",
		RateLimit: config.RateLimitConfig{
			Capacity:   1000, // effectively unlimited for these tests
			RefillRate: 1000,
		},
	}
	for _, u := range backendURLs {
		cfg.Backends = append(cfg.Backends, config.BackendConfig{URL: u, Weight: 1})
	}

	gw := &proxy.Gateway{}
	if err := gw.Reload(ctx, cfg); err != nil {
		t.Fatalf("failed to build gateway: %v", err)
	}
	return gw
}

func bodyOf(t *testing.T, gw *proxy.Gateway) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	gw.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestGatewayRoundRobinsAcrossFakeBackends(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b1 := fakeBackend(t, "backend-1")
	b2 := fakeBackend(t, "backend-2")
	b3 := fakeBackend(t, "backend-3")
	gw := newTestGateway(t, ctx, []string{b1.URL, b2.URL, b3.URL})

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		seen[bodyOf(t, gw)]++
	}

	for _, name := range []string{"backend-1", "backend-2", "backend-3"} {
		if seen[name] != 3 {
			t.Errorf("expected backend-1/2/3 to each be hit 3 times, got %v", seen)
			break
		}
	}
}

func TestGatewayFailsOverWhenABackendGoesDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b1 := fakeBackend(t, "backend-1")
	b2 := fakeBackend(t, "backend-2")
	gw := newTestGateway(t, ctx, []string{b1.URL, b2.URL})

	// Confirm both serve traffic before the failure.
	seen := map[string]int{}
	for i := 0; i < 4; i++ {
		seen[bodyOf(t, gw)]++
	}
	if seen["backend-1"] == 0 || seen["backend-2"] == 0 {
		t.Fatalf("expected both backends to serve traffic before failure, got %v", seen)
	}

	// Kill backend-1: it stops responding to /healthz entirely.
	b1.Close()

	// The active health checker runs on its own timer (DefaultOptions:
	// 2s interval), so poll until it has caught the failure rather than
	// sleeping a fixed guess.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		seen = map[string]int{}
		ok := true
		for i := 0; i < 6; i++ {
			body := bodyOf(t, gw)
			seen[body]++
			if body != "backend-2" {
				ok = false
			}
		}
		if ok {
			return // success: every request landed on the surviving backend
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("expected all traffic to land on backend-2 after backend-1 died, last seen: %v", seen)
}
