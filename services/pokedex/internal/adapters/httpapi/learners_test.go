package httpapi_test

// 技の逆引き GET /api/pokedex/moves/{key}/learners(AJ5・ADR-0251)の受け入れテスト。
// DB の代わりに storetest の偽の Querier を使う(実 DB での並び・絞り込みは services/pokedex/db の
// learners_mysql_test.go が `make test-db` で確かめる)。

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

const learnersMethod = "ListMoveLearners"

func learnersPath(key string) string { return "/api/pokedex/moves/" + key + "/learners" }

// learnersFixture は storetest.New() に、testflame を覚える種族を足した偽の Querier を返す。
//   - 9002-000(使用可能)・9001-001(使用可能。fire/water)・9001-000(使用可能)・9003-000(使用可能集合の外)
//
// learnsets の行は図鑑番号の逆順に入れる(応答が図鑑番号・フォルム番号の昇順に並ぶことを確かめるため)。
func learnersFixture() *storetest.Querier {
	q := storetest.New()
	q.Learnsets = append([]store.Learnset{
		{SpeciesKey: "9003-000", MoveID: "testflame"},
		{SpeciesKey: "9002-000", MoveID: "testflame"},
		{SpeciesKey: "9001-001", MoveID: "testflame"},
	}, q.Learnsets...) // 既存の 9001-000/testflame はこの後ろ
	return q
}

func decodeLearners(t *testing.T, body []byte) []api.SpeciesSummary {
	t.Helper()
	var out []api.SpeciesSummary
	decodeStrict(t, body, &out)
	return out
}

func learnerKeys(xs []api.SpeciesSummary) []string {
	keys := []string{}
	for _, x := range xs {
		keys = append(keys, x.Key)
	}
	return keys
}

