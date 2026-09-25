package events

// JetStream の計算イベント購読(ADR-0209 §4・§7、ADR-0212 §6)の受け入れテスト。
// AC-P3 / AC-P4 / AC-R2c / AC-R5、および at-least-once 配送の冪等性(新設 AC-R7)。
//
// **test-first(ADR-0003)**: 実装済み(`consumer.go`・`subscriber.go`)。spec-writer が最初に定めた形:
//
//	// Action は1件のメッセージの処理後にすべきこと。
//	type Action int
//	const (
//		Ack  Action = iota // 処理済み(保存した・重複だった・墓石で捨てた)。再配送させない
//		Nak                // 一時的な失敗(DB に届かない)。後で再配送させる
//		Term               // 恒久的に処理できない(壊れた JSON・未知の schemaVersion)。再配送させない
//	)
//
//	// Handler は NATS に依存しない純粋な処理本体。consume ループ(jetstream.Consumer.Consume)は
//	// メッセージから eventID と本文を取り出して Handle を呼び、戻り値の Action に従って
//	// msg.Ack() / msg.Nak() / msg.Term() を呼ぶだけにする(NATS 無しでテストできるようにするため)。
//	func NewHandler(st store.Store) *Handler
//	func (h *Handler) Handle(ctx context.Context, eventID string, data []byte) Action
//
//	// eventID は JetStream のストリームシーケンス由来で、同じメッセージの再配送では同じ値になること
//	// (受信時刻・ランダム値を混ぜない)。組み立ては EventID(streamSeq uint64) string を使う。
//	func EventID(streamSeq uint64) string
//
// consumer は record-svc 専用の durable(team-svc とは別。ADR-0209 §4)。
// durable 名と subject filter は DurableName / SubjectFilter で公開し、テストが固定する。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/record/internal/store"
)

const (
	deviceA   = "00000000-0000-4000-8000-00000000000a"
	deviceB   = "00000000-0000-4000-8000-00000000000b"
	sessionID = "00000000-0000-4000-8000-000000000051"
)

var baseTime = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

// recordingStore は store.Store の架空実装(events 用。httpapi の fakeStore とは別パッケージなので独立に持つ)。
type recordingStore struct {
	mu sync.Mutex

	saved       map[string]store.CalcEvent   // EventID → 渡された内容
	outcomes    map[string]store.SaveOutcome // EventID → 返した結果
	purgedAt    map[string]time.Time
	lastSeenAt  map[string]time.Time
	saveCalls   int
	unavailable bool
}

func newRecordingStore() *recordingStore {
	return &recordingStore{
		saved: map[string]store.CalcEvent{}, outcomes: map[string]store.SaveOutcome{},
		purgedAt: map[string]time.Time{}, lastSeenAt: map[string]time.Time{},
	}
}

func (s *recordingStore) TouchDevice(ctx context.Context, deviceID string, now time.Time) error {
	return nil
}

func (s *recordingStore) FrequentOpponents(ctx context.Context, deviceID string, limit int) ([]store.FrequentOpponent, error) {
	return nil, nil
}

func (s *recordingStore) PurgeDevice(ctx context.Context, deviceID string, now time.Time) (store.PurgeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgedAt[deviceID] = now
	return store.PurgeResult{PurgedAt: now}, nil
}

func (s *recordingStore) SaveCalcEvent(ctx context.Context, ev store.CalcEvent) (store.SaveOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCalls++
	if s.unavailable {
		return 0, fmt.Errorf("recordingStore: %w", store.ErrUnavailable)
	}
	if _, dup := s.saved[ev.EventID]; dup {
		return store.Duplicate, nil
	}
	s.saved[ev.EventID] = ev
	if p, ok := s.purgedAt[ev.DeviceID]; ok && !ev.OccurredAt.After(p) {
		s.outcomes[ev.EventID] = store.Tombstoned
		return store.Tombstoned, nil
	}
	s.outcomes[ev.EventID] = store.Stored
	if ev.OccurredAt.Sub(s.lastSeenAt[ev.DeviceID]) >= 24*time.Hour {
		s.lastSeenAt[ev.DeviceID] = ev.OccurredAt
	}
	return store.Stored, nil
}

