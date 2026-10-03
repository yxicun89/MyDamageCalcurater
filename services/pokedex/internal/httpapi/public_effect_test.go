package httpapi_test

// 公開 API の Item / Ability に効果定義(effect)を載せる(issue #211・ADR-0218)。
// 対象は Item / Ability を返す公開の操作すべて: searchItems(Item)と getSpecies(SpeciesDetail.abilities)。
// 値は内部 API(getMasterExport)の MasterItem.effect / MasterAbility.effect と同じもの(同じ DB の行を同じ方法で読む)。
// 効果を持たない持ち物・特性は effect キーを省く(null にしない)。返す前に共通マスタ
// (services/internal/master の DecodeItemEffect / DecodeAbilityEffect)で検証し、通らない効果は応答に出さず
// 503 master_unavailable にする。検証するのは「その応答に載る行」だけ(載らない行の不正で検索を止めない)。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// decodeRawList は配列の本文を数値の字面を保って読む(json.Number。float64 を経由しない)。
func decodeRawList(t *testing.T, body []byte) []any {
	t.Helper()
	var list []any
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&list); err != nil {
		t.Fatalf("配列として読めない: %v\nbody=%s", err, body)
	}
	return list
}

// decodeRawJSON は期待値の JSON を json.Number で読む(比較用)。
func decodeRawJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("期待値の JSON を読めない: %v\n%s", err, s)
	}
	return v
}

// assertEffect は obj の effect を確かめる。want が "" ならキーが無いこと(null も不可)。
func assertEffect(t *testing.T, obj map[string]any, want string) {
	t.Helper()
	got, ok := obj["effect"]
	if want == "" {
		if ok {
			t.Errorf("%v: 効果を持たないのに effect キーがある(省くこと。null も不可): %#v", obj["id"], got)
		}
		return
	}
	if !ok {
		t.Fatalf("%v: effect キーが無い(want %s)", obj["id"], want)
	}
	if w := decodeRawJSON(t, want); !reflect.DeepEqual(got, w) {
		t.Errorf("%v: effect = %#v, want %#v", obj["id"], got, w)
	}
}

func getOK(t *testing.T, h http.Handler, target string) []byte {
	t.Helper()
	rec := do(t, h, http.MethodGet, target, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200\nbody=%s", target, rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, target, true, rec)
	return rec.Body.Bytes()
}

// AC-E1: searchItems の各持ち物は、効果を持てば item_effects の JSON をそのまま effect に持ち、持たなければ effect を省く。
func TestSearchItemsCarriesEffect(t *testing.T) {
	h := newHandler(t, storetest.New())
	list := decodeRawList(t, getOK(t, h, "/api/pokedex/items"))

	tests := []struct{ name, id, want string }{
		{"倍率の効果", "testorb", `{"DamageMod":5324}`},
		{"半減の実の効果", "testberry", `{"ResistBerryType":"fire"}`},
		{"効果を持たない持ち物(メガストーン)", "teststone", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEffect(t, findByID(t, list, tt.id), tt.want)
		})
	}

	// 生成型(api.Item)で未知のフィールドなしに読める(effect は契約の項目)。
	var items []api.Item
	decodeStrict(t, getOK(t, h, "/api/pokedex/items"), &items)
	for _, it := range items {
		if it.Id == "teststone" && it.Effect != nil {
			t.Errorf("teststone の Effect = %v, want nil", *it.Effect)
		}
		if it.Id == "testorb" && it.Effect == nil {
			t.Errorf("testorb の Effect が nil")
		}
	}
}

// AC-E2: getSpecies の abilities の各特性は、効果を持てば ability_effects の JSON をそのまま effect に持ち、
// 持たなければ effect を省く(slot 順・id・nameJa は従来どおり)。
func TestGetSpeciesAbilitiesCarryEffect(t *testing.T) {
	h := newHandler(t, storetest.New())

	tests := []struct {
		key  string
		want []struct{ id, effect string }
	}{
		{"9001-000", []struct{ id, effect string }{
			{"testblaze", `{"OffBoostType":"fire","OffBoostTypeMod":6144}`},
			{"testguard", `{"DefResistType":{"fire":2048,"water":2048}}`},
		}},
		{"9002-000", []struct{ id, effect string }{
			{"testleafy", `{"ReduceSuperEffective":3072}`},
			{"testguard", `{"DefResistType":{"fire":2048,"water":2048}}`},
			{"testhidden", ""},
			{"testspecial", ""},
		}},
		{"9001-001", []struct{ id, effect string }{
			{"teststance", ""},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			var detail map[string]any
			dec := json.NewDecoder(strings.NewReader(string(getOK(t, h, "/api/pokedex/species/"+tt.key))))
			dec.UseNumber()
			if err := dec.Decode(&detail); err != nil {
				t.Fatal(err)
			}
			abilities, ok := detail["abilities"].([]any)
			if !ok || len(abilities) != len(tt.want) {
				t.Fatalf("abilities = %#v, want %d 件", detail["abilities"], len(tt.want))
			}
			for i, w := range tt.want {
				a := abilities[i].(map[string]any)
				if a["id"] != w.id {
					t.Fatalf("abilities[%d].id = %v, want %s(slot 順)", i, a["id"], w.id)
				}
				assertEffect(t, a, w.effect)
			}
		})
	}
}

