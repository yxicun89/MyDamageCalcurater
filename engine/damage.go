package engine

import (
	"fmt"
	"slices"
)

// ダメージ計算コア。第9世代の式を 4096基準の固定小数・五捨五超入(pokeRound)で実装する。
// float 近似はしない(CLAUDE.md ドメイン規約)。
//
// 補正の適用順(Bulbapedia / @smogon-calc gen9):
//
//	base = floor(floor(floor(2*L/5+2)*威力*A/D)/50)+2
//	base ×= 天候など基礎段階の補正(P1-4)
//	base ×= 急所(1.5, floor)
//	各ロール i(0..15): d = floor(base*(85+i)/100)
//	  d = pokeRound(d × タイプ一致)            (ModifierStab or Modifier4096、Adaptability は ModifierAdaptability)
//	  d = floor(d × タイプ相性)                (Effectiveness の Num/Den)
//	  d = pokeRound(d × やけど)                (物理やけどで ModifierHalf)
//	  d = pokeRound(d × その他補正)            (壁・持ち物・特性など P1-4)
//	  相性≠0 なら d = max(1, d)

// 補正値は 4096 を等倍(=1.0)とする固定小数で表す(値 = 倍率 × 4096)。
// 出典: 第9世代の補正計算(Bulbapedia / @smogon/calc gen9)。
const (
	// Modifier4096 は等倍(×1.0)。補正値の基準。
	Modifier4096 = 4096
	// ModifierHalf は ×0.5(4096 に対する比 1/2)。やけど・壁・ミストフィールド・
	// 天候による弱化・半減きのみなど、半減する補正すべてで共通。
	ModifierHalf = 2048
	// ModifierDoubleScreen はダブルの壁 ×2732/4096(ADR-0222)。
	ModifierDoubleScreen = 2732
	// ModifierSpread はダブルの全体技 ×3072/4096(×0.75。ADR-0222)。
	ModifierSpread = 3072
	// ModifierStab はタイプ一致補正 ×1.5。
	ModifierStab = 6144
	// ModifierAdaptability はタイプ一致補正を上げる特性(てきおうりょく)の一致補正 ×2.0。
	ModifierAdaptability = 8192
	// MinEffectModifier / MaxEffectModifier は持ち物・特性の効果(ItemEffect・AbilityEffect)に
	// 書ける 4096 基準の補正値の範囲 ×1/4096〜×512(issue #255。ADR-0117)。負・0 の倍率は
	// ロールを非単調にしたり全ロールを 1 に潰したりするため、Individual.Validate が拒否する。
	// 上限は engine が掛けるいちばん広いクランプ(powerModBounds の上限 ×512)と同じ。それより
	// 大きい値はどの段階でもクランプされて意味を持たず、連鎖の途中の桁あふれだけを招く。
	// 「0 は補正なし」と定義された項目の 0 はこの範囲とは別に許す。
	MinEffectModifier = 1
	MaxEffectModifier = 512 * Modifier4096

	// modifierWeatherBoost は天候による強化 ×1.5(はれ/あめの技ダメージ、
	// すなあらし/ゆきの防御実数値)。
	modifierWeatherBoost = 6144
	// modifierRoundHalf は五捨五超入・連鎖丸めで加える「Modifier4096 の半分」(0.5 に相当)。
	modifierRoundHalf = Modifier4096 / 2
)

// pokeRound は value×mod/4096 を五捨五超入(半分ちょうどは切り捨て)する。
func pokeRound(value, mod int) int {
	v := value * mod
	if v%Modifier4096 > modifierRoundHalf {
		return v/Modifier4096 + 1
	}
	return v / Modifier4096
}

// modBounds は連結した補正のクランプ範囲(両端を含む)。
type modBounds struct {
	lower, upper int
}

// 連結した補正のクランプ範囲。@smogon/calc 0.12.0 の chainMods の呼び出し(champions / gen789)と同じ値。
// 現在のマスタの補正集合では届かないが、補正を足したときに oracle と黙って乖離しないように持つ(issue #77)。
var (
	// finalModBounds は「その他補正」(壁・持ち物・特性・半減きのみ)の範囲 ×0.01〜×32。
	finalModBounds = modBounds{lower: 41, upper: 131072}
	// powerModBounds は威力補正の範囲 ×0.01〜×512。
	powerModBounds = modBounds{lower: 41, upper: 2097152}
	// statModBounds は攻撃・防御の実数値補正の範囲 ×0.1〜×32。
	statModBounds = modBounds{lower: 410, upper: 131072}
)

