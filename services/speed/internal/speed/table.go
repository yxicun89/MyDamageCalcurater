package speed

import (
	"errors"
	"fmt"
	"sort"

	"example.com/pokecalc/engine"
)

// PresetID は表の行の型(プリセット)の ID(ADR-0601 §2)。値は API の PresetId の enum と同じ。
type PresetID string

// 表の行の型の ID(ADR-0601 §2)。表示名は API に含めず、クライアントの文言資源が持つ。
const (
	// PresetUninvested は無振り(SP 0・性格補正なし・ランク 0・スカーフなし)。
	PresetUninvested PresetID = "uninvested"
	// PresetNeutralMax は準速(SP 32・性格補正なし)。
	PresetNeutralMax PresetID = "neutral-max"
	// PresetMax は最速(SP 32・素早さ上昇の性格)。
	PresetMax PresetID = "max"
	// PresetMaxScarf は最速スカーフ(最速 + こだわりスカーフ)。
	PresetMaxScarf PresetID = "max-scarf"
	// PresetMaxPlus1 は最速+1(最速 + ランク +1)。
	PresetMaxPlus1 PresetID = "max-plus1"
	// PresetMaxPlus2 は最速+2(最速 + ランク +2)。
	PresetMaxPlus2 PresetID = "max-plus2"
)

// Preset は表の 1 行の「入力の作り方の型」(ADR-0601 §2。ADR-0009 と同じ考え方でマスタではない)。
// 種族値はポケモンごとに Roster から与える。
type Preset struct {
	ID     PresetID
	SP     int
	Nature NatureEffect
	Rank   int
	Scarf  bool
}

// プリセットの指定の検証の sentinel エラー(ADR-0601 §4)。
var (
	// ErrNoPresets はプリセットの指定が空であることを表す。
	ErrNoPresets = errors.New("at least one preset is required")
	// ErrUnknownPreset は未知のプリセット ID を表す。
	ErrUnknownPreset = errors.New("unknown preset")
	// ErrDuplicatePreset は同じプリセット ID の重複を表す。
	ErrDuplicatePreset = errors.New("duplicate preset")
)

// TableEntry は表の 1 行: 1 体のポケモンと 1 つのプリセットの組(ADR-0601 §5)。
type TableEntry struct {
	Pokemon Pokemon
	Preset  PresetID
}

// Tier は同じ素早さの行をまとめた段(ADR-0601 §3)。Entries が 2 件以上なら同速。
type Tier struct {
	Speed   int
	Entries []TableEntry
}

// Table は素早さの表(ADR-0601 §3・§5)。Presets は実際に使った行の ID を ADR-0601 §2 の順で持つ。
type Table struct {
	RegulationID string
	Presets      []PresetID
	Tiers        []Tier
}

// presetDefinitions は表の 6 行の定義(ADR-0601 §2)を唯一持つ場所。Presets・NormalizePresets・
// BuildTable はすべてここから導く。呼び出し側に公開する前に複製すること(可変のグローバルを渡さない)。
var presetDefinitions = []Preset{
	{ID: PresetUninvested, SP: 0, Nature: NatureNeutral, Rank: 0, Scarf: false},
	{ID: PresetNeutralMax, SP: engine.MaxSPPerStat, Nature: NatureNeutral, Rank: 0, Scarf: false},
	{ID: PresetMax, SP: engine.MaxSPPerStat, Nature: NaturePlus, Rank: 0, Scarf: false},
	{ID: PresetMaxScarf, SP: engine.MaxSPPerStat, Nature: NaturePlus, Rank: 0, Scarf: true},
	{ID: PresetMaxPlus1, SP: engine.MaxSPPerStat, Nature: NaturePlus, Rank: 1, Scarf: false},
	{ID: PresetMaxPlus2, SP: engine.MaxSPPerStat, Nature: NaturePlus, Rank: 2, Scarf: false},
}

// Presets は 6 つのプリセットを ADR-0601 §2 の順で返す。返す slice は呼び出し側が変更してよい複製。
func Presets() []Preset {
	out := make([]Preset, len(presetDefinitions))
	copy(out, presetDefinitions)
	return out
}

