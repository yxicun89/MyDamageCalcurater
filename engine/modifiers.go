package engine

import (
	"maps"
	"slices"
)

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
	SpeedMods          []SpeedMod      // 素早さの補正(ADR-0139)。ダメージ計算は読まない
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
	StabMod                   int                   // タイプ一致補正を上げる特性: 8192(ModifierAdaptability)。0 は通常(ModifierStab)
	OffBoostType              Type                  // 攻撃実数値強化の対象技タイプ
	OffBoostTypeMod           int                   // 例 6144
	DefResistType             map[Type]int          // 相手の攻撃実数値補正。例 炎・氷技を半減する特性{fire:2048, ice:2048}
	DefImmuneTypes            []Type                // 無効にする攻撃タイプ(ふゆう)。ダメージ0、副次効果なし(ADR-0106)
	DefAbsorbTypes            map[Type]AbsorbEffect // 吸収する攻撃タイプ(ちょすい等)→副次効果。ダメージ0(ADR-0106)
	ReduceSuperEffective      int                   // 抜群技を軽減する特性等: 抜群時に軽減(例 3072)
	IgnoresBurn               bool                  // こんじょう等: やけどの攻撃半減を無効化
	Airborne                  bool                  // ふゆう等: 浮いていて接地しない(フィールドの補正が掛からない。ADR-0116)。地面技の無効は DefImmuneTypes で別に持つ
	SpeedMods                 []SpeedMod            // 素早さの補正(ADR-0139)。ダメージ計算は読まない
	IgnoresParalysisSpeedDrop bool                  // まひの素早さ半減を受けない(ADR-0139)。ダメージ計算は読まない

	// --- 特性の段階1(ADR-0176)。どれもゼロ値は「その効果なし」 ---

	// TypeConvert は攻撃側: TypeConvert.From タイプの技を To タイプに変え、威力に PowerMod を掛ける
	// (フェアリースキン等)。技の機構に type_change を持つ技は変えない。nil は無し。
	TypeConvert *TypeConvert
	// PowerMods は攻撃側: 条件つきの威力補正(テクニシャン・はがねのせいしん等)。成立した要素をすべて掛ける。
	PowerMods []ConditionalPowerMod
	// AuraType / AuraMod は両側: 攻撃側・防御側のどちらが持っていても、AuraType タイプの技の威力に AuraMod を
	// 1回だけ掛ける(フェアリーオーラ等)。組で指定する。
	AuraType Type
	AuraMod  int
	// StatMods は実数値の倍率(ItemEffect.StatMods と同じ読み方: 攻撃するときは atk/spa、
	// 受けるときは def/spd を読む)。例 ちからもち{atk:8192}、ファーコート{def:8192}。spe は使わない(SpeedMods)。
	StatMods map[StatKey]int
	// SeparateStatMods は攻撃側: ランク補正の直後に、他の補正と連鎖させずに単独で丸める実数値の倍率
	// (はりきり{atk:6144})。キーは atk / spa だけ。
	SeparateStatMods map[StatKey]int
	// CritDamageMod は攻撃側: 急所のときの最終ダメージ倍率(スナイパー 6144)。0 は補正なし。
	CritDamageMod int
	// PreventsCritical は防御側: 急所に当たらない(カブトアーマー・シェルアーマー)。
	PreventsCritical bool
	// IgnoresOpponentRanks は相手のランク補正を無視する(てんねん): 攻撃するときは防御側の防御・特防のランクを、
	// 受けるときは攻撃側の攻撃・特攻のランクを 0 として扱う。
	IgnoresOpponentRanks bool
	// IgnoresDefenderAbility は攻撃側: 防御側の特性のうち Breakable なものを無いものとして計算する(かたやぶり)。
	IgnoresDefenderAbility bool
	// Breakable は防御側: 相手が IgnoresDefenderAbility を持つとき、この特性の効果(印を含む)は無いものとして扱われる。
	Breakable bool

	// --- 特性の段階2(技のフラグ。ADR-0178)。どれもゼロ値は「その効果なし」 ---
	// 技のフラグが不明(Move.FlagsKnown が偽)なら、フラグに依存する項目は効かないものとして計算し、印を付ける。

	// PostAuraPowerMods は攻撃側: 条件つきの威力補正のうち、オーラの後・タイプ変換の補正の前に掛けるもの
	// (かたいツメ・パンクロック・ちからずく 5325、てつのこぶし・すてみ 4915)。語彙は PowerMods と同じ。
	PostAuraPowerMods []ConditionalPowerMod
	// FlagTypeConvert は攻撃側: Flag を持つ技を To タイプにする(うるおいボイス)。nil は無し。
	FlagTypeConvert *FlagTypeConvert
	// DefImmuneFlags は防御側: そのフラグの技を無効にする(ぼうおん・ぼうだん)。
	DefImmuneFlags []MoveFlag
	// DefFinalModsByFlag は防御側: 技がそのフラグを持つとき最終補正に掛ける(もふもふの接触半減・パンクロックの音半減)。
	DefFinalModsByFlag map[MoveFlag]int
	// DefFinalModsByType は防御側: 技(変換後)がそのタイプのとき最終補正に掛ける(もふもふの炎 ×2)。
	DefFinalModsByType map[Type]int
	// NoContact は攻撃側: 自分の技を接触しない扱いにする(えんかく。防御側の contact の最終補正を受けない)。
	NoContact bool

	// UnsupportedAttacker / UnsupportedDefender は ItemEffect と同じ「未対応」の印(ADR-0123)。
	UnsupportedAttacker bool
	UnsupportedDefender bool
}

