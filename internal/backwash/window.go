// Package backwash tracks Biofilter backwash window closure using process clock.
package backwash

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/clock"
	"github.com/lacsar712/aerofilt/internal/model"
)

// WindowTracker monitors backwash hold duration against process time, not wall clock.
type WindowTracker struct {
	mu      sync.RWMutex
	proc    clock.Clock
	real    clock.Clock
	window  *model.BackwashWindow
	closed  bool
}

// NewWindowTracker binds process and real clocks.
func NewWindowTracker(proc clock.Clock, real clock.Clock) *WindowTracker {
	if proc == nil {
		proc = clock.RealClock{}
	}
	if real == nil {
		real = clock.RealClock{}
	}
	return &WindowTracker{proc: proc, real: real}
}

// Open starts a new backwash window at current process instant.
func (w *WindowTracker) Open(targetC, bandC float64, minDuration time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.proc.Now()
	w.window = &model.BackwashWindow{
		TargetC:      targetC,
		BandC:        bandC,
		MinDuration:  minDuration,
		StartedAt:    w.real.Now(),
		ProcessStart: now,
		Closed:       false,
	}
	w.closed = false
}

// ElapsedProcess returns backwash duration in process time.
func (w *WindowTracker) ElapsedProcess() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.window == nil {
		return 0
	}
	return w.proc.Since(w.window.ProcessStart)
}

// Snapshot returns a copy of the current backwash window.
func (w *WindowTracker) Snapshot() *model.BackwashWindow {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.window == nil {
		return nil
	}
	cp := *w.window
	return &cp
}

// Evaluate checks profile against backwash band and minimum process duration.
func (w *WindowTracker) Evaluate(snap model.ProfileSnapshot) (ready bool, reason string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.window == nil {
		return false, "no backwash window open"
	}
	if w.closed {
		return true, "already closed"
	}
	inBand, bad := model.AllZonesInBand(snap, w.window.TargetC, w.window.BandC)
	if !inBand {
		return false, fmt.Sprintf("zones out of band: %v", bad)
	}
	elapsed := w.proc.Since(w.window.ProcessStart)
	if elapsed < w.window.MinDuration {
		return false, fmt.Sprintf("process elapsed %s < min %s", elapsed, w.window.MinDuration)
	}
	return true, "backwash complete"
}

// Close marks the backwash window closed at current process instant.
func (w *WindowTracker) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.window == nil {
		return
	}
	w.window.Closed = true
	w.window.ClosedAt = w.proc.Now()
	w.closed = true
}

// Reset clears backwash window state.
func (w *WindowTracker) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.window = nil
	w.closed = false
}

// AdvanceProcess moves process clock forward (simulation / batch step).
func (w *WindowTracker) AdvanceProcess(d time.Duration) {
	if pc, ok := w.proc.(*clock.ProcessClock); ok {
		pc.Advance(d)
	}
}

// ProcessClock returns the bound process clock when available.
func (w *WindowTracker) ProcessClock() clock.Clock { return w.proc }
