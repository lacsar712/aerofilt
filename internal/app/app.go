// Package app wires aerofilt Biofilter subsystems into an operable filter controller.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/alarm"
	"github.com/lacsar712/aerofilt/internal/api"
	"github.com/lacsar712/aerofilt/internal/batch"
	"github.com/lacsar712/aerofilt/internal/clock"
	"github.com/lacsar712/aerofilt/internal/config"
	"github.com/lacsar712/aerofilt/internal/fsm"
	"github.com/lacsar712/aerofilt/internal/filter"
	"github.com/lacsar712/aerofilt/internal/aeration"
	"github.com/lacsar712/aerofilt/internal/interlock"
	"github.com/lacsar712/aerofilt/internal/journal"
	"github.com/lacsar712/aerofilt/internal/model"
	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/blower"
	"github.com/lacsar712/aerofilt/internal/backwash"
	"github.com/lacsar712/aerofilt/internal/store"
	"github.com/lacsar712/aerofilt/internal/telemetry"
	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/level"
)

// App is the root Biofilter filter controller.
type App struct {
	cfg       config.Config
	book      *filter.Book
	store     *store.ProfileStore
	vacuum    *vacuum.Pump
	gas       *pressure.Controller
	emitter   *heater.CountingEmitter
	heater    *heater.Bank
	backwashFSM   *fsm.Machine
	gates     *interlock.Gates
	coord     *filter.Coordinator
	backwashWin   *backwash.WindowTracker
	blower    *blower.Cooler
	batches   *batch.Registry
	probe     *media.Probe
	guard     *filter.Guard
	alarms    *alarm.Bus
	telem     *telemetry.Buffer
	journal   *journal.Writer
	cancel    context.CancelFunc
	rootCtx   context.Context
	mu        sync.Mutex
	server    *api.Server
	activeBatch model.BatchID
}

// New constructs a fully wired App.
func New(cfg config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	book := filter.NewBook(cfg.FilterID, cfg.ZoneList(), cfg.DefaultSetpointC)
	st := store.NewProfileStore(cfg.FilterID)
	pump := vacuum.NewPump()
	gas := pressure.NewController(cfg.GasMaxMPa)
	_ = gas.SetSetpoint(cfg.GasSetpointMPa)
	em := heater.NewCountingEmitter(cfg.HeaterMaxKW)
	hb := heater.NewBank(cfg.HeaterMaxKW, em)
	proc := clock.NewProcessClock(time.Now().UTC())
	real := clock.RealClock{}
	backwashWin := backwash.NewWindowTracker(proc, real)
	jrn := journal.MemoryOnly(cfg.FilterID, 256)
	gates := interlock.NewGates(cfg.InterlockStrict)
	coord := filter.NewCoordinator(cfg.ChamberLeaseTTL, book, pump)
	root, cancel := context.WithCancel(context.Background())
	a := &App{
		cfg: cfg, book: book, store: st, vacuum: pump, gas: gas,
		emitter: em, heater: hb, gates: gates, coord: coord,
		backwashWin: backwashWin, blower: blower.NewCooler(cfg.BlowerRateCPerMin),
		batches: batch.NewRegistry(64), probe: media.NewProbe(0.5, 30),
		guard: filter.NewGuard(book, cfg.OverTempLimitC),
		alarms: alarm.NewBus(cfg.AlarmCapacity), telem: telemetry.NewBuffer(cfg.TelemetryBuffer),
		journal: jrn, rootCtx: root, cancel: cancel,
	}
	a.backwashFSM = fsm.NewMachine("", em)
	a.backwashFSM.SetHeaterKW(cfg.HeaterMaxKW * 0.6)
	return a, nil
}

// Close releases background context and journal.
func (a *App) Close() error {
	a.cancel()
	if a.journal != nil {
		return a.journal.Close()
	}
	return nil
}

// Config returns a copy of runtime config.
func (a *App) Config() config.Config { return a.cfg }

// AttachHTTP builds the API server with optional static file server.
func (a *App) AttachHTTP(static http.Handler) http.Handler {
	a.server = api.NewServer(a, static)
	return a.server.Handler()
}

// SeedSensors writes initial zone temperatures for all mediacouples.
func (a *App) SeedSensors(now time.Time) error {
	for _, z := range a.cfg.ZoneList() {
		for i := 0; i < a.cfg.SensorPerZone; i++ {
			sample := model.TempSample{
				ZoneID:   model.ZoneID(z),
				SensorID: model.SensorID(fmt.Sprintf("%s-t%d", z, i+1)),
				Celsius:  a.cfg.DefaultSetpointC - 400 + float64(i)*0.3,
				Quality:  0.98,
				At:       now,
				Source:   "seed",
			}
			if err := a.IngestTemperature(sample); err != nil {
				return err
			}
		}
	}
	return nil
}

