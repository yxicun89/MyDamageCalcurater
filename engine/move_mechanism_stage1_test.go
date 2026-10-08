package engine

// 技の機構の段階1(ADR-0142)の受け入れ条件。
//
// 取得元のフィールドで中身が決まる機構(多段・固定ダメージ・一撃必殺・必ず急所・防御ランク無視・
// 攻撃/防御に使う能力値)を engine が正しく計算し、未対応の印(ADR-0123)を外す。
// 正解は「同じ結果になるはずの通常の入力」との一致(等価な入力)と、式から手で導いた値で決める
// (式を再実装しない)。oracle との一致はゴールデン(golden_mechanisms_test.go)が見る。
// 種族・技・特性はすべて架空(coding-rules §1)。

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
)

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

// s1Input は統制ケース(ctrlInput: 実数値 A200 / B100・威力 100 のノーマル技・防御側 HP 175)。
func s1Input(cat MoveCategory) DamageInput {
	return ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, cat, TypeNormal)
}

func mustCalcS1(t *testing.T, in DamageInput) DamageResult {
	t.Helper()
	r, err := calcDamage(in)
	if err != nil {
		t.Fatalf("CalcDamage: %v", err)
	}
	return r
}

func withMultiHit(in DamageInput, min, max int) DamageInput {
	in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
	in.Move.Params.MultiHit = &MultiHit{Min: min, Max: max}
	return in
}

// expectedMultiHitKO は「1回の使用 = hits 個の独立な16段階」の確定数(ADR-0142 §3)。全発が per と同じ場合。
func expectedMultiHitKO(per [16]int, hp, hits int) KOChance {
	sumMax, sumMin := per[15]*hits, per[0]*hits
	if sumMax <= 0 || hp <= 0 {
		return KOChance{}
	}
	n := (hp + sumMax - 1) / sumMax
	if sumMin*n >= hp {
		return KOChance{Hits: n, Guaranteed: true}
	}
	return KOChance{Hits: n, ChancePercent: koProbability(per, hp, n*hits) * 100}
}

func assertKO(t *testing.T, got, want KOChance) {
	t.Helper()
	if got.Hits != want.Hits || got.Guaranteed != want.Guaranteed || math.Abs(got.ChancePercent-want.ChancePercent) > 1e-9 {
		t.Errorf("KO = %+v, want %+v", got, want)
	}
}

// assertMultiHit は HitRolls が hits 個の per で、Rolls が段ごとの合計であること。
func assertMultiHit(t *testing.T, got DamageResult, per [16]int, hits int) {
	t.Helper()
	if len(got.HitRolls) != hits {
		t.Fatalf("len(HitRolls) = %d, want %d", len(got.HitRolls), hits)
	}
	for h, r := range got.HitRolls {
		if r != per {
			t.Errorf("HitRolls[%d] = %v, want %v(1発は単発の計算と同じ)", h, r, per)
		}
	}
	for i := range 16 {
		if got.Rolls[i] != per[i]*hits {
			t.Errorf("Rolls[%d] = %d, want %d(同じ段の合計)", i, got.Rolls[i], per[i]*hits)
		}
	}
}

// ---------------------------------------------------------------------------
// 多段(multi_hit)
// ---------------------------------------------------------------------------

func TestStage1MultiHitFixedCount(t *testing.T) {
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		t.Run(string(cat), func(t *testing.T) {
			single := s1Input(cat)
			want := mustCalcS1(t, single)
			if want.HitRolls != nil {
				t.Fatalf("単発の技の HitRolls は nil(既存の結果の形を変えない): %v", want.HitRolls)
			}
			got := mustCalcS1(t, withMultiHit(single, 2, 2))
			assertMultiHit(t, got, want.Rolls, 2)
			assertKO(t, got.KO, expectedMultiHitKO(want.Rolls, got.DefenderHP, 2))
			if got.Unsupported != nil {
				t.Errorf("中身のある多段に印が付いた: %v", got.Unsupported)
			}
		})
	}
}

// 範囲の回数は oracle と同じ: 既定は 最小+1、攻撃側の MaxMultiHit(スキルリンク)は最大。固定回数は特性で変わらない。
func TestStage1MultiHitCountRule(t *testing.T) {
	maxHit := Ability{ID: "test-maxhit", Effect: &AbilityEffect{MaxMultiHit: true}}
	cases := []struct {
		name     string
		min, max int
		edit     func(*DamageInput)
		hits     int
	}{
		{"範囲 [2,5] の既定は 3", 2, 5, func(*DamageInput) {}, 3},
		{"範囲 [2,5] × 攻撃側 MaxMultiHit は 5", 2, 5, func(in *DamageInput) { in.Attacker.Ability = maxHit }, 5},
		{"範囲 [2,5] × 防御側 MaxMultiHit は既定の 3", 2, 5, func(in *DamageInput) { in.Defender.Ability = maxHit }, 3},
		{"固定 3 × 攻撃側 MaxMultiHit は 3", 3, 3, func(in *DamageInput) { in.Attacker.Ability = maxHit }, 3},
		{"固定 10", 10, 10, func(*DamageInput) {}, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			single := s1Input(CategoryPhysical)
			c.edit(&single)
			want := mustCalcS1(t, single)
			got := mustCalcS1(t, withMultiHit(single, c.min, c.max))
			assertMultiHit(t, got, want.Rolls, c.hits)
			assertKO(t, got.KO, expectedMultiHitKO(want.Rolls, got.DefenderHP, c.hits))
		})
	}
}

