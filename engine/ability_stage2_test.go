package engine

// ADR-0178: 技のフラグと、フラグに依存する特性(段階2)。
//
// 受け入れ条件(engine):
//   - AC-E1 語彙: AllMoveFlags は bite・bullet・contact・pulse・punch・recoil・secondary・slicing・sound の昇順で、
//     Known はその9種だけを真にする(大文字小文字を区別)。PowerCondition の語彙に move_flag が入る。
//   - AC-E2 技の入力: 未知のフラグ、FlagsKnown が偽なのにフラグがある入力は ErrInvalidMoveFlags で拒否する。
//   - AC-E3 PowerMods の move_flag(オーラの前)・PostAuraPowerMods(オーラの後)は、技がそのフラグを持つときだけ
//     威力に掛ける。単独なら「同じ倍率の威力の持ち物」と同じ結果。
//   - AC-E4 FlagTypeConvert: そのフラグの技を To タイプにする(威力補正なし)。type_change の機構の技・フラグの無い技は変えない。
//   - AC-E5 DefImmuneFlags: そのフラグの技は Nullified=immune でダメージ 0。Breakable ならかたやぶりで無視される。
//   - AC-E6 DefFinalModsByFlag / DefFinalModsByType: 技のフラグ・(変換後の)タイプで最終補正に掛ける。
//     NoContact の攻撃側には contact の最終補正を掛けない。
//   - AC-E7 フラグが不明(FlagsKnown 偽)のとき: フラグ依存の項目は効かないものとして計算し、その特性に
//     unsupported_effect の印を付ける。フラグが分かっていれば(持っていなくても)印を付けない。変化技・無視した特性には付けない。
//   - AC-E8 Individual.Validate が新しい項目の不正な値を拒否する。
//
// 期待値は段階1(ability_stage1_test.go)と同じく「同じ補正を既存の(検証済みの)項目で表した入力」との一致で書く。
// 連鎖の順・丸めの位置の正しさは oracle のゴールデン(fixed.json の effects/<id>/...。TestGoldenCoversStage2AbilityEffects)で見る。

import (
	"errors"
	"reflect"
	"testing"
)

// flagged は技にフラグを持たせ、フラグが分かっている状態にした入力を返す。
func flagged(in DamageInput, flags ...MoveFlag) DamageInput {
	in.Move.Flags = flags
	in.Move.FlagsKnown = true
	return in
}

func flagPowerMod(c PowerCondition, f MoveFlag, mod int) ConditionalPowerMod {
	return ConditionalPowerMod{Condition: c, Flag: f, Modifier: mod}
}

// withPowerItem は攻撃側に「全分類の威力×mod」の持ち物を持たせる(威力の補正1つと同じ結果になる対照)。
func withPowerItem(in DamageInput, mod int) DamageInput {
	in.Attacker.Item = &Item{ID: "testpower", Effect: &ItemEffect{PowerMod: mod}}
	return in
}

// withFinalItem は攻撃側に「最終ダメージ×mod」の持ち物を持たせる(最終補正1つと同じ結果になる対照)。
func withFinalItem(in DamageInput, mod int) DamageInput {
	in.Attacker.Item = &Item{ID: "testfinal", Effect: &ItemEffect{DamageMod: mod}}
	return in
}

// --- AC-E1 語彙 -------------------------------------------------------------------

func TestMoveFlagVocabulary(t *testing.T) {
	want := []MoveFlag{"bite", "bullet", "contact", "pulse", "punch", "recoil", "secondary", "slicing", "sound"}
	got := AllMoveFlags()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllMoveFlags()=%v, want %v", got, want)
	}
	got[0] = "changed"
	if AllMoveFlags()[0] != "bite" {
		t.Error("AllMoveFlags が内部のスライスを返している(呼び出しごとに新しいコピーにする)")
	}
	for _, f := range want {
		if !f.Known() {
			t.Errorf("%q が Known でない", f)
		}
	}
	for _, f := range []MoveFlag{"", "Contact", "SOUND", "protect", "wind", "mirror"} {
		if f.Known() {
			t.Errorf("%q が Known になった(語彙は9種だけ)", f)
		}
	}
}

