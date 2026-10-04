package master

// ADR-0176: pokedex-svc の内部 API の effect に特性の段階1の項目が入っても、calc-svc はロードでき
// (共通マスタで検証して engine の型に写す)、Lookup はその項目をディープコピーして返す
// (TypeConvert のポインタ・PowerMods のスライス・StatMods / SeparateStatMods の map を呼び出し側が書き換えても
// Store に漏れない。TestLookupReturnsCopiesOfEffects と同じ約束)。データはすべて架空。

import (
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

func stage1Export(t *testing.T) api.MasterExport {
	t.Helper()
	export := baseExport(t)
	export.Abilities = append(export.Abilities,
		api.MasterAbility{Id: "teststage1", NameJa: "テスト段階1", Effect: effect(map[string]any{
			"TypeConvert":      map[string]any{"From": "normal", "To": "fire", "PowerMod": 4915},
			"PowerMods":        []any{map[string]any{"Condition": "max_base_power", "MaxPower": 60, "Modifier": 6144}},
			"StatMods":         map[string]any{"atk": 8192},
			"SeparateStatMods": map[string]any{"atk": 6144},
			"CritDamageMod":    6144,
		})},
		api.MasterAbility{Id: "teststage1def", NameJa: "テスト段階1防御", Effect: effect(map[string]any{
			"StatMods":             map[string]any{"def": 8192},
			"PreventsCritical":     true,
			"IgnoresOpponentRanks": true,
			"Breakable":            true,
		})},
		api.MasterAbility{Id: "teststage1brk", NameJa: "テスト段階1破り", Effect: effect(map[string]any{
			"IgnoresDefenderAbility": true,
		})},
	)
	return export
}

func TestFromExportCarriesStage1AbilityEffects(t *testing.T) {
	store := newStore(t, stage1Export(t))
	for _, tc := range []struct {
		id   string
		want *engine.AbilityEffect
	}{
		{"teststage1", &engine.AbilityEffect{
			TypeConvert:      &engine.TypeConvert{From: "normal", To: "fire", PowerMod: 4915},
			PowerMods:        []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}},
			StatMods:         map[engine.StatKey]int{engine.StatAtk: 8192},
			SeparateStatMods: map[engine.StatKey]int{engine.StatAtk: 6144},
			CritDamageMod:    6144,
		}},
		{"teststage1def", &engine.AbilityEffect{
			StatMods: map[engine.StatKey]int{engine.StatDef: 8192}, PreventsCritical: true, IgnoresOpponentRanks: true, Breakable: true,
		}},
		{"teststage1brk", &engine.AbilityEffect{IgnoresDefenderAbility: true}},
	} {
		a, ok := store.Ability(tc.id)
		if !ok {
			t.Errorf("%s が引けない(ロードで落ちたか)", tc.id)
			continue
		}
		if !reflect.DeepEqual(a.Effect, tc.want) {
			t.Errorf("%s: Effect = %+v, want %+v", tc.id, a.Effect, tc.want)
		}
	}
}

func TestLookupReturnsCopiesOfStage1AbilityEffects(t *testing.T) {
	store := newStore(t, stage1Export(t))
	a, ok := store.Ability("teststage1")
	if !ok || a.Effect == nil || a.Effect.TypeConvert == nil || len(a.Effect.PowerMods) == 0 {
		t.Fatalf("teststage1 の段階1の項目が引けない: %+v", a.Effect)
	}
	a.Effect.TypeConvert.To = "water"
	a.Effect.PowerMods[0].Modifier = 9999
	a.Effect.StatMods[engine.StatAtk] = 1
	a.Effect.SeparateStatMods[engine.StatAtk] = 1

	again, _ := store.Ability("teststage1")
	if got := again.Effect.TypeConvert.To; got != "fire" {
		t.Errorf("TypeConvert の書き換えが Store に漏れた: %q", got)
	}
	if got := again.Effect.PowerMods[0].Modifier; got != 6144 {
		t.Errorf("PowerMods の書き換えが Store に漏れた: %d", got)
	}
	if got := again.Effect.StatMods[engine.StatAtk]; got != 8192 {
		t.Errorf("StatMods の書き換えが Store に漏れた: %d", got)
	}
	if got := again.Effect.SeparateStatMods[engine.StatAtk]; got != 6144 {
		t.Errorf("SeparateStatMods の書き換えが Store に漏れた: %d", got)
	}
}