func (s *recordingStore) storedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, o := range s.outcomes {
		if o == store.Stored {
			n++
		}
	}
	return n
}

// marshal はワイヤ上のイベント本文を作る。
func marshal(t *testing.T, ev calcevents.Event) []byte {
	t.Helper()
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("イベントを組み立てられない: %v", err)
	}
	return b
}

func calcEvent(deviceID string, at time.Time) calcevents.Event {
	return calcevents.Event{
		SchemaVersion: calcevents.SchemaVersion,
		DeviceID:      deviceID,
		SessionID:     sessionID,
		Operation:     calcevents.OperationCalc,
		OccurredAt:    at,
		Detail:        &calcevents.CalcDetail{MoveID: "test-beam"},
	}
}

// AC-R7(新設): at-least-once 配送。同じメッセージ(= 同じストリームシーケンス)を2回受け取っても
// 保存・集計・last_seen_at が二重にならず、2回目も ack される。
func TestRedeliveryIsIdempotent(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)
	data := marshal(t, calcEvent(deviceA, baseTime))
	id := EventID(42)

	if got := h.Handle(context.Background(), id, data); got != Ack {
		t.Fatalf("1回目 = %v, want Ack", got)
	}
	if got := h.Handle(context.Background(), id, data); got != Ack {
		t.Errorf("再配送 = %v, want Ack(重複でも ack して再配送のループにしない)", got)
	}
	if n := st.storedCount(); n != 1 {
		t.Errorf("保存された件数 = %d, want 1(冪等)", n)
	}
}

// EventID はストリームシーケンスだけから決まる(受信時刻・ランダム値を混ぜない)。
func TestEventIDIsStableAcrossRedeliveries(t *testing.T) {
	if EventID(42) != EventID(42) {
		t.Error("同じシーケンスで EventID が変わる(重複排除が効かない)")
	}
	if EventID(42) == EventID(43) {
		t.Error("違うシーケンスで EventID が同じ(別のイベントが重複扱いになる)")
	}
}

// AC-P3: 墓石。occurred_at <= purged_at のイベントは保存されず、ack される(再配送のループにしない)。
// AC-P4: occurred_at > purged_at のイベントは保存される。
func TestTombstoneDropsEarlierEvents(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)
	purgedAt := baseTime
	if _, err := st.PurgeDevice(context.Background(), deviceA, purgedAt); err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}

	tests := []struct {
		name       string
		occurredAt time.Time
		wantStored bool
	}{
		{"墓石より前", purgedAt.Add(-time.Minute), false},
		{"墓石とちょうど同時", purgedAt, false},
		{"墓石より後", purgedAt.Add(time.Minute), true},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := EventID(uint64(100 + i))
			if got := h.Handle(context.Background(), id, marshal(t, calcEvent(deviceA, tt.occurredAt))); got != Ack {
				t.Fatalf("Handle = %v, want Ack", got)
			}
			st.mu.Lock()
			outcome, ok := st.outcomes[id]
			st.mu.Unlock()
			if !ok {
				t.Fatalf("store.SaveCalcEvent が呼ばれていない")
			}
			want := store.Tombstoned
			if tt.wantStored {
				want = store.Stored
			}
			if outcome != want {
				t.Errorf("outcome = %v, want %v(occurred_at=%v purged_at=%v)", outcome, want, tt.occurredAt, purgedAt)
			}
		})
	}

	// 墓石より後のイベントだけが「保存された」状態で残ること(AC-P4)。
	st.mu.Lock()
	defer st.mu.Unlock()
	if p := st.purgedAt[deviceA]; !p.Equal(purgedAt) {
		t.Errorf("purgedAt = %v, want %v(イベント処理で墓石を動かさない)", p, purgedAt)
	}
}

