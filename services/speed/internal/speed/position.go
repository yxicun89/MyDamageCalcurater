package speed

import (
	"errors"
	"sync"

	"example.com/pokecalc/engine"
)

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

// MinimalPresets は最小の選択で使える 3 つのプリセット(ADR-0602 §5)を ADR-0601 §2 の順で返す。
// 定義は presetDefinitions が唯一の正で、ここはそこから導く(値を書き写さない)。返す slice の要素は
// 値のコピーなので、呼び出し側が変更しても presetDefinitions には影響しない。
func MinimalPresets() []Preset {
	ids := []PresetID{PresetUninvested, PresetNeutralMax, PresetMax}
	out := make([]Preset, 0, len(ids))
	for _, id := range ids {
		if def, ok := presetByID(id); ok {
			out = append(out, def)
		}
	}
	return out
}

// isMinimalPreset は id が MinimalPresets() のいずれかと一致するかを返す(ADR-0602 §5)。
func isMinimalPreset(id PresetID) bool {
	for _, p := range MinimalPresets() {
		if p.ID == id {
			return true
		}
	}
	return false
}

// rawSpeedRangeOnce は総当たり(65万通り)を初回だけ実行し、以後はキャッシュした結果を返す
// (ハンドラごとに回さない。ADR-0602 §3)。
var rawSpeedRangeOnce = sync.OnceValues(computeRawSpeedRange)

// RawSpeedRange は実数値の直接入力(mode=raw)で受け付ける最小値・最大値を返す(ADR-0602 §3)。
// 種族値 1〜255・SP 0〜engine.MaxSPPerStat・性格 3 通り・ランク -6〜+6・スカーフ有無のすべての組み合わせを
// Speed で計算した最小値と最大値で、定数として書き写さない。
func RawSpeedRange() (minSpeed, maxSpeed int) {
	return rawSpeedRangeOnce()
}

// computeRawSpeedRange は Speed の全組み合わせを総当たりして最小値・最大値を求める(ADR-0602 §3)。
// テストの bruteForceRawSpeedRange(独立に導出する)とは別の実装(名前が衝突しないようにしてある)。
func computeRawSpeedRange() (minSpeed, maxSpeed int) {
	natures := []NatureEffect{NatureMinus, NatureNeutral, NaturePlus}
	first := true
	for base := minBaseSpeed; base <= maxBaseSpeed; base++ {
		for sp := 0; sp <= engine.MaxSPPerStat; sp++ {
			for _, nature := range natures {
				for rank := minRank; rank <= maxRank; rank++ {
					for _, scarf := range []bool{false, true} {
						v, err := Speed(Input{BaseSpeed: base, SP: sp, Nature: nature, Rank: rank, Scarf: scarf})
						if err != nil {
							// base・sp・rank はここで生成した範囲内の値なので、Speed が拒否することはない。
							continue
						}
						if first || v < minSpeed {
							minSpeed = v
						}
						if first || v > maxSpeed {
							maxSpeed = v
						}
						first = false
					}
				}
			}
		}
	}
	return minSpeed, maxSpeed
}

// resolvePokemon は pokemonID を roster から探す。見つからなければ ErrUnknownPokemon(空文字も含む。
// roster に空の PokemonID は無いため自然に未知として扱われる)。
func resolvePokemon(roster Roster, pokemonID string) (Pokemon, error) {
	for _, p := range roster.Pokemon {
		if p.PokemonID == pokemonID {
			return p, nil
		}
	}
	return Pokemon{}, ErrUnknownPokemon
}

