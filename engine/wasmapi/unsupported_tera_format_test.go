package wasmapi_test

// issue #232 / ADR-0160: WASM 境界は teraType を拒否せず受け取り、engine が付けた
// 「未対応」の印(reason は unsupported_effect)を結果の unsupported にそのまま写す。
// ADR-0224: 攻撃側のテラスは計算に反映するので印(attacker_tera_type)は付かない。防御側のテラスは
// 「そのタイプを持つか」には反映するが相性には反映しない(oracle の癖)ので、印(defender_tera_type)が残る。
//
// format=double は ADR-0222 で壁・全体技を計算に反映したので形式の印を付けない(ADR-0222 §5)。
// 技の対象(move.target)が不明なダブルの攻撃技には、技の印 move_target_unknown が技の印の最後に付く。

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
		{"攻撃側の teraType は印を付けない(ADR-0224 §3)", func(req map[string]any) {
			sub(t, req, "attacker")["teraType"] = "fire"
		}, []markView{}},
		{"防御側の teraType", func(req map[string]any) {
			sub(t, req, "defender")["teraType"] = "ghost"
		}, []markView{{"defender_tera_type", "unsupported_effect", "ghost"}}},
		{"format=double は形式の印を付けない(技の対象が不明な技の印だけ。ADR-0222 §5)", func(req map[string]any) {
			req["format"] = "double"
		}, []markView{{"move", "move_target_unknown", "bodyslam"}}},
		{"format=double で技の対象が分かれば印なし", func(req map[string]any) {
			req["format"] = "double"
			sub(t, req, "move")["target"] = "single"
		}, []markView{}},
		{"技の印(機構 → move_target_unknown)の後に 防御側のテラス。攻撃側のテラス・ダブルの形式の印は無い", func(req map[string]any) {
			sub(t, req, "move")["mechanisms"] = []any{"multi_hit"}
			sub(t, req, "attacker")["teraType"] = "fire"
			sub(t, req, "defender")["teraType"] = "water"
			req["format"] = "double"
		}, []markView{
			{"move", "multi_hit", "bodyslam"},
			{"move", "move_target_unknown", "bodyslam"},
			{"defender_tera_type", "unsupported_effect", "water"},
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

// calcResultWithoutMarks は calc の result から unsupported を除いたもの(数値の比較用)。
func calcResultWithoutMarks(t *testing.T, req map[string]any) map[string]any {
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

// 防御側の teraType はタイプ相性に反映しない(ADR-0224 §2。oracle = Champions 世代の癖)ので、この入力
// (天候・フィールドなし)では数値が変わらない。format=double も、この入力(壁なし・技の対象が不明=単体扱い)では
// 数値が変わらない(ADR-0222 §3.1)。攻撃側の teraType が技と一致しない(元タイプの技 = ×1.5 のまま。T2)入力も同じ。
// 壁・全体技でダブルの数値が変わることは double_test.go とベクタ calc/double-* が検証する。
func TestWasmTeraAndFormatDoNotChangeNumbers(t *testing.T) {
	plain := calcResultWithoutMarks(t, baseCalc())
	marked := baseCalc()
	sub(t, marked, "attacker")["teraType"] = "fire"  // 元タイプ(ノーマル)の技はテラスが別タイプでも ×1.5 のまま(T2)
	sub(t, marked, "defender")["teraType"] = "ghost" // 相性に反映しない(本編 SV なら無効になる入力)
	marked["format"] = "double"
	if got := calcResultWithoutMarks(t, marked); !reflect.DeepEqual(got, plain) {
		t.Errorf("テラス・ダブルで数値が変わった\n got %v\nwant %v", got, plain)
	}
}

// ADR-0224 §1: 攻撃側の teraType はタイプ一致に反映する(境界が engine へ素通ししていること)。
// カビゴン(ノーマル)にテラスノーマルでのしかかり → ×2.0(T1)。「テラス無しのてきおうりょく」(×2.0)と同じ数値になる。
func TestWasmAttackerTeraChangesNumbers(t *testing.T) {
	plain := calcResultWithoutMarks(t, baseCalc())
	tera := baseCalc()
	sub(t, tera, "attacker")["teraType"] = "normal"
	got := calcResultWithoutMarks(t, tera)
	adapt := baseCalc()
	sub(t, adapt, "attacker")["ability"] = map[string]any{"id": "Adaptability", "effect": map[string]any{"stabMod": 8192}}
	want := calcResultWithoutMarks(t, adapt)
	if reflect.DeepEqual(got, plain) {
		t.Errorf("攻撃側の teraType で数値が変わらない(engine に渡っていない): %v", got)
	}
	if !reflect.DeepEqual(got["rolls"], want["rolls"]) || got["stab"] != true {
		t.Errorf("テラス=元タイプ=技の rolls=%v stab=%v, want %v true(×2.0)", got["rolls"], got["stab"], want["rolls"])
	}
	if marks := calcMarks(t, tera); len(marks) != 0 {
		t.Errorf("unsupported = %+v, want [](攻撃側のテラスは印なし)", marks)
	}
}

func TestWasmTeraAndFormatMarksInBulkAndReverse(t *testing.T) {
	// ダブルは形式の印ではなく、技の対象が不明な技の印で分かる(ADR-0222 §5)。
	// 攻撃側のテラスは反映するので印なし(ADR-0224 §3)。
	want := []markView{{"move", "move_target_unknown", "bodyslam"}}

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

	// 相手が攻撃側: 既知の防御側のテラスは相性に反映しないので、全候補に防御側のテラスの印が付く(ADR-0224 §3)。
	revAtk := baseReverse()
	revAtk["side"] = "attacker"
	revAtk["known"] = defenderIndividual()
	revAtk["unknownSpecies"] = snorlaxSpecies()
	sub(t, revAtk, "known")["teraType"] = "ghost"
	revAtk["format"] = "double"
	resp = invoke(t, "calcReverse", mustJSON(t, revAtk))
	r.Result.Candidates = nil
	if err := json.Unmarshal([]byte(resp), &r); err != nil || len(r.Result.Candidates) == 0 {
		t.Fatalf("calcReverse(side=attacker): %v\n%s", err, resp)
	}
	wantDef := []markView{{"move", "move_target_unknown", "bodyslam"}, {"defender_tera_type", "unsupported_effect", "ghost"}}
	for i, c := range r.Result.Candidates {
		if !reflect.DeepEqual(c.Unsupported, wantDef) {
			t.Errorf("side=attacker candidates[%d].unsupported = %+v, want %+v", i, c.Unsupported, wantDef)
		}
	}
}

// Go/WASM 一致のベクタ(tag "tera" / "double")の印が ADR-0224 §3 のとおりであること:
// 攻撃側のテラスの印は出ず、防御側のテラス(calc の defender.teraType、逆算 side=attacker の known.teraType)を
// 持つ入力には防御側のテラスの印が出る。tag tera のベクタは攻撃側か防御側のテラスを持つ。
// tag double のベクタには形式の印が付かず(ADR-0222 §5)、技の対象(move.target)が無いものには
// move_target_unknown の印が付くこと。
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
		type teraOnly struct {
			TeraType string `json:"teraType"`
		}
		var req struct {
			Move struct {
				Target string `json:"target"`
			} `json:"move"`
			Side     string   `json:"side"`
			Attacker teraOnly `json:"attacker"`
			Defender teraOnly `json:"defender"`
			Known    teraOnly `json:"known"`
		}
		if err := json.Unmarshal(v.Request, &req); err != nil {
			t.Fatalf("%s: %v", v.Name, err)
		}
		moveHasTarget := req.Move.Target != ""
		atkTera, defTera := req.Attacker.TeraType, req.Defender.TeraType
		if v.Fn == "calcReverse" {
			if req.Side == "attacker" {
				defTera = req.Known.TeraType
			} else {
				atkTera = req.Known.TeraType
			}
		}
		if tera && atkTera == "" && defTera == "" {
			t.Errorf("%s: tag tera なのにテラスを持たない", v.Name)
		}
		t.Run(v.Name, func(t *testing.T) {
			sets := collect(v.Fn, fns[v.Fn](requestWithTypeChart(t, v.Request)))
			if len(sets) == 0 {
				t.Fatal("結果が空(行・候補が無い)")
			}
			for i, marks := range sets {
				var hasAtkTera, hasFormat, hasTargetUnknown bool
				defTeraMark := ""
				for _, m := range marks {
					hasAtkTera = hasAtkTera || m.Target == "attacker_tera_type"
					if m.Target == "defender_tera_type" {
						defTeraMark = m.ID
					}
					hasFormat = hasFormat || m.Target == "format"
					hasTargetUnknown = hasTargetUnknown || (m.Target == "move" && m.Reason == "move_target_unknown")
				}
				if hasAtkTera {
					t.Errorf("[%d] 攻撃側のテラスの印がある(ADR-0224 §3 で外した): %+v", i, marks)
				}
				if defTeraMark != defTera {
					t.Errorf("[%d] 防御側のテラスの印 = %q, want %q: %+v", i, defTeraMark, defTera, marks)
				}
				if double && hasFormat {
					t.Errorf("[%d] tag double なのに format の印がある(ADR-0222 §5 で外した): %+v", i, marks)
				}
				if double && !moveHasTarget && !hasTargetUnknown {
					t.Errorf("[%d] tag double で move.target が無いのに move_target_unknown の印が無い: %+v", i, marks)
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