// NormalizePresets はプリセットの指定を検証し、ADR-0601 §2 の順に並べ直して返す(指定の順は結果に影響しない)。
// 空は ErrNoPresets、未知の ID は ErrUnknownPreset、重複は ErrDuplicatePreset を包んで返す。
func NormalizePresets(ids []PresetID) ([]PresetID, error) {
	if len(ids) == 0 {
		return nil, ErrNoPresets
	}

	seen := make(map[PresetID]bool, len(ids))
	for _, id := range ids {
		if !isKnownPreset(id) {
			return nil, fmt.Errorf("%q: %w", id, ErrUnknownPreset)
		}
		if seen[id] {
			return nil, fmt.Errorf("%q: %w", id, ErrDuplicatePreset)
		}
		seen[id] = true
	}

	normalized := make([]PresetID, 0, len(seen))
	for _, p := range presetDefinitions {
		if seen[p.ID] {
			normalized = append(normalized, p.ID)
		}
	}
	return normalized, nil
}

// isKnownPreset は id が presetDefinitions のいずれかの ID と一致するかを返す。
func isKnownPreset(id PresetID) bool {
	for _, p := range presetDefinitions {
		if p.ID == id {
			return true
		}
	}
	return false
}

// presetByID は presetDefinitions から id の定義を返す(SP2 の MinimalPresets・Position が使う)。
func presetByID(id PresetID) (Preset, bool) {
	for _, p := range presetDefinitions {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// BuildTable は roster の各ポケモンについて presets の各行の素早さを Speed で計算し、同じ値を 1 つの段に
// まとめた表を返す(ADR-0601 §3)。段は素早さの降順、段の中は pokemonId の昇順 → ADR-0601 §2 の順。
// presets の検証は NormalizePresets と同じ。Speed のエラー(種族値の範囲外など)は包んで返す。
func BuildTable(roster Roster, presets []PresetID) (Table, error) {
	normalized, err := NormalizePresets(presets)
	if err != nil {
		return Table{}, err
	}

	definitionByID := make(map[PresetID]Preset, len(presetDefinitions))
	for _, p := range presetDefinitions {
		definitionByID[p.ID] = p
	}

	// row は 1 行分の計算結果と、段の中の並びに使う 2 次キー(プリセットの ADR-0601 §2 の順)。
	type row struct {
		entry     TableEntry
		speed     int
		presetIdx int
	}

	rows := make([]row, 0, len(roster.Pokemon)*len(normalized))
	for _, pokemon := range roster.Pokemon {
		for i, id := range normalized {
			def := definitionByID[id]
			v, err := Speed(Input{
				BaseSpeed: pokemon.BaseSpeed,
				SP:        def.SP,
				Nature:    def.Nature,
				Rank:      def.Rank,
				Scarf:     def.Scarf,
			})
			if err != nil {
				return Table{}, fmt.Errorf("pokemon %s preset %s: %w", pokemon.PokemonID, id, err)
			}
			rows = append(rows, row{entry: TableEntry{Pokemon: pokemon, Preset: id}, speed: v, presetIdx: i})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].speed != rows[j].speed {
			return rows[i].speed > rows[j].speed
		}
		if rows[i].entry.Pokemon.PokemonID != rows[j].entry.Pokemon.PokemonID {
			return rows[i].entry.Pokemon.PokemonID < rows[j].entry.Pokemon.PokemonID
		}
		return rows[i].presetIdx < rows[j].presetIdx
	})

	var tiers []Tier
	for _, r := range rows {
		if len(tiers) == 0 || tiers[len(tiers)-1].Speed != r.speed {
			tiers = append(tiers, Tier{Speed: r.speed})
		}
		tiers[len(tiers)-1].Entries = append(tiers[len(tiers)-1].Entries, r.entry)
	}

	return Table{RegulationID: roster.RegulationID, Presets: normalized, Tiers: tiers}, nil
}
