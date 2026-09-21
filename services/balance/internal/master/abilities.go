package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB3 の特性の read model(ADR-0017 §2)。ADR-0014 / ADR-0016 と同じ流儀の temporary adapter。

// AbilitiesSchemaVersion is the only accepted ability read model schemaVersion.
const AbilitiesSchemaVersion = 1

// ErrInvalidAbilities reports an ability read model that violates the ADR-0017 §2 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidAbilities = errors.New("invalid ability read model")

var abilityIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxAbilityIDLength = 40

// minAbilityFactor and maxAbilityFactor bound the numerator/denominator of a
// type_multiplier or super_effective_multiplier effect (ADR-0017 §2).
const (
	minAbilityFactor = 1
	maxAbilityFactor = 16
)

// AbilityReadModel is the temporary balance-local read model of abilityId -> normalized effects.
// It is loaded once at startup and replaced by the shared master snapshot adapter later.
type AbilityReadModel struct {
	abilities map[string]balance.Ability
}

var _ balance.AbilityProvider = (*AbilityReadModel)(nil)

// abilitiesFile is the on-disk JSON shape (ADR-0017 §2).
type abilitiesFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	Abilities     []abilityEntry `json:"abilities"`
}

type abilityEntry struct {
	AbilityID string         `json:"abilityId"`
	Effects   *[]effectEntry `json:"effects"`
}

// effectEntry is the on-disk shape of one normalized effect. Pointer fields distinguish a
// missing/null value (nil) from an explicit one, which matters for numerator/denominator
// and attackType (both missing and null must be rejected the same way).
type effectEntry struct {
	Kind        string  `json:"kind"`
	AttackType  *string `json:"attackType"`
	Numerator   *int64  `json:"numerator"`
	Denominator *int64  `json:"denominator"`
}

// LoadAbilities reads and validates the JSON read model:
//
//	{"schemaVersion": 1, "abilities": [{"abilityId": "ability-9001", "effects": [{"kind": "immune", "attackType": "ground"}]}]}
func LoadAbilities(r io.Reader) (*AbilityReadModel, error) {
	var file abilitiesFile
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAbilities, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: document must contain exactly one JSON object", ErrInvalidAbilities)
	}

	if file.SchemaVersion != AbilitiesSchemaVersion {
		return nil, fmt.Errorf("%w: schemaVersion must be %d, got %d", ErrInvalidAbilities, AbilitiesSchemaVersion, file.SchemaVersion)
	}
	if len(file.Abilities) == 0 {
		return nil, fmt.Errorf("%w: abilities must not be empty", ErrInvalidAbilities)
	}

	abilities := make(map[string]balance.Ability, len(file.Abilities))
	for _, entry := range file.Abilities {
		if entry.AbilityID == "" || len(entry.AbilityID) > maxAbilityIDLength || !abilityIDPattern.MatchString(entry.AbilityID) {
			return nil, fmt.Errorf("%w: abilityId %q must match %s and be at most %d characters", ErrInvalidAbilities, entry.AbilityID, abilityIDPattern, maxAbilityIDLength)
		}
		if _, exists := abilities[entry.AbilityID]; exists {
			return nil, fmt.Errorf("%w: duplicate abilityId %q", ErrInvalidAbilities, entry.AbilityID)
		}
		if entry.Effects == nil {
			return nil, fmt.Errorf("%w: abilityId %q is missing effects", ErrInvalidAbilities, entry.AbilityID)
		}

		effects := make([]balance.AbilityEffect, 0, len(*entry.Effects))
		zeroedTypes := make(map[balance.TypeID]bool, len(*entry.Effects))
		for _, raw := range *entry.Effects {
			effect, err := parseAbilityEffect(entry.AbilityID, raw, zeroedTypes)
			if err != nil {
				return nil, err
			}
			effects = append(effects, effect)
		}

		abilities[entry.AbilityID] = balance.Ability{AbilityID: entry.AbilityID, Effects: effects}
	}

	return &AbilityReadModel{abilities: abilities}, nil
}

