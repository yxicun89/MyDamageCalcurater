package balance

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// TB3 の有理数の倍率(ADR-0017 §3)。

// ErrInvalidEffectiveness reports an Effectiveness that is not a non-negative
// fraction with a positive denominator.
var ErrInvalidEffectiveness = errors.New("invalid effectiveness")

// Effectiveness is an exact non-negative multiplier kept as an irreducible fraction
// Num/Den (Den > 0). Zero is {0, 1}. Values built by NewEffectiveness, Mul and
// Multiplier.Effectiveness are always reduced, so == compares values. Never a float.
type Effectiveness struct {
	Num int64
	Den int64
}

// gcdInt64 returns the non-negative greatest common divisor of a and b.
func gcdInt64(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// NewEffectiveness returns num/den in lowest terms. den must be positive and num
// non-negative; otherwise it returns an error wrapping ErrInvalidEffectiveness.
func NewEffectiveness(num, den int64) (Effectiveness, error) {
	if den <= 0 {
		return Effectiveness{}, fmt.Errorf("%w: denominator must be positive, got %d", ErrInvalidEffectiveness, den)
	}
	if num < 0 {
		return Effectiveness{}, fmt.Errorf("%w: numerator must be non-negative, got %d", ErrInvalidEffectiveness, num)
	}
	if num == 0 {
		return Effectiveness{Num: 0, Den: 1}, nil
	}
	g := gcdInt64(num, den)
	return Effectiveness{Num: num / g, Den: den / g}, nil
}

// Effectiveness converts the TB0 integer representation (4 = x1) to a reduced fraction:
// 0 -> 0, 1 -> 1/4, 2 -> 1/2, 4 -> 1, 8 -> 2, 16 -> 4.
func (m Multiplier) Effectiveness() Effectiveness {
	e, err := NewEffectiveness(int64(m), int64(MultiplierNormal))
	if err != nil {
		// m is a Multiplier (a non-negative uint8) and MultiplierNormal is positive,
		// so NewEffectiveness never rejects this input.
		panic(fmt.Sprintf("Multiplier(%d).Effectiveness(): %v", m, err))
	}
	return e
}

// ErrEffectivenessOverflow reports a product that does not fit in int64. Only data that
// stacks many multipliers (read model / provider) can reach it; it is never silently wrapped.
var ErrEffectivenessOverflow = errors.New("effectiveness overflow")

// Mul returns the reduced product e × o. Factors are cross-reduced before multiplying,
// and a product that would not fit in int64 returns ErrEffectivenessOverflow.
func (e Effectiveness) Mul(o Effectiveness) (Effectiveness, error) {
	if e.Den <= 0 || o.Den <= 0 || e.Num < 0 || o.Num < 0 {
		return Effectiveness{}, fmt.Errorf("%w: %+v × %+v", ErrInvalidEffectiveness, e, o)
	}
	if e.Num == 0 || o.Num == 0 {
		return Effectiveness{Num: 0, Den: 1}, nil
	}
	g1 := gcdInt64(e.Num, o.Den)
	g2 := gcdInt64(o.Num, e.Den)
	num, okNum := mulNonNegative(e.Num/g1, o.Num/g2)
	den, okDen := mulNonNegative(e.Den/g2, o.Den/g1)
	if !okNum || !okDen {
		return Effectiveness{}, fmt.Errorf("%w: %s × %s", ErrEffectivenessOverflow, e, o)
	}
	// Both inputs are reduced, so after cross-reduction the product is already in lowest terms.
	return Effectiveness{Num: num, Den: den}, nil
}

// mulNonNegative multiplies two non-negative int64 values, reporting whether the product fits.
func mulNonNegative(a, b int64) (int64, bool) {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	if hi != 0 || lo > math.MaxInt64 {
		return 0, false
	}
	return int64(lo), true
}

// Cmp compares e and o by value: -1 if e < o, 0 if equal, +1 if e > o. The cross
// products are compared in 128 bits, so it never overflows. Both values must be valid
// (non-negative numerator, positive denominator).
func (e Effectiveness) Cmp(o Effectiveness) int {
	leftHi, leftLo := bits.Mul64(uint64(e.Num), uint64(o.Den))
	rightHi, rightLo := bits.Mul64(uint64(o.Num), uint64(e.Den))
	switch {
	case leftHi < rightHi || (leftHi == rightHi && leftLo < rightLo):
		return -1
	case leftHi > rightHi || (leftHi == rightHi && leftLo > rightLo):
		return 1
	default:
		return 0
	}
}

// IsZero reports whether the value is x0.
func (e Effectiveness) IsZero() bool {
	return e.Num == 0
}

// String is the API form (DefenseMultiplier): "Num/Den", or "Num" alone when Den is 1
// ("0", "1", "3", "1/2", "3/4", "5/4").
func (e Effectiveness) String() string {
	if e.Den == 1 {
		return fmt.Sprintf("%d", e.Num)
	}
	return fmt.Sprintf("%d/%d", e.Num, e.Den)
}

// ClassifyEffectiveness maps a value to the six categories by range (ADR-0017 §3):
// 0 -> immune, 0 < x <= 1/4 -> quad_resist, 1/4 < x < 1 -> resist, 1 -> neutral,
// 1 < x < 4 -> weak, x >= 4 -> quad_weak. A value with Den <= 0 or Num < 0 returns
// an error wrapping ErrInvalidEffectiveness.
func ClassifyEffectiveness(e Effectiveness) (Category, error) {
	if e.Den <= 0 || e.Num < 0 {
		return "", fmt.Errorf("%w: %+v", ErrInvalidEffectiveness, e)
	}

	quarter := Effectiveness{Num: 1, Den: 4}
	one := Effectiveness{Num: 1, Den: 1}
	four := Effectiveness{Num: 4, Den: 1}

	switch {
	case e.Num == 0:
		return CategoryImmune, nil
	case e.Cmp(quarter) <= 0:
		return CategoryQuadResist, nil
	case e.Cmp(one) < 0:
		return CategoryResist, nil
	case e.Cmp(one) == 0:
		return CategoryNeutral, nil
	case e.Cmp(four) < 0:
		return CategoryWeak, nil
	default:
		return CategoryQuadWeak, nil
	}
}
