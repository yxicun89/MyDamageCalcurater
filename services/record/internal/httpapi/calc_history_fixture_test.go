package httpapi

// 計算履歴(GET /api/record/calc-history。ADR-0230)のテストの共通部品。
//
// **test-first(ADR-0003)**: 実装前に書いた。前提にする形(実装者向け。ADR-0230 §8 と同じ):
//
//	// store パッケージ(store.go)に足す型とメソッド:
//	type CalcHistoryCursor struct {
//		OccurredAt time.Time // この位置より古い行を返す(keyset。occurred_at DESC, event_id DESC の並びで「後ろ」)
//		EventID    string
//	}
//	type CalcHistoryQuery struct {
//		Since  time.Time          // occurred_at >= Since の行だけ(保持期間の下限。ゼロ値なら下限なし)
//		Before *CalcHistoryCursor // nil なら先頭(最新)から
//		Limit  int                // 返す最大行数(httpapi は「続きの有無」を知るため limit+1 を渡してよい)
//	}
//	type CalcHistoryRow struct {
//		EventID    string
//		OccurredAt time.Time
//		Payload    []byte // calc_events.payload(calcevents.Event の JSON)。store は中身を解釈しない
//	}
//	// Store インターフェースに追加:
//	//   operation = 'calc' の行だけを、WHERE device_id = ? で絞り、occurred_at > devices.purged_at(墓石があれば)・
//	//   occurred_at >= q.Since・(occurred_at, event_id) < (Before.OccurredAt, Before.EventID) を満たすものを
//	//   occurred_at DESC, event_id DESC で最大 q.Limit 行返す。1行も無ければ長さ0(nil でない)のスライス。
//	//   読むだけで、calc_events・devices・集計のどの行も書き換えない。
//	ListCalcHistory(ctx context.Context, deviceID string, q CalcHistoryQuery) ([]CalcHistoryRow, error)
//
//	// httpapi パッケージ(server.go)に足す: NewHandler に可変長の Option を取らせる(既存の NewHandler(st) は
//	// そのまま動く)。保持期間を渡すと、一覧はそれより古い行を返さない(Since = now - retention)。
//	// cmd/record の runServe は cfg.CalcEventsRetention を必ず渡す。
//	type Option func(*Server)
//	func WithCalcEventsRetention(d time.Duration) Option
//	func NewHandler(st store.Store, opts ...Option) http.Handler
//
// fake は上の store の規則をそのまま写す(fixture_test.go の fakeStore にメソッドを足す)。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/record/internal/store"
)

const pathCalcHistory = "/api/record/calc-history"

// historyRetention は本番の既定(RECORD_CALC_EVENTS_RETENTION_DAYS=90)と同じ長さ。テストは
// WithCalcEventsRetention で明示的に渡す(httpapi に既定値を持たせない。ADR-0211 §7)。
const historyRetention = 90 * 24 * time.Hour

