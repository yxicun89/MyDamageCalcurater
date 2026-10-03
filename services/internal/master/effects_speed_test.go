package master_test

// ADR-0139: 素早さの補正(SpeedMods・IgnoresParalysisSpeedDrop)を、持ち物・特性の効果定義の JSON に持たせる。
//   - 形: "SpeedMods": [{"Condition": <閉じた語彙>, "Modifier": <4096 基準の正の整数>}, ...]
//     特性だけ "IgnoresParalysisSpeedDrop": true(まひの素早さ半減を受けない)。
//   - 配列の順は評価の優先順(成立した最初の要素だけを掛ける)なので、正準形でも並べ替えない。
//   - ダメージ計算は読まない(engine の Rolls も未対応の印も変わらない)。
//   - 素早さの補正は SpeedMods だけで表す(StatMods の spe は拒否。表し方を1つにする)。
// データはすべて架空。

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestDecodeItemEffectSpeedMods(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
		want engine.ItemEffect
	}{
		{"常時", `{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`,
			engine.ItemEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}},
		{"キーの並びは問わない", `{"SpeedMods":[{"Modifier":2048,"Condition":"always"}]}`,
			engine.ItemEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}},
		{"未対応の印と同居できる", `{"SpeedMods":[{"Condition":"always","Modifier":2048}],"UnsupportedDefender":true}`,
			engine.ItemEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}, UnsupportedDefender: true}},
		{"ダメージの補正と同居できる", `{"DamageMod":5324,"SpeedMods":[{"Condition":"weather_rain","Modifier":6144}]}`,
			engine.ItemEffect{DamageMod: 5324, SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherRain, Modifier: 6144}}}},
		{"配列の順を保つ(評価の優先順)", `{"SpeedMods":[{"Condition":"weather_sun","Modifier":8192},{"Condition":"always","Modifier":6144}]}`,
			engine.ItemEffect{SpeedMods: []engine.SpeedMod{
				{Condition: engine.SpeedConditionWeatherSun, Modifier: 8192},
				{Condition: engine.SpeedConditionAlways, Modifier: 6144},
			}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.DecodeItemEffect([]byte(tc.raw), c)
			if err != nil {
				t.Fatalf("DecodeItemEffect = %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Errorf("got %+v, want %+v", *got, tc.want)
			}
		})
	}
}

func TestDecodeAbilityEffectSpeedMods(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
		want engine.AbilityEffect
	}{
		{"天候", `{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`,
			engine.AbilityEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherRain, Modifier: 8192}}}},
		{"場", `{"SpeedMods":[{"Condition":"terrain_electric","Modifier":8192}]}`,
			engine.AbilityEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionTerrainElectric, Modifier: 8192}}}},
		{"持ち物を失った後", `{"SpeedMods":[{"Condition":"item_lost","Modifier":8192}]}`,
			engine.AbilityEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionItemLost, Modifier: 8192}}}},
		{"状態異常 + まひの半減を受けない", `{"SpeedMods":[{"Condition":"has_status","Modifier":6144}],"IgnoresParalysisSpeedDrop":true}`,
			engine.AbilityEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionHasStatus, Modifier: 6144}}, IgnoresParalysisSpeedDrop: true}},
		{"まひの半減を受けないだけ", `{"IgnoresParalysisSpeedDrop":true}`,
			engine.AbilityEffect{IgnoresParalysisSpeedDrop: true}},
		{"防御の補正と同居できる", `{"DefResistType":{"fire":2048},"SpeedMods":[{"Condition":"weather_sand","Modifier":8192}]}`,
			engine.AbilityEffect{DefResistType: map[engine.Type]int{engine.TypeFire: 2048},
				SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherSand, Modifier: 8192}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.DecodeAbilityEffect([]byte(tc.raw), c)
			if err != nil {
				t.Fatalf("DecodeAbilityEffect = %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Errorf("got %+v, want %+v", *got, tc.want)
			}
		})
	}
}

