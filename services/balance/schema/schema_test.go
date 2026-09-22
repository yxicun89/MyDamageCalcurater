// Package schema holds the JSON Schema (draft 2020-12) definitions of the balance read
// models (services/balance/README.md "read model schema"). These tests are the only
// consumer of github.com/santhosh-tekuri/jsonschema/v6 in this module: production code
// never validates against these schemas at runtime, the loaders in internal/master do.
package schema

import (
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compile(t *testing.T, schemaPath string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile %s: %v", schemaPath, err)
	}
	return sch
}

func validateFile(t *testing.T, sch *jsonschema.Schema, path string) error {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	inst, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return sch.Validate(inst)
}

func validateString(t *testing.T, sch *jsonschema.Schema, doc string) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("unmarshal %q: %v", doc, err)
	}
	return sch.Validate(inst)
}

// TestExamplesSatisfySchema checks (a): every fictional example the loaders already accept
// (testdata/*.example.json), and the real embedded type chart (internal/master/data/
// typechart.json, byte-identical to testdata/golden/typechart.json per ADR-0015), validate
// against the schema meant to describe the same shape.
func TestExamplesSatisfySchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		schema  string
		example string
	}{
		{"pokemon-types.schema.json", "../testdata/pokemon-types.example.json"},
		{"moves.schema.json", "../testdata/moves.example.json"},
		{"abilities.schema.json", "../testdata/abilities.example.json"},
		{"type-chart.schema.json", "../internal/master/data/typechart.json"},
	}
	for _, tt := range tests {
		t.Run(tt.schema, func(t *testing.T) {
			t.Parallel()
			sch := compile(t, tt.schema)
			if err := validateFile(t, sch, tt.example); err != nil {
				t.Errorf("%s does not satisfy %s: %v", tt.example, tt.schema, err)
			}
		})
	}
}

// TestPokemonTypesSchemaRejectsInvalidDocuments checks (b): representative documents the
// loader (master.LoadPokemonTypes) already rejects are also rejected by the schema. The
// literal documents mirror internal/master/pokemon_types_test.go's "unknown entry field",
// "schemaVersion 2" and "three types" cases.
func TestPokemonTypesSchemaRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	sch := compile(t, "pokemon-types.schema.json")
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown entry field", `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","name":"x","types":["fire"]}]}`},
		{"schemaVersion 2", `{"schemaVersion":2,"pokemon":[{"pokemonId":"9001-000","types":["fire"]}]}`},
		{"three types", `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire","water","grass"]}]}`},
		// ADR-0401 §5 (2026-09-22 update): the abilityIds limit moved from 3 to 4, so 5 is now the boundary.
		{"five abilityIds", `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"],"abilityIds":["ability-9001","ability-9002","ability-9003","ability-9004","ability-9005"]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateString(t, sch, tt.doc); err == nil {
				t.Errorf("document must be rejected by the schema: %s", tt.doc)
			}
		})
	}
}

// ADR-0401 §5 (2026-09-22 update): the schema must accept the new limit of 4 abilityIds
// (some species have four ability slots), matching internal/master's loader.
func TestPokemonTypesSchemaAcceptsFourAbilityIDs(t *testing.T) {
	t.Parallel()

	sch := compile(t, "pokemon-types.schema.json")
	doc := `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"],"abilityIds":["ability-9001","ability-9002","ability-9003","ability-9004"]}]}`
	if err := validateString(t, sch, doc); err != nil {
		t.Errorf("four abilityIds must be accepted by the schema: %v", err)
	}
}

// TestMovesSchemaRejectsInvalidDocuments mirrors internal/master/moves_test.go's
// "unknown entry field", "schemaVersion 2" and "unknown category" cases.
func TestMovesSchemaRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	sch := compile(t, "moves.schema.json")
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown entry field", `{"schemaVersion":1,"moves":[{"moveId":"move-9001","name":"x","type":"fire","category":"special"}]}`},
		{"schemaVersion 2", `{"schemaVersion":2,"moves":[{"moveId":"move-9001","type":"fire","category":"special"}]}`},
		{"unknown category", `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"other"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateString(t, sch, tt.doc); err == nil {
				t.Errorf("document must be rejected by the schema: %s", tt.doc)
			}
		})
	}
}

// TestAbilitiesSchemaRejectsInvalidDocuments mirrors internal/master/abilities_test.go's
// "unknown ability field", "schemaVersion 2" and "unknown kind" cases, plus the per-kind
// required/forbidden field combinations of ADR-0017 §2.
func TestAbilitiesSchemaRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	sch := compile(t, "abilities.schema.json")
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown ability field", `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","name":"x","effects":[]}]}`},
		{"schemaVersion 2", `{"schemaVersion":2,"abilities":[{"abilityId":"ability-9001","effects":[]}]}`},
		{"unknown kind", `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"heal","attackType":"ground"}]}]}`},
		{"immune without attackType", `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"immune"}]}]}`},
		{"immune with numerator", `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"ground","numerator":1,"denominator":1}]}]}`},
		{"type_multiplier without numerator", `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"type_multiplier","attackType":"fire","denominator":2}]}]}`},
		{"super_effective_multiplier with attackType", `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"super_effective_multiplier","attackType":"fire","numerator":3,"denominator":4}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateString(t, sch, tt.doc); err == nil {
				t.Errorf("document must be rejected by the schema: %s", tt.doc)
			}
		})
	}
}

// minimalTypeChart mirrors internal/master/type_chart_data_test.go's minimalChart helper,
// so the representative documents below match the loader's own fixtures.
func minimalTypeChart(schemaVersion, extraTopLevel, effectiveness string) string {
	return `{"schemaVersion":` + schemaVersion + extraTopLevel +
		`,"source":"fixture","version":"0","generation":9,"note":"test","excludedTypes":[],` +
		`"types":["bug","dark","dragon","electric","fairy","fighting","fire","flying","ghost","grass","ground","ice","normal","poison","psychic","rock","steel","water"],` +
		`"effectiveness":{` + effectiveness + `}}`
}

// TestTypeChartSchemaRejectsInvalidDocuments mirrors internal/master/type_chart_data_test.go's
// "unknown field", "schema version 2" and "invalid code 3" cases.
func TestTypeChartSchemaRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	sch := compile(t, "type-chart.schema.json")
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown field", minimalTypeChart("1", `,"extra":true`, `"fire":{"grass":4}`)},
		{"schemaVersion 2", minimalTypeChart("2", "", `"fire":{"grass":4}`)},
		{"invalid code 3", minimalTypeChart("1", "", `"fire":{"grass":3}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateString(t, sch, tt.doc); err == nil {
				t.Errorf("document must be rejected by the schema: %s", tt.doc)
			}
		})
	}
}
