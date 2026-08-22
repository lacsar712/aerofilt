package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// ValidateTempSample checks mediacouple payload sanity.
func ValidateTempSample(s TempSample) error {
	if s.ZoneID == "" {
		return fmt.Errorf("zone id required")
	}
	if s.SensorID == "" {
		return fmt.Errorf("sensor id required")
	}
	if s.Celsius < -50 || s.Celsius > 2200 {
		return fmt.Errorf("temperature out of transducer range: %.2f C", s.Celsius)
	}
	if s.Quality < 0 || s.Quality > 1 {
		return fmt.Errorf("quality must be in [0,1]")
	}
	if s.At.IsZero() {
		return fmt.Errorf("sample timestamp required")
	}
	return nil
}

// ValidateHeaterCommand checks heater power command bounds.
func ValidateHeaterCommand(c HeaterCommand) error {
	if c.CycleID == "" {
		return fmt.Errorf("cycle id required")
	}
	if c.ZoneID == "" {
		return fmt.Errorf("zone id required")
	}
	if c.PowerKW < 0 || c.PowerKW > 500 {
		return fmt.Errorf("power out of range: %.1f kW", c.PowerKW)
	}
	if strings.TrimSpace(c.Operator) == "" {
		return fmt.Errorf("operator required")
	}
	return nil
}

// ValidateVentCommand checks vent valve request.
func ValidateVentCommand(c VentCommand) error {
	if c.CycleID == "" {
		return fmt.Errorf("cycle id required")
	}
	if strings.TrimSpace(c.Operator) == "" {
		return fmt.Errorf("operator required")
	}
	return nil
}

// TempErrorC returns absolute deviation from setpoint.
func TempErrorC(zone ZoneTemp) float64 {
	return math.Abs(zone.MeanC - zone.SetpointC)
}

// WithinBand reports whether zone temperature is inside backwash band.
func WithinBand(zone ZoneTemp, bandC float64) bool {
	if bandC < 0 {
		bandC = 0
	}
	return TempErrorC(zone) <= bandC
}

// AggregateZoneMean computes a quality-weighted mean of samples.
func AggregateZoneMean(samples []TempSample) (mean, minV, maxV float64, n int) {
	if len(samples) == 0 {
		return 0, 0, 0, 0
	}
	minV = samples[0].Celsius
	maxV = samples[0].Celsius
	var wsum, w float64
	for _, s := range samples {
		q := s.Quality
		if q <= 0 {
			q = 0.01
		}
		wsum += s.Celsius * q
		w += q
		if s.Celsius < minV {
			minV = s.Celsius
		}
		if s.Celsius > maxV {
			maxV = s.Celsius
		}
		n++
	}
	if w == 0 {
		return 0, minV, maxV, n
	}
	return wsum / w, minV, maxV, n
}

// CloneZoneMap deep-copies a zone temperature map.
func CloneZoneMap(src map[ZoneID]ZoneTemp) map[ZoneID]ZoneTemp {
	if src == nil {
		return map[ZoneID]ZoneTemp{}
	}
	dst := make(map[ZoneID]ZoneTemp, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// CloneProfileSnapshot deep-copies a profile snapshot including zone map.
func CloneProfileSnapshot(src ProfileSnapshot) ProfileSnapshot {
	return ProfileSnapshot{
		Zones:      CloneZoneMap(src.Zones),
		CapturedAt: src.CapturedAt,
		Mode:       src.Mode,
		FilterID:  src.FilterID,
		BatchID:    src.BatchID,
		BackwashPhase:  src.BackwashPhase,
	}
}

// FilterModeActive reports whether thermal processing is live.
func FilterModeActive(m FilterMode) bool {
	return m == FilterModeRamp || m == FilterModeBackwash || m == FilterModeBlower
}

// Age reports duration since t relative to now.
func Age(t, now time.Time) time.Duration {
	if t.IsZero() || now.IsZero() {
		return 0
	}
	if now.Before(t) {
		return 0
	}
	return now.Sub(t)
}

// StaleSample reports whether a temperature sample exceeded maxAge.
func StaleSample(s TempSample, now time.Time, maxAge time.Duration) bool {
	if maxAge <= 0 {
		return false
	}
	return Age(s.At, now) > maxAge
}

// ZoneSpread computes max-min spread for uniformity checks.
func ZoneSpread(zone ZoneTemp) float64 {
	return zone.MaxC - zone.MinC
}

// AllZonesInBand checks every zone in snapshot against backwash band.
func AllZonesInBand(snap ProfileSnapshot, targetC, bandC float64) (bool, []ZoneID) {
	var bad []ZoneID
	for id, z := range snap.Zones {
		if z.SensorN == 0 {
			bad = append(bad, id)
			continue
		}
		z.SetpointC = targetC
		if !WithinBand(z, bandC) {
			bad = append(bad, id)
		}
	}
	return len(bad) == 0, bad
}

// FormatTemp renders °C for operator displays.
func FormatTemp(c float64) string {
	return fmt.Sprintf("%.1f °C", c)
}

// FormatPressureMPa renders MPa for operator displays.
func FormatPressureMPa(mpa float64) string {
	return fmt.Sprintf("%.2f MPa", mpa)
}
