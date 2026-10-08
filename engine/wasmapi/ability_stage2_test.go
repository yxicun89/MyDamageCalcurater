package wasmapi_test

// ADR-0178: 技のフラグ(move.flags)と、特性の段階2の効果の項目を WASM の入力境界が受け付けて engine に渡す。
//
// 受け入れ条件(WASM 境界):
//   - AC-W1 move.flags: キーが無い・null は「不明」(engine.Move.FlagsKnown 偽)、配列は「既知」(空配列 = フラグなし)。
//     不明のときはフラグに依存する特性に未対応の印(unsupported_effect)が付き、既知なら付かない。
//     語彙に無い値は invalid_enum(Web はオンラインの公開 API の Move.flags をそのまま渡す)。
//   - AC-W2 特性の効果の新しいキー(postAuraPowerMods・powerMods の flag・flagTypeConvert・defImmuneFlags・
//     defFinalModsByFlag・defFinalModsByType・noContact)を camelCase で受け、入れ子(postAuraPowerMods・powerMods の要素・
//     flagTypeConvert)は PascalCase も受ける(Web の fromPublicEffect の渡し方。ADR-0176 §4 と同じ)。黙って捨てない。
//   - AC-W3 語彙・タイプの不正は invalid_enum、未知のキーは unknown_field。
// 数値の一致は「同じ補正を既存の持ち物の効果で表した入力」との比較で見る(境界が独自計算を持ち込まない)。

import (
	"bytes"
	"reflect"
	"testing"
)

func toughClawsEffect() map[string]any {
	return map[string]any{"postAuraPowerMods": []any{map[string]any{"condition": "move_flag", "flag": "contact", "modifier": 5325}}}
}

func setMoveFlags(t *testing.T, req map[string]any, flags any) {
	t.Helper()
	sub(t, req, "move")["flags"] = flags
}

func TestWasmMoveFlagsKnownAndUnknown(t *testing.T) {
	t.Run("既知のフラグで威力の補正が掛かる(持ち物の威力×1.3 と同じ)", func(t *testing.T) {
		req := baseCalc()
		setMoveFlags(t, req, []any{"contact"})
		setAbility(t, req, "attacker", toughClawsEffect())
		got := decodeSuccess(t, invoke(t, "calc", mustJSON(t, req)))

		want := baseCalc()
		setMoveFlags(t, want, []any{"contact"})
		sub(t, want, "attacker")["item"] = map[string]any{"id": "testpower", "nameJa": "テストパワー",
			"effect": map[string]any{"powerMod": 5325}}
		w := decodeSuccess(t, invoke(t, "calc", mustJSON(t, want)))
		if !bytes.Equal(got, w) {
			t.Errorf("フラグの威力補正が持ち物の威力補正と一致しない:\n got=%s\nwant=%s", got, w)
		}
	})

	for _, tc := range []struct {
		name     string
		flags    any // nil のときはキーを作らない
		omit     bool
		wantMark bool
	}{
		{"キーが無い = 不明", nil, true, true},
		{"null = 不明", nil, false, true},
		{"空配列 = 既知のフラグなし", []any{}, false, false},
		{"別のフラグ = 既知", []any{"sound"}, false, false},
		{"同じフラグ = 既知", []any{"contact"}, false, false},
	} {
		t.Run("印/"+tc.name, func(t *testing.T) {
			req := baseCalc()
			if !tc.omit {
				setMoveFlags(t, req, tc.flags)
			}
			setAbility(t, req, "attacker", toughClawsEffect())
			marks := calcMarks(t, req)
			want := []markView{}
			if tc.wantMark {
				want = []markView{{"attacker_ability", "unsupported_effect", "teststage1"}}
			}
			if !reflect.DeepEqual(marks, want) {
				t.Errorf("unsupported = %+v, want %+v", marks, want)
			}
		})
	}

	t.Run("不明なら補正は掛けない(特性なしと同じ数値)", func(t *testing.T) {
		req := baseCalc()
		setAbility(t, req, "attacker", toughClawsEffect())
		ctrl := baseCalc()
		gotRolls, ctrlRolls := calcRolls(t, mustJSON(t, req)), calcRolls(t, mustJSON(t, ctrl))
		if !reflect.DeepEqual(gotRolls, ctrlRolls) {
			t.Errorf("rolls = %v, want %v(フラグが不明なら効かない)", gotRolls, ctrlRolls)
		}
	})
}

