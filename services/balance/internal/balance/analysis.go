package balance

import (
	"errors"
	"fmt"
)

// TB1 contract (ADR-0014 §3・§4).

// MaxMembers is the largest party size accepted by the analysis.
const MaxMembers = 6

var (
	// ErrMemberCount reports a party outside 1..MaxMembers.
	ErrMemberCount = errors.New("members must contain between one and six entries")
	// ErrInvalidMultiplier reports a Multiplier value that cannot be classified.
	ErrInvalidMultiplier = errors.New("invalid multiplier")
)

// Category is the six-way classification of a defensive multiplier (ADR-0014 §3).
// The string values are the API enum values of DefenseCategory.
type Category string

const (
	CategoryQuadWeak   Category = "quad_weak"
	CategoryWeak       Category = "weak"
	CategoryNeutral    Category = "neutral"
	CategoryResist     Category = "resist"
	CategoryQuadResist Category = "quad_resist"
	CategoryImmune     Category = "immune"
)

// ClassifyMultiplier maps 16/8/4/2/1/0 to quad_weak/weak/neutral/resist/quad_resist/immune.
// Any other value returns an error wrapping ErrInvalidMultiplier.
func ClassifyMultiplier(m Multiplier) (Category, error) {
	switch m {
	case MultiplierQuad:
		return CategoryQuadWeak, nil
	case MultiplierDouble:
		return CategoryWeak, nil
	case MultiplierNormal:
		return CategoryNeutral, nil
	case MultiplierHalf:
		return CategoryResist, nil
	case MultiplierQuarter:
		return CategoryQuadResist, nil
	case MultiplierZero:
		return CategoryImmune, nil
	default:
		return "", fmt.Errorf("%w: %d", ErrInvalidMultiplier, m)
	}
}

// Member is one party member whose types are already resolved.
// Ability is nil for a member without an ability (TB1 behaviour, ADR-0017 §1).
type Member struct {
	PokemonID string
	Types     []TypeID
	Ability   *Ability
}

// AttackDefense is one member's defensive result against one attack type.
type AttackDefense struct {
	AttackType TypeID
	Result     DefenseResult
	Category   Category
}

// MemberDefense holds 18 AttackDefense entries in canonical attack-type order.
// AbilityID is the member's Ability.AbilityID, or "" without an ability.
type MemberDefense struct {
	PokemonID string
	AbilityID string
	Types     []TypeID
	Defense   []AttackDefense
}

// TeamSummaryEntry counts members per category for one attack type (ADR-0014 §3).
// Weak includes QuadWeak. Resist includes quarter resistances but never immunities.
type TeamSummaryEntry struct {
	AttackType TypeID
	Weak       int
	QuadWeak   int
	Resist     int
	Immune     int
	Neutral    int
}

// DefenseAnalysis is the TB1 result: members in input order and 18 summary entries
// in canonical attack-type order.
type DefenseAnalysis struct {
	Members     []MemberDefense
	TeamSummary []TeamSummaryEntry
}

// AnalyzeDefense computes the defensive type balance of 1..MaxMembers members.
func AnalyzeDefense(chart TypeChartProvider, members []Member) (DefenseAnalysis, error) {
	if len(members) < 1 || len(members) > MaxMembers {
		return DefenseAnalysis{}, ErrMemberCount
	}

	attackTypes := AllTypes()
	memberDefenses := make([]MemberDefense, len(members))
	summary := make([]TeamSummaryEntry, len(attackTypes))
	for i, attack := range attackTypes {
		summary[i] = TeamSummaryEntry{AttackType: attack}
	}

	for mi, member := range members {
		defense := make([]AttackDefense, len(attackTypes))
		for ai, attack := range attackTypes {
			result, err := CalculateDefenseWithAbility(chart, attack, member.Types, member.Ability)
			if err != nil {
				return DefenseAnalysis{}, err
			}
			category, err := ClassifyEffectiveness(result.Effectiveness)
			if err != nil {
				return DefenseAnalysis{}, err
			}
			defense[ai] = AttackDefense{AttackType: attack, Result: result, Category: category}

			switch category {
			case CategoryQuadWeak:
				summary[ai].Weak++
				summary[ai].QuadWeak++
			case CategoryWeak:
				summary[ai].Weak++
			case CategoryResist, CategoryQuadResist:
				summary[ai].Resist++
			case CategoryImmune:
				summary[ai].Immune++
			case CategoryNeutral:
				summary[ai].Neutral++
			}
		}
		abilityID := ""
		if member.Ability != nil {
			abilityID = member.Ability.AbilityID
		}
		memberDefenses[mi] = MemberDefense{PokemonID: member.PokemonID, AbilityID: abilityID, Types: member.Types, Defense: defense}
	}

	return DefenseAnalysis{Members: memberDefenses, TeamSummary: summary}, nil
}
