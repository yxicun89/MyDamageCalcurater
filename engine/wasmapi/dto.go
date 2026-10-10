package wasmapi

// 境界の DTO と engine 型への変換(ADR-0011 §3, §4)。
//
// engine の型には json タグを足さず、ここで独立に定義する。フィールド名は lowerCamelCase。
// 変換は素直な写しと列挙の検証だけで、計算・丸めは持たない。

import (
	"fmt"
	"slices"
	"sort"
	"strconv"

	"example.com/pokecalc/engine"
)

// --- 列挙(engine の定数と同じ文字列)---------------------------------------

var validTypes = map[engine.Type]bool{
	engine.TypeNormal: true, engine.TypeFire: true, engine.TypeWater: true, engine.TypeElectric: true,
	engine.TypeGrass: true, engine.TypeIce: true, engine.TypeFighting: true, engine.TypePoison: true,
	engine.TypeGround: true, engine.TypeFlying: true, engine.TypePsychic: true, engine.TypeBug: true,
	engine.TypeRock: true, engine.TypeGhost: true, engine.TypeDragon: true, engine.TypeDark: true,
	engine.TypeSteel: true, engine.TypeFairy: true,
}

var validStatKeys = map[engine.StatKey]bool{
	engine.StatHP: true, engine.StatAtk: true, engine.StatDef: true,
	engine.StatSpA: true, engine.StatSpD: true, engine.StatSpe: true,
}

var validCategories = map[engine.MoveCategory]bool{
	engine.CategoryPhysical: true, engine.CategorySpecial: true, engine.CategoryStatus: true,
}

var validFormats = map[engine.Format]bool{engine.FormatSingle: true, engine.FormatDouble: true}

var validWeathers = map[engine.Weather]bool{
	engine.WeatherNone: true, engine.WeatherSun: true, engine.WeatherRain: true,
	engine.WeatherSand: true, engine.WeatherSnow: true,
}

var validTerrains = map[engine.Terrain]bool{
	engine.TerrainNone: true, engine.TerrainElectric: true, engine.TerrainGrassy: true,
	engine.TerrainMisty: true, engine.TerrainPsychic: true,
}

var validStatuses = map[engine.Status]bool{
	engine.StatusNone: true, engine.StatusBurn: true, engine.StatusParalysis: true,
	engine.StatusPoison: true, engine.StatusBadlyPoison: true, engine.StatusSleep: true,
	engine.StatusFreeze: true,
}

func enumError(path, v string) error {
	return fail(CodeInvalidEnum, "%s に未知の値 %q", path, v)
}

// parseType はタイプを検証する。allowNone が true のとき "" (タイプなし)を許す。
func parseType(path, v string, allowNone bool) (engine.Type, error) {
	t := engine.Type(v)
	if v == "" && allowNone {
		return engine.TypeNone, nil
	}
	if !validTypes[t] {
		return "", enumError(path, v)
	}
	return t, nil
}

// parseStatKey はステータスキーを検証する。allowNone が true のとき "" (補正なし)を許す。
func parseStatKey(path, v string, allowNone bool) (engine.StatKey, error) {
	k := engine.StatKey(v)
	if v == "" && allowNone {
		return "", nil
	}
	if !validStatKeys[k] {
		return "", enumError(path, v)
	}
	return k, nil
}

// parseCategory は技の分類を検証する。allowNone が true のとき "" (全分類)を許す。
func parseCategory(path, v string, allowNone bool) (engine.MoveCategory, error) {
	c := engine.MoveCategory(v)
	if v == "" && allowNone {
		return "", nil
	}
	if !validCategories[c] {
		return "", enumError(path, v)
	}
	return c, nil
}

// --- 共通の DTO -------------------------------------------------------------

type statsDTO struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

func (s statsDTO) toEngine() engine.Stats {
	return engine.Stats{HP: s.HP, Atk: s.Atk, Def: s.Def, SpA: s.SpA, SpD: s.SpD, Spe: s.Spe}
}

func statsFrom(s engine.Stats) statsDTO {
	return statsDTO{HP: s.HP, Atk: s.Atk, Def: s.Def, SpA: s.SpA, SpD: s.SpD, Spe: s.Spe}
}

type natureDTO struct {
	Plus  string `json:"plus"`
	Minus string `json:"minus"`
}

// toEngine は性格を検証して変換する。HP は列挙としては有効で、
// 「HP に補正は掛けられない」は engine の Validate(invalid_input)が判定する。
func (n natureDTO) toEngine(path string) (engine.Nature, error) {
	plus, err := parseStatKey(path+".plus", n.Plus, true)
	if err != nil {
		return engine.Nature{}, err
	}
	minus, err := parseStatKey(path+".minus", n.Minus, true)
	if err != nil {
		return engine.Nature{}, err
	}
	return engine.Nature{Plus: plus, Minus: minus}, nil
}

func natureFrom(n engine.Nature) natureDTO {
	return natureDTO{Plus: string(n.Plus), Minus: string(n.Minus)}
}

