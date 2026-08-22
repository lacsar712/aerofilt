package fsm

import (
	"context"
	"testing"

	"github.com/lacsar712/aerofilt/internal/aeration"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestIllegalTransitionNoSideEffect(t *testing.T) {
	em := heater.NewCountingEmitter(100)
	m := NewMachine("B-1", em)
	ctx := context.Background()
	_, err := m.Transition(ctx, model.BackwashTransitionRequest{
		BatchID: "B-1", From: model.BackwashIdle, To: model.BackwashHolding, Operator: "test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if em.HeaterCount() != 0 {
		t.Fatal("illegal transition emitted heater")
	}
	if em.VentCount() != 0 {
		t.Fatal("illegal transition emitted vent")
	}
}

func TestVentOnBlower(t *testing.T) {
	em := heater.NewCountingEmitter(100)
	m := NewMachine("B-2", em)
	ctx := context.Background()
	steps := []model.BackwashPhase{
		model.BackwashEvacuating, model.BackwashPressuring, model.BackwashRamping,
		model.BackwashHolding, model.BackwashBlowering,
	}
	from := model.BackwashIdle
	for _, to := range steps {
		res, err := m.Transition(ctx, model.BackwashTransitionRequest{
			BatchID: "B-2", From: from, To: to, Operator: "test",
		})
		if err != nil {
			t.Fatalf("%s->%s: %v", from, to, err)
		}
		from = res.Phase
	}
	if em.VentCount() < 1 {
		t.Fatal("expected vent emission on blower transition")
	}
}
