package events

// JetStream の計算イベント購読(ADR-0209 §4・§7、ADR-0212 §6、ADR-0213 §5)の受け入れテスト。
// AC-P3 / AC-P4 / AC-R2d / AC-R5 / AC-R7 / AC-T8。
//
// 実装済み(record-svc の services/record/internal/events と同じ形。NATS 依存は subscriber.go に
// 分けてある)。この形を固定する:
//
//	// Action は1件のメッセージの処理後にすべきこと。
//	type Action int
//	const (
//		Ack  Action = iota // 処理済み(last_seen_at を更新した・24時間以内で書かなかった・墓石で捨てた)
//		Nak                // 一時的な失敗(DB に届かない)。後で再配送させる
//		Term               // 恒久的に処理できない(壊れた JSON・未知の schemaVersion)。再配送させない
//	)
//
//	// Handler は NATS に依存しない純粋な処理本体。consume ループはメッセージから eventID(ログ用)と
//	// 本文を取り出して Handle を呼び、戻り値の Action に従って msg.Ack() / Nak() / Term() を呼ぶ。
//	func NewHandler(st store.Store) *Handler
//	func (h *Handler) Handle(ctx context.Context, eventID string, data []byte) Action
//
//	// team-svc 専用の durable(record-svc とは**別**。同じ名前にすると配送が分かれて取りこぼす)。
//	const DurableName = "team-svc"
//	const SubjectFilter = calcevents.SubjectPrefix + "*"
//
// **eventID は重複排除に使わない**(ログの手がかりとして持つだけ)。team-svc がイベントから行うのは
// devices.last_seen_at の更新だけで、24時間規則(ADR-0209 §4)がそのまま冪等性になるため、
// record-svc のような event_id の一意制約を持たない(ADR-0213 §5)。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/team/internal/store"
)

const (
	deviceA   = "00000000-0000-4000-8000-00000000000a"
	deviceB   = "00000000-0000-4000-8000-00000000000b"
	sessionID = "00000000-0000-4000-8000-000000000051"
)

var baseTime = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

// touchCall は TouchDeviceFromEvent が受け取った引数(これ以外の情報が store に渡らないことを検査する)。
type touchCall struct {
	deviceID   string
	occurredAt time.Time
}

// recordingStore は store.Store の架空実装(events 用)。CRUD は使わないので最小限。
type recordingStore struct {
	mu sync.Mutex

	purgedAt   map[string]time.Time
	lastSeenAt map[string]time.Time

	touches  []touchCall
	outcomes []store.TouchOutcome
	// methods は呼ばれた store のメソッド名(イベント経路が CRUD を呼ばないことの検査)。
	methods []string

	unavailable bool
}

func newRecordingStore() *recordingStore {
	return &recordingStore{purgedAt: map[string]time.Time{}, lastSeenAt: map[string]time.Time{}}
}

func (s *recordingStore) note(method string) { s.methods = append(s.methods, method) }

func (s *recordingStore) TouchDevice(ctx context.Context, deviceID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("TouchDevice")
	return nil
}

func (s *recordingStore) TouchDeviceFromEvent(ctx context.Context, deviceID string, occurredAt time.Time) (store.TouchOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("TouchDeviceFromEvent")
	s.touches = append(s.touches, touchCall{deviceID: deviceID, occurredAt: occurredAt})
	if s.unavailable {
		return 0, fmt.Errorf("recordingStore: %w", store.ErrUnavailable)
	}
	var outcome store.TouchOutcome
	switch p, purged := s.purgedAt[deviceID]; {
	case purged && !occurredAt.After(p):
		outcome = store.Tombstoned
	case occurredAt.Sub(s.lastSeenAt[deviceID]) < 24*time.Hour:
		outcome = store.Skipped
	default:
		s.lastSeenAt[deviceID] = occurredAt
		outcome = store.Touched
	}
	s.outcomes = append(s.outcomes, outcome)
	return outcome, nil
}

func (s *recordingStore) ListTeams(ctx context.Context, deviceID string) ([]store.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("ListTeams")
	return nil, nil
}

func (s *recordingStore) GetTeam(ctx context.Context, deviceID, teamID string) (store.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("GetTeam")
	return store.Team{}, store.ErrNotFound
}

