package aeration

import (
	"context"
	"fmt"
	"sync"

	"github.com/lacsar712/aerofilt/internal/model"
)

type Bank struct {
	mu    sync.RWMutex
	zones map[model.ZoneID]model.AerationZone
}

func NewBank(cells []model.FilterCell) *Bank {
	zones := make(map[model.ZoneID]model.AerationZone, len(cells))
	for _, c := range cells {
		zid := model.ZoneID("zone-" + string(c.ID))
		zones[zid] = model.AerationZone{ID: zid, CellID: c.ID, TargetPct: c.AerationPct, ActualPct: c.AerationPct, Online: c.Online}
	}
	return &Bank{zones: zones}
}

func (b *Bank) SetTarget(zone model.ZoneID, pct float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	z, ok := b.zones[zone]
	if !ok {
		return fmt.Errorf("unknown zone %s", zone)
	}
	if pct < 0 { pct = 0 }
	if pct > 100 { pct = 100 }
	z.TargetPct = pct
	b.zones[zone] = z
	return nil
}

func (b *Bank) Ramp(ctx context.Context, zone model.ZoneID, steps int) error {
	if steps <= 0 { steps = 5 }
	for i := 0; i < steps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		b.mu.Lock()
		z, ok := b.zones[zone]
		if !ok {
			b.mu.Unlock()
			return fmt.Errorf("unknown zone %s", zone)
		}
		diff := z.TargetPct - z.ActualPct
		z.ActualPct += diff / float64(steps-i)
		b.zones[zone] = z
		b.mu.Unlock()
	}
	return nil
}

func (b *Bank) SuspendForWash(cell model.CellID) {
	b.mu.Lock(); defer b.mu.Unlock()
	for id, z := range b.zones {
		if z.CellID != cell { continue }
		z.TargetPct, z.ActualPct = 0, 0
		b.zones[id] = z
	}
}

func (b *Bank) RestoreAfterWash(cell model.CellID, pct float64) {
	b.mu.Lock(); defer b.mu.Unlock()
	for id, z := range b.zones {
		if z.CellID != cell { continue }
		z.TargetPct, z.ActualPct = pct, pct
		b.zones[id] = z
	}
}

func (b *Bank) Zones() []model.AerationZone {
	b.mu.RLock(); defer b.mu.RUnlock()
	out := make([]model.AerationZone, 0, len(b.zones))
	for _, z := range b.zones { out = append(out, z) }
	return out
}

func (b *Bank) Zone(id model.ZoneID) (model.AerationZone, bool) {
	b.mu.RLock(); defer b.mu.RUnlock()
	z, ok := b.zones[id]
	return z, ok
}