// chainMods は複数の 4096基準補正を連結して1つの補正にまとめる(@smogon-calc 互換)。
// 各ステップは (M*mod + modifierRoundHalf) >> 12 = 切り上げ寄りの丸め。最終適用は pokeRound で行う。
// 壁・持ち物・特性など「その他補正」はこの方式で1回にまとめないとゴールデンと一致しない。
// 結果は bounds の範囲にクランプする(oracle と同じ)。
func chainMods(mods []int, bounds modBounds) int {
	m := Modifier4096
	for _, mod := range mods {
		if mod != Modifier4096 {
			m = (m*mod + modifierRoundHalf) >> 12
		}
	}
	return min(max(m, bounds.lower), bounds.upper)
}

// DamageInput はダメージ計算の入力。
type DamageInput struct {
	Format   Format
	Attacker Individual
	Defender Individual
	Move     Move
	Field    Field
	Critical bool
	// TypeChart はタイプ相性表(マスタ由来の入力。ADR-0013)。Field と同じ「入力」の扱い。
	// ゼロ値は未設定で、CalcDamage は ErrTypeChartMissing を返す(既定の表にフォールバックしない)。
	TypeChart TypeChart
}

// NullifyKind はタイプ相性以外の理由でダメージが 0 になった理由(特性は ADR-0106、サイコフィールドは ADR-0123)。
// "" は無効化されていない。タイプ由来の無効(Effectiveness == 0)はここには含めない(ADR-0017 §5・oracle champions.ts L262)。
type NullifyKind string

const (
	NullifyNone   NullifyKind = ""
	NullifyImmune NullifyKind = "immune"
	NullifyAbsorb NullifyKind = "absorb"
	// NullifyPsychicTerrain はサイコフィールドで、優先度が正の技が接地した防御側に当たらない(ADR-0123)。
	NullifyPsychicTerrain NullifyKind = "psychic_terrain"
	// NullifyOHKOImmune は一撃必殺技が効かない理由(無効タイプを持つ・一撃必殺を防ぐ特性。ADR-0142 §4)。
	NullifyOHKOImmune NullifyKind = "ohko_immune"
)

// DamageResult はダメージ計算の結果。確定数は P1-5 で付与する。
type DamageResult struct {
	Rolls [16]int // 16段階の乱数ダメージ(非減少)。多段技は1回の使用の同じ段の合計(Σ HitRolls[h][i])
	// HitRolls は多段技の1発ごとの16段階(長さ = 回数。ADR-0142 §3)。単発の技・ダメージなしは nil。
	HitRolls      [][16]int
	Effectiveness float64 // タイプ相性(0, 0.25, 0.5, 1, 2, 4)
	STAB          bool    // タイプ一致
	Category      MoveCategory
	DefenderHP    int
	KO            KOChance    // 確定数/乱数n発
	Nullified     NullifyKind // 特性による無効・吸収(ADR-0106)・サイコフィールド(ADR-0123)でダメージが0のとき
	// Unsupported は engine が正しく計算できない技の機構・持ち物・特性の印(ADR-0123)。nil は印なし。
	// 印があっても Rolls 等は通常の式の値(正しくない可能性がある)。
	Unsupported []UnsupportedMark
}

// abilityNullification は防御側の特性がその技を無効・吸収するかを返す。
// 無効(DefImmuneTypes・技のフラグによる DefImmuneFlags。ADR-0178)が吸収(DefAbsorbTypes)に勝つ
// (ADR-0106 §決定1: 不正な重複入力でも結果を揺らさない)。
func abilityNullification(e *AbilityEffect, move Move) NullifyKind {
	if e == nil {
		return NullifyNone
	}
	moveType := move.Type
	for _, t := range e.DefImmuneTypes {
		if t == moveType {
			return NullifyImmune
		}
	}
	for _, f := range e.DefImmuneFlags {
		if move.hasFlag(f) {
			return NullifyImmune
		}
	}
	if _, ok := e.DefAbsorbTypes[moveType]; ok {
		return NullifyAbsorb
	}
	return NullifyNone
}

