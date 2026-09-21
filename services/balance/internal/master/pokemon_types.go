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

// TB1 contract (ADR-0014 §2).

// PokemonTypesSchemaVersion is the only accepted read model schemaVersion.
const PokemonTypesSchemaVersion = 1

// ErrInvalidPokemonTypes reports a read model that violates the ADR-0014 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidPokemonTypes = errors.New("invalid pokemon type read model")

var pokemonIDPattern = regexp.MustCompile(`^\d{4}-\d{3}$`)

// PokemonTypeReadModel is the temporary balance-local read model of pokemonId -> types.
// It is loaded once at startup and replaced by the shared master snapshot adapter (P2-2).
type PokemonTypeReadModel struct {
	types map[string][]balance.TypeID
}

var _ balance.PokemonTypeProvider = (*PokemonTypeReadModel)(nil)

// pokemonTypesFile is the on-disk JSON shape (ADR-0014 §2).
type pokemonTypesFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Pokemon       []pokemonTypesEntry `json:"pokemon"`
}

type pokemonTypesEntry struct {
	PokemonID string   `json:"pokemonId"`
	Types     []string `json:"types"`
}

// LoadPokemonTypes reads and validates the JSON read model:
//
//	{"schemaVersion": 1, "pokemon": [{"pokemonId": "9001-000", "types": ["fire", "flying"]}]}
func LoadPokemonTypes(r io.Reader) (*PokemonTypeReadModel, error) {
	var file pokemonTypesFile
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPokemonTypes, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: document must contain exactly one JSON object", ErrInvalidPokemonTypes)
	}

	if file.SchemaVersion != PokemonTypesSchemaVersion {
		return nil, fmt.Errorf("%w: schemaVersion must be %d, got %d", ErrInvalidPokemonTypes, PokemonTypesSchemaVersion, file.SchemaVersion)
	}
	if len(file.Pokemon) == 0 {
		return nil, fmt.Errorf("%w: pokemon must not be empty", ErrInvalidPokemonTypes)
	}

	types := make(map[string][]balance.TypeID, len(file.Pokemon))
	for _, entry := range file.Pokemon {
		if !pokemonIDPattern.MatchString(entry.PokemonID) {
			return nil, fmt.Errorf("%w: pokemonId %q must use the NNNN-NNN format", ErrInvalidPokemonTypes, entry.PokemonID)
		}
		if _, exists := types[entry.PokemonID]; exists {
			return nil, fmt.Errorf("%w: duplicate pokemonId %q", ErrInvalidPokemonTypes, entry.PokemonID)
		}
		if len(entry.Types) < 1 || len(entry.Types) > 2 {
			return nil, fmt.Errorf("%w: pokemonId %q must have one or two types", ErrInvalidPokemonTypes, entry.PokemonID)
		}

		resolved := make([]balance.TypeID, 0, len(entry.Types))
		seen := make(map[balance.TypeID]bool, len(entry.Types))
		for _, raw := range entry.Types {
			typeID := balance.TypeID(raw)
			if !typeID.Valid() {
				return nil, fmt.Errorf("%w: pokemonId %q has invalid type %q", ErrInvalidPokemonTypes, entry.PokemonID, raw)
			}
			if seen[typeID] {
				return nil, fmt.Errorf("%w: pokemonId %q has duplicate type %q", ErrInvalidPokemonTypes, entry.PokemonID, raw)
			}
			seen[typeID] = true
			resolved = append(resolved, typeID)
		}
		types[entry.PokemonID] = resolved
	}

	return &PokemonTypeReadModel{types: types}, nil
}

// LoadPokemonTypesFile opens path and delegates to LoadPokemonTypes.
func LoadPokemonTypesFile(path string) (*PokemonTypeReadModel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadPokemonTypes(f)
}

// PokemonTypes implements balance.PokemonTypeProvider. Unknown IDs wrap balance.ErrUnknownPokemon.
func (m *PokemonTypeReadModel) PokemonTypes(pokemonID string) ([]balance.TypeID, error) {
	types, ok := m.types[pokemonID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", balance.ErrUnknownPokemon, pokemonID)
	}
	result := make([]balance.TypeID, len(types))
	copy(result, types)
	return result, nil
}
