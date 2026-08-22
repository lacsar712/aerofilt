// Package filter coordinates exclusive chamber leases with defer-safe mutex handling.
package filter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
	"github.com/lacsar712/aerofilt/internal/level"
)

// Coordinator serializes Biofilter cycles so early returns never leave the lock held.
type Coordinator struct {
	mu      sync.Mutex
	lease   *model.ChamberLease
	ttl     time.Duration
	book    *Book
	vacuum  *vacuum.Pump
	lastErr string
	cycles  int
	holds   int
}

// NewCoordinator builds a chamber coordinator.
func NewCoordinator(ttl time.Duration, book *Book, pump *vacuum.Pump) *Coordinator {
	return &Coordinator{ttl: ttl, book: book, vacuum: pump}
}

// RunCycle acquires the exclusive lease, performs vacuum precheck, and always
// releases via defer so an early return cannot block later cycles.
func (c *Coordinator) RunCycle(ctx context.Context, cycleID model.CycleID, operator string, targetPa float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.holds++

	if err := c.ensureLeaseLocked(cycleID, operator, time.Now().UTC()); err != nil {
		c.lastErr = err.Error()
		return err
	}

	select {
	case <-ctx.Done():
		c.clearLeaseLocked()
		c.lastErr = ctx.Err().Error()
		return fmt.Errorf("cycle cancelled: %w", ctx.Err())
	default:
	}

	if c.vacuum != nil {
		if err := c.vacuum.EnsureTarget(ctx, targetPa); err != nil {
			c.clearLeaseLocked()
			c.lastErr = err.Error()
			return fmt.Errorf("cycle vacuum precheck: %w", err)
		}
	}

	c.cycles++
	c.clearLeaseLocked()
	return nil
}

// TryBusy reports whether a lease is currently held.
func (c *Coordinator) TryBusy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lease != nil && time.Now().Before(c.lease.Expires)
}

// LastError returns last failure text.
func (c *Coordinator) LastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

// Cycles returns successful cycle count.
func (c *Coordinator) Cycles() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cycles
}

// Holds returns diagnostic lock hold count.
func (c *Coordinator) Holds() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.holds
}

func (c *Coordinator) ensureLeaseLocked(id model.CycleID, holder string, now time.Time) error {
	if c.lease != nil && now.Before(c.lease.Expires) && c.lease.CycleID != id {
		return fmt.Errorf("chamber lease held by %s until %s", c.lease.Holder, c.lease.Expires.Format(time.RFC3339))
	}
	if holder == "" {
		holder = "system"
	}
	c.lease = &model.ChamberLease{
		CycleID:  id,
		Holder:   holder,
		Acquired: now,
		Expires:  now.Add(c.ttl),
	}
	return nil
}

func (c *Coordinator) clearLeaseLocked() {
	c.lease = nil
}

// ForceExpire clears lease (tests / operator override).
func (c *Coordinator) ForceExpire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lease = nil
}