func TestPowerConditionMoveFlagIsKnown(t *testing.T) {
	if !PowerConditionMoveFlag.Known() {
		t.Errorf("%q が Known でない", PowerConditionMoveFlag)
	}
	found := false
	for _, c := range AllPowerConditions() {
		if c == PowerConditionMoveFlag {
			found = true
		}
	}
	if !found {
		t.Errorf("AllPowerConditions()=%v に move_flag が無い", AllPowerConditions())
	}
}

// --- AC-E2 技の入力 ------------------------------------------------------------

func TestCalcDamageRejectsInvalidMoveFlags(t *testing.T) {
	base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	cases := []struct {
		name  string
		flags []MoveFlag
		known bool
	}{
		{"未知のフラグ", []MoveFlag{"protect"}, true},
		{"大文字違い", []MoveFlag{"Contact"}, true},
		{"空文字", []MoveFlag{""}, true},
		{"不明なのに値がある", []MoveFlag{MoveFlagContact}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			in.Move.Flags = tc.flags
			in.Move.FlagsKnown = tc.known
			if _, err := calcDamage(in); !errors.Is(err, ErrInvalidMoveFlags) {
				t.Errorf("err=%v, want ErrInvalidMoveFlags", err)
			}
		})
	}
	t.Run("既知のフラグ・空の既知・不明(空)は受け付ける", func(t *testing.T) {
		for _, in := range []DamageInput{flagged(base, MoveFlagContact, MoveFlagPunch), flagged(base), base} {
			if _, err := calcDamage(in); err != nil {
				t.Errorf("flags=%v known=%v: err=%v", in.Move.Flags, in.Move.FlagsKnown, err)
			}
		}
	})
	t.Run("特性が無ければフラグは数値に影響しない", func(t *testing.T) {
		assertSameRolls(t, "フラグだけ", mustCalc(t, flagged(base, MoveFlagContact, MoveFlagSound, MoveFlagRecoil)), mustCalc(t, base))
	})
}

// --- AC-E3 威力の補正 ----------------------------------------------------------

func TestFlagPowerModsActLikePowerItem(t *testing.T) {
	cases := []struct {
		name   string
		effect *AbilityEffect
		flag   MoveFlag
		mod    int
	}{
		{"オーラの前(かみつき ×1.5)", &AbilityEffect{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagBite, 6144)}}, MoveFlagBite, 6144},
		{"オーラの前(波動 ×1.5)", &AbilityEffect{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagPulse, 6144)}}, MoveFlagPulse, 6144},
		{"オーラの前(切る ×1.5)", &AbilityEffect{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagSlicing, 6144)}}, MoveFlagSlicing, 6144},
		{"オーラの後(接触 ×1.3)", &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, 5325)}}, MoveFlagContact, 5325},
		{"オーラの後(音 ×1.3)", &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagSound, 5325)}}, MoveFlagSound, 5325},
		{"オーラの後(追加効果 ×1.3)", &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagSecondary, 5325)}}, MoveFlagSecondary, 5325},
		{"オーラの後(パンチ ×1.2)", &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagPunch, 4915)}}, MoveFlagPunch, 4915},
		{"オーラの後(反動 ×1.2)", &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagRecoil, 4915)}}, MoveFlagRecoil, 4915},
	}
	for _, tc := range cases {
		for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
			t.Run(tc.name+"/"+string(cat), func(t *testing.T) {
				in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, cat, TypeNormal)
				with := flagged(in, tc.flag)
				got := mustCalc(t, withAttackerAbility(with, tc.effect))
				want := mustCalc(t, withPowerItem(with, tc.mod))
				assertSameRolls(t, "フラグを持つ技", got, want)
				assertDifferentRolls(t, "フラグを持つ技", got, mustCalc(t, with))
				if len(got.Unsupported) != 0 {
					t.Errorf("フラグが分かっているのに印が付いた: %+v", got.Unsupported)
				}

				// 別のフラグだけを持つ技・フラグの無い技には掛けない(対照)。
				other := MoveFlagBullet
				if tc.flag == other {
					other = MoveFlagContact
				}
				for _, ctrl := range []DamageInput{flagged(in, other), flagged(in)} {
					assertSameRolls(t, "フラグを持たない技", mustCalc(t, withAttackerAbility(ctrl, tc.effect)), mustCalc(t, ctrl))
				}
			})
		}
	}
}

