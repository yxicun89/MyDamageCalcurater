package engine

import "testing"

// 統制ケース(real Atk200/Def100/威力100)。P1-6で @smogon/calc 0.10.0 と照合。
// 補正段階に起因する期待値の訂正理由は ADR-0008 を参照。

func TestWeatherDamageMod(t *testing.T) {
	// 晴れ + ほのお技(攻撃みず=一致なし、防御エスパー=等倍): base 90 → pokeRound(90,6144)=135
	sun := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire)
	sun.Field.Weather = WeatherSun
	r, _ := CalcDamage(sun)
	if r.Rolls[15] != 135 {
		t.Errorf("sun fire rolls[15]=%d want 135", r.Rolls[15])
	}
	// 雨 + ほのお技: pokeRound(90,2048)=45
	rain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire)
	rain.Field.Weather = WeatherRain
	rr, _ := CalcDamage(rain)
	if rr.Rolls[15] != 45 {
		t.Errorf("rain fire rolls[15]=%d want 45", rr.Rolls[15])
	}
}

func TestTerrainDamageMod(t *testing.T) {
	// フィールドは威力を100→130に補正し、基本式の結果は116。
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
	in.Field.Terrain = TerrainElectric
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 116 {
		t.Errorf("electric terrain rolls[15]=%d want 116", r.Rolls[15])
	}
	// ミストは威力を100→50に補正、基本式の結果は46。
	in2 := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeDragon)
	in2.Field.Terrain = TerrainMisty
	r2, _ := CalcDamage(in2)
	if r2.Rolls[15] != 46 {
		t.Errorf("misty dragon rolls[15]=%d want 46", r2.Rolls[15])
	}
}

func TestScreenMod(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Field.DefenderScreens.Reflect = true
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 45 { // 90×0.5
		t.Errorf("reflect rolls[15]=%d want 45", r.Rolls[15])
	}
	// 急所は壁を貫通 → 軽減されない。base 90 → 急所 135
	inC := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	inC.Field.DefenderScreens.Reflect = true
	inC.Critical = true
	rc, _ := CalcDamage(inC)
	if rc.Rolls[15] != 135 {
		t.Errorf("crit should bypass screen rolls[15]=%d want 135", rc.Rolls[15])
	}
	// ひかりのかべは物理に効かない
	inL := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	inL.Field.DefenderScreens.LightScreen = true
	rl, _ := CalcDamage(inL)
	if rl.Rolls[15] != 90 {
		t.Errorf("light screen should not affect physical rolls[15]=%d want 90", rl.Rolls[15])
	}
}

func TestItemChoiceBand(t *testing.T) {
	// こだわりハチマキ(Atk×1.5): 200→300、base=(22*100*300/100)/50+2=134
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Item = &Item{ID: "choice_band", Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 134 {
		t.Errorf("choice band rolls[15]=%d want 134", r.Rolls[15])
	}
}

func TestItemLifeOrb(t *testing.T) {
	// いのちのたま(ダメージ×5324/4096): pokeRound(90,5324)=117
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Item = &Item{ID: "life_orb", Effect: &ItemEffect{DamageMod: 5324}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 117 {
		t.Errorf("life orb rolls[15]=%d want 117", r.Rolls[15])
	}
}

func TestItemExpertBelt(t *testing.T) {
	// たつじんのおび: 抜群時のみ ×4915。等倍なら無効。
	belt := &ItemEffect{DamageMod: 4915, OnlySuperEffective: true}
	// 等倍(エスパー): 効果なし → 90
	neutral := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	neutral.Attacker.Item = &Item{Effect: belt}
	rn, _ := CalcDamage(neutral)
	if rn.Rolls[15] != 90 {
		t.Errorf("belt neutral rolls[15]=%d want 90", rn.Rolls[15])
	}
	// 抜群(みず→いわ ×2、一致なし): 90×2=180 → ×4915: pokeRound(180,4915)=216
	sup := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategoryPhysical, TypeWater)
	sup.Attacker.Item = &Item{Effect: belt}
	rs, _ := CalcDamage(sup)
	// 180×4915/4096 = 216.0 (884700/4096=216.0)
	if rs.Rolls[15] != 216 {
		t.Errorf("belt super rolls[15]=%d want 216", rs.Rolls[15])
	}
}

func TestItemAssaultVestDefender(t *testing.T) {
	// とつげきチョッキ(SpD×1.5、特殊技): 防御 SpD100→150、base=(22*100*200/150)/50+2=60
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal)
	in.Defender.Item = &Item{ID: "assault_vest", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 60 {
		t.Errorf("assault vest rolls[15]=%d want 60", r.Rolls[15])
	}
}

func TestItemResistBerry(t *testing.T) {
	// 半減きのみ(みず): 抜群を半減。みず→いわ ×2、180 → ×0.5=90
	in := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategoryPhysical, TypeWater)
	in.Defender.Item = &Item{ID: "roseli", Effect: &ItemEffect{ResistBerryType: TypeWater}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 90 {
		t.Errorf("resist berry rolls[15]=%d want 90", r.Rolls[15])
	}
	// 等倍では発動しない
	in2 := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeWater)
	in2.Defender.Item = &Item{Effect: &ItemEffect{ResistBerryType: TypeWater}}
	r2, _ := CalcDamage(in2)
	if r2.Rolls[15] != 90 { // 等倍90のまま(未発動)
		t.Errorf("resist berry should not trigger on neutral rolls[15]=%d want 90", r2.Rolls[15])
	}
}

