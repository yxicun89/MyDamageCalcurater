package engine

// 技の追加効果(命中時のランク変化)。ADR-0107。
//
// engine は「発動するかどうか」を判定しない(ADR-0107 決定1)。乱数を持たないので、
// MoveEffect は「命中して発動したとき、何がどれだけ変わるか」と「その確率」を持つだけの
// メタデータであり、実際に発動したかどうかを決めるのは呼び出し側(judge)の責務。
//
// CalcDamage はこの値を読まない(ADR-0107 決定2。ADR-0106 の AbsorbEffect と同じ立場)。

import "fmt"

// RankTarget はランク変化の対象。
type RankTarget string

const (
	RankTargetSelf     RankTarget = "self"   // 使用者自身
	RankTargetOpponent RankTarget = "target" // 技の対象
)

// MoveEffect は技の追加効果(命中時のランク変化)。ダメージ計算はこの値を読まない(ADR-0107 決定2)。
type MoveEffect struct {
	Chance int             // 発動確率(1..100)。100 は命中すれば必ず発動
	Target RankTarget      // self / target
	Stages map[StatKey]int // 変化量(-6..+6、0 は不可)。HP は不可
}

// Validate は MoveEffect の値が意味を持つ範囲に収まっていることを確認する(ADR-0107 決定3)。
func (e MoveEffect) Validate() error {
	if e.Chance < 1 || e.Chance > 100 {
		return fmt.Errorf("engine: MoveEffect.Chance は1..100: got %d", e.Chance)
	}
	switch e.Target {
	case RankTargetSelf, RankTargetOpponent:
	default:
		return fmt.Errorf("engine: MoveEffect.Target が未知: %q", e.Target)
	}
	if len(e.Stages) == 0 {
		return fmt.Errorf("engine: MoveEffect.Stages が空")
	}
	for k, v := range e.Stages {
		if k == StatHP {
			return fmt.Errorf("engine: MoveEffect.Stages に HP は持てない")
		}
		if k != StatAtk && k != StatDef && k != StatSpA && k != StatSpD && k != StatSpe {
			return fmt.Errorf("engine: MoveEffect.Stages のキーが未知: %q", k)
		}
		if v == 0 || v < -6 || v > 6 {
			return fmt.Errorf("engine: MoveEffect.Stages[%q] は -6..+6 かつ 0 以外: got %d", k, v)
		}
	}
	return nil
}
