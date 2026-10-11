package engine

import (
	"errors"
	"testing"
)

// 素早さと行動順(ADR-0179 §2・§3。連続技 CB-2)。
//
// 判定(services/judge/internal/judge の speed.go・turnorder.go)の式を engine に移す。engine は services を import できないので、
// 判定のテスト(speed_test.go・speed_effects_test.go・turnorder_test.go)の入力と期待値をここに写し、engine で同じ値になることを示す。
// 写したケースはテスト名の「judge: 」の後に元のケース名を残す。judge の入力 → engine の入力の対応は ADR-0179 §4 の表。
// 期待値の出典は judge と同じ(@smogon/calc 0.12.0 の getFinalSpeed。ADR-0701・0702・0712・0714)。効果データは架空の倍率・条件。

// speedCase は judge の Individual と同じ形の入力。build で engine の入力に写す(ADR-0179 §4)。
type speedCase struct {
	base     int
	nature   Nature
	sp       Stats
	ranks    Ranks
	scarf    bool // judge の Scarf。engine では「常に ×1.5 の持ち物」として渡す
	tailwind bool
	ability  *AbilityEffect
	item     *ItemEffect
	weather  Weather
	terrain  Terrain
	status   Status // judge の Paralysis は StatusParalysis、Env.HasStatus は none 以外
}

var (
	speNaturePlus  = Nature{Plus: StatSpe, Minus: StatSpA}
	speNatureMinus = Nature{Plus: StatSpA, Minus: StatSpe}

	speRainDouble  = &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionWeatherRain, Modifier: 8192}}}
	speSunDouble   = &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionWeatherSun, Modifier: 8192}}}
	speSandDouble  = &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionWeatherSand, Modifier: 8192}}}
	speSnowDouble  = &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionWeatherSnow, Modifier: 8192}}}
	speElecDouble  = &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionTerrainElectric, Modifier: 8192}}}
	speStatusBoost = &AbilityEffect{
		SpeedMods:                 []SpeedMod{{Condition: SpeedConditionHasStatus, Modifier: 6144}},
		IgnoresParalysisSpeedDrop: true,
	}
	speItemLostDouble = &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionItemLost, Modifier: 8192}}}
	speAlwaysHalfItem = &ItemEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionAlways, Modifier: 2048}}}
	// speScarfItem はこだわりスカーフを効果データで表したもの(engine は持ち物 ID を見ない。ADR-0179 §2)。
	speScarfItem = &ItemEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionAlways, Modifier: 6144}}}
)

func (c speedCase) build() (Individual, Field, SpeedSide) {
	nature := c.nature
	ind := Individual{
		Species: Species{BaseStats: Stats{Spe: c.base}},
		Nature:  nature,
		SP:      c.sp,
		Ranks:   c.ranks,
		Status:  c.status,
	}
	if c.ability != nil {
		ind.Ability = Ability{ID: "test-ability", Effect: c.ability}
	}
	switch {
	case c.scarf:
		// judge は Scarf が真なら持ち物の効果データを掛けない(持ち物は 1 つ)。engine では持ち物そのものがスカーフ。
		ind.Item = &Item{ID: "test-scarf", Effect: speScarfItem}
	case c.item != nil:
		ind.Item = &Item{ID: "test-item", Effect: c.item}
	}
	return ind, Field{Weather: c.weather, Terrain: c.terrain}, SpeedSide{Tailwind: c.tailwind}
}

func mustFinalSpeed(t *testing.T, c speedCase) SpeedResult {
	t.Helper()
	ind, f, side := c.build()
	got, err := FinalSpeed(ind, f, side)
	if err != nil {
		t.Fatalf("FinalSpeed(%+v): %v", c, err)
	}
	return got
}

type speedParityCase struct {
	name string
	in   speedCase
	want int
	// naive は補正ごとに丸めた・まひを先に掛けた等の誤った実装の値(judge のテストと同じ。want と違うときだけ見る)。
	naive int
}

