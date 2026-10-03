package engine

import (
	"fmt"
	"math"
)

// SP 配分の提案(plan.md AJ3・機能 3)。定義は ADR-0150 §8。
//
// 使用者が「ここまで振りたい」能力を下限(Self.SP)として固定し、残りの SP(MaxSPTotal − 下限の合計)を
// 耐久側(H・B・D)か攻撃側(A または C と S)に回す。各能力の上限(Ceiling)も使用者が決める。
// 結果は「指数最大の組」と、目標(Goal)を渡したときの「目標を満たす最小 SP の組」の2つ。
// どちらも各能力 0..MaxSPPerStat・合計 MaxSPTotal 以内を守る。探索は engine 内の総当たり(ADR-0150 §1)。

// AllocMode は残り SP を回す側。
type AllocMode string

const (
	// AllocModeBulk は耐久側。H・B・D に回す。素早さは見ない。
	AllocModeBulk AllocMode = "bulk"
	// AllocModeOffense は攻撃側。A(物理)または C(特殊)と S に回す。
	AllocModeOffense AllocMode = "offense"
)

// BulkFocus は耐久側の指数最大で、どの耐久指数を大きくするか(ADR-0150 §8)。
type BulkFocus string

const (
	// BulkFocusPhysical は物理耐久指数 H×B を最大にする。
	BulkFocusPhysical BulkFocus = "physical"
	// BulkFocusSpecial は特殊耐久指数 H×D を最大にする。
	BulkFocusSpecial BulkFocus = "special"
	// BulkFocusBoth は小さい方の耐久指数 min(H×B, H×D) を最大にし、同点なら大きい方を最大にする。
	BulkFocusBoth BulkFocus = "both"
)

// SPAllocInput は SP 配分の提案の入力(ADR-0150 §8)。
type SPAllocInput struct {
	// Self は自分の個体。Self.SP は各能力の下限(「ここまで振りたい」)。結果の SP は各能力でこれ以上になる。
	Self Individual
	// Ceiling は残り SP を回す能力の上限(下限 ≤ 上限 ≤ MaxSPPerStat)。0 は「下限より上には振らない」。
	// 回さない能力(耐久側の A・C・S、攻撃側の H・B・D と A/C の使わない方)の値は見ない。
	Ceiling Stats
	// Mode は残り SP を回す側。
	Mode AllocMode
	// Focus は耐久側の指数最大の基準。攻撃側では見ない。
	Focus BulkFocus
	// OffenseCategory は攻撃側で使う技の分類(物理 → A、特殊 → C)。耐久側では見ない。
	OffenseCategory MoveCategory
	// MinSpeed は攻撃側の素早さの目標(S 実数値の下限。相手の素早さ + 1 を渡せば「超え」)。0 は目標なし。
	// 耐久側では 0 でなければならない(素早さは見ない)。
	MinSpeed int
	// Goal は最小 SP の目標。nil なら最小 SP の組を求めない。
	Goal *AllocGoal
}

// AllocGoal は最小 SP の組の目標(ADR-0150 §8)。判定は AJ2(§7)と同じ定義。
// 耐久側: Opponent の Move を Hits 発受けて耐える確率 ≥ しきい値。
// 攻撃側: Move で Opponent を Hits 発で倒す確率 ≥ しきい値(Move の分類は OffenseCategory と一致させる)。
type AllocGoal struct {
	Format   Format
	Opponent Individual
	Move     Move
	Field    Field
	Critical bool
	// TypeChart はタイプ相性表(ADR-0013)。解釈せず CalcDamage へ素通しする。
	TypeChart TypeChart
	// Hits は目標の発数(1..MaxAdjustHits)。
	Hits int
	// ThresholdPercent は満たすべき確率(%)。0 は DefaultAdjustThresholdPercent(100 = 確定)。
	ThresholdPercent float64
}

// AllocPlan は提案する SP の組1つ。
type AllocPlan struct {
	// SP は全6能力の SP(回さない能力は下限のまま)。
	SP Stats
	// TotalSP は SP の合計(≤ MaxSPTotal)。
	TotalSP int
	// Real は SP と性格から求めた実数値(RealStats。ランク補正は含めない)。
	Real Stats
	// PhysicalBulk・SpecialBulk は等倍の耐久指数(H 実数値 × B 実数値、H 実数値 × D 実数値。ADR-0150 §3)。
	PhysicalBulk int
	SpecialBulk  int
	// SpeedMet は素早さの目標を満たすか(攻撃側で MinSpeed > 0 のときだけ偽になりうる。それ以外は真)。
	SpeedMet bool
	// GoalMet は Goal を満たすか(Goal が nil なら偽)。
	GoalMet bool
	// ChancePercent は Goal の確率(耐久側は耐える確率、攻撃側は倒す確率。%)。Goal が nil なら 0。
	ChancePercent float64
}

