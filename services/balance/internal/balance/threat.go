package balance

import "errors"

// TB4 仮想敵診断(ADR-0400)。TB1〜3 の計算(CalculateDefenseWithAbility)を再利用する。
//
// spec-writer のスタブ: 型と関数の形だけを置き、zero 値を返す。implementer が実装する。

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
func AnalyzeThreats(chart TypeChartProvider, members, threats []Combatant) (ThreatAnalysis, error) {
	// TODO(implementer): ADR-0400 の計算。スタブは zero 値を返す。
	return ThreatAnalysis{}, nil
}
