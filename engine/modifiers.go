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
	// UnsupportedAttacker / UnsupportedDefender は、その側で持つとダメージが変わるのに効果スキーマで
	// 表せない(計算に入れていない)ことの印(ADR-0123)。計算は補正なしで行い、結果に印を付ける。
	UnsupportedAttacker bool
	UnsupportedDefender bool
}

// AbsorbEffect は吸収したときの副次効果(マスタの記述)。ゼロ値は「吸収するが副次効果は持たない」
// (もらいび等)。ダメージ計算はこの値を読まない(ADR-0106 §決定4: 現在HP・現在のランクを
// 受け取らないためモデル化しない)。
type AbsorbEffect struct {
	HealNumerator   int     // 最大HPに対する回復の分子。0 は回復なし
	HealDenominator int     // 回復の分母(1..16)。HealNumerator が 0 でないときだけ意味を持つ
	BoostStat       StatKey // 上げる能力(HP 不可)。"" は無し
	BoostStages     int     // 上げる段階(1..6)。BoostStat があるときだけ意味を持つ
}

// AbilityEffect はダメージに影響する特性の補正(4096基準)。
type AbilityEffect struct {
	StabMod              int                   // タイプ一致補正を上げる特性: 8192(ModifierAdaptability)。0 は通常(ModifierStab)
	OffBoostType         Type                  // 攻撃実数値強化の対象技タイプ
	OffBoostTypeMod      int                   // 例 6144
	DefResistType        map[Type]int          // 相手の攻撃実数値補正。例 炎・氷技を半減する特性{fire:2048, ice:2048}
	DefImmuneTypes       []Type                // 無効にする攻撃タイプ(ふゆう)。ダメージ0、副次効果なし(ADR-0106)
	DefAbsorbTypes       map[Type]AbsorbEffect // 吸収する攻撃タイプ(ちょすい等)→副次効果。ダメージ0(ADR-0106)
	ReduceSuperEffective int                   // 抜群技を軽減する特性等: 抜群時に軽減(例 3072)
	IgnoresBurn          bool                  // こんじょう等: やけどの攻撃半減を無効化
	Airborne             bool                  // ふゆう等: 浮いていて接地しない(フィールドの補正が掛からない。ADR-0116)。地面技の無効は DefImmuneTypes で別に持つ
	// UnsupportedAttacker / UnsupportedDefender は ItemEffect と同じ「未対応」の印(ADR-0123)。
	UnsupportedAttacker bool
	UnsupportedDefender bool
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

// isGrounded はその個体が接地しているかを返す(フィールドの補正の対象か。ADR-0116)。
// @smogon/calc 0.12.0 の util.isGrounded のうち engine がモデル化している条件だけを見る:
// ひこうタイプでない、かつ特性の効果が Airborne(ふゆう等)でない。
// じゅうりょく・くろいてっきゅう(必ず接地)と、ふうせん(浮く)は未モデル化(ADR-0116 §対象外)。
// テラスタイプは他の補正と同じく見ない(engine は種族のタイプで相性・一致を判定している)。
func isGrounded(in Individual) bool {
	if hasType(in, TypeFlying) {
		return false
	}
	if ae := in.Ability.Effect; ae != nil && ae.Airborne {
		return false
	}
	return true
}

// terrainDamageMod はフィールドによる技威力倍率を返す。
// 威力を上げる補正(エレキ・グラス・サイコ)は攻撃側が接地しているとき、
// ミストフィールドのドラゴン半減は防御側が接地しているときだけ掛かる(ADR-0116)。
func terrainDamageMod(terr Terrain, moveType Type, attackerGrounded, defenderGrounded bool) int {
	switch terr {
	case TerrainElectric:
		if attackerGrounded && moveType == TypeElectric {
			return 5325 // ×1.3
		}
	case TerrainGrassy:
		if attackerGrounded && moveType == TypeGrass {
			return 5325
		}
	case TerrainPsychic:
		if attackerGrounded && moveType == TypePsychic {
			return 5325
		}
	case TerrainMisty:
		if defenderGrounded && moveType == TypeDragon {
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
	return chainMods(mods, statModBounds)
}

// defensiveStatMod は防御側の持ち物による防御実数値倍率を返す。
func defensiveStatMod(in DamageInput, defKey StatKey) int {
	var mods []int
	if e := itemEffect(in.Defender.Item); e != nil {
		if m, ok := e.StatMods[defKey]; ok {
			mods = append(mods, m)
		}
	}
	return chainMods(mods, statModBounds)
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
	mods := []int{terrainDamageMod(in.Field.Terrain, in.Move.Type, isGrounded(in.Attacker), isGrounded(in.Defender))}
	if e := itemEffect(in.Attacker.Item); e != nil {
		if e.BoostType != TypeNone && e.BoostType == in.Move.Type && e.BoostTypeMod != 0 {
			mods = append(mods, e.BoostTypeMod)
		}
		if e.PowerMod != 0 && (e.PowerCategory == "" || e.PowerCategory == in.Move.Category) {
			mods = append(mods, e.PowerMod)
		}
	}
	return chainMods(mods, powerModBounds)
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
