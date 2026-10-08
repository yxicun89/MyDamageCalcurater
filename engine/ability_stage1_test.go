package engine

// ADR-0176: 特性の段階1(engine と効果データだけで足りる系統)。
//
// 受け入れ条件(engine):
//   - AC-1 TypeConvert: From タイプの技を To タイプに変え、威力に PowerMod を掛ける。変換後のタイプで相性・タイプ一致・
//     天候・タイプ依存の補正(持ち物・防御側の特性・無効)を引く。From 以外の技・type_change の機構を持つ技は変えない。
//   - AC-2 StatMods(特性): 持ち物の StatMods と同じ読み方(攻撃側 atk/spa・防御側 def/spd)。
//     SeparateStatMods はランクの直後に単独で丸め、他の実数値補正とは連鎖しない。
//   - AC-3 PowerMods: max_base_power(威力 ≤ MaxPower)・move_type(技のタイプ)のときだけ威力に掛ける。
//     AuraType: 攻撃側・防御側のどちらが持っていても、そのタイプの技に1回だけ掛ける(変換後のタイプで判定)。
//   - AC-4 急所: CritDamageMod は急所のときだけ最終ダメージに掛ける。PreventsCritical(防御側)は急所の指定を無効にし
//     (ランク・壁・×1.5 も急所でないときと同じ)、必ず急所の技の印も付けない。
//   - AC-5 IgnoresOpponentRanks: 攻撃側が持てば防御側の防御ランク、防御側が持てば攻撃側の攻撃ランクを 0 として扱う。
//     自分のランクは無視しない。防御ランク無視の技の印は、攻撃側が持つなら付けない。
//   - AC-6 IgnoresDefenderAbility: 防御側の特性が Breakable なら、その効果(印を含む)を無いものとして計算する。
//     Breakable でない特性・攻撃側の特性は無視しない。
//   - AC-7 Individual.Validate が新しい項目の不正な値を拒否する。
//
// 期待値は「同じ補正を既存の(検証済みの)持ち物の効果で表した入力」との一致で書く(補正が1つだけのときは
// 連鎖の順が結果に影響しない)。連鎖の順・丸めの位置の正しさは oracle のゴールデン(fixed.json の
// effects/<id>/...)で見る。

import (
	"errors"
	"testing"
)

func mustCalc(t *testing.T, in DamageInput) DamageResult {
	t.Helper()
	r, err := calcDamage(in)
	if err != nil {
		t.Fatalf("calcDamage: %v", err)
	}
	return r
}

func assertSameRolls(t *testing.T, label string, got, want DamageResult) {
	t.Helper()
	if got.Rolls != want.Rolls {
		t.Errorf("%s: rolls=%v, want %v", label, got.Rolls, want.Rolls)
	}
}

func assertDifferentRolls(t *testing.T, label string, got, base DamageResult) {
	t.Helper()
	if got.Rolls == base.Rolls {
		t.Errorf("%s: 効果を持たせてもダメージが変わらない: %v", label, got.Rolls)
	}
}

func withAttackerAbility(in DamageInput, e *AbilityEffect) DamageInput {
	in.Attacker.Ability = Ability{ID: "testatk", Effect: e}
	return in
}

func withDefenderAbility(in DamageInput, e *AbilityEffect) DamageInput {
	in.Defender.Ability = Ability{ID: "testdef", Effect: e}
	return in
}

// fairyConvert はノーマル技をフェアリーにして威力×1.2(フェアリースキン相当)。
func fairyConvert() *AbilityEffect {
	return &AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeFairy, PowerMod: 4915}}
}

// --- AC-1 TypeConvert ----------------------------------------------------------

