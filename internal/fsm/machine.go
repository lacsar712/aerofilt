// Package fsm implements Biofilter backwash FSM with vent/heater side effects via emitter.
package fsm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/aeration"
	"github.com/lacsar712/aerofilt/internal/model"
)

var allowed = map[model.BackwashPhase]map[model.BackwashPhase]bool{
	model.BackwashIdle: {
		model.BackwashEvacuating: true,
		model.BackwashFault:      true,
	},
	model.BackwashEvacuating: {
		model.BackwashPressuring: true,
		model.BackwashFault:      true,
		model.BackwashIdle:       true,
	},
	model.BackwashPressuring: {
		model.BackwashRamping: true,
		model.BackwashFault:   true,
	},
	model.BackwashRamping: {
		model.BackwashHolding: true,
		model.BackwashFault:   true,
	},
	model.BackwashHolding: {
		model.BackwashBlowering: true,
		model.BackwashFault:     true,
	},
	model.BackwashBlowering: {
		model.BackwashComplete: true,
		model.BackwashFault:    true,
	},
	model.BackwashComplete: {
		model.BackwashIdle: true,
	},
	model.BackwashFault: {
		model.BackwashIdle: true,
	},
}

var heaterOnTransition = map[model.BackwashPhase]map[model.BackwashPhase]bool{
	model.BackwashPressuring: {model.BackwashRamping: true},
	model.BackwashRamping:    {model.BackwashHolding: true},
}

var ventOnTransition = map[model.BackwashPhase]map[model.BackwashPhase]bool{
	model.BackwashHolding:   {model.BackwashBlowering: true},
	model.BackwashBlowering: {model.BackwashComplete: true},
	model.BackwashFault:     {model.BackwashIdle: true},
}

// Machine is the backwash state machine for one Biofilter batch.
type Machine struct {
	mu       sync.Mutex
	batchID  model.BatchID
	phase    model.BackwashPhase
	emitter  heater.Emitter
	history  []string
	heaterKW float64
}

// NewMachine creates an idle backwash FSM.
func NewMachine(batchID model.BatchID, emitter heater.Emitter) *Machine {
	return &Machine{
		batchID:  batchID,
		phase:    model.BackwashIdle,
		emitter:  emitter,
		history:  nil,
		heaterKW: 80,
	}
}

// Phase returns current backwash phase.
func (m *Machine) Phase() model.BackwashPhase {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phase
}

// CanTransition reports legality without side effects.
func CanTransition(from, to model.BackwashPhase) bool {
	next, ok := allowed[from]
	if !ok {
		return false
	}
	return next[to]
}

// Transition attempts an FSM move. Illegal transitions must NOT emit heater/vent.
func (m *Machine) Transition(ctx context.Context, req model.BackwashTransitionRequest) (model.BackwashTransitionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.BatchID != "" && req.BatchID != m.batchID {
		return model.BackwashTransitionResult{
			BatchID: m.batchID, Phase: m.phase, Accepted: false, Reason: "batch id mismatch",
		}, fmt.Errorf("batch id mismatch")
	}
	from := m.phase
	to := req.To
	if req.From != "" && req.From != from {
		return model.BackwashTransitionResult{
			BatchID: m.batchID, Phase: m.phase, Accepted: false,
			Reason: fmt.Sprintf("expected from=%s have=%s", req.From, from),
		}, fmt.Errorf("from-phase mismatch")
	}
	if from == to {
		return model.BackwashTransitionResult{BatchID: m.batchID, Phase: m.phase, Accepted: true, Reason: "noop"}, nil
	}
	if !CanTransition(from, to) {
		m.history = append(m.history, fmt.Sprintf("reject %s->%s", from, to))
		return model.BackwashTransitionResult{
			BatchID: m.batchID, Phase: m.phase, Accepted: false,
			HeaterEmitted: false, VentEmitted: false,
			Reason: fmt.Sprintf("illegal transition %s -> %s", from, to),
		}, fmt.Errorf("illegal backwash transition %s -> %s", from, to)
	}

	heaterEmit := heaterOnTransition[from][to]
	ventEmit := ventOnTransition[from][to]
	emittedHeater := false
	emittedVent := false

	if heaterEmit && m.emitter != nil {
		cmd := model.HeaterCommand{
			CycleID:  model.CycleID(fmt.Sprintf("backwash-%s-%s", m.batchID, to)),
			ZoneID:   model.ZoneID("center"),
			PowerKW:  m.heaterKW,
			IssuedAt: time.Now().UTC(),
			Operator: req.Operator,
		}
		if cmd.Operator == "" {
			cmd.Operator = "backwash-fsm"
		}
		if err := m.emitter.EmitHeater(ctx, cmd); err != nil {
			return model.BackwashTransitionResult{
				BatchID: m.batchID, Phase: m.phase, Accepted: false, Reason: err.Error(),
			}, err
		}
		emittedHeater = true
	}

	if ventEmit && m.emitter != nil {
		vcmd := model.VentCommand{
			CycleID:  model.CycleID(fmt.Sprintf("vent-%s-%s", m.batchID, to)),
			Reason:   fmt.Sprintf("transition %s->%s", from, to),
			Duration: 2 * time.Second,
			IssuedAt: time.Now().UTC(),
			Operator: req.Operator,
		}
		if vcmd.Operator == "" {
			vcmd.Operator = "backwash-fsm"
		}
		if err := m.emitter.EmitVent(ctx, vcmd); err != nil {
			return model.BackwashTransitionResult{
				BatchID: m.batchID, Phase: m.phase, Accepted: false, Reason: err.Error(),
			}, err
		}
		emittedVent = true
	}

	m.phase = to
	m.history = append(m.history, fmt.Sprintf("%s->%s heater=%v vent=%v", from, to, emittedHeater, emittedVent))
	return model.BackwashTransitionResult{
		BatchID:       m.batchID,
		Phase:         m.phase,
		Accepted:      true,
		HeaterEmitted: emittedHeater,
		VentEmitted:   emittedVent,
		Reason:        "ok",
	}, nil
}

// History returns transition log copy.
func (m *Machine) History() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.history))
	copy(out, m.history)
	return out
}

// Reset forces idle after fault clear.
func (m *Machine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phase = model.BackwashIdle
}

// SetBatchID replaces active batch after complete.
func (m *Machine) SetBatchID(id model.BatchID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchID = id
	m.phase = model.BackwashIdle
}

// SetHeaterKW configures default heater power for side effects.
func (m *Machine) SetHeaterKW(kw float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heaterKW = kw
}
