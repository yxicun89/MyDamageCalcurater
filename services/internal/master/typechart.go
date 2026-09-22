// Package master は pokedex の DB 行(素朴な行の型)を engine の型へ写像する(ADR-0100 §6)。
//
// engine は純粋に保つ(CLAUDE.md 絶対ルール2)ので、この写像は engine の外に置く。
// sqlc の生成型には依存せず、store 層が sqlc の行をここで定義する型に詰め替える
// (sqlc の出力名の変化に写像とそのテストを巻き込まないため)。
package master

import (
	"errors"
	"fmt"
	"regexp"
	"sort"

	"example.com/pokecalc/engine"
)

// ErrInvalidRow は行の値そのものが不正(形式・範囲・組の整合)。
var ErrInvalidRow = errors.New("行の値が不正")

// ErrInvalidEffect は効果定義の JSON が不正(effects.go)。
var ErrInvalidEffect = errors.New("効果定義の JSON が不正")

// タイプ・技・持ち物・特性の ID 形式、種族の key の形式(ADR-0100 §2)。
var (
	typeIDPattern     = regexp.MustCompile(`^[a-z]+$`)
	codeIDPattern     = regexp.MustCompile(`^[a-z0-9]+$`)
	speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)
)

// TypeRow は types テーブルの行。
type TypeRow struct {
	ID        string
	SortOrder int
	NameJa    string
}

// TypeChartRow は type_chart テーブルの行。
type TypeChartRow struct {
	AttackType  string
	DefenseType string
	Code        int
}

// TypeChartData は行の集合を engine.TypeChartData に変換する(検証は行うが、
// コード自体の妥当性(0/1/2/4)や未知のキーの判定は engine.NewTypeChart に委ねる)。
// Types は sort_order 順に並べる。
func TypeChartData(types []TypeRow, chart []TypeChartRow) (engine.TypeChartData, error) {
	sorted := append([]TypeRow(nil), types...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].SortOrder < sorted[j].SortOrder })

	seenID := map[string]bool{}
	seenSort := map[int]bool{}
	out := make([]engine.Type, 0, len(sorted))
	for _, t := range sorted {
		if !typeIDPattern.MatchString(t.ID) {
			return engine.TypeChartData{}, fmt.Errorf("%w: タイプ ID の形式が不正: %q", ErrInvalidRow, t.ID)
		}
		if t.NameJa == "" {
			return engine.TypeChartData{}, fmt.Errorf("%w: タイプ %q の日本語名が空", ErrInvalidRow, t.ID)
		}
		if seenID[t.ID] {
			return engine.TypeChartData{}, fmt.Errorf("%w: タイプ ID が重複: %q", ErrInvalidRow, t.ID)
		}
		seenID[t.ID] = true
		if seenSort[t.SortOrder] {
			return engine.TypeChartData{}, fmt.Errorf("%w: sort_order が重複: %d", ErrInvalidRow, t.SortOrder)
		}
		seenSort[t.SortOrder] = true
		out = append(out, engine.Type(t.ID))
	}

	seenPair := map[[2]string]bool{}
	effectiveness := map[engine.Type]map[engine.Type]int{}
	for _, c := range chart {
		key := [2]string{c.AttackType, c.DefenseType}
		if seenPair[key] {
			return engine.TypeChartData{}, fmt.Errorf("%w: 組 %q→%q が重複", ErrInvalidRow, c.AttackType, c.DefenseType)
		}
		seenPair[key] = true
		row, ok := effectiveness[engine.Type(c.AttackType)]
		if !ok {
			row = map[engine.Type]int{}
			effectiveness[engine.Type(c.AttackType)] = row
		}
		row[engine.Type(c.DefenseType)] = c.Code
	}

	return engine.TypeChartData{Types: out, Effectiveness: effectiveness}, nil
}

// TypeChart は行の集合から検証済みの engine.TypeChart を作る。
// 行そのものの不正(重複・形式)は ErrInvalidRow、表の定義としての不正
// (空・未知のキー・不正なコード)は engine.ErrInvalidTypeChart(errors.Is で判別可能)。
func TypeChart(types []TypeRow, chart []TypeChartRow) (engine.TypeChart, error) {
	data, err := TypeChartData(types, chart)
	if err != nil {
		return engine.TypeChart{}, err
	}
	c, err := engine.NewTypeChart(data)
	if err != nil {
		return engine.TypeChart{}, err
	}
	return c, nil
}
