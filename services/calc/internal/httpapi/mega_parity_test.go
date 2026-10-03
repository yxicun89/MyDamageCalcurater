package httpapi

// issue #505(ADR-0321): メガ種族の持ち物規則が HTTP(calc-svc。issue #315・ADR-0200 §4 追記)と WASM 境界
// (engine/wasmapi)で同じであることを固定する。同じ入力に、成否・エラー code・メッセージ(種族キー・持ち物 ID・
// 入力の場所を含む文言)が一致する。wasmapi には、HTTP が ID で渡す種族を fake のマスタで解決した実体
// (isMega・requiredItemId つき)を渡す。

import (
	"encoding/json"
	"net/http"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

// wasmMegaSpecies は種族の DTO に、マスタの isMega・requiredItemId(fake の megaItems)を足す。
func wasmMegaSpecies(f *fakeStore, key string) map[string]any {
	m := wasmSpecies(f.species[key])
	if required, ok := f.megaItems[key]; ok {
		m["isMega"] = true
		m["requiredItemId"] = required
	}
	return m
}

// wasmMegaIndiv は個体の WASM 入力。species にマスタの isMega・requiredItemId を載せる。
func wasmMegaIndiv(t *testing.T, f *fakeStore, in indiv) map[string]any {
	m := in.wasm(t, f)
	m["species"] = wasmMegaSpecies(f, in.speciesKey)
	return m
}

// wasmErrorMessage は wasmapi の失敗封筒から message を取り出す。
func wasmErrorMessage(t *testing.T, out string) string {
	t.Helper()
	var env struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || env.Error == nil {
		t.Fatalf("wasmapi の失敗封筒でない: %v; out=%s", err, out)
	}
	return env.Error.Message
}

func wasmMegaItems(f *fakeStore, ids []any) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		if id == nil {
			out = append(out, nil)
			continue
		}
		it := f.items[id.(string)]
		out = append(out, wasmItem(&it))
	}
	return out
}

func TestMegaItemRuleParityWithWasm(t *testing.T) {
	f := newMegaStore(t)
	h := NewHandler(f, nil)
	c := calcCases()[0]
	rc := reverseCases(t, f)[0]

	type tc struct {
		name     string
		path     string
		wasmFn   func(string) string
		httpBody map[string]any
		wasmBody map[string]any
	}
	var tests []tc

	for _, side := range []string{"attacker", "defender"} {
		for _, item := range []string{itemOrb, itemMegaStone, ""} {
			cc := c
			if side == "attacker" {
				cc.attacker = megaIndiv(item)
			} else {
				cc.defender = megaIndiv(item)
			}
			wb := calcWasmBody(t, f, cc)
			wb[side] = wasmMegaIndiv(t, f, megaIndiv(item))
			tests = append(tests, tc{"calc " + side + " item=" + item, "/api/calc", wasmapi.Calc, cc.httpBody(), wb})
		}
	}

	for _, item := range []string{itemOrb, itemMegaStone, ""} {
		hb := bulkBody(movePhysical, nil, nil)
		hb["attacker"] = megaIndiv(item).http()
		wb := bulkWasmBody(t, f, movePhysical, nil, nil)
		wb["attacker"] = wasmMegaIndiv(t, f, megaIndiv(item))
		tests = append(tests, tc{"bulk attacker item=" + item, "/api/calc/bulk", wasmapi.CalcBulk, hb, wb})
	}
	for name, variants := range map[string][]any{
		"variants 別の持ち物":        {nil, itemOrb, itemMegaStone},
		"variants メガストーンと null": {nil, itemMegaStone},
	} {
		hb := bulkBody(movePhysical, nil, variants)
		hb["defenderSpeciesKey"] = speciesMega
		wb := bulkWasmBody(t, f, movePhysical, nil, nil)
		wb["defenderSpecies"] = wasmMegaSpecies(f, speciesMega)
		wb["itemVariants"] = wasmMegaItems(f, variants)
		tests = append(tests, tc{"bulk " + name, "/api/calc/bulk", wasmapi.CalcBulk, hb, wb})
	}

	for _, item := range []string{itemOrb, itemMegaStone, ""} {
		rcc := rc
		rcc.known = megaIndiv(item)
		wb := reverseWasmBody(t, f, rcc)
		wb["known"] = wasmMegaIndiv(t, f, megaIndiv(item))
		tests = append(tests, tc{"reverse known item=" + item, "/api/calc/reverse", wasmapi.CalcReverse, rcc.httpBody(), wb})
	}
	for name, cands := range map[string][]any{
		"candidates 別の持ち物":        {nil, itemOrb},
		"candidates メガストーンと null": {nil, itemMegaStone},
	} {
		rcc := rc
		rcc.unknownSpecies = speciesMega
		ids := make([]string, 0, len(cands))
		for _, v := range cands {
			if v == nil {
				ids = append(ids, "")
			} else {
				ids = append(ids, v.(string))
			}
		}
		rcc.itemIDs = ids
		hb := rcc.httpBody()
		hb["unknownSpeciesKey"] = speciesMega
		hb["itemCandidates"] = cands
		wb := reverseWasmBody(t, f, rcc)
		wb["unknownSpecies"] = wasmMegaSpecies(f, speciesMega)
		tests = append(tests, tc{"reverse " + name, "/api/calc/reverse", wasmapi.CalcReverse, hb, wb})
	}

	sawReject, sawAccept := false, false
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, tt.path, mustJSON(t, tt.httpBody), false)
			out := tt.wasmFn(string(mustJSON(t, tt.wasmBody)))
			wasmCode := wasmError(t, out)
			if rec.Code == http.StatusOK {
				sawAccept = true
				if wasmCode != "" {
					t.Fatalf("HTTP は 200 だが WASM が失敗: %s", out)
				}
				return
			}
			sawReject = true
			e := decodeErrorBody(t, rec)
			if e.Code != wasmCode {
				t.Fatalf("code が不一致: HTTP=%q WASM=%q; wasm=%s", e.Code, wasmCode, out)
			}
			if e.Code != wasmapi.CodeInvalidInput {
				t.Fatalf("code = %q, want invalid_input", e.Code)
			}
			if wasmMsg := wasmErrorMessage(t, out); e.Message != wasmMsg {
				t.Errorf("メッセージが不一致:\n HTTP=%q\n WASM=%q", e.Message, wasmMsg)
			}
		})
	}
	if !sawReject || !sawAccept {
		t.Errorf("拒否と受理の両方のケースが必要(reject=%v accept=%v)", sawReject, sawAccept)
	}
}
