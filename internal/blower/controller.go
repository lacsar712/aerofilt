package blower

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

var ErrTripped = errors.New("blower protection tripped")

type Controller struct {
	mu        sync.Mutex
	maxPct    float64
	phase     model.BlowerPhase
	actualPct float64
	tripped   bool
	ratePct   float64
}

func NewController(maxPct, ratePctPerSec float64) *Controller {
	if maxPct <= 0 {
		maxPct = 100
	}
	if ratePctPerSec <= 0 {
		ratePctPerSec = 25
	}
	return &Controller{maxPct: maxPct, ratePct: ratePctPerSec, phase: model.BlowerIdle}
}

func (c *Controller) Phase() model.BlowerPhase {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.phase
}

func (c *Controller) ActualPct() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.actualPct
}

func (c *Controller) Trip(reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tripped = true
	c.phase = model.BlowerTripped
	c.actualPct = 0
	return fmt.Errorf("%w: %s", ErrTripped, reason)
}

func (c *Controller) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tripped = false
	c.phase = model.BlowerIdle
	c.actualPct = 0
}

func (c *Controller) Run(ctx context.Context, cmd model.BlowerCommand) (model.BlowerResult, error) {
	c.mu.Lock()
	if c.tripped {
		c.mu.Unlock()
		return model.BlowerResult{OperationID: cmd.OperationID, Phase: model.BlowerTripped, Message: model.ErrBlowerTripped.Error(), CompletedAt: time.Now().UTC()}, fmt.Errorf("%w", model.ErrBlowerTripped)
	}
	target := cmd.TargetPct
	if target > c.maxPct {
		target = c.maxPct
	}
	if target < 0 {
		target = 0
	}
	c.phase = model.BlowerPriming
	c.mu.Unlock()
	_ = ctx

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if c.tripped {
				c.mu.Unlock()
				return model.BlowerResult{}, fmt.Errorf("%w", ErrTripped)
			}
			step := c.ratePct * 0.1
			if c.actualPct < target {
				c.actualPct += step
				if c.actualPct > target {
					c.actualPct = target
				}
				c.phase = model.BlowerRamping
			}
			if c.actualPct >= target {
				c.phase = model.BlowerRunning
				actual := c.actualPct
				c.mu.Unlock()
				return model.BlowerResult{OperationID: cmd.OperationID, Phase: model.BlowerRunning, TargetPct: target, ActualPct: actual, Message: "blower at target", CompletedAt: time.Now().UTC()}, nil
			}
			c.mu.Unlock()
		}
	}
}

func (c *Controller) Idle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tripped {
		return
	}
	c.phase = model.BlowerIdle
	c.actualPct = 0
}

func IsTripped(err error) bool {
	return errors.Is(err, ErrTripped) || errors.Is(err, model.ErrBlowerTripped)
}
