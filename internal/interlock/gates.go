// Package interlock evaluates pre-cycle Biofilter filter safety gates.
package interlock

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Gates collects boolean plant conditions required before Biofilter cycle start.
type Gates struct {
	mu           sync.RWMutex
	estop        bool
	doorClosed   bool
	coolingOK    bool
	powerOK      bool
	gasOK        bool
	vacuumOK     bool
	strict       bool
	lastEval     time.Time
	lastDecision model.InterlockDecision
}

// NewGates constructs interlock gates.
func NewGates(strict bool) *Gates {
	return &Gates{
		doorClosed: true,
		coolingOK:  true,
		powerOK:    true,
		gasOK:      true,
		vacuumOK:   true,
		strict:     strict,
	}
}

// SetEStop latches emergency stop.
func (g *Gates) SetEStop(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.estop = v
}

// EStop reports emergency stop latch.
func (g *Gates) EStop() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.estop
}

// SetDoorClosed updates vessel door interlock.
func (g *Gates) SetDoorClosed(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.doorClosed = v
}

// SetCoolingOK marks blower cooling readiness.
func (g *Gates) SetCoolingOK(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.coolingOK = v
}

// SetPowerOK marks main heater power.
func (g *Gates) SetPowerOK(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.powerOK = v
}

// SetGasOK marks argon supply readiness.
func (g *Gates) SetGasOK(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.gasOK = v
}

// SetVacuumOK marks vacuum system readiness.
func (g *Gates) SetVacuumOK(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.vacuumOK = v
}

// Evaluate checks profile + plant gates for a cycle permit.
func (g *Gates) Evaluate(snap model.ProfileSnapshot, bandC float64, vacuumHealthy bool) model.InterlockDecision {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastEval = time.Now().UTC()
	d := model.InterlockDecision{Allowed: true, Code: "OK", Reasons: nil}

	if g.estop {
		d.Allowed = false
		d.Code = "ESTOP"
		d.Reasons = append(d.Reasons, "emergency stop latched")
	}
	if !g.powerOK {
		d.Allowed = false
		d.Code = "POWER"
		d.Reasons = append(d.Reasons, "heater power not ready")
	}
	if !g.doorClosed {
		d.Allowed = false
		d.Code = "DOOR"
		d.Reasons = append(d.Reasons, "vessel door not closed")
	}
	if g.strict && !g.coolingOK {
		d.Allowed = false
		d.Code = "COOLING"
		d.Reasons = append(d.Reasons, "blower cooling not ready")
	}
	if g.strict && !g.gasOK {
		d.Allowed = false
		d.Code = "GAS"
		d.Reasons = append(d.Reasons, "argon supply not ready")
	}
	if !vacuumHealthy {
		d.Allowed = false
		d.Code = "VACUUM"
		d.Reasons = append(d.Reasons, "vacuum system unhealthy")
	}
	if snap.Mode == model.FilterModeEStop {
		d.Allowed = false
		d.Code = "FURNACE_ESTOP"
		d.Reasons = append(d.Reasons, "filter mode estop")
	}
	for id, z := range snap.Zones {
		if z.SensorN == 0 {
			d.Allowed = false
			d.Code = "SENSOR"
			d.Reasons = append(d.Reasons, fmt.Sprintf("zone %s has no sensors", id))
			continue
		}
		if snap.Mode == model.FilterModeBackwash && !model.WithinBand(z, bandC) {
			d.Allowed = false
			d.Code = "TEMP"
			d.Reasons = append(d.Reasons, fmt.Sprintf("zone %s off backwash band by %.1f C", id, model.TempErrorC(z)))
		}
	}
	if len(snap.Zones) == 0 {
		d.Allowed = false
		d.Code = "EMPTY"
		d.Reasons = append(d.Reasons, "no temperature zones available")
	}
	g.lastDecision = d
	return d
}

// LastDecision returns the most recent evaluation.
func (g *Gates) LastDecision() model.InterlockDecision {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lastDecision
}

// SnapshotGates returns a diagnostic copy of gate bits.
func (g *Gates) SnapshotGates() map[string]bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]bool{
		"estop":      g.estop,
		"doorClosed": g.doorClosed,
		"coolingOK":  g.coolingOK,
		"powerOK":    g.powerOK,
		"gasOK":      g.gasOK,
		"vacuumOK":   g.vacuumOK,
		"strict":     g.strict,
	}
}
