package httpapi

import (
	"example.com/pokecalc/services/internal/api"
)

// eventBattleState は計算イベントに載せる battleState。何も指定が無い({})は省略と同じなので nil(キーを出さない)。
// 値の検証(0 以下は invalid_input)は domain.BattleStateFrom(ADR-0144 §6)。
func eventBattleState(b *api.CalcBattleState) *api.CalcBattleState {
	if b == nil || (b.AttackerCurrentHp == nil && b.DefenderCurrentHp == nil && b.Hits == nil) {
		return nil
	}
	return b
}
