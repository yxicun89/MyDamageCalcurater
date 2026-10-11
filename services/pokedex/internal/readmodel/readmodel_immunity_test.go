package readmodel_test

// P2-3b / ADR-0106 §決定 7: 無効・吸収を balance の read model(ADR-0017 §2)に出す。
//
// ADR-0105 §限界 1「特性の immune / absorb が export に出ない」の解消。
// 新しい kind は足さない: ADR-0017 §2 の immune / absorb をそのまま使い、
// services/balance/schema/abilities.schema.json の enum に既にある値に写す。
//
// ここが守るもの:
//   - DefImmuneTypes → {"kind":"immune","attackType":t} / DefAbsorbTypes → {"kind":"absorb","attackType":t}
//   - 並び順: immune(タイプ昇順)→ absorb(タイプ昇順)→ type_multiplier(タイプ昇順)→ super_effective_multiplier
//   - immune / absorb の行に numerator / denominator を書かない(schema が存在自体を拒否する)
//   - 吸収の副次効果(回復・能力上昇)は出さない(ADR-0017 §2)

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// abilityEffectsFor は export した abilities.json から1つの特性の effects を取り出す。
func abilityEffectsFor(t *testing.T, raw []byte, abilityID string) []abilityEffect {
	t.Helper()
	var f abilitiesFile
	strict(t, raw, &f)
	for _, a := range f.Abilities {
		if a.AbilityID == abilityID {
			return a.Effects
		}
	}
	t.Fatalf("特性 %s が read model に無い", abilityID)
	return nil
}

func TestExportAbilityImmunityAndAbsorb(t *testing.T) {
	q := storetest.New()
	q.AbilityEffects = append(q.AbilityEffects,
		// ふゆう相当: 地面を無効にするだけ。
		store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(`{"DefImmuneTypes":["grass"]}`)},
		// ちょすい相当: 水を吸収して回復する。副次効果は read model に出さない。
		store.AbilityEffect{AbilityID: "testspecial", Effect: json.RawMessage(
			`{"DefAbsorbTypes":{"water":{"HealNumerator":1,"HealDenominator":4}}}`)},
		// 無効・吸収・倍率・抜群軽減が1つの特性に同居する。
		store.AbilityEffect{AbilityID: "teststance", Effect: json.RawMessage(
			`{"DefResistType":{"normal":2048},"DefImmuneTypes":["fire","grass"],` +
				`"DefAbsorbTypes":{"water":{"BoostStat":"atk","BoostStages":1}},"ReduceSuperEffective":3072}`)},
	)
	files, _ := export(t, q)

	want := map[string][]abilityEffect{
		"testhidden":  {{Kind: "immune", AttackType: "grass"}},
		"testspecial": {{Kind: "absorb", AttackType: "water"}},
		"teststance": {
			{Kind: "immune", AttackType: "fire"},
			{Kind: "immune", AttackType: "grass"},
			{Kind: "absorb", AttackType: "water"},
			{Kind: "type_multiplier", AttackType: "normal", Numerator: 1, Denominator: 2},
			{Kind: "super_effective_multiplier", Numerator: 3, Denominator: 4},
		},
	}
	for id, w := range want {
		got := abilityEffectsFor(t, files.Abilities, id)
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s の effects = %+v, want %+v", id, got, w)
		}
	}

	// 無効・吸収を含む出力も services/balance/schema/abilities.schema.json に合うこと
	// (kind の enum に immune / absorb は既にある。ADR-0017 §2・ADR-0402)。
	if err := validate(t, compileSchema(t, "abilities.schema.json"), files.Abilities); err != nil {
		t.Fatalf("abilities.schema.json に合わない: %v\n%s", err, files.Abilities)
	}
}

// immune / absorb の行に numerator / denominator を書かない。
// abilities.schema.json は factor の minimum を 1 にしており、immune / absorb では
// numerator / denominator が「在ること」自体を拒否する(0 を書くと schema 検証で落ちる)。
func TestExportImmunityOmitsFactors(t *testing.T) {
	q := storetest.New()
	q.AbilityEffects = append(q.AbilityEffects,
		store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(`{"DefImmuneTypes":["grass"]}`)},
		store.AbilityEffect{AbilityID: "testspecial", Effect: json.RawMessage(`{"DefAbsorbTypes":{"water":{}}}`)},
	)
	files, _ := export(t, q)

	if bytes.Contains(files.Abilities, []byte(`"numerator":0`)) ||
		bytes.Contains(files.Abilities, []byte(`"denominator":0`)) {
		t.Errorf("immune / absorb の行に分子・分母が 0 で出ている(effectEntry に omitempty を付ける):\n%s", files.Abilities)
	}

	// 生の JSON でも immune / absorb の行にキーが無いことを確かめる(omitempty の取りこぼし防止)。
	var f struct {
		Abilities []struct {
			AbilityID string                       `json:"abilityId"`
			Effects   []map[string]json.RawMessage `json:"effects"`
		} `json:"abilities"`
	}
	if err := json.Unmarshal(files.Abilities, &f); err != nil {
		t.Fatal(err)
	}
	for _, a := range f.Abilities {
		for _, e := range a.Effects {
			kind := string(bytes.Trim(e["kind"], `"`))
			if kind != "immune" && kind != "absorb" {
				continue
			}
			for _, k := range []string{"numerator", "denominator"} {
				if _, ok := e[k]; ok {
					t.Errorf("%s の %s の行に %s がある: %v", a.AbilityID, kind, k, e)
				}
			}
			if _, ok := e["attackType"]; !ok {
				t.Errorf("%s の %s の行に attackType が無い", a.AbilityID, kind)
			}
		}
	}
}

