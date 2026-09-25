package engine

// 防御側プリセットのカタログ(ADR-0009 §1、2026-09-25 追記。攻撃側の ADR-0114 と同じ形)。
//
// 正は engine/presets/defender.json の1か所で、ビルド時に embed する(実行時のファイル I/O は無い。CLAUDE.md 絶対ルール2)。
// Web・iOS は同じ JSON を契約テストで読み、自分の定義(キー・順序・SP・性格・applies・表示名)と一致することを確かめる。
// 攻撃側と違い表示名(label)も持つ: engine がそれを一括計算の行の PresetLabel として返し、API の presetLabel に出るため。

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

//go:embed presets/defender.json
var defenderPresetJSON []byte

// defenderPresetSchemaVersion は engine が読める presets/defender.json の版。
const defenderPresetSchemaVersion = 1

// defenderPresetJSONEntry は presets/defender.json の1件の形。欠けたフィールドを検出できるようにポインタで受ける。
type defenderPresetJSONEntry struct {
	Key    *PresetKey        `json:"key"`
	Label  *string           `json:"label"`
	SP     map[StatKey]int   `json:"sp"`
	Nature *defenderNatureJS `json:"nature"`
	// Applies は既定セットに入る技の分類。"" は全分類。
	Applies *MoveCategory `json:"applies"`
}

type defenderNatureJS struct {
	Plus  *StatKey `json:"plus"`
	Minus *StatKey `json:"minus"`
}

type defenderPresetJSONCatalog struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Presets       []defenderPresetJSONEntry `json:"presets"`
}

// defenderPresets は embed したカタログ。壊れていれば起動時に panic する
// (埋め込みデータの誤りでありテストが必ず検出する。黙って空のカタログにしない)。
var defenderPresets = mustParseDefenderPresetCatalog(defenderPresetJSON)

func mustParseDefenderPresetCatalog(data []byte) []DefenderPreset {
	c, err := parseDefenderPresetCatalog(data)
	if err != nil {
		panic(err)
	}
	return c
}

// parseDefenderPresetCatalog はカタログを未知フィールド拒否・全フィールド必須で読み、定義を検証する。
func parseDefenderPresetCatalog(data []byte) ([]DefenderPreset, error) {
	var raw defenderPresetJSONCatalog
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPreset, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: JSON の後ろに余分なデータがある", ErrInvalidPreset)
	}
	if raw.SchemaVersion != defenderPresetSchemaVersion {
		return nil, fmt.Errorf("%w: schemaVersion=%d(読めるのは %d)", ErrInvalidPreset, raw.SchemaVersion, defenderPresetSchemaVersion)
	}
	if len(raw.Presets) == 0 || len(raw.Presets) > MaxBulkPresets {
		return nil, fmt.Errorf("%w: presets の件数 %d は 1..%d の範囲外", ErrInvalidPreset, len(raw.Presets), MaxBulkPresets)
	}
	out := make([]DefenderPreset, 0, len(raw.Presets))
	seen := map[PresetKey]bool{}
	for i, e := range raw.Presets {
		p, err := e.toPreset()
		if err != nil {
			return nil, fmt.Errorf("%w: presets[%d]: %v", ErrInvalidPreset, i, err)
		}
		if err := p.validate(); err != nil {
			return nil, err
		}
		if seen[p.Key] {
			return nil, fmt.Errorf("%w: key %q が重複", ErrInvalidPreset, p.Key)
		}
		seen[p.Key] = true
		out = append(out, p)
	}
	return out, nil
}

func (e defenderPresetJSONEntry) toPreset() (DefenderPreset, error) {
	if e.Key == nil || e.Label == nil || e.SP == nil || e.Nature == nil || e.Nature.Plus == nil || e.Nature.Minus == nil || e.Applies == nil {
		return DefenderPreset{}, errors.New("key・label・sp・nature.plus・nature.minus・applies は必須")
	}
	var sp Stats
	for _, k := range AllStatKeys() {
		v, ok := e.SP[k]
		if !ok {
			return DefenderPreset{}, fmt.Errorf("sp に %q が無い", k)
		}
		sp = sp.WithStat(k, v)
	}
	if len(e.SP) != len(AllStatKeys()) {
		return DefenderPreset{}, fmt.Errorf("sp は %v をちょうど持つ", AllStatKeys())
	}
	nature := Nature{Plus: *e.Nature.Plus, Minus: *e.Nature.Minus}
	if nature != NatureNeutral {
		if !slices.Contains(rankStatKeys, nature.Plus) || !slices.Contains(rankStatKeys, nature.Minus) || nature.Plus == nature.Minus {
			return DefenderPreset{}, fmt.Errorf("nature=%+v は「両方空」か「HP 以外の異なる2ステータス」でない", nature)
		}
	}
	switch *e.Applies {
	case "", CategoryPhysical, CategorySpecial, CategoryStatus:
	default:
		return DefenderPreset{}, fmt.Errorf("applies=%q は未知の技の分類", *e.Applies)
	}
	return DefenderPreset{Key: *e.Key, Label: *e.Label, SP: sp, Nature: nature, Applies: *e.Applies}, nil
}
