package master

import (
	"fmt"
	"sort"

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

// validateMegaConsistency はメガの3列の整合(ADR-0015 §3)を検証する。
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

// speciesAbilities は特性の行を検証し、slot 順の特性 ID 列を返す(1〜3件、slot は 1..3・重複不可、
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
		if a.Slot < 1 || a.Slot > 3 {
			return nil, fmt.Errorf("%w: 特性スロットが範囲外(1..3): %d", ErrInvalidRow, a.Slot)
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
