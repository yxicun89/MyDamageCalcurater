package master

// 持ち物のなげつける威力(MasterItem.flingPower。ADR-0144 §6)の取り込みのテスト。
//
//   - FromExport は flingPower を engine.Item.FlingPower に写す。キーが無い・null(古い pokedex-svc・投げられない持ち物)は 0(不明)。
//   - 0 以下の値は契約(minimum: 1)の外なので ErrInvalidMaster(黙って 0 にしない)。

import (
	"errors"
	"testing"
)

func TestFromExportMapsFlingPower(t *testing.T) {
	ex := baseExport(t)
	for i := range ex.Items {
		if ex.Items[i].Id == "testplainitem" {
			ex.Items[i].FlingPower = intPtr(130)
		}
	}
	store := newStore(t, ex)
	for id, want := range map[string]int{"testplainitem": 130, "testorb": 0} {
		it, ok := store.Item(id)
		if !ok {
			t.Fatalf("Item(%s) が見つからない", id)
		}
		if it.FlingPower != want {
			t.Errorf("Item(%s).FlingPower = %d, want %d(省略は 0 = 不明)", id, it.FlingPower, want)
		}
	}
}

func TestFromExportRejectsNonPositiveFlingPower(t *testing.T) {
	for _, v := range []int{0, -1} {
		ex := baseExport(t)
		ex.Items[0].FlingPower = intPtr(v)
		if _, err := FromExport(ex); !errors.Is(err, ErrInvalidMaster) {
			t.Errorf("flingPower %d: err = %v, want ErrInvalidMaster", v, err)
		}
	}
}
