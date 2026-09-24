package engine

// 攻撃側(自分側)プリセットのカタログ(ADR-0114。issue #71)。
//
// 正は engine/presets/attacker.json の1か所で、ビルド時に embed する(実行時のファイル I/O は無い。CLAUDE.md 絶対ルール2)。
// Web・iOS は同じ JSON を契約テストで読み、自分の定義(キー・順序・SP・性格規則)と一致することを確かめる。
// 表示の文言(「A特化」など)は持たない。各クライアントの文言資源に残す。

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

//go:embed presets/attacker.json
var attackerPresetJSON []byte

// attackerPresetSchemaVersion は engine が読める presets/attacker.json の版。
const attackerPresetSchemaVersion = 1

var (
	// ErrUnknownAttackerPreset はカタログに無い攻撃側プリセットのキーが指定された。
	ErrUnknownAttackerPreset = errors.New("未知の攻撃側プリセット")
	// ErrInvalidAttackerPreset はカタログの定義、または解決に渡した技の分類が不正。
	ErrInvalidAttackerPreset = errors.New("攻撃側プリセットの定義が不正")
)

// AttackerPresetKey は攻撃側プリセットのキー(none = 無振り、x_full = X特化、x = X振り(無補正))。
// 値の一覧は presets/attacker.json が正で、Go の定数には持たない。
type AttackerPresetKey string

// AttackerNatureRule は攻撃側プリセットの性格の決め方。
type AttackerNatureRule string

const (
	// AttackerNatureNeutral は無補正の性格。
	AttackerNatureNeutral AttackerNatureRule = "neutral"
	// AttackerNatureBoost は関連ステータス X が上昇、boostMinus[X] が下降の性格。
	AttackerNatureBoost AttackerNatureRule = "boost"
)

// AttackerPreset は攻撃側プリセット1件の定義。X(関連ステータス)は技の分類で決まる。
type AttackerPreset struct {
	Key AttackerPresetKey `json:"key"`
	// RelevantSP は X に振る SP(0..MaxSPPerStat)。他のステータスは 0。
	RelevantSP int                `json:"relevantSp"`
	Nature     AttackerNatureRule `json:"nature"`
}

// attackerPresetCatalog は presets/attacker.json の形。
type attackerPresetCatalog struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Default       AttackerPresetKey        `json:"default"`
	RelevantStat  map[MoveCategory]StatKey `json:"relevantStat"`
	BoostMinus    map[StatKey]StatKey      `json:"boostMinus"`
	Presets       []AttackerPreset         `json:"presets"`
}

// attackerPresets は embed したカタログ。壊れていれば起動時に panic する
// (埋め込みデータの誤りでありテストが必ず検出する。黙って空のカタログにしない)。
var attackerPresets = mustParseAttackerPresetCatalog(attackerPresetJSON)

func mustParseAttackerPresetCatalog(data []byte) attackerPresetCatalog {
	c, err := parseAttackerPresetCatalog(data)
	if err != nil {
		panic(err)
	}
	return c
}

// parseAttackerPresetCatalog はカタログを未知フィールド拒否で読み、定義を検証する。
func parseAttackerPresetCatalog(data []byte) (attackerPresetCatalog, error) {
	var c attackerPresetCatalog
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%w: %v", ErrInvalidAttackerPreset, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("%w: JSON の後ろに余分なデータがある", ErrInvalidAttackerPreset)
	}
	if err := c.validate(); err != nil {
		return c, fmt.Errorf("%w: %v", ErrInvalidAttackerPreset, err)
	}
	return c, nil
}

func (c attackerPresetCatalog) validate() error {
	if c.SchemaVersion != attackerPresetSchemaVersion {
		return fmt.Errorf("schemaVersion=%d(読めるのは %d)", c.SchemaVersion, attackerPresetSchemaVersion)
	}
	categories := []MoveCategory{CategoryPhysical, CategorySpecial, CategoryStatus}
	if len(c.RelevantStat) != len(categories) {
		return fmt.Errorf("relevantStat は %v をちょうど持つ", categories)
	}
	used := map[StatKey]bool{}
	for _, cat := range categories {
		stat, ok := c.RelevantStat[cat]
		if !ok {
			return fmt.Errorf("relevantStat に %q が無い", cat)
		}
		if !slices.Contains(rankStatKeys, stat) {
			return fmt.Errorf("relevantStat[%q]=%q は HP 以外のステータスでない", cat, stat)
		}
		used[stat] = true
	}
	if len(c.BoostMinus) != len(used) {
		return fmt.Errorf("boostMinus は relevantStat に現れるステータスだけをちょうど持つ")
	}
	for stat := range used {
		minus, ok := c.BoostMinus[stat]
		if !ok {
			return fmt.Errorf("boostMinus に %q が無い", stat)
		}
		if minus == stat || !slices.Contains(rankStatKeys, minus) {
			return fmt.Errorf("boostMinus[%q]=%q は X と異なる HP 以外のステータスでない", stat, minus)
		}
	}
	if len(c.Presets) == 0 {
		return errors.New("presets が空")
	}
	seen := map[AttackerPresetKey]bool{}
	for _, p := range c.Presets {
		if p.Key == "" {
			return errors.New("key が空")
		}
		if seen[p.Key] {
			return fmt.Errorf("key %q が重複", p.Key)
		}
		seen[p.Key] = true
		if p.RelevantSP < 0 || p.RelevantSP > MaxSPPerStat {
			return fmt.Errorf("%q の relevantSp=%d が範囲外 [0, %d]", p.Key, p.RelevantSP, MaxSPPerStat)
		}
		if p.Nature != AttackerNatureNeutral && p.Nature != AttackerNatureBoost {
			return fmt.Errorf("%q の nature=%q は未知の規則", p.Key, p.Nature)
		}
	}
	if !seen[c.Default] {
		return fmt.Errorf("default=%q が presets に無い", c.Default)
	}
	return nil
}

// AttackerPresetCatalog は攻撃側プリセットを画面の並び順で返す。
// 呼び出しごとに新しいスライスを返し、呼び出し側の変更が次回に漏れないようにする。
func AttackerPresetCatalog() []AttackerPreset {
	return slices.Clone(attackerPresets.Presets)
}

// DefaultAttackerPreset は既定の攻撃側プリセットのキーを返す。
func DefaultAttackerPreset() AttackerPresetKey {
	return attackerPresets.Default
}

// ResolveAttackerPreset はキーと技の分類から攻撃側の SP と性格を求める。
func ResolveAttackerPreset(key AttackerPresetKey, category MoveCategory) (Stats, Nature, error) {
	i := slices.IndexFunc(attackerPresets.Presets, func(p AttackerPreset) bool { return p.Key == key })
	if i < 0 {
		return Stats{}, Nature{}, fmt.Errorf("%w: %q", ErrUnknownAttackerPreset, key)
	}
	stat, ok := attackerPresets.RelevantStat[category]
	if !ok {
		return Stats{}, Nature{}, fmt.Errorf("%w: 技の分類 %q", ErrInvalidAttackerPreset, category)
	}
	p := attackerPresets.Presets[i]
	sp := Stats{}.WithStat(stat, p.RelevantSP)
	nature := NatureNeutral
	if p.Nature == AttackerNatureBoost {
		nature = Nature{Plus: stat, Minus: attackerPresets.BoostMinus[stat]}
	}
	return sp, nature, nil
}
