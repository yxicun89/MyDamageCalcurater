package httpapi

import (
	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// battleStateFrom は CalcRequest.battleState を engine.BattleState にする(ADR-0144 §6)。省略・null は従来どおり(ゼロ値)。
// engine の 0 は「満タン・既定」なので、API で指定された 0 や負は engine に渡さず invalid_input にする
// (黙って満タンと取り違えない)。最大 HP を超える・多段でない技の回数などの値域は engine が見る(ErrInvalidBattleState)。
func battleStateFrom(b *api.CalcBattleState) (engine.BattleState, error) {
	var s engine.BattleState
	if b == nil {
		return s, nil
	}
	for _, f := range []struct {
		name string
		src  *int
		dst  *int
	}{
		{"battleState.attackerCurrentHp", b.AttackerCurrentHp, &s.AttackerCurrentHP},
		{"battleState.defenderCurrentHp", b.DefenderCurrentHp, &s.DefenderCurrentHP},
		{"battleState.hits", b.Hits, &s.Hits},
	} {
		if f.src == nil {
			continue
		}
		if *f.src < 1 {
			return engine.BattleState{}, newError(api.InvalidInput, "%s は 1 以上でなければならない: %d", f.name, *f.src)
		}
		*f.dst = *f.src
	}
	return s, nil
}

// eventBattleState は計算イベントに載せる battleState。何も指定が無い({})は省略と同じなので nil(キーを出さない)。
func eventBattleState(b *api.CalcBattleState) *api.CalcBattleState {
	if b == nil || (b.AttackerCurrentHp == nil && b.DefenderCurrentHp == nil && b.Hits == nil) {
		return nil
	}
	return b
}
