package engine

import (
	"fmt"
)

// 倒せる/耐える最小 SP の探索(plan.md AJ2・機能 4)。定義は ADR-0800 §7。
//
//   - MinSPToKO: 自分(Attacker)の A(物理)/ C(特殊)の SP を 0 から探し、相手(Defender)を
//     Hits 発で倒す確率が ThresholdPercent 以上になる最小の SP を返す。
//   - MinSPToSurvive: 自分(Defender)の H と B(物理)/ D(特殊)の SP の組を探し、相手(Attacker)の
//     技を Hits 発受けて耐える確率が ThresholdPercent 以上になる、合計 SP 最小の組を返す。
//
// ダメージは CalcDamage の合成だけで求め、独自の式を書かない(ADR-0010 §1 と同じ)。
// 探索する能力以外の SP・性格・持ち物・ランクは呼び出し側が固定で渡す。
// 到達できないときはエラーにせず Feasible=false を返す。

// MaxAdjustHits は探索の目標発数 Hits の上限(ADR-0800 §7)。
// 確定数の表示で実用になる範囲を覆い、n 発の確率計算(O(Hits × HP × 16))と
// 耐久側の総当たり(最大 33² 通り)の積を WASM の同期実行に収めるための上限。
const MaxAdjustHits = 10

// DefaultAdjustThresholdPercent は ThresholdPercent が 0(未指定)のときに使うしきい値(%)。
// 100 = 確定(乱数の最悪側でも満たす)。ADR-0800 §7。
const DefaultAdjustThresholdPercent = 100.0

// certainPercent は「確定」を表す確率(%)。この値のしきい値は浮動小数の確率ではなく、
// 乱数の最悪側の整数比較で判定する(ADR-0800 §7)。
const certainPercent = 100.0

// AdjustSearchInput は最小 SP の探索の入力(ADR-0800 §7)。場・技・相性表は CalcDamage と同じ。
type AdjustSearchInput struct {
	Format Format
	// Attacker は攻撃側。MinSPToKO では自分(探索する側)、MinSPToSurvive では相手(固定)。
	Attacker Individual
	// Defender は防御側。MinSPToKO では相手(固定)、MinSPToSurvive では自分(探索する側)。
	Defender Individual
	Move     Move
	Field    Field
	Critical bool
	// TypeChart はタイプ相性表(ADR-0013)。解釈せず CalcDamage へ素通しする。
	TypeChart TypeChart
	// Hits は目標の発数 n(1..MaxAdjustHits)。MinSPToKO は「n 発で倒す」、MinSPToSurvive は「n 発耐える」。
	Hits int
	// ThresholdPercent は満たすべき確率(%)。0 は DefaultAdjustThresholdPercent(100 = 確定)。
	// それ以外は (0, 100] の範囲。比較は「確率 >= しきい値」(ちょうど等しいときは満たす)。
	ThresholdPercent float64
}

// KOSearchResult は MinSPToKO の結果。
type KOSearchResult struct {
	// Stat は探索した能力(物理 → StatAtk、特殊 → StatSpA)。
	Stat StatKey
	// SearchLimit は探索した SP の上限 = min(MaxSPPerStat, MaxSPTotal - 固定 SP の合計)。
	SearchLimit int
	// Feasible は SearchLimit 以内でしきい値を満たす SP があるか。
	Feasible bool
	// SP は Feasible のとき、しきい値を満たす最小の SP。Feasible=false のときは SearchLimit。
	SP int
	// ChancePercent は SP のときに Hits 発で倒せる確率(%)。
	// Feasible=false のときは到達できる最大の確率(SearchLimit での値)。
	ChancePercent float64
	// Unsupported は探索中の計算に付いた「未対応」の印(ADR-0123)。SP によらず同じ(技・場・両側の個体で決まる)。
	// nil は印なし。ReverseCandidate.Unsupported と同じ扱い。
	Unsupported []UnsupportedMark
}

// SurviveSearchResult は MinSPToSurvive の結果。
type SurviveSearchResult struct {
	// Stat は H と組にして探索した能力(物理 → StatDef、特殊 → StatSpD)。
	Stat StatKey
	// SearchLimit は HPSP + StatSP の上限 = MaxSPTotal - 固定 SP の合計(各能力は別に MaxSPPerStat まで)。
	SearchLimit int
	// Feasible は SearchLimit 以内でしきい値を満たす組があるか。
	Feasible bool
	// HPSP・StatSP は選んだ組。Feasible のときは「合計 SP 最小 → 耐久指数最大 → HPSP が小さい方」で選ぶ。
	// Feasible=false のときは耐える確率が最大の組を、同じ順で選ぶ。
	HPSP   int
	StatSP int
	// TotalSP は HPSP + StatSP。
	TotalSP int
	// BulkIndex は選んだ組の耐久指数(ADR-0800 §3。被ダメージ補正は等倍 = H 実数値 × B(D) 実数値)。
	BulkIndex int
	// ChancePercent は選んだ組で Hits 発を受けて耐える確率(%)。
	ChancePercent float64
	// Unsupported は探索中の計算に付いた「未対応」の印(ADR-0123)。SP によらず同じ(技・場・両側の個体で決まる)。
	// nil は印なし。ReverseCandidate.Unsupported と同じ扱い。
	Unsupported []UnsupportedMark
}

