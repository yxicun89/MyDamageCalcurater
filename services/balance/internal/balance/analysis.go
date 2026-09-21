package balance

import "errors"

// TB1 contract (ADR-0014 §3・§4). Bodies are intentionally unimplemented stubs returning zero values
// written by the spec step; the implementer replaces them.

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
	return "", nil // TB1: not implemented
}

// Member is one party member whose types are already resolved.
type Member struct {
	PokemonID string
	Types     []TypeID
}

// AttackDefense is one member's defensive result against one attack type.
type AttackDefense struct {
	AttackType TypeID
	Result     DefenseResult
	Category   Category
}

// MemberDefense holds 18 AttackDefense entries in canonical attack-type order.
type MemberDefense struct {
	PokemonID string
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
	return DefenseAnalysis{}, nil // TB1: not implemented
}
