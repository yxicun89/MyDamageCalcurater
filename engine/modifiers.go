package engine

// 補正(天候・フィールド・壁・持ち物・特性)。
//
// 持ち物・特性の補正定義は「マスタから解決した ItemEffect / AbilityEffect」を
// engine が受け取って適用する(CLAUDE.md: 持ち物・特性をコードにハードコードしない)。
// 天候・フィールド・壁はゲーム機構なので engine のルールとして実装する。
//
// 適用位置(ADR-0004):
//   - 天候のダメージ倍率: base 段階で個別に pokeRound
//   - 天候の防御実数値補正(すなあらし・ゆき): 持ち物より先に独立して丸める
//   - 持ち物の実数値補正(攻撃・特攻を上げる持ち物、特防を上げる持ち物等): 実数値段階で chainMods
//   - フィールド・タイプ強化持ち物: 威力段階
//   - 壁・最終ダメージ倍率: やけどの後に chainMods で1回

// ItemEffect はダメージに影響する持ち物の補正(4096基準)。
type ItemEffect struct {
	StatMods           map[StatKey]int // 実数値倍率。例 攻撃を上げる持ち物{atk:6144}, 特防を上げる持ち物{spd:6144}, 防御・特防を上げる持ち物{def:6144,spd:6144}
	DamageMod          int             // 最終ダメージ倍率。例 最終ダメージを上げる持ち物5324。0 は補正なし
	PowerMod           int             // 威力倍率。例 物理技の威力を上げる持ち物4505。0 は補正なし
	PowerCategory      MoveCategory    // PowerMod の対象分類。空は全分類
	OnlySuperEffective bool            // 抜群時のみ DamageMod を適用(抜群のときだけ効く持ち物用)
	BoostType          Type            // タイプ強化(タイプ技の威力を上げる持ち物)の対象タイプ
	BoostTypeMod       int             // 例 4915(=約1.2倍)
	ResistBerryType    Type            // 半減きのみ: このタイプの抜群技を半減(防御側)
}

// AbilityEffect はダメージに影響する特性の補正(4096基準)。
type AbilityEffect struct {
	StabMod              int          // タイプ一致補正を上げる特性: 8192(ModifierAdaptability)。0 は通常(ModifierStab)
	OffBoostType         Type         // 攻撃実数値強化の対象技タイプ
	OffBoostTypeMod      int          // 例 6144
	DefResistType        map[Type]int // 相手の攻撃実数値補正。例 炎・氷技を半減する特性{fire:2048, ice:2048}
	ReduceSuperEffective int          // 抜群技を軽減する特性等: 抜群時に軽減(例 3072)
	IgnoresBurn          bool         // こんじょう等: やけどの攻撃半減を無効化
}

func hasType(in Individual, t Type) bool {
	for _, ty := range in.Species.Types {
		if ty == t {
			return true
		}
	}
	return false
}

// weatherDamageMod は天候による技ダメージ倍率(base段階)を返す。
func weatherDamageMod(w Weather, moveType Type) int {
	switch w {
	case WeatherSun:
		if moveType == TypeFire {
			return modifierWeatherBoost // ×1.5
		}
		if moveType == TypeWater {
			return ModifierHalf // ×0.5
		}
	case WeatherRain:
		if moveType == TypeWater {
			return modifierWeatherBoost
		}
		if moveType == TypeFire {
			return ModifierHalf
		}
	}
	return Modifier4096
}

// terrainDamageMod はフィールドによる技威力倍率を返す(接地している前提)。
func terrainDamageMod(terr Terrain, moveType Type) int {
	switch terr {
	case TerrainElectric:
		if moveType == TypeElectric {
			return 5325 // ×1.3
		}
	case TerrainGrassy:
		if moveType == TypeGrass {
			return 5325
		}
	case TerrainPsychic:
		if moveType == TypePsychic {
			return 5325
		}
	case TerrainMisty:
		if moveType == TypeDragon {
			return ModifierHalf // ×0.5
		}
	}
	return Modifier4096
}

