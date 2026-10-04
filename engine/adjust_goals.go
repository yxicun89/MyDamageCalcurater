package engine

import (
	"fmt"
	"math"
	"slices"
)

// 複数の目標(素早さを上回る・耐える・倒す)をすべて満たす最小の振り方(F-11 段階 B)。定義は ADR-0177。
//
// 自分の SP の下限(Self.SP)と上限(Ceiling)の中で、目標をすべて満たす組のうち合計 SP が最小の組を返す。
// すべては満たせないときはエラーにせず Feasible=false と最も近い組を返す(ADR-0177 §5 の1本の順)。
// 目標ごとに使う能力は重ならない(outspeed = S、ko = A/C、survive = H と B/D)。探索は §6 の分解で行い、
// 結果は §5 の候補の総当たりと一致させる。ダメージは CalcDamage の合成だけで求め、素早さは RealStats・
// EffectiveStat と同じ式(applyStatStage)だけで求める。新しい式は持たない。

// MaxSPGoals は目標の件数の上限(契約 AdjustGoalsRequest.goals の maxItems と同じ。ADR-0331 §4)。
const MaxSPGoals = 6

// SPGoalKind は目標の種類(値は契約 AdjustGoalKind と同じ)。
type SPGoalKind string

const (
	// SPGoalOutspeed は自分の素早さが相手の素早さを上回る(同速は満たさない)。
	SPGoalOutspeed SPGoalKind = "outspeed"
	// SPGoalSurvive は相手の技を Hits 発受けて耐える。
	SPGoalSurvive SPGoalKind = "survive"
	// SPGoalKO は自分の技で相手を Hits 発で倒す。
	SPGoalKO SPGoalKind = "ko"
)

// SPGoal は目標1件(ADR-0177 §1・§2)。
type SPGoal struct {
	Kind     SPGoalKind
	Opponent Individual
	// Move は survive では相手の技、ko では自分の技。outspeed では読まない。
	Move Move
	// Hits は survive・ko の発数(1..MaxAdjustHits)。outspeed では読まない。
	Hits int
	// ThresholdPercent は survive・ko のしきい値(%)。0 は DefaultAdjustThresholdPercent。outspeed では読まない。
	ThresholdPercent float64
	// SelfSpeedStage は outspeed で先に使う技の自分の素早さのランク変化(-6..6。GuaranteedSelfSpeedStage で求める)。
	// survive・ko では 0 でなければならない。
	SelfSpeedStage int
}

// SPGoalsInput は SuggestSPForGoals の入力(ADR-0177 §1)。
type SPGoalsInput struct {
	Format Format
	// Self は自分の個体。Self.SP は各能力の下限。
	Self Individual
	// Ceiling は探索する能力の上限(下限 ≤ 上限 ≤ MaxSPPerStat)。0 は「下限より上には振らない」。探索しない能力の値は見ない。
	Ceiling Stats
	// Field は survive・ko の CalcDamage に素通しする場。
	Field Field
	// TypeChart はタイプ相性表(ADR-0013)。解釈せず CalcDamage へ素通しする。outspeed だけなら空でよい。
	TypeChart TypeChart
	// Goals は目標(1..MaxSPGoals)。順に意味がある(満たせないときは前の目標を優先する。ADR-0177 §5)。
	Goals []SPGoal
}

// SPGoalsPlan は提案する SP の組。
type SPGoalsPlan struct {
	// SP は全6能力の SP(探索しない能力は下限のまま)。
	SP Stats
	// TotalSP は SP の合計。
	TotalSP int
	// Real は RealStats(ランク補正なし)。
	Real Stats
}