// 防御側が持っても攻撃側の威力の補正は効かない(PowerMods・PostAuraPowerMods は攻撃側だけ)。
func TestFlagPowerModsAreAttackerOnly(t *testing.T) {
	in := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal), MoveFlagContact)
	e := &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, 5325)}}
	assertSameRolls(t, "防御側が持つ", mustCalc(t, withDefenderAbility(in, e)), mustCalc(t, in))
}

// --- AC-E4 フラグによるタイプ変換 ------------------------------------------------

func liquidVoice() *AbilityEffect {
	return &AbilityEffect{FlagTypeConvert: &FlagTypeConvert{Flag: MoveFlagSound, To: TypeWater}}
}

func TestFlagTypeConvert(t *testing.T) {
	t.Run("音の技を水にする(威力補正なし)", func(t *testing.T) {
		for _, atkTypes := range [][]Type{{TypeFire}, {TypeWater}, {TypeNormal}} {
			in := flagged(ctrlInput(atkTypes, []Type{TypeFire}, CategorySpecial, TypeNormal), MoveFlagSound)
			got := mustCalc(t, withAttackerAbility(in, liquidVoice()))
			want := in
			want.Move.Type = TypeWater
			w := mustCalc(t, want)
			assertSameRolls(t, string(atkTypes[0]), got, w)
			if got.Effectiveness != 2 || got.STAB != w.STAB {
				t.Errorf("Effectiveness=%v STAB=%v, want 2・%v(変換後の水で判定)", got.Effectiveness, got.STAB, w.STAB)
			}
		}
	})
	t.Run("音でない技は変えない", func(t *testing.T) {
		in := flagged(ctrlInput([]Type{TypeFire}, []Type{TypeFire}, CategorySpecial, TypeNormal), MoveFlagBullet)
		assertSameRolls(t, "弾の技", mustCalc(t, withAttackerAbility(in, liquidVoice())), mustCalc(t, in))
	})
	t.Run("type_change の機構を持つ技は変えない", func(t *testing.T) {
		in := flagged(ctrlInput([]Type{TypeFire}, []Type{TypeFire}, CategorySpecial, TypeNormal), MoveFlagSound)
		in.Move.Mechanisms = []MoveMechanism{MechanismTypeChange}
		assertSameRolls(t, "type_change", mustCalc(t, withAttackerAbility(in, liquidVoice())), mustCalc(t, in))
	})
	t.Run("変換後のタイプを防御側の特性が無効にする", func(t *testing.T) {
		in := flagged(ctrlInput([]Type{TypeFire}, []Type{TypePsychic}, CategorySpecial, TypeNormal), MoveFlagSound)
		in = withDefenderAbility(in, &AbilityEffect{DefImmuneTypes: []Type{TypeWater}})
		if got := mustCalc(t, withAttackerAbility(in, liquidVoice())); got.Nullified != NullifyImmune {
			t.Errorf("Nullified=%q, want immune(変換後の水の技)", got.Nullified)
		}
	})
	t.Run("表に無いタイプへの変換は ErrUnknownType", func(t *testing.T) {
		in := flagged(ctrlInput([]Type{TypeFire}, []Type{TypePsychic}, CategorySpecial, TypeNormal), MoveFlagSound)
		in = withAttackerAbility(in, &AbilityEffect{FlagTypeConvert: &FlagTypeConvert{Flag: MoveFlagSound, To: "nosuchtype"}})
		if _, err := calcDamage(in); !errors.Is(err, ErrUnknownType) {
			t.Errorf("err=%v, want ErrUnknownType", err)
		}
	})
}

// --- AC-E5 フラグによる無効 --------------------------------------------------------

