package engine

import "errors"

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

// errSPGoalsNotImplemented は段階 B の実装までのスタブのエラー(ErrInvalidAdjustInput を包まない)。
var errSPGoalsNotImplemented = errors.New("SuggestSPForGoals は未実装(ADR-0177)")

// SuggestSPForGoals は目標をすべて満たす最小の振り方を返す(ADR-0177)。
// 入力が不正なら ErrInvalidAdjustInput を包んで返す。相性表の誤りは CalcDamage のエラーを包んで返す。
func SuggestSPForGoals(in SPGoalsInput) (SPGoalsResult, error) {
	return SPGoalsResult{}, errSPGoalsNotImplemented
}

// GuaranteedSelfSpeedStage は技の追加効果のうち「確率 100% で必ず起きる、使用者自身の素早さのランク変化」の段数を返す
// (ADR-0177 §4・ADR-0107)。効果が無い・確率 100% 未満・対象が相手・素早さを含まないときは 0。
func GuaranteedSelfSpeedStage(m Move) int {
	return 0
}
