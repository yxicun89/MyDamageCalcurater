package engine

// 連続技 CalcCombo(ADR-0179 追記「CB-3」。docs/plan/combo-calc.md)。
//
// 攻撃側の手(最大 MaxComboSteps)を順に撃ち、手ごとに両側のランクを進めて CalcDamage・FinalSpeed を呼ぶ。
// 新しいダメージ式を持たない。合計の倒せる確率は、各手の 16 段階(多段は 1 発ごと)を独立とした畳み込みで求める。

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// MaxComboSteps は連続技の手の数の上限(§7 Q1)。
const MaxComboSteps = 3

// ErrInvalidComboInput は連続技の入力が不正(手の数・相手の素早さの指定・ランク変化の無い変化技・変化技の相手の技)。
var ErrInvalidComboInput = errors.New("連続技の入力が不正")

// OpponentSpeedMode は相手の素早さの扱い。
type OpponentSpeedMode string

const (
	// OpponentSpeedUnknown は相手の素早さを使わない(""も同じ)。行動順は優先度が違うときだけ決まる。
	OpponentSpeedUnknown OpponentSpeedMode = "unknown"
	// OpponentSpeedFromDefender は防御側の個体の素早さを相手の素早さとして使う。
	OpponentSpeedFromDefender OpponentSpeedMode = "from_defender"
)

// ComboSpeedField は行動順に効く場の効果。
type ComboSpeedField struct{ TrickRoom, SelfTailwind, OpponentTailwind bool }

// ComboStep は連続技の 1 手。
type ComboStep struct {
	Move     Move
	Critical bool
	Hits     int // BattleState.Hits と同じ(0 は既定。範囲の多段だけ)
}

// ComboInput は CalcCombo の入力。
type ComboInput struct {
	Format        Format
	Attacker      Individual // Ranks は 1 手目の開始時
	Defender      Individual // 相手の振り方
	Field         Field      // 全手で同じ
	TypeChart     TypeChart
	Steps         []ComboStep
	OpponentMove  *Move // nil は相手の攻撃を計算しない(行動順は優先度 0 として出す)
	OpponentSpeed OpponentSpeedMode
	SpeedField    ComboSpeedField
}

// SpeedRange は素早さの範囲(両端を含む)。Min > Max は空。
type SpeedRange struct{ Min, Max int }

// SpeedLine は相手の素早さがどの範囲なら自分が先か。
type SpeedLine struct {
	SelfSpeed     int
	SelfFirst     SpeedRange
	OpponentFirst SpeedRange
}

// ComboStepResult は 1 手の結果。
type ComboStepResult struct {
	MoveID                               string
	SelfStartRanks, SelfEndRanks         Ranks
	OpponentStartRanks, OpponentEndRanks Ranks
	Damage                               *DamageResult // 変化技は nil
	SelfSpeed                            int
	OpponentSpeed                        int // from_defender のときだけ(unknown は 0)
	SelfPriority, OpponentPriority       int
	Order                                TurnOrder
	Line                                 SpeedLine
}

// ComboKOChance は倒せる確率。
type ComboKOChance struct {
	ChancePercent float64 // 0..100
	Guaranteed    bool    // Σ最小 ≥ HP(このとき ChancePercent = 100)
}

// DisplayChancePercentTenths は画面に出せる値(0.1% 単位)。0 → 0、確定 → 1000、他は四捨五入して 1..999(KOChance と同じ規則)。
func (k ComboKOChance) DisplayChancePercentTenths() int {
	switch {
	case k.Guaranteed:
		return 1000
	case k.ChancePercent <= 0:
		return 0
	}
	return min(max(int(math.Floor(k.ChancePercent*10+0.5)), 1), 999)
}

// ComboTotal は全手の合計。
type ComboTotal struct {
	MinDamage, MaxDamage, DefenderHP int
	KO                               ComboKOChance
	KOByStep                         []ComboKOChance // i 手目までで倒せる確率(累積)
}

// DisplayPercentRangeTenths は合計ダメージの表示%(0.1% 単位)。最小は切り捨て・最大は四捨五入(ADR-0010 §3.2)。
func (t ComboTotal) DisplayPercentRangeTenths() (minTenths, maxTenths int) {
	return DisplayPercentTenthsFloor(t.MinDamage, t.DefenderHP), DisplayPercentTenthsRound(t.MaxDamage, t.DefenderHP)
}

// ComboSpeed は連続技の素早さの要約。
type ComboSpeed struct {
	FirstSelfSpeed, LastSelfSpeed int
	OpponentSpeed                 int // 1 手目の開始時。from_defender のときだけ
	OpponentSpeedKnown            bool
}

