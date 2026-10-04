package master_test

// ADR-0176: 特性の段階1の項目を、特性の効果定義の JSON に持たせる(共通マスタの厳格デコード/正準エンコード)。
//   - "TypeConvert": {"From": <型>, "To": <型>, "PowerMod": <正の整数>}(3つとも必須・From != To・表にあるタイプ)
//   - "PowerMods": [{"Condition": "max_base_power", "MaxPower": <正>, "Modifier": <正・4096 以外>}
//                   | {"Condition": "move_type", "MoveType": <型>, "Modifier": ...}](空不可・同じ要素の重複不可・配列の順を保つ)
//   - "AuraType" + "AuraMod"(組で指定)
//   - "StatMods"(atk/def/spa/spd)・"SeparateStatMods"(atk/spa だけ)
//   - "CritDamageMod"(正の整数)
//   - "PreventsCritical" / "IgnoresOpponentRanks" / "IgnoresDefenderAbility" / "Breakable"(true だけ)
//   - "Breakable" だけの定義は不正(無視される効果が無い)
//   - 持ち物の効果には使えない(未知のフィールド)
//   - 正準形は struct 定義順(…IgnoresParalysisSpeedDrop → TypeConvert → PowerMods → AuraType → AuraMod → StatMods →
//     SeparateStatMods → CritDamageMod → PreventsCritical → IgnoresOpponentRanks → IgnoresDefenderAbility → Breakable →
//     未対応の印)。入れ子のキーも定義順(From → To → PowerMod / Condition → MaxPower → MoveType → Modifier。ゼロ値は省く)。
// データはすべて架空(タイプはテスト用の表の4種)。

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestDecodeAbilityEffectStage1(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
		want engine.AbilityEffect
	}{
		{"タイプ変換", `{"TypeConvert":{"From":"normal","To":"water","PowerMod":4915}}`,
			engine.AbilityEffect{TypeConvert: &engine.TypeConvert{From: "normal", To: "water", PowerMod: 4915}}},
		{"タイプ変換(キーの並びは問わない)", `{"TypeConvert":{"PowerMod":4915,"To":"water","From":"normal"}}`,
			engine.AbilityEffect{TypeConvert: &engine.TypeConvert{From: "normal", To: "water", PowerMod: 4915}}},
		{"威力の条件(威力以下)", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"Modifier":6144}]}`,
			engine.AbilityEffect{PowerMods: []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}}},
		{"威力の条件(技のタイプ)・配列の順を保つ",
			`{"PowerMods":[{"Condition":"move_type","MoveType":"fire","Modifier":6144},{"Condition":"max_base_power","MaxPower":60,"Modifier":5325}]}`,
			engine.AbilityEffect{PowerMods: []engine.ConditionalPowerMod{
				{Condition: engine.PowerConditionMoveType, MoveType: "fire", Modifier: 6144},
				{Condition: engine.PowerConditionMaxBasePower, MaxPower: 60, Modifier: 5325},
			}}},
		{"オーラ", `{"AuraType":"grass","AuraMod":5448}`, engine.AbilityEffect{AuraType: "grass", AuraMod: 5448}},
		{"実数値(攻撃)", `{"StatMods":{"atk":8192}}`, engine.AbilityEffect{StatMods: map[engine.StatKey]int{engine.StatAtk: 8192}}},
		{"実数値(防御)・壊せる", `{"StatMods":{"def":8192},"Breakable":true}`,
			engine.AbilityEffect{StatMods: map[engine.StatKey]int{engine.StatDef: 8192}, Breakable: true}},
		{"単独で丸める実数値", `{"SeparateStatMods":{"atk":6144}}`,
			engine.AbilityEffect{SeparateStatMods: map[engine.StatKey]int{engine.StatAtk: 6144}}},
		{"急所の最終補正", `{"CritDamageMod":6144}`, engine.AbilityEffect{CritDamageMod: 6144}},
		{"急所に当たらない", `{"PreventsCritical":true,"Breakable":true}`, engine.AbilityEffect{PreventsCritical: true, Breakable: true}},
		{"相手のランクを無視", `{"IgnoresOpponentRanks":true,"Breakable":true}`, engine.AbilityEffect{IgnoresOpponentRanks: true, Breakable: true}},
		{"防御側の特性を無視", `{"IgnoresDefenderAbility":true}`, engine.AbilityEffect{IgnoresDefenderAbility: true}},
		{"既存の項目と同居(壊せる半減)", `{"DefResistType":{"fire":2048},"Breakable":true}`,
			engine.AbilityEffect{DefResistType: map[engine.Type]int{"fire": 2048}, Breakable: true}},
		{"未対応の印と同居できる", `{"Breakable":true,"UnsupportedDefender":true}`,
			engine.AbilityEffect{Breakable: true, UnsupportedDefender: true}},
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

func TestDecodeAbilityEffectStage1Rejects(t *testing.T) {
	c := testChart(t)
	over := engine.MaxEffectModifier + 1
	cases := []struct{ name, raw string }{
		{"TypeConvert が null", `{"TypeConvert":null}`},
		{"TypeConvert がオブジェクトでない", `{"TypeConvert":"water"}`},
		{"TypeConvert の To が無い", `{"TypeConvert":{"From":"normal","PowerMod":4915}}`},
		{"TypeConvert の From が無い", `{"TypeConvert":{"To":"water","PowerMod":4915}}`},
		{"TypeConvert の PowerMod が無い", `{"TypeConvert":{"From":"normal","To":"water"}}`},
		{"TypeConvert の From == To", `{"TypeConvert":{"From":"water","To":"water","PowerMod":4915}}`},
		{"TypeConvert の To が表に無い", `{"TypeConvert":{"From":"normal","To":"fairy","PowerMod":4915}}`},
		{"TypeConvert の PowerMod が 0", `{"TypeConvert":{"From":"normal","To":"water","PowerMod":0}}`},
		{"TypeConvert の PowerMod が上限超え", fmt.Sprintf(`{"TypeConvert":{"From":"normal","To":"water","PowerMod":%d}}`, over)},
		{"TypeConvert に未知のキー", `{"TypeConvert":{"From":"normal","To":"water","PowerMod":4915,"Note":"x"}}`},
		{"TypeConvert のキーの大文字小文字違い", `{"TypeConvert":{"from":"normal","To":"water","PowerMod":4915}}`},
		{"項目名の大文字小文字違い", `{"typeConvert":{"From":"normal","To":"water","PowerMod":4915}}`},
		{"PowerMods が空", `{"PowerMods":[]}`},
		{"PowerMods が配列でない", `{"PowerMods":{"Condition":"max_base_power","MaxPower":60,"Modifier":6144}}`},
		{"PowerMods の語彙に無い条件", `{"PowerMods":[{"Condition":"contact","Modifier":5325}]}`},
		{"PowerMods の条件の大文字小文字違い", `{"PowerMods":[{"Condition":"Move_Type","MoveType":"fire","Modifier":6144}]}`},
		{"PowerMods の Modifier が無い", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60}]}`},
		{"PowerMods の Modifier が中立", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"Modifier":4096}]}`},
		{"PowerMods の max_base_power に MaxPower が無い", `{"PowerMods":[{"Condition":"max_base_power","Modifier":6144}]}`},
		{"PowerMods の max_base_power に MoveType がある", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"MoveType":"fire","Modifier":6144}]}`},
		{"PowerMods の move_type に MoveType が無い", `{"PowerMods":[{"Condition":"move_type","Modifier":6144}]}`},
		{"PowerMods の move_type に MaxPower がある", `{"PowerMods":[{"Condition":"move_type","MoveType":"fire","MaxPower":60,"Modifier":6144}]}`},
		{"PowerMods の MoveType が表に無い", `{"PowerMods":[{"Condition":"move_type","MoveType":"steel","Modifier":6144}]}`},
		{"PowerMods の要素に未知のキー", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"Modifier":6144,"Note":"x"}]}`},
		{"PowerMods の同じ要素が2つ", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"Modifier":6144},{"Condition":"max_base_power","MaxPower":60,"Modifier":6144}]}`},
		{"AuraType だけ", `{"AuraType":"grass"}`},
		{"AuraMod だけ", `{"AuraMod":5448}`},
		{"AuraType が表に無い", `{"AuraType":"fairy","AuraMod":5448}`},
		{"StatMods に spe", `{"StatMods":{"spe":8192}}`},
		{"StatMods に hp", `{"StatMods":{"hp":8192}}`},
		{"StatMods が空", `{"StatMods":{}}`},
		{"SeparateStatMods に def", `{"SeparateStatMods":{"def":6144}}`},
		{"SeparateStatMods に spd", `{"SeparateStatMods":{"spd":6144}}`},
		{"SeparateStatMods が空", `{"SeparateStatMods":{}}`},
		{"CritDamageMod が 0", `{"CritDamageMod":0}`},
		{"CritDamageMod が小数", `{"CritDamageMod":6144.5}`},
		{"PreventsCritical が false", `{"PreventsCritical":false}`},
		{"IgnoresOpponentRanks が false", `{"IgnoresOpponentRanks":false}`},
		{"IgnoresDefenderAbility が false", `{"IgnoresDefenderAbility":false}`},
		{"Breakable が false", `{"DefResistType":{"fire":2048},"Breakable":false}`},
		{"Breakable だけ", `{"Breakable":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := master.DecodeAbilityEffect([]byte(tc.raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
	}
}

// 段階1の項目は特性だけのもの(持ち物の効果には使えない)。StatMods は持ち物にもともとある。
func TestDecodeItemEffectRejectsStage1AbilityFields(t *testing.T) {
	c := testChart(t)
	for _, raw := range []string{
		`{"TypeConvert":{"From":"normal","To":"water","PowerMod":4915}}`,
		`{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"Modifier":6144}]}`,
		`{"AuraType":"grass","AuraMod":5448}`,
		`{"SeparateStatMods":{"atk":6144}}`,
		`{"CritDamageMod":6144}`,
		`{"PreventsCritical":true}`,
		`{"IgnoresOpponentRanks":true}`,
		`{"IgnoresDefenderAbility":true}`,
		`{"DamageMod":5324,"Breakable":true}`,
	} {
		if _, err := master.DecodeItemEffect([]byte(raw), c); !errors.Is(err, master.ErrInvalidEffect) {
			t.Errorf("%s: err = %v, want ErrInvalidEffect", raw, err)
		}
	}
}

func TestEncodeAbilityEffectStage1Canonical(t *testing.T) {
	c := testChart(t)
	e := engine.AbilityEffect{
		DefResistType:          map[engine.Type]int{"fire": 2048},
		TypeConvert:            &engine.TypeConvert{From: "normal", To: "water", PowerMod: 4915},
		PowerMods:              []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMoveType, MoveType: "fire", Modifier: 6144}, {Condition: engine.PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}},
		AuraType:               "grass",
		AuraMod:                5448,
		StatMods:               map[engine.StatKey]int{engine.StatDef: 8192, engine.StatAtk: 8192},
		SeparateStatMods:       map[engine.StatKey]int{engine.StatAtk: 6144},
		CritDamageMod:          6144,
		PreventsCritical:       true,
		IgnoresOpponentRanks:   true,
		IgnoresDefenderAbility: true,
		Breakable:              true,
		UnsupportedDefender:    true,
	}
	got, err := master.EncodeAbilityEffect(e)
	if err != nil {
		t.Fatalf("EncodeAbilityEffect: %v", err)
	}
	want := `{"DefResistType":{"fire":2048},` +
		`"TypeConvert":{"From":"normal","To":"water","PowerMod":4915},` +
		`"PowerMods":[{"Condition":"move_type","MoveType":"fire","Modifier":6144},{"Condition":"max_base_power","MaxPower":60,"Modifier":6144}],` +
		`"AuraType":"grass","AuraMod":5448,"StatMods":{"atk":8192,"def":8192},"SeparateStatMods":{"atk":6144},"CritDamageMod":6144,` +
		`"PreventsCritical":true,"IgnoresOpponentRanks":true,"IgnoresDefenderAbility":true,"Breakable":true,"UnsupportedDefender":true}`
	if string(got) != want {
		t.Errorf("Encode =\n%s\nwant\n%s", got, want)
	}
	back, err := master.DecodeAbilityEffect(got, c)
	if err != nil {
		t.Fatalf("Decode(Encode(e)) = %v", err)
	}
	if !reflect.DeepEqual(*back, e) {
		t.Errorf("Decode(Encode(e)) = %+v, want %+v(情報が落ちた)", *back, e)
	}
}

// 段階1の各項目を1つずつ持つ定義も往復で落ちない。
func TestAbilityEffectStage1RoundTripEachField(t *testing.T) {
	c := testChart(t)
	for _, e := range []engine.AbilityEffect{
		{TypeConvert: &engine.TypeConvert{From: "normal", To: "water", PowerMod: 4915}},
		{PowerMods: []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}},
		{AuraType: "grass", AuraMod: 5448},
		{StatMods: map[engine.StatKey]int{engine.StatAtk: 8192}},
		{SeparateStatMods: map[engine.StatKey]int{engine.StatSpA: 6144}},
		{CritDamageMod: 6144},
		{PreventsCritical: true},
		{IgnoresOpponentRanks: true},
		{IgnoresDefenderAbility: true},
	} {
		raw, err := master.EncodeAbilityEffect(e)
		if err != nil {
			t.Errorf("%+v: Encode = %v", e, err)
			continue
		}
		back, err := master.DecodeAbilityEffect(raw, c)
		if err != nil {
			t.Errorf("%s: Decode = %v", raw, err)
			continue
		}
		if !reflect.DeepEqual(*back, e) {
			t.Errorf("%s: 往復 = %+v, want %+v", raw, *back, e)
		}
	}
}
