package clock

import (
	"sync"
	"time"
)

var _ Clock = (*FixedClock)(nil)

// Fixed returns a clock stopped at at, reckoning days in tz.
// It is moved by calling Advanced.
func Fixed(at time.Time, tz *time.Location) *FixedClock {
	return &FixedClock{now: at.In(checkLocation(tz))}
}

// FixedClock is the test double. The mutex is there because the code under test
// may read the clock from several goroutines while the test advances it.
type FixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *FixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FixedClock) Today() time.Time { return day(c.Now()) }

// Advance moves the clock by d. Negative durations move it back.
func (c *FixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