// OpponentTiming は相手の行動が自分の手の前か後か。
type OpponentTiming string

const (
	OpponentTimingBeforeSelf OpponentTiming = "before_self"
	OpponentTimingAfterSelf  OpponentTiming = "after_self"
)

// OpponentAction は相手の 1 回の行動。
type OpponentAction struct {
	Step                 int // 0 始まり
	Timing               OpponentTiming
	Damage               DamageResult // その時点の両側のランクで
	SurviveChancePercent float64      // ここまでの全行動を耐える確率
}

// OpponentScenario は順番が決まらない手を一方に倒した場合の相手の行動の列。
type OpponentScenario struct {
	Actions              []OpponentAction
	SurviveChancePercent float64 // 行動なしは 100
}

// OpponentAttack は連続技を撃ち終わるまでに受ける相手の攻撃。
type OpponentAttack struct {
	SelfHP                 int
	Fast, Slow             OpponentScenario // 順番が決まらない手を「相手が先」/「自分が先」とした場合
	MinActions, MaxActions int              // = len(Slow.Actions) / len(Fast.Actions)
}

// ComboResult は CalcCombo の結果。
type ComboResult struct {
	Steps          []ComboStepResult
	Total          ComboTotal
	Speed          ComboSpeed
	OpponentAttack *OpponentAttack // OpponentMove が nil なら nil
	Unsupported    []UnsupportedMark
}

func comboInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidComboInput, fmt.Sprintf(format, args...))
}

// validateComboInput は ADR-0179 追記 CB-3 §4 の順に入力を確かめる。
func validateComboInput(in ComboInput) error {
	if n := len(in.Steps); n < 1 || n > MaxComboSteps {
		return comboInvalid("手は 1..%d 手: %d", MaxComboSteps, n)
	}
	switch in.OpponentSpeed {
	case "", OpponentSpeedUnknown, OpponentSpeedFromDefender:
	default:
		return comboInvalid("相手の素早さの指定が不明: %q", in.OpponentSpeed)
	}
	for i, s := range in.Steps {
		if s.Move.Category == CategoryStatus && GuaranteedSelfStages(s.Move) == (Ranks{}) && GuaranteedTargetStages(s.Move) == (Ranks{}) {
			return comboInvalid("%d 手目の変化技 %q は確率 100%% のランク変化を持たない", i+1, s.Move.ID)
		}
	}
	if m := in.OpponentMove; m != nil && m.Category == CategoryStatus {
		return comboInvalid("相手の技 %q は変化技", m.ID)
	}
	if in.TypeChart.IsZero() {
		return ErrTypeChartMissing
	}
	if err := in.Attacker.Validate(); err != nil {
		return err
	}
	return in.Defender.Validate()
}

// swapScreens は相手の攻撃を計算するために壁を入れ替える(自分の側の壁は相手から見た防御側の壁)。
func swapScreens(f Field) Field {
	f.AttackerScreens, f.DefenderScreens = f.DefenderScreens, f.AttackerScreens
	return f
}

// drawsOf はダメージ結果の独立な 16 段階の並び(多段は 1 発ごと)。
func drawsOf(d DamageResult) [][16]int {
	if len(d.HitRolls) >= 2 {
		return d.HitRolls
	}
	return [][16]int{d.Rolls}
}

func speedLine(selfSpeed int, trickRoom bool) SpeedLine {
	slow := SpeedRange{Min: 0, Max: selfSpeed - 1}
	fast := SpeedRange{Min: selfSpeed + 1, Max: maxSpeed}
	if trickRoom {
		return SpeedLine{SelfSpeed: selfSpeed, SelfFirst: fast, OpponentFirst: slow}
	}
	return SpeedLine{SelfSpeed: selfSpeed, SelfFirst: slow, OpponentFirst: fast}
}

