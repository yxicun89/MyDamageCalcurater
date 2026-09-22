package balance

import (
	"errors"
	"fmt"
	"sort"
)

// TB5 おすすめタイプと該当ポケモン(ADR-0401)。TB1〜4 の計算(CalculateDefenseWithAbility /
// CalculateDefense / AnalyzeCoverage の attackTypesOf と同じ考え方)を再利用する。

const (
	// DefaultRecommendationLimit is the number of candidates returned when the request has no limit (ADR-0401 §3).
	DefaultRecommendationLimit = 10
	// MinRecommendationLimit and MaxRecommendationLimit bound the request limit (ADR-0401 §3).
	MinRecommendationLimit = 1
	MaxRecommendationLimit = 20
)

// ErrRecommendationLimit reports a limit outside MinRecommendationLimit..MaxRecommendationLimit.
var ErrRecommendationLimit = errors.New("limit must be between 1 and 20")

// CatalogPokemon is one pokemon of the read model (ADR-0401 §5). NameJa is "" when the read
// model has no name. AbilityIDs are the abilities it may have (0..4, read model order).
// Types keep the read model order (ADR-0014 §5.1).
type CatalogPokemon struct {
	PokemonID  string
	NameJa     string
	Types      []TypeID
	AbilityIDs []string
}

// PokemonCatalog lists every pokemon of the read model. balance treats every pokemon in the
// read model as usable (ADR-0401 §4); filtering by regulation belongs to the exporter.
// It is the replacement boundary for the shared master snapshot; the domain package does not own it.
type PokemonCatalog interface {
	AllPokemon() ([]CatalogPokemon, error)
}

// RecommendedPokemon is one pokemon listed for a candidate (ADR-0401 §8).
type RecommendedPokemon struct {
	PokemonID string
	NameJa    string
	Types     []TypeID
	// ExactMatch reports whether the pokemon's type set equals the candidate's (ADR-0401 §8).
	ExactMatch bool
}

// TypeCandidate is one recommended defense type set (ADR-0401 §3).
//
// Types are in canonical order. DefenseCovered are the defense holes the candidate takes below
// x1 by its types alone; OffenseCovered are the offense holes its own types hit at x1 or more
// (both in canonical order). Weaknesses is the number of attack types hitting it at x2 or more.
// Pokemon are the catalog pokemon for the candidate (ADR-0401 §8: a single type also lists the
// pokemon containing it that still take its defense holes), exact matches first, then pokemonId.
type TypeCandidate struct {
	Types          []TypeID
	DefenseCovered []TypeID
	OffenseCovered []TypeID
	Weaknesses     int
	Pokemon        []RecommendedPokemon
}

// AbilityPokemon is one pokemon and one of its abilities that take the attack type below x1
// while its types alone do not. Multiplier is the final value with the ability.
type AbilityPokemon struct {
	PokemonID  string
	NameJa     string
	AbilityID  string
	Multiplier Effectiveness
}

// AbilityOption lists, for one defense hole, the pokemon that an ability lets fill it (ADR-0401 §4).
type AbilityOption struct {
	AttackType TypeID
	Pokemon    []AbilityPokemon
}

// Recommendation is the TB5 result (ADR-0401 §6).
type Recommendation struct {
	DefenseHoles   []TypeID
	OffenseHoles   []TypeID
	Candidates     []TypeCandidate
	AbilityOptions []AbilityOption
}

