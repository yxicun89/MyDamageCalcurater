package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB2 の技の read model(ADR-0016 §3)。

// MovesSchemaVersion is the only accepted move read model schemaVersion.
const MovesSchemaVersion = 1

// ErrInvalidMoves reports a move read model that violates the ADR-0016 §3 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidMoves = errors.New("invalid move read model")

var moveIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxMoveIDLength = 40

// MoveReadModel is the temporary balance-local read model of moveId -> type and category.
// It is loaded once at startup and replaced by the shared master snapshot adapter later.
type MoveReadModel struct {
	moves map[string]balance.Move
}

var _ balance.MoveProvider = (*MoveReadModel)(nil)

// movesFile is the on-disk JSON shape (ADR-0016 §3).
type movesFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	Moves         []movesEntry `json:"moves"`
}

type movesEntry struct {
	MoveID   string `json:"moveId"`
	Type     string `json:"type"`
	Category string `json:"category"`
}

// LoadMoves reads and validates the JSON read model:
//
//	{"schemaVersion": 1, "moves": [{"moveId": "move-9001", "type": "fire", "category": "special"}]}
func LoadMoves(r io.Reader) (*MoveReadModel, error) {
	var file movesFile
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMoves, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: document must contain exactly one JSON object", ErrInvalidMoves)
	}

	if file.SchemaVersion != MovesSchemaVersion {
		return nil, fmt.Errorf("%w: schemaVersion must be %d, got %d", ErrInvalidMoves, MovesSchemaVersion, file.SchemaVersion)
	}
	if len(file.Moves) == 0 {
		return nil, fmt.Errorf("%w: moves must not be empty", ErrInvalidMoves)
	}

	moves := make(map[string]balance.Move, len(file.Moves))
	for _, entry := range file.Moves {
		if entry.MoveID == "" || len(entry.MoveID) > maxMoveIDLength || !moveIDPattern.MatchString(entry.MoveID) {
			return nil, fmt.Errorf("%w: moveId %q must match %s and be at most %d characters", ErrInvalidMoves, entry.MoveID, moveIDPattern, maxMoveIDLength)
		}
		if _, exists := moves[entry.MoveID]; exists {
			return nil, fmt.Errorf("%w: duplicate moveId %q", ErrInvalidMoves, entry.MoveID)
		}

		typeID := balance.TypeID(entry.Type)
		if entry.Type == "" || !typeID.Valid() {
			return nil, fmt.Errorf("%w: moveId %q has invalid type %q", ErrInvalidMoves, entry.MoveID, entry.Type)
		}

		category := balance.MoveCategory(entry.Category)
		if entry.Category == "" || !category.Valid() {
			return nil, fmt.Errorf("%w: moveId %q has invalid category %q", ErrInvalidMoves, entry.MoveID, entry.Category)
		}

		moves[entry.MoveID] = balance.Move{MoveID: entry.MoveID, Type: typeID, Category: category}
	}

	return &MoveReadModel{moves: moves}, nil
}

// LoadMovesFile opens path and delegates to LoadMoves.
func LoadMovesFile(path string) (*MoveReadModel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadMoves(f)
}

// Move implements balance.MoveProvider. Unknown IDs wrap balance.ErrUnknownMove.
func (m *MoveReadModel) Move(moveID string) (balance.Move, error) {
	move, ok := m.moves[moveID]
	if !ok {
		return balance.Move{}, fmt.Errorf("%w: %s", balance.ErrUnknownMove, moveID)
	}
	return move, nil
}