// parseAbilityEffect validates one raw effect entry of ability abilityID and converts it to
// the domain representation. zeroedTypes tracks the attack types already covered by an
// immune/absorb effect of this same ability (duplicates of those two kinds for one attack
// type are rejected; a type_multiplier for the same type is unaffected).
func parseAbilityEffect(abilityID string, raw effectEntry, zeroedTypes map[balance.TypeID]bool) (balance.AbilityEffect, error) {
	switch raw.Kind {
	case string(balance.AbilityEffectImmune), string(balance.AbilityEffectAbsorb):
		attackType, err := requireAttackType(abilityID, raw)
		if err != nil {
			return balance.AbilityEffect{}, err
		}
		if raw.Numerator != nil || raw.Denominator != nil {
			return balance.AbilityEffect{}, fmt.Errorf("%w: abilityId %q %s effect must not have numerator/denominator", ErrInvalidAbilities, abilityID, raw.Kind)
		}
		if zeroedTypes[attackType] {
			return balance.AbilityEffect{}, fmt.Errorf("%w: abilityId %q has more than one immune/absorb effect for attackType %q", ErrInvalidAbilities, abilityID, attackType)
		}
		zeroedTypes[attackType] = true
		kind := balance.AbilityEffectImmune
		if raw.Kind == string(balance.AbilityEffectAbsorb) {
			kind = balance.AbilityEffectAbsorb
		}
		return balance.AbilityEffect{Kind: kind, AttackType: attackType}, nil

	case string(balance.AbilityEffectTypeMultiplier):
		attackType, err := requireAttackType(abilityID, raw)
		if err != nil {
			return balance.AbilityEffect{}, err
		}
		factor, err := requireFactor(abilityID, raw)
		if err != nil {
			return balance.AbilityEffect{}, err
		}
		return balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: attackType, Factor: factor}, nil

	case string(balance.AbilityEffectSuperEffectiveMultiplier):
		if raw.AttackType != nil {
			return balance.AbilityEffect{}, fmt.Errorf("%w: abilityId %q super_effective_multiplier effect must not have attackType", ErrInvalidAbilities, abilityID)
		}
		factor, err := requireFactor(abilityID, raw)
		if err != nil {
			return balance.AbilityEffect{}, err
		}
		return balance.AbilityEffect{Kind: balance.AbilityEffectSuperEffectiveMultiplier, Factor: factor}, nil

	default:
		return balance.AbilityEffect{}, fmt.Errorf("%w: abilityId %q has unknown effect kind %q", ErrInvalidAbilities, abilityID, raw.Kind)
	}
}

func requireAttackType(abilityID string, raw effectEntry) (balance.TypeID, error) {
	if raw.AttackType == nil {
		return "", fmt.Errorf("%w: abilityId %q %s effect requires attackType", ErrInvalidAbilities, abilityID, raw.Kind)
	}
	attackType := balance.TypeID(*raw.AttackType)
	if !attackType.Valid() {
		return "", fmt.Errorf("%w: abilityId %q has invalid attackType %q", ErrInvalidAbilities, abilityID, *raw.AttackType)
	}
	return attackType, nil
}

func requireFactor(abilityID string, raw effectEntry) (balance.Effectiveness, error) {
	if raw.Numerator == nil || raw.Denominator == nil {
		return balance.Effectiveness{}, fmt.Errorf("%w: abilityId %q %s effect requires numerator and denominator", ErrInvalidAbilities, abilityID, raw.Kind)
	}
	num, den := *raw.Numerator, *raw.Denominator
	if num < minAbilityFactor || num > maxAbilityFactor || den < minAbilityFactor || den > maxAbilityFactor {
		return balance.Effectiveness{}, fmt.Errorf("%w: abilityId %q %s numerator/denominator must be between %d and %d, got %d/%d",
			ErrInvalidAbilities, abilityID, raw.Kind, minAbilityFactor, maxAbilityFactor, num, den)
	}
	factor, err := balance.NewEffectiveness(num, den)
	if err != nil {
		return balance.Effectiveness{}, fmt.Errorf("%w: abilityId %q %s factor %d/%d: %v", ErrInvalidAbilities, abilityID, raw.Kind, num, den, err)
	}
	return factor, nil
}

// LoadAbilitiesFile opens path and delegates to LoadAbilities.
func LoadAbilitiesFile(path string) (*AbilityReadModel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadAbilities(f)
}

// Ability implements balance.AbilityProvider. Unknown IDs wrap balance.ErrUnknownAbility.
func (m *AbilityReadModel) Ability(abilityID string) (balance.Ability, error) {
	ability, ok := m.abilities[abilityID]
	if !ok {
		return balance.Ability{}, fmt.Errorf("%w: %s", balance.ErrUnknownAbility, abilityID)
	}
	// Return a copy so mutating the caller's slice never changes the read model.
	effects := make([]balance.AbilityEffect, len(ability.Effects))
	copy(effects, ability.Effects)
	return balance.Ability{AbilityID: ability.AbilityID, Effects: effects}, nil
}