// SPAllocResult は SP 配分の提案の結果(ADR-0150 §8)。
type SPAllocResult struct {
	// Remaining は残り SP = MaxSPTotal − 下限の合計。
	Remaining int
	// MaxIndex は指数最大の組。
	MaxIndex AllocPlan
	// MinSP は目標を満たす最小 SP の組(満たせなければ §8 の代わりの組)。Goal が nil なら nil。
	MinSP *AllocPlan
	// Unsupported は Goal の計算に付いた「未対応」の印(ADR-0123)。SP によらず同じ。Goal が nil なら nil。
	Unsupported []UnsupportedMark
}

// SuggestSPAllocation は下限(Self.SP)を固定したまま残り SP を Mode の側に回し、
// 指数最大の組と(Goal があれば)目標を満たす最小 SP の組を返す(ADR-0150 §8)。
// 入力が不正なら ErrInvalidAdjustInput を包んで返す。相性表の誤りは CalcDamage のエラーを包んで返す。
func SuggestSPAllocation(in SPAllocInput) (SPAllocResult, error) {
	keys, err := validateAlloc(in)
	if err != nil {
		return SPAllocResult{}, err
	}
	var threshold float64
	if in.Goal != nil {
		if threshold, err = validateAllocGoal(in); err != nil {
			return SPAllocResult{}, err
		}
	}

	s := allocSearch{in: in, threshold: threshold, damage: map[[2]int]DamageResult{}}
	if err := s.enumerate(keys, 0, in.Self.SP); err != nil {
		return SPAllocResult{}, err
	}

	maxIndex := s.maxAny
	if s.maxSpeedMet != nil {
		maxIndex = s.maxSpeedMet
	}
	out := SPAllocResult{
		Remaining:   MaxSPTotal - in.Self.SP.Sum(),
		MaxIndex:    maxIndex.plan(),
		Unsupported: s.unsupported,
	}
	if in.Goal != nil {
		best := s.minSatisfied
		if best == nil {
			best = s.minFallback
		}
		plan := best.plan()
		out.MinSP = &plan
	}
	return out, nil
}

// allocKeys は Mode で残り SP を回す能力(ADR-0150 §8 の表)。
func allocKeys(in SPAllocInput) []StatKey {
	if in.Mode == AllocModeBulk {
		return []StatKey{StatHP, StatDef, StatSpD}
	}
	return []StatKey{reverseStat(SideAttacker, in.OffenseCategory), StatSpe}
}

// validateAlloc は Goal 以外の入力を検査し、回す能力を返す。
func validateAlloc(in SPAllocInput) ([]StatKey, error) {
	switch in.Mode {
	case AllocModeBulk:
		switch in.Focus {
		case BulkFocusPhysical, BulkFocusSpecial, BulkFocusBoth:
		default:
			return nil, fmt.Errorf("%w: 未知の耐久の基準: %q", ErrInvalidAdjustInput, in.Focus)
		}
		if in.MinSpeed != 0 {
			return nil, fmt.Errorf("%w: 耐久側は素早さを見ない: MinSpeed=%d", ErrInvalidAdjustInput, in.MinSpeed)
		}
	case AllocModeOffense:
		if in.OffenseCategory != CategoryPhysical && in.OffenseCategory != CategorySpecial {
			return nil, fmt.Errorf("%w: 攻撃側の分類 %q はダメージを与えない", ErrInvalidAdjustInput, in.OffenseCategory)
		}
		if in.MinSpeed < 0 {
			return nil, fmt.Errorf("%w: 素早さの目標が負: %d", ErrInvalidAdjustInput, in.MinSpeed)
		}
	default:
		return nil, fmt.Errorf("%w: 未知のモード: %q", ErrInvalidAdjustInput, in.Mode)
	}
	if err := in.Self.Validate(); err != nil {
		return nil, fmt.Errorf("%w: 自分: %v", ErrInvalidAdjustInput, err)
	}
	keys := allocKeys(in)
	for _, k := range keys {
		floor, ceiling := in.Self.SP.Get(k), in.Ceiling.Get(k)
		if ceiling < floor || ceiling > MaxSPPerStat {
			return nil, fmt.Errorf("%w: %s の上限は %d..%d の範囲外: %d", ErrInvalidAdjustInput, k, floor, MaxSPPerStat, ceiling)
		}
	}
	return keys, nil
}