func TestDefImmuneFlags(t *testing.T) {
	soundproof := &AbilityEffect{DefImmuneFlags: []MoveFlag{MoveFlagSound}, Breakable: true}
	in := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal), MoveFlagSound)

	t.Run("音の技は無効", func(t *testing.T) {
		got := mustCalc(t, withDefenderAbility(in, soundproof))
		if got.Nullified != NullifyImmune {
			t.Errorf("Nullified=%q, want immune", got.Nullified)
		}
		if got.MaxDamage() != 0 {
			t.Errorf("rolls=%v, want 全て 0", got.Rolls)
		}
	})
	t.Run("音でない技は当たる", func(t *testing.T) {
		ctrl := flagged(in, MoveFlagBullet)
		got := mustCalc(t, withDefenderAbility(ctrl, soundproof))
		if got.Nullified != NullifyNone {
			t.Errorf("Nullified=%q, want 無効化なし", got.Nullified)
		}
		assertSameRolls(t, "弾の技", got, mustCalc(t, ctrl))
	})
	t.Run("タイプ相性の無効が先(ゴーストにノーマルの音技)", func(t *testing.T) {
		ghost := flagged(ctrlInput([]Type{TypeWater}, []Type{TypeGhost}, CategorySpecial, TypeNormal), MoveFlagSound)
		got := mustCalc(t, withDefenderAbility(ghost, soundproof))
		if got.Effectiveness != 0 || got.Nullified != NullifyNone {
			t.Errorf("Effectiveness=%v Nullified=%q, want 0 と空(タイプ由来として報告)", got.Effectiveness, got.Nullified)
		}
	})
	t.Run("かたやぶりで無視される(Breakable)", func(t *testing.T) {
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(in, &AbilityEffect{IgnoresDefenderAbility: true}), soundproof))
		if got.Nullified != NullifyNone {
			t.Errorf("Nullified=%q, want 無効化なし", got.Nullified)
		}
	})
}

// --- AC-E6 フラグ・タイプによる最終補正 ------------------------------------------------

func TestDefFinalModsByFlagAndType(t *testing.T) {
	fluffy := &AbilityEffect{
		DefFinalModsByFlag: map[MoveFlag]int{MoveFlagContact: ModifierHalf},
		DefFinalModsByType: map[Type]int{TypeFire: 8192},
		Breakable:          true,
	}
	normalContact := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal), MoveFlagContact)

	t.Run("接触の技は半減", func(t *testing.T) {
		got := mustCalc(t, withDefenderAbility(normalContact, fluffy))
		assertSameRolls(t, "接触", got, mustCalc(t, withFinalItem(normalContact, ModifierHalf)))
	})
	t.Run("接触でない技は変わらない", func(t *testing.T) {
		ctrl := flagged(normalContact, MoveFlagBullet)
		assertSameRolls(t, "非接触", mustCalc(t, withDefenderAbility(ctrl, fluffy)), mustCalc(t, ctrl))
	})
	t.Run("炎の非接触技は ×2", func(t *testing.T) {
		fire := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeFire))
		assertSameRolls(t, "炎", mustCalc(t, withDefenderAbility(fire, fluffy)), mustCalc(t, withFinalItem(fire, 8192)))
	})
	t.Run("炎の接触技は両方(半減の後に ×2 を連鎖する)", func(t *testing.T) {
		fire := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire), MoveFlagContact)
		want := fire
		want.Attacker.Item = &Item{ID: "testfinal", Effect: &ItemEffect{DamageMod: chainMods([]int{ModifierHalf, 8192}, finalModBounds)}}
		assertSameRolls(t, "炎の接触", mustCalc(t, withDefenderAbility(fire, fluffy)), mustCalc(t, want))
	})
	t.Run("変換後のタイプで判定(水に変えた音の技には炎の ×2 を掛けない)", func(t *testing.T) {
		fireSound := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeFire), MoveFlagSound)
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(fireSound, liquidVoice()), fluffy))
		want := fireSound
		want.Move.Type = TypeWater
		assertSameRolls(t, "水に変えた", got, mustCalc(t, want))
	})
	t.Run("接触しない扱いの攻撃側には接触の半減を掛けない", func(t *testing.T) {
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(normalContact, &AbilityEffect{NoContact: true}), fluffy))
		assertSameRolls(t, "えんかく", got, mustCalc(t, normalContact))
	})
	t.Run("接触しない扱いでも炎の ×2 は掛かる", func(t *testing.T) {
		fire := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeFire), MoveFlagContact)
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(fire, &AbilityEffect{NoContact: true}), fluffy))
		assertSameRolls(t, "えんかく・炎", got, mustCalc(t, withFinalItem(fire, 8192)))
	})
	t.Run("音の技の半減(パンクロックの防御側)", func(t *testing.T) {
		punk := &AbilityEffect{DefFinalModsByFlag: map[MoveFlag]int{MoveFlagSound: ModifierHalf}, Breakable: true}
		sound := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal), MoveFlagSound)
		assertSameRolls(t, "音", mustCalc(t, withDefenderAbility(sound, punk)), mustCalc(t, withFinalItem(sound, ModifierHalf)))
	})
	t.Run("かたやぶりで無視される(Breakable)", func(t *testing.T) {
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(normalContact, &AbilityEffect{IgnoresDefenderAbility: true}), fluffy))
		assertSameRolls(t, "かたやぶり", got, mustCalc(t, normalContact))
	})
	t.Run("攻撃側が持っても効かない", func(t *testing.T) {
		assertSameRolls(t, "攻撃側", mustCalc(t, withAttackerAbility(normalContact, fluffy)), mustCalc(t, normalContact))
	})
}

