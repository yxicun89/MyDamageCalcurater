package wasmapi_test

// ADR-0139: 持ち物・特性の effect に素早さの補正(speedMods・ignoresParalysisSpeedDrop)が入っても、WASM の入力境界は
// 受け付ける(Web は公開 API の effect のキーを camelCase にしてそのまま渡す。web/src/master/onlineSource.ts の
// fromPublicEffect。境界は未知のフィールドを拒否するので、受け付けないと素早さの補正を持つ持ち物・特性の計算が
// すべて invalid_input になる)。ダメージの結果は素早さの項目の有無で変わらない。語彙に無い条件は invalid_enum。

import (
	"bytes"
	"testing"
)

func TestWasmAcceptsSpeedEffects(t *testing.T) {
	base := decodeSuccess(t, invoke(t, "calc", mustJSON(t, baseCalc())))

	req := baseCalc()
	sub(t, req, "attacker")["item"] = map[string]any{"id": "testspeedball", "nameJa": "テストすばやさだま",
		"effect": map[string]any{"speedMods": []any{map[string]any{"condition": "always", "modifier": 2048}}}}
	sub(t, req, "attacker")["ability"] = map[string]any{"id": "testswift", "nameJa": "テストすいすい",
		"effect": map[string]any{
			"speedMods":                 []any{map[string]any{"condition": "has_status", "modifier": 6144}},
			"ignoresParalysisSpeedDrop": true,
		}}
	sub(t, req, "defender")["ability"] = map[string]any{"id": "testswift2", "nameJa": "テストすいすい2",
		"effect": map[string]any{"speedMods": []any{map[string]any{"condition": "weather_rain", "modifier": 8192}}}}
	got := decodeSuccess(t, invoke(t, "calc", mustJSON(t, req)))

	if !bytes.Equal(got, base) {
		t.Errorf("素早さの項目でダメージの結果が変わった:\n got=%s\nbase=%s", got, base)
	}
}

func TestWasmRejectsInvalidSpeedCondition(t *testing.T) {
	cases := []struct {
		name string
		edit func(req map[string]any)
	}{
		{"持ち物の語彙に無い条件", func(req map[string]any) {
			sub(t, req, "attacker")["item"] = map[string]any{"id": "testspeedball", "nameJa": "テストすばやさだま",
				"effect": map[string]any{"speedMods": []any{map[string]any{"condition": "weather_fog", "modifier": 2048}}}}
		}},
		{"特性の条件の大文字違い", func(req map[string]any) {
			sub(t, req, "defender")["ability"] = map[string]any{"id": "testswift", "nameJa": "テストすいすい",
				"effect": map[string]any{"speedMods": []any{map[string]any{"condition": "Weather_Rain", "modifier": 8192}}}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			c.edit(req)
			if got := decodeError(t, invoke(t, "calc", mustJSON(t, req))); got.Code != "invalid_enum" {
				t.Errorf("code = %q, want invalid_enum", got.Code)
			}
		})
	}
}
