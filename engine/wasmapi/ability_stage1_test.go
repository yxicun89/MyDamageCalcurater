package wasmapi_test

// ADR-0176: 特性の段階1の効果の項目(typeConvert・powerMods・auraType/auraMod・statMods・separateStatMods・
// critDamageMod・preventsCritical・ignoresOpponentRanks・ignoresDefenderAbility・breakable)を、WASM の入力境界が
// 受け付けて engine にそのまま渡す(境界は未知のフィールドを拒否するので、受け付けないとその特性を選んだ計算が
// すべて invalid_input になる)。Web の fromPublicEffect はトップレベルのキーだけ小文字始まりにするので、
// typeConvert・powerMods の中のキーは PascalCase のまま渡る(ADR-0139 §4 と同じ)。語彙に無い値は invalid_enum。
// 数値の一致は「同じ補正を既存の持ち物の効果で表した入力」との比較で見る(境界が独自計算を持ち込まない)。

import (
	"bytes"
	"testing"
)

func setAbility(t *testing.T, req map[string]any, side string, effect map[string]any) {
	t.Helper()
	sub(t, req, side)["ability"] = map[string]any{"id": "teststage1", "nameJa": "テスト特性", "effect": effect}
}

func TestWasmTypeConvertMatchesConvertedMove(t *testing.T) {
	for _, tc := range []struct {
		name    string
		convert map[string]any
	}{
		{"camelCase", map[string]any{"from": "normal", "to": "fairy", "powerMod": 4915}},
		{"PascalCase(Web の渡し方)", map[string]any{"From": "normal", "To": "fairy", "PowerMod": 4915}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := baseCalc()
			setAbility(t, req, "attacker", map[string]any{"typeConvert": tc.convert})
			got := decodeSuccess(t, invoke(t, "calc", mustJSON(t, req)))

			want := baseCalc()
			move := bodySlam()
			move["type"] = "fairy"
			want["move"] = move
			sub(t, want, "attacker")["item"] = map[string]any{"id": "testfeather", "nameJa": "テストはね",
				"effect": map[string]any{"boostType": "fairy", "boostTypeMod": 4915}}
			w := decodeSuccess(t, invoke(t, "calc", mustJSON(t, want)))
			if !bytes.Equal(got, w) {
				t.Errorf("typeConvert の結果が変換後の技と一致しない:\n got=%s\nwant=%s", got, w)
			}
		})
	}
}

func TestWasmAbilityStatModsMatchItemStatMods(t *testing.T) {
	req := baseCalc()
	setAbility(t, req, "attacker", map[string]any{"statMods": map[string]any{"atk": 8192}})
	got := decodeSuccess(t, invoke(t, "calc", mustJSON(t, req)))

	want := baseCalc()
	sub(t, want, "attacker")["item"] = map[string]any{"id": "testband", "nameJa": "テストはちまき",
		"effect": map[string]any{"statMods": map[string]any{"atk": 8192}}}
	w := decodeSuccess(t, invoke(t, "calc", mustJSON(t, want)))
	if !bytes.Equal(got, w) {
		t.Errorf("特性の statMods の結果が持ち物の statMods と一致しない:\n got=%s\nwant=%s", got, w)
	}
}