func TestTypeConvertActsLikeConvertedMoveWithPowerBoost(t *testing.T) {
	cases := []struct {
		name     string
		atkTypes []Type
		defTypes []Type
		cat      MoveCategory
		edit     func(in *DamageInput)
		wantEff  float64
	}{
		{"物理・抜群になる(格闘に)", []Type{TypeWater}, []Type{TypeFighting}, CategoryPhysical, nil, 2},
		{"特殊・いまひとつになる(鋼に)", []Type{TypeWater}, []Type{TypeSteel}, CategorySpecial, nil, 0.5},
		{"変換後のタイプで一致(フェアリーの攻撃側)", []Type{TypeFairy}, []Type{TypePsychic}, CategoryPhysical, nil, 1},
		{"元のタイプの一致は失う(ノーマルの攻撃側)", []Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, nil, 1},
		{"ゴースト相手でも当たる(ノーマルのままなら無効)", []Type{TypeWater}, []Type{TypeGhost}, CategoryPhysical, nil, 1},
		{"変換後のタイプの半減(防御側の特性)", []Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, func(in *DamageInput) {
			in.Defender.Ability = Ability{ID: "testresist", Effect: &AbilityEffect{DefResistType: map[Type]int{TypeFairy: ModifierHalf}}}
		}, 1},
		{"変換後のタイプの半減きのみ(抜群)", []Type{TypeWater}, []Type{TypeDragon}, CategorySpecial, func(in *DamageInput) {
			in.Defender.Item = &Item{ID: "testberry", Effect: &ItemEffect{ResistBerryType: TypeFairy}}
		}, 2},
		{"変換後のタイプのフィールド補正(ミストのドラゴン半減は掛からない)", []Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, func(in *DamageInput) {
			in.Field.Terrain = TerrainMisty
		}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ctrlInput(tc.atkTypes, tc.defTypes, tc.cat, TypeNormal)
			if tc.edit != nil {
				tc.edit(&in)
			}
			got := mustCalc(t, withAttackerAbility(in, fairyConvert()))

			// 同じ補正を「フェアリー技 + フェアリー技の威力×1.2 の持ち物」で表した入力。
			want := in
			want.Move.Type = TypeFairy
			want.Attacker.Item = &Item{ID: "testfeather", Effect: &ItemEffect{BoostType: TypeFairy, BoostTypeMod: 4915}}
			w := mustCalc(t, want)

			assertSameRolls(t, tc.name, got, w)
			if got.Effectiveness != tc.wantEff || got.Effectiveness != w.Effectiveness {
				t.Errorf("Effectiveness=%v, want %v", got.Effectiveness, tc.wantEff)
			}
			if got.STAB != w.STAB {
				t.Errorf("STAB=%v, want %v(変換後のタイプで判定)", got.STAB, w.STAB)
			}
			if got.Nullified != NullifyNone {
				t.Errorf("Nullified=%q, want 無効化なし", got.Nullified)
			}
		})
	}
}

// 変換後のタイプを防御側の特性が無効にする(ノーマル技のままなら当たる)。
func TestTypeConvertUsesConvertedTypeForAbilityImmunity(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in = withDefenderAbility(in, &AbilityEffect{DefImmuneTypes: []Type{TypeFairy}})
	got := mustCalc(t, withAttackerAbility(in, fairyConvert()))
	if got.Nullified != NullifyImmune {
		t.Errorf("Nullified=%q, want %q(変換後のフェアリー技が無効)", got.Nullified, NullifyImmune)
	}
}

func TestTypeConvertDoesNotApply(t *testing.T) {
	t.Run("From 以外のタイプの技", func(t *testing.T) {
		in := ctrlInput([]Type{TypeWater}, []Type{TypeFighting}, CategoryPhysical, TypeFire)
		assertSameRolls(t, "炎技", mustCalc(t, withAttackerAbility(in, fairyConvert())), mustCalc(t, in))
	})
	t.Run("type_change の機構を持つ技は変えない(印は残す)", func(t *testing.T) {
		in := ctrlInput([]Type{TypeWater}, []Type{TypeFighting}, CategoryPhysical, TypeNormal)
		in.Move.Mechanisms = []MoveMechanism{MechanismTypeChange}
		got := mustCalc(t, withAttackerAbility(in, fairyConvert()))
		assertSameRolls(t, "type_change", got, mustCalc(t, in))
		if got.Effectiveness != 1 {
			t.Errorf("Effectiveness=%v, want 1(ノーマルのまま)", got.Effectiveness)
		}
		if !hasMark(got.Unsupported, UnsupportedTargetMove, UnsupportedReason(MechanismTypeChange)) {
			t.Errorf("type_change の印が無い: %+v", got.Unsupported)
		}
	})
	t.Run("防御側が持っても効かない", func(t *testing.T) {
		in := ctrlInput([]Type{TypeWater}, []Type{TypeFighting}, CategoryPhysical, TypeNormal)
		assertSameRolls(t, "防御側", mustCalc(t, withDefenderAbility(in, fairyConvert())), mustCalc(t, in))
	})
}

