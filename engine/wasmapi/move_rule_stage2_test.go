package wasmapi_test

// 技の機構の段階2(ADR-0143 §6): WASM の入力境界が 技の rule・種族の weightHg・持ち物の isMegaStone・特性の効果 weightMod を
// 受け取り、engine の計算に渡すこと。rule のキーは camelCase で、Web が公開 API の PascalCase のまま渡しても受ける
// (Go の照合は大文字小文字を区別しない。黙って捨てない)。語彙に無い値は invalid_enum、値域・機構との対応の誤りは invalid_input、
// 未知のキーは unknown_field。

import (
	"encoding/json"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

type stage2ResultView struct {
	Rolls       [16]int    `json:"rolls"`
	Unsupported []markView `json:"unsupported"`
}

func calcStage2(t *testing.T, req map[string]any) stage2ResultView {
	t.Helper()
	resp := invoke(t, "calc", mustJSON(t, req))
	var env struct {
		Result *stage2ResultView `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil || env.Result == nil {
		t.Fatalf("calc が成功しない: %v\n%s", err, resp)
	}
	return *env.Result
}

// ruleReq は baseCalc の技を機構・威力・定義に差し替えたもの。
func ruleReq(t *testing.T, mechs []any, power int, rule map[string]any) map[string]any {
	t.Helper()
	req := baseCalc()
	mv := sub(t, req, "move")
	mv["mechanisms"] = mechs
	mv["power"] = power
	if rule != nil {
		mv["rule"] = rule
	}
	return req
}

// plainReq は同じ入力で、機構と定義の無い通常の技にしたもの。
func plainReq(t *testing.T, power int, edit func(map[string]any)) map[string]any {
	t.Helper()
	req := baseCalc()
	sub(t, req, "move")["power"] = power
	if edit != nil {
		edit(req)
	}
	return req
}

func assertWasmSame(t *testing.T, got, want stage2ResultView) {
	t.Helper()
	if got.Rolls != want.Rolls {
		t.Errorf("rolls = %v, want %v", got.Rolls, want.Rolls)
	}
	if len(got.Unsupported) != 0 {
		t.Errorf("unsupported = %v, want []", got.Unsupported)
	}
}

func TestWasmStage2RuleCamelAndPascal(t *testing.T) {
	ranks := func(req map[string]any) { sub(t, req, "attacker")["ranks"] = map[string]any{"atk": 2} }
	t.Run("camelCase の威力の式", func(t *testing.T) {
		req := ruleReq(t, []any{"variable_power"}, 20, map[string]any{"powerFormula": "attacker_positive_boosts"})
		ranks(req)
		assertWasmSame(t, calcStage2(t, req), calcStage2(t, plainReq(t, 60, ranks)))
	})
	t.Run("PascalCase(Web が公開 API の rule をそのまま渡す)", func(t *testing.T) {
		req := ruleReq(t, []any{"variable_power"}, 85, map[string]any{
			"PowerBoosts": []any{map[string]any{"Condition": "attacker_no_item", "BaseMultiplier": 2}}})
		assertWasmSame(t, calcStage2(t, req), calcStage2(t, plainReq(t, 170, nil)))
	})
	t.Run("rule が null は定義なし(印が残る)", func(t *testing.T) {
		req := ruleReq(t, []any{"variable_power"}, 85, nil)
		sub(t, req, "move")["rule"] = nil
		got := calcStage2(t, req)
		if len(got.Unsupported) != 1 || got.Unsupported[0].Reason != "variable_power" {
			t.Errorf("unsupported = %v, want [variable_power]", got.Unsupported)
		}
	})
	t.Run("持ち物なしで失敗する技は 0・印なし", func(t *testing.T) {
		req := ruleReq(t, []any{"move_specific"}, 85, map[string]any{"failsWithoutDefenderItem": true, "moveSpecificResolved": true})
		got := calcStage2(t, req)
		if got.Rolls != ([16]int{}) || len(got.Unsupported) != 0 {
			t.Errorf("rolls = %v・unsupported = %v, want 0・[]", got.Rolls, got.Unsupported)
		}
	})
}

func TestWasmStage2WeightAndMegaStone(t *testing.T) {
	weightRule := map[string]any{"powerFormula": "target_weight", "moveSpecificResolved": true}
	mechs := []any{"move_specific", "variable_power"}
	withWeight := func(req map[string]any, atk, def int) {
		sub(t, sub(t, req, "attacker"), "species")["weightHg"] = atk
		sub(t, sub(t, req, "defender"), "species")["weightHg"] = def
	}
	t.Run("種族の weightHg で威力が決まる(200.0kg → 120)", func(t *testing.T) {
		req := ruleReq(t, mechs, 0, weightRule)
		withWeight(req, 4600, 2000)
		assertWasmSame(t, calcStage2(t, req), calcStage2(t, plainReq(t, 120, nil)))
	})
	t.Run("特性の効果 weightMod(99.9kg × 2 → 100)", func(t *testing.T) {
		req := ruleReq(t, mechs, 0, weightRule)
		withWeight(req, 4600, 999)
		sub(t, req, "defender")["ability"] = map[string]any{"id": "test-heavy", "nameJa": "テストおもい",
			"effect": map[string]any{"weightMod": 8192, "breakable": true}}
		want := plainReq(t, 100, func(r map[string]any) {
			sub(t, r, "defender")["ability"] = map[string]any{"id": "test-heavy", "nameJa": "テストおもい",
				"effect": map[string]any{"weightMod": 8192, "breakable": true}}
		})
		assertWasmSame(t, calcStage2(t, req), calcStage2(t, want))
	})
	t.Run("weightHg の省略は不明(印が残る)", func(t *testing.T) {
		got := calcStage2(t, ruleReq(t, mechs, 0, weightRule))
		if len(got.Unsupported) == 0 {
			t.Error("重さが不明なのに印が無い")
		}
	})

	knock := map[string]any{"powerBoosts": []any{map[string]any{"condition": "defender_item_removable", "modifier": 6144}}}
	item := func(mega bool) map[string]any {
		it := map[string]any{"id": "test-stone", "nameJa": "テストいし"}
		if mega {
			it["isMegaStone"] = true
		}
		return it
	}
	t.Run("防御側の持ち物があれば 6144", func(t *testing.T) {
		req := ruleReq(t, []any{"variable_power"}, 65, knock)
		sub(t, req, "defender")["item"] = item(false)
		want := plainReq(t, 97, func(r map[string]any) { sub(t, r, "defender")["item"] = item(false) })
		assertWasmSame(t, calcStage2(t, req), calcStage2(t, want))
	})
	t.Run("isMegaStone の持ち物は補正なし・印が残る", func(t *testing.T) {
		req := ruleReq(t, []any{"variable_power"}, 65, knock)
		sub(t, req, "defender")["item"] = item(true)
		got := calcStage2(t, req)
		want := calcStage2(t, plainReq(t, 65, func(r map[string]any) { sub(t, r, "defender")["item"] = item(true) }))
		if got.Rolls != want.Rolls {
			t.Errorf("rolls = %v, want %v", got.Rolls, want.Rolls)
		}
		if len(got.Unsupported) != 1 || got.Unsupported[0].Reason != "variable_power" {
			t.Errorf("unsupported = %v, want [variable_power]", got.Unsupported)
		}
	})
}

func TestWasmStage2InvalidRule(t *testing.T) {
	cases := []struct {
		name     string
		mechs    []any
		rule     map[string]any
		edit     func(map[string]any)
		wantCode string
	}{
		{"未知のキー", []any{"variable_power"}, map[string]any{"powerFormulas": "speed_ratio"}, nil, wasmapi.CodeUnknownField},
		{"未知の式", []any{"variable_power"}, map[string]any{"powerFormula": "hp_ratio"}, nil, wasmapi.CodeInvalidEnum},
		{"未知の条件", []any{"variable_power"}, map[string]any{"powerBoosts": []any{
			map[string]any{"condition": "attacker_hp", "baseMultiplier": 2}}}, nil, wasmapi.CodeInvalidEnum},
		{"状態の一覧に未知の値", []any{"variable_power"}, map[string]any{"powerBoosts": []any{
			map[string]any{"condition": "defender_status", "statuses": []any{"toxic"}, "baseMultiplier": 2}}}, nil, wasmapi.CodeInvalidEnum},
		{"天候のタイプが語彙に無い", []any{"type_change"}, map[string]any{"typeByWeather": map[string]any{"rain": "shadow"}}, nil, wasmapi.CodeInvalidEnum},
		{"天候のキーが語彙に無い", []any{"type_change"}, map[string]any{"typeByWeather": map[string]any{"fog": "water"}}, nil, wasmapi.CodeInvalidEnum},
		{"空の定義", []any{"variable_power"}, map[string]any{}, nil, wasmapi.CodeInvalidInput},
		{"機構との対応が無い", []any{"type_change"}, map[string]any{"powerFormula": "speed_ratio"}, nil, wasmapi.CodeInvalidInput},
		{"整数倍と補正の両方", []any{"variable_power"}, map[string]any{"powerBoosts": []any{
			map[string]any{"condition": "attacker_no_item", "baseMultiplier": 2, "modifier": 6144}}}, nil, wasmapi.CodeInvalidInput},
		{"weightHg が負", []any{"move_specific", "variable_power"}, map[string]any{"powerFormula": "target_weight", "moveSpecificResolved": true},
			func(req map[string]any) { sub(t, sub(t, req, "defender"), "species")["weightHg"] = -1 }, wasmapi.CodeInvalidInput},
		{"weightMod が範囲外", []any{"variable_power"}, map[string]any{"powerFormula": "speed_ratio"},
			func(req map[string]any) {
				sub(t, req, "attacker")["ability"] = map[string]any{"id": "test-heavy", "nameJa": "テストおもい",
					"effect": map[string]any{"weightMod": 999999}}
			}, wasmapi.CodeInvalidInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := ruleReq(t, c.mechs, 50, c.rule)
			if c.edit != nil {
				c.edit(req)
			}
			got := decodeError(t, invoke(t, "calc", mustJSON(t, req)))
			if got.Code != c.wantCode {
				t.Errorf("code = %q, want %q(message=%q)", got.Code, c.wantCode, got.Message)
			}
		})
	}
}

// 一括計算の各行も同じ定義で計算し、印を付けない(CalcDamage の合成)。
func TestWasmStage2BulkUsesRule(t *testing.T) {
	req := baseBulk()
	mv := sub(t, req, "move")
	mv["mechanisms"] = []any{"variable_power"}
	mv["power"] = 20
	mv["rule"] = map[string]any{"powerFormula": "attacker_positive_boosts"}
	resp := invoke(t, "calcBulk", mustJSON(t, req))
	var env struct {
		Result struct {
			Rows []struct {
				Result struct {
					Unsupported *[]markView `json:"unsupported"`
				} `json:"result"`
			} `json:"rows"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("JSON でない: %v\n%s", err, resp)
	}
	if len(env.Result.Rows) == 0 {
		t.Fatalf("行が無い: %s", resp)
	}
	for i, row := range env.Result.Rows {
		if row.Result.Unsupported == nil || len(*row.Result.Unsupported) != 0 {
			t.Errorf("rows[%d].result.unsupported = %v, want []", i, row.Result.Unsupported)
		}
	}
}