func runSpeedParity(t *testing.T, tests []speedParityCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustFinalSpeed(t, tt.in).Speed
			if got != tt.want {
				t.Errorf("FinalSpeed = %d, want %d", got, tt.want)
			}
			if tt.naive != 0 && tt.naive != tt.want && got == tt.naive {
				t.Errorf("FinalSpeed = %d は補正ごとに丸めた・順序を誤った値(連結 → 1 回丸め → まひ。ADR-0179 §2)", got)
			}
		})
	}
}

// S5: judge の TestSpeed。
func TestFinalSpeedJudgeParityBasic(t *testing.T) {
	n := NatureNeutral
	runSpeedParity(t, []speedParityCase{
		{"judge: 無振り・無補正", speedCase{base: 100, nature: n}, 120, 0},
		{"judge: 最速(SP32・上昇補正)", speedCase{base: 100, nature: speNaturePlus, sp: Stats{Spe: 32}}, 167, 0},
		{"judge: 下降補正", speedCase{base: 100, nature: speNatureMinus}, 108, 0},
		{"judge: 準速(SP32・無補正)", speedCase{base: 100, nature: n, sp: Stats{Spe: 32}}, 152, 0},
		{"judge: ランク +1", speedCase{base: 100, nature: n, ranks: Ranks{Spe: 1}}, 180, 0},
		{"judge: ランク -1", speedCase{base: 100, nature: n, ranks: Ranks{Spe: -1}}, 80, 0},
		{"judge: ランク +6", speedCase{base: 100, nature: n, ranks: Ranks{Spe: 6}}, 480, 0},
		{"judge: ランク -6", speedCase{base: 100, nature: n, ranks: Ranks{Spe: -6}}, 30, 0},
		{"judge: こだわりスカーフ", speedCase{base: 100, nature: n, scarf: true}, 180, 0},
		{"judge: 素早さ以外のランクは無視", speedCase{base: 100, nature: n, ranks: Ranks{Atk: 6, Def: -6}}, 120, 0},
		{"judge: ランク +1 とこだわりスカーフ", speedCase{base: 100, nature: n, ranks: Ranks{Spe: 1}, scarf: true}, 270, 0},
		{"judge: こだわりスカーフの五捨五超入(ちょうど .5 は切り捨て)", speedCase{base: 71, nature: n, scarf: true}, 136, 137},
		{"judge: こだわりスカーフの五捨五超入(別の .5)", speedCase{base: 73, nature: n, scarf: true}, 139, 140},
		{"judge: SP 合計がちょうど 66(境界値・受け付ける)", speedCase{base: 100, nature: n, sp: Stats{Spe: 32, Atk: 32, HP: 2}}, 152, 0},
	})
}

// S5: judge の TestSpeedTailwind・TestSpeedTailwindAndChoiceScarfChainBeforeRounding・TestSpeedSingleModifiersUnchangedFromJD1。
func TestFinalSpeedJudgeParityTailwindAndScarf(t *testing.T) {
	n := NatureNeutral
	runSpeedParity(t, []speedParityCase{
		{"judge: 無振り・無補正 + 追い風", speedCase{base: 100, nature: n, tailwind: true}, 240, 0},
		{"judge: 最速 + 追い風", speedCase{base: 100, nature: speNaturePlus, sp: Stats{Spe: 32}, tailwind: true}, 334, 0},
		{"judge: ランク +1 + 追い風", speedCase{base: 100, nature: n, ranks: Ranks{Spe: 1}, tailwind: true}, 360, 0},
		{"judge: ランク -1 + 追い風", speedCase{base: 100, nature: n, ranks: Ranks{Spe: -1}, tailwind: true}, 160, 0},
		{"judge: 奇数の実数値 + 追い風", speedCase{base: 71, nature: n, tailwind: true}, 182, 0},
		{"judge: 追い風なしは JD1 と同じ", speedCase{base: 100, nature: n}, 120, 0},
		{"judge: 実数値 91 + スカーフ + 追い風", speedCase{base: 71, nature: n, scarf: true, tailwind: true}, 273, 272},
		{"judge: 実数値 93 + スカーフ + 追い風", speedCase{base: 73, nature: n, scarf: true, tailwind: true}, 279, 278},
		{"judge: 実数値 167 + スカーフ + 追い風", speedCase{base: 100, nature: speNaturePlus, sp: Stats{Spe: 32}, scarf: true, tailwind: true}, 501, 500},
		{"judge: 実数値 120 + スカーフ + 追い風(偶数なので差は出ない)", speedCase{base: 100, nature: n, scarf: true, tailwind: true}, 360, 0},
		{"judge: ランク +1 + スカーフ + 追い風", speedCase{base: 100, nature: n, ranks: Ranks{Spe: 1}, scarf: true, tailwind: true}, 540, 0},
		{"judge: スカーフ単独(167 → 250.5 → 250)", speedCase{base: 100, nature: speNaturePlus, sp: Stats{Spe: 32}, scarf: true}, 250, 251},
		{"judge: 追い風単独", speedCase{base: 71, nature: n, tailwind: true}, 182, 0},
		{"judge: 補正なし", speedCase{base: 71, nature: n}, 91, 0},
	})
}