func (s *recordingStore) CreateTeam(ctx context.Context, deviceID string, t store.Team, now time.Time) (store.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("CreateTeam")
	return store.Team{}, nil
}

func (s *recordingStore) UpdateTeam(ctx context.Context, deviceID, teamID string, t store.Team, now time.Time) (store.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("UpdateTeam")
	return store.Team{}, store.ErrNotFound
}

func (s *recordingStore) DeleteTeam(ctx context.Context, deviceID, teamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("DeleteTeam")
	return store.ErrNotFound
}

func (s *recordingStore) PurgeDevice(ctx context.Context, deviceID string, now time.Time) (store.PurgeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note("PurgeDevice")
	s.purgedAt[deviceID] = now
	return store.PurgeResult{PurgedAt: now}, nil
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

// calcEvent は Detail 付きの計算イベント(team-svc は Detail を読まない)。
func calcEvent(deviceID string, at time.Time) calcevents.Event {
	return calcevents.Event{
		SchemaVersion: calcevents.SchemaVersion,
		DeviceID:      deviceID,
		SessionID:     sessionID,
		Operation:     calcevents.OperationCalc,
		OccurredAt:    at,
		Detail: &calcevents.CalcDetail{
			Format:     "single",
			MoveID:     forbiddenMoveID,
			Attacker:   api.Individual{SpeciesKey: forbiddenSpeciesKey, NatureId: forbiddenNatureID},
			Defender:   api.Individual{SpeciesKey: forbiddenSpeciesKey, NatureId: forbiddenNatureID},
			MinPercent: 41.5,
			MaxPercent: 49.25,
		},
	}
}

// Detail の中身(team-svc が保存もログ出力もしてはいけない値)。
const (
	forbiddenSpeciesKey = "9002-000"
	forbiddenNatureID   = "secret-nature"
	forbiddenMoveID     = "secret-move"
)

// AC-R2d / AC-R5: イベント消費で devices.last_seen_at を更新する(計算 API だけを使い続ける端末の
// 構築が誤って失効しないため)。更新は24時間に1回以下(store の責務)。
func TestConsumptionUpdatesLastSeenAt(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)

	// 1時間おきに3件 → last_seen_at は最初の1件だけで進む(24時間規則)。
	for i := 0; i < 3; i++ {
		at := baseTime.Add(time.Duration(i) * time.Hour)
		if got := h.Handle(context.Background(), eventID(i), marshal(t, calcEvent(deviceA, at))); got != Ack {
			t.Fatalf("Handle = %v, want Ack", got)
		}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	got := st.lastSeenAt[deviceA]
	if got.IsZero() {
		t.Fatal("イベント消費で last_seen_at が更新されていない(AC-R2d)")
	}
	if !got.Equal(baseTime) {
		t.Errorf("last_seen_at = %v, want %v(24時間以内の2件目以降では書き換えない。AC-R5)", got, baseTime)
	}
	if len(st.touches) != 3 {
		t.Errorf("TouchDeviceFromEvent の呼び出し = %d 回, want 3(抑止は store の責務。ハンドラは毎回呼ぶ)", len(st.touches))
	}
}

// AC-R7: at-least-once 配送(ADR-0212 §6)。同じイベントを2回受け取っても last_seen_at が
// 二重に進まず、2回目も ack される。team-svc は event_id の重複排除を持たず、
// 24時間規則がそのまま冪等性になる(ADR-0213 §5)。
func TestRedeliveryIsIdempotent(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)
	data := marshal(t, calcEvent(deviceA, baseTime))

	if got := h.Handle(context.Background(), eventID(42), data); got != Ack {
		t.Fatalf("1回目 = %v, want Ack", got)
	}
	if got := h.Handle(context.Background(), eventID(42), data); got != Ack {
		t.Errorf("再配送 = %v, want Ack(重複でも ack して再配送のループにしない)", got)
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if got := st.lastSeenAt[deviceA]; !got.Equal(baseTime) {
		t.Errorf("last_seen_at = %v, want %v(再配送で動かない)", got, baseTime)
	}
	if len(st.outcomes) != 2 || st.outcomes[0] != store.Touched || st.outcomes[1] != store.Skipped {
		t.Errorf("outcomes = %v, want [Touched Skipped]", st.outcomes)
	}
}

// AC-P3: 墓石。occurred_at <= purged_at のイベントは last_seen_at を進めず、ack される
// (再配送のループにしない)。AC-P4: occurred_at > purged_at のイベントは進める。
func TestTombstoneDropsEarlierEvents(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)
	purgedAt := baseTime
	if _, err := st.PurgeDevice(context.Background(), deviceA, purgedAt); err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}

	tests := []struct {
		name        string
		occurredAt  time.Time
		wantOutcome store.TouchOutcome
	}{
		{"墓石より前", purgedAt.Add(-time.Minute), store.Tombstoned},
		{"墓石とちょうど同時", purgedAt, store.Tombstoned},
		{"墓石より後", purgedAt.Add(25 * time.Hour), store.Touched},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.Handle(context.Background(), eventID(100+i), marshal(t, calcEvent(deviceA, tt.occurredAt))); got != Ack {
				t.Fatalf("Handle = %v, want Ack", got)
			}
			st.mu.Lock()
			defer st.mu.Unlock()
			if len(st.outcomes) == 0 {
				t.Fatal("store.TouchDeviceFromEvent が呼ばれていない")
			}
			if got := st.outcomes[len(st.outcomes)-1]; got != tt.wantOutcome {
				t.Errorf("outcome = %v, want %v(occurred_at=%v purged_at=%v)", got, tt.wantOutcome, tt.occurredAt, purgedAt)
			}
		})
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if p := st.purgedAt[deviceA]; !p.Equal(purgedAt) {
		t.Errorf("purgedAt = %v, want %v(イベント処理で墓石を動かさない)", p, purgedAt)
	}
}

