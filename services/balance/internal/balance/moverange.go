package balance

import (
	"errors"
	"sort"
)

// TB6 技範囲チェッカー(ADR-0404)。技 ID だけ(ポケモンは指定しない)の攻撃範囲と、
// その技構成を半減以下で受けられる実在ポケモンを返す。
//
// このファイルは spec-writer が置いたコンパイル用のスタブ(契約と失敗するテストが先)。
// 実装は AnalyzeMoveRange の中身だけで、型・エラー・並びの契約は ADR-0404 §2・§3 の正である。

const (
	// MinMoveRangeMoves and MaxMoveRangeMoves bound the request moveIds (ADR-0404 §2).
	// Unlike a TB2 member (0..4), a move range needs at least one move.
	MinMoveRangeMoves = 1
	MaxMoveRangeMoves = MaxMovesPerMember
)

var (
	// ErrMoveRangeMoveCount reports a move set outside MinMoveRangeMoves..MaxMoveRangeMoves.
	ErrMoveRangeMoveCount = errors.New("moveIds must contain between one and four moves")
	// ErrMoveRangeNoAttackMove reports a move set whose moves are all status moves: the
	// offensive range would be empty, which ADR-0404 §2 rejects instead of reporting.
	ErrMoveRangeNoAttackMove = errors.New("moveIds must contain at least one attack move")
)

// MoveRangeEntry is the move set's result against one single defense type (ADR-0404 §2).
// It is the same calculation as one member's DefenseCoverage in TB2, except that
// BestMultiplier always has a value: a move set without any attack move is rejected.
type MoveRangeEntry struct {
	DefenseType    TypeID
	BestMultiplier Multiplier
	Effective      bool // BestMultiplier >= x1
	SuperEffective bool // BestMultiplier == x2
}

// WalledByPokemon is one catalog pokemon whose own types take the move set at x1/2 or less
// (ADR-0404 §2). BestMultiplier is the largest multiplier the move set deals to its actual
// types (CalculateDefense; abilities are not considered). Types keep the read model order.
type WalledByPokemon struct {
	PokemonID      string
	NameJa         string
	Types          []TypeID
	BestMultiplier Effectiveness
}

// WalledByAbilityPokemon is one catalog pokemon and one of its read model abilities that
// bring the move set to x1/2 or less while its types alone do not (ADR-0404 §2; the same
// idea as TB5's AbilityOption). An immunity or absorption qualifies as well.
type WalledByAbilityPokemon struct {
	PokemonID      string
	NameJa         string
	AbilityID      string
	BestMultiplier Effectiveness
}

// MoveRangeAnalysis is the TB6 result (ADR-0404 §2).
//
// AttackTypes are the types of the non-status moves (deduplicated, canonical order, never
// empty). TypeChart has one entry per single defense type in canonical order. WalledBy is
// pokemonId ascending; WalledByAbility is pokemonId then abilityId ascending and is empty
// when no ability provider is given.
type MoveRangeAnalysis struct {
	AttackTypes     []TypeID
	TypeChart       []MoveRangeEntry
	WalledBy        []WalledByPokemon
	WalledByAbility []WalledByAbilityPokemon
}

// AnalyzeMoveRange computes the offensive range of a move set and the catalog pokemon that
// wall it (ADR-0404 §3).
//
// moves are the resolved moves (MinMoveRangeMoves..MaxMoveRangeMoves, distinct moveId, at
// least one non-status move). catalog is every pokemon of the read model. abilities may be
// nil: WalledByAbility is then empty.
//
// Validation order (ADR-0404 §4.2): move count (MinMoveRangeMoves..MaxMoveRangeMoves) → chart
// nil → validateCombatantMoves (duplicate moveId, move category, attack move type) → at least
// one non-status move.
func AnalyzeMoveRange(chart TypeChartProvider, moves []Move, catalog []CatalogPokemon, abilities AbilityProvider) (MoveRangeAnalysis, error) {
	if len(moves) < MinMoveRangeMoves || len(moves) > MaxMoveRangeMoves {
		return MoveRangeAnalysis{}, ErrMoveRangeMoveCount
	}
	if chart == nil {
		return MoveRangeAnalysis{}, ErrNilTypeChart
	}
	if err := validateCombatantMoves(moves); err != nil {
		return MoveRangeAnalysis{}, err
	}

	attackTypes := attackTypesOf(moves)
	if len(attackTypes) == 0 {
		return MoveRangeAnalysis{}, ErrMoveRangeNoAttackMove
	}

	// typeChart is the same calculation as one member's DefenseCoverage in TB2 (ADR-0404 §3:
	// do not re-derive the attack-type/best-matchup composition), reused by calling
	// AnalyzeCoverage for a single virtual member holding this move set.
	coverage, err := AnalyzeCoverage(chart, []CoverageMember{{PokemonID: "move-range", Moves: moves}})
	if err != nil {
		return MoveRangeAnalysis{}, err
	}
	member := coverage.Members[0]
	typeChart := make([]MoveRangeEntry, len(member.Coverage))
	for i, entry := range member.Coverage {
		// attackTypes is non-empty (checked above), so AnalyzeCoverage always sets BestMultiplier.
		typeChart[i] = MoveRangeEntry{
			DefenseType:    entry.DefenseType,
			BestMultiplier: *entry.BestMultiplier,
			Effective:      entry.Effective,
			SuperEffective: entry.SuperEffective,
		}
	}

	walledBy, walledByAbility, err := moveRangeWalledBy(chart, attackTypes, catalog, abilities)
	if err != nil {
		return MoveRangeAnalysis{}, err
	}

	return MoveRangeAnalysis{
		AttackTypes:     member.AttackTypes,
		TypeChart:       typeChart,
		WalledBy:        walledBy,
		WalledByAbility: walledByAbility,
	}, nil
}

