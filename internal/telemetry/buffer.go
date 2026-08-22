// Package telemetry buffers Biofilter filter time-series samples.
package telemetry

import (
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Buffer is a bounded ring of telemetry points.
type Buffer struct {
	mu     sync.RWMutex
	points []model.TelemetryPoint
	limit  int
}

// NewBuffer creates a telemetry buffer.
func NewBuffer(limit int) *Buffer {
	if limit < 64 {
		limit = 64
	}
	return &Buffer{points: make([]model.TelemetryPoint, 0, limit), limit: limit}
}

// Record appends one point.
func (b *Buffer) Record(metric string, value float64, tags []string, at time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	b.points = append(b.points, model.TelemetryPoint{Metric: metric, Value: value, Tags: tags, At: at})
	if len(b.points) > b.limit {
		b.points = b.points[len(b.points)-b.limit:]
	}
}

// RecordProfile stores fleet mean from a profile snapshot.
func (b *Buffer) RecordProfile(snap model.ProfileSnapshot) {
	var sum float64
	var n int
	for _, z := range snap.Zones {
		if z.SensorN > 0 {
			sum += z.MeanC
			n++
		}
	}
	if n == 0 {
		return
	}
	b.Record("fleet_mean_c", sum/float64(n), []string{string(snap.BatchID)}, snap.CapturedAt)
}

// Recent returns the last n points copy.
func (b *Buffer) Recent(n int) []model.TelemetryPoint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n <= 0 || n > len(b.points) {
		n = len(b.points)
	}
	start := len(b.points) - n
	out := make([]model.TelemetryPoint, n)
	copy(out, b.points[start:])
	return out
}

// Len returns current point count.
func (b *Buffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.points)
}

// Clear removes all points.
func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.points = b.points[:0]
}