// S5: judge の TestSpeedWithAbilityAndItemEffects・TestSpeedWithoutEffectsUnchanged。
// judge の Env.Weather / Env.Terrain は Field に、Paralysis と Env.HasStatus は Status に写す
// (has_status だけが要るケースは StatusBurn。judge の元のケース名「やけど」と同じ)。
func TestFinalSpeedJudgeParityEffects(t *testing.T) {
	n := NatureNeutral
	odd91 := speedCase{base: 71, nature: n}
	odd93 := speedCase{base: 73, nature: n}
	with := func(c speedCase, f func(*speedCase)) speedCase { f(&c); return c }

	runSpeedParity(t, []speedParityCase{
		{"judge: 雨で ×2 の特性・雨", with(odd91, func(c *speedCase) { c.ability = speRainDouble; c.weather = WeatherRain }), 182, 0},
		{"judge: 雨で ×2 の特性・天候なしは掛けない", with(odd91, func(c *speedCase) { c.ability = speRainDouble }), 91, 0},
		{"judge: 雨で ×2 の特性・晴れは掛けない", with(odd91, func(c *speedCase) { c.ability = speRainDouble; c.weather = WeatherSun }), 91, 0},
		{"judge: 晴れで ×2", with(odd91, func(c *speedCase) { c.ability = speSunDouble; c.weather = WeatherSun }), 182, 0},
		{"judge: 砂で ×2", with(odd91, func(c *speedCase) { c.ability = speSandDouble; c.weather = WeatherSand }), 182, 0},
		{"judge: 雪で ×2", with(odd91, func(c *speedCase) { c.ability = speSnowDouble; c.weather = WeatherSnow }), 182, 0},
		{"judge: エレキフィールドで ×2", with(odd91, func(c *speedCase) { c.ability = speElecDouble; c.terrain = TerrainElectric }), 182, 0},
		{"judge: エレキ以外のフィールドは掛けない", with(odd91, func(c *speedCase) { c.ability = speElecDouble; c.terrain = TerrainPsychic }), 91, 0},
		{"judge: 状態異常で ×1.5(やけど)", with(odd91, func(c *speedCase) { c.ability = speStatusBoost; c.status = StatusBurn }), 136, 0},
		{"judge: 状態異常で ×1.5 の特性・状態異常なし", with(odd91, func(c *speedCase) { c.ability = speStatusBoost }), 91, 0},
		{"judge: まひ + まひの半減を受けない特性", with(odd91, func(c *speedCase) { c.ability = speStatusBoost; c.status = StatusParalysis }), 136, 68},
		{"judge: 持ち物を失った後の特性は掛けない", with(odd91, func(c *speedCase) { c.ability = speItemLostDouble }), 91, 0},
		{"judge: 常に ×0.5 の持ち物(91)", with(odd91, func(c *speedCase) { c.item = speAlwaysHalfItem }), 45, 0},
		{"judge: 常に ×0.5 の持ち物(93)", with(odd93, func(c *speedCase) { c.item = speAlwaysHalfItem }), 46, 0},
		{"judge: 状態異常 ×1.5 + 持ち物 ×0.5 は連結してから 1 回丸める",
			with(odd93, func(c *speedCase) { c.ability = speStatusBoost; c.status = StatusBurn; c.item = speAlwaysHalfItem }), 70, 69},
		{"judge: まひ + 半減を受けない特性 + 持ち物 ×0.5(半減しない)",
			with(odd93, func(c *speedCase) { c.ability = speStatusBoost; c.status = StatusParalysis; c.item = speAlwaysHalfItem }), 70, 35},
		{"judge: まひ + 持ち物 ×0.5(特性なし)", with(odd93, func(c *speedCase) { c.status = StatusParalysis; c.item = speAlwaysHalfItem }), 23, 0},
		{"judge: 追い風 + まひ + 半減を受けない特性",
			with(odd93, func(c *speedCase) { c.tailwind = true; c.ability = speStatusBoost; c.status = StatusParalysis }), 279, 139},
		{"judge: 雨 ×2 の特性 + こだわりスカーフ",
			with(odd93, func(c *speedCase) { c.ability = speRainDouble; c.weather = WeatherRain; c.scarf = true }), 279, 278},
		{"judge: 追い風 + 雨 ×2 の特性",
			with(odd93, func(c *speedCase) { c.tailwind = true; c.ability = speRainDouble; c.weather = WeatherRain }), 372, 0},
		{"judge: 追い風 + 持ち物 ×0.5", with(odd93, func(c *speedCase) { c.tailwind = true; c.item = speAlwaysHalfItem }), 93, 92},
		{"judge: 追い風 + 雨 ×2 + 持ち物 ×0.5",
			with(odd93, func(c *speedCase) {
				c.tailwind = true
				c.ability = speRainDouble
				c.weather = WeatherRain
				c.item = speAlwaysHalfItem
			}), 186, 0},
		{"judge: ランク +1 + 追い風 + 状態異常 ×1.5 + 持ち物 ×0.5",
			with(odd93, func(c *speedCase) {
				c.ranks = Ranks{Spe: 1}
				c.tailwind = true
				c.ability = speStatusBoost
				c.status = StatusBurn
				c.item = speAlwaysHalfItem
			}), 208, 0},
		{"judge: スカーフ判定が真なら持ち物の効果データは掛けない",
			with(odd93, func(c *speedCase) { c.scarf = true; c.item = speAlwaysHalfItem }), 139, 70},
		// TestSpeedWithoutEffectsUnchanged
		{"judge: 天候・場・状態異常だけでは変わらない",
			with(odd91, func(c *speedCase) { c.weather = WeatherRain; c.terrain = TerrainElectric; c.status = StatusBurn }), 91, 0},
		{"judge: まひは従来どおり半減", with(odd91, func(c *speedCase) { c.status = StatusParalysis }), 45, 0},
	})
}

