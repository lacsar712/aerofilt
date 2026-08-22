package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/aeration"
	"github.com/lacsar712/aerofilt/internal/alarm"
	"github.com/lacsar712/aerofilt/internal/api"
	"github.com/lacsar712/aerofilt/internal/backwash"
	"github.com/lacsar712/aerofilt/internal/blower"
	"github.com/lacsar712/aerofilt/internal/clock"
	"github.com/lacsar712/aerofilt/internal/config"
	"github.com/lacsar712/aerofilt/internal/filter"
	"github.com/lacsar712/aerofilt/internal/fsm"
	"github.com/lacsar712/aerofilt/internal/interlock"
	"github.com/lacsar712/aerofilt/internal/journal"
	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
	"github.com/lacsar712/aerofilt/internal/store"
	"github.com/lacsar712/aerofilt/internal/telemetry"
)

type App struct {
	cfg config.Config
	book *filter.Book
	store *store.FilterStore
	valves *manifold.ValveBank
	emitter *backwash.Emitter
	coord *backwash.Coordinator
	washWin *backwash.Window
	machines map[model.CellID]*fsm.WashMachine
	cells *interlock.Cells
	aeration *aeration.Bank
	blower *blower.Controller
	levelMon *level.Monitor
	mediaProbe *media.Probe
	guard *filter.Guard
	alarms *alarm.Bus
	telem *telemetry.Buffer
	journal *journal.Writer
	procClock *clock.ProcessClock
	ops       *Operations
	mu sync.Mutex
	rootCtx context.Context
	cancel context.CancelFunc
}

func New(cfg config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cells := seedCells(cfg)
	profiles := seedMedia(cfg)
	book := filter.NewBook(cfg.PlantID, cells)
	st := store.NewFilterStore(cfg.PlantID, cells, profiles)
	valveSpec := make(map[model.CellID][]model.ValveID)
	for _, spec := range cfg.Cells {
		valveSpec[spec.ID] = cfg.ValvesFor(spec.ID)
	}
	valves := manifold.NewValveBank(valveSpec)
	emitter := backwash.NewEmitter(nil)
	coord := backwash.NewCoordinator(valves, emitter)
	proc := clock.NewProcessClock(time.Now().UTC())
	washWin := backwash.NewWindow(proc, cfg.WashCloseWindow, cfg.TargetHeadM, cfg.BackwashBandM)
	machines := make(map[model.CellID]*fsm.WashMachine, len(cfg.Cells))
	for _, spec := range cfg.Cells {
		machines[spec.ID] = fsm.NewWashMachine(spec.ID, emitter, valves)
	}
	bl := blower.NewController(cfg.BlowerMaxPct, 30)
	lvl := level.NewMonitor(0.6)
	med := media.NewProbe(cfg.MaxClogIndex, 0.35, 0.55, 1.2)
	root, cancel := context.WithCancel(context.Background())
	return &App{
		cfg: cfg, book: book, store: st, valves: valves, emitter: emitter, coord: coord, washWin: washWin, machines: machines,
		cells: interlock.NewCells(cfg.LeaseTTL, true), aeration: aeration.NewBank(cells), blower: bl,
		levelMon: lvl, mediaProbe: med,
		guard: filter.NewGuard(book, lvl, med, bl, cfg.MinHeadM, cfg.MaxHeadM),
		alarms: alarm.NewBus(cfg.AlarmCapacity), telem: telemetry.NewBuffer(cfg.TelemetryBuffer), journal: journal.MemoryOnly(string(cfg.PlantID), cfg.JournalCapacity),
		procClock: proc, rootCtx: root, cancel: cancel,
	}, nil
}

func seedCells(cfg config.Config) []model.FilterCell {
	out := make([]model.FilterCell, len(cfg.Cells))
	for i, spec := range cfg.Cells {
		out[i] = model.FilterCell{ID: spec.ID, FilterID: spec.FilterID, HeadM: cfg.TargetHeadM, Online: true, WashPhase: model.WashIdle, AerationPct: 65}
	}
	return out
}

