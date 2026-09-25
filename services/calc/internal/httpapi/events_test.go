package httpapi

// calc の3操作からのイベント発行(ADR-0212 §7・§7.1)。実 NATS は使わず、EventPublisher を
// 偽の実装に差し替えて呼び出し内容だけを検査する(AC-N5)。

import (
	"net/http"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
)

// fakePublisher は EventPublisher の記録用の偽実装。
type fakePublisher struct {
	calls []fakePublishCall
}

type fakePublishCall struct {
	deviceID, sessionID, operation string
	occurredAt                     time.Time
	detail                         *calcevents.CalcDetail
}

func (f *fakePublisher) Publish(deviceID, sessionID, operation string, occurredAt time.Time, detail *calcevents.CalcDetail) {
	f.calls = append(f.calls, fakePublishCall{deviceID, sessionID, operation, occurredAt, detail})
}

// AC-N3: POST /api/calc は Operation=calc・Detail 入りのイベントを1件だけ発行する。
// 技はリクエストのトップレベル moveId から取る(Individual.MoveId ではない。ADR-0212 §7)。
func TestCalcDamagePublishesCalcEventWithDetail(t *testing.T) {
	pub := &fakePublisher{}
	h := NewHandler(newFakeStore(t), pub)

	attacker := indiv{speciesKey: speciesAttacker, natureID: natureNeutral}
	defender := indiv{speciesKey: speciesDefender, natureID: natureNeutral}
	c := calcCase{name: "publish", attacker: attacker, defender: defender, moveID: movePhysical}
	before := time.Now()
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
	after := time.Now()
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	if len(pub.calls) != 1 {
		t.Fatalf("発行回数 = %d, want 1: %+v", len(pub.calls), pub.calls)
	}
	got := pub.calls[0]
	if got.deviceID != testDeviceID || got.sessionID != testSessionID {
		t.Errorf("envelope の device/session が一致しない: %+v", got)
	}
	if got.operation != calcevents.OperationCalc {
		t.Errorf("Operation = %q, want %q", got.operation, calcevents.OperationCalc)
	}
	if got.occurredAt.Before(before) || got.occurredAt.After(after) {
		t.Errorf("OccurredAt = %v, want [%v, %v] の範囲内", got.occurredAt, before, after)
	}
	if got.detail == nil {
		t.Fatal("Detail が無い(calc は Detail 入りのはず)")
	}
	if got.detail.MoveID != movePhysical {
		t.Errorf("Detail.MoveID = %q, want %q(リクエストのトップレベル moveId から取ること)", got.detail.MoveID, movePhysical)
	}
	if got.detail.Attacker.SpeciesKey != api.SpeciesKey(speciesAttacker) {
		t.Errorf("Detail.Attacker.SpeciesKey = %v, want %v", got.detail.Attacker.SpeciesKey, speciesAttacker)
	}
	// AC-N7: ViaRecommendation は常に false。
	if got.detail.ViaRecommendation {
		t.Error("ViaRecommendation が true になっている(常に false のはず。ADR-0212 §7)")
	}
}

// AC-N3: POST /api/calc/bulk は Operation=calcBulk・Detail 無しのイベントを1件だけ発行する
// (candidate ごとに分割発行しない。ADR-0212 §7.1)。
func TestCalcBulkPublishesEnvelopeOnly(t *testing.T) {
	pub := &fakePublisher{}
	h := NewHandler(newFakeStore(t), pub)

	body := map[string]any{
		"format":             "single",
		"attacker":           indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesDefender,
		"moveId":             movePhysical,
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(pub.calls) != 1 {
		t.Fatalf("発行回数 = %d, want 1(candidate ごとに分割発行しない): %+v", len(pub.calls), pub.calls)
	}
	got := pub.calls[0]
	if got.operation != calcevents.OperationBulk {
		t.Errorf("Operation = %q, want %q", got.operation, calcevents.OperationBulk)
	}
	if got.detail != nil {
		t.Errorf("Detail が入っている(envelope だけのはず): %+v", got.detail)
	}
}

// AC-N3: POST /api/calc/reverse は Operation=calcReverse・Detail 無しのイベントを1件だけ発行する。
func TestCalcReversePublishesEnvelopeOnly(t *testing.T) {
	pub := &fakePublisher{}
	h := NewHandler(newFakeStore(t), pub)

	body := map[string]any{
		"format":            "single",
		"side":              "defender",
		"known":             indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"unknownSpeciesKey": speciesDefender,
		"moveId":            movePhysical,
		"observations":      []map[string]any{{"percent": 18}},
	}
	rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), true)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(pub.calls) != 1 {
		t.Fatalf("発行回数 = %d, want 1: %+v", len(pub.calls), pub.calls)
	}
	got := pub.calls[0]
	if got.operation != calcevents.OperationReverse {
		t.Errorf("Operation = %q, want %q", got.operation, calcevents.OperationReverse)
	}
	if got.detail != nil {
		t.Errorf("Detail が入っている(envelope だけのはず): %+v", got.detail)
	}
}

// 失敗したリクエスト(invalid_input 等)ではイベントを発行しない。
func TestFailedCalcDoesNotPublish(t *testing.T) {
	pub := &fakePublisher{}
	h := NewHandler(newFakeStore(t), pub)

	// マスタに無い種族(speciesUnknown)を指定し、unknown_species で失敗させる。
	c := calcCase{name: "fail", attacker: indiv{speciesKey: speciesUnknown, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral}, moveID: movePhysical}
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), false)
	assertError(t, rec, http.StatusBadRequest, "unknown_species")
	if len(pub.calls) != 0 {
		t.Errorf("失敗したリクエストでイベントが発行された: %+v", pub.calls)
	}
}
