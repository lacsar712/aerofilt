// Package alarm maintains operator-facing Biofilter filter alarms.
package alarm

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Bus is a bounded active/historical alarm store.
type Bus struct {
	mu       sync.Mutex
	active   map[string]model.AlarmEvent
	history  []model.AlarmEvent
	capacity int
	seq      int
}

// NewBus creates an alarm bus.
func NewBus(capacity int) *Bus {
	if capacity < 16 {
		capacity = 16
	}
	return &Bus{
		active:   make(map[string]model.AlarmEvent),
		history:  make([]model.AlarmEvent, 0, capacity),
		capacity: capacity,
	}
}

// Raise inserts or refreshes an active alarm.
func (b *Bus) Raise(code, message, filterID string, sev model.AlarmSeverity) model.AlarmEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.active[code]; ok {
		existing.Message = message
		existing.RaisedAt = time.Now().UTC()
		b.active[code] = existing
		return existing
	}
	b.seq++
	ev := model.AlarmEvent{
		ID:        fmt.Sprintf("alm-%d", b.seq),
		Code:      code,
		Severity:  sev,
		Message:   message,
		FilterID: filterID,
		RaisedAt:  time.Now().UTC(),
		Active:    true,
	}
	b.active[code] = ev
	b.pushHistoryLocked(ev)
	return ev
}

// Clear deactivates an alarm by code.
func (b *Bus) Clear(code string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	ev, ok := b.active[code]
	if !ok {
		return false
	}
	now := time.Now().UTC()
	ev.Active = false
	ev.ClearedAt = &now
	delete(b.active, code)
	b.pushHistoryLocked(ev)
	return true
}

// ActiveCount returns number of active alarms.
func (b *Bus) ActiveCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.active)
}

// ListActive returns a copy of active alarms.
func (b *Bus) ListActive() []model.AlarmEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]model.AlarmEvent, 0, len(b.active))
	for _, ev := range b.active {
		out = append(out, ev)
	}
	return out
}

// History returns a copy of recent alarm events.
func (b *Bus) History() []model.AlarmEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]model.AlarmEvent, len(b.history))
	copy(out, b.history)
	return out
}

func (b *Bus) pushHistoryLocked(ev model.AlarmEvent) {
	b.history = append(b.history, ev)
	if len(b.history) > b.capacity {
		b.history = b.history[len(b.history)-b.capacity:]
	}
}

// RaiseTempImbalance helper for backwash band violations.
func (b *Bus) RaiseTempImbalance(filterID string, zones []model.ZoneID) {
	msg := fmt.Sprintf("temperature imbalance on zones %v", zones)
	b.Raise("TEMP_IMBALANCE", msg, filterID, model.SeverityCritical)
}

// RaiseVacuumLeak helper when VacuumLeak surfaces.
func (b *Bus) RaiseVacuumLeak(filterID string) {
	b.Raise("VACUUM_LEAK", "vessel leak rate exceeded during evacuation", filterID, model.SeverityCritical)
}

// RaiseEStop helper for emergency stop.
func (b *Bus) RaiseEStop(filterID string) {
	b.Raise("ESTOP", "emergency stop engaged — cycle inhibited", filterID, model.SeverityCritical)
}

// RaiseMediaFault helper for recoverable media faults.
func (b *Bus) RaiseMediaFault(filterID, sensor string) {
	b.Raise("THERMO_FAULT", fmt.Sprintf("recoverable fault on sensor %s", sensor), filterID, model.SeverityWarn)
}
