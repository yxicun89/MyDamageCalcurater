package engine

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// 連続技 CalcCombo(ADR-0179 追記「CB-3」。docs/plan/combo-calc.md §3・§5)。
//
// 受け入れ条件(この節の ID は combo-calc.md §5 の AC と同じ):
//
//	AC-1  UC-3 じゃれつく → かげうち: 手ごとの 16 段階・合計 169〜201・倒せる確率 132/256(51.5625%、表示 51.6%)
//	AC-2  相手をわんぱく H32 B32 S2 にすると合計 126〜148・倒せる確率 0
//	AC-3  りゅうのまい → げきりん: 1 手目はダメージ nil・終了時ランク 攻撃+1 素早さ+1、2 手目 258〜306 で確定
//	AC-4  素早さ 132 → 198。from_defender(169)で opponent_first → self_first
//	AC-5  unknown のときライン「197 以下は自分が先・198 同速・199 以上は相手が先」と depends_on_opponent_speed。トリックルームで逆
//	AC-6  相手の技 じしん: 1 回ぶん 103〜123(HP 132)、行動回数 1、耐える 100%
//	AC-7  相手の技 げきりん: 1 回ぶん 216〜254(HP 168)、行動回数 1、耐える 0%
//	AC-8  優先度が違えば素早さ・トリックルームを見ない。同じ優先度・同じ素早さは speed_tie
//	AC-9  ニトロチャージ → メイン技: 1 手目のダメージと行動順は素早さ +0、2 手目の行動順は +1
//	AC-10 ランクは -6..+6 に収める
//	AC-11 自分の防御が下がった後の相手の攻撃は、下がった防御で計算する
//	AC-12 確率 100% 未満の追加効果は常に適用しない(§7 Q4 (b)。印も出さない)
//	AC-13 各手のダメージは、その手の開始時のランクを入れた CalcDamage と一致する(新しい式を持たない)
//	AC-14 倒せる確率・耐える確率は、各手の 16 段階(多段は 1 発ごと)を独立とした総当たりと一致する
//	AC-15 こだわり系(ItemEffect.LocksMove)で手ごとに技が違うと choice_locked の印。計算は返す
//	AC-16 入力の検査(engine 側): 手 0 件・上限超え・ランク変化の無い変化技・変化技の相手の技・未知の相手の素早さ → ErrInvalidComboInput
//	§3-4・§7 Q6 2 手目以降で満タン前提になる防御側の持ち物・特性(FullHPOnly)・残り HP で威力が変わる技に defender_full_hp_assumed
//	§7 Q5 相手への確率 100% のランク変化(がんせきふうじ)は相手のランクに適用し、次の手の行動順に効く

// --- fixture(種族値は公開値。特性・持ち物なし。combo-calc.md §1 の前提) -------------------------------

var (
	comboMimikyu   = Species{Key: "mimikyu", Types: []Type{TypeGhost, TypeFairy}, BaseStats: Stats{HP: 55, Atk: 90, Def: 80, SpA: 50, SpD: 105, Spe: 96}}
	comboGarchomp  = Species{Key: "garchomp", Types: []Type{TypeDragon, TypeGround}, BaseStats: Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102}}
	comboDragonite = Species{Key: "dragonite", Types: []Type{TypeDragon, TypeFlying}, BaseStats: Stats{HP: 91, Atk: 134, Def: 95, SpA: 100, SpD: 100, Spe: 80}}

	comboAdamant = Nature{Plus: StatAtk, Minus: StatSpA}
	comboJolly   = Nature{Plus: StatSpe, Minus: StatSpA}
	comboImpish  = Nature{Plus: StatDef, Minus: StatSpA}

	// H2 A32 S32 と H32 B32 S2。
	comboSPAttack = Stats{HP: 2, Atk: 32, Spe: 32}
	comboSPBulk   = Stats{HP: 32, Def: 32, Spe: 2}

	comboPlayRough   = Move{ID: "playrough", Type: TypeFairy, Category: CategoryPhysical, Power: 90}
	comboShadowSneak = Move{ID: "shadowsneak", Type: TypeGhost, Category: CategoryPhysical, Power: 40, Priority: 1}
	comboEarthquake  = Move{ID: "earthquake", Type: TypeGround, Category: CategoryPhysical, Power: 100}
	comboOutrage     = Move{ID: "outrage", Type: TypeDragon, Category: CategoryPhysical, Power: 120}
	comboDragonClaw  = Move{ID: "dragonclaw", Type: TypeDragon, Category: CategoryPhysical, Power: 80}
	comboDragonDance = Move{ID: "dragondance", Type: TypeDragon, Category: CategoryStatus,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 1, StatSpe: 1}}}
	comboSwordsDance = Move{ID: "swordsdance", Type: TypeNormal, Category: CategoryStatus,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 2}}}
	comboFlameCharge = Move{ID: "flamecharge", Type: TypeFire, Category: CategoryPhysical, Power: 50,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: 1}}}
	comboCloseCombat = Move{ID: "closecombat", Type: TypeFighting, Category: CategoryPhysical, Power: 120,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatDef: -1, StatSpD: -1}}}
	comboHammerArm = Move{ID: "hammerarm", Type: TypeFighting, Category: CategoryPhysical, Power: 100,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetSelf, Stages: map[StatKey]int{StatSpe: -1}}}
	comboMetalClaw = Move{ID: "metalclaw", Type: TypeSteel, Category: CategoryPhysical, Power: 50,
		Effect: &MoveEffect{Chance: 10, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 1}}}
	comboRockTomb = Move{ID: "rocktomb", Type: TypeRock, Category: CategoryPhysical, Power: 60,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetOpponent, Stages: map[StatKey]int{StatSpe: -1}}}
	comboFireLash = Move{ID: "firelash", Type: TypeFire, Category: CategoryPhysical, Power: 80,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetOpponent, Stages: map[StatKey]int{StatDef: -1}}}
	comboLunge = Move{ID: "lunge", Type: TypeBug, Category: CategoryPhysical, Power: 80,
		Effect: &MoveEffect{Chance: 100, Target: RankTargetOpponent, Stages: map[StatKey]int{StatAtk: -1}}}
	comboBulletSeed = Move{ID: "bulletseed", Type: TypeGrass, Category: CategoryPhysical, Power: 25,
		Mechanisms: []MoveMechanism{MechanismMultiHit}, Params: MechanismParams{MultiHit: &MultiHit{Min: 2, Max: 5}}}
	comboHardPress = Move{ID: "hardpress", Type: TypeSteel, Category: CategoryPhysical,
		Mechanisms: []MoveMechanism{MechanismVariablePower}, Rule: &MoveRule{PowerFormula: PowerFormulaDefenderHPRatio}}
	// comboProtectLike はランク変化を持たない変化技(手に置けない。AC-16)。
	comboProtectLike = Move{ID: "protect", Type: TypeNormal, Category: CategoryStatus, Priority: 4}
	// comboChanceOnlyStatus は確率 100% 未満のランク変化だけを持つ変化技(確率 100% のランク変化が無いので手に置けない)。
	comboChanceOnlyStatus = Move{ID: "test-chance-status", Type: TypeNormal, Category: CategoryStatus,
		Effect: &MoveEffect{Chance: 50, Target: RankTargetSelf, Stages: map[StatKey]int{StatAtk: 1}}}
	// comboExtremeSpeedLike は優先度 +2 の攻撃技。
	comboExtremeSpeedLike = Move{ID: "extremespeed", Type: TypeNormal, Category: CategoryPhysical, Power: 80, Priority: 2}
)

