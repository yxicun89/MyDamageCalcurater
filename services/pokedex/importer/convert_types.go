package importer

// タイプ・相性表の変換(ADR-0101 §5 タイプの節)。

import (
	"fmt"
	"sort"

	"example.com/pokecalc/services/internal/master"
)

// typeConversion は Convert のタイプ処理の結果。
type typeConversion struct {
	Rows       []TypeRow
	Chart      map[string]bool // 取り込んだタイプ ID の集合
	NameToID   map[string]string
	MasterRows []master.TypeRow
	ChartRows  []master.TypeChartRow
}

func convertTypes(in Input, usedOverride map[string]bool) (typeConversion, []Finding, error) {
	excluded := map[string]bool{}
	calcTypeSet := map[string]bool{}
	for _, t := range in.Calc.Types {
		calcTypeSet[t] = true
	}
	for _, ex := range in.Config.ExcludeTypes {
		if !calcTypeSet[ex] {
			return typeConversion{}, nil, fmt.Errorf("%w: excludeTypes に calc に無いタイプ名がある: %q", ErrInvalidData, ex)
		}
		excluded[ex] = true
	}

	var order []string
	for _, t := range in.Calc.Types {
		if !excluded[t] {
			order = append(order, t)
		}
	}

	nameToID := make(map[string]string, len(order))
	idSet := make(map[string]bool, len(order))
	for _, name := range order {
		id := toID(name)
		nameToID[name] = id
		idSet[id] = true
	}

	pokeAPITypes := newPokeAPILookup(in.PokeAPI.Types)

	var warnings []Finding
	rows := make([]TypeRow, 0, len(order))
	for i, name := range order {
		id := nameToID[name]
		res := resolveJaName(id, in.Overrides.Types, pokeAPITypes[id], in.Config.NameJaLanguages, name, usedOverride)
		if res.Source == "fallback_en" {
			warnings = append(warnings, Finding{Kind: KindNameFallback, ID: id})
		}
		rows = append(rows, TypeRow{
			TypeRow:      master.TypeRow{ID: id, SortOrder: i + 1, NameJa: res.NameJa},
			NameJaSource: res.Source,
		})
	}

	var chartRows []master.TypeChartRow
	for atkName, row := range in.Calc.TypeChart {
		atkID, ok := nameToID[atkName]
		if !ok {
			continue
		}
		for defName, code := range row {
			defID, ok := nameToID[defName]
			if !ok {
				continue
			}
			chartRows = append(chartRows, master.TypeChartRow{AttackType: atkID, DefenseType: defID, Code: code})
		}
	}
	sort.Slice(chartRows, func(i, j int) bool {
		if chartRows[i].AttackType != chartRows[j].AttackType {
			return chartRows[i].AttackType < chartRows[j].AttackType
		}
		return chartRows[i].DefenseType < chartRows[j].DefenseType
	})

	masterRows := make([]master.TypeRow, 0, len(rows))
	for _, r := range rows {
		masterRows = append(masterRows, r.TypeRow)
	}

	return typeConversion{
		Rows:       rows,
		Chart:      idSet,
		NameToID:   nameToID,
		MasterRows: masterRows,
		ChartRows:  chartRows,
	}, warnings, nil
}
