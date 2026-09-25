// Package storetest はテスト専用の偽の store.Querier と架空データ(ADR-0105 §7)。
//
// pokedex-svc の読み出し(httpapi・readmodel)のテストが、実 DB の代わりに使う。本番のコードから import しない
// (import してよいのは *_test.go だけ。readmodel / httpapi の静的検査は layout_test で固定する)。
//
// 架空データの規則は ADR-0100 §7 と同じ: 図鑑番号 9001 以降・ID は test で始まる英小文字・日本語名は「テスト」で始まる。
// タイプ ID だけは一般名(fire 等)を使う(balance の JSON Schema が 18 タイプの列挙を持つため)。
//
// Querier に埋め込んだ store.Querier は nil で、上書きしていないメソッドを呼ぶと panic する
// (テストが想定していないクエリ・書き込みを呼んだことに気づくため)。
package storetest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"example.com/pokecalc/services/pokedex/internal/store"
)

// DefaultRegulationID は架空データの既定のレギュレーション。
const DefaultRegulationID = "test-a"

// ErrDB は DB の失敗を模した値(接続できない等)。
var ErrDB = errors.New("storetest: DB に接続できない")

// Querier は偽の store.Querier。フィールドを書き換えて各テストの状態を作る。
// Err* が非 nil ならそのメソッドはその値を返す。
type Querier struct {
	store.Querier

	DataVersions        []store.DataVersion
	Types               []store.Type
	TypeChart           []store.TypeChart
	Species             []store.Species
	SpeciesAbilities    []store.SpeciesAbility
	Moves               []store.Move
	MoveEffects         []store.MoveEffect
	MoveMechanisms      []store.MoveMechanism
	Items               []store.Item
	ItemEffects         []store.ItemEffect
	Abilities           []store.Ability
	AbilityEffects      []store.AbilityEffect
	Natures             []store.Nature
	Learnsets           []store.Learnset
	DefaultRegulation   *store.GetDefaultRegulationRow // nil = 既定のレギュレーションが無い(sql.ErrNoRows)
	RegulationSpecies   map[string][]string            // regulation_id → species_key
	RegulationMoves     map[string][]string
	RegulationItems     map[string][]string
	RegulationAbilities map[string][]string

	// Err は全メソッドに共通の失敗(nil なら成功)。ErrByMethod はメソッド名ごとの失敗。
	Err         error
	ErrByMethod map[string]error

	mu    sync.Mutex
	Calls []Call // 呼ばれたメソッドと引数(検索のパターン・件数の確認用)
}

// Call は呼び出しの記録。
type Call struct {
	Method string
	Arg    any
}

func (q *Querier) record(method string, arg any) error {
	q.mu.Lock()
	q.Calls = append(q.Calls, Call{Method: method, Arg: arg})
	q.mu.Unlock()
	if err := q.ErrByMethod[method]; err != nil {
		return err
	}
	return q.Err
}

// CallsOf は method の呼び出しの引数を順に返す。
func (q *Querier) CallsOf(method string) []any {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []any
	for _, c := range q.Calls {
		if c.Method == method {
			out = append(out, c.Arg)
		}
	}
	return out
}

func (q *Querier) ListDataVersions(context.Context) ([]store.DataVersion, error) {
	if err := q.record("ListDataVersions", nil); err != nil {
		return nil, err
	}
	return append([]store.DataVersion(nil), q.DataVersions...), nil
}

func (q *Querier) ListTypes(context.Context) ([]store.Type, error) {
	if err := q.record("ListTypes", nil); err != nil {
		return nil, err
	}
	return append([]store.Type(nil), q.Types...), nil
}

func (q *Querier) ListTypeChart(context.Context) ([]store.TypeChart, error) {
	if err := q.record("ListTypeChart", nil); err != nil {
		return nil, err
	}
	return append([]store.TypeChart(nil), q.TypeChart...), nil
}

func (q *Querier) ListSpecies(context.Context) ([]store.Species, error) {
	if err := q.record("ListSpecies", nil); err != nil {
		return nil, err
	}
	return append([]store.Species(nil), q.Species...), nil
}

func (q *Querier) ListAllSpeciesAbilities(context.Context) ([]store.SpeciesAbility, error) {
	if err := q.record("ListAllSpeciesAbilities", nil); err != nil {
		return nil, err
	}
	return append([]store.SpeciesAbility(nil), q.SpeciesAbilities...), nil
}