func comboInd(s Species, n Nature, sp Stats, r Ranks) Individual {
	return Individual{Species: s, Nature: n, SP: sp, Ranks: r, Status: StatusNone}
}

// comboMimikyuAdamant はいじっぱり H2 A32 S32 のミミッキュ(素早さ 148・HP 132)。
func comboMimikyuAdamant() Individual {
	return comboInd(comboMimikyu, comboAdamant, comboSPAttack, Ranks{})
}

// comboGarchompJolly はようき H2 A32 S32 のガブリアス(素早さ 169・HP 185)。
func comboGarchompJolly() Individual {
	return comboInd(comboGarchomp, comboJolly, comboSPAttack, Ranks{})
}

// comboGarchompImpish はわんぱく H32 B32 S2 のガブリアス(HP 215)。
func comboGarchompImpish() Individual {
	return comboInd(comboGarchomp, comboImpish, comboSPBulk, Ranks{})
}

// comboDragoniteAdamant はいじっぱり H2 A32 S32 のカイリュー(素早さ 132・HP 168)。
func comboDragoniteAdamant() Individual {
	return comboInd(comboDragonite, comboAdamant, comboSPAttack, Ranks{})
}

func comboSteps(moves ...Move) []ComboStep {
	steps := make([]ComboStep, len(moves))
	for i, m := range moves {
		steps[i] = ComboStep{Move: m}
	}
	return steps
}

func comboInput(att, def Individual, moves ...Move) ComboInput {
	return ComboInput{
		Format:    FormatSingle,
		Attacker:  att,
		Defender:  def,
		TypeChart: typeChartForTests(),
		Steps:     comboSteps(moves...),
	}
}

func mustCombo(t *testing.T, in ComboInput) ComboResult {
	t.Helper()
	res, err := CalcCombo(in)
	if err != nil {
		t.Fatalf("CalcCombo: %v", err)
	}
	if len(res.Steps) != len(in.Steps) {
		t.Fatalf("手の数 = %d, want %d", len(res.Steps), len(in.Steps))
	}
	return res
}

func ptrMove(m Move) *Move { return &m }

func comboRolls(v ...int) [16]int {
	var r [16]int
	copy(r[:], v)
	return r
}

// stepDraws はその手の独立な 16 段階の並び(多段は 1 発ごと。変化技は空)。
func stepDraws(d *DamageResult) [][16]int {
	if d == nil {
		return nil
	}
	if len(d.HitRolls) >= 2 {
		return d.HitRolls
	}
	return [][16]int{d.Rolls}
}

// bruteForceReach は draws の 16^len 通りを素直に数え、合計が hp 以上になる通り数と全体の通り数を返す。
// 畳み込みと独立な実装(AC-14)。
func bruteForceReach(draws [][16]int, hp int) (reach, total int) {
	idx := make([]int, len(draws))
	for {
		sum := 0
		for i, r := range draws {
			sum += r[idx[i]]
		}
		total++
		if sum >= hp {
			reach++
		}
		k := 0
		for k < len(idx) {
			idx[k]++
			if idx[k] < 16 {
				break
			}
			idx[k] = 0
			k++
		}
		if k == len(idx) {
			return reach, total
		}
	}
}

func approxEqual(a, b float64) bool {
	d := a - b
	return d < 1e-9 && d > -1e-9
}

// --- AC-1・AC-2 -------------------------------------------------------------------------------------------

func TestCalcComboUC3PlayRoughShadowSneak(t *testing.T) {
	res := mustCombo(t, comboInput(comboMimikyuAdamant(), comboGarchompJolly(), comboPlayRough, comboShadowSneak))
	want := [][16]int{
		comboRolls(138, 140, 140, 144, 144, 146, 150, 150, 152, 152, 156, 156, 158, 158, 162, 164),
		comboRolls(31, 31, 31, 33, 33, 33, 33, 34, 34, 34, 34, 36, 36, 36, 36, 37),
	}
	for i, w := range want {
		if res.Steps[i].Damage == nil || res.Steps[i].Damage.Rolls != w {
			t.Errorf("手 %d の 16 段階 = %+v, want %v", i, res.Steps[i].Damage, w)
		}
		if res.Steps[i].MoveID != []string{"playrough", "shadowsneak"}[i] {
			t.Errorf("手 %d の MoveID = %q", i, res.Steps[i].MoveID)
		}
	}
	tot := res.Total
	if tot.MinDamage != 169 || tot.MaxDamage != 201 || tot.DefenderHP != 185 {
		t.Errorf("合計 = %d〜%d / HP %d, want 169〜201 / 185", tot.MinDamage, tot.MaxDamage, tot.DefenderHP)
	}
	if tot.KO.ChancePercent != 51.5625 || tot.KO.Guaranteed {
		t.Errorf("倒せる確率 = %+v, want 51.5625(132/256)・確定でない", tot.KO)
	}
	if got := tot.KO.DisplayChancePercentTenths(); got != 516 {
		t.Errorf("表示 = %d, want 516(51.6%%)", got)
	}
	if len(tot.KOByStep) != 2 || tot.KOByStep[0].ChancePercent != 0 || tot.KOByStep[1] != tot.KO {
		t.Errorf("手ごとの累積 = %+v, want [0%%, 全手と同じ]", tot.KOByStep)
	}
	if lo, hi := tot.DisplayPercentRangeTenths(); lo != 913 || hi != 1086 {
		t.Errorf("合計の表示%% = %d〜%d, want 913〜1086(最小は切り捨て・最大は四捨五入)", lo, hi)
	}
	if res.Unsupported != nil {
		t.Errorf("印 = %+v, want nil", res.Unsupported)
	}
	if res.OpponentAttack != nil {
		t.Errorf("相手の技なしで OpponentAttack = %+v, want nil", res.OpponentAttack)
	}
}

func TestCalcComboUC3BulkyDefenderCannotKO(t *testing.T) {
	res := mustCombo(t, comboInput(comboMimikyuAdamant(), comboGarchompImpish(), comboPlayRough, comboShadowSneak))
	for i, w := range [][2]int{{102, 120}, {24, 28}} {
		d := res.Steps[i].Damage
		if d == nil || d.MinDamage() != w[0] || d.MaxDamage() != w[1] {
			t.Errorf("手 %d = %+v, want %d〜%d", i, d, w[0], w[1])
		}
	}
	tot := res.Total
	if tot.MinDamage != 126 || tot.MaxDamage != 148 || tot.DefenderHP != 215 {
		t.Errorf("合計 = %d〜%d / HP %d, want 126〜148 / 215", tot.MinDamage, tot.MaxDamage, tot.DefenderHP)
	}
	if tot.KO != (ComboKOChance{}) || tot.KO.DisplayChancePercentTenths() != 0 {
		t.Errorf("倒せる確率 = %+v, want 0(倒せない)", tot.KO)
	}
}

// --- AC-3・AC-4・AC-7 ----------------------------------------------------------------------------------------

