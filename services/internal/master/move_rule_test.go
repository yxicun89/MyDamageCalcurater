package master_test

// 技の処理の定義(effects.json の moveRules・move_rules。ADR-0143 §4)のデコード・エンコードと engine.Move.Rule への写像、
// 特性の効果 WeightMod、種族の重さ(species.weight_hg)の写像。
//
// 受け入れ条件:
//   - DecodeMoveRule は effects と同じ流儀で厳格(未知のキー・空のオブジェクト・false・語彙に無い値・相性表に無いタイプは ErrInvalidEffect)。
//     EncodeMoveRule → DecodeMoveRule の往復で同じ値(正準形)。
//   - Move は MoveRow.Rule を検証し(機構との対応は engine.Move.ValidateRule。違反は ErrInvalidRow)、engine.Move.Rule に写す。
//     Rule が nil なら engine.Move.Rule も nil。
//   - Species は SpeciesRow.WeightHg を engine.Species.WeightHg に写す(0 は不明のまま。負は ErrInvalidRow)。
//   - DecodeAbilityEffect は WeightMod を受け、値域外を拒否する。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestDecodeMoveRule(t *testing.T) {
	chart := testChart(t)
	cases := []struct {
		raw  string
		want engine.MoveRule
	}{
		{`{"PowerFormula":"target_weight","MoveSpecificResolved":true}`,
			engine.MoveRule{PowerFormula: engine.PowerFormulaTargetWeight, MoveSpecificResolved: true}},
		{`{"PowerBoosts":[{"Condition":"attacker_status","Statuses":["burn","paralysis"],"Modifier":8192}],"IgnoresBurn":true}`,
			engine.MoveRule{PowerBoosts: []engine.MovePowerBoost{{Condition: engine.MoveConditionAttackerStatus,
				Statuses: []engine.Status{engine.StatusBurn, engine.StatusParalysis}, Modifier: 8192}}, IgnoresBurn: true}},
		{`{"PowerBoosts":[{"Condition":"weather","Weathers":["sun","rain"],"BaseMultiplier":2}],"TypeByWeather":{"rain":"water","sun":"fire"},"MoveSpecificResolved":true}`,
			engine.MoveRule{
				PowerBoosts:          []engine.MovePowerBoost{{Condition: engine.MoveConditionWeather, Weathers: []engine.Weather{engine.WeatherSun, engine.WeatherRain}, BaseMultiplier: 2}},
				TypeByWeather:        map[engine.Weather]engine.Type{engine.WeatherRain: engine.TypeWater, engine.WeatherSun: engine.TypeFire},
				MoveSpecificResolved: true,
			}},
		{`{"PowerBoosts":[{"Condition":"terrain_attacker_grounded","Terrains":["grassy"],"BaseMultiplier":2}],"TypeByTerrain":{"grassy":"grass"},"SpreadInTerrain":"grassy","MoveSpecificResolved":true}`,
			engine.MoveRule{
				PowerBoosts:          []engine.MovePowerBoost{{Condition: engine.MoveConditionTerrainAttackerGrounded, Terrains: []engine.Terrain{engine.TerrainGrassy}, BaseMultiplier: 2}},
				TypeByTerrain:        map[engine.Terrain]engine.Type{engine.TerrainGrassy: engine.TypeGrass},
				SpreadInTerrain:      engine.TerrainGrassy,
				MoveSpecificResolved: true,
			}},
		{`{"TerrainPowerMods":[{"Terrain":"grassy","Modifier":2048}]}`,
			engine.MoveRule{TerrainPowerMods: []engine.TerrainPowerMod{{Terrain: engine.TerrainGrassy, Modifier: 2048}}}},
		{`{"ExtraEffectivenessType":"fire"}`, engine.MoveRule{ExtraEffectivenessType: engine.TypeFire}},
		{`{"SuperEffectiveAgainst":["water"]}`, engine.MoveRule{SuperEffectiveAgainst: []engine.Type{engine.TypeWater}}},
		{`{"PriorityBoost":{"Terrain":"grassy","Delta":1}}`, engine.MoveRule{PriorityBoost: &engine.PriorityBoost{Terrain: engine.TerrainGrassy, Delta: 1}}},
		{`{"BreaksScreens":true,"FailsWithoutDefenderItem":true,"MoveSpecificResolved":true}`,
			engine.MoveRule{BreaksScreens: true, FailsWithoutDefenderItem: true, MoveSpecificResolved: true}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := master.DecodeMoveRule([]byte(tc.raw), chart)
			if err != nil {
				t.Fatalf("DecodeMoveRule: %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("got %+v, want %+v", *got, tc.want)
			}
			raw, err := master.EncodeMoveRule(*got)
			if err != nil {
				t.Fatalf("EncodeMoveRule: %v", err)
			}
			again, err := master.DecodeMoveRule(raw, chart)
			if err != nil || !reflect.DeepEqual(*again, tc.want) {
				t.Fatalf("往復: %s → %+v (%v), want %+v", raw, again, err, tc.want)
			}
			// 正準形はもう一度書き出しても同じバイト列。
			if raw2, _ := master.EncodeMoveRule(*again); string(raw2) != string(raw) {
				t.Errorf("正準形が安定しない: %s / %s", raw, raw2)
			}
		})
	}
}

