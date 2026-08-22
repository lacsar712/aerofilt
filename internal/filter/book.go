package filter

import (
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type Book struct {
	mu    sync.RWMutex
	cells map[model.CellID]model.FilterCell
	mode  model.PlantMode
	plant model.PlantID
}

func NewBook(plant model.PlantID, cells []model.FilterCell) *Book {
	m := make(map[model.CellID]model.FilterCell, len(cells))
	for _, c := range cells {
		m[c.ID] = c
	}
	return &Book{cells: m, mode: model.PlantModeStandby, plant: plant}
}

func (b *Book) PlantID() model.PlantID { return b.plant }
func (b *Book) Mode() model.PlantMode {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.mode
}
func (b *Book) SetMode(mode model.PlantMode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mode = mode
}
func (b *Book) AllCells() []model.FilterCell {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]model.FilterCell, 0, len(b.cells))
	for _, c := range b.cells {
		out = append(out, c)
	}
	return out
}
func (b *Book) UpdateHead(sample model.HeadSample) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.cells[sample.CellID]
	if !ok {
		return model.ErrInvalidSample
	}
	if !c.Online {
		return model.ErrCellOffline
	}
	c.HeadM = sample.Meters
	b.cells[sample.CellID] = c
	return nil
}
func (b *Book) SetWashPhase(cell model.CellID, phase model.WashPhase, at time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.cells[cell]
	if !ok {
		return model.ErrInvalidSample
	}
	c.WashPhase = phase
	if phase == model.WashComplete {
		c.LastWashAt = at
	}
	b.cells[cell] = c
	return nil
}
func (b *Book) SetAeration(cell model.CellID, pct float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.cells[cell]
	if !ok {
		return model.ErrInvalidSample
	}
	c.AerationPct = pct
	b.cells[cell] = c
	return nil
}
func (b *Book) Cell(id model.CellID) (model.FilterCell, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c, ok := b.cells[id]
	return c, ok
}
func (b *Book) Snapshot(at time.Time) model.FilterSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cells := make([]model.FilterCell, 0, len(b.cells))
	for _, c := range b.cells {
		cells = append(cells, c)
	}
	return model.FilterSnapshot{PlantID: b.plant, Mode: b.mode, Cells: cells, TakenAt: at, HeadAvgM: model.AvgHead(cells)}
}
