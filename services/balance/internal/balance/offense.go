package balance

import "fmt"

// TB2 攻撃範囲(ADR-0016 §1・§2)。

// CoverageMember is one party member with its resolved moves (0..MaxMovesPerMember).
type CoverageMember struct {
	PokemonID string
	Moves     []Move
}

// DefenseCoverage is one member's offensive result against one single defense type.
// BestMultiplier is nil when the member has no attack (non-status) move; then
// Effective and SuperEffective are false.
type DefenseCoverage struct {
	DefenseType    TypeID
	BestMultiplier *Multiplier
	Effective      bool // BestMultiplier >= x1
	SuperEffective bool // BestMultiplier == x2
}

// MemberCoverage holds a member's move IDs (input order), its attack types
// (non-status move types, deduplicated, canonical order) and 18 DefenseCoverage
// entries in canonical defense-type order.
type MemberCoverage struct {
	PokemonID   string
	MoveIDs     []string
	AttackTypes []TypeID
	Coverage    []DefenseCoverage
}

// TeamCoverageEntry counts members (not moves) per single defense type.
// BestMultiplier is the best over all members, nil when no member has an attack move.
type TeamCoverageEntry struct {
	DefenseType           TypeID
	BestMultiplier        *Multiplier
	EffectiveMembers      int
	SuperEffectiveMembers int
}

// CoverageAnalysis is the TB2 result: members in input order and 18 team entries
// in canonical defense-type order.
type CoverageAnalysis struct {
	Members      []MemberCoverage
	TeamCoverage []TeamCoverageEntry
}

// AnalyzeCoverage computes the offensive coverage of 1..MaxMembers members.
//
// Validation order (ADR-0016 §6, spec-writer notes): member count → chart nil →
// per member: move count → duplicate moveId → move category → move type (attack
// moves only; a status move's type is never validated since it never contributes
// to attackTypes or coverage).
func AnalyzeCoverage(chart TypeChartProvider, members []CoverageMember) (CoverageAnalysis, error) {
	if len(members) < 1 || len(members) > MaxMembers {
		return CoverageAnalysis{}, ErrMemberCount
	}
	if chart == nil {
		return CoverageAnalysis{}, ErrNilTypeChart
	}

	for _, member := range members {
		if err := validateCombatantMoves(member.Moves); err != nil {
			return CoverageAnalysis{}, err
		}
	}

	defenseTypes := AllTypes()
	memberResults := make([]MemberCoverage, len(members))
	teamBest := make([]*Multiplier, len(defenseTypes))
	teamEffective := make([]int, len(defenseTypes))
	teamSuper := make([]int, len(defenseTypes))

	for mi, member := range members {
		moveIDs := make([]string, len(member.Moves))
		for i, move := range member.Moves {
			moveIDs[i] = move.MoveID
		}
		attackTypes := attackTypesOf(member.Moves)

		coverage := make([]DefenseCoverage, len(defenseTypes))
		for di, defense := range defenseTypes {
			var best *Multiplier
			for _, attackType := range attackTypes {
				matchup, err := chart.Matchup(attackType, defense)
				if err != nil {
					return CoverageAnalysis{}, fmt.Errorf("type chart matchup %s/%s: %w", attackType, defense, err)
				}
				if !matchup.ValidSingleType() {
					return CoverageAnalysis{}, fmt.Errorf("type chart matchup %s/%s returned invalid multiplier %d", attackType, defense, matchup)
				}
				if best == nil || matchup > *best {
					m := matchup
					best = &m
				}
			}

			effective := best != nil && *best >= MultiplierNormal
			superEffective := best != nil && *best == MultiplierDouble
			coverage[di] = DefenseCoverage{DefenseType: defense, BestMultiplier: best, Effective: effective, SuperEffective: superEffective}

			if best != nil {
				if teamBest[di] == nil || *best > *teamBest[di] {
					b := *best
					teamBest[di] = &b
				}
				if effective {
					teamEffective[di]++
				}
				if superEffective {
					teamSuper[di]++
				}
			}
		}

		memberResults[mi] = MemberCoverage{
			PokemonID:   member.PokemonID,
			MoveIDs:     moveIDs,
			AttackTypes: attackTypes,
			Coverage:    coverage,
		}
	}

	team := make([]TeamCoverageEntry, len(defenseTypes))
	for di, defense := range defenseTypes {
		team[di] = TeamCoverageEntry{
			DefenseType:           defense,
			BestMultiplier:        teamBest[di],
			EffectiveMembers:      teamEffective[di],
			SuperEffectiveMembers: teamSuper[di],
		}
	}

	return CoverageAnalysis{Members: memberResults, TeamCoverage: team}, nil
}

// attackTypesOf returns the distinct types of the non-status moves, in canonical
// type order (design doc §6: a member's own repeated or same-type moves count once).
func attackTypesOf(moves []Move) []TypeID {
	seen := make(map[TypeID]struct{}, len(moves))
	for _, move := range moves {
		if move.Category == MoveCategoryStatus {
			continue
		}
		seen[move.Type] = struct{}{}
	}

	types := make([]TypeID, 0, len(seen))
	for _, t := range AllTypes() {
		if _, ok := seen[t]; ok {
			types = append(types, t)
		}
	}
	return types
}
