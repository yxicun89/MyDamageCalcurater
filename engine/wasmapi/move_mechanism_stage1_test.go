package wasmapi_test

// 技の機構の段階1(ADR-0142 §8): WASM の入力境界が move.mechanismParams と特性の効果
// maxMultiHit・preventsOHKO を受け取り、結果に1発ごとの16段階 hitRolls(常に配列。単発は [])を出すこと。

import (
	"encoding/json"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

type stage1ResultView struct {
	Rolls       [16]int    `json:"rolls"`
	HitRolls    *[][16]int `json:"hitRolls"`
	DefenderHP  int        `json:"defenderHP"`
	Unsupported []markView `json:"unsupported"`
	KO          struct {
		Hits       int  `json:"hits"`
		Guaranteed bool `json:"guaranteed"`
	} `json:"ko"`
}

func calcStage1(t *testing.T, req map[string]any) stage1ResultView {
	t.Helper()
	resp := invoke(t, "calc", mustJSON(t, req))
	var env struct {
		Result *stage1ResultView `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil || env.Result == nil {
		t.Fatalf("calc が成功しない: %v\n%s", err, resp)
	}
	if env.Result.HitRolls == nil {
		t.Fatalf("result.hitRolls が無い(null も不可。単発は [])%s", resp)
	}
	return *env.Result
}

func TestWasmStage1HitRolls(t *testing.T) {
	single := calcStage1(t, baseCalc())
	if len(*single.HitRolls) != 0 {
		t.Fatalf("単発の hitRolls = %v, want []", *single.HitRolls)
	}

	req := baseCalc()
	mv := sub(t, req, "move")
	mv["mechanisms"] = []any{"multi_hit"}
	mv["mechanismParams"] = map[string]any{"multiHit": map[string]any{"min": 2, "max": 2}}
	got := calcStage1(t, req)
	if len(*got.HitRolls) != 2 {
		t.Fatalf("len(hitRolls) = %d, want 2", len(*got.HitRolls))
	}
	for h, r := range *got.HitRolls {
		if r != single.Rolls {
			t.Errorf("hitRolls[%d] = %v, want %v", h, r, single.Rolls)
		}
	}
	for i := range 16 {
		if got.Rolls[i] != 2*single.Rolls[i] {
			t.Errorf("rolls[%d] = %d, want %d(段の合計)", i, got.Rolls[i], 2*single.Rolls[i])
		}
	}
	if len(got.Unsupported) != 0 {
		t.Errorf("unsupported = %v, want []", got.Unsupported)
	}
}

func TestWasmStage1AbilityEffects(t *testing.T) {
	t.Run("maxMultiHit で範囲の最大回数", func(t *testing.T) {
		req := baseCalc()
		mv := sub(t, req, "move")
		mv["mechanisms"] = []any{"multi_hit"}
		mv["mechanismParams"] = map[string]any{"multiHit": map[string]any{"min": 2, "max": 5}}
		sub(t, req, "attacker")["ability"] = map[string]any{"id": "test-maxhit", "nameJa": "テストれんぞく",
			"effect": map[string]any{"maxMultiHit": true}}
		if got := calcStage1(t, req); len(*got.HitRolls) != 5 {
			t.Errorf("len(hitRolls) = %d, want 5", len(*got.HitRolls))
		}
	})
	t.Run("一撃必殺 × preventsOHKO は 0", func(t *testing.T) {
		req := baseCalc()
		mv := sub(t, req, "move")
		mv["power"] = 0
		mv["mechanisms"] = []any{"ohko"}
		mv["mechanismParams"] = map[string]any{"ohko": map[string]any{"immuneType": nil}}
		hit := calcStage1(t, req)
		if hit.Rolls[0] != hit.DefenderHP || !hit.KO.Guaranteed || hit.KO.Hits != 1 {
			t.Fatalf("一撃必殺: rolls[0]=%d HP=%d KO=%+v, want HP・確定1発", hit.Rolls[0], hit.DefenderHP, hit.KO)
		}
		sub(t, req, "defender")["ability"] = map[string]any{"id": "test-sturdy", "nameJa": "テストがんじょう",
			"effect": map[string]any{"preventsOHKO": true, "breakable": true}}
		if got := calcStage1(t, req); got.Rolls[15] != 0 {
			t.Errorf("preventsOHKO: rolls = %v, want 0", got.Rolls)
		}
	})
	t.Run("固定ダメージ(レベル)", func(t *testing.T) {
		req := baseCalc()
		mv := sub(t, req, "move")
		mv["power"] = 0
		mv["mechanisms"] = []any{"fixed_damage"}
		mv["mechanismParams"] = map[string]any{"fixedDamage": map[string]any{"level": true, "value": 0}}
		got := calcStage1(t, req)
		for i, r := range got.Rolls {
			if r != 50 {
				t.Fatalf("rolls[%d] = %d, want 50", i, r)
			}
		}
		if len(got.Unsupported) != 0 {
			t.Errorf("unsupported = %v, want [](zero_power も付けない)", got.Unsupported)
		}
	})
	t.Run("攻撃に使う能力値・防御に使う能力値", func(t *testing.T) {
		req := baseCalc()
		mv := sub(t, req, "move")
		mv["mechanisms"] = []any{"alt_offense_stat", "alt_defense_stat"}
		mv["mechanismParams"] = map[string]any{"offenseStat": "def", "offensePokemon": "defender", "defenseStat": "spd"}
		if got := calcStage1(t, req); len(got.Unsupported) != 0 {
			t.Errorf("unsupported = %v, want []", got.Unsupported)
		}
	})
}

func TestWasmStage1InvalidParams(t *testing.T) {
	cases := []struct {
		name     string
		mechs    []any
		params   map[string]any
		wantCode string
	}{
		{"offenseStat が語彙に無い", []any{"alt_offense_stat"}, map[string]any{"offenseStat": "luck"}, wasmapi.CodeInvalidEnum},
		{"offenseStat が hp", []any{"alt_offense_stat"}, map[string]any{"offenseStat": "hp"}, wasmapi.CodeInvalidEnum},
		{"offensePokemon が語彙に無い", []any{"alt_offense_stat"}, map[string]any{"offensePokemon": "ally"}, wasmapi.CodeInvalidEnum},
		{"defenseStat が語彙に無い", []any{"alt_defense_stat"}, map[string]any{"defenseStat": "luck"}, wasmapi.CodeInvalidEnum},
		{"ohko の immuneType が語彙に無い", []any{"ohko"}, map[string]any{"ohko": map[string]any{"immuneType": "shadow"}}, wasmapi.CodeInvalidEnum},
		{"multiHit の範囲が逆", []any{"multi_hit"}, map[string]any{"multiHit": map[string]any{"min": 5, "max": 2}}, wasmapi.CodeInvalidInput},
		{"multiHit の上限超過", []any{"multi_hit"}, map[string]any{"multiHit": map[string]any{"min": 2, "max": 11}}, wasmapi.CodeInvalidInput},
		{"fixedDamage が level と value の両方", []any{"fixed_damage"}, map[string]any{"fixedDamage": map[string]any{"level": true, "value": 40}}, wasmapi.CodeInvalidInput},
		{"機構の無い multiHit", []any{}, map[string]any{"multiHit": map[string]any{"min": 2, "max": 2}}, wasmapi.CodeInvalidInput},
		{"未知のキー", []any{"multi_hit"}, map[string]any{"multiHits": map[string]any{"min": 2, "max": 2}}, wasmapi.CodeUnknownField},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCalc()
			mv := sub(t, req, "move")
			mv["mechanisms"] = c.mechs
			mv["mechanismParams"] = c.params
			got := decodeError(t, invoke(t, "calc", mustJSON(t, req)))
			if got.Code != c.wantCode {
				t.Errorf("code = %q, want %q(message=%q)", got.Code, c.wantCode, got.Message)
			}
		})
	}
}

func TestWasmStage1BulkRowsHaveHitRolls(t *testing.T) {
	req := baseBulk()
	mv := sub(t, req, "move")
	mv["mechanisms"] = []any{"multi_hit"}
	mv["mechanismParams"] = map[string]any{"multiHit": map[string]any{"min": 3, "max": 3}}
	resp := invoke(t, "calcBulk", mustJSON(t, req))
	var env struct {
		Result struct {
			Rows []struct {
				Result struct {
					HitRolls *[][16]int `json:"hitRolls"`
				} `json:"result"`
			} `json:"rows"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("JSON でない: %v\n%s", err, resp)
	}
	if len(env.Result.Rows) == 0 {
		t.Fatalf("行が無い: %s", resp)
	}
	for i, row := range env.Result.Rows {
		if row.Result.HitRolls == nil || len(*row.Result.HitRolls) != 3 {
			t.Errorf("rows[%d].result.hitRolls = %v, want 3 発", i, row.Result.HitRolls)
		}
	}
}