func TestCalcComboDragonDanceOutrage(t *testing.T) {
	res := mustCombo(t, comboInput(comboDragoniteAdamant(), comboGarchompImpish(), comboDragonDance, comboOutrage))
	s0, s1 := res.Steps[0], res.Steps[1]
	if s0.Damage != nil {
		t.Errorf("変化技の手のダメージ = %+v, want nil", s0.Damage)
	}
	if s0.SelfStartRanks != (Ranks{}) || s0.SelfEndRanks != (Ranks{Atk: 1, Spe: 1}) {
		t.Errorf("1 手目のランク = %+v → %+v, want {} → 攻撃+1 素早さ+1", s0.SelfStartRanks, s0.SelfEndRanks)
	}
	if s1.SelfStartRanks != (Ranks{Atk: 1, Spe: 1}) {
		t.Errorf("2 手目の開始ランク = %+v, want 1 手目の終了と同じ", s1.SelfStartRanks)
	}
	if s1.Damage == nil || s1.Damage.MinDamage() != 258 || s1.Damage.MaxDamage() != 306 {
		t.Errorf("2 手目 = %+v, want 258〜306", s1.Damage)
	}
	if !res.Total.KO.Guaranteed || res.Total.KO.ChancePercent != 100 || res.Total.KO.DisplayChancePercentTenths() != 1000 {
		t.Errorf("倒せる確率 = %+v, want 確定(100%%)", res.Total.KO)
	}
	if res.Total.KOByStep[0] != (ComboKOChance{}) {
		t.Errorf("1 手目までで倒せる確率 = %+v, want 0", res.Total.KOByStep[0])
	}

	// 同じ相手に げきりん 1 手だけなら 170〜204(確定 2 発)。
	one := mustCombo(t, comboInput(comboDragoniteAdamant(), comboGarchompImpish(), comboOutrage))
	d := one.Steps[0].Damage
	if d == nil || d.MinDamage() != 170 || d.MaxDamage() != 204 || d.KO != (KOChance{Hits: 2, Guaranteed: true}) {
		t.Errorf("げきりん 1 手 = %+v, want 170〜204・確定 2 発", d)
	}
	if one.Total.KO != (ComboKOChance{}) {
		t.Errorf("1 手で倒せる確率 = %+v, want 0", one.Total.KO)
	}
}

func TestCalcComboSpeedFromDefender(t *testing.T) {
	in := comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboDragonDance, comboOutrage)
	in.OpponentSpeed = OpponentSpeedFromDefender
	res := mustCombo(t, in)
	for i, w := range []struct {
		self, opp int
		order     TurnOrder
	}{{132, 169, TurnOrderOpponentFirst}, {198, 169, TurnOrderSelfFirst}} {
		s := res.Steps[i]
		if s.SelfSpeed != w.self || s.OpponentSpeed != w.opp || s.Order != w.order {
			t.Errorf("手 %d: 素早さ %d vs %d・%s, want %d vs %d・%s", i, s.SelfSpeed, s.OpponentSpeed, s.Order, w.self, w.opp, w.order)
		}
		if s.SelfPriority != 0 || s.OpponentPriority != 0 {
			t.Errorf("手 %d: 優先度 %d vs %d, want 0 vs 0(相手の技なしは優先度 0)", i, s.SelfPriority, s.OpponentPriority)
		}
	}
	sp := res.Speed
	if sp.FirstSelfSpeed != 132 || sp.LastSelfSpeed != 198 || !sp.OpponentSpeedKnown || sp.OpponentSpeed != 169 {
		t.Errorf("Speed = %+v, want 132 → 198・相手 169", sp)
	}
}

func TestCalcComboSpeedLineUnknown(t *testing.T) {
	for _, mode := range []OpponentSpeedMode{OpponentSpeedUnknown, ""} {
		in := comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboDragonDance, comboOutrage)
		in.OpponentSpeed = mode
		res := mustCombo(t, in)
		s := res.Steps[1]
		want := SpeedLine{SelfSpeed: 198, SelfFirst: SpeedRange{Min: 0, Max: 197}, OpponentFirst: SpeedRange{Min: 199, Max: maxSpeed}}
		if s.Line != want {
			t.Errorf("mode %q: ライン = %+v, want %+v", mode, s.Line, want)
		}
		if s.Order != TurnOrderDependsOnOpponentSpeed || res.Steps[0].Order != TurnOrderDependsOnOpponentSpeed {
			t.Errorf("mode %q: 行動順 = %s / %s, want depends_on_opponent_speed", mode, res.Steps[0].Order, s.Order)
		}
		if s.OpponentSpeed != 0 || res.Speed.OpponentSpeedKnown || res.Speed.OpponentSpeed != 0 {
			t.Errorf("mode %q: 相手の素早さを知らないのに値がある: %d / %+v", mode, s.OpponentSpeed, res.Speed)
		}
	}

	// トリックルーム中は向きが逆。
	in := comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboDragonDance, comboOutrage)
	in.SpeedField.TrickRoom = true
	res := mustCombo(t, in)
	want := SpeedLine{SelfSpeed: 198, SelfFirst: SpeedRange{Min: 199, Max: maxSpeed}, OpponentFirst: SpeedRange{Min: 0, Max: 197}}
	if res.Steps[1].Line != want || res.Steps[1].Order != TurnOrderDependsOnOpponentSpeed {
		t.Errorf("トリックルーム: ライン = %+v・%s, want %+v・depends_on_opponent_speed", res.Steps[1].Line, res.Steps[1].Order, want)
	}

	// 追い風(自分側)は手の素早さに掛かる(ADR-0179 §2)。
	in = comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboOutrage)
	in.SpeedField.SelfTailwind = true
	if got := mustCombo(t, in).Steps[0].SelfSpeed; got != 264 {
		t.Errorf("追い風の素早さ = %d, want 264(132 × 2)", got)
	}
	// 追い風(相手側)は from_defender の相手の素早さに掛かる。
	in = comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboOutrage)
	in.OpponentSpeed = OpponentSpeedFromDefender
	in.SpeedField.OpponentTailwind = true
	if got := mustCombo(t, in).Steps[0].OpponentSpeed; got != 338 {
		t.Errorf("相手側の追い風の素早さ = %d, want 338(169 × 2)", got)
	}
}

// --- AC-6・AC-7・AC-11(相手の技) ----------------------------------------------------------------------

func TestCalcComboOpponentAttackSurvives(t *testing.T) {
	in := comboInput(comboMimikyuAdamant(), comboGarchompJolly(), comboPlayRough, comboShadowSneak)
	in.OpponentMove = ptrMove(comboEarthquake)
	in.OpponentSpeed = OpponentSpeedFromDefender
	res := mustCombo(t, in)
	if res.Steps[0].Order != TurnOrderOpponentFirst || res.Steps[1].Order != TurnOrderSelfFirst {
		t.Fatalf("行動順 = %s / %s, want opponent_first / self_first(かげうちの優先度 +1)", res.Steps[0].Order, res.Steps[1].Order)
	}
	oa := res.OpponentAttack
	if oa == nil {
		t.Fatal("OpponentAttack = nil")
	}
	if oa.SelfHP != 132 || oa.MinActions != 1 || oa.MaxActions != 1 {
		t.Errorf("SelfHP %d・回数 %d..%d, want 132・1..1", oa.SelfHP, oa.MinActions, oa.MaxActions)
	}
	for name, sc := range map[string]OpponentScenario{"Fast": oa.Fast, "Slow": oa.Slow} {
		if len(sc.Actions) != 1 {
			t.Fatalf("%s: 行動 %d 回, want 1", name, len(sc.Actions))
		}
		a := sc.Actions[0]
		if a.Step != 0 || a.Timing != OpponentTimingBeforeSelf || a.Damage.MinDamage() != 103 || a.Damage.MaxDamage() != 123 {
			t.Errorf("%s: 行動 = 手 %d・%s・%d〜%d, want 手 0・before_self・103〜123", name, a.Step, a.Timing, a.Damage.MinDamage(), a.Damage.MaxDamage())
		}
		if a.SurviveChancePercent != 100 || sc.SurviveChancePercent != 100 {
			t.Errorf("%s: 耐える確率 = %v / %v, want 100", name, a.SurviveChancePercent, sc.SurviveChancePercent)
		}
	}

	// 相手の素早さを知らないとき: 1 手目は順番が決まらない(Fast は相手が先・Slow は自分が先)が、どちらも最後の手より前なので 1 回。
	in.OpponentSpeed = OpponentSpeedUnknown
	oa = mustCombo(t, in).OpponentAttack
	if oa.MinActions != 1 || oa.MaxActions != 1 {
		t.Fatalf("unknown: 回数 %d..%d, want 1..1", oa.MinActions, oa.MaxActions)
	}
	if oa.Fast.Actions[0].Timing != OpponentTimingBeforeSelf || oa.Slow.Actions[0].Timing != OpponentTimingAfterSelf {
		t.Errorf("unknown: Fast %s・Slow %s, want before_self・after_self", oa.Fast.Actions[0].Timing, oa.Slow.Actions[0].Timing)
	}
}

