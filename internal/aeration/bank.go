// Package heater implements resistance heater banks and side-effect emission.
package heater

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Emitter applies heater side effects from the backwash FSM.
type Emitter interface {
	EmitHeater(ctx context.Context, cmd model.HeaterCommand) error
	EmitVent(ctx context.Context, cmd model.VentCommand) error
}

// CountingEmitter records heater/vent emissions for diagnostics.
type CountingEmitter struct {
	heaterCount atomic.Int64
	ventCount   atomic.Int64
	maxKW       float64
	mu          sync.Mutex
	lastHeater  model.HeaterCommand
	lastVent    model.VentCommand
}

// NewCountingEmitter creates an emitter with power clamp.
func NewCountingEmitter(maxKW float64) *CountingEmitter {
	return &CountingEmitter{maxKW: maxKW}
}

// EmitHeater applies heater power command.
func (e *CountingEmitter) EmitHeater(ctx context.Context, cmd model.HeaterCommand) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("heater emit cancelled: %w", ctx.Err())
	default:
	}
	if err := model.ValidateHeaterCommand(cmd); err != nil {
		return err
	}
	if cmd.PowerKW > e.maxKW {
		cmd.PowerKW = e.maxKW
	}
	e.mu.Lock()
	e.lastHeater = cmd
	e.mu.Unlock()
	e.heaterCount.Add(1)
	return nil
}

// EmitVent applies vent valve command.
func (e *CountingEmitter) EmitVent(ctx context.Context, cmd model.VentCommand) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("vent emit cancelled: %w", ctx.Err())
	default:
	}
	if err := model.ValidateVentCommand(cmd); err != nil {
		return err
	}
	e.mu.Lock()
	e.lastVent = cmd
	e.mu.Unlock()
	e.ventCount.Add(1)
	return nil
}

// HeaterCount returns total heater emissions.
func (e *CountingEmitter) HeaterCount() int64 { return e.heaterCount.Load() }

// VentCount returns total vent emissions.
func (e *CountingEmitter) VentCount() int64 { return e.ventCount.Load() }

// Bank manages multi-zone heater power allocation.
type Bank struct {
	mu      sync.RWMutex
	maxKW   float64
	phase   model.HeaterPhase
	zones   map[model.ZoneID]float64
	emitter Emitter
}

// NewBank creates a heater bank.
func NewBank(maxKW float64, emitter Emitter) *Bank {
	return &Bank{
		maxKW:   maxKW,
		phase:   model.HeaterOff,
		zones:   make(map[model.ZoneID]float64),
		emitter: emitter,
	}
}

// Phase returns current heater phase.
func (b *Bank) Phase() model.HeaterPhase {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.phase
}

// Apply sends heater command through emitter.
func (b *Bank) Apply(ctx context.Context, cmd model.HeaterCommand) (model.HeaterResult, error) {
	if cmd.IssuedAt.IsZero() {
		cmd.IssuedAt = time.Now().UTC()
	}
	if cmd.PowerKW > b.maxKW {
		cmd.PowerKW = b.maxKW
	}
	b.mu.Lock()
	b.phase = model.HeaterActive
	b.zones[cmd.ZoneID] = cmd.PowerKW
	b.mu.Unlock()

	if b.emitter == nil {
		return model.HeaterResult{
			CycleID: cmd.CycleID, Phase: model.HeaterActive, AppliedKW: cmd.PowerKW,
			Emitted: false, Message: "no emitter", CompletedAt: time.Now().UTC(),
		}, nil
	}
	if err := b.emitter.EmitHeater(ctx, cmd); err != nil {
		b.mu.Lock()
		b.phase = model.HeaterFault
		b.mu.Unlock()
		return model.HeaterResult{
			CycleID: cmd.CycleID, Phase: model.HeaterFault, Message: err.Error(),
			CompletedAt: time.Now().UTC(),
		}, err
	}
	return model.HeaterResult{
		CycleID: cmd.CycleID, Phase: model.HeaterActive, AppliedKW: cmd.PowerKW,
		Emitted: true, Message: "ok", CompletedAt: time.Now().UTC(),
	}, nil
}

// Idle sets heater bank to off.
func (b *Bank) Idle() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.phase = model.HeaterOff
	for z := range b.zones {
		b.zones[z] = 0
	}
}

// ZonePower returns allocated kW for a zone.
func (b *Bank) ZonePower(zone model.ZoneID) float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.zones[zone]
}

// TotalKW returns sum of zone allocations.
func (b *Bank) TotalKW() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var sum float64
	for _, kw := range b.zones {
		sum += kw
	}
	return sum
}

// RampPower steps heater power toward targetKW. Each step passes ctx into the
// emitter so an abort cancel stops further PWM climb.
func (b *Bank) RampPower(ctx context.Context, zone model.ZoneID, targetKW float64, steps int) error {
	if steps < 1 {
		steps = 1
	}
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("heater ramp cancelled: %w", ctx.Err())
		default:
		}
		kw := targetKW * float64(i) / float64(steps)
		cmd := model.HeaterCommand{
			CycleID:  model.CycleID(fmt.Sprintf("heater-ramp-%d", i)),
			ZoneID:   zone,
			PowerKW:  kw,
			IssuedAt: time.Now().UTC(),
			Operator: "heater-ramp",
		}
		if _, err := b.Apply(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}
