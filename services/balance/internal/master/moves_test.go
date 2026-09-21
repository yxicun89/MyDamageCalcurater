package master

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB2 の技の read model(ADR-0016 §3)。

const exampleMovesPath = "../../testdata/moves.example.json"

// overlayMovesPath is the copy the local overlay mounts as a ConfigMap (same reason as
// overlayPokemonTypesPath). testdata/moves.example.json is the source of truth.
const overlayMovesPath = "../../deploy/k8s/overlays/local/moves.example.json"

func TestMovesSchemaVersion(t *testing.T) {
	t.Parallel()

	if MovesSchemaVersion != 1 {
		t.Fatalf("MovesSchemaVersion = %d, want 1 (ADR-0016 §3)", MovesSchemaVersion)
	}
}

func TestOverlayMovesExampleMatchesTestdataExample(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(exampleMovesPath)
	if err != nil {
		t.Fatalf("read %s: %v", exampleMovesPath, err)
	}
	got, err := os.ReadFile(overlayMovesPath)
	if err != nil {
		t.Fatalf("read %s: %v (the local overlay must mount the move example, ADR-0016 §3)", overlayMovesPath, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s is out of sync with %s (the source of truth); copy it over", overlayMovesPath, exampleMovesPath)
	}
}

func TestLoadMoves(t *testing.T) {
	t.Parallel()

	input := `{"schemaVersion":1,"moves":[
		{"moveId":"move-9001","type":"fire","category":"special"},
		{"moveId":"move-9002","type":"fire","category":"physical"},
		{"moveId":"move-9006","type":"grass","category":"status"},
		{"moveId":"9007","type":"ground","category":"physical"},
		{"moveId":"x","type":"normal","category":"physical"}
	]}`
	model, err := LoadMoves(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadMoves() error = %v", err)
	}
	if model == nil {
		t.Fatal("LoadMoves() returned nil model without error")
	}

	tests := []balance.Move{
		{MoveID: "move-9001", Type: balance.TypeFire, Category: balance.MoveCategorySpecial},
		{MoveID: "move-9002", Type: balance.TypeFire, Category: balance.MoveCategoryPhysical},
		{MoveID: "move-9006", Type: balance.TypeGrass, Category: balance.MoveCategoryStatus},
		{MoveID: "9007", Type: balance.TypeGround, Category: balance.MoveCategoryPhysical},
		{MoveID: "x", Type: balance.TypeNormal, Category: balance.MoveCategoryPhysical},
	}
	for _, want := range tests {
		got, err := model.Move(want.MoveID)
		if err != nil {
			t.Errorf("Move(%s) error = %v", want.MoveID, err)
			continue
		}
		if got != want {
			t.Errorf("Move(%s) = %+v, want %+v", want.MoveID, got, want)
		}
	}
}

func TestLoadMovesAcceptsFortyCharacterID(t *testing.T) {
	t.Parallel()

	id := "move-9099-" + strings.Repeat("a", 30)
	if len(id) != 40 {
		t.Fatalf("test ID length = %d, want 40", len(id))
	}
	model, err := LoadMoves(strings.NewReader(`{"schemaVersion":1,"moves":[{"moveId":"` + id + `","type":"fire","category":"special"}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadMoves() = %v, %v, want a 40-character moveId to be accepted", model, err)
	}
	if got, err := model.Move(id); err != nil || got.MoveID != id {
		t.Errorf("Move(%s) = %+v, %v", id, got, err)
	}
}

func TestMoveReadModelUnknownMove(t *testing.T) {
	t.Parallel()

	model, err := LoadMoves(strings.NewReader(`{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"special"}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadMoves() = %v, %v", model, err)
	}
	for _, id := range []string{"move-9002", "move-9999", "MOVE-9001", "move-9001 ", ""} {
		got, err := model.Move(id)
		if !errors.Is(err, balance.ErrUnknownMove) {
			t.Errorf("Move(%q) error = %v, want balance.ErrUnknownMove", id, err)
		}
		if got != (balance.Move{}) {
			t.Errorf("Move(%q) = %+v, want the zero Move on error", id, got)
		}
	}
}

func TestLoadMovesRejectsInvalidReadModel(t *testing.T) {
	t.Parallel()

	const fire = `{"moveId":"move-9001","type":"fire","category":"special"}`
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty input", input: ``},
		{name: "broken JSON", input: `{"schemaVersion":1,"moves":[`},
		{name: "not an object", input: `[]`},
		{name: "null document", input: `null`},
		{name: "trailing JSON", input: `{"schemaVersion":1,"moves":[` + fire + `]} {}`},
		{name: "missing schemaVersion", input: `{"moves":[` + fire + `]}`},
		{name: "schemaVersion 0", input: `{"schemaVersion":0,"moves":[` + fire + `]}`},
		{name: "schemaVersion 2", input: `{"schemaVersion":2,"moves":[` + fire + `]}`},
		{name: "schemaVersion string", input: `{"schemaVersion":"1","moves":[` + fire + `]}`},
		{name: "missing moves key", input: `{"schemaVersion":1}`},
		{name: "null moves", input: `{"schemaVersion":1,"moves":null}`},
		{name: "empty moves array", input: `{"schemaVersion":1,"moves":[]}`},
		{name: "unknown top-level field", input: `{"schemaVersion":1,"source":"x","moves":[` + fire + `]}`},
		{name: "unknown entry field", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","name":"x","type":"fire","category":"special"}]}`},
		{name: "power field is not part of the schema", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"special","power":90}]}`},
		{name: "missing moveId", input: `{"schemaVersion":1,"moves":[{"type":"fire","category":"special"}]}`},
		{name: "empty moveId", input: `{"schemaVersion":1,"moves":[{"moveId":"","type":"fire","category":"special"}]}`},
		{name: "uppercase moveId", input: `{"schemaVersion":1,"moves":[{"moveId":"Move-9001","type":"fire","category":"special"}]}`},
		{name: "underscore in moveId", input: `{"schemaVersion":1,"moves":[{"moveId":"move_9001","type":"fire","category":"special"}]}`},
		{name: "space in moveId", input: `{"schemaVersion":1,"moves":[{"moveId":"move 9001","type":"fire","category":"special"}]}`},
		{name: "surrounding space in moveId", input: `{"schemaVersion":1,"moves":[{"moveId":" move-9001","type":"fire","category":"special"}]}`},
		{name: "leading hyphen", input: `{"schemaVersion":1,"moves":[{"moveId":"-move-9001","type":"fire","category":"special"}]}`},
		{name: "trailing hyphen", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001-","type":"fire","category":"special"}]}`},
		{name: "double hyphen", input: `{"schemaVersion":1,"moves":[{"moveId":"move--9001","type":"fire","category":"special"}]}`},
		{name: "moveId of 41 characters", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9099-` + strings.Repeat("a", 31) + `","type":"fire","category":"special"}]}`},
		{name: "duplicate moveId", input: `{"schemaVersion":1,"moves":[` + fire + `,{"moveId":"move-9001","type":"water","category":"physical"}]}`},
		{name: "missing type", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","category":"special"}]}`},
		{name: "empty type", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"","category":"special"}]}`},
		{name: "unknown type", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"stellar","category":"special"}]}`},
		{name: "uppercase type", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"Fire","category":"special"}]}`},
		{name: "type array", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":["fire"],"category":"special"}]}`},
		{name: "missing category", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire"}]}`},
		{name: "empty category", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":""}]}`},
		{name: "unknown category", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"other"}]}`},
		{name: "uppercase category", input: `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"Special"}]}`},
		{name: "invalid entry after valid ones", input: `{"schemaVersion":1,"moves":[` + fire + `,{"moveId":"move-9002","type":"fire","category":"other"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, err := LoadMoves(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("LoadMoves() error = nil, want error (model=%v)", model)
			}
			if !errors.Is(err, ErrInvalidMoves) {
				t.Errorf("LoadMoves() error = %v, want errors.Is(_, ErrInvalidMoves)", err)
			}
			if model != nil {
				t.Errorf("LoadMoves() returned a partial model on error")
			}
		})
	}
}

func TestLoadMovesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	invalid := filepath.Join(dir, "invalid.json")
	empty := filepath.Join(dir, "empty.json")
	writeFile(t, valid, `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"special"}]}`)
	writeFile(t, invalid, `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"other"}]}`)
	writeFile(t, empty, ``)

	model, err := LoadMovesFile(valid)
	if err != nil || model == nil {
		t.Fatalf("LoadMovesFile(valid) = %v, %v", model, err)
	}
	want := balance.Move{MoveID: "move-9001", Type: balance.TypeFire, Category: balance.MoveCategorySpecial}
	if got, err := model.Move("move-9001"); err != nil || got != want {
		t.Errorf("Move(move-9001) = %+v, %v, want %+v", got, err, want)
	}

	for _, path := range []string{invalid, empty} {
		if model, err := LoadMovesFile(path); !errors.Is(err, ErrInvalidMoves) || model != nil {
			t.Errorf("LoadMovesFile(%s) = %v, %v, want nil and ErrInvalidMoves", filepath.Base(path), model, err)
		}
	}
	missing := filepath.Join(dir, "missing.json")
	if model, err := LoadMovesFile(missing); err == nil || model != nil {
		t.Errorf("LoadMovesFile(missing) = %v, %v, want error", model, err)
	}
	if model, err := LoadMovesFile(dir); err == nil || model != nil {
		t.Errorf("LoadMovesFile(directory) = %v, %v, want error", model, err)
	}
}