func TestCalcComboOpponentAttackFaints(t *testing.T) {
	in := comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboDragonDance, comboOutrage)
	in.OpponentMove = ptrMove(comboOutrage)
	in.OpponentSpeed = OpponentSpeedFromDefender
	oa := mustCombo(t, in).OpponentAttack
	if oa == nil || oa.SelfHP != 168 || oa.MinActions != 1 || oa.MaxActions != 1 {
		t.Fatalf("OpponentAttack = %+v, want SelfHP 168・回数 1", oa)
	}
	a := oa.Fast.Actions[0]
	if a.Damage.MinDamage() != 216 || a.Damage.MaxDamage() != 254 || a.Timing != OpponentTimingBeforeSelf {
		t.Errorf("1 回ぶん = %d〜%d・%s, want 216〜254・before_self", a.Damage.MinDamage(), a.Damage.MaxDamage(), a.Timing)
	}
	if oa.Fast.SurviveChancePercent != 0 || a.SurviveChancePercent != 0 {
		t.Errorf("耐える確率 = %v, want 0", oa.Fast.SurviveChancePercent)
	}
}

// 最後の手を相手が先に動くと、最後の手の前にも相手が行動する(行動回数 = 手の数)。順番が決まらないと最小と最大が分かれる。
func TestCalcComboOpponentActionCount(t *testing.T) {
	// ガブリアス(169)の げきりん × 2 を、相手(素早さが分からない)の げきりんに対して。
	in := comboInput(comboGarchompJolly(), comboDragoniteAdamant(), comboOutrage, comboOutrage)
	in.OpponentMove = ptrMove(comboOutrage)
	oa := mustCombo(t, in).OpponentAttack
	if oa.MinActions != 1 || oa.MaxActions != 2 || len(oa.Slow.Actions) != 1 || len(oa.Fast.Actions) != 2 {
		t.Fatalf("回数 %d..%d(Slow %d・Fast %d), want 1..2", oa.MinActions, oa.MaxActions, len(oa.Slow.Actions), len(oa.Fast.Actions))
	}
	if oa.Fast.Actions[1].Step != 1 || oa.Fast.Actions[1].Timing != OpponentTimingBeforeSelf {
		t.Errorf("Fast の 2 回目 = 手 %d・%s, want 手 1・before_self", oa.Fast.Actions[1].Step, oa.Fast.Actions[1].Timing)
	}
	// 相手の素早さが分かれば 1 通り(カイリュー 132 < 169 なので自分が先 → 1 回)。
	in.OpponentSpeed = OpponentSpeedFromDefender
	oa = mustCombo(t, in).OpponentAttack
	if oa.MinActions != 1 || oa.MaxActions != 1 || !reflect.DeepEqual(oa.Fast, oa.Slow) {
		t.Errorf("from_defender: 回数 %d..%d・Fast と Slow が違う: %+v / %+v", oa.MinActions, oa.MaxActions, oa.Fast, oa.Slow)
	}
	// 相手の技なし → nil(耐える確率を出さない。行動順は優先度 0 として出す)。
	in.OpponentMove = nil
	if res := mustCombo(t, in); res.OpponentAttack != nil || res.Steps[0].Order != TurnOrderSelfFirst {
		t.Errorf("相手の技なし: OpponentAttack %+v・行動順 %s, want nil・self_first", res.OpponentAttack, res.Steps[0].Order)
	}
}

func TestCalcComboOpponentDamageAfterSelfDefenseDrop(t *testing.T) {
	self, opp := comboGarchompJolly(), comboDragoniteAdamant()
	opDamage := func(selfRanks Ranks) DamageResult {
		t.Helper()
		s := self
		s.Ranks = selfRanks
		d, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: opp, Defender: s, Move: comboDragonClaw})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	before, after := opDamage(Ranks{}), opDamage(Ranks{Def: -1, SpD: -1})
	if after.MaxDamage() <= before.MaxDamage() {
		t.Fatalf("前提: 防御 -1 のほうが痛いはず: %d vs %d", after.MaxDamage(), before.MaxDamage())
	}

	// トリックルーム: 相手(132)が両方の手で先。1 回目はインファイトの前(防御 ±0)、2 回目は後(防御 -1)。
	in := comboInput(self, opp, comboCloseCombat, comboDragonClaw)
	in.OpponentMove = ptrMove(comboDragonClaw)
	in.OpponentSpeed = OpponentSpeedFromDefender
	in.SpeedField.TrickRoom = true
	oa := mustCombo(t, in).OpponentAttack
	if len(oa.Fast.Actions) != 2 {
		t.Fatalf("トリックルーム: 行動 %d 回, want 2", len(oa.Fast.Actions))
	}
	if !reflect.DeepEqual(oa.Fast.Actions[0].Damage, before) || !reflect.DeepEqual(oa.Fast.Actions[1].Damage, after) {
		t.Errorf("トリックルーム: 1 回目 %v・2 回目 %v, want %v・%v", oa.Fast.Actions[0].Damage.Rolls, oa.Fast.Actions[1].Damage.Rolls, before.Rolls, after.Rolls)
	}

	// トリックルームなし: 自分(169)が先。相手の行動は 1 手目のインファイトの後(防御 -1)の 1 回だけ。
	in.SpeedField.TrickRoom = false
	oa = mustCombo(t, in).OpponentAttack
	if len(oa.Fast.Actions) != 1 || oa.Fast.Actions[0].Timing != OpponentTimingAfterSelf || !reflect.DeepEqual(oa.Fast.Actions[0].Damage, after) {
		t.Errorf("トリックルームなし: %+v, want 手 0 の after_self・防御 -1 のダメージ", oa.Fast.Actions)
	}
}

// 相手の攻撃は、自分の側の壁(Field.AttackerScreens)を相手から見た防御側の壁として計算する。
func TestCalcComboOpponentAttackUsesSelfSideScreens(t *testing.T) {
	self, opp := comboMimikyuAdamant(), comboGarchompJolly()
	in := comboInput(self, opp, comboPlayRough)
	in.Field.AttackerScreens.Reflect = true
	in.OpponentMove = ptrMove(comboEarthquake)
	in.OpponentSpeed = OpponentSpeedFromDefender
	want, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: opp, Defender: self, Move: comboEarthquake,
		Field: Field{DefenderScreens: Screens{Reflect: true}}})
	if err != nil {
		t.Fatal(err)
	}
	got := mustCombo(t, in).OpponentAttack.Fast.Actions[0].Damage
	if !reflect.DeepEqual(got, want) {
		t.Errorf("相手の攻撃 = %v, want 自分の側のリフレクターで %v", got.Rolls, want.Rolls)
	}
}

