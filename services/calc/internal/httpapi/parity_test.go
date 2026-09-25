package httpapi

// HTTP(calc-svc)と WASM 境界(engine/wasmapi)のパリティ(ADR-0200 AC-9。ADR-0011 §10 の持ち越し)。
//
// 同じ失敗は同じ code、同じ入力は同じ数値になることを固定する。wasmapi には、HTTP が ID で渡す
// 個体・技・持ち物を fake のマスタで解決した実体(ADR-0011 §3 の DTO)と相性表を渡す。

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
)

// wasmError は wasmapi の失敗封筒から code を取り出す(成功なら "")。
func wasmError(t *testing.T, out string) string {
	t.Helper()
	var env struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("wasmapi の出力が JSON でない: %v; out=%s", err, out)
	}
	if env.Error == nil {
		return ""
	}
	return env.Error.Code
}

// wasmResult は wasmapi の成功封筒の result を汎用の値で返す。
func wasmResult(t *testing.T, out string) any {
	t.Helper()
	var env struct {
		Result any `json:"result"`
		Error  any `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("wasmapi の出力が JSON でない: %v; out=%s", err, out)
	}
	if env.Error != nil {
		t.Fatalf("wasmapi が失敗した: %s", out)
	}
	return dropEmptyUnsupported(t, env.Result)
}

// dropEmptyUnsupported は WASM の結果から「未対応」の印(unsupported。ADR-0123)を取り除く。
// HTTP の契約(api/openapi.yaml)にはまだ印の項目が無い(API レーンに追加を依頼中)ため、比べられない。
// ここのパリティの入力は印が付かないものだけなので、空でない印が出たら黙って捨てずに失敗させる。
// HTTP の契約に印が入ったら、この関数を消して印も比べる。
func dropEmptyUnsupported(t *testing.T, v any) any {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		if u, ok := x["unsupported"]; ok {
			if arr, isArr := u.([]any); !isArr || len(arr) != 0 {
				t.Fatalf("WASM の結果に空でない印がある(HTTP の契約に印が無いので比べられない): %v", u)
			}
			delete(x, "unsupported")
		}
		for k, child := range x {
			x[k] = dropEmptyUnsupported(t, child)
		}
	case []any:
		for i, child := range x {
			x[i] = dropEmptyUnsupported(t, child)
		}
	}
	return v
}

func calcWasmBody(t *testing.T, f *fakeStore, c calcCase) map[string]any {
	t.Helper()
	body := map[string]any{
		"format":    "single",
		"attacker":  c.attacker.wasm(t, f),
		"defender":  c.defender.wasm(t, f),
		"move":      wasmMove(f.moves[c.moveID]),
		"critical":  c.critical,
		"typeChart": wasmTypeChart(t),
	}
	if c.field != nil {
		body["field"] = c.field
	}
	return body
}

func bulkWasmBody(t *testing.T, f *fakeStore, moveID string, presetKeys any, itemIDs []string) map[string]any {
	t.Helper()
	body := map[string]any{
		"format":          "single",
		"attacker":        bulkAttacker().wasm(t, f),
		"defenderSpecies": wasmSpecies(f.species[speciesDefender]),
		"move":            wasmMove(f.moves[moveID]),
		"typeChart":       wasmTypeChart(t),
	}
	if presetKeys != nil {
		body["presetKeys"] = presetKeys
	}
	if itemIDs != nil {
		items := make([]any, 0, len(itemIDs))
		for _, id := range itemIDs {
			if id == "" {
				items = append(items, nil)
				continue
			}
			it := f.items[id]
			items = append(items, wasmItem(&it))
		}
		body["itemVariants"] = items
	}
	return body
}

func reverseWasmBody(t *testing.T, f *fakeStore, c reverseCase) map[string]any {
	t.Helper()
	obs := make([]any, 0, len(c.observations))
	for _, o := range c.observations {
		m := map[string]any{}
		if o.Percent != 0 {
			m["percent"] = o.Percent
		}
		if o.PercentTenths != 0 {
			m["percentTenths"] = o.PercentTenths
		}
		if o.Damage != 0 {
			m["damage"] = o.Damage
		}
		obs = append(obs, m)
	}
	body := map[string]any{
		"format":         "single",
		"side":           string(c.side),
		"known":          c.known.wasm(t, f),
		"unknownSpecies": wasmSpecies(f.species[c.unknownSpecies]),
		"move":           wasmMove(f.moves[c.moveID]),
		"observations":   obs,
		"maxCandidates":  c.maxCandidates,
		"typeChart":      wasmTypeChart(t),
	}
	if c.itemIDs != nil {
		items := make([]any, 0, len(c.itemIDs))
		for _, id := range c.itemIDs {
			if id == "" {
				items = append(items, nil)
				continue
			}
			it := f.items[id]
			items = append(items, wasmItem(&it))
		}
		body["itemCandidates"] = items
	}
	return body
}

// AC-9: 同じ失敗は HTTP と WASM で同じ code。
func TestErrorCodeParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	c := calcCases()[0]
	rc := reverseCases(t, store)[0]

	type pair struct {
		http []byte // HTTP の本文
		wasm string // wasmapi のリクエスト
	}
	both := func(httpBody, wasmBody map[string]any, mutateHTTP, mutateWasm func(map[string]any)) pair {
		mutateHTTP(httpBody)
		mutateWasm(wasmBody)
		return pair{http: mustJSON(t, httpBody), wasm: string(mustJSON(t, wasmBody))}
	}
	same := func(f func(map[string]any)) (func(map[string]any), func(map[string]any)) { return f, f }

	tests := []struct {
		name     string
		path     string
		wasm     func(string) string
		req      pair
		wantCode string
	}{
		{"calc: 壊れた JSON", "/api/calc", wasmapi.Calc, pair{http: []byte(`{"format":`), wasm: `{"format":`}, wasmapi.CodeInvalidJSON},
		{"calc: 未知のフィールド", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["bogus"] = 1 })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeUnknownField},
		{"calc: 未知の天候", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["field"] = map[string]any{"weather": "sunny"} })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidEnum},
		{"calc: 未知の状態異常", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["attacker"].(map[string]any)["status"] = "confused" })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidEnum},
		{"calc: SP 合計 67", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) {
				b["attacker"].(map[string]any)["sp"] = statsMap(engine.Stats{Atk: 32, Def: 32, Spe: 3})
			})
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidInput},
		{"calc: ランク 7", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["attacker"].(map[string]any)["ranks"] = ranksMap(engine.Ranks{Atk: 7}) })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidInput},
		// critic 指摘 R3: format / teraType / terrain の invalid_enum、型不一致の invalid_json、
		// level の invalid_input も HTTP と WASM で同じ code であることを固定する。
		{"calc: 未知の format", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["format"] = "triple" })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidEnum},
		{"calc: 未知の teraType", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["attacker"].(map[string]any)["teraType"] = "cosmic" })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidEnum},
		{"calc: 未知の terrain", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["field"] = map[string]any{"terrain": "swamp"} })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidEnum},
		{"calc: 型不一致(SP が文字列)", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) {
				b["attacker"].(map[string]any)["sp"] = map[string]any{"hp": "a", "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}
			})
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidJSON},
		{"calc: level が範囲外", "/api/calc", wasmapi.Calc, func() pair {
			mh, mw := same(func(b map[string]any) { b["attacker"].(map[string]any)["level"] = 51 })
			return both(c.httpBody(), calcWasmBody(t, store, c), mh, mw)
		}(), wasmapi.CodeInvalidInput},
		{"bulk: 重複した preset", "/api/calc/bulk", wasmapi.CalcBulk, pair{
			http: mustJSON(t, bulkBody(movePhysical, []any{"hp", "hp"}, nil)),
			wasm: string(mustJSON(t, bulkWasmBody(t, store, movePhysical, []any{"hp", "hp"}, nil))),
		}, wasmapi.CodeDuplicatePreset},
		{"bulk: 未知の preset", "/api/calc/bulk", wasmapi.CalcBulk, pair{
			http: mustJSON(t, bulkBody(movePhysical, []any{"hx"}, nil)),
			wasm: string(mustJSON(t, bulkWasmBody(t, store, movePhysical, []any{"hx"}, nil))),
		}, wasmapi.CodeUnknownPreset},
		{"reverse: 観測 0 件", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["observations"] = []any{} })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeNoObservation},
		{"reverse: percent と damage の両方", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["observations"] = []any{map[string]any{"percent": 40, "damage": 50}} })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeInvalidObservation},
		{"reverse: percent が 101", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["observations"] = []any{map[string]any{"percent": 101}} })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeInvalidObservation},
		{"reverse: percent に小数", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["observations"] = []any{map[string]any{"percent": 12.5}} })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeInvalidJSON},
		{"reverse: side が未知", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["side"] = "sideways" })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeInvalidReverseSide},
		{"reverse: percentTenths が範囲外", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["observations"] = []any{map[string]any{"percentTenths": 1001}} })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeInvalidObservation},
		{"reverse: damage が1未満", "/api/calc/reverse", wasmapi.CalcReverse, func() pair {
			mh, mw := same(func(b map[string]any) { b["observations"] = []any{map[string]any{"damage": -1}} })
			return both(rc.httpBody(), reverseWasmBody(t, store, rc), mh, mw)
		}(), wasmapi.CodeInvalidObservation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 前提: WASM 境界が実際にこの code を返す(wasmapi の挙動を先に確かめる)。
			if got := wasmError(t, tt.wasm(tt.req.wasm)); got != tt.wantCode {
				t.Fatalf("前提: wasmapi の code = %q, want %q", got, tt.wantCode)
			}
			rec := post(t, h, tt.path, tt.req.http, false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

// AC-9: 同じ入力なら calc の CalcResult は WASM の result と完全に同じ(キーも値も)。
func TestCalcResultParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	for _, c := range calcCases() {
		t.Run(c.name, func(t *testing.T) {
			want := wasmResult(t, wasmapi.Calc(string(mustJSON(t, calcWasmBody(t, store, c)))))
			rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
			var got any
			decodeInto(t, rec, &got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("HTTP と WASM の結果が違う\nHTTP: %s\nWASM: %s", mustJSON(t, got), mustJSON(t, want))
			}
		})
	}
}

// normalizeBulkRow は HTTP の行を WASM の行の形にそろえる。違いは契約上の表現だけ:
// 持ち物なし・無補正は HTTP が null、WASM が ""。natureId は HTTP だけにある。
func normalizeBulkRow(row map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range row {
		out[k] = v
	}
	if out["itemId"] == nil {
		out["itemId"] = ""
	}
	def := map[string]any{}
	for k, v := range row["defender"].(map[string]any) {
		def[k] = v
	}
	delete(def, "natureId")
	nature := map[string]any{}
	for k, v := range def["nature"].(map[string]any) {
		if v == nil {
			v = ""
		}
		nature[k] = v
	}
	def["nature"] = nature
	out["defender"] = def
	return out
}

// AC-9: 同じ入力なら bulk の各行(preset・持ち物・defender の SP/性格/実数値・result)は WASM と同じ。
func TestBulkResultParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	tests := []struct {
		name    string
		moveID  string
		presets []any
		itemIDs []string
	}{
		{"物理・既定セット", movePhysical, nil, nil},
		{"特殊・指定順・持ち物3通り", moveSpecial, []any{"hd_full", "none"}, []string{"", itemShell, itemPlain}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var httpItems any
			if tt.itemIDs != nil {
				items := make([]any, 0, len(tt.itemIDs))
				for _, id := range tt.itemIDs {
					if id == "" {
						items = append(items, nil)
					} else {
						items = append(items, id)
					}
				}
				httpItems = items
			}
			var httpPresets any
			if tt.presets != nil {
				httpPresets = tt.presets
			}
			wasmOut := wasmResult(t, wasmapi.CalcBulk(string(mustJSON(t,
				bulkWasmBody(t, store, tt.moveID, httpPresets, tt.itemIDs))))).(map[string]any)
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, bulkBody(tt.moveID, httpPresets, httpItems)), true)
			var got map[string]any
			decodeInto(t, rec, &got)

			if got["defenderSpeciesKey"] != wasmOut["defenderSpeciesKey"] {
				t.Errorf("defenderSpeciesKey = %v, want %v", got["defenderSpeciesKey"], wasmOut["defenderSpeciesKey"])
			}
			gotRows, _ := got["rows"].([]any)
			wantRows, _ := wasmOut["rows"].([]any)
			if len(gotRows) != len(wantRows) {
				t.Fatalf("行数 = %d, want %d", len(gotRows), len(wantRows))
			}
			for i := range wantRows {
				g := normalizeBulkRow(gotRows[i].(map[string]any))
				if !reflect.DeepEqual(g, wantRows[i]) {
					t.Errorf("rows[%d] が WASM と違う\nHTTP: %s\nWASM: %s", i, mustJSON(t, g), mustJSON(t, wantRows[i]))
				}
			}
		})
	}
}

// normalizeReverseCandidate は HTTP の候補を WASM の候補の形にそろえる。違いは契約上の表現だけ:
// 持ち物なし・無補正は HTTP が null、WASM が ""。natureId は HTTP だけにある。
func normalizeReverseCandidate(c map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range c {
		out[k] = v
	}
	delete(out, "natureId")
	if out["itemId"] == nil {
		out["itemId"] = ""
	}
	nature := map[string]any{}
	for k, v := range c["nature"].(map[string]any) {
		if v == nil {
			v = ""
		}
		nature[k] = v
	}
	out["nature"] = nature
	return out
}

// critic 指摘 R3 / AC-9: 同じ入力なら reverse の結果(側の別なく)は WASM と完全に同じ。
// side=defender(cases[0])と side=attacker(cases[2])の各1件を見る(TestCalcReverseNatureIDs と同じ選び方)。
func TestReverseResultParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	cases := reverseCases(t, store)
	for _, c := range []reverseCase{cases[0], cases[2]} {
		t.Run(c.name, func(t *testing.T) {
			wasmOut := wasmResult(t, wasmapi.CalcReverse(string(mustJSON(t, reverseWasmBody(t, store, c))))).(map[string]any)
			rec := post(t, h, "/api/calc/reverse", mustJSON(t, c.httpBody()), true)
			var got map[string]any
			decodeInto(t, rec, &got)

			for _, k := range []string{"side", "stat", "assumedHpSp", "exactCount"} {
				if got[k] != wasmOut[k] {
					t.Errorf("%s = %v, want %v", k, got[k], wasmOut[k])
				}
			}
			gotCands, _ := got["candidates"].([]any)
			wantCands, _ := wasmOut["candidates"].([]any)
			if len(gotCands) != len(wantCands) {
				t.Fatalf("候補数 = %d, want %d", len(gotCands), len(wantCands))
			}
			for i := range wantCands {
				g := normalizeReverseCandidate(gotCands[i].(map[string]any))
				if !reflect.DeepEqual(g, wantCands[i]) {
					t.Errorf("candidates[%d] が WASM と違う\nHTTP: %s\nWASM: %s", i, mustJSON(t, g), mustJSON(t, wantCands[i]))
				}
			}
		})
	}
}