// MinSPToKO は Attacker の A/C の SP を探索し、Defender を Hits 発で倒す確率が
// ThresholdPercent 以上になる最小の SP を返す(ADR-0800 §7)。
// Attacker.SP の探索する能力の値は無視する(上書きする)。
// 入力が不正なら ErrInvalidAdjustInput を包んで返す。相性表の誤りは CalcDamage のエラーを包んで返す。
func MinSPToKO(in AdjustSearchInput) (KOSearchResult, error) {
	threshold, err := validateAdjustSearch(in)
	if err != nil {
		return KOSearchResult{}, err
	}
	stat := reverseStat(SideAttacker, in.Move.Category)
	attacker := in.Attacker
	attacker.SP = attacker.SP.WithStat(stat, 0)
	if err := validateAdjustIndividuals(attacker, in.Defender); err != nil {
		return KOSearchResult{}, err
	}
	limit := min(MaxSPPerStat, MaxSPTotal-attacker.SP.Sum())

	result := KOSearchResult{Stat: stat, SearchLimit: limit, SP: limit}
	for sp := 0; sp <= limit; sp++ {
		attacker.SP = attacker.SP.WithStat(stat, sp)
		res, err := calcAdjustDamage(in, attacker, in.Defender)
		if err != nil {
			return KOSearchResult{}, err
		}
		// 印は SP によらず同じなので、最初の計算の印を使う。
		if sp == 0 {
			result.Unsupported = res.Unsupported
		}
		chance := koChancePercent(res, in.Hits)
		if meetsKOThreshold(res, in.Hits, chance, threshold) {
			result.Feasible, result.SP, result.ChancePercent = true, sp, chance
			return result, nil
		}
		// SP を上げるほど確率は下がらないので、上限での値が到達できる最大の確率になる。
		result.ChancePercent = chance
	}
	return result, nil
}

// MinSPToSurvive は Defender の H と B/D の SP の組を探索し、Attacker の技を Hits 発受けて耐える確率が
// ThresholdPercent 以上になる、合計 SP 最小の組を返す(ADR-0800 §7)。
// Defender.SP の H と B/D の値は無視する(上書きする)。
// 入力が不正なら ErrInvalidAdjustInput を包んで返す。相性表の誤りは CalcDamage のエラーを包んで返す。
func MinSPToSurvive(in AdjustSearchInput) (SurviveSearchResult, error) {
	threshold, err := validateAdjustSearch(in)
	if err != nil {
		return SurviveSearchResult{}, err
	}
	stat := reverseStat(SideDefender, in.Move.Category)
	defender := in.Defender
	defender.SP = defender.SP.WithStat(StatHP, 0).WithStat(stat, 0)
	if err := validateAdjustIndividuals(in.Attacker, defender); err != nil {
		return SurviveSearchResult{}, err
	}
	limit := MaxSPTotal - defender.SP.Sum()

	var feasible, fallback *surviveCandidate
	var unsupported []UnsupportedMark
	for hpSP := 0; hpSP <= MaxSPPerStat && hpSP <= limit; hpSP++ {
		for statSP := 0; statSP <= MaxSPPerStat && hpSP+statSP <= limit; statSP++ {
			defender.SP = defender.SP.WithStat(StatHP, hpSP).WithStat(stat, statSP)
			res, err := calcAdjustDamage(in, in.Attacker, defender)
			if err != nil {
				return SurviveSearchResult{}, err
			}
			// 印は SP によらず同じなので、最初の計算の印を使う。
			if hpSP == 0 && statSP == 0 {
				unsupported = res.Unsupported
			}
			// 被ダメージ補正は全候補で共通なので等倍(Modifier4096)で並べる(ADR-0800 §7)。
			index, err := BulkIndex(defender, in.Move.Category, Modifier4096)
			if err != nil {
				return SurviveSearchResult{}, err
			}
			c := surviveCandidate{
				hpSP: hpSP, statSP: statSP, index: index,
				chance: certainPercent - koChancePercent(res, in.Hits),
			}
			if meetsSurviveThreshold(res, in.Hits, c.chance, threshold) {
				if feasible == nil || c.before(*feasible) {
					feasible = &c
				}
			}
			if fallback == nil || c.chance > fallback.chance || (c.chance == fallback.chance && c.before(*fallback)) {
				fallback = &c
			}
		}
	}

	best := fallback
	if feasible != nil {
		best = feasible
	}
	return SurviveSearchResult{
		Stat: stat, SearchLimit: limit, Feasible: feasible != nil,
		HPSP: best.hpSP, StatSP: best.statSP, TotalSP: best.hpSP + best.statSP,
		BulkIndex: best.index, ChancePercent: best.chance, Unsupported: unsupported,
	}, nil
}

