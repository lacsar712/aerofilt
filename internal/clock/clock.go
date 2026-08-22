// Package clock provides wall and process clocks for backwash window timing.
package clock

import (
	"sync"
	"sync/atomic"
	"time"
)

// Clock abstracts time sources used by backwash hold windows.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Advance(d time.Duration)
}

// RealClock uses the system wall clock. Advance is a no-op.
type RealClock struct{}

func (RealClock) Now() time.Time                  { return time.Now() }
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }
func (RealClock) Advance(d time.Duration)         {}

// ProcessClock advances only when Advance is called, so wall-clock stalls
// (operator pause / processing halt) do not alone close backwash windows.
type ProcessClock struct {
	mu      sync.RWMutex
	current time.Time
	started time.Time
	ticks   atomic.Int64
}

// NewProcessClock starts at origin.
func NewProcessClock(origin time.Time) *ProcessClock {
	if origin.IsZero() {
		origin = time.Unix(0, 0).UTC()
	}
	return &ProcessClock{current: origin, started: origin}
}

// Now returns the process instant.
func (c *ProcessClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// Since returns duration from t to current process time.
func (c *ProcessClock) Since(t time.Time) time.Duration {
	return c.Now().Sub(t)
}

// Advance moves the process clock forward by d.
func (c *ProcessClock) Advance(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	c.current = c.current.Add(d)
	c.mu.Unlock()
	c.ticks.Add(1)
}

// Set forces an absolute process instant (tests).
func (c *ProcessClock) Set(t time.Time) {
	c.mu.Lock()
	c.current = t
	c.mu.Unlock()
}

// Ticks returns how many Advance calls have been applied.
func (c *ProcessClock) Ticks() int64 { return c.ticks.Load() }

// StartedAt returns when the process clock was created.
func (c *ProcessClock) StartedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.started
}
