package importer_test

// ADR-0139: 素早さの補正の定義は data/importer/effects.json の speedItems・speedAbilities 節に置き
// (ダメージの効果定義 items・abilities とは別の節。ゴールデンの写し testdata/golden/effects.json は
// ダメージの節だけを写すので、素早さの定義はゴールデン・oracle の照合に入らない)、
// 取り込みで同じ ID のダメージの定義と1つの効果 JSON に合わせて item_effects / ability_effects に入れる。
//   - 素早さの節に書けるのは素早さの項目だけ(持ち物: SpeedMods、特性: SpeedMods・IgnoresParalysisSpeedDrop)。
//   - ダメージの節に素早さの項目を書かない(定義の置き場を1つにする)。
//   - 値の検証は共通マスタ(master.DecodeItemEffect / DecodeAbilityEffect)が行い、正準形で投入する。
//   - 取り込まない ID の素早さの定義は投入せず effect-unused の警告にする(ダメージの節と同じ)。
//   - 網羅性(ADR-0103 §6)はダメージの指標なので、素早さの節の定義は数えない。
// データはすべて架空(fixture の ID)。

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

func effectRowsByID(rows []importer.EffectRow) (map[string]string, []string) {
	out := map[string]string{}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		out[r.ID] = string(r.Effect)
		ids = append(ids, r.ID)
	}
	return out, ids
}

func TestDecodeEffectsFileAcceptsSpeedSections(t *testing.T) {
	raw := []byte(`{
  "schemaVersion": 1,
  "items": {"testorb": {"DamageMod": 5324}},
  "abilities": {"testguard": {"DefResistType": {"fire": 2048}}},
  "speedItems": {"testorb": {"SpeedMods": [{"Condition": "always", "Modifier": 2048}]}},
  "speedAbilities": {"teststance": {"SpeedMods": [{"Condition": "weather_rain", "Modifier": 8192}]}}
}`)
	f, err := importer.DecodeEffectsFile(raw)
	if err != nil {
		t.Fatalf("DecodeEffectsFile = %v", err)
	}
	if len(f.SpeedItems) != 1 || len(f.SpeedAbilities) != 1 {
		t.Errorf("SpeedItems = %v, SpeedAbilities = %v", f.SpeedItems, f.SpeedAbilities)
	}
	// 素早さの節は省略できる(既存のファイルの形のまま読める)。
	if _, err := importer.DecodeEffectsFile([]byte(`{"schemaVersion":1,"items":{},"abilities":{}}`)); err != nil {
		t.Errorf("素早さの節が無いファイル: %v", err)
	}
	// 節の名前の大文字小文字違い・未知の節は従来どおり拒否する。
	for _, bad := range []string{
		`{"schemaVersion":1,"items":{},"abilities":{},"SpeedItems":{}}`,
		`{"schemaVersion":1,"items":{},"abilities":{},"speed":{}}`,
	} {
		if _, err := importer.DecodeEffectsFile([]byte(bad)); err == nil {
			t.Errorf("DecodeEffectsFile(%s) = nil, want error", bad)
		}
	}
}

