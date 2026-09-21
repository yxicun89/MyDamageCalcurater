package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
)

// TB3: BALANCE_ABILITIES_PATH(ADR-0017 §2。空文字は未設定扱い)。

const exampleAbilitiesPath = "../../testdata/abilities.example.json"

func TestAbilitiesPathEnvName(t *testing.T) {
	t.Parallel()

	if abilitiesPathEnv != "BALANCE_ABILITIES_PATH" {
		t.Fatalf("abilitiesPathEnv = %q, want BALANCE_ABILITIES_PATH (ADR-0017)", abilitiesPathEnv)
	}
}

func TestAbilityProviderFromEnvUnsetOrEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "unset", env: map[string]string{}},
		{name: "empty string", env: map[string]string{abilitiesPathEnv: ""}},
		{name: "only the other read models are set", env: map[string]string{pokemonTypesPathEnv: examplePokemonTypesPath, movesPathEnv: exampleMovesPath}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := abilityProviderFromEnv(lookupFrom(tt.env))
			if err != nil {
				t.Fatalf("error = %v, want nil when %s is unset or empty", err, abilitiesPathEnv)
			}
			// Must be an untyped nil interface: a typed nil pointer would bypass the 503 path.
			if provider != nil {
				t.Fatalf("provider = %#v, want nil interface", provider)
			}
		})
	}
}

func TestAbilityProviderFromEnvLoadsExample(t *testing.T) {
	t.Parallel()

	provider, err := abilityProviderFromEnv(lookupFrom(map[string]string{abilitiesPathEnv: exampleAbilitiesPath}))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if provider == nil {
		t.Fatal("provider = nil, want the loaded read model")
	}
	ability, err := provider.Ability("ability-9001")
	if err != nil {
		t.Fatalf("Ability(ability-9001) error = %v", err)
	}
	want := balance.Ability{AbilityID: "ability-9001", Effects: []balance.AbilityEffect{{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeGround}}}
	if !reflect.DeepEqual(ability, want) {
		t.Errorf("Ability(ability-9001) = %+v, want %+v", ability, want)
	}
	if _, err := provider.Ability("ability-9999"); !errors.Is(err, balance.ErrUnknownAbility) {
		t.Errorf("Ability(ability-9999) error = %v, want ErrUnknownAbility", err)
	}
}

func TestAbilityProviderFromEnvFailsOnBadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"schemaVersion":1,"abilities":[{"abilityId":"ability-9001","effects":[{"kind":"immune","attackType":"stellar"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "invalid read model", path: invalid, wantErr: master.ErrInvalidAbilities},
		{name: "empty file", path: empty, wantErr: master.ErrInvalidAbilities},
		{name: "missing file", path: filepath.Join(dir, "missing.json")},
		{name: "directory", path: dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := abilityProviderFromEnv(lookupFrom(map[string]string{abilitiesPathEnv: tt.path}))
			if err == nil {
				t.Fatalf("error = nil, want startup failure (provider=%v)", provider)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if provider != nil {
				t.Errorf("provider = %v, want nil interface on error", provider)
			}
		})
	}
}
