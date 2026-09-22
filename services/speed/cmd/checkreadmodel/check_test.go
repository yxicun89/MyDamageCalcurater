package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/master"
)

// examplePath は架空データの read model の正(ADR-0600 §4)。pokedex export の speed-pokemon.json と同じ形。
const examplePath = "../../testdata/pokemon.example.json"

// exampleDir は架空データの example を、read model のファイル名で一時ディレクトリに置く(balance と同じやり方)。
func exampleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read %s: %v", examplePath, err)
	}
	if err := os.WriteFile(filepath.Join(dir, pokemonReadModelFile), data, 0o600); err != nil {
		t.Fatalf("write %s: %v", pokemonReadModelFile, err)
	}
	return dir
}

// examplePokemonCount は example に入っているポケモンの数を、check とは別に数える。
func examplePokemonCount(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read %s: %v", examplePath, err)
	}
	var file struct {
		Pokemon []json.RawMessage `json:"pokemon"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal %s: %v", examplePath, err)
	}
	if len(file.Pokemon) == 0 {
		t.Fatalf("%s looks empty", examplePath)
	}
	return len(file.Pokemon)
}

// 受け入れ条件 1: 正しい read model のディレクトリなら check は成功し、"read model ok: N pokemon" を返す。
func TestCheckAcceptsExample(t *testing.T) {
	t.Parallel()

	summary, err := check(exampleDir(t))
	if err != nil {
		t.Fatalf("check() error = %v, want ok", err)
	}
	if !strings.HasPrefix(summary, "read model ok: ") || !strings.HasSuffix(summary, " pokemon") {
		t.Fatalf("summary = %q, want the form %q", summary, "read model ok: <n> pokemon")
	}
	if want := fmt.Sprintf("read model ok: %d pokemon", examplePokemonCount(t)); summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
	if strings.Contains(summary, "\n") {
		t.Errorf("summary = %q, want a single line", summary)
	}
}

// 受け入れ条件 2: ファイルが無ければエラー。ファイル名が分かるメッセージで、fs.ErrNotExist を包む。
func TestCheckRejectsMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	summary, err := check(dir)
	if err == nil {
		t.Fatalf("check() = %q, want an error for a directory without %s", summary, pokemonReadModelFile)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("check() error = %v, want it to wrap fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), pokemonReadModelFile) {
		t.Errorf("check() error = %v, want the message to name %s", err, pokemonReadModelFile)
	}
}

// 受け入れ条件 3: 不正な read model は master.LoadPokemonFile のエラー(ErrInvalidPokemon)がそのまま伝わる。
func TestCheckRejectsInvalidFile(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		label string
		body  string
	}{
		{"malformed JSON", `{"schemaVersion":`},
		{"wrong schemaVersion", `{"schemaVersion":2,"regulationId":"example","pokemon":[{"pokemonId":"9001-000","nameJa":"テストカソウドリ","types":["fire"],"baseSpeed":100}]}`},
		{"empty pokemon", `{"schemaVersion":1,"regulationId":"example","pokemon":[]}`},
		{"unknown field", `{"schemaVersion":1,"regulationId":"example","pokemon":[],"extra":1}`},
		{"invalid pokemonId", `{"schemaVersion":1,"regulationId":"example","pokemon":[{"pokemonId":"bad","nameJa":"テストカソウドリ","types":["fire"],"baseSpeed":100}]}`},
		{"baseSpeed out of range", `{"schemaVersion":1,"regulationId":"example","pokemon":[{"pokemonId":"9001-000","nameJa":"テストカソウドリ","types":["fire"],"baseSpeed":0}]}`},
	} {
		tt := tt
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()
			dir := exampleDir(t)
			if err := os.WriteFile(filepath.Join(dir, pokemonReadModelFile), []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			summary, err := check(dir)
			if err == nil {
				t.Fatalf("check() = %q, want an error for %s", summary, tt.label)
			}
			if !errors.Is(err, master.ErrInvalidPokemon) {
				t.Errorf("check() error = %v, want it to wrap master.ErrInvalidPokemon", err)
			}
			if !strings.Contains(err.Error(), pokemonReadModelFile) {
				t.Errorf("check() error = %v, want the message to name %s", err, pokemonReadModelFile)
			}
		})
	}
}

// 受け入れ条件 4: read model のファイル名は pokedex export の出力と同じ(ADR-0105 §120・ADR-0603 §1)。
func TestReadModelFileName(t *testing.T) {
	t.Parallel()

	if pokemonReadModelFile != "speed-pokemon.json" {
		t.Errorf("pokemonReadModelFile = %q, want speed-pokemon.json", pokemonReadModelFile)
	}
}