func seedMedia(cfg config.Config) []model.MediaProfile {
	seen := map[model.FilterID]bool{}
	var out []model.MediaProfile
	now := time.Now().UTC()
	for _, spec := range cfg.Cells {
		if seen[spec.FilterID] {
			continue
		}
		seen[spec.FilterID] = true
		out = append(out, model.MediaProfile{FilterID: spec.FilterID, BedDepthM: 1.8, VoidRatio: 0.42, ClogIndex: 0.55, MediaType: "anthracite", UpdatedAt: now})
	}
	return out
}

func (a *App) Close() error { a.cancel(); return a.journal.Close() }
func (a *App) Config() config.Config { return a.cfg }
func (a *App) AttachHTTP() http.Handler { return api.NewServer(a).Handler() }

func (a *App) SeedHead(now time.Time) error {
	for _, spec := range a.cfg.Cells {
		if err := a.IngestHead(model.HeadSample{CellID: spec.ID, SensorID: model.SensorID(string(spec.ID) + "-head"), Meters: a.cfg.TargetHeadM, Quality: 0.95, At: now, Source: "seed"}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) IngestHead(sample model.HeadSample) error {
	if err := a.levelMon.Validate(sample, a.cfg.MinHeadM, a.cfg.MaxHeadM); err != nil {
		if level.IsLow(err) || level.IsHigh(err) {
			a.alarms.RaiseHeadFault(a.cfg.PlantID, sample.CellID, err.Error())
		}
		return err
	}
	if err := a.book.UpdateHead(sample); err != nil {
		return err
	}
	a.syncStore()
	a.telem.RecordHead(sample)
	a.telem.RecordSnapshot(a.store.Snapshot(time.Now().UTC()))
	return nil
}

func (a *App) Status() model.PlantStatus {
	now := time.Now().UTC()
	snap := a.store.Snapshot(now)
	decision := a.cells.Evaluate(snap, a.cfg.Cells[0].ID)
	phase := model.WashIdle
	if m := a.machines[a.cfg.Cells[0].ID]; m != nil {
		phase = m.Phase()
	}
	return model.PlantStatus{PlantID: a.cfg.PlantID, Mode: snap.Mode, Filter: snap, WashPhase: phase, BlowerPhase: a.blower.Phase(), InterlockOK: decision.Allowed, ActiveAlarms: a.alarms.ActiveCount(), UpdatedAt: now}
}

func (a *App) Alarms() []model.AlarmEvent { return a.alarms.ListActive() }
func (a *App) Telemetry(n int) []model.TelemetryPoint { return a.telem.Recent(n) }

func (a *App) RequestBackwash(req model.BackwashRequest) (model.BackwashResult, error) {
	if req.OperationID == "" {
		req.OperationID = model.OperationID(fmt.Sprintf("BW-%d", time.Now().Unix()))
	}
	if req.IssuedAt.IsZero() {
		req.IssuedAt = time.Now().UTC()
	}
	if req.AirRateCMS <= 0 {
		req.AirRateCMS = a.cfg.DefaultAirCMS
	}
	if req.WaterRateCMS <= 0 {
		req.WaterRateCMS = a.cfg.DefaultWaterCMS
	}
	if err := a.cells.Lock(req.CellID, req.OperationID); err != nil {
		return model.BackwashResult{}, err
	}
	defer a.cells.Unlock(req.CellID)
	snap := a.store.Snapshot(time.Now().UTC())
	decision := a.cells.Evaluate(snap, req.CellID)
	if !decision.Allowed {
		a.alarms.Raise("INTERLOCK", fmt.Sprintf("%s: %v", decision.Code, decision.Reasons), a.cfg.PlantID, model.SeverityCritical)
		return model.BackwashResult{OperationID: req.OperationID, Accepted: false, Message: decision.Code}, fmt.Errorf("interlock denied: %s", decision.Code)
	}
	cell, ok := a.book.Cell(req.CellID)
	if !ok {
		return model.BackwashResult{}, model.ErrInvalidSample
	}
	sample := model.HeadSample{CellID: req.CellID, Meters: cell.HeadM, Quality: 0.9, At: req.IssuedAt}
	var profile model.MediaProfile
	for _, m := range snap.Media {
		if m.FilterID == req.FilterID {
			profile = m
			break
		}
	}
	if err := a.guard.Evaluate(req.CellID, profile, sample); err != nil {
		return model.BackwashResult{}, err
	}
	res, err := a.coord.Run(a.activeCtx(), req)
	if err != nil {
		return res, err
	}
	a.aeration.SuspendForWash(req.CellID)
	a.book.SetMode(model.PlantModeBackwash)
	_ = a.book.SetWashPhase(req.CellID, res.Phase, res.CompletedAt)
	if m, ok := a.machines[req.CellID]; ok {
		m.SetPhase(res.Phase)
	}
	a.washWin.Open()
	a.syncStore()
	return res, nil
}

func (a *App) TransitionWash(req model.WashTransitionRequest) (model.WashTransitionResult, error) {
	if req.At.IsZero() {
		req.At = time.Now().UTC()
	}
	var cellID model.CellID
	for _, spec := range a.cfg.Cells {
		if spec.FilterID == req.FilterID {
			cellID = spec.ID
			break
		}
	}
	m, ok := a.machines[cellID]
	if !ok {
		return model.WashTransitionResult{}, fmt.Errorf("no wash machine for filter %s", req.FilterID)
	}
	res, err := m.Transition(a.activeCtx(), req)
	if err != nil {
		return res, err
	}
	for _, spec := range a.cfg.Cells {
		if spec.FilterID == req.FilterID {
			_ = a.book.SetWashPhase(spec.ID, res.Phase, req.At)
		}
	}
	if res.Phase == model.WashComplete {
		a.washWin.Close()
		a.book.SetMode(model.PlantModeStandby)
		a.aeration.RestoreAfterWash(cellID, 65)
		a.valves.Close(cellID)
	}
	a.syncStore()
	return res, nil
}

func (a *App) EvaluateWashWindow() (bool, string) { return a.washWin.Evaluate(a.store.Snapshot(time.Now().UTC())) }
func (a *App) EmergencyStop() error {
	a.cells.SetEStop(true)
	a.book.SetMode(model.PlantModeEStop)
	a.blower.Idle()
	a.alarms.RaiseEStop(a.cfg.PlantID)
	a.cancel()
	a.mu.Lock()
	a.rootCtx, a.cancel = context.WithCancel(context.Background())
	a.mu.Unlock()
	return nil
}
func (a *App) ClearEmergencyStop() error {
	a.cells.SetEStop(false)
	a.book.SetMode(model.PlantModeStandby)
	a.blower.Idle()
	_ = a.alarms.Clear("ESTOP")
	return nil
}
func (a *App) activeCtx() context.Context { a.mu.Lock(); defer a.mu.Unlock(); return a.rootCtx }
func (a *App) syncStore() {
	now := time.Now().UTC()
	snap := a.store.Snapshot(now)
	a.store.ReplaceAll(a.book.AllCells(), snap.Media, a.book.Mode())
}
func (a *App) ProcessClock() *clock.ProcessClock { return a.procClock }
func (a *App) WashWindow() *backwash.Window { return a.washWin }
func (a *App) Coordinator() *backwash.Coordinator { return a.coord }
func (a *App) Cells() *interlock.Cells { return a.cells }
func (a *App) Store() *store.FilterStore { return a.store }
func (a *App) Machine(cell model.CellID) *fsm.WashMachine { return a.machines[cell] }
func (a *App) AdvanceProcess(d time.Duration) { a.procClock.Advance(d) }
func (a *App) Blower() *blower.Controller { return a.blower }

func (a *App) TouchMediaAfterWash(filter model.FilterID, at time.Time) error {
	snap := a.store.Snapshot(at)
	for _, p := range snap.Media {
		if p.FilterID != filter {
			continue
		}
		cp := p
		a.mediaProbe.Touch(&cp, at)
		a.store.UpdateMedia(cp)
		return nil
	}
	return errors.New("filter not found")
}
