package importer

// 技の変換(ADR-0002 追記 P2-1c・ADR-0101 §4)。

import (
	"fmt"
	"sort"
	"strings"
)

// isFragment は calc の技が「断片」(type が無い。番兵 (No Move) も含む)かどうか。
func isFragment(m CalcMove) bool {
	return m.Type == "" || m.Name == "(No Move)"
}

// resolveCalcCategory は calc の category 省略を Status として解決する。
func resolveCalcCategory(category string) string {
	if category == "" {
		return "status"
	}
	return strings.ToLower(category)
}

type moveConversion struct {
	Rows     []MoveRow
	Included map[string]bool
}

func convertMoves(in Input, typeNameToID map[string]string) (moveConversion, []Finding, []Finding, error) {
	type calcEntry struct {
		move     CalcMove
		fragment bool
	}
	calcByID := map[string]calcEntry{}
	for _, m := range in.Calc.Moves {
		calcByID[toID(m.Name)] = calcEntry{move: m, fragment: isFragment(m)}
	}
	sdByID := map[string]ShowdownMove{}
	for _, m := range in.Showdown.Moves {
		sdByID[m.ID] = m
	}

	idSet := map[string]bool{}
	for id := range calcByID {
		idSet[id] = true
	}
	for id := range sdByID {
		idSet[id] = true
	}
	ids := sortedKeysRaw(idSet)

	var warnings, blockers []Finding
	included := map[string]bool{}
	rows := make([]MoveRow, 0, len(ids))

	for _, id := range ids {
		ce, calcHas := calcByID[id]
		sm, sdHas := sdByID[id]
		sdStandard := sdHas && sm.IsNonstandard == nil
		calcFull := calcHas && !ce.fragment

		switch {
		case calcFull && sdStandard:
			cm := ce.move
			typeID, ok := typeNameToID[sm.Type]
			if !ok {
				return moveConversion{}, nil, nil, fmt.Errorf("%w: 技 %q が除外したタイプを使っている: %q", ErrInvalidData, id, sm.Type)
			}
			finalCategory := resolveCalcCategory(cm.Category)
			if cm.Type != sm.Type {
				f := Finding{Kind: KindMoveTypeMismatch, ID: id}
				if finalCategory == "status" {
					warnings = append(warnings, f)
				} else {
					blockers = append(blockers, f)
				}
			}
			if finalCategory != strings.ToLower(sm.Category) {
				blockers = append(blockers, Finding{Kind: KindMoveValueMismatch, ID: id, Detail: "category"})
			}
			if cm.BasePower != sm.BasePower {
				blockers = append(blockers, Finding{Kind: KindMoveValueMismatch, ID: id, Detail: "basePower"})
			}
			if cm.Priority != sm.Priority {
				warnings = append(warnings, Finding{Kind: KindMoveValueMismatch, ID: id, Detail: "priority"})
			}
			included[id] = true
			rows = append(rows, MoveRow{
				ID: id, NameEn: sm.Name, Type: typeID, Category: finalCategory,
				Power: cm.BasePower, Accuracy: sm.Accuracy, PP: sm.PP, Priority: sm.Priority,
			})
		case calcFull:
			warnings = append(warnings, Finding{Kind: KindMoveExcluded, ID: id})
		case sdStandard:
			typeID, ok := typeNameToID[sm.Type]
			if !ok {
				return moveConversion{}, nil, nil, fmt.Errorf("%w: 技 %q が除外したタイプを使っている: %q", ErrInvalidData, id, sm.Type)
			}
			warnings = append(warnings, Finding{Kind: KindMoveShowdownOnly, ID: id})
			included[id] = true
			rows = append(rows, MoveRow{
				ID: id, NameEn: sm.Name, Type: typeID, Category: strings.ToLower(sm.Category),
				Power: sm.BasePower, Accuracy: sm.Accuracy, PP: sm.PP, Priority: sm.Priority,
			})
		case calcHas && ce.fragment:
			warnings = append(warnings, Finding{Kind: KindMoveExcluded, ID: id})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return moveConversion{Rows: rows, Included: included}, warnings, blockers, nil
}
