package engine

// 実数値計算。ポケモンチャンピオンズは Lv50・個体値31固定なので、標準式を
// 展開した次の式で求まる(SP を努力値 max(0,8×SP−4) に換算すると標準式と一致)。
//
//	HP  = 種族値 + HPStatOffset(75) + SP
//	その他 = floor((種族値 + OtherStatOffset(20) + SP) × 性格補正)
//
// 性格補正は float 近似を避け整数演算(×11/10, ×9/10)で行う。

// realOtherStat は HP 以外の実数値を計算する。
func realOtherStat(base, sp int, n Nature, k StatKey) int {
	return applyNature(base+OtherStatOffset+sp, n, k)
}

// applyNature は性格補正を整数演算で適用する。v は非負を前提とする。
func applyNature(v int, n Nature, k StatKey) int {
	if k == StatHP || n.IsNeutral() {
		return v
	}
	switch k {
	case n.Plus:
		return v * 11 / 10 // +10%(floor)
	case n.Minus:
		return v * 9 / 10 // -10%(floor)
	}
	return v
}

// RealStats は SP と性格から実数値を計算する。ランク補正は含まない
// (ランクは戦闘中の一時補正であり、ダメージ計算時に EffectiveStat で適用する)。
func RealStats(in Individual) Stats {
	b := in.Species.BaseStats
	return Stats{
		HP:  b.HP + HPStatOffset + in.SP.HP,
		Atk: realOtherStat(b.Atk, in.SP.Atk, in.Nature, StatAtk),
		Def: realOtherStat(b.Def, in.SP.Def, in.Nature, StatDef),
		SpA: realOtherStat(b.SpA, in.SP.SpA, in.Nature, StatSpA),
		SpD: realOtherStat(b.SpD, in.SP.SpD, in.Nature, StatSpD),
		Spe: realOtherStat(b.Spe, in.SP.Spe, in.Nature, StatSpe),
	}
}

// ランク補正倍率(stage -6..+6)。index = stage+6。
// positive: (2+n)/2、negative: 2/(2+|n|)。
var rankNumerator = [13]int{2, 2, 2, 2, 2, 2, 2, 3, 4, 5, 6, 7, 8}
var rankDenominator = [13]int{8, 7, 6, 5, 4, 3, 2, 2, 2, 2, 2, 2, 2}

// applyStatStage はランク補正を実数値に適用する(floor)。
func applyStatStage(stat, stage int) int {
	if stage < -6 {
		stage = -6
	}
	if stage > 6 {
		stage = 6
	}
	i := stage + 6
	return stat * rankNumerator[i] / rankDenominator[i]
}

// EffectiveStat は実数値にランク補正を適用した、戦闘中の攻撃/防御などの値を返す。
// HP はランクを持たないため実数値をそのまま返す。
func EffectiveStat(in Individual, k StatKey) int {
	real := RealStats(in)
	v := real.Get(k)
	if k == StatHP {
		return v
	}
	return applyStatStage(v, in.Ranks.Get(k))
}
