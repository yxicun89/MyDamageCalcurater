package httpapi

// AC-9 相当・issue #271-b/#270 の API レーン担当分: engine の「未対応」の印(ADR-0123)が
// HTTP の応答(CalcResult.unsupported)に実際に非空の状態で現れることを確かめる
// (parity_test.go は印が付かない入力しか使わないため、非空のケースはここで別に固定する)。

import (
	"testing"

	"example.com/pokecalc/engine"
)

const moveOHKO = "test-ohko" // normal / physical / 80。常に印が付く機構(MechanismOHKO)を持つ

// TestCalcResultCarriesUnsupportedMark は、機構を持つ技で計算すると
// CalcResult.unsupported に {target:"move", reason:"ohko", id:moveOHKO} が入ることを確かめる。
func TestCalcResultCarriesUnsupportedMark(t *testing.T) {
	store := newFakeStore(t)
	store.moves[moveOHKO] = engine.Move{
		ID: moveOHKO, NameJa: "テストいちげき", Type: engine.TypeNormal, Category: engine.CategoryPhysical,
		Power: 80, Mechanisms: []engine.MoveMechanism{engine.MechanismOHKO},
	}
	h := NewHandler(store, nil)

	c := calcCase{name: "ohko", attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral}, moveID: moveOHKO}
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)

	var got struct {
		Unsupported []struct {
			Target string `json:"target"`
			Reason string `json:"reason"`
			Id     string `json:"id"`
		} `json:"unsupported"`
	}
	decodeInto(t, rec, &got)

	if len(got.Unsupported) != 1 {
		t.Fatalf("unsupported の件数 = %d, want 1: %+v", len(got.Unsupported), got.Unsupported)
	}
	want := struct {
		Target string
		Reason string
		Id     string
	}{"move", "ohko", moveOHKO}
	if got.Unsupported[0].Target != want.Target || got.Unsupported[0].Reason != want.Reason || got.Unsupported[0].Id != want.Id {
		t.Errorf("unsupported[0] = %+v, want %+v", got.Unsupported[0], want)
	}
}

// TestBulkResultCarriesUnsupportedMark は bulk の各行(BulkCalcRow.result は CalcResult を再利用する)にも
// 同じ印が乗ることを確かめる。
func TestBulkResultCarriesUnsupportedMark(t *testing.T) {
	store := newFakeStore(t)
	store.moves[moveOHKO] = engine.Move{
		ID: moveOHKO, NameJa: "テストいちげき", Type: engine.TypeNormal, Category: engine.CategoryPhysical,
		Power: 80, Mechanisms: []engine.MoveMechanism{engine.MechanismOHKO},
	}
	h := NewHandler(store, nil)

	body := map[string]any{
		"format":             "single",
		"attacker":           indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesDefender,
		"moveId":             moveOHKO,
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)

	var got struct {
		Rows []struct {
			Result struct {
				Unsupported []map[string]any `json:"unsupported"`
			} `json:"result"`
		} `json:"rows"`
	}
	decodeInto(t, rec, &got)

	if len(got.Rows) == 0 {
		t.Fatal("rows が空")
	}
	for i, row := range got.Rows {
		if len(row.Result.Unsupported) != 1 {
			t.Errorf("rows[%d].result.unsupported の件数 = %d, want 1: %+v", i, len(row.Result.Unsupported), row.Result.Unsupported)
		}
	}
}
