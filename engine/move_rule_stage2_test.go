package engine

// 技の機構の段階2(ADR-0143)の受け入れ条件。
//
// 既存の入力(両側の個体・状態異常・持ち物・特性・ランク・場)と種族の重さだけで決まる技の処理を、閉じた語彙
// (MoveRule)で表して engine が計算し、未対応の印(ADR-0123)を外す。
// 正解は「同じ結果になるはずの通常の入力」との一致(等価な入力)と、式から手で導いた値で決める(式を再実装しない)。
// oracle との一致はゴールデン(golden_mechanisms_stage2_test.go)が見る。種族・技・特性はすべて架空(coding-rules §1)。

import (
	"errors"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

// s2Input は統制ケース(ctrlInput: 実数値 A200 / B100・攻撃側 water・防御側 psychic)の技を差し替えたもの。
func s2Input(cat MoveCategory, typ Type, power int) DamageInput {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, cat, typ)
	in.Move.Power = power
	return in
}

// withRule は技に機構と定義を載せる(ID は架空)。
func withRule(in DamageInput, mechs []MoveMechanism, r MoveRule) DamageInput {
	in.Move.ID = "test-rule-move"
	in.Move.Mechanisms = mechs
	in.Move.Rule = &r
	return in
}

// plain は同じ入力で、機構と定義を持たない通常の技(威力・タイプを差し替える)にしたもの。
func plain(in DamageInput, typ Type, power int) DamageInput {
	in.Move.ID = "test-plain-move"
	in.Move.Mechanisms = nil
	in.Move.Rule = nil
	in.Move.Params = MechanismParams{}
	in.Move.Type = typ
	in.Move.Power = power
	return in
}

func mustCalcS2(t *testing.T, in DamageInput) DamageResult {
	t.Helper()
	r, err := calcDamage(in)
	if err != nil {
		t.Fatalf("CalcDamage: %v", err)
	}
	return r
}

// assertSameAsPlain は got が「等価な通常の入力」want と同じ16段階・相性で、印が無いこと。
func assertSameAsPlain(t *testing.T, gotIn, wantIn DamageInput) DamageResult {
	t.Helper()
	got := mustCalcS2(t, gotIn)
	want := mustCalcS2(t, wantIn)
	if got.Rolls != want.Rolls {
		t.Errorf("rolls = %v, want %v(等価な通常の入力: 威力 %d・タイプ %s)", got.Rolls, want.Rolls, wantIn.Move.Power, wantIn.Move.Type)
	}
	if got.Effectiveness != want.Effectiveness {
		t.Errorf("相性 = %v, want %v", got.Effectiveness, want.Effectiveness)
	}
	if got.Unsupported != nil {
		t.Errorf("段階2で計算する技に印が付いた: %v", got.Unsupported)
	}
	return got
}

var allStatuses = []Status{StatusBurn, StatusParalysis, StatusPoison, StatusBadlyPoison, StatusSleep, StatusFreeze}

// ---------------------------------------------------------------------------
// 語彙(閉じた型)
// ---------------------------------------------------------------------------

func TestMoveRuleVocabulary(t *testing.T) {
	wantFormulas := []PowerFormula{
		PowerFormulaPositiveBoosts, PowerFormulaSpeedRatio, PowerFormulaInverseSpeedRatio,
		PowerFormulaTargetWeight, PowerFormulaWeightRatio, PowerFormulaHitIndex,
		// 段階3(ADR-0144)で足した式。定義順の末尾に足す(値は TestStage3Vocabulary が見る)。
		PowerFormulaAttackerHPScaled, PowerFormulaAttackerHPLow, PowerFormulaDefenderHPRatio, PowerFormulaAttackerItemFling,
	}
	if got := AllPowerFormulas(); !reflect.DeepEqual(got, wantFormulas) {
		t.Errorf("AllPowerFormulas = %v, want %v(定義順)", got, wantFormulas)
	}
	wantValues := []string{"attacker_positive_boosts", "speed_ratio", "inverse_speed_ratio", "target_weight", "weight_ratio", "hit_index",
		"attacker_hp_scaled", "attacker_hp_low", "defender_hp_ratio", "attacker_item_fling"}
	for i, f := range wantFormulas {
		if string(f) != wantValues[i] || !f.Known() {
			t.Errorf("PowerFormula %q: 値 %q・Known が真であること", f, wantValues[i])
		}
	}
	wantConds := []MoveCondition{
		MoveConditionAttackerStatus, MoveConditionDefenderStatus, MoveConditionAttackerNoItem,
		MoveConditionDefenderItemRemovable, MoveConditionWeather,
		MoveConditionTerrainAttackerGrounded, MoveConditionTerrainDefenderGrounded,
	}
	if got := AllMoveConditions(); !reflect.DeepEqual(got, wantConds) {
		t.Errorf("AllMoveConditions = %v, want %v(定義順)", got, wantConds)
	}
	wantCondValues := []string{"attacker_status", "defender_status", "attacker_no_item", "defender_item_removable",
		"weather", "terrain_attacker_grounded", "terrain_defender_grounded"}
	for i, c := range wantConds {
		if string(c) != wantCondValues[i] || !c.Known() {
			t.Errorf("MoveCondition %q: 値 %q・Known が真であること", c, wantCondValues[i])
		}
	}
	for _, bad := range []string{"", "Speed_Ratio", "hp_ratio"} {
		if PowerFormula(bad).Known() || MoveCondition(bad).Known() {
			t.Errorf("%q を既知としてはいけない(大文字小文字を区別する)", bad)
		}
	}
	// 返り値はコピー(呼び出し側が書き換えても語彙は変わらない)。
	f := AllPowerFormulas()
	f[0] = "x"
	if AllPowerFormulas()[0] != PowerFormulaPositiveBoosts {
		t.Error("AllPowerFormulas がコピーを返していない")
	}
	if NullifyMoveFailed != "move_failed" {
		t.Errorf("NullifyMoveFailed = %q, want move_failed", NullifyMoveFailed)
	}
}

// ---------------------------------------------------------------------------
// 状態異常の条件(PowerBoosts の attacker_status・defender_status、IgnoresBurn)
// ---------------------------------------------------------------------------

