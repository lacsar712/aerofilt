package manifold

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type ValveBank struct {
	mu     sync.Mutex
	valves map[model.ValveID]model.ValveState
	byCell map[model.CellID][]model.ValveID
	clock  func() time.Time
}

func NewValveBank(specs map[model.CellID][]model.ValveID) *ValveBank {
	valves := make(map[model.ValveID]model.ValveState)
	byCell := make(map[model.CellID][]model.ValveID, len(specs))
	for cell, ids := range specs {
		cp := make([]model.ValveID, len(ids))
		copy(cp, ids)
		byCell[cell] = cp
		for _, id := range ids {
			valves[id] = model.ValveState{ID: id}
		}
	}
	return &ValveBank{valves: valves, byCell: byCell, clock: func() time.Time { return time.Now().UTC() }}
}

func (v *ValveBank) Open(ctx context.Context, cell model.CellID, phase model.WashPhase) (int, error) {
	ids, ok := v.byCell[cell]
	if !ok {
		return 0, fmt.Errorf("unknown cell %s", cell)
	}
	target := valvesForPhase(phase, ids)
	if len(target) == 0 {
		return 0, fmt.Errorf("no valves for phase %s", phase)
	}
	now := v.clock()
	v.mu.Lock()
	defer v.mu.Unlock()
	opened := 0
	for _, id := range target {
		st := v.valves[id]
		st.Open = true
		st.OpenPct = openPctForPhase(phase)
		st.At = now
		v.valves[id] = st
		opened++
	}
	return opened, nil
}

func (v *ValveBank) Close(cell model.CellID) int {
	ids := v.byCell[cell]
	now := v.clock()
	v.mu.Lock()
	defer v.mu.Unlock()
	closed := 0
	for _, id := range ids {
		st, ok := v.valves[id]
		if !ok {
			continue
		}
		if st.Open {
			closed++
		}
		st.Open = false
		st.OpenPct = 0
		st.At = now
		v.valves[id] = st
	}
	return closed
}

func (v *ValveBank) States() []model.ValveState {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]model.ValveState, 0, len(v.valves))
	for _, st := range v.valves {
		out = append(out, st)
	}
	return out
}

func (v *ValveBank) OpenCount(cell model.CellID) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	n := 0
	for _, id := range v.byCell[cell] {
		if v.valves[id].Open {
			n++
		}
	}
	return n
}

func valvesForPhase(phase model.WashPhase, ids []model.ValveID) []model.ValveID {
	if len(ids) == 0 {
		return nil
	}
	switch phase {
	case model.WashDraining:
		return []model.ValveID{ids[0]}
	case model.WashAirScour:
		if len(ids) > 1 {
			return []model.ValveID{ids[1]}
		}
	case model.WashRinsing:
		if len(ids) > 2 {
			return []model.ValveID{ids[2]}
		}
		if len(ids) > 1 {
			return []model.ValveID{ids[1]}
		}
	case model.WashRestoring:
		return ids
	}
	return nil
}

func openPctForPhase(phase model.WashPhase) float64 {
	switch phase {
	case model.WashDraining:
		return 100
	case model.WashAirScour:
		return 85
	case model.WashRinsing:
		return 70
	case model.WashRestoring:
		return 50
	default:
		return 0
	}
}
