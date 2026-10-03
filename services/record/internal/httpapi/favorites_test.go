package httpapi

// お気に入り(手動ピン留め)の受け入れテスト(ADR-0227。P5-3c)。AC-F1〜AC-F10。
// 端末分離(AC-D1・AC-D2・AC-D3)の実操作での検証も、record ではここで初めて持つ(ADR-0209 §6・
// isolation_test.go の TestRecordOperationsHaveNoPathParameters の申し送り)。
//
// **test-first(ADR-0003)**: 実装前に書いた。実装者が追加するもの(このテストが前提にする形):
//
//	// registerRecordRoutes に3つを足す(生成ラッパ経由)。
//	//   GET    /api/record/favorites                → ListFavorites
//	//   POST   /api/record/favorites                → CreateFavorite
//	//   DELETE /api/record/favorites/:favoriteId    → DeleteFavorite
//	// 3つとも touchAndRun を通す(お気に入りの操作も「端末の利用」。ADR-0227 §5)。
//
// 正規化(ADR-0227 §2): 保存・応答・重複判定に使う individual は「既定値を補った形」。
// level=50・ranks は6キーすべて(省略は0)・status は none・abilityId/itemId/teraType は null と
// 省略を同じ「未指定」とし、応答ではキーごと省く。label の空文字は null。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

const pathFavorites = "/api/record/favorites"

func favoritePath(id string) string { return pathFavorites + "/" + id }

// favoriteIDPattern は契約の FavoriteId の pattern。
var favoriteIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

// spBlock は SP の6値(順に hp atk def spa spd spe)。
func spBlock(hp, atk, def, spa, spd, spe int) map[string]any {
	return map[string]any{"hp": hp, "atk": atk, "def": def, "spa": spa, "spd": spd, "spe": spe}
}

// minimalIndividual は必須項目だけの個体(既定値の補完を確かめるため、任意項目を送らない)。
func minimalIndividual(speciesKey string) map[string]any {
	return map[string]any{"speciesKey": speciesKey, "natureId": "fake-nature", "sp": spBlock(32, 0, 2, 0, 0, 32)}
}

// fullIndividual は任意項目まで埋めた個体(往復で落ちないことを確かめる)。
func fullIndividual(speciesKey string) map[string]any {
	return map[string]any{
		"speciesKey": speciesKey,
		"level":      50,
		"natureId":   "fake-nature",
		"abilityId":  "fake-ability",
		"itemId":     "fake-item",
		"sp":         spBlock(4, 32, 0, 0, 0, 30),
		"ranks":      map[string]any{"atk": 1, "def": 0, "spa": 0, "spd": 0, "spe": -1},
		"teraType":   "steel",
		"status":     "burn",
	}
}

func favoriteBody(t *testing.T, label any, individual map[string]any) []byte {
	t.Helper()
	m := map[string]any{"individual": individual}
	if label != nil {
		m["label"] = label
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeFavorite(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) api.Favorite {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var got api.Favorite
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Favorite として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}

func decodeFavorites(t *testing.T, rec *httptest.ResponseRecorder) []api.Favorite {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) == "null" {
		t.Fatal("一覧が null(空なら空配列 [] を返す)")
	}
	var got []api.Favorite
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Favorite の配列として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}

// createFavorite は作成して 201 の結果を返す。
func createFavorite(t *testing.T, h http.Handler, deviceID string, label any, individual map[string]any) api.Favorite {
	t.Helper()
	return decodeFavorite(t, serve(t, h, http.MethodPost, pathFavorites, headers(deviceID), favoriteBody(t, label, individual)), http.StatusCreated)
}

// --- AC-F1: 作成と往復 ------------------------------------------------------

// AC-F1: 作成は 201。id(FavoriteId の形式)・createdAt・updatedAt はサーバーが決め、作成直後は
// createdAt == updatedAt。任意項目は往復で落ちず、省いた項目は既定値を補った形で返る。
func TestCreateFavoriteReturnsServerAssignedFields(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	got := createFavorite(t, h, deviceA, "HB特化", fullIndividual(speciesGuard))
	if !favoriteIDPattern.MatchString(got.Id) {
		t.Errorf("id = %q は FavoriteId の形式(正の整数の10進)でない", got.Id)
	}
	if got.CreatedAt.IsZero() || !got.CreatedAt.Equal(got.UpdatedAt) {
		t.Errorf("createdAt = %v, updatedAt = %v(作成直後は等しく、ゼロでない)", got.CreatedAt, got.UpdatedAt)
	}
	if got.Label == nil || *got.Label != "HB特化" {
		t.Errorf("label = %v, want HB特化", got.Label)
	}
	ind := got.Individual
	if string(ind.SpeciesKey) != speciesGuard || ind.NatureId != "fake-nature" {
		t.Errorf("individual の種族・性格が往復しない: %+v", ind)
	}
	if ind.AbilityId == nil || *ind.AbilityId != "fake-ability" || ind.ItemId == nil || *ind.ItemId != "fake-item" {
		t.Errorf("abilityId / itemId が往復しない: %v / %v", ind.AbilityId, ind.ItemId)
	}
	if ind.TeraType == nil || *ind.TeraType != api.PokeTypeSteel {
		t.Errorf("teraType が往復しない: %v", ind.TeraType)
	}
	if ind.Status == nil || *ind.Status != api.StatusConditionBurn {
		t.Errorf("status が往復しない: %v", ind.Status)
	}
	if ind.Sp != (api.StatBlock{Hp: 4, Atk: 32, Def: 0, Spa: 0, Spd: 0, Spe: 30}) {
		t.Errorf("sp が往復しない: %+v", ind.Sp)
	}
	if ind.Ranks == nil || ind.Ranks.Atk == nil || *ind.Ranks.Atk != 1 || ind.Ranks.Spe == nil || *ind.Ranks.Spe != -1 {
		t.Errorf("ranks が往復しない: %+v", ind.Ranks)
	}

	// 一覧にも同じものが出る。
	list := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil))
	if len(list) != 1 || list[0].Id != got.Id {
		t.Fatalf("一覧 = %+v, want 作った1件(id %s)", list, got.Id)
	}
}