// CalcCombo は攻撃側の連続技(最大 MaxComboSteps 手)を計算する。
func CalcCombo(in ComboInput) (ComboResult, error) {
	if err := validateComboInput(in); err != nil {
		return ComboResult{}, err
	}
	fromDefender := in.OpponentSpeed == OpponentSpeedFromDefender
	defHP := RealStats(in.Defender).HP
	selfHP := RealStats(in.Attacker).HP

	res := ComboResult{Steps: make([]ComboStepResult, 0, len(in.Steps))}
	res.Total.DefenderHP = defHP
	targetFold := newKOFold(defHP)
	var fastActs, slowActs []OpponentAction
	var undet [4]bool // 攻撃側の特性・持ち物・防御側の特性・持ち物
	var marks []UnsupportedMark

	cur, oppCur := in.Attacker.Ranks, in.Defender.Ranks
	last := len(in.Steps) - 1
	for i, step := range in.Steps {
		self, opp := in.Attacker, in.Defender
		self.Ranks, opp.Ranks = cur, oppCur
		sr := ComboStepResult{MoveID: step.Move.ID, SelfStartRanks: cur, OpponentStartRanks: oppCur}

		sp, err := FinalSpeed(self, in.Field, SpeedSide{Tailwind: in.SpeedField.SelfTailwind})
		if err != nil {
			return ComboResult{}, err
		}
		sr.SelfSpeed = sp.Speed
		undet[0] = undet[0] || sp.AbilityUndetermined
		undet[1] = undet[1] || sp.ItemUndetermined
		bySpeed := TurnOrderDependsOnOpponentSpeed
		if fromDefender {
			op, err := FinalSpeed(opp, in.Field, SpeedSide{Tailwind: in.SpeedField.OpponentTailwind})
			if err != nil {
				return ComboResult{}, err
			}
			sr.OpponentSpeed = op.Speed
			undet[2] = undet[2] || op.AbilityUndetermined
			undet[3] = undet[3] || op.ItemUndetermined
			bySpeed = CompareSpeed(sr.SelfSpeed, sr.OpponentSpeed, in.SpeedField.TrickRoom)
		}
		sr.SelfPriority = EffectivePriority(step.Move, self, in.Field)
		if in.OpponentMove != nil {
			sr.OpponentPriority = EffectivePriority(*in.OpponentMove, opp, in.Field)
		}
		sr.Order = CompareTurnOrder(sr.SelfPriority, sr.OpponentPriority, bySpeed)
		sr.Line = speedLine(sr.SelfSpeed, in.SpeedField.TrickRoom)

		// 相手が先に動く場合の行動(この手の前)。
		oppFast := sr.Order != TurnOrderSelfFirst
		oppSlow := sr.Order == TurnOrderOpponentFirst
		oppAttack := func(c, oc Ranks) (DamageResult, error) {
			s, o := in.Attacker, in.Defender
			s.Ranks, o.Ranks = c, oc
			return CalcDamage(DamageInput{Format: in.Format, Attacker: o, Defender: s, Move: *in.OpponentMove,
				Field: swapScreens(in.Field), TypeChart: in.TypeChart})
		}
		if in.OpponentMove != nil && (oppFast || oppSlow) {
			d, err := oppAttack(cur, oppCur)
			if err != nil {
				return ComboResult{}, err
			}
			act := OpponentAction{Step: i, Timing: OpponentTimingBeforeSelf, Damage: d}
			if oppFast {
				fastActs = append(fastActs, act)
			}
			if oppSlow {
				slowActs = append(slowActs, act)
			}
		}

		hit := true
		if step.Move.Category != CategoryStatus {
			d, err := CalcDamage(DamageInput{Format: in.Format, Attacker: self, Defender: opp, Move: step.Move, Field: in.Field,
				Critical: step.Critical, TypeChart: in.TypeChart, State: BattleState{Hits: step.Hits}})
			if err != nil {
				return ComboResult{}, err
			}
			sr.Damage = &d
			hit = d.Effectiveness != 0 && d.Nullified == NullifyNone
			res.Total.MinDamage += d.MinDamage()
			res.Total.MaxDamage += d.MaxDamage()
			for _, r := range drawsOf(d) {
				targetFold.add(r)
			}
			for _, m := range d.Unsupported {
				if !slices.Contains(marks, m) {
					marks = append(marks, m)
				}
			}
		}
		if hit {
			cur = cur.AddClamped(GuaranteedSelfStages(step.Move))
			oppCur = oppCur.AddClamped(GuaranteedTargetStages(step.Move))
		}
		sr.SelfEndRanks, sr.OpponentEndRanks = cur, oppCur

		p, g := targetFold.reach()
		res.Total.KOByStep = append(res.Total.KOByStep, comboKO(p, g))

		// 相手が後に動く場合の行動(この手の後。最後の手の後は数えない)。
		if in.OpponentMove != nil && i < last && (!oppFast || !oppSlow) {
			d, err := oppAttack(cur, oppCur)
			if err != nil {
				return ComboResult{}, err
			}
			act := OpponentAction{Step: i, Timing: OpponentTimingAfterSelf, Damage: d}
			if !oppFast {
				fastActs = append(fastActs, act)
			}
			if !oppSlow {
				slowActs = append(slowActs, act)
			}
		}
		res.Steps = append(res.Steps, sr)
	}
	res.Total.KO = res.Total.KOByStep[last]

	res.Speed = ComboSpeed{FirstSelfSpeed: res.Steps[0].SelfSpeed, LastSelfSpeed: res.Steps[last].SelfSpeed,
		OpponentSpeed: res.Steps[0].OpponentSpeed, OpponentSpeedKnown: fromDefender}

	if in.OpponentMove != nil {
		fast, slow := surviveScenario(fastActs, selfHP), surviveScenario(slowActs, selfHP)
		res.OpponentAttack = &OpponentAttack{SelfHP: selfHP, Fast: fast, Slow: slow,
			MinActions: len(slow.Actions), MaxActions: len(fast.Actions)}
	}
	res.Unsupported = comboMarks(in, marks, undet)
	return res, nil
}

