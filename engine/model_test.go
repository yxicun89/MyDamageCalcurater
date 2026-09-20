package engine

import "testing"

func sampleSpecies() Species {
	return Species{
		Key:       "0445-000",
		DexNo:     445,
		NameJa:    "ガブリアス",
		Types:     []Type{TypeDragon, TypeGround},
		BaseStats: Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102},
	}
}

func TestStatsGetWithStat(t *testing.T) {
	s := Stats{HP: 1, Atk: 2, Def: 3, SpA: 4, SpD: 5, Spe: 6}
	for k, want := range map[StatKey]int{
		StatHP: 1, StatAtk: 2, StatDef: 3, StatSpA: 4, StatSpD: 5, StatSpe: 6,
	} {
		if got := s.Get(k); got != want {
			t.Errorf("Get(%s)=%d want %d", k, got, want)
		}
	}
	s2 := s.WithStat(StatSpe, 99)
	if s2.Spe != 99 {
		t.Errorf("WithStat did not set Spe: %d", s2.Spe)
	}
	if s.Spe != 6 {
		t.Errorf("WithStat mutated original: %d", s.Spe)
	}
	if s.Sum() != 21 {
		t.Errorf("Sum=%d want 21", s.Sum())
	}
}

func TestNatureMultiplier(t *testing.T) {
	n := Nature{Plus: StatAtk, Minus: StatSpA}
	if got := n.Multiplier(StatAtk); got != 1.1 {
		t.Errorf("plus mult=%v want 1.1", got)
	}
	if got := n.Multiplier(StatSpA); got != 0.9 {
		t.Errorf("minus mult=%v want 0.9", got)
	}
	if got := n.Multiplier(StatDef); got != 1.0 {
		t.Errorf("neutral stat mult=%v want 1.0", got)
	}
	if got := n.Multiplier(StatHP); got != 1.0 {
		t.Errorf("hp mult=%v want 1.0", got)
	}
	// 無補正性格はすべて等倍
	if !NatureNeutral.IsNeutral() {
		t.Error("NatureNeutral should be neutral")
	}
	if got := NatureNeutral.Multiplier(StatAtk); got != 1.0 {
		t.Errorf("neutral nature mult=%v want 1.0", got)
	}
	// Plus==Minus は打ち消し合って等倍
	if !(Nature{Plus: StatAtk, Minus: StatAtk}).IsNeutral() {
		t.Error("Plus==Minus should be neutral")
	}
}

func TestIndividualValidate(t *testing.T) {
	base := func() Individual {
		return Individual{Species: sampleSpecies(), Nature: NatureNeutral}
	}
	tests := []struct {
		name    string
		mut     func(*Individual)
		wantErr bool
	}{
		{"ok", func(i *Individual) {}, false},
		{"sp max per stat ok", func(i *Individual) { i.SP = Stats{Atk: 32, Spe: 32} }, false},
		{"sp over per stat", func(i *Individual) { i.SP = Stats{Atk: 33} }, true},
		{"sp negative", func(i *Individual) { i.SP = Stats{Atk: -1} }, true},
		{"sp sum ok 66", func(i *Individual) { i.SP = Stats{HP: 32, Atk: 32, Def: 2} }, false},
		{"sp sum over 66", func(i *Individual) { i.SP = Stats{HP: 32, Atk: 32, Def: 3} }, true},
		{"rank ok", func(i *Individual) { i.Ranks = Ranks{Atk: 6, Def: -6} }, false},
		{"rank over", func(i *Individual) { i.Ranks = Ranks{Atk: 7} }, true},
		{"rank under", func(i *Individual) { i.Ranks = Ranks{SpD: -7} }, true},
		{"nature hp plus", func(i *Individual) { i.Nature = Nature{Plus: StatHP, Minus: StatAtk} }, true},
		{"no types", func(i *Individual) { i.Species.Types = nil }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base()
			tt.mut(&in)
			err := in.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatConstants(t *testing.T) {
	// 要件: engine の入力には最初から format を持たせる。
	// 定数が定義され、区別できることを確認する(入力への組込みは計算入力を定義する P1-3)。
	if FormatSingle == FormatDouble {
		t.Fatal("single と double が同値")
	}
	if FormatSingle != "single" || FormatDouble != "double" {
		t.Errorf("format 値が不正: %q %q", FormatSingle, FormatDouble)
	}
}

func TestEffectiveLevel(t *testing.T) {
	if (Individual{}).EffectiveLevel() != DefaultLevel {
		t.Error("zero level should default to 50")
	}
	if (Individual{Level: 100}).EffectiveLevel() != 100 {
		t.Error("explicit level should be kept")
	}
}