// AC-T8(ADR-0209 §4・ADR-0213 §5): team-svc はイベントの中身(個体・ダメージ)を保存しない。
// store に渡すのは端末 ID と発生時刻だけで、CRUD のメソッドは呼ばない。
func TestConsumerPassesOnlyDeviceAndTime(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)

	if got := h.Handle(context.Background(), eventID(200), marshal(t, calcEvent(deviceA, baseTime))); got != Ack {
		t.Fatalf("Handle = %v, want Ack", got)
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	for _, m := range st.methods {
		if m != "TouchDeviceFromEvent" {
			t.Errorf("イベント処理で store.%s が呼ばれた(team-svc がイベントから行うのは"+
				"last_seen_at の更新だけ。ADR-0209 §4)", m)
		}
	}
	if len(st.touches) != 1 {
		t.Fatalf("TouchDeviceFromEvent = %d 回, want 1", len(st.touches))
	}
	if st.touches[0].deviceID != deviceA || !st.touches[0].occurredAt.Equal(baseTime) {
		t.Errorf("渡された引数 = %+v, want {deviceA baseTime}", st.touches[0])
	}
}

// AC-T8 の構造的な保証: store.Store の「イベント経路が呼ぶメソッド」は、端末 ID と時刻しか
// 受け取れない形であること(引数に計算の中身を運べる型が無い)。ここが緩むと、後からイベントの
// 中身を保存する実装を足せてしまう(ADR-0209 §4 違反)。
func TestTouchDeviceFromEventSignatureCarriesNoPayload(t *testing.T) {
	st := reflect.TypeOf((*store.Store)(nil)).Elem()
	m, ok := st.MethodByName("TouchDeviceFromEvent")
	if !ok {
		t.Fatal("store.Store に TouchDeviceFromEvent が無い")
	}
	want := []string{"context.Context", "string", "time.Time"}
	if got := m.Type.NumIn(); got != len(want) {
		t.Fatalf("引数の数 = %d, want %d(イベントの中身を渡す引数を足さない)", got, len(want))
	}
	for i, w := range want {
		if got := m.Type.In(i).String(); got != w {
			t.Errorf("引数 %d = %s, want %s", i, got, w)
		}
	}
}

