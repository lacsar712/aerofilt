package fsm_test

import (
	"context"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/backwash"
	"github.com/lacsar712/aerofilt/internal/fsm"
	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestWashMachineTransition(t *testing.T) {
	valves := manifold.NewValveBank(map[model.CellID][]model.ValveID{"cell-a": {"v1", "v2", "v3"}})
	em := backwash.NewEmitter(nil)
	m := fsm.NewWashMachine("cell-a", em, valves)
	m.SetPhase(model.WashPreparing)
	res, err := m.Transition(context.Background(), model.WashTransitionRequest{FilterID: "f1", From: model.WashPreparing, To: model.WashDraining, At: time.Now().UTC()})
	if err != nil || !res.Accepted || !res.ValveEmitted {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestIllegalTransition(t *testing.T) {
	valves := manifold.NewValveBank(map[model.CellID][]model.ValveID{"cell-a": {"v1"}})
	m := fsm.NewWashMachine("cell-a", backwash.NewEmitter(nil), valves)
	_, err := m.Transition(context.Background(), model.WashTransitionRequest{FilterID: "f1", To: model.WashRinsing})
	if err == nil {
		t.Fatal("expected illegal transition")
	}
}