// AC-F1(正規化): 必須項目だけの個体は、既定値を補った形で保存・応答する(ADR-0227 §2)。
// label を省いた・空文字は null。
func TestCreateFavoriteNormalizesDefaults(t *testing.T) {
	for _, tt := range []struct {
		name  string
		label any
	}{
		{"label 省略", nil},
		{"label 空文字", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, tt.label, minimalIndividual(speciesGuard)))
			got := decodeFavorite(t, rec, http.StatusCreated)
			if got.Label != nil {
				t.Errorf("label = %q, want null", *got.Label)
			}
			if !strings.Contains(rec.Body.String(), `"label":null`) {
				t.Errorf("label は必須(nullable)なので null として出す: %s", rec.Body.String())
			}
			ind := got.Individual
			if ind.Level == nil || *ind.Level != 50 {
				t.Errorf("level = %v, want 50(既定値を補う)", ind.Level)
			}
			if ind.Status == nil || *ind.Status != api.StatusConditionNone {
				t.Errorf("status = %v, want none", ind.Status)
			}
			if ind.Ranks == nil {
				t.Fatal("ranks が無い(6キーすべて 0 で補う)")
			}
			for name, v := range map[string]*int{"atk": ind.Ranks.Atk, "def": ind.Ranks.Def, "spa": ind.Ranks.Spa, "spd": ind.Ranks.Spd, "spe": ind.Ranks.Spe} {
				if v == nil || *v != 0 {
					t.Errorf("ranks.%s = %v, want 0", name, v)
				}
			}
			if ind.AbilityId != nil || ind.ItemId != nil || ind.TeraType != nil {
				t.Errorf("未指定の abilityId / itemId / teraType が値を持つ: %v %v %v", ind.AbilityId, ind.ItemId, ind.TeraType)
			}
		})
	}
}

// AC-F1: 要求に id / createdAt / updatedAt を入れたら 400 unknown_field(サーバーが決める値)。
// 契約に無いキーは individual の中も含めて unknown_field、壊れた JSON・空の本文は invalid_json。
func TestCreateFavoriteRejectsMalformedBodies(t *testing.T) {
	validInd := `{"speciesKey":"9002-000","natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}}`
	tests := []struct {
		name     string
		body     []byte
		wantCode api.ErrorCode
	}{
		{"id を送る", []byte(`{"id":"1","individual":` + validInd + `}`), api.UnknownField},
		{"createdAt を送る", []byte(`{"createdAt":"2026-01-01T00:00:00Z","individual":` + validInd + `}`), api.UnknownField},
		{"updatedAt を送る", []byte(`{"updatedAt":"2026-01-01T00:00:00Z","individual":` + validInd + `}`), api.UnknownField},
		{"契約にないキー", []byte(`{"memo":"x","individual":` + validInd + `}`), api.UnknownField},
		{"individual に契約にないキー", []byte(`{"individual":{"speciesKey":"9002-000","natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"moveIds":["m"]}}`), api.UnknownField},
		{"JSON として壊れている", []byte(`{`), api.InvalidJson},
		{"本文が空", nil, api.InvalidJson},
		{"teraType が未知", []byte(`{"individual":{"speciesKey":"9002-000","natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"teraType":"stellar-x"}}`), api.InvalidEnum},
		{"status が未知", []byte(`{"individual":{"speciesKey":"9002-000","natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"confused"}}`), api.InvalidEnum},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA), tt.body)
			assertErrorBody(t, rec, http.StatusBadRequest, tt.wantCode)
			if n := len(st.favoritesOf(deviceA)); n != 0 {
				t.Errorf("拒否した要求で行が %d 件できた", n)
			}
		})
	}
}