// AC-R2c / AC-R5: イベント消費でも devices.last_seen_at を更新する(HTTP 要求が無い端末を
// 「未使用」と誤判定しないため)。更新は24時間に1回以下(store の責務)。
func TestConsumptionUpdatesLastSeenAt(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)

	// 1時間おきに3件 → last_seen_at は最初の1件だけで進む(24時間規則)。
	for i := 0; i < 3; i++ {
		id := EventID(uint64(200 + i))
		at := baseTime.Add(time.Duration(i) * time.Hour)
		if got := h.Handle(context.Background(), id, marshal(t, calcEvent(deviceA, at))); got != Ack {
			t.Fatalf("Handle = %v, want Ack", got)
		}
	}
	st.mu.Lock()
	got := st.lastSeenAt[deviceA]
	st.mu.Unlock()
	if !got.Equal(baseTime) {
		t.Errorf("last_seen_at = %v, want %v(24時間以内の2件目以降では書き換えない。AC-R5)", got, baseTime)
	}
	if got.IsZero() {
		t.Error("イベント消費で last_seen_at が更新されていない(AC-R2c)")
	}
}

// イベントは自分の端末のデータだけを触る(ADR-0209 §6-1)。
func TestEventsAreScopedToTheirDevice(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)

	h.Handle(context.Background(), EventID(300), marshal(t, calcEvent(deviceA, baseTime)))

	st.mu.Lock()
	defer st.mu.Unlock()
	for _, ev := range st.saved {
		if ev.DeviceID != deviceA {
			t.Errorf("端末 %q のイベントが保存された, want %q だけ", ev.DeviceID, deviceA)
		}
	}
	if _, ok := st.lastSeenAt[deviceB]; ok {
		t.Errorf("端末 B の last_seen_at が更新された")
	}
}

// DB に届かないときは Nak(後で再配送)。イベントを黙って捨てない。
// record-svc 自身が 503 になっても、この失敗はこのプロセスの中に収まる(calc-svc を呼ばない。
// CLAUDE.md 絶対ルール5: 計算 API は保存に依存しない)。
func TestStoreUnavailableIsNaked(t *testing.T) {
	st := newRecordingStore()
	st.unavailable = true
	h := NewHandler(st)

	if got := h.Handle(context.Background(), EventID(400), marshal(t, calcEvent(deviceA, baseTime))); got != Nak {
		t.Errorf("DB 到達不能のとき = %v, want Nak(再配送させる)", got)
	}
	if !errors.Is(fmt.Errorf("x: %w", store.ErrUnavailable), store.ErrUnavailable) {
		t.Fatal("テストの前提が壊れている")
	}
}

// 壊れた本文・未知の schemaVersion は Term(恒久的に処理できないので再配送させない)。
func TestUndecodableMessagesAreTerminated(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)

	future := calcEvent(deviceA, baseTime)
	future.SchemaVersion = calcevents.SchemaVersion + 1

	tests := []struct {
		name string
		data []byte
	}{
		{"JSON として壊れている", []byte(`{`)},
		{"配列", []byte(`[]`)},
		{"端末 ID が空", marshal(t, calcEvent("", baseTime))},
		{"未知の schemaVersion", marshal(t, future)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.Handle(context.Background(), EventID(500), tt.data); got != Term {
				t.Errorf("Handle = %v, want Term", got)
			}
		})
	}
	if n := st.storedCount(); n != 0 {
		t.Errorf("壊れたメッセージが %d 件保存された, want 0", n)
	}
}

// record-svc の durable consumer は team-svc と別で、CALC_EVENTS の全端末の subject を購読する
// (ADR-0209 §4・ADR-0212 §4)。名前を取り違えると配送が分かれてイベントを取りこぼす。
func TestConsumerIdentity(t *testing.T) {
	if DurableName == "" {
		t.Fatal("DurableName が空(durable consumer にならない)")
	}
	if want := calcevents.SubjectPrefix + "*"; SubjectFilter != want {
		t.Errorf("SubjectFilter = %q, want %q", SubjectFilter, want)
	}
	// team-svc の durable と同じ名前にしない(P5-4 は別名を使う)。
	for _, forbidden := range []string{"team", "TEAM"} {
		if DurableName == forbidden {
			t.Errorf("DurableName = %q は team-svc と衝突する", DurableName)
		}
	}
}