func TestAbilityAdaptability(t *testing.T) {
	// てきおうりょく: STAB 8192。攻撃みず/技みず(一致)/防御エスパー等倍。90×2=180
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeWater)
	in.Attacker.Ability = Ability{ID: "adaptability", Effect: &AbilityEffect{StabMod: 8192}}
	r, _ := CalcDamage(in)
	if !r.STAB || r.Rolls[15] != 180 {
		t.Errorf("adaptability stab=%v rolls[15]=%d want true 180", r.STAB, r.Rolls[15])
	}
}

func TestAbilityThickFat(t *testing.T) {
	// あついしぼう: 相手の攻撃実数値200→100、基本式の結果は46。
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire)
	in.Defender.Ability = Ability{ID: "thick_fat", Effect: &AbilityEffect{DefResistType: map[Type]int{TypeFire: 2048, TypeIce: 2048}}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 46 {
		t.Errorf("thick fat rolls[15]=%d want 46", r.Rolls[15])
	}
}

func TestWeatherSandSpDBoost(t *testing.T) {
	// すなあらし: いわタイプの特防×1.5。特殊技エスパー(いわに等倍)。SpD100→150 → base 60
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategorySpecial, TypePsychic)
	in.Field.Weather = WeatherSand
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 60 {
		t.Errorf("sand spd boost rolls[15]=%d want 60", r.Rolls[15])
	}
}

func TestWeatherSnowDefBoost(t *testing.T) {
	// ゆき: こおりタイプの防御×1.5。物理技ノーマル(こおりに等倍)。Def100→150 → base 60
	in := ctrlInput([]Type{TypeWater}, []Type{TypeIce}, CategoryPhysical, TypeNormal)
	in.Field.Weather = WeatherSnow
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 60 {
		t.Errorf("snow def boost rolls[15]=%d want 60", r.Rolls[15])
	}
}

func TestItemTypeBoost(t *testing.T) {
	// もくたん: 威力100→120、基本式の結果は107。
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire)
	in.Attacker.Item = &Item{ID: "charcoal", Effect: &ItemEffect{BoostType: TypeFire, BoostTypeMod: 4915}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 107 {
		t.Errorf("type boost rolls[15]=%d want 107", r.Rolls[15])
	}
}

func TestAbilityReduceSuperEffective(t *testing.T) {
	// フィルター/ハードロック相当: 抜群を ×0.75(3072)。みず→いわ ×2、180 → pokeRound(180,3072)=135
	in := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategoryPhysical, TypeWater)
	in.Defender.Ability = Ability{ID: "filter", Effect: &AbilityEffect{ReduceSuperEffective: 3072}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 135 {
		t.Errorf("reduce super-effective rolls[15]=%d want 135", r.Rolls[15])
	}
	// 等倍では軽減されない
	in2 := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in2.Defender.Ability = Ability{Effect: &AbilityEffect{ReduceSuperEffective: 3072}}
	r2, _ := CalcDamage(in2)
	if r2.Rolls[15] != 90 {
		t.Errorf("reduce should not apply on neutral rolls[15]=%d want 90", r2.Rolls[15])
	}
}

func TestRankThenItemOrder(t *testing.T) {
	// 実数値補正はランクの「後」に適用する(ADR-0004)。floor で順序を判別できる値で固定。
	// 実 Atk200 → ランク-1: applyStatStage(200,-1)=133 → こだわり: pokeRound(133,6144)=199
	// base = (22*100*199/100)/50+2 = 89。順序が逆(先に持ち物→ランク)だと 200 になり base が変わる。
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Ranks = Ranks{Atk: -1}
	in.Attacker.Item = &Item{ID: "choice_band", Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}}}
	r, _ := CalcDamage(in)
	if r.Rolls[15] != 89 {
		t.Errorf("rank-then-item rolls[15]=%d want 89", r.Rolls[15])
	}
}
