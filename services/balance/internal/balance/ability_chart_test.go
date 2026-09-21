package balance_test

import (
	"fmt"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB3 の回帰(ADR-0017 §1・§3): 特性なしのメンバーは TB1 と完全に同じ結果になる。
// 同梱の相性表で単タイプ 18 + 2 タイプ 153 = 171 通りを確認する。

func TestAnalyzeDefenseWithoutAbilityKeepsTB1ResultsOnDataChart(t *testing.T) {
	t.Parallel()

	chart := testTypeChart()
	combos := allDefenseTypeCombos()
	if len(combos) != 171 {
		t.Fatalf("combos = %d, want 171", len(combos))
	}
	noEffects := &balance.Ability{AbilityID: "ability-9005"}

	for start := 0; start < len(combos); start += balance.MaxMembers {
		end := min(start+balance.MaxMembers, len(combos))
		plain := make([]balance.Member, 0, end-start)
		withEmptyAbility := make([]balance.Member, 0, end-start)
		for i, types := range combos[start:end] {
			id := fmt.Sprintf("9%03d-000", start+i+1)
			plain = append(plain, balance.Member{PokemonID: id, Types: types})
			withEmptyAbility = append(withEmptyAbility, balance.Member{PokemonID: id, Types: types, Ability: noEffects})
		}

		got, err := balance.AnalyzeDefense(chart, plain)
		if err != nil {
			t.Fatalf("AnalyzeDefense(%v) error = %v", plain, err)
		}
		gotEmpty, err := balance.AnalyzeDefense(chart, withEmptyAbility)
		if err != nil {
			t.Fatalf("AnalyzeDefense(empty ability) error = %v", err)
		}

		for mi, member := range plain {
			if got.Members[mi].AbilityID != "" {
				t.Errorf("%v: AbilityID = %q, want empty without an ability", member.Types, got.Members[mi].AbilityID)
			}
			if gotEmpty.Members[mi].AbilityID != "ability-9005" {
				t.Errorf("%v: AbilityID = %q, want ability-9005", member.Types, gotEmpty.Members[mi].AbilityID)
			}
			for ai, attack := range balance.AllTypes() {
				entry := got.Members[mi].Defense[ai]
				typeOnly, err := balance.CalculateDefense(chart, attack, member.Types)
				if err != nil {
					t.Fatalf("CalculateDefense(%s, %v) error = %v", attack, member.Types, err)
				}
				withNil, err := balance.CalculateDefenseWithAbility(chart, attack, member.Types, nil)
				if err != nil {
					t.Fatalf("CalculateDefenseWithAbility(nil) error = %v", err)
				}
				if withNil != typeOnly {
					t.Errorf("%v vs %s: CalculateDefenseWithAbility(nil) = %+v, want CalculateDefense %+v", member.Types, attack, withNil, typeOnly)
				}
				if entry.Result != typeOnly {
					t.Errorf("%v vs %s: result %+v, want %+v", member.Types, attack, entry.Result, typeOnly)
				}
				if entry.Result.Effectiveness != entry.Result.Multiplier.Effectiveness() {
					t.Errorf("%v vs %s: effectiveness %s, want the type matchup %s", member.Types, attack, entry.Result.Effectiveness, entry.Result.Multiplier)
				}
				if entry.Result.Effectiveness.String() != entry.Result.Multiplier.String() {
					t.Errorf("%v vs %s: label %q, want the TB1 label %q", member.Types, attack, entry.Result.Effectiveness.String(), entry.Result.Multiplier.String())
				}
				if entry.Result.Source != balance.EffectSourceType {
					t.Errorf("%v vs %s: source %d, want type", member.Types, attack, entry.Result.Source)
				}
				if entry.Result.Multiplier == balance.MultiplierZero {
					// The effect of a type immunity is undecided in ADR-0017; it is never absorb or multiplier.
					if entry.Result.Effect != balance.DefenseEffectNone && entry.Result.Effect != balance.DefenseEffectImmune {
						t.Errorf("%v vs %s: type immunity effect %q, want none or immune", member.Types, attack, entry.Result.Effect)
					}
				} else if entry.Result.Effect != balance.DefenseEffectNone {
					t.Errorf("%v vs %s: effect %q, want none", member.Types, attack, entry.Result.Effect)
				}
				wantCategory, err := balance.ClassifyMultiplier(entry.Result.Multiplier)
				if err != nil {
					t.Fatalf("ClassifyMultiplier(%d) error = %v", entry.Result.Multiplier, err)
				}
				if entry.Category != wantCategory {
					t.Errorf("%v vs %s: category %q, want %q", member.Types, attack, entry.Category, wantCategory)
				}
				// An ability without effects changes nothing (ADR-0017 §2 "effects は空可").
				if empty := gotEmpty.Members[mi].Defense[ai]; empty != entry {
					t.Errorf("%v vs %s: with an effectless ability %+v, want %+v", member.Types, attack, empty, entry)
				}
			}
		}
		for i := range got.TeamSummary {
			if got.TeamSummary[i] != gotEmpty.TeamSummary[i] {
				t.Errorf("summary %s: with an effectless ability %+v, want %+v", got.TeamSummary[i].AttackType, gotEmpty.TeamSummary[i], got.TeamSummary[i])
			}
		}
	}
}
