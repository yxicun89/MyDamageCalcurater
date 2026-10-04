package judge

import (
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
)

// 素早さに効く特性・持ち物をマスタの効果データ(ADR-0139 の SpeedMods)で反映する(issue 235 第2段・ADR-0714)。
// judge は ID の分岐を持たず、Individual に渡された効果データと環境(天候・場・状態異常)だけで評価する。
//
// 期待値の出典は @smogon/calc 0.12.0 の Champions 世代(Generations.get(0))の getFinalSpeed
// (dist/mechanics/util.js)。追い風 → 特性 → 持ち物の順に speedMods へ積み、chainMods で連結してから
// pokeRound を 1 回、そのあとに まひ(Quick Feet 相当の特性を除く)で floor(x × 50 / 100)。
// 下の数値は実数値 rawStats.spe を直接与えて getFinalSpeed を呼んだ結果と一致することを確かめてある
// (手順は ADR-0714「テストの期待値」)。テストの効果データは架空の倍率・条件の組で、実データの ID は書かない。

// 架空の効果データ(倍率は 4096 基準)。
var (
	rainDouble  = AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherRain, Modifier: 8192}}}
	sunDouble   = AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherSun, Modifier: 8192}}}
	sandDouble  = AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherSand, Modifier: 8192}}}
	snowDouble  = AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherSnow, Modifier: 8192}}}
	elecDouble  = AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionTerrainElectric, Modifier: 8192}}}
	statusBoost = AbilitySpeedEffect{
		Mods:                      []engine.SpeedMod{{Condition: engine.SpeedConditionHasStatus, Modifier: 6144}},
		IgnoresParalysisSpeedDrop: true,
	}
	itemLostDouble = AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionItemLost, Modifier: 8192}}}
	alwaysHalfItem = ItemSpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}
)

// 実数値 91(種族値 71・無振り・無補正)と 93(種族値 73)。奇数なので丸めの順序・回数が値に出る。
func odd91() Individual { return Individual{BaseSpeed: 71, Nature: engine.NatureNeutral} }
func odd93() Individual { return Individual{BaseSpeed: 73, Nature: engine.NatureNeutral} }

func TestSpeedEnvironmentHolds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  SpeedEnvironment
		cond engine.SpeedCondition
		want bool
	}{
		{"always は環境が空でも成立", SpeedEnvironment{}, engine.SpeedConditionAlways, true},
		{"晴れ", SpeedEnvironment{Weather: "sun"}, engine.SpeedConditionWeatherSun, true},
		{"雨", SpeedEnvironment{Weather: "rain"}, engine.SpeedConditionWeatherRain, true},
		{"砂", SpeedEnvironment{Weather: "sand"}, engine.SpeedConditionWeatherSand, true},
		{"雪", SpeedEnvironment{Weather: "snow"}, engine.SpeedConditionWeatherSnow, true},
		{"別の天候では成立しない", SpeedEnvironment{Weather: "sun"}, engine.SpeedConditionWeatherRain, false},
		{"天候 none は天候なし", SpeedEnvironment{Weather: "none"}, engine.SpeedConditionWeatherRain, false},
		{"天候が空は天候なし", SpeedEnvironment{}, engine.SpeedConditionWeatherSun, false},
		{"エレキフィールド", SpeedEnvironment{Terrain: "electric"}, engine.SpeedConditionTerrainElectric, true},
		{"他のフィールドでは成立しない", SpeedEnvironment{Terrain: "grassy"}, engine.SpeedConditionTerrainElectric, false},
		{"状態異常あり", SpeedEnvironment{HasStatus: true}, engine.SpeedConditionHasStatus, true},
		{"状態異常なし", SpeedEnvironment{}, engine.SpeedConditionHasStatus, false},
		// judge は「持ち物を失った」入力を持たないので、どの環境でも成立しない(ADR-0714 §2)。
		{"item_lost は成立しない", SpeedEnvironment{Weather: "rain", Terrain: "electric", HasStatus: true}, engine.SpeedConditionItemLost, false},
		// 語彙に無い条件は安全側(成立しない)。
		{"未知の条件は成立しない", SpeedEnvironment{Weather: "rain", HasStatus: true}, engine.SpeedCondition("weather_fog"), false},
		{"大文字小文字違いは未知", SpeedEnvironment{Weather: "rain"}, engine.SpeedCondition("Weather_Rain"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.env.Holds(tt.cond); got != tt.want {
				t.Errorf("SpeedEnvironment%+v.Holds(%q) = %v, want %v", tt.env, tt.cond, got, tt.want)
			}
		})
	}
}

