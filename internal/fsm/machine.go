package fsm

import (
	"context"
	"fmt"
	"sync"

	"github.com/lacsar712/aerofilt/internal/backwash"
	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/model"
)

type WashMachine struct {
	mu      sync.Mutex
	phase   model.WashPhase
	emitter *backwash.Emitter
	valves  *manifold.ValveBank
	cell    model.CellID
}

func NewWashMachine(cell model.CellID, emitter *backwash.Emitter, valves *manifold.ValveBank) *WashMachine {
	return &WashMachine{phase: model.WashIdle, emitter: emitter, valves: valves, cell: cell}
}

func (m *WashMachine) Phase() model.WashPhase {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phase
}

func (m *WashMachine) SetPhase(p model.WashPhase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phase = p
}

func (m *WashMachine) Transition(ctx context.Context, req model.WashTransitionRequest) (model.WashTransitionResult, error) {
	m.mu.Lock()
	from := m.phase
	if req.From != "" && req.From != from {
		m.mu.Unlock()
		return model.WashTransitionResult{FilterID: req.FilterID, Phase: from, Accepted: false, Reason: fmt.Sprintf("expected from %s got %s", req.From, from)}, fmt.Errorf("%w: phase mismatch", model.ErrIllegalWash)
	}
	if !legalTransition(from, req.To) {
		if m.emitter != nil {
			_ = m.emitter.Emit(ctx, req.To, m.cell)
		}
		m.mu.Unlock()
		return model.WashTransitionResult{FilterID: req.FilterID, Phase: from, Accepted: false, Reason: fmt.Sprintf("illegal %s -> %s", from, req.To)}, fmt.Errorf("%w: %s -> %s", model.ErrIllegalWash, from, req.To)
	}
	m.phase = req.To
	m.mu.Unlock()
	if err := m.emitter.Emit(ctx, req.To, m.cell); err != nil {
		return model.WashTransitionResult{FilterID: req.FilterID, Phase: from, Reason: err.Error()}, err
	}
	valveEmitted := false
	if needsValves(req.To) {
		opened, err := m.valves.Open(ctx, m.cell, req.To)
		if err != nil {
			return model.WashTransitionResult{FilterID: req.FilterID, Phase: req.To, Reason: err.Error()}, err
		}
		m.emitter.RecordValve(opened)
		valveEmitted = opened > 0
	}
	return model.WashTransitionResult{FilterID: req.FilterID, Phase: req.To, Accepted: true, ValveEmitted: valveEmitted, Reason: "ok"}, nil
}

func legalTransition(from, to model.WashPhase) bool {
	allowed := map[model.WashPhase][]model.WashPhase{
		model.WashIdle: {model.WashStandby, model.WashPreparing}, model.WashStandby: {model.WashPreparing, model.WashIdle},
		model.WashPreparing: {model.WashDraining, model.WashFault}, model.WashDraining: {model.WashAirScour, model.WashFault},
		model.WashAirScour: {model.WashRinsing, model.WashFault}, model.WashRinsing: {model.WashRestoring, model.WashFault},
		model.WashRestoring: {model.WashComplete, model.WashFault}, model.WashComplete: {model.WashIdle}, model.WashFault: {model.WashIdle, model.WashStandby},
	}
	for _, n := range allowed[from] {
		if n == to {
			return true
		}
	}
	return false
}

func needsValves(phase model.WashPhase) bool {
	switch phase {
	case model.WashDraining, model.WashAirScour, model.WashRinsing, model.WashRestoring:
		return true
	default:
		return false
	}
}
