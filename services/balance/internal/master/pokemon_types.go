package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB1 contract (ADR-0014 §2). ADR-0401 §5 extends each entry with the optional
// nameJa/abilityIds used by the TB5 catalog; schemaVersion stays 1.

// PokemonTypesSchemaVersion is the only accepted read model schemaVersion.
const PokemonTypesSchemaVersion = 1

// ErrInvalidPokemonTypes reports a read model that violates the ADR-0014 format.
// A broken file must fail as a whole; it is never partially used.
var ErrInvalidPokemonTypes = errors.New("invalid pokemon type read model")

var pokemonIDPattern = regexp.MustCompile(`^\d{4}-\d{3}$`)

// catalogAbilityIDPattern mirrors the AbilityId schema (ADR-0017 §2 / ADR-0401 §5). The
// 40-character limit is checked separately: the pattern itself has no length bound.
var catalogAbilityIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxCatalogAbilityIDLength = 40
const maxCatalogAbilityCount = 3
const maxCatalogNameJaRunes = 64

// PokemonTypeReadModel is the temporary balance-local read model of pokemonId -> types.
// It is loaded once at startup and replaced by the shared master snapshot adapter (P2-2).
type PokemonTypeReadModel struct {
	types map[string][]balance.TypeID
	// catalog holds every entry (ADR-0401 §5), pokemonId ascending.
	catalog []balance.CatalogPokemon
}

var _ balance.PokemonTypeProvider = (*PokemonTypeReadModel)(nil)

// pokemonTypesFile is the on-disk JSON shape (ADR-0014 §2 / ADR-0401 §5).
type pokemonTypesFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Pokemon       []pokemonTypesEntry `json:"pokemon"`
}

// pokemonTypesEntry uses pointer fields for nameJa/abilityIds so an explicit JSON
// null decodes the same as an omitted field (ADR-0401 §7.5), while still failing
// on the wrong JSON type (a number, array, or object).
type pokemonTypesEntry struct {
	PokemonID  string    `json:"pokemonId"`
	NameJa     *string   `json:"nameJa"`
	Types      []string  `json:"types"`
	AbilityIDs *[]string `json:"abilityIds"`
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
	catalog := make([]balance.CatalogPokemon, 0, len(file.Pokemon))
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

		nameJa, err := parseCatalogNameJa(entry.PokemonID, entry.NameJa)
		if err != nil {
			return nil, err
		}
		abilityIDs, err := parseCatalogAbilityIDs(entry.PokemonID, entry.AbilityIDs)
		if err != nil {
			return nil, err
		}
		catalog = append(catalog, balance.CatalogPokemon{
			PokemonID:  entry.PokemonID,
			NameJa:     nameJa,
			Types:      resolved,
			AbilityIDs: abilityIDs,
		})
	}

	sort.Slice(catalog, func(i, j int) bool { return catalog[i].PokemonID < catalog[j].PokemonID })

	return &PokemonTypeReadModel{types: types, catalog: catalog}, nil
}

// parseCatalogNameJa validates an optional nameJa (ADR-0401 §5): 1..64 runes when present,
// "" (and the catalog's absent marker) when omitted or null.
func parseCatalogNameJa(pokemonID string, raw *string) (string, error) {
	if raw == nil {
		return "", nil
	}
	runes := []rune(*raw)
	if len(runes) < 1 || len(runes) > maxCatalogNameJaRunes {
		return "", fmt.Errorf("%w: pokemonId %q nameJa must be 1 to %d characters", ErrInvalidPokemonTypes, pokemonID, maxCatalogNameJaRunes)
	}
	return *raw, nil
}

// parseCatalogAbilityIDs validates an optional abilityIds (ADR-0401 §5): 0..3 entries,
// each matching the AbilityId format (ADR-0017 §2), no duplicates. Omitted or null decodes
// to nil, same as an explicit empty array (both give a non-nil, zero-length result).
func parseCatalogAbilityIDs(pokemonID string, raw *[]string) ([]string, error) {
	if raw == nil {
		return []string{}, nil
	}
	ids := *raw
	if len(ids) > maxCatalogAbilityCount {
		return nil, fmt.Errorf("%w: pokemonId %q abilityIds must have at most %d entries", ErrInvalidPokemonTypes, pokemonID, maxCatalogAbilityCount)
	}
	seen := make(map[string]bool, len(ids))
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > maxCatalogAbilityIDLength || !catalogAbilityIDPattern.MatchString(id) {
			return nil, fmt.Errorf("%w: pokemonId %q has invalid abilityId %q", ErrInvalidPokemonTypes, pokemonID, id)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: pokemonId %q has duplicate abilityId %q", ErrInvalidPokemonTypes, pokemonID, id)
		}
		seen[id] = true
		resolved = append(resolved, id)
	}
	return resolved, nil
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

var _ balance.PokemonCatalog = (*PokemonTypeReadModel)(nil)

// AllPokemon implements balance.PokemonCatalog (ADR-0401 §5): every entry with its optional
// nameJa and abilityIds, pokemonId ascending. The returned slice and its Types/AbilityIDs
// slices are all copies, so mutating them never changes the read model.
func (m *PokemonTypeReadModel) AllPokemon() ([]balance.CatalogPokemon, error) {
	result := make([]balance.CatalogPokemon, len(m.catalog))
	for i, p := range m.catalog {
		types := make([]balance.TypeID, len(p.Types))
		copy(types, p.Types)
		abilityIDs := make([]string, len(p.AbilityIDs))
		copy(abilityIDs, p.AbilityIDs)
		result[i] = balance.CatalogPokemon{
			PokemonID:  p.PokemonID,
			NameJa:     p.NameJa,
			Types:      types,
			AbilityIDs: abilityIDs,
		}
	}
	return result, nil
}
