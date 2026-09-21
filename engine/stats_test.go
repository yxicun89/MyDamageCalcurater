package engine

import "testing"

func indiv(base Stats, sp Stats, n Nature, ranks Ranks) Individual {
	return Individual{
		Species: Species{Types: []Type{TypeNormal}, BaseStats: base},
		Nature:  n,
		SP:      sp,
		Ranks:   ranks,
	}
}

func TestRealStatsKnownValues(t *testing.T) {
	// 種族値が攻撃寄り(HP108/Atk130/Def95/SpA80/SpD85/Spe102)の種族 Lv50。既知の実数値で確認。
	garchomp := Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102}

	// A特化(いじっぱり: +Atk/-SpA、Atk に SP32、HP に SP32)
	adamant := Nature{Plus: StatAtk, Minus: StatSpA}
	got := RealStats(indiv(garchomp, Stats{HP: 32, Atk: 32}, adamant, Ranks{}))
	if got.HP != 215 { // 108+75+32
		t.Errorf("HP=%d want 215", got.HP)
	}
	if got.Atk != 200 { // floor((130+20+32)*1.1)=floor(182*1.1)=200
		t.Errorf("Atk=%d want 200", got.Atk)
	}
	if got.SpA != 90 { // floor((80+20+0)*0.9)=floor(90)=90
		t.Errorf("SpA=%d want 90", got.SpA)
	}

	// 無振り無補正(すべて SP0、まじめ)
	neutral := RealStats(indiv(garchomp, Stats{}, NatureNeutral, Ranks{}))
	if neutral.HP != 183 { // 108+75+0
		t.Errorf("HP=%d want 183", neutral.HP)
	}
	if neutral.Atk != 150 { // 130+20+0
		t.Errorf("Atk=%d want 150", neutral.Atk)
	}
	if neutral.Spe != 122 { // 102+20+0
		t.Errorf("Spe=%d want 122", neutral.Spe)
	}
}

func TestRealStatsBoundaries(t *testing.T) {
	base := Stats{HP: 1, Atk: 1, Def: 1, SpA: 1, SpD: 1, Spe: 1}
	// SP0 と SP32 の下限・上限
	lo := RealStats(indiv(base, Stats{}, NatureNeutral, Ranks{}))
	if lo.HP != 76 || lo.Atk != 21 { // 1+75, 1+20
		t.Errorf("lo HP=%d Atk=%d", lo.HP, lo.Atk)
	}
	hi := RealStats(indiv(base, Stats{HP: 32, Atk: 32}, NatureNeutral, Ranks{}))
	if hi.HP != 108 || hi.Atk != 53 { // 1+75+32, 1+20+32
		t.Errorf("hi HP=%d Atk=%d", hi.HP, hi.Atk)
	}
}

func TestApplyStatStage(t *testing.T) {
	tests := []struct {
		stat, stage, want int
	}{
		{200, 0, 200},
		{200, 1, 300},  // *3/2
		{200, 2, 400},  // *4/2
		{200, 6, 800},  // *8/2
		{200, -1, 133}, // *2/3 = 133.33 floor
		{200, -2, 100}, // *2/4
		{200, -6, 50},  // *2/8
		{200, 7, 800},  // クランプして +6
		{200, -7, 50},  // クランプして -6
	}
	for _, tt := range tests {
		if got := applyStatStage(tt.stat, tt.stage); got != tt.want {
			t.Errorf("applyStatStage(%d,%d)=%d want %d", tt.stat, tt.stage, got, tt.want)
		}
	}
}

func TestEffectiveStatAppliesRank(t *testing.T) {
	garchomp := Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102}
	in := indiv(garchomp, Stats{Atk: 32}, Nature{Plus: StatAtk, Minus: StatSpA}, Ranks{Atk: 1})
	// 実 Atk 200、ランク+1 → 300
	if got := EffectiveStat(in, StatAtk); got != 300 {
		t.Errorf("EffectiveStat Atk=%d want 300", got)
	}
	// HP はランクを持たない → 実数値そのまま
	if got := EffectiveStat(in, StatHP); got != RealStats(in).HP {
		t.Errorf("EffectiveStat HP applied rank")
	}
}
