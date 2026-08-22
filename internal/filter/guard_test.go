package filter_test

import (
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/blower"
	"github.com/lacsar712/aerofilt/internal/filter"
	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestGuardEvaluate(t *testing.T) {
	book := filter.NewBook("plant", []model.FilterCell{{ID: "cell-a", Online: true, HeadM: 1.5}})
	guard := filter.NewGuard(book, level.NewMonitor(0.5), media.NewProbe(0.9, 0.3, 0.6, 1.0), blower.NewController(100, 10), 0.5, 3.0)
	if err := guard.Evaluate("cell-a", model.MediaProfile{FilterID: "f1", BedDepthM: 1.5, VoidRatio: 0.4, ClogIndex: 0.3}, model.HeadSample{CellID: "cell-a", Meters: 1.5, Quality: 0.9}); err != nil {
		t.Fatal(err)
	}
}

func TestSetWashPhase(t *testing.T) {
	book := filter.NewBook("plant", []model.FilterCell{{ID: "c", Online: true}})
	_ = book.SetWashPhase("c", model.WashComplete, time.Now().UTC())
	c, _ := book.Cell("c")
	if c.WashPhase != model.WashComplete {
		t.Fatal("phase not set")
	}
}