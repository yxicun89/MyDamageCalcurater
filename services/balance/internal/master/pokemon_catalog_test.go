package master

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB5 のポケモン read model の拡張(ADR-0401 §5)。nameJa(1〜64 文字)と abilityIds(0〜3 件・重複なし・
// ADR-0017 §2 の ID 形式)は省略可能で、schemaVersion は 1 のまま。名前と ID はすべて架空。

const exampleAbilitiesPathForCatalog = "../../testdata/abilities.example.json"

var _ balance.PokemonCatalog = (*PokemonTypeReadModel)(nil)

func loadCatalog(t *testing.T, input string) *PokemonTypeReadModel {
	t.Helper()
	model, err := LoadPokemonTypes(strings.NewReader(input))
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypes() = %v, %v", model, err)
	}
	return model
}

func allPokemon(t *testing.T, model *PokemonTypeReadModel) []balance.CatalogPokemon {
	t.Helper()
	all, err := model.AllPokemon()
	if err != nil {
		t.Fatalf("AllPokemon() error = %v", err)
	}
	return all
}

// catalogView renders one entry as "id|nameJa|types|abilityIds" for comparison.
func catalogView(p balance.CatalogPokemon) string {
	return fmt.Sprintf("%s|%s|%v|%v", p.PokemonID, p.NameJa, p.Types, p.AbilityIDs)
}

// ADR-0401 §5: nameJa and abilityIds are read when present; AllPokemon lists every entry
// (pokemonId ascending) and PokemonTypes keeps working.
func TestLoadPokemonTypesReadsNameAndAbilities(t *testing.T) {
	t.Parallel()

	fortyCharAbilityID := "ability-9099-" + strings.Repeat("a", 27)
	sixtyFourRunes := strings.Repeat("カ", 64)
	model := loadCatalog(t, `{"schemaVersion":1,"pokemon":[
		{"pokemonId":"9003-000","nameJa":"`+sixtyFourRunes+`","types":["water","ground"],"abilityIds":["ability-9001","ability-9002","`+fortyCharAbilityID+`"]},
		{"pokemonId":"9001-000","nameJa":"カソウバード","types":["flying","fire"],"abilityIds":["ability-9002"]},
		{"pokemonId":"9002-001","types":["grass"],"abilityIds":[]},
		{"pokemonId":"9002-000","nameJa":"A","types":["grass"]}
	]}`)

	want := []string{
		"9001-000|カソウバード|[flying fire]|[ability-9002]",
		"9002-000|A|[grass]|[]",
		"9002-001||[grass]|[]",
		"9003-000|" + sixtyFourRunes + "|[water ground]|[ability-9001 ability-9002 " + fortyCharAbilityID + "]",
	}
	all := allPokemon(t, model)
	got := make([]string, len(all))
	for i, p := range all {
		got[i] = catalogView(p)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("AllPokemon() =\n%v\nwant (pokemonId ascending, read model type/ability order)\n%v", got, want)
	}

	if types, err := model.PokemonTypes("9001-000"); err != nil || fmt.Sprint(types) != "[flying fire]" {
		t.Errorf("PokemonTypes(9001-000) = %v, %v, want [flying fire]", types, err)
	}
}

// ADR-0401 §5: a schemaVersion 1 file without the new fields (the TB1〜4 files) still loads,
// and AllPokemon gives empty names and no abilities.
func TestLoadPokemonTypesWithoutOptionalFields(t *testing.T) {
	t.Parallel()

	model := loadCatalog(t, `{"schemaVersion":1,"pokemon":[
		{"pokemonId":"9002-000","types":["grass"]},
		{"pokemonId":"9001-000","types":["fire","flying"]}
	]}`)
	all := allPokemon(t, model)
	if len(all) != 2 {
		t.Fatalf("AllPokemon() = %+v, want 2 entries", all)
	}
	for i, wantID := range []string{"9001-000", "9002-000"} {
		p := all[i]
		if p.PokemonID != wantID || p.NameJa != "" || len(p.AbilityIDs) != 0 || len(p.Types) == 0 {
			t.Errorf("AllPokemon()[%d] = %+v, want %s without name or abilities", i, p, wantID)
		}
	}
}

func TestPokemonCatalogReturnsIndependentSlices(t *testing.T) {
	t.Parallel()

	model := loadCatalog(t, `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","nameJa":"カソウバード","types":["fire","flying"],"abilityIds":["ability-9001","ability-9002"]}]}`)
	first := allPokemon(t, model)
	if len(first) != 1 || len(first[0].Types) != 2 || len(first[0].AbilityIDs) != 2 {
		t.Fatalf("AllPokemon() = %+v", first)
	}
	first[0].Types[0] = balance.TypeWater
	first[0].AbilityIDs[0] = "ability-9999"
	first[0] = balance.CatalogPokemon{}

	second := allPokemon(t, model)
	if catalogView(second[0]) != "9001-000|カソウバード|[fire flying]|[ability-9001 ability-9002]" {
		t.Errorf("mutating a returned value changed the read model: %+v", second[0])
	}
	if types, err := model.PokemonTypes("9001-000"); err != nil || types[0] != balance.TypeFire {
		t.Errorf("PokemonTypes(9001-000) = %v, %v after mutating AllPokemon()", types, err)
	}
}