// 語彙のすべての値を、特性では受け付ける(item_lost 以外は持ち物でも受け付ける)。
func TestDecodeSpeedModsAcceptsEveryCondition(t *testing.T) {
	c := testChart(t)
	for _, cond := range engine.AllSpeedConditions() {
		raw := fmt.Sprintf(`{"SpeedMods":[{"Condition":%q,"Modifier":8192}]}`, cond)
		if _, err := master.DecodeAbilityEffect([]byte(raw), c); err != nil {
			t.Errorf("特性 %s: %v", cond, err)
		}
		_, err := master.DecodeItemEffect([]byte(raw), c)
		if cond == engine.SpeedConditionItemLost {
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("持ち物 %s: err = %v, want ErrInvalidEffect(持ち物を失った後に持ち物の補正は無い)", cond, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("持ち物 %s: %v", cond, err)
		}
	}
}

func TestDecodeSpeedModsRejects(t *testing.T) {
	c := testChart(t)
	over := engine.MaxEffectModifier + 1
	common := []struct{ name, raw string }{
		{"null", `{"SpeedMods":null}`},
		{"配列でない", `{"SpeedMods":{"Condition":"always","Modifier":2048}}`},
		{"空の配列", `{"SpeedMods":[]}`},
		{"要素がオブジェクトでない", `{"SpeedMods":[2048]}`},
		{"要素が null", `{"SpeedMods":[null]}`},
		{"Condition が無い", `{"SpeedMods":[{"Modifier":2048}]}`},
		{"Modifier が無い", `{"SpeedMods":[{"Condition":"always"}]}`},
		{"要素に未知のキー", `{"SpeedMods":[{"Condition":"always","Modifier":2048,"Note":"x"}]}`},
		{"要素のキーの大文字小文字違い", `{"SpeedMods":[{"condition":"always","Modifier":2048}]}`},
		{"項目名の大文字小文字違い", `{"speedMods":[{"Condition":"always","Modifier":2048}]}`},
		{"語彙に無い条件", `{"SpeedMods":[{"Condition":"weather_hail","Modifier":2048}]}`},
		{"条件の大文字小文字違い", `{"SpeedMods":[{"Condition":"Always","Modifier":2048}]}`},
		{"条件が空文字", `{"SpeedMods":[{"Condition":"","Modifier":2048}]}`},
		{"条件が文字列でない", `{"SpeedMods":[{"Condition":1,"Modifier":2048}]}`},
		{"Modifier が 0", `{"SpeedMods":[{"Condition":"always","Modifier":0}]}`},
		{"Modifier が負", `{"SpeedMods":[{"Condition":"always","Modifier":-2048}]}`},
		{"Modifier が小数", `{"SpeedMods":[{"Condition":"always","Modifier":2048.5}]}`},
		{"Modifier が文字列", `{"SpeedMods":[{"Condition":"always","Modifier":"2048"}]}`},
		{"Modifier が中立(4096 は補正なしと区別できない)", `{"SpeedMods":[{"Condition":"always","Modifier":4096}]}`},
		{"Modifier が上限を超える", fmt.Sprintf(`{"SpeedMods":[{"Condition":"always","Modifier":%d}]}`, over)},
		{"同じ条件が2つ", `{"SpeedMods":[{"Condition":"always","Modifier":2048},{"Condition":"always","Modifier":8192}]}`},
	}
	for _, tc := range common {
		t.Run("持ち物/"+tc.name, func(t *testing.T) {
			if _, err := master.DecodeItemEffect([]byte(tc.raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
		t.Run("特性/"+tc.name, func(t *testing.T) {
			if _, err := master.DecodeAbilityEffect([]byte(tc.raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
	}

	itemOnly := []struct{ name, raw string }{
		// まひの半減を受けないのは特性の性質(持ち物の項目ではない)。
		{"持ち物に IgnoresParalysisSpeedDrop", `{"IgnoresParalysisSpeedDrop":true}`},
		// 素早さの補正は SpeedMods だけで表す(StatMods の spe と2通りに書けないようにする)。
		{"StatMods の spe", `{"StatMods":{"spe":6144}}`},
		{"StatMods の spe と他の能力", `{"StatMods":{"atk":6144,"spe":6144}}`},
	}
	for _, tc := range itemOnly {
		t.Run("持ち物/"+tc.name, func(t *testing.T) {
			if _, err := master.DecodeItemEffect([]byte(tc.raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
	}

	abilityOnly := []struct{ name, raw string }{
		{"IgnoresParalysisSpeedDrop が false", `{"IgnoresParalysisSpeedDrop":false}`},
		{"IgnoresParalysisSpeedDrop が文字列", `{"IgnoresParalysisSpeedDrop":"true"}`},
	}
	for _, tc := range abilityOnly {
		t.Run("特性/"+tc.name, func(t *testing.T) {
			if _, err := master.DecodeAbilityEffect([]byte(tc.raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
	}
}

// 正準形: struct 定義順(持ち物は ResistBerryType の後・未対応の印の前、特性は Airborne の後に
// SpeedMods → IgnoresParalysisSpeedDrop → 未対応の印)。要素のキーは Condition → Modifier。配列は並べ替えない。
func TestEncodeSpeedEffectsCanonical(t *testing.T) {
	c := testChart(t)
	t.Run("持ち物", func(t *testing.T) {
		cases := []struct {
			effect engine.ItemEffect
			want   string
		}{
			{engine.ItemEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}},
				`{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`},
			{engine.ItemEffect{ResistBerryType: engine.TypeFire, SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}, UnsupportedDefender: true},
				`{"ResistBerryType":"fire","SpeedMods":[{"Condition":"always","Modifier":2048}],"UnsupportedDefender":true}`},
			{engine.ItemEffect{SpeedMods: []engine.SpeedMod{
				{Condition: engine.SpeedConditionWeatherSun, Modifier: 8192},
				{Condition: engine.SpeedConditionAlways, Modifier: 6144},
			}}, `{"SpeedMods":[{"Condition":"weather_sun","Modifier":8192},{"Condition":"always","Modifier":6144}]}`},
		}
		for _, tc := range cases {
			got, err := master.EncodeItemEffect(tc.effect)
			if err != nil || string(got) != tc.want {
				t.Errorf("EncodeItemEffect = %s, %v; want %s", got, err, tc.want)
				continue
			}
			back, err := master.DecodeItemEffect(got, c)
			if err != nil || !reflect.DeepEqual(*back, tc.effect) {
				t.Errorf("Decode(Encode(e)) = %+v, %v; want %+v", back, err, tc.effect)
			}
		}
	})
	t.Run("特性", func(t *testing.T) {
		cases := []struct {
			effect engine.AbilityEffect
			want   string
		}{
			{engine.AbilityEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherRain, Modifier: 8192}}},
				`{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`},
			{engine.AbilityEffect{IgnoresParalysisSpeedDrop: true}, `{"IgnoresParalysisSpeedDrop":true}`},
			{engine.AbilityEffect{
				Airborne:                  true,
				SpeedMods:                 []engine.SpeedMod{{Condition: engine.SpeedConditionHasStatus, Modifier: 6144}},
				IgnoresParalysisSpeedDrop: true,
				UnsupportedAttacker:       true,
			}, `{"Airborne":true,"SpeedMods":[{"Condition":"has_status","Modifier":6144}],"IgnoresParalysisSpeedDrop":true,"UnsupportedAttacker":true}`},
		}
		for _, tc := range cases {
			got, err := master.EncodeAbilityEffect(tc.effect)
			if err != nil || string(got) != tc.want {
				t.Errorf("EncodeAbilityEffect = %s, %v; want %s", got, err, tc.want)
				continue
			}
			back, err := master.DecodeAbilityEffect(got, c)
			if err != nil || !reflect.DeepEqual(*back, tc.effect) {
				t.Errorf("Decode(Encode(e)) = %+v, %v; want %+v", back, err, tc.effect)
			}
		}
	})
}

// 既存の効果(素早さの項目なし)の正準形は変わらない(取り込み済みのデータ・dataVersion を動かさない)。
func TestEncodeWithoutSpeedEffectsIsUnchanged(t *testing.T) {
	item, err := master.EncodeItemEffect(engine.ItemEffect{DamageMod: 5324, UnsupportedDefender: true})
	if err != nil || string(item) != `{"DamageMod":5324,"UnsupportedDefender":true}` {
		t.Errorf("EncodeItemEffect = %s, %v", item, err)
	}
	ab, err := master.EncodeAbilityEffect(engine.AbilityEffect{IgnoresBurn: true, Airborne: true})
	if err != nil || string(ab) != `{"IgnoresBurn":true,"Airborne":true}` {
		t.Errorf("EncodeAbilityEffect = %s, %v", ab, err)
	}
}

// 素早さの項目はダメージ計算に効かない(engine の Rolls も未対応の印も変わらない。ゴールデン不変の根拠)。
// 持ち物は TestItemRolesMatchEngineDamage の SpeedMods の行でも確かめる。ここでは特性を両側に持たせる。
func TestSpeedEffectsDoNotChangeDamage(t *testing.T) {
	speedOnly := &engine.AbilityEffect{
		SpeedMods: []engine.SpeedMod{
			{Condition: engine.SpeedConditionAlways, Modifier: 8192},
			{Condition: engine.SpeedConditionHasStatus, Modifier: 6144},
		},
		IgnoresParalysisSpeedDrop: true,
	}
	ability := engine.Ability{ID: "testspeedability", NameJa: "テストすばやさとくせい", Effect: speedOnly}
	item := &engine.Item{ID: "testspeeditem", NameJa: "テストすばやさどうぐ", Effect: &engine.ItemEffect{
		SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}},
	}}
	for i, base := range probeInputs(t) {
		without, err := engine.CalcDamage(base)
		if err != nil {
			t.Fatalf("CalcDamage(なし): %v", err)
		}
		for _, side := range []string{"attacker", "defender"} {
			with := base
			if side == "attacker" {
				with.Attacker.Ability, with.Attacker.Item = ability, item
			} else {
				with.Defender.Ability, with.Defender.Item = ability, item
			}
			got, err := engine.CalcDamage(with)
			if err != nil {
				t.Fatalf("CalcDamage(%s): %v", side, err)
			}
			if got.Rolls != without.Rolls {
				t.Errorf("入力 %d・%s: 素早さの項目で Rolls が変わった", i, side)
			}
			if !slices.Equal(got.Unsupported, without.Unsupported) {
				t.Errorf("入力 %d・%s: 素早さの項目で未対応の印が変わった: %+v", i, side, got.Unsupported)
			}
		}
	}
}
