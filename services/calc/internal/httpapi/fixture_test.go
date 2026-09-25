package httpapi

// テスト用の架空マスタ(fakeStore)と、同じ入力を「HTTP のリクエスト」「engine/wasmapi のリクエスト」
// 「engine の入力」の3つの形で組み立てるビルダー。
//
// 種族・技・持ち物・特性・性格はすべて架空。相性表だけは testdata/golden/typechart.json
// (数値と英語 ID のみ。ADR-0015 と同じ扱い)を読む。httpapi のテストは master の実装に依存しないよう、
// Store を fake で差し替える(master の読み込みは master パッケージのテストが見る)。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/master"
)

const sharedTypeChartPath = "../../../../testdata/golden/typechart.json"

// 架空マスタの ID。
const (
	speciesAttacker = "9001-000" // テストモン(normal)
	speciesDefender = "9002-000" // テストガード(water/steel)
	speciesLeaf     = "9003-000" // テストリーフ(grass)
	speciesGhost    = "9004-000" // テストゴースト(ghost。無効相性の境界値テスト用。critic 指摘 R4)
	speciesUnknown  = "9999-000" // マスタに無い

	movePhysical = "test-beam"  // normal / physical / 80
	moveSpecial  = "test-wave"  // grass / special / 90
	moveStatus   = "test-glare" // normal / status / 0
	moveFire     = "test-flare" // fire / physical / 75

	itemOrb   = "test-orb"   // 最終ダメージ 5324
	itemShell = "test-shell" // 防御・特防 6144
	itemPlain = "test-stone" // 効果なし

	abilityPlain = "test-plain" // 効果なし
	abilityBoost = "test-boost" // タイプ一致 8192

	natureNeutral = "test-neutral-a" // 無補正の代表
	natureDefUp   = "test-def-up"    // +B/-A
	natureAtkUp   = "test-atk-up"    // +A/-C
	natureSpAUp   = "test-spa-up"    // +C/-A
	// +D/-A の性格は fake に置かない(natureId が null になる経路を見るため)。
)

// fakeStore は master.Store の架空実装。panicOn が true ならすべての参照で panic する。
type fakeStore struct {
	species   map[string]engine.Species
	moves     map[string]engine.Move
	items     map[string]engine.Item
	abilities map[string]engine.Ability
	natures   map[string]engine.Nature
	chart     engine.TypeChart
	panicOn   bool
}

var _ master.Store = (*fakeStore)(nil)

func (f *fakeStore) check() {
	if f.panicOn {
		panic("fake store: 意図的な panic(テスト)")
	}
}

func (f *fakeStore) Species(key string) (engine.Species, bool) {
	f.check()
	s, ok := f.species[key]
	if ok {
		s.Types = append([]engine.Type(nil), s.Types...)
		s.Abilities = append([]string(nil), s.Abilities...)
	}
	return s, ok
}

func (f *fakeStore) Move(id string) (engine.Move, bool) {
	f.check()
	m, ok := f.moves[id]
	return m, ok
}

func (f *fakeStore) Item(id string) (engine.Item, bool) {
	f.check()
	it, ok := f.items[id]
	return it, ok
}

func (f *fakeStore) Ability(id string) (engine.Ability, bool) {
	f.check()
	a, ok := f.abilities[id]
	return a, ok
}

func (f *fakeStore) Nature(id string) (engine.Nature, bool) {
	f.check()
	n, ok := f.natures[id]
	return n, ok
}

// NatureID は master.Store の規則(無補正は ID 昇順の最初、それ以外は一致する ID)を素直に写す。
func (f *fakeStore) NatureID(n engine.Nature) (string, bool) {
	f.check()
	ids := make([]string, 0, len(f.natures))
	for id := range f.natures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m := f.natures[id]
		if (n.IsNeutral() && m.IsNeutral()) || m == n {
			return id, true
		}
	}
	return "", false
}

func (f *fakeStore) TypeChart() engine.TypeChart {
	f.check()
	return f.chart
}

// rawTypeChart は typechart.json の types / effectiveness(wasmapi の typeChart DTO と同じ形)。
type rawTypeChart struct {
	Types         []string                  `json:"types"`
	Effectiveness map[string]map[string]int `json:"effectiveness"`
}

