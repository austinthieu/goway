package circuitbreaker

import (
	"testing"
	"time"
)

func testOptions() Options {
	return Options{FailureThreshold: 3, OpenDuration: 20 * time.Millisecond, HalfOpenTrials: 1}
}

func TestClosedStaysClosedBelowThreshold(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	if b.State() != Closed {
		t.Fatalf("expected Closed below threshold, got %v", b.State())
	}
	if !b.Allow() {
		t.Fatal("expected Allow() true while Closed")
	}
}

func TestTripsOpenAtThreshold(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	if b.State() != Open {
		t.Fatalf("expected Open at threshold, got %v", b.State())
	}
	if b.Allow() {
		t.Fatal("expected Allow() false immediately after trip")
	}
	if b.Reserve() {
		t.Fatal("expected Reserve() false while Open")
	}
}

func TestOpenTransitionsToHalfOpenAfterDuration(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()

	time.Sleep(30 * time.Millisecond)

	if !b.Allow() {
		t.Fatal("expected Allow() true once OpenDuration has elapsed")
	}
	if b.State() != HalfOpen {
		t.Fatalf("expected HalfOpen after elapsed Open duration, got %v", b.State())
	}
}

func TestAllowDoesNotConsumeHalfOpenTrial(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	time.Sleep(30 * time.Millisecond)

	// Filtering-style repeated Allow() calls must not burn the trial slot.
	for i := 0; i < 5; i++ {
		if !b.Allow() {
			t.Fatalf("Allow() call %d: expected true, filtering must be side-effect free", i)
		}
	}
	if !b.Reserve() {
		t.Fatal("expected Reserve() to still succeed after repeated Allow() calls")
	}
}

func TestHalfOpenReserveOnlyAllowsConfiguredTrials(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	time.Sleep(30 * time.Millisecond)
	b.Allow() // transitions Open -> HalfOpen

	if !b.Reserve() {
		t.Fatal("expected first Reserve() in HalfOpen to succeed")
	}
	if b.Reserve() {
		t.Fatal("expected second concurrent Reserve() in HalfOpen to fail (trial budget exhausted)")
	}
}

func TestHalfOpenSuccessRecoversToClosed(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	time.Sleep(30 * time.Millisecond)
	b.Allow()
	b.Reserve()

	b.RecordSuccess()
	if b.State() != Closed {
		t.Fatalf("expected Closed after HalfOpen success, got %v", b.State())
	}
}

func TestHalfOpenFailureReTripsOpen(t *testing.T) {
	b := New(testOptions())
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	time.Sleep(30 * time.Millisecond)
	b.Allow()
	b.Reserve()

	b.RecordFailure()
	if b.State() != Open {
		t.Fatalf("expected Open after HalfOpen failure, got %v", b.State())
	}
}
