// Package ramp builds programmed Biofilter temperature ramp profiles.
package ramp

import (
	"fmt"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Builder constructs ramp profiles from segment specifications.
type Builder struct {
	id       string
	name     string
	segments []model.RampSegment
	backwash     model.BackwashWindow
}

// NewBuilder creates an empty ramp builder.
func NewBuilder(id, name string) *Builder {
	return &Builder{id: id, name: name, segments: make([]model.RampSegment, 0, 8)}
}

// AddSegment appends one ramp leg.
func (b *Builder) AddSegment(fromC, toC, rateCPerMin float64) error {
	if rateCPerMin <= 0 {
		return fmt.Errorf("ramp rate must be positive")
	}
	if fromC == toC {
		return fmt.Errorf("ramp segment requires temperature change")
	}
	b.segments = append(b.segments, model.RampSegment{
		FromC: fromC, ToC: toC, RateCPerMin: rateCPerMin,
	})
	return nil
}

// WithBackwash attaches backwash window requirements.
func (b *Builder) WithBackwash(targetC, bandC float64, minDuration time.Duration) *Builder {
	b.backwash = model.BackwashWindow{
		TargetC: targetC, BandC: bandC, MinDuration: minDuration,
	}
	return b
}

// Build returns the completed ramp profile.
func (b *Builder) Build() (model.RampProfile, error) {
	if len(b.segments) == 0 {
		return model.RampProfile{}, fmt.Errorf("at least one ramp segment required")
	}
	if b.backwash.MinDuration <= 0 {
		return model.RampProfile{}, fmt.Errorf("backwash min duration required")
	}
	return model.RampProfile{
		ID: b.id, Name: b.name, Segments: copySegments(b.segments), Backwash: b.backwash,
	}, nil
}

func copySegments(in []model.RampSegment) []model.RampSegment {
	out := make([]model.RampSegment, len(in))
	copy(out, in)
	return out
}

// Duration estimates total ramp time excluding backwash.
func Duration(p model.RampProfile) time.Duration {
	var total time.Duration
	for _, seg := range p.Segments {
		delta := seg.ToC - seg.FromC
		if delta < 0 {
			delta = -delta
		}
		if seg.RateCPerMin <= 0 {
			continue
		}
		minutes := delta / seg.RateCPerMin
		total += time.Duration(minutes * float64(time.Minute))
	}
	return total
}

// TargetAt returns expected temperature after elapsed ramp time.
func TargetAt(p model.RampProfile, elapsed time.Duration) float64 {
	remaining := elapsed
	for _, seg := range p.Segments {
		delta := seg.ToC - seg.FromC
		if delta < 0 {
			delta = -delta
		}
		if seg.RateCPerMin <= 0 {
			continue
		}
		segDur := time.Duration(delta/seg.RateCPerMin) * time.Minute
		if remaining >= segDur {
			remaining -= segDur
			continue
		}
		fraction := remaining.Minutes() * seg.RateCPerMin / delta
		if seg.ToC > seg.FromC {
			return seg.FromC + fraction*delta
		}
		return seg.FromC - fraction*delta
	}
	if len(p.Segments) == 0 {
		return 0
	}
	return p.Segments[len(p.Segments)-1].ToC
}

// StandardBiofilter returns a typical superalloy Biofilter ramp profile.
func StandardBiofilter() (model.RampProfile, error) {
	b := NewBuilder("std-biofilter", "Standard superalloy Biofilter")
	if err := b.AddSegment(25, 600, 10); err != nil {
		return model.RampProfile{}, err
	}
	if err := b.AddSegment(600, 1180, 8); err != nil {
		return model.RampProfile{}, err
	}
	b.WithBackwash(1180, 5, 2*time.Hour)
	return b.Build()
}
