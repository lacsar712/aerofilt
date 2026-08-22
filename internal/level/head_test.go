package level_test

import (
	"testing"

	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestHeadLow(t *testing.T) {
	err := level.NewMonitor(0.5).Validate(model.HeadSample{Meters: 0.2, Quality: 0.9}, 0.8, 3.0)
	if !level.IsLow(err) {
		t.Fatalf("expected low: %v", err)
	}
}