// IngestTemperature updates filter book and store snapshot.
func (a *App) IngestTemperature(sample model.TempSample) error {
	if err := a.probe.ValidateSample(sample, a.cfg.DefaultSetpointC); err != nil {
		if media.IsRecoverable(err) {
			a.alarms.RaiseMediaFault(a.cfg.FilterID, string(sample.SensorID))
		}
	}
	if err := a.book.UpdateSensor(sample); err != nil {
		return err
	}
	zones := a.book.AllZones()
	phase := a.backwashFSM.Phase()
	a.store.ReplaceAll(zones, a.book.Mode(), phase, a.book.Batch())
	snap := a.store.Snapshot(time.Now().UTC())
	a.telem.RecordProfile(snap)
	ok, bad := a.book.Balanced(a.cfg.BackwashBandC)
	if !ok && a.book.Mode() == model.FilterModeBackwash {
		a.alarms.RaiseTempImbalance(a.cfg.FilterID, bad)
	} else {
		_ = a.alarms.Clear("TEMP_IMBALANCE")
	}
	if err := a.guard.Evaluate(); err != nil {
		if filter.Procedure(err) == "vent" {
			a.alarms.Raise("OVER_TEMP", err.Error(), a.cfg.FilterID, model.SeverityCritical)
		}
	}
	return nil
}

// Status builds operator status.
func (a *App) Status() model.FilterStatus {
	now := time.Now().UTC()
	snap := a.ProfileSnapshot()
	vac := a.vacuum.Reading(now)
	gas := a.gas.Reading(now)
	decision := a.gates.LastDecision()
	return model.FilterStatus{
		FilterID:    a.cfg.FilterID,
		Mode:         a.book.Mode(),
		Profile:      snap,
		BackwashPhase:    a.backwashFSM.Phase(),
		HeaterPhase:  a.heater.Phase(),
		VacuumPa:     vac.Pascals,
		GasMPa:       gas.MPa,
		BackwashWindow:   a.backwashWin.Snapshot(),
		InterlockOK:  decision.Allowed || decision.Code == "",
		ActiveAlarms: a.alarms.ActiveCount(),
		ActiveBatch:  a.activeBatch,
		UpdatedAt:    now,
	}
}

// ProfileSnapshot returns a deep-copied profile view from the store.
func (a *App) ProfileSnapshot() model.ProfileSnapshot {
	return a.store.CloneSnapshot(time.Now().UTC())
}

// Alarms lists active alarms.
func (a *App) Alarms() []model.AlarmEvent { return a.alarms.ListActive() }

// Telemetry returns recent points.
func (a *App) Telemetry(n int) []model.TelemetryPoint { return a.telem.Recent(n) }

// EmergencyStop cancels root context and latches interlock / filter mode.
func (a *App) EmergencyStop() error {
	a.gates.SetEStop(true)
	a.book.SetMode(model.FilterModeEStop)
	a.heater.Idle()
	a.alarms.RaiseEStop(a.cfg.FilterID)
	a.cancel()
	a.mu.Lock()
	a.rootCtx, a.cancel = context.WithCancel(context.Background())
	a.mu.Unlock()
	_ = a.journal.Append("estop", map[string]string{"filter": a.cfg.FilterID})
	return nil
}

// ClearEmergencyStop clears estop latch after operator acknowledgment.
func (a *App) ClearEmergencyStop() error {
	a.gates.SetEStop(false)
	a.book.SetMode(model.FilterModeStandby)
	a.heater.Idle()
	_ = a.alarms.Clear("ESTOP")
	return nil
}

func (a *App) activeCtx() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rootCtx
}

// StartCycle evaluates interlocks then runs chamber coordinator vacuum precheck.
func (a *App) StartCycle(batchID model.BatchID, operator string) error {
	if batchID == "" {
		batchID = model.BatchID(fmt.Sprintf("B-%d", time.Now().Unix()))
	}
	snap := a.ProfileSnapshot()
	decision := a.gates.Evaluate(snap, a.cfg.BackwashBandC, a.vacuum.Online())
	if !decision.Allowed {
		a.alarms.Raise("INTERLOCK", fmt.Sprintf("%s: %v", decision.Code, decision.Reasons), a.cfg.FilterID, model.SeverityCritical)
		return fmt.Errorf("interlock denied: %s", decision.Code)
	}
	ctx := a.activeCtx()
	cycleID := model.CycleID(fmt.Sprintf("C-%s", batchID))
	if err := a.coord.RunCycle(ctx, cycleID, operator, a.cfg.VacuumTargetPa); err != nil {
		if errors.Is(err, vacuum.VacuumLeak) {
			a.alarms.RaiseVacuumLeak(a.cfg.FilterID)
		}
		return err
	}
	a.activeBatch = batchID
	a.book.SetBatch(batchID)
	a.backwashFSM.SetBatchID(batchID)
	a.book.SetMode(model.FilterModeEvacuate)
	_ = a.batches.Register(model.BatchRecord{
		ID: batchID, Alloy: "IN718", PartCount: 12, ProfileID: "std-biofilter",
		Phase: model.BackwashEvacuating, Operator: operator,
	})
	_ = a.journal.Append("cycle_start", map[string]string{"batch": string(batchID), "operator": operator})
	_ = a.alarms.Clear("INTERLOCK")
	return nil
}

