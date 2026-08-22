package blower

import (
	"context"
	"testing"
)

func TestBlowerPurgeRespectsCancel(t *testing.T) {
	c := NewCooler(12)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.PurgeRamp(ctx, 8)
	if err == nil {
		t.Fatal("expected purge ramp to stop on cancelled context")
	}
	if c.ValveOpen() >= 1 {
		t.Fatalf("cancelled purge still reached full open: %.2f", c.ValveOpen())
	}
}
