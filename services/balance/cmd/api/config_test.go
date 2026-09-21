package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
)

const examplePokemonTypesPath = "../../testdata/pokemon-types.example.json"

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

func TestPokemonTypesPathEnvName(t *testing.T) {
	t.Parallel()

	if pokemonTypesPathEnv != "BALANCE_POKEMON_TYPES_PATH" {
		t.Fatalf("pokemonTypesPathEnv = %q, want BALANCE_POKEMON_TYPES_PATH (ADR-0014)", pokemonTypesPathEnv)
	}
}

func TestPokemonTypeProviderFromEnvUnset(t *testing.T) {
	t.Parallel()

	provider, err := pokemonTypeProviderFromEnv(lookupFrom(map[string]string{}))
	if err != nil {
		t.Fatalf("error = %v, want nil when %s is unset", err, pokemonTypesPathEnv)
	}
	// Must be an untyped nil interface: a typed nil pointer would bypass the 503 path.
	if provider != nil {
		t.Fatalf("provider = %#v, want nil interface when %s is unset", provider, pokemonTypesPathEnv)
	}
}

func TestPokemonTypeProviderFromEnvLoadsExample(t *testing.T) {
	t.Parallel()

	provider, err := pokemonTypeProviderFromEnv(lookupFrom(map[string]string{pokemonTypesPathEnv: examplePokemonTypesPath}))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if provider == nil {
		t.Fatal("provider = nil, want the loaded read model")
	}
	types, err := provider.PokemonTypes("9001-000")
	if err != nil {
		t.Fatalf("PokemonTypes(9001-000) error = %v", err)
	}
	if fmt.Sprint(types) != fmt.Sprint([]balance.TypeID{balance.TypeFire, balance.TypeFlying}) {
		t.Errorf("PokemonTypes(9001-000) = %v, want [fire flying]", types)
	}
	if _, err := provider.PokemonTypes("9999-999"); !errors.Is(err, balance.ErrUnknownPokemon) {
		t.Errorf("PokemonTypes(9999-999) error = %v, want ErrUnknownPokemon", err)
	}
}

func TestPokemonTypeProviderFromEnvFailsOnBadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["stellar"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "invalid read model", path: invalid, wantErr: master.ErrInvalidPokemonTypes},
		{name: "missing file", path: filepath.Join(dir, "missing.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := pokemonTypeProviderFromEnv(lookupFrom(map[string]string{pokemonTypesPathEnv: tt.path}))
			if err == nil {
				t.Fatalf("error = nil, want startup failure (provider=%v)", provider)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if provider != nil {
				t.Errorf("provider = %v, want nil on error", provider)
			}
		})
	}
}
