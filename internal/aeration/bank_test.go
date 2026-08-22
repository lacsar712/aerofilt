package heater

import (
	"context"
	"testing"

	"github.com/lacsar712/aerofilt/internal/model"
)

func TestHeaterRampRespectsCancel(t *testing.T) {
	em := NewCountingEmitter(120)
	b := NewBank(120, em)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := b.RampPower(ctx, model.ZoneID("center"), 90, 10)
	if err == nil {
		t.Fatal("expected heater ramp to stop on cancelled context")
	}
	if em.HeaterCount() != 0 {
		t.Fatalf("cancelled ramp still emitted heater steps: %d", em.HeaterCount())
	}
	if b.TotalKW() != 0 {
		t.Fatalf("cancelled ramp still raised bank power to %.1f", b.TotalKW())
	}
}
