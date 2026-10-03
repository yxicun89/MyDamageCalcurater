// Package speedeffects builds judge's small ID -> speed-effect table from the master's ability and
// item effects and caches it (issue 235 第2段・ADR-0714 §1). It reads only the speed fields
// (SpeedMods and IgnoresParalysisSpeedDrop; ADR-0139) and keeps no species, moves or type chart.
package speedeffects

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/judge/internal/client"
	"example.com/pokecalc/services/judge/internal/judge"
)

// MaxSpeedModsPerEffect は 1 つの効果の SpeedMods の上限。巨大な配列で 1 件の評価が重くならないようにする。
// 語彙(engine.AllSpeedConditions)の件数より大きく、条件の重複を拒否するので正しいデータは超えない。
const MaxSpeedModsPerEffect = 16

const (
	keySpeedMods                 = "SpeedMods"
	keyIgnoresParalysisSpeedDrop = "IgnoresParalysisSpeedDrop"
	keyCondition                 = "Condition"
	keyModifier                  = "Modifier"
)

// DecodeAbilitySpeedEffect reads the speed fields of a master ability effect. null and an effect
// without speed fields mean "no speed effect"; a malformed speed field is an error (that one id
// becomes undeterminable).
func DecodeAbilitySpeedEffect(raw json.RawMessage) (judge.AbilitySpeedEffect, error) {
	fields, err := effectFields(raw)
	if err != nil {
		return judge.AbilitySpeedEffect{}, err
	}
	mods, err := decodeSpeedMods(fields)
	if err != nil {
		return judge.AbilitySpeedEffect{}, err
	}
	ignores, err := decodeIgnoresParalysis(fields)
	if err != nil {
		return judge.AbilitySpeedEffect{}, err
	}
	return judge.AbilitySpeedEffect{Mods: mods, IgnoresParalysisSpeedDrop: ignores}, nil
}

// DecodeItemSpeedEffect reads the speed fields of a master item effect. An item has no
// IgnoresParalysisSpeedDrop (ADR-0139), so that key is not read.
func DecodeItemSpeedEffect(raw json.RawMessage) (judge.ItemSpeedEffect, error) {
	fields, err := effectFields(raw)
	if err != nil {
		return judge.ItemSpeedEffect{}, err
	}
	mods, err := decodeSpeedMods(fields)
	if err != nil {
		return judge.ItemSpeedEffect{}, err
	}
	return judge.ItemSpeedEffect{Mods: mods}, nil
}

// effectFields splits an effect into its top-level members. The keys are matched exactly:
// encoding/json would quietly fold a differently-cased key into a known field.
func effectFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, errors.New("effect is not an object")
	}
	return fields, nil
}

func decodeSpeedMods(fields map[string]json.RawMessage) ([]engine.SpeedMod, error) {
	rawMods, present := fields[keySpeedMods]
	if !present {
		return nil, nil
	}
	var elems []json.RawMessage
	if string(rawMods) == "null" || json.Unmarshal(rawMods, &elems) != nil {
		return nil, errors.New("SpeedMods is not an array")
	}
	if len(elems) == 0 || len(elems) > MaxSpeedModsPerEffect {
		return nil, fmt.Errorf("SpeedMods must have 1..%d elements", MaxSpeedModsPerEffect)
	}
	mods := make([]engine.SpeedMod, 0, len(elems))
	seen := make(map[engine.SpeedCondition]bool, len(elems))
	for _, elem := range elems {
		mod, err := decodeSpeedMod(elem)
		if err != nil {
			return nil, err
		}
		if seen[mod.Condition] {
			return nil, errors.New("SpeedMods repeats a condition")
		}
		seen[mod.Condition] = true
		mods = append(mods, mod)
	}
	return mods, nil
}

func decodeSpeedMod(elem json.RawMessage) (engine.SpeedMod, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(elem, &members); err != nil || len(members) != 2 {
		return engine.SpeedMod{}, errors.New("SpeedMods element must have exactly Condition and Modifier")
	}
	rawCondition, okC := members[keyCondition]
	rawModifier, okM := members[keyModifier]
	if !okC || !okM {
		return engine.SpeedMod{}, errors.New("SpeedMods element must have exactly Condition and Modifier")
	}
	var condition string
	if err := json.Unmarshal(rawCondition, &condition); err != nil || condition == "" {
		return engine.SpeedMod{}, errors.New("Condition must be a non-empty string")
	}
	// Unknown conditions are accepted here and judged undeterminable at evaluation, so a newer
	// master vocabulary does not invalidate the other elements or ids.
	modifier, err := strconv.ParseInt(string(rawModifier), 10, 64)
	if err != nil {
		return engine.SpeedMod{}, errors.New("Modifier must be an integer")
	}
	if modifier < engine.MinEffectModifier || modifier > engine.MaxEffectModifier || modifier == engine.Modifier4096 {
		return engine.SpeedMod{}, errors.New("Modifier is out of range or neutral")
	}
	return engine.SpeedMod{Condition: engine.SpeedCondition(condition), Modifier: int(modifier)}, nil
}

func decodeIgnoresParalysis(fields map[string]json.RawMessage) (bool, error) {
	raw, present := fields[keyIgnoresParalysisSpeedDrop]
	if !present {
		return false, nil
	}
	switch string(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New("IgnoresParalysisSpeedDrop must be a boolean")
	}
}

// Table maps an ability or item id to its speed effect. An id whose effect has no speed fields is
// present with an empty effect ("known to have no effect"); an id whose speed fields are malformed,
// or that is not in the master, is absent ("cannot be determined"). The zero Table knows nothing.
// A Table is immutable once built, so concurrent requests share it.
type Table struct {
	abilities map[string]judge.AbilitySpeedEffect
	items     map[string]judge.ItemSpeedEffect
}

// BuildTable builds the table from the master's abilities and items.
func BuildTable(master client.MasterEffects) Table {
	t := Table{
		abilities: make(map[string]judge.AbilitySpeedEffect, len(master.Abilities)),
		items:     make(map[string]judge.ItemSpeedEffect, len(master.Items)),
	}
	for _, e := range master.Abilities {
		if effect, err := DecodeAbilitySpeedEffect(e.Effect); err == nil {
			t.abilities[e.ID] = effect
		}
	}
	for _, e := range master.Items {
		if effect, err := DecodeItemSpeedEffect(e.Effect); err == nil {
			t.items[e.ID] = effect
		}
	}
	return t
}

// Ability returns the speed effect of the ability id; ok is false when it cannot be determined.
// The returned Mods is a copy.
func (t Table) Ability(id string) (judge.AbilitySpeedEffect, bool) {
	e, ok := t.abilities[id]
	e.Mods = append([]engine.SpeedMod(nil), e.Mods...)
	return e, ok
}

// Item returns the speed effect of the item id; ok is false when it cannot be determined.
// The returned Mods is a copy.
func (t Table) Item(id string) (judge.ItemSpeedEffect, bool) {
	e, ok := t.items[id]
	e.Mods = append([]engine.SpeedMod(nil), e.Mods...)
	return e, ok
}