// positionResult は roster の 6 プリセットの表(ADR-0601 §2)を組み立て、speedValue の位置
// (faster/slower/tie)を数える(ADR-0602 §3)。Tie は同速なしでも空 slice(nil にしない)。
func positionResult(roster Roster, speedValue int, pokemon *Pokemon) (PositionResult, error) {
	defs := Presets()
	ids := make([]PresetID, len(defs))
	for i, p := range defs {
		ids[i] = p.ID
	}

	table, err := BuildTable(roster, ids)
	if err != nil {
		return PositionResult{}, err
	}

	result := PositionResult{Speed: speedValue, Pokemon: pokemon, Tie: []TableEntry{}}
	for _, tier := range table.Tiers {
		switch {
		case tier.Speed > speedValue:
			result.Faster += len(tier.Entries)
		case tier.Speed < speedValue:
			result.Slower += len(tier.Entries)
		default:
			result.Tie = append(result.Tie, tier.Entries...)
		}
	}
	return result, nil
}

// positionFromPreset は mode=preset を計算する(ADR-0602 §2): preset は最小の選択の 3 つのいずれかで
// なければ ErrNotMinimalPreset、pokemonId が roster に無ければ ErrUnknownPokemon。
func positionFromPreset(roster Roster, req PositionRequest) (PositionResult, error) {
	if !isMinimalPreset(req.Preset) {
		return PositionResult{}, ErrNotMinimalPreset
	}
	pokemon, err := resolvePokemon(roster, req.PokemonID)
	if err != nil {
		return PositionResult{}, err
	}
	def, ok := presetByID(req.Preset)
	if !ok {
		return PositionResult{}, ErrNotMinimalPreset
	}
	speedValue, err := Speed(Input{BaseSpeed: pokemon.BaseSpeed, SP: def.SP, Nature: def.Nature, Rank: def.Rank, Scarf: req.Scarf})
	if err != nil {
		return PositionResult{}, err
	}
	return positionResult(roster, speedValue, &pokemon)
}

// positionFromCustom は mode=custom を計算する(ADR-0602 §2): SP・性格・ランクの検証は Speed に任せ
// (sentinel を複製しない)、pokemonId が roster に無ければ ErrUnknownPokemon。
func positionFromCustom(roster Roster, req PositionRequest) (PositionResult, error) {
	pokemon, err := resolvePokemon(roster, req.PokemonID)
	if err != nil {
		return PositionResult{}, err
	}
	speedValue, err := Speed(Input{BaseSpeed: pokemon.BaseSpeed, SP: req.SP, Nature: req.Nature, Rank: req.Rank, Scarf: req.Scarf})
	if err != nil {
		return PositionResult{}, err
	}
	return positionResult(roster, speedValue, &pokemon)
}

// positionFromRaw は mode=raw を計算する(ADR-0602 §2): value は RawSpeedRange の外なら
// ErrInvalidRawSpeed。pokemonId は表示用の任意項目で、省略時は未知の検査をしない(ADR-0602 §4)。
func positionFromRaw(roster Roster, req PositionRequest) (PositionResult, error) {
	minSpeed, maxSpeed := RawSpeedRange()
	if req.Value < minSpeed || req.Value > maxSpeed {
		return PositionResult{}, ErrInvalidRawSpeed
	}

	var pokemon *Pokemon
	if req.PokemonID != "" {
		resolved, err := resolvePokemon(roster, req.PokemonID)
		if err != nil {
			return PositionResult{}, err
		}
		pokemon = &resolved
	}
	return positionResult(roster, req.Value, pokemon)
}

// Position は自分の素早さを求め、roster の 6 プリセットの表(ADR-0601 §2)の中の位置を返す(ADR-0602 §3)。
// Faster / Slower は自分より速い行・遅い行の数、Tie は自分と同じ値の段の行(無ければ空。自分自身の行も含む)。
func Position(roster Roster, req PositionRequest) (PositionResult, error) {
	switch req.Mode {
	case PositionModePreset:
		return positionFromPreset(roster, req)
	case PositionModeCustom:
		return positionFromCustom(roster, req)
	case PositionModeRaw:
		return positionFromRaw(roster, req)
	default:
		return PositionResult{}, ErrInvalidMode
	}
}
