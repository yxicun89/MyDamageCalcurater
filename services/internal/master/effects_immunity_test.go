package master_test

// P2-3b / ADR-0106 §決定 5: 特性の効果定義に無効(DefImmuneTypes)・吸収(DefAbsorbTypes)を足す。
//
// ここが守るもの:
//   - 往復(Decode(Encode(e)) == e)で情報が落ちない
//   - 正準形(ゼロ値省略・struct 定義順・配列/map のキー昇順)
//   - 厳格デコード(未知のキー・空・不正な値・矛盾を落とす。黙って無視しない)
//   - 吸収の「副次効果なし」({})は正しい値で、トップレベルの「効果が空」の禁止とは別

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestAbilityEffectImmunityRoundTrip(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		e    engine.AbilityEffect
	}{
		{"無効だけ", engine.AbilityEffect{DefImmuneTypes: []engine.Type{"fire"}}},
		{"無効が複数(昇順で戻る)", engine.AbilityEffect{DefImmuneTypes: []engine.Type{"fire", "grass", "water"}}},
		{"吸収(回復)", engine.AbilityEffect{DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{
			"water": {HealNumerator: 1, HealDenominator: 4},
		}}},
		{"吸収(能力上昇)", engine.AbilityEffect{DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{
			"grass": {BoostStat: engine.StatAtk, BoostStages: 1},
		}}},
		{"吸収(副次効果なし)", engine.AbilityEffect{DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{
			"fire": {},
		}}},
		{"無効と吸収が別タイプで同居", engine.AbilityEffect{
			DefImmuneTypes: []engine.Type{"fire"},
			DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{"water": {HealNumerator: 1, HealDenominator: 4}},
		}},
		{"倍率効果と同居", engine.AbilityEffect{
			DefResistType:        map[engine.Type]int{"fire": 2048},
			DefImmuneTypes:       []engine.Type{"grass"},
			DefAbsorbTypes:       map[engine.Type]engine.AbsorbEffect{"water": {BoostStat: engine.StatSpA, BoostStages: 1}},
			ReduceSuperEffective: 3072,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := master.EncodeAbilityEffect(tc.e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := master.DecodeAbilityEffect(raw, c)
			if err != nil {
				t.Fatalf("Decode(%s): %v", raw, err)
			}
			if got == nil || !reflect.DeepEqual(*got, tc.e) {
				t.Fatalf("往復で変わった: %s → %+v, want %+v", raw, got, tc.e)
			}
		})
	}
}