func TestStage2StatusBoosts(t *testing.T) {
	facade := MoveRule{
		PowerBoosts: []MovePowerBoost{{Condition: MoveConditionAttackerStatus,
			Statuses: []Status{StatusBurn, StatusParalysis, StatusPoison, StatusBadlyPoison}, Modifier: 8192}},
		IgnoresBurn: true,
	}
	facadeNoIgnore := facade
	facadeNoIgnore.IgnoresBurn = false
	hex := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionDefenderStatus, Statuses: allStatuses, BaseMultiplier: 2}}}
	poisonMod := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionDefenderStatus,
		Statuses: []Status{StatusPoison, StatusBadlyPoison}, Modifier: 8192}}}
	vp := []MoveMechanism{MechanismVariablePower}

	cases := []struct {
		name      string
		rule      MoveRule
		power     int
		atkStatus Status
		defStatus Status
		// 等価な通常の入力: 威力と、攻撃側の状態(やけどの半減を受けない技は状態なしと同じ)。
		wantPower     int
		wantAtkStatus Status
	}{
		{"攻撃側やけど: 8192 で倍・やけどの半減を受けない", facade, 70, StatusBurn, StatusNone, 140, StatusNone},
		{"攻撃側まひ: 8192 で倍", facade, 70, StatusParalysis, StatusNone, 140, StatusParalysis},
		{"攻撃側猛毒: 8192 で倍", facade, 70, StatusBadlyPoison, StatusNone, 140, StatusBadlyPoison},
		{"攻撃側ねむり: 一覧に無いので通常", facade, 70, StatusSleep, StatusNone, 70, StatusSleep},
		{"攻撃側状態なし: 通常", facade, 70, StatusNone, StatusNone, 70, StatusNone},
		{"IgnoresBurn が無ければやけどの半減を受ける", facadeNoIgnore, 70, StatusBurn, StatusNone, 140, StatusBurn},
		{"防御側ねむり: 威力 ×2(整数)", hex, 65, StatusNone, StatusSleep, 130, StatusNone},
		{"防御側こおり: 威力 ×2", hex, 65, StatusNone, StatusFreeze, 130, StatusNone},
		{"防御側状態なし: 通常", hex, 65, StatusNone, StatusNone, 65, StatusNone},
		{"防御側猛毒: 毒の条件で 8192", poisonMod, 65, StatusNone, StatusBadlyPoison, 130, StatusNone},
		{"防御側やけど: 毒の条件は不成立", poisonMod, 65, StatusNone, StatusBurn, 65, StatusNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeNormal, c.power)
			in.Attacker.Status = c.atkStatus
			in.Defender.Status = c.defStatus
			want := plain(in, TypeNormal, c.wantPower)
			want.Attacker.Status = c.wantAtkStatus
			assertSameAsPlain(t, withRule(in, vp, c.rule), want)
		})
	}
}

// 威力の補正(Modifier)は連鎖の最初に入り、特性の条件(威力 60 以下)は補正前の威力で判定する(oracle の basePower)。
// 威力 60・条件成立の 6144 と テクニシャン相当の 6144: chainMods = 9216 → pokeRound(60 × 9216 / 4096) = 135。
// 整数倍(BaseMultiplier)は特性の条件の前に威力を変える: 威力 50 × 2 = 100 はテクニシャン相当の対象外。
func TestStage2PowerBoostChainPosition(t *testing.T) {
	technician := Ability{ID: "test-technician", Effect: &AbilityEffect{PowerMods: []ConditionalPowerMod{
		{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}}}
	vp := []MoveMechanism{MechanismVariablePower}

	t.Run("Modifier は連鎖の最初・条件は補正前の威力", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeNormal, 60)
		in.Attacker.Ability = technician
		rule := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionAttackerNoItem, Modifier: 6144}}}
		want := plain(in, TypeNormal, 135)
		want.Attacker.Ability = Ability{}
		assertSameAsPlain(t, withRule(in, vp, rule), want)
	})
	t.Run("BaseMultiplier は特性の条件より前", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeNormal, 50)
		in.Attacker.Ability = technician
		in.Defender.Status = StatusSleep
		rule := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionDefenderStatus, Statuses: allStatuses, BaseMultiplier: 2}}}
		want := plain(in, TypeNormal, 100)
		want.Attacker.Ability = Ability{}
		assertSameAsPlain(t, withRule(in, vp, rule), want)
	})
}

// ---------------------------------------------------------------------------
// 持ち物の条件(attacker_no_item・defender_item_removable)・持ち物が無いと失敗
// ---------------------------------------------------------------------------

func TestStage2ItemConditions(t *testing.T) {
	someItem := &Item{ID: "test-item"}
	megaStone := &Item{ID: "test-stone", MegaStone: true}
	vp := []MoveMechanism{MechanismVariablePower}
	noItem := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionAttackerNoItem, BaseMultiplier: 2}}}
	knock := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionDefenderItemRemovable, Modifier: 6144}}}

	t.Run("攻撃側が持ち物なし: 威力 ×2", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeFlying, 55)
		assertSameAsPlain(t, withRule(in, vp, noItem), plain(in, TypeFlying, 110))
	})
	t.Run("攻撃側が持ち物あり: 通常", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeFlying, 55)
		in.Attacker.Item = someItem
		assertSameAsPlain(t, withRule(in, vp, noItem), plain(in, TypeFlying, 55))
	})
	t.Run("防御側が持ち物あり: 6144(65 → 97.5 → 五捨五超入で 97)", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeDark, 65)
		in.Defender.Item = someItem
		assertSameAsPlain(t, withRule(in, vp, knock), plain(in, TypeDark, 97))
	})
	t.Run("防御側が持ち物なし: 通常", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeDark, 65)
		assertSameAsPlain(t, withRule(in, vp, knock), plain(in, TypeDark, 65))
	})
	t.Run("防御側がメガストーン: 補正なし・印を残す(防御側に合うかを engine は知らない)", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeDark, 65)
		in.Defender.Item = megaStone
		got := mustCalcS2(t, withRule(in, vp, knock))
		want := mustCalcS2(t, plain(in, TypeDark, 65))
		if got.Rolls != want.Rolls {
			t.Errorf("rolls = %v, want %v(補正なし)", got.Rolls, want.Rolls)
		}
		wantMarks := []UnsupportedMark{{Target: UnsupportedTargetMove, Reason: UnsupportedReason(MechanismVariablePower), ID: "test-rule-move"}}
		if !reflect.DeepEqual(got.Unsupported, wantMarks) {
			t.Errorf("Unsupported = %v, want %v", got.Unsupported, wantMarks)
		}
	})

	fails := MoveRule{FailsWithoutDefenderItem: true, MoveSpecificResolved: true}
	ms := []MoveMechanism{MechanismMoveSpecific}
	t.Run("防御側が持ち物なしなら失敗(move_failed・印なし)", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGhost, 110)
		got := mustCalcS2(t, withRule(in, ms, fails))
		if got.Rolls != ([16]int{}) || got.Nullified != NullifyMoveFailed {
			t.Errorf("rolls = %v・Nullified = %q, want 0・move_failed", got.Rolls, got.Nullified)
		}
		if got.KO != (KOChance{}) || got.Unsupported != nil {
			t.Errorf("KO = %+v・Unsupported = %v, want ゼロ値・印なし", got.KO, got.Unsupported)
		}
	})
	t.Run("防御側が持ち物ありなら通常", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGhost, 110)
		in.Defender.Item = someItem
		assertSameAsPlain(t, withRule(in, ms, fails), plain(in, TypeGhost, 110))
	})
	t.Run("タイプ相性の無効が先(Nullified は空)", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGhost, 110)
		in.Defender.Species.Types = []Type{TypeNormal}
		got := mustCalcS2(t, withRule(in, ms, fails))
		if got.Effectiveness != 0 || got.Nullified != NullifyNone {
			t.Errorf("相性 = %v・Nullified = %q, want 0・空", got.Effectiveness, got.Nullified)
		}
	})
	t.Run("特性の無効より先", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGhost, 110)
		in.Defender.Ability = Ability{ID: "test-ghostproof", Effect: &AbilityEffect{DefImmuneTypes: []Type{TypeGhost}}}
		if got := mustCalcS2(t, withRule(in, ms, fails)); got.Nullified != NullifyMoveFailed {
			t.Errorf("Nullified = %q, want move_failed", got.Nullified)
		}
	})
}

