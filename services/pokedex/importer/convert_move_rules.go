package importer

// 技の処理の定義(effects.json の moveRules。ADR-0143 §4)と種族の重さ(weightkg)の変換。

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// buildMoveRules は effects.json の moveRules を検証・正準化して move_rules の行にする。
//   - moves 表に無い技のキーは投入せず effect-unused の警告にする(持ち物と同じ)。
//   - 変化技の定義・形や語彙の不正は ErrInvalidData(機構との対応は、最終防衛線の validateOutputMapsToEngine が
//     実際の技で確かめる)。
//
// 行は技 ID の昇順。
func buildMoveRules(raw map[string]json.RawMessage, moves []MoveRow, chart engine.TypeChart) ([]EffectRow, []Finding, error) {
	category := make(map[string]string, len(moves))
	for _, m := range moves {
		category[m.ID] = m.Category
	}
	ids := make([]string, 0, len(raw))
	for id := range raw {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var rows []EffectRow
	var warnings []Finding
	for _, id := range ids {
		cat, ok := category[id]
		if !ok {
			warnings = append(warnings, Finding{Kind: KindEffectUnused, ID: id})
			continue
		}
		if cat == "status" {
			return nil, nil, fmt.Errorf("%w: 変化技 %q に処理の定義がある", ErrInvalidData, id)
		}
		rule, err := master.DecodeMoveRule(raw[id], chart)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: 技 %q の処理の定義: %v", ErrInvalidData, id, err)
		}
		canon, err := master.EncodeMoveRule(*rule)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: 技 %q の処理の定義: %v", ErrInvalidData, id, err)
		}
		rows = append(rows, EffectRow{ID: id, Effect: canon})
	}
	return rows, warnings, nil
}

// weightKgPattern は取得物の重さ(kg)の字面: 整数か小数1桁までの10進(指数表記・符号なし)。
var weightKgPattern = regexp.MustCompile(`^([0-9]+)(?:\.([0-9]))?$`)

// maxWeightHg は species.weight_hg(SMALLINT UNSIGNED)に入る最大の重さ。
const maxWeightHg = 65535

// weightHgOf は取得物の重さ(kg の数値の字面)を hg の整数にする。浮動小数を経由しない(6.9 → 69)。
// 小数2桁以上・0 以下・指数表記・列に入らない大きさは ErrInvalidData。id は報告用。
func weightHgOf(id string, kg *json.Number) (int, error) {
	if kg == nil {
		return 0, fmt.Errorf("%w: 種族 %q に weightkg が無い", ErrInvalidData, id)
	}
	m := weightKgPattern.FindStringSubmatch(kg.String())
	if m == nil {
		return 0, fmt.Errorf("%w: 種族 %q の weightkg が小数1桁までの正の10進でない: %s", ErrInvalidData, id, kg.String())
	}
	whole, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("%w: 種族 %q の weightkg: %v", ErrInvalidData, id, err)
	}
	tenth := 0
	if m[2] != "" {
		tenth = int(m[2][0] - '0')
	}
	hg := whole*10 + tenth
	if hg <= 0 || hg > maxWeightHg {
		return 0, fmt.Errorf("%w: 種族 %q の重さが範囲外(0.1..%.1f kg): %s", ErrInvalidData, id, float64(maxWeightHg)/10, kg.String())
	}
	return hg, nil
}
