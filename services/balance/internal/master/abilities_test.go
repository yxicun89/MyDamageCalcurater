package master

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB3 の特性の read model(ADR-0017 §2)。特性 ID は架空(ability-9001 以降)。

const exampleAbilitiesPath = "../../testdata/abilities.example.json"

// overlayAbilitiesPath is the copy the local overlay mounts as a ConfigMap (same reason as
// overlayPokemonTypesPath). testdata/abilities.example.json is the source of truth.
const overlayAbilitiesPath = "../../deploy/k8s/overlays/local/abilities.example.json"

func mustEffectiveness(t *testing.T, num, den int64) balance.Effectiveness {
	t.Helper()
	e, err := balance.NewEffectiveness(num, den)
	if err != nil {
		t.Fatalf("NewEffectiveness(%d, %d) error = %v", num, den, err)
	}
	return e
}

func TestAbilitiesSchemaVersion(t *testing.T) {
	t.Parallel()

	if AbilitiesSchemaVersion != 1 {
		t.Fatalf("AbilitiesSchemaVersion = %d, want 1 (ADR-0017 §2)", AbilitiesSchemaVersion)
	}
}

func TestOverlayAbilitiesExampleMatchesTestdataExample(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(exampleAbilitiesPath)
	if err != nil {
		t.Fatalf("read %s: %v", exampleAbilitiesPath, err)
	}
	got, err := os.ReadFile(overlayAbilitiesPath)
	if err != nil {
		t.Fatalf("read %s: %v (the local overlay must mount the ability example, ADR-0017 §2)", overlayAbilitiesPath, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s is out of sync with %s (the source of truth); copy it over", overlayAbilitiesPath, exampleAbilitiesPath)
	}
}

func TestLoadAbilities(t *testing.T) {
	t.Parallel()

	input := `{"schemaVersion":1,"abilities":[
		{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"ground"}]},
		{"abilityId":"ability-9002","effects":[{"kind":"absorb","attackType":"water"}]},
		{"abilityId":"ability-9003","effects":[{"kind":"type_multiplier","attackType":"fire","numerator":1,"denominator":2}]},
		{"abilityId":"ability-9004","effects":[{"kind":"super_effective_multiplier","numerator":3,"denominator":4}]},
		{"abilityId":"ability-9005","effects":[]},
		{"abilityId":"ability-9006","effects":[{"kind":"type_multiplier","attackType":"fire","numerator":2,"denominator":4}]},
		{"abilityId":"ability-9007","effects":[
			{"kind":"immune","attackType":"electric"},
			{"kind":"absorb","attackType":"water"},
			{"kind":"type_multiplier","attackType":"ice","numerator":16,"denominator":1},
			{"kind":"type_multiplier","attackType":"ice","numerator":1,"denominator":16},
			{"kind":"type_multiplier","attackType":"electric","numerator":16,"denominator":16},
			{"kind":"super_effective_multiplier","numerator":5,"denominator":4}
		]},
		{"abilityId":"9008","effects":[{"kind":"immune","attackType":"fairy"}]},
		{"abilityId":"x","effects":[]}
	]}`
	model, err := LoadAbilities(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadAbilities() error = %v", err)
	}
	if model == nil {
		t.Fatal("LoadAbilities() returned nil model without error")
	}

	tests := []balance.Ability{
		{AbilityID: "ability-9001", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeGround}}},
		{AbilityID: "ability-9002", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeWater}}},
		{AbilityID: "ability-9003", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: mustEffectiveness(t, 1, 2)}}},
		{AbilityID: "ability-9004", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectSuperEffectiveMultiplier, Factor: mustEffectiveness(t, 3, 4)}}},
		// The factor is stored in lowest terms.
		{AbilityID: "ability-9006", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: balance.Effectiveness{Num: 1, Den: 2}}}},
		// Effects keep the file order. type_multiplier may repeat an attack type; immune/absorb of different types may coexist.
		{AbilityID: "ability-9007", Effects: []balance.AbilityEffect{
			{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeElectric},
			{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeWater},
			{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeIce, Factor: mustEffectiveness(t, 16, 1)},
			{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeIce, Factor: mustEffectiveness(t, 1, 16)},
			{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeElectric, Factor: mustEffectiveness(t, 1, 1)},
			{Kind: balance.AbilityEffectSuperEffectiveMultiplier, Factor: mustEffectiveness(t, 5, 4)},
		}},
		{AbilityID: "9008", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeFairy}}},
	}
	for _, want := range tests {
		got, err := model.Ability(want.AbilityID)
		if err != nil {
			t.Errorf("Ability(%s) error = %v", want.AbilityID, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Ability(%s) = %+v, want %+v", want.AbilityID, got, want)
		}
	}

	// An empty effects array is a valid ability without type-related effects.
	for _, id := range []string{"ability-9005", "x"} {
		got, err := model.Ability(id)
		if err != nil {
			t.Errorf("Ability(%s) error = %v", id, err)
			continue
		}
		if got.AbilityID != id || len(got.Effects) != 0 {
			t.Errorf("Ability(%s) = %+v, want no effects", id, got)
		}
	}
}

