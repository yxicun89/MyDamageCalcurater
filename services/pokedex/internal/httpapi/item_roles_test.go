package httpapi_test

// 持ち物の役割(roles)・メガストーンの判定(isMegaStone)と、種族の詳細の基本種(baseSpeciesKey・baseSpeciesNameJa)の
// 公開 API のテスト(ADR-0175)。データは架空(storetest)。
//   - searchItems は各持ち物に roles(空配列可)と isMegaStone を常にキーごと返す(古いクライアントは無視できる省略可の項目)。
//   - roles は共通マスタの master.ItemRoles で導く(pokedex で1か所)。メガストーンは常に空配列。
//   - isMegaStone はいずれかのメガ種族の requiredItemId に現れるか(使用可能集合で絞らない)。
//   - getSpecies はメガ種族の基本種のキーと日本語名を返す(メガでなければ null。キーは常に出す)。
//   - 内部 API の MasterItem・read model は変えない(calc-svc・balance・speed の loader の互換)。

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

const itemsPath = "/api/pokedex/items"

// itemRolesQuerier は storetest.New() に、役割の組み合わせを網羅する架空の持ち物を足す。
//   - testvest: 防御・特防(防御側だけ)
//   - testboth: 攻撃と特防(両方)
//   - testnoeffect: 効果なし(どちらでもない)
//   - teststone2: 使用可能集合の外のメガ種族(9004-001)のストーン。効果を持たせても役割は空
//
// 既存の testorb(DamageMod → 攻撃側)・testberry(ResistBerryType → 防御側)・teststone(メガストーン)はそのまま使う。
func itemRolesQuerier() *storetest.Querier {
	q := storetest.New()
	q.Items = append(q.Items,
		store.Item{ID: "testvest", NameJa: "テストベスト", NameJaSource: "pokeapi", NameEn: "Test Vest"},
		store.Item{ID: "testboth", NameJa: "テストりょうほう", NameJaSource: "pokeapi", NameEn: "Test Both"},
		store.Item{ID: "testnoeffect", NameJa: "テストこうかなし", NameJaSource: "pokeapi", NameEn: "Test No Effect"},
		store.Item{ID: "teststone2", NameJa: "テストナイト2", NameJaSource: "fallback_en", NameEn: "Teststone Two"},
	)
	q.ItemEffects = append(q.ItemEffects,
		store.ItemEffect{ItemID: "testvest", Effect: json.RawMessage(`{"StatMods": {"def": 6144, "spd": 6144}}`)},
		store.ItemEffect{ItemID: "testboth", Effect: json.RawMessage(`{"StatMods": {"atk": 6144, "spd": 6144}}`)},
		store.ItemEffect{ItemID: "teststone2", Effect: json.RawMessage(`{"DamageMod": 5324}`)},
	)
	q.Species = append(q.Species,
		store.Species{Key: "9004-000", DexNo: 9004, Form: 0, ShowdownID: "testouter", NameJa: "テストソト", NameJaSource: "pokeapi", NameEn: "Testouter",
			Type1: "water", BaseHp: 70, BaseAtk: 70, BaseDef: 70, BaseSpa: 70, BaseSpd: 70, BaseSpe: 70},
		store.Species{Key: "9004-001", DexNo: 9004, Form: 1, ShowdownID: "testoutermega", NameJa: "テストメガソト", NameJaSource: "generated", NameEn: "Testouter-Mega",
			Type1: "water", BaseHp: 70, BaseAtk: 90, BaseDef: 90, BaseSpa: 90, BaseSpd: 90, BaseSpe: 90,
			IsMega: true, BaseSpeciesKey: sql.NullString{String: "9004-000", Valid: true}, RequiredItemID: sql.NullString{String: "teststone2", Valid: true}},
	)
	reg := q.RegulationItems[storetest.DefaultRegulationID]
	q.RegulationItems[storetest.DefaultRegulationID] = append(slices.Clone(reg), "testvest", "testboth", "testnoeffect", "teststone2")
	// 9004-000 / 9004-001 は使用可能集合に入れない(使えないメガのストーンも isMegaStone)。
	return q
}

// rawItems は searchItems の本文を、キーの有無が分かる形で読む。
func rawItems(t *testing.T, body []byte) map[string]map[string]any {
	t.Helper()
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("配列として読めない: %v\nbody=%s", err, body)
	}
	out := make(map[string]map[string]any, len(list))
	for _, it := range list {
		out[it["id"].(string)] = it
	}
	return out
}