// SPGoalOutcome は目標1件の結果(Goals と同じ順)。
type SPGoalOutcome struct {
	Kind SPGoalKind
	// Met は Plan でこの目標を満たすか。
	Met bool
	// ChancePercent は survive では耐える確率、ko では倒す確率(%)。outspeed では 0。
	ChancePercent float64
	// SelfSpeed は outspeed で比べた自分の素早さ(Plan の S 実数値に SelfSpeedRank を掛けた値)。他の種類では 0。
	SelfSpeed int
	// OpponentSpeed は outspeed で比べた相手の素早さ(EffectiveStat(Opponent, StatSpe))。他の種類では 0。
	OpponentSpeed int
	// SelfSpeedRank は outspeed で自分に掛けたランク = clamp(Self.Ranks.Spe + SelfSpeedStage, -6, 6)。他の種類では 0。
	SelfSpeedRank int
}

// SPGoalsResult は SuggestSPForGoals の結果。
type SPGoalsResult struct {
	// Feasible はすべての目標を満たすか。
	Feasible bool
	// Remaining は MaxSPTotal − Plan.TotalSP。
	Remaining int
	Plan      SPGoalsPlan
	Goals     []SPGoalOutcome
	// Unsupported は survive・ko の CalcDamage の印を目標の順に連結し、同じ印を除いたもの(ADR-0177 §7)。印なしは nil。
	Unsupported []UnsupportedMark
}

// goalSaturated は満たした目標の「近さ」(ADR-0177 §5 の2つめのキー)。満たした目標はそれ以上区別しない。
const goalSaturated = math.MaxInt64

// SuggestSPForGoals は目標をすべて満たす最小の振り方を返す(ADR-0177)。
// 入力が不正なら ErrInvalidAdjustInput を包んで返す。相性表の誤りは CalcDamage のエラーを包んで返す。
func SuggestSPForGoals(in SPGoalsInput) (SPGoalsResult, error) {
	s, err := newGoalsSearch(in)
	if err != nil {
		return SPGoalsResult{}, err
	}
	unsupported, err := s.unsupportedMarks()
	if err != nil {
		return SPGoalsResult{}, err
	}
	if err := s.prepareOffense(); err != nil {
		return SPGoalsResult{}, err
	}
	table, err := s.bulkTable()
	if err != nil {
		return SPGoalsResult{}, err
	}
	best := s.pickBest(table)
	return s.result(best, unsupported), nil
}

// GuaranteedSelfSpeedStage は技の追加効果のうち「確率 100% で必ず起きる、使用者自身の素早さのランク変化」の段数を返す
// (ADR-0177 §4・ADR-0107)。効果が無い・確率 100% 未満・対象が相手・素早さを含まないときは 0。
func GuaranteedSelfSpeedStage(m Move) int {
	e := m.Effect
	if e == nil || e.Chance != 100 || e.Target != RankTargetSelf {
		return 0
	}
	return e.Stages[StatSpe]
}

// offenseStatKeys は素早さ・攻撃側(耐久側の表を引く側)の能力(ADR-0177 §6)。
var offenseStatKeys = [3]StatKey{StatSpe, StatAtk, StatSpA}

// goalEval は1つの組での目標1件の判定。
type goalEval struct {
	met bool
	// progress は §5 の「近さ」。満たせば goalSaturated、満たさない outspeed は自分の素早さ、survive・ko は事象の組数。
	progress int64
	outcome  SPGoalOutcome
}

// goalsBulkPart は耐久側(H・B・D)の組1つと、その組で決まる目標(survive)の判定。
type goalsBulkPart struct {
	sp    Stats // H・B・D 以外は下限
	sum   int   // H + B + D
	met   int
	evals []goalEval // survive の目標だけ有効(目標の順)
	bulk  []int64
	key   []int64
}

// goalsSearch は探索の状態。
type goalsSearch struct {
	in         SPGoalsInput
	thresholds []float64
	searched   map[StatKey]bool
	// speedEvals・offenseEvals は outspeed・ko の目標ごとに、S・A/C の SP(0..MaxSPPerStat)で引く判定。
	speedEvals   map[int]*[MaxSPPerStat + 1]goalEval
	offenseEvals map[int]*[MaxSPPerStat + 1]goalEval
	// damage は survive・ko の CalcDamage の結果の使い回し。キーは (目標の番号, 実数値の組)(ADR-0177 §6)。
	damage map[[3]int]DamageResult
}