// 効果定義が不正なら export を失敗させる(黙って落とさない)。
func TestExportRejectsInvalidImmunityEffect(t *testing.T) {
	cases := []struct {
		name   string
		effect string
	}{
		{"表に無いタイプの無効", `{"DefImmuneTypes":["nosuchtype"]}`},
		{"空の無効", `{"DefImmuneTypes":[]}`},
		{"同じタイプが無効と吸収の両方", `{"DefImmuneTypes":["water"],"DefAbsorbTypes":{"water":{}}}`},
		{"吸収の値に未知のキー", `{"DefAbsorbTypes":{"water":{"Heal":1}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := storetest.New()
			q.AbilityEffects = append(q.AbilityEffects,
				store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(tc.effect)})
			if _, _, err := readmodel.Export(context.Background(), q); err == nil {
				t.Fatal("不正な効果定義で export が成功した")
			} else if !errors.Is(err, readmodel.ErrInvalidExport) {
				t.Errorf("err = %v, want ErrInvalidExport", err)
			}
		})
	}
}

// ADR-0145: このイメージが知らないトップレベルのフィールドは読み捨てる(古いイメージが落ちない)。
// 出力の JSON は未知のフィールドが無いときと同じバイト列で、読み捨てた名前は Report にだけ出る。
func TestExportIgnoresUnknownEffectFields(t *testing.T) {
	withEffect := func(effect string) *storetest.Querier {
		q := storetest.New()
		q.AbilityEffects = append(q.AbilityEffects,
			store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(effect)})
		return q
	}
	plain, plainRep := export(t, withEffect(`{"DefImmuneTypes":["grass"]}`))
	if len(plainRep.IgnoredEffectFields) != 0 {
		t.Fatalf("未知のフィールドが無いのに IgnoredEffectFields = %+v", plainRep.IgnoredEffectFields)
	}
	files, rep := export(t, withEffect(`{"DefImmuneTypes":["grass"],"FutureField":{"x":1},"AnotherField":true}`))
	if !bytes.Equal(files.Abilities, plain.Abilities) || !bytes.Equal(files.Metadata, plain.Metadata) ||
		!bytes.Equal(files.PokemonTypes, plain.PokemonTypes) || !bytes.Equal(files.SpeedPokemon, plain.SpeedPokemon) {
		t.Error("未知のフィールドがあると read model の出力が変わった")
	}
	want := []readmodel.IgnoredEffectField{{Field: "AnotherField", Count: 1}, {Field: "FutureField", Count: 1}}
	if !reflect.DeepEqual(rep.IgnoredEffectFields, want) {
		t.Errorf("IgnoredEffectFields = %+v, want %+v", rep.IgnoredEffectFields, want)
	}
	// 未知のフィールドだけの定義は、防御側の効果なし(空配列)として成功する。
	only, onlyRep := export(t, withEffect(`{"FutureField":1}`))
	if got := abilityEffectsFor(t, only.Abilities, "testhidden"); len(got) != 0 {
		t.Errorf("未知のフィールドだけの特性の effects = %+v, want 空", got)
	}
	if len(onlyRep.IgnoredEffectFields) != 1 {
		t.Errorf("IgnoredEffectFields = %+v", onlyRep.IgnoredEffectFields)
	}
}

// 未知のフィールドを許しても、知っているフィールドの検証は厳格なまま。
func TestExportStillRejectsBrokenKnownFieldsAlongsideUnknown(t *testing.T) {
	for _, effect := range []string{
		`{"FutureField":1,"DefImmuneTypes":["nosuchtype"]}`,
		`{"FutureField":1,"ReduceSuperEffective":"x"}`,
		`{"FutureField":1,"DefAbsorbTypes":{"water":{"Heal":1}}}`,
		`{"FutureField":1,"DefImmuneTypes":["water"],"DefAbsorbTypes":{"water":{}}}`,
		`{"TypeConvert":{"From":"fire","To":"fire","PowerMod":4915},"FutureField":1}`,
	} {
		q := storetest.New()
		q.AbilityEffects = append(q.AbilityEffects,
			store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(effect)})
		if _, _, err := readmodel.Export(context.Background(), q); !errors.Is(err, readmodel.ErrInvalidExport) {
			t.Errorf("%s: err = %v, want ErrInvalidExport", effect, err)
		}
	}
}