func TestTypeConvertToUnknownTypeIsError(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in = withAttackerAbility(in, &AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: Type("nosuchtype"), PowerMod: 4915}})
	if _, err := calcDamage(in); !errors.Is(err, ErrUnknownType) {
		t.Errorf("err=%v, want ErrUnknownType(表に無いタイプへ変えたときに等倍として計算しない)", err)
	}
}

func hasMark(marks []UnsupportedMark, target UnsupportedTarget, reason UnsupportedReason) bool {
	for _, m := range marks {
		if m.Target == target && m.Reason == reason {
			return true
		}
	}
	return false
}

// --- AC-2 実数値補正 -------------------------------------------------------------

func TestAbilityStatModsActLikeItemStatMods(t *testing.T) {
	cases := []struct {
		name     string
		side     string // "a" / "d"
		mods     map[StatKey]int
		cat      MoveCategory
		wantDiff bool
	}{
		{"攻撃側 atk×2 で物理", "a", map[StatKey]int{StatAtk: 8192}, CategoryPhysical, true},
		{"攻撃側 atk×2 で特殊(効かない)", "a", map[StatKey]int{StatAtk: 8192}, CategorySpecial, false},
		{"防御側 def×2 で物理", "d", map[StatKey]int{StatDef: 8192}, CategoryPhysical, true},
		{"防御側 def×2 で特殊(効かない)", "d", map[StatKey]int{StatDef: 8192}, CategorySpecial, false},
		{"攻撃側が def を持っても効かない", "a", map[StatKey]int{StatDef: 8192}, CategoryPhysical, false},
		{"防御側が atk を持っても効かない", "d", map[StatKey]int{StatAtk: 8192}, CategoryPhysical, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, tc.cat, TypeNormal)
			e := &AbilityEffect{StatMods: tc.mods}
			viaItem := base
			var got DamageResult
			if tc.side == "a" {
				got = mustCalc(t, withAttackerAbility(base, e))
				viaItem.Attacker.Item = &Item{ID: "testitem", Effect: &ItemEffect{StatMods: tc.mods}}
			} else {
				got = mustCalc(t, withDefenderAbility(base, e))
				viaItem.Defender.Item = &Item{ID: "testitem", Effect: &ItemEffect{StatMods: tc.mods}}
			}
			assertSameRolls(t, "持ち物の StatMods と同じ", got, mustCalc(t, viaItem))
			if tc.wantDiff {
				assertDifferentRolls(t, tc.name, got, mustCalc(t, base))
			} else {
				assertSameRolls(t, tc.name, got, mustCalc(t, base))
			}
		})
	}
}

// はりきり相当: ランクの後に単独で丸め(floor(atk×1.5))、その後に持ち物の実数値補正を連鎖とは別に掛ける。
// 攻撃 201(種族値 181)・持ち物 ×4505 で、単独丸めは 301 → 331、連鎖させると 332 になる組。
func TestSeparateStatModsRoundBeforeChainedMods(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Species.BaseStats = in.Attacker.Species.BaseStats.WithStat(StatAtk, 181)
	in.Attacker.Item = &Item{ID: "testitem", Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 4505}}}
	in = withAttackerAbility(in, &AbilityEffect{SeparateStatMods: map[StatKey]int{StatAtk: 6144}})
	got := mustCalc(t, in)
	// atk=331, def=100: floor(floor(22*100*331/100)/50)+2 = 147。乱数 85% は floor(147*85/100)=124。
	if got.Rolls[15] != 147 || got.Rolls[0] != 124 {
		t.Errorf("rolls[0]=%d rolls[15]=%d, want 124 147(単独で丸める。連鎖させると 148)", got.Rolls[0], got.Rolls[15])
	}
}