// surviveCandidate は耐久側の探索の組1つ。
type surviveCandidate struct {
	hpSP, statSP, index int
	chance              float64
}

// before は ADR-0800 §6 の順(合計 SP が小さい → 耐久指数が大きい → HPSP が小さい)で c が o より前かを返す。
func (c surviveCandidate) before(o surviveCandidate) bool {
	if tc, to := c.hpSP+c.statSP, o.hpSP+o.statSP; tc != to {
		return tc < to
	}
	if c.index != o.index {
		return c.index > o.index
	}
	return c.hpSP < o.hpSP
}

// validateAdjustSearch は個体以外の入力(発数・しきい値・技)を検査し、解決済みのしきい値を返す。
func validateAdjustSearch(in AdjustSearchInput) (float64, error) {
	if in.Hits < 1 || in.Hits > MaxAdjustHits {
		return 0, fmt.Errorf("%w: 発数は 1..%d の範囲外: %d", ErrInvalidAdjustInput, MaxAdjustHits, in.Hits)
	}
	threshold := in.ThresholdPercent
	if threshold == 0 {
		threshold = DefaultAdjustThresholdPercent
	}
	// NaN・Inf・負・100 超をまとめて拒否する(NaN はどの比較も偽)。
	if !(threshold > 0 && threshold <= certainPercent) {
		return 0, fmt.Errorf("%w: しきい値は (0, %v] の範囲外: %v", ErrInvalidAdjustInput, certainPercent, in.ThresholdPercent)
	}
	if in.Move.Category != CategoryPhysical && in.Move.Category != CategorySpecial {
		return 0, fmt.Errorf("%w: 分類 %q はダメージを与えない", ErrInvalidAdjustInput, in.Move.Category)
	}
	if in.Move.Power <= 0 {
		return 0, fmt.Errorf("%w: 威力が 0 以下: %d", ErrInvalidAdjustInput, in.Move.Power)
	}
	return threshold, nil
}

// validateAdjustIndividuals は探索する能力の SP を 0 にした両側の個体を検査する。
func validateAdjustIndividuals(attacker, defender Individual) error {
	if err := attacker.Validate(); err != nil {
		return fmt.Errorf("%w: 攻撃側: %v", ErrInvalidAdjustInput, err)
	}
	if err := defender.Validate(); err != nil {
		return fmt.Errorf("%w: 防御側: %v", ErrInvalidAdjustInput, err)
	}
	return nil
}

// calcAdjustDamage は探索中の1組のダメージを CalcDamage で求める。
func calcAdjustDamage(in AdjustSearchInput, attacker, defender Individual) (DamageResult, error) {
	res, err := CalcDamage(DamageInput{
		Format: in.Format, Attacker: attacker, Defender: defender, Move: in.Move,
		Field: in.Field, Critical: in.Critical, TypeChart: in.TypeChart,
	})
	if err != nil {
		return DamageResult{}, fmt.Errorf("最小 SP の探索のダメージ計算: %w", err)
	}
	return res, nil
}

// koChancePercent は hits 発で倒す確率(%)。乱数の最悪側・最良側で決まるときは畳み込みを省く。
func koChancePercent(res DamageResult, hits int) float64 {
	switch {
	case res.MinDamage()*hits >= res.DefenderHP:
		return certainPercent
	case res.MaxDamage()*hits < res.DefenderHP:
		return 0
	}
	return koProbability(res.Rolls, res.DefenderHP, hits) * certainPercent
}

// meetsKOThreshold は倒す確率がしきい値以上か。確定(100)は最小ロールの整数比較で判定する。
func meetsKOThreshold(res DamageResult, hits int, chance, threshold float64) bool {
	if threshold == certainPercent {
		return res.MinDamage()*hits >= res.DefenderHP
	}
	return chance >= threshold
}

// meetsSurviveThreshold は耐える確率がしきい値以上か。確定(100)は最大ロールの整数比較で判定する。
func meetsSurviveThreshold(res DamageResult, hits int, chance, threshold float64) bool {
	if threshold == certainPercent {
		return res.MaxDamage()*hits < res.DefenderHP
	}
	return chance >= threshold
}
