package wasmapi_test

// issue #231 / ADR-0116: WASM の入力境界が特性の効果 airborne(浮いている)を受け取り、
// フィールドの補正を掛けないこと。落ちると HTTP(calc-svc)と結果が食い違う(ADR-0011)。

import (
	"slices"
	"testing"
)

func dragonMove() map[string]any {
	return map[string]any{"id": "dragonclaw", "nameJa": "テストドラゴン", "type": "dragon", "category": "physical", "power": 80, "priority": 0}
}

func electricMove() map[string]any {
	return map[string]any{"id": "thunderpunch", "nameJa": "テストでんき", "type": "electric", "category": "physical", "power": 80, "priority": 0}
}

// terrainRequest は side(attacker/defender)に airborne の特性を載せ、地形を terrain にした calc リクエスト。
func terrainRequest(t *testing.T, side, terrain string, airborne bool, move map[string]any) string {
	t.Helper()
	req := baseCalc()
	req["move"] = move
	req["field"] = map[string]any{"weather": "none", "terrain": terrain}
	if airborne {
		ind := sub(t, req, side)
		ind["ability"] = map[string]any{"id": "testability", "nameJa": "テストとくせい", "effect": map[string]any{"airborne": true}}
	}
	return mustJSON(t, req)
}

func TestWasmAirborneSkipsTerrain(t *testing.T) {
	cases := []struct {
		name, side, terrain string
		move                map[string]any
	}{
		{"浮いた攻撃側はエレキフィールドで強化されない", "attacker", "electric", electricMove()},
		{"浮いた防御側はミストフィールドでドラゴン半減されない", "defender", "misty", dragonMove()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			none := calcRolls(t, terrainRequest(t, c.side, "none", true, c.move))
			air := calcRolls(t, terrainRequest(t, c.side, c.terrain, true, c.move))
			grounded := calcRolls(t, terrainRequest(t, c.side, c.terrain, false, c.move))
			if !slices.Equal(air, none) {
				t.Errorf("airborne で地形の補正が掛かった: %v, want 地形なし %v", air, none)
			}
			if slices.Equal(grounded, none) {
				t.Errorf("接地しているのに地形の補正が掛からない: %v", grounded)
			}
		})
	}
}
