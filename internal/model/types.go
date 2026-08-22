// Package model holds shared Biofilter filter backwash domain types.
package model

import "time"

// ZoneID identifies a filter temperature zone (mediacouple group).
type ZoneID string

// SensorID identifies one mediacouple or RTD.
type SensorID string

// BatchID identifies a Biofilter load batch in the filter.
type BatchID string

// CycleID identifies one Biofilter thermal cycle run.
type CycleID string

// FilterMode is the operating mode of the Biofilter vessel controller.
type FilterMode string

const (
	FilterModeStandby  FilterMode = "standby"
	FilterModeEvacuate FilterMode = "evacuate"
	FilterModeRamp     FilterMode = "ramp"
	FilterModeBackwash     FilterMode = "backwash"
	FilterModeBlower   FilterMode = "blower"
	FilterModeHold     FilterMode = "hold"
	FilterModeEStop    FilterMode = "estop"
)

// BackwashPhase is the Biofilter backwash finite-state machine phase.
type BackwashPhase string

const (
	BackwashIdle       BackwashPhase = "idle"
	BackwashEvacuating BackwashPhase = "evacuating"
	BackwashPressuring BackwashPhase = "pressuring"
	BackwashRamping    BackwashPhase = "ramping"
	BackwashHolding    BackwashPhase = "holding"
	BackwashBlowering  BackwashPhase = "blowering"
	BackwashComplete   BackwashPhase = "complete"
	BackwashFault      BackwashPhase = "fault"
)

// HeaterPhase tracks resistance heater bank lifecycle.
type HeaterPhase string

const (
	HeaterOff     HeaterPhase = "off"
	HeaterWarming HeaterPhase = "warming"
	HeaterActive  HeaterPhase = "active"
	HeaterTrim    HeaterPhase = "trim"
	HeaterFault   HeaterPhase = "fault"
)

// AlarmSeverity classifies operator-facing alarms.
type AlarmSeverity string

const (
	SeverityInfo     AlarmSeverity = "info"
	SeverityWarn     AlarmSeverity = "warn"
	SeverityCritical AlarmSeverity = "critical"
)

// TempSample is one mediacouple reading in °C.
type TempSample struct {
	ZoneID   ZoneID    `json:"zoneId"`
	SensorID SensorID  `json:"sensorId"`
	Celsius  float64   `json:"celsius"`
	At       time.Time `json:"at"`
	Quality  float64   `json:"quality"`
	Source   string    `json:"source"`
}

