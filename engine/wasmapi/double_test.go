package wasmapi_test

// issue #232 案B のダブル分・#288・ADR-0222: WASM の入力境界が
//   - move.target(技の対象。"" = 不明 / "single" / "spread")を受け取り、engine.Move.Target に渡す
//   - format=double を engine に渡し、結果の数値が変わる(壁 2732/4096・全体技 3072/4096)
//   - ダブルで技の対象が不明な攻撃技に技の印 move_target_unknown を出す
// ことを確かめる。数値の一致(境界は素通し)は vectors_test.go のベクタ(double / spread タグ)が見る。
// テラスタルは計算に使わない(ポケモンチャンピオンズに無い)。teraType の印は PR #497 / ADR-0160 の担当。
// 印は技の印(target=move)だけを比べる(#497 の format の印とは独立にする)。

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWasmMoveTargetEnum(t *testing.T) {
	for _, ok := range []string{"", "single", "spread"} {
		t.Run("受理/"+ok, func(t *testing.T) {
			req := baseCalc()
			sub(t, req, "move")["target"] = ok
			var got calcResultView
			decodeEnvelope(t, invoke(t, "calc", mustJSON(t, req)), &got)
		})
	}
	for _, bad := range []string{"Spread", "allAdjacent", "allAdjacentFoes", "aoe", " single"} {
		t.Run("拒否/"+bad, func(t *testing.T) {
			for _, fn := range []string{"calc", "calcBulk", "calcReverse"} {
				var req map[string]any
				switch fn {
				case "calc":
					req = baseCalc()
				case "calcBulk":
					req = baseBulk()
				default:
					req = baseReverse()
				}
				sub(t, req, "move")["target"] = bad
				if got := decodeError(t, invoke(t, fn, mustJSON(t, req))); got.Code != "invalid_enum" {
					t.Errorf("%s: code = %q, want invalid_enum", fn, got.Code)
				}
			}
		})
	}
}

func TestWasmMoveTargetUnknownMark(t *testing.T) {
	cases := []struct {
		name string
		edit func(req map[string]any)
		want []markView
	}{
		{"ダブル・対象省略は印", func(req map[string]any) { req["format"] = "double" },
			[]markView{{"move", "move_target_unknown", "bodyslam"}}},
		{"ダブル・対象が空文字も印", func(req map[string]any) {
			req["format"] = "double"
			sub(t, req, "move")["target"] = ""
		}, []markView{{"move", "move_target_unknown", "bodyslam"}}},
		{"ダブル・対象 single は印なし", func(req map[string]any) {
			req["format"] = "double"
			sub(t, req, "move")["target"] = "single"
		}, []markView{}},
		{"シングル・対象省略は印なし", func(map[string]any) {}, []markView{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			c.edit(req)
			got := []markView{}
			for _, m := range calcMarks(t, req) {
				if m.Target == "move" {
					got = append(got, m)
				}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("技の印 = %+v, want %+v", got, c.want)
			}
		})
	}
}

// 形式・技の対象が結果の数値に効く(境界で落とされていない)。
func TestWasmDoubleChangesResult(t *testing.T) {
	maxDamage := func(t *testing.T, req map[string]any) int {
		t.Helper()
		var got calcResultView
		decodeEnvelope(t, invoke(t, "calc", mustJSON(t, req)), &got)
		return got.MaxDamage
	}
	base := maxDamage(t, baseCalc())
	cases := []struct {
		name string
		edit func(req map[string]any)
		cmp  func(got, base int) bool
	}{
		// 全体技(ダブル)で ×0.75。
		{"ダブルの全体技で減る", func(req map[string]any) {
			req["format"] = "double"
			sub(t, req, "move")["target"] = "spread"
		}, func(got, base int) bool { return got < base }},
		{"シングルの全体技は変わらない", func(req map[string]any) { sub(t, req, "move")["target"] = "spread" },
			func(got, base int) bool { return got == base }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			c.edit(req)
			if got := maxDamage(t, req); !c.cmp(got, base) {
				t.Errorf("maxDamage = %d(基準 %d)", got, base)
			}
		})
	}
	// ダブルの壁は 2732/4096: シングルの壁(1/2)より大きい。
	single := baseCalc()
	single["field"] = map[string]any{"weather": "none", "terrain": "none", "defenderScreens": map[string]any{"reflect": true}}
	double := baseCalc()
	double["format"] = "double"
	double["field"] = map[string]any{"weather": "none", "terrain": "none", "defenderScreens": map[string]any{"reflect": true}}
	sub(t, double, "move")["target"] = "single"
	if s, d := maxDamage(t, single), maxDamage(t, double); !(d > s && d < base) {
		t.Errorf("壁: single=%d double=%d base=%d(single < double < base のはず)", s, d, base)
	}
}

