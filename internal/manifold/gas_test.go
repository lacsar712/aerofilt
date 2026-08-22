package pressure

import (
	"context"
	"testing"
)

func TestPressurizeRespectsCancel(t *testing.T) {
	c := NewController(200)
	_ = c.SetSetpoint(100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.PressurizeCtx(ctx, 100)
	if err == nil {
		t.Fatal("expected pressurize to stop on cancelled context")
	}
	if c.CurrentMPa() >= 100 {
		t.Fatalf("cancelled pressurize still reached setpoint: %.1f", c.CurrentMPa())
	}
}