// 相手への確率 100% のランク変化(ほのおのムチの防御 -1・とびかかるの攻撃 -1)は、後の手と相手の攻撃に効く。
func TestCalcComboTargetStagesAffectLaterDamage(t *testing.T) {
	self, opp := comboGarchompJolly(), comboDragoniteAdamant()
	in := comboInput(self, opp, comboFireLash, comboDragonClaw)
	res := mustCombo(t, in)
	if res.Steps[0].OpponentEndRanks != (Ranks{Def: -1}) || res.Steps[1].OpponentStartRanks != (Ranks{Def: -1}) {
		t.Errorf("相手のランク = %+v → 次の手 %+v, want 防御 -1", res.Steps[0].OpponentEndRanks, res.Steps[1].OpponentStartRanks)
	}

	// とびかかる(相手の攻撃 -1)の後の相手の攻撃は攻撃 -1。トリックルームで相手が先 → 2 回目が -1。
	in = comboInput(self, opp, comboLunge, comboDragonClaw)
	in.OpponentMove = ptrMove(comboDragonClaw)
	in.OpponentSpeed = OpponentSpeedFromDefender
	in.SpeedField.TrickRoom = true
	weak := opp
	weak.Ranks = Ranks{Atk: -1}
	want, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: weak, Defender: self, Move: comboDragonClaw})
	if err != nil {
		t.Fatal(err)
	}
	oa := mustCombo(t, in).OpponentAttack
	if len(oa.Fast.Actions) != 2 || !reflect.DeepEqual(oa.Fast.Actions[1].Damage, want) {
		t.Errorf("とびかかるの後の相手の攻撃 = %+v, want 攻撃 -1 の %v", oa.Fast.Actions, want.Rolls)
	}
}

// §7 Q5: がんせきふうじ(相手の素早さ -1)の後は、相手の素早さが下がった値で行動順を決める。
func TestCalcComboRockTombChangesTurnOrder(t *testing.T) {
	in := comboInput(comboMimikyuAdamant(), comboGarchompJolly(), comboRockTomb, comboPlayRough)
	in.OpponentSpeed = OpponentSpeedFromDefender
	res := mustCombo(t, in)
	s0, s1 := res.Steps[0], res.Steps[1]
	if s0.OpponentSpeed != 169 || s0.Order != TurnOrderOpponentFirst {
		t.Errorf("1 手目: 相手 %d・%s, want 169・opponent_first", s0.OpponentSpeed, s0.Order)
	}
	if s1.OpponentStartRanks != (Ranks{Spe: -1}) || s1.OpponentSpeed != 112 || s1.Order != TurnOrderSelfFirst {
		t.Errorf("2 手目: 相手 %+v・%d・%s, want 素早さ -1・112・self_first", s1.OpponentStartRanks, s1.OpponentSpeed, s1.Order)
	}
}

// --- AC-8 ------------------------------------------------------------------------------------------------

func TestCalcComboTurnOrderPriority(t *testing.T) {
	cases := []struct {
		name      string
		self, opp Individual
		move      Move
		oppMove   *Move
		mode      OpponentSpeedMode
		trickRoom bool
		want      TurnOrder
	}{
		{"自分の優先度が高い・自分が遅い", comboMimikyuAdamant(), comboGarchompJolly(), comboShadowSneak, nil, OpponentSpeedFromDefender, false, TurnOrderSelfFirst},
		{"自分の優先度が高い・トリックルームで自分が先のまま", comboMimikyuAdamant(), comboGarchompJolly(), comboShadowSneak, nil, OpponentSpeedFromDefender, true, TurnOrderSelfFirst},
		{"相手の優先度が高い・自分が速い", comboGarchompJolly(), comboMimikyuAdamant(), comboOutrage, ptrMove(comboShadowSneak), OpponentSpeedFromDefender, false, TurnOrderOpponentFirst},
		{"相手の優先度が高い・トリックルーム", comboGarchompJolly(), comboMimikyuAdamant(), comboOutrage, ptrMove(comboShadowSneak), OpponentSpeedFromDefender, true, TurnOrderOpponentFirst},
		{"優先度 +2 対 +1", comboGarchompJolly(), comboMimikyuAdamant(), comboShadowSneak, ptrMove(comboExtremeSpeedLike), OpponentSpeedFromDefender, false, TurnOrderOpponentFirst},
		{"同じ優先度・同じ素早さ", comboGarchompJolly(), comboGarchompJolly(), comboOutrage, nil, OpponentSpeedFromDefender, false, TurnOrderSpeedTie},
		{"同じ優先度・同じ素早さ・トリックルーム", comboGarchompJolly(), comboGarchompJolly(), comboOutrage, nil, OpponentSpeedFromDefender, true, TurnOrderSpeedTie},
		{"素早さが分からなくても優先度が違えば決まる", comboMimikyuAdamant(), comboGarchompJolly(), comboShadowSneak, nil, OpponentSpeedUnknown, false, TurnOrderSelfFirst},
		{"素早さが分からず優先度が同じ", comboMimikyuAdamant(), comboGarchompJolly(), comboPlayRough, nil, OpponentSpeedUnknown, false, TurnOrderDependsOnOpponentSpeed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := comboInput(c.self, c.opp, c.move)
			in.OpponentMove = c.oppMove
			in.OpponentSpeed = c.mode
			in.SpeedField.TrickRoom = c.trickRoom
			if got := mustCombo(t, in).Steps[0].Order; got != c.want {
				t.Errorf("行動順 = %s, want %s", got, c.want)
			}
		})
	}
}

func TestTurnOrderDependsOnOpponentSpeedValue(t *testing.T) {
	// 出力の語彙は combo-calc.md §2-2 と同じ文字列(API・WASM の DTO にそのまま写す)。
	if TurnOrderDependsOnOpponentSpeed != "depends_on_opponent_speed" || OpponentSpeedUnknown != "unknown" || OpponentSpeedFromDefender != "from_defender" ||
		OpponentTimingBeforeSelf != "before_self" || OpponentTimingAfterSelf != "after_self" {
		t.Error("語彙の文字列が仕様と違う")
	}
	if MaxComboSteps != 3 {
		t.Errorf("MaxComboSteps = %d, want 3(§7 Q1)", MaxComboSteps)
	}
}

// --- AC-9・AC-10・AC-12 ----------------------------------------------------------------------------------

func TestCalcComboFlameChargeSpeedTiming(t *testing.T) {
	self, opp := comboGarchompJolly(), comboGarchompJolly() // 同速 169
	in := comboInput(self, opp, comboFlameCharge, comboOutrage)
	in.OpponentSpeed = OpponentSpeedFromDefender
	res := mustCombo(t, in)
	s0, s1 := res.Steps[0], res.Steps[1]
	if s0.SelfSpeed != 169 || s0.Order != TurnOrderSpeedTie {
		t.Errorf("1 手目: 素早さ %d・%s, want 169(+0)・speed_tie", s0.SelfSpeed, s0.Order)
	}
	if s1.SelfStartRanks != (Ranks{Spe: 1}) || s1.SelfSpeed != 253 || s1.Order != TurnOrderSelfFirst {
		t.Errorf("2 手目: %+v・素早さ %d・%s, want 素早さ +1・253・self_first", s1.SelfStartRanks, s1.SelfSpeed, s1.Order)
	}
	want, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: self, Defender: opp, Move: comboFlameCharge})
	if err != nil {
		t.Fatal(err)
	}
	if s0.Damage == nil || !reflect.DeepEqual(*s0.Damage, want) {
		t.Errorf("1 手目のダメージ = %+v, want ランク変更なしの CalcDamage %v", s0.Damage, want.Rolls)
	}
}