func TestLoadAbilitiesAcceptsFortyCharacterID(t *testing.T) {
	t.Parallel()

	id := "ability-9099-" + strings.Repeat("a", 27)
	if len(id) != 40 {
		t.Fatalf("test ID length = %d, want 40", len(id))
	}
	model, err := LoadAbilities(strings.NewReader(`{"schemaVersion":1,"abilities":[{"abilityId":"` + id + `","effects":[]}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadAbilities() = %v, %v, want a 40-character abilityId to be accepted", model, err)
	}
	if got, err := model.Ability(id); err != nil || got.AbilityID != id {
		t.Errorf("Ability(%s) = %+v, %v", id, got, err)
	}
}

func TestAbilityReadModelUnknownAbility(t *testing.T) {
	t.Parallel()

	model, err := LoadAbilities(strings.NewReader(`{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"ground"}]},{"abilityId":"ability-90010","effects":[]}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadAbilities() = %v, %v", model, err)
	}
	for _, id := range []string{"ability-9002", "ability-9999", "ABILITY-9001", "ability-9001 ", "ability-900", ""} {
		got, err := model.Ability(id)
		if !errors.Is(err, balance.ErrUnknownAbility) {
			t.Errorf("Ability(%q) error = %v, want balance.ErrUnknownAbility", id, err)
		}
		if !reflect.DeepEqual(got, balance.Ability{}) {
			t.Errorf("Ability(%q) = %+v, want the zero Ability on error", id, got)
		}
	}
	if got, err := model.Ability("ability-90010"); err != nil || got.AbilityID != "ability-90010" || len(got.Effects) != 0 {
		t.Errorf("Ability(ability-90010) = %+v, %v (lookup must be exact)", got, err)
	}
}

func TestAbilityReadModelReturnsCopies(t *testing.T) {
	t.Parallel()

	model, err := LoadAbilities(strings.NewReader(`{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"ground"}]}]}`))
	if err != nil || model == nil {
		t.Fatalf("LoadAbilities() = %v, %v", model, err)
	}
	first, err := model.Ability("ability-9001")
	if err != nil || len(first.Effects) != 1 {
		t.Fatalf("Ability(ability-9001) = %+v, %v", first, err)
	}
	first.Effects[0] = balance.AbilityEffect{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeWater}
	second, err := model.Ability("ability-9001")
	if err != nil {
		t.Fatalf("Ability(ability-9001) error = %v", err)
	}
	want := []balance.AbilityEffect{{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeGround}}
	if !reflect.DeepEqual(second.Effects, want) {
		t.Errorf("mutating a returned slice changed the read model: %+v", second.Effects)
	}
}

