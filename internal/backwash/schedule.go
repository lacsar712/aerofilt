package backwash

import (
	"fmt"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Schedule recommends the next wash phase based on current cell phase and elapsed time.
type Schedule struct {
	drainMin   time.Duration
	airMin     time.Duration
	rinseMin   time.Duration
	restoreMin time.Duration
}

// NewSchedule defines minimum dwell times for each wash phase.
func NewSchedule(drain, air, rinse, restore time.Duration) *Schedule {
	if drain <= 0 {
		drain = 30 * time.Second
	}
	if air <= 0 {
		air = 45 * time.Second
	}
	if rinse <= 0 {
		rinse = 60 * time.Second
	}
	if restore <= 0 {
		restore = 30 * time.Second
	}
	return &Schedule{drainMin: drain, airMin: air, rinseMin: rinse, restoreMin: restore}
}

// NextPhase returns the recommended successor phase when dwell time is satisfied.
func (s *Schedule) NextPhase(current model.WashPhase, elapsed time.Duration) (model.WashPhase, bool) {
	switch current {
	case model.WashDraining:
		return model.WashAirScour, elapsed >= s.drainMin
	case model.WashAirScour:
		return model.WashRinsing, elapsed >= s.airMin
	case model.WashRinsing:
		return model.WashRestoring, elapsed >= s.rinseMin
	case model.WashRestoring:
		return model.WashComplete, elapsed >= s.restoreMin
	default:
		return current, false
	}
}

// MinDuration returns configured minimum dwell for a phase.
func (s *Schedule) MinDuration(phase model.WashPhase) time.Duration {
	switch phase {
	case model.WashDraining:
		return s.drainMin
	case model.WashAirScour:
		return s.airMin
	case model.WashRinsing:
		return s.rinseMin
	case model.WashRestoring:
		return s.restoreMin
	default:
		return 0
	}
}

// Describe returns a human-readable schedule summary for operators.
func (s *Schedule) Describe() string {
	return fmt.Sprintf("drain=%s air=%s rinse=%s restore=%s", s.drainMin, s.airMin, s.rinseMin, s.restoreMin)
}

// Plan builds an ordered list of phases from drain through complete.
func (s *Schedule) Plan() []model.WashPhase {
	return []model.WashPhase{model.WashDraining, model.WashAirScour, model.WashRinsing, model.WashRestoring, model.WashComplete}
}

// TotalMinimum returns sum of configured phase minimums.
func (s *Schedule) TotalMinimum() time.Duration {
	return s.drainMin + s.airMin + s.rinseMin + s.restoreMin
}

// ReadyForClose reports whether a wash sequence has passed all phase minimums plus close window.
func (s *Schedule) ReadyForClose(phase model.WashPhase, phaseElapsed, windowElapsed, closeWindow time.Duration) bool {
	if phase != model.WashRestoring && phase != model.WashComplete {
		return false
	}
	return windowElapsed >= closeWindow && phaseElapsed >= s.restoreMin
}
