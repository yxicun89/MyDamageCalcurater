package engine

// 技の機構の段階3(ADR-0144)の受け入れ条件。
//
// 対戦の状態の入力(BattleState: 攻撃側・防御側の残り HP と、範囲の多段技の回数)と、データで足りる残りの処理
// (なげつけるの持ち物の威力・分類の切り替え・持ち物による接地)を engine が計算し、未対応の印を外す。
// 正解は「同じ結果になるはずの通常の入力」との一致(等価な入力)と、式から手で導いた値で決める(式を再実装しない)。
// oracle との一致はゴールデン(golden_mechanisms_stage3_test.go)が見る。種族・技・特性・持ち物はすべて架空。
//
// フィクスチャの HP: mkIndiv の種族値 HP 100・SP 0 → 実数値 175(両側とも)。

import (
	"errors"
	"reflect"
	"testing"
)

// s3MaxHP は ctrlInput の両側の最大 HP(種族値 100 + 75 + SP 0)。
const s3MaxHP = 175

// s3Rule は段階3の技(機構と定義)を載せる。威力・分類・タイプは呼び出し側が決める。
func s3Rule(cat MoveCategory, typ Type, power int, mechs []MoveMechanism, r MoveRule) DamageInput {
	return withRule(s2Input(cat, typ, power), mechs, r)
}

func mustCalcS3(t *testing.T, in DamageInput) DamageResult {
	t.Helper()
	r, err := calcDamage(in)
	if err != nil {
		t.Fatalf("CalcDamage: %v", err)
	}
	return r
}

// assertFilled は16段階がすべて v で、印が無いこと(固定ダメージの式)。
func assertFilled(t *testing.T, got DamageResult, v int) {
	t.Helper()
	for i, d := range got.Rolls {
		if d != v {
			t.Fatalf("rolls[%d] = %d, want すべて %d(%v)", i, d, v, got.Rolls)
		}
	}
	if got.Unsupported != nil {
		t.Errorf("段階3で計算する技に印が付いた: %v", got.Unsupported)
	}
}

