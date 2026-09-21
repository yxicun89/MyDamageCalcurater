package master

import (
	"errors"
	"io"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB2 の技の read model(ADR-0016 §3)。
//
// TODO(TB2 implementer): 以下はテストをコンパイルさせるための契約とスタブ。
// LoadMoves / Move の本体は未実装(zero 値を返す)。

// MovesSchemaVersion is the only accepted move read model schemaVersion.
const MovesSchemaVersion = 1

// ErrInvalidMoves reports a move read model that violates the ADR-0016 §3 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidMoves = errors.New("invalid move read model")

// MoveReadModel is the temporary balance-local read model of moveId -> type and category.
// It is loaded once at startup and replaced by the shared master snapshot adapter later.
type MoveReadModel struct{}

var _ balance.MoveProvider = (*MoveReadModel)(nil)

// LoadMoves reads and validates the JSON read model:
//
//	{"schemaVersion": 1, "moves": [{"moveId": "move-9001", "type": "fire", "category": "special"}]}
func LoadMoves(r io.Reader) (*MoveReadModel, error) {
	return nil, nil
}

// LoadMovesFile opens path and delegates to LoadMoves.
func LoadMovesFile(path string) (*MoveReadModel, error) {
	return nil, nil
}

// Move implements balance.MoveProvider. Unknown IDs wrap balance.ErrUnknownMove.
func (m *MoveReadModel) Move(moveID string) (balance.Move, error) {
	return balance.Move{}, nil
}
