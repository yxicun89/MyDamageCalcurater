package httpapi_test

// searchItems の isMegaStone が、取得元の持ち物データから導いた items.is_mega_stone も反映すること
// (ADR-0140・issue #607)。対応するメガ種族が取り込まれない(どの種族にも要求されない)ストーンも true で、
// 役割(roles)は空になる(ADR-0175 §1: メガストーンは常に空配列)。データは架空(storetest)。

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func TestSearchItemsIsMegaStoneFromItemColumn(t *testing.T) {
	q := storetest.New()
	q.Items = append(q.Items,
		store.Item{ID: "teststonelone", NameJa: "テストストーンひとり", NameJaSource: "fallback_en", NameEn: "Test Stone Lone", IsMegaStone: true},
	)
	// 効果を持たせても、ストーンなら役割は空。
	q.ItemEffects = append(q.ItemEffects, store.ItemEffect{ItemID: "teststonelone", Effect: json.RawMessage(`{"DamageMod": 5324}`)})
	reg := q.RegulationItems[storetest.DefaultRegulationID]
	q.RegulationItems[storetest.DefaultRegulationID] = append(slices.Clone(reg), "teststonelone")

	rec := do(t, newHandler(t, q), http.MethodGet, itemsPath+"?limit=200", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, itemsPath+"?limit=200", true, rec)
	items := rawItems(t, rec.Body.Bytes())
	item, ok := items["teststonelone"]
	if !ok {
		t.Fatalf("teststonelone が応答に無い: %s", rec.Body.String())
	}
	if item["isMegaStone"] != true {
		t.Errorf("isMegaStone = %v, want true(items.is_mega_stone が真)", item["isMegaStone"])
	}
	if got := rolesOf(t, item); len(got) != 0 {
		t.Errorf("roles = %v, want [](メガストーンは役割を持たない)", got)
	}
	// 列が偽で、メガ種族にも要求されない持ち物は従来どおり false。
	if items["testorb"]["isMegaStone"] != false {
		t.Errorf("testorb の isMegaStone = %v, want false", items["testorb"]["isMegaStone"])
	}
}
