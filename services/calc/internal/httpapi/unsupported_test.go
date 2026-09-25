package httpapi

// AC-9 相当・issue #271-b/#270 の API レーン担当分: engine の「未対応」の印(ADR-0123)が
// HTTP の応答(CalcResult.unsupported・ReverseCandidate.unsupported)に実際に非空の状態で
// 現れることを確かめる(parity_test.go は印が付かない入力しか使わないため、非空のケースは
// ここで別に固定する)。

import (
	"testing"

	"example.com/pokecalc/engine"
)

const (
	moveOHKO              = "test-ohko"                // normal / physical / 80。常に印が付く機構(MechanismOHKO)を持つ
	itemUnsupportedAtk    = "test-item-unsupported"    // 攻撃側が持つと未対応の効果(UnsupportedAttacker)
	abilityUnsupportedDef = "test-ability-unsupported" // 防御側が持つと未対応の効果(UnsupportedDefender)
)

// unsupportedMark は応答の unsupported 要素1件をデコードするための型。
type unsupportedMark struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
	Id     string `json:"id"`
}

// newStoreWithOHKO は moveOHKO(常に印が付く技)を持つ fakeStore を作る。
func newStoreWithOHKO(t *testing.T) *fakeStore {
	t.Helper()
	store := newFakeStore(t)
	store.moves[moveOHKO] = engine.Move{
		ID: moveOHKO, NameJa: "テストいちげき", Type: engine.TypeNormal, Category: engine.CategoryPhysical,
		Power: 80, Mechanisms: []engine.MoveMechanism{engine.MechanismOHKO},
	}
	return store
}

// TestCalcResultCarriesUnsupportedMark は、機構を持つ技で計算すると
// CalcResult.unsupported に {target:"move", reason:"ohko", id:moveOHKO} が入ることを確かめる。
func TestCalcResultCarriesUnsupportedMark(t *testing.T) {
	store := newStoreWithOHKO(t)
	h := NewHandler(store, nil)

	c := calcCase{name: "ohko", attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral}, moveID: moveOHKO}
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)

	var got struct {
		Unsupported []unsupportedMark `json:"unsupported"`
	}
	decodeInto(t, rec, &got)

	if len(got.Unsupported) != 1 {
		t.Fatalf("unsupported の件数 = %d, want 1: %+v", len(got.Unsupported), got.Unsupported)
	}
	want := unsupportedMark{"move", "ohko", moveOHKO}
	if got.Unsupported[0] != want {
		t.Errorf("unsupported[0] = %+v, want %+v", got.Unsupported[0], want)
	}
}

// TestBulkResultCarriesUnsupportedMark は bulk の各行(BulkCalcRow.result は CalcResult を再利用する)にも
// 同じ印が乗ることを確かめる。
func TestBulkResultCarriesUnsupportedMark(t *testing.T) {
	store := newStoreWithOHKO(t)
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
				Unsupported []unsupportedMark `json:"unsupported"`
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

// TestReverseCandidatesCarryUnsupportedMark(critic 指摘。重要): 逆算の各候補にも同じ印が乗ることを
// 確かめる(ADR-0123 §2「逆算は各候補の Unsupported に同じ印が付く(SP によらない)」)。
func TestReverseCandidatesCarryUnsupportedMark(t *testing.T) {
	store := newStoreWithOHKO(t)
	h := NewHandler(store, nil)

	c := reverseCase{
		name: "ohko", side: engine.SideDefender,
		known:          indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		unknownSpecies: speciesDefender, moveID: moveOHKO,
		observations: []engine.Observation{{Percent: 20}},
	}
	rec := post(t, h, "/api/calc/reverse", mustJSON(t, c.httpBody()), true)

	var got struct {
		Candidates []struct {
			Unsupported []unsupportedMark `json:"unsupported"`
		} `json:"candidates"`
	}
	decodeInto(t, rec, &got)

	if len(got.Candidates) == 0 {
		t.Fatal("candidates が空")
	}
	want := unsupportedMark{"move", "ohko", moveOHKO}
	for i, cand := range got.Candidates {
		if len(cand.Unsupported) != 1 || cand.Unsupported[0] != want {
			t.Errorf("candidates[%d].unsupported = %+v, want [%+v]", i, cand.Unsupported, want)
		}
	}
}

// TestUnsupportedMarksCoverAllTargetsAndOrder(critic 指摘。重要): target が move 以外(持ち物・特性)でも
// 正しく写ることと、複数の印の並び順(技 → 攻撃側持ち物 → 攻撃側特性 → 防御側持ち物 → 防御側特性。
// ADR-0123 §2)を確かめる。Target の写し間違い(例: 常に "move" に固定する退行)を検知する。
func TestUnsupportedMarksCoverAllTargetsAndOrder(t *testing.T) {
	store := newStoreWithOHKO(t)
	store.items[itemUnsupportedAtk] = engine.Item{
		ID: itemUnsupportedAtk, NameJa: "テストみらいのどうぐ", Effect: &engine.ItemEffect{UnsupportedAttacker: true},
	}
	store.abilities[abilityUnsupportedDef] = engine.Ability{
		ID: abilityUnsupportedDef, NameJa: "テストみらいのとくせい", Effect: &engine.AbilityEffect{UnsupportedDefender: true},
	}
	h := NewHandler(store, nil)

	c := calcCase{
		name:     "複合",
		attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, itemID: itemUnsupportedAtk},
		defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral, abilityID: abilityUnsupportedDef},
		moveID:   moveOHKO,
	}
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)

	var got struct {
		Unsupported []unsupportedMark `json:"unsupported"`
	}
	decodeInto(t, rec, &got)

	want := []unsupportedMark{
		{"move", "ohko", moveOHKO},
		{"attacker_item", "unsupported_effect", itemUnsupportedAtk},
		{"defender_ability", "unsupported_effect", abilityUnsupportedDef},
	}
	if len(got.Unsupported) != len(want) {
		t.Fatalf("unsupported = %+v, want %+v", got.Unsupported, want)
	}
	for i := range want {
		if got.Unsupported[i] != want[i] {
			t.Errorf("unsupported[%d] = %+v, want %+v(並び順: 技→攻撃側持ち物→攻撃側特性→防御側持ち物→防御側特性)",
				i, got.Unsupported[i], want[i])
		}
	}
}
