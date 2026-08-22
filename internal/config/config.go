package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type Config struct {
	PlantID         model.PlantID `json:"plant_id"`
	ListenAddr      string        `json:"listen_addr"`
	WashCloseWindow time.Duration `json:"wash_close_window"`
	MinHeadM        float64       `json:"min_head_m"`
	MaxHeadM        float64       `json:"max_head_m"`
	BackwashBandM   float64       `json:"backwash_band_m"`
	TargetHeadM     float64       `json:"target_head_m"`
	DefaultAirCMS   float64       `json:"default_air_cms"`
	DefaultWaterCMS float64       `json:"default_water_cms"`
	MaxClogIndex    float64       `json:"max_clog_index"`
	BlowerMaxPct    float64       `json:"blower_max_pct"`
	LeaseTTL        time.Duration `json:"lease_ttl"`
	AlarmCapacity   int           `json:"alarm_capacity"`
	TelemetryBuffer int           `json:"telemetry_buffer"`
	JournalCapacity int           `json:"journal_capacity"`
	Cells           []CellSpec    `json:"cells"`
}

type CellSpec struct {
	ID       model.CellID    `json:"id"`
	FilterID model.FilterID  `json:"filter_id"`
	Valves   []model.ValveID `json:"valves"`
}

func Default() Config {
	return Config{
		PlantID: "plant-north", ListenAddr: ":8080", WashCloseWindow: 45 * time.Second,
		MinHeadM: 0.8, MaxHeadM: 3.2, BackwashBandM: 0.15, TargetHeadM: 1.6,
		DefaultAirCMS: 12.0, DefaultWaterCMS: 8.0, MaxClogIndex: 0.85, BlowerMaxPct: 100,
		LeaseTTL: 2 * time.Minute, AlarmCapacity: 64, TelemetryBuffer: 512, JournalCapacity: 256,
		Cells: []CellSpec{
			{ID: "cell-a", FilterID: "filter-1", Valves: []model.ValveID{"v-drain-a", "v-air-a", "v-rinse-a"}},
			{ID: "cell-b", FilterID: "filter-1", Valves: []model.ValveID{"v-drain-b", "v-air-b", "v-rinse-b"}},
			{ID: "cell-c", FilterID: "filter-2", Valves: []model.ValveID{"v-drain-c", "v-air-c", "v-rinse-c"}},
		},
	}
}

func (c Config) Validate() error {
	if c.PlantID == "" {
		return fmt.Errorf("plant_id required")
	}
	if c.WashCloseWindow <= 0 {
		return fmt.Errorf("wash_close_window must be positive")
	}
	if c.MinHeadM >= c.MaxHeadM {
		return fmt.Errorf("min_head_m must be less than max_head_m")
	}
	if len(c.Cells) == 0 {
		return fmt.Errorf("at least one cell required")
	}
	return nil
}

func LoadJSON(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, cfg.Validate()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func (c Config) CellIDs() []model.CellID {
	out := make([]model.CellID, len(c.Cells))
	for i, spec := range c.Cells {
		out[i] = spec.ID
	}
	return out
}

func (c Config) ValvesFor(cell model.CellID) []model.ValveID {
	for _, spec := range c.Cells {
		if spec.ID == cell {
			cp := make([]model.ValveID, len(spec.Valves))
			copy(cp, spec.Valves)
			return cp
		}
	}
	return nil
}
