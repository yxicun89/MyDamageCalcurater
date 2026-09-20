package engine

import "testing"

func TestTypeEffectivenessSingle(t *testing.T) {
	tests := []struct {
		atk  Type
		def  []Type
		want float64
	}{
		{TypeFire, []Type{TypeGrass}, 2.0},
		{TypeFire, []Type{TypeWater}, 0.5},
		{TypeNormal, []Type{TypeGhost}, 0.0},
		{TypeGround, []Type{TypeFlying}, 0.0},
		{TypeElectric, []Type{TypeGround}, 0.0},
		{TypeDragon, []Type{TypeFairy}, 0.0},
		{TypeWater, []Type{TypeFire}, 2.0},
		{TypeNormal, []Type{TypePsychic}, 1.0},
	}
	for _, tt := range tests {
		_, _, got := TypeEffectiveness(tt.atk, tt.def)
		if got != tt.want {
			t.Errorf("%s vs %v = %v want %v", tt.atk, tt.def, got, tt.want)
		}
	}
}

func TestTypeEffectivenessDual(t *testing.T) {
	tests := []struct {
		atk  Type
		def  []Type
		want float64
	}{
		{TypeWater, []Type{TypeRock, TypeGround}, 4.0}, // ゴローニャ: 水×2×2
		{TypeFire, []Type{TypeWater, TypeRock}, 0.25},  // 火 vs 水0.5×岩0.5
		{TypeGrass, []Type{TypeWater, TypeGround}, 4.0},
		{TypeIce, []Type{TypeDragon, TypeGround}, 4.0}, // カイリュー系: 氷×2×2
		{TypeNormal, []Type{TypeRock, TypeGhost}, 0.0}, // ゴースト無効が優先
		{TypeFighting, []Type{TypeNormal, TypeFlying}, 1.0},
	}
	for _, tt := range tests {
		_, _, got := TypeEffectiveness(tt.atk, tt.def)
		if got != tt.want {
			t.Errorf("%s vs %v = %v want %v", tt.atk, tt.def, got, tt.want)
		}
	}
}