// TestResolveSpeedMod: 配列の順は評価の優先順で、成立した最初の要素だけを掛ける(ADR-0139 §2)。
// 成立を判定できない条件(item_lost・語彙に無い条件)に当たったら、そこで評価を止めて「確定できない」を返し、
// 後ろの要素は掛けない(ADR-0714 §2: 前の要素が実は成立していたかもしれないので、後ろを掛けると誤る)。
func TestResolveSpeedMod(t *testing.T) {
	t.Parallel()

	mod := func(c engine.SpeedCondition, m int) engine.SpeedMod {
		return engine.SpeedMod{Condition: c, Modifier: m}
	}

	tests := []struct {
		name             string
		mods             []engine.SpeedMod
		env              SpeedEnvironment
		wantModifier     int
		wantApplied      bool
		wantUndetermined bool
	}{
		{"効果なし(空)", nil, SpeedEnvironment{Weather: "rain"}, 0, false, false},
		{"成立", []engine.SpeedMod{mod(engine.SpeedConditionWeatherRain, 8192)}, SpeedEnvironment{Weather: "rain"}, 8192, true, false},
		{"不成立は確定(印にしない)", []engine.SpeedMod{mod(engine.SpeedConditionWeatherRain, 8192)}, SpeedEnvironment{}, 0, false, false},
		{
			"成立した最初の要素だけ(後ろが成立しても掛けない)",
			[]engine.SpeedMod{mod(engine.SpeedConditionHasStatus, 6144), mod(engine.SpeedConditionAlways, 8192)},
			SpeedEnvironment{HasStatus: true}, 6144, true, false,
		},
		{
			"最初が不成立なら次の要素",
			[]engine.SpeedMod{mod(engine.SpeedConditionWeatherSun, 8192), mod(engine.SpeedConditionAlways, 2048)},
			SpeedEnvironment{Weather: "rain"}, 2048, true, false,
		},
		{"item_lost は確定できない", []engine.SpeedMod{mod(engine.SpeedConditionItemLost, 8192)}, SpeedEnvironment{}, 0, false, true},
		{
			"確定できない条件の後ろは掛けない",
			[]engine.SpeedMod{mod(engine.SpeedConditionItemLost, 8192), mod(engine.SpeedConditionAlways, 2048)},
			SpeedEnvironment{}, 0, false, true,
		},
		{
			"成立が確定できない条件より前にあればそれを掛ける",
			[]engine.SpeedMod{mod(engine.SpeedConditionAlways, 2048), mod(engine.SpeedConditionItemLost, 8192)},
			SpeedEnvironment{}, 2048, true, false,
		},
		{"未知の条件は確定できない", []engine.SpeedMod{mod("weather_fog", 8192)}, SpeedEnvironment{Weather: "rain"}, 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			modifier, applied, undetermined := ResolveSpeedMod(tt.mods, tt.env)
			if modifier != tt.wantModifier || applied != tt.wantApplied || undetermined != tt.wantUndetermined {
				t.Errorf("ResolveSpeedMod = (%d, %v, %v), want (%d, %v, %v)",
					modifier, applied, undetermined, tt.wantModifier, tt.wantApplied, tt.wantUndetermined)
			}
			if applied && undetermined {
				t.Error("applied と undetermined は同時に true にならない")
			}
		})
	}
}

