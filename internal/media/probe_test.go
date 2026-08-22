package media

import (
	"errors"
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

func TestRecoverableMediaFaultChain(t *testing.T) {
	p := NewProbe(0.8, 10)
	sample := model.TempSample{
		ZoneID: "crown", SensorID: "t1", Celsius: 1180, Quality: 0.6,
		At: time.Now().UTC(), Source: "test",
	}
	err := p.ValidateSample(sample, 1180)
	if err == nil {
		t.Fatal("expected recoverable fault")
	}
	if !errors.Is(err, RecoverableMediaFault) {
		t.Fatalf("errors.Is failed: %v classify=%s", err, Classify(err))
	}
	wrapped := Wrap(err, "probe check")
	if !errors.Is(wrapped, RecoverableMediaFault) {
		t.Fatal("wrap broke RecoverableMediaFault chain")
	}
}
