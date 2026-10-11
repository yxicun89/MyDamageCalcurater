package httpapi

// 契約で必須の sp(と StatBlock の6キー)の欠落は 400 invalid_input にする(issue #316)。
// 生成型は値型でゼロ値が入るため、キーの有無は生の JSON で確かめる(judge の toStats と同じ方式)。

import (
	"net/http"
	"testing"
)

func TestRequiredSPIsEnforced(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	endpoints := []struct {
		name, path, individual string
		body                   func() map[string]any
	}{
		{"calc attacker", "/api/calc", "attacker", calcBody},
		{"calc defender", "/api/calc", "defender", calcBody},
		{"bulk attacker", "/api/calc/bulk", "attacker", func() map[string]any { return bulkBody(movePhysical, nil, nil) }},
		{"reverse known", "/api/calc/reverse", "known", func() map[string]any { return reverseCases(t, store)[0].httpBody() }},
	}
	mutations := map[string]func(ind map[string]any){
		"sp 欠落":         func(ind map[string]any) { delete(ind, "sp") },
		"sp が null":     func(ind map[string]any) { ind["sp"] = nil },
		"sp.atk が null": func(ind map[string]any) { ind["sp"].(map[string]any)["atk"] = nil },
		"sp に atk のみ":   func(ind map[string]any) { ind["sp"] = map[string]any{"atk": 32} },
		"spe キーだけ欠落":    func(ind map[string]any) { delete(ind["sp"].(map[string]any), "spe") },
		"hp キーだけ欠落":     func(ind map[string]any) { delete(ind["sp"].(map[string]any), "hp") },
	}
	for _, ep := range endpoints {
		for mname, mutate := range mutations {
			t.Run(ep.name+"/"+mname, func(t *testing.T) {
				b := ep.body()
				mutate(b[ep.individual].(map[string]any))
				rec := post(t, h, ep.path, mustJSON(t, b), false)
				assertError(t, rec, http.StatusBadRequest, "invalid_input")
			})
		}
	}
}

// encoding/json はキー名の大文字小文字を区別せず束縛するので、"Attacker" / "SP" でも検査をすり抜けない。
func TestRequiredSPIsEnforcedForCaseVariantKeys(t *testing.T) {
	h := NewHandler(newFakeStore(t), nil)
	b := calcBody()
	ind := b["attacker"].(map[string]any)
	delete(ind, "sp")
	b["Attacker"] = ind
	delete(b, "attacker")
	assertError(t, post(t, h, "/api/calc", mustJSON(t, b), false), http.StatusBadRequest, "invalid_input")

	b = calcBody()
	ind = b["attacker"].(map[string]any)
	ind["SP"] = map[string]any{"atk": 32}
	delete(ind, "sp")
	assertError(t, post(t, h, "/api/calc", mustJSON(t, b), false), http.StatusBadRequest, "invalid_input")
}

// sp の欠落の検査は構文・未知フィールドの後、format の列挙検証より前(ADR-0200 §4)。
func TestRequiredSPIsCheckedBeforeFormatEnum(t *testing.T) {
	h := NewHandler(newFakeStore(t), nil)
	b := calcBody()
	b["format"] = "triple"
	delete(attackerOf(b), "sp")
	assertError(t, post(t, h, "/api/calc", mustJSON(t, b), false), http.StatusBadRequest, "invalid_input")
}