// ---------------------------------------------------------------------------
// 天候(weather・TypeByWeather)
// ---------------------------------------------------------------------------

func weatherBallRule() MoveRule {
	return MoveRule{
		PowerBoosts: []MovePowerBoost{{Condition: MoveConditionWeather,
			Weathers: []Weather{WeatherSun, WeatherRain, WeatherSand, WeatherSnow}, BaseMultiplier: 2}},
		TypeByWeather:        map[Weather]Type{WeatherSun: TypeFire, WeatherRain: TypeWater, WeatherSand: TypeRock, WeatherSnow: TypeIce},
		MoveSpecificResolved: true,
	}
}

func TestStage2WeatherRules(t *testing.T) {
	mechs := []MoveMechanism{MechanismMoveSpecific, MechanismTypeChange}
	cases := []struct {
		weather  Weather
		wantType Type
		wantPow  int
	}{
		{WeatherSun, TypeFire, 100},
		{WeatherRain, TypeWater, 100},
		{WeatherSand, TypeRock, 100},
		{WeatherSnow, TypeIce, 100},
		{WeatherNone, TypeNormal, 50},
	}
	for _, c := range cases {
		t.Run("タイプと威力が天候で変わる/"+string(c.weather), func(t *testing.T) {
			in := s2Input(CategorySpecial, TypeNormal, 50)
			in.Field.Weather = c.weather
			assertSameAsPlain(t, withRule(in, mechs, weatherBallRule()), plain(in, c.wantType, c.wantPow))
		})
	}
	t.Run("スキン系の特性では変えない(type_change の技)", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeNormal, 50)
		in.Attacker.Ability = Ability{ID: "test-skin", Effect: &AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeFairy, PowerMod: 4915}}}
		want := plain(in, TypeNormal, 50)
		want.Attacker.Ability = Ability{}
		assertSameAsPlain(t, withRule(in, mechs, weatherBallRule()), want)
	})

	halve := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionWeather,
		Weathers: []Weather{WeatherRain, WeatherSand, WeatherSnow}, Modifier: 2048}}}
	for _, w := range []Weather{WeatherRain, WeatherSand, WeatherSnow} {
		t.Run("雨・砂・雪で威力半分/"+string(w), func(t *testing.T) {
			in := s2Input(CategorySpecial, TypeGrass, 120)
			in.Field.Weather = w
			assertSameAsPlain(t, withRule(in, []MoveMechanism{MechanismVariablePower}, halve), plain(in, TypeGrass, 60))
		})
	}
	t.Run("晴れでは通常", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeGrass, 120)
		in.Field.Weather = WeatherSun
		assertSameAsPlain(t, withRule(in, []MoveMechanism{MechanismVariablePower}, halve), plain(in, TypeGrass, 120))
	})
}

// ---------------------------------------------------------------------------
// フィールドと接地(terrain_*・TypeByTerrain・TerrainPowerMods・SpreadInTerrain・PriorityBoost)
// ---------------------------------------------------------------------------

var airborne = Ability{ID: "test-float", Effect: &AbilityEffect{Airborne: true}}

func terrainPulseRule() MoveRule {
	return MoveRule{
		PowerBoosts: []MovePowerBoost{{Condition: MoveConditionTerrainAttackerGrounded,
			Terrains: []Terrain{TerrainElectric, TerrainGrassy, TerrainMisty, TerrainPsychic}, BaseMultiplier: 2}},
		TypeByTerrain:        map[Terrain]Type{TerrainElectric: TypeElectric, TerrainGrassy: TypeGrass, TerrainMisty: TypeFairy, TerrainPsychic: TypePsychic},
		MoveSpecificResolved: true,
	}
}