type ranksDTO struct {
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

func (r ranksDTO) toEngine() engine.Ranks {
	return engine.Ranks{Atk: r.Atk, Def: r.Def, SpA: r.SpA, SpD: r.SpD, Spe: r.Spe}
}

type screensDTO struct {
	Reflect     bool `json:"reflect"`
	LightScreen bool `json:"lightScreen"`
	AuroraVeil  bool `json:"auroraVeil"`
}

func (s screensDTO) toEngine() engine.Screens {
	return engine.Screens{Reflect: s.Reflect, LightScreen: s.LightScreen, AuroraVeil: s.AuroraVeil}
}

// typeChartDTO はリクエストに載せるタイプ相性表(ADR-0011 §13 / ADR-0013)。
// 倍率は整数コード(0=無効 / 1=いまひとつ / 2=等倍 / 4=抜群)のまま渡し、等倍は省略できる。
// 境界は表を解釈せず、綴りの検証だけをして engine.NewTypeChart に渡す。
type typeChartDTO struct {
	Types         []string                  `json:"types"`
	Effectiveness map[string]map[string]int `json:"effectiveness"`
}

// toEngine は綴りを検証して engine の検証済みの表にする。
// 省略(null・空オブジェクト)は engine.ErrTypeChartMissing、定義の不正は engine.ErrInvalidTypeChart。
// 既定の表は補わない。
func (c typeChartDTO) toEngine(path string) (engine.TypeChart, error) {
	if len(c.Types) == 0 && len(c.Effectiveness) == 0 {
		return engine.TypeChart{}, fmt.Errorf("%w: リクエストに %s が無い", engine.ErrTypeChartMissing, path)
	}
	types := make([]engine.Type, 0, len(c.Types))
	for i, v := range c.Types {
		t, err := parseType(fmt.Sprintf("%s.types[%d]", path, i), v, false)
		if err != nil {
			return engine.TypeChart{}, err
		}
		types = append(types, t)
	}
	effectiveness, err := c.effectivenessToEngine(path + ".effectiveness")
	if err != nil {
		return engine.TypeChart{}, err
	}
	return engine.NewTypeChart(engine.TypeChartData{Types: types, Effectiveness: effectiveness})
}

// effectivenessToEngine は effectiveness の攻撃側・防御側のキーを検証して engine の形にする。
func (c typeChartDTO) effectivenessToEngine(path string) (map[engine.Type]map[engine.Type]int, error) {
	if c.Effectiveness == nil {
		return nil, nil
	}
	out := make(map[engine.Type]map[engine.Type]int, len(c.Effectiveness))
	// キーを整列して走査する(複数の不正があっても報告が毎回同じになるように)。
	for _, atkKey := range sortedKeys(c.Effectiveness) {
		atk, err := parseType(path, atkKey, false)
		if err != nil {
			return nil, err
		}
		row := c.Effectiveness[atkKey]
		outRow := make(map[engine.Type]int, len(row))
		for _, defKey := range sortedKeys(row) {
			def, err := parseType(path+"."+atkKey, defKey, false)
			if err != nil {
				return nil, err
			}
			outRow[def] = row[defKey]
		}
		out[atk] = outRow
	}
	return out, nil
}

type speciesDTO struct {
	Key       string   `json:"key"`
	DexNo     int      `json:"dexNo"`
	Form      int      `json:"form"`
	NameJa    string   `json:"nameJa"`
	Types     []string `json:"types"`
	BaseStats statsDTO `json:"baseStats"`
	Abilities []string `json:"abilities"`
	// IsMega・RequiredItemID はメガシンカ後の種族の印と必須の持ち物(issue #505。ADR-0321)。
	// engine.Species には渡さず、境界の持ち物検証(mega.go)だけが使う。省略は通常の種族。
	IsMega         bool   `json:"isMega"`
	RequiredItemID string `json:"requiredItemId"`
	// WeightHg は種族の重さ(hg。ADR-0143)。省略・0 は不明(重さで威力が決まる技に未対応の印が付く)。
	WeightHg int `json:"weightHg"`
}

func (s speciesDTO) toEngine(path string) (engine.Species, error) {
	var types []engine.Type
	if s.Types != nil {
		types = make([]engine.Type, 0, len(s.Types))
	}
	for i, v := range s.Types {
		t, err := parseType(fmt.Sprintf("%s.types[%d]", path, i), v, false)
		if err != nil {
			return engine.Species{}, err
		}
		types = append(types, t)
	}
	return engine.Species{
		Key: s.Key, DexNo: s.DexNo, Form: s.Form, NameJa: s.NameJa,
		Types: types, BaseStats: s.BaseStats.toEngine(), Abilities: s.Abilities, WeightHg: s.WeightHg,
	}, nil
}

type moveDTO struct {
	ID       string `json:"id"`
	NameJa   string `json:"nameJa"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Power    int    `json:"power"`
	Priority int    `json:"priority"`
	// Mechanisms は技の機構(ADR-0121。省略・空は通常の技)。未対応の印に使う(ADR-0123)。
	Mechanisms []string `json:"mechanisms"`
	// MechanismParams は機構の中身(ADR-0142 §8)。省略・null は中身なし。
	MechanismParams *mechanismParamsDTO `json:"mechanismParams"`
	// Target は技の対象("" | single | spread。ADR-0222)。省略は不明。
	Target string `json:"target"`
	// Flags は技のフラグ(ADR-0178)。キーが無い・null は不明(FlagsKnown 偽)、配列は既知(空配列 = フラグなし)。
	Flags *[]string `json:"flags"`
	// Rule は技の処理の定義(ADR-0143)。省略・null は定義なし。キーは camelCase(PascalCase も受ける)。
	Rule *moveRuleDTO `json:"rule"`
}

func (m moveDTO) toEngine(path string) (engine.Move, error) {
	typ, err := parseType(path+".type", m.Type, true)
	if err != nil {
		return engine.Move{}, err
	}
	cat, err := parseCategory(path+".category", m.Category, false)
	if err != nil {
		return engine.Move{}, err
	}
	mechanisms, err := parseMechanisms(path+".mechanisms", m.Mechanisms)
	if err != nil {
		return engine.Move{}, err
	}
	target := engine.MoveTarget(m.Target)
	switch target {
	case "", engine.MoveTargetSingle, engine.MoveTargetSpread:
	default:
		return engine.Move{}, enumError(path+".target", m.Target)
	}
	params, err := m.MechanismParams.toEngine(path + ".mechanismParams")
	if err != nil {
		return engine.Move{}, err
	}
	out := engine.Move{ID: m.ID, NameJa: m.NameJa, Type: typ, Category: cat, Power: m.Power, Priority: m.Priority,
		Mechanisms: mechanisms, Params: params, Target: target}
	if out.Rule, err = m.Rule.toEngine(path + ".rule"); err != nil {
		return engine.Move{}, err
	}
	if m.Flags != nil {
		if out.Flags, err = parseMoveFlags(path+".flags", *m.Flags); err != nil {
			return engine.Move{}, err
		}
		out.FlagsKnown = true
	}
	return out, nil
}

// parseMoveFlags は技のフラグを検証する(未知の値は invalid_enum、重複は invalid_input。ADR-0178)。
func parseMoveFlags(path string, vs []string) ([]engine.MoveFlag, error) {
	if len(vs) == 0 {
		return nil, nil
	}
	out := make([]engine.MoveFlag, 0, len(vs))
	for i, v := range vs {
		f := engine.MoveFlag(v)
		if !f.Known() {
			return nil, enumError(fmt.Sprintf("%s[%d]", path, i), v)
		}
		if slices.Contains(out, f) {
			return nil, fail(CodeInvalidInput, "%s にフラグ %q が重複している", path, v)
		}
		out = append(out, f)
	}
	return out, nil
}

// mechanismParamsDTO は技の機構の中身(ADR-0142 §8)。能力値・ポケモン・タイプが語彙に無ければ invalid_enum。
// 値域・機構との対応は engine の CalcDamage が検証する(invalid_input)。
type mechanismParamsDTO struct {
	MultiHit       *multiHitDTO    `json:"multiHit"`
	FixedDamage    *fixedDamageDTO `json:"fixedDamage"`
	Ohko           *ohkoDTO        `json:"ohko"`
	OffenseStat    string          `json:"offenseStat"`
	OffensePokemon string          `json:"offensePokemon"`
	DefenseStat    string          `json:"defenseStat"`
}

type multiHitDTO struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type fixedDamageDTO struct {
	Level bool `json:"level"`
	Value int  `json:"value"`
}

type ohkoDTO struct {
	ImmuneType string `json:"immuneType"`
}

// parseUsedStatKey は攻撃・防御に使う能力値を検証する(HP は不可)。allowNone のとき "" を許す。
func parseUsedStatKey(path, v string) (engine.StatKey, error) {
	k, err := parseStatKey(path, v, true)
	if err != nil {
		return "", err
	}
	if k == engine.StatHP {
		return "", enumError(path, v)
	}
	return k, nil
}

func (p *mechanismParamsDTO) toEngine(path string) (engine.MechanismParams, error) {
	if p == nil {
		return engine.MechanismParams{}, nil
	}
	var out engine.MechanismParams
	var err error
	if p.MultiHit != nil {
		out.MultiHit = &engine.MultiHit{Min: p.MultiHit.Min, Max: p.MultiHit.Max}
	}
	if p.FixedDamage != nil {
		out.FixedDamage = &engine.FixedDamage{Level: p.FixedDamage.Level, Value: p.FixedDamage.Value}
	}
	if p.Ohko != nil {
		immune, err := parseType(path+".ohko.immuneType", p.Ohko.ImmuneType, true)
		if err != nil {
			return engine.MechanismParams{}, err
		}
		out.OHKO = &engine.OHKO{ImmuneType: immune}
	}
	if out.OffenseStat, err = parseUsedStatKey(path+".offenseStat", p.OffenseStat); err != nil {
		return engine.MechanismParams{}, err
	}
	switch engine.OffensePokemon(p.OffensePokemon) {
	case "", engine.OffensePokemonAttacker, engine.OffensePokemonDefender:
		out.OffensePokemon = engine.OffensePokemon(p.OffensePokemon)
	default:
		return engine.MechanismParams{}, enumError(path+".offensePokemon", p.OffensePokemon)
	}
	if out.DefenseStat, err = parseUsedStatKey(path+".defenseStat", p.DefenseStat); err != nil {
		return engine.MechanismParams{}, err
	}
	return out, nil
}

// parseMechanisms は技の機構を検証する(未知の値は invalid_enum、重複は invalid_input)。
// 並びは engine が印を付けるときに整えるので、ここでは変えない。
func parseMechanisms(path string, vs []string) ([]engine.MoveMechanism, error) {
	if len(vs) == 0 {
		return nil, nil
	}
	out := make([]engine.MoveMechanism, 0, len(vs))
	for i, v := range vs {
		m := engine.MoveMechanism(v)
		if !m.Known() {
			return nil, enumError(fmt.Sprintf("%s[%d]", path, i), v)
		}
		if slices.Contains(out, m) {
			return nil, fail(CodeInvalidInput, "%s に機構 %q が重複している", path, v)
		}
		out = append(out, m)
	}
	return out, nil
}

// speedModDTO は素早さの補正の1行(ADR-0139)。語彙に無い条件は invalid_enum。
// modifier の範囲は検証しない: ダメージ計算が読まない値で、共通マスタが取り込み時に検証済みのため。
// Web は要素のキーを PascalCase のまま渡すが、encoding/json の照合は大文字小文字を区別しないので受け付ける(テストで固定)。
type speedModDTO struct {
	Condition string `json:"condition"`
	Modifier  int    `json:"modifier"`
}

func speedModsToEngine(path string, ds []speedModDTO) ([]engine.SpeedMod, error) {
	if ds == nil {
		return nil, nil
	}
	out := make([]engine.SpeedMod, 0, len(ds))
	for i, d := range ds {
		c := engine.SpeedCondition(d.Condition)
		if !c.Known() {
			return nil, enumError(fmt.Sprintf("%s[%d].condition", path, i), d.Condition)
		}
		out = append(out, engine.SpeedMod{Condition: c, Modifier: d.Modifier})
	}
	return out, nil
}

type itemEffectDTO struct {
	StatMods           map[string]int `json:"statMods"`
	DamageMod          int            `json:"damageMod"`
	PowerMod           int            `json:"powerMod"`
	PowerCategory      string         `json:"powerCategory"`
	OnlySuperEffective bool           `json:"onlySuperEffective"`
	BoostType          string         `json:"boostType"`
	BoostTypeMod       int            `json:"boostTypeMod"`
	ResistBerryType    string         `json:"resistBerryType"`
	SpeedMods          []speedModDTO  `json:"speedMods"` // 素早さの補正(ADR-0139)。ダメージ計算は読まない
	// UnsupportedAttacker / UnsupportedDefender は「未対応」の印(ADR-0123)。
	UnsupportedAttacker bool `json:"unsupportedAttacker"`
	UnsupportedDefender bool `json:"unsupportedDefender"`
	// Grounds は持ち物で接地する(くろいてっきゅう型。ADR-0144)。
	Grounds bool `json:"grounds"`
}

func (e itemEffectDTO) toEngine(path string) (*engine.ItemEffect, error) {
	out := &engine.ItemEffect{
		DamageMod: e.DamageMod, PowerMod: e.PowerMod, OnlySuperEffective: e.OnlySuperEffective,
		BoostTypeMod:        e.BoostTypeMod,
		UnsupportedAttacker: e.UnsupportedAttacker, UnsupportedDefender: e.UnsupportedDefender,
		Grounds: e.Grounds,
	}
	if e.StatMods != nil {
		out.StatMods = make(map[engine.StatKey]int, len(e.StatMods))
		// キーを整列して走査する(複数の不正があっても報告が毎回同じになるように)。
		for _, k := range sortedKeys(e.StatMods) {
			key, err := parseStatKey(path+".statMods", k, false)
			if err != nil {
				return nil, err
			}
			out.StatMods[key] = e.StatMods[k]
		}
	}
	var err error
	if out.PowerCategory, err = parseCategory(path+".powerCategory", e.PowerCategory, true); err != nil {
		return nil, err
	}
	if out.BoostType, err = parseType(path+".boostType", e.BoostType, true); err != nil {
		return nil, err
	}
	if out.ResistBerryType, err = parseType(path+".resistBerryType", e.ResistBerryType, true); err != nil {
		return nil, err
	}
	if out.SpeedMods, err = speedModsToEngine(path+".speedMods", e.SpeedMods); err != nil {
		return nil, err
	}
	return out, nil
}

type itemDTO struct {
	ID     string         `json:"id"`
	NameJa string         `json:"nameJa"`
	Effect *itemEffectDTO `json:"effect"`
	// IsMegaStone はメガストーンか(ADR-0143。防御側の持ち物を払い落とす技の補正に使う)。省略は偽。
	IsMegaStone bool `json:"isMegaStone"`
	// FlingPower はなげつけるの威力(ADR-0144)。省略は 0(不明)。負は個体の検証で invalid_input。
	FlingPower int `json:"flingPower"`
}

// itemToEngine は nil(持ち物なし)を nil のまま返す。
func itemToEngine(path string, d *itemDTO) (*engine.Item, error) {
	if d == nil {
		return nil, nil
	}
	item := &engine.Item{ID: d.ID, NameJa: d.NameJa, MegaStone: d.IsMegaStone, FlingPower: d.FlingPower}
	if d.Effect != nil {
		eff, err := d.Effect.toEngine(path + ".effect")
		if err != nil {
			return nil, err
		}
		item.Effect = eff
	}
	return item, nil
}

func itemsToEngine(path string, ds []*itemDTO) ([]*engine.Item, error) {
	if ds == nil {
		return nil, nil
	}
	out := make([]*engine.Item, 0, len(ds))
	for i, d := range ds {
		item, err := itemToEngine(fmt.Sprintf("%s[%d]", path, i), d)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// absorbEffectDTO は防御側が吸収したときの副次効果(ADR-0106 §決定2)。
// ゼロ値({})は「吸収するが副次効果は持たない」(もらいび等)を表す正しい値。
type absorbEffectDTO struct {
	HealNumerator   int    `json:"healNumerator"`
	HealDenominator int    `json:"healDenominator"`
	BoostStat       string `json:"boostStat"`
	BoostStages     int    `json:"boostStages"`
}

func (e absorbEffectDTO) toEngine(path string) (engine.AbsorbEffect, error) {
	var out engine.AbsorbEffect
	hasNum, hasDen := e.HealNumerator != 0, e.HealDenominator != 0
	if hasNum != hasDen {
		return engine.AbsorbEffect{}, fail(CodeInvalidInput,
			"%s: healNumerator と healDenominator は組で指定する", path)
	}
	if hasNum {
		if e.HealDenominator < 1 || e.HealDenominator > 16 {
			return engine.AbsorbEffect{}, fail(CodeInvalidInput, "%s.healDenominator は1..16の範囲", path)
		}
		if e.HealNumerator < 1 || e.HealNumerator > e.HealDenominator {
			return engine.AbsorbEffect{}, fail(CodeInvalidInput,
				"%s.healNumerator は1..healDenominatorの範囲", path)
		}
		out.HealNumerator, out.HealDenominator = e.HealNumerator, e.HealDenominator
	}
	hasStat, hasStages := e.BoostStat != "", e.BoostStages != 0
	if hasStat != hasStages {
		return engine.AbsorbEffect{}, fail(CodeInvalidInput,
			"%s: boostStat と boostStages は組で指定する", path)
	}
	if hasStat {
		if e.BoostStat == string(engine.StatHP) {
			return engine.AbsorbEffect{}, enumError(path+".boostStat", e.BoostStat)
		}
		stat, err := parseStatKey(path+".boostStat", e.BoostStat, false)
		if err != nil {
			return engine.AbsorbEffect{}, err
		}
		if e.BoostStages < 1 || e.BoostStages > 6 {
			return engine.AbsorbEffect{}, fail(CodeInvalidInput, "%s.boostStages は1..6の範囲", path)
		}
		out.BoostStat, out.BoostStages = stat, e.BoostStages
	}
	return out, nil
}

type abilityEffectDTO struct {
	StabMod                   int                        `json:"stabMod"`
	OffBoostType              string                     `json:"offBoostType"`
	OffBoostTypeMod           int                        `json:"offBoostTypeMod"`
	DefResistType             map[string]int             `json:"defResistType"`
	DefImmuneTypes            []string                   `json:"defImmuneTypes"`
	DefAbsorbTypes            map[string]absorbEffectDTO `json:"defAbsorbTypes"`
	ReduceSuperEffective      int                        `json:"reduceSuperEffective"`
	IgnoresBurn               bool                       `json:"ignoresBurn"`
	Airborne                  bool                       `json:"airborne"`                  // 浮いている(フィールドの補正の対象外。ADR-0116)
	SpeedMods                 []speedModDTO              `json:"speedMods"`                 // 素早さの補正(ADR-0139)
	IgnoresParalysisSpeedDrop bool                       `json:"ignoresParalysisSpeedDrop"` // まひの素早さ半減を受けない(ADR-0139)
	// 特性の段階1(ADR-0176)。入れ子(typeConvert・powerMods の要素)のキーは Web が PascalCase のまま渡す
	// (encoding/json は大小を区別せずに照合するので受け付ける)。
	TypeConvert            *typeConvertDTO          `json:"typeConvert"`
	PowerMods              []conditionalPowerModDTO `json:"powerMods"`
	AuraType               string                   `json:"auraType"`
	AuraMod                int                      `json:"auraMod"`
	StatMods               map[string]int           `json:"statMods"`
	SeparateStatMods       map[string]int           `json:"separateStatMods"`
	CritDamageMod          int                      `json:"critDamageMod"`
	PreventsCritical       bool                     `json:"preventsCritical"`
	IgnoresOpponentRanks   bool                     `json:"ignoresOpponentRanks"`
	IgnoresDefenderAbility bool                     `json:"ignoresDefenderAbility"`
	Breakable              bool                     `json:"breakable"`
	MaxMultiHit            bool                     `json:"maxMultiHit"`  // 範囲のある多段技が常に最大回数(ADR-0142 §3)
	PreventsOHKO           bool                     `json:"preventsOHKO"` // 一撃必殺技が効かない(ADR-0142 §4)
	// 特性の段階2(技のフラグ。ADR-0178)。入れ子(postAuraPowerMods の要素・flagTypeConvert)は PascalCase も受ける。
	PostAuraPowerMods   []conditionalPowerModDTO `json:"postAuraPowerMods"`
	FlagTypeConvert     *flagTypeConvertDTO      `json:"flagTypeConvert"`
	DefImmuneFlags      []string                 `json:"defImmuneFlags"`
	DefFinalModsByFlag  map[string]int           `json:"defFinalModsByFlag"`
	DefFinalModsByType  map[string]int           `json:"defFinalModsByType"`
	NoContact           bool                     `json:"noContact"`
	WeightMod           int                      `json:"weightMod"`           // 重さの補正(ADR-0143)
	UnsupportedAttacker bool                     `json:"unsupportedAttacker"` // 「未対応」の印(ADR-0123)
	UnsupportedDefender bool                     `json:"unsupportedDefender"`
}

// typeConvertDTO は技のタイプの変換(ADR-0176)。タイプが語彙に無ければ invalid_enum。
type typeConvertDTO struct {
	From     string `json:"from"`
	To       string `json:"to"`
	PowerMod int    `json:"powerMod"`
}

// flagTypeConvertDTO はフラグによる技のタイプの変換(ADR-0178)。フラグ・タイプが語彙に無ければ invalid_enum。
type flagTypeConvertDTO struct {
	Flag string `json:"flag"`
	To   string `json:"to"`
}

// conditionalPowerModDTO は条件つきの威力補正1つ(ADR-0176)。条件・タイプ・フラグが語彙に無ければ invalid_enum。
// 値域(条件ごとの項目の組・補正値の範囲)は engine の Individual.Validate が見る。
type conditionalPowerModDTO struct {
	Condition string `json:"condition"`
	MaxPower  int    `json:"maxPower"`
	MoveType  string `json:"moveType"`
	Flag      string `json:"flag"` // move_flag の条件のフラグ(ADR-0178)
	Modifier  int    `json:"modifier"`
}

// conditionalPowerModsToEngine は条件つきの威力補正の並び(powerMods・postAuraPowerMods)を engine の型にする。
func conditionalPowerModsToEngine(path string, ds []conditionalPowerModDTO) ([]engine.ConditionalPowerMod, error) {
	if ds == nil {
		return nil, nil
	}
	out := make([]engine.ConditionalPowerMod, 0, len(ds))
	for i, d := range ds {
		p := fmt.Sprintf("%s[%d]", path, i)
		c := engine.PowerCondition(d.Condition)
		if !c.Known() {
			return nil, enumError(p+".condition", d.Condition)
		}
		mt, err := parseType(p+".moveType", d.MoveType, true)
		if err != nil {
			return nil, err
		}
		f := engine.MoveFlag(d.Flag)
		if d.Flag != "" && !f.Known() {
			return nil, enumError(p+".flag", d.Flag)
		}
		out = append(out, engine.ConditionalPowerMod{
			Condition: c, MaxPower: d.MaxPower, MoveType: mt, Flag: f, Modifier: d.Modifier,
		})
	}
	return out, nil
}

// stage2ToEngine は特性の段階2の項目(ADR-0178)を out に写す。値域は engine の Individual.Validate が見る。
func (e abilityEffectDTO) stage2ToEngine(path string, out *engine.AbilityEffect) error {
	out.NoContact = e.NoContact
	var err error
	if out.PostAuraPowerMods, err = conditionalPowerModsToEngine(path+".postAuraPowerMods", e.PostAuraPowerMods); err != nil {
		return err
	}
	if c := e.FlagTypeConvert; c != nil {
		f := engine.MoveFlag(c.Flag)
		if !f.Known() {
			return enumError(path+".flagTypeConvert.flag", c.Flag)
		}
		to, err := parseType(path+".flagTypeConvert.to", c.To, false)
		if err != nil {
			return err
		}
		out.FlagTypeConvert = &engine.FlagTypeConvert{Flag: f, To: to}
	}
	if e.DefImmuneFlags != nil {
		out.DefImmuneFlags = make([]engine.MoveFlag, 0, len(e.DefImmuneFlags))
		for i, v := range e.DefImmuneFlags {
			f := engine.MoveFlag(v)
			if !f.Known() {
				return enumError(fmt.Sprintf("%s.defImmuneFlags[%d]", path, i), v)
			}
			out.DefImmuneFlags = append(out.DefImmuneFlags, f)
		}
	}
	if e.DefFinalModsByFlag != nil {
		out.DefFinalModsByFlag = make(map[engine.MoveFlag]int, len(e.DefFinalModsByFlag))
		for _, k := range sortedKeys(e.DefFinalModsByFlag) {
			f := engine.MoveFlag(k)
			if !f.Known() {
				return enumError(path+".defFinalModsByFlag", k)
			}
			out.DefFinalModsByFlag[f] = e.DefFinalModsByFlag[k]
		}
	}
	if e.DefFinalModsByType != nil {
		out.DefFinalModsByType = make(map[engine.Type]int, len(e.DefFinalModsByType))
		for _, k := range sortedKeys(e.DefFinalModsByType) {
			t, err := parseType(path+".defFinalModsByType", k, false)
			if err != nil {
				return err
			}
			out.DefFinalModsByType[t] = e.DefFinalModsByType[k]
		}
	}
	return nil
}

// abilityStatModsToEngine は特性の実数値の倍率のキーを検証して engine の型にする(未知のキーは invalid_enum)。
func abilityStatModsToEngine(path string, m map[string]int) (map[engine.StatKey]int, error) {
	if m == nil {
		return nil, nil
	}
	out := make(map[engine.StatKey]int, len(m))
	for _, k := range sortedKeys(m) {
		key, err := parseStatKey(path, k, false)
		if err != nil {
			return nil, err
		}
		out[key] = m[k]
	}
	return out, nil
}

// stage1ToEngine は特性の段階1の項目(ADR-0176)を out に写す。
func (e abilityEffectDTO) stage1ToEngine(path string, out *engine.AbilityEffect) error {
	out.AuraMod, out.CritDamageMod = e.AuraMod, e.CritDamageMod
	out.PreventsCritical, out.IgnoresOpponentRanks = e.PreventsCritical, e.IgnoresOpponentRanks
	out.IgnoresDefenderAbility, out.Breakable = e.IgnoresDefenderAbility, e.Breakable
	out.MaxMultiHit, out.PreventsOHKO = e.MaxMultiHit, e.PreventsOHKO
	var err error
	if c := e.TypeConvert; c != nil {
		tc := &engine.TypeConvert{PowerMod: c.PowerMod}
		if tc.From, err = parseType(path+".typeConvert.from", c.From, false); err != nil {
			return err
		}
		if tc.To, err = parseType(path+".typeConvert.to", c.To, false); err != nil {
			return err
		}
		out.TypeConvert = tc
	}
	if out.PowerMods, err = conditionalPowerModsToEngine(path+".powerMods", e.PowerMods); err != nil {
		return err
	}
	if out.AuraType, err = parseType(path+".auraType", e.AuraType, true); err != nil {
		return err
	}
	if out.StatMods, err = abilityStatModsToEngine(path+".statMods", e.StatMods); err != nil {
		return err
	}
	out.SeparateStatMods, err = abilityStatModsToEngine(path+".separateStatMods", e.SeparateStatMods)
	return err
}

func (e abilityEffectDTO) toEngine(path string) (*engine.AbilityEffect, error) {
	out := &engine.AbilityEffect{
		StabMod: e.StabMod, OffBoostTypeMod: e.OffBoostTypeMod,
		ReduceSuperEffective: e.ReduceSuperEffective, IgnoresBurn: e.IgnoresBurn,
		Airborne: e.Airborne, IgnoresParalysisSpeedDrop: e.IgnoresParalysisSpeedDrop, WeightMod: e.WeightMod,
		UnsupportedAttacker: e.UnsupportedAttacker, UnsupportedDefender: e.UnsupportedDefender,
	}
	var err error
	if out.SpeedMods, err = speedModsToEngine(path+".speedMods", e.SpeedMods); err != nil {
		return nil, err
	}
	if err = e.stage1ToEngine(path, out); err != nil {
		return nil, err
	}
	if err = e.stage2ToEngine(path, out); err != nil {
		return nil, err
	}
	if out.OffBoostType, err = parseType(path+".offBoostType", e.OffBoostType, true); err != nil {
		return nil, err
	}
	if e.DefResistType != nil {
		out.DefResistType = make(map[engine.Type]int, len(e.DefResistType))
		for _, k := range sortedKeys(e.DefResistType) {
			t, err := parseType(path+".defResistType", k, false)
			if err != nil {
				return nil, err
			}
			out.DefResistType[t] = e.DefResistType[k]
		}
	}
	// immuneSet は defAbsorbTypes との重複検査に使う(同じタイプを両方に書くのは不正。ADR-0106 §決定1)。
	immuneSet := make(map[engine.Type]bool, len(e.DefImmuneTypes))
	if e.DefImmuneTypes != nil {
		if len(e.DefImmuneTypes) == 0 {
			return nil, fail(CodeInvalidInput, "%s.defImmuneTypes は空にできない", path)
		}
		out.DefImmuneTypes = make([]engine.Type, 0, len(e.DefImmuneTypes))
		for i, v := range e.DefImmuneTypes {
			t, err := parseType(fmt.Sprintf("%s.defImmuneTypes[%d]", path, i), v, false)
			if err != nil {
				return nil, err
			}
			if immuneSet[t] {
				return nil, fail(CodeInvalidInput, "%s.defImmuneTypes にタイプ %q が重複している", path, v)
			}
			immuneSet[t] = true
			out.DefImmuneTypes = append(out.DefImmuneTypes, t)
		}
	}
	if e.DefAbsorbTypes != nil {
		if len(e.DefAbsorbTypes) == 0 {
			return nil, fail(CodeInvalidInput, "%s.defAbsorbTypes は空にできない", path)
		}
		out.DefAbsorbTypes = make(map[engine.Type]engine.AbsorbEffect, len(e.DefAbsorbTypes))
		for _, k := range sortedKeys(e.DefAbsorbTypes) {
			t, err := parseType(path+".defAbsorbTypes", k, false)
			if err != nil {
				return nil, err
			}
			if immuneSet[t] {
				return nil, fail(CodeInvalidInput,
					"%s.defAbsorbTypes と defImmuneTypes に同じタイプ %q がある", path, k)
			}
			abs, err := e.DefAbsorbTypes[k].toEngine(fmt.Sprintf("%s.defAbsorbTypes.%s", path, k))
			if err != nil {
				return nil, err
			}
			out.DefAbsorbTypes[t] = abs
		}
	}
	return out, nil
}

type abilityDTO struct {
	ID     string            `json:"id"`
	NameJa string            `json:"nameJa"`
	Effect *abilityEffectDTO `json:"effect"`
}

func (a abilityDTO) toEngine(path string) (engine.Ability, error) {
	out := engine.Ability{ID: a.ID, NameJa: a.NameJa}
	if a.Effect != nil {
		eff, err := a.Effect.toEngine(path + ".effect")
		if err != nil {
			return engine.Ability{}, err
		}
		out.Effect = eff
	}
	return out, nil
}

type individualDTO struct {
	Species  speciesDTO `json:"species"`
	Level    int        `json:"level"`
	Nature   natureDTO  `json:"nature"`
	Ability  abilityDTO `json:"ability"`
	Item     *itemDTO   `json:"item"`
	SP       statsDTO   `json:"sp"`
	Ranks    ranksDTO   `json:"ranks"`
	TeraType string     `json:"teraType"`
	Status   string     `json:"status"`
}

func (d individualDTO) toEngine(path string) (engine.Individual, error) {
	species, err := d.Species.toEngine(path + ".species")
	if err != nil {
		return engine.Individual{}, err
	}
	nature, err := d.Nature.toEngine(path + ".nature")
	if err != nil {
		return engine.Individual{}, err
	}
	ability, err := d.Ability.toEngine(path + ".ability")
	if err != nil {
		return engine.Individual{}, err
	}
	item, err := itemToEngine(path+".item", d.Item)
	if err != nil {
		return engine.Individual{}, err
	}
	tera, err := parseType(path+".teraType", d.TeraType, true)
	if err != nil {
		return engine.Individual{}, err
	}
	status := engine.Status(d.Status)
	if d.Status == "" {
		status = engine.StatusNone
	} else if !validStatuses[status] {
		return engine.Individual{}, enumError(path+".status", d.Status)
	}
	return engine.Individual{
		Species: species, Level: d.Level, Nature: nature, Ability: ability, Item: item,
		SP: d.SP.toEngine(), Ranks: d.Ranks.toEngine(), TeraType: tera, Status: status,
	}, nil
}

type fieldDTO struct {
	Weather         string     `json:"weather"`
	Terrain         string     `json:"terrain"`
	AttackerScreens screensDTO `json:"attackerScreens"`
	DefenderScreens screensDTO `json:"defenderScreens"`
}

func (f fieldDTO) toEngine() (engine.Field, error) {
	weather := engine.Weather(f.Weather)
	if f.Weather == "" {
		weather = engine.WeatherNone
	} else if !validWeathers[weather] {
		return engine.Field{}, enumError("field.weather", f.Weather)
	}
	terrain := engine.Terrain(f.Terrain)
	if f.Terrain == "" {
		terrain = engine.TerrainNone
	} else if !validTerrains[terrain] {
		return engine.Field{}, enumError("field.terrain", f.Terrain)
	}
	return engine.Field{
		Weather: weather, Terrain: terrain,
		AttackerScreens: f.AttackerScreens.toEngine(), DefenderScreens: f.DefenderScreens.toEngine(),
	}, nil
}

// parseFormat は形式を検証する。省略("")は single。
func parseFormat(v string) (engine.Format, error) {
	if v == "" {
		return engine.FormatSingle, nil
	}
	f := engine.Format(v)
	if !validFormats[f] {
		return "", enumError("format", v)
	}
	return f, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- 結果の DTO -------------------------------------------------------------

// tenthPercent は 0.1% 単位の整数で持つ表示%。JSON では小数第1位をちょうど1桁持つ
// 数値リテラル(734 -> 73.4、1000 -> 100.0、0 -> 0.0)にする。float を経由しないので
// 73.4 が 73.40000000000001 になる余地がなく、書式が1つに決まって Go と WASM が
// バイト一致する(ADR-0011 §3、ADR-0010 §3.2)。
type tenthPercent int

// MarshalJSON は tenths を "73.4" の形の JSON 数値にする。
func (p tenthPercent) MarshalJSON() ([]byte, error) {
	n := int(p)
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	return []byte(sign + strconv.Itoa(n/10) + "." + strconv.Itoa(n%10)), nil
}

type koDTO struct {
	Hits          int     `json:"hits"`
	Guaranteed    bool    `json:"guaranteed"`
	ChancePercent float64 `json:"chancePercent"`
	// DisplayChancePercent は画面に出す値(確定は 100.0、倒せないときは 0.0)。
	// ChancePercent は engine の生値で、確定のとき 0(ADR-0006)。
	DisplayChancePercent tenthPercent `json:"displayChancePercent"`
}

type calcResultDTO struct {
	Rolls [16]int `json:"rolls"`
	// HitRolls は多段技の1発ごとの16段階(ADR-0142 §3)。常に配列(単発・ダメージなしは [])。
	HitRolls      [][16]int    `json:"hitRolls"`
	MinDamage     int          `json:"minDamage"`
	MaxDamage     int          `json:"maxDamage"`
	MinPercent    tenthPercent `json:"minPercent"`
	MaxPercent    tenthPercent `json:"maxPercent"`
	DefenderHP    int          `json:"defenderHP"`
	Effectiveness float64      `json:"effectiveness"`
	STAB          bool         `json:"stab"`
	Category      string       `json:"category"`
	KO            koDTO        `json:"ko"`
	// Unsupported は「未対応」の印(ADR-0123)。印なしは空配列(null にしない)。
	Unsupported []unsupportedMarkDTO `json:"unsupported"`
}

// unsupportedMarkDTO は印1つ(engine.UnsupportedMark の写し)。
type unsupportedMarkDTO struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
	ID     string `json:"id"`
}

// unsupportedFrom は印を写す。nil も空配列にする。
func unsupportedFrom(ms []engine.UnsupportedMark) []unsupportedMarkDTO {
	out := make([]unsupportedMarkDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, unsupportedMarkDTO{Target: string(m.Target), Reason: string(m.Reason), ID: m.ID})
	}
	return out
}

// calcResultFrom は engine の結果を写す。パーセントは表示%(0.1% 単位。
// 最小側は切り捨て・最大側は四捨五入)で、engine の値を丸め直さず素通しする。
func calcResultFrom(r engine.DamageResult) calcResultDTO {
	minTenths, maxTenths := r.DisplayPercentRangeTenths()
	return calcResultDTO{
		Rolls:         r.Rolls,
		HitRolls:      append([][16]int{}, r.HitRolls...),
		MinDamage:     r.MinDamage(),
		MaxDamage:     r.MaxDamage(),
		MinPercent:    tenthPercent(minTenths),
		MaxPercent:    tenthPercent(maxTenths),
		DefenderHP:    r.DefenderHP,
		Effectiveness: r.Effectiveness,
		STAB:          r.STAB,
		Category:      string(r.Category),
		KO: koDTO{
			Hits: r.KO.Hits, Guaranteed: r.KO.Guaranteed, ChancePercent: r.KO.ChancePercent,
			DisplayChancePercent: tenthPercent(r.KO.DisplayChancePercentTenths()),
		},
		Unsupported: unsupportedFrom(r.Unsupported),
	}
}
