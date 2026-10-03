package engine

// ADR-0139: 素早さの補正(SpeedMods)の型と発動条件の語彙。
// engine は素早さを計算しない(ダメージ計算はこの値を読まない)。型と語彙だけを持ち、
// マスタ(services/internal/master)と判定(services/judge)が同じ語彙を参照する単一の正にする。

import (
	"reflect"
	"testing"
)

func TestAllSpeedConditions(t *testing.T) {
	want := []SpeedCondition{
		SpeedConditionAlways,
		SpeedConditionWeatherSun,
		SpeedConditionWeatherRain,
		SpeedConditionWeatherSand,
		SpeedConditionWeatherSnow,
		SpeedConditionTerrainElectric,
		SpeedConditionHasStatus,
		SpeedConditionItemLost,
	}
	if got := AllSpeedConditions(); !reflect.DeepEqual(got, want) {
		t.Errorf("AllSpeedConditions() = %v, want %v", got, want)
	}
	// 戻り値を書き換えても次の呼び出しに漏れない(可変のグローバルを返さない)。
	got := AllSpeedConditions()
	got[0] = "changed"
	if AllSpeedConditions()[0] != SpeedConditionAlways {
		t.Error("AllSpeedConditions の戻り値の書き換えが漏れた")
	}
}

func TestSpeedConditionWireValues(t *testing.T) {
	// 値はマスタの効果定義(JSON)にそのまま書く文字列。変えると取り込み済みのデータが読めなくなる。
	want := map[SpeedCondition]string{
		SpeedConditionAlways:          "always",
		SpeedConditionWeatherSun:      "weather_sun",
		SpeedConditionWeatherRain:     "weather_rain",
		SpeedConditionWeatherSand:     "weather_sand",
		SpeedConditionWeatherSnow:     "weather_snow",
		SpeedConditionTerrainElectric: "terrain_electric",
		SpeedConditionHasStatus:       "has_status",
		SpeedConditionItemLost:        "item_lost",
	}
	for c, s := range want {
		if string(c) != s {
			t.Errorf("%v = %q, want %q", c, string(c), s)
		}
	}
}

func TestSpeedConditionKnown(t *testing.T) {
	for _, c := range AllSpeedConditions() {
		if !c.Known() {
			t.Errorf("%q.Known() = false, want true", c)
		}
	}
	for _, c := range []SpeedCondition{"", "Always", "weather_hail", "terrain_grassy", "status", "paralysis"} {
		if c.Known() {
			t.Errorf("%q.Known() = true, want false(閉じた語彙)", c)
		}
	}
}

// SpeedMods は持ち物・特性の効果の項目。特性だけが IgnoresParalysisSpeedDrop を持つ。
func TestSpeedModsFieldsOnEffects(t *testing.T) {
	item := ItemEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionAlways, Modifier: 2048}}}
	ability := AbilityEffect{
		SpeedMods:                 []SpeedMod{{Condition: SpeedConditionWeatherRain, Modifier: 8192}},
		IgnoresParalysisSpeedDrop: true,
	}
	if item.SpeedMods[0].Modifier != 2048 || ability.SpeedMods[0].Condition != SpeedConditionWeatherRain || !ability.IgnoresParalysisSpeedDrop {
		t.Errorf("項目が保持されない: %+v %+v", item, ability)
	}
}