func TestSeparateStatModsAfterRanksAndOnlyOffensive(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Ranks.Atk = 1 // 200 → 300 → ×1.5 = 450
	got := mustCalc(t, withAttackerAbility(in, &AbilityEffect{SeparateStatMods: map[StatKey]int{StatAtk: 6144}}))
	// atk=450, def=100: floor(9900/50)+2 = 200。
	if got.Rolls[15] != 200 || got.Rolls[0] != 170 {
		t.Errorf("rolls[0]=%d rolls[15]=%d, want 170 200", got.Rolls[0], got.Rolls[15])
	}

	sp := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal)
	assertSameRolls(t, "atk の補正は特殊技に効かない",
		mustCalc(t, withAttackerAbility(sp, &AbilityEffect{SeparateStatMods: map[StatKey]int{StatAtk: 6144}})), mustCalc(t, sp))

	def := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	assertSameRolls(t, "防御側が持っても効かない",
		mustCalc(t, withDefenderAbility(def, &AbilityEffect{SeparateStatMods: map[StatKey]int{StatAtk: 6144}})), mustCalc(t, def))
}

// --- AC-3 威力補正 ----------------------------------------------------------------

func TestConditionalPowerMods(t *testing.T) {
	technician := &AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}}
	steely := &AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveType, MoveType: TypeSteel, Modifier: 6144}}}
	cases := []struct {
		name     string
		effect   *AbilityEffect
		moveType Type
		power    int
		apply    bool
	}{
		{"威力 60(境界)", technician, TypeNormal, 60, true},
		{"威力 40", technician, TypeNormal, 40, true},
		{"威力 61", technician, TypeNormal, 61, false},
		{"鋼技", steely, TypeSteel, 80, true},
		{"鋼以外", steely, TypeNormal, 80, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, tc.moveType)
			in.Move.Power = tc.power
			got := mustCalc(t, withAttackerAbility(in, tc.effect))
			want := in
			if tc.apply {
				want.Attacker.Item = &Item{ID: "testpower", Effect: &ItemEffect{PowerMod: 6144}}
			}
			assertSameRolls(t, tc.name, got, mustCalc(t, want))
		})
	}
}

func TestConditionalPowerModMoveTypeUsesConvertedType(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	both := &AbilityEffect{
		TypeConvert: &TypeConvert{From: TypeNormal, To: TypeSteel, PowerMod: 4915},
		PowerMods:   []ConditionalPowerMod{{Condition: PowerConditionMoveType, MoveType: TypeSteel, Modifier: 6144}},
	}
	convertOnly := &AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeSteel, PowerMod: 4915}}
	assertDifferentRolls(t, "変換後の鋼技に move_type が掛かる",
		mustCalc(t, withAttackerAbility(in, both)), mustCalc(t, withAttackerAbility(in, convertOnly)))
}

func TestAuraType(t *testing.T) {
	aura := &AbilityEffect{AuraType: TypeFairy, AuraMod: 5448}
	fairy := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFairy)
	viaItem := fairy
	viaItem.Attacker.Item = &Item{ID: "testpower", Effect: &ItemEffect{PowerMod: 5448}}
	want := mustCalc(t, viaItem)

	assertSameRolls(t, "攻撃側が持つ", mustCalc(t, withAttackerAbility(fairy, aura)), want)
	assertSameRolls(t, "防御側が持つ", mustCalc(t, withDefenderAbility(fairy, aura)), want)
	assertSameRolls(t, "両側が持っても1回", mustCalc(t, withDefenderAbility(withAttackerAbility(fairy, aura), aura)), want)

	other := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	assertSameRolls(t, "別タイプには効かない", mustCalc(t, withDefenderAbility(other, aura)), mustCalc(t, other))

	// 変換後のフェアリー技にも掛かる。
	conv := withAttackerAbility(other, fairyConvert())
	assertDifferentRolls(t, "変換後のタイプで判定", mustCalc(t, withDefenderAbility(conv, aura)), mustCalc(t, conv))
}

// --- AC-4 急所 -------------------------------------------------------------------

