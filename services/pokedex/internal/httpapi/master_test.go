package httpapi_test

// 内部 API GET /internal/pokedex/master(getMasterExport。ADR-0204・ADR-0105 §2)のテスト。
// calc-svc の services/calc/internal/master.DecodeExport / FromExport が受け取る形そのものを返すこと。

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

const masterPath = "/internal/pokedex/master"

// decodeExport は 200 の本文を生成型 MasterExport に厳格に読む(未知のフィールド・後続のデータを拒否)。
func decodeExport(t *testing.T, body []byte) api.MasterExport {
	t.Helper()
	var ex api.MasterExport
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ex); err != nil {
		t.Fatalf("MasterExport として読めない: %v\nbody=%s", err, body)
	}
	if dec.More() {
		t.Fatalf("本文に後続のデータがある")
	}
	return ex
}

// rawExport は本文を数値の字面を保ったまま汎用の形で読む(effect の「そのまま」を確かめる用)。
func rawExport(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("JSON として読めない: %v", err)
	}
	return m
}

func findByID(t *testing.T, list []any, id string) map[string]any {
	t.Helper()
	for _, v := range list {
		m := v.(map[string]any)
		if m["id"] == id {
			return m
		}
	}
	t.Fatalf("id %q が無い", id)
	return nil
}

// AC-I1: ヘッダ無しで 200、本文は契約(MasterExport)どおり。
func TestMasterExportMatchesContract(t *testing.T) {
	h := newHandler(t, storetest.New())
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200(内部 API は端末ID/セッションID を要らない)\nbody=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	decodeExport(t, rec.Body.Bytes())
}

// AC-I2: 中身は DB の行のとおり。使用可能集合で絞らない(既定のレギュレーションの外の種族・技・持ち物・特性も入る)。
// species に showdownId、メガの3列、slot 順の特性。dataVersion は data_versions の source=version を source 昇順に「,」で連結。
func TestMasterExportContent(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	ex := decodeExport(t, rec.Body.Bytes())

	if ex.SchemaVersion != api.MasterExportSchemaVersionN1 {
		t.Errorf("schemaVersion = %d, want 1", ex.SchemaVersion)
	}
	wantVersion := "calc=test-calc-1,pokeapi=cafef00dcafef00dcafef00dcafef00dcafef00d,showdown=abad1deaabad1deaabad1deaabad1deaabad1dea"
	if ex.DataVersion != wantVersion {
		t.Errorf("dataVersion = %q, want %q", ex.DataVersion, wantVersion)
	}

	// 件数: DB の全行(絞らない)。
	counts := []struct {
		name      string
		got, want int
	}{
		{"types", len(ex.Types), len(q.Types)},
		{"typeChart", len(ex.TypeChart), len(q.TypeChart)},
		{"species", len(ex.Species), len(q.Species)},
		{"moves", len(ex.Moves), len(q.Moves)},
		{"items", len(ex.Items), len(q.Items)},
		{"abilities", len(ex.Abilities), len(q.Abilities)},
		{"natures", len(ex.Natures), len(q.Natures)},
	}
	for _, c := range counts {
		if c.got != c.want {
			t.Errorf("%s の件数 %d, want %d(使用可能集合で絞らない)", c.name, c.got, c.want)
		}
	}

	species := map[string]api.MasterSpecies{}
	for _, s := range ex.Species {
		species[s.Key] = s
	}
	if _, ok := species["9003-000"]; !ok {
		t.Errorf("既定のレギュレーションの外の種族 9003-000 が無い(絞らない)")
	}
	mon := species["9001-000"]
	if mon.ShowdownId != "testmon" || mon.NameJa != "テストモン" || mon.DexNo != 9001 || mon.Form != 0 {
		t.Errorf("9001-000 = %+v", mon)
	}
	if mon.Type1 != api.PokeTypeFire || mon.Type2 != nil {
		t.Errorf("9001-000 のタイプ = %v / %v, want fire / null", mon.Type1, mon.Type2)
	}
	if mon.BaseStats != (api.StatBlock{Hp: 80, Atk: 90, Def: 70, Spa: 100, Spd: 75, Spe: 85}) {
		t.Errorf("9001-000 の種族値 = %+v", mon.BaseStats)
	}
	if mon.IsMega || mon.BaseSpeciesKey != nil || mon.RequiredItemId != nil {
		t.Errorf("9001-000 はメガでない: %+v", mon)
	}
	wantAbilities := []api.MasterSpeciesAbility{{Slot: 1, AbilityId: "testblaze"}, {Slot: 3, AbilityId: "testguard"}}
	if !reflect.DeepEqual(mon.Abilities, wantAbilities) {
		t.Errorf("9001-000 の特性 = %+v, want slot 順 %+v", mon.Abilities, wantAbilities)
	}
	mega := species["9001-001"]
	if !mega.IsMega || mega.BaseSpeciesKey == nil || *mega.BaseSpeciesKey != "9001-000" ||
		mega.RequiredItemId == nil || *mega.RequiredItemId != "teststone" || mega.Type2 == nil || *mega.Type2 != api.PokeTypeWater {
		t.Errorf("9001-001(メガ)= %+v", mega)
	}
	leaf := species["9002-000"]
	if len(leaf.Abilities) != 4 || leaf.Abilities[3].Slot != 4 {
		t.Errorf("9002-000 の特性 = %+v, want slot 1〜4 の4件(内部 API は切り詰めない)", leaf.Abilities)
	}

	moves := map[string]api.MasterMove{}
	for _, m := range ex.Moves {
		moves[m.Id] = m
	}
	if got := moves["teststrike"]; got.Type != api.PokeTypeNormal || got.Category != api.Physical || got.Power != 40 || got.Priority != 1 || got.NameJa != "テストうちこみ" {
		t.Errorf("teststrike = %+v", got)
	}
	if _, ok := moves["testbanned"]; !ok {
		t.Errorf("使用可能集合の外の技 testbanned が無い(絞らない)")
	}

	natures := map[string]api.MasterNature{}
	for _, n := range ex.Natures {
		natures[n.Id] = n
	}
	if n := natures["testbrave"]; n.Plus == nil || *n.Plus != api.StatKeyAtk || n.Minus == nil || *n.Minus != api.StatKeySpe || n.NameJa != "テストゆうかん" {
		t.Errorf("testbrave = %+v", n)
	}
	if n := natures["testneutral"]; n.Plus != nil || n.Minus != nil {
		t.Errorf("無補正の testneutral の plus/minus が null でない: %+v", n)
	}

	var typeIDs []string
	for _, ty := range ex.Types {
		typeIDs = append(typeIDs, string(ty.Id))
		if ty.NameJa == "" || ty.SortOrder <= 0 {
			t.Errorf("types の行が不完全: %+v", ty)
		}
	}
	sort.Strings(typeIDs)
	if strings.Join(typeIDs, ",") != "fire,grass,normal,water" {
		t.Errorf("types = %v", typeIDs)
	}
}

