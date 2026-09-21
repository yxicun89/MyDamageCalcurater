package balance

import "errors"

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

// NewEffectiveness returns num/den in lowest terms. den must be positive and num
// non-negative; otherwise it returns an error wrapping ErrInvalidEffectiveness.
func NewEffectiveness(num, den int64) (Effectiveness, error) {
	// TODO(TB3): implement (ADR-0017 §3).
	return Effectiveness{}, nil
}

// Effectiveness converts the TB0 integer representation (4 = x1) to a reduced fraction:
// 0 -> 0, 1 -> 1/4, 2 -> 1/2, 4 -> 1, 8 -> 2, 16 -> 4.
func (m Multiplier) Effectiveness() Effectiveness {
	// TODO(TB3): implement.
	return Effectiveness{}
}

// Mul returns the reduced product e × o.
func (e Effectiveness) Mul(o Effectiveness) Effectiveness {
	// TODO(TB3): implement.
	return Effectiveness{}
}

// Cmp compares e and o by value: -1 if e < o, 0 if equal, +1 if e > o.
func (e Effectiveness) Cmp(o Effectiveness) int {
	// TODO(TB3): implement.
	return 0
}

// IsZero reports whether the value is x0.
func (e Effectiveness) IsZero() bool {
	// TODO(TB3): implement.
	return false
}

// String is the API form (DefenseMultiplier): "Num/Den", or "Num" alone when Den is 1
// ("0", "1", "3", "1/2", "3/4", "5/4").
func (e Effectiveness) String() string {
	// TODO(TB3): implement.
	return ""
}

// ClassifyEffectiveness maps a value to the six categories by range (ADR-0017 §3):
// 0 -> immune, 0 < x <= 1/4 -> quad_resist, 1/4 < x < 1 -> resist, 1 -> neutral,
// 1 < x < 4 -> weak, x >= 4 -> quad_weak. A value with Den <= 0 or Num < 0 returns
// an error wrapping ErrInvalidEffectiveness.
func ClassifyEffectiveness(e Effectiveness) (Category, error) {
	// TODO(TB3): implement.
	return "", nil
}