// screenDamageMod は壁による軽減倍率を返す。急所は壁を貫通するため呼び出し側で除外する。
// シングルは ModifierHalf(×0.5)。
func screenDamageMod(in DamageInput) int {
	s := in.Field.DefenderScreens
	switch in.Move.Category {
	case CategoryPhysical:
		if s.Reflect || s.AuroraVeil {
			return ModifierHalf
		}
	case CategorySpecial:
		if s.LightScreen || s.AuroraVeil {
			return ModifierHalf
		}
	}
	return Modifier4096
}

// offensiveStatMod は攻撃側の持ち物による攻撃実数値の倍率(chainMods 済み)を返す。
func offensiveStatMod(in DamageInput, atkKey StatKey) int {
	var mods []int
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.OffBoostType == in.Move.Type && ae.OffBoostTypeMod != 0 {
		mods = append(mods, ae.OffBoostTypeMod)
	}
	if de := in.Defender.Ability.Effect; de != nil {
		if m, ok := de.DefResistType[in.Move.Type]; ok {
			mods = append(mods, m)
		}
	}
	if e := itemEffect(in.Attacker.Item); e != nil {
		if m, ok := e.StatMods[atkKey]; ok {
			mods = append(mods, m)
		}
	}
	return chainMods(mods)
}

// defensiveStatMod は防御側の持ち物による防御実数値倍率を返す。
func defensiveStatMod(in DamageInput, defKey StatKey) int {
	var mods []int
	if e := itemEffect(in.Defender.Item); e != nil {
		if m, ok := e.StatMods[defKey]; ok {
			mods = append(mods, m)
		}
	}
	return chainMods(mods)
}

func weatherDefenseMod(in DamageInput, defKey StatKey) int {
	if in.Field.Weather == WeatherSand && defKey == StatSpD && hasType(in.Defender, TypeRock) {
		return modifierWeatherBoost
	}
	if in.Field.Weather == WeatherSnow && defKey == StatDef && hasType(in.Defender, TypeIce) {
		return modifierWeatherBoost
	}
	return Modifier4096
}

func powerModifier(in DamageInput) int {
	mods := []int{terrainDamageMod(in.Field.Terrain, in.Move.Type)}
	if e := itemEffect(in.Attacker.Item); e != nil {
		if e.BoostType != TypeNone && e.BoostType == in.Move.Type && e.BoostTypeMod != 0 {
			mods = append(mods, e.BoostTypeMod)
		}
		if e.PowerMod != 0 && (e.PowerCategory == "" || e.PowerCategory == in.Move.Category) {
			mods = append(mods, e.PowerMod)
		}
	}
	return chainMods(mods)
}

func itemEffect(i *Item) *ItemEffect {
	if i == nil {
		return nil
	}
	return i.Effect
}

// otherModifiers はやけどの後に chainMods で1回適用する「その他補正」の一覧を返す。
// 壁・抜群軽減特性・持ち物ダメージ倍率・半減きのみの順。
// eff は CalcDamage が1回だけ引いた技のタイプ相性(抜群判定に使う)。
func otherModifiers(in DamageInput, eff Effectiveness) []int {
	var mods []int
	superEffective := eff.IsSuperEffective()

	if !in.Critical {
		if sm := screenDamageMod(in); sm != Modifier4096 {
			mods = append(mods, sm)
		}
	}
	if de := in.Defender.Ability.Effect; de != nil && de.ReduceSuperEffective != 0 && superEffective {
		mods = append(mods, de.ReduceSuperEffective)
	}
	// 攻撃側の持ち物
	if e := itemEffect(in.Attacker.Item); e != nil {
		if e.DamageMod != 0 && (!e.OnlySuperEffective || superEffective) {
			mods = append(mods, e.DamageMod)
		}
	}
	// 防御側の持ち物(半減きのみ)
	if e := itemEffect(in.Defender.Item); e != nil {
		if e.ResistBerryType != TypeNone && e.ResistBerryType == in.Move.Type && (superEffective || in.Move.Type == TypeNormal) {
			mods = append(mods, ModifierHalf)
		}
	}
	return mods
}
