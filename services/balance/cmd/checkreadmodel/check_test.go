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
	if err != nil || !strings.HasPrefix(summary, "read model ok: ") {
		t.Fatalf("check() = %q, %v; want ok", summary, err)
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
		t.Run("invalid "+name, func(t *testing.T) {
			t.Parallel()
			dir := exampleDir(t)
			if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := check(dir); err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("check() error = %v, want an error naming %s", err, name)
			}
		})
	}
}