// S6: 成立を判定できない条件(item_lost・語彙に無い条件)は、既存の finalSpeed(oracle の既定)と同じく飛ばして次の要素へ進み、
// 判定できなかったことを *Undetermined で返す。掛けた要素より前に無ければ立てない(ADR-0179 §2)。
func TestFinalSpeedUndetermined(t *testing.T) {
	odd91 := speedCase{base: 71, nature: NatureNeutral}
	mod := func(c SpeedCondition, m int) SpeedMod { return SpeedMod{Condition: c, Modifier: m} }
	ability := func(mods ...SpeedMod) *AbilityEffect { return &AbilityEffect{SpeedMods: mods} }
	item := func(mods ...SpeedMod) *ItemEffect { return &ItemEffect{SpeedMods: mods} }
	with := func(c speedCase, f func(*speedCase)) speedCase { f(&c); return c }

	tests := []struct {
		name                  string
		in                    speedCase
		wantSpeed             int
		wantAbility, wantItem bool
	}{
		{"効果なしは立てない", odd91, 91, false, false},
		{"item_lost だけ: 掛けずに立てる(judge と同じ値)",
			with(odd91, func(c *speedCase) { c.ability = ability(mod(SpeedConditionItemLost, 8192)) }), 91, true, false},
		{"成立した要素が前にあれば立てない",
			with(odd91, func(c *speedCase) {
				c.ability = ability(mod(SpeedConditionAlways, 2048), mod(SpeedConditionItemLost, 8192))
			}), 45, false, false},
		// judge(ResolveSpeedMod)はここで止めて何も掛けない(91)。engine は oracle と既存の finalSpeed に合わせて次へ進む(ADR-0179 §2)。
		{"item_lost の後ろの成立した要素は掛け、立てる(judge と違う唯一の並び)",
			with(odd91, func(c *speedCase) {
				c.ability = ability(mod(SpeedConditionItemLost, 8192), mod(SpeedConditionAlways, 2048))
			}), 45, true, false},
		{"不成立は確定なので立てない",
			with(odd91, func(c *speedCase) { c.ability = ability(mod(SpeedConditionWeatherRain, 8192)) }), 91, false, false},
		{"語彙に無い条件(特性)は立てる",
			with(odd91, func(c *speedCase) {
				c.ability = ability(mod(SpeedCondition("weather_fog"), 8192))
				c.weather = WeatherRain
			}),
			91, true, false},
		{"語彙に無い条件(持ち物)は持ち物の側に立てる",
			with(odd91, func(c *speedCase) { c.item = item(mod(SpeedCondition("weather_fog"), 2048)) }), 91, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustFinalSpeed(t, tt.in)
			if got.Speed != tt.wantSpeed || got.AbilityUndetermined != tt.wantAbility || got.ItemUndetermined != tt.wantItem {
				t.Errorf("FinalSpeed = %+v, want {Speed:%d AbilityUndetermined:%v ItemUndetermined:%v}",
					got, tt.wantSpeed, tt.wantAbility, tt.wantItem)
			}
		})
	}
}

