package master

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

const examplePokemonTypesPath = "../../testdata/pokemon-types.example.json"

// overlayPokemonTypesPath is the copy the local overlay mounts as a ConfigMap
// (Kustomize's load restrictor forbids referencing a file outside the overlay
// directory tree). testdata/pokemon-types.example.json remains the single source
// of truth; this test keeps the copy from silently drifting.
const overlayPokemonTypesPath = "../../deploy/k8s/overlays/local/pokemon-types.example.json"

func TestOverlayExampleMatchesTestdataExample(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(examplePokemonTypesPath)
	if err != nil {
		t.Fatalf("read %s: %v", examplePokemonTypesPath, err)
	}
	got, err := os.ReadFile(overlayPokemonTypesPath)
	if err != nil {
		t.Fatalf("read %s: %v", overlayPokemonTypesPath, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s is out of sync with %s (the source of truth); copy it over", overlayPokemonTypesPath, examplePokemonTypesPath)
	}
}

func TestLoadPokemonTypes(t *testing.T) {
	t.Parallel()

	input := `{"schemaVersion":1,"pokemon":[
		{"pokemonId":"9001-000","types":["fire","flying"]},
		{"pokemonId":"9002-000","types":["grass"]},
		{"pokemonId":"9002-001","types":["water","ground"]}
	]}`
	model, err := LoadPokemonTypes(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadPokemonTypes() error = %v", err)
	}
	if model == nil {
		t.Fatal("LoadPokemonTypes() returned nil model without error")
	}

	tests := []struct {
		id   string
		want []balance.TypeID
	}{
		{"9001-000", []balance.TypeID{balance.TypeFire, balance.TypeFlying}},
		{"9002-000", []balance.TypeID{balance.TypeGrass}},
		{"9002-001", []balance.TypeID{balance.TypeWater, balance.TypeGround}},
	}
	for _, tt := range tests {
		got, err := model.PokemonTypes(tt.id)
		if err != nil {
			t.Errorf("PokemonTypes(%s) error = %v", tt.id, err)
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("PokemonTypes(%s) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

// TestLoadPokemonTypesPreservesReadModelOrder guards ADR-0014 §5.1: the returned
// types must stay in the order the read model listed them, not the canonical
// (normal ... fairy) order used elsewhere in the response.
func TestLoadPokemonTypesPreservesReadModelOrder(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemonTypes(strings.NewReader(`{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["flying","fire"]}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypes() = %v, %v", model, err)
	}
	got, err := model.PokemonTypes("9001-000")
	if err != nil {
		t.Fatalf("PokemonTypes() error = %v", err)
	}
	want := []balance.TypeID{balance.TypeFlying, balance.TypeFire}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("PokemonTypes() = %v, want %v (read model order, not canonical order)", got, want)
	}
}

func TestPokemonTypeReadModelUnknownPokemon(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemonTypes(strings.NewReader(`{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypes() = %v, %v", model, err)
	}
	for _, id := range []string{"9001-001", "9999-000", ""} {
		if _, err := model.PokemonTypes(id); !errors.Is(err, balance.ErrUnknownPokemon) {
			t.Errorf("PokemonTypes(%q) error = %v, want balance.ErrUnknownPokemon", id, err)
		}
	}
}

func TestPokemonTypeReadModelReturnsIndependentSlices(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemonTypes(strings.NewReader(`{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire","flying"]}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypes() = %v, %v", model, err)
	}
	first, err := model.PokemonTypes("9001-000")
	if err != nil || len(first) != 2 {
		t.Fatalf("PokemonTypes() = %v, %v", first, err)
	}
	first[0] = balance.TypeWater
	second, err := model.PokemonTypes("9001-000")
	if err != nil {
		t.Fatalf("PokemonTypes() error = %v", err)
	}
	if second[0] != balance.TypeFire {
		t.Errorf("mutating a returned slice changed the read model: %v", second)
	}
}

func TestLoadPokemonTypesRejectsInvalidReadModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty input", input: ``},
		{name: "broken JSON", input: `{"schemaVersion":1,"pokemon":[`},
		{name: "not an object", input: `[]`},
		{name: "trailing JSON", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"]}]} {}`},
		{name: "missing schemaVersion", input: `{"pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`},
		{name: "missing pokemon key", input: `{"schemaVersion":1}`},
		{name: "empty pokemon array", input: `{"schemaVersion":1,"pokemon":[]}`},
		{name: "schemaVersion 0", input: `{"schemaVersion":0,"pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`},
		{name: "schemaVersion 2", input: `{"schemaVersion":2,"pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`},
		{name: "schemaVersion string", input: `{"schemaVersion":"1","pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`},
		{name: "unknown top-level field", input: `{"schemaVersion":1,"source":"x","pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`},
		{name: "unknown entry field", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","name":"x","types":["fire"]}]}`},
		{name: "ID without form suffix", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001","types":["fire"]}]}`},
		{name: "ID with letters", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"abcd-000","types":["fire"]}]}`},
		{name: "ID too long", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"09001-000","types":["fire"]}]}`},
		{name: "ID with surrounding space", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":" 9001-000","types":["fire"]}]}`},
		{name: "empty ID", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"","types":["fire"]}]}`},
		{name: "duplicate ID", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"]},{"pokemonId":"9001-000","types":["water"]}]}`},
		{name: "zero types", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":[]}]}`},
		{name: "missing types", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000"}]}`},
		{name: "three types", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire","water","grass"]}]}`},
		{name: "unknown type", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["stellar"]}]}`},
		{name: "uppercase type", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["Fire"]}]}`},
		{name: "empty type", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":[""]}]}`},
		{name: "duplicate type", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire","fire"]}]}`},
		{name: "invalid entry after valid ones", input: `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"]},{"pokemonId":"9002-000","types":["stellar"]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, err := LoadPokemonTypes(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("LoadPokemonTypes() error = nil, want error (model=%v)", model)
			}
			if !errors.Is(err, ErrInvalidPokemonTypes) {
				t.Errorf("LoadPokemonTypes() error = %v, want errors.Is(_, ErrInvalidPokemonTypes)", err)
			}
			if model != nil {
				t.Errorf("LoadPokemonTypes() returned a partial model on error")
			}
		})
	}
}

func TestLoadPokemonTypesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	invalid := filepath.Join(dir, "invalid.json")
	empty := filepath.Join(dir, "empty.json")
	writeFile(t, valid, `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire","flying"]}]}`)
	writeFile(t, invalid, `{"schemaVersion":2,"pokemon":[]}`)
	writeFile(t, empty, ``)

	model, err := LoadPokemonTypesFile(valid)
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypesFile(valid) = %v, %v", model, err)
	}
	if got, err := model.PokemonTypes("9001-000"); err != nil || fmt.Sprint(got) != fmt.Sprint([]balance.TypeID{balance.TypeFire, balance.TypeFlying}) {
		t.Errorf("PokemonTypes(9001-000) = %v, %v", got, err)
	}

	for _, path := range []string{invalid, empty} {
		if _, err := LoadPokemonTypesFile(path); !errors.Is(err, ErrInvalidPokemonTypes) {
			t.Errorf("LoadPokemonTypesFile(%s) error = %v, want ErrInvalidPokemonTypes", filepath.Base(path), err)
		}
	}
	missing := filepath.Join(dir, "missing.json")
	if model, err := LoadPokemonTypesFile(missing); err == nil || model != nil {
		t.Errorf("LoadPokemonTypesFile(missing) = %v, %v, want error", model, err)
	}
	if _, err := LoadPokemonTypesFile(dir); err == nil {
		t.Error("LoadPokemonTypesFile(directory) error = nil, want error")
	}
}

// TestExamplePokemonTypesIsValidAndFictional guards the committed example (ADR-0002/0014):
// it must load, use only fictional IDs from 9001-000, and cover single/dual types.
func TestExamplePokemonTypesIsValidAndFictional(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemonTypesFile(examplePokemonTypesPath)
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypesFile(example) = %v, %v", model, err)
	}

	raw, err := os.ReadFile(examplePokemonTypesPath)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	ids := regexp.MustCompile(`"pokemonId"\s*:\s*"((\d{4})-\d{3})"`).FindAllStringSubmatch(string(raw), -1)
	if len(ids) == 0 {
		t.Fatal("example has no pokemonId entries")
	}
	for _, id := range ids {
		if id[2] < "9001" {
			t.Errorf("example pokemonId %s must be fictional (>= 9001-000)", id[1])
		}
	}

	var single, dual bool
	chart, err := EmbeddedTypeChart()
	if err != nil {
		t.Fatalf("EmbeddedTypeChart() error = %v", err)
	}
	var quadWeak, immune bool
	for _, id := range ids {
		pokemonID := id[1]
		types, err := model.PokemonTypes(pokemonID)
		if err != nil {
			t.Fatalf("PokemonTypes(%s) error = %v", pokemonID, err)
		}
		switch len(types) {
		case 1:
			single = true
		case 2:
			dual = true
		}
		for _, attack := range balance.AllTypes() {
			result, err := balance.CalculateDefense(chart, attack, types)
			if err != nil {
				t.Fatalf("CalculateDefense(%s, %v) error = %v", attack, types, err)
			}
			quadWeak = quadWeak || result.Multiplier == balance.MultiplierQuad
			immune = immune || result.Multiplier == balance.MultiplierZero
		}
	}
	if !single || !dual || !quadWeak || !immune {
		t.Errorf("example must cover single=%v dual=%v quadWeak=%v immune=%v (all true)", single, dual, quadWeak, immune)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