// --- AC-F2: マスタを引かずに判断できる入力検証 -----------------------------------

// AC-F2: 範囲外は 400 invalid_input。**境界ちょうどは通る**(「以上で弾く」実装にしない)。
// ラベルの長さは Unicode コードポイントで数える(日本語30文字は通り、31文字は弾く)。
func TestCreateFavoriteValidatesInput(t *testing.T) {
	ind := func(mut func(m map[string]any)) map[string]any {
		m := minimalIndividual(speciesGuard)
		mut(m)
		return m
	}
	jp30 := strings.Repeat("あ", 30)
	jp31 := strings.Repeat("あ", 31)

	tests := []struct {
		name       string
		label      any
		individual map[string]any
		wantOK     bool
	}{
		{"SP 各32・合計66 ちょうど", nil, ind(func(m map[string]any) { m["sp"] = spBlock(32, 32, 2, 0, 0, 0) }), true},
		{"SP 33", nil, ind(func(m map[string]any) { m["sp"] = spBlock(33, 0, 0, 0, 0, 0) }), false},
		{"SP 負", nil, ind(func(m map[string]any) { m["sp"] = spBlock(-1, 0, 0, 0, 0, 0) }), false},
		{"SP 合計67", nil, ind(func(m map[string]any) { m["sp"] = spBlock(32, 32, 3, 0, 0, 0) }), false},
		{"ランク +6 / -6 ちょうど", nil, ind(func(m map[string]any) { m["ranks"] = map[string]any{"atk": 6, "spe": -6} }), true},
		{"ランク +7", nil, ind(func(m map[string]any) { m["ranks"] = map[string]any{"atk": 7} }), false},
		{"ランク -7", nil, ind(func(m map[string]any) { m["ranks"] = map[string]any{"def": -7} }), false},
		{"レベル 50", nil, ind(func(m map[string]any) { m["level"] = 50 }), true},
		{"レベル 51", nil, ind(func(m map[string]any) { m["level"] = 51 }), false},
		{"speciesKey の形式違い", nil, ind(func(m map[string]any) { m["speciesKey"] = "garchomp" }), false},
		{"speciesKey 空", nil, ind(func(m map[string]any) { m["speciesKey"] = "" }), false},
		{"natureId 空", nil, ind(func(m map[string]any) { m["natureId"] = "" }), false},
		{"sp の欠落", nil, ind(func(m map[string]any) { delete(m, "sp") }), false},
		{"sp のキー欠落", nil, ind(func(m map[string]any) { m["sp"] = map[string]any{"hp": 0} }), false},
		{"ラベル30文字(日本語)ちょうど", jp30, minimalIndividual(speciesGuard), true},
		{"ラベル31文字(日本語)", jp31, minimalIndividual(speciesGuard), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, tt.label, tt.individual))
			if tt.wantOK {
				if rec.Code != http.StatusCreated {
					t.Errorf("status = %d, want 201(境界ちょうどは通る); body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
			if n := len(st.favoritesOf(deviceA)); n != 0 {
				t.Errorf("拒否した要求で行が %d 件できた", n)
			}
		})
	}

	// individual そのものの欠落も invalid_input(必須項目の欠落。calc-svc の sp 欠落と同じ扱い)。
	t.Run("individual の欠落", func(t *testing.T) {
		rec := serve(t, NewHandler(newFakeStore()), http.MethodPost, pathFavorites, headers(deviceA), []byte(`{"label":"x"}`))
		assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
	})
}