func TestCalcComboRanksClamp(t *testing.T) {
	// 攻撃 +6 で つるぎのまい → +6 のまま。次の手は +6 で計算。
	self := comboGarchompJolly()
	self.Ranks = Ranks{Atk: 6}
	res := mustCombo(t, comboInput(self, comboGarchompImpish(), comboSwordsDance, comboOutrage))
	if res.Steps[0].SelfEndRanks != (Ranks{Atk: 6}) || res.Steps[1].SelfStartRanks != (Ranks{Atk: 6}) {
		t.Errorf("攻撃 +6 + 2 = %+v, want +6", res.Steps[0].SelfEndRanks)
	}

	// 素早さ -6 で アームハンマー(自分の素早さ -1)→ -6 のまま。
	self = comboGarchompJolly()
	self.Ranks = Ranks{Spe: -6}
	res = mustCombo(t, comboInput(self, comboGarchompImpish(), comboHammerArm, comboHammerArm))
	if res.Steps[0].SelfEndRanks.Spe != -6 || res.Steps[1].SelfEndRanks.Spe != -6 {
		t.Errorf("素早さ -6 - 1 = %d / %d, want -6", res.Steps[0].SelfEndRanks.Spe, res.Steps[1].SelfEndRanks.Spe)
	}

	// 相手の素早さ -6 で がんせきふうじ → -6 のまま。
	opp := comboGarchompJolly()
	opp.Ranks = Ranks{Spe: -6}
	res = mustCombo(t, comboInput(comboMimikyuAdamant(), opp, comboRockTomb, comboRockTomb))
	if res.Steps[0].OpponentEndRanks.Spe != -6 || res.Steps[1].OpponentEndRanks.Spe != -6 {
		t.Errorf("相手の素早さ -6 - 1 = %d, want -6", res.Steps[1].OpponentEndRanks.Spe)
	}

	// りゅうのまい × 3(3 手 = MaxComboSteps)で +3。
	res = mustCombo(t, comboInput(comboDragoniteAdamant(), comboGarchompImpish(), comboDragonDance, comboDragonDance, comboDragonDance))
	if res.Steps[2].SelfEndRanks != (Ranks{Atk: 3, Spe: 3}) || res.Total.MaxDamage != 0 || res.Total.KO != (ComboKOChance{}) {
		t.Errorf("りゅうのまい × 3 = %+v・合計 %d・%+v, want 攻撃+3 素早さ+3・0・倒せない", res.Steps[2].SelfEndRanks, res.Total.MaxDamage, res.Total.KO)
	}
}

// 当たらなかった攻撃技(タイプで無効・特性で無効/吸収)はランク変化を起こさない(インファイトをゴーストに撃っても防御は下がらない)。
func TestCalcComboNoStagesWhenMoveDoesNotHit(t *testing.T) {
	res := mustCombo(t, comboInput(comboGarchompJolly(), comboMimikyuAdamant(), comboCloseCombat, comboEarthquake))
	if res.Steps[0].Damage == nil || res.Steps[0].Damage.Effectiveness != 0 {
		t.Fatalf("前提: かくとう → ゴーストは無効: %+v", res.Steps[0].Damage)
	}
	if res.Steps[0].SelfEndRanks != (Ranks{}) {
		t.Errorf("無効の技でランクが変わった: %+v", res.Steps[0].SelfEndRanks)
	}

	absorber := comboGarchompImpish()
	absorber.Ability = Ability{ID: "test-flash-fire", Effect: &AbilityEffect{DefAbsorbTypes: map[Type]AbsorbEffect{TypeFire: {}}}}
	res = mustCombo(t, comboInput(comboGarchompJolly(), absorber, comboFlameCharge, comboOutrage))
	if res.Steps[0].Damage.Nullified != NullifyAbsorb || res.Steps[0].SelfEndRanks != (Ranks{}) {
		t.Errorf("吸収された ニトロチャージ: %s・%+v, want absorb・ランク変化なし", res.Steps[0].Damage.Nullified, res.Steps[0].SelfEndRanks)
	}
	// 変化技は相性を見ない(りゅうのまいはいつでも積める)。
	res = mustCombo(t, comboInput(comboDragoniteAdamant(), comboMimikyuAdamant(), comboDragonDance, comboOutrage))
	if res.Steps[0].SelfEndRanks != (Ranks{Atk: 1, Spe: 1}) {
		t.Errorf("りゅうのまい: %+v", res.Steps[0].SelfEndRanks)
	}
}

func TestCalcComboChanceEffectNotApplied(t *testing.T) {
	res := mustCombo(t, comboInput(comboGarchompJolly(), comboGarchompImpish(), comboMetalClaw, comboMetalClaw))
	for i, s := range res.Steps {
		if s.SelfStartRanks != (Ranks{}) || s.SelfEndRanks != (Ranks{}) || s.OpponentEndRanks != (Ranks{}) {
			t.Errorf("手 %d: 確率 10%% の攻撃 +1 を適用した: %+v → %+v", i, s.SelfStartRanks, s.SelfEndRanks)
		}
	}
	// 確率 100% 未満の効果には印を付けない(§7 Q4 (b))。
	if res.Unsupported != nil {
		t.Errorf("印 = %+v, want nil", res.Unsupported)
	}
}

// --- AC-13・AC-14 -----------------------------------------------------------------------------------------

func TestCalcComboStepDamageMatchesCalcDamage(t *testing.T) {
	crit := comboInput(comboGarchompJolly(), comboDragoniteAdamant(), comboFireLash, comboDragonClaw, comboOutrage)
	crit.Steps[1].Critical = true
	hits := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboSwordsDance, comboBulletSeed, comboCloseCombat)
	hits.Steps[1].Hits = 3
	cases := map[string]ComboInput{
		"UC-3":         comboInput(comboMimikyuAdamant(), comboGarchompJolly(), comboPlayRough, comboShadowSneak),
		"相手の防御 -1 と急所": crit,
		"つるぎのまい・多段 3 回・インファイト": hits,
		"ニトロチャージ × 3":          comboInput(comboGarchompJolly(), comboGarchompImpish(), comboFlameCharge, comboFlameCharge, comboFlameCharge),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			res := mustCombo(t, in)
			for i, s := range res.Steps {
				if in.Steps[i].Move.Category == CategoryStatus {
					continue // 変化技の手は Damage nil(TestCalcComboDragonDanceOutrage)。CalcDamage と比べない
				}
				att, def := in.Attacker, in.Defender
				att.Ranks, def.Ranks = s.SelfStartRanks, s.OpponentStartRanks
				want, err := calcDamage(DamageInput{Format: in.Format, Attacker: att, Defender: def, Move: in.Steps[i].Move,
					Field: in.Field, Critical: in.Steps[i].Critical, State: BattleState{Hits: in.Steps[i].Hits}})
				if err != nil {
					t.Fatal(err)
				}
				if s.Damage == nil || !reflect.DeepEqual(*s.Damage, want) {
					t.Errorf("手 %d = %+v, want CalcDamage %+v", i, s.Damage, want)
				}
			}
			if in.Steps[0].Move.ID == "bulletseed" || (len(in.Steps) > 1 && in.Steps[1].Move.ID == "bulletseed") {
				if got := len(res.Steps[1].Damage.HitRolls); got != 3 {
					t.Errorf("多段の 1 発ごとの 16 段階 = %d 件, want 3", got)
				}
			}
		})
	}
}

