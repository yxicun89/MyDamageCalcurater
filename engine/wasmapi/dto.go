package wasmapi

// 境界の DTO と engine 型への変換(ADR-0011 §3, §4)。
//
// engine の型には json タグを足さず、ここで独立に定義する。フィールド名は lowerCamelCase。
// 変換は素直な写しと列挙の検証だけで、計算・丸めは持たない。

import (
	"fmt"
	"sort"

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
		Types: types, BaseStats: s.BaseStats.toEngine(), Abilities: s.Abilities,
	}, nil
}

type moveDTO struct {
	ID       string `json:"id"`
	NameJa   string `json:"nameJa"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Power    int    `json:"power"`
	Priority int    `json:"priority"`
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
	return engine.Move{ID: m.ID, NameJa: m.NameJa, Type: typ, Category: cat, Power: m.Power, Priority: m.Priority}, nil
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
}

func (e itemEffectDTO) toEngine(path string) (*engine.ItemEffect, error) {
	out := &engine.ItemEffect{
		DamageMod: e.DamageMod, PowerMod: e.PowerMod, OnlySuperEffective: e.OnlySuperEffective,
		BoostTypeMod: e.BoostTypeMod,
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
	return out, nil
}

type itemDTO struct {
	ID     string         `json:"id"`
	NameJa string         `json:"nameJa"`
	Effect *itemEffectDTO `json:"effect"`
}

// itemToEngine は nil(持ち物なし)を nil のまま返す。
func itemToEngine(path string, d *itemDTO) (*engine.Item, error) {
	if d == nil {
		return nil, nil
	}
	item := &engine.Item{ID: d.ID, NameJa: d.NameJa}
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

type abilityEffectDTO struct {
	StabMod              int            `json:"stabMod"`
	OffBoostType         string         `json:"offBoostType"`
	OffBoostTypeMod      int            `json:"offBoostTypeMod"`
	DefResistType        map[string]int `json:"defResistType"`
	ReduceSuperEffective int            `json:"reduceSuperEffective"`
	IgnoresBurn          bool           `json:"ignoresBurn"`
}

func (e abilityEffectDTO) toEngine(path string) (*engine.AbilityEffect, error) {
	out := &engine.AbilityEffect{
		StabMod: e.StabMod, OffBoostTypeMod: e.OffBoostTypeMod,
		ReduceSuperEffective: e.ReduceSuperEffective, IgnoresBurn: e.IgnoresBurn,
	}
	var err error
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

type koDTO struct {
	Hits          int     `json:"hits"`
	Guaranteed    bool    `json:"guaranteed"`
	ChancePercent float64 `json:"chancePercent"`
}

type calcResultDTO struct {
	Rolls         [16]int `json:"rolls"`
	MinDamage     int     `json:"minDamage"`
	MaxDamage     int     `json:"maxDamage"`
	MinPercent    int     `json:"minPercent"`
	MaxPercent    int     `json:"maxPercent"`
	DefenderHP    int     `json:"defenderHP"`
	Effectiveness float64 `json:"effectiveness"`
	STAB          bool    `json:"stab"`
	Category      string  `json:"category"`
	KO            koDTO   `json:"ko"`
}

// calcResultFrom は engine の結果を写す。パーセントは engine.DisplayPercent(整数%)。
func calcResultFrom(r engine.DamageResult) calcResultDTO {
	return calcResultDTO{
		Rolls:         r.Rolls,
		MinDamage:     r.MinDamage(),
		MaxDamage:     r.MaxDamage(),
		MinPercent:    engine.DisplayPercent(r.MinDamage(), r.DefenderHP),
		MaxPercent:    engine.DisplayPercent(r.MaxDamage(), r.DefenderHP),
		DefenderHP:    r.DefenderHP,
		Effectiveness: r.Effectiveness,
		STAB:          r.STAB,
		Category:      string(r.Category),
		KO: koDTO{
			Hits: r.KO.Hits, Guaranteed: r.KO.Guaranteed, ChancePercent: r.KO.ChancePercent,
		},
	}
}
