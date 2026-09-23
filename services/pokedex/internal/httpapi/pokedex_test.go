package httpapi_test

// 公開の検索 API(/api/pokedex/*。契約は api/openapi.yaml の searchSpecies / getSpecies / searchMoves / getMove /
// searchItems / listNatures。ADR-0105 §3)のテスト。LIKE の評価・照合順序は DB の仕事なので、ここでは偽の Querier に渡る
// パターン・件数・レギュレーションと、応答の形・エラーを確かめる(実際の前方一致は db の -tags mysql のテスト)。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func decodeStrict(t *testing.T, body []byte, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("本文を読めない: %v\nbody=%s", err, body)
	}
}

// AC-P1: 各操作の 200 が契約どおり(ヘッダ付き)。
func TestPublicEndpointsMatchContract(t *testing.T) {
	h := newHandler(t, storetest.New())
	for _, target := range []string{
		"/api/pokedex/species",
		"/api/pokedex/species?q=" + url.QueryEscape("テスト") + "&format=single&limit=10",
		"/api/pokedex/species/9001-000",
		"/api/pokedex/species/9001-001",
		"/api/pokedex/moves",
		"/api/pokedex/moves?q=" + url.QueryEscape("テスト") + "&limit=2",
		"/api/pokedex/moves/teststrike",
		"/api/pokedex/items",
		"/api/pokedex/items?q=" + url.QueryEscape("テスト"),
		"/api/pokedex/natures",
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, target, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
			}
			validateAgainstContract(t, http.MethodGet, target, true, rec)
		})
	}
}