// ZoneTemp aggregates sensors into a zone setpoint view.
type ZoneTemp struct {
	ZoneID      ZoneID    `json:"zoneId"`
	MeanC       float64   `json:"meanC"`
	MinC        float64   `json:"minC"`
	MaxC        float64   `json:"maxC"`
	SpreadC     float64   `json:"spreadC"`
	SensorN     int       `json:"sensorN"`
	SetpointC   float64   `json:"setpointC"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ProfileSnapshot is a deep-copied view of all zone temperatures.
type ProfileSnapshot struct {
	Zones      map[ZoneID]ZoneTemp `json:"zones"`
	CapturedAt time.Time           `json:"capturedAt"`
	Mode       FilterMode         `json:"mode"`
	FilterID  string              `json:"filterId"`
	BatchID    BatchID             `json:"batchId,omitempty"`
	BackwashPhase  BackwashPhase           `json:"backwashPhase"`
}

// HeaterCommand is a request to apply heater power to a zone group.
type HeaterCommand struct {
	CycleID   CycleID   `json:"cycleId"`
	ZoneID    ZoneID    `json:"zoneId"`
	PowerKW   float64   `json:"powerKW"`
	Duration  time.Duration `json:"duration"`
	IssuedAt  time.Time `json:"issuedAt"`
	Operator  string    `json:"operator"`
}

// HeaterResult records whether a heater side effect was emitted.
type HeaterResult struct {
	CycleID     CycleID     `json:"cycleId"`
	Phase       HeaterPhase `json:"phase"`
	AppliedKW   float64     `json:"appliedKW"`
	Emitted     bool        `json:"emitted"`
	Cancelled   bool        `json:"cancelled"`
	Message     string      `json:"message"`
	CompletedAt time.Time   `json:"completedAt"`
}

// VentCommand asks the vacuum vent valve to open for pressure relief.
type VentCommand struct {
	CycleID  CycleID   `json:"cycleId"`
	Reason   string    `json:"reason"`
	Duration time.Duration `json:"duration"`
	IssuedAt time.Time `json:"issuedAt"`
	Operator string    `json:"operator"`
}

// VentResult records vent valve actuation outcome.
type VentResult struct {
	CycleID     CycleID   `json:"cycleId"`
	Opened      bool      `json:"opened"`
	VentedPa    float64   `json:"ventedPa"`
	Message     string    `json:"message"`
	CompletedAt time.Time `json:"completedAt"`
}

// BackwashTransitionRequest asks the backwash FSM to advance a phase.
type BackwashTransitionRequest struct {
	BatchID  BatchID   `json:"batchId"`
	From     BackwashPhase `json:"from"`
	To       BackwashPhase `json:"to"`
	Operator string    `json:"operator"`
	At       time.Time `json:"at"`
}

// BackwashTransitionResult reports FSM outcome and actuator side effects.
type BackwashTransitionResult struct {
	BatchID       BatchID   `json:"batchId"`
	Phase         BackwashPhase `json:"phase"`
	Accepted      bool      `json:"accepted"`
	HeaterEmitted bool      `json:"heaterEmitted"`
	VentEmitted   bool      `json:"ventEmitted"`
	Reason        string    `json:"reason"`
}

// VacuumReading is absolute pressure in the vessel (Pa).
type VacuumReading struct {
	Pascals  float64   `json:"pascals"`
	At       time.Time `json:"at"`
	LeakRate float64   `json:"leakRatePaPerS"`
	Source   string    `json:"source"`
}

// GasPressureReading is argon isostatic pressure during Biofilter (MPa).
type GasPressureReading struct {
	MPa      float64   `json:"mpa"`
	At       time.Time `json:"at"`
	Setpoint float64   `json:"setpointMpa"`
	Source   string    `json:"source"`
}

// BackwashWindow defines the required hold band and duration for Biofilter backwash.
type BackwashWindow struct {
	TargetC      float64       `json:"targetC"`
	BandC        float64       `json:"bandC"`
	MinDuration  time.Duration `json:"minDuration"`
	StartedAt    time.Time     `json:"startedAt"`
	ProcessStart time.Time     `json:"processStart"`
	Closed       bool          `json:"closed"`
	ClosedAt     time.Time     `json:"closedAt,omitempty"`
}

// RampSegment is one leg of a programmed temperature ramp.
type RampSegment struct {
	FromC      float64       `json:"fromC"`
	ToC        float64       `json:"toC"`
	RateCPerMin float64      `json:"rateCPerMin"`
	HoldAt     time.Time     `json:"holdAt,omitempty"`
}

// RampProfile is an ordered Biofilter thermal ramp program.
type RampProfile struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Segments []RampSegment `json:"segments"`
	Backwash     BackwashWindow    `json:"backwash"`
}

// BatchRecord tracks one Biofilter load through the filter.
type BatchRecord struct {
	ID          BatchID     `json:"id"`
	Alloy       string      `json:"alloy"`
	PartCount   int         `json:"partCount"`
	ProfileID   string      `json:"profileId"`
	Phase       BackwashPhase   `json:"phase"`
	StartedAt   time.Time   `json:"startedAt"`
	CompletedAt *time.Time  `json:"completedAt,omitempty"`
	Operator    string      `json:"operator"`
}

// AlarmEvent is an operator console notification.
type AlarmEvent struct {
	ID        string        `json:"id"`
	Code      string        `json:"code"`
	Severity  AlarmSeverity `json:"severity"`
	Message   string        `json:"message"`
	FilterID string        `json:"filterId"`
	RaisedAt  time.Time     `json:"raisedAt"`
	ClearedAt *time.Time    `json:"clearedAt,omitempty"`
	Active    bool          `json:"active"`
}

// TelemetryPoint is a single time-series sample for dashboards.
type TelemetryPoint struct {
	Metric string    `json:"metric"`
	Value  float64   `json:"value"`
	Tags   []string  `json:"tags"`
	At     time.Time `json:"at"`
}

// FilterStatus is the consolidated operator status payload.
type FilterStatus struct {
	FilterID     string          `json:"filterId"`
	Mode          FilterMode     `json:"mode"`
	Profile       ProfileSnapshot `json:"profile"`
	BackwashPhase     BackwashPhase       `json:"backwashPhase"`
	HeaterPhase   HeaterPhase     `json:"heaterPhase"`
	VacuumPa      float64         `json:"vacuumPa"`
	GasMPa        float64         `json:"gasMpa"`
	BackwashWindow    *BackwashWindow     `json:"backwashWindow,omitempty"`
	InterlockOK   bool            `json:"interlockOk"`
	ActiveAlarms  int             `json:"activeAlarms"`
	ActiveBatch   BatchID         `json:"activeBatch,omitempty"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// InterlockDecision is the result of a pre-cycle safety evaluation.
type InterlockDecision struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons"`
	Code    string   `json:"code"`
}

// ChamberLease represents exclusive ownersbiofilter of the filter chamber path.
type ChamberLease struct {
	CycleID  CycleID   `json:"cycleId"`
	Holder   string    `json:"holder"`
	Acquired time.Time `json:"acquired"`
	Expires  time.Time `json:"expires"`
}
