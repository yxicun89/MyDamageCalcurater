package engine

// 一括計算(P1-7)。防御側の代表調整(プリセット)× 持ち物バリアントの全行を一度に返す。
//
// 要件(docs/requirements.md「相手側の一括表示」): 相手側は入力させず、
// 無振り / H振り / H振り+B(D)補正 / HB(HD)振り / HB(HD)特化 を並べて表示する。
//
// 設計は ADR-0009:
//   - プリセット定義は引数(データ)として受け取る。既定値は presets/defender.json を embed して持ち
//     (2026-09-25 追記)、WASM と calc-svc の双方が同じ既定で動く。呼び出し側は Presets で上書きできる。
//   - engine に持ち物一覧は持ち込まない。解決済みの *Item を受け取るだけ(ADR-0005)。
//   - CalcBulk は CalcDamage の合成にすぎない。独自のダメージ計算をしてはならない。

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
)

// プリセット定義・選択の不正。呼び出し側は errors.Is で判別する。
var (
	// ErrUnknownPreset は PresetKeys に定義の無いキーが含まれていた。
	ErrUnknownPreset = errors.New("未知の防御側プリセット")
	// ErrDuplicatePreset はプリセットのキーが重複している(行が一意にならない)。
	ErrDuplicatePreset = errors.New("防御側プリセットが重複している")
	// ErrInvalidPreset はプリセット定義そのものが不正(SP 範囲・合計、性格、キー空)。
	ErrInvalidPreset = errors.New("防御側プリセットの定義が不正")
	// ErrTooManyPresets は Presets / PresetKeys の件数が MaxBulkPresets を超えている
	// (issue #110。ADR-0208 §4・ADR-0108)。HTTP を経由しない直接呼び出し・WASM でも
	// 行数(len(presets) × len(itemVariants))を増幅させないための防御。
	ErrTooManyPresets = errors.New("防御側プリセットの件数が上限を超えている")
	// ErrTooManyItemVariants は ItemVariants の件数が MaxBulkItemVariants を超えている
	// (issue #110。ADR-0208 §4・ADR-0108)。
	ErrTooManyItemVariants = errors.New("持ち物バリアントの件数が上限を超えている")
)

// 件数の上限(issue #110。ADR-0208 §1 の契約値と同じ。ADR-0108)。
const (
	// MaxBulkPresets は Presets / PresetKeys それぞれの件数上限。
	// DefenderPreset の enum は8値なので「全種類を1回ずつ」が上限になる。
	MaxBulkPresets = 8
	// MaxBulkItemVariants は ItemVariants の件数上限。
	MaxBulkItemVariants = 64
)

// PresetKey は防御側の代表調整のキー。OpenAPI の DefenderPreset enum と1対1に対応する。
// カタログ(値・順序)の正は presets/defender.json。定数は呼び出し側・テストが名指しするために残す
// (JSON との一致は TestDefenderPresetJSONKeysMatchConstants が確かめる)。
type PresetKey string

// ADR-0009 §1 のカタログ順(耐久が上がる順)に並べる。
const (
	PresetNone    PresetKey = "none"     // 無振り
	PresetHP      PresetKey = "hp"       // H振り
	PresetHBBoost PresetKey = "hb_boost" // H振り + B補正(H32 / B0 / B上昇性格)
	PresetHB      PresetKey = "hb"       // HB振り(H32 / B32 / 補正なし)
	PresetHBFull  PresetKey = "hb_full"  // HB特化(H32 / B32 / B上昇性格)
	PresetHDBoost PresetKey = "hd_boost" // H振り + D補正(H32 / D0 / D上昇性格)
	PresetHD      PresetKey = "hd"       // HD振り(H32 / D32 / 補正なし)
	PresetHDFull  PresetKey = "hd_full"  // HD特化(H32 / D32 / D上昇性格)
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
	// TypeChart はタイプ相性表(ADR-0013)。CalcBulk は解釈せず DamageInput へ素通しする。
	TypeChart TypeChart

	// Presets は使用するプリセット定義。空のときは既定カタログ / 既定セットを使う(ADR-0009)。
	Presets []DefenderPreset
	// PresetKeys は Presets(空なら DefenderPresetCatalog())からキーで選ぶ。
	// 指定した順序がそのまま行の順序になる。空のときは選別しない。
	PresetKeys []PresetKey
	// ItemVariants は差し替えて比較する持ち物(解決済み)。nil / 空は「素の1通り」= 持ち物なし。
	ItemVariants []*Item
	// DefenderAbilities は防御側の特性の候補(解決済み。issue #272・ADR-0126)。0..MaxAbilityCandidates 件。
	// nil / 空は「特性なし」の1通り(従来どおり)。結果が同じになる特性は1行にまとめ、違えば行を分ける。
	// DefenderSpecies.Abilities が空でなければ、その中の ID だけを受け付ける。
	DefenderAbilities []Ability
}

