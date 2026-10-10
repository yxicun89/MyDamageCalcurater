package httpapi

// POST /api/calc の対戦の状態 battleState(ADR-0144 §6)の受け入れテスト。
//
//   - battleState(attackerCurrentHp・defenderCurrentHp・hits)は engine の DamageInput.State にそのまま写す
//     (期待値は同じ入力を engine.CalcDamage に直接渡した結果。HTTP 境界は素通し)。省略は従来と同じ。
//   - 残り HP が 1 未満(0 を含む。engine の 0 = 満タンと取り違えない)・最大 HP を超える、範囲の多段でない技の回数・範囲外の回数は
//     400 invalid_input。battleState の未知のキーは 400 unknown_field。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

const (
	moveRangeMulti = "test-volley"  // normal / physical / 25・2〜5 回
	moveHPScaled   = "test-eruptor" // fire / special / 150・威力 × 攻撃側の残り HP / 最大 HP
	moveHalfHP     = "test-halver"  // normal / physical / 0・防御側の残り HP の半分の固定ダメージ
)

// stage3Store は fixture に段階3の技を足したもの。
func stage3Store(t *testing.T) *fakeStore {
	t.Helper()
	f := newFakeStore(t)
	f.moves[moveRangeMulti] = engine.Move{ID: moveRangeMulti, NameJa: "テストれんだ", Type: engine.TypeNormal,
		Category: engine.CategoryPhysical, Power: 25, Mechanisms: []engine.MoveMechanism{engine.MechanismMultiHit},
		Params: engine.MechanismParams{MultiHit: &engine.MultiHit{Min: 2, Max: 5}}}
	f.moves[moveHPScaled] = engine.Move{ID: moveHPScaled, NameJa: "テストふんしゅつ", Type: engine.TypeFire,
		Category: engine.CategorySpecial, Power: 150, Mechanisms: []engine.MoveMechanism{engine.MechanismVariablePower},
		Rule: &engine.MoveRule{PowerFormula: engine.PowerFormulaAttackerHPScaled}}
	f.moves[moveHalfHP] = engine.Move{ID: moveHalfHP, NameJa: "テストはんぶん", Type: engine.TypeNormal,
		Category: engine.CategoryPhysical, Mechanisms: []engine.MoveMechanism{engine.MechanismFixedDamage},
		Rule: &engine.MoveRule{FixedDamageFormula: engine.FixedDamageDefenderHalfHP}}
	return f
}

func TestCalcDamageBattleStateMatchesEngine(t *testing.T) {
	store := stage3Store(t)
	h := NewHandler(store, nil)
	attacker := indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{Atk: 32, SpA: 32}}
	defender := indiv{speciesKey: speciesDefender, natureID: natureNeutral}
	// テストモン HP 80 → 155、テストガード HP 95 → 170(SP 0)。
	cases := []struct {
		name   string
		moveID string
		state  map[string]any
		eng    engine.BattleState
	}{
		{"防御側の残り HP で確定数", movePhysical, map[string]any{"defenderCurrentHp": 40}, engine.BattleState{DefenderCurrentHP: 40}},
		{"攻撃側の残り HP で威力", moveHPScaled, map[string]any{"attackerCurrentHp": 77}, engine.BattleState{AttackerCurrentHP: 77}},
		{"防御側の残り HP の半分", moveHalfHP, map[string]any{"defenderCurrentHp": 91}, engine.BattleState{DefenderCurrentHP: 91}},
		{"多段の回数", moveRangeMulti, map[string]any{"hits": 5}, engine.BattleState{Hits: 5}},
		{"すべて", moveRangeMulti, map[string]any{"attackerCurrentHp": 155, "defenderCurrentHp": 1, "hits": 2},
			engine.BattleState{AttackerCurrentHP: 155, DefenderCurrentHP: 1, Hits: 2}},
		{"空のオブジェクトは省略と同じ", movePhysical, map[string]any{}, engine.BattleState{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cc := calcCase{attacker: attacker, defender: defender, moveID: c.moveID}
			in := cc.engineInput(t, store)
			in.State = c.eng
			want, err := engine.CalcDamage(in)
			if err != nil {
				t.Fatalf("engine.CalcDamage = %v", err)
			}
			body := cc.httpBody()
			body["battleState"] = c.state
			rec := post(t, h, "/api/calc", mustJSON(t, body), true)
			var got api.CalcResult
			decodeInto(t, rec, &got)
			assertCalcResultMatchesEngine(t, got, want)
			if len(got.Unsupported) != 0 {
				t.Errorf("unsupported = %v, want []", got.Unsupported)
			}
		})
	}
}

func TestCalcDamageBattleStateErrors(t *testing.T) {
	store := stage3Store(t)
	h := NewHandler(store, nil)
	attacker := indiv{speciesKey: speciesAttacker, natureID: natureNeutral}
	defender := indiv{speciesKey: speciesDefender, natureID: natureNeutral}
	cases := []struct {
		name     string
		moveID   string
		state    any
		wantCode string
	}{
		{"攻撃側の残り HP が最大(155)を超える", movePhysical, map[string]any{"attackerCurrentHp": 156}, "invalid_input"},
		{"防御側の残り HP が最大(170)を超える", movePhysical, map[string]any{"defenderCurrentHp": 171}, "invalid_input"},
		{"残り HP が 0(満タンの意味にしない)", movePhysical, map[string]any{"attackerCurrentHp": 0}, "invalid_input"},
		{"残り HP が負", movePhysical, map[string]any{"defenderCurrentHp": -3}, "invalid_input"},
		{"多段でない技に回数", movePhysical, map[string]any{"hits": 2}, "invalid_input"},
		{"範囲の外の回数", moveRangeMulti, map[string]any{"hits": 6}, "invalid_input"},
		{"回数 0", moveRangeMulti, map[string]any{"hits": 0}, "invalid_input"},
		{"未知のキー", movePhysical, map[string]any{"currentHp": 10}, "unknown_field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := calcCase{attacker: attacker, defender: defender, moveID: c.moveID}.httpBody()
			body["battleState"] = c.state
			rec := post(t, h, "/api/calc", mustJSON(t, body), false)
			assertError(t, rec, http.StatusBadRequest, c.wantCode)
		})
	}
}
