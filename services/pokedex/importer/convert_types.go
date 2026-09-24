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

	chartRows, err := convertTypeChart(in.Calc.TypeChart, order, nameToID, excluded)
	if err != nil {
		return typeConversion{}, nil, err
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

// convertTypeChart は calc の相性表(等倍の組は省略)を行にする(#269)。除外タイプの名前だけは
// 攻撃側・防御側のどちらでも捨ててよい。それ以外の未知の名前(表記・大文字小文字の変化)や、
// 取り込むタイプの攻撃側の行の欠落(等倍だけのタイプも空オブジェクトで明示される)は、
// 相性表が黙って欠けて calc が等倍で計算する状態に落ちるので ErrInvalidData で止める。
// 行の有無ではなくキーの有無で判定するのは、等倍しかないタイプを許すため。
func convertTypeChart(table map[string]map[string]int, order []string, nameToID map[string]string, excluded map[string]bool) ([]master.TypeChartRow, error) {
	for _, name := range order {
		if _, ok := table[name]; !ok {
			return nil, fmt.Errorf("%w: calc の相性表に取り込むタイプ %q の攻撃側の行が無い(キーの表記の変化を疑う)", ErrInvalidData, name)
		}
	}
	var rows []master.TypeChartRow
	for _, atkName := range sortedKeysRaw(table) {
		if excluded[atkName] {
			continue
		}
		atkID, ok := nameToID[atkName]
		if !ok {
			return nil, fmt.Errorf("%w: calc の相性表の攻撃側に types に無いタイプ名がある: %q", ErrInvalidData, atkName)
		}
		for _, defName := range sortedKeysRaw(table[atkName]) {
			if excluded[defName] {
				continue
			}
			defID, ok := nameToID[defName]
			if !ok {
				return nil, fmt.Errorf("%w: calc の相性表の防御側に types に無いタイプ名がある: %q(攻撃側 %q)", ErrInvalidData, defName, atkName)
			}
			rows = append(rows, master.TypeChartRow{AttackType: atkID, DefenseType: defID, Code: table[atkName][defName]})
		}
	}
	// キーはそろっていても中身がすべて空(取得側で effectiveness が取れず空オブジェクトに落ちた形)
	// なら、全組み合わせ等倍になる。等倍でない組が1つも無いタイプ相性は無いので止める。
	if len(order) > 0 && len(rows) == 0 {
		return nil, fmt.Errorf("%w: calc の相性表に等倍でない組が1件も無い(取り込むタイプ %d 件。取得元の形の変化を疑う)", ErrInvalidData, len(order))
	}
	return rows, nil
}