// S7: judge の TestSpeedRejectsOutOfRangeInput。engine は 1 つの sentinel(ErrInvalidSpeedInput)で返す。
// 種族値は素早さだけを見る(タイプ・他の種族値が空でも素早さは求められる)。
func TestFinalSpeedRejectsOutOfRange(t *testing.T) {
	n := NatureNeutral
	tests := []struct {
		name string
		in   speedCase
	}{
		{"judge: 種族値が 0", speedCase{base: 0, nature: n}},
		{"judge: 種族値が負", speedCase{base: -1, nature: n}},
		{"judge: 種族値が 255 超", speedCase{base: 256, nature: n}},
		{"judge: SP が負", speedCase{base: 100, nature: n, sp: Stats{Spe: -1}}},
		{"judge: SP が 32 超", speedCase{base: 100, nature: n, sp: Stats{Spe: 33}}},
		{"judge: 素早さ以外の SP も範囲外なら弾く", speedCase{base: 100, nature: n, sp: Stats{Atk: 33}}},
		{"judge: SP の合計が 66 超", speedCase{base: 100, nature: n, sp: Stats{HP: 32, Atk: 32, Spe: 32}}},
		{"judge: SP の合計がちょうど 67(境界値・拒否する)", speedCase{base: 100, nature: n, sp: Stats{Spe: 32, Atk: 32, HP: 3}}},
		{"judge: ランクが -6 未満", speedCase{base: 100, nature: n, ranks: Ranks{Spe: -7}}},
		{"judge: ランクが +6 超", speedCase{base: 100, nature: n, ranks: Ranks{Spe: 7}}},
		{"judge: 素早さ以外のランクも範囲外なら弾く", speedCase{base: 100, nature: n, ranks: Ranks{Atk: 7}}},
		{"性格補正が HP", speedCase{base: 100, nature: Nature{Plus: StatHP, Minus: StatAtk}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ind, f, side := tt.in.build()
			got, err := FinalSpeed(ind, f, side)
			if !errors.Is(err, ErrInvalidSpeedInput) {
				t.Errorf("err = %v, want ErrInvalidSpeedInput", err)
			}
			if got != (SpeedResult{}) {
				t.Errorf("エラーのときはゼロ値: got %+v", got)
			}
		})
	}

	t.Run("境界は受け付ける(種族値 1・255、ランク ±6)", func(t *testing.T) {
		for _, c := range []speedCase{
			{base: 1, nature: n}, {base: 255, nature: n},
			{base: 100, nature: n, ranks: Ranks{Atk: 6, Def: -6, SpA: 6, SpD: -6, Spe: 6}},
		} {
			ind, f, side := c.build()
			if _, err := FinalSpeed(ind, f, side); err != nil {
				t.Errorf("FinalSpeed(%+v): %v", c, err)
			}
		}
	})
}

