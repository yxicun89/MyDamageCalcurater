package master

// ADR-0139: pokedex-svc の内部 API の effect に素早さの補正(SpeedMods・IgnoresParalysisSpeedDrop)が入っても、
// calc-svc はロードでき(共通マスタで検証して engine の型に写す)、Lookup はその項目もコピーして返す。
// calc-svc の計算は素早さの項目を読まない(ダメージは変わらない。services/internal/master のテストで確かめる)。
// データはすべて架空。

import (
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

func speedExport(t *testing.T) api.MasterExport {
	t.Helper()
	export := baseExport(t)
	export.Items = append(export.Items,
		api.MasterItem{Id: "testspeedball", NameJa: "テストすばやさだま", Effect: effect(map[string]any{
			"SpeedMods": []any{map[string]any{"Condition": "always", "Modifier": 2048}},
		})},
	)
	export.Abilities = append(export.Abilities,
		api.MasterAbility{Id: "testswift", NameJa: "テストすいすい", Effect: effect(map[string]any{
			"SpeedMods": []any{
				map[string]any{"Condition": "weather_rain", "Modifier": 8192},
				map[string]any{"Condition": "has_status", "Modifier": 6144},
			},
			"IgnoresParalysisSpeedDrop": true,
		})},
	)
	return export
}

func TestFromExportCarriesSpeedEffects(t *testing.T) {
	store := newStore(t, speedExport(t))

	ball, ok := store.Item("testspeedball")
	if !ok {
		t.Fatal("testspeedball が引けない")
	}
	wantItem := &engine.ItemEffect{SpeedMods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}
	if !reflect.DeepEqual(ball.Effect, wantItem) {
		t.Errorf("Item.Effect = %+v, want %+v", ball.Effect, wantItem)
	}

	swift, ok := store.Ability("testswift")
	if !ok {
		t.Fatal("testswift が引けない")
	}
	wantAbility := &engine.AbilityEffect{
		SpeedMods: []engine.SpeedMod{
			{Condition: engine.SpeedConditionWeatherRain, Modifier: 8192},
			{Condition: engine.SpeedConditionHasStatus, Modifier: 6144},
		},
		IgnoresParalysisSpeedDrop: true,
	}
	if !reflect.DeepEqual(swift.Effect, wantAbility) {
		t.Errorf("Ability.Effect = %+v, want %+v", swift.Effect, wantAbility)
	}
}

// Lookup が返す Effect の SpeedMods(スライス)を書き換えても Store に漏れない(TestLookupReturnsCopiesOfEffects と同じ約束)。
func TestLookupReturnsCopiesOfSpeedEffects(t *testing.T) {
	store := newStore(t, speedExport(t))

	ball, _ := store.Item("testspeedball")
	ball.Effect.SpeedMods[0].Modifier = 9999
	ball.Effect.SpeedMods[0].Condition = engine.SpeedConditionItemLost
	again, _ := store.Item("testspeedball")
	if got := again.Effect.SpeedMods[0]; got.Modifier != 2048 || got.Condition != engine.SpeedConditionAlways {
		t.Errorf("Item の SpeedMods の書き換えが Store に漏れた: %+v", again.Effect.SpeedMods)
	}

	swift, _ := store.Ability("testswift")
	swift.Effect.SpeedMods[1].Modifier = 9999
	again2, _ := store.Ability("testswift")
	if got := again2.Effect.SpeedMods[1].Modifier; got != 6144 {
		t.Errorf("Ability の SpeedMods の書き換えが Store に漏れた: %+v", again2.Effect.SpeedMods)
	}
}

// 不正な素早さの項目はロード時に ErrInvalidMaster(部分的な Store を返さない)。
func TestFromExportRejectsInvalidSpeedEffects(t *testing.T) {
	export := baseExport(t)
	export.Abilities = append(export.Abilities,
		api.MasterAbility{Id: "testbadspeed", NameJa: "テストふせいすばやさ", Effect: effect(map[string]any{
			"SpeedMods": []any{map[string]any{"Condition": "weather_fog", "Modifier": 8192}},
		})},
	)
	if _, err := FromExport(export); err == nil {
		t.Fatal("FromExport = nil, want 語彙に無い条件で失敗")
	}
}