// TypeConvert は技のタイプの変換と、変換したときの威力補正(ADR-0176)。
// From・To は相性表にあるタイプで、From != To。PowerMod は 4096 基準の正の整数(MinEffectModifier..MaxEffectModifier)。
type TypeConvert struct {
	From     Type
	To       Type
	PowerMod int
}

// PowerCondition は条件つきの威力補正(ConditionalPowerMod)の条件の語彙(ADR-0176)。閉じた語彙。
type PowerCondition string

const (
	// PowerConditionMaxBasePower は技の威力が MaxPower 以下(テクニシャン: 60 以下)。
	PowerConditionMaxBasePower PowerCondition = "max_base_power"
	// PowerConditionMoveType は技のタイプが MoveType(タイプ変換の後のタイプ。はがねのせいしん)。
	PowerConditionMoveType PowerCondition = "move_type"
)

// AllPowerConditions は語彙のすべてを定義順で返す(呼び出しごとに新しいスライス)。
func AllPowerConditions() []PowerCondition {
	return []PowerCondition{PowerConditionMaxBasePower, PowerConditionMoveFlag, PowerConditionMoveType}
}

// Known は c が語彙にあるか(大文字小文字を区別する)。
func (c PowerCondition) Known() bool {
	for _, k := range AllPowerConditions() {
		if c == k {
			return true
		}
	}
	return false
}

// ConditionalPowerMod は条件つきの威力補正1つ(ADR-0176)。
//   - max_base_power: MaxPower(1 以上)を使い、MoveType は空。
//   - move_type: MoveType(相性表にあるタイプ)を使い、MaxPower は 0。
//   - move_flag: Flag(既知のフラグ)を使い、MaxPower 0・MoveType 空(ADR-0178)。
//
// Modifier は 4096 基準の正の整数で、4096(中立)は不可。
type ConditionalPowerMod struct {
	Condition PowerCondition
	MaxPower  int
	MoveType  Type
	// Flag は move_flag の条件のフラグ(ADR-0178)。他の条件では空。
	Flag     MoveFlag
	Modifier int
}

// hasType は「そのタイプを持つか」を返す。テラスタル中(TeraType 指定あり)は TeraType だけを見る
// (@smogon/calc の Pokemon.hasType。ADR-0224)。テラス無しは元のタイプ。
func hasType(in Individual, t Type) bool {
	if in.TeraType != TypeNone {
		return in.TeraType == t
	}
	return hasOriginalType(in, t)
}