// S8: 追い風なしの FinalSpeed は、ジャイロボール等の威力の式が使う既存の finalSpeed(ADR-0143 §1)と全入力で一致する。
// finalSpeed を FinalSpeed に寄せてもゴールデンが変わらないことの前提(ADR-0179 §2)。
func TestFinalSpeedMatchesInternalFinalSpeed(t *testing.T) {
	abilities := []*AbilityEffect{nil, speRainDouble, speStatusBoost, speItemLostDouble, speElecDouble,
		{SpeedMods: []SpeedMod{{Condition: SpeedConditionItemLost, Modifier: 8192}, {Condition: SpeedConditionAlways, Modifier: 2048}}}}
	items := []*ItemEffect{nil, speAlwaysHalfItem, speScarfItem}
	statuses := []Status{"", StatusNone, StatusParalysis, StatusBurn}
	fields := []Field{{}, {Weather: WeatherRain}, {Weather: WeatherSun, Terrain: TerrainElectric}}
	natures := []Nature{NatureNeutral, speNaturePlus, speNatureMinus}

	count := 0
	for _, base := range []int{1, 30, 71, 73, 100, 142, 200, 255} {
		for _, nature := range natures {
			for _, spe := range []int{0, 1, 31, 32} {
				for rank := -6; rank <= 6; rank += 3 {
					for _, ae := range abilities {
						for _, ie := range items {
							for _, st := range statuses {
								for _, f := range fields {
									c := speedCase{base: base, nature: nature, sp: Stats{Spe: spe}, ranks: Ranks{Spe: rank},
										ability: ae, item: ie, status: st}
									ind, _, _ := c.build()
									got, err := FinalSpeed(ind, f, SpeedSide{})
									if err != nil {
										t.Fatalf("FinalSpeed(%+v): %v", c, err)
									}
									if want := finalSpeed(ind, f); got.Speed != want {
										t.Fatalf("FinalSpeed = %d, finalSpeed = %d (%+v, field %+v)", got.Speed, want, c, f)
									}
									count++
								}
							}
						}
					}
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("比較した組が 0")
	}
}

// S9: judge の TestCompareSpeed と トリックルームの表(ADR-0702 §3)。
// judge の SpeedComparison{Outspeeds, SpeedTie} は 1 つの語彙 TurnOrder にした(ADR-0179 §3)。
func TestCompareSpeed(t *testing.T) {
	tests := []struct {
		name      string
		self, opp int
		trickRoom bool
		want      TurnOrder
	}{
		{"judge: 自分の方が速い", 167, 120, false, TurnOrderSelfFirst},
		{"judge: 自分の方が遅い", 120, 167, false, TurnOrderOpponentFirst},
		{"judge: 同速", 120, 120, false, TurnOrderSpeedTie},
		{"judge: こだわりスカーフで抜ける", 180, 167, false, TurnOrderSelfFirst},
		{"judge: 相手のスカーフも考慮する", 167, 180, false, TurnOrderOpponentFirst},
		{"judge: トリックルーム中: 速い方が後になる", 167, 120, true, TurnOrderOpponentFirst},
		{"judge: トリックルーム中: 遅い方が先に動く", 120, 167, true, TurnOrderSelfFirst},
		{"judge: 同速はトリックルーム中でも反転しない", 120, 120, true, TurnOrderSpeedTie},
		{"judge: 追い風で速くなってもトリックルームでは後", 240, 167, true, TurnOrderOpponentFirst},
		{"1 差で抜ける(連続技の 198 と 197)", 198, 197, false, TurnOrderSelfFirst},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareSpeed(tt.self, tt.opp, tt.trickRoom); got != tt.want {
				t.Errorf("CompareSpeed(%d, %d, %v) = %q, want %q", tt.self, tt.opp, tt.trickRoom, got, tt.want)
			}
		})
	}

	// FinalSpeed → CompareSpeed をつないだ値も judge の CompareSpeed と同じ(fast 167・neutral 120・scarfed 180・追い風 240)。
	t.Run("FinalSpeed とつなぐ", func(t *testing.T) {
		fast := mustFinalSpeed(t, speedCase{base: 100, nature: speNaturePlus, sp: Stats{Spe: 32}}).Speed
		neutral := mustFinalSpeed(t, speedCase{base: 100, nature: NatureNeutral}).Speed
		scarfed := mustFinalSpeed(t, speedCase{base: 100, nature: NatureNeutral, scarf: true}).Speed
		tailwind := mustFinalSpeed(t, speedCase{base: 100, nature: NatureNeutral, tailwind: true}).Speed
		if fast != 167 || neutral != 120 || scarfed != 180 || tailwind != 240 {
			t.Fatalf("素早さ = %d/%d/%d/%d, want 167/120/180/240", fast, neutral, scarfed, tailwind)
		}
		if got := CompareSpeed(scarfed, fast, false); got != TurnOrderSelfFirst {
			t.Errorf("スカーフ 180 対 167 = %q", got)
		}
	})
}

// S10: judge の TestCompareTurnOrderPriorityWins。優先度が違えば素早さの比較の結果を見ない。
func TestCompareTurnOrderPriorityWins(t *testing.T) {
	tests := []struct {
		name                      string
		selfPriority, oppPriority int
		bySpeed                   TurnOrder
		want                      TurnOrder
	}{
		{"judge: 遅くても優先度が高ければ先に動く", 1, 0, TurnOrderOpponentFirst, TurnOrderSelfFirst},
		{"judge: 速くても優先度が低ければ後になる", 0, 1, TurnOrderSelfFirst, TurnOrderOpponentFirst},
		{"judge: 同速でも優先度が高ければ先に動き、tie にはならない", 1, 0, TurnOrderSpeedTie, TurnOrderSelfFirst},
		{"judge: 同速で優先度が低ければ後になり、tie にはならない", 0, 1, TurnOrderSpeedTie, TurnOrderOpponentFirst},
		{"judge: 負の優先度どうしでも高い方が先に動く", -1, -6, TurnOrderOpponentFirst, TurnOrderSelfFirst},
		{"judge: 負の優先度どうしで低い方は後になる", -6, -1, TurnOrderSelfFirst, TurnOrderOpponentFirst},
		{"judge: 相手だけが先制技(優先度 0 対 +1)", 0, 1, TurnOrderOpponentFirst, TurnOrderOpponentFirst},
		{"judge: どちらも同じ先制技なら優先度では決まらない(速い方が先)", 1, 1, TurnOrderSelfFirst, TurnOrderSelfFirst},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareTurnOrder(tt.selfPriority, tt.oppPriority, tt.bySpeed); got != tt.want {
				t.Errorf("CompareTurnOrder(%d, %d, %q) = %q, want %q", tt.selfPriority, tt.oppPriority, tt.bySpeed, got, tt.want)
			}
		})
	}

	// judge の TestCompareTurnOrderIgnoresSpeedValuesWhenPrioritiesDiffer。
	for _, s := range []TurnOrder{TurnOrderSelfFirst, TurnOrderOpponentFirst, TurnOrderSpeedTie, ""} {
		if got := CompareTurnOrder(1, 0, s); got != TurnOrderSelfFirst {
			t.Errorf("CompareTurnOrder(1, 0, %q) = %q(優先度が違えば素早さを見ない)", s, got)
		}
		if got := CompareTurnOrder(0, 1, s); got != TurnOrderOpponentFirst {
			t.Errorf("CompareTurnOrder(0, 1, %q) = %q(優先度が違えば素早さを見ない)", s, got)
		}
	}
}

