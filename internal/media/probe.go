// Package media validates mediacouple readings and media fault classification.
package media

import (
	"errors"
	"fmt"

	"github.com/lacsar712/aerofilt/internal/model"
)

// RecoverableMediaFault marks a transient mediacouple drift that may continue
// after sensor trim without aborting the Biofilter cycle.
var RecoverableMediaFault = errors.New("recoverable mediacouple fault")

// ErrMediaOpen is a hard open-circuit mediacouple fault.
var ErrMediaOpen = errors.New("mediacouple open circuit")

// ErrMediaShort is a hard shorted mediacouple fault.
var ErrMediaShort = errors.New("mediacouple short circuit")

// Probe wraps zone mediacouple validation with sentinel error chains.
type Probe struct {
	minQuality float64
	maxDriftC  float64
}

// NewProbe creates a mediacouple validator.
func NewProbe(minQuality, maxDriftC float64) *Probe {
	if minQuality <= 0 {
		minQuality = 0.5
	}
	if maxDriftC <= 0 {
		maxDriftC = 25
	}
	return &Probe{minQuality: minQuality, maxDriftC: maxDriftC}
}

// ValidateSample checks one reading against expected band.
func (p *Probe) ValidateSample(sample model.TempSample, expectedC float64) error {
	if err := model.ValidateTempSample(sample); err != nil {
		return err
	}
	if sample.Quality < p.minQuality {
		return fmt.Errorf("sensor %s quality %.2f: %w", sample.SensorID, sample.Quality, RecoverableMediaFault)
	}
	drift := sample.Celsius - expectedC
	if drift < 0 {
		drift = -drift
	}
	if drift > p.maxDriftC {
		return fmt.Errorf("sensor %s drift %.1f C: %w", sample.SensorID, drift, RecoverableMediaFault)
	}
	if sample.Quality < 0.05 {
		return fmt.Errorf("sensor %s open: %w", sample.SensorID, ErrMediaOpen)
	}
	return nil
}

// IsRecoverable reports whether err is a recoverable media fault.
func IsRecoverable(err error) bool {
	return errors.Is(err, RecoverableMediaFault)
}

// Classify maps media errors to operator codes.
func Classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, RecoverableMediaFault):
		return "recoverable"
	case errors.Is(err, ErrMediaOpen):
		return "open"
	case errors.Is(err, ErrMediaShort):
		return "short"
	default:
		return "unknown"
	}
}

// Wrap preserves RecoverableMediaFault across package boundaries.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}
