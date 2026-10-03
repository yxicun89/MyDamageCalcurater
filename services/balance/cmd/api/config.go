package main

import (
	"fmt"
	"strconv"

	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/httpapi"
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

// movesPathEnv names the balance-local move read model (ADR-0016 §3).
const movesPathEnv = "BALANCE_MOVES_PATH"

// moveProviderFromEnv loads the read model named by BALANCE_MOVES_PATH.
// Unset or empty: (nil, nil) — an untyped nil interface, so coverage answers 503.
// Set but unreadable or invalid: an error, and main must exit non-zero.
func moveProviderFromEnv(lookup func(string) (string, bool)) (balance.MoveProvider, error) {
	path, ok := lookup(movesPathEnv)
	if !ok || path == "" {
		return nil, nil
	}
	model, err := master.LoadMovesFile(path)
	if err != nil {
		// Return an untyped nil interface, not a nil *MoveReadModel wrapped in a
		// non-nil interface (the classic typed-nil pitfall).
		return nil, err
	}
	return model, nil
}

// abilitiesPathEnv names the balance-local ability read model (ADR-0017 §2).
const abilitiesPathEnv = "BALANCE_ABILITIES_PATH"

// abilityProviderFromEnv loads the read model named by BALANCE_ABILITIES_PATH.
// Unset or empty: (nil, nil) — an untyped nil interface, so analyze answers 503 only
// when a member names an abilityId (ADR-0017 §4).
// Set but unreadable or invalid: an error, and main must exit non-zero.
func abilityProviderFromEnv(lookup func(string) (string, bool)) (balance.AbilityProvider, error) {
	path, ok := lookup(abilitiesPathEnv)
	if !ok || path == "" {
		return nil, nil
	}
	model, err := master.LoadAbilitiesFile(path)
	if err != nil {
		// Return an untyped nil interface, not a nil *AbilityReadModel wrapped in a
		// non-nil interface (the classic typed-nil pitfall).
		return nil, err
	}
	return model, nil
}

// dataVersionFromEnv は BALANCE_POKEMON_TYPES_PATH と同じディレクトリの metadata.json から read model の版を読む
// (ADR-0138)。パス未設定・metadata.json 無しは ("", nil)(版不明)。あるのに不正ならエラー(main は非 0 で終了する)。
func dataVersionFromEnv(lookup func(string) (string, bool)) (string, error) {
	path, ok := lookup(pokemonTypesPathEnv)
	if !ok || path == "" {
		return "", nil
	}
	return master.LoadDataVersionNextTo(path)
}

// maxConcurrentRecommendationsEnv caps the recommendations computed at once (issue #298, ADR-0409).
const maxConcurrentRecommendationsEnv = "BALANCE_MAX_CONCURRENT_RECOMMENDATIONS"

// maxConcurrentRecommendationsFromEnv reads the cap. Unset or empty: httpapi.DefaultMaxConcurrentRecommendations.
// Not a positive integer: an error, and main must exit non-zero.
func maxConcurrentRecommendationsFromEnv(lookup func(string) (string, bool)) (int, error) {
	value, ok := lookup(maxConcurrentRecommendationsEnv)
	if !ok || value == "" {
		return httpapi.DefaultMaxConcurrentRecommendations, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", maxConcurrentRecommendationsEnv, value)
	}
	return n, nil
}
