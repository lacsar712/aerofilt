// Package blower implements controlled cooling after Biofilter backwash.
package blower

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Cooler simulates argon blower or filter cooling after backwash.
type Cooler struct {
	mu           sync.RWMutex
	rateCPerMin  float64
	active       bool
	startTempC   float64
	currentTempC float64
	startedAt    time.Time
	valveOpen    float64 // 0..1 purge valve fraction
}

// NewCooler creates a blower controller.
func NewCooler(rateCPerMin float64) *Cooler {
	if rateCPerMin <= 0 {
		rateCPerMin = 10
	}
	return &Cooler{rateCPerMin: rateCPerMin}
}

// Start begins blower from current fleet temperature.
func (c *Cooler) Start(ctx context.Context, startC float64) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("blower cancelled: %w", ctx.Err())
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = true
	c.startTempC = startC
	c.currentTempC = startC
	c.startedAt = time.Now().UTC()
	c.valveOpen = 0
	return nil
}

// PurgeRamp opens the blower purge valve across multiple ramp steps.
// Caller cancel must stop mid-ramp before the valve reaches full open.
func (c *Cooler) PurgeRamp(ctx context.Context, steps int) error {
	if steps < 1 {
		steps = 1
	}
	c.mu.Lock()
	c.active = true
	c.startedAt = time.Now().UTC()
	c.mu.Unlock()
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("blower purge cancelled: %w", ctx.Err())
		default:
		}
		c.mu.Lock()
		c.valveOpen = float64(i) / float64(steps)
		c.mu.Unlock()
	}
	return nil
}

// ValveOpen returns the current purge valve fraction (0..1).
func (c *Cooler) ValveOpen() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.valveOpen
}

// Step advances blower by elapsed process time.
func (c *Cooler) Step(elapsed time.Duration) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.active {
		return c.currentTempC
	}
	delta := c.rateCPerMin * elapsed.Minutes()
	c.currentTempC -= delta
	if c.currentTempC < 25 {
		c.currentTempC = 25
		c.active = false
	}
	return c.currentTempC
}

// Active reports whether blower is running.
func (c *Cooler) Active() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

// Complete forces blower completion.
func (c *Cooler) Complete() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = false
	c.currentTempC = 25
}

// Status returns blower diagnostic snapshot.
func (c *Cooler) Status() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"active":      c.active,
		"startC":      c.startTempC,
		"currentC":    c.currentTempC,
		"rateCPerMin": c.rateCPerMin,
		"startedAt":   c.startedAt,
		"valveOpen":   c.valveOpen,
	}
}

// EstimateDuration returns time to reach target from start.
func (c *Cooler) EstimateDuration(fromC, toC float64) time.Duration {
	if c.rateCPerMin <= 0 || fromC <= toC {
		return 0
	}
	minutes := (fromC - toC) / c.rateCPerMin
	return time.Duration(minutes * float64(time.Minute))
}

// ApplyToSnapshot reduces all zone means uniformly (simulation helper).
func (c *Cooler) ApplyToSnapshot(snap model.ProfileSnapshot, targetC float64) model.ProfileSnapshot {
	out := model.CloneProfileSnapshot(snap)
	c.mu.RLock()
	delta := c.startTempC - c.currentTempC
	c.mu.RUnlock()
	if delta <= 0 {
		return out
	}
	for id, z := range out.Zones {
		z.MeanC -= delta
		if z.MeanC < targetC {
			z.MeanC = targetC
		}
		z.MinC -= delta
		z.MaxC -= delta
		out.Zones[id] = z
	}
	return out
}