// BackwashTransition forwards to backwash FSM (illegal transitions do not emit heater/vent).
func (a *App) BackwashTransition(req model.BackwashTransitionRequest) (model.BackwashTransitionResult, error) {
	if req.At.IsZero() {
		req.At = time.Now().UTC()
	}
	if req.BatchID == "" {
		req.BatchID = a.activeBatch
	}
	ctx := a.activeCtx()
	res, err := a.backwashFSM.Transition(ctx, req)
	if err != nil {
		return res, err
	}
	_ = a.batches.UpdatePhase(req.BatchID, res.Phase)
	a.syncModeFromPhase(res.Phase)
	if res.Phase == model.BackwashHolding {
		target, band, minDur := a.cfg.BackwashWindowSpec()
		a.backwashWin.Open(target, band, minDur)
		a.book.SetMode(model.FilterModeBackwash)
	}
	if res.Phase == model.BackwashBlowering {
		a.book.SetMode(model.FilterModeBlower)
		fleet := a.store.FleetMean()
		_ = a.blower.Start(ctx, fleet)
	}
	if res.Phase == model.BackwashComplete {
		_ = a.batches.Complete(req.BatchID)
		a.backwashWin.Close()
		a.book.SetMode(model.FilterModeStandby)
		a.activeBatch = ""
	}
	a.SyncStoreFromBook()
	_ = a.journal.Append("backwash_transition", res)
	return res, nil
}

func (a *App) syncModeFromPhase(phase model.BackwashPhase) {
	switch phase {
	case model.BackwashRamping:
		a.book.SetMode(model.FilterModeRamp)
	case model.BackwashPressuring:
		_ = a.gas.Pressurize(a.cfg.GasSetpointMPa)
	case model.BackwashEvacuating:
		a.book.SetMode(model.FilterModeEvacuate)
	}
}

// SyncStoreFromBook refreshes store from book aggregates.
func (a *App) SyncStoreFromBook() {
	a.store.ReplaceAll(a.book.AllZones(), a.book.Mode(), a.backwashFSM.Phase(), a.book.Batch())
}

// EvaluateBackwashWindow checks whether backwash window can close on current profile.
func (a *App) EvaluateBackwashWindow() (bool, string) {
	return a.backwashWin.Evaluate(a.ProfileSnapshot())
}

// EmitterCount exposes side-effect emission counts for diagnostics.
func (a *App) EmitterHeaterCount() int64 { return a.emitter.HeaterCount() }
func (a *App) EmitterVentCount() int64   { return a.emitter.VentCount() }

// RampHeater steps heater PWM using the caller abort context so cancel stops climb.
func (a *App) RampHeater(ctx context.Context, zone model.ZoneID, targetKW float64, steps int) error {
	return a.heater.RampPower(ctx, zone, targetKW, steps)
}

// PurgeBlower runs multi-step blower valve ramp under caller cancel.
func (a *App) PurgeBlower(ctx context.Context, steps int) error {
	return a.blower.PurgeRamp(ctx, steps)
}

// PressurizeGas ramps argon under caller cancel.
func (a *App) PressurizeGas(ctx context.Context, targetMPa float64) error {
	return a.gas.PressurizeCtx(ctx, targetMPa)
}

// Blower exposes blower cooler for tests.
func (a *App) Blower() *blower.Cooler { return a.blower }

// Gas exposes pressure controller for tests.
func (a *App) Gas() *pressure.Controller { return a.gas }

// HeaterBank exposes heater bank for tests.
func (a *App) HeaterBank() *heater.Bank { return a.heater }

// Book exposes filter book for tests.
func (a *App) Book() *filter.Book { return a.book }

// Gates exposes interlock gates for tests.
func (a *App) Gates() *interlock.Gates { return a.gates }

// BackwashMachine exposes backwash FSM for tests.
func (a *App) BackwashMachine() *fsm.Machine { return a.backwashFSM }

// BackwashWindow exposes backwash window tracker for tests.
func (a *App) BackwashWindow() *backwash.WindowTracker { return a.backwashWin }

// Coordinator exposes chamber coordinator for tests.
func (a *App) Coordinator() *filter.Coordinator { return a.coord }
