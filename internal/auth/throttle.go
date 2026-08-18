package auth

import (
	"sync"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
)

const (
	freeAttempts = 5
	baseWindow   = time.Second
	maxWindow    = 15 * time.Minute
)

const sweepAt = 1024

type record struct {
	attempts int
	until    time.Time
	seen     time.Time
}

type memoryThrottle struct {
	clk clock.Clock

	mu     sync.Mutex
	failed map[string]*record
}

func NewThrottle(clk clock.Clock) Throttle {
	if clk == nil {
		panic("auth: nil clock")
	}
	return &memoryThrottle{clk: clk, failed: make(map[string]*record)}
}

func (t *memoryThrottle) Allow(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, found := t.failed[key]
	if !found {
		return true
	}
	return !t.clk.Now().Before(rec.until)
}

func (t *memoryThrottle) Fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.clk.Now()

	rec, found := t.failed[key]
	if !found {
		if len(t.failed) >= sweepAt {
			t.sweep(now)
		}
		rec = &record{}
		t.failed[key] = rec
	}

	rec.attempts++
	rec.seen = now
	if rec.attempts >= freeAttempts {
		rec.until = now.Add(window(rec.attempts))
	}
}

func (t *memoryThrottle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.failed, key)
}

// sweep drops every key idle for longer than the cap. The caller holds the lock.
func (t *memoryThrottle) sweep(now time.Time) {
	for key, rec := range t.failed {
		if now.Sub(rec.seen) > maxWindow {
			delete(t.failed, key)
		}
	}
}

// window is how long the key waits after this many attempts. The fifth failure
// waits one second, the sixth waits two, and so on up to the cap.
func window(attempts int) time.Duration {
	over := attempts - freeAttempts
	if over < 0 {
		return 0
	}

	const maxShift = 62
	if over > maxShift {
		return maxWindow
	}

	held := baseWindow << over
	if held > maxWindow || held <= 0 {
		return maxWindow
	}
	return held
}
