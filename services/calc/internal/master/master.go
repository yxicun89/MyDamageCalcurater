// Package master は calc-svc の**暫定の**マスタ境界(ADR-0016)。
//
// データレーンの共通マスタ(services/internal/master。plan.md P2-2a)が main に入ったら、
// Store の実装をそちらに差し替える。httpapi は Store インターフェースにだけ依存する。
//
// 起動時にスナップショット(暫定 JSON スキーマ。services/calc/README.md)と
// タイプ相性表(testdata/golden/typechart.json と同じ schema。ADR-0013 / ADR-0015)を
// メモリに読み込む。フォールバックの既定データは持たない(ADR-0013)。
package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// ロード時の失敗。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidSnapshot はマスタのスナップショットが暫定スキーマを満たさない
	// (壊れた JSON・未知のフィールド・ID の重複・未知のタイプ/分類/ステータスキー・性格が HP を指す等)。
	ErrInvalidSnapshot = errors.New("マスタのスナップショットが不正")
	// ErrInvalidTypeChart はタイプ相性表のデータが schema を満たさない、または表として不正。
	ErrInvalidTypeChart = errors.New("タイプ相性表のデータが不正")
)

// --- 列挙(生成型 api.PokeType / api.StatKey / api.MoveCategory の Valid() を使う。
// 18種類のタイプ・6ステータスキー・3分類は openapi.yaml が正で、engine の定数と同じ文字列。
// 独自の一覧を持たない(critic 指摘 O1: engine/wasmapi/api とここで別々に列挙を持つと
// openapi.yaml を直しても追従し忘れる余地ができる)。 ------------------------------------

// validType はタイプの綴りを検証する(空・未知は不正)。
func validType(v string) (engine.Type, bool) {
	if !api.PokeType(v).Valid() {
		return "", false
	}
	return engine.Type(v), true
}

// validStatKey はステータスキーの綴りを検証する(空・未知は不正)。
func validStatKey(v string) (engine.StatKey, bool) {
	if !api.StatKey(v).Valid() {
		return "", false
	}
	return engine.StatKey(v), true
}

// validCategory は技の分類の綴りを検証する(空・未知は不正)。
func validCategory(v string) (engine.MoveCategory, bool) {
	if !api.MoveCategory(v).Valid() {
		return "", false
	}
	return engine.MoveCategory(v), true
}

// parseRequiredType はタイプを検証する(空・未知は不正)。
func parseRequiredType(v string) (engine.Type, bool) {
	return validType(v)
}

// parseOptionalType は "" を TypeNone として許すタイプ検証。
func parseOptionalType(v string) (engine.Type, error) {
	if v == "" {
		return engine.TypeNone, nil
	}
	t, ok := validType(v)
	if !ok {
		return "", fmt.Errorf("%w: 未知のタイプ %q", ErrInvalidSnapshot, v)
	}
	return t, nil
}

// parseOptionalCategory は "" を「全分類」として許す分類検証。
func parseOptionalCategory(v string) (engine.MoveCategory, error) {
	if v == "" {
		return "", nil
	}
	c, ok := validCategory(v)
	if !ok {
		return "", fmt.Errorf("%w: 未知の分類 %q", ErrInvalidSnapshot, v)
	}
	return c, nil
}

// Store は calc-svc が計算に使うマスタの参照口。実装は並行に呼ばれても安全であること。
// 返す値は呼び出し側が書き換えても Store の中身に影響しない(スライスは複製して返す)。
type Store interface {
	// Species は種族キー({図鑑番号4桁}-{フォルム3桁})で種族を引く。
	Species(key string) (engine.Species, bool)
	// Move は技 ID で技を引く。
	Move(id string) (engine.Move, bool)
	// Item は持ち物 ID で持ち物(効果つき)を引く。
	Item(id string) (engine.Item, bool)
	// Ability は特性 ID で特性(効果つき)を引く。
	Ability(id string) (engine.Ability, bool)
	// Nature は性格 ID で性格補正を引く。
	Nature(id string) (engine.Nature, bool)
	// NatureID は性格補正の構造値から性格 ID を引く(ADR-0016)。
	//   - 無補正(n.IsNeutral())は、マスタ中の無補正性格を ID の昇順で並べた最初のもの。
	//   - それ以外は (Plus, Minus) が一致する性格のうち ID の昇順で最初のもの。
	//   - 該当が無ければ ("", false)。
	NatureID(n engine.Nature) (string, bool)
	// TypeChart は検証済みのタイプ相性表を返す。
	TypeChart() engine.TypeChart
}

