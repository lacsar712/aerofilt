// Package filter maintains concurrent temperature readings for Biofilter zones.
package filter

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// ErrOverTemp is raised when a zone mean exceeds the hard filter limit.
var ErrOverTemp = errors.New("filter over temperature limit")

// Book holds per-sensor latest samples keyed by zone then sensor.
type Book struct {
	mu       sync.RWMutex
	samples  map[model.ZoneID]map[model.SensorID]model.TempSample
	setpoint map[model.ZoneID]float64
	filter  string
	mode     model.FilterMode
	batch    model.BatchID
}

// NewBook creates an empty filter temperature book.
func NewBook(filterID string, zoneIDs []string, defaultSetpoint float64) *Book {
	b := &Book{
		samples:  make(map[model.ZoneID]map[model.SensorID]model.TempSample),
		setpoint: make(map[model.ZoneID]float64),
		filter:  filterID,
		mode:     model.FilterModeStandby,
	}
	for _, z := range zoneIDs {
		zid := model.ZoneID(z)
		b.samples[zid] = make(map[model.SensorID]model.TempSample)
		b.setpoint[zid] = defaultSetpoint
	}
	return b
}

// SetMode updates operating mode under write lock.
func (b *Book) SetMode(m model.FilterMode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mode = m
}

// Mode returns current filter mode.
func (b *Book) Mode() model.FilterMode {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.mode
}

// SetBatch assigns active batch id.
func (b *Book) SetBatch(id model.BatchID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batch = id
}

// Batch returns active batch id.
func (b *Book) Batch() model.BatchID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.batch
}

// SetSetpoint updates a zone temperature setpoint.
func (b *Book) SetSetpoint(zone model.ZoneID, c float64) error {
	if c <= 0 || c > 2200 {
		return fmt.Errorf("setpoint out of range: %.2f", c)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.samples[zone]; !ok {
		return fmt.Errorf("unknown zone %s", zone)
	}
	b.setpoint[zone] = c
	return nil
}

// UpdateSensor records a concurrent temperature sample from one mediacouple.
func (b *Book) UpdateSensor(sample model.TempSample) error {
	if err := model.ValidateTempSample(sample); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	zoneMap, ok := b.samples[sample.ZoneID]
	if !ok {
		zoneMap = make(map[model.SensorID]model.TempSample)
		b.samples[sample.ZoneID] = zoneMap
		if _, spOK := b.setpoint[sample.ZoneID]; !spOK {
			b.setpoint[sample.ZoneID] = sample.Celsius
		}
	}
	zoneMap[sample.SensorID] = sample
	return nil
}

// ZoneView aggregates sensors for one zone under read lock.
func (b *Book) ZoneView(zone model.ZoneID) (model.ZoneTemp, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.zoneViewLocked(zone)
}

func (b *Book) zoneViewLocked(zone model.ZoneID) (model.ZoneTemp, bool) {
	zoneMap, ok := b.samples[zone]
	if !ok {
		return model.ZoneTemp{}, false
	}
	list := make([]model.TempSample, 0, len(zoneMap))
	var latest time.Time
	for _, s := range zoneMap {
		list = append(list, s)
		if s.At.After(latest) {
			latest = s.At
		}
	}
	mean, minV, maxV, n := model.AggregateZoneMean(list)
	sp := b.setpoint[zone]
	return model.ZoneTemp{
		ZoneID:    zone,
		MeanC:     mean,
		MinC:      minV,
		MaxC:      maxV,
		SpreadC:   maxV - minV,
		SensorN:   n,
		SetpointC: sp,
		UpdatedAt: latest,
	}, true
}

// AllZones returns aggregated temperature for every known zone.
func (b *Book) AllZones() map[model.ZoneID]model.ZoneTemp {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make(map[model.ZoneID]model.ZoneTemp, len(b.samples))
	for z := range b.samples {
		if v, ok := b.zoneViewLocked(z); ok {
			out[z] = v
		}
	}
	return out
}

// Snapshot deep-copies zone aggregates so callers cannot pollute the book.
func (b *Book) Snapshot(now time.Time, phase model.BackwashPhase) model.ProfileSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	zones := make(map[model.ZoneID]model.ZoneTemp, len(b.samples))
	for z := range b.samples {
		if v, ok := b.zoneViewLocked(z); ok {
			zones[z] = v
		}
	}
	return model.ProfileSnapshot{
		Zones:      zones,
		CapturedAt: now,
		Mode:       b.mode,
		FilterID:  b.filter,
		BatchID:    b.batch,
		BackwashPhase:  phase,
	}
}

// SensorCount returns total mediacouples currently reporting.
func (b *Book) SensorCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, m := range b.samples {
		n += len(m)
	}
	return n
}

// DropStale removes samples older than maxAge relative to now.
func (b *Book) DropStale(now time.Time, maxAge time.Duration) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	removed := 0
	for z, m := range b.samples {
		for sid, s := range m {
			if model.StaleSample(s, now, maxAge) {
				delete(m, sid)
				removed++
			}
		}
		b.samples[z] = m
	}
	return removed
}

// Uniform reports whether every zone is within uniformity band of mean fleet temp.
func (b *Book) Uniform(limitC float64) (bool, []model.ZoneID) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.samples) == 0 {
		return false, nil
	}
	var sum float64
	var n int
	for z := range b.samples {
		v, ok := b.zoneViewLocked(z)
		if ok && v.SensorN > 0 {
			sum += v.MeanC
			n++
		}
	}
	if n == 0 {
		return false, nil
	}
	fleetMean := sum / float64(n)
	var bad []model.ZoneID
	for z := range b.samples {
		v, ok := b.zoneViewLocked(z)
		if !ok || v.SensorN == 0 {
			bad = append(bad, z)
			continue
		}
		if abs(v.MeanC-fleetMean) > limitC {
			bad = append(bad, z)
		}
	}
	return len(bad) == 0, bad
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// CheckOverTemp returns ErrOverTemp wrapped when any zone mean exceeds limit.
func (b *Book) CheckOverTemp(limitC float64) error {
	if limitC <= 0 {
		return fmt.Errorf("over-temp limit must be positive")
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for z := range b.samples {
		v, ok := b.zoneViewLocked(z)
		if !ok || v.SensorN == 0 {
			continue
		}
		if v.MeanC > limitC {
			return fmt.Errorf("zone %s mean %.1f > limit %.1f: %w", z, v.MeanC, limitC, ErrOverTemp)
		}
	}
	return nil
}

// Balanced reports whether every zone is within backwash band of its setpoint.
func (b *Book) Balanced(bandC float64) (bool, []model.ZoneID) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var bad []model.ZoneID
	for z := range b.samples {
		v, ok := b.zoneViewLocked(z)
		if !ok || v.SensorN == 0 {
			bad = append(bad, z)
			continue
		}
		if !model.WithinBand(v, bandC) {
			bad = append(bad, z)
		}
	}
	return len(bad) == 0, bad
}
