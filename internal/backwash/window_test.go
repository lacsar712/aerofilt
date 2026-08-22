package backwash

import (
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/clock"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestBackwashWindowProcessClock(t *testing.T) {
	proc := clock.NewProcessClock(time.Unix(0, 0).UTC())
	w := NewWindowTracker(proc, clock.RealClock{})
	w.Open(1180, 5, time.Hour)
	proc.Advance(30 * time.Minute)
	if w.ElapsedProcess() < 30*time.Minute {
		t.Fatalf("process elapsed %s", w.ElapsedProcess())
	}
	snap := model.ProfileSnapshot{
		Zones: map[model.ZoneID]model.ZoneTemp{
			"center": {ZoneID: "center", MeanC: 1180, SensorN: 1, SetpointC: 1180},
		},
	}
	ready, _ := w.Evaluate(snap)
	if ready {
		t.Fatal("backwash should not close before min duration")
	}
	proc.Advance(31 * time.Minute)
	ready, reason := w.Evaluate(snap)
	if !ready {
		t.Fatalf("backwash should close: %s", reason)
	}
}
