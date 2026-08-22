package backwash

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/clock"
	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/model"
)

type Emitter struct {
	mu         sync.Mutex
	valveCount int64
	events     []string
	onEmit     func(ctx context.Context, phase model.WashPhase, cell model.CellID) error
}

func NewEmitter(hook func(context.Context, model.WashPhase, model.CellID) error) *Emitter {
	return &Emitter{onEmit: hook}
}

func (e *Emitter) Emit(ctx context.Context, phase model.WashPhase, cell model.CellID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	e.events = append(e.events, string(phase)+":"+string(cell))
	e.mu.Unlock()
	if e.onEmit != nil {
		return e.onEmit(ctx, phase, cell)
	}
	return nil
}

func (e *Emitter) ValveCount() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.valveCount
}

func (e *Emitter) RecordValve(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.valveCount += int64(n)
}

func (e *Emitter) Events() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.events))
	copy(out, e.events)
	return out
}

type Coordinator struct {
	valves  *manifold.ValveBank
	emitter *Emitter
	phase   model.WashPhase
	mu      sync.Mutex
	opMu    sync.Mutex
}

func NewCoordinator(valves *manifold.ValveBank, emitter *Emitter) *Coordinator {
	return &Coordinator{valves: valves, emitter: emitter, phase: model.WashIdle}
}

func (c *Coordinator) Run(ctx context.Context, req model.BackwashRequest) (model.BackwashResult, error) {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	if req.OperationID == "" {
		return model.BackwashResult{}, fmt.Errorf("operation id required")
	}
	if req.CellID == "" {
		return model.BackwashResult{}, fmt.Errorf("cell id required")
	}
	phase := model.WashPreparing
	c.mu.Lock()
	c.phase = phase
	c.mu.Unlock()
	if err := c.emitter.Emit(ctx, phase, req.CellID); err != nil {
		return model.BackwashResult{}, err
	}
	drainPhase := model.WashDraining
	opened, err := c.valves.Open(ctx, req.CellID, drainPhase)
	if err != nil {
		return model.BackwashResult{}, err
	}
	c.emitter.RecordValve(opened)
	now := time.Now().UTC()
	res := model.BackwashResult{OperationID: req.OperationID, Phase: drainPhase, Accepted: true, ValvesOpen: opened, Message: "backwash coordinator started", CompletedAt: now}
	c.mu.Lock()
	c.phase = drainPhase
	c.mu.Unlock()
	return res, nil
}

// ApplyOpenSequence opens valves for each wash phase step, honouring cancellation between steps.
func (c *Coordinator) ApplyOpenSequence(ctx context.Context, cell model.CellID, phases []model.WashPhase) error {
	for i, phase := range phases {
		if i > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("open sequence cancelled at step %d: %w", i, ctx.Err())
			default:
			}
		}
		opened, err := c.valves.Open(ctx, cell, phase)
		if err != nil {
			return err
		}
		c.emitter.RecordValve(opened)
	}
	return nil
}

func (c *Coordinator) Phase() model.WashPhase {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.phase
}

type Window struct {
	mu      sync.RWMutex
	proc    *clock.ProcessClock
	window  time.Duration
	started time.Time
	open    bool
	targetM float64
	bandM   float64
}

func NewWindow(proc *clock.ProcessClock, closeWindow time.Duration, targetM, bandM float64) *Window {
	return &Window{proc: proc, window: closeWindow, targetM: targetM, bandM: bandM}
}

func (w *Window) Open() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.started = w.proc.Now()
	w.open = true
}

func (w *Window) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open = false
}

func (w *Window) Evaluate(snap model.FilterSnapshot) (bool, string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.open {
		return false, "window not open"
	}
	elapsed := w.proc.Since(w.started)
	if elapsed < w.window {
		return false, fmt.Sprintf("process elapsed %s < window %s", elapsed, w.window)
	}
	ok, bad := model.CellsInHeadBand(snap.Cells, w.targetM, w.bandM)
	if !ok {
		return false, fmt.Sprintf("cells out of band: %v", bad)
	}
	return true, "wash window satisfied"
}

func (w *Window) Elapsed() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.open {
		return 0
	}
	return w.proc.Since(w.started)
}

func (w *Window) IsOpen() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.open
}
