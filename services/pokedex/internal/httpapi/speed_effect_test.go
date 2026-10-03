package httpapi_test

// ADR-0139 §4: 素早さの補正(SpeedMods・IgnoresParalysisSpeedDrop)は持ち物・特性の effect の中の項目として運ぶ。
//   - 内部 API(getMasterExport)は item_effects / ability_effects の JSON をそのまま返す(MasterEffect は
//     additionalProperties: true の自由形式なので、契約 api/openapi.yaml は変えない)。判定はここから読む。
//   - 公開 API(searchItems)の effect も同じ値・同じ形(ADR-0218)。返す前の共通マスタの検証を通る。
//   - 素早さの項目だけの持ち物は roles が空(ダメージ計算の役割にならない。ADR-0175 §1)。
// データはすべて架空。

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// speedEffectQuerier は storetest.New() に素早さの補正を持つ持ち物・特性を足す。
//   - testspeedball: 素早さの項目だけの持ち物(使用可能集合に入れる)
//   - testorb: 既存の DamageMod に素早さの項目を足す
//   - teststance: 素早さの項目だけの特性
func speedEffectQuerier() *storetest.Querier {
	q := storetest.New()
	q.Items = append(q.Items, store.Item{ID: "testspeedball", NameJa: "テストすばやさだま", NameJaSource: "pokeapi", NameEn: "Test Speed Ball"})
	for i, e := range q.ItemEffects {
		if e.ItemID == "testorb" {
			q.ItemEffects[i].Effect = json.RawMessage(`{"DamageMod": 5324, "SpeedMods": [{"Condition": "weather_sun", "Modifier": 6144}]}`)
		}
	}
	q.ItemEffects = append(q.ItemEffects,
		store.ItemEffect{ItemID: "testspeedball", Effect: json.RawMessage(`{"SpeedMods": [{"Condition": "always", "Modifier": 2048}]}`)},
	)
	q.AbilityEffects = append(q.AbilityEffects,
		store.AbilityEffect{AbilityID: "teststance", Effect: json.RawMessage(
			`{"SpeedMods": [{"Condition": "has_status", "Modifier": 6144}], "IgnoresParalysisSpeedDrop": true}`)},
	)
	reg := q.RegulationItems[storetest.DefaultRegulationID]
	q.RegulationItems[storetest.DefaultRegulationID] = append(slices.Clone(reg), "testspeedball")
	return q
}

func decodeWithNumbers(t *testing.T, s string) any {
	t.Helper()
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMasterExportCarriesSpeedEffects(t *testing.T) {
	h := newHandler(t, speedEffectQuerier())
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	// 契約は変えない: 素早さの項目を含む effect が今の MasterEffect のまま検証を通る。
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)

	raw := rawExport(t, rec.Body.Bytes())
	items, ok := raw["items"].([]any)
	if !ok {
		t.Fatal("items が配列でない")
	}
	abilities, ok := raw["abilities"].([]any)
	if !ok {
		t.Fatal("abilities が配列でない")
	}
	tests := []struct {
		name string
		list []any
		id   string
		want string
	}{
		{"素早さだけの持ち物", items, "testspeedball", `{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`},
		{"ダメージと素早さの持ち物", items, "testorb", `{"DamageMod":5324,"SpeedMods":[{"Condition":"weather_sun","Modifier":6144}]}`},
		{"素早さだけの特性", abilities, "teststance", `{"SpeedMods":[{"Condition":"has_status","Modifier":6144}],"IgnoresParalysisSpeedDrop":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findByID(t, tt.list, tt.id)
			if want := decodeWithNumbers(t, tt.want); !reflect.DeepEqual(got["effect"], want) {
				t.Errorf("effect = %#v, want %#v", got["effect"], want)
			}
		})
	}
}

func TestSearchItemsCarriesSpeedEffects(t *testing.T) {
	rec := do(t, newHandler(t, speedEffectQuerier()), http.MethodGet, itemsPath+"?limit=200", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d(素早さの項目を含む効果も共通マスタの検証を通る)\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, itemsPath+"?limit=200", true, rec)
	items := rawItems(t, rec.Body.Bytes())

	ball, ok := items["testspeedball"]
	if !ok {
		t.Fatalf("testspeedball が応答に無い: %s", rec.Body.String())
	}
	var want any // rawItems は json.Unmarshal(数値は float64)で読むので、同じ読み方で比べる
	if err := json.Unmarshal([]byte(`{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ball["effect"], want) {
		t.Errorf("testspeedball の effect = %#v, want %#v(内部 API と同じ値・同じ形)", ball["effect"], want)
	}
	if got := rolesOf(t, ball); len(got) != 0 {
		t.Errorf("testspeedball の roles = %v, want [](素早さはダメージ計算の役割にならない)", got)
	}
	if got := rolesOf(t, items["testorb"]); !reflect.DeepEqual(got, []string{"attacker"}) {
		t.Errorf("testorb の roles = %v, want [attacker](素早さの項目を足しても役割は変わらない)", got)
	}
}
