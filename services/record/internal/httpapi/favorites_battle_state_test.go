package httpapi

// お気に入り・計算履歴の calc に対戦の状態 battleState(残り HP・多段の回数。ADR-0144)を持たせる受け入れテスト。
//
//   - 契約の CalcBattleState と同じ値域: 残り HP は 1 以上(0 は満タンの意味にしない)、回数は 1..10。範囲外は 400 invalid_input、
//     未知のキーは 400 unknown_field。最大 HP との照合はマスタが要るのでしない(計算するとき calc-svc が見る)。
//   - 省略・null・{} は省略のまま保存し、"battleState" キーを出さない(既存の calc とバイト列が同じ = 重複判定が変わらない)。
//   - 指定したキーだけを往復する。計算履歴は calc-svc が発行したイベントの battleState を同じ正規化で返す。

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
)

func calcWithBattleState(state any) map[string]any {
	c := calcInput()
	c["battleState"] = state
	return c
}

func TestCreateFavoriteBattleStateRoundTrips(t *testing.T) {
	h := NewHandler(newFakeStore())
	got := decodeFavorite(t, serve(t, h, http.MethodPost, pathFavorites, headers(deviceA),
		favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard),
			calcWithBattleState(map[string]any{"attackerCurrentHp": 77, "defenderCurrentHp": 1, "hits": 10}))), http.StatusCreated)
	b := got.Calc.BattleState
	if b == nil || b.AttackerCurrentHp == nil || *b.AttackerCurrentHp != 77 || b.DefenderCurrentHp == nil || *b.DefenderCurrentHp != 1 ||
		b.Hits == nil || *b.Hits != 10 {
		t.Fatalf("battleState が往復しない: %+v", b)
	}

	// 指定したキーだけを返す(省略したキーを 0 や null にしない)。
	rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA),
		favoriteWithCalcBody(t, nil, minimalIndividual(speciesLeaf), calcWithBattleState(map[string]any{"hits": 3})))
	calc := rawKeys(t, mustRaw(t, rawKeys(t, rec.Body.Bytes())["calc"]))
	if !jsonSemanticallyEqual(t, calc["battleState"], []byte(`{"hits":3}`)) {
		t.Errorf("battleState = %s, want {\"hits\":3}", calc["battleState"])
	}

	// 一覧でも返る。
	list := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil))
	n := 0
	for _, f := range list {
		if f.Calc != nil && f.Calc.BattleState != nil {
			n++
		}
	}
	if n != 2 {
		t.Errorf("一覧の battleState を持つ行 = %d, want 2", n)
	}
}

func mustRaw(t *testing.T, r json.RawMessage) []byte {
	t.Helper()
	return []byte(r)
}

// 省略・null・{} は従来の calc と同じ保存(キーを出さない)。同じ内容は重複として 200 になる。
func TestCreateFavoriteBattleStateOmittedIsUnchanged(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	body := func(c map[string]any) []byte { return favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c) }

	rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), body(calcInput()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	for name, state := range map[string]any{"null": nil, "空のオブジェクト": map[string]any{}} {
		rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), body(calcWithBattleState(state)))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200(省略と同じ内容 = 既存)", name, rec.Code)
		}
		if _, ok := rawKeys(t, mustRaw(t, rawKeys(t, rec.Body.Bytes())["calc"]))["battleState"]; ok {
			t.Errorf("%s: 応答の calc に battleState のキーがある", name)
		}
	}
	if n := len(st.favoritesOf(deviceA)); n != 1 {
		t.Errorf("行 = %d, want 1", n)
	}
}

func TestCreateFavoriteBattleStateValidation(t *testing.T) {
	cases := []struct {
		name     string
		state    any
		wantCode api.ErrorCode
	}{
		{"攻撃側の残り HP が 0", map[string]any{"attackerCurrentHp": 0}, api.InvalidInput},
		{"防御側の残り HP が負", map[string]any{"defenderCurrentHp": -1}, api.InvalidInput},
		{"回数が 0", map[string]any{"hits": 0}, api.InvalidInput},
		{"回数が 11", map[string]any{"hits": 11}, api.InvalidInput},
		{"未知のキー", map[string]any{"currentHp": 10}, api.UnknownField},
		{"配列", []any{1}, api.InvalidJson},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA),
				favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), calcWithBattleState(c.state)))
			assertErrorBody(t, rec, http.StatusBadRequest, c.wantCode)
			if n := len(st.favoritesOf(deviceA)); n != 0 {
				t.Errorf("拒否した要求で行が %d 件できた", n)
			}
		})
	}
}

// 計算履歴: calc-svc が発行したイベントの battleState を、お気に入りと同じ正規化で返す。無ければキーを出さない。
func TestCalcHistoryCarriesBattleState(t *testing.T) {
	st := newFakeStore()
	now := historyNow()
	var req api.CalcRequest
	raw, _ := json.Marshal(calcInput())
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	one, ten := 1, 10
	payload := func(state *api.CalcBattleState) []byte {
		b, err := json.Marshal(calcevents.Event{
			SchemaVersion: calcevents.SchemaVersion, DeviceID: deviceA, SessionID: sessionID,
			Operation: calcevents.OperationCalc, OccurredAt: now,
			Detail: &calcevents.CalcDetail{
				Format: string(req.Format), Attacker: req.Attacker, Defender: req.Defender, MoveID: req.MoveId,
				BattleState: state, MinPercent: 1, MaxPercent: 2,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	st.addEvent(deviceA, "ev-with", calcevents.OperationCalc, now, payload(&api.CalcBattleState{DefenderCurrentHp: &one, Hits: &ten}))
	st.addEvent(deviceA, "ev-empty", calcevents.OperationCalc, now.Add(-time.Minute), payload(&api.CalcBattleState{}))
	st.addEvent(deviceA, "ev-none", calcevents.OperationCalc, now.Add(-2*time.Minute), payload(nil))

	rec := serve(t, historyHandler(st), http.MethodGet, pathCalcHistory, headers(deviceA), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	page := decodeHistory(t, rec)
	if len(page.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(page.Items))
	}
	b := page.Items[0].Calc.BattleState
	if b == nil || b.AttackerCurrentHp != nil || b.DefenderCurrentHp == nil || *b.DefenderCurrentHp != 1 || b.Hits == nil || *b.Hits != 10 {
		t.Errorf("履歴の battleState = %+v, want defenderCurrentHp 1・hits 10", b)
	}
	for i, name := range map[int]string{1: "空のオブジェクト", 2: "無し"} {
		if page.Items[i].Calc.BattleState != nil {
			t.Errorf("%s: battleState = %+v, want 省略", name, page.Items[i].Calc.BattleState)
		}
	}
}
