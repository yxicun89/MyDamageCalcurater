package master

import (
	"fmt"
	"sort"
	"strings"

	"example.com/pokecalc/engine"
)

// SpeciesRow は species テーブルの行。
type SpeciesRow struct {
	Key        string
	DexNo      int
	Form       int
	ShowdownID string
	NameJa     string
	NameEn     string
	Type1      string // "" は不正(NOT NULL)
	Type2      string // "" は NULL(タイプなし)
	BaseHP     int
	BaseAtk    int
	BaseDef    int
	BaseSpA    int
	BaseSpD    int
	BaseSpe    int
	IsMega     bool
	// BaseSpeciesKey / RequiredItemID は "" が NULL を表す。
	BaseSpeciesKey string
	RequiredItemID string
}

// MegaNamePrefix はメガ種族の日本語名の先頭に付ける語(メガルカリオ)。importer が名前を生成するとき
// (MegaNameJa)と、種族検索が「メガを除いた基本種名でも当たる」規則(ADR-0324)で使う。名前の表ではなく
// 命名規則の1語(CLAUDE.md の「リストをハードコードしない」の対象外)。
const MegaNamePrefix = "メガ"

// megaFormePrefix は Showdown のメガのフォーム名の先頭("Mega"、"Mega-X"、"Mega-Y")。
const megaFormePrefix = "Mega"

// MegaNameJa は基本種の日本語名とメガのフォーム名から、メガ種族の日本語名を機械的に作る
// (メガ + 基本種名 + フォーム識別子)。フォーム名が "Mega" だけなら識別子なし(メガルカリオ)、
// "Mega-X" なら末尾に X(メガリザードンX)。基本種名が空、またはフォーム名がその形でないときは "" を返す(生成しない)。
func MegaNameJa(baseNameJa, forme string) string {
	if strings.TrimSpace(baseNameJa) == "" {
		return ""
	}
	// 規則に合うのは "Mega" か "Mega-<識別子>" だけ。M-Mega・地方の姿などは推測しない(ADR-0140 §3)。
	suffix := ""
	if forme != megaFormePrefix {
		rest, ok := strings.CutPrefix(forme, megaFormePrefix+"-")
		if !ok || rest == "" {
			return ""
		}
		suffix = rest
	}
	return MegaNamePrefix + baseNameJa + suffix
}

// SpeciesAbilityRow は species_abilities テーブルの行。
type SpeciesAbilityRow struct {
	Slot      int
	AbilityID string
}

// Species は種族の行(+特性の行)を engine.Species に写像する。
// chart がゼロ値(未設定)のときは黙ってタイプを通さず ErrInvalidRow。
func Species(row SpeciesRow, abilities []SpeciesAbilityRow, chart engine.TypeChart) (engine.Species, error) {
	if chart.IsZero() {
		return engine.Species{}, fmt.Errorf("%w: タイプ相性表が未設定", ErrInvalidRow)
	}
	if expected := fmt.Sprintf("%04d-%03d", row.DexNo, row.Form); row.Key != expected {
		return engine.Species{}, fmt.Errorf("%w: key %q が dex_no/form から期待される %q と一致しない", ErrInvalidRow, row.Key, expected)
	}
	// key の等価性チェックだけでは、DexNo/Form 自体が範囲外でも key を作り直して
	// 通り抜けられてしまう(例 DexNo=0, Key="0000-000")ため、範囲は別に検証する。
	if row.DexNo < 1 || row.DexNo > 9999 {
		return engine.Species{}, fmt.Errorf("%w: dex_no が範囲外(1..9999): %d", ErrInvalidRow, row.DexNo)
	}
	if row.Form < 0 || row.Form > 999 {
		return engine.Species{}, fmt.Errorf("%w: form が範囲外(0..999): %d", ErrInvalidRow, row.Form)
	}
	if row.NameJa == "" {
		return engine.Species{}, fmt.Errorf("%w: 日本語名が空", ErrInvalidRow)
	}
	if !codeIDPattern.MatchString(row.ShowdownID) {
		return engine.Species{}, fmt.Errorf("%w: showdown_id の形式が不正: %q", ErrInvalidRow, row.ShowdownID)
	}
	if row.BaseSpeciesKey != "" && !speciesKeyPattern.MatchString(row.BaseSpeciesKey) {
		return engine.Species{}, fmt.Errorf("%w: base_species_key の形式が不正: %q", ErrInvalidRow, row.BaseSpeciesKey)
	}
	if row.RequiredItemID != "" && !codeIDPattern.MatchString(row.RequiredItemID) {
		return engine.Species{}, fmt.Errorf("%w: required_item_id の形式が不正: %q", ErrInvalidRow, row.RequiredItemID)
	}

	types, err := speciesTypes(row, chart)
	if err != nil {
		return engine.Species{}, err
	}

	stats := engine.Stats{HP: row.BaseHP, Atk: row.BaseAtk, Def: row.BaseDef, SpA: row.BaseSpA, SpD: row.BaseSpD, Spe: row.BaseSpe}
	for _, k := range engine.AllStatKeys() {
		if v := stats.Get(k); v < 1 || v > 255 {
			return engine.Species{}, fmt.Errorf("%w: 種族値 %s が範囲外(1..255): %d", ErrInvalidRow, k, v)
		}
	}

	if err := validateMegaConsistency(row); err != nil {
		return engine.Species{}, err
	}

	abilityIDs, err := speciesAbilities(abilities)
	if err != nil {
		return engine.Species{}, err
	}

	return engine.Species{
		Key:       row.Key,
		DexNo:     row.DexNo,
		Form:      row.Form,
		NameJa:    row.NameJa,
		Types:     types,
		BaseStats: stats,
		Abilities: abilityIDs,
	}, nil
}