// blockedByPsychicTerrain は、サイコフィールドで優先度が正の攻撃技が接地した防御側に当たらないかを返す
// (@smogon/calc 0.12.0 champions.js: move.priority > 0 && field.hasTerrain('Psychic') && isGrounded(defender)。
// ADR-0121 §5・ADR-0123)。接地の判定は ADR-0116 の isGrounded と同じ。
func blockedByPsychicTerrain(in DamageInput) bool {
	return in.Move.Category != CategoryStatus && in.Move.Priority > 0 &&
		in.Field.Terrain == TerrainPsychic && isGrounded(in.Defender)
}

// MinDamage / MaxDamage は 16段階の下限・上限。
func (r DamageResult) MinDamage() int { return r.Rolls[0] }
func (r DamageResult) MaxDamage() int { return r.Rolls[15] }

// stabModifier はタイプ一致補正値を返す(通常 ModifierStab、てきおうりょく等 AbilityEffect.StabMod
// (ModifierAdaptability)、不一致 Modifier4096)。
// 4096 を基準に、元のタイプ一致で +2048、テラスタイプ一致(teraType 指定時。ADR-0224)で +2048 を足す
// (テラス = 元のタイプ = 技で ×2.0、テラスが別タイプでも元タイプの技は ×1.5 のまま、テラスだけ一致で ×1.5)。
// 特性の強化分(StabMod − ModifierStab)は「そのタイプを持つ」技のときだけ足し、テラスが元のタイプのときは
// その半分にする(@smogon/calc の getStabMod と同じ)。
func stabModifier(in DamageInput, moveType Type) (int, bool) {
	if moveType == TypeNone {
		return Modifier4096, false
	}
	const stabBonus = ModifierStab - Modifier4096 // 一致1種類あたりの加算分
	mod := Modifier4096
	if hasOriginalType(in.Attacker, moveType) {
		mod += stabBonus
	}
	teraMatch := in.Attacker.TeraType != TypeNone && in.Attacker.TeraType == moveType
	if teraMatch {
		mod += stabBonus
	}
	// てきおうりょく等: 技のタイプを持つときだけ加算。テラスが元タイプのときは半分(ADR-0224)。
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.StabMod != 0 && hasType(in.Attacker, moveType) {
		bonus := ae.StabMod - ModifierStab
		if teraMatch && hasOriginalType(in.Attacker, moveType) {
			bonus /= 2
		}
		mod += bonus
	}
	return mod, mod != Modifier4096
}

// burnModifier は物理やけどによる攻撃半減(ModifierHalf)を返す。
// やけど無効化の特性(こんじょう等)は AbilityEffect.IgnoresBurn で表す。
func burnModifier(in DamageInput) int {
	ignores := ruleIgnoresBurn(in)
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.IgnoresBurn {
		ignores = true
	}
	if in.Move.Category == CategoryPhysical &&
		in.Attacker.Status == StatusBurn && !ignores {
		return ModifierHalf
	}
	return Modifier4096
}

// attackDefenseStats は使用する攻撃・防御の実効値を返す。
// 急所時は攻撃側の不利なランク(負)と防御側の有利なランク(正)を無視する。
//
// 参照する能力値(ADR-0142 §5): 攻撃は OffensePokemon の個体の OffenseStat(空なら技の分類)の実数値とランク、
// 防御は DefenseStat(空なら技の分類)の実数値・ランク・天候の補正・防御側の実数値の補正を使う。
// 攻撃側の実数値の補正(持ち物・特性)は技の分類のキーで引く(oracle と同じ)。防御ランク無視の技は防御側のランクを 0 にする。
func attackDefenseStats(in DamageInput) (atk, def int) {
	// 補正を引くキー(技の分類)と、実数値・ランクを引くキー。
	atkModKey, defModKey := StatAtk, StatDef
	if in.Move.Category != CategoryPhysical {
		atkModKey, defModKey = StatSpA, StatSpD
	}
	atkKey, defKey := atkModKey, defModKey
	if k := in.Move.Params.OffenseStat; k != "" {
		atkKey = k
	}
	if k := in.Move.Params.DefenseStat; k != "" {
		defKey = k
	}
	offender := in.Attacker
	if in.Move.Params.OffensePokemon == OffensePokemonDefender {
		offender = in.Defender
	}
	atkStage := offender.Ranks.Get(atkKey)
	defStage := in.Defender.Ranks.Get(defKey)
	if in.Critical {
		if atkStage < 0 {
			atkStage = 0
		}
		if defStage > 0 {
			defStage = 0
		}
	}
	// 相手のランクを無視する特性(てんねん。ADR-0176)。
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.IgnoresOpponentRanks {
		defStage = 0
	}
	if de := in.Defender.Ability.Effect; de != nil && de.IgnoresOpponentRanks {
		atkStage = 0
	}
	if slices.Contains(in.Move.Mechanisms, MechanismIgnoreDefenseRanks) {
		defStage = 0
	}
	atk = applyStatStage(RealStats(offender).Get(atkKey), atkStage)
	def = applyStatStage(RealStats(in.Defender).Get(defKey), defStage)
	// ランクの直後に単独で丸める攻撃側の実数値補正(はりきり。ADR-0176)。他の補正と連鎖しない。
	if ae := in.Attacker.Ability.Effect; ae != nil {
		if m, ok := ae.SeparateStatMods[atkModKey]; ok {
			atk = pokeRound(atk, m)
		}
	}
	// 天候の防御補正は持ち物より先に独立して丸める。
	def = pokeRound(def, weatherDefenseMod(in, defKey))
	atk = max(1, pokeRound(atk, offensiveStatMod(in, atkModKey)))
	def = max(1, pokeRound(def, defensiveStatMod(in, defKey)))
	return atk, def
}

