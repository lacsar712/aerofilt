package filter

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
	"github.com/lacsar712/aerofilt/internal/level"
)

func TestGuardOverTempChain(t *testing.T) {
	book := NewBook("f1", []string{"center"}, 1180)
	g := NewGuard(book, 1200)
	sample := model.TempSample{
		ZoneID: "center", SensorID: "t1", Celsius: 1250, Quality: 1,
		At: time.Now().UTC(), Source: "test",
	}
	_ = book.UpdateSensor(sample)
	err := g.Evaluate()
	if err == nil {
		t.Fatal("expected over temp")
	}
	if !errors.Is(err, ErrOverTemp) {
		t.Fatalf("errors.Is ErrOverTemp failed: %v procedure=%s", err, Procedure(err))
	}
	wrapped := fmt.Errorf("outer: %w", err)
	if !errors.Is(wrapped, ErrOverTemp) {
		t.Fatal("double-wrap broke ErrOverTemp chain")
	}
}

func TestCoordinatorDeferUnlock(t *testing.T) {
	book := NewBook("f1", []string{"center"}, 1180)
	coord := NewCoordinator(time.Second, book, nil)
	if err := coord.RunCycle(context.Background(), "c1", "op", 0.1); err != nil {
		t.Fatal(err)
	}
	if coord.TryBusy() {
		t.Fatal("lease should be released after cycle")
	}
}

func TestCoordinatorUnlockOnVacuumFail(t *testing.T) {
	book := NewBook("f1", []string{"center"}, 1180)
	pump := vacuum.NewPump()
	pump.SetOnline(false)
	coord := NewCoordinator(time.Second, book, pump)
	if err := coord.RunCycle(context.Background(), "c-fail", "op", 0.1); err == nil {
		t.Fatal("expected vacuum precheck failure")
	}
	done := make(chan error, 1)
	go func() {
		pump.SetOnline(true)
		done <- coord.RunCycle(context.Background(), "c-ok", "op", 0.1)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second cycle after failed precheck: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("chamber lock not released after failed cycle; subsequent start blocked")
	}
}