var (
	chartOnce  sync.Once
	chartRaw   rawTypeChart
	chartValue engine.TypeChart
	chartErr   error
)

// sharedTypeChart は共有の相性表を1度だけ読む(engine の表と、wasmapi に渡す JSON の両方)。
func sharedTypeChart(t testing.TB) (engine.TypeChart, rawTypeChart) {
	t.Helper()
	chartOnce.Do(func() {
		b, err := os.ReadFile(sharedTypeChartPath)
		if err != nil {
			chartErr = err
			return
		}
		if err := json.Unmarshal(b, &chartRaw); err != nil {
			chartErr = err
			return
		}
		data := engine.TypeChartData{Effectiveness: map[engine.Type]map[engine.Type]int{}}
		for _, ty := range chartRaw.Types {
			data.Types = append(data.Types, engine.Type(ty))
		}
		for atk, row := range chartRaw.Effectiveness {
			r := map[engine.Type]int{}
			for def, code := range row {
				r[engine.Type(def)] = code
			}
			data.Effectiveness[engine.Type(atk)] = r
		}
		chartValue, chartErr = engine.NewTypeChart(data)
	})
	if chartErr != nil {
		t.Fatalf("共有の相性表を読めない: %v", chartErr)
	}
	return chartValue, chartRaw
}

func newFakeStore(t testing.TB) *fakeStore {
	t.Helper()
	chart, _ := sharedTypeChart(t)
	return &fakeStore{
		species: map[string]engine.Species{
			speciesAttacker: {Key: speciesAttacker, DexNo: 9001, NameJa: "テストモン", Types: []engine.Type{engine.TypeNormal},
				BaseStats: engine.Stats{HP: 80, Atk: 100, Def: 70, SpA: 60, SpD: 70, Spe: 90}, Abilities: []string{abilityPlain}},
			speciesDefender: {Key: speciesDefender, DexNo: 9002, NameJa: "テストガード", Types: []engine.Type{engine.TypeWater, engine.TypeSteel},
				BaseStats: engine.Stats{HP: 95, Atk: 60, Def: 90, SpA: 70, SpD: 85, Spe: 50}, Abilities: []string{abilityPlain}},
			speciesLeaf: {Key: speciesLeaf, DexNo: 9003, NameJa: "テストリーフ", Types: []engine.Type{engine.TypeGrass},
				BaseStats: engine.Stats{HP: 70, Atk: 80, Def: 65, SpA: 95, SpD: 80, Spe: 85}, Abilities: []string{abilityBoost}},
			speciesGhost: {Key: speciesGhost, DexNo: 9004, NameJa: "テストゴースト", Types: []engine.Type{engine.TypeGhost},
				BaseStats: engine.Stats{HP: 90, Atk: 70, Def: 90, SpA: 70, SpD: 90, Spe: 60}, Abilities: []string{abilityPlain}},
		},
		moves: map[string]engine.Move{
			movePhysical: {ID: movePhysical, NameJa: "テストビーム", Type: engine.TypeNormal, Category: engine.CategoryPhysical, Power: 80},
			moveSpecial:  {ID: moveSpecial, NameJa: "テストウェーブ", Type: engine.TypeGrass, Category: engine.CategorySpecial, Power: 90},
			moveStatus:   {ID: moveStatus, NameJa: "テストにらみ", Type: engine.TypeNormal, Category: engine.CategoryStatus},
			moveFire:     {ID: moveFire, NameJa: "テストフレア", Type: engine.TypeFire, Category: engine.CategoryPhysical, Power: 75},
		},
		items: map[string]engine.Item{
			itemOrb:   {ID: itemOrb, NameJa: "テストのたま", Effect: &engine.ItemEffect{DamageMod: 5324}},
			itemShell: {ID: itemShell, NameJa: "テストのから", Effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatDef: 6144, engine.StatSpD: 6144}}},
			itemPlain: {ID: itemPlain, NameJa: "テストのいし"},
		},
		abilities: map[string]engine.Ability{
			abilityPlain: {ID: abilityPlain, NameJa: "テストとくせい"},
			abilityBoost: {ID: abilityBoost, NameJa: "テストいっち", Effect: &engine.AbilityEffect{StabMod: 8192}},
		},
		natures: map[string]engine.Nature{
			natureNeutral:    engine.NatureNeutral,
			"test-neutral-z": engine.NatureNeutral, // 代表にならない方(ID が大きい)
			natureDefUp:      {Plus: engine.StatDef, Minus: engine.StatAtk},
			natureAtkUp:      {Plus: engine.StatAtk, Minus: engine.StatSpA},
			natureSpAUp:      {Plus: engine.StatSpA, Minus: engine.StatAtk},
		},
		chart: chart,
	}
}