// applyAbilityPreconditions は、計算の前に決める特性の効果(ADR-0176)を入力に反映したコピーを返す。
//   - かたやぶり: 攻撃側が IgnoresDefenderAbility を持ち、防御側の特性が Breakable なら防御側の特性の効果を nil にする
//     (無効・浮遊・急所無効・ランク無視・未対応の印を含めて、特性が無いものとして計算する)。
//   - タイプ変換: 攻撃側の TypeConvert の From タイプの攻撃技(type_change の機構を持たないもの)を To タイプにする。
//     To が相性表に無ければ ErrUnknownType。converted は変換したか(威力の補正に使う)。
//   - フラグによるタイプ変換(ADR-0178): 攻撃側の FlagTypeConvert の Flag を持つ攻撃技(type_change の機構を
//     持たないもの)を To タイプにする。威力の補正は無い。To が相性表に無ければ ErrUnknownType。
//   - 必ず急所: always_crit の攻撃技は急所として計算する(ADR-0142 §5)。急所の無効より前に立てる。
//   - 急所の無効: 防御側が PreventsCritical なら急所の指定を外す。
func applyAbilityPreconditions(in DamageInput) (out DamageInput, converted bool, err error) {
	ae := in.Attacker.Ability.Effect
	if ae != nil && ae.IgnoresDefenderAbility {
		if de := in.Defender.Ability.Effect; de != nil && de.Breakable {
			in.Defender.Ability.Effect = nil
		}
	}
	if ae != nil && ae.TypeConvert != nil && in.Move.Category != CategoryStatus &&
		in.Move.Type == ae.TypeConvert.From && !slices.Contains(in.Move.Mechanisms, MechanismTypeChange) {
		if err := in.TypeChart.requireKnown("攻撃側の特性の変換後のタイプ", ae.TypeConvert.To); err != nil {
			return DamageInput{}, false, err
		}
		in.Move.Type = ae.TypeConvert.To
		converted = true
	} else if ae != nil && ae.FlagTypeConvert != nil && in.Move.Category != CategoryStatus &&
		in.Move.hasFlag(ae.FlagTypeConvert.Flag) && !slices.Contains(in.Move.Mechanisms, MechanismTypeChange) {
		if err := in.TypeChart.requireKnown("攻撃側の特性のフラグによる変換後のタイプ", ae.FlagTypeConvert.To); err != nil {
			return DamageInput{}, false, err
		}
		in.Move.Type = ae.FlagTypeConvert.To
	}
	if in.Move.Category != CategoryStatus && slices.Contains(in.Move.Mechanisms, MechanismAlwaysCrit) {
		in.Critical = true
	}
	if de := in.Defender.Ability.Effect; de != nil && de.PreventsCritical {
		in.Critical = false
	}
	return in, converted, nil
}

