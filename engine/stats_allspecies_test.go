//go:build allspecies

package engine

import "testing"

// standardStat は @smogon/calc(gen9)が使う標準式を独立に実装したもの。
// Lv50・個体値31固定。EV は SP から max(0,8×SP−4) で換算する。
// engine の実数値がこの標準式と全 base・全 SP で一致することを確認し、
// チャンピオンズ式の展開が正しいことを担保する(最終照合はゴールデン P1-6)。
func standardEVFromSP(sp int) int {
	ev := 8*sp - 4
	if ev < 0 {
		return 0
	}
	return ev
}

func standardHP(base, sp int) int {
	ev := standardEVFromSP(sp)
	return (2*base+FixedIV+ev/4)*DefaultLevel/100 + DefaultLevel + 10
}

func standardOther(base, sp int, mult10 int) int {
	ev := standardEVFromSP(sp)
	inner := (2*base+FixedIV+ev/4)*DefaultLevel/100 + 5
	// 性格補正は整数 ×(mult10/10)。mult10 ∈ {9,10,11}
	return inner * mult10 / 10
}

func TestAllSpeciesRealStatFormula(t *testing.T) {
	natures := []struct {
		mult10 int
		n      Nature
	}{
		{11, Nature{Plus: StatAtk, Minus: StatSpA}}, // +Atk
		{10, NatureNeutral},                         // 無補正
		{9, Nature{Plus: StatSpA, Minus: StatAtk}},  // -Atk
	}
	// base 1..255 を「全ポケモン」の代理として全 SP 0..32 で網羅。
	for base := 1; base <= 255; base++ {
		for sp := 0; sp <= MaxSPPerStat; sp++ {
			// HP は性格補正なし
			gotHP := RealStats(Individual{
				Species: Species{Types: []Type{TypeNormal}, BaseStats: Stats{HP: base}},
				SP:      Stats{HP: sp},
			}).HP
			if wantHP := standardHP(base, sp); gotHP != wantHP {
				t.Fatalf("HP base=%d sp=%d got=%d want=%d", base, sp, gotHP, wantHP)
			}
			// Atk を代表に性格補正3種を検証
			for _, nc := range natures {
				got := RealStats(Individual{
					Species: Species{Types: []Type{TypeNormal}, BaseStats: Stats{Atk: base}},
					Nature:  nc.n,
					SP:      Stats{Atk: sp},
				}).Atk
				if want := standardOther(base, sp, nc.mult10); got != want {
					t.Fatalf("Atk base=%d sp=%d mult10=%d got=%d want=%d", base, sp, nc.mult10, got, want)
				}
			}
		}
	}
}