func (q *Querier) ListMoves(context.Context) ([]store.Move, error) {
	if err := q.record("ListMoves", nil); err != nil {
		return nil, err
	}
	return append([]store.Move(nil), q.Moves...), nil
}

func (q *Querier) ListMoveEffects(context.Context) ([]store.MoveEffect, error) {
	if err := q.record("ListMoveEffects", nil); err != nil {
		return nil, err
	}
	return append([]store.MoveEffect(nil), q.MoveEffects...), nil
}

func (q *Querier) ListMoveMechanisms(context.Context) ([]store.MoveMechanism, error) {
	if err := q.record("ListMoveMechanisms", nil); err != nil {
		return nil, err
	}
	return append([]store.MoveMechanism(nil), q.MoveMechanisms...), nil
}

func (q *Querier) ListItems(context.Context) ([]store.Item, error) {
	if err := q.record("ListItems", nil); err != nil {
		return nil, err
	}
	return append([]store.Item(nil), q.Items...), nil
}

func (q *Querier) ListItemEffects(context.Context) ([]store.ItemEffect, error) {
	if err := q.record("ListItemEffects", nil); err != nil {
		return nil, err
	}
	return append([]store.ItemEffect(nil), q.ItemEffects...), nil
}

func (q *Querier) ListAbilities(context.Context) ([]store.Ability, error) {
	if err := q.record("ListAbilities", nil); err != nil {
		return nil, err
	}
	return append([]store.Ability(nil), q.Abilities...), nil
}

func (q *Querier) ListAbilityEffects(context.Context) ([]store.AbilityEffect, error) {
	if err := q.record("ListAbilityEffects", nil); err != nil {
		return nil, err
	}
	return append([]store.AbilityEffect(nil), q.AbilityEffects...), nil
}

func (q *Querier) ListNatures(context.Context) ([]store.Nature, error) {
	if err := q.record("ListNatures", nil); err != nil {
		return nil, err
	}
	return append([]store.Nature(nil), q.Natures...), nil
}

func (q *Querier) GetDefaultRegulation(context.Context) (store.GetDefaultRegulationRow, error) {
	if err := q.record("GetDefaultRegulation", nil); err != nil {
		return store.GetDefaultRegulationRow{}, err
	}
	if q.DefaultRegulation == nil {
		return store.GetDefaultRegulationRow{}, sql.ErrNoRows
	}
	return *q.DefaultRegulation, nil
}

func (q *Querier) ListRegulationSpeciesKeys(_ context.Context, regulationID string) ([]string, error) {
	if err := q.record("ListRegulationSpeciesKeys", regulationID); err != nil {
		return nil, err
	}
	return append([]string(nil), q.RegulationSpecies[regulationID]...), nil
}

func (q *Querier) ListRegulationMoveIDs(_ context.Context, regulationID string) ([]string, error) {
	if err := q.record("ListRegulationMoveIDs", regulationID); err != nil {
		return nil, err
	}
	return append([]string(nil), q.RegulationMoves[regulationID]...), nil
}

func (q *Querier) ListRegulationAbilityIDs(_ context.Context, regulationID string) ([]string, error) {
	if err := q.record("ListRegulationAbilityIDs", regulationID); err != nil {
		return nil, err
	}
	return append([]string(nil), q.RegulationAbilities[regulationID]...), nil
}

// 検索: 偽物は「集合に含まれるものを全件」返し、パターン・件数は記録するだけ(LIKE の評価は DB の仕事。
// 実際の前方一致と照合順序は db の -tags mysql のテストが確かめる)。ただし Limit は守る。

func (q *Querier) SearchSpecies(_ context.Context, arg store.SearchSpeciesParams) ([]store.SearchSpeciesRow, error) {
	if err := q.record("SearchSpecies", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationSpecies[arg.RegulationID])
	var out []store.SearchSpeciesRow
	for _, s := range q.Species {
		if in[s.Key] && len(out) < int(arg.Limit) {
			out = append(out, store.SearchSpeciesRow{Key: s.Key, DexNo: s.DexNo, Form: s.Form, NameJa: s.NameJa, Type1: s.Type1, Type2: s.Type2})
		}
	}
	return out, nil
}

func (q *Querier) SearchMoves(_ context.Context, arg store.SearchMovesParams) ([]store.SearchMovesRow, error) {
	if err := q.record("SearchMoves", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationMoves[arg.RegulationID])
	var out []store.SearchMovesRow
	for _, m := range q.Moves {
		if in[m.ID] && len(out) < int(arg.Limit) {
			out = append(out, store.SearchMovesRow{ID: m.ID, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: m.Priority})
		}
	}
	return out, nil
}

