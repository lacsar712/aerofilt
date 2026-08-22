package media_test

import (
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestProbeClogged(t *testing.T) {
	p := media.NewProbe(0.8, 0.35, 0.55, 1.0)
	err := p.Evaluate(model.MediaProfile{FilterID: "f1", BedDepthM: 1.5, VoidRatio: 0.4, ClogIndex: 0.9})
	if !media.IsClogged(err) {
		t.Fatalf("expected clogged: %v", err)
	}
}

func TestProbeTouch(t *testing.T) {
	p := media.NewProbe(0.8, 0.35, 0.55, 1.0)
	prof := model.MediaProfile{ClogIndex: 0.8}
	p.Touch(&prof, time.Now().UTC())
	if prof.ClogIndex >= 0.8 {
		t.Fatal("clog index should decrease")
	}
}