func hasMoveMark(marks []UnsupportedMark, reason UnsupportedReason) bool {
	for _, m := range marks {
		if m.Target == UnsupportedTargetMove && m.Reason == reason {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 語彙
// ---------------------------------------------------------------------------

func TestStage3Vocabulary(t *testing.T) {
	for f, want := range map[PowerFormula]string{
		PowerFormulaAttackerHPScaled:  "attacker_hp_scaled",
		PowerFormulaAttackerHPLow:     "attacker_hp_low",
		PowerFormulaDefenderHPRatio:   "defender_hp_ratio",
		PowerFormulaAttackerItemFling: "attacker_item_fling",
	} {
		if string(f) != want || !f.Known() {
			t.Errorf("PowerFormula %q: 値 %q・Known が真であること", f, want)
		}
	}
	wantFixed := []FixedDamageFormula{
		FixedDamageAttackerCurrentHP, FixedDamageDefenderHalfHP, FixedDamageDefenderMinusAttackerHP,
	}
	if got := AllFixedDamageFormulas(); !reflect.DeepEqual(got, wantFixed) {
		t.Errorf("AllFixedDamageFormulas = %v, want %v(定義順)", got, wantFixed)
	}
	wantValues := []string{"attacker_current_hp", "defender_current_hp_half", "defender_minus_attacker_hp"}
	for i, f := range wantFixed {
		if string(f) != wantValues[i] || !f.Known() {
			t.Errorf("FixedDamageFormula %q: 値 %q・Known が真であること", f, wantValues[i])
		}
	}
	for _, bad := range []string{"", "Attacker_Current_HP", "level"} {
		if FixedDamageFormula(bad).Known() {
			t.Errorf("%q を既知としてはいけない", bad)
		}
	}
	f := AllFixedDamageFormulas()
	f[0] = "x"
	if AllFixedDamageFormulas()[0] != FixedDamageAttackerCurrentHP {
		t.Error("AllFixedDamageFormulas がコピーを返していない")
	}
}

// ---------------------------------------------------------------------------
// 残り HP(BattleState の AttackerCurrentHP・DefenderCurrentHP)
// ---------------------------------------------------------------------------

// 威力 × 攻撃側の残り HP / 最大 HP(切り捨て・最小 1)。ふんか・しおふき型。省略(0)は満タン。
func TestStage3AttackerHPScaled(t *testing.T) {
	rule := MoveRule{PowerFormula: PowerFormulaAttackerHPScaled}
	vp := []MoveMechanism{MechanismVariablePower}
	cases := []struct {
		cur, wantPower int
	}{
		{0, 150},       // 省略は満タン
		{s3MaxHP, 150}, // 満タン
		{100, 85},      // floor(150 × 100 / 175) = 85
		{88, 75},       // floor(150 × 88 / 175) = 75
		{1, 1},         // floor(150 / 175) = 0 → 最小 1
	}
	for _, c := range cases {
		in := s3Rule(CategorySpecial, TypeFire, 150, vp, rule)
		in.State.AttackerCurrentHP = c.cur
		assertSameAsPlain(t, in, plain(in, TypeFire, c.wantPower))
	}
}

// 攻撃側の残り HP の 48 分率 p = floor(48 × 残り / 最大) の段: ≤1:200, ≤4:150, ≤9:100, ≤16:80, ≤32:40, else 20。
// きしかいせい・じたばた型(登録された威力は 0)。
func TestStage3AttackerHPLow(t *testing.T) {
	rule := MoveRule{PowerFormula: PowerFormulaAttackerHPLow}
	vp := []MoveMechanism{MechanismVariablePower}
	cases := []struct {
		cur, wantPower int
	}{
		{0, 20},       // 満タン p = 48
		{s3MaxHP, 20}, // p = 48
		{121, 20},     // p = floor(5808/175) = 33
		{120, 40},     // p = 32
		{62, 40},      // p = 17
		{60, 80},      // p = 16
		{29, 100},     // p = 7
		{8, 150},      // p = 2
		{6, 200},      // p = 1
		{1, 200},      // p = 0
	}
	for _, c := range cases {
		in := s3Rule(CategoryPhysical, TypeFighting, 0, vp, rule)
		in.State.AttackerCurrentHP = c.cur
		got := assertSameAsPlain(t, in, plain(in, TypeFighting, c.wantPower))
		if hasMoveMark(got.Unsupported, UnsupportedZeroPower) {
			t.Errorf("残り %d: 威力 0 の技に zero_power の印が残った", c.cur)
		}
	}
}

// 防御側の残り HP の割合で威力 100 まで(oracle・Showdown の 4096 の丸め)。ハードプレス型(登録された威力は 0)。
func TestStage3DefenderHPRatio(t *testing.T) {
	rule := MoveRule{PowerFormula: PowerFormulaDefenderHPRatio}
	vp := []MoveMechanism{MechanismVariablePower}
	cases := []struct {
		cur, wantPower int
	}{
		{0, 100},       // 満タン
		{s3MaxHP, 100}, // floor(floor((100 × 409600 + 2047) / 4096) / 100) = 100
		{88, 50},       // floor(88 × 4096 / 175) = 2059 → floor((100 × 205900 + 2047) / 4096) = 5027 → 50
		{1, 1},         // 23 → 2300 → floor(232047 / 4096) = 56 → 0 → 1
	}
	for _, c := range cases {
		in := s3Rule(CategoryPhysical, TypeSteel, 0, vp, rule)
		in.State.DefenderCurrentHP = c.cur
		assertSameAsPlain(t, in, plain(in, TypeSteel, c.wantPower))
	}
}

// 固定ダメージの式(fixed_damage の機構・登録された威力は 0)。タイプ相性の無効は先に効く。
func TestStage3FixedDamageFormulas(t *testing.T) {
	fd := []MoveMechanism{MechanismFixedDamage}
	cases := []struct {
		name           string
		formula        FixedDamageFormula
		typ            Type
		atkCur, defCur int
		want           int
		failed         bool
	}{
		{"いのちがけ型(満タン)", FixedDamageAttackerCurrentHP, TypeFighting, 0, 0, s3MaxHP, false},
		{"いのちがけ型(残り 37)", FixedDamageAttackerCurrentHP, TypeFighting, 37, 0, 37, false},
		{"いかりのまえば型(満タン)", FixedDamageDefenderHalfHP, TypeNormal, 0, 0, 87, false},
		{"いかりのまえば型(残り 51)", FixedDamageDefenderHalfHP, TypeNormal, 0, 51, 25, false},
		{"いかりのまえば型(残り 1 は最小 1)", FixedDamageDefenderHalfHP, TypeNormal, 0, 1, 1, false},
		{"がむしゃら型(防御側 175 − 攻撃側 50)", FixedDamageDefenderMinusAttackerHP, TypeNormal, 50, 0, 125, false},
		{"がむしゃら型(防御側 100 − 攻撃側 1)", FixedDamageDefenderMinusAttackerHP, TypeNormal, 1, 100, 99, false},
		{"がむしゃら型(両側満タンは失敗)", FixedDamageDefenderMinusAttackerHP, TypeNormal, 0, 0, 0, true},
		{"がむしゃら型(攻撃側の方が多いと失敗)", FixedDamageDefenderMinusAttackerHP, TypeNormal, 100, 60, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s3Rule(CategoryPhysical, c.typ, 0, fd, MoveRule{FixedDamageFormula: c.formula})
			in.State = BattleState{AttackerCurrentHP: c.atkCur, DefenderCurrentHP: c.defCur}
			got := mustCalcS3(t, in)
			if c.failed {
				if got.Nullified != NullifyMoveFailed || got.Rolls != ([16]int{}) || got.KO.Hits != 0 {
					t.Errorf("Nullified = %q・rolls = %v・KO = %+v, want move_failed・0・倒せない", got.Nullified, got.Rolls, got.KO)
				}
				if got.Unsupported != nil {
					t.Errorf("失敗は正しい結果なので印を付けない: %v", got.Unsupported)
				}
				return
			}
			assertFilled(t, got, c.want)
		})
	}
	// タイプ相性の無効(ゴーストにノーマル・かくとう)は固定ダメージより先。
	for _, f := range AllFixedDamageFormulas() {
		typ := TypeNormal
		if f == FixedDamageAttackerCurrentHP {
			typ = TypeFighting
		}
		in := s3Rule(CategoryPhysical, typ, 0, fd, MoveRule{FixedDamageFormula: f})
		in.Defender.Species.Types = []Type{TypeGhost}
		in.State.AttackerCurrentHP = 10
		got := mustCalcS3(t, in)
		if got.Effectiveness != 0 || got.Rolls != ([16]int{}) {
			t.Errorf("%s: ゴーストへの相性 = %v・rolls = %v, want 0・0", f, got.Effectiveness, got.Rolls)
		}
	}
}

// HP で決まる固定ダメージのうち、1 発ごとにダメージが変わる型(いかりのまえば型・がむしゃら型)は確定数を 1 発で数える。
// いのちがけ型は変えない(ComputeKO と同じ)。
func TestStage3FixedDamageKO(t *testing.T) {
	fd := []MoveMechanism{MechanismFixedDamage}
	one := KOChance{Hits: 1, Guaranteed: true}
	cases := []struct {
		name           string
		formula        FixedDamageFormula
		atkCur, defCur int
		want           KOChance
	}{
		{"いかりのまえば型 残り 100", FixedDamageDefenderHalfHP, 0, 100, KOChance{}},
		{"いかりのまえば型 残り 1", FixedDamageDefenderHalfHP, 0, 1, one},
		{"いかりのまえば型 残り 175", FixedDamageDefenderHalfHP, 0, 175, KOChance{}},
		{"がむしゃら型 100−1", FixedDamageDefenderMinusAttackerHP, 1, 100, KOChance{}},
		{"いのちがけ型は通常の数え方", FixedDamageAttackerCurrentHP, 0, 175, ComputeKO(filledRolls(175), 175)},
	}
	for _, c := range cases {
		in := s3Rule(CategoryPhysical, TypeNormal, 0, fd, MoveRule{FixedDamageFormula: c.formula})
		in.State = BattleState{AttackerCurrentHP: c.atkCur, DefenderCurrentHP: c.defCur}
		if c.formula == FixedDamageAttackerCurrentHP {
			in.Move.Type = TypeFighting
		}
		if got := mustCalcS3(t, in).KO; got != c.want {
			t.Errorf("%s: KO = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// 防御側の残り HP は確定数に効く(oracle の getKOChance は defender.curHP())。表示%の分母(DefenderHP)は最大 HP のまま。
func TestStage3DefenderCurrentHPDrivesKO(t *testing.T) {
	base := s2Input(CategoryPhysical, TypeNormal, 100)
	full := mustCalcS3(t, base)
	for _, cur := range []int{1, 60, 80, 120, s3MaxHP} {
		in := base
		in.State.DefenderCurrentHP = cur
		got := mustCalcS3(t, in)
		if got.Rolls != full.Rolls {
			t.Errorf("残り %d: 通常の技の rolls が変わった %v, want %v", cur, got.Rolls, full.Rolls)
		}
		if got.DefenderHP != s3MaxHP {
			t.Errorf("残り %d: DefenderHP = %d, want 最大 HP %d(表示%%の分母)", cur, got.DefenderHP, s3MaxHP)
		}
		if want := ComputeKO(full.Rolls, cur); !reflect.DeepEqual(got.KO, want) {
			t.Errorf("残り %d: KO = %+v, want %+v(残り HP で数える)", cur, got.KO, want)
		}
	}
	// 多段技も残り HP で数える。
	mh := s2Input(CategoryPhysical, TypeNormal, 25)
	mh.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
	mh.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 5}
	mh.State.DefenderCurrentHP = 30
	got := mustCalcS3(t, mh)
	if want := computeKOHits(got.Rolls, got.HitRolls, 30); !reflect.DeepEqual(got.KO, want) {
		t.Errorf("多段: KO = %+v, want %+v", got.KO, want)
	}
}

// ---------------------------------------------------------------------------
// 多段の回数(BattleState.Hits)
// ---------------------------------------------------------------------------

func TestStage3Hits(t *testing.T) {
	mk := func(hits int, skillLink bool) DamageInput {
		in := s2Input(CategoryPhysical, TypeNormal, 25)
		in.Move.ID = "test-range-multihit"
		in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		in.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 5}
		in.State.Hits = hits
		if skillLink {
			in.Attacker.Ability = Ability{ID: "test-max-hits", Effect: &AbilityEffect{MaxMultiHit: true}}
		}
		return in
	}
	single := mustCalcS3(t, plain(s2Input(CategoryPhysical, TypeNormal, 25), TypeNormal, 25))
	for _, c := range []struct {
		hits      int
		skillLink bool
		wantHits  int
	}{
		{0, false, 3}, // 省略は段階1の既定(最小 + 1)
		{0, true, 5},  // 省略 + 最大回数の特性は最大
		{2, false, 2},
		{4, false, 4},
		{5, false, 5},
		{2, true, 2}, // 指定が既定に勝つ(oracle の options.hits と同じ)
	} {
		got := mustCalcS3(t, mk(c.hits, c.skillLink))
		if len(got.HitRolls) != c.wantHits {
			t.Fatalf("hits %d(特性 %v): len(HitRolls) = %d, want %d", c.hits, c.skillLink, len(got.HitRolls), c.wantHits)
		}
		for i := range got.Rolls {
			if got.Rolls[i] != single.Rolls[i]*c.wantHits {
				t.Fatalf("hits %d: rolls[%d] = %d, want %d × %d", c.hits, i, got.Rolls[i], single.Rolls[i], c.wantHits)
			}
		}
		if got.Unsupported != nil {
			t.Errorf("hits %d: 印 %v", c.hits, got.Unsupported)
		}
	}
}

// ---------------------------------------------------------------------------
// 入力の検証(ErrInvalidBattleState)
// ---------------------------------------------------------------------------

func TestStage3BattleStateValidation(t *testing.T) {
	rangeMH := func(in DamageInput) DamageInput {
		in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		in.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 5}
		return in
	}
	fixedMH := func(in DamageInput) DamageInput {
		in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		in.Move.Params.MultiHit = &MultiHit{Min: 2, Max: 2}
		return in
	}
	cases := []struct {
		name  string
		edit  func(DamageInput) DamageInput
		state BattleState
		ok    bool
	}{
		{"攻撃側の残り = 最大", nil, BattleState{AttackerCurrentHP: s3MaxHP}, true},
		{"攻撃側の残り 1", nil, BattleState{AttackerCurrentHP: 1}, true},
		{"攻撃側の残りが最大を超える", nil, BattleState{AttackerCurrentHP: s3MaxHP + 1}, false},
		{"攻撃側の残りが負", nil, BattleState{AttackerCurrentHP: -1}, false},
		{"防御側の残り = 最大", nil, BattleState{DefenderCurrentHP: s3MaxHP}, true},
		{"防御側の残りが最大を超える", nil, BattleState{DefenderCurrentHP: s3MaxHP + 1}, false},
		{"防御側の残りが負", nil, BattleState{DefenderCurrentHP: -5}, false},
		{"範囲の多段で回数が範囲内", rangeMH, BattleState{Hits: 5}, true},
		{"範囲の多段で最小未満", rangeMH, BattleState{Hits: 1}, false},
		{"範囲の多段で最大超え", rangeMH, BattleState{Hits: 6}, false},
		{"回数が負", rangeMH, BattleState{Hits: -1}, false},
		{"固定回数の多段に回数", fixedMH, BattleState{Hits: 2}, false},
		{"多段でない技に回数", nil, BattleState{Hits: 2}, false},
		{"多段の機構があり中身が無い技に回数", func(in DamageInput) DamageInput {
			in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
			return in
		}, BattleState{Hits: 3}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s2Input(CategoryPhysical, TypeNormal, 25)
			if c.edit != nil {
				in = c.edit(in)
			}
			in.State = c.state
			_, err := calcDamage(in)
			if c.ok && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !c.ok && !errors.Is(err, ErrInvalidBattleState) {
				t.Fatalf("err = %v, want ErrInvalidBattleState", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// なげつける型(attacker_item_fling。持ち物の投げつける威力 Item.FlingPower)
// ---------------------------------------------------------------------------

func TestStage3Fling(t *testing.T) {
	rule := MoveRule{PowerFormula: PowerFormulaAttackerItemFling, MoveSpecificResolved: true}
	mechs := []MoveMechanism{MechanismMoveSpecific, MechanismVariablePower}
	mk := func(item *Item) DamageInput {
		in := s3Rule(CategoryPhysical, TypeDark, 0, mechs, rule)
		in.Attacker.Item = item
		return in
	}

	// 持ち物の威力で計算する(効果を持たない持ち物)。
	in := mk(&Item{ID: "test-heavy-ball", FlingPower: 130})
	got := assertSameAsPlain(t, in, plain(in, TypeDark, 130))
	if hasMoveMark(got.Unsupported, UnsupportedZeroPower) {
		t.Error("威力の決まった技に zero_power の印")
	}

	// 持ち物なしは失敗(正しい結果なので印なし)。
	got = mustCalcS3(t, mk(nil))
	if got.Nullified != NullifyMoveFailed || got.Rolls != ([16]int{}) || got.Unsupported != nil {
		t.Errorf("持ち物なし: Nullified = %q・rolls = %v・印 %v, want move_failed・0・なし", got.Nullified, got.Rolls, got.Unsupported)
	}

	// 攻撃側の補正を持つ持ち物を投げる型は、実機で効果が乗るか未確認なので攻撃側の持ち物の印を残す(数値は oracle どおり)。
	{
		in := mk(&Item{ID: "test-orb", FlingPower: 30, Effect: &ItemEffect{DamageMod: 5324}})
		got := mustCalcS3(t, in)
		if len(got.Unsupported) != 1 || got.Unsupported[0].Target != UnsupportedTargetAttackerItem {
			t.Errorf("補正を持つ持ち物のなげつける: 印 %v, want 攻撃側の持ち物の印", got.Unsupported)
		}
	}

	// 防御側向けの補正(防御・特防)だけの持ち物では攻撃側の印を付けない。
	if got := mustCalcS3(t, mk(&Item{ID: "test-def", FlingPower: 30, Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: 6144}}})); got.Unsupported != nil {
		t.Errorf("防御の補正だけの持ち物: 印 %v, want なし", got.Unsupported)
	}

	// 威力が不明(0)・メガストーンは印を残す(数値は従来どおり 0)。
	for name, item := range map[string]*Item{
		"威力が不明":  {ID: "test-unknown-fling"},
		"メガストーン": {ID: "test-stone", MegaStone: true, FlingPower: 80},
	} {
		got := mustCalcS3(t, mk(item))
		if !hasMoveMark(got.Unsupported, UnsupportedReason(MechanismVariablePower)) {
			t.Errorf("%s: variable_power の印が無い: %v", name, got.Unsupported)
		}
	}

	// 負の威力は個体の検証で止める。
	bad := mk(&Item{ID: "test-bad-fling", FlingPower: -1})
	if _, err := calcDamage(bad); err == nil {
		t.Error("FlingPower -1: エラーにならない")
	}
}

// ---------------------------------------------------------------------------
// 分類の切り替え(CategoryByStats。シェルアームズ型)
// ---------------------------------------------------------------------------

// ランク補正後の 攻撃 / 防御側の防御 と 特攻 / 防御側の特防 を比べ、前者が大きいときだけ物理(接触あり)。同じなら特殊。
func TestStage3CategoryByStats(t *testing.T) {
	rule := MoveRule{CategoryByStats: true, MoveSpecificResolved: true}
	ms := []MoveMechanism{MechanismMoveSpecific}
	mk := func(atkRank, defRank int) DamageInput {
		in := s3Rule(CategorySpecial, TypePoison, 90, ms, rule)
		in.Move.FlagsKnown = true
		in.Attacker.Ranks.Atk = atkRank
		in.Defender.Ranks.Def = defRank
		return in
	}
	asCategory := func(in DamageInput, cat MoveCategory, flags []MoveFlag) DamageInput {
		p := plain(in, TypePoison, 90)
		p.Move.Category = cat
		p.Move.Flags = flags
		p.Move.FlagsKnown = true
		return p
	}

	// 攻撃 200 / 防御 100 = 特攻 200 / 特防 100 → 同じは特殊。
	in := mk(0, 0)
	got := assertSameAsPlain(t, in, asCategory(in, CategorySpecial, nil))
	if got.Category != CategorySpecial {
		t.Errorf("同じ: Category = %q, want special", got.Category)
	}
	// 攻撃 +1(300)→ 物理。
	in = mk(1, 0)
	got = assertSameAsPlain(t, in, asCategory(in, CategoryPhysical, []MoveFlag{MoveFlagContact}))
	if got.Category != CategoryPhysical {
		t.Errorf("攻撃 +1: Category = %q, want physical", got.Category)
	}
	// 攻撃 +1・防御側の防御 +2(200): 300/200 < 200/100 → 特殊。
	in = mk(1, 2)
	assertSameAsPlain(t, in, asCategory(in, CategorySpecial, nil))

	// 物理になったときは接触技として特性の条件に効く。
	contactBoost := &AbilityEffect{PowerMods: []ConditionalPowerMod{{Condition: PowerConditionMoveFlag, Flag: MoveFlagContact, Modifier: 5325}}}
	in = mk(1, 0)
	in.Attacker.Ability = Ability{ID: "test-contact-boost", Effect: contactBoost}
	want := asCategory(in, CategoryPhysical, []MoveFlag{MoveFlagContact})
	assertSameAsPlain(t, in, want)
}

// ---------------------------------------------------------------------------
// 持ち物による接地(ItemEffect.Grounds。くろいてっきゅう型)
// ---------------------------------------------------------------------------

func TestStage3ItemGrounds(t *testing.T) {
	grounds := &Item{ID: "test-heavy-iron", Effect: &ItemEffect{Grounds: true}}

	// 飛行タイプの攻撃側でも、持ち物で接地してエレキフィールドの補正を受ける(飛行でない攻撃側と同じ)。
	in := s2Input(CategorySpecial, TypeElectric, 90)
	in.Field.Terrain = TerrainElectric
	in.Attacker.Species.Types = []Type{TypeFlying}
	in.Attacker.Item = grounds
	ground := in
	ground.Attacker.Species.Types = []Type{TypeNormal}
	ground.Attacker.Item = nil
	assertSameAsPlain(t, in, plain(ground, TypeElectric, 90))

	// 持ち物が無ければ浮いたまま(補正なし)。
	noItem := in
	noItem.Attacker.Item = nil
	if a, b := mustCalcS3(t, noItem), mustCalcS3(t, in); a.Rolls == b.Rolls {
		t.Error("接地の持ち物が無いのにフィールドの補正が掛かった(または持ち物で掛からない)")
	}

	// 浮く特性(Airborne)より持ち物の接地が勝つ(oracle の isGrounded)。
	lev := s2Input(CategorySpecial, TypeElectric, 90)
	lev.Field.Terrain = TerrainElectric
	lev.Attacker.Ability = Ability{ID: "test-float", Effect: &AbilityEffect{Airborne: true}}
	lev.Attacker.Item = grounds
	levGround := lev
	levGround.Attacker.Ability = Ability{}
	levGround.Attacker.Item = nil
	assertSameAsPlain(t, lev, plain(levGround, TypeElectric, 90))

	// 防御側も接地する: 飛行タイプでもサイコフィールドの先制技が当たらない。
	def := s2Input(CategoryPhysical, TypeNormal, 40)
	def.Move.Priority = 1
	def.Field.Terrain = TerrainPsychic
	def.Defender.Species.Types = []Type{TypeFlying}
	def.Defender.Item = grounds
	if got := mustCalcS3(t, def); got.Nullified != NullifyPsychicTerrain {
		t.Errorf("接地した飛行の防御側: Nullified = %q, want psychic_terrain", got.Nullified)
	}
}

// ---------------------------------------------------------------------------
// 印と検証
// ---------------------------------------------------------------------------

func TestStage3MarksRemoved(t *testing.T) {
	cases := []struct {
		name string
		in   DamageInput
	}{
		{"ふんか型", s3Rule(CategorySpecial, TypeFire, 150, []MoveMechanism{MechanismVariablePower}, MoveRule{PowerFormula: PowerFormulaAttackerHPScaled})},
		{"じたばた型", s3Rule(CategoryPhysical, TypeNormal, 0, []MoveMechanism{MechanismVariablePower}, MoveRule{PowerFormula: PowerFormulaAttackerHPLow})},
		{"ハードプレス型", s3Rule(CategoryPhysical, TypeSteel, 0, []MoveMechanism{MechanismVariablePower}, MoveRule{PowerFormula: PowerFormulaDefenderHPRatio})},
		{"いのちがけ型", s3Rule(CategorySpecial, TypeFighting, 0, []MoveMechanism{MechanismFixedDamage}, MoveRule{FixedDamageFormula: FixedDamageAttackerCurrentHP})},
		{"いかりのまえば型", s3Rule(CategoryPhysical, TypeNormal, 0, []MoveMechanism{MechanismFixedDamage}, MoveRule{FixedDamageFormula: FixedDamageDefenderHalfHP})},
		{"シェルアームズ型", s3Rule(CategorySpecial, TypePoison, 90, []MoveMechanism{MechanismMoveSpecific}, MoveRule{CategoryByStats: true, MoveSpecificResolved: true})},
	}
	for _, c := range cases {
		got := mustCalcS3(t, c.in)
		if got.Unsupported != nil {
			t.Errorf("%s: 印 %v, want なし", c.name, got.Unsupported)
		}
	}
	// 定義の無い固定ダメージの技(威力 0)は従来どおり fixed_damage と zero_power の印。
	in := s2Input(CategoryPhysical, TypeNormal, 0)
	in.Move.ID = "test-no-rule"
	in.Move.Mechanisms = []MoveMechanism{MechanismFixedDamage}
	got := mustCalcS3(t, in)
	if !hasMoveMark(got.Unsupported, UnsupportedReason(MechanismFixedDamage)) || !hasMoveMark(got.Unsupported, UnsupportedZeroPower) {
		t.Errorf("定義なし: 印 %v, want fixed_damage と zero_power", got.Unsupported)
	}
}

func TestStage3ValidateRule(t *testing.T) {
	vp := []MoveMechanism{MechanismVariablePower}
	fd := []MoveMechanism{MechanismFixedDamage}
	ms := []MoveMechanism{MechanismMoveSpecific}
	cases := []struct {
		name  string
		mechs []MoveMechanism
		power int
		rule  MoveRule
		edit  func(*Move)
	}{
		{"未知の固定ダメージの式", fd, 0, MoveRule{FixedDamageFormula: "level"}, nil},
		{"固定ダメージの式に fixed_damage が無い", vp, 0, MoveRule{FixedDamageFormula: FixedDamageAttackerCurrentHP}, nil},
		{"固定ダメージの式と中身(Params.FixedDamage)の両方", fd, 0, MoveRule{FixedDamageFormula: FixedDamageAttackerCurrentHP},
			func(m *Move) { m.Params.FixedDamage = &FixedDamage{Value: 20} }},
		{"固定ダメージの式と威力の式の両方", []MoveMechanism{MechanismFixedDamage, MechanismVariablePower}, 0,
			MoveRule{FixedDamageFormula: FixedDamageDefenderHalfHP, PowerFormula: PowerFormulaAttackerHPLow}, nil},
		{"分類の切り替えに move_specific が無い", vp, 90, MoveRule{CategoryByStats: true}, nil},
		{"威力 × 残り HP の式で威力 0", vp, 0, MoveRule{PowerFormula: PowerFormulaAttackerHPScaled}, nil},
		{"なげつける型に威力の機構が無い", []MoveMechanism{MechanismTypeChange}, 0, MoveRule{PowerFormula: PowerFormulaAttackerItemFling}, nil},
	}
	_ = ms
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := s3Rule(CategoryPhysical, TypeNormal, c.power, c.mechs, c.rule)
			if c.edit != nil {
				c.edit(&in.Move)
			}
			if _, err := calcDamage(in); !errors.Is(err, ErrInvalidMoveRule) {
				t.Fatalf("err = %v, want ErrInvalidMoveRule", err)
			}
		})
	}
	// 変化技には分類の切り替えも固定ダメージの式も書けない。
	st := s3Rule(CategoryStatus, TypeNormal, 0, ms, MoveRule{CategoryByStats: true, MoveSpecificResolved: true})
	if _, err := calcDamage(st); !errors.Is(err, ErrInvalidMoveRule) {
		t.Errorf("変化技の定義: err = %v, want ErrInvalidMoveRule", err)
	}
}

// 状態の入力は省略(ゼロ値)で従来と同じ結果(既存のゴールデン・テストが変わらないことの単体の確認)。
func TestStage3ZeroStateIsUnchanged(t *testing.T) {
	in := s2Input(CategoryPhysical, TypeNormal, 100)
	explicit := in
	explicit.State = BattleState{AttackerCurrentHP: s3MaxHP, DefenderCurrentHP: s3MaxHP}
	a, b := mustCalcS3(t, in), mustCalcS3(t, explicit)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("省略 = %+v, 満タンの明示 = %+v(同じであること)", a, b)
	}
}