func rolesOf(t *testing.T, item map[string]any) []string {
	t.Helper()
	raw, ok := item["roles"]
	if !ok {
		t.Fatalf("roles のキーが無い(pokedex-svc は空配列でも常に返す): %v", item)
	}
	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("roles が配列でない(null を返さない): %#v", raw)
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

// AC-1・AC-2: searchItems の各持ち物が、効果から導いた roles と isMegaStone を常にキーごと返す。
func TestSearchItemsReturnsRolesAndMegaStone(t *testing.T) {
	rec := do(t, newHandler(t, itemRolesQuerier()), http.MethodGet, itemsPath+"?limit=200", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, itemsPath+"?limit=200", true, rec)
	items := rawItems(t, rec.Body.Bytes())

	tests := []struct {
		id          string
		roles       []string
		isMegaStone bool
	}{
		{"testorb", []string{"attacker"}, false},              // DamageMod
		{"testberry", []string{"defender"}, false},            // ResistBerryType
		{"testvest", []string{"defender"}, false},             // StatMods def・spd
		{"testboth", []string{"attacker", "defender"}, false}, // StatMods atk・spd(並びは attacker → defender)
		{"testnoeffect", []string{}, false},                   // 効果なし
		{"teststone", []string{}, true},                       // 使用可能集合の中のメガ(9001-001)のストーン
		{"teststone2", []string{}, true},                      // 集合の外のメガ(9004-001)のストーン。効果があっても空
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			item, ok := items[tt.id]
			if !ok {
				t.Fatalf("%s が応答に無い(役割が空でも一覧には返す): %s", tt.id, rec.Body.String())
			}
			if got := rolesOf(t, item); !reflect.DeepEqual(got, tt.roles) {
				t.Errorf("roles = %v, want %v", got, tt.roles)
			}
			got, ok := item["isMegaStone"]
			if !ok {
				t.Fatalf("isMegaStone のキーが無い(pokedex-svc は常に返す): %v", item)
			}
			if got != tt.isMegaStone {
				t.Errorf("isMegaStone = %v, want %v", got, tt.isMegaStone)
			}
		})
	}
	// 使用可能集合の外の持ち物(testplain)は従来どおり返さない(役割の追加で集合が変わらない)。
	if _, ok := items["testplain"]; ok {
		t.Errorf("使用可能集合の外の testplain が返った")
	}
}