// S10: judge の TestCompareTurnOrderFallsBackToSpeed(優先度 -6・0・1・5 どうし)と
// TestCompareTurnOrderMatchesCompareSpeedUnderTrickRoom。
func TestCompareTurnOrderFallsBackToSpeed(t *testing.T) {
	for _, p := range []int{-6, 0, 1, 5} {
		for _, s := range []TurnOrder{TurnOrderSelfFirst, TurnOrderOpponentFirst, TurnOrderSpeedTie} {
			if got := CompareTurnOrder(p, p, s); got != s {
				t.Errorf("CompareTurnOrder(%d, %d, %q) = %q, want %q", p, p, s, got, s)
			}
		}
	}

	const fast, slow = 167, 120
	tests := []struct {
		name                      string
		self, opp                 int
		trickRoom                 bool
		selfPriority, oppPriority int
		want                      TurnOrder
	}{
		{"judge: 同優先度・通常: 速い方が先", fast, slow, false, 0, 0, TurnOrderSelfFirst},
		{"judge: 同優先度・トリックルーム: 速い方が後", fast, slow, true, 0, 0, TurnOrderOpponentFirst},
		{"judge: 同優先度・トリックルーム: 遅い方が先", slow, fast, true, 0, 0, TurnOrderSelfFirst},
		{"judge: 優先度が高い側はトリックルームでも先", slow, fast, false, 1, 0, TurnOrderSelfFirst},
		{"judge: 優先度が高い側はトリックルーム中でも先", slow, fast, true, 1, 0, TurnOrderSelfFirst},
		{"judge: 優先度が低い側はトリックルーム中でも後", fast, slow, true, 0, 1, TurnOrderOpponentFirst},
		{"judge: 優先度が低い側は通常でも後", fast, slow, false, 0, 1, TurnOrderOpponentFirst},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareTurnOrder(tt.selfPriority, tt.oppPriority, CompareSpeed(tt.self, tt.opp, tt.trickRoom))
			if got != tt.want {
				t.Errorf("行動順 = %q, want %q", got, tt.want)
			}
		})
	}
}