// 確定数は合計で数える: 1発では確定3発でも、2回当たる技は確定2回(ADR-0142 §3)。
func TestStage1MultiHitKOUsesTotal(t *testing.T) {
	in := s1Input(CategoryPhysical)
	in.Move.Power = 40
	single := mustCalcS1(t, in)
	got := mustCalcS1(t, withMultiHit(in, 2, 2))
	want := expectedMultiHitKO(single.Rolls, single.DefenderHP, 2)
	assertKO(t, got.KO, want)
	if got.KO.Hits >= single.KO.Hits && single.KO.Hits > 1 {
		t.Errorf("多段の確定数 %d が単発の %d より少なくない(合計で数えていない)", got.KO.Hits, single.KO.Hits)
	}
}

// 無効のときはダメージ 0・HitRolls なし・倒せない。
func TestStage1MultiHitImmune(t *testing.T) {
	in := withMultiHit(ctrlInput([]Type{TypeWater}, []Type{TypeGhost}, CategoryPhysical, TypeNormal), 2, 5)
	got := mustCalcS1(t, in)
	if got.Rolls != ([16]int{}) || got.HitRolls != nil || got.KO.Hits != 0 {
		t.Errorf("無効: Rolls=%v HitRolls=%v KO=%+v, want 0・nil・0", got.Rolls, got.HitRolls, got.KO)
	}
}

// ---------------------------------------------------------------------------
// 固定ダメージ(fixed_damage)
// ---------------------------------------------------------------------------

func fixedDamageInput(cat MoveCategory, moveType Type, defTypes []Type, fd FixedDamage) DamageInput {
	in := ctrlInput([]Type{TypeNormal}, defTypes, cat, moveType)
	in.Move.Power = 0
	in.Move.Mechanisms = []MoveMechanism{MechanismFixedDamage}
	in.Move.Params.FixedDamage = &fd
	return in
}

func allRolls(v int) [16]int {
	var r [16]int
	for i := range r {
		r[i] = v
	}
	return r
}

func TestStage1FixedDamage(t *testing.T) {
	boost := &Item{ID: "test-boost", Effect: &ItemEffect{DamageMod: 5324}}
	cases := []struct {
		name string
		fd   FixedDamage
		edit func(*DamageInput)
		want int
	}{
		{"レベル(Lv50)", FixedDamage{Level: true}, func(*DamageInput) {}, DefaultLevel},
		{"数値", FixedDamage{Value: 40}, func(*DamageInput) {}, 40},
		{"急所・やけど・壁・持ち物・ランク・抜群・一致でも変わらない", FixedDamage{Level: true}, func(in *DamageInput) {
			in.Critical = true
			in.Attacker.Status = StatusBurn
			in.Attacker.Item = boost
			in.Attacker.Ranks.Atk = 6
			in.Defender.Ranks.Def = -6
			in.Field.DefenderScreens.Reflect = true
		}, DefaultLevel},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// かくとう技 → ノーマルの防御側は抜群、攻撃側はノーマルなので一致しない。一致の確認は別に足す。
			in := fixedDamageInput(CategoryPhysical, TypeFighting, []Type{TypeNormal}, c.fd)
			c.edit(&in)
			got := mustCalcS1(t, in)
			if got.Rolls != allRolls(c.want) {
				t.Errorf("Rolls = %v, want 全段 %d", got.Rolls, c.want)
			}
			if got.HitRolls != nil {
				t.Errorf("固定ダメージの HitRolls = %v, want nil", got.HitRolls)
			}
			assertKO(t, got.KO, ComputeKO(allRolls(c.want), got.DefenderHP))
			if got.Unsupported != nil {
				t.Errorf("中身のある固定ダメージに印(zero_power を含む)が付いた: %v", got.Unsupported)
			}
		})
	}

	t.Run("タイプ一致でも変わらない", func(t *testing.T) {
		in := fixedDamageInput(CategorySpecial, TypeNormal, []Type{TypePsychic}, FixedDamage{Level: true})
		if got := mustCalcS1(t, in); got.Rolls != allRolls(DefaultLevel) {
			t.Errorf("Rolls = %v, want 全段 %d", got.Rolls, DefaultLevel)
		}
	})
	t.Run("タイプ相性の無効は 0", func(t *testing.T) {
		in := fixedDamageInput(CategoryPhysical, TypeFighting, []Type{TypeGhost}, FixedDamage{Level: true})
		got := mustCalcS1(t, in)
		if got.Rolls != ([16]int{}) || got.Effectiveness != 0 || got.KO.Hits != 0 {
			t.Errorf("Rolls=%v eff=%v KO=%+v, want 0・0・倒せない", got.Rolls, got.Effectiveness, got.KO)
		}
	})
	t.Run("特性による無効は 0・Nullified=immune", func(t *testing.T) {
		in := fixedDamageInput(CategoryPhysical, TypeFighting, []Type{TypeNormal}, FixedDamage{Level: true})
		in.Defender.Ability = Ability{ID: "test-immune", Effect: &AbilityEffect{DefImmuneTypes: []Type{TypeFighting}}}
		got := mustCalcS1(t, in)
		if got.Rolls != ([16]int{}) || got.Nullified != NullifyImmune {
			t.Errorf("Rolls=%v Nullified=%q, want 0・immune", got.Rolls, got.Nullified)
		}
	})
	t.Run("中身が無い固定ダメージは従来どおり 0 と印", func(t *testing.T) {
		in := fixedDamageInput(CategoryPhysical, TypeFighting, []Type{TypeNormal}, FixedDamage{Level: true})
		in.Move.Params.FixedDamage = nil
		got := mustCalcS1(t, in)
		want := []UnsupportedMark{moveMark("m", UnsupportedReason(MechanismFixedDamage)), moveMark("m", UnsupportedZeroPower)}
		if got.MaxDamage() != 0 || !reflect.DeepEqual(got.Unsupported, want) {
			t.Errorf("max=%d Unsupported=%v, want 0 %v", got.MaxDamage(), got.Unsupported, want)
		}
	})
}