// AC-F3: マスタに存在しない ID(形式は正しい)でも 201 で保存できる。record-svc は pokedex-svc に
// 依存しない(ADR-0227 §2・ADR-0213 §3 と同じ理由)ので unknown_species 等を返さない。
func TestCreateFavoriteDoesNotConsultMaster(t *testing.T) {
	st := newFakeStore()
	m := minimalIndividual("9999-999")
	m["natureId"] = "no-such-nature"
	m["abilityId"] = "no-such-ability"
	m["itemId"] = "no-such-item"
	got := createFavorite(t, NewHandler(st), deviceA, nil, m)
	if string(got.Individual.SpeciesKey) != "9999-999" {
		t.Errorf("speciesKey = %q, want 9999-999", got.Individual.SpeciesKey)
	}
	// store には個体の種族が SpeciesKey として渡る(favorites.species_key 列)。
	rows := st.favoritesOf(deviceA)
	if len(rows) != 1 || rows[0].SpeciesKey != "9999-999" {
		t.Errorf("store の SpeciesKey = %+v, want 9999-999", rows)
	}
}

// --- AC-F4: 冪等(同じ内容の二重作成) -----------------------------------------

// AC-F4: 同じ内容(label + 正規化した individual)の2回目は 200 で、同じ id を返し、行は増えない。
// updatedAt は進み、一覧の先頭に来る。label が違えば別のお気に入り(201)。
func TestCreateFavoriteIsIdempotentForSameContent(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	first := createFavorite(t, h, deviceA, "物理受け", minimalIndividual(speciesGuard))
	time.Sleep(2 * time.Millisecond)
	other := createFavorite(t, h, deviceA, nil, minimalIndividual(speciesLeaf))
	time.Sleep(2 * time.Millisecond)

	// 既定値を明示しても(level 50・status none・ranks 0・abilityId null)同じ内容として扱う。
	explicit := minimalIndividual(speciesGuard)
	explicit["level"] = 50
	explicit["status"] = "none"
	explicit["ranks"] = map[string]any{"atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}
	explicit["abilityId"] = nil
	again := decodeFavorite(t, serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, "物理受け", explicit)), http.StatusOK)

	if again.Id != first.Id {
		t.Errorf("2回目の id = %s, want %s(既存を返す)", again.Id, first.Id)
	}
	if !again.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("createdAt が変わった: %v → %v", first.CreatedAt, again.CreatedAt)
	}
	if !again.UpdatedAt.After(first.UpdatedAt) {
		t.Errorf("updatedAt = %v は1回目 %v より後(再ピン留めで進む)", again.UpdatedAt, first.UpdatedAt)
	}
	if n := len(st.favoritesOf(deviceA)); n != 2 {
		t.Errorf("行数 = %d, want 2(同じ内容は増えない)", n)
	}
	list := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil))
	if len(list) != 2 || list[0].Id != first.Id || list[1].Id != other.Id {
		t.Errorf("一覧の順 = %v, want [%s %s](再ピン留めしたものが先頭)", favoriteIDs(list), first.Id, other.Id)
	}

	// label だけ違えば別のお気に入り。
	createFavorite(t, h, deviceA, "特殊受け", minimalIndividual(speciesGuard))
	if n := len(st.favoritesOf(deviceA)); n != 3 {
		t.Errorf("label 違いの行数 = %d, want 3", n)
	}

	// 別の端末の同じ内容は別物(端末をまたいで重複判定しない)。
	createFavorite(t, h, deviceB, "物理受け", minimalIndividual(speciesGuard))
}

// AC-F4: store に渡す Snapshot は正規化済みで、同じ内容なら同じバイト列(store は Snapshot の
// バイト列で重複を判定する。ADR-0227 §2)。中身に端末 ID・セッション ID を含めない。
func TestFavoriteSnapshotIsCanonical(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	createFavorite(t, h, deviceA, nil, minimalIndividual(speciesGuard))

	// キーの順序・既定値の明示の有無が違う同じ内容を、別端末で作る。
	explicit := map[string]any{
		"status": "none", "ranks": map[string]any{}, "sp": spBlock(32, 0, 2, 0, 0, 32),
		"natureId": "fake-nature", "level": 50, "speciesKey": speciesGuard, "itemId": nil,
	}
	createFavorite(t, h, deviceB, "", explicit)

	a, b := st.favoritesOf(deviceA), st.favoritesOf(deviceB)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("行数 A=%d B=%d, want 1/1", len(a), len(b))
	}
	if string(a[0].Snapshot) != string(b[0].Snapshot) {
		t.Errorf("同じ内容の Snapshot が一致しない:\nA=%s\nB=%s", a[0].Snapshot, b[0].Snapshot)
	}
	for _, forbidden := range []string{deviceA, deviceB, sessionID} {
		if strings.Contains(string(a[0].Snapshot), forbidden) {
			t.Errorf("Snapshot に端末 ID / セッション ID %q が入っている: %s", forbidden, a[0].Snapshot)
		}
	}
	var snap map[string]json.RawMessage
	if err := json.Unmarshal(a[0].Snapshot, &snap); err != nil {
		t.Fatalf("Snapshot が JSON でない: %v", err)
	}
	if _, ok := snap["individual"]; !ok || len(snap) != 2 {
		t.Errorf("Snapshot のキー = %v, want label と individual の2つ", keysOf(snap))
	}
}