// 一括・逆算でも形式・技の対象が効く。
func TestWasmBulkAndReverseUseDouble(t *testing.T) {
	bulkMax := func(t *testing.T, req map[string]any) []int {
		t.Helper()
		var b struct {
			Result struct {
				Rows []struct {
					Result struct {
						MaxDamage int `json:"maxDamage"`
					} `json:"result"`
				} `json:"rows"`
			} `json:"result"`
		}
		resp := invoke(t, "calcBulk", mustJSON(t, req))
		if err := json.Unmarshal([]byte(resp), &b); err != nil || len(b.Result.Rows) == 0 {
			t.Fatalf("calcBulk: %v\n%s", err, resp)
		}
		out := make([]int, len(b.Result.Rows))
		for i, r := range b.Result.Rows {
			out[i] = r.Result.MaxDamage
		}
		return out
	}
	base := bulkMax(t, baseBulk())
	spread := baseBulk()
	spread["format"] = "double"
	sub(t, spread, "move")["target"] = "spread"
	for name, req := range map[string]map[string]any{"ダブルの全体技": spread} {
		got := bulkMax(t, req)
		for i := range got {
			if got[i] == base[i] {
				t.Errorf("一括 %s: rows[%d] の maxDamage が変わらない(%d)", name, i, got[i])
			}
		}
	}

	revCands := func(t *testing.T, req map[string]any) string {
		t.Helper()
		var r struct {
			Result struct {
				Candidates json.RawMessage `json:"candidates"`
			} `json:"result"`
		}
		resp := invoke(t, "calcReverse", mustJSON(t, req))
		if err := json.Unmarshal([]byte(resp), &r); err != nil || len(r.Result.Candidates) == 0 {
			t.Fatalf("calcReverse: %v\n%s", err, resp)
		}
		return string(r.Result.Candidates)
	}
	rbase := revCands(t, baseReverse())
	rdouble := baseReverse()
	rdouble["format"] = "double"
	sub(t, rdouble, "move")["target"] = "spread"
	for name, req := range map[string]map[string]any{"ダブルの全体技": rdouble} {
		if got := revCands(t, req); got == rbase {
			t.Errorf("逆算 %s: 候補が変わらない", name)
		}
	}
}

// ベクタ(Go/WASM 一致テストの入力)にダブルの数値を通すものがある(vectors_test.go の必須タグとは別に固定する。
// PR #497 が同じ必須タグの一覧を変えるので、ここで独立に見る)。
func TestVectorsCoverDoubleScenarios(t *testing.T) {
	tags := map[string]int{}
	fns := map[string]bool{}
	for _, v := range loadVectors(t) {
		isDouble := false
		for _, tag := range v.Tags {
			tags[tag]++
			if tag == "double" {
				isDouble = true
			}
		}
		if isDouble {
			fns[v.Fn] = true
		}
	}
	for _, tag := range []string{"double", "spread"} {
		if tags[tag] == 0 {
			t.Errorf("必須タグ %q のベクタが無い", tag)
		}
	}
	for _, fn := range []string{"calc", "calcBulk", "calcReverse"} {
		if !fns[fn] {
			t.Errorf("fn=%q の double ベクタが無い", fn)
		}
	}
}
