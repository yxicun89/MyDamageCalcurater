package importer

import "sort"

// computeEffectCoverage は取り込んだ持ち物・特性を、config.json で選んだダメージに効くハンドラと
// 効果定義(effects.json)に照らして網羅性を計算する(ADR-0103 §6)。
func computeEffectCoverage(rc *ReconcileConfig, in Input, out Output) (EffectCoverage, []Finding) {
	effectHooks := make(map[string]bool, len(rc.EffectHooks))
	for _, hook := range rc.EffectHooks {
		effectHooks[hook] = true
	}

	itemHooks := make(map[string][]string, len(in.Showdown.Items))
	for _, item := range in.Showdown.Items {
		itemHooks[item.ID] = item.Hooks
	}
	abilityHooks := make(map[string][]string, len(in.Showdown.Abilities))
	for _, ability := range in.Showdown.Abilities {
		abilityHooks[ability.ID] = ability.Hooks
	}

	itemIDs := make([]string, 0, len(out.Items))
	for _, item := range out.Items {
		itemIDs = append(itemIDs, item.ID)
	}
	abilityIDs := make([]string, 0, len(out.Abilities))
	for _, ability := range out.Abilities {
		abilityIDs = append(abilityIDs, ability.ID)
	}

	items, itemWarnings := coverageStats(itemIDs, itemHooks, effectHooks, in.Effects.Items)
	abilities, abilityWarnings := coverageStats(abilityIDs, abilityHooks, effectHooks, in.Effects.Abilities)
	return EffectCoverage{Items: items, Abilities: abilities}, append(itemWarnings, abilityWarnings...)
}

func coverageStats[T any](ids []string, hooks map[string][]string, effectHooks map[string]bool, definitions map[string]T) (CoverageStats, []Finding) {
	sortedIDs := append([]string(nil), ids...)
	sort.Strings(sortedIDs)
	stats := CoverageStats{Imported: len(sortedIDs)}
	var warnings []Finding
	for _, id := range sortedIDs {
		hasDamageHook := false
		for _, hook := range hooks[id] {
			if effectHooks[hook] {
				hasDamageHook = true
				break
			}
		}
		_, defined := definitions[id]
		if hasDamageHook {
			stats.WithDamageHooks++
		}
		if defined {
			stats.Defined++
		}
		switch {
		case hasDamageHook && !defined:
			stats.Missing++
			stats.MissingIDs = append(stats.MissingIDs, id)
			warnings = append(warnings, Finding{Kind: KindEffectMissing, ID: id})
		case defined && !hasDamageHook:
			stats.NoHook++
			stats.NoHookIDs = append(stats.NoHookIDs, id)
			warnings = append(warnings, Finding{Kind: KindEffectNoHook, ID: id})
		}
	}
	return stats, warnings
}

func computeNames(in Input, out Output) map[string]NameStats {
	stats := map[string]NameStats{}
	stats["species"] = nameStatsForSpecies(in, out.Species)
	stats["moves"] = nameStatsForRows(in.Config.NameJaLanguages, in.PokeAPI.Moves, out.Moves, func(r MoveRow) (string, string) { return r.ID, r.NameJaSource })
	stats["items"] = nameStatsForRows(in.Config.NameJaLanguages, in.PokeAPI.Items, out.Items, func(r NamedRow) (string, string) { return r.ID, r.NameJaSource })
	stats["abilities"] = nameStatsForRows(in.Config.NameJaLanguages, in.PokeAPI.Abilities, out.Abilities, func(r NamedRow) (string, string) { return r.ID, r.NameJaSource })
	stats["types"] = nameStatsForRows(in.Config.NameJaLanguages, in.PokeAPI.Types, out.Types, func(r TypeRow) (string, string) { return r.ID, r.NameJaSource })
	return stats
}

func nameStatsForRows[T any](languages []string, entries []PokeAPIName, rows []T, fields func(T) (string, string)) NameStats {
	lookup := newPokeAPILookup(entries)
	stats := NameStats{ByLanguage: map[string]int{}}
	for _, row := range rows {
		id, source := fields(row)
		addNameStat(&stats, id, source, lookup[id], languages)
	}
	sort.Strings(stats.FallbackIDs)
	return stats
}