// どの項目も受け付ける(効果がダメージを変えることも確かめる。黙って捨てていないこと)。
func TestWasmAcceptsEveryStage1AbilityField(t *testing.T) {
	base := decodeSuccess(t, invoke(t, "calc", mustJSON(t, baseCalc())))
	cases := []struct {
		name   string
		side   string
		effect map[string]any
		edit   func(req map[string]any)
	}{
		{"powerMods max_base_power", "attacker", map[string]any{"powerMods": []any{
			map[string]any{"Condition": "max_base_power", "MaxPower": 85, "Modifier": 6144}}}, nil},
		{"powerMods move_type", "attacker", map[string]any{"powerMods": []any{
			map[string]any{"condition": "move_type", "moveType": "normal", "modifier": 6144}}}, nil},
		{"auraType(防御側)", "defender", map[string]any{"auraType": "normal", "auraMod": 5448}, nil},
		{"separateStatMods", "attacker", map[string]any{"separateStatMods": map[string]any{"atk": 6144}}, nil},
		{"critDamageMod", "attacker", map[string]any{"critDamageMod": 6144}, func(req map[string]any) { req["critical"] = true }},
		{"preventsCritical", "defender", map[string]any{"preventsCritical": true, "breakable": true}, func(req map[string]any) { req["critical"] = true }},
		{"ignoresOpponentRanks", "defender", map[string]any{"ignoresOpponentRanks": true}, func(req map[string]any) {
			sub(t, req, "attacker")["ranks"] = map[string]any{"atk": 2}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := baseCalc()
			if tc.edit != nil {
				tc.edit(ctrl)
			}
			ctrlRes := decodeSuccess(t, invoke(t, "calc", mustJSON(t, ctrl)))
			req := baseCalc()
			if tc.edit != nil {
				tc.edit(req)
			}
			setAbility(t, req, tc.side, tc.effect)
			got := decodeSuccess(t, invoke(t, "calc", mustJSON(t, req)))
			if bytes.Equal(got, ctrlRes) {
				t.Errorf("効果の有無で結果が変わらない(境界で捨てていないか): %s", got)
			}
		})
	}

	t.Run("ignoresDefenderAbility は防御側の breakable な特性を無視", func(t *testing.T) {
		withResist := baseCalc()
		setAbility(t, withResist, "defender", map[string]any{"defResistType": map[string]any{"normal": 2048}, "breakable": true})
		req := baseCalc()
		setAbility(t, req, "defender", map[string]any{"defResistType": map[string]any{"normal": 2048}, "breakable": true})
		setAbility(t, req, "attacker", map[string]any{"ignoresDefenderAbility": true})
		got := decodeSuccess(t, invoke(t, "calc", mustJSON(t, req)))
		if !bytes.Equal(got, base) {
			t.Errorf("防御側の特性を無視した結果が、特性なしと一致しない:\n got=%s\nbase=%s", got, base)
		}
	})
}

func TestWasmRejectsInvalidStage1AbilityFields(t *testing.T) {
	cases := []struct {
		name   string
		effect map[string]any
		code   string
	}{
		{"typeConvert.to が未知のタイプ", map[string]any{"typeConvert": map[string]any{"from": "normal", "to": "nosuch", "powerMod": 4915}}, "invalid_enum"},
		{"typeConvert.from が未知のタイプ", map[string]any{"typeConvert": map[string]any{"from": "Normal", "to": "fairy", "powerMod": 4915}}, "invalid_enum"},
		{"powerMods の語彙に無い条件", map[string]any{"powerMods": []any{map[string]any{"condition": "contact", "modifier": 5325}}}, "invalid_enum"},
		{"powerMods.moveType が未知のタイプ", map[string]any{"powerMods": []any{map[string]any{"condition": "move_type", "moveType": "nosuch", "modifier": 6144}}}, "invalid_enum"},
		{"auraType が未知のタイプ", map[string]any{"auraType": "nosuch", "auraMod": 5448}, "invalid_enum"},
		{"statMods のキーが未知", map[string]any{"statMods": map[string]any{"attack": 8192}}, "invalid_enum"},
		{"separateStatMods のキーが未知", map[string]any{"separateStatMods": map[string]any{"attack": 6144}}, "invalid_enum"},
		{"typeConvert に未知のキー", map[string]any{"typeConvert": map[string]any{"from": "normal", "to": "fairy", "powerMod": 4915, "note": "x"}}, "unknown_field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseCalc()
			setAbility(t, req, "attacker", tc.effect)
			if got := decodeError(t, invoke(t, "calc", mustJSON(t, req))); got.Code != tc.code {
				t.Errorf("code = %q, want %q", got.Code, tc.code)
			}
		})
	}
}
