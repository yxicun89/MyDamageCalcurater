package balance

import (
	"errors"
	"testing"
)

type stubTypeChart map[[2]TypeID]Multiplier

func (s stubTypeChart) Matchup(attack, defense TypeID) (Multiplier, error) {
	if multiplier, ok := s[[2]TypeID{attack, defense}]; ok {
		return multiplier, nil
	}
	return MultiplierNormal, nil
}

type failingTypeChart struct {
	multiplier Multiplier
	err        error
}

func (s failingTypeChart) Matchup(TypeID, TypeID) (Multiplier, error) {
	return s.multiplier, s.err
}

func TestCalculateDefense(t *testing.T) {
	t.Parallel()

	chart := stubTypeChart{
		{TypeWater, TypeRock}:   MultiplierDouble,
		{TypeWater, TypeGround}: MultiplierDouble,
		{TypeFire, TypeWater}:   MultiplierHalf,
		{TypeFire, TypeRock}:    MultiplierHalf,
		{TypeNormal, TypeGhost}: MultiplierZero,
	}

	tests := []struct {
		name       string
		attack     TypeID
		defense    []TypeID
		multiplier Multiplier
	}{
		{name: "single", attack: TypeWater, defense: []TypeID{TypeRock}, multiplier: MultiplierDouble},
		{name: "quad", attack: TypeWater, defense: []TypeID{TypeRock, TypeGround}, multiplier: MultiplierQuad},
		{name: "quarter", attack: TypeFire, defense: []TypeID{TypeWater, TypeRock}, multiplier: MultiplierQuarter},
		{name: "immune", attack: TypeNormal, defense: []TypeID{TypeRock, TypeGhost}, multiplier: MultiplierZero},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CalculateDefense(chart, tt.attack, tt.defense)
			if err != nil {
				t.Fatalf("CalculateDefense() error = %v", err)
			}
			if got.Multiplier != tt.multiplier {
				t.Errorf("multiplier = %d, want %d", got.Multiplier, tt.multiplier)
			}
			if got.Source != EffectSourceType {
				t.Errorf("source = %d, want EffectSourceType", got.Source)
			}
		})
	}
}

func TestCalculateDefenseRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	chart := stubTypeChart{}
	tests := []struct {
		name    string
		attack  TypeID
		defense []TypeID
		wantErr error
	}{
		{name: "invalid attack", attack: TypeID("stellar"), defense: []TypeID{TypeFire}, wantErr: ErrInvalidType},
		{name: "empty defense", attack: TypeFire, wantErr: ErrDefenseTypeCount},
		{name: "too many defense types", attack: TypeFire, defense: []TypeID{TypeWater, TypeRock, TypeGround}, wantErr: ErrDefenseTypeCount},
		{name: "duplicate defense types", attack: TypeFire, defense: []TypeID{TypeWater, TypeWater}, wantErr: ErrDuplicateDefenseType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := CalculateDefense(chart, tt.attack, tt.defense)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateDefenseRejectsInvalidProviderResults(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("provider unavailable")
	tests := []struct {
		name    string
		chart   TypeChartProvider
		wantErr error
	}{
		{name: "nil provider", chart: nil, wantErr: ErrNilTypeChart},
		{name: "provider error", chart: failingTypeChart{err: providerErr}, wantErr: providerErr},
		{name: "invalid provider multiplier", chart: failingTypeChart{multiplier: MultiplierQuarter}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := CalculateDefense(tt.chart, TypeFire, []TypeID{TypeGrass})
			if err == nil {
				t.Fatal("CalculateDefense() error = nil, want non-nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}