// validateAllocGoal は Goal を §7 と同じ範囲で検査し、解決済みのしきい値を返す。
func validateAllocGoal(in SPAllocInput) (float64, error) {
	g := in.Goal
	threshold, err := validateAdjustSearch(g.searchInput(Individual{}, Individual{}))
	if err != nil {
		return 0, err
	}
	if in.Mode == AllocModeOffense && g.Move.Category != in.OffenseCategory {
		return 0, fmt.Errorf("%w: 目標の技の分類 %q が攻撃側の分類 %q と違う", ErrInvalidAdjustInput, g.Move.Category, in.OffenseCategory)
	}
	if err := g.Opponent.Validate(); err != nil {
		return 0, fmt.Errorf("%w: 相手: %v", ErrInvalidAdjustInput, err)
	}
	return threshold, nil
}

// searchInput は AJ2 の検査・ダメージ計算の部品に渡す入力を作る。
func (g *AllocGoal) searchInput(attacker, defender Individual) AdjustSearchInput {
	return AdjustSearchInput{
		Format: g.Format, Attacker: attacker, Defender: defender, Move: g.Move,
		Field: g.Field, Critical: g.Critical, TypeChart: g.TypeChart,
		Hits: g.Hits, ThresholdPercent: g.ThresholdPercent,
	}
}

// allocCand は候補1つの評価。
type allocCand struct {
	sp         Stats
	total      int
	real       Stats
	phys, spec int
	speedMet   bool
	goalMet    bool
	// goalCount は目標の事象(耐久側は耐える、攻撃側は倒す)の組数(ロール数^Hits 通り中)。
	// 確率の浮動小数の誤差で同点が崩れないよう、満たせないときの順は組数で比べる。
	goalCount int64
	chance    float64
}

func (c *allocCand) plan() AllocPlan {
	return AllocPlan{
		SP: c.sp, TotalSP: c.total, Real: c.real, PhysicalBulk: c.phys, SpecialBulk: c.spec,
		SpeedMet: c.speedMet, GoalMet: c.goalMet, ChancePercent: c.chance,
	}
}

// allocSearch は候補の総当たりの状態。
type allocSearch struct {
	in        SPAllocInput
	threshold float64
	// damage は Goal の CalcDamage の結果のキャッシュ。耐久側は (H 実数値, 技の分類の B/D 実数値) で決まる。
	// 攻撃側のダメージは攻撃実数値だけで決まり S によらないが、キーには (攻撃実数値, S 実数値) を使う
	// (保守的な選択。S 違いの重複計算が増えるだけで結果は変わらない。ADR-0150 §8 の計算量)。
	damage      map[[2]int]DamageResult
	calculated  bool
	unsupported []UnsupportedMark

	maxSpeedMet, maxAny       *allocCand
	minSatisfied, minFallback *allocCand
}

// enumerate は回す能力を各 [下限, 上限] で総当たりし(合計 ≤ MaxSPTotal)、各候補を評価する。
func (s *allocSearch) enumerate(keys []StatKey, i int, sp Stats) error {
	if i == len(keys) {
		return s.visit(sp)
	}
	k := keys[i]
	for v := s.in.Self.SP.Get(k); v <= s.in.Ceiling.Get(k); v++ {
		next := sp.WithStat(k, v)
		if next.Sum() > MaxSPTotal {
			break
		}
		if err := s.enumerate(keys, i+1, next); err != nil {
			return err
		}
	}
	return nil
}

// visit は候補1つを評価し、各基準の最良と比べる。
func (s *allocSearch) visit(sp Stats) error {
	self := s.in.Self
	self.SP = sp
	c := &allocCand{sp: sp, total: sp.Sum(), real: RealStats(self)}
	var err error
	if c.phys, err = BulkIndex(self, CategoryPhysical, Modifier4096); err != nil {
		return err
	}
	if c.spec, err = BulkIndex(self, CategorySpecial, Modifier4096); err != nil {
		return err
	}
	c.speedMet = s.in.Mode == AllocModeBulk || c.real.Spe >= s.in.MinSpeed
	if s.in.Goal != nil {
		if err := s.evalGoal(self, c); err != nil {
			return err
		}
	}

	if c.speedMet && (s.maxSpeedMet == nil || allocBefore(s.maxIndexKey(c, true), s.maxIndexKey(s.maxSpeedMet, true), c, s.maxSpeedMet)) {
		s.maxSpeedMet = c
	}
	if s.maxAny == nil || allocBefore(s.maxIndexKey(c, false), s.maxIndexKey(s.maxAny, false), c, s.maxAny) {
		s.maxAny = c
	}
	if s.in.Goal != nil {
		if c.goalMet && c.speedMet && (s.minSatisfied == nil || allocBefore(s.minSPKey(c), s.minSPKey(s.minSatisfied), c, s.minSatisfied)) {
			s.minSatisfied = c
		}
		if s.minFallback == nil || allocBefore(s.fallbackKey(c), s.fallbackKey(s.minFallback), c, s.minFallback) {
			s.minFallback = c
		}
	}
	return nil
}

