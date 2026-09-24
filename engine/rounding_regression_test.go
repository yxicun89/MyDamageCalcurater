package engine

import "testing"

// 期待値はローカル @smogon/calc 0.10.0/gen9 の統制ケースから取得。
// A/C=200、B/D=100、HP=175、Lv50、IV31、EV0、Serious、威力100。
func TestModifierRoundingRegression(t *testing.T) {
	terrain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
	terrain.Field.Terrain = TerrainElectric
	tests := []struct {
		name        string
		input       DamageInput
		index, want int
	}{
		{"STAB before effectiveness", ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeWater), 1, 230},
		{"terrain power", terrain, 15, 116},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calcDamage(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls[tt.index] != tt.want {
				t.Fatalf("roll[%d]=%d want %d", tt.index, got.Rolls[tt.index], tt.want)
			}
		})
	}
}

func TestWeatherBeforeItemRounding(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategorySpecial, TypePsychic)
	in.Defender.Species.BaseStats.SpD = 81 // 実数値101 → 砂151 → 特防1.5倍の持ち物226
	in.Move.Power = 108
	in.Field.Weather = WeatherSand
	in.Defender.Item = &Item{Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxDamage() != 44 {
		t.Fatalf("max=%d want 44", got.MaxDamage())
	}
}

func TestCalcDamageRejectsInvalidLevelAndStats(t *testing.T) {
	for _, level := range []int{-1, 1, 49, 51, 100} {
		in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeNormal)
		in.Attacker.Level = level
		if _, err := calcDamage(in); err == nil {
			t.Errorf("accepted level=%d", level)
		}
	}
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeNormal)
	in.Defender.Species.BaseStats.Def = -20
	if _, err := calcDamage(in); err == nil {
		t.Error("accepted negative base stat")
	}
}

// TestChainModsRoundHalfUp は補正の連結の各ステップが (M*mod + 2048) >> 12(0.5 を切り上げる丸め)で
// あることを固定する(@smogon/calc の chainMods と同じ。issue #303)。切り捨てに変わると golden が無くても落ちる。
func TestChainModsRoundHalfUp(t *testing.T) {
	tests := []struct {
		name string
		mods []int
		want int
	}{
		{"empty is identity", nil, Modifier4096},
		{"4096 is skipped", []int{Modifier4096, 5324}, 5324},
		// 4915×5324/4096 = 6388.54… → 6389(切り捨てなら 6388)
		{"fraction above half rounds up", []int{4915, 5324}, 6389},
		// 2048×6145/4096 = 3072.5 ちょうど → 3073(連結は半分ちょうども切り上げる。pokeRound とは逆)
		{"exact half rounds up", []int{ModifierHalf, 6145}, 3073},
		// 5324×5324/4096 = 6920.16… → 6920
		{"fraction below half rounds down", []int{5324, 5324}, 6920},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chainMods(tt.mods); got != tt.want {
				t.Fatalf("chainMods(%v)=%d want %d", tt.mods, got, tt.want)
			}
		})
	}
}