// moveRangeWalledByThreshold is the ADR-0404 §2 basis for "walled": x1/2 or less.
var moveRangeWalledByThreshold = Effectiveness{Num: 1, Den: 2}

// moveRangeWalledBy computes WalledBy and WalledByAbility (ADR-0404 §2・§3): for each catalog
// pokemon (pokemonId ascending), the largest multiplier the move set deals to its actual types
// over every attack type. A pokemon that is not walled by its types alone is then checked,
// abilityId ascending, against each of its read model abilities (skipping one the provider does
// not know, ADR-0401 §7.2); abilities is a separate pass and left empty when abilities is nil.
func moveRangeWalledBy(chart TypeChartProvider, attackTypes []TypeID, catalog []CatalogPokemon, abilities AbilityProvider) ([]WalledByPokemon, []WalledByAbilityPokemon, error) {
	sorted := make([]CatalogPokemon, len(catalog))
	copy(sorted, catalog)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PokemonID < sorted[j].PokemonID })

	var walledBy []WalledByPokemon
	var walledByAbility []WalledByAbilityPokemon
	for _, pokemon := range sorted {
		best, err := moveRangeBestEffectiveness(chart, attackTypes, pokemon.Types, nil)
		if err != nil {
			return nil, nil, err
		}
		if best.Cmp(moveRangeWalledByThreshold) <= 0 {
			types := make([]TypeID, len(pokemon.Types))
			copy(types, pokemon.Types)
			walledBy = append(walledBy, WalledByPokemon{
				PokemonID:      pokemon.PokemonID,
				NameJa:         pokemon.NameJa,
				Types:          types,
				BestMultiplier: best,
			})
			continue
		}
		if abilities == nil {
			continue
		}

		abilityIDs := make([]string, len(pokemon.AbilityIDs))
		copy(abilityIDs, pokemon.AbilityIDs)
		sort.Strings(abilityIDs)
		for _, abilityID := range abilityIDs {
			ability, err := abilities.Ability(abilityID)
			if err != nil {
				if errors.Is(err, ErrUnknownAbility) {
					// export inconsistency: skip only this ability (ADR-0401 §7.2).
					continue
				}
				return nil, nil, err
			}
			withAbility, err := moveRangeBestEffectiveness(chart, attackTypes, pokemon.Types, &ability)
			if err != nil {
				return nil, nil, err
			}
			if withAbility.Cmp(moveRangeWalledByThreshold) <= 0 {
				walledByAbility = append(walledByAbility, WalledByAbilityPokemon{
					PokemonID:      pokemon.PokemonID,
					NameJa:         pokemon.NameJa,
					AbilityID:      abilityID,
					BestMultiplier: withAbility,
				})
			}
		}
	}
	return walledBy, walledByAbility, nil
}

// moveRangeBestEffectiveness is the largest multiplier defenseTypes takes from any of
// attackTypes (with ability applied when not nil; CalculateDefenseWithAbility accepts nil).
func moveRangeBestEffectiveness(chart TypeChartProvider, attackTypes, defenseTypes []TypeID, ability *Ability) (Effectiveness, error) {
	var best Effectiveness
	for i, attack := range attackTypes {
		result, err := CalculateDefenseWithAbility(chart, attack, defenseTypes, ability)
		if err != nil {
			return Effectiveness{}, err
		}
		if i == 0 || result.Effectiveness.Cmp(best) > 0 {
			best = result.Effectiveness
		}
	}
	return best, nil
}
