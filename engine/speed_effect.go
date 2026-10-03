package engine

// 素早さの補正(SpeedMods)の型と発動条件の語彙(ADR-0139)。
// engine は素早さを計算しない。ダメージ計算はこの値を読まない。
// マスタ(services/internal/master)と判定(services/judge)が同じ語彙を参照する単一の正。

// SpeedCondition は素早さの補正が成立する条件(閉じた語彙)。値は効果定義の JSON にそのまま書く。
type SpeedCondition string

const (
	SpeedConditionAlways          SpeedCondition = "always"
	SpeedConditionWeatherSun      SpeedCondition = "weather_sun"
	SpeedConditionWeatherRain     SpeedCondition = "weather_rain"
	SpeedConditionWeatherSand     SpeedCondition = "weather_sand"
	SpeedConditionWeatherSnow     SpeedCondition = "weather_snow"
	SpeedConditionTerrainElectric SpeedCondition = "terrain_electric"
	SpeedConditionHasStatus       SpeedCondition = "has_status"
	SpeedConditionItemLost        SpeedCondition = "item_lost" // 特性だけ
)

// AllSpeedConditions は語彙のすべてを定義順で返す(呼び出しごとに新しいスライス)。
func AllSpeedConditions() []SpeedCondition {
	return []SpeedCondition{
		SpeedConditionAlways,
		SpeedConditionWeatherSun,
		SpeedConditionWeatherRain,
		SpeedConditionWeatherSand,
		SpeedConditionWeatherSnow,
		SpeedConditionTerrainElectric,
		SpeedConditionHasStatus,
		SpeedConditionItemLost,
	}
}

// Known は語彙に含まれる条件かを返す。
func (c SpeedCondition) Known() bool {
	for _, k := range AllSpeedConditions() {
		if c == k {
			return true
		}
	}
	return false
}

// SpeedMod は条件つきの素早さ倍率(4096 基準。例 8192 = ×2)。
// 配列の順は評価の優先順で、成立した最初の要素だけを掛ける。
type SpeedMod struct {
	Condition SpeedCondition
	Modifier  int
}