func TestLoadAbilitiesRejectsInvalidReadModel(t *testing.T) {
	t.Parallel()

	const immune = `{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"ground"}]}`
	one := func(effect string) string {
		return `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[` + effect + `]}]}`
	}
	tests := []struct {
		name  string
		input string
	}{
		// Document shape.
		{name: "empty input", input: ``},
		{name: "broken JSON", input: `{"schemaVersion":1,"abilities":[`},
		{name: "not an object", input: `[]`},
		{name: "null document", input: `null`},
		{name: "trailing JSON", input: `{"schemaVersion":1,"abilities":[` + immune + `]} {}`},
		{name: "missing schemaVersion", input: `{"abilities":[` + immune + `]}`},
		{name: "schemaVersion 0", input: `{"schemaVersion":0,"abilities":[` + immune + `]}`},
		{name: "schemaVersion 2", input: `{"schemaVersion":2,"abilities":[` + immune + `]}`},
		{name: "schemaVersion string", input: `{"schemaVersion":"1","abilities":[` + immune + `]}`},
		{name: "missing abilities key", input: `{"schemaVersion":1}`},
		{name: "null abilities", input: `{"schemaVersion":1,"abilities":null}`},
		{name: "empty abilities array", input: `{"schemaVersion":1,"abilities":[]}`},
		{name: "unknown top-level field", input: `{"schemaVersion":1,"source":"x","abilities":[` + immune + `]}`},
		{name: "unknown ability field", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","name":"x","effects":[]}]}`},

		// abilityId.
		{name: "missing abilityId", input: `{"schemaVersion":1,"abilities":[{"effects":[]}]}`},
		{name: "empty abilityId", input: `{"schemaVersion":1,"abilities":[{"abilityId":"","effects":[]}]}`},
		{name: "uppercase abilityId", input: `{"schemaVersion":1,"abilities":[{"abilityId":"Ability-9001","effects":[]}]}`},
		{name: "underscore in abilityId", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability_9001","effects":[]}]}`},
		{name: "space in abilityId", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability 9001","effects":[]}]}`},
		{name: "surrounding space in abilityId", input: `{"schemaVersion":1,"abilities":[{"abilityId":" ability-9001","effects":[]}]}`},
		{name: "leading hyphen", input: `{"schemaVersion":1,"abilities":[{"abilityId":"-ability-9001","effects":[]}]}`},
		{name: "trailing hyphen", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001-","effects":[]}]}`},
		{name: "double hyphen", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability--9001","effects":[]}]}`},
		{name: "abilityId of 41 characters", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9099-` + strings.Repeat("a", 28) + `","effects":[]}]}`},
		{name: "abilityId number", input: `{"schemaVersion":1,"abilities":[{"abilityId":9001,"effects":[]}]}`},
		{name: "duplicate abilityId", input: `{"schemaVersion":1,"abilities":[` + immune + `,{"abilityId":"ability-9001","effects":[]}]}`},

		// effects.
		{name: "missing effects", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001"}]}`},
		{name: "null effects", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":null}]}`},
		{name: "effects object", input: `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":{"kind":"immune","attackType":"ground"}}]}`},
		{name: "null effect", input: one(`null`)},
		{name: "unknown effect field", input: one(`{"kind":"immune","attackType":"ground","note":"x"}`)},

		// kind.
		{name: "missing kind", input: one(`{"attackType":"ground"}`)},
		{name: "empty kind", input: one(`{"kind":"","attackType":"ground"}`)},
		{name: "unknown kind", input: one(`{"kind":"heal","attackType":"ground"}`)},
		{name: "uppercase kind", input: one(`{"kind":"Immune","attackType":"ground"}`)},

		// immune.
		{name: "immune without attackType", input: one(`{"kind":"immune"}`)},
		{name: "immune with null attackType", input: one(`{"kind":"immune","attackType":null}`)},
		{name: "immune with empty attackType", input: one(`{"kind":"immune","attackType":""}`)},
		{name: "immune with unknown attackType", input: one(`{"kind":"immune","attackType":"stellar"}`)},
		{name: "immune with uppercase attackType", input: one(`{"kind":"immune","attackType":"Ground"}`)},
		{name: "immune with attackType array", input: one(`{"kind":"immune","attackType":["ground"]}`)},
		{name: "immune with numerator", input: one(`{"kind":"immune","attackType":"ground","numerator":1}`)},
		{name: "immune with denominator", input: one(`{"kind":"immune","attackType":"ground","denominator":2}`)},

		// absorb.
		{name: "absorb without attackType", input: one(`{"kind":"absorb"}`)},
		{name: "absorb with unknown attackType", input: one(`{"kind":"absorb","attackType":"stellar"}`)},
		{name: "absorb with numerator and denominator", input: one(`{"kind":"absorb","attackType":"water","numerator":1,"denominator":2}`)},

		// type_multiplier.
		{name: "type_multiplier without attackType", input: one(`{"kind":"type_multiplier","numerator":1,"denominator":2}`)},
		{name: "type_multiplier with unknown attackType", input: one(`{"kind":"type_multiplier","attackType":"stellar","numerator":1,"denominator":2}`)},
		{name: "type_multiplier without numerator", input: one(`{"kind":"type_multiplier","attackType":"fire","denominator":2}`)},
		{name: "type_multiplier without denominator", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":1}`)},
		{name: "type_multiplier with null numerator", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":null,"denominator":2}`)},
		{name: "type_multiplier numerator 0", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":0,"denominator":2}`)},
		{name: "type_multiplier numerator 17", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":17,"denominator":2}`)},
		{name: "type_multiplier negative numerator", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":-1,"denominator":2}`)},
		{name: "type_multiplier denominator 0", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":1,"denominator":0}`)},
		{name: "type_multiplier denominator 17", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":1,"denominator":17}`)},
		{name: "type_multiplier negative denominator", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":1,"denominator":-2}`)},
		{name: "type_multiplier fractional numerator", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":1.5,"denominator":2}`)},
		{name: "type_multiplier string numerator", input: one(`{"kind":"type_multiplier","attackType":"fire","numerator":"1","denominator":2}`)},

		// super_effective_multiplier.
		{name: "super_effective_multiplier with attackType", input: one(`{"kind":"super_effective_multiplier","attackType":"fire","numerator":3,"denominator":4}`)},
		{name: "super_effective_multiplier without numerator", input: one(`{"kind":"super_effective_multiplier","denominator":4}`)},
		{name: "super_effective_multiplier without denominator", input: one(`{"kind":"super_effective_multiplier","numerator":3}`)},
		{name: "super_effective_multiplier numerator 0", input: one(`{"kind":"super_effective_multiplier","numerator":0,"denominator":4}`)},
		{name: "super_effective_multiplier numerator 17", input: one(`{"kind":"super_effective_multiplier","numerator":17,"denominator":4}`)},
		{name: "super_effective_multiplier denominator 0", input: one(`{"kind":"super_effective_multiplier","numerator":3,"denominator":0}`)},
		{name: "super_effective_multiplier denominator 17", input: one(`{"kind":"super_effective_multiplier","numerator":3,"denominator":17}`)},

		// Same attack type immune/absorb twice within one ability.
		{name: "immune twice for one type", input: one(`{"kind":"immune","attackType":"ground"},{"kind":"immune","attackType":"ground"}`)},
		{name: "absorb twice for one type", input: one(`{"kind":"absorb","attackType":"water"},{"kind":"absorb","attackType":"water"}`)},
		{name: "immune and absorb for one type", input: one(`{"kind":"immune","attackType":"water"},{"kind":"absorb","attackType":"water"}`)},
		{name: "absorb, a multiplier, then immune for one type", input: one(`{"kind":"absorb","attackType":"water"},{"kind":"type_multiplier","attackType":"water","numerator":1,"denominator":2},{"kind":"immune","attackType":"water"}`)},

		{name: "invalid entry after valid ones", input: `{"schemaVersion":1,"abilities":[` + immune + `,{"abilityId":"ability-9002","effects":[{"kind":"absorb","attackType":"stellar"}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, err := LoadAbilities(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("LoadAbilities() error = nil, want error (model=%v)", model)
			}
			if !errors.Is(err, ErrInvalidAbilities) {
				t.Errorf("LoadAbilities() error = %v, want errors.Is(_, ErrInvalidAbilities)", err)
			}
			if model != nil {
				t.Errorf("LoadAbilities() returned a partial model on error")
			}
		})
	}
}

// TestLoadAbilitiesAllowsSameTypeAcrossAbilities: the duplicate rule is per ability.
func TestLoadAbilitiesAllowsSameTypeAcrossAbilities(t *testing.T) {
	t.Parallel()

	input := `{"schemaVersion":1,"abilities":[
		{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"ground"}]},
		{"abilityId":"ability-9002","effects":[{"kind":"absorb","attackType":"ground"}]},
		{"abilityId":"ability-9003","effects":[{"kind":"immune","attackType":"ground"},{"kind":"type_multiplier","attackType":"ground","numerator":2,"denominator":1}]}
	]}`
	if model, err := LoadAbilities(strings.NewReader(input)); err != nil || model == nil {
		t.Fatalf("LoadAbilities() = %v, %v, want success", model, err)
	}
}

func TestLoadAbilitiesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	invalid := filepath.Join(dir, "invalid.json")
	empty := filepath.Join(dir, "empty.json")
	writeFile(t, valid, `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9002","effects":[{"kind":"absorb","attackType":"water"}]}]}`)
	writeFile(t, invalid, `{"schemaVersion":1,"abilities":[{"abilityId":"ability-9002","effects":[{"kind":"absorb","attackType":"stellar"}]}]}`)
	writeFile(t, empty, ``)

	model, err := LoadAbilitiesFile(valid)
	if err != nil || model == nil {
		t.Fatalf("LoadAbilitiesFile(valid) = %v, %v", model, err)
	}
	want := balance.Ability{AbilityID: "ability-9002", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeWater}}}
	if got, err := model.Ability("ability-9002"); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Ability(ability-9002) = %+v, %v, want %+v", got, err, want)
	}

	for _, path := range []string{invalid, empty} {
		if model, err := LoadAbilitiesFile(path); !errors.Is(err, ErrInvalidAbilities) || model != nil {
			t.Errorf("LoadAbilitiesFile(%s) = %v, %v, want nil and ErrInvalidAbilities", filepath.Base(path), model, err)
		}
	}
	missing := filepath.Join(dir, "missing.json")
	if model, err := LoadAbilitiesFile(missing); err == nil || model != nil {
		t.Errorf("LoadAbilitiesFile(missing) = %v, %v, want error", model, err)
	}
	if model, err := LoadAbilitiesFile(dir); err == nil || model != nil {
		t.Errorf("LoadAbilitiesFile(directory) = %v, %v, want error", model, err)
	}
}

// TestExampleAbilitiesIsValidAndFictional guards the committed example (ADR-0002/0017):
// it must load, use only fictional IDs from ability-9001, and cover all four kinds and an
// ability without effects.
func TestExampleAbilitiesIsValidAndFictional(t *testing.T) {
	t.Parallel()

	model, err := LoadAbilitiesFile(exampleAbilitiesPath)
	if err != nil || model == nil {
		t.Fatalf("LoadAbilitiesFile(example) = %v, %v", model, err)
	}

	raw, err := os.ReadFile(exampleAbilitiesPath)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	allIDs := regexp.MustCompile(`"abilityId"\s*:\s*"([^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(allIDs) == 0 {
		t.Fatal("example has no abilityId entries")
	}
	fictional := regexp.MustCompile(`^ability-(\d+)$`)
	for _, id := range allIDs {
		match := fictional.FindStringSubmatch(id[1])
		if match == nil {
			t.Errorf("example abilityId %q must be a fictional ability-NNNN ID", id[1])
			continue
		}
		if n, err := strconv.Atoi(match[1]); err != nil || n < 9001 {
			t.Errorf("example abilityId %q must be fictional (>= ability-9001)", id[1])
		}
	}
	if strings.Contains(string(raw), `"name"`) {
		t.Error("example must not contain names (only fictional IDs and normalized effects)")
	}

	kinds := map[balance.AbilityEffectKind]bool{}
	var withoutEffects bool
	for _, id := range allIDs {
		ability, err := model.Ability(id[1])
		if err != nil {
			t.Fatalf("Ability(%s) error = %v", id[1], err)
		}
		if len(ability.Effects) == 0 {
			withoutEffects = true
		}
		for _, effect := range ability.Effects {
			kinds[effect.Kind] = true
		}
	}
	for _, kind := range []balance.AbilityEffectKind{
		balance.AbilityEffectImmune, balance.AbilityEffectAbsorb,
		balance.AbilityEffectTypeMultiplier, balance.AbilityEffectSuperEffectiveMultiplier,
	} {
		if !kinds[kind] {
			t.Errorf("example must contain an effect of kind %q", kind)
		}
	}
	if !withoutEffects {
		t.Error("example must contain an ability with an empty effects array")
	}
}