// ---------------------------------------------------------------------------
// 一撃必殺(ohko)
// ---------------------------------------------------------------------------

func ohkoInput(moveType Type, defTypes []Type, immune Type) DamageInput {
	in := ctrlInput([]Type{TypeWater}, defTypes, CategoryPhysical, moveType)
	in.Move.Power = 0
	in.Move.Mechanisms = []MoveMechanism{MechanismOHKO}
	in.Move.Params.OHKO = &OHKO{ImmuneType: immune}
	return in
}

func TestStage1OHKO(t *testing.T) {
	sturdy := Ability{ID: "test-sturdy", Effect: &AbilityEffect{PreventsOHKO: true, Breakable: true}}
	unbreakable := Ability{ID: "test-sturdy2", Effect: &AbilityEffect{PreventsOHKO: true}}
	breaker := Ability{ID: "test-breaker", Effect: &AbilityEffect{IgnoresDefenderAbility: true}}
	cases := []struct {
		name     string
		in       func() DamageInput
		lands    bool
		nullify  NullifyKind
		wantZero bool
	}{
		{"当たれば倒す", func() DamageInput { return ohkoInput(TypeGround, []Type{TypePsychic}, TypeNone) }, true, NullifyNone, false},
		{"タイプ相性の無効(じめん → ひこう)", func() DamageInput { return ohkoInput(TypeGround, []Type{TypeFlying}, TypeNone) }, false, NullifyNone, true},
		{"ImmuneType を持つ相手には効かない", func() DamageInput { return ohkoInput(TypeIce, []Type{TypeIce, TypePsychic}, TypeIce) }, false, NullifyOHKOImmune, true},
		{"ImmuneType を持たない相手には効く", func() DamageInput { return ohkoInput(TypeIce, []Type{TypePsychic}, TypeIce) }, true, NullifyNone, false},
		{"PreventsOHKO の防御側には効かない", func() DamageInput {
			in := ohkoInput(TypeGround, []Type{TypePsychic}, TypeNone)
			in.Defender.Ability = sturdy
			return in
		}, false, NullifyOHKOImmune, true},
		{"かたやぶりは Breakable な PreventsOHKO を無視する", func() DamageInput {
			in := ohkoInput(TypeGround, []Type{TypePsychic}, TypeNone)
			in.Attacker.Ability, in.Defender.Ability = breaker, sturdy
			return in
		}, true, NullifyNone, false},
		{"かたやぶりでも Breakable でない PreventsOHKO は効く", func() DamageInput {
			in := ohkoInput(TypeGround, []Type{TypePsychic}, TypeNone)
			in.Attacker.Ability, in.Defender.Ability = breaker, unbreakable
			return in
		}, false, NullifyOHKOImmune, true},
		{"攻撃側が PreventsOHKO を持っても効く", func() DamageInput {
			in := ohkoInput(TypeGround, []Type{TypePsychic}, TypeNone)
			in.Attacker.Ability = sturdy
			return in
		}, true, NullifyNone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mustCalcS1(t, c.in())
			if c.lands {
				if got.Rolls != allRolls(got.DefenderHP) {
					t.Errorf("Rolls = %v, want 全段 = 防御側の最大 HP %d", got.Rolls, got.DefenderHP)
				}
				assertKO(t, got.KO, KOChance{Hits: 1, Guaranteed: true})
			}
			if c.wantZero && (got.Rolls != ([16]int{}) || got.KO.Hits != 0) {
				t.Errorf("Rolls=%v KO=%+v, want 0・倒せない", got.Rolls, got.KO)
			}
			if got.Nullified != c.nullify {
				t.Errorf("Nullified = %q, want %q", got.Nullified, c.nullify)
			}
			if got.Unsupported != nil {
				t.Errorf("中身のある一撃必殺に印(zero_power を含む)が付いた: %v", got.Unsupported)
			}
		})
	}

	t.Run("中身が無い一撃必殺は従来どおり印", func(t *testing.T) {
		in := ohkoInput(TypeGround, []Type{TypePsychic}, TypeNone)
		in.Move.Params.OHKO = nil
		got := mustCalcS1(t, in)
		want := []UnsupportedMark{moveMark("m", UnsupportedReason(MechanismOHKO)), moveMark("m", UnsupportedZeroPower)}
		if got.MaxDamage() != 0 || !reflect.DeepEqual(got.Unsupported, want) {
			t.Errorf("max=%d Unsupported=%v, want 0 %v", got.MaxDamage(), got.Unsupported, want)
		}
	})
}

// ---------------------------------------------------------------------------
// 必ず急所(always_crit)・防御ランク無視(ignore_defense_ranks)
// ---------------------------------------------------------------------------

