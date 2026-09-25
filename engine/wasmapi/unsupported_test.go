package wasmapi_test

// issue #271-b / #270 案 B / ADR-0123: WASM の入力境界が技の機構(move.mechanisms)と持ち物・特性の
// 「未対応」の印(effect.unsupportedAttacker / unsupportedDefender)を受け取り、結果の unsupported に
// 印を出すこと。印は常に配列で出す(印なしは [])。Web はまだ表示しない。

import (
	"encoding/json"
	"reflect"
	"testing"
)

type markView struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
	ID     string `json:"id"`
}

func calcMarks(t *testing.T, req map[string]any) []markView {
	t.Helper()
	resp := invoke(t, "calc", mustJSON(t, req))
	var env struct {
		Result struct {
			Unsupported *[]markView `json:"unsupported"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("JSON でない: %v\n%s", err, resp)
	}
	if env.Result.Unsupported == nil {
		t.Fatalf("result.unsupported が無い(null も不可): %s", resp)
	}
	return *env.Result.Unsupported
}

func TestWasmUnsupportedMarks(t *testing.T) {
	cases := []struct {
		name string
		edit func(req map[string]any)
		want []markView
	}{
		{"印なしは空配列", func(map[string]any) {}, []markView{}},
		{"技の機構", func(req map[string]any) {
			sub(t, req, "move")["mechanisms"] = []any{"variable_power", "multi_hit"}
		}, []markView{{"move", "multi_hit", "bodyslam"}, {"move", "variable_power", "bodyslam"}}},
		{"威力 0 の攻撃技", func(req map[string]any) {
			sub(t, req, "move")["power"] = 0
		}, []markView{{"move", "zero_power", "bodyslam"}}},
		{"防御側の未対応の持ち物", func(req map[string]any) {
			sub(t, req, "defender")["item"] = map[string]any{"id": "testitem", "nameJa": "テストどうぐ",
				"effect": map[string]any{"unsupportedDefender": true}}
		}, []markView{{"defender_item", "unsupported_effect", "testitem"}}},
		{"攻撃側の未対応の特性", func(req map[string]any) {
			sub(t, req, "attacker")["ability"] = map[string]any{"id": "testability", "nameJa": "テストとくせい",
				"effect": map[string]any{"unsupportedAttacker": true}}
		}, []markView{{"attacker_ability", "unsupported_effect", "testability"}}},
		{"攻撃側用の印を防御側が持っても印なし", func(req map[string]any) {
			sub(t, req, "defender")["ability"] = map[string]any{"id": "testability", "nameJa": "テストとくせい",
				"effect": map[string]any{"unsupportedAttacker": true}}
		}, []markView{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			c.edit(req)
			if got := calcMarks(t, req); !reflect.DeepEqual(got, c.want) {
				t.Errorf("unsupported = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestWasmUnsupportedMarksInBulkAndReverse(t *testing.T) {
	bulk := baseBulk()
	sub(t, bulk, "move")["mechanisms"] = []any{"multi_hit"}
	resp := invoke(t, "calcBulk", mustJSON(t, bulk))
	var b struct {
		Result struct {
			Rows []struct {
				Result struct {
					Unsupported []markView `json:"unsupported"`
				} `json:"result"`
			} `json:"rows"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &b); err != nil || len(b.Result.Rows) == 0 {
		t.Fatalf("calcBulk: %v\n%s", err, resp)
	}
	want := []markView{{"move", "multi_hit", "bodyslam"}}
	for i, r := range b.Result.Rows {
		if !reflect.DeepEqual(r.Result.Unsupported, want) {
			t.Errorf("rows[%d].result.unsupported = %+v, want %+v", i, r.Result.Unsupported, want)
		}
	}

	rev := baseReverse()
	sub(t, rev, "move")["mechanisms"] = []any{"multi_hit"}
	resp = invoke(t, "calcReverse", mustJSON(t, rev))
	var r struct {
		Result struct {
			Candidates []map[string]json.RawMessage `json:"candidates"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &r); err != nil || len(r.Result.Candidates) == 0 {
		t.Fatalf("calcReverse: %v\n%s", err, resp)
	}
	for i, c := range r.Result.Candidates {
		var got []markView
		if err := json.Unmarshal(c["unsupported"], &got); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("candidates[%d].unsupported = %s, want %+v", i, c["unsupported"], want)
		}
	}
}

func TestWasmRejectsInvalidMechanisms(t *testing.T) {
	cases := []struct {
		name       string
		mechanisms []any
		code       string
	}{
		{"未知の値", []any{"teleport"}, "invalid_enum"},
		{"大文字違い", []any{"Multi_Hit"}, "invalid_enum"},
		{"重複", []any{"ohko", "ohko"}, "invalid_input"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			sub(t, req, "move")["mechanisms"] = c.mechanisms
			if got := decodeError(t, invoke(t, "calc", mustJSON(t, req))); got.Code != c.code {
				t.Errorf("code = %q, want %q", got.Code, c.code)
			}
		})
	}
}