// speciesTypes はタイプ1・タイプ2を検証し、engine.Species.Types(1〜2個)を返す。
func speciesTypes(row SpeciesRow, chart engine.TypeChart) ([]engine.Type, error) {
	if row.Type1 == "" || !chart.Has(engine.Type(row.Type1)) {
		return nil, fmt.Errorf("%w: type1 が不正: %q", ErrInvalidRow, row.Type1)
	}
	types := []engine.Type{engine.Type(row.Type1)}
	if row.Type2 == "" {
		return types, nil
	}
	if row.Type2 == row.Type1 {
		return nil, fmt.Errorf("%w: type2 が type1 と同じ", ErrInvalidRow)
	}
	if !chart.Has(engine.Type(row.Type2)) {
		return nil, fmt.Errorf("%w: type2 が不正: %q", ErrInvalidRow, row.Type2)
	}
	return append(types, engine.Type(row.Type2)), nil
}

// validateMegaConsistency はメガの3列の整合(ADR-0100 §3)を検証する。
func validateMegaConsistency(row SpeciesRow) error {
	hasBaseKey := row.BaseSpeciesKey != ""
	hasItem := row.RequiredItemID != ""
	switch {
	case row.IsMega && (!hasBaseKey || !hasItem):
		return fmt.Errorf("%w: メガなのに base_species_key/required_item_id が揃っていない", ErrInvalidRow)
	case !row.IsMega && (hasBaseKey || hasItem):
		return fmt.Errorf("%w: メガでないのに base_species_key/required_item_id がある", ErrInvalidRow)
	case row.IsMega && row.BaseSpeciesKey == row.Key:
		return fmt.Errorf("%w: base_species_key が自分自身: %q", ErrInvalidRow, row.Key)
	}
	return nil
}

// speciesAbilities は特性の行を検証し、slot 順の特性 ID 列を返す(1〜4件、slot は 1..4・重複不可、
// 特性 ID の重複も不可)。
func speciesAbilities(abilities []SpeciesAbilityRow) ([]string, error) {
	if len(abilities) == 0 {
		return nil, fmt.Errorf("%w: 特性が無い", ErrInvalidRow)
	}
	sorted := append([]SpeciesAbilityRow(nil), abilities...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slot < sorted[j].Slot })

	seenSlot := map[int]bool{}
	seenID := map[string]bool{}
	out := make([]string, 0, len(sorted))
	for _, a := range sorted {
		if a.Slot < 1 || a.Slot > 4 {
			return nil, fmt.Errorf("%w: 特性スロットが範囲外(1..4): %d", ErrInvalidRow, a.Slot)
		}
		if seenSlot[a.Slot] {
			return nil, fmt.Errorf("%w: 特性スロットが重複: %d", ErrInvalidRow, a.Slot)
		}
		seenSlot[a.Slot] = true
		if !codeIDPattern.MatchString(a.AbilityID) {
			return nil, fmt.Errorf("%w: 特性 ID の形式が不正: %q", ErrInvalidRow, a.AbilityID)
		}
		if seenID[a.AbilityID] {
			return nil, fmt.Errorf("%w: 同じ特性が複数スロットにある: %q", ErrInvalidRow, a.AbilityID)
		}
		seenID[a.AbilityID] = true
		out = append(out, a.AbilityID)
	}
	return out, nil
}