// パンクロック(攻撃側の音 ×1.3・防御側の音の半減)は同じ定義で両側に効く。かたやぶりは防御側だけを無視する。
func TestPunkRockBothSides(t *testing.T) {
	punk := &AbilityEffect{
		PostAuraPowerMods:  []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagSound, 5325)},
		DefFinalModsByFlag: map[MoveFlag]int{MoveFlagSound: ModifierHalf},
		Breakable:          true,
	}
	sound := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal), MoveFlagSound)
	assertSameRolls(t, "攻撃側", mustCalc(t, withAttackerAbility(sound, punk)), mustCalc(t, withPowerItem(sound, 5325)))
	assertSameRolls(t, "防御側", mustCalc(t, withDefenderAbility(sound, punk)), mustCalc(t, withFinalItem(sound, ModifierHalf)))
	breaker := &AbilityEffect{IgnoresDefenderAbility: true}
	assertSameRolls(t, "かたやぶりの攻撃側 × 防御側のパンクロック", mustCalc(t, withDefenderAbility(withAttackerAbility(sound, breaker), punk)), mustCalc(t, sound))
}

// --- AC-E7 フラグが不明なとき -------------------------------------------------------

func TestFlagDependentAbilityMarkedWhenFlagsUnknown(t *testing.T) {
	unknown := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal) // FlagsKnown 偽・Flags 空
	cases := []struct {
		name     string
		attacker *AbilityEffect
		defender *AbilityEffect
		target   UnsupportedTarget
	}{
		{"攻撃側 PowerMods(move_flag)", &AbilityEffect{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagBite, 6144)}}, nil, UnsupportedTargetAttackerAbility},
		{"攻撃側 PostAuraPowerMods", &AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, 5325)}}, nil, UnsupportedTargetAttackerAbility},
		{"攻撃側 FlagTypeConvert", liquidVoice(), nil, UnsupportedTargetAttackerAbility},
		{"防御側 DefImmuneFlags", nil, &AbilityEffect{DefImmuneFlags: []MoveFlag{MoveFlagBullet}, Breakable: true}, UnsupportedTargetDefenderAbility},
		{"防御側 DefFinalModsByFlag", nil, &AbilityEffect{DefFinalModsByFlag: map[MoveFlag]int{MoveFlagContact: ModifierHalf}, Breakable: true}, UnsupportedTargetDefenderAbility},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := unknown
			if tc.attacker != nil {
				in = withAttackerAbility(in, tc.attacker)
			}
			if tc.defender != nil {
				in = withDefenderAbility(in, tc.defender)
			}
			got := mustCalc(t, in)
			if !hasMark(got.Unsupported, tc.target, UnsupportedEffect) {
				t.Errorf("フラグが不明なのに %s の印が無い: %+v", tc.target, got.Unsupported)
			}
			assertSameRolls(t, "フラグ依存の項目は効かない", got, mustCalc(t, unknown))
			if got.Nullified != NullifyNone {
				t.Errorf("Nullified=%q, want 無効化なし(フラグが不明なので無効にしない)", got.Nullified)
			}

			// フラグが分かっていれば(持っていなくても)印を付けない。
			known := flagged(in)
			if r := mustCalc(t, known); hasMark(r.Unsupported, tc.target, UnsupportedEffect) {
				t.Errorf("フラグが分かっているのに印が付いた: %+v", r.Unsupported)
			}
		})
	}

	t.Run("フラグに依存しない項目だけなら印を付けない", func(t *testing.T) {
		e := &AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, Modifier: 6144}}, DefFinalModsByType: map[Type]int{TypeFire: 8192}, NoContact: true}
		got := mustCalc(t, withDefenderAbility(withAttackerAbility(unknown, e), e))
		if len(got.Unsupported) != 0 {
			t.Errorf("フラグに依存しない項目に印が付いた: %+v", got.Unsupported)
		}
	})
	t.Run("変化技には付けない", func(t *testing.T) {
		status := unknown
		status.Move.Category = CategoryStatus
		status.Move.Power = 0
		got := mustCalc(t, withAttackerAbility(status, liquidVoice()))
		if len(got.Unsupported) != 0 {
			t.Errorf("変化技に印が付いた: %+v", got.Unsupported)
		}
	})
	t.Run("かたやぶりで無視した防御側の特性には付けない", func(t *testing.T) {
		in := withAttackerAbility(unknown, &AbilityEffect{IgnoresDefenderAbility: true})
		in = withDefenderAbility(in, &AbilityEffect{DefImmuneFlags: []MoveFlag{MoveFlagSound}, Breakable: true})
		if got := mustCalc(t, in); hasMark(got.Unsupported, UnsupportedTargetDefenderAbility, UnsupportedEffect) {
			t.Errorf("無視した特性に印が付いた: %+v", got.Unsupported)
		}
	})
}

