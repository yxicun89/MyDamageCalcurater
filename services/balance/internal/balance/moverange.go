package balance

import "errors"

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
func AnalyzeMoveRange(chart TypeChartProvider, moves []Move, catalog []CatalogPokemon, abilities AbilityProvider) (MoveRangeAnalysis, error) {
	return MoveRangeAnalysis{}, nil
}