// TestSpeedWithAbilityAndItemEffects: 特性・持ち物の素早さ効果を連結に入れる。期待値は getFinalSpeed の値。
func TestSpeedWithAbilityAndItemEffects(t *testing.T) {
	t.Parallel()

	with := func(in Individual, f func(*Individual)) Individual { f(&in); return in }

	tests := []struct {
		name string
		in   Individual
		want int
		// naive は補正ごとに丸めた・まひを先に掛けたなどの誤った実装の値(want と違うときだけ見る)。
		naive int
	}{
		{"雨で ×2 の特性・雨", with(odd91(), func(i *Individual) { i.Ability = rainDouble; i.Env.Weather = "rain" }), 182, 182},
		{"雨で ×2 の特性・天候なしは掛けない", with(odd91(), func(i *Individual) { i.Ability = rainDouble }), 91, 91},
		{"雨で ×2 の特性・晴れは掛けない", with(odd91(), func(i *Individual) { i.Ability = rainDouble; i.Env.Weather = "sun" }), 91, 91},
		{"晴れで ×2", with(odd91(), func(i *Individual) { i.Ability = sunDouble; i.Env.Weather = "sun" }), 182, 182},
		{"砂で ×2", with(odd91(), func(i *Individual) { i.Ability = sandDouble; i.Env.Weather = "sand" }), 182, 182},
		{"雪で ×2", with(odd91(), func(i *Individual) { i.Ability = snowDouble; i.Env.Weather = "snow" }), 182, 182},
		{"エレキフィールドで ×2", with(odd91(), func(i *Individual) { i.Ability = elecDouble; i.Env.Terrain = "electric" }), 182, 182},
		{"エレキ以外のフィールドは掛けない", with(odd91(), func(i *Individual) { i.Ability = elecDouble; i.Env.Terrain = "psychic" }), 91, 91},
		// 91 × 1.5 = 136.5 → 五捨五超入で 136。
		{"状態異常で ×1.5(やけど)", with(odd91(), func(i *Individual) { i.Ability = statusBoost; i.Env.HasStatus = true }), 136, 136},
		{"状態異常で ×1.5 の特性・状態異常なし", with(odd91(), func(i *Individual) { i.Ability = statusBoost }), 91, 91},
		// まひの半減を受けない特性: まひでも ×1.5 だけ(136)。半減すると 68。
		{
			"まひ + まひの半減を受けない特性",
			with(odd91(), func(i *Individual) { i.Ability = statusBoost; i.Env.HasStatus = true; i.Paralysis = true }),
			136, 68,
		},
		// 持ち物を失った後の条件は成立しない(judge は入力を持たない)ので掛けない。
		{"持ち物を失った後の特性は掛けない", with(odd91(), func(i *Individual) { i.Ability = itemLostDouble }), 91, 91},
		// 91 × 0.5 = 45.5 → 五捨五超入で 45。93 → 46.5 → 46。
		{"常に ×0.5 の持ち物(91)", with(odd91(), func(i *Individual) { i.Item = alwaysHalfItem }), 45, 45},
		{"常に ×0.5 の持ち物(93)", with(odd93(), func(i *Individual) { i.Item = alwaysHalfItem }), 46, 46},
		// 連結: chain(6144, 2048) = 3072 → 93 × 0.75 = 69.75 → 70。補正ごとに丸めると 139 → 69.5 → 69。
		{
			"状態異常 ×1.5 + 持ち物 ×0.5 は連結してから 1 回丸める",
			with(odd93(), func(i *Individual) { i.Ability = statusBoost; i.Env.HasStatus = true; i.Item = alwaysHalfItem }),
			70, 69,
		},
		{
			"まひ + 半減を受けない特性 + 持ち物 ×0.5(半減しない)",
			with(odd93(), func(i *Individual) {
				i.Ability = statusBoost
				i.Env.HasStatus = true
				i.Paralysis = true
				i.Item = alwaysHalfItem
			}),
			70, 35,
		},
		// 93 × 0.5 = 46.5 → 46 → まひで 23。
		{"まひ + 持ち物 ×0.5(特性なし)", with(odd93(), func(i *Individual) { i.Paralysis = true; i.Item = alwaysHalfItem }), 23, 23},
		// chain(8192, 6144) = 12288 → 93 × 3 = 279(まひの半減なし)。
		{
			"追い風 + まひ + 半減を受けない特性",
			with(odd93(), func(i *Individual) {
				i.Tailwind = true
				i.Ability = statusBoost
				i.Env.HasStatus = true
				i.Paralysis = true
			}),
			279, 139,
		},
		// chain(8192, 6144) = 12288 → 279。スカーフで先に丸めると 139 → 278。
		{
			"雨 ×2 の特性 + こだわりスカーフ",
			with(odd93(), func(i *Individual) { i.Ability = rainDouble; i.Env.Weather = "rain"; i.Scarf = true }),
			279, 278,
		},
		{
			"追い風 + 雨 ×2 の特性",
			with(odd93(), func(i *Individual) { i.Tailwind = true; i.Ability = rainDouble; i.Env.Weather = "rain" }),
			372, 372,
		},
		{"追い風 + 持ち物 ×0.5", with(odd93(), func(i *Individual) { i.Tailwind = true; i.Item = alwaysHalfItem }), 93, 92},
		{
			"追い風 + 雨 ×2 + 持ち物 ×0.5",
			with(odd93(), func(i *Individual) {
				i.Tailwind = true
				i.Ability = rainDouble
				i.Env.Weather = "rain"
				i.Item = alwaysHalfItem
			}),
			186, 186,
		},
		// ランク +1: floor(93 × 3 / 2) = 139 → chain(8192, 6144, 2048) = 6144 → 139 × 1.5 = 208.5 → 208。
		{
			"ランク +1 + 追い風 + 状態異常 ×1.5 + 持ち物 ×0.5",
			with(odd93(), func(i *Individual) {
				i.Ranks = engine.Ranks{Spe: 1}
				i.Tailwind = true
				i.Ability = statusBoost
				i.Env.HasStatus = true
				i.Item = alwaysHalfItem
			}),
			208, 208,
		},
		// こだわりスカーフと持ち物の効果データが両方あるとき、スカーフを優先して持ち物の効果は掛けない
		// (@smogon/calc は Choice Scarf を先に見る else-if。持ち物は 1 つなので、データに効果が付いても二重に掛けない)。
		{
			"スカーフ判定が真なら持ち物の効果データは掛けない",
			with(odd93(), func(i *Individual) { i.Scarf = true; i.Item = alwaysHalfItem }),
			139, 70,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Speed(tt.in)
			if err != nil {
				t.Fatalf("Speed: %v", err)
			}
			if got != tt.want {
				t.Errorf("Speed(%+v) = %d, want %d", tt.in, got, tt.want)
			}
			if tt.naive != tt.want && got == tt.naive {
				t.Errorf("Speed = %d は誤った順序・丸めの値(連結 → 1 回丸め → まひ。ADR-0714 §3)", got)
			}
		})
	}
}

