package media

import (
	"errors"
	"fmt"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

var (
	ErrClogged   = errors.New("filter media clogged beyond threshold")
	ErrVoidRatio = errors.New("media void ratio out of range")
	ErrBedDepth  = errors.New("media bed depth invalid")
)

type Probe struct {
	maxClog, minVoid, maxVoid, minBedDepth float64
}

func NewProbe(maxClog, minVoid, maxVoid, minBedDepth float64) *Probe {
	if maxClog <= 0 { maxClog = 0.85 }
	if minVoid <= 0 { minVoid = 0.35 }
	if maxVoid <= minVoid { maxVoid = 0.55 }
	if minBedDepth <= 0 { minBedDepth = 1.0 }
	return &Probe{maxClog: maxClog, minVoid: minVoid, maxVoid: maxVoid, minBedDepth: minBedDepth}
}

func (p *Probe) Evaluate(profile model.MediaProfile) error {
	if profile.BedDepthM < p.minBedDepth {
		return fmt.Errorf("%w: %.2f m", ErrBedDepth, profile.BedDepthM)
	}
	if profile.VoidRatio < p.minVoid || profile.VoidRatio > p.maxVoid {
		return fmt.Errorf("%w: %.3f", ErrVoidRatio, profile.VoidRatio)
	}
	if profile.ClogIndex >= p.maxClog {
		return fmt.Errorf("%w: index %.3f >= %.3f", ErrClogged, profile.ClogIndex, p.maxClog)
	}
	return nil
}

func (p *Probe) NeedsBackwash(profile model.MediaProfile) bool { return profile.ClogIndex >= p.maxClog*0.7 }

func (p *Probe) Touch(profile *model.MediaProfile, at time.Time) {
	profile.ClogIndex *= 0.35
	if profile.ClogIndex < 0.05 { profile.ClogIndex = 0.05 }
	profile.UpdatedAt = at
}

func IsClogged(err error) bool { return errors.Is(err, ErrClogged) }
func IsVoidBad(err error) bool { return errors.Is(err, ErrVoidRatio) }