package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exampleDir copies the fictional examples into a temp dir under the read model file names.
func exampleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range map[string]string{
		pokemonTypesFile: "../../testdata/pokemon-types.example.json",
		movesFile:        "../../testdata/moves.example.json",
		abilitiesFile:    "../../testdata/abilities.example.json",
	} {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestCheckAcceptsExamples(t *testing.T) {
	t.Parallel()

	summary, err := check(exampleDir(t))
	if err != nil {
		t.Fatalf("check() error = %v, want ok", err)
	}
	// The example pokemon-types.example.json lists a known, fixed number of pokemon (ADR-0401).
	wantPokemonCount, err := os.ReadFile("../../testdata/pokemon-types.example.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	if !strings.Contains(string(wantPokemonCount), `"pokemonId"`) {
		t.Fatalf("example fixture looks empty")
	}
	if !strings.HasPrefix(summary, "read model ok: ") || !strings.HasSuffix(summary, " pokemon") {
		t.Errorf("summary = %q, want the form %q", summary, "read model ok: <n> pokemon")
	}
}

func TestCheckRejectsMissingOrInvalidFiles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{pokemonTypesFile, movesFile, abilitiesFile} {
		name := name
		t.Run("missing "+name, func(t *testing.T) {
			t.Parallel()
			dir := exampleDir(t)
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			if _, err := check(dir); err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("check() error = %v, want an error naming %s", err, name)
			}
		})
		for _, invalid := range []struct {
			label string
			body  string
		}{
			{"wrong schemaVersion", `{"schemaVersion":2}`},
			{"malformed JSON", `{"schemaVersion":`},
		} {
			invalid := invalid
			t.Run("invalid "+name+" ("+invalid.label+")", func(t *testing.T) {
				t.Parallel()
				dir := exampleDir(t)
				if err := os.WriteFile(filepath.Join(dir, name), []byte(invalid.body), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := check(dir); err == nil || !strings.Contains(err.Error(), name) {
					t.Errorf("check() error = %v, want an error naming %s", err, name)
				}
			})
		}
	}
}