func TestConvertMergesSpeedEffects(t *testing.T) {
	in := loadFixture(t)
	in.Effects.SpeedItems = map[string]json.RawMessage{
		// ダメージの定義がある持ち物に足す → 1つの効果 JSON に合わせる。
		"testorb": json.RawMessage(`{"SpeedMods": [{"Condition": "always", "Modifier": 2048}]}`),
		// ダメージの定義が無い持ち物 → 素早さだけの行ができる。
		"testmonite": json.RawMessage(`{"SpeedMods": [{"Modifier": 2048, "Condition": "always"}]}`),
		// 取り込まない持ち物 → 投入しない(effect-unused)。
		"testunusedspeed": json.RawMessage(`{"SpeedMods": [{"Condition": "always", "Modifier": 2048}]}`),
	}
	in.Effects.SpeedAbilities = map[string]json.RawMessage{
		"testguard":  json.RawMessage(`{"IgnoresParalysisSpeedDrop": true}`),
		"teststance": json.RawMessage(`{"SpeedMods": [{"Condition": "weather_rain", "Modifier": 8192}]}`),
		"testleaf": json.RawMessage(`{"SpeedMods": [{"Condition": "has_status", "Modifier": 6144}], ` +
			`"IgnoresParalysisSpeedDrop": true}`),
	}
	out, rep := convertOK(t, in)

	gotItems, itemIDs := effectRowsByID(out.ItemEffects)
	wantItems := map[string]string{
		"testberry":  `{"ResistBerryType":"fire"}`,
		"testmonite": `{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`,
		"testorb":    `{"DamageMod":5324,"SpeedMods":[{"Condition":"always","Modifier":2048}]}`,
	}
	if !reflect.DeepEqual(gotItems, wantItems) {
		t.Errorf("item_effects = %v, want %v", gotItems, wantItems)
	}
	if !sort.StringsAreSorted(itemIDs) {
		t.Errorf("item_effects の並びが ID 昇順でない: %v", itemIDs)
	}

	gotAbilities, abilityIDs := effectRowsByID(out.AbilityEffects)
	wantAbilities := map[string]string{
		"testguard":  `{"DefResistType":{"fire":2048},"IgnoresParalysisSpeedDrop":true}`,
		"testleaf":   `{"SpeedMods":[{"Condition":"has_status","Modifier":6144}],"IgnoresParalysisSpeedDrop":true}`,
		"teststance": `{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`,
	}
	if !reflect.DeepEqual(gotAbilities, wantAbilities) {
		t.Errorf("ability_effects = %v, want %v", gotAbilities, wantAbilities)
	}
	if !sort.StringsAreSorted(abilityIDs) {
		t.Errorf("ability_effects の並びが ID 昇順でない: %v", abilityIDs)
	}

	if !hasFinding(rep.Warnings, importer.KindEffectUnused, "testunusedspeed") {
		t.Error("取り込まない持ち物の素早さの定義(testunusedspeed)が effect-unused の警告に無い")
	}
}

// 素早さの節が無い・空なら、出力は従来と1バイトも変わらない(既存の dataVersion を動かさない)。
func TestConvertWithoutSpeedEffectsIsUnchanged(t *testing.T) {
	base, _ := convertOK(t, loadFixture(t))
	in := loadFixture(t)
	in.Effects.SpeedItems = map[string]json.RawMessage{}
	in.Effects.SpeedAbilities = map[string]json.RawMessage{}
	empty, _ := convertOK(t, in)
	if !reflect.DeepEqual(base.ItemEffects, empty.ItemEffects) || !reflect.DeepEqual(base.AbilityEffects, empty.AbilityEffects) {
		t.Error("空の素早さの節で効果の行が変わった")
	}
}

func TestConvertRejectsMisplacedSpeedEffects(t *testing.T) {
	speed := json.RawMessage(`{"SpeedMods": [{"Condition": "always", "Modifier": 2048}]}`)
	cases := []struct {
		name string
		edit func(in *importer.Input)
	}{
		{"素早さの節(持ち物)にダメージの項目", func(in *importer.Input) {
			in.Effects.SpeedItems = map[string]json.RawMessage{"testorb": json.RawMessage(`{"DamageMod": 5324}`)}
		}},
		{"素早さの節(持ち物)に未対応の印", func(in *importer.Input) {
			in.Effects.SpeedItems = map[string]json.RawMessage{"testorb": json.RawMessage(`{"UnsupportedDefender": true}`)}
		}},
		{"素早さの節(特性)にダメージの項目", func(in *importer.Input) {
			in.Effects.SpeedAbilities = map[string]json.RawMessage{"teststance": json.RawMessage(`{"StabMod": 8192}`)}
		}},
		{"素早さの節が空のオブジェクト", func(in *importer.Input) {
			in.Effects.SpeedAbilities = map[string]json.RawMessage{"teststance": json.RawMessage(`{}`)}
		}},
		{"素早さの節がオブジェクトでない", func(in *importer.Input) {
			in.Effects.SpeedItems = map[string]json.RawMessage{"testorb": json.RawMessage(`[]`)}
		}},
		{"ダメージの節(持ち物)に SpeedMods", func(in *importer.Input) {
			in.Effects.Items["testorb"] = json.RawMessage(`{"DamageMod": 5324, "SpeedMods": [{"Condition": "always", "Modifier": 2048}]}`)
		}},
		{"ダメージの節(特性)に SpeedMods", func(in *importer.Input) {
			in.Effects.Abilities["testguard"] = speed
		}},
		{"ダメージの節(特性)に IgnoresParalysisSpeedDrop", func(in *importer.Input) {
			in.Effects.Abilities["testguard"] = json.RawMessage(`{"DefResistType": {"fire": 2048}, "IgnoresParalysisSpeedDrop": true}`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			tc.edit(&in)
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidInput) {
				t.Fatalf("err = %v, want importer.ErrInvalidInput(定義の置き場が違う)", err)
			}
		})
	}
}

