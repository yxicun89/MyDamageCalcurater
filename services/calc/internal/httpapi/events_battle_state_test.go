package httpapi

// 計算イベントの battleState(ADR-0144。お気に入り・計算履歴が battleState を保存・返却するため、record-svc が読む)。
// 指定があればリクエストのまま Detail に載せ、無ければ nil(JSON でキーが出ない)。

import (
	"testing"
)

func TestCalcEventCarriesBattleState(t *testing.T) {
	store := stage3Store(t)
	attacker := indiv{speciesKey: speciesAttacker, natureID: natureNeutral}
	defender := indiv{speciesKey: speciesDefender, natureID: natureNeutral}

	pub := &fakePublisher{}
	h := NewHandler(store, pub)
	body := calcCase{attacker: attacker, defender: defender, moveID: moveRangeMulti}.httpBody()
	body["battleState"] = map[string]any{"defenderCurrentHp": 40, "hits": 4}
	if rec := post(t, h, "/api/calc", mustJSON(t, body), true); rec.Code != 200 {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if len(pub.calls) != 1 {
		t.Fatalf("発行回数 = %d, want 1", len(pub.calls))
	}
	b := pub.calls[0].detail.BattleState
	if b == nil || b.DefenderCurrentHp == nil || *b.DefenderCurrentHp != 40 || b.Hits == nil || *b.Hits != 4 || b.AttackerCurrentHp != nil {
		t.Errorf("Detail.BattleState = %+v, want defenderCurrentHp 40・hits 4", b)
	}

	pub = &fakePublisher{}
	h = NewHandler(store, pub)
	plain := calcCase{attacker: attacker, defender: defender, moveID: movePhysical}.httpBody()
	if rec := post(t, h, "/api/calc", mustJSON(t, plain), true); rec.Code != 200 {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if pub.calls[0].detail.BattleState != nil {
		t.Errorf("battleState を送らないのに Detail.BattleState = %+v", pub.calls[0].detail.BattleState)
	}
}
