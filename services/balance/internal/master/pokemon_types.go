package master

import (
	"errors"
	"io"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB1 contract (ADR-0014 §2). Bodies are intentionally unimplemented stubs
// returning zero values; the implementer replaces them.

// PokemonTypesSchemaVersion is the only accepted read model schemaVersion.
const PokemonTypesSchemaVersion = 1

// ErrInvalidPokemonTypes reports a read model that violates the ADR-0014 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidPokemonTypes = errors.New("invalid pokemon type read model")

// PokemonTypeReadModel is the temporary balance-local read model of pokemonId -> types.
// It is loaded once at startup and replaced by the shared master snapshot adapter (P2-2).
type PokemonTypeReadModel struct {
	types map[string][]balance.TypeID
}

var _ balance.PokemonTypeProvider = (*PokemonTypeReadModel)(nil)

// LoadPokemonTypes reads and validates the JSON read model:
//
//	{"schemaVersion": 1, "pokemon": [{"pokemonId": "9001-000", "types": ["fire", "flying"]}]}
func LoadPokemonTypes(r io.Reader) (*PokemonTypeReadModel, error) {
	return nil, nil // TB1: not implemented
}

// LoadPokemonTypesFile opens path and delegates to LoadPokemonTypes.
func LoadPokemonTypesFile(path string) (*PokemonTypeReadModel, error) {
	return nil, nil // TB1: not implemented
}

// PokemonTypes implements balance.PokemonTypeProvider. Unknown IDs wrap balance.ErrUnknownPokemon.
func (m *PokemonTypeReadModel) PokemonTypes(pokemonID string) ([]balance.TypeID, error) {
	return nil, nil // TB1: not implemented
}