// 正準形: 配列はタイプ ID 昇順、map のキーも昇順、ゼロ値のフィールドは省く。
// 並び順は DefResistType → DefImmuneTypes → DefAbsorbTypes → ReduceSuperEffective(ADR-0106 §決定 5)。
func TestEncodeAbilityEffectImmunityIsCanonical(t *testing.T) {
	cases := []struct {
		name string
		e    engine.AbilityEffect
		want string
	}{
		{"無効はタイプ ID 昇順", engine.AbilityEffect{DefImmuneTypes: []engine.Type{"water", "fire", "grass"}},
			`{"DefImmuneTypes":["fire","grass","water"]}`},
		{"吸収の副次効果なしは空オブジェクト", engine.AbilityEffect{
			DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{"fire": {}}},
			`{"DefAbsorbTypes":{"fire":{}}}`},
		{"吸収の副次効果は struct 定義順", engine.AbilityEffect{
			DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{"water": {HealNumerator: 1, HealDenominator: 4}}},
			`{"DefAbsorbTypes":{"water":{"HealNumerator":1,"HealDenominator":4}}}`},
		{"吸収の map のキーは昇順", engine.AbilityEffect{
			DefAbsorbTypes: map[engine.Type]engine.AbsorbEffect{
				"water": {BoostStat: engine.StatSpA, BoostStages: 1},
				"fire":  {},
			}},
			`{"DefAbsorbTypes":{"fire":{},"water":{"BoostStat":"spa","BoostStages":1}}}`},
		{"フィールドの順序", engine.AbilityEffect{
			DefResistType:        map[engine.Type]int{"ice": 2048},
			DefImmuneTypes:       []engine.Type{"grass"},
			DefAbsorbTypes:       map[engine.Type]engine.AbsorbEffect{"water": {}},
			ReduceSuperEffective: 3072,
		},
			`{"DefResistType":{"ice":2048},"DefImmuneTypes":["grass"],"DefAbsorbTypes":{"water":{}},"ReduceSuperEffective":3072}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.EncodeAbilityEffect(tc.e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Encode = %s, want %s", got, tc.want)
			}
		})
	}
}

// 厳格デコード: 不正な定義を黙って無視しない(無視すると「無効にならない」静かな間違いになる)。
func TestDecodeAbilityEffectImmunityRejects(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
	}{
		{"無効が配列でない", `{"DefImmuneTypes":"fire"}`},
		{"無効が空配列", `{"DefImmuneTypes":[]}`},
		{"無効に表に無いタイプ", `{"DefImmuneTypes":["nosuchtype"]}`},
		{"無効に空文字", `{"DefImmuneTypes":[""]}`},
		{"無効に重複", `{"DefImmuneTypes":["fire","fire"]}`},
		{"無効の要素が文字列でない", `{"DefImmuneTypes":[1]}`},
		{"吸収がオブジェクトでない", `{"DefAbsorbTypes":["water"]}`},
		{"吸収が空オブジェクト", `{"DefAbsorbTypes":{}}`},
		{"吸収に表に無いタイプ", `{"DefAbsorbTypes":{"nosuchtype":{}}}`},
		{"吸収の値がオブジェクトでない", `{"DefAbsorbTypes":{"water":1}}`},
		{"吸収の値に未知のキー", `{"DefAbsorbTypes":{"water":{"Heal":1}}}`},
		{"キーの大文字小文字違い", `{"defImmuneTypes":["fire"]}`},
		{"回復の分子だけ", `{"DefAbsorbTypes":{"water":{"HealNumerator":1}}}`},
		{"回復の分母だけ", `{"DefAbsorbTypes":{"water":{"HealDenominator":4}}}`},
		{"回復の分子が0", `{"DefAbsorbTypes":{"water":{"HealNumerator":0,"HealDenominator":4}}}`},
		{"回復が小数", `{"DefAbsorbTypes":{"water":{"HealNumerator":0.5,"HealDenominator":4}}}`},
		{"回復の分母が範囲外", `{"DefAbsorbTypes":{"water":{"HealNumerator":1,"HealDenominator":17}}}`},
		{"回復が1より大きい", `{"DefAbsorbTypes":{"water":{"HealNumerator":5,"HealDenominator":4}}}`},
		{"上げる能力だけで段階が無い", `{"DefAbsorbTypes":{"grass":{"BoostStat":"atk"}}}`},
		{"段階だけで能力が無い", `{"DefAbsorbTypes":{"grass":{"BoostStages":1}}}`},
		{"上げる能力が hp", `{"DefAbsorbTypes":{"grass":{"BoostStat":"hp","BoostStages":1}}}`},
		{"上げる能力が不正", `{"DefAbsorbTypes":{"grass":{"BoostStat":"atkk","BoostStages":1}}}`},
		{"段階が0", `{"DefAbsorbTypes":{"grass":{"BoostStat":"atk","BoostStages":0}}}`},
		{"段階が7", `{"DefAbsorbTypes":{"grass":{"BoostStat":"atk","BoostStages":7}}}`},
		{"段階が負", `{"DefAbsorbTypes":{"grass":{"BoostStat":"atk","BoostStages":-1}}}`},
		{"同じタイプが無効と吸収の両方", `{"DefImmuneTypes":["water"],"DefAbsorbTypes":{"water":{}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.DecodeAbilityEffect([]byte(tc.raw), c)
			if err == nil {
				t.Fatalf("不正な定義を受け入れた: %s → %+v", tc.raw, got)
			}
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
	}
}

// 吸収の副次効果なし({})は正しい値。トップレベルの「効果が空」の禁止(ADR-0100 §6)と混ざらないこと。
func TestDecodeAbilityEffectAbsorbWithoutSideEffect(t *testing.T) {
	c := testChart(t)
	got, err := master.DecodeAbilityEffect([]byte(`{"DefAbsorbTypes":{"fire":{}}}`), c)
	if err != nil {
		t.Fatalf("副次効果なしの吸収を拒否した: %v", err)
	}
	want := map[engine.Type]engine.AbsorbEffect{"fire": {}}
	if got == nil || !reflect.DeepEqual(got.DefAbsorbTypes, want) {
		t.Errorf("DefAbsorbTypes = %+v, want %+v", got, want)
	}
}