// --- AC-E8 検証 -------------------------------------------------------------------

func TestStage2AbilityEffectValidation(t *testing.T) {
	over := MaxEffectModifier + 1
	invalid := []struct {
		name string
		e    AbilityEffect
	}{
		{"PowerMods: move_flag で Flag 空", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveFlag, Modifier: 6144}}}},
		{"PowerMods: move_flag で未知の Flag", AbilityEffect{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, "protect", 6144)}}},
		{"PowerMods: move_flag で MaxPower あり", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveFlag, Flag: MoveFlagBite, MaxPower: 60, Modifier: 6144}}}},
		{"PowerMods: move_flag で MoveType あり", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveFlag, Flag: MoveFlagBite, MoveType: TypeFire, Modifier: 6144}}}},
		{"PowerMods: max_base_power で Flag あり", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMaxBasePower, MaxPower: 60, Flag: MoveFlagBite, Modifier: 6144}}}},
		{"PowerMods: move_type で Flag あり", AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveType, MoveType: TypeSteel, Flag: MoveFlagBite, Modifier: 6144}}}},
		{"PostAuraPowerMods: 語彙に無い条件", AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{{Condition: "contact", Modifier: 5325}}}},
		{"PostAuraPowerMods: Modifier 中立", AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, Modifier4096)}}},
		{"PostAuraPowerMods: Modifier 0", AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, 0)}}},
		{"PostAuraPowerMods: Modifier 上限超え", AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, over)}}},
		{"PostAuraPowerMods: move_flag で Flag 空", AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveFlag, Modifier: 5325}}}},
		{"FlagTypeConvert: Flag 空", AbilityEffect{FlagTypeConvert: &FlagTypeConvert{To: TypeWater}}},
		{"FlagTypeConvert: 未知の Flag", AbilityEffect{FlagTypeConvert: &FlagTypeConvert{Flag: "protect", To: TypeWater}}},
		{"FlagTypeConvert: To 空", AbilityEffect{FlagTypeConvert: &FlagTypeConvert{Flag: MoveFlagSound}}},
		{"DefImmuneFlags: 未知の値", AbilityEffect{DefImmuneFlags: []MoveFlag{"protect"}}},
		{"DefImmuneFlags: 重複", AbilityEffect{DefImmuneFlags: []MoveFlag{MoveFlagSound, MoveFlagSound}}},
		{"DefFinalModsByFlag: 未知のキー", AbilityEffect{DefFinalModsByFlag: map[MoveFlag]int{"protect": ModifierHalf}}},
		{"DefFinalModsByFlag: 0", AbilityEffect{DefFinalModsByFlag: map[MoveFlag]int{MoveFlagContact: 0}}},
		{"DefFinalModsByFlag: 上限超え", AbilityEffect{DefFinalModsByFlag: map[MoveFlag]int{MoveFlagContact: over}}},
		{"DefFinalModsByType: 0", AbilityEffect{DefFinalModsByType: map[Type]int{TypeFire: 0}}},
		{"DefFinalModsByType: 上限超え", AbilityEffect{DefFinalModsByType: map[Type]int{TypeFire: over}}},
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
		{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagBite, 6144)}},
		{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, 5325)}},
		{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagSound, 5325)}, DefFinalModsByFlag: map[MoveFlag]int{MoveFlagSound: ModifierHalf}, Breakable: true},
		{FlagTypeConvert: &FlagTypeConvert{Flag: MoveFlagSound, To: TypeWater}},
		{DefImmuneFlags: []MoveFlag{MoveFlagBullet}, Breakable: true},
		{DefFinalModsByFlag: map[MoveFlag]int{MoveFlagContact: ModifierHalf}, DefFinalModsByType: map[Type]int{TypeFire: 8192}, Breakable: true},
		{NoContact: true},
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