// --- AC-F5: 件数の上限 --------------------------------------------------------

// AC-F5: 1端末 100 件まで。100件目は通り、101件目は 400 invalid_input。上限でも同じ内容の
// 再ピン留めは 200(行が増えないので上限に当たらない)。別の端末は影響を受けない。
func TestCreateFavoriteLimitPerDevice(t *testing.T) {
	if store.MaxFavoritesPerDevice != 100 {
		t.Fatalf("MaxFavoritesPerDevice = %d, want 100(契約の description・maxItems と同じ値。ADR-0227 §3)", store.MaxFavoritesPerDevice)
	}
	st := newFakeStore()
	st.seed(deviceA, 0, 0, store.MaxFavoritesPerDevice-1)
	h := NewHandler(st)

	hundredth := createFavorite(t, h, deviceA, "100件目", minimalIndividual(speciesGuard))

	rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, "101件目", minimalIndividual(speciesGuard)))
	assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
	if n := len(st.favoritesOf(deviceA)); n != store.MaxFavoritesPerDevice {
		t.Errorf("行数 = %d, want %d(上限を超えない)", n, store.MaxFavoritesPerDevice)
	}

	again := decodeFavorite(t, serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, "100件目", minimalIndividual(speciesGuard))), http.StatusOK)
	if again.Id != hundredth.Id {
		t.Errorf("上限での再ピン留め id = %s, want %s", again.Id, hundredth.Id)
	}

	createFavorite(t, h, deviceB, "101件目", minimalIndividual(speciesGuard)) // B は 201

	// 1件消せばまた作れる。
	if rec := serve(t, h, http.MethodDelete, favoritePath(hundredth.Id), headers(deviceA), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("削除 status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	createFavorite(t, h, deviceA, "101件目", minimalIndividual(speciesGuard))
}

// --- AC-F6: 一覧 --------------------------------------------------------------

// AC-F6: 一覧は updatedAt の降順(同時刻は id の降順)。1件も無い端末は空配列(null にしない・404 にしない)。
func TestListFavoritesOrderAndEmpty(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	if got := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)); len(got) != 0 {
		t.Fatalf("空の端末の一覧 = %d 件, want 0", len(got))
	}

	// 同時刻の3件(seed は同じ f.now で作る)+ 新しい1件。
	st.seed(deviceA, 0, 0, 3)
	newest := createFavorite(t, h, deviceA, nil, minimalIndividual(speciesLeaf))

	got := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil))
	if len(got) != 4 || got[0].Id != newest.Id {
		t.Fatalf("一覧 = %v, want 先頭が %s の4件", favoriteIDs(got), newest.Id)
	}
	// 同時刻(seed の3件)は id の降順。
	ids := favoriteIDs(got[1:])
	want := []string{"3", "2", "1"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("同時刻の並び = %v, want %v(id の降順)", ids, want)
	}
}

// 保存済みの行の Snapshot が読めない(個体として解釈できない)ときは、その行を黙って飛ばさず
// 500 internal にする(ADR-0227 §2。サーバー側の不備で、黙って捨てると利用者のピンが消えたように見える)。
func TestListFavoritesWithCorruptSnapshotIsInternal(t *testing.T) {
	st := newFakeStore()
	st.mu.Lock()
	st.insertFavoriteLocked(deviceA, speciesGuard, []byte(`{"seed":1}`), st.now)
	st.mu.Unlock()
	rec := serve(t, NewHandler(st), http.MethodGet, pathFavorites, headers(deviceA), nil)
	assertErrorBody(t, rec, http.StatusInternalServerError, api.Internal)
}

// --- AC-F7: 削除と端末分離(AC-D1・AC-D2) ---------------------------------------

