package master

import (
	"fmt"

	"example.com/pokecalc/engine"
)

// MoveRow は moves テーブルの行(engine が使わない accuracy・pp・name_en は含まない)。
// Effect は move_effects に行が無ければ nil(空スライスも「効果なし」として扱う)。
// Mechanisms は move_mechanisms の機構(ADR-0121)。行が無ければ空(通常の技)。
type MoveRow struct {
	ID       string
	NameJa   string
	Type     string
	Category string
	Power    int
	Priority int
	Effect   []byte
	// Mechanisms は Move で検証するが、engine.Move にはまだ載せない(未対応の印は D16)。
	Mechanisms []string
}

// moveCategories は moves.category として許される値(ADR-0100 §3)。
var moveCategories = map[string]engine.MoveCategory{
	"physical": engine.CategoryPhysical,
	"special":  engine.CategorySpecial,
	"status":   engine.CategoryStatus,
}

// Move は技の行を engine.Move に写像する。
func Move(row MoveRow, chart engine.TypeChart) (engine.Move, error) {
	if chart.IsZero() {
		return engine.Move{}, fmt.Errorf("%w: タイプ相性表が未設定", ErrInvalidRow)
	}
	if !codeIDPattern.MatchString(row.ID) {
		return engine.Move{}, fmt.Errorf("%w: 技 ID の形式が不正: %q", ErrInvalidRow, row.ID)
	}
	if row.NameJa == "" {
		return engine.Move{}, fmt.Errorf("%w: 日本語名が空", ErrInvalidRow)
	}
	if row.Type == "" || !chart.Has(engine.Type(row.Type)) {
		return engine.Move{}, fmt.Errorf("%w: タイプが不正: %q", ErrInvalidRow, row.Type)
	}
	category, ok := moveCategories[row.Category]
	if !ok {
		return engine.Move{}, fmt.Errorf("%w: 分類が不正: %q", ErrInvalidRow, row.Category)
	}
	if row.Power < 0 || row.Power > 999 {
		return engine.Move{}, fmt.Errorf("%w: 威力が範囲外(0..999): %d", ErrInvalidRow, row.Power)
	}
	if category == engine.CategoryStatus && row.Power != 0 {
		return engine.Move{}, fmt.Errorf("%w: 変化技なのに威力がある: %d", ErrInvalidRow, row.Power)
	}
	if row.Priority < -7 || row.Priority > 5 {
		return engine.Move{}, fmt.Errorf("%w: 優先度が範囲外(-7..5): %d", ErrInvalidRow, row.Priority)
	}
	if _, err := MoveMechanismsOf(row); err != nil {
		return engine.Move{}, err
	}
	var effect *engine.MoveEffect
	if len(row.Effect) > 0 {
		e, err := DecodeMoveEffect(row.Effect)
		if err != nil {
			return engine.Move{}, err
		}
		effect = e
	}
	return engine.Move{
		ID:       row.ID,
		NameJa:   row.NameJa,
		Type:     engine.Type(row.Type),
		Category: category,
		Power:    row.Power,
		Priority: row.Priority,
		Effect:   effect,
	}, nil
}