// validateAgainstTypeChart は入力の表が設定済みで、入力に現れるタイプ ID(技・両側の種族・
// 両側のテラス)がすべて表にあることを確かめる。表に無い ID を等倍にしない(ADR-0013 §P1-13.3)。
// 個体のタイプ数の検証は Individual.Validate の責務で、ここでは ID だけを見る。
func validateAgainstTypeChart(in DamageInput) error {
	chart := in.TypeChart
	if err := chart.requireKnown("技のタイプ", in.Move.Type); err != nil {
		return err
	}
	if err := chart.requireKnown("攻撃側の種族タイプ", in.Attacker.Species.Types...); err != nil {
		return err
	}
	if err := chart.requireKnown("防御側の種族タイプ", in.Defender.Species.Types...); err != nil {
		return err
	}
	if err := chart.requireKnown("攻撃側のテラスタイプ", in.Attacker.TeraType); err != nil {
		return err
	}
	return chart.requireKnown("防御側のテラスタイプ", in.Defender.TeraType)
}

// CalcDamage は 1 vs 1 のダメージを計算する。
// in.TypeChart が未設定なら ErrTypeChartMissing、表に無いタイプがあれば ErrUnknownType を返す。
func CalcDamage(in DamageInput) (DamageResult, error) {
	res, hasKO, err := calcDamageNoKO(in)
	if err != nil {
		return DamageResult{}, err
	}
	if hasKO {
		res.KO = res.computeKO()
	}
	return res, nil
}

// calcDamageNoKO は CalcDamage から確定数(KO)を除いた計算。KO は Rolls と DefenderHP だけで決まり、
// 作るのが重い(ADR-0126 追記 2026-10-02)ので、KO を使わない逆算・一括計算のまとめ判定はこちらを呼ぶ。
// hasKO は CalcDamage が KO を計算する経路(ダメージが出る)だったか。false のとき KO はゼロ値のまま。
func calcDamageNoKO(in DamageInput) (res DamageResult, hasKO bool, err error) {
	if err := in.Attacker.Validate(); err != nil {
		return DamageResult{}, false, err
	}
	if err := in.Defender.Validate(); err != nil {
		return DamageResult{}, false, err
	}
	if err := validateAgainstTypeChart(in); err != nil {
		return DamageResult{}, false, err
	}
	switch in.Move.Target {
	case "", MoveTargetSingle, MoveTargetSpread:
	default:
		return DamageResult{}, false, fmt.Errorf("%w: %q", ErrUnknownMoveTarget, in.Move.Target)
	}
	if err := validateMoveFlags(in.Move); err != nil {
		return DamageResult{}, false, err
	}

	if err := in.Move.ValidateParams(in.TypeChart); err != nil {
		return DamageResult{}, false, err
	}
	if err := in.Move.ValidateRule(in.TypeChart); err != nil {
		return DamageResult{}, false, err
	}

	// 特性の段階1(ADR-0176): 防御側の特性の無視・技のタイプの変換・急所の無効を、計算の最初に1回だけ決める。
	// 以降は in.Move.Type が変換後のタイプ、in.Defender.Ability.Effect が無視した後の効果になる。
	in, typeConverted, err := applyAbilityPreconditions(in)
	if err != nil {
		return DamageResult{}, false, err
	}

	// 技の処理の定義(ADR-0143 §2 の 1): 優先度・タイプ・全体技を決める。
	in = applyRulePreconditions(in)

	res = DamageResult{
		Category:    in.Move.Category,
		DefenderHP:  RealStats(in.Defender).HP,
		Unsupported: unsupportedMarks(in),
	}

	moveType := in.Move.Type
	eff, err := moveEffectiveness(in, moveType)
	if err != nil {
		return DamageResult{}, false, err
	}
	res.Effectiveness = eff.Multiplier()
	_, res.STAB = stabModifier(in, moveType)

	// タイプ由来の無効が先(oracle champions.ts L262 / ADR-0017 §5)。
	// 同じタイプを特性でも無効にしている場合は、タイプ由来として報告する(Nullified は空のまま)。
	// 防御側が持ち物なしで失敗する技(ポルターガイスト型)は、タイプの無効の後・特性の無効の前(ADR-0143 §2)。
	if !eff.IsImmune() && in.Move.Category != CategoryStatus && ruleFailed(in) {
		res.Nullified = NullifyMoveFailed
	} else if !eff.IsImmune() {
		res.Nullified = abilityNullification(in.Defender.Ability.Effect, in.Move)
	}
	// サイコフィールドの先制技は、タイプ・特性による無効の後に判定する(oracle champions.js と同じ順)。
	if !eff.IsImmune() && res.Nullified == NullifyNone && blockedByPsychicTerrain(in) {
		res.Nullified = NullifyPsychicTerrain
	}

	// 一撃必殺・固定ダメージ(ADR-0142 §4)。無効の判定の後、威力 0 の早期 return より前。
	if in.Move.Category != CategoryStatus && !eff.IsImmune() && res.Nullified == NullifyNone {
		if o := in.Move.Params.OHKO; o != nil {
			if (o.ImmuneType != TypeNone && hasType(in.Defender, o.ImmuneType)) ||
				(in.Defender.Ability.Effect != nil && in.Defender.Ability.Effect.PreventsOHKO) {
				res.Nullified = NullifyOHKOImmune
				return res, false, nil
			}
			res.Rolls = filledRolls(res.DefenderHP)
			return res, true, nil
		}
		if fd := in.Move.Params.FixedDamage; fd != nil {
			v := fd.Value
			if fd.Level {
				v = in.Attacker.EffectiveLevel()
			}
			res.Rolls = filledRolls(v)
			return res, true, nil
		}
	}

	// 威力の式で基本威力が決まる技は威力 0 でも計算する(ADR-0143 §2 の 4)。
	basePower := ruleBasePower(in, 0)
	if in.Move.Category == CategoryStatus || basePower <= 0 || eff.IsImmune() || res.Nullified != NullifyNone {
		return res, false, nil
	}

	hits := multiHitCount(in)
	if r := in.Move.Rule; r != nil && r.PowerFormula == PowerFormulaHitIndex && hits > 1 {
		// h 発目は威力 × h で、1発ごとに計算する。
		res.HitRolls = make([][16]int, hits)
		for h := range res.HitRolls {
			res.HitRolls[h] = damageRolls(in, ruleBasePower(in, h), typeConverted, eff)
			for i, d := range res.HitRolls[h] {
				res.Rolls[i] += d
			}
		}
		return res, true, nil
	}
	per := damageRolls(in, basePower, typeConverted, eff)
	res.Rolls = per
	// 多段技: 1発は同じ計算で、回数ぶん並べる。Rolls は同じ段の合計(ADR-0142 §3)。
	if hits > 1 {
		res.HitRolls = make([][16]int, hits)
		for h := range res.HitRolls {
			res.HitRolls[h] = per
		}
		for i := range res.Rolls {
			res.Rolls[i] = per[i] * hits
		}
	}
	return res, true, nil
}