// AC-F7: 削除は 204(本文なし)、2回目は 404 not_found。他のお気に入りは残る。
func TestDeleteFavorite(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	a := createFavorite(t, h, deviceA, nil, minimalIndividual(speciesGuard))
	b := createFavorite(t, h, deviceA, nil, minimalIndividual(speciesLeaf))

	rec := serve(t, h, http.MethodDelete, favoritePath(a.Id), headers(deviceA), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "" {
		t.Errorf("204 に本文がある: %q", body)
	}
	assertErrorBody(t, serve(t, h, http.MethodDelete, favoritePath(a.Id), headers(deviceA), nil), http.StatusNotFound, api.NotFound)

	list := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil))
	if len(list) != 1 || list[0].Id != b.Id {
		t.Errorf("残り = %v, want [%s]", favoriteIDs(list), b.Id)
	}
}

// AC-D1: 端末 A のお気に入りは端末 B の一覧に出ない。store は B の端末 ID でしか呼ばれない。
func TestFavoritesAreScopedToTheDevice(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	createFavorite(t, h, deviceA, "A のピン", minimalIndividual(speciesGuard))
	st.calls = nil

	got := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceB), nil))
	if len(got) != 0 {
		t.Errorf("端末 B の一覧 = %d 件, want 0(A のピンが混ざらない)", len(got))
	}
	for _, id := range st.deviceIDsTouched() {
		if id != deviceB {
			t.Errorf("store が端末 %q で呼ばれた, want %q だけ(ADR-0209 §6-1)", id, deviceB)
		}
	}
}

// AC-D2: 端末 B が端末 A の favoriteId を指して DELETE すると 404 not_found(403 にしない)。
// 応答本文に A の情報(id・label・種族)を含まず、A の行は消えない。形式違い・実在しない ID も同じ 404。
func TestOtherDevicesFavoriteIsNotFound(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	a := createFavorite(t, h, deviceA, "秘密のラベル", minimalIndividual(speciesGuard))

	for _, tt := range []struct{ name, id string }{
		{"他端末の ID", a.Id},
		{"実在しない ID", "999999"},
		{"0", "0"},
		{"先頭が0", "0" + a.Id},
		{"数字でない", "abc"},
		{"int64 を超える", "99999999999999999999"},
		{"負", "-1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, h, http.MethodDelete, favoritePath(tt.id), headers(deviceB), nil)
			assertErrorBody(t, rec, http.StatusNotFound, api.NotFound)
			for _, leak := range []string{"秘密のラベル", speciesGuard, deviceA} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("404 の本文に A の情報 %q が出ている: %s", leak, rec.Body.String())
				}
			}
		})
	}
	if n := len(st.favoritesOf(deviceA)); n != 1 {
		t.Errorf("端末 A の行 = %d 件, want 1(B の削除で消えない)", n)
	}
}