// 性質テスト: 倒せる確率(全手・手ごとの累積)と耐える確率が、総当たりの数え上げと一致する(AC-14)。
func TestCalcComboKOMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x434f4d42, 0x4f330003)) // 固定の種(再現できるように)
	species := []Species{comboMimikyu, comboGarchomp, comboDragonite}
	natures := []Nature{comboAdamant, comboJolly, comboImpish, NatureNeutral}
	pool := []Move{comboPlayRough, comboShadowSneak, comboEarthquake, comboOutrage, comboDragonClaw, comboDragonDance,
		comboSwordsDance, comboFlameCharge, comboCloseCombat, comboFireLash, comboBulletSeed, comboRockTomb}
	randSP := func() Stats {
		var sp Stats
		left := MaxSPTotal
		for _, k := range []StatKey{StatHP, StatAtk, StatDef, StatSpe} {
			v := min(rng.IntN(MaxSPPerStat+1), left)
			sp = sp.WithStat(k, v)
			left -= v
		}
		return sp
	}
	randInd := func() Individual {
		return comboInd(species[rng.IntN(len(species))], natures[rng.IntN(len(natures))], randSP(), Ranks{})
	}
	partial := 0
	for iter := 0; iter < 60; iter++ {
		n := 1 + rng.IntN(MaxComboSteps)
		in := ComboInput{Format: FormatSingle, Attacker: randInd(), Defender: randInd(), TypeChart: typeChartForTests(),
			OpponentSpeed: OpponentSpeedUnknown}
		multi := false
		for len(in.Steps) < n {
			m := pool[rng.IntN(len(pool))]
			step := ComboStep{Move: m, Critical: rng.IntN(4) == 0}
			if m.Params.MultiHit != nil {
				if multi {
					continue // 総当たりの大きさを抑える(多段は 1 手まで。16^5 通り以下)
				}
				multi = true
				step.Hits = 2 + rng.IntN(2)
			}
			in.Steps = append(in.Steps, step)
		}
		if rng.IntN(2) == 0 {
			in.OpponentMove = ptrMove(pool[rng.IntN(4)])
		}
		res := mustCombo(t, in)

		var draws [][16]int
		minSum, maxSum := 0, 0
		for i, s := range res.Steps {
			draws = append(draws, stepDraws(s.Damage)...)
			if s.Damage != nil {
				minSum += s.Damage.MinDamage()
				maxSum += s.Damage.MaxDamage()
			}
			reach, total := bruteForceReach(draws, res.Total.DefenderHP)
			want := float64(reach) / float64(total) * 100
			got := res.Total.KOByStep[i]
			if !approxEqual(got.ChancePercent, want) || got.Guaranteed != (reach == total) {
				t.Fatalf("iter %d 手 %d: 累積 %+v, want %v%%(%d/%d)。入力 %+v", iter, i, got, want, reach, total, in.Steps)
			}
			if reach > 0 && reach < total {
				partial++
			}
		}
		if res.Total.KO != res.Total.KOByStep[len(res.Steps)-1] {
			t.Fatalf("iter %d: 全手 %+v ≠ 最後の累積 %+v", iter, res.Total.KO, res.Total.KOByStep[len(res.Steps)-1])
		}
		if res.Total.MinDamage != minSum || res.Total.MaxDamage != maxSum {
			t.Fatalf("iter %d: 合計 %d〜%d, want %d〜%d", iter, res.Total.MinDamage, res.Total.MaxDamage, minSum, maxSum)
		}

		if oa := res.OpponentAttack; oa != nil {
			for name, sc := range map[string]OpponentScenario{"Fast": oa.Fast, "Slow": oa.Slow} {
				var od [][16]int
				for j, a := range sc.Actions {
					od = append(od, stepDraws(&a.Damage)...)
					reach, total := bruteForceReach(od, oa.SelfHP)
					want := float64(total-reach) / float64(total) * 100
					if !approxEqual(a.SurviveChancePercent, want) {
						t.Fatalf("iter %d %s 行動 %d: 耐える %v, want %v", iter, name, j, a.SurviveChancePercent, want)
					}
				}
				last := 100.0
				if len(sc.Actions) > 0 {
					last = sc.Actions[len(sc.Actions)-1].SurviveChancePercent
				}
				if sc.SurviveChancePercent != last {
					t.Fatalf("iter %d %s: 耐える %v ≠ 最後の行動 %v", iter, name, sc.SurviveChancePercent, last)
				}
			}
			if oa.MinActions != len(oa.Slow.Actions) || oa.MaxActions != len(oa.Fast.Actions) {
				t.Fatalf("iter %d: 回数 %d..%d と Slow %d・Fast %d が違う", iter, oa.MinActions, oa.MaxActions, len(oa.Slow.Actions), len(oa.Fast.Actions))
			}
		}
	}
	if partial == 0 {
		t.Fatal("乱数で 0% と 100% の間になる手が 1 つも無い(性質テストが空振り。種か範囲を見直す)")
	}
}

func TestComboKOChanceDisplay(t *testing.T) {
	cases := []struct {
		k    ComboKOChance
		want int
	}{
		{ComboKOChance{}, 0},
		{ComboKOChance{ChancePercent: 100, Guaranteed: true}, 1000},
		{ComboKOChance{ChancePercent: 51.5625}, 516},
		{ComboKOChance{ChancePercent: 0.01}, 1},
		{ComboKOChance{ChancePercent: 99.99}, 999},
	}
	for _, c := range cases {
		if got := c.k.DisplayChancePercentTenths(); got != c.want {
			t.Errorf("%+v → %d, want %d", c.k, got, c.want)
		}
	}
}

// --- AC-15・§3-4・印 ---------------------------------------------------------------------------------------

func TestCalcComboChoiceLocked(t *testing.T) {
	lock := &Item{ID: "test-choice", Effect: &ItemEffect{LocksMove: true}}
	self := comboGarchompJolly()
	self.Item = lock
	mark := UnsupportedMark{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedChoiceLocked, ID: "test-choice"}

	res := mustCombo(t, comboInput(self, comboGarchompImpish(), comboOutrage, comboEarthquake))
	if !slices.Contains(res.Unsupported, mark) {
		t.Errorf("技が違う: 印 %+v に %+v が無い", res.Unsupported, mark)
	}
	if res.Steps[1].Damage == nil || res.Steps[1].Damage.MaxDamage() == 0 {
		t.Errorf("印があっても計算は返す: %+v", res.Steps[1].Damage)
	}
	res = mustCombo(t, comboInput(self, comboGarchompImpish(), comboOutrage, comboOutrage))
	if slices.Contains(res.Unsupported, mark) {
		t.Errorf("同じ技: 印 %+v, want choice_locked なし", res.Unsupported)
	}
	// こだわらない持ち物は印なし。
	self.Item = &Item{ID: "test-plain", Effect: &ItemEffect{}}
	if res := mustCombo(t, comboInput(self, comboGarchompImpish(), comboOutrage, comboEarthquake)); res.Unsupported != nil {
		t.Errorf("こだわらない持ち物: 印 %+v, want nil", res.Unsupported)
	}
}

