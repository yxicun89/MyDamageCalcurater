package engine

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
	// ModifierStab はタイプ一致補正 ×1.5。
	ModifierStab = 6144
	// ModifierAdaptability はタイプ一致補正を上げる特性(てきおうりょく)の一致補正 ×2.0。
	ModifierAdaptability = 8192

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

// chainMods は複数の 4096基準補正を連結して1つの補正にまとめる(@smogon-calc 互換)。
// 各ステップは (M*mod + modifierRoundHalf) >> 12 = 切り上げ寄りの丸め。最終適用は pokeRound で行う。
// 壁・持ち物・特性など「その他補正」はこの方式で1回にまとめないとゴールデンと一致しない。
func chainMods(mods []int) int {
	m := Modifier4096
	for _, mod := range mods {
		if mod != Modifier4096 {
			m = (m*mod + modifierRoundHalf) >> 12
		}
	}
	return m
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

// NullifyKind は特性でダメージが 0 になった理由(ADR-0106)。"" は無効化されていない。
// タイプ由来の無効(Effectiveness == 0)はここには含めない(ADR-0017 §5・oracle champions.ts L262)。
type NullifyKind string

const (
	NullifyNone   NullifyKind = ""
	NullifyImmune NullifyKind = "immune"
	NullifyAbsorb NullifyKind = "absorb"
)

// DamageResult はダメージ計算の結果。確定数は P1-5 で付与する。
type DamageResult struct {
	Rolls         [16]int // 16段階の乱数ダメージ(非減少)
	Effectiveness float64 // タイプ相性(0, 0.25, 0.5, 1, 2, 4)
	STAB          bool    // タイプ一致
	Category      MoveCategory
	DefenderHP    int
	KO            KOChance    // 確定数/乱数n発
	Nullified     NullifyKind // 特性による無効・吸収でダメージが0のとき(ADR-0106)
}

// abilityNullification は防御側の特性がその攻撃タイプを無効・吸収するかを返す。
// 無効(DefImmuneTypes)が吸収(DefAbsorbTypes)に勝つ(ADR-0106 §決定1: 不正な重複入力でも結果を揺らさない)。
func abilityNullification(e *AbilityEffect, moveType Type) NullifyKind {
	if e == nil {
		return NullifyNone
	}
	for _, t := range e.DefImmuneTypes {
		if t == moveType {
			return NullifyImmune
		}
	}
	if _, ok := e.DefAbsorbTypes[moveType]; ok {
		return NullifyAbsorb
	}
	return NullifyNone
}

// MinDamage / MaxDamage は 16段階の下限・上限。
func (r DamageResult) MinDamage() int { return r.Rolls[0] }
func (r DamageResult) MaxDamage() int { return r.Rolls[15] }

// stabModifier はタイプ一致補正値を返す(通常 ModifierStab、てきおうりょく等 AbilityEffect.StabMod
// (ModifierAdaptability)、不一致 Modifier4096)。
func stabModifier(in DamageInput, moveType Type) (int, bool) {
	if moveType == TypeNone || !hasType(in.Attacker, moveType) {
		return Modifier4096, false
	}
	mod := ModifierStab
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.StabMod != 0 {
		mod = ae.StabMod
	}
	return mod, true
}

// burnModifier は物理やけどによる攻撃半減(ModifierHalf)を返す。
// やけど無効化の特性(こんじょう等)は AbilityEffect.IgnoresBurn で表す。
func burnModifier(in DamageInput) int {
	ignores := false
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
func attackDefenseStats(in DamageInput) (atk, def int) {
	var atkKey, defKey StatKey
	if in.Move.Category == CategoryPhysical {
		atkKey, defKey = StatAtk, StatDef
	} else {
		atkKey, defKey = StatSpA, StatSpD
	}
	atkStage := in.Attacker.Ranks.Get(atkKey)
	defStage := in.Defender.Ranks.Get(defKey)
	if in.Critical {
		if atkStage < 0 {
			atkStage = 0
		}
		if defStage > 0 {
			defStage = 0
		}
	}
	atk = applyStatStage(RealStats(in.Attacker).Get(atkKey), atkStage)
	def = applyStatStage(RealStats(in.Defender).Get(defKey), defStage)
	// 天候の防御補正は持ち物より先に独立して丸める。
	def = pokeRound(def, weatherDefenseMod(in, defKey))
	atk = max(1, pokeRound(atk, offensiveStatMod(in, atkKey)))
	def = max(1, pokeRound(def, defensiveStatMod(in, defKey)))
	return atk, def
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
	if err := in.Attacker.Validate(); err != nil {
		return DamageResult{}, err
	}
	if err := in.Defender.Validate(); err != nil {
		return DamageResult{}, err
	}
	if err := validateAgainstTypeChart(in); err != nil {
		return DamageResult{}, err
	}

	res := DamageResult{
		Category:   in.Move.Category,
		DefenderHP: RealStats(in.Defender).HP,
	}

	moveType := in.Move.Type
	eff, err := in.TypeChart.Effectiveness(moveType, in.Defender.Species.Types)
	if err != nil {
		return DamageResult{}, err
	}
	res.Effectiveness = eff.Multiplier()
	_, res.STAB = stabModifier(in, moveType)

	// タイプ由来の無効が先(oracle champions.ts L262 / ADR-0017 §5)。
	// 同じタイプを特性でも無効にしている場合は、タイプ由来として報告する(Nullified は空のまま)。
	if !eff.IsImmune() {
		res.Nullified = abilityNullification(in.Defender.Ability.Effect, moveType)
	}

	// 変化技・威力0・無効相性・特性による無効/吸収はダメージ0。
	if in.Move.Category == CategoryStatus || in.Move.Power <= 0 || eff.IsImmune() || res.Nullified != NullifyNone {
		return res, nil
	}

	level := in.Attacker.EffectiveLevel()
	atk, def := attackDefenseStats(in)

	// 基礎ダメージ(すべて floor)
	power := max(1, pokeRound(in.Move.Power, powerModifier(in)))
	base := (((2*level/5+2)*power*atk)/def)/50 + 2

	// 基礎段階の補正(天候のダメージ倍率は個別に pokeRound)→ 急所
	if wm := weatherDamageMod(in.Field.Weather, moveType); wm != Modifier4096 {
		base = pokeRound(base, wm)
	}
	if in.Critical {
		base = base * 3 / 2 // ×1.5 floor
	}

	stabMod, _ := stabModifier(in, moveType)
	burnMod := burnModifier(in)

	otherMod := chainMods(otherModifiers(in, eff))

	for i := 0; i < 16; i++ {
		d := base * (85 + i) / 100 // 乱数(floor)
		// STAB の五捨五超入を済ませてから相性を掛けて floor する。
		d = pokeRound(d, stabMod) * eff.Num / eff.Den
		d = pokeRound(d, burnMod)  // やけど
		d = pokeRound(d, otherMod) // その他補正(壁・持ち物・特性 P1-4)
		if d < 1 {
			d = 1 // 相性≠0 なら最低1ダメージ
		}
		res.Rolls[i] = d
	}
	res.KO = ComputeKO(res.Rolls, res.DefenderHP)
	return res, nil
}
