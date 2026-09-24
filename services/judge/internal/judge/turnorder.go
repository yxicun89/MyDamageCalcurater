// JD4(ADR-0704 §2)の先制判定。優先度が違えば優先度が高い方が必ず先に動く(素早さも
// トリックルームも見ない)。優先度が同じなら素早さ(SpeedComparison.Outspeeds。トリック
// ルーム反映済み)に従う。優先度も素早さも同じならどちらが先か決まらない(Tie)。
package judge

// TurnOrder is the result of comparing two moves' priority and, when they tie, battle speed
// (ADR-0704 §2). Tie が true のとき AttackerMovesFirst は false になる: 優先度も素早さも
// 同じ場合、ゲームでも行動順は乱数で決まり、どちらが先かを断定できない
// (ADR-0700 §6-1・ADR-0602 と同じ立場)。
type TurnOrder struct {
	AttackerMovesFirst bool
	Tie                bool
}

// CompareTurnOrder decides which side moves first. When attackerPriority and defenderPriority
// differ, the higher priority always wins regardless of speed (a priority move outruns a
// faster foe, and trick room does not reverse priority). When they're equal, the outcome falls
// back to speed's own comparison (speed.Outspeeds, which already reflects trick room per
// ADR-0702 §3) and speed.SpeedTie decides the tie.
func CompareTurnOrder(attackerPriority, defenderPriority int, speed SpeedComparison) TurnOrder {
	if attackerPriority != defenderPriority {
		return TurnOrder{AttackerMovesFirst: attackerPriority > defenderPriority}
	}
	// speed.Outspeeds and speed.SpeedTie are never both true (CompareSpeed's own invariant), but
	// AttackerMovesFirst && Tie must not happen here either; don't depend on that invariant
	// holding across a future CompareSpeed change (ADR-0704 §3 受け入れ条件3).
	return TurnOrder{AttackerMovesFirst: speed.Outspeeds && !speed.SpeedTie, Tie: speed.SpeedTie}
}
