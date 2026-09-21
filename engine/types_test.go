package engine

import "testing"

// ドメイン定数の値を固定する。式の出典(CLAUDE.md ドメイン規約・第9世代の補正値)と
// 食い違う変更に気付くためのテストで、計算ロジックの検証はゴールデンテストが担う。
func TestDomainConstants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"DefaultLevel", DefaultLevel, 50},
		{"FixedIV", FixedIV, 31},
		{"MaxSPPerStat", MaxSPPerStat, 32},
		{"MaxSPTotal", MaxSPTotal, 66},
		// HP = 種族値 + 75 + SP / その他 = floor((種族値 + 20 + SP) × 性格補正)
		{"HPStatOffset", HPStatOffset, 75},
		{"OtherStatOffset", OtherStatOffset, 20},
		// 4096 基準(値 = 倍率 × 4096)
		{"Modifier4096", Modifier4096, 4096},
		{"ModifierHalf", ModifierHalf, 2048},
		{"ModifierStab", ModifierStab, 6144},
		{"ModifierAdaptability", ModifierAdaptability, 8192},
		{"modifierWeatherBoost", modifierWeatherBoost, 6144},
		{"modifierRoundHalf", modifierRoundHalf, 2048},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

// 実数値の式が名前付き定数と一致していること(SP0・補正なしで 種族値+定数)。
func TestRealStatsUseOffsets(t *testing.T) {
	base := Stats{HP: 100, Atk: 100, Def: 100, SpA: 100, SpD: 100, Spe: 100}
	got := RealStats(indiv(base, Stats{}, NatureNeutral, Ranks{}))
	if got.HP != 100+HPStatOffset {
		t.Errorf("HP=%d want %d", got.HP, 100+HPStatOffset)
	}
	for _, k := range []StatKey{StatAtk, StatDef, StatSpA, StatSpD, StatSpe} {
		if v := got.Get(k); v != 100+OtherStatOffset {
			t.Errorf("%s=%d want %d", k, v, 100+OtherStatOffset)
		}
	}
}
