package aeration_test

import (
	"context"
	"testing"

	"github.com/lacsar712/aerofilt/internal/aeration"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestBankRamp(t *testing.T) {
	bank := aeration.NewBank([]model.FilterCell{{ID: "cell-a", Online: true, AerationPct: 50}})
	zid := model.ZoneID("zone-cell-a")
	_ = bank.SetTarget(zid, 80)
	if err := bank.Ramp(context.Background(), zid, 4); err != nil {
		t.Fatal(err)
	}
	z, ok := bank.Zone(zid)
	if !ok || z.ActualPct < 70 {
		t.Fatalf("zone=%+v", z)
	}
}