// indiv は1体の個体を ID で表したもの。3つの形(HTTP / wasmapi / engine)に写せる。
type indiv struct {
	speciesKey string
	natureID   string
	abilityID  string // "" は省略
	itemID     string // "" は省略(持ち物なし)
	moveID     string // "" は省略(Individual.moveId)
	sp         engine.Stats
	ranks      engine.Ranks
	tera       engine.Type   // "" は省略
	status     engine.Status // "" は省略
}

func statsMap(s engine.Stats) map[string]any {
	return map[string]any{"hp": s.HP, "atk": s.Atk, "def": s.Def, "spa": s.SpA, "spd": s.SpD, "spe": s.Spe}
}

func ranksMap(r engine.Ranks) map[string]any {
	return map[string]any{"atk": r.Atk, "def": r.Def, "spa": r.SpA, "spd": r.SpD, "spe": r.Spe}
}

// http は HTTP(api.Individual)の形。
func (in indiv) http() map[string]any {
	m := map[string]any{
		"speciesKey": in.speciesKey,
		"natureId":   in.natureID,
		"sp":         statsMap(in.sp),
		"ranks":      ranksMap(in.ranks),
	}
	if in.abilityID != "" {
		m["abilityId"] = in.abilityID
	}
	if in.itemID != "" {
		m["itemId"] = in.itemID
	}
	if in.moveID != "" {
		m["moveId"] = in.moveID
	}
	if in.tera != "" {
		m["teraType"] = string(in.tera)
	}
	if in.status != "" {
		m["status"] = string(in.status)
	}
	return m
}

// engine は同じ個体を engine.Individual にする(ID は fake から解決する)。
func (in indiv) engine(t testing.TB, f *fakeStore) engine.Individual {
	t.Helper()
	sp, ok := f.species[in.speciesKey]
	if !ok {
		t.Fatalf("fixture の種族 %q が無い", in.speciesKey)
	}
	n, ok := f.natures[in.natureID]
	if !ok {
		t.Fatalf("fixture の性格 %q が無い", in.natureID)
	}
	out := engine.Individual{
		Species: sp, Level: engine.DefaultLevel, Nature: n, SP: in.sp, Ranks: in.ranks,
		TeraType: in.tera, Status: engine.StatusNone,
	}
	if in.status != "" {
		out.Status = in.status
	}
	if in.abilityID != "" {
		out.Ability = f.abilities[in.abilityID]
	}
	if in.itemID != "" {
		it := f.items[in.itemID]
		out.Item = &it
	}
	return out
}

// wasm は同じ個体を engine/wasmapi の individual DTO(解決済みの実体)にする。
func (in indiv) wasm(t testing.TB, f *fakeStore) map[string]any {
	t.Helper()
	e := in.engine(t, f)
	return map[string]any{
		"species":  wasmSpecies(e.Species),
		"level":    engine.DefaultLevel,
		"nature":   wasmNature(e.Nature),
		"ability":  wasmAbility(e.Ability),
		"item":     wasmItem(e.Item),
		"sp":       statsMap(e.SP),
		"ranks":    ranksMap(e.Ranks),
		"teraType": string(e.TeraType),
		"status":   string(e.Status),
	}
}

func wasmSpecies(s engine.Species) map[string]any {
	types := make([]string, 0, len(s.Types))
	for _, ty := range s.Types {
		types = append(types, string(ty))
	}
	return map[string]any{
		"key": s.Key, "dexNo": s.DexNo, "form": s.Form, "nameJa": s.NameJa,
		"types": types, "baseStats": statsMap(s.BaseStats), "abilities": s.Abilities,
	}
}

func wasmNature(n engine.Nature) map[string]any {
	return map[string]any{"plus": string(n.Plus), "minus": string(n.Minus)}
}