// TestExampleMovesIsValidAndFictional guards the committed example (ADR-0002/0016):
// it must load, use only fictional IDs from move-9001, and cover physical/special/status,
// several moves of one attack type, and both an immunity and a super effective matchup.
func TestExampleMovesIsValidAndFictional(t *testing.T) {
	t.Parallel()

	model, err := LoadMovesFile(exampleMovesPath)
	if err != nil || model == nil {
		t.Fatalf("LoadMovesFile(example) = %v, %v", model, err)
	}

	raw, err := os.ReadFile(exampleMovesPath)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	allIDs := regexp.MustCompile(`"moveId"\s*:\s*"([^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(allIDs) == 0 {
		t.Fatal("example has no moveId entries")
	}
	fictional := regexp.MustCompile(`^move-(\d+)$`)
	for _, id := range allIDs {
		match := fictional.FindStringSubmatch(id[1])
		if match == nil {
			t.Errorf("example moveId %q must be a fictional move-NNNN ID", id[1])
			continue
		}
		if n, err := strconv.Atoi(match[1]); err != nil || n < 9001 {
			t.Errorf("example moveId %q must be fictional (>= move-9001)", id[1])
		}
	}
	for _, forbidden := range []string{`"name"`, `"power"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("example must not contain %s (only fictional IDs, types and categories)", forbidden)
		}
	}

	chart, err := EmbeddedTypeChart()
	if err != nil {
		t.Fatalf("EmbeddedTypeChart() error = %v", err)
	}
	categories := map[balance.MoveCategory]bool{}
	attackTypeCount := map[balance.TypeID]int{}
	var immune, superEffective bool
	for _, id := range allIDs {
		move, err := model.Move(id[1])
		if err != nil {
			t.Fatalf("Move(%s) error = %v", id[1], err)
		}
		categories[move.Category] = true
		if move.Category == balance.MoveCategoryStatus {
			continue
		}
		attackTypeCount[move.Type]++
		for _, defense := range balance.AllTypes() {
			m, err := chart.Matchup(move.Type, defense)
			if err != nil {
				t.Fatalf("Matchup(%s, %s) error = %v", move.Type, defense, err)
			}
			immune = immune || m == balance.MultiplierZero
			superEffective = superEffective || m == balance.MultiplierDouble
		}
	}
	var sameType bool
	for _, n := range attackTypeCount {
		sameType = sameType || n >= 2
	}
	physical, special, status := categories[balance.MoveCategoryPhysical], categories[balance.MoveCategorySpecial], categories[balance.MoveCategoryStatus]
	if !physical || !special || !status || !sameType || !immune || !superEffective {
		t.Errorf("example must cover physical=%v special=%v status=%v sameTypeAttacks=%v immune=%v superEffective=%v (all true)",
			physical, special, status, sameType, immune, superEffective)
	}
}

func TestMoveReadModelLookupIsExact(t *testing.T) {
	t.Parallel()

	model, err := LoadMoves(strings.NewReader(`{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"special"},{"moveId":"move-90010","type":"water","category":"physical"}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadMoves() = %v, %v", model, err)
	}
	got, err := model.Move("move-90010")
	if err != nil || got.Type != balance.TypeWater {
		t.Errorf("Move(move-90010) = %+v, %v, want water", got, err)
	}
	if got, err := model.Move("move-9001"); err != nil || fmt.Sprint(got.Type) != "fire" {
		t.Errorf("Move(move-9001) = %+v, %v, want fire", got, err)
	}
}
