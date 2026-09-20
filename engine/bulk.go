package engine

// 一括計算(P1-7)。防御側の代表調整(プリセット)× 持ち物バリアントの全行を一度に返す。
//
// 要件(docs/requirements.md「相手側の一括表示」): 相手側は入力させず、
// 無振り / H振り / HB特化(またはHD特化)/ H振り+B(D)補正 を並べて表示する。
//
// 設計は ADR-0009:
//   - プリセット定義は引数(データ)として受け取る。既定値は engine の純粋関数として持ち、
//     WASM と calc-svc の双方が同じ既定で動く。呼び出し側は Presets で上書きできる。
//   - engine に持ち物一覧は持ち込まない。解決済みの *Item を受け取るだけ(ADR-0005)。
//   - CalcBulk は CalcDamage の合成にすぎない。独自のダメージ計算をしてはならない。

import (
	"errors"
	"fmt"
)

// プリセット定義・選択の不正。呼び出し側は errors.Is で判別する。
var (
	// ErrUnknownPreset は PresetKeys に定義の無いキーが含まれていた。
	ErrUnknownPreset = errors.New("未知の防御側プリセット")
	// ErrDuplicatePreset はプリセットのキーが重複している(行が一意にならない)。
	ErrDuplicatePreset = errors.New("防御側プリセットが重複している")
	// ErrInvalidPreset はプリセット定義そのものが不正(SP 範囲・合計、性格、キー空)。
	ErrInvalidPreset = errors.New("防御側プリセットの定義が不正")
)

// PresetKey は防御側の代表調整のキー。OpenAPI の DefenderPreset enum と1対1に対応する。
type PresetKey string

const (
	PresetNone    PresetKey = "none"     // 無振り
	PresetHP      PresetKey = "hp"       // H振り
	PresetHB      PresetKey = "hb"       // HB特化
	PresetHD      PresetKey = "hd"       // HD特化
	PresetHBBoost PresetKey = "hb_boost" // H振り + B補正
	PresetHDBoost PresetKey = "hd_boost" // H振り + D補正
)

// DefenderPreset は防御側の代表調整の定義。マスタのキー(性格ID等)は持たず、
// SP と性格補正を構造値として持つため、WASM でもそのまま使える。
type DefenderPreset struct {
	Key    PresetKey
	Label  string // 表示名。空のときは Key を表示名に使う
	SP     Stats  // 各 0..32、合計 <= 66
	Nature Nature // HP には補正を掛けられない
	// Applies は既定セットに入る技の分類。空("")は全分類。
	// physical なら物理技の既定セット、special なら特殊技の既定セットにだけ入る。
	Applies MoveCategory
}

// BulkInput は一括計算の入力。攻撃側・技・場は1つに固定し、防御側だけを振り替える。
type BulkInput struct {
	Format          Format
	Attacker        Individual
	DefenderSpecies Species
	Move            Move
	Field           Field
	Critical        bool

	// Presets は使用するプリセット定義。空のときは既定カタログ / 既定セットを使う(ADR-0009)。
	Presets []DefenderPreset
	// PresetKeys は Presets(空なら DefenderPresetCatalog())からキーで選ぶ。
	// 指定した順序がそのまま行の順序になる。空のときは選別しない。
	PresetKeys []PresetKey
	// ItemVariants は差し替えて比較する持ち物(解決済み)。nil / 空は「素の1通り」= 持ち物なし。
	ItemVariants []*Item
}

// BulkRow は一括計算の1行(プリセット × 持ち物)。
type BulkRow struct {
	Preset      PresetKey
	PresetLabel string
	Item        *Item  // 渡された *Item をそのまま保持する(nil は持ち物なし)
	ItemID      string // Item.ID。Item が nil なら空文字
	Defender    Individual
	Result      DamageResult
}

// BulkResult は一括計算の結果。Rows の順序はプリセット優先(preset-major)で決定的。
type BulkResult struct {
	DefenderSpeciesKey string
	Rows               []BulkRow
}

// DefenderPresetCatalog は既定のプリセット定義6件を耐久が上がる順に返す(ADR-0009)。
// 呼び出しごとに新しいスライスを返し、呼び出し側の変更が次回に漏れないようにする。
func DefenderPresetCatalog() []DefenderPreset {
	return []DefenderPreset{
		{Key: PresetNone, Label: "無振り", SP: Stats{}, Nature: NatureNeutral},
		{Key: PresetHP, Label: "H振り", SP: Stats{HP: 32}, Nature: NatureNeutral},
		{Key: PresetHBBoost, Label: "H振り+B補正", SP: Stats{HP: 32}, Nature: Nature{Plus: StatDef, Minus: StatAtk}, Applies: CategoryPhysical},
		{Key: PresetHB, Label: "HB特化", SP: Stats{HP: 32, Def: 32}, Nature: Nature{Plus: StatDef, Minus: StatAtk}, Applies: CategoryPhysical},
		{Key: PresetHDBoost, Label: "H振り+D補正", SP: Stats{HP: 32}, Nature: Nature{Plus: StatSpD, Minus: StatAtk}, Applies: CategorySpecial},
		{Key: PresetHD, Label: "HD特化", SP: Stats{HP: 32, SpD: 32}, Nature: Nature{Plus: StatSpD, Minus: StatAtk}, Applies: CategorySpecial},
	}
}

