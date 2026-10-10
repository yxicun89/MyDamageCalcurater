package wasmapi_test

// 技の機構の段階3(ADR-0144 §6): WASM の calc の入力境界が、対戦の状態 battleState(attackerCurrentHp・defenderCurrentHp・hits)と、
// 持ち物の flingPower・持ち物の効果 grounds・技の rule の fixedDamageFormula・categoryByStats を受け取り、engine に渡すこと。
// battleState の省略は従来と同じ結果。値域の外(残り HP が最大を超える・範囲の多段でない技の回数 等)は invalid_input、
// battleState の未知のキーは unknown_field、語彙に無い式は invalid_enum。
//
// フィクスチャ: 攻撃側 snorlax(HP 実数値 235)・防御側 blissey(HP 実数値 330)。

import (
	"encoding/json"
	"testing"
)

type stage3ResultView struct {
	Rolls       [16]int      `json:"rolls"`
	HitRolls    [][16]int    `json:"hitRolls"`
	Category    string       `json:"category"`
	DefenderHP  int          `json:"defenderHP"`
	KO          stage3KOView `json:"ko"`
	Unsupported []markView   `json:"unsupported"`
}

type stage3KOView struct {
	Hits       int  `json:"hits"`
	Guaranteed bool `json:"guaranteed"`
}

