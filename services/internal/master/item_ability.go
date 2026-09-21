package master

import (
	"fmt"

	"example.com/pokecalc/engine"
)

// ItemRow は items + item_effects の行。Effect は item_effects に行が無ければ nil。
type ItemRow struct {
	ID     string
	NameJa string
	Effect []byte
}

// AbilityRow は abilities + ability_effects の行。Effect は ability_effects に行が無ければ nil。
type AbilityRow struct {
	ID     string
	NameJa string
	Effect []byte
}

// Item は持ち物の行を engine.Item に写像する。Effect が nil なら補正なし。
func Item(row ItemRow, chart engine.TypeChart) (engine.Item, error) {
	if !codeIDPattern.MatchString(row.ID) {
		return engine.Item{}, fmt.Errorf("%w: 持ち物 ID の形式が不正: %q", ErrInvalidRow, row.ID)
	}
	if row.NameJa == "" {
		return engine.Item{}, fmt.Errorf("%w: 日本語名が空", ErrInvalidRow)
	}
	var effect *engine.ItemEffect
	if row.Effect != nil {
		e, err := DecodeItemEffect(row.Effect, chart)
		if err != nil {
			return engine.Item{}, err
		}
		effect = e
	}
	return engine.Item{ID: row.ID, NameJa: row.NameJa, Effect: effect}, nil
}

// Ability は特性の行を engine.Ability に写像する。Effect が nil なら補正なし。
func Ability(row AbilityRow, chart engine.TypeChart) (engine.Ability, error) {
	if !codeIDPattern.MatchString(row.ID) {
		return engine.Ability{}, fmt.Errorf("%w: 特性 ID の形式が不正: %q", ErrInvalidRow, row.ID)
	}
	if row.NameJa == "" {
		return engine.Ability{}, fmt.Errorf("%w: 日本語名が空", ErrInvalidRow)
	}
	var effect *engine.AbilityEffect
	if row.Effect != nil {
		e, err := DecodeAbilityEffect(row.Effect, chart)
		if err != nil {
			return engine.Ability{}, err
		}
		effect = e
	}
	return engine.Ability{ID: row.ID, NameJa: row.NameJa, Effect: effect}, nil
}
