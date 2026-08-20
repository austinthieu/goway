package ratelimit

import (
	"net/http"
	"testing"
)

func TestClientIDPrefersAPIKeyOverIP(t *testing.T) {
	r := &http.Request{Header: http.Header{"X-Api-Key": []string{"abc123"}}, RemoteAddr: "10.0.0.1:5555"}
	if got := ClientID(r); got != "key:abc123" {
		t.Fatalf("expected key:abc123, got %q", got)
	}
}

func TestClientIDFallsBackToIP(t *testing.T) {
	r := &http.Request{Header: http.Header{}, RemoteAddr: "10.0.0.1:5555"}
	if got := ClientID(r); got != "ip:10.0.0.1" {
		t.Fatalf("expected ip:10.0.0.1, got %q", got)
	}
}

func TestLimiterEnforcesPerClientCapacity(t *testing.T) {
	l := New(Options{Capacity: 3, RefillRate: 0})

	for i := 0; i < 3; i++ {
		if !l.Allow("client-a") {
			t.Fatalf("expected request %d for client-a to be allowed within capacity", i)
		}
	}
	if l.Allow("client-a") {
		t.Fatal("expected client-a to be rate limited after exhausting capacity")
	}

	// A different client has its own independent bucket.
	if !l.Allow("client-b") {
		t.Fatal("expected client-b to be unaffected by client-a's limit")
	}
}
