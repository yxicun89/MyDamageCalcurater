package wasmapi_test

// P2-3b / ADR-0106 §決定 6: WASM の入力境界が無効・吸収の特性を受け取れること。
//
// 足さないと、ブラウザ(WASM)だけ無効・吸収が落ちて HTTP と結果が食い違う
// (ADR-0011「同じ入力で同じ結果」が壊れる)。ここでは calc の応答のダメージが 0 になることと、
// 不正な定義が既存のエラーコードに写ることを固定する。

import (
	"encoding/json"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

// groundMove は defenderIndividual(ノーマル単)に等倍で通る地面技。
func groundMove() map[string]any {
	return map[string]any{"id": "drillrun", "nameJa": "テストじめん", "type": "ground", "category": "physical", "power": 80, "priority": 0}
}

// withDefenderAbility は defender に特性(効果定義つき)を載せた calc リクエストを作る。
func withDefenderAbility(t *testing.T, effect map[string]any, move map[string]any) string {
	t.Helper()
	req := baseCalc()
	req["move"] = move
	def := sub(t, req, "defender")
	def["ability"] = map[string]any{"id": "testability", "nameJa": "テストとくせい", "effect": effect}
	return mustJSON(t, req)
}

func calcRolls(t *testing.T, request string) []int {
	t.Helper()
	var env struct {
		Result *struct {
			Rolls []int `json:"rolls"`
		} `json:"result"`
		Error *errorView `json:"error"`
	}
	if err := json.Unmarshal([]byte(wasmapi.Calc(request)), &env); err != nil {
		t.Fatalf("レスポンスが JSON ではない: %v", err)
	}
	if env.Error != nil {
		t.Fatalf("成功するはずが error: %+v", *env.Error)
	}
	if env.Result == nil || len(env.Result.Rolls) != 16 {
		t.Fatalf("rolls が 16 個でない: %+v", env.Result)
	}
	return env.Result.Rolls
}

func TestWasmAbilityImmunityZeroesDamage(t *testing.T) {
	cases := []struct {
		name   string
		effect map[string]any
	}{
		{"無効(defImmuneTypes)", map[string]any{"defImmuneTypes": []any{"ground"}}},
		{"吸収(回復)", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{"healNumerator": 1, "healDenominator": 4}},
		}},
		{"吸収(能力上昇)", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{"boostStat": "atk", "boostStages": 1}},
		}},
		{"吸収(副次効果なし)", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rolls := calcRolls(t, withDefenderAbility(t, tc.effect, groundMove()))
			for i, d := range rolls {
				if d != 0 {
					t.Fatalf("rolls[%d]=%d, want 0(WASM 境界で無効・吸収が落ちている): %v", i, d, rolls)
				}
			}
		})
	}
}

// 対象外のタイプの技には効かない(境界が効果を取り違えていないこと)。
func TestWasmAbilityImmunityDoesNotApplyToOtherTypes(t *testing.T) {
	want := calcRolls(t, mustJSON(t, baseCalc())) // 特性なしのノーマル技
	got := calcRolls(t, withDefenderAbility(t, map[string]any{"defImmuneTypes": []any{"ground"}}, bodySlam()))
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("対象外のタイプで値が変わった: got %v want %v", got, want)
		}
	}
}

// 不正な効果定義は既存のエラーコード語彙に写る(ADR-0011 §5)。黙って無視しない。
func TestWasmAbilityImmunityRejectsInvalidEffect(t *testing.T) {
	cases := []struct {
		name     string
		effect   map[string]any
		wantCode string
	}{
		{"未知のフィールド", map[string]any{"defImmuneTypez": []any{"ground"}}, wasmapi.CodeUnknownField},
		{"吸収の未知のフィールド", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{"healNumeratorr": 1}},
		}, wasmapi.CodeUnknownField},
		{"表に無いタイプの無効", map[string]any{"defImmuneTypes": []any{"nosuchtype"}}, wasmapi.CodeInvalidEnum},
		{"表に無いタイプの吸収", map[string]any{
			"defAbsorbTypes": map[string]any{"nosuchtype": map[string]any{}},
		}, wasmapi.CodeInvalidEnum},
		{"上げる能力が hp", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{"boostStat": "hp", "boostStages": 1}},
		}, wasmapi.CodeInvalidEnum},
		{"無効と吸収に同じタイプ", map[string]any{
			"defImmuneTypes": []any{"ground"},
			"defAbsorbTypes": map[string]any{"ground": map[string]any{}},
		}, wasmapi.CodeInvalidInput},
		{"無効のタイプが重複", map[string]any{
			"defImmuneTypes": []any{"ground", "ground"},
		}, wasmapi.CodeInvalidInput},
		{"回復の分子だけで分母が無い", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{"healNumerator": 1}},
		}, wasmapi.CodeInvalidInput},
		{"上げる段階が範囲外", map[string]any{
			"defAbsorbTypes": map[string]any{"ground": map[string]any{"boostStat": "atk", "boostStages": 7}},
		}, wasmapi.CodeInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeError(t, wasmapi.Calc(withDefenderAbility(t, tc.effect, groundMove())))
			if got.Code != tc.wantCode {
				t.Errorf("code=%q, want %q(message=%q)", got.Code, tc.wantCode, got.Message)
			}
		})
	}
}
