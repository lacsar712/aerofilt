// Package batch tracks Biofilter load batches through the filter.
package batch

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Registry maintains active and completed Biofilter batches.
type Registry struct {
	mu        sync.RWMutex
	active    map[model.BatchID]model.BatchRecord
	completed []model.BatchRecord
	limit     int
}

// NewRegistry creates a batch registry.
func NewRegistry(limit int) *Registry {
	if limit < 8 {
		limit = 8
	}
	return &Registry{
		active:    make(map[model.BatchID]model.BatchRecord),
		completed: make([]model.BatchRecord, 0, limit),
		limit:     limit,
	}
}

// Register starts a new batch.
func (r *Registry) Register(rec model.BatchRecord) error {
	if rec.ID == "" {
		return fmt.Errorf("batch id required")
	}
	if rec.StartedAt.IsZero() {
		rec.StartedAt = time.Now().UTC()
	}
	if rec.Phase == "" {
		rec.Phase = model.BackwashIdle
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.active[rec.ID]; exists {
		return fmt.Errorf("batch %s already active", rec.ID)
	}
	r.active[rec.ID] = rec
	return nil
}

// UpdatePhase updates batch backwash phase.
func (r *Registry) UpdatePhase(id model.BatchID, phase model.BackwashPhase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.active[id]
	if !ok {
		return fmt.Errorf("batch %s not found", id)
	}
	rec.Phase = phase
	r.active[id] = rec
	return nil
}

// Complete moves batch to completed history.
func (r *Registry) Complete(id model.BatchID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.active[id]
	if !ok {
		return fmt.Errorf("batch %s not found", id)
	}
	now := time.Now().UTC()
	rec.CompletedAt = &now
	rec.Phase = model.BackwashComplete
	delete(r.active, id)
	r.completed = append(r.completed, rec)
	if len(r.completed) > r.limit {
		r.completed = r.completed[len(r.completed)-r.limit:]
	}
	return nil
}

// Active returns copy of active batches.
func (r *Registry) Active() []model.BatchRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.BatchRecord, 0, len(r.active))
	for _, rec := range r.active {
		out = append(out, rec)
	}
	return out
}

// Get returns one active batch.
func (r *Registry) Get(id model.BatchID) (model.BatchRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.active[id]
	return rec, ok
}

// Completed returns recent completed batches copy.
func (r *Registry) Completed() []model.BatchRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.BatchRecord, len(r.completed))
	copy(out, r.completed)
	return out
}
