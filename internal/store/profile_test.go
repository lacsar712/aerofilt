package store

import (
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

func TestSnapshotDeepCopy(t *testing.T) {
	s := NewProfileStore("biofilter-1")
	_ = s.UpsertZone(model.ZoneTemp{ZoneID: "center", MeanC: 800, SensorN: 2})
	snap := s.Snapshot(time.Now().UTC())
	snap.Zones["center"] = model.ZoneTemp{ZoneID: "center", MeanC: 0}
	snap2 := s.Snapshot(time.Now().UTC())
	if snap2.Zones["center"].MeanC == 0 {
		t.Fatal("snapshot mutation affected store")
	}
}

func TestCloneSnapshot(t *testing.T) {
	s := NewProfileStore("biofilter-1")
	_ = s.UpsertZone(model.ZoneTemp{ZoneID: "crown", MeanC: 900, SensorN: 1})
	clone := s.CloneSnapshot(time.Now().UTC())
	clone.Zones["crown"] = model.ZoneTemp{MeanC: 1}
	snap := s.Snapshot(time.Now().UTC())
	if snap.Zones["crown"].MeanC == 1 {
		t.Fatal("clone mutation affected store")
	}
}