func TestCalcComboDefenderFullHPAssumed(t *testing.T) {
	sashItem := &Item{ID: "test-sash", Effect: &ItemEffect{FullHPOnly: true}}
	scaleAbility := Ability{ID: "test-scale", Effect: &AbilityEffect{FullHPOnly: true}}

	withItem := comboGarchompImpish()
	withItem.Item = sashItem
	withAbility := comboGarchompImpish()
	withAbility.Ability = scaleAbility

	itemMark := UnsupportedMark{Target: UnsupportedTargetDefenderItem, Reason: UnsupportedDefenderFullHPAssumed, ID: "test-sash"}
	abilityMark := UnsupportedMark{Target: UnsupportedTargetDefenderAbility, Reason: UnsupportedDefenderFullHPAssumed, ID: "test-scale"}
	moveMark := UnsupportedMark{Target: UnsupportedTargetMove, Reason: UnsupportedDefenderFullHPAssumed, ID: "hardpress"}

	cases := []struct {
		name string
		in   ComboInput
		mark UnsupportedMark
		want bool
	}{
		{"持ち物・2 手", comboInput(comboGarchompJolly(), withItem, comboOutrage, comboOutrage), itemMark, true},
		{"持ち物・1 手(満タンは正しい)", comboInput(comboGarchompJolly(), withItem, comboOutrage), itemMark, false},
		{"持ち物・2 手目が変化技だけ", comboInput(comboGarchompJolly(), withItem, comboOutrage, comboSwordsDance), itemMark, false},
		{"特性・2 手", comboInput(comboGarchompJolly(), withAbility, comboOutrage, comboOutrage), abilityMark, true},
		{"残り HP で威力が変わる技を 2 手目", comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage, comboHardPress), moveMark, true},
		{"残り HP で威力が変わる技を 1 手目だけ", comboInput(comboGarchompJolly(), comboGarchompImpish(), comboHardPress, comboOutrage), moveMark, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := mustCombo(t, c.in)
			if got := slices.Contains(res.Unsupported, c.mark); got != c.want {
				t.Errorf("印 %+v に %+v: %v, want %v", res.Unsupported, c.mark, got, c.want)
			}
		})
	}
}

// 各手の印は手の順に連結し、重複を除く。連続技の印はその後(ADR-0179 追記 CB-3 §5 の順)。
func TestCalcComboUnsupportedMarksConcatenated(t *testing.T) {
	odd := Move{ID: "test-odd", Type: TypeNormal, Category: CategoryPhysical, Power: 80, Mechanisms: []MoveMechanism{MechanismMoveSpecific}}
	self := comboGarchompJolly()
	self.Item = &Item{ID: "test-choice", Effect: &ItemEffect{LocksMove: true}}
	res := mustCombo(t, comboInput(self, comboGarchompImpish(), odd, odd, comboOutrage))
	want := []UnsupportedMark{
		{Target: UnsupportedTargetMove, Reason: UnsupportedReason(MechanismMoveSpecific), ID: "test-odd"},
		{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedChoiceLocked, ID: "test-choice"},
	}
	if !reflect.DeepEqual(res.Unsupported, want) {
		t.Errorf("印 = %+v, want %+v", res.Unsupported, want)
	}
	if res.Steps[0].Damage.Unsupported == nil {
		t.Error("手の Damage.Unsupported は CalcDamage のまま残す")
	}
}

// 素早さの補正の条件を判定できない(FinalSpeed の *Undetermined)ときは speed_undetermined の印。
func TestCalcComboSpeedUndeterminedMark(t *testing.T) {
	self := comboGarchompJolly()
	self.Ability = Ability{ID: "test-unburden", Effect: &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionItemLost, Modifier: 8192}}}}
	opp := comboGarchompJolly()
	opp.Ability = Ability{ID: "test-unburden-opp", Effect: &AbilityEffect{SpeedMods: []SpeedMod{{Condition: SpeedConditionItemLost, Modifier: 8192}}}}
	selfMark := UnsupportedMark{Target: UnsupportedTargetAttackerAbility, Reason: UnsupportedSpeedUndetermined, ID: "test-unburden"}
	oppMark := UnsupportedMark{Target: UnsupportedTargetDefenderAbility, Reason: UnsupportedSpeedUndetermined, ID: "test-unburden-opp"}

	in := comboInput(self, opp, comboOutrage)
	res := mustCombo(t, in)
	if !slices.Contains(res.Unsupported, selfMark) || slices.Contains(res.Unsupported, oppMark) {
		t.Errorf("unknown: 印 %+v, want 自分の印だけ(相手の素早さは計算しない)", res.Unsupported)
	}
	in.OpponentSpeed = OpponentSpeedFromDefender
	res = mustCombo(t, in)
	if !slices.Contains(res.Unsupported, selfMark) || !slices.Contains(res.Unsupported, oppMark) {
		t.Errorf("from_defender: 印 %+v, want 両方", res.Unsupported)
	}
}

// --- AC-16 ------------------------------------------------------------------------------------------------

func TestCalcComboRejectsInvalidInput(t *testing.T) {
	badSP := comboGarchompJolly()
	badSP.SP = Stats{HP: 33}
	four := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage, comboOutrage, comboOutrage, comboOutrage)
	cases := []struct {
		name string
		in   ComboInput
		want error
	}{
		{"手 0 件", comboInput(comboGarchompJolly(), comboGarchompImpish()), ErrInvalidComboInput},
		{"手 0 件は個体の検査より前", comboInput(badSP, comboGarchompImpish()), ErrInvalidComboInput},
		{"上限超え", four, ErrInvalidComboInput},
		{"上限超えは個体の検査より前", ComboInput{Attacker: badSP, Defender: comboGarchompImpish(), TypeChart: typeChartForTests(), Steps: four.Steps}, ErrInvalidComboInput},
		{"ランク変化の無い変化技", comboInput(comboGarchompJolly(), comboGarchompImpish(), comboProtectLike, comboOutrage), ErrInvalidComboInput},
		{"確率 100% 未満の変化だけの変化技", comboInput(comboGarchompJolly(), comboGarchompImpish(), comboChanceOnlyStatus), ErrInvalidComboInput},
		{"相手の技が変化技", func() ComboInput {
			in := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage)
			in.OpponentMove = ptrMove(comboDragonDance)
			return in
		}(), ErrInvalidComboInput},
		{"未知の相手の素早さ", func() ComboInput {
			in := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage)
			in.OpponentSpeed = "fast"
			return in
		}(), ErrInvalidComboInput},
		{"範囲の多段でない技に回数", func() ComboInput {
			in := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage)
			in.Steps[0].Hits = 2
			return in
		}(), ErrInvalidBattleState},
		{"多段の回数が範囲外", func() ComboInput {
			in := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboBulletSeed)
			in.Steps[0].Hits = 6
			return in
		}(), ErrInvalidBattleState},
		{"相性表なし", func() ComboInput {
			in := comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage)
			in.TypeChart = TypeChart{}
			return in
		}(), ErrTypeChartMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CalcCombo(c.in)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}

	// 変化技だけの手でも個体は検査する(CalcDamage を呼ばない手で検査が抜けない)。
	if _, err := CalcCombo(comboInput(badSP, comboGarchompImpish(), comboDragonDance)); err == nil {
		t.Error("SP 33 の攻撃側・変化技だけ: err = nil, want 検査エラー")
	}
	// 上限ちょうど(3 手)は通る。
	if _, err := CalcCombo(comboInput(comboGarchompJolly(), comboGarchompImpish(), comboOutrage, comboOutrage, comboOutrage)); err != nil {
		t.Errorf("3 手: %v", err)
	}
}

// 入力を書き換えない(呼び出し側の Individual・Steps をそのまま再利用できる)。
func TestCalcComboDoesNotMutateInput(t *testing.T) {
	in := comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboDragonDance, comboRockTomb, comboOutrage)
	in.OpponentMove = ptrMove(comboOutrage)
	before := comboInput(comboDragoniteAdamant(), comboGarchompJolly(), comboDragonDance, comboRockTomb, comboOutrage)
	before.OpponentMove = ptrMove(comboOutrage)
	mustCombo(t, in)
	if !reflect.DeepEqual(in.Attacker, before.Attacker) || !reflect.DeepEqual(in.Defender, before.Defender) || !reflect.DeepEqual(in.Steps, before.Steps) {
		t.Error("CalcCombo が入力を書き換えた")
	}
}
