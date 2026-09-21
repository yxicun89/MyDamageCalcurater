package main

import "example.com/pokecalc/services/balance/internal/balance"

// pokemonTypesPathEnv names the balance-local pokemon type read model (ADR-0014 §2).
const pokemonTypesPathEnv = "BALANCE_POKEMON_TYPES_PATH"

// pokemonTypeProviderFromEnv loads the read model named by BALANCE_POKEMON_TYPES_PATH.
// Unset: (nil, nil) — an untyped nil interface, so analyze answers 503.
// Set but unreadable or invalid: an error, and main must exit non-zero.
// TB1: stub returning zero values; the implementer replaces it.
func pokemonTypeProviderFromEnv(lookup func(string) (string, bool)) (balance.PokemonTypeProvider, error) {
	return nil, nil // TB1: not implemented
}
