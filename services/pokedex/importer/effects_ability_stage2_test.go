package importer_test

// ADR-0178: 本番の効果定義(data/importer/effects.json)で、特性の段階2(技のフラグに依存する特性)の値を固定する。
// ゴールデンのベクタは生成時の効果を埋め込むので、effects.json の値を書き換えても照合では気づけない。
// ここで ADR-0178 §5 の表と完全一致させる(longreach は effects_ability_stage1_test.go で固定済み)。

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestRepoEffectsStage2Abilities(t *testing.T) {
	raw, err := os.ReadFile(repoDataPath("effects.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Abilities map[string]json.RawMessage `json:"abilities"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("data/importer/effects.json: %v", err)
	}
	chart := fullTypeChart(t)
	flagMod := func(f engine.MoveFlag, mod int) []engine.ConditionalPowerMod {
		return []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMoveFlag, Flag: f, Modifier: mod}}
	}
	want := map[string]engine.AbilityEffect{
		"punkrock": {
			PostAuraPowerMods:  flagMod(engine.MoveFlagSound, 5325),
			DefFinalModsByFlag: map[engine.MoveFlag]int{engine.MoveFlagSound: 2048},
			Breakable:          true,
		},
		"ironfist":     {PostAuraPowerMods: flagMod(engine.MoveFlagPunch, 4915)},
		"toughclaws":   {PostAuraPowerMods: flagMod(engine.MoveFlagContact, 5325)},
		"sheerforce":   {PostAuraPowerMods: flagMod(engine.MoveFlagSecondary, 5325)},
		"reckless":     {PostAuraPowerMods: flagMod(engine.MoveFlagRecoil, 4915)},
		"strongjaw":    {PowerMods: flagMod(engine.MoveFlagBite, 6144)},
		"megalauncher": {PowerMods: flagMod(engine.MoveFlagPulse, 6144)},
		"sharpness":    {PowerMods: flagMod(engine.MoveFlagSlicing, 6144)},
		"soundproof":   {DefImmuneFlags: []engine.MoveFlag{engine.MoveFlagSound}, Breakable: true},
		"bulletproof":  {DefImmuneFlags: []engine.MoveFlag{engine.MoveFlagBullet}, Breakable: true},
		"fluffy": {
			DefFinalModsByFlag: map[engine.MoveFlag]int{engine.MoveFlagContact: 2048},
			DefFinalModsByType: map[engine.Type]int{engine.TypeFire: 8192},
			Breakable:          true,
		},
		"auraguard":   {DefFinalModsByFlag: map[engine.MoveFlag]int{engine.MoveFlagContact: 2048}, Breakable: true},
		"liquidvoice": {FlagTypeConvert: &engine.FlagTypeConvert{Flag: engine.MoveFlagSound, To: engine.TypeWater}},
	}
	for id, w := range want {
		def, ok := file.Abilities[id]
		if !ok {
			t.Errorf("data/importer/effects.json に %q が無い(ADR-0178)", id)
			continue
		}
		got, err := master.DecodeAbilityEffect(def, chart)
		if err != nil {
			t.Errorf("%s の定義を読めない: %v", id, err)
			continue
		}
		if !reflect.DeepEqual(*got, w) {
			t.Errorf("%s = %+v, want %+v", id, *got, w)
		}
	}
}
