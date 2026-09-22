package speed

import "errors"

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

// Presets は 6 つのプリセットを ADR-0601 §2 の順で返す。返す slice は呼び出し側が変更してよい複製。
func Presets() []Preset {
	panic("unimplemented")
}

// NormalizePresets はプリセットの指定を検証し、ADR-0601 §2 の順に並べ直して返す(指定の順は結果に影響しない)。
// 空は ErrNoPresets、未知の ID は ErrUnknownPreset、重複は ErrDuplicatePreset を包んで返す。
func NormalizePresets(ids []PresetID) ([]PresetID, error) {
	panic("unimplemented")
}

// BuildTable は roster の各ポケモンについて presets の各行の素早さを Speed で計算し、同じ値を 1 つの段に
// まとめた表を返す(ADR-0601 §3)。段は素早さの降順、段の中は pokemonId の昇順 → ADR-0601 §2 の順。
// presets の検証は NormalizePresets と同じ。Speed のエラー(種族値の範囲外など)は包んで返す。
func BuildTable(roster Roster, presets []PresetID) (Table, error) {
	panic("unimplemented")
}