func comboKO(prob float64, guaranteed bool) ComboKOChance {
	if guaranteed {
		return ComboKOChance{ChancePercent: 100, Guaranteed: true}
	}
	return ComboKOChance{ChancePercent: prob * 100}
}

// surviveScenario は行動の列に、ここまでの全行動を耐える確率を付ける。
func surviveScenario(acts []OpponentAction, hp int) OpponentScenario {
	sc := OpponentScenario{Actions: acts, SurviveChancePercent: 100}
	fold := newKOFold(hp)
	for i := range sc.Actions {
		for _, r := range drawsOf(sc.Actions[i].Damage) {
			fold.add(r)
		}
		p, g := fold.reach()
		sc.Actions[i].SurviveChancePercent = 100 - comboKO(p, g).ChancePercent
		sc.SurviveChancePercent = sc.Actions[i].SurviveChancePercent
	}
	return sc
}

// comboMarks は ADR-0179 追記 CB-3 §5 の順に印を並べる。印が無ければ nil。
func comboMarks(in ComboInput, stepMarks []UnsupportedMark, undet [4]bool) []UnsupportedMark {
	marks := stepMarks
	add := func(m UnsupportedMark) { marks = append(marks, m) }

	ids := [4]struct {
		target UnsupportedTarget
		id     string
	}{
		{UnsupportedTargetAttackerAbility, in.Attacker.Ability.ID},
		{UnsupportedTargetAttackerItem, comboItemID(in.Attacker.Item)},
		{UnsupportedTargetDefenderAbility, in.Defender.Ability.ID},
		{UnsupportedTargetDefenderItem, comboItemID(in.Defender.Item)},
	}
	for i, u := range undet {
		if u {
			add(UnsupportedMark{Target: ids[i].target, Reason: UnsupportedSpeedUndetermined, ID: ids[i].id})
		}
	}

	if e := itemEffect(in.Attacker.Item); e != nil && e.LocksMove {
		first := in.Steps[0].Move.ID
		if slices.ContainsFunc(in.Steps, func(s ComboStep) bool { return s.Move.ID != first }) {
			add(UnsupportedMark{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedChoiceLocked, ID: in.Attacker.Item.ID})
		}
	}

	laterAttack := false
	var hpMoves []string
	for _, s := range in.Steps[1:] {
		if s.Move.Category == CategoryStatus {
			continue
		}
		laterAttack = true
		if r := s.Move.Rule; r != nil && (r.PowerFormula == PowerFormulaDefenderHPRatio ||
			r.FixedDamageFormula == FixedDamageDefenderHalfHP || r.FixedDamageFormula == FixedDamageDefenderMinusAttackerHP) &&
			!slices.Contains(hpMoves, s.Move.ID) {
			hpMoves = append(hpMoves, s.Move.ID)
		}
	}
	if laterAttack {
		if e := itemEffect(in.Defender.Item); e != nil && e.FullHPOnly {
			add(UnsupportedMark{Target: UnsupportedTargetDefenderItem, Reason: UnsupportedDefenderFullHPAssumed, ID: in.Defender.Item.ID})
		}
		if e := in.Defender.Ability.Effect; e != nil && e.FullHPOnly {
			add(UnsupportedMark{Target: UnsupportedTargetDefenderAbility, Reason: UnsupportedDefenderFullHPAssumed, ID: in.Defender.Ability.ID})
		}
	}
	for _, id := range hpMoves {
		add(UnsupportedMark{Target: UnsupportedTargetMove, Reason: UnsupportedDefenderFullHPAssumed, ID: id})
	}
	if len(marks) == 0 {
		return nil
	}
	return marks
}

func comboItemID(i *Item) string {
	if i == nil {
		return ""
	}
	return i.ID
}