// AC-1: 応答の roles は共通マスタの master.ItemRoles と同じ値(pokedex で導く規則は1か所。ハンドラが別の規則を持たない)。
func TestSearchItemsRolesComeFromMasterItemRoles(t *testing.T) {
	q := itemRolesQuerier()
	rec := do(t, newHandler(t, q), http.MethodGet, itemsPath+"?limit=200", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	var items []api.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	chart := storetestChart(t, q)
	effects := map[string][]byte{}
	for _, e := range q.ItemEffects {
		effects[e.ItemID] = e.Effect
	}
	for _, it := range items {
		if it.Roles == nil || it.IsMegaStone == nil {
			t.Fatalf("%s: roles / isMegaStone が無い", it.Id)
		}
		row := master.ItemRow{ID: it.Id, NameJa: it.NameJa, Effect: effects[it.Id]}
		mapped, err := master.Item(row, chart)
		if err != nil {
			t.Fatalf("%s: master.Item: %v", it.Id, err)
		}
		want := master.ItemRoles(mapped.Effect, *it.IsMegaStone)
		got := make([]master.ItemRole, 0, len(*it.Roles))
		for _, r := range *it.Roles {
			got = append(got, master.ItemRole(r))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: roles = %v, want master.ItemRoles = %v", it.Id, got, want)
		}
	}
}

// storetestChart は偽の DB の相性表を共通マスタで engine の型にする(効果の検証と同じ表)。
func storetestChart(t *testing.T, q *storetest.Querier) engine.TypeChart {
	t.Helper()
	types := make([]master.TypeRow, 0, len(q.Types))
	for _, ty := range q.Types {
		types = append(types, master.TypeRow{ID: ty.ID, SortOrder: int(ty.SortOrder), NameJa: ty.NameJa})
	}
	rows := make([]master.TypeChartRow, 0, len(q.TypeChart))
	for _, c := range q.TypeChart {
		rows = append(rows, master.TypeChartRow{AttackType: c.AttackType, DefenseType: c.DefenseType, Code: int(c.Code)})
	}
	c, err := master.TypeChart(types, rows)
	if err != nil {
		t.Fatalf("TypeChart: %v", err)
	}
	return c
}

// AC-4: 契約の ItemRole の enum と、共通マスタの役割の全値が一致する(どちらかだけに値を足さない)。
func TestItemRoleEnumMatchesContract(t *testing.T) {
	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	ref, ok := doc.Components.Schemas["ItemRole"]
	if !ok || ref.Value == nil {
		t.Fatal("契約に ItemRole が無い")
	}
	var contract []string
	for _, v := range ref.Value.Enum {
		contract = append(contract, v.(string))
	}
	var roles []string
	for _, r := range master.AllItemRoles() {
		roles = append(roles, string(r))
	}
	if !reflect.DeepEqual(contract, roles) {
		t.Errorf("契約の ItemRole = %v, master.AllItemRoles = %v", contract, roles)
	}
	// 生成型の定数も同じ文字列(手書きの型を作らない。絶対ルール1)。
	if string(api.ItemRoleAttacker) != string(master.ItemRoleAttacker) || string(api.ItemRoleDefender) != string(master.ItemRoleDefender) {
		t.Errorf("生成型の定数 %q/%q と master の %q/%q が違う", api.ItemRoleAttacker, api.ItemRoleDefender, master.ItemRoleAttacker, master.ItemRoleDefender)
	}
}

// AC-6(互換): 足した項目は省略可(required に入れない)。古いサーバーの応答(項目なし)も新しい契約に合い、
// 古いクライアントは未知の項目を無視できる(additionalProperties: false にしない)。
func TestNewItemAndSpeciesFieldsAreOptionalInContract(t *testing.T) {
	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	item := doc.Components.Schemas["Item"].Value
	if !reflect.DeepEqual(item.Required, []string{"id", "nameJa"}) {
		t.Errorf("Item.required = %v, want [id nameJa](roles・isMegaStone を必須にしない)", item.Required)
	}
	for _, name := range []string{"roles", "isMegaStone"} {
		if _, ok := item.Properties[name]; !ok {
			t.Errorf("Item に %s が無い", name)
		}
	}
	if ap := item.AdditionalProperties.Has; ap != nil && !*ap {
		t.Error("Item が additionalProperties: false(古いクライアント・サーバーの互換を壊す)")
	}

	detail := doc.Components.Schemas["SpeciesDetail"].Value
	found := map[string]bool{}
	for _, part := range detail.AllOf {
		for _, r := range part.Value.Required {
			if r == "baseSpeciesKey" || r == "baseSpeciesNameJa" {
				t.Errorf("SpeciesDetail の %s が required(古いサーバーの応答が契約に合わなくなる)", r)
			}
		}
		for name, p := range part.Value.Properties {
			if name == "baseSpeciesKey" || name == "baseSpeciesNameJa" {
				found[name] = true
				if !p.Value.Nullable {
					t.Errorf("SpeciesDetail.%s が nullable でない(メガでなければ null)", name)
				}
			}
		}
	}
	for _, name := range []string{"baseSpeciesKey", "baseSpeciesNameJa"} {
		if !found[name] {
			t.Errorf("SpeciesDetail に %s が無い", name)
		}
	}
}

// AC-6(互換): 項目を持たない古いサーバーの応答も、新しい契約の Item として読める(クライアントの生成型が拒否しない)。
func TestOldServerItemWithoutRolesStillDecodes(t *testing.T) {
	var it api.Item
	if err := json.Unmarshal([]byte(`{"id":"testorb","nameJa":"テストだま"}`), &it); err != nil {
		t.Fatalf("古い形の Item を読めない: %v", err)
	}
	if it.Roles != nil || it.IsMegaStone != nil {
		t.Errorf("古い形の Item で roles / isMegaStone が埋まった: %+v", it)
	}
}

// AC-5: getSpecies はメガ種族の基本種のキーと日本語名を返す。メガでなければ null(キーは常に出す)。
// 実データで baseSpeciesKey が null に見えたのは、SpeciesDetail に項目が無く出していなかったため(ADR-0175 §3)。
func TestGetSpeciesBaseSpeciesFields(t *testing.T) {
	h := newHandler(t, itemRolesQuerier())
	tests := []struct {
		name              string
		key               string
		wantKey, wantName any // nil は JSON の null
	}{
		{"使用可能集合の中のメガ", "9001-001", "9001-000", "テストモン"},
		{"使用可能集合の外のメガ", "9004-001", "9004-000", "テストソト"},
		{"非メガ", "9001-000", nil, nil},
		{"レギュレーション外の非メガ", "9003-000", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := "/api/pokedex/species/" + tt.key
			rec := do(t, h, http.MethodGet, target, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
			}
			validateAgainstContract(t, http.MethodGet, target, true, rec)
			var raw map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			for _, f := range []struct {
				key  string
				want any
			}{{"baseSpeciesKey", tt.wantKey}, {"baseSpeciesNameJa", tt.wantName}} {
				got, ok := raw[f.key]
				if !ok {
					t.Errorf("%s のキーが無い(メガでなくても null で出す): %s", f.key, rec.Body.String())
					continue
				}
				if got != f.want {
					t.Errorf("%s = %v, want %v", f.key, got, f.want)
				}
			}
			// 既存の項目は変わらない(requiredItemId はキーが1つだけ。埋め込みとの重複が無い)。
			if _, ok := raw["requiredItemId"]; !ok {
				t.Errorf("requiredItemId のキーが無い")
			}
		})
	}
}

// AC-7: 内部 API の MasterItem には役割を足さない(calc-svc の loader は未知のフィールドを拒否する。ADR-0175 §5)。
func TestMasterExportItemsHaveNoRoleFields(t *testing.T) {
	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	mi := doc.Components.Schemas["MasterItem"].Value
	for _, name := range []string{"roles", "isMegaStone"} {
		if _, ok := mi.Properties[name]; ok {
			t.Errorf("MasterItem に %s がある(calc-svc の loader が拒否する)", name)
		}
	}
	rec := do(t, newHandler(t, itemRolesQuerier()), http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	ex := rawExport(t, rec.Body.Bytes())
	for _, v := range ex["items"].([]any) {
		m := v.(map[string]any)
		for _, name := range []string{"roles", "isMegaStone"} {
			if _, ok := m[name]; ok {
				t.Errorf("内部 API の持ち物 %v に %s がある", m["id"], name)
			}
		}
	}
}
