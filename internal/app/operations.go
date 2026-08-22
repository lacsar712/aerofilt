package app

import (
	"time"

	"github.com/lacsar712/aerofilt/internal/backwash"
	"github.com/lacsar712/aerofilt/internal/filter"
	"github.com/lacsar712/aerofilt/internal/interlock"
	"github.com/lacsar712/aerofilt/internal/model"
)

// Operations exposes auxiliary plant operations built on core subsystems.
type Operations struct {
	app      *App
	registry *filter.Registry
	policy   *interlock.Policy
	schedule *backwash.Schedule
}

// Operations returns the auxiliary operations facade for the app.
func (a *App) Operations() *Operations {
	if a.ops == nil {
		a.ops = &Operations{
			app:      a,
			registry: filter.NewRegistry(32),
			policy:   interlock.NewPolicy(a.cfg.MinHeadM, a.cfg.MaxHeadM, a.cfg.MaxClogIndex, false),
			schedule: backwash.NewSchedule(30*time.Second, 45*time.Second, 60*time.Second, 30*time.Second),
		}
	}
	return a.ops
}

// Admit evaluates extended interlock policy before a backwash is requested.
func (o *Operations) Admit(cell model.CellID, filterID model.FilterID, sample model.HeadSample, profile model.MediaProfile) (model.InterlockDecision, error) {
	snap := o.app.store.Snapshot(time.Now().UTC())
	return o.policy.AdmitBackwash(snap, cell, sample, profile)
}

// RecordWashStart tracks wash start in the cell registry.
func (o *Operations) RecordWashStart(req model.BackwashRequest, phase model.WashPhase) {
	o.registry.Start(req.OperationID, req.FilterID, req.CellID, phase, req.Operator, req.IssuedAt)
}

// RecordWashComplete marks the active wash complete in the registry.
func (o *Operations) RecordWashComplete(cell model.CellID, phase model.WashPhase, at time.Time) error {
	return o.registry.Complete(cell, phase, at)
}

// SinceLastWash returns duration since the last completed wash on a cell.
func (o *Operations) SinceLastWash(cell model.CellID) time.Duration {
	return o.registry.SinceLastWash(cell, time.Now().UTC())
}

// NextScheduledPhase recommends the next wash phase when dwell is satisfied.
func (o *Operations) NextScheduledPhase(current model.WashPhase, elapsed time.Duration) (model.WashPhase, bool) {
	return o.schedule.NextPhase(current, elapsed)
}

// ScheduleDescription returns operator-readable phase timing summary.
func (o *Operations) ScheduleDescription() string { return o.schedule.Describe() }

// MinimumWashDuration returns total configured minimum wash sequence time.
func (o *Operations) MinimumWashDuration() time.Duration { return o.schedule.TotalMinimum() }

// ReadyForWindowClose combines restore dwell and process window for close readiness.
func (o *Operations) ReadyForWindowClose(phase model.WashPhase, phaseElapsed time.Duration) bool {
	return o.schedule.ReadyForClose(phase, phaseElapsed, o.app.washWin.Elapsed(), o.app.cfg.WashCloseWindow)
}

// WashHistory returns prior wash records for a cell.
func (o *Operations) WashHistory(cell model.CellID) []filter.WashRecord {
	return o.registry.History(cell)
}

// PolicyExplain renders a denied interlock decision for operators.
func (o *Operations) PolicyExplain(decision model.InterlockDecision) string {
	return o.policy.Explain(decision)
}

// PlannedPhases returns the ordered wash phase plan.
func (o *Operations) PlannedPhases() []model.WashPhase { return o.schedule.Plan() }
