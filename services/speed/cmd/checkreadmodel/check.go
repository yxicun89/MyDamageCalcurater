package main

import (
	"fmt"
	"path/filepath"

	"example.com/pokecalc/services/speed/internal/master"
)

// pokemonReadModelFile は read model のディレクトリの中のファイル名(ADR-0603 §1、ADR-0105 §120)。
// balance は3ファイルだが speed は1ファイルだけ。
const pokemonReadModelFile = "speed-pokemon.json"

// check は dir の read model を service と同じ loader(master.LoadPokemonFile)で読み、
// "read model ok: <n> pokemon" の形の 1 行の summary を返す。
// ConfigMap の大きさは見ない(scripts/k3d-deploy-readmodel.sh の役割。balance と同じ分担)。
func check(dir string) (string, error) {
	readModel, err := master.LoadPokemonFile(filepath.Join(dir, pokemonReadModelFile))
	if err != nil {
		return "", fmt.Errorf("%s: %w", pokemonReadModelFile, err)
	}
	roster, err := readModel.Roster()
	if err != nil {
		return "", fmt.Errorf("%s: %w", pokemonReadModelFile, err)
	}
	return fmt.Sprintf("read model ok: %d pokemon", len(roster.Pokemon)), nil
}
