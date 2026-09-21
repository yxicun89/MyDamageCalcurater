package balance

import (
	"errors"
	"fmt"
)

// TB2 の moveId → 技のタイプ・分類の解決(ADR-0016 §3)。
//
// TODO(TB2 implementer): 以下はテストをコンパイルさせるための契約とスタブ。
// ResolveMoves の本体は未実装(zero 値を返す)。

// MaxMovesPerMember is the largest number of moveIds one member may send (ADR-0016 §1).
const MaxMovesPerMember = 4

var (
	// ErrUnknownMove reports a moveId that the provider does not know.
	ErrUnknownMove = errors.New("unknown move")
	// ErrNilMoves reports a missing MoveProvider.
	ErrNilMoves = errors.New("move provider is required")
	// ErrMoveCount reports a member with more than MaxMovesPerMember moves.
	ErrMoveCount = errors.New("a member must have at most four moves")
	// ErrDuplicateMove reports the same moveId twice within one member.
	ErrDuplicateMove = errors.New("moves of a member must be distinct")
	// ErrInvalidMoveCategory reports a category other than physical/special/status.
	ErrInvalidMoveCategory = errors.New("invalid move category")
)

// MoveCategory is the damage class of a move. The values are the read model values.
type MoveCategory string

const (
	MoveCategoryPhysical MoveCategory = "physical"
	MoveCategorySpecial  MoveCategory = "special"
	MoveCategoryStatus   MoveCategory = "status"
)

// Valid reports whether c is physical, special or status.
func (c MoveCategory) Valid() bool {
	return false
}

// Move is one resolved move.
type Move struct {
	MoveID   string
	Type     TypeID
	Category MoveCategory
}

// MoveProvider resolves a moveId to its type and category. Unknown IDs return an
// error wrapping ErrUnknownMove. It is the replacement boundary for the shared
// master snapshot; the domain package does not own it.
type MoveProvider interface {
	Move(moveID string) (Move, error)
}

// ResolveMoves looks up each moveId in order and returns the moves.
// A nil provider returns ErrNilMoves. An unknown ID returns *UnknownMoveError
// carrying only the ID (adapter detail dropped). Other provider errors are propagated.
func ResolveMoves(provider MoveProvider, moveIDs []string) ([]Move, error) {
	return nil, nil
}

// UnknownMoveError は provider に無い moveId を表す。errors.Is(err, ErrUnknownMove) が真になる。
type UnknownMoveError struct {
	MoveID string
}

func (e *UnknownMoveError) Error() string {
	return fmt.Sprintf("%s: %s", ErrUnknownMove, e.MoveID)
}

func (e *UnknownMoveError) Unwrap() error { return nil }