func TestCritDamageMod(t *testing.T) {
	sniper := &AbilityEffect{CritDamageMod: 6144}
	crit := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	crit.Critical = true
	viaItem := crit
	viaItem.Attacker.Item = &Item{ID: "testorb", Effect: &ItemEffect{DamageMod: 6144}}
	assertSameRolls(t, "急所", mustCalc(t, withAttackerAbility(crit, sniper)), mustCalc(t, viaItem))

	plain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	assertSameRolls(t, "急所でない", mustCalc(t, withAttackerAbility(plain, sniper)), mustCalc(t, plain))
}

func TestPreventsCritical(t *testing.T) {
	armor := &AbilityEffect{PreventsCritical: true}
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.Ranks.Atk = -2
	in.Defender.Ranks.Def = 2
	in.Field.DefenderScreens.Reflect = true
	noCrit := mustCalc(t, in)

	crit := in
	crit.Critical = true
	assertDifferentRolls(t, "前提: 急所でダメージが変わる組", mustCalc(t, crit), noCrit)
	assertSameRolls(t, "急所に当たらない(ランク・壁・×1.5 も急所でないときと同じ)",
		mustCalc(t, withDefenderAbility(crit, armor)), noCrit)

	sniperCrit := withAttackerAbility(crit, &AbilityEffect{CritDamageMod: 6144})
	assertSameRolls(t, "急所でなければ CritDamageMod も掛からない", mustCalc(t, withDefenderAbility(sniperCrit, armor)), noCrit)

	assertSameRolls(t, "攻撃側が持っても効かない", mustCalc(t, withAttackerAbility(crit, armor)), mustCalc(t, crit))
}

func TestPreventsCriticalHandlesAlwaysCritMark(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Move.Mechanisms = []MoveMechanism{MechanismAlwaysCrit}
	got := mustCalc(t, withDefenderAbility(in, &AbilityEffect{PreventsCritical: true}))
	if hasMark(got.Unsupported, UnsupportedTargetMove, UnsupportedReason(MechanismAlwaysCrit)) {
		t.Errorf("急所に当たらない相手には必ず急所の技も通常の式で正しい(印は不要): %+v", got.Unsupported)
	}
}

// --- AC-5 ランク無視 -------------------------------------------------------------

func TestIgnoresOpponentRanks(t *testing.T) {
	unaware := &AbilityEffect{IgnoresOpponentRanks: true}
	base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	zero := mustCalc(t, base)

	for _, tc := range []struct {
		name string
		edit func(in *DamageInput)
	}{
		{"防御側が持つ: 攻撃側の攻撃+2 を無視", func(in *DamageInput) { in.Attacker.Ranks.Atk = 2; *in = withDefenderAbility(*in, unaware) }},
		{"防御側が持つ: 攻撃側の攻撃-2 も無視", func(in *DamageInput) { in.Attacker.Ranks.Atk = -2; *in = withDefenderAbility(*in, unaware) }},
		{"攻撃側が持つ: 防御側の防御+2 を無視", func(in *DamageInput) { in.Defender.Ranks.Def = 2; *in = withAttackerAbility(*in, unaware) }},
		{"攻撃側が持つ: 防御側の防御-2 も無視", func(in *DamageInput) { in.Defender.Ranks.Def = -2; *in = withAttackerAbility(*in, unaware) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.edit(&in)
			assertSameRolls(t, tc.name, mustCalc(t, in), zero)
		})
	}

	t.Run("自分のランクは無視しない", func(t *testing.T) {
		in := base
		in.Attacker.Ranks.Atk = 2
		assertDifferentRolls(t, "攻撃側が持ち自分の攻撃+2", mustCalc(t, withAttackerAbility(in, unaware)), zero)
	})
}

func TestIgnoresOpponentRanksHandlesIgnoreDefenseRanksMark(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Move.Mechanisms = []MoveMechanism{MechanismIgnoreDefenseRanks}
	in.Defender.Ranks.Def = 2
	got := mustCalc(t, withAttackerAbility(in, &AbilityEffect{IgnoresOpponentRanks: true}))
	if hasMark(got.Unsupported, UnsupportedTargetMove, UnsupportedReason(MechanismIgnoreDefenseRanks)) {
		t.Errorf("攻撃側が相手のランクを無視するなら防御ランク無視の技も通常の式で正しい: %+v", got.Unsupported)
	}
}

