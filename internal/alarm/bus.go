package alarm

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type Bus struct {
	mu      sync.Mutex
	active  map[string]model.AlarmEvent
	history []model.AlarmEvent
	cap     int
	seq     int
}

func NewBus(capacity int) *Bus {
	if capacity <= 0 {
		capacity = 32
	}
	return &Bus{active: make(map[string]model.AlarmEvent), cap: capacity}
}

func (b *Bus) Raise(code, message string, plant model.PlantID, sev model.Severity) model.AlarmEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	ev := model.AlarmEvent{ID: fmt.Sprintf("ALM-%d", b.seq), Code: code, Message: message, PlantID: plant, Severity: sev, RaisedAt: time.Now().UTC(), Active: true}
	b.active[code] = ev
	b.history = append(b.history, ev)
	if len(b.history) > b.cap*4 {
		b.history = b.history[len(b.history)-b.cap*2:]
	}
	return ev
}

func (b *Bus) RaiseEStop(plant model.PlantID) { b.Raise("ESTOP", model.ErrPlantEStop.Error(), plant, model.SeverityCritical) }
func (b *Bus) RaiseHeadFault(plant model.PlantID, cell model.CellID, msg string) {
	b.Raise("HEAD_FAULT", fmt.Sprintf("%s: %s", cell, msg), plant, model.SeverityWarning)
}
func (b *Bus) RaiseMediaFault(plant model.PlantID, filter model.FilterID) {
	b.Raise("MEDIA_CLOG", string(filter), plant, model.SeverityWarning)
}
func (b *Bus) Clear(code string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	ev, ok := b.active[code]
	if !ok {
		return false
	}
	ev.Active = false
	delete(b.active, code)
	b.history = append(b.history, ev)
	return true
}
func (b *Bus) ListActive() []model.AlarmEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]model.AlarmEvent, 0, len(b.active))
	for _, ev := range b.active {
		out = append(out, ev)
	}
	return out
}
func (b *Bus) ActiveCount() int { b.mu.Lock(); defer b.mu.Unlock(); return len(b.active) }