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
			got, err := CalcDamage(tt.input)
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
	in.Defender.Species.BaseStats.SpD = 81 // 実数値101 → 砂151 → チョッキ226
	in.Move.Power = 108
	in.Field.Weather = WeatherSand
	in.Defender.Item = &Item{Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
	got, err := CalcDamage(in)
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
		if _, err := CalcDamage(in); err == nil {
			t.Errorf("accepted level=%d", level)
		}
	}
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeNormal)
	in.Defender.Species.BaseStats.Def = -20
	if _, err := CalcDamage(in); err == nil {
		t.Error("accepted negative base stat")
	}
}