func newGoalsSearch(in SPGoalsInput) (*goalsSearch, error) {
	if len(in.Goals) == 0 || len(in.Goals) > MaxSPGoals {
		return nil, fmt.Errorf("%w: 目標の件数は 1..%d の範囲外: %d", ErrInvalidAdjustInput, MaxSPGoals, len(in.Goals))
	}
	if err := in.Self.Validate(); err != nil {
		return nil, fmt.Errorf("%w: 自分: %v", ErrInvalidAdjustInput, err)
	}
	s := &goalsSearch{
		in: in, thresholds: make([]float64, len(in.Goals)), searched: map[StatKey]bool{},
		speedEvals: map[int]*[MaxSPPerStat + 1]goalEval{}, offenseEvals: map[int]*[MaxSPPerStat + 1]goalEval{},
		damage: map[[3]int]DamageResult{},
	}
	for i, g := range in.Goals {
		if err := g.Opponent.Validate(); err != nil {
			return nil, fmt.Errorf("%w: 目標 %d の相手: %v", ErrInvalidAdjustInput, i, err)
		}
		switch g.Kind {
		case SPGoalOutspeed:
			if g.SelfSpeedStage < -6 || g.SelfSpeedStage > 6 {
				return nil, fmt.Errorf("%w: 目標 %d の素早さの段数は -6..6 の範囲外: %d", ErrInvalidAdjustInput, i, g.SelfSpeedStage)
			}
			s.searched[StatSpe] = true
		case SPGoalSurvive, SPGoalKO:
			threshold, err := validateAdjustSearch(AdjustSearchInput{Move: g.Move, Hits: g.Hits, ThresholdPercent: g.ThresholdPercent})
			if err != nil {
				return nil, fmt.Errorf("目標 %d: %w", i, err)
			}
			if g.SelfSpeedStage != 0 {
				return nil, fmt.Errorf("%w: 目標 %d(%s)に素早さの段数がある: %d", ErrInvalidAdjustInput, i, g.Kind, g.SelfSpeedStage)
			}
			s.thresholds[i] = threshold
			if g.Kind == SPGoalKO {
				s.searched[reverseStat(SideAttacker, g.Move.Category)] = true
			} else {
				s.searched[StatHP] = true
				s.searched[reverseStat(SideDefender, g.Move.Category)] = true
			}
		default:
			return nil, fmt.Errorf("%w: 目標 %d の種類が未知: %q", ErrInvalidAdjustInput, i, g.Kind)
		}
	}
	for _, k := range AllStatKeys() {
		if !s.searched[k] {
			continue
		}
		if c := in.Ceiling.Get(k); c < in.Self.SP.Get(k) || c > MaxSPPerStat {
			return nil, fmt.Errorf("%w: %s の上限 %d は下限 %d..%d の範囲外", ErrInvalidAdjustInput, k, c, in.Self.SP.Get(k), MaxSPPerStat)
		}
	}
	return s, nil
}

// statRange は能力 k の探索範囲。探索しない能力は下限だけ。
func (s *goalsSearch) statRange(k StatKey) (lo, hi int) {
	lo = s.in.Self.SP.Get(k)
	if s.searched[k] {
		return lo, s.in.Ceiling.Get(k)
	}
	return lo, lo
}

// minSum は能力の組 keys の下限の合計。
func (s *goalsSearch) minSum(keys [3]StatKey) int {
	sum := 0
	for _, k := range keys {
		sum += s.in.Self.SP.Get(k)
	}
	return sum
}

// selfWith は SP を sp にした自分。
func (s *goalsSearch) selfWith(sp Stats) Individual {
	self := s.in.Self
	self.SP = sp
	return self
}

