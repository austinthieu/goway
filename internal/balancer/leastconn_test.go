package balancer

import "testing"

func TestLeastConnectionsPicksMinimum(t *testing.T) {
	pool := newTestPool(t, 1, 1, 1)
	pool[0].IncConn()
	pool[0].IncConn()
	pool[1].IncConn()
	// pool[2] has 0 active connections.

	lc := &LeastConnections{}
	got := lc.Next(pool)
	if got != pool[2] {
		t.Fatalf("expected the backend with 0 active connections, got %s", got.URL)
	}
}

func TestLeastConnectionsTiesBreakByPoolOrder(t *testing.T) {
	pool := newTestPool(t, 1, 1, 1)
	// All tied at 0 connections.

	lc := &LeastConnections{}
	got := lc.Next(pool)
	if got != pool[0] {
		t.Fatalf("expected first-found-in-pool-order tie-break (pool[0]), got %s", got.URL)
	}
}

func TestLeastConnectionsSkipsIneligibleEvenIfLeastLoaded(t *testing.T) {
	pool := newTestPool(t, 1, 1)
	pool[1].SetHealthy(false) // pool[1] has fewer connections but is ineligible
	pool[0].IncConn()

	lc := &LeastConnections{}
	got := lc.Next(pool)
	if got != pool[0] {
		t.Fatalf("expected the only eligible backend despite higher load, got %s", got.URL)
	}
}