// AC-I3: effect は item_effects / ability_effects の JSON をそのまま返す。行が無ければ effect キーは null(キーは省かない)。
// 数値の字面を保つ(float64 を経由して 5324.0 → 5324・2^53+1 → 2^53 のように変えない。calc-svc の DecodeExport は
// 字面を保って共通マスタの整数検査に渡すため、pokedex 側で丸めると検査をすり抜ける。ADR-0204 §2)。
func TestMasterExportEffectIsVerbatim(t *testing.T) {
	q := storetest.New()
	q.ItemEffects = append(q.ItemEffects, store.ItemEffect{ItemID: "testplain", Effect: json.RawMessage(`{"DamageMod": 5324.0, "BoostTypeMod": 9007199254740993}`)})
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	raw := rawExport(t, rec.Body.Bytes())
	items := raw["items"].([]any)
	abilities := raw["abilities"].([]any)

	tests := []struct {
		name string
		got  map[string]any
		want string // effect の期待値(JSON。null は行が無い)
	}{
		{"持ち物の効果", findByID(t, items, "testorb"), `{"DamageMod":5324}`},
		{"持ち物の効果(文字列の値)", findByID(t, items, "testberry"), `{"ResistBerryType":"fire"}`},
		{"効果の行が無い持ち物", findByID(t, items, "teststone"), `null`},
		{"数値の字面を保つ", findByID(t, items, "testplain"), `{"DamageMod":5324.0,"BoostTypeMod":9007199254740993}`},
		{"特性の効果(入れ子の map)", findByID(t, abilities, "testguard"), `{"DefResistType":{"water":2048,"fire":2048}}`},
		{"効果の行が無い特性", findByID(t, abilities, "testhidden"), `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect, ok := tt.got["effect"]
			if !ok {
				t.Fatalf("effect キーが無い(null でもキーは必須。ADR-0204 §2)")
			}
			var want any
			dec := json.NewDecoder(strings.NewReader(tt.want))
			dec.UseNumber()
			if err := dec.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(effect, want) {
				t.Errorf("effect = %#v, want %#v", effect, want)
			}
		})
	}
}

// AC-I4: DB に未投入・DB に接続できない・マスタとして不完全なときは 503 master_unavailable(契約どおりの Error)。
// 部分的なマスタを 200 で返さない。
func TestMasterExportUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *storetest.Querier)
	}{
		{"data_versions が空(未投入)", func(q *storetest.Querier) { q.DataVersions = nil }},
		{"types が空", func(q *storetest.Querier) { q.Types = nil }},
		{"natures が空(000006 の後に未投入)", func(q *storetest.Querier) { q.Natures = nil }},
		{"DB に接続できない", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"一部のクエリが失敗", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListAbilityEffects": storetest.ErrDB}
		}},
		{"行が無い(sql.ErrNoRows)", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListSpecies": sql.ErrNoRows}
		}},
		{"効果の JSON が壊れている", func(q *storetest.Querier) {
			q.ItemEffects = []store.ItemEffect{{ItemID: "testorb", Effect: json.RawMessage(`{"DamageMod":`)}}
		}},
		{"効果の JSON がオブジェクトでない", func(q *storetest.Querier) {
			q.AbilityEffects = []store.AbilityEffect{{AbilityID: "testguard", Effect: json.RawMessage(`[1]`)}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, masterPath, false)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
			if strings.Contains(rec.Body.String(), storetest.ErrDB.Error()) {
				t.Errorf("DB の内部エラーの文言を応答に出している: %s", rec.Body.String())
			}
		})
	}
}

// AC-I5: 内部 API はヘッダがあってもなくても同じ(端末ID/セッションID を検証しない)。GET 以外は 404 not_found。
func TestMasterExportHeadersAndMethods(t *testing.T) {
	h := newHandler(t, storetest.New())
	if rec := do(t, h, http.MethodGet, masterPath, true); rec.Code != http.StatusOK {
		t.Errorf("ヘッダ付きで status = %d, want 200", rec.Code)
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		assertError(t, do(t, h, m, masterPath, false), http.StatusNotFound, api.NotFound)
	}
}

// AC-I6: 運用エンドポイント。/healthz は DB に触れずに 200(DB が落ちていても liveness で再起動させない)。
func TestHealthzDoesNotTouchDB(t *testing.T) {
	q := storetest.New()
	q.Err = storetest.ErrDB
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, "/healthz", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d, want 200", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Errorf("/healthz の本文 = %s", rec.Body.String())
	}
	if len(q.Calls) != 0 {
		t.Errorf("/healthz が DB を呼んだ: %+v", q.Calls)
	}
}

// AC-I7: pokedex-svc は calc の操作を持たない(ルートが無い = 404 not_found。ヘッダ・本文の検証より先)。
// 未知のパスも 404 not_found(Error 形式)。
func TestCalcRoutesAndUnknownPathsAreNotFound(t *testing.T) {
	h := newHandler(t, storetest.New())
	for _, p := range []string{"/api/calc", "/api/calc/bulk", "/api/calc/reverse"} {
		assertError(t, do(t, h, http.MethodPost, p, false), http.StatusNotFound, api.NotFound)
		assertError(t, do(t, h, http.MethodPost, p, true), http.StatusNotFound, api.NotFound)
	}
	assertError(t, do(t, h, http.MethodGet, "/internal/unknown", false), http.StatusNotFound, api.NotFound)
	assertError(t, do(t, h, http.MethodGet, "/api/pokedex/abilities", true), http.StatusNotFound, api.NotFound)
}

// AC-I8: panic は 500 internal(Error 形式・内部情報を出さない)。
func TestPanicIsInternalError(t *testing.T) {
	h := newHandler(t, &panickingQuerier{Querier: storetest.New()})
	rec := do(t, h, http.MethodGet, masterPath, false)
	assertError(t, rec, http.StatusInternalServerError, api.Internal)
	if strings.Contains(rec.Body.String(), "goroutine") || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("内部情報を出している: %s", rec.Body.String())
	}
}

// panickingQuerier は ListTypes で panic する。
type panickingQuerier struct{ *storetest.Querier }

func (panickingQuerier) ListTypes(context.Context) ([]store.Type, error) { panic("boom") }