// damageRolls は基本威力 basePower の1発の16段階を返す。basePower は式・整数倍を反映済みの威力で、
// 特性の条件(テクニシャン等)はこの値で判定する(oracle の basePower)。
func damageRolls(in DamageInput, basePower int, typeConverted bool, eff Effectiveness) (rolls [16]int) {
	in.Move.Power = basePower
	moveType := in.Move.Type
	level := in.Attacker.EffectiveLevel()
	atk, def := attackDefenseStats(in)

	// 基礎ダメージ(すべて floor)
	power := max(1, pokeRound(in.Move.Power, powerModifier(in, typeConverted)))
	base := (((2*level/5+2)*power*atk)/def)/50 + 2

	// ダブルの全体技(ADR-0222)。天候・急所より前。
	if in.Format == FormatDouble && in.Move.Target == MoveTargetSpread {
		base = pokeRound(base, ModifierSpread)
	}
	// 基礎段階の補正(天候のダメージ倍率は個別に pokeRound)→ 急所
	if wm := weatherDamageMod(in.Field.Weather, moveType); wm != Modifier4096 {
		base = pokeRound(base, wm)
	}
	if in.Critical {
		base = base * 3 / 2 // ×1.5 floor
	}

	stabMod, _ := stabModifier(in, moveType)
	burnMod := burnModifier(in)

	otherMod := chainMods(otherModifiers(in, eff), finalModBounds)

	for i := 0; i < 16; i++ {
		d := base * (85 + i) / 100 // 乱数(floor)
		// STAB の五捨五超入を済ませてから相性を掛けて floor する。
		d = pokeRound(d, stabMod) * eff.Num / eff.Den
		d = pokeRound(d, burnMod)  // やけど
		d = pokeRound(d, otherMod) // その他補正(壁・持ち物・特性 P1-4)
		if d < 1 {
			d = 1 // 相性≠0 なら最低1ダメージ
		}
		rolls[i] = d
	}
	return rolls
}

// filledRolls は全 16 段が v の結果(固定ダメージ・一撃必殺)。
func filledRolls(v int) (r [16]int) {
	for i := range r {
		r[i] = v
	}
	return r
}
