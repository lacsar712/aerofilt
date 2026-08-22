package telemetry

import (
	"sort"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Series groups telemetry points by metric name.
type Series struct {
	Metric string
	Points []model.TelemetryPoint
}

// GroupByMetric splits a slice into per-metric series sorted by time.
func GroupByMetric(points []model.TelemetryPoint) []Series {
	byMetric := make(map[string][]model.TelemetryPoint)
	for _, p := range points {
		byMetric[p.Metric] = append(byMetric[p.Metric], p)
	}
	out := make([]Series, 0, len(byMetric))
	for m, pts := range byMetric {
		sort.Slice(pts, func(i, j int) bool { return pts[i].At.Before(pts[j].At) })
		out = append(out, Series{Metric: m, Points: pts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metric < out[j].Metric })
	return out
}

// LastValue returns the most recent value for a metric.
func LastValue(points []model.TelemetryPoint, metric string) (float64, time.Time, bool) {
	var last model.TelemetryPoint
	found := false
	for _, p := range points {
		if p.Metric == metric && (!found || p.At.After(last.At)) {
			last = p
			found = true
		}
	}
	if !found {
		return 0, time.Time{}, false
	}
	return last.Value, last.At, true
}

// Average computes mean value for a metric over the slice.
func Average(points []model.TelemetryPoint, metric string) float64 {
	var sum float64
	var n int
	for _, p := range points {
		if p.Metric == metric {
			sum += p.Value
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