// Snapshot は検証済みのマスタのスナップショット(暫定スキーマ。services/calc/README.md)。
type Snapshot struct {
	species   map[string]engine.Species
	moves     map[string]engine.Move
	items     map[string]engine.Item
	abilities map[string]engine.Ability
	natures   map[string]engine.Nature
}

// --- スナップショットの JSON 形(暫定スキーマ) -------------------------------

type snapshotFile struct {
	SchemaVersion int                `json:"schemaVersion"`
	Species       []speciesEntryJSON `json:"species"`
	Moves         []moveEntryJSON    `json:"moves"`
	Items         []itemEntryJSON    `json:"items"`
	Abilities     []abilityEntryJSON `json:"abilities"`
	Natures       []natureEntryJSON  `json:"natures"`
}

type statsEntryJSON struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

func (s statsEntryJSON) toEngine() engine.Stats {
	return engine.Stats{HP: s.HP, Atk: s.Atk, Def: s.Def, SpA: s.SpA, SpD: s.SpD, Spe: s.Spe}
}

type speciesEntryJSON struct {
	Key       string         `json:"key"`
	DexNo     int            `json:"dexNo"`
	Form      int            `json:"form"`
	NameJa    string         `json:"nameJa"`
	Types     []string       `json:"types"`
	BaseStats statsEntryJSON `json:"baseStats"`
	Abilities []string       `json:"abilities"`
}

