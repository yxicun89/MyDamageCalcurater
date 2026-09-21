package balance_test

import (
	"fmt"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
)

// allDefenseTypeCombos returns the 18 single and 153 dual type combinations.
func allDefenseTypeCombos() [][]balance.TypeID {
	types := balance.AllTypes()
	combos := make([][]balance.TypeID, 0, 171)
	for i, first := range types {
		combos = append(combos, []balance.TypeID{first})
		for _, second := range types[i+1:] {
			combos = append(combos, []balance.TypeID{first, second})
		}
	}
	return combos
}

// TestAnalyzeDefenseAgreesWithCalculateDefenseOnDataChart runs every single and
// dual type through AnalyzeDefense (six at a time) with the bundled data chart and checks
// that each entry equals CalculateDefense, that categories follow ClassifyMultiplier,
// and that the team summary invariant holds for all 18 attack types.
func TestAnalyzeDefenseAgreesWithCalculateDefenseOnDataChart(t *testing.T) {
	t.Parallel()

	chart := testTypeChart()
	combos := allDefenseTypeCombos()
	if len(combos) != 171 {
		t.Fatalf("combos = %d, want 171", len(combos))
	}
	for start := 0; start < len(combos); start += balance.MaxMembers {
		end := min(start+balance.MaxMembers, len(combos))
		members := make([]balance.Member, 0, end-start)
		for i, types := range combos[start:end] {
			members = append(members, balance.Member{PokemonID: fmt.Sprintf("9%03d-000", start+i+1), Types: types})
		}

		got, err := balance.AnalyzeDefense(chart, members)
		if err != nil {
			t.Fatalf("AnalyzeDefense(%v) error = %v", members, err)
		}
		if len(got.Members) != len(members) || len(got.TeamSummary) != 18 {
			t.Fatalf("members=%d summary=%d, want %d and 18", len(got.Members), len(got.TeamSummary), len(members))
		}
		for mi, member := range members {
			if len(got.Members[mi].Defense) != 18 {
				t.Fatalf("%v: defense length = %d, want 18", member.Types, len(got.Members[mi].Defense))
			}
			for ai, attack := range balance.AllTypes() {
				want, err := balance.CalculateDefense(chart, attack, member.Types)
				if err != nil {
					t.Fatalf("CalculateDefense(%s, %v) error = %v", attack, member.Types, err)
				}
				entry := got.Members[mi].Defense[ai]
				if entry.AttackType != attack || entry.Result != want {
					t.Errorf("%v vs %s: got %+v, want attack %s result %+v", member.Types, attack, entry, attack, want)
				}
				wantCategory, err := balance.ClassifyMultiplier(want.Multiplier)
				if err != nil {
					t.Fatalf("ClassifyMultiplier(%d) error = %v", want.Multiplier, err)
				}
				if entry.Category != wantCategory {
					t.Errorf("%v vs %s: category %q, want %q", member.Types, attack, entry.Category, wantCategory)
				}
			}
		}
		for _, entry := range got.TeamSummary {
			if entry.Weak+entry.Resist+entry.Immune+entry.Neutral != len(members) {
				t.Errorf("%s: weak+resist+immune+neutral = %d, want %d", entry.AttackType, entry.Weak+entry.Resist+entry.Immune+entry.Neutral, len(members))
			}
			if entry.QuadWeak > entry.Weak {
				t.Errorf("%s: quadWeak %d > weak %d", entry.AttackType, entry.QuadWeak, entry.Weak)
			}
		}
	}
}

// testTypeChart は同梱の相性表(P1-13 のデータ)を返す。読めないのはテスト環境の不備なので panic する。
func testTypeChart() *master.TypeChart {
	chart, err := master.EmbeddedTypeChart()
	if err != nil {
		panic(err)
	}
	return chart
}