// S11: 技の優先度 + 技の処理の定義の PriorityBoost(使用者が接地していて、そのフィールド)。applyRulePreconditions と同じ式。
func TestEffectivePriority(t *testing.T) {
	boost := &MoveRule{PriorityBoost: &PriorityBoost{Terrain: TerrainGrassy, Delta: 1}}
	grounded := Individual{Species: Species{Types: []Type{TypeNormal}, BaseStats: Stats{Spe: 100}}}
	flying := Individual{Species: Species{Types: []Type{TypeFlying}, BaseStats: Stats{Spe: 100}}}
	flyingIronBall := flying
	flyingIronBall.Item = &Item{ID: "test-grounds", Effect: &ItemEffect{Grounds: true}}

	tests := []struct {
		name string
		move Move
		user Individual
		f    Field
		want int
	}{
		{"定義なし・優先度 0", Move{Category: CategoryPhysical, Power: 80}, grounded, Field{}, 0},
		{"定義なし・先制技 +1(かげうち型)", Move{Category: CategoryPhysical, Power: 40, Priority: 1}, grounded, Field{}, 1},
		{"定義なし・負の優先度", Move{Category: CategoryPhysical, Power: 150, Priority: -3}, grounded, Field{}, -3},
		{"PriorityBoost・接地・グラスフィールド", Move{Category: CategoryPhysical, Power: 55, Rule: boost}, grounded, Field{Terrain: TerrainGrassy}, 1},
		{"PriorityBoost・接地・フィールドなし", Move{Category: CategoryPhysical, Power: 55, Rule: boost}, grounded, Field{}, 0},
		{"PriorityBoost・接地・別のフィールド", Move{Category: CategoryPhysical, Power: 55, Rule: boost}, grounded, Field{Terrain: TerrainElectric}, 0},
		{"PriorityBoost・ひこうタイプは浮いている", Move{Category: CategoryPhysical, Power: 55, Rule: boost}, flying, Field{Terrain: TerrainGrassy}, 0},
		{"PriorityBoost・接地させる持ち物", Move{Category: CategoryPhysical, Power: 55, Rule: boost}, flyingIronBall, Field{Terrain: TerrainGrassy}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectivePriority(tt.move, tt.user, tt.f); got != tt.want {
				t.Errorf("EffectivePriority = %d, want %d", got, tt.want)
			}
			// applyRulePreconditions(ダメージ計算が使う優先度)と同じ値。
			in := applyRulePreconditions(DamageInput{Attacker: tt.user, Move: tt.move, Field: tt.f})
			if in.Move.Priority != tt.want {
				t.Errorf("applyRulePreconditions の優先度 = %d, EffectivePriority = %d", in.Move.Priority, tt.want)
			}
		})
	}
}
