package domain

// 調整(ADR-0250)の値域の検査。ID の解決・列挙の検証より前に見る(ADR-0250 §5・ADR-0208 §3)。
// 失敗はすべて invalid_input(engine のゼロ値「既定」をそのまま渡さない。ADR-0250 §3)。

import "example.com/pokecalc/engine"

// CheckAdjustHits は発数 1..engine.MaxAdjustHits を確かめる(欠落 = 0 も拒否する)。
func CheckAdjustHits(label string, hits int) error {
	if hits < 1 || hits > engine.MaxAdjustHits {
		return Errorf(CodeInvalidInput, "%s は 1..%d でなければならない: %d", label, engine.MaxAdjustHits, hits)
	}
	return nil
}

// AdjustThreshold はしきい値を確かめて engine に渡す値にする。省略は 0(engine の既定 = 100)、
// 明示した値は (0, 100] でなければならない(ADR-0250 §3)。
func AdjustThreshold(label string, p *float64) (float64, error) {
	if p == nil {
		return 0, nil
	}
	if !(*p > 0 && *p <= engine.DefaultAdjustThresholdPercent) {
		return 0, Errorf(CodeInvalidInput, "%s は (0, %v] でなければならない: %v", label, engine.DefaultAdjustThresholdPercent, *p)
	}
	return *p, nil
}

// Ceiling は能力ごとの SP の上限の指定(省略した項目は nil = engine.MaxSPPerStat)。
type Ceiling struct {
	HP, Atk, Def, SpA, SpD, Spe *int
}

// AdjustCeiling は能力ごとの上限を確かめ、省略を engine.MaxSPPerStat で補う(ADR-0250 §3)。
func AdjustCeiling(c *Ceiling) (engine.Stats, error) {
	var in Ceiling
	if c != nil {
		in = *c
	}
	var out engine.Stats
	for _, f := range []struct {
		key engine.StatKey
		v   *int
	}{
		{engine.StatHP, in.HP}, {engine.StatAtk, in.Atk}, {engine.StatDef, in.Def},
		{engine.StatSpA, in.SpA}, {engine.StatSpD, in.SpD}, {engine.StatSpe, in.Spe},
	} {
		v := engine.MaxSPPerStat
		if f.v != nil {
			v = *f.v
		}
		if v < 0 || v > engine.MaxSPPerStat {
			return engine.Stats{}, Errorf(CodeInvalidInput, "ceiling.%s は 0..%d でなければならない: %d", f.key, engine.MaxSPPerStat, v)
		}
		out = out.WithStat(f.key, v)
	}
	return out, nil
}

// AdjustModifier は補正を確かめて engine に渡す値にする。省略は 4096、明示した値は
// engine.MinEffectModifier..engine.MaxEffectModifier でなければならない(技の有無にかかわらず検査する。ADR-0250 §3・§5)。
func AdjustModifier(label string, p *int) (int, error) {
	if p == nil {
		return engine.Modifier4096, nil
	}
	if *p < engine.MinEffectModifier || *p > engine.MaxEffectModifier {
		return 0, Errorf(CodeInvalidInput, "%s は %d..%d でなければならない: %d", label, engine.MinEffectModifier, engine.MaxEffectModifier, *p)
	}
	return *p, nil
}

// CheckAdjustGoalCount は複数の目標の件数 1..engine.MaxSPGoals を確かめる(ADR-0177 §9)。
func CheckAdjustGoalCount(n int) error {
	if n < 1 || n > engine.MaxSPGoals {
		return Errorf(CodeInvalidInput, "goals は 1..%d 件でなければならない: %d", engine.MaxSPGoals, n)
	}
	return nil
}

// AdjustMinSpeed は素早さの目標(省略は 0 = 目標なし)を確かめる。負は invalid_input。
func AdjustMinSpeed(p *int) (int, error) {
	v := 0
	if p != nil {
		v = *p
	}
	if v < 0 {
		return 0, Errorf(CodeInvalidInput, "minSpeed は 0 以上でなければならない: %d", v)
	}
	return v, nil
}

// ParseAllocMode は振り分けの側を検証する(必須。欠落も invalid_enum)。
func ParseAllocMode(m Enum) (engine.AllocMode, error) {
	if !m.Known {
		return "", Errorf(CodeInvalidEnum, "mode に未知の値 %q", m.Value)
	}
	return engine.AllocMode(m.Value), nil
}

// ParseBulkFocus は耐久の重みを検証する(省略はゼロ値。使う側での欠落は engine が invalid_input にする。ADR-0250 §4)。
func ParseBulkFocus(f *Enum) (engine.BulkFocus, error) {
	if f == nil {
		return "", nil
	}
	if !f.Known {
		return "", Errorf(CodeInvalidEnum, "focus に未知の値 %q", f.Value)
	}
	return engine.BulkFocus(f.Value), nil
}

// ParseOffenseCategory は攻撃の分類を検証する(省略はゼロ値。使う側での欠落は engine が invalid_input にする。ADR-0250 §4)。
func ParseOffenseCategory(c *Enum) (engine.MoveCategory, error) {
	if c == nil {
		return "", nil
	}
	if !c.Known {
		return "", Errorf(CodeInvalidEnum, "offenseCategory に未知の値 %q", c.Value)
	}
	return engine.MoveCategory(c.Value), nil
}

// ParseSPGoalKind は目標の種類を検証する(必須。label は入力の場所。例 "goals[0]")。
func ParseSPGoalKind(label string, k Enum) (engine.SPGoalKind, error) {
	if !k.Known {
		return "", Errorf(CodeInvalidEnum, "%s.kind に未知の値 %q", label, k.Value)
	}
	return engine.SPGoalKind(k.Value), nil
}