func TestStage1AlwaysCrit(t *testing.T) {
	shell := Ability{ID: "test-shell", Effect: &AbilityEffect{PreventsCritical: true, Breakable: true}}
	breaker := Ability{ID: "test-breaker", Effect: &AbilityEffect{IgnoresDefenderAbility: true}}
	sniper := Ability{ID: "test-sniper", Effect: &AbilityEffect{CritDamageMod: 6144}}
	cases := []struct {
		name     string
		edit     func(*DamageInput)
		wantCrit bool
	}{
		{"急所の指定なしでも急所", func(*DamageInput) {}, true},
		{"急所のときに無視されるランク(攻撃側の負・防御側の正)も急所どおり", func(in *DamageInput) {
			in.Attacker.Ranks.Atk, in.Defender.Ranks.Def = -2, 2
		}, true},
		{"急所に当たらない防御側には急所にならない", func(in *DamageInput) { in.Defender.Ability = shell }, false},
		{"かたやぶりは急所に当たらない特性を無視する", func(in *DamageInput) {
			in.Attacker.Ability, in.Defender.Ability = breaker, shell
		}, true},
		{"急所の威力を上げる特性が効く", func(in *DamageInput) { in.Attacker.Ability = sniper }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain := s1Input(CategoryPhysical)
			c.edit(&plain)
			ref := plain
			ref.Critical = c.wantCrit
			want := mustCalcS1(t, ref)

			in := plain
			in.Move.Mechanisms = []MoveMechanism{MechanismAlwaysCrit}
			got := mustCalcS1(t, in)
			if got.Rolls != want.Rolls || got.KO != want.KO {
				t.Errorf("Rolls = %v, want %v(Critical=%v と同じ)", got.Rolls, want.Rolls, c.wantCrit)
			}
			if got.Unsupported != nil {
				t.Errorf("必ず急所に印が付いた: %v", got.Unsupported)
			}
		})
	}
}

