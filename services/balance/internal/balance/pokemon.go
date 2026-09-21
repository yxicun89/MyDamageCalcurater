package balance

import "errors"

// TB1 contract (ADR-0014 §2). Bodies are intentionally unimplemented stubs returning zero values.

var (
	// ErrUnknownPokemon reports a pokemonId that the provider does not know.
	ErrUnknownPokemon = errors.New("unknown pokemon")
	// ErrNilPokemonTypes reports a missing PokemonTypeProvider.
	ErrNilPokemonTypes = errors.New("pokemon type provider is required")
)

// PokemonTypeProvider resolves a pokemonId ("NNNN-NNN") to its one or two types.
// Unknown IDs return an error wrapping ErrUnknownPokemon. It is the replacement
// boundary for the shared master snapshot (P2-2); the domain package does not own it.
type PokemonTypeProvider interface {
	PokemonTypes(pokemonID string) ([]TypeID, error)
}

// ResolveMembers looks up each pokemonId in order (duplicates allowed) and returns
// the members with their types. An unknown ID returns an error wrapping ErrUnknownPokemon.
func ResolveMembers(provider PokemonTypeProvider, pokemonIDs []string) ([]Member, error) {
	if provider == nil {
		return nil, ErrNilPokemonTypes
	}

	members := make([]Member, len(pokemonIDs))
	for i, id := range pokemonIDs {
		types, err := provider.PokemonTypes(id)
		if err != nil {
			return nil, err
		}
		members[i] = Member{PokemonID: id, Types: types}
	}
	return members, nil
}
