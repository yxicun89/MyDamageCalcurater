package wasmapi_test

// issue #232 / ADR-0160: WASM 境界は teraType と format=double を拒否せず受け取り、engine が付けた
// 「未対応」の印(attacker_tera_type / defender_tera_type / format。reason は unsupported_effect)を
// 結果の unsupported にそのまま写す。数値は印の有無で変わらない。

import (
	"encoding/json"
	"reflect"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

func TestWasmTeraAndFormatMarks(t *testing.T) {
	cases := []struct {
		name string
		edit func(req map[string]any)
		want []markView
	}{
		{"teraType 空・format 空(既定)は印なし", func(req map[string]any) {
			sub(t, req, "attacker")["teraType"] = ""
			req["format"] = ""
		}, []markView{}},
		{"攻撃側の teraType", func(req map[string]any) {
			sub(t, req, "attacker")["teraType"] = "fire"
		}, []markView{{"attacker_tera_type", "unsupported_effect", "fire"}}},
		{"防御側の teraType", func(req map[string]any) {
			sub(t, req, "defender")["teraType"] = "ghost"
		}, []markView{{"defender_tera_type", "unsupported_effect", "ghost"}}},
		{"format=double", func(req map[string]any) {
			req["format"] = "double"
		}, []markView{{"format", "unsupported_effect", "double"}}},
		{"技の機構の印の後に テラス(攻撃側 → 防御側) → 形式", func(req map[string]any) {
			sub(t, req, "move")["mechanisms"] = []any{"multi_hit"}
			sub(t, req, "attacker")["teraType"] = "fire"
			sub(t, req, "defender")["teraType"] = "water"
			req["format"] = "double"
		}, []markView{
			{"move", "multi_hit", "bodyslam"},
			{"attacker_tera_type", "unsupported_effect", "fire"},
			{"defender_tera_type", "unsupported_effect", "water"},
			{"format", "unsupported_effect", "double"},
		}},
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

// 数値(rolls・ko 等)は teraType・format の有無で変わらない(unsupported 以外のキーが完全に同じ)。
func TestWasmTeraAndFormatDoNotChangeNumbers(t *testing.T) {
	result := func(req map[string]any) map[string]any {
		t.Helper()
		var env struct {
			Result map[string]any `json:"result"`
		}
		resp := invoke(t, "calc", mustJSON(t, req))
		if err := json.Unmarshal([]byte(resp), &env); err != nil || env.Result == nil {
			t.Fatalf("calc: %v\n%s", err, resp)
		}
		delete(env.Result, "unsupported")
		return env.Result
	}
	plain := result(baseCalc())
	marked := baseCalc()
	sub(t, marked, "attacker")["teraType"] = "normal" // 技と同じタイプ(実装されれば STAB が変わる入力)
	sub(t, marked, "defender")["teraType"] = "ghost"  // 実装されれば無効になる入力
	marked["format"] = "double"
	if got := result(marked); !reflect.DeepEqual(got, plain) {
		t.Errorf("テラス・ダブルで数値が変わった\n got %v\nwant %v", got, plain)
	}
}

func TestWasmTeraAndFormatMarksInBulkAndReverse(t *testing.T) {
	want := []markView{{"attacker_tera_type", "unsupported_effect", "fire"}, {"format", "unsupported_effect", "double"}}

	bulk := baseBulk()
	sub(t, bulk, "attacker")["teraType"] = "fire"
	bulk["format"] = "double"
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
	for i, r := range b.Result.Rows {
		if !reflect.DeepEqual(r.Result.Unsupported, want) {
			t.Errorf("rows[%d].result.unsupported = %+v, want %+v", i, r.Result.Unsupported, want)
		}
	}

	rev := baseReverse() // side=defender: known は攻撃側
	sub(t, rev, "known")["teraType"] = "fire"
	rev["format"] = "double"
	resp = invoke(t, "calcReverse", mustJSON(t, rev))
	var r struct {
		Result struct {
			Candidates []struct {
				Unsupported []markView `json:"unsupported"`
			} `json:"candidates"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &r); err != nil || len(r.Result.Candidates) == 0 {
		t.Fatalf("calcReverse: %v\n%s", err, resp)
	}
	for i, c := range r.Result.Candidates {
		if !reflect.DeepEqual(c.Unsupported, want) {
			t.Errorf("candidates[%d].unsupported = %+v, want %+v", i, c.Unsupported, want)
		}
	}
}

// Go/WASM 一致のベクタ(tag "tera" / "double")が、テラス・形式の印を実際に出す入力であること。
// ベクタは scripts/wasm-conformance.mjs で WASM とバイト比較されるので、印の写しの Go/WASM 一致もこれで守る。
func TestVectorsTeraAndDoubleProduceMarks(t *testing.T) {
	type marksOnly struct {
		Unsupported []markView `json:"unsupported"`
	}
	collect := func(fn, resp string) [][]markView {
		t.Helper()
		var out [][]markView
		switch fn {
		case "calc":
			var env struct {
				Result marksOnly `json:"result"`
			}
			if err := json.Unmarshal([]byte(resp), &env); err != nil {
				t.Fatalf("%v\n%s", err, resp)
			}
			out = append(out, env.Result.Unsupported)
		case "calcBulk":
			var env struct {
				Result struct {
					Rows []struct {
						Result marksOnly `json:"result"`
					} `json:"rows"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(resp), &env); err != nil {
				t.Fatalf("%v\n%s", err, resp)
			}
			for _, row := range env.Result.Rows {
				out = append(out, row.Result.Unsupported)
			}
		case "calcReverse":
			var env struct {
				Result struct {
					Candidates []marksOnly `json:"candidates"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(resp), &env); err != nil {
				t.Fatalf("%v\n%s", err, resp)
			}
			for _, c := range env.Result.Candidates {
				out = append(out, c.Unsupported)
			}
		}
		return out
	}
	fns := map[string]func(string) string{"calc": wasmapi.Calc, "calcBulk": wasmapi.CalcBulk, "calcReverse": wasmapi.CalcReverse}

	seen := map[string]bool{}
	for _, v := range loadVectors(t) {
		var tera, double bool
		for _, tag := range v.Tags {
			tera = tera || tag == "tera"
			double = double || tag == "double"
		}
		if !tera && !double {
			continue
		}
		seen[v.Fn] = true
		t.Run(v.Name, func(t *testing.T) {
			sets := collect(v.Fn, fns[v.Fn](requestWithTypeChart(t, v.Request)))
			if len(sets) == 0 {
				t.Fatal("結果が空(行・候補が無い)")
			}
			for i, marks := range sets {
				var hasTera, hasFormat bool
				for _, m := range marks {
					hasTera = hasTera || m.Target == "attacker_tera_type" || m.Target == "defender_tera_type"
					hasFormat = hasFormat || (m.Target == "format" && m.ID == "double")
				}
				if tera && !hasTera {
					t.Errorf("[%d] tag tera なのにテラスの印が無い: %+v", i, marks)
				}
				if double && !hasFormat {
					t.Errorf("[%d] tag double なのに format の印が無い: %+v", i, marks)
				}
			}
		})
	}
	for _, fn := range []string{"calc", "calcBulk", "calcReverse"} {
		if !seen[fn] {
			t.Errorf("fn=%q に tag tera / double のベクタが無い(ADR-0160 AC-8)", fn)
		}
	}
}
