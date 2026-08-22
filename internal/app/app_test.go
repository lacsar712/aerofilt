package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/config"
	"github.com/lacsar712/aerofilt/internal/fsm"
	"github.com/lacsar712/aerofilt/internal/model"
	"github.com/lacsar712/aerofilt/internal/level"
)

func TestAppSeedAndStatus(t *testing.T) {
	cfg := config.Default()
	cfg.FilterID = "test-biofilter"
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	now := time.Now().UTC()
	if err := a.SeedSensors(now); err != nil {
		t.Fatal(err)
	}
	st := a.Status()
	if st.FilterID != "test-biofilter" {
		t.Fatalf("filter id %s", st.FilterID)
	}
	if a.Book().SensorCount() == 0 {
		t.Fatal("expected seeded sensors")
	}
}

func TestStartCycleInterlockEStop(t *testing.T) {
	cfg := config.Default()
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	_ = a.SeedSensors(time.Now().UTC())
	a.Gates().SetEStop(true)
	if err := a.StartCycle("B-001", "op"); err == nil {
		t.Fatal("expected interlock denial")
	}
}

func TestBackwashFSMIllegalNoEmit(t *testing.T) {
	cfg := config.Default()
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	a.backwashFSM.SetBatchID("B-002")
	before := a.EmitterHeaterCount()
	_, err = a.BackwashTransition(model.BackwashTransitionRequest{
		BatchID: "B-002", From: model.BackwashIdle, To: model.BackwashHolding, Operator: "test",
	})
	if err == nil {
		t.Fatal("expected illegal transition error")
	}
	if a.EmitterHeaterCount() != before {
		t.Fatal("illegal transition must not emit heater")
	}
}

func TestBackwashFSMLegalEmitHeater(t *testing.T) {
	cfg := config.Default()
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	a.backwashFSM.SetBatchID("B-003")
	ctx := context.Background()
	steps := []model.BackwashPhase{
		model.BackwashEvacuating, model.BackwashPressuring, model.BackwashRamping, model.BackwashHolding,
	}
	from := model.BackwashIdle
	for _, to := range steps {
		_, err := a.backwashFSM.Transition(ctx, model.BackwashTransitionRequest{
			BatchID: "B-003", From: from, To: to, Operator: "test",
		})
		if err != nil {
			t.Fatalf("transition %s->%s: %v", from, to, err)
		}
		from = to
	}
	if a.EmitterHeaterCount() < 1 {
		t.Fatal("expected heater emission on legal ramp transitions")
	}
}

func TestVacuumLeakChain(t *testing.T) {
	cfg := config.Default()
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	_ = a.SeedSensors(time.Now().UTC())
	a.vacuum.SetLeakRate(1.0)
	err = a.StartCycle("B-leak", "op")
	if err == nil {
		t.Fatal("expected vacuum leak error")
	}
	if !errors.Is(err, vacuum.VacuumLeak) {
		t.Fatalf("errors.Is VacuumLeak failed: %v", err)
	}
}

func TestProfileSnapshotCopySafe(t *testing.T) {
	cfg := config.Default()
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	_ = a.SeedSensors(time.Now().UTC())
	s1 := a.ProfileSnapshot()
	s2 := a.ProfileSnapshot()
	for id := range s1.Zones {
		s1.Zones[id] = model.ZoneTemp{ZoneID: id, MeanC: 9999}
		break
	}
	s3 := a.ProfileSnapshot()
	for id, z := range s3.Zones {
		if z.MeanC == 9999 {
			t.Fatalf("mutation leaked to store via zone %s", id)
		}
	}
	_ = s2
}

func TestCanTransition(t *testing.T) {
	if fsm.CanTransition(model.BackwashIdle, model.BackwashHolding) {
		t.Fatal("idle->holding should be illegal")
	}
	if !fsm.CanTransition(model.BackwashIdle, model.BackwashEvacuating) {
		t.Fatal("idle->evacuating should be legal")
	}
}