func TestConvertRejectsInvalidSpeedEffects(t *testing.T) {
	cases := []struct {
		name string
		edit func(in *importer.Input)
	}{
		{"語彙に無い条件", func(in *importer.Input) {
			in.Effects.SpeedAbilities = map[string]json.RawMessage{
				"teststance": json.RawMessage(`{"SpeedMods": [{"Condition": "weather_fog", "Modifier": 8192}]}`)}
		}},
		{"持ち物に item_lost", func(in *importer.Input) {
			in.Effects.SpeedItems = map[string]json.RawMessage{
				"testorb": json.RawMessage(`{"SpeedMods": [{"Condition": "item_lost", "Modifier": 8192}]}`)}
		}},
		{"Modifier が小数", func(in *importer.Input) {
			in.Effects.SpeedItems = map[string]json.RawMessage{
				"testorb": json.RawMessage(`{"SpeedMods": [{"Condition": "always", "Modifier": 2048.5}]}`)}
		}},
		{"持ち物に IgnoresParalysisSpeedDrop", func(in *importer.Input) {
			in.Effects.SpeedItems = map[string]json.RawMessage{"testorb": json.RawMessage(`{"IgnoresParalysisSpeedDrop": true}`)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			tc.edit(&in)
			_, _, err := importer.Convert(in)
			if !errors.Is(err, master.ErrInvalidEffect) && !errors.Is(err, importer.ErrInvalidInput) {
				t.Fatalf("err = %v, want master.ErrInvalidEffect か importer.ErrInvalidInput", err)
			}
		})
	}
}

// 網羅性(ADR-0103 §6)はダメージの指標。素早さの節の定義は「定義済み」に数えず、ダメージのハンドラが無くても
// effect-no-hook にしない(素早さだけの特性・持ち物が警告で埋もれないように)。
func TestReconcileEffectCoverageIgnoresSpeedEffects(t *testing.T) {
	in := reconcileInput(t)
	sdAbility(t, &in, "teststance").Hooks = []string{"onModifySpe"}
	in.Effects.SpeedAbilities = map[string]json.RawMessage{
		"teststance": json.RawMessage(`{"SpeedMods": [{"Condition": "weather_rain", "Modifier": 8192}]}`),
	}
	base := reconcileInput(t)
	_, want := reconcileOK(t, base)
	_, rec := reconcileOK(t, in)
	if hasFinding(rec.Report.Warnings, importer.KindEffectNoHook, "teststance") {
		t.Error("素早さだけの定義を effect-no-hook にした")
	}
	if rec.EffectCoverage.Abilities.Defined != want.EffectCoverage.Abilities.Defined {
		t.Errorf("Abilities.Defined = %d, want %d(素早さの定義はダメージの網羅性に数えない)",
			rec.EffectCoverage.Abilities.Defined, want.EffectCoverage.Abilities.Defined)
	}
}
