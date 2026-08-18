package auth

import (
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
)

func newTestThrottle(t *testing.T) (Throttle, *clock.FixedClock) {
	t.Helper()
	clk := clock.Fixed(time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC), time.UTC)
	return NewThrottle(clk), clk
}

func TestThrottleAllowsTheFreeAttempts(t *testing.T) {
	throttle, _ := newTestThrottle(t)

	for i := range freeAttempts {
		if !throttle.Allow("a@b.c") {
			t.Fatalf("Attempt %d was held back, want %d free", i+1, freeAttempts)
		}
		throttle.Fail("a@b.c")
	}

	if throttle.Allow("a@b.c") {
		t.Errorf("Attempt %d was allowed, want it held back", freeAttempts+1)
	}
}

func TestThrottleWindowDoublesAndIsCapped(t *testing.T) {
	throttle, clk := newTestThrottle(t)

	// The first four failures cost nothing.
	for range freeAttempts - 1 {
		throttle.Fail("a@b.c")
	}

	// The fifth failure waits one second, the sixth waits two, the seventh four.
	for _, want := range []time.Duration{baseWindow, 2 * baseWindow, 4 * baseWindow} {
		throttle.Fail("a@b.c")

		clk.Advance(want - time.Millisecond)
		if throttle.Allow("a@b.c") {
			t.Fatalf("Key opened before the %v window closed", want)
		}

		clk.Advance(time.Millisecond)
		if !throttle.Allow("a@b.c") {
			t.Fatalf("Key stayed shut after the %v window closed", want)
		}
	}

	// Far past the cap, the window is still fifteen minutes.
	for range 40 {
		throttle.Fail("a@b.c")
	}
	clk.Advance(maxWindow)
	if !throttle.Allow("a@b.c") {
		t.Errorf("Window grew past the %v cap", maxWindow)
	}
}

func TestThrottleResetClearsTheKey(t *testing.T) {
	throttle, _ := newTestThrottle(t)

	for range freeAttempts + 3 {
		throttle.Fail("a@b.c")
	}
	if throttle.Allow("a@b.c") {
		t.Fatal("Key was not held back after the free attempts")
	}

	throttle.Reset("a@b.c")

	if !throttle.Allow("a@b.c") {
		t.Error("Reset did not open the key")
	}
}

func TestThrottleKeepsKeysApart(t *testing.T) {
	throttle, _ := newTestThrottle(t)

	for range freeAttempts + 1 {
		throttle.Fail("a@b.c")
	}

	if !throttle.Allow("d@e.f") {
		t.Error("Failures on one key held back another")
	}
}
