package master

import (
	"errors"
	"io"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB3 の特性の read model(ADR-0017 §2)。ADR-0014 / ADR-0016 と同じ流儀の temporary adapter。

// AbilitiesSchemaVersion is the only accepted ability read model schemaVersion.
const AbilitiesSchemaVersion = 1

// ErrInvalidAbilities reports an ability read model that violates the ADR-0017 §2 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidAbilities = errors.New("invalid ability read model")

// AbilityReadModel is the temporary balance-local read model of abilityId -> normalized effects.
// It is loaded once at startup and replaced by the shared master snapshot adapter later.
type AbilityReadModel struct {
	abilities map[string]balance.Ability
}

var _ balance.AbilityProvider = (*AbilityReadModel)(nil)

// LoadAbilities reads and validates the JSON read model:
//
//	{"schemaVersion": 1, "abilities": [{"abilityId": "ability-9001", "effects": [{"kind": "immune", "attackType": "ground"}]}]}
func LoadAbilities(r io.Reader) (*AbilityReadModel, error) {
	// TODO(TB3): implement (ADR-0017 §2).
	return nil, nil
}

// LoadAbilitiesFile opens path and delegates to LoadAbilities.
func LoadAbilitiesFile(path string) (*AbilityReadModel, error) {
	// TODO(TB3): implement.
	return nil, nil
}

// Ability implements balance.AbilityProvider. Unknown IDs wrap balance.ErrUnknownAbility.
func (m *AbilityReadModel) Ability(abilityID string) (balance.Ability, error) {
	// TODO(TB3): implement.
	return balance.Ability{}, nil
}