// AC-P2: 検索は既定のレギュレーションの使用可能集合で絞り、q は LIKE の特殊文字(\ % _)をエスケープした前方一致のパターンで
// DB に渡す。q の省略・空は全件(limit まで)。limit の既定は 50。
func TestSearchPassesPatternLimitAndRegulation(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		method      string
		wantPattern string
		wantLimit   int32
	}{
		{"種族: 既定", "/api/pokedex/species", "SearchSpecies", "%", 50},
		{"種族: q が空", "/api/pokedex/species?q=", "SearchSpecies", "%", 50},
		{"種族: 前方一致", "/api/pokedex/species?q=" + url.QueryEscape("テスト") + "&limit=10", "SearchSpecies", "テスト%", 10},
		{"種族: 特殊文字のエスケープ", "/api/pokedex/species?q=" + url.QueryEscape(`テ%_\`), "SearchSpecies", `テ\%\_\\%`, 50},
		{"種族: limit の上限", "/api/pokedex/species?limit=200", "SearchSpecies", "%", 200},
		{"技", "/api/pokedex/moves?q=" + url.QueryEscape("テストほ") + "&limit=1", "SearchMoves", "テストほ%", 1},
		{"技: 既定", "/api/pokedex/moves", "SearchMoves", "%", 50},
		{"持ち物", "/api/pokedex/items?q=" + url.QueryEscape("100%"), "SearchItems", `100\%%`, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, tt.target, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
			}
			calls := q.CallsOf(tt.method)
			if len(calls) != 1 {
				t.Fatalf("%s の呼び出し %d 回, want 1", tt.method, len(calls))
			}
			var reg, pattern string
			var limit int32
			switch a := calls[0].(type) {
			case store.SearchSpeciesParams:
				reg, pattern, limit = a.RegulationID, a.Pattern, a.Limit
			case store.SearchMovesParams:
				reg, pattern, limit = a.RegulationID, a.Pattern, a.Limit
			case store.SearchItemsParams:
				reg, pattern, limit = a.RegulationID, a.Pattern, a.Limit
			default:
				t.Fatalf("想定外の引数 %T", a)
			}
			if reg != storetest.DefaultRegulationID {
				t.Errorf("regulation = %q, want 既定の %q(GetDefaultRegulation の値。コードに書かない)", reg, storetest.DefaultRegulationID)
			}
			if pattern != tt.wantPattern {
				t.Errorf("pattern = %q, want %q", pattern, tt.wantPattern)
			}
			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
		})
	}
}

// AC-P3: 応答の中身。種族は types = [type1] か [type1, type2]、技・持ち物は DB の値、一致なしは [](null でない)。
func TestSearchResponses(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)

	var species []api.SpeciesSummary
	decodeStrict(t, do(t, h, http.MethodGet, "/api/pokedex/species", true).Body.Bytes(), &species)
	want := []api.SpeciesSummary{
		{Key: "9001-000", DexNo: 9001, Form: 0, NameJa: "テストモン", Types: []api.PokeType{api.PokeTypeFire}},
		{Key: "9001-001", DexNo: 9001, Form: 1, NameJa: "テストメガモン", Types: []api.PokeType{api.PokeTypeFire, api.PokeTypeWater}},
		{Key: "9002-000", DexNo: 9002, Form: 0, NameJa: "テストリーフ", Types: []api.PokeType{api.PokeTypeGrass}},
	}
	if !reflect.DeepEqual(species, want) {
		t.Errorf("species =\n%+v\nwant\n%+v", species, want)
	}

	var moves []api.Move
	decodeStrict(t, do(t, h, http.MethodGet, "/api/pokedex/moves", true).Body.Bytes(), &moves)
	byID := map[string]api.Move{}
	for _, m := range moves {
		byID[m.Id] = m
	}
	if m := byID["teststrike"]; m.NameJa != "テストうちこみ" || m.Type != api.PokeTypeNormal || m.Category != api.Physical || m.Power != 40 ||
		m.Priority == nil || *m.Priority != 1 {
		t.Errorf("teststrike = %+v", m)
	}
	if _, ok := byID["testbanned"]; ok {
		t.Errorf("使用可能集合の外の技 testbanned が検索に出た")
	}

	var items []api.Item
	decodeStrict(t, do(t, h, http.MethodGet, "/api/pokedex/items", true).Body.Bytes(), &items)
	if len(items) != 3 {
		t.Errorf("items = %+v, want 使用可能な3件", items)
	}

	q.RegulationSpecies = map[string][]string{}
	rec := do(t, h, http.MethodGet, "/api/pokedex/species?q="+url.QueryEscape("該当なし"), true)
	if rec.Code != http.StatusOK || bytes.TrimSpace(rec.Body.Bytes())[0] != '[' || string(bytes.TrimSpace(rec.Body.Bytes())) != "[]" {
		t.Errorf("一致なしの応答 = %d %s, want 200 []", rec.Code, rec.Body.String())
	}
}

// AC-P4: 種族の詳細。レギュレーションの外の種族も引ける(詳細はマスタの参照)。特性は slot 順の {id, nameJa}、
// learnset は習得技 ∩ 既定のレギュレーションの使用可能な技(ID 昇順)。無い種族は 404 not_found。
func TestGetSpecies(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)

	rec := do(t, h, http.MethodGet, "/api/pokedex/species/9001-000", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	var d api.SpeciesDetail
	decodeStrict(t, rec.Body.Bytes(), &d)
	if d.Key != "9001-000" || d.NameJa != "テストモン" || !reflect.DeepEqual(d.Types, []api.PokeType{api.PokeTypeFire}) {
		t.Errorf("detail = %+v", d)
	}
	if d.BaseStats != (api.StatBlock{Hp: 80, Atk: 90, Def: 70, Spa: 100, Spd: 75, Spe: 85}) {
		t.Errorf("baseStats = %+v", d.BaseStats)
	}
	wantAbilities := []api.Ability{{Id: "testblaze", NameJa: "テストもうか"}, {Id: "testguard", NameJa: "テストまもり"}}
	if !reflect.DeepEqual(d.Abilities, wantAbilities) {
		t.Errorf("abilities = %+v, want %+v", d.Abilities, wantAbilities)
	}
	if d.Learnset == nil || !reflect.DeepEqual(*d.Learnset, []string{"testflame", "teststrike"}) {
		t.Errorf("learnset = %v, want [testflame teststrike](使用可能集合の外の testbanned を除く)", d.Learnset)
	}
	if calls := q.CallsOf("ListSpeciesLearnset"); len(calls) != 1 ||
		calls[0].(store.ListSpeciesLearnsetParams).RegulationID != storetest.DefaultRegulationID {
		t.Errorf("ListSpeciesLearnset の呼び出し = %+v", calls)
	}

	if rec := do(t, h, http.MethodGet, "/api/pokedex/species/9003-000", true); rec.Code != http.StatusOK {
		t.Errorf("レギュレーションの外の種族の詳細 = %d, want 200", rec.Code)
	}
	assertError(t, do(t, h, http.MethodGet, "/api/pokedex/species/9099-000", true), http.StatusNotFound, api.NotFound)
}

// AC-P4b: 技の詳細。判定レーンが優先度(priority)を個別に引くための経路(2026-09-22 の依頼)。
// レギュレーションの外の技も引ける(絞り込みは検索の仕事)。無い技は 404 not_found。
func TestGetMove(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)

	rec := do(t, h, http.MethodGet, "/api/pokedex/moves/teststrike", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	var m api.Move
	decodeStrict(t, rec.Body.Bytes(), &m)
	want := api.Move{Id: "teststrike", NameJa: "テストうちこみ", Type: api.PokeTypeNormal, Category: api.Physical, Power: 40}
	priority := 1
	want.Priority = &priority
	if !reflect.DeepEqual(m, want) {
		t.Errorf("move = %+v, want %+v", m, want)
	}

	rec = do(t, h, http.MethodGet, "/api/pokedex/moves/testbanned", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("レギュレーションの外の技の詳細 = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	var banned api.Move
	decodeStrict(t, rec.Body.Bytes(), &banned)
	if banned.Id != "testbanned" || banned.NameJa != "テストきんじて" {
		t.Errorf("testbanned = %+v, want id/nameJa がすり替わっていないこと", banned)
	}

	assertError(t, do(t, h, http.MethodGet, "/api/pokedex/moves/unknownmove", true), http.StatusNotFound, api.NotFound)

	// 未投入(技0件)は searchMoves 等の一覧系と違い 503 ではなく 404(GetDefaultRegulation を経由しないため。
	// api/openapi.yaml の getMove の説明どおり)。
	empty := storetest.New()
	empty.Moves = nil
	hEmpty := newHandler(t, empty)
	assertError(t, do(t, hEmpty, http.MethodGet, "/api/pokedex/moves/teststrike", true), http.StatusNotFound, api.NotFound)
}

// AC-P5: 性格の一覧(natures テーブル)。無補正は plus / minus とも null。
func TestListNatures(t *testing.T) {
	h := newHandler(t, storetest.New())
	rec := do(t, h, http.MethodGet, "/api/pokedex/natures", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got []api.Nature
	decodeStrict(t, rec.Body.Bytes(), &got)
	atk, spe := api.StatKeyAtk, api.StatKeySpe
	want := []api.Nature{
		{Id: "testbrave", NameJa: "テストゆうかん", Plus: &atk, Minus: &spe},
		{Id: "testneutral", NameJa: "テストむほせい"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("natures = %+v, want %+v", got, want)
	}
}

// AC-P6: 入力の検証(DB を呼ぶ前に 400)。ヘッダの欠落は missing_header、limit の範囲外・型違い・種族キーの形式は
// invalid_input、format の未知の値は invalid_enum。
func TestPublicInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		withHeaders bool
		code        api.ErrorCode
	}{
		{"ヘッダ無し(種族)", "/api/pokedex/species", false, api.MissingHeader},
		{"ヘッダ無し(詳細)", "/api/pokedex/species/9001-000", false, api.MissingHeader},
		{"ヘッダ無し(技)", "/api/pokedex/moves", false, api.MissingHeader},
		{"ヘッダ無し(技詳細)", "/api/pokedex/moves/teststrike", false, api.MissingHeader},
		{"ヘッダ無し(持ち物)", "/api/pokedex/items", false, api.MissingHeader},
		{"ヘッダ無し(性格)", "/api/pokedex/natures", false, api.MissingHeader},
		{"limit=0", "/api/pokedex/species?limit=0", true, api.InvalidInput},
		{"limit=201", "/api/pokedex/moves?limit=201", true, api.InvalidInput},
		{"limit が整数でない", "/api/pokedex/items?limit=abc", true, api.InvalidInput},
		{"format が未知", "/api/pokedex/species?format=triple", true, api.InvalidEnum},
		{"種族キーの形式", "/api/pokedex/species/abc", true, api.InvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			h := newHandler(t, q)
			assertError(t, do(t, h, http.MethodGet, tt.target, tt.withHeaders), http.StatusBadRequest, tt.code)
			for _, c := range q.Calls {
				switch c.Method {
				case "SearchSpecies", "SearchMoves", "SearchItems", "GetSpeciesByKey", "GetMove":
					t.Errorf("入力が不正なのに %s を呼んだ", c.Method)
				}
			}
		})
	}
}

// AC-P7: 未投入(既定のレギュレーションが無い・性格が空)・DB の失敗は 503 master_unavailable(契約の Error)。
func TestPublicUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		target string
		mutate func(q *storetest.Querier)
	}{
		{"既定のレギュレーションが無い(種族)", "/api/pokedex/species", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"既定のレギュレーションが無い(技)", "/api/pokedex/moves", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"既定のレギュレーションが無い(持ち物)", "/api/pokedex/items", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"既定のレギュレーションが無い(詳細)", "/api/pokedex/species/9001-000", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"性格が空", "/api/pokedex/natures", func(q *storetest.Querier) { q.Natures = nil }},
		{"DB に接続できない(種族)", "/api/pokedex/species", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"DB に接続できない(詳細)", "/api/pokedex/species/9001-000", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"DB に接続できない(技詳細)", "/api/pokedex/moves/teststrike", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"DB に接続できない(性格)", "/api/pokedex/natures", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"検索のクエリだけ失敗", "/api/pokedex/moves", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"SearchMoves": storetest.ErrDB}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, tt.target, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			validateAgainstContract(t, http.MethodGet, tt.target, true, rec)
		})
	}
}
