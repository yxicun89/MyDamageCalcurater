package importer

// markMegaStones は持ち物の行の IsMegaStone を決める(ADR-0140)。真 ⇔ Showdown の megaStone が空でない
// または取り込んだメガ種族の required_item_id に現れる。後者だけ(megaStone が空)なら食い違いの警告を返す。
func markMegaStones(rows []NamedRow, sdItems []ShowdownItem, species []SpeciesRow) []Finding {
	fromShowdown := map[string]bool{}
	for _, it := range sdItems {
		if len(it.MegaStone) > 0 {
			fromShowdown[it.ID] = true
		}
	}
	required := map[string]bool{}
	for _, sp := range species {
		if sp.IsMega && sp.RequiredItemID != "" {
			required[sp.RequiredItemID] = true
		}
	}
	var warnings []Finding
	for i := range rows {
		id := rows[i].ID
		rows[i].IsMegaStone = fromShowdown[id] || required[id]
		if required[id] && !fromShowdown[id] {
			warnings = append(warnings, Finding{Kind: KindItemMegaStoneMismatch, ID: id})
		}
	}
	return warnings
}

// 持ち物の集合(ADR-0101 §5 最終段落: 技と同じ規則)。
// 両方にあり Showdown で使用可(isNonstandard が null)→ 取り込む(値の食い違いは無いので警告なし)。
// Showdown だけで使用可 → 取り込むが item-showdown-only の警告。
// calc だけ、または Showdown で使用不可・無い → 取り込まず item-excluded の警告。
func itemPresence(calcNames []string, sdItems []ShowdownItem) (included map[string]bool, nameEn map[string]string, warnings []Finding) {
	calcSet := map[string]bool{}
	for _, n := range calcNames {
		calcSet[toID(n)] = true
	}
	sdByID := map[string]ShowdownItem{}
	for _, it := range sdItems {
		sdByID[it.ID] = it
	}

	idSet := map[string]bool{}
	for id := range calcSet {
		idSet[id] = true
	}
	for id := range sdByID {
		idSet[id] = true
	}

	included = map[string]bool{}
	nameEn = map[string]string{}
	for _, id := range sortedKeysRaw(idSet) {
		calcHas := calcSet[id]
		it, sdHas := sdByID[id]
		sdStandard := sdHas && it.IsNonstandard == nil

		switch {
		case calcHas && sdStandard:
			included[id] = true
			nameEn[id] = it.Name
		case calcHas:
			warnings = append(warnings, Finding{Kind: KindItemExcluded, ID: id})
		case sdStandard:
			included[id] = true
			nameEn[id] = it.Name
			warnings = append(warnings, Finding{Kind: KindItemShowdownOnly, ID: id})
		}
	}
	return included, nameEn, warnings
}
