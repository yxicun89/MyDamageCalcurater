package importer

// 特性・持ち物の名前付け、習得技、レギュレーション、効果定義、override 未使用検査
// (ADR-0101 §2・§5・§6・§7)。

import (
	"encoding/json"
	"fmt"
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// buildNamedRows は持ち物/特性の NamedRow を、日本語名の解決込みで作る。
func buildNamedRows(ids []string, nameEn map[string]string, pokeapiEntries []PokeAPIName, overrides map[string]string,
	languages []string, usedOverride map[string]bool) ([]NamedRow, []Finding) {
	lookup := newPokeAPILookup(pokeapiEntries)
	sortedIDs := append([]string(nil), ids...)
	sort.Strings(sortedIDs)

	var warnings []Finding
	rows := make([]NamedRow, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		res := resolveJaName(id, overrides, lookup[id], languages, nameEn[id], usedOverride)
		if res.Source == "fallback_en" {
			warnings = append(warnings, Finding{Kind: KindNameFallback, ID: id})
		}
		rows = append(rows, NamedRow{ID: id, NameJa: res.NameJa, NameJaSource: res.Source, NameEn: nameEn[id]})
	}
	return rows, warnings
}

// checkOverrideUnused はどの行にも当たらなかった override のキーを警告にする。
func checkOverrideUnused(overrides map[string]string, used map[string]bool) []Finding {
	var out []Finding
	for _, id := range sortedKeysRaw(overrides) {
		if !used[id] {
			out = append(out, Finding{Kind: KindOverrideUnused, ID: id})
		}
	}
	return out
}

// buildLearnsets は Showdown の learnsets を、取り込んだ種族・技だけに絞って行にする。
// 自分の習得技が無いフォーム(メガ・別フォーム)は baseSpecies の習得技を使う。
func buildLearnsets(learnsets map[string][]string, speciesRows []SpeciesRow, baseSpeciesShowdownID map[string]string, includedMoves map[string]bool) []LearnsetRow {
	var rows []LearnsetRow
	for _, sp := range speciesRows {
		list, ok := learnsets[sp.ShowdownID]
		if !ok {
			list = learnsets[baseSpeciesShowdownID[sp.ShowdownID]]
		}
		for _, moveID := range list {
			if includedMoves[moveID] {
				rows = append(rows, LearnsetRow{SpeciesKey: sp.Key, MoveID: moveID})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SpeciesKey != rows[j].SpeciesKey {
			return rows[i].SpeciesKey < rows[j].SpeciesKey
		}
		return rows[i].MoveID < rows[j].MoveID
	})
	return rows
}

// buildRegulations はレギュレーション定義から集合の行を作る(v1: 定義の showdownMod が
// スナップショットの mod と一致する前提で、取り込んだ全種族・技・持ち物・特性がその集合になる)。
func buildRegulations(regs RegulationsFile, showdownMod string, speciesKeys, moveIDs, itemIDs, abilityIDs []string) (
	[]RegulationRow, []RegulationMemberRow, []RegulationMemberRow, []RegulationMemberRow, []RegulationMemberRow, error) {

	defaults := 0
	for _, r := range regs.Regulations {
		if r.ShowdownMod != showdownMod {
			return nil, nil, nil, nil, nil, fmt.Errorf("%w: レギュレーション %q の showdownMod %q がスナップショットの mod %q と違う",
				ErrInvalidData, r.ID, r.ShowdownMod, showdownMod)
		}
		if r.IsDefault {
			defaults++
		}
	}
	if defaults > 1 {
		return nil, nil, nil, nil, nil, fmt.Errorf("%w: 既定のレギュレーションが %d 件ある", ErrInvalidData, defaults)
	}

	var rows []RegulationRow
	var sp, mv, it, ab []RegulationMemberRow
	for _, r := range regs.Regulations {
		rows = append(rows, RegulationRow{ID: r.ID, NameJa: r.NameJa, IsDefault: r.IsDefault, StartsOn: r.StartsOn, EndsOn: r.EndsOn})
		for _, k := range speciesKeys {
			sp = append(sp, RegulationMemberRow{RegulationID: r.ID, MemberID: k})
		}
		for _, k := range moveIDs {
			mv = append(mv, RegulationMemberRow{RegulationID: r.ID, MemberID: k})
		}
		for _, k := range itemIDs {
			it = append(it, RegulationMemberRow{RegulationID: r.ID, MemberID: k})
		}
		for _, k := range abilityIDs {
			ab = append(ab, RegulationMemberRow{RegulationID: r.ID, MemberID: k})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	sortMembers := func(m []RegulationMemberRow) {
		sort.Slice(m, func(i, j int) bool {
			if m[i].RegulationID != m[j].RegulationID {
				return m[i].RegulationID < m[j].RegulationID
			}
			return m[i].MemberID < m[j].MemberID
		})
	}
	sortMembers(sp)
	sortMembers(mv)
	sortMembers(it)
	sortMembers(ab)
	return rows, sp, mv, it, ab, nil
}

// buildItemEffects / buildAbilityEffects は effects.json の定義を検証・正準化する
// (取り込む持ち物/特性に無い ID は投入せず effect-unused の警告にする)。
func buildItemEffects(raw map[string]json.RawMessage, included map[string]bool, chart engine.TypeChart) ([]EffectRow, []Finding, error) {
	var rows []EffectRow
	var warnings []Finding
	for _, id := range sortedKeysRaw(raw) {
		if !included[id] {
			warnings = append(warnings, Finding{Kind: KindEffectUnused, ID: id})
			continue
		}
		eff, err := master.DecodeItemEffect(raw[id], chart)
		if err != nil {
			return nil, nil, err
		}
		canon, err := master.EncodeItemEffect(*eff)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, EffectRow{ID: id, Effect: canon})
	}
	return rows, warnings, nil
}

func buildAbilityEffects(raw map[string]json.RawMessage, included map[string]bool, chart engine.TypeChart) ([]EffectRow, []Finding, error) {
	var rows []EffectRow
	var warnings []Finding
	for _, id := range sortedKeysRaw(raw) {
		if !included[id] {
			warnings = append(warnings, Finding{Kind: KindEffectUnused, ID: id})
			continue
		}
		eff, err := master.DecodeAbilityEffect(raw[id], chart)
		if err != nil {
			return nil, nil, err
		}
		canon, err := master.EncodeAbilityEffect(*eff)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, EffectRow{ID: id, Effect: canon})
	}
	return rows, warnings, nil
}