// hasOriginalType は種族の元のタイプを持つかを返す(テラスを見ない。oracle の hasOriginalType)。
func hasOriginalType(in Individual, t Type) bool {
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
// テラスタル中はテラスタイプで判定する(hasType。ADR-0224)。
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
// シングル・形式未指定は ModifierHalf(×0.5)、ダブルは ModifierDoubleScreen(2732/4096。ADR-0222)。
func screenDamageMod(in DamageInput) int {
	s := in.Field.DefenderScreens
	half := ModifierHalf
	if in.Format == FormatDouble {
		half = ModifierDoubleScreen
	}
	switch in.Move.Category {
	case CategoryPhysical:
		if s.Reflect || s.AuroraVeil {
			return half
		}
	case CategorySpecial:
		if s.LightScreen || s.AuroraVeil {
			return half
		}
	}
	return Modifier4096
}

// offensiveStatMod は攻撃側の持ち物による攻撃実数値の倍率(chainMods 済み)を返す。
func offensiveStatMod(in DamageInput, atkKey StatKey) int {
	var mods []int
	if ae := in.Attacker.Ability.Effect; ae != nil {
		if m, ok := ae.StatMods[atkKey]; ok {
			mods = append(mods, m)
		}
		if ae.OffBoostType == in.Move.Type && ae.OffBoostTypeMod != 0 {
			mods = append(mods, ae.OffBoostTypeMod)
		}
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
	if de := in.Defender.Ability.Effect; de != nil {
		if m, ok := de.StatMods[defKey]; ok {
			mods = append(mods, m)
		}
	}
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

// powerModifier は威力の補正(chainMods 済み)を返す。連鎖の順は oracle と同じ
// フィールド → 攻撃側の条件つき補正(PowerMods)→ オーラ → オーラの後の条件つき補正(PostAuraPowerMods。ADR-0178)
// → タイプ変換 → 持ち物(ADR-0176)。
// in.Move.Type は変換後のタイプ。converted はタイプ変換したか。
func powerModifier(in DamageInput, converted bool) int {
	mods := []int{terrainDamageMod(in.Field.Terrain, in.Move.Type, isGrounded(in.Attacker), isGrounded(in.Defender))}
	ae := in.Attacker.Ability.Effect
	if ae != nil {
		for _, pm := range ae.PowerMods {
			if powerConditionHolds(pm, in.Move) {
				mods = append(mods, pm.Modifier)
			}
		}
	}
	if m := auraMod(in); m != 0 {
		mods = append(mods, m)
	}
	if ae != nil {
		for _, pm := range ae.PostAuraPowerMods {
			if powerConditionHolds(pm, in.Move) {
				mods = append(mods, pm.Modifier)
			}
		}
	}
	if converted {
		mods = append(mods, ae.TypeConvert.PowerMod)
	}
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

// powerConditionHolds は条件つきの威力補正の条件が技 m(タイプは変換後)で成り立つかを返す。
func powerConditionHolds(pm ConditionalPowerMod, m Move) bool {
	switch pm.Condition {
	case PowerConditionMaxBasePower:
		return m.Power <= pm.MaxPower
	case PowerConditionMoveType:
		return m.Type == pm.MoveType
	case PowerConditionMoveFlag:
		return m.hasFlag(pm.Flag)
	}
	return false
}

// auraMod は攻撃側・防御側のどちらかが技のタイプのオーラを持つときの威力補正を返す(両側が持っても1回。攻撃側を優先)。
// 無ければ 0。
func auraMod(in DamageInput) int {
	for _, e := range []*AbilityEffect{in.Attacker.Ability.Effect, in.Defender.Ability.Effect} {
		if e != nil && e.AuraType != TypeNone && e.AuraType == in.Move.Type && e.AuraMod != 0 {
			return e.AuraMod
		}
	}
	return 0
}

func itemEffect(i *Item) *ItemEffect {
	if i == nil {
		return nil
	}
	return i.Effect
}

// otherModifiers はやけどの後に chainMods で1回適用する「その他補正」の一覧を返す。
// 壁・急所の補正の特性・防御側のフラグの補正(ADR-0178)・抜群軽減特性・防御側のタイプの補正(ADR-0178)・
// 持ち物ダメージ倍率・半減きのみの順(oracle の calculateFinalModsChampions と同じ)。
// eff は CalcDamage が1回だけ引いた技のタイプ相性(抜群判定に使う)。
func otherModifiers(in DamageInput, eff Effectiveness) []int {
	var mods []int
	superEffective := eff.IsSuperEffective()

	if !in.Critical {
		if sm := screenDamageMod(in); sm != Modifier4096 {
			mods = append(mods, sm)
		}
	}
	if ae := in.Attacker.Ability.Effect; ae != nil && in.Critical && ae.CritDamageMod != 0 {
		mods = append(mods, ae.CritDamageMod)
	}
	de := in.Defender.Ability.Effect
	// 割り当てを避けるため、フラグの最終補正を持つときだけキーを整列する(ADR-0178)。
	if de != nil && len(de.DefFinalModsByFlag) > 0 {
		noContact := in.Attacker.Ability.Effect != nil && in.Attacker.Ability.Effect.NoContact
		for _, f := range slices.Sorted(maps.Keys(de.DefFinalModsByFlag)) {
			if f == MoveFlagContact && noContact {
				continue
			}
			if in.Move.hasFlag(f) {
				mods = append(mods, de.DefFinalModsByFlag[f])
			}
		}
	}
	if de != nil && de.ReduceSuperEffective != 0 && superEffective {
		mods = append(mods, de.ReduceSuperEffective)
	}
	if de != nil {
		if m, ok := de.DefFinalModsByType[in.Move.Type]; ok {
			mods = append(mods, m)
		}
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