// TestSpeedWithoutEffectsUnchanged: 効果データ・環境のゼロ値は第1段の値と同じ(回帰。ADR-0714 §5)。
// 環境だけ(天候・場・状態異常)があっても、効果データが無ければ素早さは変わらない。
func TestSpeedWithoutEffectsUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want int
	}{
		{"補正なし", odd91(), 91},
		{"天候・場・状態異常だけでは変わらない", Individual{
			BaseSpeed: 71, Nature: engine.NatureNeutral,
			Env: SpeedEnvironment{Weather: "rain", Terrain: "electric", HasStatus: true},
		}, 91},
		{"まひは従来どおり半減", Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Paralysis: true, Env: SpeedEnvironment{HasStatus: true}}, 45},
		{"スカーフ + 追い風は従来どおり", Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Scarf: true, Tailwind: true}, 273},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Speed(tt.in)
			if err != nil {
				t.Fatalf("Speed: %v", err)
			}
			if got != tt.want {
				t.Errorf("Speed(%+v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestAppliedSpeedFactorsWithEffects: 連鎖順 rank → tailwind → ability → choiceScarf / item → paralysis
// (ADR-0714 §4)。実際に掛けたものだけを入れる。
func TestAppliedSpeedFactorsWithEffects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want []string
	}{
		{"特性の条件が成立", Individual{Ability: rainDouble, Env: SpeedEnvironment{Weather: "rain"}}, []string{SpeedFactorAbility}},
		{"特性の条件が不成立なら入らない", Individual{Ability: rainDouble, Env: SpeedEnvironment{Weather: "sun"}}, []string{}},
		{"持ち物を失った後の特性は入らない", Individual{Ability: itemLostDouble}, []string{}},
		{"持ち物の効果", Individual{Item: alwaysHalfItem}, []string{SpeedFactorItem}},
		{"スカーフなら item ではなく choiceScarf", Individual{Scarf: true, Item: alwaysHalfItem}, []string{SpeedFactorChoiceScarf}},
		{
			"まひの半減を受けない特性 + まひ: paralysis は入らない(掛けていない)",
			Individual{Ability: statusBoost, Env: SpeedEnvironment{HasStatus: true}, Paralysis: true},
			[]string{SpeedFactorAbility},
		},
		{
			"全部: rank → tailwind → ability → item → paralysis",
			Individual{
				Ranks: engine.Ranks{Spe: 2}, Tailwind: true,
				Ability: rainDouble, Env: SpeedEnvironment{Weather: "rain", HasStatus: true},
				Item: alwaysHalfItem, Paralysis: true,
			},
			[]string{SpeedFactorRank, SpeedFactorTailwind, SpeedFactorAbility, SpeedFactorItem, SpeedFactorParalysis},
		},
		{
			"全部(スカーフ): rank → tailwind → ability → choiceScarf → paralysis",
			Individual{
				Ranks: engine.Ranks{Spe: -1}, Tailwind: true,
				Ability: elecDouble, Env: SpeedEnvironment{Terrain: "electric", HasStatus: true},
				Scarf: true, Paralysis: true,
			},
			[]string{SpeedFactorRank, SpeedFactorTailwind, SpeedFactorAbility, SpeedFactorChoiceScarf, SpeedFactorParalysis},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := AppliedSpeedFactors(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppliedSpeedFactors = %#v, want %#v", got, tt.want)
			}
		})
	}
	if SpeedFactorAbility != "ability" || SpeedFactorItem != "item" {
		t.Errorf("SpeedFactorAbility/Item = %q/%q, want ability/item(契約の SpeedFactor)", SpeedFactorAbility, SpeedFactorItem)
	}
}