// RecommendTypes finds the party's holes and the type sets that fill them (ADR-0401 §2〜4).
//
// members are the resolved party (1..MaxMembers; types, optional ability, moves). catalog is
// every pokemon of the read model. abilities may be nil: AbilityOptions is then empty.
// limit must be MinRecommendationLimit..MaxRecommendationLimit.
func RecommendTypes(chart TypeChartProvider, members []Combatant, catalog []CatalogPokemon, abilities AbilityProvider, limit int) (Recommendation, error) {
	if len(members) < 1 || len(members) > MaxMembers {
		return Recommendation{}, ErrMemberCount
	}
	if limit < MinRecommendationLimit || limit > MaxRecommendationLimit {
		return Recommendation{}, ErrRecommendationLimit
	}
	if chart == nil {
		return Recommendation{}, ErrNilTypeChart
	}
	for _, member := range members {
		if err := validateDefenseTypes(member.Types); err != nil {
			return Recommendation{}, err
		}
	}
	for _, member := range members {
		if err := validateCombatantMoves(member.Moves); err != nil {
			return Recommendation{}, err
		}
	}
	for _, member := range members {
		if err := validateCombatantAbility(member.Ability); err != nil {
			return Recommendation{}, err
		}
	}

	allTypes := AllTypes()

	defenseHoles, err := recommendDefenseHoles(chart, members)
	if err != nil {
		return Recommendation{}, err
	}
	offenseHoles, err := recommendOffenseHoles(chart, members)
	if err != nil {
		return Recommendation{}, err
	}

	candidates, err := recommendCandidates(chart, defenseHoles, offenseHoles, allTypes)
	if err != nil {
		return Recommendation{}, err
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for i := range candidates {
		pokemon, err := matchingPokemon(chart, catalog, candidates[i])
		if err != nil {
			return Recommendation{}, err
		}
		candidates[i].Pokemon = pokemon
	}

	var abilityOptions []AbilityOption
	if abilities != nil {
		abilityOptions = make([]AbilityOption, len(defenseHoles))
		for i, attack := range defenseHoles {
			pokemon, err := abilityOptionsFor(chart, attack, catalog, abilities)
			if err != nil {
				return Recommendation{}, err
			}
			abilityOptions[i] = AbilityOption{AttackType: attack, Pokemon: pokemon}
		}
	}

	return Recommendation{
		DefenseHoles:   defenseHoles,
		OffenseHoles:   offenseHoles,
		Candidates:     candidates,
		AbilityOptions: abilityOptions,
	}, nil
}

// recommendDefenseHoles finds every attack type that no member resists or is immune to,
// with the members' abilities applied (ADR-0401 §2), by reusing TB1's team aggregation
// (AnalyzeDefense) instead of re-deriving categories per attack type: a hole is exactly an
// attack type whose TeamSummaryEntry has no resist and no immune member (ADR-0014 §3:
// Resist already includes quad_resist and never immunities, so entry.Resist == 0 &&
// entry.Immune == 0 is "no member resists or is immune to it"). Every member × attack type is
// evaluated, so a broken type chart is always an error (the previous per-type loop could stop early).
func recommendDefenseHoles(chart TypeChartProvider, members []Combatant) ([]TypeID, error) {
	defenseMembers := make([]Member, len(members))
	for i, member := range members {
		defenseMembers[i] = Member{PokemonID: member.PokemonID, Types: member.Types, Ability: member.Ability}
	}
	analysis, err := AnalyzeDefense(chart, defenseMembers)
	if err != nil {
		return nil, err
	}

	var holes []TypeID
	for _, entry := range analysis.TeamSummary {
		if entry.Resist == 0 && entry.Immune == 0 {
			holes = append(holes, entry.AttackType)
		}
	}
	return holes, nil
}

// recommendOffenseHoles finds every single defense type the party's own moves cannot hit at
// x1 or more (ADR-0401 §2), by reusing TB2's team aggregation (AnalyzeCoverage) instead of
// re-deriving attack types and best matchups per defense type: a hole is exactly a defense
// type whose TeamCoverageEntry has no effective member. A party without any attack move has
// no offense hole at all (§7.1: a team of only status moves counts the same as no move),
// which AnalyzeCoverage itself cannot report (every TeamCoverageEntry.EffectiveMembers would
// be 0, indistinguishable from "every type is a hole"), so that case is detected separately
// from whether any member's resolved MemberCoverage.AttackTypes is non-empty.
func recommendOffenseHoles(chart TypeChartProvider, members []Combatant) ([]TypeID, error) {
	coverageMembers := make([]CoverageMember, len(members))
	for i, member := range members {
		coverageMembers[i] = CoverageMember{PokemonID: member.PokemonID, Moves: member.Moves}
	}
	analysis, err := AnalyzeCoverage(chart, coverageMembers)
	if err != nil {
		return nil, err
	}

	hasAttack := false
	for _, member := range analysis.Members {
		if len(member.AttackTypes) > 0 {
			hasAttack = true
			break
		}
	}
	if !hasAttack {
		return nil, nil
	}

	var holes []TypeID
	for _, entry := range analysis.TeamCoverage {
		if entry.EffectiveMembers == 0 {
			holes = append(holes, entry.DefenseType)
		}
	}
	return holes, nil
}

// allRecommendationCombos returns the 171 candidate type sets (18 single, 153 dual) in
// canonical order: every single type first (canonical order), then duals by their first
// then second type (both canonical order). This is also the ADR-0401 §3 tie-break order,
// so a stable sort by (total desc, weaknesses asc) alone reproduces it for ties.
func allRecommendationCombos(allTypes []TypeID) [][]TypeID {
	combos := make([][]TypeID, 0, len(allTypes)+len(allTypes)*(len(allTypes)-1)/2)
	for _, t := range allTypes {
		combos = append(combos, []TypeID{t})
	}
	for i := 0; i < len(allTypes); i++ {
		for j := i + 1; j < len(allTypes); j++ {
			combos = append(combos, []TypeID{allTypes[i], allTypes[j]})
		}
	}
	return combos
}

// recommendCandidates scores every type-set combination against the holes (ADR-0401 §3) and
// returns the ones that fill at least one hole, in the final order (candidates without any
// hole are left out before the limit is applied by the caller).
func recommendCandidates(chart TypeChartProvider, defenseHoles, offenseHoles, allTypes []TypeID) ([]TypeCandidate, error) {
	var candidates []TypeCandidate
	for _, combo := range allRecommendationCombos(allTypes) {
		var defenseCovered []TypeID
		for _, attack := range defenseHoles {
			result, err := CalculateDefense(chart, attack, combo)
			if err != nil {
				return nil, err
			}
			if result.Multiplier < MultiplierNormal {
				defenseCovered = append(defenseCovered, attack)
			}
		}

		var offenseCovered []TypeID
		for _, defense := range offenseHoles {
			hit := false
			for _, own := range combo {
				matchup, err := chart.Matchup(own, defense)
				if err != nil {
					return nil, fmt.Errorf("type chart matchup %s/%s: %w", own, defense, err)
				}
				if !matchup.ValidSingleType() {
					return nil, fmt.Errorf("type chart matchup %s/%s returned invalid multiplier %d", own, defense, matchup)
				}
				if matchup >= MultiplierNormal {
					hit = true
					break
				}
			}
			if hit {
				offenseCovered = append(offenseCovered, defense)
			}
		}

		if len(defenseCovered)+len(offenseCovered) == 0 {
			continue
		}

		weaknesses := 0
		for _, attack := range allTypes {
			result, err := CalculateDefense(chart, attack, combo)
			if err != nil {
				return nil, err
			}
			if result.Multiplier >= MultiplierDouble {
				weaknesses++
			}
		}

		candidates = append(candidates, TypeCandidate{
			Types:          combo,
			DefenseCovered: defenseCovered,
			OffenseCovered: offenseCovered,
			Weaknesses:     weaknesses,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		ti := len(candidates[i].DefenseCovered) + len(candidates[i].OffenseCovered)
		tj := len(candidates[j].DefenseCovered) + len(candidates[j].OffenseCovered)
		if ti != tj {
			return ti > tj
		}
		return candidates[i].Weaknesses < candidates[j].Weaknesses
	})
	return candidates, nil
}

// matchingPokemon returns the catalog pokemon for a candidate (ADR-0401 §8). A dual-type
// candidate lists the pokemon whose type set equals it. A single-type candidate also lists the
// pokemon that contain its type, except those whose actual types no longer take every one of the
// candidate's covered defense holes below x1. Exact matches come first, then pokemonId ascending.
func matchingPokemon(chart TypeChartProvider, catalog []CatalogPokemon, candidate TypeCandidate) ([]RecommendedPokemon, error) {
	want := typeSet(candidate.Types)
	one := Effectiveness{Num: 1, Den: 1}
	result := make([]RecommendedPokemon, 0, len(catalog))
	for _, pokemon := range catalog {
		have := typeSet(pokemon.Types)
		exact := typeSetEqual(have, want)
		if !exact {
			if len(candidate.Types) != 1 {
				continue
			}
			if _, ok := have[candidate.Types[0]]; !ok {
				continue
			}
			keepsHoles := true
			for _, attack := range candidate.DefenseCovered {
				defense, err := CalculateDefense(chart, attack, pokemon.Types)
				if err != nil {
					return nil, err
				}
				if defense.Effectiveness.Cmp(one) >= 0 {
					keepsHoles = false
					break
				}
			}
			if !keepsHoles {
				continue
			}
		}
		types := make([]TypeID, len(pokemon.Types))
		copy(types, pokemon.Types)
		result = append(result, RecommendedPokemon{PokemonID: pokemon.PokemonID, NameJa: pokemon.NameJa, Types: types, ExactMatch: exact})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ExactMatch != result[j].ExactMatch {
			return result[i].ExactMatch
		}
		return result[i].PokemonID < result[j].PokemonID
	})
	return result, nil
}

func typeSet(types []TypeID) map[TypeID]struct{} {
	set := make(map[TypeID]struct{}, len(types))
	for _, t := range types {
		set[t] = struct{}{}
	}
	return set
}

func typeSetEqual(a, b map[TypeID]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for t := range a {
		if _, ok := b[t]; !ok {
			return false
		}
	}
	return true
}

// abilityOptionsFor lists, for one defense hole (attack), the catalog pokemon that one of
// their abilities takes below x1 while their types alone do not (ADR-0401 §4), pokemonId
// ascending then abilityId ascending (§7.3). An abilityId the provider does not know is
// skipped (§7.2: an export inconsistency does not fail the whole endpoint); any other
// provider failure is propagated.
func abilityOptionsFor(chart TypeChartProvider, attack TypeID, catalog []CatalogPokemon, abilities AbilityProvider) ([]AbilityPokemon, error) {
	sorted := make([]CatalogPokemon, len(catalog))
	copy(sorted, catalog)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PokemonID < sorted[j].PokemonID })

	var result []AbilityPokemon
	one := Effectiveness{Num: 1, Den: 1}
	for _, pokemon := range sorted {
		base, err := CalculateDefense(chart, attack, pokemon.Types)
		if err != nil {
			return nil, err
		}
		// Types alone already cover the hole: not an ability option (ADR-0401 §4).
		if base.Multiplier < MultiplierNormal {
			continue
		}

		abilityIDs := make([]string, len(pokemon.AbilityIDs))
		copy(abilityIDs, pokemon.AbilityIDs)
		sort.Strings(abilityIDs)
		for _, abilityID := range abilityIDs {
			ability, err := abilities.Ability(abilityID)
			if err != nil {
				if errors.Is(err, ErrUnknownAbility) {
					continue
				}
				return nil, err
			}
			withAbility, err := CalculateDefenseWithAbility(chart, attack, pokemon.Types, &ability)
			if err != nil {
				return nil, err
			}
			if withAbility.Effectiveness.Cmp(one) < 0 {
				result = append(result, AbilityPokemon{
					PokemonID:  pokemon.PokemonID,
					NameJa:     pokemon.NameJa,
					AbilityID:  abilityID,
					Multiplier: withAbility.Effectiveness,
				})
			}
		}
	}
	return result, nil
}
