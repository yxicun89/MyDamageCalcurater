// Package master は speed の暫定の read model(架空データの JSON)を読む(ADR-0600 §4)。
package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"example.com/pokecalc/services/speed/internal/speed"
)

// PokemonSchemaVersion は受け付ける唯一の schemaVersion(ADR-0600 §4)。
const PokemonSchemaVersion = 1

// ErrInvalidPokemon は ADR-0600 §4 の形式に反する read model を表す。
// 壊れたファイルは全体を失敗させ、一部だけを使うことはしない。
var ErrInvalidPokemon = errors.New("invalid speed pokemon read model")

var (
	pokemonIDPattern    = regexp.MustCompile(`^\d{4}-\d{3}$`)
	regulationIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	typeIDPattern       = regexp.MustCompile(`^[a-z]+$`)
)

// baseSpeed・types 個数の範囲(ADR-0600 §4)。
const (
	minBaseSpeed = 1
	maxBaseSpeed = 255
	minTypes     = 1
	maxTypes     = 2
)

// PokemonReadModel は起動時に 1 度だけ読み込む、1 つのレギュレーションの使用可能集合。
// SP0〜SP3 は speed 内の架空データ、SP4 以降は pokedex export の実データ(同じ形式・同じ
// loader。adapter の差し替えは無い。ADR-0603 §5)。
type PokemonReadModel struct {
	regulationID string
	pokemon      []speed.Pokemon
}

var _ speed.PokemonProvider = (*PokemonReadModel)(nil)

// pokemonFile は on-disk の JSON の形(ADR-0600 §4)。
type pokemonFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	RegulationID  string         `json:"regulationId"`
	Pokemon       []pokemonEntry `json:"pokemon"`
}

type pokemonEntry struct {
	PokemonID string   `json:"pokemonId"`
	NameJa    string   `json:"nameJa"`
	Types     []string `json:"types"`
	BaseSpeed int      `json:"baseSpeed"`
}

// LoadPokemon は JSON の read model を読み、ADR-0600 §4 の全項目を検証する。
// 不正なら ErrInvalidPokemon を包んだエラーを返す。
func LoadPokemon(r io.Reader) (*PokemonReadModel, error) {
	var file pokemonFile
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPokemon, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: document must contain exactly one JSON object", ErrInvalidPokemon)
	}

	if file.SchemaVersion != PokemonSchemaVersion {
		return nil, fmt.Errorf("%w: schemaVersion must be %d, got %d", ErrInvalidPokemon, PokemonSchemaVersion, file.SchemaVersion)
	}
	if !regulationIDPattern.MatchString(file.RegulationID) {
		return nil, fmt.Errorf("%w: regulationId %q is invalid", ErrInvalidPokemon, file.RegulationID)
	}
	if len(file.Pokemon) == 0 {
		return nil, fmt.Errorf("%w: pokemon must not be empty", ErrInvalidPokemon)
	}

	seenIDs := make(map[string]bool, len(file.Pokemon))
	pokemon := make([]speed.Pokemon, 0, len(file.Pokemon))
	for _, entry := range file.Pokemon {
		if !pokemonIDPattern.MatchString(entry.PokemonID) {
			return nil, fmt.Errorf("%w: pokemonId %q must use the NNNN-NNN format", ErrInvalidPokemon, entry.PokemonID)
		}
		if seenIDs[entry.PokemonID] {
			return nil, fmt.Errorf("%w: duplicate pokemonId %q", ErrInvalidPokemon, entry.PokemonID)
		}
		seenIDs[entry.PokemonID] = true

		if strings.TrimSpace(entry.NameJa) == "" {
			return nil, fmt.Errorf("%w: pokemonId %q has an empty nameJa", ErrInvalidPokemon, entry.PokemonID)
		}

		if len(entry.Types) < minTypes || len(entry.Types) > maxTypes {
			return nil, fmt.Errorf("%w: pokemonId %q must have one or two types", ErrInvalidPokemon, entry.PokemonID)
		}
		seenTypes := make(map[string]bool, len(entry.Types))
		types := make([]string, len(entry.Types))
		for i, t := range entry.Types {
			if !typeIDPattern.MatchString(t) {
				return nil, fmt.Errorf("%w: pokemonId %q has invalid type %q", ErrInvalidPokemon, entry.PokemonID, t)
			}
			if seenTypes[t] {
				return nil, fmt.Errorf("%w: pokemonId %q has duplicate type %q", ErrInvalidPokemon, entry.PokemonID, t)
			}
			seenTypes[t] = true
			types[i] = t
		}

		if entry.BaseSpeed < minBaseSpeed || entry.BaseSpeed > maxBaseSpeed {
			return nil, fmt.Errorf("%w: pokemonId %q baseSpeed must be between %d and %d", ErrInvalidPokemon, entry.PokemonID, minBaseSpeed, maxBaseSpeed)
		}

		pokemon = append(pokemon, speed.Pokemon{
			PokemonID: entry.PokemonID,
			NameJa:    entry.NameJa,
			Types:     types,
			BaseSpeed: entry.BaseSpeed,
		})
	}

	return &PokemonReadModel{regulationID: file.RegulationID, pokemon: pokemon}, nil
}

// LoadPokemonFile は path を開いて LoadPokemon に渡す。
func LoadPokemonFile(path string) (*PokemonReadModel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadPokemon(f)
}

// Roster は speed.PokemonProvider の実装。ファイルの並び順のまま、呼び出し側が変更してよい複製を返す。
func (m *PokemonReadModel) Roster() (speed.Roster, error) {
	pokemon := make([]speed.Pokemon, len(m.pokemon))
	for i, p := range m.pokemon {
		types := make([]string, len(p.Types))
		copy(types, p.Types)
		pokemon[i] = speed.Pokemon{PokemonID: p.PokemonID, NameJa: p.NameJa, Types: types, BaseSpeed: p.BaseSpeed}
	}
	return speed.Roster{RegulationID: m.regulationID, Pokemon: pokemon}, nil
}