// AC-E3(parity): 公開 API の effect は内部 API(getMasterExport)の同じ ID の effect と同じ値
// (内部の null ⇔ 公開ではキーなし)。同じ Querier から両方を引いて比べる。
func TestPublicEffectMatchesMasterExport(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)

	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("内部 API: status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	ex := rawExport(t, rec.Body.Bytes())
	internalEffect := func(kind, id string) (any, bool) {
		list, ok := ex[kind].([]any)
		if !ok {
			t.Fatalf("内部 API の %s が配列でない", kind)
		}
		e := findByID(t, list, id)["effect"]
		return e, e != nil
	}
	compare := func(t *testing.T, kind string, obj map[string]any) {
		t.Helper()
		id := obj["id"].(string)
		want, wantHas := internalEffect(kind, id)
		got, gotHas := obj["effect"]
		if gotHas != wantHas || (wantHas && !reflect.DeepEqual(got, want)) {
			t.Errorf("%s %s: 公開 effect = %#v(キーあり=%v), 内部 effect = %#v", kind, id, got, gotHas, want)
		}
	}

	items := decodeRawList(t, getOK(t, h, fmt.Sprintf("/api/pokedex/items?limit=%d", 200)))
	if len(items) == 0 {
		t.Fatal("持ち物が0件(比べる対象が無い)")
	}
	for _, it := range items {
		compare(t, "items", it.(map[string]any))
	}

	seenAbility := 0
	for _, sp := range q.Species {
		var detail map[string]any
		dec := json.NewDecoder(strings.NewReader(string(getOK(t, h, "/api/pokedex/species/"+sp.Key))))
		dec.UseNumber()
		if err := dec.Decode(&detail); err != nil {
			t.Fatal(err)
		}
		for _, a := range detail["abilities"].([]any) {
			compare(t, "abilities", a.(map[string]any))
			seenAbility++
		}
	}
	if seenAbility == 0 {
		t.Fatal("特性が0件(比べる対象が無い)")
	}
}

// AC-E4: 応答に載る行の効果が共通マスタの厳格検証を通らなければ、効果を応答に出さず 503 master_unavailable
// (契約の Error)。検証は services/internal/master の DecodeItemEffect / DecodeAbilityEffect に委ねる
// (上限は engine.MaxEffectModifier。pokedex 側に別の規則を書かない)。
func TestPublicInvalidEffectIsUnavailable(t *testing.T) {
	overMax := fmt.Sprintf(`{"DamageMod": %d}`, engine.MaxEffectModifier+1)
	tests := []struct {
		name   string
		target string
		mutate func(q *storetest.Querier)
		leak   string // 応答本文に出てはならない字面
	}{
		{"持ち物: 未知のキー", "/api/pokedex/items", setItemEffect("testorb", `{"NoSuchField": 4096}`), "NoSuchField"},
		{"持ち物: キーの大文字小文字違い", "/api/pokedex/items", setItemEffect("testorb", `{"damageMod": 5324}`), "damageMod"},
		{"持ち物: 小数", "/api/pokedex/items", setItemEffect("testorb", `{"DamageMod": 5324.5}`), "5324.5"},
		{"持ち物: 上限超え(MaxEffectModifier+1)", "/api/pokedex/items", setItemEffect("testorb", overMax), fmt.Sprint(engine.MaxEffectModifier + 1)},
		{"持ち物: 0", "/api/pokedex/items", setItemEffect("testorb", `{"DamageMod": 0}`), `"DamageMod"`},
		{"持ち物: 空のオブジェクト", "/api/pokedex/items", setItemEffect("testorb", `{}`), `"effect"`},
		{"持ち物: 相性表に無いタイプ", "/api/pokedex/items", setItemEffect("testberry", `{"ResistBerryType": "nosuchtype"}`), "nosuchtype"},
		{"持ち物: 壊れた JSON", "/api/pokedex/items", setItemEffect("testorb", `{"DamageMod": `), `"effect"`},
		{"特性: 相性表に無いタイプ", "/api/pokedex/species/9001-000", setAbilityEffect("testblaze", `{"OffBoostType": "nosuchtype", "OffBoostTypeMod": 6144}`), "nosuchtype"},
		{"特性: 未知のキー", "/api/pokedex/species/9001-000", setAbilityEffect("testguard", `{"NoSuchField": true}`), "NoSuchField"},
		{"特性: 上限超え", "/api/pokedex/species/9002-000", setAbilityEffect("testleafy", fmt.Sprintf(`{"ReduceSuperEffective": %d}`, engine.MaxEffectModifier+1)), fmt.Sprint(engine.MaxEffectModifier + 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, tt.target, true)
			body := rec.Body.String()
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			validateAgainstContract(t, http.MethodGet, tt.target, true, rec)
			if strings.Contains(body, tt.leak) {
				t.Errorf("不正な効果の中身(%q)が応答に出た: %s", tt.leak, body)
			}
		})
	}
}

