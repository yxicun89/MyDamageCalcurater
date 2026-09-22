package balance

import (
	"errors"
	"fmt"
)

// TB4 仮想敵診断(ADR-0400)。TB1〜3 の計算(CalculateDefenseWithAbility)を再利用する。

// ErrThreatCount reports a threat list outside 1..MaxMembers (ADR-0400 §2).
var ErrThreatCount = errors.New("threats must contain between one and six entries")

// Combatant is one member or one threat whose types, optional ability and moves are
// already resolved. Ability is nil without an ability. Moves may include status moves,
// which never contribute to an attack type.
type Combatant struct {
	PokemonID string
	Types     []TypeID
	Ability   *Ability
	Moves     []Move
}

// ThreatMatchup is one member's result against one threat (ADR-0400 §2・§3).
//
// Incoming is the largest multiplier the member receives from the threat's attack types
// (member types and member ability), nil when the threat has no attack move.
// Outgoing is the largest multiplier the member's attack types deal to the threat
// (threat types and threat ability), nil when the member has no attack move.
// Safe = Incoming < 1 (false when nil). SuperEffective = Outgoing >= 2 (false when nil).
type ThreatMatchup struct {
	PokemonID      string
	Incoming       *Effectiveness
	Outgoing       *Effectiveness
	Safe           bool
	SuperEffective bool
}

// ThreatResult is the result for one threat. AbilityID is the threat's Ability.AbilityID,
// or "" without an ability. AttackTypes are the types of the threat's non-status moves,
// deduplicated, in canonical order. Matchups follow the member order.
type ThreatResult struct {
	PokemonID             string
	AbilityID             string
	AttackTypes           []TypeID
	Matchups              []ThreatMatchup
	SafeMembers           int
	SuperEffectiveMembers int
}

// ThreatAnalysis is the TB4 result: one ThreatResult per threat, in input order.
type ThreatAnalysis struct {
	Threats []ThreatResult
}

// AnalyzeThreats computes, for each threat, every member's incoming and outgoing
// multiplier (ADR-0400 §3). members and threats must each contain 1..MaxMembers entries.
//
// Validation order (ADR-0400 §6, mirrors AnalyzeCoverage/ADR-0016 §6): member count
// → threat count → chart nil (regardless of any attack move, §6.2) → moves of every
// combatant (members then threats, same checks as AnalyzeCoverage: move count,
// duplicate moveId, move category, attack move type; §6.1) → ability effects of
// every combatant (members then threats, validated in full regardless of any attack
// move, §6.3). Every combatant's types are checked first (§6.7).
func AnalyzeThreats(chart TypeChartProvider, members, threats []Combatant) (ThreatAnalysis, error) {
	if len(members) < 1 || len(members) > MaxMembers {
		return ThreatAnalysis{}, ErrMemberCount
	}
	if len(threats) < 1 || len(threats) > MaxMembers {
		return ThreatAnalysis{}, ErrThreatCount
	}
	if chart == nil {
		return ThreatAnalysis{}, ErrNilTypeChart
	}

	// Types are validated even when the other side has no attack move (ADR-0400 §6.7).
	for _, side := range [][]Combatant{members, threats} {
		for _, combatant := range side {
			if err := validateDefenseTypes(combatant.Types); err != nil {
				return ThreatAnalysis{}, err
			}
		}
	}
	for _, side := range [][]Combatant{members, threats} {
		for _, combatant := range side {
			if err := validateCombatantMoves(combatant.Moves); err != nil {
				return ThreatAnalysis{}, err
			}
		}
	}
	for _, side := range [][]Combatant{members, threats} {
		for _, combatant := range side {
			if err := validateCombatantAbility(combatant.Ability); err != nil {
				return ThreatAnalysis{}, err
			}
		}
	}

	memberAttackTypes := make([][]TypeID, len(members))
	for mi, member := range members {
		memberAttackTypes[mi] = attackTypesOf(member.Moves)
	}

	results := make([]ThreatResult, len(threats))
	for ti, threat := range threats {
		threatAttackTypes := attackTypesOf(threat.Moves)

		matchups := make([]ThreatMatchup, len(members))
		safeMembers, superEffectiveMembers := 0, 0
		for mi, member := range members {
			incoming, err := bestDefense(chart, threatAttackTypes, member.Types, member.Ability)
			if err != nil {
				return ThreatAnalysis{}, err
			}
			outgoing, err := bestDefense(chart, memberAttackTypes[mi], threat.Types, threat.Ability)
			if err != nil {
				return ThreatAnalysis{}, err
			}

			safe := incoming != nil && incoming.Cmp(Effectiveness{Num: 1, Den: 1}) < 0
			superEffective := outgoing != nil && outgoing.Cmp(Effectiveness{Num: 2, Den: 1}) >= 0
			matchups[mi] = ThreatMatchup{
				PokemonID:      member.PokemonID,
				Incoming:       incoming,
				Outgoing:       outgoing,
				Safe:           safe,
				SuperEffective: superEffective,
			}
			if safe {
				safeMembers++
			}
			if superEffective {
				superEffectiveMembers++
			}
		}

		abilityID := ""
		if threat.Ability != nil {
			abilityID = threat.Ability.AbilityID
		}
		results[ti] = ThreatResult{
			PokemonID:             threat.PokemonID,
			AbilityID:             abilityID,
			AttackTypes:           threatAttackTypes,
			Matchups:              matchups,
			SafeMembers:           safeMembers,
			SuperEffectiveMembers: superEffectiveMembers,
		}
	}

	return ThreatAnalysis{Threats: results}, nil
}

