package wasmapi_test

// issue #255: WASM 境界に届く範囲外の種族値・重複タイプ・補正値は invalid_input になる
// (巨大確保で落ちない・非単調なロールや全ロール 1 を成功として返さない。ADR-0117)。

import (
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

func TestOutOfRangeInputIsInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		fn      string
		request func(t *testing.T) string
	}{
		{"calc: 防御側の HP 種族値が巨大", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "defender"), "species")["baseStats"] = stats(2000000000, 10, 10, 75, 135, 55)
			return mustJSON(t, r)
		}},
		{"calc: 攻撃側の種族値が 256", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "attacker"), "species")["baseStats"] = stats(160, 256, 65, 65, 110, 30)
			return mustJSON(t, r)
		}},
		{"calc: 種族値が 0", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "attacker"), "species")["baseStats"] = stats(160, 110, 0, 65, 110, 30)
			return mustJSON(t, r)
		}},
		{"calc: 防御側のタイプが重複", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "defender"), "species")["types"] = []any{"normal", "normal"}
			return mustJSON(t, r)
		}},
		{"calc: 持ち物の damageMod が負", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["item"] = map[string]any{"id": "x", "effect": map[string]any{"damageMod": -4096}}
			return mustJSON(t, r)
		}},
		{"calc: 持ち物の statMods が 0", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["item"] = map[string]any{"id": "x", "effect": map[string]any{"statMods": map[string]any{"atk": 0}}}
			return mustJSON(t, r)
		}},
		{"calc: 持ち物の powerMod が上限超過", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["item"] = map[string]any{"id": "x", "effect": map[string]any{"powerMod": 512*4096 + 1}}
			return mustJSON(t, r)
		}},
		{"calc: 特性の offBoostTypeMod が負(非単調なロールの再現)", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["ability"] = map[string]any{
				"id": "x", "effect": map[string]any{"offBoostType": "normal", "offBoostTypeMod": -4096},
			}
			return mustJSON(t, r)
		}},
		{"calc: 特性の stabMod が負", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["ability"] = map[string]any{"id": "x", "effect": map[string]any{"stabMod": -1}}
			return mustJSON(t, r)
		}},
		{"calc: 特性の defResistType が 0", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "defender")["ability"] = map[string]any{"id": "x", "effect": map[string]any{"defResistType": map[string]any{"normal": 0}}}
			return mustJSON(t, r)
		}},
		{"calcBulk: 防御側の種族のタイプが重複", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			sub(t, r, "defenderSpecies")["types"] = []any{"normal", "normal"}
			return mustJSON(t, r)
		}},
		{"calcBulk: 防御側の種族の HP 種族値が巨大", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			sub(t, r, "defenderSpecies")["baseStats"] = stats(2000000000, 10, 10, 75, 135, 55)
			return mustJSON(t, r)
		}},
		{"calcReverse: 推定側の種族値が 256", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			sub(t, r, "unknownSpecies")["baseStats"] = stats(255, 10, 256, 75, 135, 55)
			return mustJSON(t, r)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decodeError(t, invoke(t, c.fn, c.request(t)))
			if got.Code != wasmapi.CodeInvalidInput {
				t.Errorf("code: got %q want %q(message=%q)", got.Code, wasmapi.CodeInvalidInput, got.Message)
			}
		})
	}
}

// issue #317: ダメージを与えられない技の逆算は、候補ではなく invalid_input になる(ADR-0117 §3)。
func TestReverseWithNoDamageMoveIsInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		request func(t *testing.T) string
	}{
		{"変化技", func(t *testing.T) string {
			r := baseReverse()
			r["move"] = map[string]any{"id": "glare", "nameJa": "テストにらみ", "type": "normal", "category": "status", "power": 0, "priority": 0}
			return mustJSON(t, r)
		}},
		{"威力 0 の攻撃技", func(t *testing.T) string {
			r := baseReverse()
			sub(t, r, "move")["power"] = 0
			return mustJSON(t, r)
		}},
		{"タイプ相性の無効(ゴースト)", func(t *testing.T) string {
			r := baseReverse()
			sub(t, r, "unknownSpecies")["types"] = []any{"ghost"}
			return mustJSON(t, r)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decodeError(t, invoke(t, "calcReverse", c.request(t)))
			if got.Code != wasmapi.CodeInvalidInput {
				t.Errorf("code: got %q want %q(message=%q)", got.Code, wasmapi.CodeInvalidInput, got.Message)
			}
		})
	}
}
