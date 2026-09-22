package speed

import "errors"

// PositionMode は「自分のポケモン」の入力の型(ADR-0602 §2)。値は API の PositionRequest.mode と同じ。
type PositionMode string

const (
	// PositionModePreset は最小の選択(ポケモン + 無振り/準速/最速 + スカーフ)。
	PositionModePreset PositionMode = "preset"
	// PositionModeCustom は自由入力(ポケモン + SP + 性格 + ランク + スカーフ)。
	PositionModeCustom PositionMode = "custom"
	// PositionModeRaw は実数値の直接入力(計算しない)。
	PositionModeRaw PositionMode = "raw"
)

// 自分の位置の入力の検証の sentinel エラー(ADR-0602 §3)。
var (
	// ErrUnknownPokemon は pokemonId が roster に無いことを表す(API では 422 unknown_pokemon)。
	ErrUnknownPokemon = errors.New("unknown pokemonId")
	// ErrInvalidMode は mode が preset / custom / raw 以外であることを表す。
	ErrInvalidMode = errors.New("mode must be preset, custom or raw")
	// ErrNotMinimalPreset は preset が最小の選択の 3 つ(ADR-0602 §5)以外であることを表す。
	ErrNotMinimalPreset = errors.New("preset must be one of the minimal presets")
	// ErrInvalidRawSpeed は実数値が RawSpeedRange の外であることを表す。
	ErrInvalidRawSpeed = errors.New("raw speed is out of range")
)

// PositionRequest は「自分のポケモン」1 通り分の入力(ADR-0602 §2)。
// mode ごとに使うフィールドが違う。要らないフィールドが入っていないかの検査は HTTP 層が行う(ADR-0602 §2)。
type PositionRequest struct {
	Mode      PositionMode
	PokemonID string
	Preset    PresetID
	SP        int
	Nature    NatureEffect
	Rank      int
	Scarf     bool
	Value     int
}

// PositionResult は自分の実数値と、表の中の位置(ADR-0602 §3)。
// Pokemon は PokemonID を解決できたときだけ非 nil(raw で省略したときは nil)。
type PositionResult struct {
	Speed   int
	Pokemon *Pokemon
	Faster  int
	Slower  int
	Tie     []TableEntry
}

// errPositionUnimplemented は SP2 の spec-writer が置いたスタブの印。実装は implementer が入れる。
// panic ではなく error にしてあるのは、同じパッケージの既存のテストを道連れに落とさないため。
var errPositionUnimplemented = errors.New("position is not implemented yet")

// MinimalPresets は最小の選択で使える 3 つのプリセット(ADR-0602 §5)を ADR-0601 §2 の順で返す。
// 定義は presetDefinitions が唯一の正で、ここはそこから導く(値を書き写さない)。
func MinimalPresets() []Preset {
	return nil // unimplemented
}

// RawSpeedRange は実数値の直接入力(mode=raw)で受け付ける最小値・最大値を返す(ADR-0602 §3)。
// 種族値 1〜255・SP 0〜engine.MaxSPPerStat・性格 3 通り・ランク -6〜+6・スカーフ有無のすべての組み合わせを
// Speed で計算した最小値と最大値で、定数として書き写さない。
func RawSpeedRange() (minSpeed, maxSpeed int) {
	return 0, 0 // unimplemented
}

// Position は自分の素早さを求め、roster の 6 プリセットの表(ADR-0601 §2)の中の位置を返す(ADR-0602 §3)。
// Faster / Slower は自分より速い行・遅い行の数、Tie は自分と同じ値の段の行(無ければ空)。
func Position(roster Roster, req PositionRequest) (PositionResult, error) {
	return PositionResult{}, errPositionUnimplemented
}