// DefaultDefenderPresets は技の分類に応じた既定セットを返す。
// 物理なら B 系、特殊なら D 系。変化技・分類なしは none と hp だけ。
func DefaultDefenderPresets(category MoveCategory) []DefenderPreset {
	var out []DefenderPreset
	for _, p := range DefenderPresetCatalog() {
		if p.Applies == "" || p.Applies == category {
			out = append(out, p)
		}
	}
	return out
}

// Defender はプリセットと種族・持ち物から防御側個体を組み立てる。
// Level=50、Status=none、Ranks=0、Ability=ゼロ値に固定する(ADR-0009)。
func (p DefenderPreset) Defender(species Species, item *Item) Individual {
	return Individual{
		Species: species,
		Level:   DefaultLevel,
		Nature:  p.Nature,
		SP:      p.SP,
		Item:    item,
		Status:  StatusNone,
	}
}

// validate はプリセット定義の妥当性(キー非空・SP 範囲と合計・性格が HP を指さない)を検証する。
func (p DefenderPreset) validate() error {
	if p.Key == "" {
		return fmt.Errorf("%w: キーが空", ErrInvalidPreset)
	}
	for _, k := range AllStatKeys {
		if v := p.SP.Get(k); v < 0 || v > MaxSPPerStat {
			return fmt.Errorf("%w: %q の SP %s は 0..%d の範囲外: %d", ErrInvalidPreset, p.Key, k, MaxSPPerStat, v)
		}
	}
	if sum := p.SP.Sum(); sum > MaxSPTotal {
		return fmt.Errorf("%w: %q の SP 合計が上限 %d を超過: %d", ErrInvalidPreset, p.Key, MaxSPTotal, sum)
	}
	if p.Nature.Plus == StatHP || p.Nature.Minus == StatHP {
		return fmt.Errorf("%w: %q の性格補正は HP に適用できない", ErrInvalidPreset, p.Key)
	}
	return nil
}

// selectPresets は入力から使用するプリセットを決める(ADR-0009 §3, §5)。入力は変更しない。
func selectPresets(in BulkInput) ([]DefenderPreset, error) {
	if len(in.PresetKeys) > 0 {
		source := in.Presets
		if len(source) == 0 {
			source = DefenderPresetCatalog()
		}
		byKey := make(map[PresetKey]DefenderPreset, len(source))
		for _, p := range source {
			if _, dup := byKey[p.Key]; dup {
				return nil, fmt.Errorf("%w: %q", ErrDuplicatePreset, p.Key)
			}
			byKey[p.Key] = p
		}
		out := make([]DefenderPreset, 0, len(in.PresetKeys))
		seen := make(map[PresetKey]bool, len(in.PresetKeys))
		for _, key := range in.PresetKeys {
			if seen[key] {
				return nil, fmt.Errorf("%w: %q", ErrDuplicatePreset, key)
			}
			seen[key] = true
			p, ok := byKey[key]
			if !ok {
				return nil, fmt.Errorf("%w: %q", ErrUnknownPreset, key)
			}
			out = append(out, p)
		}
		return out, nil
	}
	if len(in.Presets) > 0 {
		seen := make(map[PresetKey]bool, len(in.Presets))
		for _, p := range in.Presets {
			if seen[p.Key] {
				return nil, fmt.Errorf("%w: %q", ErrDuplicatePreset, p.Key)
			}
			seen[p.Key] = true
		}
		return in.Presets, nil
	}
	return DefaultDefenderPresets(in.Move.Category), nil
}

// CalcBulk は防御側の代表調整 × 持ち物バリアントの全行を計算する。
// 各行は同じ入力に対する CalcDamage と完全に一致しなければならない。
// エラー時は部分的な行を返さない。
func CalcBulk(in BulkInput) (BulkResult, error) {
	presets, err := selectPresets(in)
	if err != nil {
		return BulkResult{}, err
	}
	for _, p := range presets {
		if err := p.validate(); err != nil {
			return BulkResult{}, err
		}
	}

	// 持ち物なしの素の1通り。渡されたスライスは書き換えない。
	variants := in.ItemVariants
	if len(variants) == 0 {
		variants = []*Item{nil}
	}

	rows := make([]BulkRow, 0, len(presets)*len(variants))
	for _, p := range presets {
		label := p.Label
		if label == "" {
			label = string(p.Key)
		}
		for _, item := range variants {
			def := p.Defender(in.DefenderSpecies, item)
			res, err := CalcDamage(DamageInput{
				Format:   in.Format,
				Attacker: in.Attacker,
				Defender: def,
				Move:     in.Move,
				Field:    in.Field,
				Critical: in.Critical,
			})
			if err != nil {
				return BulkResult{}, fmt.Errorf("防御側プリセット %q の計算: %w", p.Key, err)
			}
			row := BulkRow{
				Preset:      p.Key,
				PresetLabel: label,
				Item:        item,
				Defender:    def,
				Result:      res,
			}
			if item != nil {
				row.ItemID = item.ID
			}
			rows = append(rows, row)
		}
	}
	return BulkResult{DefenderSpeciesKey: in.DefenderSpecies.Key, Rows: rows}, nil
}
