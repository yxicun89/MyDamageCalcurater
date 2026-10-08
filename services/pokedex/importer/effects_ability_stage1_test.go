package importer_test

// ADR-0176: 本番の効果定義(data/importer/effects.json)で、特性の段階1の特性が「未対応の印」から
// 計算に入る効果へ移っていること(ニンフィアのフェアリースキン等。2026-10-04 のユーザーの実使用)。
// oracle と一致することは tools/golden の生成器とゴールデンが確かめる。ここが守るのは本番データの移行の取りこぼし。

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestRepoEffectsStage1Abilities(t *testing.T) {
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
	convert := func(to engine.Type) engine.AbilityEffect {
		return engine.AbilityEffect{TypeConvert: &engine.TypeConvert{From: engine.TypeNormal, To: to, PowerMod: 4915}}
	}
	want := map[string]engine.AbilityEffect{
		"pixilate":     convert(engine.TypeFairy),
		"refrigerate":  convert(engine.TypeIce),
		"aerilate":     convert(engine.TypeFlying),
		"dragonize":    convert(engine.TypeDragon),
		"hugepower":    {StatMods: map[engine.StatKey]int{engine.StatAtk: 8192}},
		"purepower":    {StatMods: map[engine.StatKey]int{engine.StatAtk: 8192}},
		"hustle":       {SeparateStatMods: map[engine.StatKey]int{engine.StatAtk: 6144}},
		"furcoat":      {StatMods: map[engine.StatKey]int{engine.StatDef: 8192}, Breakable: true},
		"technician":   {PowerMods: []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}},
		"steelyspirit": {PowerMods: []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMoveType, MoveType: engine.TypeSteel, Modifier: 6144}}},
		"fairyaura":    {AuraType: engine.TypeFairy, AuraMod: 5448},
		"sniper":       {CritDamageMod: 6144},
		"battlearmor":  {PreventsCritical: true, Breakable: true},
		"shellarmor":   {PreventsCritical: true, Breakable: true},
		"moldbreaker":  {IgnoresDefenderAbility: true},
		"unaware":      {IgnoresOpponentRanks: true, Breakable: true},
		// 段階1で計算に入れないもの(oracle の調査で新たに見つかった漏れ)は印を付ける。
		"merciless": {UnsupportedAttacker: true},
		"longreach": {UnsupportedAttacker: true},
	}
	for id, w := range want {
		def, ok := file.Abilities[id]
		if !ok {
			t.Errorf("data/importer/effects.json に %q が無い(ADR-0176)", id)
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