// AC-D3: 端末 ID をクエリ・ボディで受け取らない。400 unknown_field で、ヘッダの端末を上書きできない。
func TestFavoritesRejectDeviceIDOutsideHeader(t *testing.T) {
	ind := `{"speciesKey":"9002-000","natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}}`
	tests := []struct {
		name, method, target string
		body                 []byte
	}{
		{"一覧のクエリに deviceId", http.MethodGet, pathFavorites + "?deviceId=" + deviceB, nil},
		{"一覧のクエリに device_id", http.MethodGet, pathFavorites + "?device_id=" + deviceB, nil},
		{"一覧のボディに deviceId", http.MethodGet, pathFavorites, []byte(`{"deviceId":"` + deviceB + `"}`)},
		{"作成のクエリに deviceId", http.MethodPost, pathFavorites + "?deviceId=" + deviceB, []byte(`{"individual":` + ind + `}`)},
		{"作成のボディに deviceId", http.MethodPost, pathFavorites, []byte(`{"deviceId":"` + deviceB + `","individual":` + ind + `}`)},
		{"削除のクエリに deviceId", http.MethodDelete, favoritePath("1") + "?deviceId=" + deviceB, nil},
		{"削除のボディに deviceId", http.MethodDelete, favoritePath("1"), []byte(`{"deviceId":"` + deviceB + `"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.seed(deviceB, 0, 0, 1) // B の id "1"
			rec := serve(t, NewHandler(st), tt.method, tt.target, headers(deviceA), tt.body)
			assertErrorBody(t, rec, http.StatusBadRequest, api.UnknownField)
			for _, id := range st.deviceIDsTouched() {
				if id == deviceB {
					t.Errorf("ヘッダ外の deviceId で端末 %q が参照された(ADR-0209 §6-4)", id)
				}
			}
			if n := len(st.favoritesOf(deviceB)); n != 1 {
				t.Errorf("端末 B の行 = %d 件, want 1", n)
			}
			if n := len(st.favoritesOf(deviceA)); n != 0 {
				t.Errorf("拒否した要求で端末 A に行が %d 件できた", n)
			}
		})
	}
}

// ヘッダの欠落は生成ラッパで 400 missing_header(UUID 形式の検証は gateway。ADR-0202)。
func TestFavoritesRequireHeaders(t *testing.T) {
	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, pathFavorites},
		{http.MethodPost, pathFavorites},
		{http.MethodDelete, favoritePath("1")},
	} {
		t.Run(tt.method, func(t *testing.T) {
			st := newFakeStore()
			hdr := http.Header{}
			hdr.Set("X-Session-Id", sessionID)
			rec := serve(t, NewHandler(st), tt.method, tt.path, hdr, nil)
			assertErrorBody(t, rec, http.StatusBadRequest, api.MissingHeader)
			if len(st.deviceIDsTouched()) != 0 {
				t.Error("ヘッダが無いのに store が呼ばれた")
			}
		})
	}
}

// --- AC-F8: last_seen_at(ADR-0209 §4) ------------------------------------------

// AC-F8: お気に入りの3操作はどれも「端末 ID を含む HTTP 要求」なので TouchDevice をヘッダの端末で呼ぶ
// (ADR-0227 §5)。失敗した要求(404・400 invalid_input)でも、ヘッダ検証を通った後なら呼ぶ。
func TestFavoriteOperationsTouchDevice(t *testing.T) {
	ind := minimalIndividual(speciesGuard)
	tests := []struct {
		name, method, path string
		body               func(t *testing.T) []byte
	}{
		{"一覧", http.MethodGet, pathFavorites, nil},
		{"作成", http.MethodPost, pathFavorites, func(t *testing.T) []byte { return favoriteBody(t, nil, ind) }},
		{"削除(404)", http.MethodDelete, favoritePath("424242"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			var b []byte
			if tt.body != nil {
				b = tt.body(t)
			}
			serve(t, NewHandler(st), tt.method, tt.path, headers(deviceA), b)
			st.mu.Lock()
			defer st.mu.Unlock()
			touched := false
			for _, c := range st.calls {
				if c.method == "TouchDevice" && c.deviceID == deviceA {
					touched = true
				}
			}
			if !touched {
				t.Errorf("TouchDevice(%s) が呼ばれていない(ADR-0209 §4)。calls=%+v", deviceA, st.calls)
			}
		})
	}
}

// --- AC-F9: DB 障害 -------------------------------------------------------------

// AC-F9: DB に届かないときは、どの操作も 503 store_unavailable(内部のエラー文を外に出さない)。
func TestFavoriteOperationsReturnStoreUnavailable(t *testing.T) {
	tests := []struct {
		name, method, path string
		withBody           bool
	}{
		{"一覧", http.MethodGet, pathFavorites, false},
		{"作成", http.MethodPost, pathFavorites, true},
		{"削除", http.MethodDelete, favoritePath("1"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.unavailable = true
			var b []byte
			if tt.withBody {
				b = favoriteBody(t, nil, minimalIndividual(speciesGuard))
			}
			rec := serve(t, NewHandler(st), tt.method, tt.path, headers(deviceA), b)
			assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)
			if strings.Contains(rec.Body.String(), store.ErrUnavailable.Error()) {
				t.Errorf("DB のエラー文がクライアントに出ている: %s", rec.Body.String())
			}
		})
	}
}

// --- AC-F10: 全削除との整合(ADR-0209 §5) ----------------------------------------

// AC-F10: API で作ったお気に入りは、端末単位の全削除で消え、deleted.favorites に数えられる。
// 全削除の後でも新しいピン留めはできる(墓石は計算イベント用で、利用者の新しい操作は止めない。ADR-0227 §6)。
// 全削除は他端末のお気に入りを消さない(AC-P5)。
func TestPurgeDeletesFavoritesCreatedViaAPI(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	createFavorite(t, h, deviceA, nil, minimalIndividual(speciesGuard))
	createFavorite(t, h, deviceA, nil, minimalIndividual(speciesLeaf))
	createFavorite(t, h, deviceB, nil, minimalIndividual(speciesGuard))

	res := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))
	if res.Status != api.Completed || res.Deleted.Favorites != 2 {
		t.Errorf("全削除 = %+v, want completed・favorites 2", res)
	}
	if got := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)); len(got) != 0 {
		t.Errorf("全削除後の一覧 = %d 件, want 0", len(got))
	}
	if got := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceB), nil)); len(got) != 1 {
		t.Errorf("端末 B の一覧 = %d 件, want 1(A の全削除で消えない)", len(got))
	}

	// 全削除の後の新しいピン留めは保存される。
	createFavorite(t, h, deviceA, nil, minimalIndividual(speciesGuard))
}

// --- 更新は持たない(ADR-0227 §1) -------------------------------------------------

// 更新(PUT / PATCH)の操作は契約に無い。サーバーも受け付けず(2xx にしない)、行を変えない。
// ラベルを変えたいときは「削除して作り直す」(ADR-0227 §1)。
func TestFavoritesHaveNoUpdateOperation(t *testing.T) {
	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	for _, p := range []string{pathFavorites, "/api/record/favorites/{favoriteId}"} {
		item := doc.Paths.Find(p)
		if item == nil {
			t.Fatalf("契約に %s が無い", p)
		}
		if item.Put != nil || item.Patch != nil {
			t.Errorf("%s に PUT / PATCH がある(ADR-0227 §1: 更新は持たない)", p)
		}
	}

	st := newFakeStore()
	h := NewHandler(st)
	fav := createFavorite(t, h, deviceA, "元のラベル", minimalIndividual(speciesGuard))
	before := st.favoritesOf(deviceA)
	for _, m := range []string{http.MethodPut, http.MethodPatch} {
		rec := serve(t, h, m, favoritePath(fav.Id), headers(deviceA), favoriteBody(t, "新しいラベル", minimalIndividual(speciesGuard)))
		if rec.Code >= 200 && rec.Code < 300 {
			t.Errorf("%s の status = %d(更新は受け付けない)", m, rec.Code)
		}
	}
	after := st.favoritesOf(deviceA)
	if len(after) != 1 || string(after[0].Snapshot) != string(before[0].Snapshot) || !after[0].UpdatedAt.Equal(before[0].UpdatedAt) {
		t.Errorf("PUT / PATCH で行が変わった: before=%+v after=%+v", before, after)
	}
}

// --- AC-L1: ログ(ADR-0209 §3) ---------------------------------------------------

// AC-L1: お気に入りの中身(ラベル・個体の種族・技・持ち物・特性・性格)と要求本文はログに出さない。
// 成功・失敗(400・404・503)のどの経路でも同じ。出してよい端末 ID とエラーコードは失敗の経路で出る。
func TestFavoriteLogsDoNotLeakContent(t *testing.T) {
	const (
		forbiddenLabel   = "ログに出ないラベル"
		forbiddenSpecies = "9876-001"
		forbiddenNature  = "forbidden-nature"
		forbiddenItem    = "forbidden-item"
	)
	ind := minimalIndividual(forbiddenSpecies)
	ind["natureId"] = forbiddenNature
	ind["itemId"] = forbiddenItem
	bad := minimalIndividual(forbiddenSpecies)
	bad["natureId"] = forbiddenNature
	bad["sp"] = spBlock(33, 0, 0, 0, 0, 0)

	st := newFakeStore()
	buf := captureLogs(t)
	h := NewHandler(st)

	fav := createFavorite(t, h, deviceA, forbiddenLabel, ind)
	serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, forbiddenLabel, ind)) // 200(重複)
	serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, forbiddenLabel, bad)) // 400
	serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)
	serve(t, h, http.MethodDelete, favoritePath(fav.Id), headers(deviceB), nil) // 404
	st.unavailable = true
	serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, forbiddenLabel, ind)) // 503
	serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)                                   // 503

	logs := buf.String()
	for _, forbidden := range []string{forbiddenLabel, forbiddenSpecies, forbiddenNature, forbiddenItem, `"individual"`, `"label"`} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("ログに出してはいけない値 %q が含まれている(ADR-0209 §3):\n%s", forbidden, logs)
		}
	}
	for _, want := range []string{deviceA, string(api.StoreUnavailable)} {
		if !strings.Contains(logs, want) {
			t.Errorf("ログに %q が無い(失敗の経路で原因を追えない):\n%s", want, logs)
		}
	}
}

// --- 補助 ---------------------------------------------------------------------

func favoriteIDs(list []api.Favorite) []string {
	out := make([]string, 0, len(list))
	for _, f := range list {
		out = append(out, f.Id)
	}
	return out
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