// AC-E5: 検証するのは応答に載る行だけ。載らない行(使用可能集合の外の持ち物・q に一致しない持ち物・
// 別の種族の特性)の効果が不正でも、その応答は 200 のまま。
func TestPublicEffectValidationOnlyCoversReturnedRows(t *testing.T) {
	tests := []struct {
		name   string
		target string
		mutate func(q *storetest.Querier)
	}{
		{"使用可能集合の外の持ち物", "/api/pokedex/items", setItemEffect("testplain", `{"NoSuchField": 1}`)},
		{"q に一致しない持ち物", "/api/pokedex/items?q=" + url.QueryEscape("テストだま"), setItemEffect("testberry", `{"NoSuchField": 1}`)},
		{"別の種族だけが持つ特性", "/api/pokedex/species/9001-000", setAbilityEffect("testunused", `{"NoSuchField": 1}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			getOK(t, newHandler(t, q), tt.target)
		})
	}
}

// AC-E6: 契約の検証が effect の形を見ている(空振りしない)。MasterEffect はオブジェクトなので、文字列・数値の
// effect は契約違反。オブジェクトの effect と、effect の無い持ち物は契約どおり。
func TestItemEffectContractCheckIsNotVacuous(t *testing.T) {
	const target = "/api/pokedex/items"
	h := newHandler(t, storetest.New())
	good := do(t, h, http.MethodGet, target, true)
	ct := good.Header().Get("Content-Type")
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{"effect がオブジェクト", `[{"id":"testorb","nameJa":"x","effect":{"DamageMod":5324}}]`, true},
		{"effect が無い", `[{"id":"teststone","nameJa":"x"}]`, true},
		{"effect が文字列", `[{"id":"testorb","nameJa":"x","effect":"DamageMod"}]`, false},
		{"effect が数値", `[{"id":"testorb","nameJa":"x","effect":5324}]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", ct)
			rec.WriteHeader(http.StatusOK)
			rec.WriteString(tt.body)
			err := responseContractError(contractRoute(t, http.MethodGet, target, true), rec)
			if tt.ok && err != nil {
				t.Errorf("契約どおりの応答を拒否した: %v", err)
			}
			if !tt.ok && err == nil {
				t.Errorf("契約に合わない応答(%s)が検証を通った", tt.body)
			}
		})
	}
}

func setItemEffect(id, raw string) func(q *storetest.Querier) {
	return func(q *storetest.Querier) {
		for i := range q.ItemEffects {
			if q.ItemEffects[i].ItemID == id {
				q.ItemEffects[i].Effect = json.RawMessage(raw)
				return
			}
		}
		q.ItemEffects = append(q.ItemEffects, store.ItemEffect{ItemID: id, Effect: json.RawMessage(raw)})
	}
}

func setAbilityEffect(id, raw string) func(q *storetest.Querier) {
	return func(q *storetest.Querier) {
		for i := range q.AbilityEffects {
			if q.AbilityEffects[i].AbilityID == id {
				q.AbilityEffects[i].Effect = json.RawMessage(raw)
				return
			}
		}
		q.AbilityEffects = append(q.AbilityEffects, store.AbilityEffect{AbilityID: id, Effect: json.RawMessage(raw)})
	}
}
