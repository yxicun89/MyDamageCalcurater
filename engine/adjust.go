package engine

import (
	"errors"
	"fmt"
	"math"
)

// 調整(自分の SP を決める機能)の土台になる量。定義と根拠は ADR-0800。
//

// ErrInvalidAdjustInput は調整の入力(個体・技の分類・威力・補正・SP)が不正なときのエラー。
var ErrInvalidAdjustInput = errors.New("調整の入力が不正")

// HPLineDivisor は HP ラインの単位。天候・どく・たべのこし等の HP の 1/16 を単位とする効果に由来する
// プロトコル定数(ADR-0800 §4)。
const HPLineDivisor = 16

// maxIndexPower は火力指数に渡せる威力の上限。
// 実数値(最大でも 400 未満)× 威力 × 補正(最大 MaxEffectModifier = 2^21)が int64 に収まる範囲として
// int32 の上限を採る(400 × 2^31 × 2^21 ≈ 1.8e18 < 2^63 ≈ 9.2e18)。
const maxIndexPower = math.MaxInt32

// HPLineKind は HP 実数値が属するライン。
type HPLineKind string

const (
	// HPLineNone はどのラインにも属さない。
	HPLineNone HPLineKind = ""
	// HPLine16n は HP が 16 の倍数(HP mod 16 == 0)。
	HPLine16n HPLineKind = "16n"
	// HPLine16nMinus1 は HP が 16 の倍数 - 1(HP mod 16 == 15)。
	HPLine16nMinus1 HPLineKind = "16n-1"
)

// HPLinePoint は HP の SP を変えて届くライン上の点。
type HPLinePoint struct {
	HP      int // そのラインの HP 実数値
	SP      int // その HP にするための HP の SP(0..MaxSPPerStat)
	SPDelta int // 現在の SP からの差(次のラインは正、前のラインは負)
}

// HPLineReport は HP の SP 1つに対する 16n / 16n-1 ラインの報告(ADR-0800 §4)。
// Next* は SP が現在より大きい中で最小のライン、Prev* は SP が現在より小さい中で最大のライン。
// HP の SP の範囲 0..MaxSPPerStat に無ければ nil。
type HPLineReport struct {
	HP            int        // 現在の HP 実数値
	SP            int        // 現在の HP の SP
	Current       HPLineKind // 現在の HP が属するライン
	Next16n       *HPLinePoint
	Prev16n       *HPLinePoint
	Next16nMinus1 *HPLinePoint
	Prev16nMinus1 *HPLinePoint
}

// FirepowerIndex は火力指数 floor(攻撃実数値 × 威力 × modifier / 4096) を返す(ADR-0800 §2)。
// 攻撃実数値は category が physical なら A、special なら C の RealStats(ランク補正は含めない)。
// modifier はタイプ一致・持ち物などを呼び出し側が掛け合わせた 4096 基準の補正。
// 個体・分類・威力・補正が不正なら ErrInvalidAdjustInput を包んで返す。
func FirepowerIndex(attacker Individual, category MoveCategory, power, modifier int) (int, error) {
	if err := attacker.Validate(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidAdjustInput, err)
	}
	if power < 1 || power > maxIndexPower {
		return 0, fmt.Errorf("%w: 威力は 1..%d の範囲外: %d", ErrInvalidAdjustInput, maxIndexPower, power)
	}
	if err := validateModifier("補正", modifier, false); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidAdjustInput, err)
	}
	real := RealStats(attacker)
	var stat int
	switch category {
	case CategoryPhysical:
		stat = real.Atk
	case CategorySpecial:
		stat = real.SpA
	default:
		return 0, fmt.Errorf("%w: 分類 %q は指数を持たない", ErrInvalidAdjustInput, category)
	}
	return stat * power * modifier / Modifier4096, nil
}

// BulkIndex は耐久指数 floor(H × B(D) × 4096 / damageModifier) を返す(ADR-0800 §3)。
// category が physical なら B、special なら D の RealStats を使う(ランク補正は含めない)。
// damageModifier は受けるダメージの倍率を呼び出し側が掛け合わせた 4096 基準の補正(半減 2048 で指数が2倍)。
// 個体・分類・補正が不正なら ErrInvalidAdjustInput を包んで返す。
func BulkIndex(defender Individual, category MoveCategory, damageModifier int) (int, error) {
	if err := defender.Validate(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidAdjustInput, err)
	}
	if err := validateModifier("被ダメージ補正", damageModifier, false); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidAdjustInput, err)
	}
	real := RealStats(defender)
	var stat int
	switch category {
	case CategoryPhysical:
		stat = real.Def
	case CategorySpecial:
		stat = real.SpD
	default:
		return 0, fmt.Errorf("%w: 分類 %q は指数を持たない", ErrInvalidAdjustInput, category)
	}
	return real.HP * stat * Modifier4096 / damageModifier, nil
}

// HPLines は HP 種族値と HP の SP から、16n / 16n-1 ラインの報告を返す(ADR-0800 §4)。
// HP = baseHP + HPStatOffset + hpSP。合計 66 の制約は見ない。
// baseHP が MinBaseStat..MaxBaseStat、hpSP が 0..MaxSPPerStat の外なら ErrInvalidAdjustInput を包んで返す。
func HPLines(baseHP, hpSP int) (HPLineReport, error) {
	if baseHP < MinBaseStat || baseHP > MaxBaseStat {
		return HPLineReport{}, fmt.Errorf("%w: HP 種族値は %d..%d の範囲外: %d", ErrInvalidAdjustInput, MinBaseStat, MaxBaseStat, baseHP)
	}
	if hpSP < 0 || hpSP > MaxSPPerStat {
		return HPLineReport{}, fmt.Errorf("%w: HP の SP は 0..%d の範囲外: %d", ErrInvalidAdjustInput, MaxSPPerStat, hpSP)
	}
	hpAt := func(sp int) int {
		return RealStats(Individual{Species: Species{BaseStats: Stats{HP: baseHP}}, SP: Stats{HP: sp}}).HP
	}
	// 16n は mod 0、16n-1 は mod HPLineDivisor-1。
	const (
		rem16n       = 0
		rem16nMinus1 = HPLineDivisor - 1
	)
	next := func(rem int) *HPLinePoint {
		for sp := hpSP + 1; sp <= MaxSPPerStat; sp++ {
			if hp := hpAt(sp); hp%HPLineDivisor == rem {
				return &HPLinePoint{HP: hp, SP: sp, SPDelta: sp - hpSP}
			}
		}
		return nil
	}
	prev := func(rem int) *HPLinePoint {
		for sp := hpSP - 1; sp >= 0; sp-- {
			if hp := hpAt(sp); hp%HPLineDivisor == rem {
				return &HPLinePoint{HP: hp, SP: sp, SPDelta: sp - hpSP}
			}
		}
		return nil
	}
	hp := hpAt(hpSP)
	current := HPLineNone
	switch hp % HPLineDivisor {
	case rem16n:
		current = HPLine16n
	case rem16nMinus1:
		current = HPLine16nMinus1
	}
	return HPLineReport{
		HP: hp, SP: hpSP, Current: current,
		Next16n: next(rem16n), Prev16n: prev(rem16n),
		Next16nMinus1: next(rem16nMinus1), Prev16nMinus1: prev(rem16nMinus1),
	}, nil
}