// unsupportedMarks は survive・ko の下限の組での印を目標の順に連結し、同じ印を除く(ADR-0177 §7)。
func (s *goalsSearch) unsupportedMarks() ([]UnsupportedMark, error) {
	var out []UnsupportedMark
	for i, g := range s.in.Goals {
		if g.Kind == SPGoalOutspeed {
			continue
		}
		res, err := s.calc(i, s.in.Self, nil)
		if err != nil {
			return nil, err
		}
		for _, m := range res.Unsupported {
			if !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out, nil
}

// calc は目標 i の CalcDamage を求める。cacheKey があれば (目標, 実数値の組) で使い回す。
func (s *goalsSearch) calc(i int, self Individual, cacheKey *[2]int) (DamageResult, error) {
	var key [3]int
	if cacheKey != nil {
		key = [3]int{i, cacheKey[0], cacheKey[1]}
		if res, ok := s.damage[key]; ok {
			return res, nil
		}
	}
	g := s.in.Goals[i]
	attacker, defender := g.Opponent, self
	if g.Kind == SPGoalKO {
		attacker, defender = self, g.Opponent
	}
	res, err := CalcDamage(DamageInput{
		Format: s.in.Format, Attacker: attacker, Defender: defender, Move: g.Move,
		Field: s.in.Field, TypeChart: s.in.TypeChart,
	})
	if err != nil {
		return DamageResult{}, fmt.Errorf("目標の探索のダメージ計算: %w", err)
	}
	if cacheKey != nil {
		s.damage[key] = res
	}
	return res, nil
}

// damageEval は survive・ko の目標 i を self で判定する。満たすかは組数で決める(確率の小数の誤差で境界を崩さない)。
// しきい値 100 は乱数の最悪側の整数比較(ADR-0150 §7)。
func (s *goalsSearch) damageEval(i int, self Individual, cacheKey [2]int) (goalEval, error) {
	g := s.in.Goals[i]
	res, err := s.calc(i, self, &cacheKey)
	if err != nil {
		return goalEval{}, err
	}
	chance := koChancePercent(res, g.Hits)
	if g.Kind == SPGoalSurvive {
		chance = certainPercent - chance
	}
	count := goalEventCount(chance, len(res.Rolls), g.Hits)
	threshold := s.thresholds[i]
	var met bool
	switch {
	case threshold == certainPercent && g.Kind == SPGoalKO:
		met = meetsKOThreshold(res, g.Hits, chance, threshold)
	case threshold == certainPercent:
		met = meetsSurviveThreshold(res, g.Hits, chance, threshold)
	default:
		met = float64(count)*certainPercent/float64(goalEventTotal(len(res.Rolls), g.Hits)) >= threshold
	}
	e := goalEval{met: met, progress: count, outcome: SPGoalOutcome{Kind: g.Kind, Met: met, ChancePercent: chance}}
	if met {
		e.progress = goalSaturated
	}
	return e, nil
}

// goalEventTotal は事象の全組数(ロール数^hits)。
func goalEventTotal(rolls, hits int) int64 {
	total := int64(1)
	for range hits {
		total *= int64(rolls)
	}
	return total
}

// prepareOffense は outspeed・ko の判定を、使う能力(S・A/C)の SP ごとに求める。
func (s *goalsSearch) prepareOffense() error {
	for i, g := range s.in.Goals {
		switch g.Kind {
		case SPGoalOutspeed:
			rank := min(6, max(-6, s.in.Self.Ranks.Spe+g.SelfSpeedStage))
			opponent := EffectiveStat(g.Opponent, StatSpe)
			table := &[MaxSPPerStat + 1]goalEval{}
			lo, hi := s.statRange(StatSpe)
			for v := lo; v <= hi; v++ {
				self := s.selfWith(s.in.Self.SP.WithStat(StatSpe, v))
				self.Ranks.Spe = rank
				speed := EffectiveStat(self, StatSpe)
				e := goalEval{met: speed > opponent, progress: int64(speed), outcome: SPGoalOutcome{
					Kind: g.Kind, Met: speed > opponent, SelfSpeed: speed, OpponentSpeed: opponent, SelfSpeedRank: rank,
				}}
				if e.met {
					e.progress = goalSaturated
				}
				table[v] = e
			}
			s.speedEvals[i] = table
		case SPGoalKO:
			k := reverseStat(SideAttacker, g.Move.Category)
			table := &[MaxSPPerStat + 1]goalEval{}
			lo, hi := s.statRange(k)
			for v := lo; v <= hi; v++ {
				self := s.selfWith(s.in.Self.SP.WithStat(k, v))
				e, err := s.damageEval(i, self, [2]int{RealStats(self).Get(k), 0})
				if err != nil {
					return err
				}
				table[v] = e
			}
			s.offenseEvals[i] = table
		}
	}
	return nil
}

// bulkTable は耐久側(H・B・D)を総当たりし、合計 r(0..MaxSPTotal)以下で最良の組を累積で返す(ADR-0177 §6 の1)。
// 比べるキーは §5 を耐久側に絞ったもの: 満たす survive の数 → survive の近さ(順)→ 合計 小 → 耐久指数 → H, B, D の辞書順。
func (s *goalsSearch) bulkTable() ([]*goalsBulkPart, error) {
	n := len(s.in.Goals)
	budget := MaxSPTotal - s.minSum(offenseStatKeys)
	best := make([]*goalsBulkPart, MaxSPTotal+1)
	hasPhys, hasSpec := false, false
	for _, g := range s.in.Goals {
		if g.Kind == SPGoalSurvive {
			hasPhys = hasPhys || g.Move.Category == CategoryPhysical
			hasSpec = hasSpec || g.Move.Category != CategoryPhysical
		}
	}
	evals := make([]goalEval, n)
	var key []int64
	hLo, hHi := s.statRange(StatHP)
	bLo, bHi := s.statRange(StatDef)
	dLo, dHi := s.statRange(StatSpD)
	for h := hLo; h <= hHi; h++ {
		for b := bLo; b <= bHi; b++ {
			for d := dLo; d <= dHi; d++ {
				sum := h + b + d
				if sum > budget {
					break
				}
				sp := s.in.Self.SP.WithStat(StatHP, h).WithStat(StatDef, b).WithStat(StatSpD, d)
				self := s.selfWith(sp)
				real := RealStats(self)
				met := 0
				key = key[:0]
				key = append(key, 0)
				for i, g := range s.in.Goals {
					if g.Kind != SPGoalSurvive {
						continue
					}
					e, err := s.damageEval(i, self, [2]int{real.HP, real.Get(reverseStat(SideDefender, g.Move.Category))})
					if err != nil {
						return nil, err
					}
					evals[i] = e
					if e.met {
						met++
					}
					key = append(key, e.progress)
				}
				key[0] = int64(met)
				key = append(key, -int64(sum))
				bulkStart := len(key)
				phys, spec := int64(real.HP*real.Def), int64(real.HP*real.SpD)
				switch {
				case hasPhys && hasSpec:
					key = append(key, min(phys, spec), max(phys, spec))
				case hasPhys:
					key = append(key, phys)
				case hasSpec:
					key = append(key, spec)
				}
				bulkEnd := len(key)
				key = append(key, -int64(h), -int64(b), -int64(d))
				if cur := best[sum]; cur == nil || goalsKeyGreater(key, cur.key) {
					best[sum] = &goalsBulkPart{
						sp: sp, sum: sum, met: met, evals: slices.Clone(evals),
						bulk: slices.Clone(key[bulkStart:bulkEnd]), key: slices.Clone(key),
					}
				}
			}
		}
	}
	for r := 1; r <= MaxSPTotal; r++ {
		if prev := best[r-1]; prev != nil && (best[r] == nil || goalsKeyGreater(prev.key, best[r].key)) {
			best[r] = prev
		}
	}
	return best, nil
}

// goalsChoice は選んだ組(耐久側の最良と S・A・C)。
type goalsChoice struct {
	bulk          *goalsBulkPart
	spe, atk, spa int
}

// pickBest は S・A・C を総当たりし、残りの予算での耐久側の最良と組み合わせて §5 の全キーで比べる(ADR-0177 §6 の2)。
func (s *goalsSearch) pickBest(table []*goalsBulkPart) goalsChoice {
	var best goalsChoice
	var bestKey, key []int64
	sLo, sHi := s.statRange(StatSpe)
	aLo, aHi := s.statRange(StatAtk)
	cLo, cHi := s.statRange(StatSpA)
	for v := sLo; v <= sHi; v++ {
		for a := aLo; a <= aHi; a++ {
			for c := cLo; c <= cHi; c++ {
				budget := MaxSPTotal - (v + a + c)
				if budget < 0 {
					break
				}
				part := table[budget]
				if part == nil {
					continue
				}
				choice := goalsChoice{bulk: part, spe: v, atk: a, spa: c}
				key = s.fullKey(key[:0], choice)
				if bestKey == nil || goalsKeyGreater(key, bestKey) {
					bestKey = append(bestKey[:0], key...)
					best = choice
				}
			}
		}
	}
	return best
}

// offenseEval は S・A・C が決まったときの outspeed・ko の目標 i の判定。
func (s *goalsSearch) offenseEval(i int, ch goalsChoice) goalEval {
	g := s.in.Goals[i]
	if g.Kind == SPGoalOutspeed {
		return s.speedEvals[i][ch.spe]
	}
	if reverseStat(SideAttacker, g.Move.Category) == StatAtk {
		return s.offenseEvals[i][ch.atk]
	}
	return s.offenseEvals[i][ch.spa]
}

// evalOf は選んだ組での目標 i の判定。
func (s *goalsSearch) evalOf(i int, ch goalsChoice) goalEval {
	if s.in.Goals[i].Kind == SPGoalSurvive {
		return ch.bulk.evals[i]
	}
	return s.offenseEval(i, ch)
}

// fullKey は §5 の全キー(大きい方が前): 満たす数 → 近さ(目標の順)→ 合計 SP 小 → 耐久指数 → H, A, B, C, D, S の辞書順。
func (s *goalsSearch) fullKey(key []int64, ch goalsChoice) []int64 {
	met := ch.bulk.met
	for i, g := range s.in.Goals {
		if g.Kind != SPGoalSurvive && s.offenseEval(i, ch).met {
			met++
		}
	}
	key = append(key, int64(met))
	for i := range s.in.Goals {
		key = append(key, s.evalOf(i, ch).progress)
	}
	key = append(key, -int64(ch.bulk.sum+ch.spe+ch.atk+ch.spa))
	key = append(key, ch.bulk.bulk...)
	sp := ch.sp()
	for _, k := range AllStatKeys() {
		key = append(key, -int64(sp.Get(k)))
	}
	return key
}

// sp は選んだ組の全6能力の SP。
func (ch goalsChoice) sp() Stats {
	return ch.bulk.sp.WithStat(StatSpe, ch.spe).WithStat(StatAtk, ch.atk).WithStat(StatSpA, ch.spa)
}

// result は選んだ組を結果に写す。
func (s *goalsSearch) result(ch goalsChoice, unsupported []UnsupportedMark) SPGoalsResult {
	sp := ch.sp()
	total := sp.Sum()
	out := SPGoalsResult{
		Feasible: true, Remaining: MaxSPTotal - total,
		Plan:        SPGoalsPlan{SP: sp, TotalSP: total, Real: RealStats(s.selfWith(sp))},
		Goals:       make([]SPGoalOutcome, len(s.in.Goals)),
		Unsupported: unsupported,
	}
	for i := range s.in.Goals {
		e := s.evalOf(i, ch)
		out.Goals[i] = e.outcome
		out.Feasible = out.Feasible && e.met
	}
	return out
}

// goalsKeyGreater は a が b より前(辞書式に大きい)かを返す。
func goalsKeyGreater(a, b []int64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
