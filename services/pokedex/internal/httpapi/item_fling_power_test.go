package httpapi_test

// 持ち物のなげつける威力(items.fling_power)を内部 API・公開 API に出すことのテスト(ADR-0144 §6)。
//
// 受け入れ条件(pokedex-svc):
//   - 内部 API(getMasterExport)の MasterItem.flingPower は items.fling_power。NULL はキーを省く。応答は契約に合う。
//   - 公開 API(searchItems)の Item.flingPower も同じ値。NULL はキーを省く(null を書かない)。
//
// データは架空(storetest)。値はこのテストで q に入れる。

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func querierWithFlingPower(t *testing.T) *storetest.Querier {
	t.Helper()
	q := storetest.New()
	found := false
	for i := range q.Items {
		if q.Items[i].ID == "testorb" {
			q.Items[i].FlingPower = sql.NullInt16{Int16: 30, Valid: true}
			found = true
		}
	}
	if !found {
		t.Fatal("storetest に testorb が無い")
	}
	return q
}

func TestSearchItemsCarriesFlingPower(t *testing.T) {
	rec := do(t, newHandler(t, querierWithFlingPower(t)), http.MethodGet, itemsPath+"?limit=200", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, itemsPath+"?limit=200", true, rec)
	items := rawItems(t, rec.Body.Bytes())
	if len(items) < 2 {
		t.Fatalf("持ち物が 2 件未満: %s", rec.Body.String())
	}
	for id, item := range items {
		v, ok := item["flingPower"]
		if id == "testorb" {
			if !ok || v != float64(30) {
				t.Errorf("testorb.flingPower = %v(キー %v), want 30", v, ok)
			}
			continue
		}
		if ok {
			t.Errorf("%s.flingPower = %v, want キーなし(NULL)", id, v)
		}
	}
}

func TestMasterExportCarriesFlingPower(t *testing.T) {
	rec := do(t, newHandler(t, querierWithFlingPower(t)), http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	raw := rawExport(t, rec.Body.Bytes())
	seen := false
	for _, v := range raw["items"].([]any) {
		it := v.(map[string]any)
		fp, ok := it["flingPower"]
		if it["id"] == "testorb" {
			seen = true
			if !ok || fp != json.Number("30") { // rawExport は UseNumber
				t.Errorf("testorb.flingPower = %v(キー %v), want 30", fp, ok)
			}
			continue
		}
		if ok {
			t.Errorf("%v.flingPower = %v, want キーなし(NULL)", it["id"], fp)
		}
	}
	if !seen {
		t.Fatal("内部 API に testorb が無い")
	}
}