// DB に届かないときは Nak(後で再配送)。イベントを黙って捨てない。
// team-svc 自身が止まっても、この失敗はこのプロセスの中に収まる(CLAUDE.md 絶対ルール5)。
func TestStoreUnavailableIsNaked(t *testing.T) {
	st := newRecordingStore()
	st.unavailable = true
	h := NewHandler(st)

	if got := h.Handle(context.Background(), eventID(300), marshal(t, calcEvent(deviceA, baseTime))); got != Nak {
		t.Errorf("DB 到達不能のとき = %v, want Nak(再配送させる)", got)
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
			if got := h.Handle(context.Background(), eventID(400), tt.data); got != Term {
				t.Errorf("Handle = %v, want Term", got)
			}
		})
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.touches) != 0 {
		t.Errorf("壊れたメッセージで store が %d 回呼ばれた, want 0", len(st.touches))
	}
}

// Detail の無いイベント(calcBulk / calcReverse)も同じように last_seen_at を進める
// (team-svc は Detail を使わないので、Operation で扱いを変えない)。
func TestEventsWithoutDetailAreHandled(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)
	ev := calcevents.Event{
		SchemaVersion: calcevents.SchemaVersion,
		DeviceID:      deviceA,
		SessionID:     sessionID,
		Operation:     calcevents.OperationBulk,
		OccurredAt:    baseTime,
	}
	if got := h.Handle(context.Background(), eventID(500), marshal(t, ev)); got != Ack {
		t.Fatalf("Handle = %v, want Ack", got)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if got := st.lastSeenAt[deviceA]; !got.Equal(baseTime) {
		t.Errorf("last_seen_at = %v, want %v", got, baseTime)
	}
}

// イベントは自分の端末のデータだけを触る(ADR-0209 §6-1)。
func TestEventsAreScopedToTheirDevice(t *testing.T) {
	st := newRecordingStore()
	h := NewHandler(st)

	h.Handle(context.Background(), eventID(600), marshal(t, calcEvent(deviceA, baseTime)))

	st.mu.Lock()
	defer st.mu.Unlock()
	for _, c := range st.touches {
		if c.deviceID != deviceA {
			t.Errorf("端末 %q で呼ばれた, want %q だけ", c.deviceID, deviceA)
		}
	}
	if _, ok := st.lastSeenAt[deviceB]; ok {
		t.Error("端末 B の last_seen_at が更新された")
	}
}

// AC-L1(イベント経路): 計算の中身をログに出さない(ADR-0209 §3)。
func TestConsumerLogsDoNotLeakDetail(t *testing.T) {
	buf := captureLogs(t)
	st := newRecordingStore()
	h := NewHandler(st)

	h.Handle(context.Background(), eventID(700), marshal(t, calcEvent(deviceA, baseTime)))
	h.Handle(context.Background(), eventID(701), []byte(`{`)) // 失敗の経路も見る
	st.unavailable = true
	h.Handle(context.Background(), eventID(702), marshal(t, calcEvent(deviceA, baseTime.Add(48*time.Hour))))

	logs := buf.String()
	for _, forbidden := range []string{forbiddenSpeciesKey, forbiddenNatureID, forbiddenMoveID, "41.5", "49.25", `"detail"`} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("ログに計算の中身 %q が出ている(ADR-0209 §3):\n%s", forbidden, logs)
		}
	}
}

// team-svc の durable consumer は record-svc と**別**で、CALC_EVENTS の全端末の subject を購読する
// (ADR-0209 §4・ADR-0212 §4)。同じ durable 名を使うと配送が分かれてイベントを取りこぼす。
func TestConsumerIdentity(t *testing.T) {
	if DurableName == "" {
		t.Fatal("DurableName が空(durable consumer にならない)")
	}
	if DurableName == "record-svc" {
		t.Error("DurableName が record-svc と同じ(配送が分かれて取りこぼす。ADR-0209 §4)")
	}
	if !strings.Contains(DurableName, "team") {
		t.Errorf("DurableName = %q(team-svc のものと分かる名前にする)", DurableName)
	}
	if want := calcevents.SubjectPrefix + "*"; SubjectFilter != want {
		t.Errorf("SubjectFilter = %q, want %q", SubjectFilter, want)
	}
}

// eventID はテストが使うログ用の識別子(重複排除には使わない。ADR-0213 §5)。
func eventID(n int) string { return fmt.Sprintf("calc-events-%d", n) }

// captureLogs は slog の既定ロガーをバッファに差し替え、テストの終わりに戻す。
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}
