package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/services/speed/internal/master"
)

const examplePokemonPath = "../../testdata/pokemon.example.json"

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

func TestEnvNames(t *testing.T) {
	t.Parallel()

	if pokemonPathEnv != "SPEED_POKEMON_PATH" {
		t.Errorf("pokemonPathEnv = %q, want SPEED_POKEMON_PATH (ADR-0600 §4)", pokemonPathEnv)
	}
	if portEnv != "PORT" {
		t.Errorf("portEnv = %q, want PORT", portEnv)
	}
}

func TestPokemonProviderFromEnvUnsetOrEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
	}{
		{"未設定", map[string]string{}},
		// Kubernetes が未設定の変数を空文字で渡すことがあるので、空文字は未設定と同じに扱う(balance と同じ)。
		{"空文字", map[string]string{pokemonPathEnv: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := pokemonProviderFromEnv(lookupFrom(tt.env))
			if err != nil {
				t.Fatalf("error = %v, want nil (the service starts without the read model)", err)
			}
			// 型なしの nil であること。型付きの nil ポインタだと 503 の分岐を素通りする。
			if provider != nil {
				t.Fatalf("provider = %#v, want a nil interface", provider)
			}
		})
	}
}

func TestPokemonProviderFromEnvLoadsExample(t *testing.T) {
	t.Parallel()

	provider, err := pokemonProviderFromEnv(lookupFrom(map[string]string{pokemonPathEnv: examplePokemonPath}))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if provider == nil {
		t.Fatal("provider = nil, want the loaded read model")
	}
	roster, err := provider.Roster()
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	if roster.RegulationID != "example" || len(roster.Pokemon) == 0 {
		t.Errorf("Roster() = %+v, want the example read model", roster)
	}
}

func TestPokemonProviderFromEnvFailsOnBadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"schemaVersion":1,"regulationId":"example","pokemon":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "不正な read model", path: invalid, wantErr: master.ErrInvalidPokemon},
		{name: "存在しないファイル", path: filepath.Join(dir, "missing.json"), wantErr: os.ErrNotExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := pokemonProviderFromEnv(lookupFrom(map[string]string{pokemonPathEnv: tt.path}))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want errors.Is(_, %v) (startup must fail)", err, tt.wantErr)
			}
			if provider != nil {
				t.Errorf("provider = %#v, want a nil interface on error", provider)
			}
		})
	}
}

func TestPortFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"未設定は 8080", map[string]string{}, "8080"},
		{"空文字は 8080", map[string]string{portEnv: ""}, "8080"},
		{"設定値を使う", map[string]string{portEnv: "9090"}, "9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := portFromEnv(lookupFrom(tt.env)); got != tt.want {
				t.Errorf("portFromEnv = %q, want %q", got, tt.want)
			}
		})
	}
}