func TestStage2TerrainRules(t *testing.T) {
	typeMechs := []MoveMechanism{MechanismMoveSpecific, MechanismTypeChange}
	vp := []MoveMechanism{MechanismVariablePower}

	for _, c := range []struct {
		terrain  Terrain
		wantType Type
	}{{TerrainElectric, TypeElectric}, {TerrainGrassy, TypeGrass}, {TerrainMisty, TypeFairy}, {TerrainPsychic, TypePsychic}} {
		t.Run("攻撃側が接地: フィールドのタイプ・威力 ×2/"+string(c.terrain), func(t *testing.T) {
			in := s2Input(CategorySpecial, TypeNormal, 50)
			in.Field.Terrain = c.terrain
			assertSameAsPlain(t, withRule(in, typeMechs, terrainPulseRule()), plain(in, c.wantType, 100))
		})
	}
	t.Run("攻撃側が浮いている: 変えない", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeNormal, 50)
		in.Field.Terrain = TerrainElectric
		in.Attacker.Ability = airborne
		assertSameAsPlain(t, withRule(in, typeMechs, terrainPulseRule()), plain(in, TypeNormal, 50))
	})
	t.Run("フィールドなし: 変えない", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeNormal, 50)
		assertSameAsPlain(t, withRule(in, typeMechs, terrainPulseRule()), plain(in, TypeNormal, 50))
	})

	rising := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionTerrainDefenderGrounded,
		Terrains: []Terrain{TerrainElectric}, BaseMultiplier: 2}}}
	t.Run("防御側が接地 × エレキフィールド: 威力 ×2", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeElectric, 70)
		in.Field.Terrain = TerrainElectric
		assertSameAsPlain(t, withRule(in, vp, rising), plain(in, TypeElectric, 140))
	})
	t.Run("防御側が浮いている: 通常", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeElectric, 70)
		in.Field.Terrain = TerrainElectric
		in.Defender.Ability = airborne
		assertSameAsPlain(t, withRule(in, vp, rising), plain(in, TypeElectric, 70))
	})

	misty := MoveRule{PowerBoosts: []MovePowerBoost{{Condition: MoveConditionTerrainAttackerGrounded,
		Terrains: []Terrain{TerrainMisty}, Modifier: 6144}}}
	t.Run("攻撃側が接地 × ミストフィールド: 6144", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeFairy, 100)
		in.Field.Terrain = TerrainMisty
		assertSameAsPlain(t, withRule(in, vp, misty), plain(in, TypeFairy, 150))
	})
	t.Run("攻撃側が浮いている × ミストフィールド: 通常", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypeFairy, 100)
		in.Field.Terrain = TerrainMisty
		in.Attacker.Ability = airborne
		assertSameAsPlain(t, withRule(in, vp, misty), plain(in, TypeFairy, 100))
	})

	// 技の補正 6144 とサイコフィールドの 5325 は連鎖する: chainMods(6144, 5325) = 7988 → pokeRound(80 × 7988 / 4096 = 156.02) = 156。
	// 等価な入力はフィールドなしの威力 156(フィールドは威力だけに効く)。
	expanding := MoveRule{
		PowerBoosts:          []MovePowerBoost{{Condition: MoveConditionTerrainAttackerGrounded, Terrains: []Terrain{TerrainPsychic}, Modifier: 6144}},
		SpreadInTerrain:      TerrainPsychic,
		MoveSpecificResolved: true,
	}
	efMechs := []MoveMechanism{MechanismMoveSpecific, MechanismVariablePower}
	t.Run("サイコフィールド: 技の補正とフィールドの補正が連鎖する", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypePsychic, 80)
		in.Field.Terrain = TerrainPsychic
		want := plain(in, TypePsychic, 156)
		want.Field.Terrain = TerrainNone
		assertSameAsPlain(t, withRule(in, efMechs, expanding), want)
	})
	t.Run("ダブル × サイコフィールド: 全体技の補正", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypePsychic, 80)
		in.Format = FormatDouble
		in.Move.Target = MoveTargetSingle
		in.Field.Terrain = TerrainPsychic
		want := plain(in, TypePsychic, 156)
		want.Field.Terrain = TerrainNone
		want.Move.Target = MoveTargetSpread
		assertSameAsPlain(t, withRule(in, efMechs, expanding), want)
	})
	t.Run("ダブル × フィールドなし: 単体のまま", func(t *testing.T) {
		in := s2Input(CategorySpecial, TypePsychic, 80)
		in.Format = FormatDouble
		in.Move.Target = MoveTargetSingle
		assertSameAsPlain(t, withRule(in, efMechs, expanding), plain(in, TypePsychic, 80))
	})

	quake := MoveRule{TerrainPowerMods: []TerrainPowerMod{{Terrain: TerrainGrassy, Modifier: 2048}}}
	fs := []MoveMechanism{MechanismFieldSpecific}
	t.Run("グラスフィールド × 防御側が接地: 威力半分", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGround, 100)
		in.Field.Terrain = TerrainGrassy
		assertSameAsPlain(t, withRule(in, fs, quake), plain(in, TypeGround, 50))
	})
	t.Run("グラスフィールド × 防御側が浮いている: 通常(無効の特性は持たない架空の浮遊)", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGround, 100)
		in.Field.Terrain = TerrainGrassy
		in.Defender.Ability = airborne
		assertSameAsPlain(t, withRule(in, fs, quake), plain(in, TypeGround, 100))
	})
	t.Run("天候があっても field_specific の印は付かない(中身がある)", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGround, 100)
		in.Field.Weather = WeatherSun
		assertSameAsPlain(t, withRule(in, fs, quake), plain(in, TypeGround, 100))
	})
}

// 優先度の変化(PriorityBoost)はサイコフィールドの判定に使う。グラスフィールドで +1 の技は、サイコフィールドと同時に起きないので
// 結果は常に通常と同じで、サイコフィールドでも印を付けない。判定に使われることは架空の「サイコフィールドで +1」で確かめる。
func TestStage2PriorityBoost(t *testing.T) {
	pc := []MoveMechanism{MechanismPriorityChange}
	grassy := MoveRule{PriorityBoost: &PriorityBoost{Terrain: TerrainGrassy, Delta: 1}}
	t.Run("サイコフィールドで印なし・当たる", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGrass, 55)
		in.Field.Terrain = TerrainPsychic
		got := assertSameAsPlain(t, withRule(in, pc, grassy), plain(in, TypeGrass, 55))
		if got.Nullified != NullifyNone {
			t.Errorf("Nullified = %q, want 空", got.Nullified)
		}
	})
	t.Run("グラスフィールドで通常", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGrass, 55)
		in.Field.Terrain = TerrainGrassy
		assertSameAsPlain(t, withRule(in, pc, grassy), plain(in, TypeGrass, 55))
	})
	psychic := MoveRule{PriorityBoost: &PriorityBoost{Terrain: TerrainPsychic, Delta: 1}}
	t.Run("優先度が上がった技はサイコフィールドで接地した防御側に当たらない", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGrass, 55)
		in.Field.Terrain = TerrainPsychic
		got := mustCalcS2(t, withRule(in, pc, psychic))
		if got.Nullified != NullifyPsychicTerrain || got.Rolls != ([16]int{}) {
			t.Errorf("Nullified = %q・rolls = %v, want psychic_terrain・0", got.Nullified, got.Rolls)
		}
	})
	t.Run("攻撃側が浮いていると優先度は変わらない", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeGrass, 55)
		in.Field.Terrain = TerrainPsychic
		in.Attacker.Ability = airborne
		if got := mustCalcS2(t, withRule(in, pc, psychic)); got.Nullified != NullifyNone {
			t.Errorf("Nullified = %q, want 空", got.Nullified)
		}
	})
}

// ---------------------------------------------------------------------------
// 威力の式: 攻撃側の正のランク・素早さ比
// ---------------------------------------------------------------------------