func nameStatsForSpecies(in Input, rows []SpeciesRow) NameStats {
	species := newPokeAPILookup(in.PokeAPI.Species)
	forms := newPokeAPILookup(in.PokeAPI.Forms)
	stats := NameStats{ByLanguage: map[string]int{}}
	for _, row := range rows {
		lookup := species[row.ShowdownID]
		if row.Form != 0 {
			lookup = forms[row.ShowdownID]
		}
		addNameStat(&stats, row.ShowdownID, row.NameJaSource, lookup, in.Config.NameJaLanguages)
	}
	sort.Strings(stats.FallbackIDs)
	return stats
}

func addNameStat(stats *NameStats, id, source string, names map[string]string, languages []string) {
	switch source {
	case "override":
		stats.Override++
	case "pokeapi":
		stats.PokeAPI++
		for _, language := range languages {
			if names[language] != "" {
				stats.ByLanguage[language]++
				break
			}
		}
	case "fallback_en":
		stats.FallbackEn++
		stats.FallbackIDs = append(stats.FallbackIDs, id)
	}
}

func computeShowdownOnlySpecies(in Input, out Output, report Report) []ShowdownOnlySpecies {
	imported := map[string]bool{}
	for _, species := range out.Species {
		imported[species.ShowdownID] = true
	}
	matchedByCalc := map[string]bool{}
	matchKey := buildSpeciesMatchKey(in.Showdown.Species)
	calcNums := map[int]bool{}
	sdByID := map[string]ShowdownSpecies{}
	for _, species := range in.Showdown.Species {
		sdByID[species.ID] = species
	}
	for _, species := range in.Calc.Species {
		if id, ok := matchKey[toID(species.Name)]; ok {
			matchedByCalc[id] = true
			calcNums[sdByID[id].Num] = true
		}
	}
	folded := map[string]string{}
	different := map[string]bool{}
	for _, finding := range report.Warnings {
		switch finding.Kind {
		case KindFormFolded:
			folded[finding.ID] = finding.Detail
		case KindSpeciesShowdownOnly:
			different[finding.ID] = true
		}
	}

	var result []ShowdownOnlySpecies
	for _, species := range in.Showdown.Species {
		if species.IsNonstandard != nil || imported[species.ID] || matchedByCalc[species.ID] {
			continue
		}
		entry := ShowdownOnlySpecies{ID: species.ID, Num: species.Num}
		switch {
		case folded[species.ID] != "":
			entry.Reason = "folded"
			entry.Representative = folded[species.ID]
		case different[species.ID]:
			entry.Reason = "different-performance"
		case !calcNums[species.Num]:
			entry.Reason = "num-not-in-calc"
		default:
			entry.Reason = "unresolvable"
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func computeFoldedLearnsets(in Input, _ Output, report Report, includedMoves map[string]bool, rules learnsetRules) ([]FoldedLearnsetDiff, []Finding) {
	resolver := newLearnsetResolver(in.Showdown.Species, in.Showdown.Learnsets, rules)
	var result []FoldedLearnsetDiff
	var warnings []Finding
	for _, finding := range report.Warnings {
		if finding.Kind != KindFormFolded {
			continue
		}
		folded, err := resolver.resolve(finding.ID)
		if err != nil {
			continue // Convert が全種族を検証済みなので、ここでは起きない防御的な分岐。
		}
		representative, err := resolver.resolve(finding.Detail)
		if err != nil {
			continue
		}
		onlyFolded := setDifference(folded, representative, includedMoves)
		onlyRepresentative := setDifference(representative, folded, includedMoves)
		if len(onlyFolded) == 0 && len(onlyRepresentative) == 0 {
			continue
		}
		result = append(result, FoldedLearnsetDiff{
			ID: finding.ID, Representative: finding.Detail,
			OnlyInFolded: onlyFolded, OnlyInRepresentative: onlyRepresentative,
		})
		warnings = append(warnings, Finding{Kind: KindFormLearnsetDiff, ID: finding.ID, Detail: finding.Detail})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, warnings
}

func setDifference(left, right, included map[string]bool) []string {
	var result []string
	for id := range left {
		if included[id] && !right[id] {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func computeLearnsetStats(in Input, out Output, rules learnsetRules) LearnsetStats {
	included := map[string]bool{}
	for _, move := range out.Moves {
		included[move.ID] = true
	}
	_, bySpecies, err := buildLearnsets(in.Showdown.Species, in.Showdown.Learnsets, out.Species, included, rules)
	if err != nil {
		return LearnsetStats{}
	}
	total := 0
	for _, count := range bySpecies {
		total += count
	}
	if total == 0 {
		bySpecies = nil
	}
	return LearnsetStats{Inherited: total, InheritedBySpecies: bySpecies}
}
