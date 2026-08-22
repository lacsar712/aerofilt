package app

import (
	"time"

	"github.com/lacsar712/aerofilt/internal/ramp"
)

// DefaultRampProfile returns the standard Biofilter ramp used at startup.
func DefaultRampProfile() (ramp.Builder, error) {
	p, err := ramp.StandardBiofilter()
	if err != nil {
		return *ramp.NewBuilder("", ""), err
	}
	b := ramp.NewBuilder(p.ID, p.Name)
	for _, seg := range p.Segments {
		if err := b.AddSegment(seg.FromC, seg.ToC, seg.RateCPerMin); err != nil {
			return *b, err
		}
	}
	b.WithBackwash(p.Backwash.TargetC, p.Backwash.BandC, p.Backwash.MinDuration)
	return *b, nil
}

// RampDurationEstimate returns expected ramp time for the default profile.
func RampDurationEstimate() time.Duration {
	p, err := ramp.StandardBiofilter()
	if err != nil {
		return 0
	}
	return ramp.Duration(p)
}