// bestDefense returns the largest CalculateDefenseWithAbility effectiveness over
// attackTypes against defenseTypes/ability, or nil when attackTypes is empty
// (ADR-0400 §3: null without an attack move on the attacking side).
func bestDefense(chart TypeChartProvider, attackTypes, defenseTypes []TypeID, ability *Ability) (*Effectiveness, error) {
	if len(attackTypes) == 0 {
		return nil, nil
	}
	var best *Effectiveness
	for _, attackType := range attackTypes {
		result, err := CalculateDefenseWithAbility(chart, attackType, defenseTypes, ability)
		if err != nil {
			return nil, err
		}
		if best == nil || result.Effectiveness.Cmp(*best) > 0 {
			value := result.Effectiveness
			best = &value
		}
	}
	return best, nil
}

// validateCombatantMoves checks one combatant's moves; AnalyzeCoverage and AnalyzeThreats share it (ADR-0400 §6.1 / ADR-0016 §6): move count, duplicate
// moveId, move category, and (for non-status moves only) attack move type.
func validateCombatantMoves(moves []Move) error {
	if len(moves) > MaxMovesPerMember {
		return ErrMoveCount
	}
	seen := make(map[string]struct{}, len(moves))
	for _, move := range moves {
		if _, ok := seen[move.MoveID]; ok {
			return ErrDuplicateMove
		}
		seen[move.MoveID] = struct{}{}
	}
	for _, move := range moves {
		if !move.Category.Valid() {
			return fmt.Errorf("%w: %q", ErrInvalidMoveCategory, move.Category)
		}
	}
	for _, move := range moves {
		if move.Category == MoveCategoryStatus {
			continue
		}
		if !move.Type.Valid() {
			return fmt.Errorf("%w: attack %q", ErrInvalidType, move.Type)
		}
	}
	return nil
}

// validateCombatantAbility validates every effect of ability, whatever the attack
// moves on either side (ADR-0400 §6.3 / ADR-0017 §5). A nil ability is valid.
func validateCombatantAbility(ability *Ability) error {
	if ability == nil {
		return nil
	}
	for _, effect := range ability.Effects {
		if err := validateAbilityEffect(effect); err != nil {
			return err
		}
	}
	return nil
}