// AC-L1: 200 の応答は契約(SpeciesSummary の配列)どおり。リクエスト(limit・offset 付きを含む)も契約に合う。
func TestListMoveLearnersMatchesContract(t *testing.T) {
	h := newHandler(t, learnersFixture())
	for _, target := range []string{
		learnersPath("testflame"),
		learnersPath("testflame") + "?limit=2",
		learnersPath("testflame") + "?limit=1&offset=1",
		learnersPath("testflame") + "?offset=100", // 末尾を超える → []
		learnersPath("testbanned"),                // 使用可能集合の外の技 → []
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

// AC-L2: 使用可能集合の外の種族(9003-000)は出さない。並びは図鑑番号・フォルム番号の昇順。
// 要約の中身(タイプ2つの種族を含む)は searchSpecies と同じ写し方。
func TestListMoveLearnersFiltersAndOrders(t *testing.T) {
	q := learnersFixture()
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, learnersPath("testflame"), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	got := decodeLearners(t, rec.Body.Bytes())
	want := []api.SpeciesSummary{
		{Key: "9001-000", DexNo: 9001, Form: 0, NameJa: "テストモン", Types: []api.PokeType{api.PokeTypeFire}},
		{Key: "9001-001", DexNo: 9001, Form: 1, NameJa: "テストメガモン", Types: []api.PokeType{api.PokeTypeFire, api.PokeTypeWater}},
		{Key: "9002-000", DexNo: 9002, Form: 0, NameJa: "テストリーフ", Types: []api.PokeType{api.PokeTypeGrass}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("learners = %+v\nwant %+v", got, want)
	}

	// 既定のレギュレーション(DB から引いた ID)と技の ID を store に渡す。limit・offset の既定は 50・0。
	calls := q.CallsOf(learnersMethod)
	if len(calls) != 1 {
		t.Fatalf("%s の呼び出し = %d 回, want 1", learnersMethod, len(calls))
	}
	wantArg := store.ListMoveLearnersParams{RegulationID: storetest.DefaultRegulationID, MoveID: "testflame", Limit: 50, Offset: 0}
	if arg, _ := calls[0].(store.ListMoveLearnersParams); arg != wantArg {
		t.Errorf("%s の引数 = %+v, want %+v", learnersMethod, calls[0], wantArg)
	}
}

// AC-L3: 使用可能な種族 S について「S が技 M の逆引きに出る」⇔「getSpecies(S).learnset に M がある」
// (getSpecies の絞り込みと同じ規則)。架空データの全ての技で確かめる。
func TestListMoveLearnersAgreesWithGetSpeciesLearnset(t *testing.T) {
	q := learnersFixture()
	h := newHandler(t, q)

	inLearnset := map[string]map[string]bool{} // species → move → true
	for _, key := range q.RegulationSpecies[storetest.DefaultRegulationID] {
		rec := do(t, h, http.MethodGet, "/api/pokedex/species/"+key, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("getSpecies(%s) = %d", key, rec.Code)
		}
		var d api.SpeciesDetail
		decodeStrict(t, rec.Body.Bytes(), &d)
		inLearnset[key] = map[string]bool{}
		if d.Learnset != nil {
			for _, m := range *d.Learnset {
				inLearnset[key][m] = true
			}
		}
	}
	for _, m := range q.Moves {
		rec := do(t, h, http.MethodGet, learnersPath(m.ID), true)
		if rec.Code != http.StatusOK {
			t.Fatalf("learners(%s) = %d\nbody=%s", m.ID, rec.Code, rec.Body.String())
		}
		listed := map[string]bool{}
		for _, s := range decodeLearners(t, rec.Body.Bytes()) {
			listed[s.Key] = true
		}
		for species, moves := range inLearnset {
			if listed[species] != moves[m.ID] {
				t.Errorf("技 %s・種族 %s: 逆引きに出る=%v, getSpecies の learnset にある=%v(一致すること)",
					m.ID, species, listed[species], moves[m.ID])
			}
		}
		for species := range listed {
			if _, ok := inLearnset[species]; !ok {
				t.Errorf("技 %s の逆引きに使用可能集合の外の種族 %s が出た", m.ID, species)
			}
		}
	}
}

// AC-L4: 技がマスタにあっても使用可能集合の外(testbanned。9001-000 が覚える)なら 200 []。
// 覚える種族が無い技も 200 [](null にしない)。
func TestListMoveLearnersEmpty(t *testing.T) {
	for _, tt := range []struct {
		name   string
		key    string
		mutate func(q *storetest.Querier)
	}{
		{"使用可能集合の外の技", "testbanned", func(*storetest.Querier) {}},
		{"覚える種族が無い技", "teststrike", func(q *storetest.Querier) { q.Learnsets = nil }},
		{"覚えるのが集合の外の種族だけ", "testglare", func(q *storetest.Querier) {
			q.Learnsets = []store.Learnset{{SpeciesKey: "9003-000", MoveID: "testglare"}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := learnersFixture()
			tt.mutate(q)
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, learnersPath(tt.key), true)
			if rec.Code != http.StatusOK || string(bytes.TrimSpace(rec.Body.Bytes())) != "[]" {
				t.Errorf("応答 = %d %s, want 200 []", rec.Code, rec.Body.String())
			}
		})
	}
}

// AC-L5: limit・offset のページング。ページを順に連結すると全件と同じ並び。末尾を超える offset は []。
// 指定した limit・offset は store にそのまま渡る。
func TestListMoveLearnersPaging(t *testing.T) {
	q := learnersFixture()
	h := newHandler(t, q)
	all := []string{"9001-000", "9001-001", "9002-000"}

	var pages []string
	for offset := 0; offset <= len(all); offset += 2 {
		target := fmt.Sprintf("%s?limit=2&offset=%d", learnersPath("testflame"), offset)
		rec := do(t, h, http.MethodGet, target, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d\nbody=%s", target, rec.Code, rec.Body.String())
		}
		page := learnerKeys(decodeLearners(t, rec.Body.Bytes()))
		if len(page) > 2 {
			t.Errorf("%s: %d 件, want limit(2)以下", target, len(page))
		}
		pages = append(pages, page...)
	}
	if !reflect.DeepEqual(pages, all) {
		t.Errorf("ページの連結 = %v, want %v", pages, all)
	}

	rec := do(t, h, http.MethodGet, learnersPath("testflame")+"?limit=1&offset=1", true)
	if got := learnerKeys(decodeLearners(t, rec.Body.Bytes())); !reflect.DeepEqual(got, []string{"9001-001"}) {
		t.Errorf("limit=1&offset=1 = %v, want [9001-001]", got)
	}
	rec = do(t, h, http.MethodGet, learnersPath("testflame")+"?offset=3", true)
	if rec.Code != http.StatusOK || string(bytes.TrimSpace(rec.Body.Bytes())) != "[]" {
		t.Errorf("末尾ちょうどの offset = %d %s, want 200 []", rec.Code, rec.Body.String())
	}
	// 契約の上限ちょうど(limit=200・offset=10000)は受け付ける。
	rec = do(t, h, http.MethodGet, learnersPath("testflame")+"?limit=200&offset=10000", true)
	if rec.Code != http.StatusOK {
		t.Errorf("上限ちょうど = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}

	var gotArgs []store.ListMoveLearnersParams
	for _, c := range q.CallsOf(learnersMethod) {
		gotArgs = append(gotArgs, c.(store.ListMoveLearnersParams))
	}
	wantLast := store.ListMoveLearnersParams{RegulationID: storetest.DefaultRegulationID, MoveID: "testflame", Limit: 200, Offset: 10000}
	if len(gotArgs) == 0 || gotArgs[len(gotArgs)-1] != wantLast {
		t.Errorf("最後の %s の引数 = %+v, want %+v", learnersMethod, gotArgs, wantLast)
	}
}

// AC-L6: 技がマスタに無い key は 404 not_found(契約の Error)。逆引きのクエリを呼ばない。
// "batch" も技の ID として扱われ、/moves/batch(getMovesByIds)には食われない。
func TestListMoveLearnersUnknownMove(t *testing.T) {
	for _, key := range []string{"unknownmove", "batch"} {
		t.Run(key, func(t *testing.T) {
			q := learnersFixture()
			h := newHandler(t, q)
			target := learnersPath(key)
			rec := do(t, h, http.MethodGet, target, true)
			assertError(t, rec, http.StatusNotFound, api.NotFound)
			validateAgainstContract(t, http.MethodGet, target, true, rec)
			if n := len(q.CallsOf("GetMove")); n != 1 {
				t.Errorf("GetMove の呼び出し = %d 回, want 1(技の存在を確かめる)", n)
			}
			if n := len(q.CallsOf(learnersMethod)); n != 0 {
				t.Errorf("技が無いのに %s を %d 回呼んだ", learnersMethod, n)
			}
		})
	}
	// 既存の技の詳細(/moves/{key})は新しいルートに影響されない。
	h := newHandler(t, learnersFixture())
	if rec := do(t, h, http.MethodGet, "/api/pokedex/moves/testflame", true); rec.Code != http.StatusOK {
		t.Errorf("getMove = %d, want 200", rec.Code)
	}
}

// AC-L7: 入力の検証は DB を呼ぶ前(400)。ヘッダ欠落は missing_header、limit・offset の範囲外・整数でない値は invalid_input。
func TestListMoveLearnersInputValidation(t *testing.T) {
	base := learnersPath("testflame")
	tests := []struct {
		name        string
		target      string
		withHeaders bool
		code        api.ErrorCode
	}{
		{"ヘッダ無し", base, false, api.MissingHeader},
		{"limit=0", base + "?limit=0", true, api.InvalidInput},
		{"limit=201", base + "?limit=201", true, api.InvalidInput},
		{"limit が整数でない", base + "?limit=abc", true, api.InvalidInput},
		{"offset=-1", base + "?offset=-1", true, api.InvalidInput},
		{"offset=10001", base + "?offset=10001", true, api.InvalidInput},
		{"offset が整数でない", base + "?offset=x", true, api.InvalidInput},
		{"offset が int32 を超える", base + "?offset=4294967296", true, api.InvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := learnersFixture()
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, tt.target, tt.withHeaders)
			assertError(t, rec, http.StatusBadRequest, tt.code)
			validateResponseAgainstContract(t, http.MethodGet, tt.target, tt.withHeaders, rec)
			if len(q.Calls) != 0 {
				t.Errorf("入力が不正なのに DB を呼んだ: %+v", q.Calls)
			}
		})
	}
}

// AC-L8: 既定のレギュレーションが無い(マスタ未投入)・DB の失敗は 503 master_unavailable(getMove の 404 と違い一覧系の流儀)。
// 既定のレギュレーションが無いときは、技の key が未知でも 503(getSpecies と同じく既定のレギュレーションを先に引く)。
func TestListMoveLearnersUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		mutate func(q *storetest.Querier)
	}{
		{"既定のレギュレーションが無い", "testflame", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"マスタ未投入(技も規則も無い)", "testflame", func(q *storetest.Querier) {
			q.DefaultRegulation, q.Moves, q.Species, q.Learnsets = nil, nil, nil, nil
		}},
		{"既定のレギュレーションが無く key も未知", "unknownmove", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"DB に接続できない", "testflame", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"技の取得だけ失敗(ErrNoRows 以外)", "testflame", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"GetMove": storetest.ErrDB}
		}},
		{"逆引きのクエリだけ失敗", "testflame", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{learnersMethod: storetest.ErrDB}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := learnersFixture()
			tt.mutate(q)
			h := newHandler(t, q)
			target := learnersPath(tt.key)
			rec := do(t, h, http.MethodGet, target, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			validateAgainstContract(t, http.MethodGet, target, true, rec)
			if bytes.Contains(rec.Body.Bytes(), []byte(storetest.ErrDB.Error())) {
				t.Errorf("DB のエラー文を応答に出している: %s", rec.Body.String())
			}
		})
	}
}
