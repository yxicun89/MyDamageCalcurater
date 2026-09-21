// Package balance contains the pure type-balance domain model.
package balance

import "fmt"

// TypeID is the stable lowercase identifier used at the service boundary.
type TypeID string

const (
	TypeNormal   TypeID = "normal"
	TypeFire     TypeID = "fire"
	TypeWater    TypeID = "water"
	TypeElectric TypeID = "electric"
	TypeGrass    TypeID = "grass"
	TypeIce      TypeID = "ice"
	TypeFighting TypeID = "fighting"
	TypePoison   TypeID = "poison"
	TypeGround   TypeID = "ground"
	TypeFlying   TypeID = "flying"
	TypePsychic  TypeID = "psychic"
	TypeBug      TypeID = "bug"
	TypeRock     TypeID = "rock"
	TypeGhost    TypeID = "ghost"
	TypeDragon   TypeID = "dragon"
	TypeDark     TypeID = "dark"
	TypeSteel    TypeID = "steel"
	TypeFairy    TypeID = "fairy"
)

var allTypes = [...]TypeID{
	TypeNormal, TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce,
	TypeFighting, TypePoison, TypeGround, TypeFlying, TypePsychic, TypeBug,
	TypeRock, TypeGhost, TypeDragon, TypeDark, TypeSteel, TypeFairy,
}

// AllTypes returns a copy in the canonical display order.
func AllTypes() []TypeID {
	types := make([]TypeID, len(allTypes))
	copy(types, allTypes[:])
	return types
}

// Valid reports whether t is one of the supported 18 types.
func (t TypeID) Valid() bool {
	for _, candidate := range allTypes {
		if candidate == t {
			return true
		}
	}
	return false
}

// Multiplier uses 4 as the exact representation of neutral damage.
type Multiplier uint8

const (
	MultiplierZero    Multiplier = 0
	MultiplierQuarter Multiplier = 1
	MultiplierHalf    Multiplier = 2
	MultiplierNormal  Multiplier = 4
	MultiplierDouble  Multiplier = 8
	MultiplierQuad    Multiplier = 16
)

func (m Multiplier) String() string {
	switch m {
	case MultiplierZero:
		return "0"
	case MultiplierQuarter:
		return "1/4"
	case MultiplierHalf:
		return "1/2"
	case MultiplierNormal:
		return "1"
	case MultiplierDouble:
		return "2"
	case MultiplierQuad:
		return "4"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

// ValidSingleType reports whether m can result from one attack/defense matchup.
func (m Multiplier) ValidSingleType() bool {
	return m == MultiplierZero || m == MultiplierHalf || m == MultiplierNormal || m == MultiplierDouble
}

// EffectSource distinguishes type-derived effects from future ability effects.
type EffectSource uint8

const (
	EffectSourceType EffectSource = iota
	EffectSourceAbility
)

// DefenseResult is stable from TB0 onward so TB3 can add ability-derived results.
type DefenseResult struct {
	Multiplier Multiplier
	Source     EffectSource
}
