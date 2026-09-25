package master_test

// 効果定義の補正値の上限(PR #359 のレビューの軽微指摘・issue #270)。
// engine の Individual.Validate は補正値を MinEffectModifier..MaxEffectModifier(×1/4096〜×512)に
// 制限する。取込時にも同じ上限で弾き、計算のたびに 400 になる効果定義を DB に入れない。

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestDecodeEffectRejectsModifierAboveEngineMax(t *testing.T) {
	c := testChart(t)
	maxLit := strconv.Itoa(engine.MaxEffectModifier)
	overLit := strconv.Itoa(engine.MaxEffectModifier + 1)
	item := []string{
		`{"DamageMod":%s}`,
		`{"PowerMod":%s}`,
		`{"BoostType":"fire","BoostTypeMod":%s}`,
		`{"StatMods":{"atk":%s}}`,
	}
	ability := []string{
		`{"StabMod":%s}`,
		`{"OffBoostType":"fire","OffBoostTypeMod":%s}`,
		`{"ReduceSuperEffective":%s}`,
		`{"DefResistType":{"fire":%s}}`,
	}
	decoders := []struct {
		name    string
		formats []string
		decode  func([]byte) error
	}{
		{"item", item, func(b []byte) error { _, err := master.DecodeItemEffect(b, c); return err }},
		{"ability", ability, func(b []byte) error { _, err := master.DecodeAbilityEffect(b, c); return err }},
	}
	for _, d := range decoders {
		for _, f := range d.formats {
			atMax := strings.Replace(f, "%s", maxLit, 1)
			over := strings.Replace(f, "%s", overLit, 1)
			t.Run(d.name+"/上限ちょうど/"+atMax, func(t *testing.T) {
				if err := d.decode([]byte(atMax)); err != nil {
					t.Fatalf("上限ちょうど(MaxEffectModifier)を拒否した: %v", err)
				}
			})
			t.Run(d.name+"/上限超え/"+over, func(t *testing.T) {
				if err := d.decode([]byte(over)); !errors.Is(err, master.ErrInvalidEffect) {
					t.Fatalf("MaxEffectModifier+1 が ErrInvalidEffect にならない: %v", err)
				}
			})
		}
	}
}