func TestStage2PositiveBoosts(t *testing.T) {
	rule := MoveRule{PowerFormula: PowerFormulaPositiveBoosts}
	cases := []struct {
		name  string
		ranks Ranks
		want  int
	}{
		{"ランクなし", Ranks{}, 20},
		{"正のランクだけ数える(+2 +1 -1 +3 → 6)", Ranks{Atk: 2, Def: 1, SpA: -1, Spe: 3}, 140},
		{"負だけ", Ranks{Atk: -2, SpD: -6}, 20},
		{"最大(+6 × 5)", Ranks{Atk: 6, Def: 6, SpA: 6, SpD: 6, Spe: 6}, 620},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategorySpecial, TypePsychic, 20)
			in.Attacker.Ranks = c.ranks
			assertSameAsPlain(t, withRule(in, []MoveMechanism{MechanismVariablePower}, rule), plain(in, TypePsychic, c.want))
		})
	}
}

// 素早さ: 種族値 B・SP 0・性格補正なしの実数値は B + 20。ランク → 特性・持ち物の素早さの補正(成立した最初の1つずつ)の連鎖 →
// pokeRound → まひ(×50/100 floor。IgnoresParalysisSpeedDrop で受けない)。
func TestStage2SpeedRatio(t *testing.T) {
	gyro := MoveRule{PowerFormula: PowerFormulaInverseSpeedRatio}
	electro := MoveRule{PowerFormula: PowerFormulaSpeedRatio}
	quickFeet := Ability{ID: "test-quickfeet", Effect: &AbilityEffect{
		SpeedMods: []SpeedMod{{Condition: SpeedConditionHasStatus, Modifier: 6144}}, IgnoresParalysisSpeedDrop: true}}
	swim := Ability{ID: "test-swim", Effect: &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionWeatherRain, Modifier: 8192}}}}
	firstOnly := Ability{ID: "test-first", Effect: &AbilityEffect{SpeedMods: []SpeedMod{
		{Condition: SpeedConditionWeatherSun, Modifier: 8192}, {Condition: SpeedConditionAlways, Modifier: 6144}, {Condition: SpeedConditionAlways, Modifier: 8192}}}}
	heavyBall := &Item{ID: "test-heavyball", Effect: &ItemEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionAlways, Modifier: 2048}}}}

	cases := []struct {
		name             string
		rule             MoveRule
		atkBase, defBase int
		edit             func(*DamageInput)
		wantPower        int
	}{
		// 攻撃側 50・防御側 150: floor(25 × 150 / 50) + 1 = 76
		{"ジャイロ: 遅いほど強い", gyro, 30, 130, func(*DamageInput) {}, 76},
		// 攻撃側 50 のランク -1 → 33: floor(25 × 150 / 33) + 1 = 114
		{"ジャイロ: 攻撃側のランク", gyro, 30, 130, func(in *DamageInput) { in.Attacker.Ranks.Spe = -1 }, 114},
		// 攻撃側まひ 50 → 25: floor(25 × 150 / 25) + 1 = 151 → 上限 150
		{"ジャイロ: まひと上限 150", gyro, 30, 130, func(in *DamageInput) { in.Attacker.Status = StatusParalysis }, 150},
		// まひ × 素早さの補正(状態異常で 6144)・半減を受けない: 50 → 75 → floor(25 × 150 / 75) + 1 = 51
		{"ジャイロ: 状態異常の補正・まひの半減なし", gyro, 30, 130, func(in *DamageInput) {
			in.Attacker.Status = StatusParalysis
			in.Attacker.Ability = quickFeet
		}, 51},
		// 防御側の持ち物 2048: 150 → 75 → floor(25 × 75 / 50) + 1 = 38
		{"ジャイロ: 防御側の持ち物の補正", gyro, 30, 130, func(in *DamageInput) { in.Defender.Item = heavyBall }, 38},
		// 雨で 8192: 50 → 100 → floor(25 × 150 / 100) + 1 = 38
		{"ジャイロ: 天候の補正", gyro, 30, 130, func(in *DamageInput) {
			in.Attacker.Ability = swim
			in.Field.Weather = WeatherRain
		}, 38},
		// 成立した最初の要素だけ: 6144 → 75 → 51
		{"ジャイロ: 特性の補正は成立した最初の1つ", gyro, 30, 130, func(in *DamageInput) { in.Attacker.Ability = firstOnly }, 51},
		// 攻撃側 200・防御側 50: 比 4 → 150
		{"エレキ: 比 4 以上は 150", electro, 180, 30, func(*DamageInput) {}, 150},
		// 攻撃側 100・防御側 50: 比 2 → 80
		{"エレキ: 比 2 は 80", electro, 80, 30, func(*DamageInput) {}, 80},
		// 攻撃側 99・防御側 50: 比 1(floor)→ 60
		{"エレキ: 比は切り捨て", electro, 79, 30, func(*DamageInput) {}, 60},
		// 攻撃側 50・防御側 150: 比 0 → 40
		{"エレキ: 遅いと 40", electro, 30, 130, func(*DamageInput) {}, 40},
		// 防御側まひ 150 → 75: 比 floor(200 / 75) = 2 → 80
		{"エレキ: 防御側のまひ", electro, 180, 130, func(in *DamageInput) { in.Defender.Status = StatusParalysis }, 80},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeSteel, 0)
			in.Attacker.Species.BaseStats.Spe = c.atkBase
			in.Defender.Species.BaseStats.Spe = c.defBase
			c.edit(&in)
			assertSameAsPlain(t, withRule(in, []MoveMechanism{MechanismVariablePower}, c.rule), plain(in, TypeSteel, c.wantPower))
		})
	}
}

// ---------------------------------------------------------------------------
// 相性(ExtraEffectivenessType・SuperEffectiveAgainst)
// ---------------------------------------------------------------------------

