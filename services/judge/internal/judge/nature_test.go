package judge

import (
	"errors"
	"testing"

	"example.com/pokecalc/engine"
)

// exampleNatures は pokedex-svc の性格一覧から作った架空の表(実マスタは使わない)。
// 無補正の性格は Plus / Minus とも空(上流の null に対応)。
func exampleNatures() []Nature {
	return []Nature{
		{ID: "test-plus-spe", Plus: engine.StatSpe, Minus: engine.StatSpA},
		{ID: "test-minus-spe", Plus: engine.StatSpA, Minus: engine.StatSpe},
		{ID: "test-neutral"},
	}
}

// TestNatureTableLookup: 性格 ID から engine.Nature を引く(ADR-0701 §4)。
// 1 リクエストにつき 1 回取った一覧を、attacker と defender の両方に使い回すための表。
func TestNatureTableLookup(t *testing.T) {
	t.Parallel()

	table := NewNatureTable(exampleNatures())

	tests := []struct {
		name string
		id   string
		want engine.Nature
	}{
		{"上昇補正", "test-plus-spe", engine.Nature{Plus: engine.StatSpe, Minus: engine.StatSpA}},
		{"下降補正", "test-minus-spe", engine.Nature{Plus: engine.StatSpA, Minus: engine.StatSpe}},
		{"無補正", "test-neutral", engine.NatureNeutral},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := table.Lookup(tt.id)
			if err != nil {
				t.Fatalf("Lookup(%q): %v", tt.id, err)
			}
			if got != tt.want {
				t.Errorf("Lookup(%q) = %+v, want %+v", tt.id, got, tt.want)
			}
		})
	}
}

// TestNatureTableLookupNeutralIsNeutral: 無補正の性格は engine から見ても補正なしであること。
// engine.Nature は Plus と Minus が等しいとき無補正になるので、空の組がそのまま無補正になる。
func TestNatureTableLookupNeutralIsNeutral(t *testing.T) {
	t.Parallel()

	got, err := NewNatureTable(exampleNatures()).Lookup("test-neutral")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !got.IsNeutral() {
		t.Errorf("Lookup(test-neutral) = %+v, want a neutral nature", got)
	}
}

// TestNatureTableLookupUnknown: 一覧に無い natureId は ErrUnknownNature(httpapi で 422
// unknown_nature になる。ADR-0701 §6)。黙って無補正に倒さない ― 性格補正を取り違えた判定は
// 外からは正しく見えるまま間違うため。
func TestNatureTableLookupUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		natures []Nature
		id      string
	}{
		{"一覧に無い ID", exampleNatures(), "test-missing"},
		{"空文字", exampleNatures(), ""},
		{"一覧が空", nil, "test-neutral"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewNatureTable(tt.natures).Lookup(tt.id)
			if !errors.Is(err, ErrUnknownNature) {
				t.Errorf("Lookup(%q) err = %v, want ErrUnknownNature", tt.id, err)
			}
			if got != (engine.Nature{}) {
				t.Errorf("Lookup(%q) = %+v, want the zero value on error", tt.id, got)
			}
		})
	}
}