func TestWasmRejectsInvalidMoveFlags(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags any
		code  string
	}{
		{"語彙に無い値", []any{"protect"}, "invalid_enum"},
		{"大文字違い", []any{"Contact"}, "invalid_enum"},
		{"空文字", []any{""}, "invalid_enum"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := baseCalc()
			setMoveFlags(t, req, tc.flags)
			if got := decodeError(t, invoke(t, "calc", mustJSON(t, req))); got.Code != tc.code {
				t.Errorf("code = %q, want %q", got.Code, tc.code)
			}
		})
	}
}

// どの項目も受け付け、効果がダメージを変える(黙って捨てていない)。技は既知のフラグを持たせる。
func TestWasmAcceptsEveryStage2AbilityField(t *testing.T) {
	cases := []struct {
		name   string
		side   string
		flags  []any
		effect map[string]any
		edit   func(req map[string]any)
	}{
		{"postAuraPowerMods(camelCase)", "attacker", []any{"contact"}, toughClawsEffect(), nil},
		{"postAuraPowerMods(PascalCase)", "attacker", []any{"punch"}, map[string]any{"postAuraPowerMods": []any{
			map[string]any{"Condition": "move_flag", "Flag": "punch", "Modifier": 4915}}}, nil},
		{"powerMods の move_flag", "attacker", []any{"bite"}, map[string]any{"powerMods": []any{
			map[string]any{"condition": "move_flag", "flag": "bite", "modifier": 6144}}}, nil},
		{"flagTypeConvert", "attacker", []any{"sound"}, map[string]any{"flagTypeConvert": map[string]any{"flag": "sound", "to": "fighting"}}, nil},
		{"flagTypeConvert(PascalCase)", "attacker", []any{"sound"}, map[string]any{"flagTypeConvert": map[string]any{"Flag": "sound", "To": "fighting"}}, nil},
		{"defImmuneFlags", "defender", []any{"sound"}, map[string]any{"defImmuneFlags": []any{"sound"}, "breakable": true}, nil},
		{"defFinalModsByFlag", "defender", []any{"contact"}, map[string]any{"defFinalModsByFlag": map[string]any{"contact": 2048}, "breakable": true}, nil},
		{"defFinalModsByType", "defender", []any{}, map[string]any{"defFinalModsByType": map[string]any{"normal": 8192}}, nil},
		{"noContact(防御側の接触の半減を受けない)", "attacker", []any{"contact"}, map[string]any{"noContact": true}, func(req map[string]any) {
			setAbilityWithID(t, req, "defender", "testfluffy", map[string]any{"defFinalModsByFlag": map[string]any{"contact": 2048}})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := baseCalc()
			setMoveFlags(t, ctrl, tc.flags)
			if tc.edit != nil {
				tc.edit(ctrl)
			}
			ctrlRes := decodeSuccess(t, invoke(t, "calc", mustJSON(t, ctrl)))
			req := baseCalc()
			setMoveFlags(t, req, tc.flags)
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
}

func TestWasmRejectsInvalidStage2AbilityFields(t *testing.T) {
	cases := []struct {
		name   string
		effect map[string]any
		code   string
	}{
		{"powerMods の flag が語彙に無い", map[string]any{"powerMods": []any{map[string]any{"condition": "move_flag", "flag": "wind", "modifier": 6144}}}, "invalid_enum"},
		{"postAuraPowerMods の語彙に無い条件", map[string]any{"postAuraPowerMods": []any{map[string]any{"condition": "contact", "modifier": 5325}}}, "invalid_enum"},
		{"flagTypeConvert.flag が語彙に無い", map[string]any{"flagTypeConvert": map[string]any{"flag": "wind", "to": "water"}}, "invalid_enum"},
		{"flagTypeConvert.to が未知のタイプ", map[string]any{"flagTypeConvert": map[string]any{"flag": "sound", "to": "nosuch"}}, "invalid_enum"},
		{"defImmuneFlags の値が語彙に無い", map[string]any{"defImmuneFlags": []any{"wind"}}, "invalid_enum"},
		{"defFinalModsByFlag のキーが語彙に無い", map[string]any{"defFinalModsByFlag": map[string]any{"wind": 2048}}, "invalid_enum"},
		{"defFinalModsByType のキーが未知のタイプ", map[string]any{"defFinalModsByType": map[string]any{"nosuch": 8192}}, "invalid_enum"},
		{"flagTypeConvert に未知のキー", map[string]any{"flagTypeConvert": map[string]any{"flag": "sound", "to": "water", "powerMod": 4915}}, "unknown_field"},
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

func setAbilityWithID(t *testing.T, req map[string]any, side, id string, effect map[string]any) {
	t.Helper()
	sub(t, req, side)["ability"] = map[string]any{"id": id, "nameJa": "テスト特性", "effect": effect}
}
