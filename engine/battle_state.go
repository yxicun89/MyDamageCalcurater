package engine

// 対戦の状態(BattleState。ADR-0144)。
//
// 残り HP と、範囲の多段技の回数。ゼロ値(省略)は従来の計算と同じ(満タン・段階1の既定の回数)。

import (
	"errors"
	"fmt"
)

// ErrInvalidBattleState は対戦の状態の入力が不正(残り HP が負・最大超え、回数が負・範囲の多段でない技・範囲外)。
var ErrInvalidBattleState = errors.New("対戦の状態が不正")

// BattleState は計算の対象になる「この1回の攻撃」の前提となる対戦の状態。
type BattleState struct {
	AttackerCurrentHP int // 0 は満タン。1..攻撃側の最大 HP
	DefenderCurrentHP int // 0 は満タン。1..防御側の最大 HP
	Hits              int // 0 は段階1の既定。範囲の多段技(Min < Max)だけ・Min..Max
}

// validateBattleState は状態の値域を確かめる(最大 HP は両側の実数値)。
func validateBattleState(in DamageInput) error {
	s := in.State
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidBattleState, fmt.Sprintf(format, args...))
	}
	if maxHP := RealStats(in.Attacker).HP; s.AttackerCurrentHP < 0 || s.AttackerCurrentHP > maxHP {
		return bad("攻撃側の残り HP は 0(満タン)..%d: %d", maxHP, s.AttackerCurrentHP)
	}
	if maxHP := RealStats(in.Defender).HP; s.DefenderCurrentHP < 0 || s.DefenderCurrentHP > maxHP {
		return bad("防御側の残り HP は 0(満タン)..%d: %d", maxHP, s.DefenderCurrentHP)
	}
	switch mh := in.Move.Params.MultiHit; {
	case s.Hits == 0:
	case s.Hits < 0:
		return bad("回数が負: %d", s.Hits)
	case mh == nil || mh.Min >= mh.Max:
		return bad("回数は範囲の多段技だけに指定できる: %d", s.Hits)
	case s.Hits < mh.Min || s.Hits > mh.Max:
		return bad("回数は %d..%d: %d", mh.Min, mh.Max, s.Hits)
	}
	return nil
}

// currentHP は残り HP の入力 cur(0 は満タン)を実数値の残り HP にする。
func currentHP(cur, maxHP int) int {
	if cur == 0 {
		return maxHP
	}
	return cur
}

func attackerCurrentHP(in DamageInput) int {
	return currentHP(in.State.AttackerCurrentHP, RealStats(in.Attacker).HP)
}

func defenderCurrentHP(in DamageInput) int {
	return currentHP(in.State.DefenderCurrentHP, RealStats(in.Defender).HP)
}

// attackerHPLowPower はきしかいせい・じたばた型の表(p = floor(48 × 残り / 最大) の段)。
func attackerHPLowPower(cur, maxHP int) int {
	switch p := 48 * cur / maxHP; {
	case p <= 1:
		return 200
	case p <= 4:
		return 150
	case p <= 9:
		return 100
	case p <= 16:
		return 80
	case p <= 32:
		return 40
	}
	return 20
}

// defenderHPRatioPower はハードプレス型の式(Showdown と同じ 4096 の丸め。0 は 1)。
func defenderHPRatioPower(cur, maxHP int) int {
	b := 100 * (cur * Modifier4096 / maxHP)
	return max(1, (100*b+2047)/Modifier4096/100)
}
