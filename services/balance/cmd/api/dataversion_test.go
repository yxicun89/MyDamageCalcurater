package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/services/balance/internal/master"
)

func TestDataVersionFromEnv(t *testing.T) {
	t.Parallel()

	modelPath := func(t *testing.T, metadata string) string {
		t.Helper()
		dir := t.TempDir()
		if metadata != "" {
			if err := os.WriteFile(filepath.Join(dir, master.MetadataFile), []byte(metadata), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(dir, "pokemon-types.json")
	}

	t.Run("unset path is unknown", func(t *testing.T) {
		t.Parallel()
		got, err := dataVersionFromEnv(lookupFrom(map[string]string{}))
		if err != nil || got != "" {
			t.Fatalf("got (%q, %v), want (\"\", nil)", got, err)
		}
	})
	t.Run("metadata next to the read model", func(t *testing.T) {
		t.Parallel()
		path := modelPath(t, `{"schemaVersion":1,"dataVersion":"v1-abcd1234"}`)
		got, err := dataVersionFromEnv(lookupFrom(map[string]string{pokemonTypesPathEnv: path}))
		if err != nil || got != "v1-abcd1234" {
			t.Fatalf("got (%q, %v)", got, err)
		}
	})
	t.Run("missing metadata is unknown", func(t *testing.T) {
		t.Parallel()
		path := modelPath(t, "")
		got, err := dataVersionFromEnv(lookupFrom(map[string]string{pokemonTypesPathEnv: path}))
		if err != nil || got != "" {
			t.Fatalf("got (%q, %v)", got, err)
		}
	})
	t.Run("broken metadata fails", func(t *testing.T) {
		t.Parallel()
		path := modelPath(t, `{`)
		_, err := dataVersionFromEnv(lookupFrom(map[string]string{pokemonTypesPathEnv: path}))
		if !errors.Is(err, master.ErrInvalidMetadata) {
			t.Fatalf("error = %v, want ErrInvalidMetadata", err)
		}
	})
}