func TestStage1IgnoreDefenseRanks(t *testing.T) {
	cases := []struct {
		name  string
		cat   MoveCategory
		ranks Ranks
	}{
		{"物理 × 防御 +6", CategoryPhysical, Ranks{Def: 6}},
		{"物理 × 防御 -6(負のランクも無視)", CategoryPhysical, Ranks{Def: -6}},
		{"特殊 × 特防 +3", CategorySpecial, Ranks{SpD: 3}},
		{"特殊 × 特防 -2", CategorySpecial, Ranks{SpD: -2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := mustCalcS1(t, s1Input(c.cat))
			in := s1Input(c.cat)
			in.Defender.Ranks = c.ranks
			in.Move.Mechanisms = []MoveMechanism{MechanismIgnoreDefenseRanks}
			got := mustCalcS1(t, in)
			if got.Rolls != want.Rolls {
				t.Errorf("Rolls = %v, want %v(ランク 0 と同じ)", got.Rolls, want.Rolls)
			}
			if got.Unsupported != nil {
				t.Errorf("防御ランク無視に印が付いた: %v", got.Unsupported)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 攻撃に使う能力値(alt_offense_stat)
// ---------------------------------------------------------------------------

// 攻撃側の防御で攻撃する技は、「防御の値を攻撃に持つ個体」の通常の物理技と同じ。
// 攻撃側の実数値の補正(atk の StatMods)とやけどは技の分類(物理)で掛かる(oracle。ADR-0142 §5)。
func TestStage1AltOffenseStatOwnDefense(t *testing.T) {
	huge := Ability{ID: "test-huge", Effect: &AbilityEffect{StatMods: map[StatKey]int{StatAtk: 8192}}}
	furCoatLike := Ability{ID: "test-fur", Effect: &AbilityEffect{StatMods: map[StatKey]int{StatDef: 8192}}}
	unaware := Ability{ID: "test-unaware", Effect: &AbilityEffect{IgnoresOpponentRanks: true}}
	cases := []struct {
		name string
		edit func(atk *Individual, in *DamageInput)
	}{
		{"基本", func(*Individual, *DamageInput) {}},
		{"攻撃の実数値の補正は掛かる", func(a *Individual, _ *DamageInput) { a.Ability = huge }},
		{"防御の実数値の補正は攻撃に掛からない", func(a *Individual, _ *DamageInput) { a.Ability = furCoatLike }},
		{"やけど", func(a *Individual, _ *DamageInput) { a.Status = StatusBurn }},
		{"急所は負のランクを無視", func(a *Individual, in *DamageInput) { a.Ranks.Def = -2; in.Critical = true }},
		{"防御側のてんねんはランクを無視", func(a *Individual, in *DamageInput) { a.Ranks.Def = 4; in.Defender.Ability = unaware }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			atk := mkIndiv([]Type{TypeFighting}, Stats{Atk: 60, Def: 150})
			atk.SP = Stats{Atk: 4, Def: 20}
			atk.Ranks = Ranks{Atk: -3, Def: 2}
			in := DamageInput{
				Format:   FormatSingle,
				Defender: mkIndiv([]Type{TypePsychic}, Stats{Def: 80, SpD: 80}),
				Move:     Move{ID: "test-press", Type: TypeFighting, Category: CategoryPhysical, Power: 80},
			}
			c.edit(&atk, &in)

			bp := in
			bp.Attacker = atk
			bp.Move.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
			bp.Move.Params.OffenseStat = StatDef

			eq := in
			eq.Attacker = atk
			eq.Attacker.Species.BaseStats.Atk = atk.Species.BaseStats.Def
			eq.Attacker.SP.Atk = atk.SP.Def
			eq.Attacker.Ranks.Atk = atk.Ranks.Def

			got, want := mustCalcS1(t, bp), mustCalcS1(t, eq)
			if got.Rolls != want.Rolls {
				t.Errorf("Rolls = %v, want %v(防御を攻撃に持つ個体の通常の技)", got.Rolls, want.Rolls)
			}
			if got.Unsupported != nil {
				t.Errorf("中身のある alt_offense_stat に印が付いた: %v", got.Unsupported)
			}
		})
	}
}

// 防御側の攻撃で攻撃する技は、「防御側の攻撃の種族値・SP・性格・ランクを持つ攻撃側」の通常の物理技と同じ。
// 攻撃側自身の攻撃のランクは使わない。攻撃側の実数値の補正は攻撃側の特性で掛かる(oracle)。
func TestStage1AltOffenseFromDefender(t *testing.T) {
	huge := Ability{ID: "test-huge", Effect: &AbilityEffect{StatMods: map[StatKey]int{StatAtk: 8192}}}
	unaware := Ability{ID: "test-unaware", Effect: &AbilityEffect{IgnoresOpponentRanks: true}}
	cases := []struct {
		name string
		edit func(atk, def *Individual, in *DamageInput)
	}{
		{"基本", func(*Individual, *Individual, *DamageInput) {}},
		{"攻撃側の特性の補正は掛かる", func(a, _ *Individual, _ *DamageInput) { a.Ability = huge }},
		{"急所は防御側の負のランクを無視", func(_, d *Individual, in *DamageInput) { d.Ranks.Atk = -2; in.Critical = true }},
		{"防御側のてんねんは防御側自身の攻撃のランクも無視(oracle)", func(_, d *Individual, _ *DamageInput) {
			d.Ranks.Atk = 3
			d.Ability = unaware
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			atk := mkIndiv([]Type{TypeDark}, Stats{Atk: 60})
			atk.Ranks.Atk = 6
			def := mkIndiv([]Type{TypePsychic}, Stats{Atk: 130, Def: 80, SpD: 80})
			def.SP = Stats{Atk: 32}
			def.Nature = Nature{Plus: StatAtk, Minus: StatSpA}
			def.Ranks.Atk = 1
			in := DamageInput{Format: FormatSingle, Move: Move{ID: "test-foul", Type: TypeDark, Category: CategoryPhysical, Power: 95}}
			c.edit(&atk, &def, &in)

			fp := in
			fp.Attacker, fp.Defender = atk, def
			fp.Move.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
			fp.Move.Params.OffensePokemon = OffensePokemonDefender

			eq := in
			eq.Attacker, eq.Defender = atk, def
			eq.Attacker.Species.BaseStats.Atk = def.Species.BaseStats.Atk
			eq.Attacker.SP.Atk = def.SP.Atk
			eq.Attacker.Nature = def.Nature
			eq.Attacker.Ranks.Atk = def.Ranks.Atk

			got, want := mustCalcS1(t, fp), mustCalcS1(t, eq)
			if got.Rolls != want.Rolls {
				t.Errorf("Rolls = %v, want %v(防御側の攻撃を持つ攻撃側の通常の技)", got.Rolls, want.Rolls)
			}
			if got.Unsupported != nil {
				t.Errorf("中身のある alt_offense_stat に印が付いた: %v", got.Unsupported)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 防御に使う能力値(alt_defense_stat)
// ---------------------------------------------------------------------------

func psyshockInput(defTypes []Type) DamageInput {
	in := s1Input(CategorySpecial)
	in.Defender = mkIndiv(defTypes, Stats{Def: 150, SpD: 60})
	in.Defender.SP = Stats{Def: 20}
	in.Defender.Ranks = Ranks{Def: 1, SpD: -4}
	in.Move.Power = 80
	in.Move.Mechanisms = []MoveMechanism{MechanismAltDefenseStat}
	in.Move.Params.DefenseStat = StatDef
	return in
}

// 特殊技で防御を参照する技は、「防御の値を特防に持つ防御側」への通常の特殊技と同じ(壁は分類どおり ひかりのかべ)。
func TestStage1AltDefenseStatEquivalence(t *testing.T) {
	for _, screens := range []Screens{{}, {LightScreen: true}} {
		ps := psyshockInput([]Type{TypePsychic})
		ps.Field.DefenderScreens = screens
		eq := ps
		eq.Move.Mechanisms, eq.Move.Params = nil, MechanismParams{}
		eq.Defender.Species.BaseStats.SpD = ps.Defender.Species.BaseStats.Def
		eq.Defender.SP.SpD = ps.Defender.SP.Def
		eq.Defender.Ranks.SpD = ps.Defender.Ranks.Def
		got, want := mustCalcS1(t, ps), mustCalcS1(t, eq)
		if got.Rolls != want.Rolls {
			t.Errorf("壁 %+v: Rolls = %v, want %v", screens, got.Rolls, want.Rolls)
		}
		if got.Unsupported != nil {
			t.Errorf("中身のある alt_defense_stat に印が付いた: %v", got.Unsupported)
		}
	}
}

func TestStage1AltDefenseStatModifiers(t *testing.T) {
	base := func(defTypes []Type) DamageResult { return mustCalcS1(t, psyshockInput(defTypes)) }
	t.Run("リフレクターは掛からない(壁は分類で決まる)", func(t *testing.T) {
		in := psyshockInput([]Type{TypePsychic})
		in.Field.DefenderScreens.Reflect = true
		if got, want := mustCalcS1(t, in), base([]Type{TypePsychic}); got.Rolls != want.Rolls {
			t.Errorf("Rolls = %v, want %v", got.Rolls, want.Rolls)
		}
	})
	t.Run("すなあらしの岩の特防の補正は掛からない", func(t *testing.T) {
		in := psyshockInput([]Type{TypeRock})
		in.Field.Weather = WeatherSand
		if got, want := mustCalcS1(t, in), base([]Type{TypeRock}); got.Rolls != want.Rolls {
			t.Errorf("Rolls = %v, want %v", got.Rolls, want.Rolls)
		}
	})
	t.Run("ゆきの氷の防御の補正は掛かる", func(t *testing.T) {
		in := psyshockInput([]Type{TypeIce})
		in.Field.Weather = WeatherSnow
		if got, plain := mustCalcS1(t, in), base([]Type{TypeIce}); got.MaxDamage() >= plain.MaxDamage() {
			t.Errorf("max = %d, want < %d(防御が上がる)", got.MaxDamage(), plain.MaxDamage())
		}
	})
	t.Run("防御側の特防の持ち物の補正は掛からない", func(t *testing.T) {
		in := psyshockInput([]Type{TypePsychic})
		in.Defender.Item = &Item{ID: "test-vest", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
		if got, want := mustCalcS1(t, in), base([]Type{TypePsychic}); got.Rolls != want.Rolls {
			t.Errorf("Rolls = %v, want %v", got.Rolls, want.Rolls)
		}
	})
	t.Run("防御側の防御の持ち物の補正は掛かる", func(t *testing.T) {
		in := psyshockInput([]Type{TypePsychic})
		in.Defender.Item = &Item{ID: "test-defitem", Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: 6144}}}
		if got, plain := mustCalcS1(t, in), base([]Type{TypePsychic}); got.MaxDamage() >= plain.MaxDamage() {
			t.Errorf("max = %d, want < %d", got.MaxDamage(), plain.MaxDamage())
		}
	})
}

// ---------------------------------------------------------------------------
// 印(ADR-0142 §10)と中身の検証(§2)
// ---------------------------------------------------------------------------

// 段階1の機構の印だけが外れ、段階1外の機構の印は残る。
func TestStage1MarksOnlyStage1Removed(t *testing.T) {
	in := withMultiHit(s1Input(CategoryPhysical), 3, 3)
	in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit, MechanismVariablePower}
	got := mustCalcS1(t, in)
	if w := []UnsupportedMark{moveMark("m", UnsupportedReason(MechanismVariablePower))}; !reflect.DeepEqual(got.Unsupported, w) {
		t.Errorf("Unsupported = %v, want %v", got.Unsupported, w)
	}
	if len(got.HitRolls) != 3 {
		t.Errorf("段階1の分(回数)は計算する: len(HitRolls) = %d, want 3", len(got.HitRolls))
	}

	// 機構だけで決まる always_crit・ignore_defense_ranks は、どの入力でも印を付けない。
	in = s1Input(CategorySpecial)
	in.Defender.Ranks.SpD = 2
	in.Move.Mechanisms = []MoveMechanism{MechanismAlwaysCrit, MechanismIgnoreDefenseRanks}
	if got := mustCalcS1(t, in); got.Unsupported != nil {
		t.Errorf("Unsupported = %v, want nil", got.Unsupported)
	}
}

func TestStage1InvalidParams(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(*DamageInput)
		wantErr error
	}{
		{"機構の無い MultiHit", func(in *DamageInput) { in.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 2} }, ErrInvalidMechanismParams},
		{"MultiHit の Min が 0", func(in *DamageInput) { *in = withMultiHit(*in, 0, 2) }, ErrInvalidMechanismParams},
		{"MultiHit の範囲が逆", func(in *DamageInput) { *in = withMultiHit(*in, 5, 2) }, ErrInvalidMechanismParams},
		{"MultiHit の Max が 1", func(in *DamageInput) { *in = withMultiHit(*in, 1, 1) }, ErrInvalidMechanismParams},
		{"MultiHit の Max が上限超過", func(in *DamageInput) { *in = withMultiHit(*in, 2, MaxMultiHits+1) }, ErrInvalidMechanismParams},
		{"機構の無い FixedDamage", func(in *DamageInput) { in.Move.Params.FixedDamage = &FixedDamage{Level: true} }, ErrInvalidMechanismParams},
		{"FixedDamage が Level と Value の両方", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismFixedDamage}
			in.Move.Params.FixedDamage = &FixedDamage{Level: true, Value: 40}
		}, ErrInvalidMechanismParams},
		{"FixedDamage がどちらも無い", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismFixedDamage}
			in.Move.Params.FixedDamage = &FixedDamage{}
		}, ErrInvalidMechanismParams},
		{"FixedDamage の Value が負", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismFixedDamage}
			in.Move.Params.FixedDamage = &FixedDamage{Value: -1}
		}, ErrInvalidMechanismParams},
		{"機構の無い OHKO", func(in *DamageInput) { in.Move.Params.OHKO = &OHKO{} }, ErrInvalidMechanismParams},
		{"OHKO の ImmuneType が相性表に無い", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismOHKO}
			in.Move.Params.OHKO = &OHKO{ImmuneType: "test-unknown-type"}
		}, ErrUnknownType},
		{"機構の無い OffenseStat", func(in *DamageInput) { in.Move.Params.OffenseStat = StatDef }, ErrInvalidMechanismParams},
		{"OffenseStat が HP", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
			in.Move.Params.OffenseStat = StatHP
		}, ErrInvalidMechanismParams},
		{"OffensePokemon が未知", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
			in.Move.Params.OffensePokemon = "test-ally"
		}, ErrInvalidMechanismParams},
		{"機構の無い DefenseStat", func(in *DamageInput) { in.Move.Params.DefenseStat = StatDef }, ErrInvalidMechanismParams},
		{"DefenseStat が未知", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismAltDefenseStat}
			in.Move.Params.DefenseStat = "luck"
		}, ErrInvalidMechanismParams},
		{"変化技の中身", func(in *DamageInput) {
			in.Move.Category, in.Move.Power = CategoryStatus, 0
			*in = withMultiHit(*in, 2, 2)
		}, ErrInvalidMechanismParams},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s1Input(CategoryPhysical)
			c.edit(&in)
			if _, err := calcDamage(in); !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 一括計算・逆算・調整(ADR-0142 §6)