// BulkRow は一括計算の1行(プリセット × 持ち物)。
type BulkRow struct {
	Preset      PresetKey
	PresetLabel string
	Item        *Item  // 渡された *Item をそのまま保持する(nil は持ち物なし)
	ItemID      string // Item.ID。Item が nil なら空文字
	// Ability はこの行の計算に使った防御側の特性(Defender.Ability と同じ。特性なしはゼロ値)。
	Ability Ability
	// AbilityIDs はこの行と結果が完全に同じになる特性の ID(Ability.ID が先頭。渡した順)。
	// DefenderAbilities を渡さなかったときは nil。
	AbilityIDs []string
	Defender   Individual
	Result     DamageResult
}

// BulkResult は一括計算の結果。Rows の順序はプリセット → 特性(グループの代表の渡した順)→ 持ち物で決定的。
type BulkResult struct {
	DefenderSpeciesKey string
	Rows               []BulkRow
}

// DefenderPresetCatalog は既定のプリセット定義を耐久が上がる順に返す(ADR-0009 §1)。
// 正は presets/defender.json(defender_preset.go が embed して読む。ADR-0009 2026-09-25 追記)。
// 呼び出しごとに新しいスライスを返し、呼び出し側の変更が次回に漏れないようにする。
//
// 期待値は engine/bulk_test.go の TestDefenderPresetCatalogDefinitions が正。
func DefenderPresetCatalog() []DefenderPreset {
	return slices.Clone(defenderPresets)
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
// Level=50、Status=none、Ranks=0、Ability=ゼロ値に固定する(ADR-0009)。特性は CalcBulk が
// BulkInput.DefenderAbilities から載せる(ADR-0126)。
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
	for _, k := range AllStatKeys() {
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
	// 件数の上限は、プリセットの選別・検証(selectPresets)より前に見る(issue #110。
	// ADR-0208 §4・ADR-0108)。巨大な入力に対して以降の一切の追加の仕事をしないため。
	if len(in.Presets) > MaxBulkPresets {
		return BulkResult{}, fmt.Errorf("%w: %d 件", ErrTooManyPresets, len(in.Presets))
	}
	if len(in.PresetKeys) > MaxBulkPresets {
		return BulkResult{}, fmt.Errorf("%w: %d 件", ErrTooManyPresets, len(in.PresetKeys))
	}
	if len(in.ItemVariants) > MaxBulkItemVariants {
		return BulkResult{}, fmt.Errorf("%w: %d 件", ErrTooManyItemVariants, len(in.ItemVariants))
	}

	presets, err := selectPresets(in)
	if err != nil {
		return BulkResult{}, err
	}
	for _, p := range presets {
		if err := p.validate(); err != nil {
			return BulkResult{}, err
		}
	}

	abilities, err := abilityCandidates("DefenderAbilities", in.DefenderSpecies, in.DefenderAbilities)
	if err != nil {
		return BulkResult{}, err
	}

	// 持ち物なしの素の1通り。渡されたスライスは書き換えない。
	variants := in.ItemVariants
	if len(variants) == 0 {
		variants = []*Item{nil}
	}

	// results[a][p][v] は特性 a・プリセット p・持ち物 v の CalcDamage。特性のまとめは全行の一致で決める。
	results := make([][][]DamageResult, len(abilities))
	for a, ability := range abilities {
		results[a] = make([][]DamageResult, len(presets))
		for pi, p := range presets {
			results[a][pi] = make([]DamageResult, len(variants))
			for vi, item := range variants {
				def := p.Defender(in.DefenderSpecies, item)
				def.Ability = ability
				res, err := CalcDamage(DamageInput{
					Format:    in.Format,
					Attacker:  in.Attacker,
					Defender:  def,
					Move:      in.Move,
					Field:     in.Field,
					Critical:  in.Critical,
					TypeChart: in.TypeChart,
				})
				if err != nil {
					return BulkResult{}, fmt.Errorf("防御側プリセット %q・特性 %q の計算: %w", p.Key, ability.ID, err)
				}
				results[a][pi][vi] = res
			}
		}
	}
	groups := groupAbilities(len(abilities), func(i, j int) bool {
		return reflect.DeepEqual(results[i], results[j])
	})

	rows := make([]BulkRow, 0, len(presets)*len(groups)*len(variants))
	for pi, p := range presets {
		label := p.Label
		if label == "" {
			label = string(p.Key)
		}
		for _, group := range groups {
			ability := abilities[group[0]]
			ids := groupAbilityIDs(abilities, group)
			for vi, item := range variants {
				def := p.Defender(in.DefenderSpecies, item)
				def.Ability = ability
				row := BulkRow{
					Preset:      p.Key,
					PresetLabel: label,
					Item:        item,
					Ability:     ability,
					AbilityIDs:  slices.Clone(ids),
					Defender:    def,
					Result:      results[group[0]][pi][vi],
				}
				if item != nil {
					row.ItemID = item.ID
				}
				rows = append(rows, row)
			}
		}
	}
	return BulkResult{DefenderSpeciesKey: in.DefenderSpecies.Key, Rows: rows}, nil
}
