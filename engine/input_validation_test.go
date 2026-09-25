package engine

// issue #255: 種族値・タイプ・持ち物/特性の補正値の値域検査(ADR-0117)。
// 範囲外の入力は Individual.Validate が拒否し、CalcDamage は巨大確保や意味の無い結果を返さない。

import "testing"

func TestIndividualValidateInputRanges(t *testing.T) {
	base := func() Individual {
		return Individual{Species: sampleSpecies(), Nature: NatureNeutral}
	}
	item := func(e ItemEffect) *Item { return &Item{ID: "testitem", Effect: &e} }
	ability := func(e AbilityEffect) Ability { return Ability{ID: "testability", Effect: &e} }
	tests := []struct {
		name    string
		mut     func(*Individual)
		wantErr bool
	}{
		// 種族値 MinBaseStat..MaxBaseStat(HP を含む)
		{"base stat min ok", func(i *Individual) { i.Species.BaseStats.Spe = MinBaseStat }, false},
		{"base stat max ok", func(i *Individual) { i.Species.BaseStats.HP = MaxBaseStat }, false},
		{"base stat zero", func(i *Individual) { i.Species.BaseStats.Def = 0 }, true},
		{"base stat negative", func(i *Individual) { i.Species.BaseStats.Atk = -1 }, true},
		{"base stat over", func(i *Individual) { i.Species.BaseStats.SpA = MaxBaseStat + 1 }, true},
		{"base hp huge", func(i *Individual) { i.Species.BaseStats.HP = 2000000000 }, true},
		{"base hp zero", func(i *Individual) { i.Species.BaseStats.HP = 0 }, true},

		// タイプの重複
		{"single type ok", func(i *Individual) { i.Species.Types = []Type{TypeFire} }, false},
		{"duplicate types", func(i *Individual) { i.Species.Types = []Type{TypeFire, TypeFire} }, true},

		// 持ち物の補正値(0 は補正なしとして許す項目と、許さない StatMods)
		{"item nil effect ok", func(i *Individual) { i.Item = &Item{ID: "plain"} }, false},
		{"item damageMod zero ok", func(i *Individual) { i.Item = item(ItemEffect{DamageMod: 0}) }, false},
		{"item damageMod min ok", func(i *Individual) { i.Item = item(ItemEffect{DamageMod: MinEffectModifier}) }, false},
		{"item damageMod max ok", func(i *Individual) { i.Item = item(ItemEffect{DamageMod: MaxEffectModifier}) }, false},
		{"item damageMod negative", func(i *Individual) { i.Item = item(ItemEffect{DamageMod: -4096}) }, true},
		{"item damageMod over", func(i *Individual) { i.Item = item(ItemEffect{DamageMod: MaxEffectModifier + 1}) }, true},
		{"item powerMod negative", func(i *Individual) { i.Item = item(ItemEffect{PowerMod: -1}) }, true},
		{"item boostTypeMod negative", func(i *Individual) {
			i.Item = item(ItemEffect{BoostType: TypeFire, BoostTypeMod: -4915})
		}, true},
		{"item statMods ok", func(i *Individual) { i.Item = item(ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}}) }, false},
		{"item statMods zero", func(i *Individual) { i.Item = item(ItemEffect{StatMods: map[StatKey]int{StatDef: 0}}) }, true},
		{"item statMods negative", func(i *Individual) { i.Item = item(ItemEffect{StatMods: map[StatKey]int{StatSpD: -6144}}) }, true},

		// 特性の補正値
		{"ability stabMod zero ok", func(i *Individual) { i.Ability = ability(AbilityEffect{StabMod: 0}) }, false},
		{"ability stabMod ok", func(i *Individual) { i.Ability = ability(AbilityEffect{StabMod: ModifierAdaptability}) }, false},
		{"ability stabMod negative", func(i *Individual) { i.Ability = ability(AbilityEffect{StabMod: -1}) }, true},
		{"ability offBoostTypeMod negative", func(i *Individual) {
			i.Ability = ability(AbilityEffect{OffBoostType: TypeFire, OffBoostTypeMod: -4096})
		}, true},
		{"ability reduceSuperEffective over", func(i *Individual) {
			i.Ability = ability(AbilityEffect{ReduceSuperEffective: MaxEffectModifier + 1})
		}, true},
		{"ability defResistType ok", func(i *Individual) {
			i.Ability = ability(AbilityEffect{DefResistType: map[Type]int{TypeFire: ModifierHalf}})
		}, false},
		{"ability defResistType zero", func(i *Individual) {
			i.Ability = ability(AbilityEffect{DefResistType: map[Type]int{TypeFire: 0}})
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base()
			tt.mut(&in)
			err := in.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

// 補正値の上限は engine のいちばん広いクランプ(威力)と同じ(ADR-0117)。
func TestMaxEffectModifierMatchesWidestClamp(t *testing.T) {
	for _, b := range []modBounds{powerModBounds, statModBounds} {
		if b.upper > MaxEffectModifier {
			t.Errorf("クランプの上限 %d が MaxEffectModifier %d を超える", b.upper, MaxEffectModifier)
		}
	}
	if powerModBounds.upper != MaxEffectModifier {
		t.Errorf("MaxEffectModifier=%d want powerModBounds.upper=%d", MaxEffectModifier, powerModBounds.upper)
	}
}

// CalcDamage は範囲外の入力を計算前に拒否する(巨大な HP で確定数の確率表を確保しない)。
func TestCalcDamageRejectsOutOfRangeInput(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*DamageInput)
	}{
		{"huge defender hp", func(in *DamageInput) { in.Defender.Species.BaseStats.HP = 2000000000 }},
		{"duplicate defender types", func(in *DamageInput) { in.Defender.Species.Types = []Type{TypeFire, TypeFire} }},
		{"negative attacker damageMod", func(in *DamageInput) {
			in.Attacker.Item = &Item{ID: "x", Effect: &ItemEffect{DamageMod: -4096}}
		}},
		{"negative attacker offBoostTypeMod", func(in *DamageInput) {
			in.Attacker.Ability = Ability{ID: "x", Effect: &AbilityEffect{OffBoostType: TypeNormal, OffBoostTypeMod: -4096}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			tt.mut(&in)
			if r, err := calcDamage(in); err == nil {
				t.Fatalf("エラーになるはずが成功した: rolls=%v", r.Rolls)
			}
		})
	}
}

// PowerCategory が空の威力補正は、物理・特殊のどちらの技にも掛かる(issue #255 のコメント。
// modifiers.go の `e.PowerCategory == ""` の分岐はゴールデンでも検証されていないためここで固定する)。
// 分類を指定した補正は、その分類の技にだけ掛かる。
func TestItemPowerModCategory(t *testing.T) {
	const mod = 4505
	tests := []struct {
		name     string
		category MoveCategory // 持ち物の PowerCategory
		move     MoveCategory
		applied  bool
	}{
		{"empty category, physical move", "", CategoryPhysical, true},
		{"empty category, special move", "", CategorySpecial, true},
		{"physical category, physical move", CategoryPhysical, CategoryPhysical, true},
		{"physical category, special move", CategoryPhysical, CategorySpecial, false},
		{"special category, physical move", CategorySpecial, CategoryPhysical, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, tt.move, TypeNormal)
			boosted := plain
			boosted.Attacker.Item = &Item{ID: "x", Effect: &ItemEffect{PowerMod: mod, PowerCategory: tt.category}}
			rp, err := calcDamage(plain)
			if err != nil {
				t.Fatal(err)
			}
			rb, err := calcDamage(boosted)
			if err != nil {
				t.Fatal(err)
			}
			// 統制ケース: 威力100 → pokeRound(100,4505)=110 → floor(22×110×200/100/50)+2 = 98(補正なしは 90)
			want := rp.Rolls[15]
			if tt.applied {
				want = 98
			}
			if rp.Rolls[15] != 90 || rb.Rolls[15] != want {
				t.Errorf("rolls[15] plain=%d boosted=%d want plain=90 boosted=%d", rp.Rolls[15], rb.Rolls[15], want)
			}
		})
	}
}