func wasmMove(m engine.Move) map[string]any {
	return map[string]any{
		"id": m.ID, "nameJa": m.NameJa, "type": string(m.Type), "category": string(m.Category),
		"power": m.Power, "priority": m.Priority,
	}
}

func wasmAbility(a engine.Ability) map[string]any {
	out := map[string]any{"id": a.ID, "nameJa": a.NameJa, "effect": nil}
	if e := a.Effect; e != nil {
		resist := map[string]any{}
		for ty, v := range e.DefResistType {
			resist[string(ty)] = v
		}
		out["effect"] = map[string]any{
			"stabMod": e.StabMod, "offBoostType": string(e.OffBoostType), "offBoostTypeMod": e.OffBoostTypeMod,
			"defResistType": resist, "reduceSuperEffective": e.ReduceSuperEffective, "ignoresBurn": e.IgnoresBurn,
			"airborne": e.Airborne,
		}
	}
	return out
}

func wasmItem(it *engine.Item) any {
	if it == nil {
		return nil
	}
	out := map[string]any{"id": it.ID, "nameJa": it.NameJa, "effect": nil}
	if e := it.Effect; e != nil {
		mods := map[string]any{}
		for k, v := range e.StatMods {
			mods[string(k)] = v
		}
		out["effect"] = map[string]any{
			"statMods": mods, "damageMod": e.DamageMod, "powerMod": e.PowerMod, "powerCategory": string(e.PowerCategory),
			"onlySuperEffective": e.OnlySuperEffective, "boostType": string(e.BoostType), "boostTypeMod": e.BoostTypeMod,
			"resistBerryType": string(e.ResistBerryType),
		}
	}
	return out
}

func wasmTypeChart(t testing.TB) map[string]any {
	t.Helper()
	_, raw := sharedTypeChart(t)
	return map[string]any{"types": raw.Types, "effectiveness": raw.Effectiveness}
}

// --- HTTP の呼び出し --------------------------------------------------------

const (
	testDeviceID  = "00000000-0000-4000-8000-000000000001"
	testSessionID = "00000000-0000-4000-8000-000000000002"
)

// validHeaders は必須ヘッダ(X-Device-Id / X-Session-Id)をそろえたヘッダ。
func validHeaders() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("X-Device-Id", testDeviceID)
	h.Set("X-Session-Id", testSessionID)
	return h
}

func mustJSON(t testing.TB, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("JSON 化に失敗: %v", err)
	}
	return b
}

// serve は handler に1リクエストを送る。body が nil なら本文なし。
func serve(t testing.TB, h http.Handler, method, path string, header http.Header, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// post は妥当なヘッダで JSON を POST し、レスポンスを契約(api/openapi.yaml)に照らして検証する。
// checkRequest が true ならリクエストも契約に照らす(成功ケースの入力が契約どおりであることの確認)。
func post(t *testing.T, h http.Handler, path string, body []byte, checkRequest bool) *httptest.ResponseRecorder {
	t.Helper()
	header := validHeaders()
	rec := serve(t, h, http.MethodPost, path, header, body)
	assertContract(t, http.MethodPost, path, header, body, rec, checkRequest)
	return rec
}

// errorBody はエラー本文。未知のフィールドを許さずに読む(Error 形式ちょうどであること)。
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// decodeErrorBody は本文が {"code","message"} ちょうどであることを確かめて返す。
func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	dec.DisallowUnknownFields()
	var e errorBody
	if err := dec.Decode(&e); err != nil {
		t.Fatalf("エラー本文が Error 形式でない: %v; body=%s", err, rec.Body.String())
	}
	if e.Code == "" || e.Message == "" {
		t.Fatalf("エラー本文の code / message が空: %s", rec.Body.String())
	}
	return e
}

// assertError は HTTP ステータスと code を確かめる。
func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if got := decodeErrorBody(t, rec); got.Code != wantCode {
		t.Errorf("code = %q, want %q; message=%q", got.Code, wantCode, got.Message)
	}
}

func decodeInto(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("レスポンスを読めない: %v; body=%s", err, rec.Body.String())
	}
}

// tenths は engine の 0.1% 単位の整数を、API が返す小数第1位の値にする(float の近似ではなく ÷10 のみ)。
func tenths(v int) float64 { return float64(v) / 10 }
