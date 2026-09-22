// Package master は speed の暫定の read model(架空データの JSON)を読む(ADR-0600 §4)。
package master

import (
	"errors"
	"io"

	"example.com/pokecalc/services/speed/internal/speed"
)

// PokemonSchemaVersion は受け付ける唯一の schemaVersion(ADR-0600 §4)。
const PokemonSchemaVersion = 1

// ErrInvalidPokemon は ADR-0600 §4 の形式に反する read model を表す。
// 壊れたファイルは全体を失敗させ、一部だけを使うことはしない。
var ErrInvalidPokemon = errors.New("invalid speed pokemon read model")

// PokemonReadModel は起動時に 1 度だけ読み込む、1 つのレギュレーションの使用可能集合。
// SP4 で pokedex の read model の adapter に差し替える。
type PokemonReadModel struct{}

var _ speed.PokemonProvider = (*PokemonReadModel)(nil)

// LoadPokemon は JSON の read model を読み、ADR-0600 §4 の全項目を検証する。
// 不正なら ErrInvalidPokemon を包んだエラーを返す。
func LoadPokemon(r io.Reader) (*PokemonReadModel, error) {
	panic("unimplemented")
}

// LoadPokemonFile は path を開いて LoadPokemon に渡す。
func LoadPokemonFile(path string) (*PokemonReadModel, error) {
	panic("unimplemented")
}

// Roster は speed.PokemonProvider の実装。ファイルの並び順のまま、呼び出し側が変更してよい複製を返す。
func (m *PokemonReadModel) Roster() (speed.Roster, error) {
	panic("unimplemented")
}
