package httpapi

// issue #232 / ADR-0160: calc-svc は teraType・format=double を拒否せず(iOS の構築メンバーは teraType を
// 送る)、engine が付けたテラス・形式の「未対応」の印を CalcResult.unsupported(BulkCalcRow.result も同じ)・
// ReverseCandidate.unsupported に写す。契約の形は変えない(target・reason は enum にしない。ADR-0215)。
// HTTP と WASM で印を含めて同じ応答になる。

import (
	"net/http"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
)

var (
	markAttackerTeraFire = unsupportedMark{"attacker_tera_type", "unsupported_effect", "fire"}
	markDefenderTeraFire = unsupportedMark{"defender_tera_type", "unsupported_effect", "fire"}
	markFormatDouble     = unsupportedMark{"format", "unsupported_effect", "double"}
)

func TestCalcTeraAndFormatMarks(t *testing.T) {
	cases := []struct {
		name   string
		atk    engine.Type
		def    engine.Type
		format string
		want   []unsupportedMark
	}{
		{"テラスなし・シングルは印なし", "", "", "single", []unsupportedMark{}},
		{"攻撃側のテラス", engine.TypeFire, "", "single", []unsupportedMark{markAttackerTeraFire}},
		{"防御側のテラス", "", engine.TypeFire, "single", []unsupportedMark{markDefenderTeraFire}},
		{"ダブル", "", "", "double", []unsupportedMark{markFormatDouble}},
		{"両側のテラス + ダブル(攻撃側 → 防御側 → 形式の順)", engine.TypeFire, engine.TypeFire, "double",
			[]unsupportedMark{markAttackerTeraFire, markDefenderTeraFire, markFormatDouble}},
	}
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cc := calcCase{
				attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, tera: c.atk},
				defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral, tera: c.def},
				moveID:   movePhysical,
			}
			body := cc.httpBody()
			body["format"] = c.format
			rec := post(t, h, "/api/calc", mustJSON(t, body), true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200(テラス・ダブルを拒否しない。ADR-0160 §1): %s", rec.Code, rec.Body.String())
			}
			var got struct {
				Unsupported []unsupportedMark `json:"unsupported"`
			}
			decodeInto(t, rec, &got)
			if !reflect.DeepEqual(got.Unsupported, c.want) {
				t.Errorf("unsupported = %+v, want %+v", got.Unsupported, c.want)
			}
		})
	}
}

func TestBulkAndReverseTeraAndFormatMarks(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	want := []unsupportedMark{markAttackerTeraFire, markFormatDouble}

	attacker := bulkAttacker()
	attacker.tera = engine.TypeFire
	bulk := bulkBody(movePhysical, nil, nil)
	bulk["attacker"] = attacker.http()
	bulk["format"] = "double"
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, bulk), true)
	var b struct {
		Rows []struct {
			Result struct {
				Unsupported []unsupportedMark `json:"unsupported"`
			} `json:"result"`
		} `json:"rows"`
	}
	decodeInto(t, rec, &b)
	if len(b.Rows) == 0 {
		t.Fatal("rows が空")
	}
	for i, row := range b.Rows {
		if !reflect.DeepEqual(row.Result.Unsupported, want) {
			t.Errorf("rows[%d].result.unsupported = %+v, want %+v", i, row.Result.Unsupported, want)
		}
	}

	rc := reverseCases(t, store)[0] // side=defender: known は攻撃側
	rc.known.tera = engine.TypeFire
	rev := rc.httpBody()
	rev["format"] = "double"
	rec = post(t, h, "/api/calc/reverse", mustJSON(t, rev), true)
	var r struct {
		Candidates []struct {
			Unsupported []unsupportedMark `json:"unsupported"`
		} `json:"candidates"`
	}
	decodeInto(t, rec, &r)
	if len(r.Candidates) == 0 {
		t.Fatal("candidates が空")
	}
	for i, c := range r.Candidates {
		if !reflect.DeepEqual(c.Unsupported, want) {
			t.Errorf("candidates[%d].unsupported = %+v, want %+v", i, c.Unsupported, want)
		}
	}
}

// HTTP と WASM で、テラス・ダブルの入力の応答(印を含む)が同じ。
func TestTeraAndFormatParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)

	t.Run("calc", func(t *testing.T) {
		c := calcCase{
			attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, tera: engine.TypeFire},
			defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral, tera: engine.TypeFire},
			moveID:   movePhysical,
		}
		wasmBody := calcWasmBody(t, store, c)
		wasmBody["format"] = "double"
		want := wasmResult(t, wasmapi.Calc(string(mustJSON(t, wasmBody))))
		httpBody := c.httpBody()
		httpBody["format"] = "double"
		rec := post(t, h, "/api/calc", mustJSON(t, httpBody), true)
		var got any
		decodeInto(t, rec, &got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("HTTP と WASM の結果が違う\nHTTP: %s\nWASM: %s", mustJSON(t, got), mustJSON(t, want))
		}
		// 一致していても両方とも印が無いのでは意味がない(写し漏れが両側で揃う退行)。
		marks, _ := got.(map[string]any)["unsupported"].([]any)
		if len(marks) != 3 {
			t.Errorf("unsupported = %v, want 3 件(攻撃側テラス・防御側テラス・形式)", marks)
		}
	})

	t.Run("reverse", func(t *testing.T) {
		rc := reverseCases(t, store)[0]
		rc.known.tera = engine.TypeFire
		wasmBody := reverseWasmBody(t, store, rc)
		wasmBody["format"] = "double"
		wasmOut := wasmResult(t, wasmapi.CalcReverse(string(mustJSON(t, wasmBody)))).(map[string]any)
		httpBody := rc.httpBody()
		httpBody["format"] = "double"
		rec := post(t, h, "/api/calc/reverse", mustJSON(t, httpBody), true)
		var got map[string]any
		decodeInto(t, rec, &got)
		gotCands, _ := got["candidates"].([]any)
		wantCands, _ := wasmOut["candidates"].([]any)
		if len(gotCands) != len(wantCands) || len(gotCands) == 0 {
			t.Fatalf("候補数 = %d, want %d(> 0)", len(gotCands), len(wantCands))
		}
		for i := range wantCands {
			g := normalizeReverseCandidate(gotCands[i].(map[string]any))
			if !reflect.DeepEqual(g, wantCands[i]) {
				t.Errorf("candidates[%d] が WASM と違う\nHTTP: %s\nWASM: %s", i, mustJSON(t, g), mustJSON(t, wantCands[i]))
			}
			if marks, _ := g["unsupported"].([]any); len(marks) != 2 {
				t.Errorf("candidates[%d].unsupported = %v, want 2 件(攻撃側テラス・形式)", i, marks)
			}
		}
	})
}