// TestIgnoredSpeedInputsFor: 第2段の Ignored(ADR-0714 §4)。「指定された」かつ「データで素早さへの効き方を
// 確定できなかった(Resolved=false)」ときだけ入る。fieldWeather は天候があり、abilityId が入るときだけ。
func TestIgnoredSpeedInputsFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   SpeedInputResolution
		want []string
	}{
		{"何も無い", SpeedInputResolution{}, []string{}},
		{"特性が確定していれば入らない(効果なし・不成立・反映済み)",
			SpeedInputResolution{AbilitySpecified: true, AbilityResolved: true, HasWeather: true}, []string{}},
		{"特性が確定できない", SpeedInputResolution{AbilitySpecified: true}, []string{IgnoredSpeedAbility}},
		{"特性が確定できない + 天候", SpeedInputResolution{AbilitySpecified: true, HasWeather: true},
			[]string{IgnoredSpeedAbility, IgnoredSpeedWeather}},
		{"持ち物が確定していれば入らない", SpeedInputResolution{ItemSpecified: true, ItemResolved: true}, []string{}},
		{"持ち物が確定できない", SpeedInputResolution{ItemSpecified: true}, []string{IgnoredSpeedItem}},
		{"持ち物だけ確定できない + 天候では fieldWeather は入らない",
			SpeedInputResolution{AbilitySpecified: true, AbilityResolved: true, ItemSpecified: true, HasWeather: true},
			[]string{IgnoredSpeedItem}},
		{"全部は abilityId → itemId → fieldWeather",
			SpeedInputResolution{AbilitySpecified: true, ItemSpecified: true, HasWeather: true},
			[]string{IgnoredSpeedAbility, IgnoredSpeedItem, IgnoredSpeedWeather}},
		{"指定されていないなら Resolved が false でも入らない", SpeedInputResolution{HasWeather: true}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IgnoredSpeedInputsFor(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IgnoredSpeedInputsFor(%+v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// TestIgnoredSpeedInputsMatchesUnresolved: データが引けないとき(全て Resolved=false)の Ignored は
// 第1段の IgnoredSpeedInputs と同じ(マスタ取得失敗時のフェイルソフトの応答が第1段と一致する。ADR-0714 §1)。
func TestIgnoredSpeedInputsMatchesUnresolved(t *testing.T) {
	t.Parallel()

	for _, ability := range []string{"", "a"} {
		for _, item := range []string{"", "other", "scarf"} {
			for _, hasWeather := range []bool{false, true} {
				isScarf := item == "scarf"
				old := IgnoredSpeedInputs(ability, item, isScarf, hasWeather)
				got := IgnoredSpeedInputsFor(SpeedInputResolution{
					AbilitySpecified: ability != "",
					ItemSpecified:    item != "" && !isScarf,
					HasWeather:       hasWeather,
				})
				if !reflect.DeepEqual(got, old) {
					t.Errorf("ability=%q item=%q weather=%v: IgnoredSpeedInputsFor = %#v, 第1段 = %#v",
						ability, item, hasWeather, got, old)
				}
			}
		}
	}
}
