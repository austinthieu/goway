package tokenbucket

import (
	"sync"
	"testing"
	"time"
)

func TestAllowConsumesUpToCapacity(t *testing.T) {
	b := New(3, 1)
	if !b.Allow() || !b.Allow() || !b.Allow() {
		t.Fatal("expected first 3 Allow() calls to succeed at capacity 3")
	}
	if b.Allow() {
		t.Fatal("expected 4th Allow() to fail once bucket is empty")
	}
}

func TestRefillOverTime(t *testing.T) {
	b := New(2, 10) // 10 tokens/sec
	clock := time.Now()
	b.now = func() time.Time { return clock }
	b.lastRefill = clock

	if !b.Allow() || !b.Allow() {
		t.Fatal("expected bucket to start full")
	}
	if b.Allow() {
		t.Fatal("expected bucket to be empty")
	}

	clock = clock.Add(150 * time.Millisecond) // should refill ~1.5 tokens
	if !b.Allow() {
		t.Fatal("expected a token to be available after refill")
	}
}

func TestRefillCapsAtCapacity(t *testing.T) {
	b := New(2, 100) // fast refill
	clock := time.Now()
	b.now = func() time.Time { return clock }
	b.lastRefill = clock

	b.Allow()
	b.Allow()

	clock = clock.Add(10 * time.Second) // would overflow capacity without capping
	if !b.AllowN(2) {
		t.Fatal("expected bucket to be capped at capacity, not overflowed")
	}
	if b.Allow() {
		t.Fatal("expected bucket to be empty again after draining the capped refill")
	}
}

func TestConcurrentAllowDoesNotOverAdmit(t *testing.T) {
	b := New(100, 0) // no refill, fixed budget of 100 tokens

	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0

	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Allow() {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if admitted != 100 {
		t.Fatalf("expected exactly 100 admitted under concurrent load, got %d", admitted)
	}
}
