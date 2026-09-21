package balance

import (
	"errors"
	"testing"
)

// TB3 の有理数の倍率(ADR-0017 §3)。float を使わず、既約分数 Num/Den で持つ。

func eff(num, den int64) Effectiveness { return Effectiveness{Num: num, Den: den} }

func TestNewEffectivenessReduces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		num, den int64
		want     Effectiveness
	}{
		{0, 1, eff(0, 1)},
		{0, 5, eff(0, 1)},
		{0, 16, eff(0, 1)},
		{1, 1, eff(1, 1)},
		{4, 4, eff(1, 1)},
		{16, 16, eff(1, 1)},
		{2, 1, eff(2, 1)},
		{8, 2, eff(4, 1)},
		{1, 2, eff(1, 2)},
		{2, 4, eff(1, 2)},
		{6, 8, eff(3, 4)},
		{3, 4, eff(3, 4)},
		{10, 8, eff(5, 4)},
		{15, 10, eff(3, 2)},
		{1, 16, eff(1, 16)},
		{16, 1, eff(16, 1)},
	}
	for _, tt := range tests {
		got, err := NewEffectiveness(tt.num, tt.den)
		if err != nil {
			t.Errorf("NewEffectiveness(%d, %d) error = %v", tt.num, tt.den, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NewEffectiveness(%d, %d) = %+v, want %+v (lowest terms)", tt.num, tt.den, got, tt.want)
		}
	}
}

func TestNewEffectivenessRejectsInvalidFractions(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ num, den int64 }{{1, 0}, {0, 0}, {1, -2}, {-1, 2}, {-1, -2}} {
		if _, err := NewEffectiveness(tt.num, tt.den); !errors.Is(err, ErrInvalidEffectiveness) {
			t.Errorf("NewEffectiveness(%d, %d) error = %v, want ErrInvalidEffectiveness", tt.num, tt.den, err)
		}
	}
}

func TestMultiplierEffectiveness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		multiplier Multiplier
		want       Effectiveness
	}{
		{MultiplierZero, eff(0, 1)},
		{MultiplierQuarter, eff(1, 4)},
		{MultiplierHalf, eff(1, 2)},
		{MultiplierNormal, eff(1, 1)},
		{MultiplierDouble, eff(2, 1)},
		{MultiplierQuad, eff(4, 1)},
	}
	for _, tt := range tests {
		if got := tt.multiplier.Effectiveness(); got != tt.want {
			t.Errorf("Multiplier(%d).Effectiveness() = %+v, want %+v", tt.multiplier, got, tt.want)
		}
		// The TB1 labels are a subset of the fraction labels (ADR-0017 §3).
		if got := tt.multiplier.Effectiveness().String(); got != tt.multiplier.String() {
			t.Errorf("Multiplier(%d).Effectiveness().String() = %q, want the TB1 label %q", tt.multiplier, got, tt.multiplier.String())
		}
	}
}

func TestEffectivenessMul(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b, want Effectiveness
	}{
		{eff(2, 1), eff(1, 2), eff(1, 1)},
		{eff(2, 1), eff(3, 4), eff(3, 2)},
		{eff(4, 1), eff(3, 4), eff(3, 1)},
		{eff(1, 2), eff(5, 4), eff(5, 8)},
		{eff(2, 1), eff(5, 4), eff(5, 2)},
		{eff(1, 4), eff(5, 4), eff(5, 16)},
		{eff(1, 2), eff(1, 2), eff(1, 4)},
		{eff(0, 1), eff(5, 4), eff(0, 1)},
		{eff(5, 4), eff(0, 1), eff(0, 1)},
		{eff(1, 1), eff(1, 1), eff(1, 1)},
		{eff(4, 1), eff(16, 1), eff(64, 1)},
		{eff(1, 4), eff(1, 16), eff(1, 64)},
	}
	for _, tt := range tests {
		if got, err := tt.a.Mul(tt.b); err != nil || got != tt.want {
			t.Errorf("%+v.Mul(%+v) = %+v, %v; want %+v", tt.a, tt.b, got, err, tt.want)
		}
		if got, err := tt.b.Mul(tt.a); err != nil || got != tt.want {
			t.Errorf("%+v.Mul(%+v) = %+v, %v; want %+v (commutative)", tt.b, tt.a, got, err, tt.want)
		}
	}
}

func TestEffectivenessCmpAndIsZero(t *testing.T) {
	t.Parallel()

	ordered := []Effectiveness{eff(0, 1), eff(1, 16), eff(3, 16), eff(1, 4), eff(5, 16), eff(1, 2), eff(3, 4), eff(1, 1), eff(5, 4), eff(3, 2), eff(2, 1), eff(5, 2), eff(3, 1), eff(4, 1), eff(5, 1), eff(8, 1)}
	for i, a := range ordered {
		for j, b := range ordered {
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := a.Cmp(b); got != want {
				t.Errorf("%+v.Cmp(%+v) = %d, want %d", a, b, got, want)
			}
		}
		if got, want := a.IsZero(), i == 0; got != want {
			t.Errorf("%+v.IsZero() = %v, want %v", a, got, want)
		}
	}
}

func TestEffectivenessString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value Effectiveness
		want  string
	}{
		{eff(0, 1), "0"},
		{eff(1, 1), "1"},
		{eff(2, 1), "2"},
		{eff(3, 1), "3"},
		{eff(4, 1), "4"},
		{eff(16, 1), "16"},
		{eff(1, 4), "1/4"},
		{eff(1, 2), "1/2"},
		{eff(3, 4), "3/4"},
		{eff(5, 4), "5/4"},
		{eff(3, 2), "3/2"},
		{eff(5, 2), "5/2"},
		{eff(5, 8), "5/8"},
		{eff(3, 16), "3/16"},
	}
	for _, tt := range tests {
		if got := tt.value.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.value, got, tt.want)
		}
	}
}

