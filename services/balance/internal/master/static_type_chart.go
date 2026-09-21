// Package master provides replaceable adapters for shared master data.
package master

import (
	"fmt"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TemporaryTypeChart is a development adapter, not the permanent master.
// It mirrors the current generation 6+ table already used by damage-calc.
type TemporaryTypeChart struct {
	matchups map[[2]balance.TypeID]balance.Multiplier
}

// NewTemporaryTypeChart constructs the TB0 development type chart.
func NewTemporaryTypeChart() *TemporaryTypeChart {
	chart := &TemporaryTypeChart{matchups: make(map[[2]balance.TypeID]balance.Multiplier)}
	chart.add(balance.TypeNormal, nil, []balance.TypeID{balance.TypeRock, balance.TypeSteel}, []balance.TypeID{balance.TypeGhost})
	chart.add(balance.TypeFire, []balance.TypeID{balance.TypeGrass, balance.TypeIce, balance.TypeBug, balance.TypeSteel}, []balance.TypeID{balance.TypeFire, balance.TypeWater, balance.TypeRock, balance.TypeDragon}, nil)
	chart.add(balance.TypeWater, []balance.TypeID{balance.TypeFire, balance.TypeGround, balance.TypeRock}, []balance.TypeID{balance.TypeWater, balance.TypeGrass, balance.TypeDragon}, nil)
	chart.add(balance.TypeElectric, []balance.TypeID{balance.TypeWater, balance.TypeFlying}, []balance.TypeID{balance.TypeElectric, balance.TypeGrass, balance.TypeDragon}, []balance.TypeID{balance.TypeGround})
	chart.add(balance.TypeGrass, []balance.TypeID{balance.TypeWater, balance.TypeGround, balance.TypeRock}, []balance.TypeID{balance.TypeFire, balance.TypeGrass, balance.TypePoison, balance.TypeFlying, balance.TypeBug, balance.TypeDragon, balance.TypeSteel}, nil)
	chart.add(balance.TypeIce, []balance.TypeID{balance.TypeGrass, balance.TypeGround, balance.TypeFlying, balance.TypeDragon}, []balance.TypeID{balance.TypeFire, balance.TypeWater, balance.TypeIce, balance.TypeSteel}, nil)
	chart.add(balance.TypeFighting, []balance.TypeID{balance.TypeNormal, balance.TypeIce, balance.TypeRock, balance.TypeDark, balance.TypeSteel}, []balance.TypeID{balance.TypePoison, balance.TypeFlying, balance.TypePsychic, balance.TypeBug, balance.TypeFairy}, []balance.TypeID{balance.TypeGhost})
	chart.add(balance.TypePoison, []balance.TypeID{balance.TypeGrass, balance.TypeFairy}, []balance.TypeID{balance.TypePoison, balance.TypeGround, balance.TypeRock, balance.TypeGhost}, []balance.TypeID{balance.TypeSteel})
	chart.add(balance.TypeGround, []balance.TypeID{balance.TypeFire, balance.TypeElectric, balance.TypePoison, balance.TypeRock, balance.TypeSteel}, []balance.TypeID{balance.TypeGrass, balance.TypeBug}, []balance.TypeID{balance.TypeFlying})
	chart.add(balance.TypeFlying, []balance.TypeID{balance.TypeGrass, balance.TypeFighting, balance.TypeBug}, []balance.TypeID{balance.TypeElectric, balance.TypeRock, balance.TypeSteel}, nil)
	chart.add(balance.TypePsychic, []balance.TypeID{balance.TypeFighting, balance.TypePoison}, []balance.TypeID{balance.TypePsychic, balance.TypeSteel}, []balance.TypeID{balance.TypeDark})
	chart.add(balance.TypeBug, []balance.TypeID{balance.TypeGrass, balance.TypePsychic, balance.TypeDark}, []balance.TypeID{balance.TypeFire, balance.TypeFighting, balance.TypePoison, balance.TypeFlying, balance.TypeGhost, balance.TypeSteel, balance.TypeFairy}, nil)
	chart.add(balance.TypeRock, []balance.TypeID{balance.TypeFire, balance.TypeIce, balance.TypeFlying, balance.TypeBug}, []balance.TypeID{balance.TypeFighting, balance.TypeGround, balance.TypeSteel}, nil)
	chart.add(balance.TypeGhost, []balance.TypeID{balance.TypePsychic, balance.TypeGhost}, []balance.TypeID{balance.TypeDark}, []balance.TypeID{balance.TypeNormal})
	chart.add(balance.TypeDragon, []balance.TypeID{balance.TypeDragon}, []balance.TypeID{balance.TypeSteel}, []balance.TypeID{balance.TypeFairy})
	chart.add(balance.TypeDark, []balance.TypeID{balance.TypePsychic, balance.TypeGhost}, []balance.TypeID{balance.TypeFighting, balance.TypeDark, balance.TypeFairy}, nil)
	chart.add(balance.TypeSteel, []balance.TypeID{balance.TypeIce, balance.TypeRock, balance.TypeFairy}, []balance.TypeID{balance.TypeFire, balance.TypeWater, balance.TypeElectric, balance.TypeSteel}, nil)
	chart.add(balance.TypeFairy, []balance.TypeID{balance.TypeFighting, balance.TypeDragon, balance.TypeDark}, []balance.TypeID{balance.TypeFire, balance.TypePoison, balance.TypeSteel}, nil)
	return chart
}

func (c *TemporaryTypeChart) add(attack balance.TypeID, doubles, halves, zeroes []balance.TypeID) {
	for _, defense := range doubles {
		c.matchups[[2]balance.TypeID{attack, defense}] = balance.MultiplierDouble
	}
	for _, defense := range halves {
		c.matchups[[2]balance.TypeID{attack, defense}] = balance.MultiplierHalf
	}
	for _, defense := range zeroes {
		c.matchups[[2]balance.TypeID{attack, defense}] = balance.MultiplierZero
	}
}

// Matchup implements balance.TypeChartProvider.
func (c *TemporaryTypeChart) Matchup(attack, defense balance.TypeID) (balance.Multiplier, error) {
	if !attack.Valid() {
		return 0, fmt.Errorf("%w: attack %q", balance.ErrInvalidType, attack)
	}
	if !defense.Valid() {
		return 0, fmt.Errorf("%w: defense %q", balance.ErrInvalidType, defense)
	}
	if multiplier, ok := c.matchups[[2]balance.TypeID{attack, defense}]; ok {
		return multiplier, nil
	}
	return balance.MultiplierNormal, nil
}
