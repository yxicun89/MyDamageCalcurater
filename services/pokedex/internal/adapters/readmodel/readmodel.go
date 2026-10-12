// Package readmodel は `pokedex export`(balance・speed 向けの read model 6ファイル。ADR-0100 §8・ADR-0105 §5・ADR-0128)の
// 入出力。読み出しと組み立ては app.ExportReadModel(正規化は domain)が行い、ここは DB の口の用意と
// ファイルへの書き出しだけ(ADR-0811・ADR-0180)。HTTP に依存しない。
package readmodel

import (
	"context"
	"os"
	"path/filepath"

	"example.com/pokecalc/services/pokedex/internal/adapters/readtx"
	"example.com/pokecalc/services/pokedex/internal/adapters/store"
	"example.com/pokecalc/services/pokedex/internal/app"
	"example.com/pokecalc/services/pokedex/internal/domain"
)

// File* は WriteDir が書くファイル名(balance・speed の設定 BALANCE_*_PATH・SPEED_POKEMON_PATH が指す名前。ADR-0105 §5)。
const (
	FilePokemonTypes = "pokemon-types.json"
	FileMoves        = "moves.json"
	FileAbilities    = "abilities.json"
	FileSpeedPokemon = "speed-pokemon.json"
	FileTypeChart    = "type-chart.json" // balance の BALANCE_TYPE_CHART_PATH が指す名前(ADR-0128)
	FileMetadata     = "metadata.json"   // dataVersion。WriteDir は最後に書く(ADR-0128)
)

// ErrNoDefaultRegulation は既定のレギュレーションが無い(regulations に is_default=1 の行が無い)。
var ErrNoDefaultRegulation = app.ErrNoDefaultRegulation

// ErrInvalidExport は出力として成立しない(使用可能集合が空・効果定義が検証を通らない・
// 係数が 1〜16 の比にならない)。
var ErrInvalidExport = domain.ErrInvalidExport

// Files は export の出力(6ファイル)。
type Files app.ReadModelFiles

// Report は Export の副次的な結果。
type Report = app.ReadModelReport

// TruncatedAbility は balance の上限を超えて落とした特性(pokemonId・abilityId)。
type TruncatedAbility = app.TruncatedAbility

// IgnoredEffectField は読み捨てた効果フィールド名と、それを持っていた特性の数。
type IgnoredEffectField = app.IgnoredEffectField

// Export は DB を読み、balance・speed 向けの6ファイルを組み立てる(ADR-0105 §5)。
// 対象は既定のレギュレーションの使用可能集合。失敗時は Files のゼロ値を返す(部分的な出力をしない)。
func Export(ctx context.Context, b readtx.Beginner) (Files, Report, error) {
	files, rep, err := app.ExportReadModel(ctx, store.NewBeginner(b))
	if err != nil {
		return Files{}, Report{}, err
	}
	return Files(files), rep, nil
}

// WriteDir は6ファイルを dir に書く(metadata.json を最後に。読めた時点で他の5ファイルは同じ export の中身)。無ければ作る。各ファイルは同じディレクトリの一時ファイルに
// 書いてから rename する(読む側が書きかけを読まない。ADR-0105 §5)。dir がファイルならエラー。
func (f Files) WriteDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{
		{FilePokemonTypes, f.PokemonTypes},
		{FileMoves, f.Moves},
		{FileAbilities, f.Abilities},
		{FileSpeedPokemon, f.SpeedPokemon},
		{FileTypeChart, f.TypeChart},
		{FileMetadata, f.Metadata},
	}
	for _, file := range files {
		if err := writeFileAtomic(dir, file.name, file.data); err != nil {
			return err
		}
	}
	return nil
}

func writeFileAtomic(dir, name string, data []byte) (err error) {
	tmp, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpPath, filepath.Join(dir, name)); err != nil {
		return err
	}
	return nil
}