func (q *Querier) SearchItems(_ context.Context, arg store.SearchItemsParams) ([]store.SearchItemsRow, error) {
	if err := q.record("SearchItems", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationItems[arg.RegulationID])
	var out []store.SearchItemsRow
	for _, it := range q.Items {
		if in[it.ID] && len(out) < int(arg.Limit) {
			out = append(out, store.SearchItemsRow{ID: it.ID, NameJa: it.NameJa})
		}
	}
	return out, nil
}

func (q *Querier) GetSpeciesByKey(_ context.Context, key string) (store.Species, error) {
	if err := q.record("GetSpeciesByKey", key); err != nil {
		return store.Species{}, err
	}
	for _, s := range q.Species {
		if s.Key == key {
			return s, nil
		}
	}
	return store.Species{}, sql.ErrNoRows
}

func (q *Querier) GetMove(_ context.Context, id string) (store.GetMoveRow, error) {
	if err := q.record("GetMove", id); err != nil {
		return store.GetMoveRow{}, err
	}
	for _, m := range q.Moves {
		if m.ID == id {
			return store.GetMoveRow{ID: m.ID, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: m.Priority}, nil
		}
	}
	return store.GetMoveRow{}, sql.ErrNoRows
}

func (q *Querier) GetMovesByIDs(_ context.Context, ids []string) ([]store.GetMovesByIDsRow, error) {
	if err := q.record("GetMovesByIDs", ids); err != nil {
		return nil, err
	}
	in := set(ids)
	var out []store.GetMovesByIDsRow
	for _, m := range q.Moves {
		if in[m.ID] {
			out = append(out, store.GetMovesByIDsRow{ID: m.ID, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: m.Priority})
		}
	}
	return out, nil
}

func (q *Querier) ListSpeciesAbilityNames(_ context.Context, speciesKey string) ([]store.ListSpeciesAbilityNamesRow, error) {
	if err := q.record("ListSpeciesAbilityNames", speciesKey); err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, a := range q.Abilities {
		names[a.ID] = a.NameJa
	}
	var out []store.ListSpeciesAbilityNamesRow
	for _, sa := range q.SpeciesAbilities {
		if sa.SpeciesKey == speciesKey {
			out = append(out, store.ListSpeciesAbilityNamesRow{Slot: sa.Slot, ID: sa.AbilityID, NameJa: names[sa.AbilityID]})
		}
	}
	return out, nil
}

func (q *Querier) ListSpeciesLearnset(_ context.Context, arg store.ListSpeciesLearnsetParams) ([]string, error) {
	if err := q.record("ListSpeciesLearnset", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationMoves[arg.RegulationID])
	var out []string
	for _, l := range q.Learnsets {
		if l.SpeciesKey == arg.SpeciesKey && in[l.MoveID] {
			out = append(out, l.MoveID)
		}
	}
	return out, nil
}

