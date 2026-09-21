package balance

import (
	"errors"
	"fmt"
	"testing"
)

// TB2 の技の解決(ADR-0016 §3)。

type stubMoves map[string]Move

func (s stubMoves) Move(moveID string) (Move, error) {
	if move, ok := s[moveID]; ok {
		return move, nil
	}
	return Move{}, fmt.Errorf("%w: %s", ErrUnknownMove, moveID)
}

type failingMoves struct{ err error }

func (f failingMoves) Move(string) (Move, error) { return Move{}, f.err }

var fictionalMoves = stubMoves{
	"move-9001": {MoveID: "move-9001", Type: TypeFire, Category: MoveCategorySpecial},
	"move-9002": {MoveID: "move-9002", Type: TypeFire, Category: MoveCategoryPhysical},
	"move-9006": {MoveID: "move-9006", Type: TypeGrass, Category: MoveCategoryStatus},
}

func TestMaxMovesPerMember(t *testing.T) {
	t.Parallel()

	if MaxMovesPerMember != 4 {
		t.Fatalf("MaxMovesPerMember = %d, want 4 (ADR-0016 §1)", MaxMovesPerMember)
	}
}

func TestMoveCategoryValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		category MoveCategory
		want     bool
	}{
		{MoveCategoryPhysical, true},
		{MoveCategorySpecial, true},
		{MoveCategoryStatus, true},
		{"physical", true},
		{"special", true},
		{"status", true},
		{"", false},
		{"Physical", false},
		{"STATUS", false},
		{"other", false},
		{" physical", false},
	}
	for _, tt := range tests {
		if got := tt.category.Valid(); got != tt.want {
			t.Errorf("MoveCategory(%q).Valid() = %v, want %v", tt.category, got, tt.want)
		}
	}
}

func TestResolveMoves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ids  []string
		want []Move
	}{
		{
			name: "keeps request order including status moves",
			ids:  []string{"move-9006", "move-9002", "move-9001"},
			want: []Move{
				{MoveID: "move-9006", Type: TypeGrass, Category: MoveCategoryStatus},
				{MoveID: "move-9002", Type: TypeFire, Category: MoveCategoryPhysical},
				{MoveID: "move-9001", Type: TypeFire, Category: MoveCategorySpecial},
			},
		},
		{
			name: "single move",
			ids:  []string{"move-9001"},
			want: []Move{{MoveID: "move-9001", Type: TypeFire, Category: MoveCategorySpecial}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveMoves(fictionalMoves, tt.ids)
			if err != nil {
				t.Fatalf("ResolveMoves() error = %v", err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("ResolveMoves() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveMovesEmpty(t *testing.T) {
	t.Parallel()

	got, err := ResolveMoves(fictionalMoves, nil)
	if err != nil {
		t.Fatalf("ResolveMoves(nil) error = %v, want nil (0 moves is valid)", err)
	}
	if len(got) != 0 {
		t.Errorf("ResolveMoves(nil) = %v, want no moves", got)
	}
}

func TestResolveMovesErrors(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("move read model broken")
	tests := []struct {
		name     string
		provider MoveProvider
		ids      []string
		wantErr  error
	}{
		{name: "unknown move", provider: fictionalMoves, ids: []string{"move-9999"}, wantErr: ErrUnknownMove},
		{name: "unknown after known", provider: fictionalMoves, ids: []string{"move-9001", "move-9999"}, wantErr: ErrUnknownMove},
		{name: "nil provider", provider: nil, ids: []string{"move-9001"}, wantErr: ErrNilMoves},
		{name: "nil provider with no moves", provider: nil, ids: nil, wantErr: ErrNilMoves},
		{name: "provider error is propagated", provider: failingMoves{err: providerErr}, ids: []string{"move-9001"}, wantErr: providerErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveMoves(tt.provider, tt.ids)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ResolveMoves() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if got != nil {
				t.Errorf("ResolveMoves() = %v, want nil moves on error", got)
			}
		})
	}
}

// A provider failure other than an unknown ID must not be reported as an unknown move
// (it becomes a 500, not a 422).
func TestResolveMovesProviderErrorIsNotUnknownMove(t *testing.T) {
	t.Parallel()

	_, err := ResolveMoves(failingMoves{err: errors.New("backend down")}, []string{"move-9001"})
	var unknown *UnknownMoveError
	if errors.As(err, &unknown) || errors.Is(err, ErrUnknownMove) {
		t.Fatalf("err = %v, want a non-unknown-move error", err)
	}
}

func TestResolveMovesUnknownMoveCarriesOnlyTheID(t *testing.T) {
	t.Parallel()

	provider := failingMoves{err: fmt.Errorf("%w: move-9999 (read from /secret/moves.json)", ErrUnknownMove)}
	_, err := ResolveMoves(provider, []string{"move-9999"})

	var unknown *UnknownMoveError
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want *UnknownMoveError", err)
	}
	if unknown.MoveID != "move-9999" {
		t.Errorf("MoveID = %q, want move-9999", unknown.MoveID)
	}
	if got, want := err.Error(), "unknown move: move-9999"; got != want {
		t.Errorf("Error() = %q, want %q (adapter detail must be dropped)", got, want)
	}
}

func TestUnknownMoveErrorUnwrapsOnlyToErrUnknownMove(t *testing.T) {
	t.Parallel()

	var err error = &UnknownMoveError{MoveID: "move-9999"}
	if !errors.Is(err, ErrUnknownMove) {
		t.Errorf("errors.Is(err, ErrUnknownMove) = false, want true")
	}
	if got := errors.Unwrap(err); got != ErrUnknownMove {
		t.Errorf("errors.Unwrap(err) = %v, want ErrUnknownMove", got)
	}
	if errors.Is(err, ErrUnknownPokemon) {
		t.Errorf("errors.Is(err, ErrUnknownPokemon) = true, want false")
	}
	if got, want := err.Error(), "unknown move: move-9999"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
