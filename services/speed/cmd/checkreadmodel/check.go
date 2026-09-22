package main

import "errors"

// pokemonReadModelFile は read model のディレクトリの中のファイル名(ADR-0603 §1、ADR-0105 §120)。
// balance は3ファイルだが speed は1ファイルだけ。
const pokemonReadModelFile = "speed-pokemon.json"

// errNotImplemented は SP4 のスタブ。implementer が check を実装したら消す。
var errNotImplemented = errors.New("checkreadmodel: not implemented yet (SP4)")

// check は dir の read model を service と同じ loader(master.LoadPokemonFile)で読み、
// "read model ok: <n> pokemon" の形の 1 行の summary を返す。
// ConfigMap の大きさは見ない(scripts/k3d-deploy-readmodel.sh の役割。balance と同じ分担)。
func check(dir string) (string, error) {
	return "", errNotImplemented
}
