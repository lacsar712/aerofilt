package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/app"
	"github.com/lacsar712/aerofilt/internal/blower"
	"github.com/lacsar712/aerofilt/internal/config"
	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestRequestBackwashFlow(t *testing.T) {
	application, err := app.New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	_ = application.SeedHead(time.Now().UTC())
	res, err := application.RequestBackwash(model.BackwashRequest{OperationID: "bw-1", FilterID: "filter-1", CellID: "cell-a", Operator: "op", IssuedAt: time.Now().UTC()})
	if err != nil || !res.Accepted || res.ValvesOpen == 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestRequestBackwashEStop(t *testing.T) {
	application, err := app.New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	_ = application.SeedHead(time.Now().UTC())
	_ = application.EmergencyStop()
	_, err = application.RequestBackwash(model.BackwashRequest{OperationID: "bw-2", FilterID: "filter-1", CellID: "cell-a"})
	if !errors.Is(err, model.ErrPlantEStop) {
		t.Fatalf("expected estop, got %v", err)
	}
}

func TestWashWindowClose(t *testing.T) {
	application, err := app.New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	_ = application.SeedHead(time.Now().UTC())
	_, _ = application.RequestBackwash(model.BackwashRequest{OperationID: "bw-3", FilterID: "filter-1", CellID: "cell-a"})
	application.AdvanceProcess(application.Config().WashCloseWindow + time.Second)
	ok, reason := application.EvaluateWashWindow()
	if !ok {
		t.Fatalf("window not ready: %s", reason)
	}
}

func TestTransitionWash(t *testing.T) {
	application, err := app.New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	_ = application.SeedHead(time.Now().UTC())
	application.Machine("cell-a").SetPhase(model.WashPreparing)
	res, err := application.TransitionWash(model.WashTransitionRequest{FilterID: "filter-1", From: model.WashPreparing, To: model.WashDraining, At: time.Now().UTC()})
	if err != nil || !res.Accepted {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestCrossPackageErrors(t *testing.T) {
	application, err := app.New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	err = application.IngestHead(model.HeadSample{CellID: "cell-a", Meters: 0.1, Quality: 0.9, At: time.Now().UTC()})
	if !level.IsLow(err) {
		t.Fatalf("expected low head: %v", err)
	}
	application.Blower().Trip("test")
	_, err = application.RequestBackwash(model.BackwashRequest{OperationID: "bw-4", FilterID: "filter-1", CellID: "cell-b"})
	if err == nil || !blower.IsTripped(err) {
		t.Fatalf("expected blower tripped: %v", err)
	}
	probe := media.NewProbe(0.85, 0.35, 0.55, 1.0)
	err = probe.Evaluate(model.MediaProfile{FilterID: "filter-1", BedDepthM: 1.8, VoidRatio: 0.42, ClogIndex: 0.99})
	if !media.IsClogged(err) {
		t.Fatalf("expected clogged: %v", err)
	}
}

func TestSnapshotIsolation(t *testing.T) {
	application, err := app.New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	snap := application.Store().Snapshot(time.Now().UTC())
	snap.Cells[0].HeadM = 99
	if application.Store().Snapshot(time.Now().UTC()).Cells[0].HeadM == 99 {
		t.Fatal("store snapshot not isolated")
	}
}
