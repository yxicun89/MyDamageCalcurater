package importer

// 技の変換(ADR-0002 追記 P2-1c・ADR-0101 §4)。

import (
	"fmt"
	"sort"
	"strings"

	"example.com/pokecalc/services/internal/master"
)

// sentinelMoveName は calc の技一覧の番兵(実在しない技。ADR-0002 追記 P2-1c)。
const sentinelMoveName = "(No Move)"

// isFragment は calc の技が「断片」(type が無い。番兵 (No Move) も含む)かどうか。
func isFragment(m CalcMove) bool {
	return m.Type == "" || m.Name == sentinelMoveName
}

// resolveCalcCategory は calc の category 省略を Status として解決する。
func resolveCalcCategory(category string) string {
	if category == "" {
		return "status"
	}
	return strings.ToLower(category)
}

// 技の PP・命中の許容範囲。migration の moves の CHECK(chk_moves_pp・chk_moves_accuracy。
// ADR-0100 §3)が正で、TestMoveRangesMatchMigrationCheck で一致を確かめる(#310)。
// 命中 0 は必中(NULL で保存)なので範囲とは別に許す。
const (
	MovePPMin       = 1
	MovePPMax       = 64
	MoveAccuracyMin = 1
	MoveAccuracyMax = 100
)

// validateMoveRange は取得元の PP・命中が DB に入る範囲かを確かめる。範囲外を投入に回すと、
// 型変換の桁あふれで値が黙って変わる(pp=300 → 44)か、CHECK 違反で投入が失敗する(#310)。
func validateMoveRange(m ShowdownMove) error {
	if m.PP < MovePPMin || m.PP > MovePPMax {
		return fmt.Errorf("%w: 技 %q の pp %d が範囲 %d..%d の外", ErrInvalidData, m.ID, m.PP, MovePPMin, MovePPMax)
	}
	if m.Accuracy != 0 && (m.Accuracy < MoveAccuracyMin || m.Accuracy > MoveAccuracyMax) {
		return fmt.Errorf("%w: 技 %q の accuracy %d が 0(必中)または %d..%d の外", ErrInvalidData, m.ID, m.Accuracy, MoveAccuracyMin, MoveAccuracyMax)
	}
	return nil
}

// moveTargetOf は Showdown の技の対象を検証して返す。無い・空・未知の値は ErrInvalidData
// (黙って既定の対象にしない。ADR-0115・ADR-0136 §3)。
func moveTargetOf(m ShowdownMove) (string, error) {
	if m.Target == nil || !master.IsMoveTarget(*m.Target) {
		got := "(無い)"
		if m.Target != nil {
			got = fmt.Sprintf("%q", *m.Target)
		}
		return "", fmt.Errorf("%w: 技 %q の target が不正: %s", ErrInvalidData, m.ID, got)
	}
	return *m.Target, nil
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
			if err := validateMoveRange(sm); err != nil {
				return moveConversion{}, nil, nil, err
			}
			target, err := moveTargetOf(sm)
			if err != nil {
				return moveConversion{}, nil, nil, err
			}
			if cm.Target == nil {
				return moveConversion{}, nil, nil, fmt.Errorf("%w: calc の技 %q に target が無い", ErrInvalidData, id)
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
			// calc は全体技にだけ target を持つ(省略は "")。持つなら Showdown と同じ値、省略なら Showdown は全体技でない。
			if (*cm.Target != "" && *cm.Target != target) || (*cm.Target == "" && master.MoveTarget(target).IsSpread()) {
				f := Finding{Kind: KindMoveValueMismatch, ID: id, Detail: "target"}
				if finalCategory == "status" {
					warnings = append(warnings, f)
				} else {
					blockers = append(blockers, f)
				}
			}
			included[id] = true
			rows = append(rows, MoveRow{
				ID: id, NameEn: sm.Name, Type: typeID, Category: finalCategory,
				Power: cm.BasePower, Accuracy: sm.Accuracy, PP: sm.PP, Priority: sm.Priority, Target: target,
			})
		case calcFull:
			warnings = append(warnings, Finding{Kind: KindMoveExcluded, ID: id})
		case sdStandard:
			typeID, ok := typeNameToID[sm.Type]
			if !ok {
				return moveConversion{}, nil, nil, fmt.Errorf("%w: 技 %q が除外したタイプを使っている: %q", ErrInvalidData, id, sm.Type)
			}
			if err := validateMoveRange(sm); err != nil {
				return moveConversion{}, nil, nil, err
			}
			target, err := moveTargetOf(sm)
			if err != nil {
				return moveConversion{}, nil, nil, err
			}
			warnings = append(warnings, Finding{Kind: KindMoveShowdownOnly, ID: id})
			included[id] = true
			rows = append(rows, MoveRow{
				ID: id, NameEn: sm.Name, Type: typeID, Category: strings.ToLower(sm.Category),
				Power: sm.BasePower, Accuracy: sm.Accuracy, PP: sm.PP, Priority: sm.Priority, Target: target,
			})
		case calcHas && ce.fragment:
			warnings = append(warnings, Finding{Kind: KindMoveExcluded, ID: id})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return moveConversion{Rows: rows, Included: included}, warnings, blockers, nil
}
