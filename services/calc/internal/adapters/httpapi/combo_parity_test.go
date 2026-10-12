package httpapi

// POST /api/calc/combo と wasmapi.CalcCombo の一致(AC-18。ADR-0253・ADR-0179 追記 CB-5)。
//
// 同じ入力(comboCases)を calc-svc と WASM 境界に通し、応答のキーも値も同じであることを固定する。
// wasmapi には、HTTP が ID で渡す個体・技を fake のマスタで解決した実体を渡す(parity_test.go と同じ流儀)。

import (
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
)

// wasmComboMove は連続技の技を WASM の DTO にする。parity_test.go の wasmMove に、連続技が読む追加効果と
// 多段の機構を足したもの。
func wasmComboMove(m engine.Move) map[string]any {
	out := wasmMove(m)
	if e := m.Effect; e != nil {
		stages := map[string]any{}
		for k, v := range e.Stages {
			stages[string(k)] = v
		}
		out["effect"] = map[string]any{"chance": e.Chance, "target": string(e.Target), "stages": stages}
	}
	if len(m.Mechanisms) > 0 {
		mech := make([]string, 0, len(m.Mechanisms))
		for _, x := range m.Mechanisms {
			mech = append(mech, string(x))
		}
		out["mechanisms"] = mech
	}
	if mh := m.Params.MultiHit; mh != nil {
		out["mechanismParams"] = map[string]any{"multiHit": map[string]any{"min": mh.Min, "max": mh.Max}}
	}
	return out
}

func (c comboCase) wasmBody(t *testing.T, f *fakeStore) map[string]any {
	t.Helper()
	steps := make([]any, 0, len(c.steps))
	for _, s := range c.steps {
		st := map[string]any{"move": wasmComboMove(f.moves[s.moveID])}
		if s.critical != nil {
			st["critical"] = *s.critical
		}
		if s.hits != nil {
			st["hits"] = *s.hits
		}
		steps = append(steps, st)
	}
	body := map[string]any{
		"format":    "single",
		"attacker":  c.attacker.wasm(t, f),
		"defender":  c.defender.wasm(t, f),
		"steps":     steps,
		"typeChart": wasmTypeChart(t),
	}
	if c.field != nil {
		body["field"] = c.field
	}
	if c.opponentMove != "" {
		body["opponentMove"] = wasmComboMove(f.moves[c.opponentMove])
	}
	if c.opponentSpeed != "" {
		body["opponentSpeed"] = c.opponentSpeed
	}
	if sf := c.speedField; sf != nil {
		body["speedField"] = map[string]any{"trickRoom": sf.TrickRoom, "selfTailwind": sf.SelfTailwind, "opponentTailwind": sf.OpponentTailwind}
	}
	return body
}

// AC-18: 同じ入力なら /api/calc/combo の応答は WASM の calcCombo の result と完全に同じ(キーも値も)。
// 単発・変化技を含む手・多段・相手の技あり・相手の素早さ unknown / from_defender・トリックルーム・印を comboCases が含む。
func TestCalcComboResultParityWithWasm(t *testing.T) {
	store := newComboStore(t)
	h := NewHandler(store, nil)
	for _, c := range comboCases() {
		t.Run(c.name, func(t *testing.T) {
			want := wasmResult(t, wasmapi.CalcCombo(string(mustJSON(t, c.wasmBody(t, store)))))
			rec := post(t, h, pathCombo, mustJSON(t, c.httpBody()), true)
			var got any
			decodeInto(t, rec, &got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("HTTP と WASM の結果が違う\nHTTP: %s\nWASM: %s", mustJSON(t, got), mustJSON(t, want))
			}
		})
	}
}

// 前提: 突き合わせが空振りしない(印・相手の技・unknown の分かれ・トリックルームが実際に結果へ出ている)。
func TestCalcComboParityCasesCoverFeatures(t *testing.T) {
	store := newComboStore(t)
	var marks, opp, trick, multi bool
	for _, c := range comboCases() {
		res, err := engine.CalcCombo(c.engineInput(t, store))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		marks = marks || len(res.Unsupported) > 0
		opp = opp || res.OpponentAttack != nil
		trick = trick || (c.speedField != nil && c.speedField.TrickRoom)
		for _, s := range c.steps {
			multi = multi || s.hits != nil
		}
	}
	if !marks || !opp || !trick || !multi {
		t.Errorf("comboCases の網羅が足りない: 印=%v 相手の技=%v トリックルーム=%v 多段=%v", marks, opp, trick, multi)
	}
}
