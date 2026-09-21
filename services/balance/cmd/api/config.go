package main

import (
	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
)

// pokemonTypesPathEnv names the balance-local pokemon type read model (ADR-0014 §2).
const pokemonTypesPathEnv = "BALANCE_POKEMON_TYPES_PATH"

// pokemonTypeProviderFromEnv loads the read model named by BALANCE_POKEMON_TYPES_PATH.
// Unset (or set to the empty string, ADR-0014 §5.3): (nil, nil) — an untyped nil
// interface, so analyze answers 503.
// Set but unreadable or invalid: an error, and main must exit non-zero.
func pokemonTypeProviderFromEnv(lookup func(string) (string, bool)) (balance.PokemonTypeProvider, error) {
	path, ok := lookup(pokemonTypesPathEnv)
	if !ok || path == "" {
		return nil, nil
	}
	model, err := master.LoadPokemonTypesFile(path)
	if err != nil {
		// Return an untyped nil interface, not a nil *PokemonTypeReadModel wrapped
		// in a non-nil interface (the classic typed-nil pitfall).
		return nil, err
	}
	return model, nil
}
