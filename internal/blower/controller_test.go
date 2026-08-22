package blower_test

import (
	"context"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/blower"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestBlowerRun(t *testing.T) {
	c := blower.NewController(100, 50)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := c.Run(ctx, model.BlowerCommand{OperationID: "b1", TargetPct: 50})
	if err != nil || res.Phase != model.BlowerRunning {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestBlowerTrip(t *testing.T) {
	c := blower.NewController(100, 50)
	_ = c.Trip("overcurrent")
	_, err := c.Run(context.Background(), model.BlowerCommand{TargetPct: 50})
	if !blower.IsTripped(err) {
		t.Fatalf("expected tripped: %v", err)
	}
}
