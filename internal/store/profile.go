// Package store persists temperature profile maps and exposes deep-copy snapshots.
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// ProfileStore is the cross-request source of truth for zone temperature aggregates.
type ProfileStore struct {
	mu        sync.RWMutex
	filterID string
	mode      model.FilterMode
	backwashPhase model.BackwashPhase
	batchID   model.BatchID
	zones     map[model.ZoneID]model.ZoneTemp
	meta      map[string]string
}

// NewProfileStore allocates an empty store for a filter.
func NewProfileStore(filterID string) *ProfileStore {
	return &ProfileStore{
		filterID: filterID,
		mode:      model.FilterModeStandby,
		backwashPhase: model.BackwashIdle,
		zones:     make(map[model.ZoneID]model.ZoneTemp),
		meta:      make(map[string]string),
	}
}

// UpsertZone replaces a zone aggregate under write lock.
func (s *ProfileStore) UpsertZone(z model.ZoneTemp) error {
	if z.ZoneID == "" {
		return fmt.Errorf("zone id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zones[z.ZoneID] = z
	return nil
}

// ReplaceAll atomically replaces the entire zone map from a filter book view.
func (s *ProfileStore) ReplaceAll(zones map[model.ZoneID]model.ZoneTemp, mode model.FilterMode, phase model.BackwashPhase, batch model.BatchID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zones = model.CloneZoneMap(zones)
	s.mode = mode
	s.backwashPhase = phase
	s.batchID = batch
}

// GetZone returns one zone copy.
func (s *ProfileStore) GetZone(id model.ZoneID) (model.ZoneTemp, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	z, ok := s.zones[id]
	return z, ok
}

// Snapshot returns a deep-copied profile snapshot safe for concurrent HTTP handlers.
func (s *ProfileStore) Snapshot(now time.Time) model.ProfileSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return model.ProfileSnapshot{
		Zones:      model.CloneZoneMap(s.zones),
		CapturedAt: now,
		Mode:       s.mode,
		FilterID:  s.filterID,
		BatchID:    s.batchID,
		BackwashPhase:  s.backwashPhase,
	}
}

// CloneSnapshot returns an independent copy of the latest snapshot.
func (s *ProfileStore) CloneSnapshot(now time.Time) model.ProfileSnapshot {
	return model.CloneProfileSnapshot(s.Snapshot(now))
}

// SetMeta stores operator annotations without touching temperature maps.
func (s *ProfileStore) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[key] = value
}

// Meta returns a copy of annotation map.
func (s *ProfileStore) Meta() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.meta))
	for k, v := range s.meta {
		out[k] = v
	}
	return out
}

// ZoneCount reports how many zones are currently tracked.
func (s *ProfileStore) ZoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.zones)
}

// Clear removes all zones (used on filter reset).
func (s *ProfileStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zones = make(map[model.ZoneID]model.ZoneTemp)
	s.mode = model.FilterModeStandby
	s.backwashPhase = model.BackwashIdle
	s.batchID = ""
}

// DiffSetpoints returns zones whose mean drifts beyond backwash band.
func (s *ProfileStore) DiffSetpoints(bandC float64) []model.ZoneID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.ZoneID
	for id, z := range s.zones {
		if !model.WithinBand(z, bandC) {
			out = append(out, id)
		}
	}
	return out
}

// FleetMean returns quality-weighted mean across all zones.
func (s *ProfileStore) FleetMean() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sum float64
	var n int
	for _, z := range s.zones {
		if z.SensorN > 0 {
			sum += z.MeanC
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