func TestDecodeMoveRuleRejects(t *testing.T) {
	chart := testChart(t)
	for _, raw := range []string{
		`{}`,
		`null`,
		`[]`,
		`{"Unknown":1}`,
		`{"powerFormula":"target_weight"}`, // キーは Go のフィールド名(大文字小文字を区別する)
		`{"PowerFormula":"hp_ratio"}`,
		`{"PowerFormula":""}`,
		`{"IgnoresBurn":false}`,
		`{"MoveSpecificResolved":1}`,
		`{"PowerBoosts":[]}`,
		`{"PowerBoosts":[{"Condition":"attacker_hp","BaseMultiplier":2}]}`,
		`{"PowerBoosts":[{"Condition":"defender_status","Statuses":["toxic"],"BaseMultiplier":2}]}`,
		`{"PowerBoosts":[{"Condition":"weather","Weathers":["fog"],"BaseMultiplier":2}]}`,
		`{"PowerBoosts":[{"Condition":"attacker_no_item","BaseMultiplier":2,"Modifier":6144}]}`,
		`{"PowerBoosts":[{"Condition":"attacker_no_item","Modifier":4096}]}`,
		`{"TypeByWeather":{"rain":"shadow"}}`,
		`{"TypeByWeather":{}}`,
		`{"TypeByTerrain":{"volcanic":"fire"}}`,
		`{"ExtraEffectivenessType":"shadow"}`,
		`{"SuperEffectiveAgainst":["water","water"]}`,
		`{"PriorityBoost":{"Terrain":"grassy","Delta":0}}`,
		`{"TerrainPowerMods":[{"Terrain":"none","Modifier":2048}]}`,
		`{"SpreadInTerrain":"none","MoveSpecificResolved":true}`,
	} {
		if _, err := master.DecodeMoveRule([]byte(raw), chart); !errors.Is(err, master.ErrInvalidEffect) {
			t.Errorf("%s: err = %v, want ErrInvalidEffect", raw, err)
		}
	}
}

func ruleRow(mechs []string, rule string) master.MoveRow {
	row := testMoveRow()
	row.Category = "physical"
	row.Mechanisms = mechs
	if rule != "" {
		row.Rule = []byte(rule)
	}
	return row
}

func TestMoveMapsRule(t *testing.T) {
	chart := testChart(t)
	t.Run("定義なしは nil", func(t *testing.T) {
		got, err := master.Move(ruleRow([]string{"variable_power"}, ""), chart)
		if err != nil {
			t.Fatalf("Move: %v", err)
		}
		if got.Rule != nil {
			t.Fatalf("Rule = %+v, want nil", got.Rule)
		}
	})
	t.Run("定義を写す", func(t *testing.T) {
		got, err := master.Move(ruleRow([]string{"move_specific", "variable_power"}, `{"PowerFormula":"weight_ratio","MoveSpecificResolved":true}`), chart)
		if err != nil {
			t.Fatalf("Move: %v", err)
		}
		want := &engine.MoveRule{PowerFormula: engine.PowerFormulaWeightRatio, MoveSpecificResolved: true}
		if !reflect.DeepEqual(got.Rule, want) {
			t.Fatalf("Rule = %+v, want %+v", got.Rule, want)
		}
	})
	for _, tc := range []struct {
		name string
		row  master.MoveRow
	}{
		{"機構との対応が無い", ruleRow([]string{"type_change"}, `{"PowerFormula":"speed_ratio"}`)},
		{"hit_index に多段の中身が無い", ruleRow([]string{"variable_power"}, `{"PowerFormula":"hit_index"}`)},
		{"デコードできない", ruleRow([]string{"variable_power"}, `{"PowerFormula":"hp_ratio"}`)},
		{"変化技の定義", func() master.MoveRow {
			r := ruleRow(nil, `{"MoveSpecificResolved":true}`)
			r.Category, r.Power = "status", 0
			return r
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := master.Move(tc.row, chart); !errors.Is(err, master.ErrInvalidRow) {
				t.Fatalf("err = %v, want ErrInvalidRow", err)
			}
		})
	}
}

func TestSpeciesMapsWeight(t *testing.T) {
	chart := testChart(t)
	abilities := []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testguard"}}
	for _, tc := range []struct {
		name string
		hg   int
	}{{"重さあり", 905}, {"不明(0)", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			row := baseSpeciesRow()
			row.WeightHg = tc.hg
			got, err := master.Species(row, abilities, chart)
			if err != nil {
				t.Fatalf("Species: %v", err)
			}
			if got.WeightHg != tc.hg {
				t.Fatalf("WeightHg = %d, want %d", got.WeightHg, tc.hg)
			}
		})
	}
	row := baseSpeciesRow()
	row.WeightHg = -1
	if _, err := master.Species(row, abilities, chart); !errors.Is(err, master.ErrInvalidRow) {
		t.Fatalf("負の重さ: err = %v, want ErrInvalidRow", err)
	}
}

func TestDecodeAbilityEffectWeightMod(t *testing.T) {
	chart := testChart(t)
	got, err := master.DecodeAbilityEffect([]byte(`{"WeightMod":8192,"Breakable":true}`), chart)
	if err != nil {
		t.Fatalf("DecodeAbilityEffect: %v", err)
	}
	if want := (engine.AbilityEffect{WeightMod: 8192, Breakable: true}); !reflect.DeepEqual(*got, want) {
		t.Fatalf("got %+v, want %+v", *got, want)
	}
	raw, err := master.EncodeAbilityEffect(*got)
	if err != nil {
		t.Fatalf("EncodeAbilityEffect: %v", err)
	}
	if again, err := master.DecodeAbilityEffect(raw, chart); err != nil || !reflect.DeepEqual(*again, *got) {
		t.Fatalf("往復: %s → %+v (%v)", raw, again, err)
	}
	for _, raw := range []string{`{"WeightMod":0}`, `{"WeightMod":-1}`, `{"WeightMod":99999999}`, `{"WeightMod":1.5}`} {
		if _, err := master.DecodeAbilityEffect([]byte(raw), chart); !errors.Is(err, master.ErrInvalidEffect) {
			t.Errorf("%s: err = %v, want ErrInvalidEffect", raw, err)
		}
	}
}