// ADR-0401 §5: an invalid nameJa or abilityIds fails the whole file with ErrInvalidPokemonTypes.
func TestLoadPokemonTypesRejectsInvalidNameOrAbilities(t *testing.T) {
	t.Parallel()

	entry := func(fields string) string {
		return `{"schemaVersion":1,"pokemon":[{"pokemonId":"9001-000","types":["fire"]},{"pokemonId":"9002-000","types":["grass"],` + fields + `}]}`
	}
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty nameJa", input: entry(`"nameJa":""`)},
		{name: "nameJa of 65 characters", input: entry(`"nameJa":"` + strings.Repeat("カ", 65) + `"`)},
		{name: "nameJa is a number", input: entry(`"nameJa":9002`)},
		{name: "nameJa is an array", input: entry(`"nameJa":["カソウリーフ"]`)},
		{name: "four abilityIds", input: entry(`"abilityIds":["ability-9001","ability-9002","ability-9003","ability-9004"]`)},
		{name: "duplicate abilityId", input: entry(`"abilityIds":["ability-9001","ability-9001"]`)},
		{name: "empty abilityId", input: entry(`"abilityIds":[""]`)},
		{name: "uppercase abilityId", input: entry(`"abilityIds":["Ability-9001"]`)},
		{name: "underscore abilityId", input: entry(`"abilityIds":["ability_9001"]`)},
		{name: "double hyphen abilityId", input: entry(`"abilityIds":["ability--9001"]`)},
		{name: "trailing hyphen abilityId", input: entry(`"abilityIds":["ability-9001-"]`)},
		{name: "abilityId with a space", input: entry(`"abilityIds":["ability 9001"]`)},
		{name: "abilityId of 41 characters", input: entry(`"abilityIds":["ability-9099-` + strings.Repeat("a", 28) + `"]`)},
		{name: "abilityId is a number", input: entry(`"abilityIds":[9001]`)},
		{name: "abilityIds is a string", input: entry(`"abilityIds":"ability-9001"`)},
		{name: "another unknown entry field is still rejected", input: entry(`"nameEn":"x"`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, err := LoadPokemonTypes(strings.NewReader(tt.input))
			if !errors.Is(err, ErrInvalidPokemonTypes) {
				t.Errorf("LoadPokemonTypes() error = %v, want errors.Is(_, ErrInvalidPokemonTypes)", err)
			}
			if model != nil {
				t.Errorf("LoadPokemonTypes() returned a partial model on error")
			}
		})
	}
}

// ADR-0401 §5 / ADR-0002: the committed example uses the new optional fields with fictional
// names, keeps at least one entry without them (v1 compatibility), and names only abilities
// that exist in the ability example.
func TestExamplePokemonTypesHasNamesAndAbilities(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemonTypesFile(examplePokemonTypesPath)
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypesFile(example) = %v, %v", model, err)
	}
	abilities, err := LoadAbilitiesFile(exampleAbilitiesPathForCatalog)
	if err != nil || abilities == nil {
		t.Fatalf("LoadAbilitiesFile(example) = %v, %v", abilities, err)
	}
	all := allPokemon(t, model)

	raw, err := os.ReadFile(examplePokemonTypesPath)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	if ids := regexp.MustCompile(`"pokemonId"`).FindAllIndex(raw, -1); len(all) != len(ids) {
		t.Errorf("AllPokemon() = %d entries, want every entry of the example (%d)", len(all), len(ids))
	}

	var named, unnamed, withAbilities, withoutAbilities bool
	for _, p := range all {
		if p.NameJa != "" {
			named = true
		} else {
			unnamed = true
		}
		if len(p.AbilityIDs) > 0 {
			withAbilities = true
		} else {
			withoutAbilities = true
		}
		for _, id := range p.AbilityIDs {
			if _, err := abilities.Ability(id); err != nil {
				t.Errorf("example %s names %s, which is not in %s: %v", p.PokemonID, id, exampleAbilitiesPathForCatalog, err)
			}
		}
	}
	if !named || !unnamed || !withAbilities || !withoutAbilities {
		t.Errorf("example must include named=%v unnamed=%v withAbilities=%v withoutAbilities=%v (all true)", named, unnamed, withAbilities, withoutAbilities)
	}
}