func set(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// New は架空データを入れた偽の Querier を返す(呼ぶたびに新しいコピー)。
//
//   - 種族: 9001-000 テストモン(fire。特性 slot1 testblaze・slot3 testguard)、9001-001 テストメガモン(メガ。fire/water)、
//     9002-000 テストリーフ(grass。特性 slot1〜4 の4つ)、9003-000 テストキンシ(water。既定のレギュレーションの外)
//   - 技: testflame / teststrike / testglare(使用可能)、testbanned(使用可能集合の外)
//   - 持ち物: testorb(効果あり)・teststone(メガストーン。効果なし)・testberry(効果あり)、testplain(集合の外)
//   - 特性: testblaze(攻撃側の効果だけ)・testguard(炎・水を半減)・testleafy(抜群を 3/4)・testhidden・teststance・testspecial、
//     testunused(集合の外。効果あり)
//   - 性格: testbrave(atk↑ spe↓)・testneutral(無補正)
func New() *Querier {
	imported := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	return &Querier{
		DataVersions: []store.DataVersion{
			{Source: "showdown", Version: "abad1deaabad1deaabad1deaabad1deaabad1dea", Checksum: "1111111111111111111111111111111111111111111111111111111111111111", ImportedAt: imported},
			{Source: "calc", Version: "test-calc-1", Checksum: "2222222222222222222222222222222222222222222222222222222222222222", ImportedAt: imported},
			{Source: "pokeapi", Version: "cafef00dcafef00dcafef00dcafef00dcafef00d", Checksum: "3333333333333333333333333333333333333333333333333333333333333333", ImportedAt: imported},
		},
		Types: []store.Type{
			{ID: "normal", SortOrder: 1, NameJa: "テストノーマル", NameJaSource: "pokeapi"},
			{ID: "fire", SortOrder: 2, NameJa: "テストほのお", NameJaSource: "pokeapi"},
			{ID: "water", SortOrder: 3, NameJa: "テストみず", NameJaSource: "pokeapi"},
			{ID: "grass", SortOrder: 4, NameJa: "テストくさ", NameJaSource: "override"},
		},
		TypeChart: []store.TypeChart{
			{AttackType: "fire", DefenseType: "grass", Code: 4},
			{AttackType: "fire", DefenseType: "water", Code: 1},
			{AttackType: "water", DefenseType: "fire", Code: 4},
			{AttackType: "grass", DefenseType: "water", Code: 4},
			{AttackType: "grass", DefenseType: "fire", Code: 1},
			{AttackType: "normal", DefenseType: "normal", Code: 2},
		},
		Species: []store.Species{
			{Key: "9001-000", DexNo: 9001, Form: 0, ShowdownID: "testmon", NameJa: "テストモン", NameJaSource: "pokeapi", NameEn: "Testmon",
				Type1: "fire", BaseHp: 80, BaseAtk: 90, BaseDef: 70, BaseSpa: 100, BaseSpd: 75, BaseSpe: 85},
			{Key: "9001-001", DexNo: 9001, Form: 1, ShowdownID: "testmonmega", NameJa: "テストメガモン", NameJaSource: "pokeapi", NameEn: "Testmon-Mega",
				Type1: "fire", Type2: ns("water"), BaseHp: 80, BaseAtk: 120, BaseDef: 90, BaseSpa: 130, BaseSpd: 95, BaseSpe: 105,
				IsMega: true, BaseSpeciesKey: ns("9001-000"), RequiredItemID: ns("teststone")},
			{Key: "9002-000", DexNo: 9002, Form: 0, ShowdownID: "testleaf", NameJa: "テストリーフ", NameJaSource: "pokeapi", NameEn: "Testleaf",
				Type1: "grass", BaseHp: 60, BaseAtk: 60, BaseDef: 60, BaseSpa: 60, BaseSpd: 60, BaseSpe: 60},
			{Key: "9003-000", DexNo: 9003, Form: 0, ShowdownID: "testbanned", NameJa: "テストキンシ", NameJaSource: "override", NameEn: "Testbanned",
				Type1: "water", BaseHp: 50, BaseAtk: 50, BaseDef: 50, BaseSpa: 50, BaseSpd: 50, BaseSpe: 200},
		},
		SpeciesAbilities: []store.SpeciesAbility{
			{SpeciesKey: "9001-000", Slot: 3, AbilityID: "testguard"},
			{SpeciesKey: "9001-000", Slot: 1, AbilityID: "testblaze"},
			{SpeciesKey: "9001-001", Slot: 1, AbilityID: "teststance"},
			{SpeciesKey: "9002-000", Slot: 4, AbilityID: "testspecial"},
			{SpeciesKey: "9002-000", Slot: 2, AbilityID: "testguard"},
			{SpeciesKey: "9002-000", Slot: 1, AbilityID: "testleafy"},
			{SpeciesKey: "9002-000", Slot: 3, AbilityID: "testhidden"},
			{SpeciesKey: "9003-000", Slot: 1, AbilityID: "testunused"},
		},
		Moves: []store.Move{
			{ID: "testflame", NameJa: "テストほのおわざ", NameJaSource: "pokeapi", NameEn: "Test Flame", Type: "fire", Category: "special", Power: 90, Accuracy: sql.NullInt16{Int16: 100, Valid: true}, Pp: 15, Priority: 0},
			{ID: "teststrike", NameJa: "テストうちこみ", NameJaSource: "override", NameEn: "Test Strike", Type: "normal", Category: "physical", Power: 40, Accuracy: sql.NullInt16{Int16: 100, Valid: true}, Pp: 30, Priority: 1},
			{ID: "testglare", NameJa: "テストにらみ", NameJaSource: "pokeapi", NameEn: "Test Glare", Type: "normal", Category: "status", Power: 0, Pp: 30, Priority: 0},
			{ID: "testbanned", NameJa: "テストきんじて", NameJaSource: "pokeapi", NameEn: "Test Banned", Type: "water", Category: "special", Power: 80, Accuracy: sql.NullInt16{Int16: 100, Valid: true}, Pp: 10, Priority: 0},
		},
		// testflame は複数の機構を持つ(わざと逆順に入れて、応答が昇順に並び替わることを確認する。ADR-0121)。
		// teststrike 等は機構を持たない(通常の技。応答は空配列になる)。
		MoveMechanisms: []store.MoveMechanism{
			{MoveID: "testflame", Mechanism: "variable_power"},
			{MoveID: "testflame", Mechanism: "multi_hit"},
		},
		Items: []store.Item{
			{ID: "testorb", NameJa: "テストだま", NameJaSource: "pokeapi", NameEn: "Test Orb"},
			{ID: "teststone", NameJa: "テストナイト", NameJaSource: "pokeapi", NameEn: "Teststone"},
			{ID: "testberry", NameJa: "テストのみ", NameJaSource: "override", NameEn: "Test Berry"},
			{ID: "testplain", NameJa: "テストふつう", NameJaSource: "pokeapi", NameEn: "Test Plain"},
		},
		// MySQL の JSON 列は正規化した文字列(キーの後に空白)を返す。
		ItemEffects: []store.ItemEffect{
			{ItemID: "testorb", Effect: json.RawMessage(`{"DamageMod": 5324}`)},
			{ItemID: "testberry", Effect: json.RawMessage(`{"ResistBerryType": "fire"}`)},
		},
		Abilities: []store.Ability{
			{ID: "testblaze", NameJa: "テストもうか", NameJaSource: "pokeapi", NameEn: "Test Blaze"},
			{ID: "testguard", NameJa: "テストまもり", NameJaSource: "pokeapi", NameEn: "Test Guard"},
			{ID: "testleafy", NameJa: "テストはっぱ", NameJaSource: "pokeapi", NameEn: "Test Leafy"},
			{ID: "testhidden", NameJa: "テストかくれ", NameJaSource: "override", NameEn: "Test Hidden"},
			{ID: "teststance", NameJa: "テストかまえ", NameJaSource: "pokeapi", NameEn: "Test Stance"},
			{ID: "testspecial", NameJa: "テストとくべつ", NameJaSource: "override", NameEn: "Test Special"},
			{ID: "testunused", NameJa: "テストみしよう", NameJaSource: "override", NameEn: "Test Unused"},
		},
		AbilityEffects: []store.AbilityEffect{
			{AbilityID: "testblaze", Effect: json.RawMessage(`{"OffBoostType": "fire", "OffBoostTypeMod": 6144}`)},
			{AbilityID: "testguard", Effect: json.RawMessage(`{"DefResistType": {"water": 2048, "fire": 2048}}`)},
			{AbilityID: "testleafy", Effect: json.RawMessage(`{"ReduceSuperEffective": 3072}`)},
			{AbilityID: "testunused", Effect: json.RawMessage(`{"StabMod": 8192}`)},
		},
		Natures: []store.Nature{
			{ID: "testbrave", NameJa: "テストゆうかん", NameJaSource: "pokeapi", NameEn: "Testbrave", Plus: ns("atk"), Minus: ns("spe")},
			{ID: "testneutral", NameJa: "テストむほせい", NameJaSource: "override", NameEn: "Testneutral"},
		},
		Learnsets: []store.Learnset{
			{SpeciesKey: "9001-000", MoveID: "testflame"},
			{SpeciesKey: "9001-000", MoveID: "teststrike"},
			{SpeciesKey: "9001-000", MoveID: "testbanned"},
			{SpeciesKey: "9002-000", MoveID: "testglare"},
		},
		DefaultRegulation: &store.GetDefaultRegulationRow{ID: DefaultRegulationID, NameJa: "テストレギュレーションA", IsDefault: true},
		RegulationSpecies: map[string][]string{
			DefaultRegulationID: {"9001-000", "9001-001", "9002-000"},
			"test-b":            {"9003-000"},
		},
		RegulationMoves: map[string][]string{
			DefaultRegulationID: {"testflame", "testglare", "teststrike"},
			"test-b":            {"testbanned"},
		},
		RegulationItems: map[string][]string{
			DefaultRegulationID: {"testberry", "testorb", "teststone"},
		},
		RegulationAbilities: map[string][]string{
			DefaultRegulationID: {"testblaze", "testguard", "testhidden", "testleafy", "testspecial", "teststance"},
			"test-b":            {"testunused"},
		},
	}
}