// evalGoal は Goal の判定を c に入れる。判定は §7(AJ2)と同じ部品を使う。
func (s *allocSearch) evalGoal(self Individual, c *allocCand) error {
	g := s.in.Goal
	attacker, defender := g.Opponent, self
	key := [2]int{c.real.HP, c.real.Get(reverseStat(SideDefender, g.Move.Category))}
	if s.in.Mode == AllocModeOffense {
		attacker, defender = self, g.Opponent
		key = [2]int{c.real.Get(reverseStat(SideAttacker, g.Move.Category)), c.real.Spe}
	}
	res, ok := s.damage[key]
	if !ok {
		var err error
		if res, err = calcAdjustDamage(g.searchInput(attacker, defender), attacker, defender); err != nil {
			return err
		}
		s.damage[key] = res
	}
	if !s.calculated {
		// 印は SP によらず同じなので、最初の計算の印を使う。
		s.calculated = true
		if len(res.Unsupported) > 0 {
			s.unsupported = res.Unsupported
		}
	}
	ko := koChancePercent(res, g.Hits)
	if s.in.Mode == AllocModeOffense {
		c.chance = ko
		c.goalMet = meetsKOThreshold(res, g.Hits, ko, s.threshold)
	} else {
		c.chance = certainPercent - ko
		c.goalMet = meetsSurviveThreshold(res, g.Hits, c.chance, s.threshold)
	}
	c.goalCount = goalEventCount(c.chance, len(res.Rolls), g.Hits)
	return nil
}

// goalEventCount は確率(%)を「ロール数^hits 通り中の組数」に戻す。
// 組数は 16^MaxAdjustHits = 2^40 以下で float64 に正確に載り、確率の誤差は 0.5 組より十分小さい。
func goalEventCount(chancePercent float64, rolls, hits int) int64 {
	total := int64(1)
	for range hits {
		total *= int64(rolls)
	}
	return int64(math.Round(chancePercent / certainPercent * float64(total)))
}

// offense は攻撃側で比べる攻撃実数値(物理 → A、特殊 → C)。
func (s *allocSearch) offense(c *allocCand) int {
	return c.real.Get(reverseStat(SideAttacker, s.in.OffenseCategory))
}

// maxIndexKey は指数最大の順の先頭のキー(大きい方が前)。続きは allocBefore の合計 SP → 辞書順。
// speedFeasible は素早さ目標を満たす候補の中で比べるか(攻撃側だけ並べ方が変わる)。
func (s *allocSearch) maxIndexKey(c *allocCand, speedFeasible bool) []int64 {
	switch {
	case s.in.Mode == AllocModeBulk && s.in.Focus == BulkFocusPhysical:
		return []int64{int64(c.phys), -int64(c.total)}
	case s.in.Mode == AllocModeBulk && s.in.Focus == BulkFocusSpecial:
		return []int64{int64(c.spec), -int64(c.total)}
	case s.in.Mode == AllocModeBulk:
		return []int64{int64(min(c.phys, c.spec)), int64(max(c.phys, c.spec)), -int64(c.total)}
	case speedFeasible:
		return []int64{int64(s.offense(c)), int64(c.real.Spe), -int64(c.total)}
	}
	return []int64{int64(c.real.Spe), int64(s.offense(c)), -int64(c.total)}
}

// minSPKey は最小 SP の順の先頭のキー(合計 SP 小 → 指数 大)。
func (s *allocSearch) minSPKey(c *allocCand) []int64 {
	index := s.offense(c)
	if s.in.Mode == AllocModeBulk {
		index = c.phys
		if s.in.Goal.Move.Category == CategorySpecial {
			index = c.spec
		}
	}
	return []int64{-int64(c.total), int64(index)}
}

// fallbackKey は目標を満たす組が無いときの順の先頭のキー(ADR-0150 §8)。
// 耐久側: 耐える確率 大 → 最小 SP の順。攻撃側: min(S, MinSpeed) 大 → 倒す確率 大 → 最小 SP の順。
func (s *allocSearch) fallbackKey(c *allocCand) []int64 {
	head := []int64{c.goalCount}
	if s.in.Mode == AllocModeOffense {
		head = []int64{int64(min(c.real.Spe, s.in.MinSpeed)), c.goalCount}
	}
	return append(head, s.minSPKey(c)...)
}

// allocBefore は a が b より前かを返す。キー(大きい方が前)で決まらなければ §6 の辞書順
// (能力の固定順 H, A, B, C, D, S で SP の列が小さい方)で締める。
func allocBefore(keyA, keyB []int64, a, b *allocCand) bool {
	for i := range keyA {
		if keyA[i] != keyB[i] {
			return keyA[i] > keyB[i]
		}
	}
	for _, k := range AllStatKeys() {
		if va, vb := a.sp.Get(k), b.sp.Get(k); va != vb {
			return va < vb
		}
	}
	return false
}
