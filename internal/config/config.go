// Package config loads Biofilter filter backwash runtime settings.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds Biofilter filter control parameters.
type Config struct {
	FilterID           string        `json:"filterId"`
	ListenAddr          string        `json:"listenAddr"`
	StaticDir           string        `json:"staticDir"`
	DefaultSetpointC    float64       `json:"defaultSetpointC"`
	BackwashBandC           float64       `json:"backwashBandC"`
	MinBackwashDuration     time.Duration `json:"minBackwashDuration"`
	SensorStaleAfter    time.Duration `json:"sensorStaleAfter"`
	ChamberLeaseTTL     time.Duration `json:"chamberLeaseTtl"`
	HeaterMaxKW         float64       `json:"heaterMaxKW"`
	VacuumTargetPa      float64       `json:"vacuumTargetPa"`
	VacuumLeakLimit     float64       `json:"vacuumLeakLimitPaPerS"`
	GasSetpointMPa      float64       `json:"gasSetpointMpa"`
	GasMaxMPa           float64       `json:"gasMaxMpa"`
	ZoneIDs             []string      `json:"zoneIds"`
	SensorPerZone       int           `json:"sensorPerZone"`
	TelemetryBuffer     int           `json:"telemetryBuffer"`
	AlarmCapacity       int           `json:"alarmCapacity"`
	InterlockStrict     bool          `json:"interlockStrict"`
	JournalPath         string        `json:"journalPath"`
	RampRateCPerMin     float64       `json:"rampRateCPerMin"`
	BlowerRateCPerMin   float64       `json:"blowerRateCPerMin"`
	UniformityLimitC    float64       `json:"uniformityLimitC"`
	OverTempLimitC      float64       `json:"overTempLimitC"`
}

// Default returns production-safe Biofilter defaults for a single filter.
func Default() Config {
	return Config{
		FilterID:         "biofilter-01",
		ListenAddr:        ":8090",
		StaticDir:         "web",
		DefaultSetpointC:  1180,
		BackwashBandC:         5,
		MinBackwashDuration:   2 * time.Hour,
		SensorStaleAfter:  5 * time.Second,
		ChamberLeaseTTL:   45 * time.Second,
		HeaterMaxKW:       120,
		VacuumTargetPa:    0.1,
		VacuumLeakLimit:   0.05,
		GasSetpointMPa:    100,
		GasMaxMPa:         120,
		ZoneIDs:           []string{"crown", "left", "right", "floor", "center"},
		SensorPerZone:     4,
		TelemetryBuffer:   4096,
		AlarmCapacity:     256,
		InterlockStrict:   true,
		JournalPath:       "aerofilt.journal",
		RampRateCPerMin:   8,
		BlowerRateCPerMin: 15,
		UniformityLimitC:  8,
		OverTempLimitC:    1250,
	}
}

// Validate checks configuration bounds before filter start.
func (c Config) Validate() error {
	if c.FilterID == "" {
		return fmt.Errorf("filterId required")
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("listenAddr required")
	}
	if c.DefaultSetpointC <= 0 || c.DefaultSetpointC > 2000 {
		return fmt.Errorf("defaultSetpointC out of range")
	}
	if c.BackwashBandC < 0 {
		return fmt.Errorf("backwashBandC must be >= 0")
	}
	if c.MinBackwashDuration <= 0 {
		return fmt.Errorf("minBackwashDuration must be positive")
	}
	if c.SensorStaleAfter <= 0 {
		return fmt.Errorf("sensorStaleAfter must be positive")
	}
	if c.ChamberLeaseTTL <= 0 {
		return fmt.Errorf("chamberLeaseTtl must be positive")
	}
	if c.HeaterMaxKW <= 0 {
		return fmt.Errorf("heaterMaxKW must be positive")
	}
	if c.VacuumTargetPa < 0 {
		return fmt.Errorf("vacuumTargetPa must be >= 0")
	}
	if c.GasSetpointMPa <= 0 || c.GasSetpointMPa > c.GasMaxMPa {
		return fmt.Errorf("gas pressure band invalid")
	}
	if len(c.ZoneIDs) == 0 {
		return fmt.Errorf("at least one zone required")
	}
	if c.SensorPerZone < 1 {
		return fmt.Errorf("sensorPerZone must be >= 1")
	}
	if c.TelemetryBuffer < 64 {
		return fmt.Errorf("telemetryBuffer too small")
	}
	if c.AlarmCapacity < 16 {
		return fmt.Errorf("alarmCapacity too small")
	}
	if c.RampRateCPerMin <= 0 {
		return fmt.Errorf("rampRateCPerMin must be positive")
	}
	if c.OverTempLimitC <= c.DefaultSetpointC {
		return fmt.Errorf("overTempLimitC must exceed default setpoint")
	}
	return nil
}

// LoadJSON reads config from a JSON file; missing file yields Default.
func LoadJSON(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	stale := durationOr(raw, "sensorStaleAfter", cfg.SensorStaleAfter)
	lease := durationOr(raw, "chamberLeaseTtl", cfg.ChamberLeaseTTL)
	backwash := durationOr(raw, "minBackwashDuration", cfg.MinBackwashDuration)
	delete(raw, "sensorStaleAfter")
	delete(raw, "chamberLeaseTtl")
	delete(raw, "minBackwashDuration")
	trimmed, err := json.Marshal(raw)
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(trimmed, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	cfg.SensorStaleAfter = stale
	cfg.ChamberLeaseTTL = lease
	cfg.MinBackwashDuration = backwash
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func durationOr(raw map[string]json.RawMessage, key string, fallback time.Duration) time.Duration {
	v, ok := raw[key]
	if !ok {
		return fallback
	}
	var asStr string
	if err := json.Unmarshal(v, &asStr); err == nil {
		d, err := time.ParseDuration(asStr)
		if err == nil {
			return d
		}
	}
	var asNs int64
	if err := json.Unmarshal(v, &asNs); err == nil && asNs > 0 {
		return time.Duration(asNs)
	}
	return fallback
}

// ZoneList returns configured zone identifiers.
func (c Config) ZoneList() []string {
	out := make([]string, len(c.ZoneIDs))
	copy(out, c.ZoneIDs)
	return out
}

// ClampHeaterKW clamps requested heater power to plant max.
func (c Config) ClampHeaterKW(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > c.HeaterMaxKW {
		return c.HeaterMaxKW
	}
	return v
}

// BackwashWindowSpec builds a backwash window from config defaults.
func (c Config) BackwashWindowSpec() (target, band float64, minDur time.Duration) {
	return c.DefaultSetpointC, c.BackwashBandC, c.MinBackwashDuration
}