// TestClassifyEffectivenessBoundaries checks the ADR-0017 §3 ranges:
// 0 immune, (0, 1/4] quad_resist, (1/4, 1) resist, 1 neutral, (1, 4) weak, [4, ∞) quad_weak.
func TestClassifyEffectivenessBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value Effectiveness
		want  Category
	}{
		{eff(0, 1), CategoryImmune},
		{eff(1, 64), CategoryQuadResist},
		{eff(1, 16), CategoryQuadResist},
		{eff(3, 16), CategoryQuadResist},
		{eff(1, 4), CategoryQuadResist},
		{eff(5, 16), CategoryResist},
		{eff(3, 8), CategoryResist},
		{eff(1, 2), CategoryResist},
		{eff(5, 8), CategoryResist},
		{eff(3, 4), CategoryResist},
		{eff(15, 16), CategoryResist},
		{eff(1, 1), CategoryNeutral},
		{eff(17, 16), CategoryWeak},
		{eff(5, 4), CategoryWeak},
		{eff(3, 2), CategoryWeak},
		{eff(2, 1), CategoryWeak},
		{eff(5, 2), CategoryWeak},
		{eff(3, 1), CategoryWeak},
		{eff(15, 4), CategoryWeak},
		{eff(63, 16), CategoryWeak},
		{eff(4, 1), CategoryQuadWeak},
		{eff(5, 1), CategoryQuadWeak},
		{eff(8, 1), CategoryQuadWeak},
		{eff(16, 1), CategoryQuadWeak},
	}
	for _, tt := range tests {
		got, err := ClassifyEffectiveness(tt.value)
		if err != nil {
			t.Errorf("ClassifyEffectiveness(%s) error = %v", tt.value, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ClassifyEffectiveness(%s) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

// TestClassifyEffectivenessAgreesWithClassifyMultiplier keeps the TB1 six values unchanged.
func TestClassifyEffectivenessAgreesWithClassifyMultiplier(t *testing.T) {
	t.Parallel()

	for _, m := range []Multiplier{MultiplierZero, MultiplierQuarter, MultiplierHalf, MultiplierNormal, MultiplierDouble, MultiplierQuad} {
		want, err := ClassifyMultiplier(m)
		if err != nil {
			t.Fatalf("ClassifyMultiplier(%d) error = %v", m, err)
		}
		got, err := ClassifyEffectiveness(m.Effectiveness())
		if err != nil || got != want {
			t.Errorf("ClassifyEffectiveness(%s) = %q, %v, want %q", m, got, err, want)
		}
	}
}

func TestClassifyEffectivenessRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, value := range []Effectiveness{{}, eff(1, 0), eff(1, -4), eff(-1, 4)} {
		if _, err := ClassifyEffectiveness(value); !errors.Is(err, ErrInvalidEffectiveness) {
			t.Errorf("ClassifyEffectiveness(%+v) error = %v, want ErrInvalidEffectiveness", value, err)
		}
	}
}

func TestEffectivenessMulOverflowBoundary(t *testing.T) {
	t.Parallel()

	// 2^62 × 1 fits; 2^62 × 2 = 2^63 does not fit in int64.
	big := Effectiveness{Num: 1 << 62, Den: 1}
	if got, err := big.Mul(Effectiveness{Num: 1, Den: 1}); err != nil || got != big {
		t.Fatalf("2^62 × 1 = %+v, %v; want 2^62", got, err)
	}
	if _, err := big.Mul(Effectiveness{Num: 2, Den: 1}); !errors.Is(err, ErrEffectivenessOverflow) {
		t.Errorf("2^62 × 2: err = %v, want ErrEffectivenessOverflow", err)
	}
	// Cross-reduction keeps a product in range when it can be reduced: 2^62 × 2/4 = 2^61.
	if got, err := big.Mul(Effectiveness{Num: 1, Den: 2}); err != nil || got != (Effectiveness{Num: 1 << 61, Den: 1}) {
		t.Errorf("2^62 × 1/2 = %+v, %v; want 2^61", got, err)
	}
}

func TestEffectivenessCmpDoesNotOverflow(t *testing.T) {
	t.Parallel()

	huge := Effectiveness{Num: 1 << 62, Den: 1}
	quarter := Effectiveness{Num: 1, Den: 4}
	if huge.Cmp(quarter) != 1 || quarter.Cmp(huge) != -1 {
		t.Errorf("Cmp(2^62, 1/4) must be 1 and the reverse -1")
	}
	if got, err := ClassifyEffectiveness(huge); err != nil || got != CategoryQuadWeak {
		t.Errorf("ClassifyEffectiveness(2^62) = %q, %v; want quad_weak", got, err)
	}
}

func TestEffectivenessMulReducesUnreducedInputs(t *testing.T) {
	t.Parallel()

	if got, err := eff(1, 1).Mul(eff(2, 4)); err != nil || got != eff(1, 2) {
		t.Errorf("1 × 2/4 = %+v, %v; want 1/2", got, err)
	}
	if got, err := eff(3, 6).Mul(eff(4, 2)); err != nil || got != eff(1, 1) {
		t.Errorf("3/6 × 4/2 = %+v, %v; want 1", got, err)
	}
}