func calcStage3(t *testing.T, req map[string]any) stage3ResultView {
	t.Helper()
	resp := invoke(t, "calc", mustJSON(t, req))
	var env struct {
		Result *stage3ResultView `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil || env.Result == nil {
		t.Fatalf("calc が成功しない: %v\n%s", err, resp)
	}
	return *env.Result
}

func TestWasmStage3BattleStateOmittedIsUnchanged(t *testing.T) {
	base := calcStage3(t, baseCalc())
	for name, state := range map[string]any{
		"空のオブジェクト": map[string]any{},
		"null":     nil,
		"満タンの明示":   map[string]any{"attackerCurrentHp": 235, "defenderCurrentHp": 330},
	} {
		req := baseCalc()
		req["battleState"] = state
		got := calcStage3(t, req)
		if got.Rolls != base.Rolls || got.KO != base.KO || got.DefenderHP != base.DefenderHP {
			t.Errorf("%s: %+v, want 省略と同じ %+v", name, got, base)
		}
	}
}

func TestWasmStage3CurrentHP(t *testing.T) {
	t.Run("防御側の残り HP の割合の威力(defender_hp_ratio)", func(t *testing.T) {
		req := ruleReq(t, []any{"variable_power"}, 0, map[string]any{"powerFormula": "defender_hp_ratio"})
		req["battleState"] = map[string]any{"defenderCurrentHp": 165}
		// floor(165 × 4096 / 330) = 2048 → 威力 50。
		assertWasmSame(t, stage2View(calcStage3(t, req)), stage2View(calcStage3(t, plainReq(t, 50, nil))))
	})
	t.Run("攻撃側の残り HP の固定ダメージ(PascalCase の定義)", func(t *testing.T) {
		req := ruleReq(t, []any{"fixed_damage"}, 0, map[string]any{"FixedDamageFormula": "attacker_current_hp"})
		req["battleState"] = map[string]any{"attackerCurrentHp": 100}
		got := calcStage3(t, req)
		for i, d := range got.Rolls {
			if d != 100 {
				t.Fatalf("rolls[%d] = %d, want すべて 100", i, d)
			}
		}
		if len(got.Unsupported) != 0 {
			t.Errorf("unsupported = %v, want []", got.Unsupported)
		}
	})
	t.Run("防御側の残り HP で確定数を数える・表示の分母は最大 HP", func(t *testing.T) {
		req := baseCalc()
		req["battleState"] = map[string]any{"defenderCurrentHp": 1}
		got := calcStage3(t, req)
		if got.KO.Hits != 1 || !got.KO.Guaranteed {
			t.Errorf("ko = %+v, want 確定1発", got.KO)
		}
		if got.DefenderHP != 330 {
			t.Errorf("defenderHP = %d, want 330(最大 HP)", got.DefenderHP)
		}
	})
}

func TestWasmStage3Hits(t *testing.T) {
	req := baseCalc()
	mv := sub(t, req, "move")
	mv["power"] = 25
	mv["mechanisms"] = []any{"multi_hit"}
	mv["mechanismParams"] = map[string]any{"multiHit": map[string]any{"min": 2, "max": 5}}
	req["battleState"] = map[string]any{"hits": 5}
	got := calcStage3(t, req)
	if len(got.HitRolls) != 5 {
		t.Fatalf("len(hitRolls) = %d, want 5(指定した回数)", len(got.HitRolls))
	}
	delete(req, "battleState")
	if got := calcStage3(t, req); len(got.HitRolls) != 3 {
		t.Errorf("省略: len(hitRolls) = %d, want 3(段階1の既定)", len(got.HitRolls))
	}
}

func TestWasmStage3ItemData(t *testing.T) {
	t.Run("なげつける型(持ち物の flingPower)", func(t *testing.T) {
		req := ruleReq(t, []any{"move_specific", "variable_power"}, 0,
			map[string]any{"powerFormula": "attacker_item_fling", "moveSpecificResolved": true})
		sub(t, req, "attacker")["item"] = map[string]any{"id": "test-ball", "nameJa": "テストの玉", "flingPower": 130}
		want := plainReq(t, 130, func(r map[string]any) {
			sub(t, r, "attacker")["item"] = map[string]any{"id": "test-ball", "nameJa": "テストの玉"}
		})
		assertWasmSame(t, stage2View(calcStage3(t, req)), stage2View(calcStage3(t, want)))
	})
	t.Run("持ち物の効果 grounds で飛行の攻撃側が接地", func(t *testing.T) {
		edit := func(r map[string]any, flying bool, item any) {
			mv := sub(t, r, "move")
			mv["type"], mv["category"] = "electric", "special"
			sub(t, r, "field")["terrain"] = "electric"
			atk := sub(t, r, "attacker")
			if flying {
				sp := snorlaxSpecies()
				sp["types"] = []any{"flying"}
				atk["species"] = sp
			}
			if item != nil {
				atk["item"] = item
			}
		}
		req := plainReq(t, 90, func(r map[string]any) {
			edit(r, true, map[string]any{"id": "test-iron", "nameJa": "テストの鉄", "effect": map[string]any{"grounds": true}})
		})
		want := plainReq(t, 90, func(r map[string]any) { edit(r, false, nil) })
		assertWasmSame(t, stage2View(calcStage3(t, req)), stage2View(calcStage3(t, want)))
	})
	t.Run("分類の切り替え(categoryByStats)", func(t *testing.T) {
		req := ruleReq(t, []any{"move_specific"}, 90, map[string]any{"categoryByStats": true, "moveSpecificResolved": true})
		sub(t, req, "move")["category"] = "special"
		// 攻撃側は攻撃に振り特攻が下がる性格、防御側は防御が極端に低い → 物理。
		if got := calcStage3(t, req); got.Category != "physical" {
			t.Errorf("category = %q, want physical", got.Category)
		}
	})
}

func TestWasmStage3Errors(t *testing.T) {
	cases := []struct {
		name     string
		edit     func(map[string]any)
		wantCode string
	}{
		{"攻撃側の残り HP が最大を超える", func(r map[string]any) { r["battleState"] = map[string]any{"attackerCurrentHp": 236} }, "invalid_input"},
		{"防御側の残り HP が最大を超える", func(r map[string]any) { r["battleState"] = map[string]any{"defenderCurrentHp": 331} }, "invalid_input"},
		{"残り HP が負", func(r map[string]any) { r["battleState"] = map[string]any{"attackerCurrentHp": -1} }, "invalid_input"},
		{"多段でない技に回数", func(r map[string]any) { r["battleState"] = map[string]any{"hits": 3} }, "invalid_input"},
		{"battleState の未知のキー", func(r map[string]any) { r["battleState"] = map[string]any{"currentHp": 10} }, "unknown_field"},
		{"持ち物の flingPower が負", func(r map[string]any) {
			sub(t, r, "attacker")["item"] = map[string]any{"id": "test-ball", "nameJa": "テストの玉", "flingPower": -1}
		}, "invalid_input"},
		{"語彙に無い固定ダメージの式", func(r map[string]any) {
			mv := sub(t, r, "move")
			mv["mechanisms"] = []any{"fixed_damage"}
			mv["power"] = 0
			mv["rule"] = map[string]any{"fixedDamageFormula": "level"}
		}, "invalid_enum"},
		{"固定ダメージの式に fixed_damage の機構が無い", func(r map[string]any) {
			mv := sub(t, r, "move")
			mv["mechanisms"] = []any{"variable_power"}
			mv["rule"] = map[string]any{"fixedDamageFormula": "attacker_current_hp"}
		}, "invalid_input"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			c.edit(req)
			got := decodeError(t, invoke(t, "calc", mustJSON(t, req)))
			if got.Code != c.wantCode {
				t.Errorf("code = %q, want %q(message=%q)", got.Code, c.wantCode, got.Message)
			}
		})
	}
}

// stage2View は段階2のテストの比較関数(assertWasmSame)に渡す形にする。
func stage2View(r stage3ResultView) stage2ResultView {
	return stage2ResultView{Rolls: r.Rolls, Unsupported: r.Unsupported}
}