// 威力の連鎖の位置(ADR-0178 §5): フィールド → PowerMods(move_flag)→ オーラ → PostAuraPowerMods → 持ち物。
// Champions の入力ではフィールドの補正とオーラが同じ技に同時に掛からないので、oracle のゴールデンでは位置を見分けられない。
// テスト用のオーラ(でんき)で、位置を入れ替えると結果が変わる補正値を探し、正しい位置の結果と一致することを確かめる。
func TestFlagPowerModsChainPosition(t *testing.T) {
	base := flagged(ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeElectric), MoveFlagContact)
	base.Field.Terrain = TerrainElectric
	terrain := terrainDamageMod(TerrainElectric, TypeElectric, true, true)
	plain := base
	plain.Field.Terrain = TerrainNone
	const flagMod = 5325
	cases := []struct {
		name   string
		effect *AbilityEffect
		right  func(aura int) []int
		wrong  func(aura int) []int
	}{
		{"PowerMods(move_flag)はオーラの前",
			&AbilityEffect{PowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, flagMod)}},
			func(a int) []int { return []int{terrain, flagMod, a} }, func(a int) []int { return []int{terrain, a, flagMod} }},
		{"PostAuraPowerMods はオーラの後",
			&AbilityEffect{PostAuraPowerMods: []ConditionalPowerMod{flagPowerMod(PowerConditionMoveFlag, MoveFlagContact, flagMod)}},
			func(a int) []int { return []int{terrain, a, flagMod} }, func(a int) []int { return []int{terrain, flagMod, a} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for aura := Modifier4096 + 1; aura < 2*Modifier4096; aura++ {
				r, w := chainMods(tc.right(aura), powerModBounds), chainMods(tc.wrong(aura), powerModBounds)
				if pokeRound(base.Move.Power, r) == pokeRound(base.Move.Power, w) {
					continue
				}
				want, wrong := mustCalc(t, withPowerItem(plain, r)), mustCalc(t, withPowerItem(plain, w))
				if want.Rolls == wrong.Rolls {
					continue
				}
				in := withDefenderAbility(withAttackerAbility(base, tc.effect), &AbilityEffect{AuraType: TypeElectric, AuraMod: aura})
				assertSameRolls(t, tc.name, mustCalc(t, in), want)
				return
			}
			t.Fatal("位置を入れ替えると結果が変わるオーラの補正値が見つからない(テストの前提を見直す)")
		})
	}
}
