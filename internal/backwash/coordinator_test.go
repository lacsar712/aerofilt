package backwash_test

import (
	"context"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/backwash"
	"github.com/lacsar712/aerofilt/internal/clock"
	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestCoordinatorRunOpensValves(t *testing.T) {
	valves := manifold.NewValveBank(map[model.CellID][]model.ValveID{"cell-a": {"v1", "v2", "v3"}})
	coord := backwash.NewCoordinator(valves, backwash.NewEmitter(nil))
	res, err := coord.Run(context.Background(), model.BackwashRequest{OperationID: "op-1", CellID: "cell-a", FilterID: "f1", IssuedAt: time.Now().UTC()})
	if err != nil || !res.Accepted || res.ValvesOpen == 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestWindowProcessClock(t *testing.T) {
	proc := clock.NewProcessClock(time.Now().UTC())
	win := backwash.NewWindow(proc, time.Second, 1.6, 0.15)
	win.Open()
	proc.Advance(2 * time.Second)
	ok, _ := win.Evaluate(model.FilterSnapshot{Cells: []model.FilterCell{{ID: "c", HeadM: 1.6, Online: true}}})
	if !ok {
		t.Fatal("window should be satisfied")
	}
}
