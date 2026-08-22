package filter

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Registry tracks historical wash operations per cell for scheduling analytics.
type Registry struct {
	mu      sync.RWMutex
	records map[model.CellID][]WashRecord
	cap     int
}

// WashRecord captures one completed or in-flight wash on a cell.
type WashRecord struct {
	OperationID model.OperationID
	FilterID    model.FilterID
	Phase       model.WashPhase
	StartedAt   time.Time
	CompletedAt time.Time
	Operator    string
}

// NewRegistry creates a bounded in-memory wash history per cell.
func NewRegistry(capPerCell int) *Registry {
	if capPerCell <= 0 {
		capPerCell = 16
	}
	return &Registry{records: make(map[model.CellID][]WashRecord), cap: capPerCell}
}

// Start records the beginning of a wash operation.
func (r *Registry) Start(op model.OperationID, filter model.FilterID, cell model.CellID, phase model.WashPhase, operator string, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := WashRecord{OperationID: op, FilterID: filter, Phase: phase, StartedAt: at, Operator: operator}
	r.records[cell] = append(r.records[cell], rec)
	r.trim(cell)
}

// Complete marks the latest open record complete for a cell.
func (r *Registry) Complete(cell model.CellID, phase model.WashPhase, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.records[cell]
	if len(list) == 0 {
		return fmt.Errorf("no wash record for cell %s", cell)
	}
	last := list[len(list)-1]
	if !last.CompletedAt.IsZero() {
		return fmt.Errorf("latest wash already complete")
	}
	last.Phase = phase
	last.CompletedAt = at
	list[len(list)-1] = last
	r.records[cell] = list
	return nil
}

// Last returns the most recent wash record for a cell.
func (r *Registry) Last(cell model.CellID) (WashRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.records[cell]
	if len(list) == 0 {
		return WashRecord{}, false
	}
	return list[len(list)-1], true
}

// SinceLastWash returns duration since the last completed wash, or zero if none.
func (r *Registry) SinceLastWash(cell model.CellID, now time.Time) time.Duration {
	rec, ok := r.Last(cell)
	if !ok || rec.CompletedAt.IsZero() {
		return 0
	}
	return now.Sub(rec.CompletedAt)
}

// History returns a copy of records for a cell.
func (r *Registry) History(cell model.CellID) []WashRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.records[cell]
	out := make([]WashRecord, len(list))
	copy(out, list)
	return out
}

func (r *Registry) trim(cell model.CellID) {
	list := r.records[cell]
	if len(list) <= r.cap {
		return
	}
	r.records[cell] = list[len(list)-r.cap:]
}
