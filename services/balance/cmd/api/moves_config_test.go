package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
)

// TB2: BALANCE_MOVES_PATH(ADR-0016 §3。空文字は未設定扱い、ADR-0014 §5.3 準用)。

const exampleMovesPath = "../../testdata/moves.example.json"

func TestMovesPathEnvName(t *testing.T) {
	t.Parallel()

	if movesPathEnv != "BALANCE_MOVES_PATH" {
		t.Fatalf("movesPathEnv = %q, want BALANCE_MOVES_PATH (ADR-0016)", movesPathEnv)
	}
}

func TestMoveProviderFromEnvUnsetOrEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "unset", env: map[string]string{}},
		{name: "empty string", env: map[string]string{movesPathEnv: ""}},
		{name: "only the pokemon read model is set", env: map[string]string{pokemonTypesPathEnv: examplePokemonTypesPath}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := moveProviderFromEnv(lookupFrom(tt.env))
			if err != nil {
				t.Fatalf("error = %v, want nil when %s is unset or empty", err, movesPathEnv)
			}
			// Must be an untyped nil interface: a typed nil pointer would bypass the 503 path.
			if provider != nil {
				t.Fatalf("provider = %#v, want nil interface", provider)
			}
		})
	}
}

func TestMoveProviderFromEnvLoadsExample(t *testing.T) {
	t.Parallel()

	provider, err := moveProviderFromEnv(lookupFrom(map[string]string{movesPathEnv: exampleMovesPath}))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if provider == nil {
		t.Fatal("provider = nil, want the loaded read model")
	}
	move, err := provider.Move("move-9001")
	if err != nil {
		t.Fatalf("Move(move-9001) error = %v", err)
	}
	want := balance.Move{MoveID: "move-9001", Type: balance.TypeFire, Category: balance.MoveCategorySpecial}
	if move != want {
		t.Errorf("Move(move-9001) = %+v, want %+v", move, want)
	}
	if _, err := provider.Move("move-9999"); !errors.Is(err, balance.ErrUnknownMove) {
		t.Errorf("Move(move-9999) error = %v, want ErrUnknownMove", err)
	}
}

func TestMoveProviderFromEnvFailsOnBadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"stellar","category":"special"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "invalid read model", path: invalid, wantErr: master.ErrInvalidMoves},
		{name: "missing file", path: filepath.Join(dir, "missing.json")},
		{name: "directory", path: dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := moveProviderFromEnv(lookupFrom(map[string]string{movesPathEnv: tt.path}))
			if err == nil {
				t.Fatalf("error = nil, want startup failure (provider=%v)", provider)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if provider != nil {
				t.Errorf("provider = %v, want nil interface on error", provider)
			}
		})
	}
}
