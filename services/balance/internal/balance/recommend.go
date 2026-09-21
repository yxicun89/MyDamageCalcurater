package balance

import "errors"

// TB5 おすすめタイプと該当ポケモン(ADR-0401)。
//
// spec-writer が置いたコンパイル用のスタブ。RecommendTypes は zero 値を返すだけで、
// 実装は implementer が ADR-0401 §2〜4 と recommend_test.go に従って書く。

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
// model has no name. AbilityIDs are the abilities it may have (0..3, read model order).
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

// RecommendedPokemon is one pokemon whose type set equals the candidate's.
type RecommendedPokemon struct {
	PokemonID string
	NameJa    string
	Types     []TypeID
}

// TypeCandidate is one recommended defense type set (ADR-0401 §3).
//
// Types are in canonical order. DefenseCovered are the defense holes the candidate takes below
// x1 by its types alone; OffenseCovered are the offense holes its own types hit at x1 or more
// (both in canonical order). Weaknesses is the number of attack types hitting it at x2 or more.
// Pokemon are the catalog pokemon with the same type set, pokemonId ascending.
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
	// TODO(implementer): ADR-0401 の実装。
	return Recommendation{}, nil
}
