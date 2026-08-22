package level

import (
	"errors"
	"fmt"

	"github.com/lacsar712/aerofilt/internal/model"
)

var (
	ErrHeadLow    = errors.New("head level below minimum")
	ErrHeadHigh   = errors.New("head level above maximum")
	ErrUnstable   = errors.New("head reading unstable")
	ErrBadQuality = errors.New("head sample quality too low")
)

type Monitor struct {
	minQuality float64
}

func NewMonitor(minQuality float64) *Monitor {
	if minQuality <= 0 {
		minQuality = 0.5
	}
	return &Monitor{minQuality: minQuality}
}

func (m *Monitor) Validate(sample model.HeadSample, minM, maxM float64) error {
	if sample.Meters < 0 {
		return fmt.Errorf("%w: negative reading", model.ErrInvalidSample)
	}
	if sample.Quality < m.minQuality {
		return fmt.Errorf("%w: quality %.2f", ErrBadQuality, sample.Quality)
	}
	if sample.Meters < minM {
		return fmt.Errorf("%w: %.3f < %.3f", ErrHeadLow, sample.Meters, minM)
	}
	if sample.Meters > maxM {
		return fmt.Errorf("%w: %.3f > %.3f", ErrHeadHigh, sample.Meters, maxM)
	}
	return nil
}

func IsLow(err error) bool     { return errors.Is(err, ErrHeadLow) }
func IsHigh(err error) bool    { return errors.Is(err, ErrHeadHigh) }
func IsUnstable(err error) bool { return errors.Is(err, ErrUnstable) }
