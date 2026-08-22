package manifold_test

import (
	"context"
	"testing"

	"github.com/lacsar712/aerofilt/internal/manifold"
	"github.com/lacsar712/aerofilt/internal/model"
)

func TestValveBankOpen(t *testing.T) {
	bank := manifold.NewValveBank(map[model.CellID][]model.ValveID{"cell-a": {"v1", "v2", "v3"}})
	n, err := bank.Open(context.Background(), "cell-a", model.WashDraining)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
