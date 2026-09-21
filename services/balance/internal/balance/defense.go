package balance

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidType          = errors.New("invalid type")
	ErrDefenseTypeCount     = errors.New("defense type count must be one or two")
	ErrDuplicateDefenseType = errors.New("defense types must be distinct")
	ErrNilTypeChart         = errors.New("type chart is required")
)

// TypeChartProvider is the replacement boundary for the shared master data.
// TB0 wires a temporary static adapter; the domain package does not own it.
type TypeChartProvider interface {
	Matchup(attack, defense TypeID) (Multiplier, error)
}

// CalculateDefense combines one or two defensive types without floating point.
func CalculateDefense(chart TypeChartProvider, attack TypeID, defenseTypes []TypeID) (DefenseResult, error) {
	if chart == nil {
		return DefenseResult{}, ErrNilTypeChart
	}
	if !attack.Valid() {
		return DefenseResult{}, fmt.Errorf("%w: attack %q", ErrInvalidType, attack)
	}
	if len(defenseTypes) < 1 || len(defenseTypes) > 2 {
		return DefenseResult{}, ErrDefenseTypeCount
	}
	if len(defenseTypes) == 2 && defenseTypes[0] == defenseTypes[1] {
		return DefenseResult{}, ErrDuplicateDefenseType
	}

	combined := MultiplierNormal
	for _, defense := range defenseTypes {
		if !defense.Valid() {
			return DefenseResult{}, fmt.Errorf("%w: defense %q", ErrInvalidType, defense)
		}
		matchup, err := chart.Matchup(attack, defense)
		if err != nil {
			return DefenseResult{}, fmt.Errorf("type chart matchup %s/%s: %w", attack, defense, err)
		}
		if !matchup.ValidSingleType() {
			return DefenseResult{}, fmt.Errorf("type chart matchup %s/%s returned invalid multiplier %d", attack, defense, matchup)
		}
		combined = Multiplier(uint16(combined) * uint16(matchup) / uint16(MultiplierNormal))
	}

	return DefenseResult{
		Multiplier:    combined,
		Source:        EffectSourceType,
		Effectiveness: combined.Effectiveness(),
		Effect:        DefenseEffectNone,
	}, nil
}