func TestStage2Effectiveness(t *testing.T) {
	ec := []MoveMechanism{MechanismEffectivenessChange}
	press := MoveRule{ExtraEffectivenessType: TypeFlying}
	dry := MoveRule{SuperEffectiveAgainst: []Type{TypeWater}}
	cases := []struct {
		name     string
		rule     MoveRule
		moveType Type
		defTypes []Type
		wantEff  float64
		// 等価な通常の入力: 防御側を psychic のままにして、同じ相性になる技のタイプ(一致しないタイプ)。
		plainType Type
		plainDef  []Type
	}{
		{"格闘 × 飛行の相性を掛ける: くさ → 2", press, TypeFighting, []Type{TypeGrass}, 2, TypeFlying, []Type{TypeGrass}},
		{"格闘 × 飛行: いわ → 2 × 0.5 = 1", press, TypeFighting, []Type{TypeRock}, 1, TypeNormal, []Type{TypePsychic}},
		{"格闘 × 飛行: ゴースト → 0", press, TypeFighting, []Type{TypeGhost}, 0, TypeNormal, []Type{TypeGhost}},
		{"水への相性を 2: みず → 2", dry, TypeIce, []Type{TypeWater}, 2, TypeFighting, []Type{TypeRock}},
		{"水への相性を 2: みず・じめん → 4", dry, TypeIce, []Type{TypeWater, TypeGround}, 4, TypeFighting, []Type{TypeRock, TypeSteel}},
		{"水への相性を 2: ほのお → 0.5 のまま", dry, TypeIce, []Type{TypeFire}, 0.5, TypeFighting, []Type{TypePoison}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategorySpecial, c.moveType, 80)
			in.Defender.Species.Types = c.defTypes
			want := plain(in, c.plainType, 80)
			want.Defender.Species.Types = c.plainDef
			got := assertSameAsPlain(t, withRule(in, ec, c.rule), want)
			if got.Effectiveness != c.wantEff {
				t.Errorf("相性 = %v, want %v", got.Effectiveness, c.wantEff)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 壁を壊す・何発目か
// ---------------------------------------------------------------------------

func TestStage2BreaksScreens(t *testing.T) {
	rule := MoveRule{BreaksScreens: true, MoveSpecificResolved: true}
	ms := []MoveMechanism{MechanismMoveSpecific}
	for _, c := range []struct {
		name    string
		format  Format
		screens Screens
	}{
		{"リフレクター", FormatSingle, Screens{Reflect: true}},
		{"オーロラベール", FormatSingle, Screens{AuroraVeil: true}},
		{"ダブルのリフレクター", FormatDouble, Screens{Reflect: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeFighting, 75)
			in.Format = c.format
			in.Move.Target = MoveTargetSingle
			in.Field.DefenderScreens = c.screens
			want := plain(in, TypeFighting, 75)
			want.Field.DefenderScreens = Screens{}
			assertSameAsPlain(t, withRule(in, ms, rule), want)
		})
	}
}

func TestStage2HitIndex(t *testing.T) {
	rule := MoveRule{PowerFormula: PowerFormulaHitIndex}
	mechs := []MoveMechanism{MechanismMultiHit, MechanismVariablePower}
	technician := Ability{ID: "test-technician", Effect: &AbilityEffect{PowerMods: []ConditionalPowerMod{
		{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}}}
	for _, c := range []struct {
		name string
		edit func(*DamageInput)
	}{
		{"h 発目の威力は 威力 × h", func(*DamageInput) {}},
		{"特性の条件は発ごとの威力で判定(20・40・60 はすべて 60 以下)", func(in *DamageInput) { in.Attacker.Ability = technician }},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeIce, 20)
			c.edit(&in)
			in = withRule(in, mechs, rule)
			in.Move.Params.MultiHit = &MultiHit{Min: 3, Max: 3}
			got := mustCalcS2(t, in)
			if len(got.HitRolls) != 3 {
				t.Fatalf("len(HitRolls) = %d, want 3", len(got.HitRolls))
			}
			var sum [16]int
			for h := range 3 {
				want := mustCalcS2(t, plain(in, TypeIce, 20*(h+1)))
				if got.HitRolls[h] != want.Rolls {
					t.Errorf("HitRolls[%d] = %v, want %v(威力 %d)", h, got.HitRolls[h], want.Rolls, 20*(h+1))
				}
				for i := range 16 {
					sum[i] += want.Rolls[i]
				}
			}
			if got.Rolls != sum {
				t.Errorf("Rolls = %v, want %v(同じ段の合計)", got.Rolls, sum)
			}
			if wantKO := computeKOHits(got.Rolls, got.HitRolls, got.DefenderHP); got.KO != wantKO {
				t.Errorf("KO = %+v, want %+v(発ごとに違う16段階の畳み込み)", got.KO, wantKO)
			}
			if got.Unsupported != nil {
				t.Errorf("印が付いた: %v", got.Unsupported)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 重さ(target_weight・weight_ratio・特性の WeightMod)
// ---------------------------------------------------------------------------

func TestStage2Weight(t *testing.T) {
	target := MoveRule{PowerFormula: PowerFormulaTargetWeight, MoveSpecificResolved: true}
	ratio := MoveRule{PowerFormula: PowerFormulaWeightRatio, MoveSpecificResolved: true}
	mechs := []MoveMechanism{MechanismMoveSpecific, MechanismVariablePower}
	heavy := Ability{ID: "test-heavy", Effect: &AbilityEffect{WeightMod: 8192, Breakable: true}}
	light := Ability{ID: "test-light", Effect: &AbilityEffect{WeightMod: 2048, Breakable: true}}
	breaker := Ability{ID: "test-breaker", Effect: &AbilityEffect{IgnoresDefenderAbility: true}}

	cases := []struct {
		name         string
		rule         MoveRule
		atkHg, defHg int
		edit         func(*DamageInput)
		want         int
	}{
		{"防御側 200.0kg 以上は 120", target, 100, 2000, nil, 120},
		{"防御側 199.9kg は 100", target, 100, 1999, nil, 100},
		{"防御側 100.0kg は 100", target, 100, 1000, nil, 100},
		{"防御側 99.9kg は 80", target, 100, 999, nil, 80},
		{"防御側 25.0kg は 60", target, 100, 250, nil, 60},
		{"防御側 10.0kg は 40", target, 100, 100, nil, 40},
		{"防御側 9.9kg は 20", target, 100, 99, nil, 20},
		{"防御側の重さの補正 ×2: 99.9kg → 199.8kg は 100", target, 100, 999, func(in *DamageInput) { in.Defender.Ability = heavy }, 100},
		{"かたやぶりで防御側の重さの補正を無視", target, 100, 999, func(in *DamageInput) {
			in.Defender.Ability = heavy
			in.Attacker.Ability = breaker
		}, 80},
		{"比ちょうど 5 は 120(整数で比べる)", ratio, 4600, 920, nil, 120},
		{"比 5 未満は 100", ratio, 4599, 920, nil, 100},
		{"比ちょうど 3 は 80(浮動小数なら 6.9/2.3 < 3)", ratio, 69, 23, nil, 80},
		{"比ちょうど 2 は 60", ratio, 200, 100, nil, 60},
		{"攻撃側が軽いと 40", ratio, 100, 200, nil, 40},
		{"攻撃側の重さの補正 ×0.5: 400 → 200 / 100 は 60", ratio, 400, 100, func(in *DamageInput) { in.Attacker.Ability = light }, 60},
		{"重さの補正の切り捨てと最小 1: 1 × 0.5 → 1(比 4 → 100)", ratio, 4, 1, func(in *DamageInput) { in.Defender.Ability = light }, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeFighting, 0)
			in.Attacker.Species.WeightHg = c.atkHg
			in.Defender.Species.WeightHg = c.defHg
			if c.edit != nil {
				c.edit(&in)
			}
			assertSameAsPlain(t, withRule(in, mechs, c.rule), plain(in, TypeFighting, c.want))
		})
	}

	// 重さが不明(0)なら計算できないので、従来どおり(威力 0 → ダメージ 0)で印を残す。
	for _, c := range []struct {
		name         string
		rule         MoveRule
		atkHg, defHg int
	}{
		{"防御側の重さが不明", target, 100, 0},
		{"攻撃側の重さが不明", ratio, 0, 100},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeFighting, 0)
			in.Attacker.Species.WeightHg = c.atkHg
			in.Defender.Species.WeightHg = c.defHg
			got := mustCalcS2(t, withRule(in, mechs, c.rule))
			if got.Rolls != ([16]int{}) {
				t.Errorf("rolls = %v, want 0(従来どおり)", got.Rolls)
			}
			want := []UnsupportedMark{
				{Target: UnsupportedTargetMove, Reason: UnsupportedReason(MechanismVariablePower), ID: "test-rule-move"},
				{Target: UnsupportedTargetMove, Reason: UnsupportedZeroPower, ID: "test-rule-move"},
			}
			if !reflect.DeepEqual(got.Unsupported, want) {
				t.Errorf("Unsupported = %v, want %v", got.Unsupported, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 印(ADR-0143 §3)
// ---------------------------------------------------------------------------

func TestStage2Marks(t *testing.T) {
	mark := func(r UnsupportedReason) UnsupportedMark {
		return UnsupportedMark{Target: UnsupportedTargetMove, Reason: r, ID: "test-rule-move"}
	}
	cases := []struct {
		name  string
		mechs []MoveMechanism
		rule  *MoveRule
		power int
		edit  func(*DamageInput)
		want  []UnsupportedMark
	}{
		{"定義なしは従来どおり", []MoveMechanism{MechanismMoveSpecific, MechanismVariablePower}, nil, 0, nil,
			[]UnsupportedMark{mark(UnsupportedReason(MechanismMoveSpecific)), mark(UnsupportedReason(MechanismVariablePower)), mark(UnsupportedZeroPower)}},
		{"威力の式はあるが MoveSpecificResolved が無い: move_specific だけ残る", []MoveMechanism{MechanismMoveSpecific, MechanismVariablePower},
			&MoveRule{PowerFormula: PowerFormulaPositiveBoosts}, 20, nil,
			[]UnsupportedMark{mark(UnsupportedReason(MechanismMoveSpecific))}},
		{"MoveSpecificResolved だけ: variable_power は残る", []MoveMechanism{MechanismMoveSpecific, MechanismVariablePower},
			&MoveRule{MoveSpecificResolved: true}, 0, nil,
			[]UnsupportedMark{mark(UnsupportedReason(MechanismVariablePower)), mark(UnsupportedZeroPower)}},
		{"定義なしの priority_change はサイコフィールドで従来どおり印", []MoveMechanism{MechanismPriorityChange}, nil, 55,
			func(in *DamageInput) { in.Field.Terrain = TerrainPsychic },
			[]UnsupportedMark{mark(UnsupportedReason(MechanismPriorityChange))}},
		{"PriorityBoost があればサイコフィールドでも印なし", []MoveMechanism{MechanismPriorityChange},
			&MoveRule{PriorityBoost: &PriorityBoost{Terrain: TerrainGrassy, Delta: 1}}, 55,
			func(in *DamageInput) { in.Field.Terrain = TerrainPsychic }, nil},
		{"定義なしの field_specific は天候があれば従来どおり印", []MoveMechanism{MechanismFieldSpecific}, nil, 100,
			func(in *DamageInput) { in.Field.Weather = WeatherRain },
			[]UnsupportedMark{mark(UnsupportedReason(MechanismFieldSpecific))}},
		{"TypeByWeather があれば type_change の印なし", []MoveMechanism{MechanismTypeChange},
			&MoveRule{TypeByWeather: map[Weather]Type{WeatherRain: TypeWater}}, 50, nil, nil},
		{"相性の中身があれば effectiveness_change の印なし", []MoveMechanism{MechanismEffectivenessChange},
			&MoveRule{SuperEffectiveAgainst: []Type{TypeWater}}, 70, nil, nil},
		{"段階2外の機構(fixed_damage の中身なし)は残る", []MoveMechanism{MechanismFixedDamage, MechanismMoveSpecific},
			&MoveRule{MoveSpecificResolved: true}, 0, nil,
			[]UnsupportedMark{mark(UnsupportedReason(MechanismFixedDamage)), mark(UnsupportedZeroPower)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeNormal, c.power)
			in.Move.ID = "test-rule-move"
			in.Move.Mechanisms = c.mechs
			in.Move.Rule = c.rule
			if c.edit != nil {
				c.edit(&in)
			}
			got := mustCalcS2(t, in)
			if !reflect.DeepEqual(got.Unsupported, c.want) {
				t.Errorf("Unsupported = %v, want %v", got.Unsupported, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 検証(ErrInvalidMoveRule)
// ---------------------------------------------------------------------------

func TestStage2ValidateRule(t *testing.T) {
	vp := []MoveMechanism{MechanismVariablePower}
	boost := func(b MovePowerBoost) MoveRule { return MoveRule{PowerBoosts: []MovePowerBoost{b}} }
	cases := []struct {
		name    string
		mechs   []MoveMechanism
		rule    MoveRule
		status  bool // 変化技にする
		wantErr error
	}{
		{"空の定義", vp, MoveRule{}, false, ErrInvalidMoveRule},
		{"未知の式", vp, MoveRule{PowerFormula: "hp_ratio"}, false, ErrInvalidMoveRule},
		{"威力の中身に対応する機構が無い", []MoveMechanism{MechanismTypeChange}, MoveRule{PowerFormula: PowerFormulaSpeedRatio}, false, ErrInvalidMoveRule},
		{"hit_index に多段の中身が無い", vp, MoveRule{PowerFormula: PowerFormulaHitIndex}, false, ErrInvalidMoveRule},
		{"未知の条件", vp, boost(MovePowerBoost{Condition: "attacker_hp", BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"状態の条件で一覧が空", vp, boost(MovePowerBoost{Condition: MoveConditionDefenderStatus, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"状態の一覧に未知の値", vp, boost(MovePowerBoost{Condition: MoveConditionDefenderStatus, Statuses: []Status{"toxic"}, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"状態の一覧に none", vp, boost(MovePowerBoost{Condition: MoveConditionDefenderStatus, Statuses: []Status{StatusNone}, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"天候の条件にフィールドの一覧", vp, boost(MovePowerBoost{Condition: MoveConditionWeather, Weathers: []Weather{WeatherRain}, Terrains: []Terrain{TerrainMisty}, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"天候の一覧に none", vp, boost(MovePowerBoost{Condition: MoveConditionWeather, Weathers: []Weather{WeatherNone}, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"フィールドの一覧に none", vp, boost(MovePowerBoost{Condition: MoveConditionTerrainAttackerGrounded, Terrains: []Terrain{TerrainNone}, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"持ち物の条件に状態の一覧", vp, boost(MovePowerBoost{Condition: MoveConditionAttackerNoItem, Statuses: []Status{StatusBurn}, BaseMultiplier: 2}), false, ErrInvalidMoveRule},
		{"整数倍と補正の両方", vp, boost(MovePowerBoost{Condition: MoveConditionAttackerNoItem, BaseMultiplier: 2, Modifier: 6144}), false, ErrInvalidMoveRule},
		{"整数倍も補正も無い", vp, boost(MovePowerBoost{Condition: MoveConditionAttackerNoItem}), false, ErrInvalidMoveRule},
		{"整数倍が 1", vp, boost(MovePowerBoost{Condition: MoveConditionAttackerNoItem, BaseMultiplier: 1}), false, ErrInvalidMoveRule},
		{"補正が中立 4096", vp, boost(MovePowerBoost{Condition: MoveConditionAttackerNoItem, Modifier: Modifier4096}), false, ErrInvalidMoveRule},
		{"補正が範囲外", vp, boost(MovePowerBoost{Condition: MoveConditionAttackerNoItem, Modifier: MaxEffectModifier + 1}), false, ErrInvalidMoveRule},
		{"フィールドの補正に field_specific が無い", vp, MoveRule{TerrainPowerMods: []TerrainPowerMod{{Terrain: TerrainGrassy, Modifier: 2048}}}, false, ErrInvalidMoveRule},
		{"フィールドの補正のフィールドが none", []MoveMechanism{MechanismFieldSpecific}, MoveRule{TerrainPowerMods: []TerrainPowerMod{{Terrain: TerrainNone, Modifier: 2048}}}, false, ErrInvalidMoveRule},
		{"天候のタイプに type_change が無い", vp, MoveRule{TypeByWeather: map[Weather]Type{WeatherRain: TypeWater}}, false, ErrInvalidMoveRule},
		{"天候のタイプのキーが none", []MoveMechanism{MechanismTypeChange}, MoveRule{TypeByWeather: map[Weather]Type{WeatherNone: TypeWater}}, false, ErrInvalidMoveRule},
		{"天候のタイプが相性表に無い", []MoveMechanism{MechanismTypeChange}, MoveRule{TypeByWeather: map[Weather]Type{WeatherRain: "shadow"}}, false, ErrUnknownType},
		{"フィールドのタイプが相性表に無い", []MoveMechanism{MechanismTypeChange}, MoveRule{TypeByTerrain: map[Terrain]Type{TerrainMisty: "shadow"}}, false, ErrUnknownType},
		{"相性の中身に effectiveness_change が無い", vp, MoveRule{ExtraEffectivenessType: TypeFlying}, false, ErrInvalidMoveRule},
		{"相性のタイプが相性表に無い", []MoveMechanism{MechanismEffectivenessChange}, MoveRule{SuperEffectiveAgainst: []Type{"shadow"}}, false, ErrUnknownType},
		{"優先度の中身に priority_change が無い", vp, MoveRule{PriorityBoost: &PriorityBoost{Terrain: TerrainGrassy, Delta: 1}}, false, ErrInvalidMoveRule},
		{"優先度の変化が 0", []MoveMechanism{MechanismPriorityChange}, MoveRule{PriorityBoost: &PriorityBoost{Terrain: TerrainGrassy}}, false, ErrInvalidMoveRule},
		{"壁を壊すに move_specific が無い", vp, MoveRule{BreaksScreens: true}, false, ErrInvalidMoveRule},
		{"MoveSpecificResolved に move_specific が無い", vp, MoveRule{MoveSpecificResolved: true}, false, ErrInvalidMoveRule},
		{"SpreadInTerrain が none 以外の未知", []MoveMechanism{MechanismMoveSpecific}, MoveRule{SpreadInTerrain: "volcanic", MoveSpecificResolved: true}, false, ErrInvalidMoveRule},
		{"変化技の定義", nil, MoveRule{MoveSpecificResolved: true}, true, ErrInvalidMoveRule},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeNormal, 50)
			if c.status {
				in.Move.Category, in.Move.Power = CategoryStatus, 0
			}
			in = withRule(in, c.mechs, c.rule)
			if _, err := calcDamage(in); !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if err := in.Move.ValidateRule(typeChartForTests()); !errors.Is(err, c.wantErr) {
				t.Fatalf("ValidateRule = %v, want %v", err, c.wantErr)
			}
		})
	}
	t.Run("定義なしは検証を通る", func(t *testing.T) {
		in := s2Input(CategoryPhysical, TypeNormal, 50)
		in.Move.Mechanisms = []MoveMechanism{MechanismVariablePower}
		if err := in.Move.ValidateRule(typeChartForTests()); err != nil {
			t.Fatalf("ValidateRule(nil) = %v, want nil", err)
		}
	})
}

// 特性の重さの補正の値域(ADR-0143 §1。他の補正値と同じ MinEffectModifier..MaxEffectModifier)。
func TestStage2WeightModValidation(t *testing.T) {
	for _, v := range []int{-1, MinEffectModifier - 1, MaxEffectModifier + 1} {
		in := s2Input(CategoryPhysical, TypeNormal, 50)
		in.Attacker.Ability = Ability{ID: "test-weight", Effect: &AbilityEffect{WeightMod: v}}
		if v == 0 {
			continue
		}
		if _, err := calcDamage(in); err == nil {
			t.Errorf("WeightMod %d: エラーにならない", v)
		}
	}
}
