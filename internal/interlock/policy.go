package interlock

import (
	"fmt"

	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
)

// Policy bundles cross-domain interlock rules used before backwash admission.
type Policy struct {
	minHeadM      float64
	maxHeadM      float64
	maxClog       float64
	allowParallel bool
}

// NewPolicy constructs an interlock policy from plant limits.
func NewPolicy(minHeadM, maxHeadM, maxClog float64, allowParallel bool) *Policy {
	return &Policy{minHeadM: minHeadM, maxHeadM: maxHeadM, maxClog: maxClog, allowParallel: allowParallel}
}

// CheckHead uses level package errors for out-of-range head.
func (p *Policy) CheckHead(sample model.HeadSample) error {
	mon := level.NewMonitor(0.55)
	return mon.Validate(sample, p.minHeadM, p.maxHeadM)
}

// CheckMedia uses media package errors for clogged beds.
func (p *Policy) CheckMedia(profile model.MediaProfile) error {
	probe := media.NewProbe(p.maxClog, 0.35, 0.55, 1.0)
	return probe.Evaluate(profile)
}

// AdmitBackwash evaluates whether a cell may enter backwash given plant snapshot.
func (p *Policy) AdmitBackwash(snap model.FilterSnapshot, cell model.CellID, sample model.HeadSample, profile model.MediaProfile) (model.InterlockDecision, error) {
	if snap.Mode == model.PlantModeEStop {
		return model.InterlockDecision{Allowed: false, Code: "ESTOP", Reasons: []string{model.ErrPlantEStop.Error()}}, model.ErrPlantEStop
	}
	if err := p.CheckHead(sample); err != nil {
		if level.IsLow(err) {
			return model.InterlockDecision{Allowed: false, Code: "HEAD_LOW", Reasons: []string{err.Error()}}, err
		}
		if level.IsHigh(err) {
			return model.InterlockDecision{Allowed: false, Code: "HEAD_HIGH", Reasons: []string{err.Error()}}, err
		}
		return model.InterlockDecision{Allowed: false, Code: "HEAD_BAD", Reasons: []string{err.Error()}}, err
	}
	if err := p.CheckMedia(profile); err != nil {
		if media.IsClogged(err) {
			return model.InterlockDecision{Allowed: false, Code: "MEDIA_CLOG", Reasons: []string{err.Error()}}, err
		}
		return model.InterlockDecision{Allowed: false, Code: "MEDIA_BAD", Reasons: []string{err.Error()}}, err
	}
	if !p.allowParallel {
		for _, c := range snap.Cells {
			if c.ID == cell {
				continue
			}
			if c.WashPhase != model.WashIdle && c.WashPhase != model.WashComplete {
				return model.InterlockDecision{Allowed: false, Code: "PARALLEL", Reasons: []string{fmt.Sprintf("cell %s washing", c.ID)}}, fmt.Errorf("parallel wash denied")
			}
		}
	}
	return model.InterlockDecision{Allowed: true, Code: "OK"}, nil
}

// Explain returns an operator-facing summary for a denied decision.
func (p *Policy) Explain(decision model.InterlockDecision) string {
	if decision.Allowed {
		return "interlock clear"
	}
	if len(decision.Reasons) == 0 {
		return decision.Code
	}
	return fmt.Sprintf("%s: %s", decision.Code, decision.Reasons[0])
}