type moveEntryJSON struct {
	ID       string `json:"id"`
	NameJa   string `json:"nameJa"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Power    int    `json:"power"`
	Priority int    `json:"priority"`
}

type itemEffectJSON struct {
	StatMods           map[string]int `json:"statMods"`
	DamageMod          int            `json:"damageMod"`
	PowerMod           int            `json:"powerMod"`
	PowerCategory      string         `json:"powerCategory"`
	OnlySuperEffective bool           `json:"onlySuperEffective"`
	BoostType          string         `json:"boostType"`
	BoostTypeMod       int            `json:"boostTypeMod"`
	ResistBerryType    string         `json:"resistBerryType"`
}

func (e itemEffectJSON) toEngine() (*engine.ItemEffect, error) {
	out := &engine.ItemEffect{
		DamageMod: e.DamageMod, PowerMod: e.PowerMod, OnlySuperEffective: e.OnlySuperEffective,
		BoostTypeMod: e.BoostTypeMod,
	}
	if e.StatMods != nil {
		out.StatMods = make(map[engine.StatKey]int, len(e.StatMods))
		for k, v := range e.StatMods {
			sk, ok := validStatKey(k)
			if !ok {
				return nil, fmt.Errorf("%w: 持ち物効果の statMods に未知のステータスキー %q", ErrInvalidSnapshot, k)
			}
			out.StatMods[sk] = v
		}
	}
	var err error
	if out.PowerCategory, err = parseOptionalCategory(e.PowerCategory); err != nil {
		return nil, err
	}
	if out.BoostType, err = parseOptionalType(e.BoostType); err != nil {
		return nil, err
	}
	if out.ResistBerryType, err = parseOptionalType(e.ResistBerryType); err != nil {
		return nil, err
	}
	return out, nil
}

type itemEntryJSON struct {
	ID     string          `json:"id"`
	NameJa string          `json:"nameJa"`
	Effect *itemEffectJSON `json:"effect"`
}

type abilityEffectJSON struct {
	StabMod              int            `json:"stabMod"`
	OffBoostType         string         `json:"offBoostType"`
	OffBoostTypeMod      int            `json:"offBoostTypeMod"`
	DefResistType        map[string]int `json:"defResistType"`
	ReduceSuperEffective int            `json:"reduceSuperEffective"`
	IgnoresBurn          bool           `json:"ignoresBurn"`
}

func (e abilityEffectJSON) toEngine() (*engine.AbilityEffect, error) {
	out := &engine.AbilityEffect{
		StabMod: e.StabMod, OffBoostTypeMod: e.OffBoostTypeMod,
		ReduceSuperEffective: e.ReduceSuperEffective, IgnoresBurn: e.IgnoresBurn,
	}
	var err error
	if out.OffBoostType, err = parseOptionalType(e.OffBoostType); err != nil {
		return nil, err
	}
	if e.DefResistType != nil {
		out.DefResistType = make(map[engine.Type]int, len(e.DefResistType))
		for k, v := range e.DefResistType {
			t, ok := validType(k)
			if !ok {
				return nil, fmt.Errorf("%w: 特性効果の defResistType に未知のタイプ %q", ErrInvalidSnapshot, k)
			}
			out.DefResistType[t] = v
		}
	}
	return out, nil
}

type abilityEntryJSON struct {
	ID     string             `json:"id"`
	NameJa string             `json:"nameJa"`
	Effect *abilityEffectJSON `json:"effect"`
}

type natureEntryJSON struct {
	ID     string  `json:"id"`
	NameJa string  `json:"nameJa"`
	Plus   *string `json:"plus"`
	Minus  *string `json:"minus"`
}

func parseNatureStat(v *string) (engine.StatKey, error) {
	if v == nil {
		return "", nil
	}
	sk, ok := validStatKey(*v)
	if !ok {
		return "", fmt.Errorf("%w: 性格補正に未知のステータスキー %q", ErrInvalidSnapshot, *v)
	}
	return sk, nil
}

// LoadSnapshot はスナップショットを読んで検証する。
// 不正はすべて ErrInvalidSnapshot で包んで返す(部分的なスナップショットは返さない)。
func LoadSnapshot(r io.Reader) (*Snapshot, error) {
	var file snapshotFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSnapshot, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: JSON の後ろに余計なデータがある", ErrInvalidSnapshot)
	}
	if file.SchemaVersion != 1 {
		return nil, fmt.Errorf("%w: schemaVersion は 1 でなければならない: %d", ErrInvalidSnapshot, file.SchemaVersion)
	}

	species, err := convertSpecies(file.Species)
	if err != nil {
		return nil, err
	}
	moves, err := convertMoves(file.Moves)
	if err != nil {
		return nil, err
	}
	items, err := convertItems(file.Items)
	if err != nil {
		return nil, err
	}
	abilities, err := convertAbilities(file.Abilities)
	if err != nil {
		return nil, err
	}
	natures, err := convertNatures(file.Natures)
	if err != nil {
		return nil, err
	}
	return &Snapshot{species: species, moves: moves, items: items, abilities: abilities, natures: natures}, nil
}

func convertSpecies(entries []speciesEntryJSON) (map[string]engine.Species, error) {
	out := make(map[string]engine.Species, len(entries))
	for _, e := range entries {
		if e.Key == "" {
			return nil, fmt.Errorf("%w: 種族キーが空", ErrInvalidSnapshot)
		}
		if _, dup := out[e.Key]; dup {
			return nil, fmt.Errorf("%w: 種族キーが重複している: %q", ErrInvalidSnapshot, e.Key)
		}
		if len(e.Types) < 1 || len(e.Types) > 2 {
			return nil, fmt.Errorf("%w: 種族 %q のタイプ数は1〜2個: %d", ErrInvalidSnapshot, e.Key, len(e.Types))
		}
		types := make([]engine.Type, 0, len(e.Types))
		for _, t := range e.Types {
			et, ok := parseRequiredType(t)
			if !ok {
				return nil, fmt.Errorf("%w: 種族 %q に未知のタイプ %q", ErrInvalidSnapshot, e.Key, t)
			}
			types = append(types, et)
		}
		out[e.Key] = engine.Species{
			Key: e.Key, DexNo: e.DexNo, Form: e.Form, NameJa: e.NameJa,
			Types: types, BaseStats: e.BaseStats.toEngine(), Abilities: append([]string(nil), e.Abilities...),
		}
	}
	return out, nil
}

func convertMoves(entries []moveEntryJSON) (map[string]engine.Move, error) {
	out := make(map[string]engine.Move, len(entries))
	for _, e := range entries {
		if e.ID == "" {
			return nil, fmt.Errorf("%w: 技IDが空", ErrInvalidSnapshot)
		}
		if _, dup := out[e.ID]; dup {
			return nil, fmt.Errorf("%w: 技IDが重複している: %q", ErrInvalidSnapshot, e.ID)
		}
		typ, ok := parseRequiredType(e.Type)
		if !ok {
			return nil, fmt.Errorf("%w: 技 %q に未知のタイプ %q", ErrInvalidSnapshot, e.ID, e.Type)
		}
		cat, ok := validCategory(e.Category)
		if !ok {
			return nil, fmt.Errorf("%w: 技 %q に未知の分類 %q", ErrInvalidSnapshot, e.ID, e.Category)
		}
		out[e.ID] = engine.Move{ID: e.ID, NameJa: e.NameJa, Type: typ, Category: cat, Power: e.Power, Priority: e.Priority}
	}
	return out, nil
}

func convertItems(entries []itemEntryJSON) (map[string]engine.Item, error) {
	out := make(map[string]engine.Item, len(entries))
	for _, e := range entries {
		if e.ID == "" {
			return nil, fmt.Errorf("%w: 持ち物IDが空", ErrInvalidSnapshot)
		}
		if _, dup := out[e.ID]; dup {
			return nil, fmt.Errorf("%w: 持ち物IDが重複している: %q", ErrInvalidSnapshot, e.ID)
		}
		it := engine.Item{ID: e.ID, NameJa: e.NameJa}
		if e.Effect != nil {
			eff, err := e.Effect.toEngine()
			if err != nil {
				return nil, err
			}
			it.Effect = eff
		}
		out[e.ID] = it
	}
	return out, nil
}

func convertAbilities(entries []abilityEntryJSON) (map[string]engine.Ability, error) {
	out := make(map[string]engine.Ability, len(entries))
	for _, e := range entries {
		if e.ID == "" {
			return nil, fmt.Errorf("%w: 特性IDが空", ErrInvalidSnapshot)
		}
		if _, dup := out[e.ID]; dup {
			return nil, fmt.Errorf("%w: 特性IDが重複している: %q", ErrInvalidSnapshot, e.ID)
		}
		ab := engine.Ability{ID: e.ID, NameJa: e.NameJa}
		if e.Effect != nil {
			eff, err := e.Effect.toEngine()
			if err != nil {
				return nil, err
			}
			ab.Effect = eff
		}
		out[e.ID] = ab
	}
	return out, nil
}

func convertNatures(entries []natureEntryJSON) (map[string]engine.Nature, error) {
	out := make(map[string]engine.Nature, len(entries))
	for _, e := range entries {
		if e.ID == "" {
			return nil, fmt.Errorf("%w: 性格IDが空", ErrInvalidSnapshot)
		}
		if _, dup := out[e.ID]; dup {
			return nil, fmt.Errorf("%w: 性格IDが重複している: %q", ErrInvalidSnapshot, e.ID)
		}
		plus, err := parseNatureStat(e.Plus)
		if err != nil {
			return nil, err
		}
		minus, err := parseNatureStat(e.Minus)
		if err != nil {
			return nil, err
		}
		if plus == engine.StatHP || minus == engine.StatHP {
			return nil, fmt.Errorf("%w: 性格 %q の補正は HP を指せない", ErrInvalidSnapshot, e.ID)
		}
		out[e.ID] = engine.Nature{Plus: plus, Minus: minus}
	}
	return out, nil
}

// --- タイプ相性表の JSON 形(testdata/golden/typechart.json と同じ schema) ----

type typeChartFile struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Source        string                    `json:"source"`
	Version       string                    `json:"version"`
	Generation    int                       `json:"generation"`
	Note          string                    `json:"note"`
	ExcludedTypes []string                  `json:"excludedTypes"`
	Types         []string                  `json:"types"`
	Effectiveness map[string]map[string]int `json:"effectiveness"`
}

// LoadTypeChart は testdata/golden/typechart.json と同じ schema(schemaVersion 1)の相性表を読み、
// engine.NewTypeChart で検証済みの表にする。不正はすべて ErrInvalidTypeChart で包む。
func LoadTypeChart(r io.Reader) (engine.TypeChart, error) {
	var file typeChartFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return engine.TypeChart{}, fmt.Errorf("%w: %v", ErrInvalidTypeChart, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return engine.TypeChart{}, fmt.Errorf("%w: JSON の後ろに余計なデータがある", ErrInvalidTypeChart)
	}
	if file.SchemaVersion != 1 {
		return engine.TypeChart{}, fmt.Errorf("%w: schemaVersion は 1 でなければならない: %d", ErrInvalidTypeChart, file.SchemaVersion)
	}
	if len(file.Types) == 0 {
		return engine.TypeChart{}, fmt.Errorf("%w: types が空", ErrInvalidTypeChart)
	}

	seen := make(map[string]bool, len(file.Types))
	types := make([]engine.Type, 0, len(file.Types))
	for _, t := range file.Types {
		et, ok := validType(t)
		if !ok {
			return engine.TypeChart{}, fmt.Errorf("%w: 未知のタイプ %q", ErrInvalidTypeChart, t)
		}
		if seen[t] {
			return engine.TypeChart{}, fmt.Errorf("%w: タイプが重複している: %q", ErrInvalidTypeChart, t)
		}
		seen[t] = true
		types = append(types, et)
	}
	if len(file.Effectiveness) == 0 {
		return engine.TypeChart{}, fmt.Errorf("%w: effectiveness が空", ErrInvalidTypeChart)
	}

	effectiveness := make(map[engine.Type]map[engine.Type]int, len(file.Effectiveness))
	for atk, row := range file.Effectiveness {
		et, ok := validType(atk)
		if !ok {
			return engine.TypeChart{}, fmt.Errorf("%w: 未知の攻撃タイプ %q", ErrInvalidTypeChart, atk)
		}
		if row == nil {
			return engine.TypeChart{}, fmt.Errorf("%w: %q の行が null", ErrInvalidTypeChart, atk)
		}
		outRow := make(map[engine.Type]int, len(row))
		for def, code := range row {
			edt, ok := validType(def)
			if !ok {
				return engine.TypeChart{}, fmt.Errorf("%w: 未知の防御タイプ %q", ErrInvalidTypeChart, def)
			}
			outRow[edt] = code
		}
		effectiveness[et] = outRow
	}

	chart, err := engine.NewTypeChart(engine.TypeChartData{Types: types, Effectiveness: effectiveness})
	if err != nil {
		return engine.TypeChart{}, fmt.Errorf("%w: %v", ErrInvalidTypeChart, err)
	}
	return chart, nil
}

// MemoryStore はスナップショットと相性表をメモリに持つ Store。
type MemoryStore struct {
	species   map[string]engine.Species
	moves     map[string]engine.Move
	items     map[string]engine.Item
	abilities map[string]engine.Ability
	natures   map[string]engine.Nature
	chart     engine.TypeChart
}

var _ Store = (*MemoryStore)(nil)

// New はスナップショットと相性表から Store を作る。
// 相性表がゼロ値なら ErrInvalidTypeChart、スナップショットの種族・技・持ち物・特性に
// 相性表に無いタイプが現れたら engine.ErrUnknownType で包んだエラーを返す
// (計算時ではなく起動時に気づくため)。
func New(snapshot *Snapshot, chart engine.TypeChart) (*MemoryStore, error) {
	if chart.IsZero() {
		return nil, fmt.Errorf("%w: 相性表が未設定", ErrInvalidTypeChart)
	}
	for key, sp := range snapshot.species {
		for _, t := range sp.Types {
			if !chart.Has(t) {
				return nil, fmt.Errorf("%w: 種族 %q のタイプ %q が相性表に無い", engine.ErrUnknownType, key, t)
			}
		}
	}
	for id, mv := range snapshot.moves {
		if mv.Type != engine.TypeNone && !chart.Has(mv.Type) {
			return nil, fmt.Errorf("%w: 技 %q のタイプ %q が相性表に無い", engine.ErrUnknownType, id, mv.Type)
		}
	}
	for id, it := range snapshot.items {
		if it.Effect == nil {
			continue
		}
		if t := it.Effect.BoostType; t != engine.TypeNone && !chart.Has(t) {
			return nil, fmt.Errorf("%w: 持ち物 %q の boostType %q が相性表に無い", engine.ErrUnknownType, id, t)
		}
		if t := it.Effect.ResistBerryType; t != engine.TypeNone && !chart.Has(t) {
			return nil, fmt.Errorf("%w: 持ち物 %q の resistBerryType %q が相性表に無い", engine.ErrUnknownType, id, t)
		}
	}
	for id, ab := range snapshot.abilities {
		if ab.Effect == nil {
			continue
		}
		if t := ab.Effect.OffBoostType; t != engine.TypeNone && !chart.Has(t) {
			return nil, fmt.Errorf("%w: 特性 %q の offBoostType %q が相性表に無い", engine.ErrUnknownType, id, t)
		}
		for t := range ab.Effect.DefResistType {
			if !chart.Has(t) {
				return nil, fmt.Errorf("%w: 特性 %q の defResistType %q が相性表に無い", engine.ErrUnknownType, id, t)
			}
		}
	}
	return &MemoryStore{
		species: snapshot.species, moves: snapshot.moves, items: snapshot.items,
		abilities: snapshot.abilities, natures: snapshot.natures, chart: chart,
	}, nil
}

// Species は Store を実装する。返すスライスはコピーで、書き換えても Store に影響しない。
func (s *MemoryStore) Species(key string) (engine.Species, bool) {
	sp, ok := s.species[key]
	if !ok {
		return engine.Species{}, false
	}
	sp.Types = append([]engine.Type(nil), sp.Types...)
	sp.Abilities = append([]string(nil), sp.Abilities...)
	return sp, true
}

// Move は Store を実装する。
func (s *MemoryStore) Move(id string) (engine.Move, bool) {
	mv, ok := s.moves[id]
	return mv, ok
}

// Item は Store を実装する。Effect(内部の map を含む)はコピーを返す(critic 指摘 O2:
// 呼び出し側が書き換えても Store に影響しない、という doc の約束を Effect にも適用する)。
func (s *MemoryStore) Item(id string) (engine.Item, bool) {
	it, ok := s.items[id]
	if !ok {
		return engine.Item{}, false
	}
	it.Effect = copyItemEffect(it.Effect)
	return it, true
}

// Ability は Store を実装する。Effect(内部の map を含む)はコピーを返す(O2)。
func (s *MemoryStore) Ability(id string) (engine.Ability, bool) {
	ab, ok := s.abilities[id]
	if !ok {
		return engine.Ability{}, false
	}
	ab.Effect = copyAbilityEffect(ab.Effect)
	return ab, true
}

// copyItemEffect は *engine.ItemEffect のディープコピーを返す(nil は nil のまま)。
func copyItemEffect(e *engine.ItemEffect) *engine.ItemEffect {
	if e == nil {
		return nil
	}
	out := *e
	if e.StatMods != nil {
		out.StatMods = make(map[engine.StatKey]int, len(e.StatMods))
		for k, v := range e.StatMods {
			out.StatMods[k] = v
		}
	}
	return &out
}

// copyAbilityEffect は *engine.AbilityEffect のディープコピーを返す(nil は nil のまま)。
func copyAbilityEffect(e *engine.AbilityEffect) *engine.AbilityEffect {
	if e == nil {
		return nil
	}
	out := *e
	if e.DefResistType != nil {
		out.DefResistType = make(map[engine.Type]int, len(e.DefResistType))
		for k, v := range e.DefResistType {
			out.DefResistType[k] = v
		}
	}
	return &out
}

// Nature は Store を実装する。
func (s *MemoryStore) Nature(id string) (engine.Nature, bool) {
	n, ok := s.natures[id]
	return n, ok
}

// NatureID は Store を実装する(ADR-0016 の写像規則)。
func (s *MemoryStore) NatureID(n engine.Nature) (string, bool) {
	ids := make([]string, 0, len(s.natures))
	for id := range s.natures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m := s.natures[id]
		if (n.IsNeutral() && m.IsNeutral()) || m == n {
			return id, true
		}
	}
	return "", false
}

// TypeChart は Store を実装する。
func (s *MemoryStore) TypeChart() engine.TypeChart { return s.chart }
