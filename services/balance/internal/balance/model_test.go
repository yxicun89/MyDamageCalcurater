package balance

import "testing"

func TestTypeIDsAreCompleteAndValid(t *testing.T) {
	t.Parallel()

	if len(AllTypes()) != 18 {
		t.Fatalf("AllTypes length = %d, want 18", len(AllTypes()))
	}

	seen := make(map[TypeID]bool, 18)
	for _, typeID := range AllTypes() {
		if !typeID.Valid() {
			t.Errorf("%q must be valid", typeID)
		}
		if seen[typeID] {
			t.Errorf("duplicate type %q", typeID)
		}
		seen[typeID] = true
	}

	if TypeID("stellar").Valid() {
		t.Error("unsupported type stellar must be invalid")
	}
}

func TestMultiplierLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		multiplier Multiplier
		want       string
	}{
		{MultiplierZero, "0"},
		{MultiplierQuarter, "1/4"},
		{MultiplierHalf, "1/2"},
		{MultiplierNormal, "1"},
		{MultiplierDouble, "2"},
		{MultiplierQuad, "4"},
	}
	for _, tt := range tests {
		if got := tt.multiplier.String(); got != tt.want {
			t.Errorf("Multiplier(%d).String() = %q, want %q", tt.multiplier, got, tt.want)
		}
	}
}

func TestEffectSourceValuesRemainDistinct(t *testing.T) {
	t.Parallel()

	if EffectSourceType == EffectSourceAbility {
		t.Fatal("type and ability effect sources must remain distinguishable")
	}
}