// ---------------------------------------------------------------------------

func TestStage1BulkMultiHitMatchesCalc(t *testing.T) {
	in := withMultiHit(s1Input(CategoryPhysical), 2, 5)
	bulk, err := calcBulk(BulkInput{
		Format: FormatSingle, Attacker: in.Attacker, DefenderSpecies: in.Defender.Species, Move: in.Move,
		PresetKeys: []PresetKey{PresetNone}, ItemVariants: []*Item{nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bulk.Rows) == 0 {
		t.Fatal("行が無い")
	}
	for _, row := range bulk.Rows {
		single := in
		single.Defender = row.Defender
		want := mustCalcS1(t, single)
		if !reflect.DeepEqual(row.Result.HitRolls, want.HitRolls) || row.Result.Rolls != want.Rolls {
			t.Errorf("行 %s: HitRolls/Rolls が CalcDamage と違う: %v / %v", row.Preset, row.Result.HitRolls, want.HitRolls)
		}
		assertKO(t, row.Result.KO, want.KO)
	}
}

// 多段の逆算は、観測を「1回の使用の合計」として取り得る合計の集合と照合する(段の合計だけと照合しない)。
func TestStage1ReverseMultiHitMatchesMixedTotals(t *testing.T) {
	const trueSP = 10
	in := s1Input(CategoryPhysical)
	unknown := Individual{Species: in.Defender.Species, Level: DefaultLevel, Nature: NatureNeutral,
		SP: Stats{HP: MaxSPPerStat, Def: trueSP}, Status: StatusNone}
	probe := in
	probe.Defender = unknown
	per := mustCalcS1(t, probe).Rolls

	// 2発の合計で、同じ段の合計(2×per[k])にならない値を選ぶ。
	obs := -1
	for i := 0; i < 16 && obs < 0; i++ {
		for j := i + 1; j < 16; j++ {
			s := per[i] + per[j]
			if !slices.ContainsFunc(per[:], func(r int) bool { return 2*r == s }) {
				obs = s
				break
			}
		}
	}
	if obs < 0 {
		t.Fatalf("fixture: 段の合計で表せない2発の合計が無い(per=%v)。威力・種族値を変える", per)
	}

	rev, err := calcReverse(ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: in.Attacker, UnknownSpecies: in.Defender.Species,
		Move: withMultiHit(in, 2, 2).Move, Observations: []Observation{{Damage: obs}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rev.Candidates {
		if c.NatureClass != NatureClassNeutral || c.ItemID != "" {
			continue
		}
		if !c.Exact || !slices.ContainsFunc(c.Ranges, func(r SPRange) bool { return r.Min <= trueSP && trueSP <= r.Max }) {
			t.Errorf("無補正の候補: Exact=%v Ranges=%v, want Exact で SP %d を含む(観測 %d = 2発の合計)", c.Exact, c.Ranges, trueSP, obs)
		}
		return
	}
	t.Fatal("無補正・持ち物なしの候補が無い")
}

// 逆算で探索する能力は、技が攻撃・防御に使う能力値(ADR-0142 §6)。
func TestStage1ReverseSearchesUsedStat(t *testing.T) {
	in := s1Input(CategoryPhysical)
	press := in.Move
	press.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
	press.Params.OffenseStat = StatDef
	rev, err := calcReverse(ReverseInput{
		Format: FormatSingle, Side: SideAttacker, Known: in.Defender, UnknownSpecies: in.Attacker.Species,
		Move: press, Observations: []Observation{{Percent: 30}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rev.Stat != StatDef {
		t.Errorf("攻撃側の防御で攻撃する技の逆算: Stat = %q, want def", rev.Stat)
	}

	sp := s1Input(CategorySpecial)
	shock := sp.Move
	shock.Mechanisms = []MoveMechanism{MechanismAltDefenseStat}
	shock.Params.DefenseStat = StatDef
	rev, err = calcReverse(ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: sp.Attacker, UnknownSpecies: sp.Defender.Species,
		Move: shock, Observations: []Observation{{Percent: 30}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rev.Stat != StatDef {
		t.Errorf("特殊技で防御を参照する技の逆算: Stat = %q, want def", rev.Stat)
	}
}

// 防御側の攻撃で攻撃する技を防御側から逆算すると、防御側の攻撃を探索しないので印を残す(ADR-0142 §6)。
func TestStage1ReverseFromDefenderKeepsMarkForOpponentOffense(t *testing.T) {
	in := s1Input(CategoryPhysical)
	foul := in.Move
	foul.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
	foul.Params.OffensePokemon = OffensePokemonDefender
	rev, err := calcReverse(ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: in.Attacker, UnknownSpecies: in.Defender.Species,
		Move: foul, Observations: []Observation{{Percent: 30}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := moveMark("m", UnsupportedReason(MechanismAltOffenseStat))
	for _, c := range rev.Candidates {
		if !slices.Contains(c.Unsupported, want) {
			t.Errorf("候補 (%s, %q) の印 = %v, want %v を含む", c.NatureClass, c.ItemID, c.Unsupported, want)
		}
	}
}

// 調整: 2回当たる技で1回の使用で倒す/耐える = 同じ1発の単発の技で2発。
func TestStage1AdjustMultiHitEquivalence(t *testing.T) {
	for _, thr := range []float64{100, 50} {
		single := koSearchInput(t, CategoryPhysical, 60, 2, thr, Stats{}, NatureNeutral)
		multi := koSearchInput(t, CategoryPhysical, 60, 1, thr, Stats{}, NatureNeutral)
		multi.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		multi.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 2}
		want, err := MinSPToKO(single)
		if err != nil {
			t.Fatal(err)
		}
		got, err := MinSPToKO(multi)
		if err != nil {
			t.Fatal(err)
		}
		if got.Feasible != want.Feasible || got.SP != want.SP || got.Stat != want.Stat || math.Abs(got.ChancePercent-want.ChancePercent) > 1e-9 {
			t.Errorf("MinSPToKO(しきい値 %v) = %+v, want %+v", thr, got, want)
		}

		ss := surviveSearchInput(t, CategoryPhysical, 60, 2, thr, Stats{}, NatureNeutral)
		ms := surviveSearchInput(t, CategoryPhysical, 60, 1, thr, Stats{}, NatureNeutral)
		ms.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		ms.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 2}
		sw, err := MinSPToSurvive(ss)
		if err != nil {
			t.Fatal(err)
		}
		sg, err := MinSPToSurvive(ms)
		if err != nil {
			t.Fatal(err)
		}
		if sg.Feasible != sw.Feasible || sg.HPSP != sw.HPSP || sg.StatSP != sw.StatSP || math.Abs(sg.ChancePercent-sw.ChancePercent) > 1e-9 {
			t.Errorf("MinSPToSurvive(しきい値 %v) = %+v, want %+v", thr, sg, sw)
		}
	}
}

// 10回当たる技 × 目標 10 発でも、組数の桁あふれで壊れない(確率は 0..100)。
func TestStage1AdjustManyHitsDoesNotOverflow(t *testing.T) {
	for _, thr := range []float64{100, 50} {
		in := koSearchInput(t, CategoryPhysical, 10, MaxAdjustHits, thr, Stats{}, NatureNeutral)
		in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		in.Move.Params.MultiHit = &MultiHit{Min: MaxMultiHits, Max: MaxMultiHits}
		got, err := MinSPToKO(in)
		if err != nil {
			t.Fatal(err)
		}
		if got.ChancePercent < 0 || got.ChancePercent > 100 || math.IsNaN(got.ChancePercent) {
			t.Errorf("ChancePercent = %v, want 0..100", got.ChancePercent)
		}

		goal := SPGoal{Kind: SPGoalKO, Opponent: in.Defender, Move: in.Move, Hits: MaxAdjustHits, ThresholdPercent: thr}
		self := Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, Status: StatusNone}
		res, err := SuggestSPForGoals(goalsInput(t, self, statsAll(MaxSPPerStat), goal))
		if err != nil {
			t.Fatal(err)
		}
		if c := res.Goals[0].ChancePercent; c < 0 || c > 100 || math.IsNaN(c) {
			t.Errorf("目標の ChancePercent = %v, want 0..100", c)
		}
	}
}

func TestStage1GoalsMultiHitEquivalence(t *testing.T) {
	foe := Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{HP: 32}, Status: StatusNone}
	self := Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, Status: StatusNone}
	for _, thr := range []float64{100, 50} {
		single := SPGoal{Kind: SPGoalKO, Opponent: foe, Move: adjMove(CategoryPhysical, 60), Hits: 2, ThresholdPercent: thr}
		multi := single
		multi.Hits = 1
		multi.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		multi.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 2}
		want, err := SuggestSPForGoals(goalsInput(t, self, statsAll(MaxSPPerStat), single))
		if err != nil {
			t.Fatal(err)
		}
		got, err := SuggestSPForGoals(goalsInput(t, self, statsAll(MaxSPPerStat), multi))
		if err != nil {
			t.Fatal(err)
		}
		if got.Feasible != want.Feasible || got.Plan != want.Plan || got.Goals[0].Met != want.Goals[0].Met ||
			math.Abs(got.Goals[0].ChancePercent-want.Goals[0].ChancePercent) > 1e-9 || got.Unsupported != nil {
			t.Errorf("しきい値 %v: got %+v, want %+v", thr, got, want)
		}
	}
}

// 調整の探索する能力は技が使う能力値: 防御で攻撃する技の MinSPToKO は def、防御を参照する特殊技の MinSPToSurvive は def。
func TestStage1AdjustSearchesUsedStat(t *testing.T) {
	ko := koSearchInput(t, CategoryPhysical, 80, 2, 100, Stats{}, NatureNeutral)
	ko.Move.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
	ko.Move.Params.OffenseStat = StatDef
	got, err := MinSPToKO(ko)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stat != StatDef {
		t.Errorf("MinSPToKO.Stat = %q, want def", got.Stat)
	}

	sv := surviveSearchInput(t, CategorySpecial, 80, 2, 100, Stats{}, NatureNeutral)
	sv.Move.Mechanisms = []MoveMechanism{MechanismAltDefenseStat}
	sv.Move.Params.DefenseStat = StatDef
	sg, err := MinSPToSurvive(sv)
	if err != nil {
		t.Fatal(err)
	}
	if sg.Stat != StatDef {
		t.Errorf("MinSPToSurvive.Stat = %q, want def", sg.Stat)
	}
}

// 目標の探索・配分は参照する能力値が違う技を段階2に回すので、印を残す(ADR-0142 §6)。
func TestStage1GoalsKeepMarkForAltStats(t *testing.T) {
	foe := Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{HP: 32}, Status: StatusNone}
	self := Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, Status: StatusNone}
	move := adjMove(CategoryPhysical, 80)
	move.Mechanisms = []MoveMechanism{MechanismAltOffenseStat}
	move.Params.OffenseStat = StatDef
	res, err := SuggestSPForGoals(goalsInput(t, self, statsAll(MaxSPPerStat),
		SPGoal{Kind: SPGoalKO, Opponent: foe, Move: move, Hits: 2, ThresholdPercent: 100}))
	if err != nil {
		t.Fatal(err)
	}
	want := moveMark(move.ID, UnsupportedReason(MechanismAltOffenseStat))
	if !slices.Contains(res.Unsupported, want) {
		t.Errorf("Unsupported = %v, want %v を含む", res.Unsupported, want)
	}
}
