// Package pressure manages argon isostatic gas pressure during Biofilter.
package pressure

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// ErrPressureOverLimit marks gas pressure above vessel rating.
var ErrPressureOverLimit = errors.New("gas pressure over limit")

// ErrPressureUnderSetpoint marks failure to reach Biofilter setpoint.
var ErrPressureUnderSetpoint = errors.New("gas pressure under setpoint")

// ErrRegulatorFault marks gas regulator malfunction.
var ErrRegulatorFault = errors.New("gas regulator fault")

// Controller simulates argon pressure control for Biofilter.
type Controller struct {
	mu        sync.RWMutex
	currentMPa float64
	setpoint  float64
	maxMPa    float64
	online    bool
}

// NewController creates a gas pressure controller.
func NewController(maxMPa float64) *Controller {
	return &Controller{maxMPa: maxMPa, online: true}
}

// SetSetpoint updates target isostatic pressure.
func (c *Controller) SetSetpoint(mpa float64) error {
	if mpa <= 0 || mpa > c.maxMPa {
		return fmt.Errorf("setpoint %.2f out of range [0, %.2f]", mpa, c.maxMPa)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setpoint = mpa
	return nil
}

// Reading returns current gas pressure reading.
func (c *Controller) Reading(now time.Time) model.GasPressureReading {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return model.GasPressureReading{
		MPa:      c.currentMPa,
		At:       now,
		Setpoint: c.setpoint,
		Source:   "regulator",
	}
}

// Pressurize ramps gas to setpoint; wraps ErrPressureOverLimit with %w.
func (c *Controller) Pressurize(targetMPa float64) error {
	return c.PressurizeCtx(context.Background(), targetMPa)
}

// PressurizeCtx ramps gas to setpoint and exits early when ctx is cancelled.
func (c *Controller) PressurizeCtx(ctx context.Context, targetMPa float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.online {
		return fmt.Errorf("regulator: %w", ErrRegulatorFault)
	}
	if targetMPa > c.maxMPa {
		return fmt.Errorf("requested %.2f > max %.2f: %w", targetMPa, c.maxMPa, ErrPressureOverLimit)
	}
	for c.currentMPa < targetMPa {
		select {
		case <-ctx.Done():
			return fmt.Errorf("pressurize cancelled: %w", ctx.Err())
		default:
		}
		c.currentMPa += 5
		if c.currentMPa > targetMPa {
			c.currentMPa = targetMPa
		}
		if c.currentMPa > c.maxMPa {
			return fmt.Errorf("ramp exceeded max: %w", ErrPressureOverLimit)
		}
	}
	return nil
}

// CurrentMPa returns the live gas pressure under lock.
func (c *Controller) CurrentMPa() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentMPa
}

// Depressurize vents gas to ambient.
func (c *Controller) Depressurize() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentMPa = 0
}

// AtSetpoint reports whether current pressure is within tolerance.
func (c *Controller) AtSetpoint(toleranceMPa float64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.setpoint == 0 {
		return c.currentMPa == 0
	}
	diff := c.currentMPa - c.setpoint
	if diff < 0 {
		diff = -diff
	}
	return diff <= toleranceMPa
}

// Classify maps pressure errors to operator codes.
func Classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrPressureOverLimit):
		return "over"
	case errors.Is(err, ErrPressureUnderSetpoint):
		return "under"
	case errors.Is(err, ErrRegulatorFault):
		return "regulator"
	default:
		return "unknown"
	}
}
