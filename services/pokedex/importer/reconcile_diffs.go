package importer

// calc と Showdown の値の全項目比較(ADR-0103 §3)。Convert の判定(取り込む/止める)とは
// 独立に、比較できる組を全部並べる。

import (
	"sort"
	"strconv"
	"strings"
)

// buildSpeciesMatchKey は calc の種族名 → 対応する Showdown の種族 ID(ADR-0101 §5 の対応規則:
// 名前そのもの、または baseForme 付きの名前)。
func buildSpeciesMatchKey(showdownSpecies []ShowdownSpecies) map[string]string {
	m := make(map[string]string, len(showdownSpecies))
	for _, s := range showdownSpecies {
		m[toID(s.Name)] = s.ID
		if s.BaseForme != "" {
			m[toID(s.Name+"-"+s.BaseForme)] = s.ID
		}
	}
	return m
}

func sortValueDiffs(diffs []ValueDiff) {
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].ID != diffs[j].ID {
			return diffs[i].ID < diffs[j].ID
		}
		return diffs[i].Field < diffs[j].Field
	})
}

// moveSeverity は ADR-0103 §3 の表のとおり: Imported でなければ常に info。Imported なら
// type は変化技なら warning・攻撃技なら blocker(規則3)、category/basePower は blocker、
// priority は warning。
func moveSeverity(field string, imported bool, calcStatus bool) string {
	if !imported {
		return "info"
	}
	switch field {
	case "type":
		if calcStatus {
			return "warning"
		}
		return "blocker"
	case "priority":
		return "warning"
	default: // category, basePower
		return "blocker"
	}
}

// computeMoveValueDiffs は calc が断片でなく Showdown に同じ ID がある組を全部比較する。
func computeMoveValueDiffs(in Input) []ValueDiff {
	calcByID := map[string]CalcMove{}
	for _, m := range in.Calc.Moves {
		if m.Name == sentinelMoveName {
			continue
		}
		calcByID[toID(m.Name)] = m
	}

	var diffs []ValueDiff
	for _, sm := range in.Showdown.Moves {
		cm, ok := calcByID[sm.ID]
		if !ok || isFragment(cm) {
			continue
		}
		imported := sm.IsNonstandard == nil
		calcCategory := resolveCalcCategory(cm.Category)
		calcStatus := calcCategory == "status"

		if !strings.EqualFold(cm.Type, sm.Type) {
			diffs = append(diffs, ValueDiff{
				Entity: "move", ID: sm.ID, Field: "type",
				Calc: strings.ToLower(cm.Type), Showdown: strings.ToLower(sm.Type),
				Severity: moveSeverity("type", imported, calcStatus), Imported: imported,
			})
		}
		sdCategory := strings.ToLower(sm.Category)
		if calcCategory != sdCategory {
			diffs = append(diffs, ValueDiff{
				Entity: "move", ID: sm.ID, Field: "category",
				Calc: calcCategory, Showdown: sdCategory,
				Severity: moveSeverity("category", imported, calcStatus), Imported: imported,
			})
		}
		if cm.BasePower != sm.BasePower {
			diffs = append(diffs, ValueDiff{
				Entity: "move", ID: sm.ID, Field: "basePower",
				Calc: strconv.Itoa(cm.BasePower), Showdown: strconv.Itoa(sm.BasePower),
				Severity: moveSeverity("basePower", imported, calcStatus), Imported: imported,
			})
		}
		if cm.Priority != sm.Priority {
			diffs = append(diffs, ValueDiff{
				Entity: "move", ID: sm.ID, Field: "priority",
				Calc: strconv.Itoa(cm.Priority), Showdown: strconv.Itoa(sm.Priority),
				Severity: moveSeverity("priority", imported, calcStatus), Imported: imported,
			})
		}
	}
	sortValueDiffs(diffs)
	return diffs
}

// computeSpeciesValueDiffs は calc の種族と対応する Showdown の種族(matchKey で見つかる組)を
// 全部比較する。対応が無い(除外設定の calc 種族など)は比較の対象外。
func computeSpeciesValueDiffs(in Input) []ValueDiff {
	matchKey := buildSpeciesMatchKey(in.Showdown.Species)
	sdByID := map[string]ShowdownSpecies{}
	for _, s := range in.Showdown.Species {
		sdByID[s.ID] = s
	}
	excludeSet := map[string]bool{}
	for _, name := range in.Config.ExcludeCalcSpecies {
		excludeSet[name] = true
	}

	var diffs []ValueDiff
	for _, c := range in.Calc.Species {
		sdID, ok := matchKey[toID(c.Name)]
		if !ok {
			continue
		}
		sd := sdByID[sdID]
		imported := sd.IsNonstandard == nil && c.BaseStats.HP != 1 && !excludeSet[c.Name]
		severity := "info"
		if imported {
			severity = "blocker"
		}

		calcTypes := joinTypeIDs(c.Types)
		sdTypes := joinTypeIDs(sd.Types)
		if calcTypes != sdTypes {
			diffs = append(diffs, ValueDiff{Entity: "species", ID: sd.ID, Field: "types", Calc: calcTypes, Showdown: sdTypes, Severity: severity, Imported: imported})
		}

		statFields := []struct {
			name     string
			calc, sd int
		}{
			{"hp", c.BaseStats.HP, sd.BaseStats.HP},
			{"atk", c.BaseStats.Atk, sd.BaseStats.Atk},
			{"def", c.BaseStats.Def, sd.BaseStats.Def},
			{"spa", c.BaseStats.SpA, sd.BaseStats.SpA},
			{"spd", c.BaseStats.SpD, sd.BaseStats.SpD},
			{"spe", c.BaseStats.Spe, sd.BaseStats.Spe},
		}
		for _, f := range statFields {
			if f.calc != f.sd {
				diffs = append(diffs, ValueDiff{Entity: "species", ID: sd.ID, Field: f.name, Calc: strconv.Itoa(f.calc), Showdown: strconv.Itoa(f.sd), Severity: severity, Imported: imported})
			}
		}
	}
	sortValueDiffs(diffs)
	return diffs
}

func joinTypeIDs(types []string) string {
	ids := make([]string, 0, len(types))
	for _, t := range types {
		ids = append(ids, toID(t))
	}
	return strings.Join(ids, "/")
}