func (f *fakeStore) ListCalcHistory(ctx context.Context, deviceID string, q store.CalcHistoryQuery) ([]store.CalcHistoryRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ListCalcHistory", deviceID)
	if f.unavailable {
		return nil, fmt.Errorf("fake: %w", store.ErrUnavailable)
	}
	d := f.state(deviceID)
	out := make([]store.CalcHistoryRow, 0)
	for _, ev := range d.events {
		if ev.Operation != calcevents.OperationCalc {
			continue
		}
		if !d.purgedAt.IsZero() && !ev.OccurredAt.After(d.purgedAt) {
			continue
		}
		if !q.Since.IsZero() && ev.OccurredAt.Before(q.Since) {
			continue
		}
		if q.Before != nil && !historyLess(ev.OccurredAt, ev.EventID, q.Before.OccurredAt, q.Before.EventID) {
			continue
		}
		out = append(out, store.CalcHistoryRow{
			EventID: ev.EventID, OccurredAt: ev.OccurredAt, Payload: append([]byte(nil), ev.Payload...),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return historyLess(out[j].OccurredAt, out[j].EventID, out[i].OccurredAt, out[i].EventID)
	})
	if q.Limit >= 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// historyLess は (at, id) が (bAt, bID) より「古い」(並びで後ろ)かどうか。
func historyLess(at time.Time, id string, bAt time.Time, bID string) bool {
	if !at.Equal(bAt) {
		return at.Before(bAt)
	}
	return id < bID
}

// recordingStore は ListCalcHistory に渡された問い合わせを記録する(保持期間の下限を httpapi が
// 正しく渡すことを確かめるため)。他のメソッドは fakeStore のまま。
type recordingStore struct {
	*fakeStore
	queries []store.CalcHistoryQuery
}

func (r *recordingStore) ListCalcHistory(ctx context.Context, deviceID string, q store.CalcHistoryQuery) ([]store.CalcHistoryRow, error) {
	r.queries = append(r.queries, q)
	return r.fakeStore.ListCalcHistory(ctx, deviceID, q)
}

// historyHandler は保持期間つきの record-svc ハンドラ(本番の runServe と同じ組み立て)。
func historyHandler(st store.Store) http.Handler {
	return NewHandler(st, WithCalcEventsRetention(historyRetention))
}

// calcEventPayload は calc-svc が発行し record-svc の consumer が保存するのと同じ形の payload
// (calcevents.Event の JSON。consumer.go の json.Marshal(ev) と同じ)を作る。calc は calcInput() 等の
// 計算の入力(CalcRequest と同じ形の map)。
func calcEventPayload(t *testing.T, deviceID string, occurredAt time.Time, calc map[string]any, minPercent, maxPercent float64) []byte {
	t.Helper()
	raw, err := json.Marshal(calc)
	if err != nil {
		t.Fatal(err)
	}
	var req api.CalcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("calc が CalcRequest として読めない: %v; %s", err, raw)
	}
	ev := calcevents.Event{
		SchemaVersion: calcevents.SchemaVersion,
		DeviceID:      deviceID,
		SessionID:     sessionID,
		Operation:     calcevents.OperationCalc,
		OccurredAt:    occurredAt,
		Detail: &calcevents.CalcDetail{
			Format: string(req.Format), Attacker: req.Attacker, Defender: req.Defender,
			MoveID: req.MoveId, Field: req.Field, Options: req.Options,
			MinPercent: minPercent, MaxPercent: maxPercent,
		},
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// envelopePayload は Detail を持たない操作(calcBulk / calcReverse)の payload。
func envelopePayload(t *testing.T, deviceID, operation string, occurredAt time.Time) []byte {
	t.Helper()
	b, err := json.Marshal(calcevents.Event{
		SchemaVersion: calcevents.SchemaVersion, DeviceID: deviceID, SessionID: sessionID,
		Operation: operation, OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// addEvent は fake の calc_events に1行足す(consumer を通さず、保存済みの状態を直接作る)。
func (f *fakeStore) addEvent(deviceID, eventID, operation string, occurredAt time.Time, payload []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.state(deviceID)
	species := ""
	if operation == calcevents.OperationCalc {
		species = speciesLeaf
	}
	d.events = append(d.events, store.CalcEvent{
		EventID: eventID, DeviceID: deviceID, SessionID: sessionID, Operation: operation,
		OccurredAt: occurredAt, DefenderSpeciesKey: species, Payload: payload,
	})
}

// addCalc は「1件の計算」のイベントを足す。moveId で行を見分けられるようにする。
func (f *fakeStore) addCalc(t *testing.T, deviceID, eventID string, occurredAt time.Time, moveID string) {
	t.Helper()
	calc := calcInput()
	calc["moveId"] = moveID
	f.addEvent(deviceID, eventID, calcevents.OperationCalc, occurredAt, calcEventPayload(t, deviceID, occurredAt, calc, 12.3, 45.6))
}

// eventsOf はその端末の calc_events の写し(読み取りで書き換わっていないことを確かめる)。
func (f *fakeStore) eventsOf(deviceID string) []store.CalcEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.CalcEvent(nil), f.state(deviceID).events...)
}

// historyNow は fake の行の時刻の基準(保持期間の判定は httpapi の time.Now() なので、実時刻の近くに置く)。
func historyNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// decodeHistory は 200 の本文を CalcHistoryPage として読む。
func decodeHistory(t *testing.T, rec *httptest.ResponseRecorder) api.CalcHistoryPage {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got api.CalcHistoryPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("CalcHistoryPage として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}

// moveIDs は1ページの行の moveId を順に並べる(行の見分けに使う)。
func moveIDs(p api.CalcHistoryPage) []string {
	out := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		out = append(out, it.Calc.MoveId)
	}
	return out
}

// historyPageURL は limit と cursor を付けた一覧の URL。
func historyPageURL(limit int, cursor *string) string {
	u := fmt.Sprintf("%s?limit=%d", pathCalcHistory, limit)
	if cursor != nil {
		u += "&cursor=" + *cursor
	}
	return u
}
