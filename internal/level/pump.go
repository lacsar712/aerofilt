// Package vacuum manages Biofilter vessel evacuation and leak detection.
package vacuum

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// VacuumLeak marks an unacceptable vessel leak rate during evacuation.
var VacuumLeak = errors.New("vacuum leak rate exceeded")

// ErrPumpOffline marks vacuum pump not ready.
var ErrPumpOffline = errors.New("vacuum pump offline")

// ErrVentStuck marks vent valve failure.
var ErrVentStuck = errors.New("vent valve stuck")

// Pump simulates a turbomolecular + roughing pump stack.
type Pump struct {
	mu         sync.RWMutex
	pressurePa float64
	leakRate   float64
	online     bool
	ventOpen   bool
	targetPa   float64
	history    []model.VacuumReading
}

// NewPump creates an offline pump at atmospheric pressure.
func NewPump() *Pump {
	return &Pump{
		pressurePa: 101325,
		online:     true,
		history:    make([]model.VacuumReading, 0, 64),
	}
}

// SetOnline toggles pump availability.
func (p *Pump) SetOnline(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.online = v
}

// Online reports pump readiness.
func (p *Pump) Online() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.online
}

// Reading returns latest vacuum reading copy.
func (p *Pump) Reading(now time.Time) model.VacuumReading {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return model.VacuumReading{
		Pascals:  p.pressurePa,
		At:       now,
		LeakRate: p.leakRate,
		Source:   "pump",
	}
}

// EnsureTarget evacuates to target pressure; wraps VacuumLeak with %w on failure.
func (p *Pump) EnsureTarget(ctx context.Context, targetPa float64) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("vacuum cancelled: %w", ctx.Err())
	default:
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.online {
		return fmt.Errorf("roughing pump: %w", ErrPumpOffline)
	}
	p.targetPa = targetPa
	for p.pressurePa > targetPa {
		select {
		case <-ctx.Done():
			return fmt.Errorf("vacuum cancelled: %w", ctx.Err())
		default:
		}
		p.pressurePa *= 0.5
		if p.pressurePa < targetPa {
			p.pressurePa = targetPa
		}
	}
	if p.leakRate > 0 && p.leakRate*p.pressurePa > 0.01 {
		return fmt.Errorf("leak rate %.4f Pa/s at %.2f Pa: %w", p.leakRate, p.pressurePa, VacuumLeak)
	}
	return nil
}

// SetLeakRate configures simulated leak for tests.
func (p *Pump) SetLeakRate(rate float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.leakRate = rate
}

// OpenVent opens the vent valve to relieve pressure.
func (p *Pump) OpenVent(ctx context.Context, duration time.Duration) (model.VentResult, error) {
	select {
	case <-ctx.Done():
		return model.VentResult{}, fmt.Errorf("vent cancelled: %w", ctx.Err())
	default:
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ventOpen {
		return model.VentResult{}, fmt.Errorf("vent already open: %w", ErrVentStuck)
	}
	p.ventOpen = true
	startPa := p.pressurePa
	_ = duration
	p.pressurePa = 101325
	p.ventOpen = false
	return model.VentResult{
		Opened:      true,
		VentedPa:    startPa,
		Message:     "vent complete",
		CompletedAt: time.Now().UTC(),
	}, nil
}

// Classify maps vacuum errors to operator codes.
func Classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, VacuumLeak):
		return "leak"
	case errors.Is(err, ErrPumpOffline):
		return "offline"
	case errors.Is(err, ErrVentStuck):
		return "vent"
	default:
		return "unknown"
	}
}
