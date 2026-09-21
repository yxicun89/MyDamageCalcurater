package master

import (
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

func TestTemporaryTypeChartCoversAllMatchups(t *testing.T) {
	t.Parallel()

	chart := NewTemporaryTypeChart()
	for _, attack := range balance.AllTypes() {
		for _, defense := range balance.AllTypes() {
			multiplier, err := chart.Matchup(attack, defense)
			if err != nil {
				t.Errorf("Matchup(%s, %s) error = %v", attack, defense, err)
			}
			if !multiplier.ValidSingleType() {
				t.Errorf("Matchup(%s, %s) = %d, want 0, 1/2, 1, or 2", attack, defense, multiplier)
			}
		}
	}
}

func TestTemporaryTypeChartRepresentativeMatchups(t *testing.T) {
	t.Parallel()

	chart := NewTemporaryTypeChart()
	tests := []struct {
		attack  balance.TypeID
		defense balance.TypeID
		want    balance.Multiplier
	}{
		{balance.TypeFire, balance.TypeGrass, balance.MultiplierDouble},
		{balance.TypeFire, balance.TypeWater, balance.MultiplierHalf},
		{balance.TypeNormal, balance.TypeGhost, balance.MultiplierZero},
		{balance.TypeElectric, balance.TypeGround, balance.MultiplierZero},
		{balance.TypeDragon, balance.TypeFairy, balance.MultiplierZero},
		{balance.TypeNormal, balance.TypePsychic, balance.MultiplierNormal},
	}
	for _, tt := range tests {
		got, err := chart.Matchup(tt.attack, tt.defense)
		if err != nil {
			t.Fatalf("Matchup(%s, %s) error = %v", tt.attack, tt.defense, err)
		}
		if got != tt.want {
			t.Errorf("Matchup(%s, %s) = %d, want %d", tt.attack, tt.defense, got, tt.want)
		}
	}
}

func TestTemporaryTypeChartExactNonNeutralRelations(t *testing.T) {
	t.Parallel()

	type relation struct {
		attack  balance.TypeID
		doubles []balance.TypeID
		halves  []balance.TypeID
		zeroes  []balance.TypeID
	}
	relations := []relation{
		{balance.TypeNormal, nil, []balance.TypeID{balance.TypeRock, balance.TypeSteel}, []balance.TypeID{balance.TypeGhost}},
		{balance.TypeFire, []balance.TypeID{balance.TypeGrass, balance.TypeIce, balance.TypeBug, balance.TypeSteel}, []balance.TypeID{balance.TypeFire, balance.TypeWater, balance.TypeRock, balance.TypeDragon}, nil},
		{balance.TypeWater, []balance.TypeID{balance.TypeFire, balance.TypeGround, balance.TypeRock}, []balance.TypeID{balance.TypeWater, balance.TypeGrass, balance.TypeDragon}, nil},
		{balance.TypeElectric, []balance.TypeID{balance.TypeWater, balance.TypeFlying}, []balance.TypeID{balance.TypeElectric, balance.TypeGrass, balance.TypeDragon}, []balance.TypeID{balance.TypeGround}},
		{balance.TypeGrass, []balance.TypeID{balance.TypeWater, balance.TypeGround, balance.TypeRock}, []balance.TypeID{balance.TypeFire, balance.TypeGrass, balance.TypePoison, balance.TypeFlying, balance.TypeBug, balance.TypeDragon, balance.TypeSteel}, nil},
		{balance.TypeIce, []balance.TypeID{balance.TypeGrass, balance.TypeGround, balance.TypeFlying, balance.TypeDragon}, []balance.TypeID{balance.TypeFire, balance.TypeWater, balance.TypeIce, balance.TypeSteel}, nil},
		{balance.TypeFighting, []balance.TypeID{balance.TypeNormal, balance.TypeIce, balance.TypeRock, balance.TypeDark, balance.TypeSteel}, []balance.TypeID{balance.TypePoison, balance.TypeFlying, balance.TypePsychic, balance.TypeBug, balance.TypeFairy}, []balance.TypeID{balance.TypeGhost}},
		{balance.TypePoison, []balance.TypeID{balance.TypeGrass, balance.TypeFairy}, []balance.TypeID{balance.TypePoison, balance.TypeGround, balance.TypeRock, balance.TypeGhost}, []balance.TypeID{balance.TypeSteel}},
		{balance.TypeGround, []balance.TypeID{balance.TypeFire, balance.TypeElectric, balance.TypePoison, balance.TypeRock, balance.TypeSteel}, []balance.TypeID{balance.TypeGrass, balance.TypeBug}, []balance.TypeID{balance.TypeFlying}},
		{balance.TypeFlying, []balance.TypeID{balance.TypeGrass, balance.TypeFighting, balance.TypeBug}, []balance.TypeID{balance.TypeElectric, balance.TypeRock, balance.TypeSteel}, nil},
		{balance.TypePsychic, []balance.TypeID{balance.TypeFighting, balance.TypePoison}, []balance.TypeID{balance.TypePsychic, balance.TypeSteel}, []balance.TypeID{balance.TypeDark}},
		{balance.TypeBug, []balance.TypeID{balance.TypeGrass, balance.TypePsychic, balance.TypeDark}, []balance.TypeID{balance.TypeFire, balance.TypeFighting, balance.TypePoison, balance.TypeFlying, balance.TypeGhost, balance.TypeSteel, balance.TypeFairy}, nil},
		{balance.TypeRock, []balance.TypeID{balance.TypeFire, balance.TypeIce, balance.TypeFlying, balance.TypeBug}, []balance.TypeID{balance.TypeFighting, balance.TypeGround, balance.TypeSteel}, nil},
		{balance.TypeGhost, []balance.TypeID{balance.TypePsychic, balance.TypeGhost}, []balance.TypeID{balance.TypeDark}, []balance.TypeID{balance.TypeNormal}},
		{balance.TypeDragon, []balance.TypeID{balance.TypeDragon}, []balance.TypeID{balance.TypeSteel}, []balance.TypeID{balance.TypeFairy}},
		{balance.TypeDark, []balance.TypeID{balance.TypePsychic, balance.TypeGhost}, []balance.TypeID{balance.TypeFighting, balance.TypeDark, balance.TypeFairy}, nil},
		{balance.TypeSteel, []balance.TypeID{balance.TypeIce, balance.TypeRock, balance.TypeFairy}, []balance.TypeID{balance.TypeFire, balance.TypeWater, balance.TypeElectric, balance.TypeSteel}, nil},
		{balance.TypeFairy, []balance.TypeID{balance.TypeFighting, balance.TypeDragon, balance.TypeDark}, []balance.TypeID{balance.TypeFire, balance.TypePoison, balance.TypeSteel}, nil},
	}

	want := make(map[[2]balance.TypeID]balance.Multiplier)
	for _, relation := range relations {
		for _, defense := range relation.doubles {
			want[[2]balance.TypeID{relation.attack, defense}] = balance.MultiplierDouble
		}
		for _, defense := range relation.halves {
			want[[2]balance.TypeID{relation.attack, defense}] = balance.MultiplierHalf
		}
		for _, defense := range relation.zeroes {
			want[[2]balance.TypeID{relation.attack, defense}] = balance.MultiplierZero
		}
	}

	chart := NewTemporaryTypeChart()
	for _, attack := range balance.AllTypes() {
		for _, defense := range balance.AllTypes() {
			expected := balance.MultiplierNormal
			if nonNeutral, ok := want[[2]balance.TypeID{attack, defense}]; ok {
				expected = nonNeutral
			}
			got, err := chart.Matchup(attack, defense)
			if err != nil {
				t.Fatalf("Matchup(%s, %s) error = %v", attack, defense, err)
			}
			if got != expected {
				t.Errorf("Matchup(%s, %s) = %d, want %d", attack, defense, got, expected)
			}
		}
	}
}

func TestTemporaryTypeChartRejectsUnknownTypes(t *testing.T) {
	t.Parallel()

	chart := NewTemporaryTypeChart()
	if _, err := chart.Matchup(balance.TypeID("stellar"), balance.TypeFire); err == nil {
		t.Fatal("unknown attack type must return an error")
	}
	if _, err := chart.Matchup(balance.TypeFire, balance.TypeID("stellar")); err == nil {
		t.Fatal("unknown defense type must return an error")
	}
}
