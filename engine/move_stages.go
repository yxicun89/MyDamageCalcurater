package engine

// 確率 100% のランク変化(ADR-0179 §1。連続技 CB-2)。
// 確率 100% 未満は常に返さない(docs/plan/combo-calc.md §7 Q4)。命中率は見ない。

// stagesToRanks は効果の Stages を Ranks に写す(atk/def/spa/spd/spe。HP は持たない)。
func stagesToRanks(stages map[StatKey]int) Ranks {
	return Ranks{
		Atk: stages[StatAtk],
		Def: stages[StatDef],
		SpA: stages[StatSpA],
		SpD: stages[StatSpD],
		Spe: stages[StatSpe],
	}
}

func guaranteedStages(m Move, target RankTarget) Ranks {
	e := m.Effect
	if e == nil || e.Chance != 100 || e.Target != target {
		return Ranks{}
	}
	return stagesToRanks(e.Stages)
}

// GuaranteedSelfStages は技の追加効果のうち、確率 100% で必ず起きる使用者自身のランク変化を返す(無ければゼロ値)。
func GuaranteedSelfStages(m Move) Ranks { return guaranteedStages(m, RankTargetSelf) }

// GuaranteedTargetStages は確率 100% で必ず起きる、技の対象(相手)のランク変化を返す(無ければゼロ値)。
func GuaranteedTargetStages(m Move) Ranks { return guaranteedStages(m, RankTargetOpponent) }

// AddClamped は r に d を足し、各能力を -6..+6 に収めた値を返す。
func (r Ranks) AddClamped(d Ranks) Ranks {
	c := func(a, b int) int { return max(-6, min(6, a+b)) }
	return Ranks{
		Atk: c(r.Atk, d.Atk),
		Def: c(r.Def, d.Def),
		SpA: c(r.SpA, d.SpA),
		SpD: c(r.SpD, d.SpD),
		Spe: c(r.Spe, d.Spe),
	}
}
