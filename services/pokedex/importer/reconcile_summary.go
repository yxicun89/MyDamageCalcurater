package importer

// 件数の要約(ADR-0103 §4)。集合の演算は raw なスナップショットから直接行い、
// Convert の取り込み判定とは独立に計算する(Imported だけ Output に依る)。

func countFindingsByKind(findings []Finding) map[FindingKind]int {
	counts := map[FindingKind]int{}
	for _, f := range findings {
		counts[f.Kind]++
	}
	return counts
}

func setSummary(calc, showdown, showdownStandard map[string]bool, imported int) SetSummary {
	both, calcOnly := 0, 0
	for id := range calc {
		if showdownStandard[id] {
			both++
		} else {
			calcOnly++
		}
	}
	showdownOnly := 0
	for id := range showdownStandard {
		if !calc[id] {
			showdownOnly++
		}
	}
	return SetSummary{
		Calc: len(calc), Showdown: len(showdown), ShowdownStandard: len(showdownStandard),
		Both: both, CalcOnly: calcOnly, ShowdownOnly: showdownOnly, Imported: imported,
	}
}

func computeSummary(in Input, out Output, partial bool) Summary {
	return Summary{
		Moves:     computeMoveSetSummary(in, out, partial),
		Species:   computeSpeciesSetSummary(in, out, partial),
		Items:     computeItemSetSummary(in, out, partial),
		Abilities: computeAbilitySetSummary(in, out, partial),
	}
}

func addFindingCounts(summary *Summary, warnings, blockers []Finding) {
	summary.WarningCounts = countFindingsByKind(warnings)
	summary.BlockerCounts = countFindingsByKind(blockers)
	if len(summary.WarningCounts) == 0 {
		summary.WarningCounts = nil
	}
	if len(summary.BlockerCounts) == 0 {
		summary.BlockerCounts = nil
	}
}

func computeMoveSetSummary(in Input, out Output, partial bool) SetSummary {
	calc := map[string]bool{}
	for _, m := range in.Calc.Moves {
		if m.Name == sentinelMoveName {
			continue
		}
		calc[toID(m.Name)] = true
	}
	showdown := map[string]bool{}
	standard := map[string]bool{}
	for _, m := range in.Showdown.Moves {
		showdown[m.ID] = true
		if m.IsNonstandard == nil {
			standard[m.ID] = true
		}
	}
	imported := 0
	if !partial {
		imported = len(out.Moves)
	}
	return setSummary(calc, showdown, standard, imported)
}

func computeSpeciesSetSummary(in Input, out Output, partial bool) SetSummary {
	matchKey := buildSpeciesMatchKey(in.Showdown.Species)
	calcCount, both, calcOnly := 0, 0, 0
	for _, s := range in.Calc.Species {
		calcCount++
		if _, ok := matchKey[toID(s.Name)]; ok {
			both++
		} else {
			calcOnly++
		}
	}
	showdown := map[string]bool{}
	standard := map[string]bool{}
	matchedStandard := map[string]bool{}
	for _, s := range in.Calc.Species {
		if id, ok := matchKey[toID(s.Name)]; ok {
			matchedStandard[id] = true
		}
	}
	for _, s := range in.Showdown.Species {
		showdown[s.ID] = true
		if s.IsNonstandard == nil {
			standard[s.ID] = true
		}
	}
	imported := 0
	if !partial {
		imported = len(out.Species)
	}
	showdownOnly := 0
	for id := range standard {
		if !matchedStandard[id] {
			showdownOnly++
		}
	}
	return SetSummary{Calc: calcCount, Showdown: len(showdown), ShowdownStandard: len(standard), Both: both, CalcOnly: calcOnly, ShowdownOnly: showdownOnly, Imported: imported}
}

func computeItemSetSummary(in Input, out Output, partial bool) SetSummary {
	calc := map[string]bool{}
	for _, n := range in.Calc.Items {
		calc[toID(n)] = true
	}
	showdown := map[string]bool{}
	standard := map[string]bool{}
	for _, it := range in.Showdown.Items {
		showdown[it.ID] = true
		if it.IsNonstandard == nil {
			standard[it.ID] = true
		}
	}
	imported := 0
	if !partial {
		imported = len(out.Items)
	}
	return setSummary(calc, showdown, standard, imported)
}

func computeAbilitySetSummary(in Input, out Output, partial bool) SetSummary {
	calc := map[string]bool{}
	for _, n := range in.Calc.Abilities {
		calc[toID(n)] = true
	}
	showdown := map[string]bool{}
	standard := map[string]bool{}
	for _, a := range in.Showdown.Abilities {
		showdown[a.ID] = true
		if a.IsNonstandard == nil {
			standard[a.ID] = true
		}
	}
	imported := 0
	if !partial {
		imported = len(out.Abilities)
	}
	return setSummary(calc, showdown, standard, imported)
}