// --- AC-6 防御側の特性を無視 -----------------------------------------------------

func TestIgnoresDefenderAbility(t *testing.T) {
	breaker := &AbilityEffect{IgnoresDefenderAbility: true}
	fire := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire)

	t.Run("Breakable な防御側の特性は無いものとして計算", func(t *testing.T) {
		resist := &AbilityEffect{DefResistType: map[Type]int{TypeFire: ModifierHalf}, Breakable: true}
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(fire, breaker), resist))
		assertSameRolls(t, "半減を無視", got, mustCalc(t, fire))
	})
	t.Run("Breakable でない特性は無視しない", func(t *testing.T) {
		resist := &AbilityEffect{DefResistType: map[Type]int{TypeFire: ModifierHalf}}
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(fire, breaker), resist))
		assertSameRolls(t, "半減のまま", got, mustCalc(t, withDefenderAbility(fire, resist)))
	})
	t.Run("無効も無視する(浮いていても当たる)", func(t *testing.T) {
		ground := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeGround)
		levitate := &AbilityEffect{DefImmuneTypes: []Type{TypeGround}, Airborne: true, Breakable: true}
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(ground, breaker), levitate))
		if got.Nullified != NullifyNone {
			t.Errorf("Nullified=%q, want 無効化なし", got.Nullified)
		}
		assertSameRolls(t, "無効を無視", got, mustCalc(t, ground))
	})
	t.Run("急所に当たらない特性も無視する", func(t *testing.T) {
		crit := fire
		crit.Critical = true
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(crit, breaker), &AbilityEffect{PreventsCritical: true, Breakable: true}))
		assertSameRolls(t, "急所が通る", got, mustCalc(t, crit))
	})
	t.Run("ランク無視の特性も無視する", func(t *testing.T) {
		in := fire
		in.Attacker.Ranks.Atk = 2
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(in, breaker), &AbilityEffect{IgnoresOpponentRanks: true, Breakable: true}))
		assertSameRolls(t, "攻撃ランクが効く", got, mustCalc(t, in))
	})
	t.Run("無視した特性の未対応の印は付けない", func(t *testing.T) {
		marked := &AbilityEffect{UnsupportedDefender: true, Breakable: true}
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(fire, breaker), marked))
		if hasMark(got.Unsupported, UnsupportedTargetDefenderAbility, UnsupportedEffect) {
			t.Errorf("無視した特性に印が付いた: %+v", got.Unsupported)
		}
		kept := mustCalc(t, withDefenderAbility(withAttackerAbility(fire, breaker), &AbilityEffect{UnsupportedDefender: true}))
		if !hasMark(kept.Unsupported, UnsupportedTargetDefenderAbility, UnsupportedEffect) {
			t.Errorf("Breakable でない特性の印は残す: %+v", kept.Unsupported)
		}
	})
	t.Run("持たなければ無視しない", func(t *testing.T) {
		resist := &AbilityEffect{DefResistType: map[Type]int{TypeFire: ModifierHalf}, Breakable: true}
		assertDifferentRolls(t, "半減が効く", mustCalc(t, withDefenderAbility(fire, resist)), mustCalc(t, fire))
	})
	t.Run("攻撃側の特性は無視されない(防御側が持っても効かない)", func(t *testing.T) {
		boost := &AbilityEffect{StatMods: map[StatKey]int{StatAtk: 8192}, Breakable: true}
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(fire, boost), breaker))
		assertSameRolls(t, "攻撃側の補正は残る", got, mustCalc(t, withAttackerAbility(fire, boost)))
	})
}

// --- AC-7 検証 -------------------------------------------------------------------

