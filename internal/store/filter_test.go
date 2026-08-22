package store_test

import (
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
	"github.com/lacsar712/aerofilt/internal/store"
)

func TestSnapshotDeepCopy(t *testing.T) {
	st := store.NewFilterStore("plant", []model.FilterCell{{ID: "cell-a", FilterID: "f1", HeadM: 1.5, Online: true}}, nil)
	snap := st.Snapshot(time.Now().UTC())
	snap.Cells[0].HeadM = 9.9
	if st.Snapshot(time.Now().UTC()).Cells[0].HeadM == 9.9 {
		t.Fatal("snapshot not deep copied")
	}
}
