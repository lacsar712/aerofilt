package interlock

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type Cells struct {
	mu     sync.Mutex
	leases map[model.CellID]model.OperationLease
	ttl    time.Duration
	eStop  bool
	strict bool
}

func NewCells(ttl time.Duration, strict bool) *Cells {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &Cells{leases: make(map[model.CellID]model.OperationLease), ttl: ttl, strict: strict}
}

func (c *Cells) SetEStop(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eStop = on
}

func (c *Cells) EStop() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eStop
}

func (c *Cells) Lock(cell model.CellID, op model.OperationID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eStop {
		return model.ErrPlantEStop
	}
	now := time.Now().UTC()
	if lease, ok := c.leases[cell]; ok {
		if now.Before(lease.Expires) && lease.OperationID != op {
			return fmt.Errorf("%w: held by %s", model.ErrLeaseHeld, lease.Holder)
		}
	}
	c.leases[cell] = model.OperationLease{OperationID: op, Holder: string(op), Acquired: now, Expires: now.Add(c.ttl)}
	return nil
}

func (c *Cells) Unlock(cell model.CellID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.leases, cell)
}

func (c *Cells) Evaluate(snap model.FilterSnapshot, cell model.CellID) model.InterlockDecision {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eStop {
		return model.InterlockDecision{Allowed: false, Code: "ESTOP", Reasons: []string{"plant estop engaged"}}
	}
	if snap.Mode == model.PlantModeMaintenance {
		return model.InterlockDecision{Allowed: false, Code: "MAINT", Reasons: []string{"plant in maintenance"}}
	}
	for _, fc := range snap.Cells {
		if fc.ID != cell {
			continue
		}
		if !fc.Online {
			return model.InterlockDecision{Allowed: false, Code: "OFFLINE", Reasons: []string{model.ErrCellOffline.Error()}}
		}
		if fc.WashPhase != model.WashIdle && fc.WashPhase != model.WashComplete {
			if c.strict {
				return model.InterlockDecision{Allowed: false, Code: "BUSY", Reasons: []string{"cell already washing"}}
			}
		}
		return model.InterlockDecision{Allowed: true, Code: "OK"}
	}
	return model.InterlockDecision{Allowed: false, Code: "UNKNOWN", Reasons: []string{"cell not found"}}
}

func (c *Cells) Held(cell model.CellID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	lease, ok := c.leases[cell]
	if !ok {
		return false
	}
	return time.Now().UTC().Before(lease.Expires)
}
