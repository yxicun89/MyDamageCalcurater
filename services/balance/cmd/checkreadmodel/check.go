package main

import (
	"fmt"
	"path/filepath"

	"example.com/pokecalc/services/balance/internal/master"
)

// Read model file names inside the directory (ADR-0403).
const (
	pokemonTypesFile = "pokemon-types.json"
	movesFile        = "moves.json"
	abilitiesFile    = "abilities.json"
)

// check loads the three read models in dir and returns a one-line summary.
func check(dir string) (string, error) {
	pokemon, err := master.LoadPokemonTypesFile(filepath.Join(dir, pokemonTypesFile))
	if err != nil {
		return "", fmt.Errorf("%s: %w", pokemonTypesFile, err)
	}
	if _, err := master.LoadMovesFile(filepath.Join(dir, movesFile)); err != nil {
		return "", fmt.Errorf("%s: %w", movesFile, err)
	}
	if _, err := master.LoadAbilitiesFile(filepath.Join(dir, abilitiesFile)); err != nil {
		return "", fmt.Errorf("%s: %w", abilitiesFile, err)
	}
	all, err := pokemon.AllPokemon()
	if err != nil {
		return "", fmt.Errorf("%s: %w", pokemonTypesFile, err)
	}
	return fmt.Sprintf("read model ok: %d pokemon", len(all)), nil
}