func TestStage1AbilityEffectValidation(t *testing.T) {
	over := MaxEffectModifier + 1
	invalid := []struct {
		name string
		e    AbilityEffect
	}{
		{"TypeConvert: To が空", AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, PowerMod: 4915}}},
		{"TypeConvert: From が空", AbilityEffect{TypeConvert: &TypeConvert{To: TypeFairy, PowerMod: 4915}}},
		{"TypeConvert: From == To", AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeNormal, PowerMod: 4915}}},
		{"TypeConvert: PowerMod 0", AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeFairy}}},
		{"TypeConvert: PowerMod 上限超え", AbilityEffect{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeFairy, PowerMod: over}}},
		{"PowerMods: 語彙に無い条件", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: "contact", Modifier: 5325}}}},
		{"PowerMods: Modifier 0", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60}}}},
		{"PowerMods: Modifier 中立", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: Modifier4096}}}},
		{"PowerMods: Modifier 上限超え", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: over}}}},
		{"PowerMods: max_base_power で MaxPower 0", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, Modifier: 6144}}}},
		{"PowerMods: max_base_power で MoveType あり", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, MoveType: TypeSteel, Modifier: 6144}}}},
		{"PowerMods: move_type で MoveType 空", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveType, Modifier: 6144}}}},
		{"PowerMods: move_type で MaxPower あり", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveType, MoveType: TypeSteel, MaxPower: 60, Modifier: 6144}}}},
		{"AuraType だけ", AbilityEffect{AuraType: TypeFairy}},
		{"AuraMod だけ", AbilityEffect{AuraMod: 5448}},
		{"AuraMod 上限超え", AbilityEffect{AuraType: TypeFairy, AuraMod: over}},
		{"StatMods 0", AbilityEffect{StatMods: map[StatKey]int{StatAtk: 0}}},
		{"StatMods 上限超え", AbilityEffect{StatMods: map[StatKey]int{StatDef: over}}},
		{"StatMods に spe", AbilityEffect{StatMods: map[StatKey]int{StatSpe: 8192}}},
		{"StatMods に hp", AbilityEffect{StatMods: map[StatKey]int{StatHP: 8192}}},
		{"SeparateStatMods に def", AbilityEffect{SeparateStatMods: map[StatKey]int{StatDef: 6144}}},
		{"SeparateStatMods 0", AbilityEffect{SeparateStatMods: map[StatKey]int{StatAtk: 0}}},
		{"SeparateStatMods 上限超え", AbilityEffect{SeparateStatMods: map[StatKey]int{StatAtk: over}}},
		{"CritDamageMod 負", AbilityEffect{CritDamageMod: -1}},
		{"CritDamageMod 上限超え", AbilityEffect{CritDamageMod: over}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			in := mkIndiv([]Type{TypeWater}, Stats{Atk: 100})
			e := tc.e
			in.Ability = Ability{ID: "testbad", Effect: &e}
			if err := in.Validate(); err == nil {
				t.Errorf("Validate() = nil, want エラー")
			}
		})
	}

	valid := []AbilityEffect{
		{TypeConvert: &TypeConvert{From: TypeNormal, To: TypeFairy, PowerMod: 4915}},
		{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}},
		{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveType, MoveType: TypeSteel, Modifier: 6144}}},
		{AuraType: TypeFairy, AuraMod: 5448},
		{StatMods: map[StatKey]int{StatAtk: 8192}},
		{StatMods: map[StatKey]int{StatDef: 8192}},
		{SeparateStatMods: map[StatKey]int{StatAtk: 6144}},
		{SeparateStatMods: map[StatKey]int{StatSpA: 6144}},
		{CritDamageMod: 6144},
		{PreventsCritical: true, Breakable: true},
		{IgnoresOpponentRanks: true, Breakable: true},
		{IgnoresDefenderAbility: true},
	}
	for i, e := range valid {
		in := mkIndiv([]Type{TypeWater}, Stats{Atk: 100})
		e := e
		in.Ability = Ability{ID: "testok", Effect: &e}
		if err := in.Validate(); err != nil {
			t.Errorf("valid[%d] %+v: Validate() = %v", i, e, err)
		}
	}
}

func TestAllPowerConditionsAreKnown(t *testing.T) {
	got := AllPowerConditions()
	if len(got) != 3 {
		t.Fatalf("AllPowerConditions()=%v, want 3件(段階2で move_flag を足した。ADR-0178)", got)
	}
	for _, c := range got {
		if !c.Known() {
			t.Errorf("%q が Known でない", c)
		}
	}
	for _, c := range []PowerCondition{"", "Max_Base_Power", "contact"} {
		if c.Known() {
			t.Errorf("%q が Known になった", c)
		}
	}
}
