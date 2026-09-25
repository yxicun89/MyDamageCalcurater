package httpapi

// record-svc の httpapi テストの共通部品。
//
// **test-first(ADR-0003)**: 実装済み(`server.go`)。以下は spec-writer が最初に定めた
// `NewHandler` の形(実装もこのとおり):
//
//	// NewHandler は record-svc の HTTP ハンドラ全体を組み立てる(calc-svc の NewHandler と同じ形)。
//	//   GET    /api/record/frequent-opponents  → ListFrequentOpponents(生成ラッパ経由)
//	//   DELETE /api/record/device-data         → DeleteRecordDeviceData(生成ラッパ経由)
//	//   GET    /healthz                        → 常に 200(DB に触れない)
//	//   GET    /readyz                         → DB に届けば 200、届かなければ 503 store_unavailable
//	//   calc / pokedex / internal の操作        → ルートに登録せず 404 not_found
//	func NewHandler(st store.Store) http.Handler
//
// 端末 ID はヘッダ(X-Device-Id)だけが正で、ハンドラは store のメソッドにその値だけを渡す
// (ADR-0209 §2・§6-1)。fake は「どの端末 ID で呼ばれたか」を記録し、越境を検知できるようにする。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

// 架空の端末 ID / セッション ID(版4の形。gateway が検証済みの値が来る前提)。
const (
	deviceA   = "00000000-0000-4000-8000-00000000000a"
	deviceB   = "00000000-0000-4000-8000-00000000000b"
	sessionID = "00000000-0000-4000-8000-000000000051"
)

// 架空の種族(pokedex のマスタには依存しない。record-svc は speciesKey を返すだけ)。
const (
	speciesGuard = "9002-000"
	speciesLeaf  = "9003-000"
)

// deviceState は fakeStore が端末ごとに持つ状態。
type deviceState struct {
	lastSeenAt time.Time
	purgedAt   time.Time // ゼロ値なら墓石なし
	events     []store.CalcEvent
	aggregates []store.FrequentOpponent
	favorites  int
}

// fakeStore は store.Store の架空実装。端末ごとに状態を分けて持ち、
// 「他端末のデータに触れていないこと」をテストから検査できるようにする。
type fakeStore struct {
	mu sync.Mutex

	devices map[string]*deviceState

	// now は fake の時計(PurgeDevice が返す purgedAt に使う)。呼ばれるたびに tick 進む。
	now  time.Time
	tick time.Duration

	// purgeLimit は1回の PurgeDevice で消せる行数の上限(AC-P2 の partial を作るため)。
	purgeLimit int

	// unavailable が true なら全メソッドが store.ErrUnavailable を返す(AC-P6・store_unavailable)。
	unavailable bool
	// unavailableAfterTombstone が true なら、墓石を立てた**後**に ErrUnavailable を返す(AC-P6)。
	unavailableAfterTombstone bool

	// calls は呼ばれたメソッドと端末 ID の記録(越境の検知に使う)。
	calls []storeCall
	// seenEventIDs は SaveCalcEvent の重複排除の記録。
	seenEventIDs map[string]bool
}

type storeCall struct {
	method   string
	deviceID string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		devices:      map[string]*deviceState{},
		now:          time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
		tick:         time.Second,
		purgeLimit:   1000,
		seenEventIDs: map[string]bool{},
	}
}

func (f *fakeStore) state(deviceID string) *deviceState {
	d, ok := f.devices[deviceID]
	if !ok {
		d = &deviceState{}
		f.devices[deviceID] = d
	}
	return d
}

func (f *fakeStore) record(method, deviceID string) {
	f.calls = append(f.calls, storeCall{method: method, deviceID: deviceID})
}

// deviceIDsTouched は fake が呼ばれた端末 ID の集合(重複なし・昇順)。
func (f *fakeStore) deviceIDsTouched() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, c := range f.calls {
		if !seen[c.deviceID] {
			seen[c.deviceID] = true
			out = append(out, c.deviceID)
		}
	}
	sort.Strings(out)
	return out
}

func (f *fakeStore) advance() time.Time {
	f.now = f.now.Add(f.tick)
	return f.now
}

func (f *fakeStore) TouchDevice(ctx context.Context, deviceID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("TouchDevice", deviceID)
	if f.unavailable {
		return fmt.Errorf("fake: %w", store.ErrUnavailable)
	}
	d := f.state(deviceID)
	if now.Sub(d.lastSeenAt) < 24*time.Hour {
		return nil // 24時間規則(AC-R5)。fake 側も同じ規則で書かない。
	}
	d.lastSeenAt = now
	return nil
}

func (f *fakeStore) FrequentOpponents(ctx context.Context, deviceID string, limit int) ([]store.FrequentOpponent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("FrequentOpponents", deviceID)
	if f.unavailable {
		return nil, fmt.Errorf("fake: %w", store.ErrUnavailable)
	}
	rows := append([]store.FrequentOpponent(nil), f.state(deviceID).aggregates...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].SpeciesKey < rows[j].SpeciesKey
	})
	if limit >= 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (f *fakeStore) PurgeDevice(ctx context.Context, deviceID string, now time.Time) (store.PurgeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("PurgeDevice", deviceID)
	if f.unavailable {
		return store.PurgeResult{}, fmt.Errorf("fake: %w", store.ErrUnavailable)
	}
	d := f.state(deviceID)

	// (1) 墓石 + journal(ADR-0209 §5.2 の順序。呼ばれるたびに現在時刻へ更新する)。
	d.purgedAt = f.advance()
	if f.unavailableAfterTombstone {
		return store.PurgeResult{}, fmt.Errorf("fake: %w", store.ErrUnavailable)
	}

	budget := f.purgeLimit
	var del store.Deleted
	// (2) calc_events → (3) 集計 → (4) favorites の順に上限まで消す。
	take := func(n int) int {
		if n > budget {
			n = budget
		}
		budget -= n
		return n
	}
	n := take(len(d.events))
	d.events = d.events[n:]
	del.CalcEvents = n

	n = take(len(d.aggregates))
	d.aggregates = d.aggregates[n:]
	del.Aggregates = n

	n = take(d.favorites)
	d.favorites -= n
	del.Favorites = n

	remaining := len(d.events)+len(d.aggregates)+d.favorites > 0
	return store.PurgeResult{PurgedAt: d.purgedAt, Deleted: del, Remaining: remaining}, nil
}

func (f *fakeStore) SaveCalcEvent(ctx context.Context, ev store.CalcEvent) (store.SaveOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("SaveCalcEvent", ev.DeviceID)
	if f.unavailable {
		return 0, fmt.Errorf("fake: %w", store.ErrUnavailable)
	}
	if f.seenEventIDs[ev.EventID] {
		return store.Duplicate, nil
	}
	d := f.state(ev.DeviceID)
	if !d.purgedAt.IsZero() && !ev.OccurredAt.After(d.purgedAt) {
		f.seenEventIDs[ev.EventID] = true
		return store.Tombstoned, nil
	}
	f.seenEventIDs[ev.EventID] = true
	d.events = append(d.events, ev)
	if ev.DefenderSpeciesKey != "" {
		f.bumpAggregate(d, ev)
	}
	if ev.OccurredAt.Sub(d.lastSeenAt) >= 24*time.Hour {
		d.lastSeenAt = ev.OccurredAt
	}
	return store.Stored, nil
}

func (f *fakeStore) bumpAggregate(d *deviceState, ev store.CalcEvent) {
	for i := range d.aggregates {
		if d.aggregates[i].SpeciesKey == ev.DefenderSpeciesKey {
			d.aggregates[i].Count++
			d.aggregates[i].Score++
			d.aggregates[i].LastCalculatedAt = ev.OccurredAt
			return
		}
	}
	d.aggregates = append(d.aggregates, store.FrequentOpponent{
		SpeciesKey: ev.DefenderSpeciesKey, Score: 1, Count: 1, LastCalculatedAt: ev.OccurredAt,
	})
}

// seed は端末にデータを積む(件数だけを決め、中身は問わない)。
func (f *fakeStore) seed(deviceID string, events, aggregates, favorites int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.state(deviceID)
	for i := 0; i < events; i++ {
		d.events = append(d.events, store.CalcEvent{EventID: fmt.Sprintf("%s-%d", deviceID, i), DeviceID: deviceID})
	}
	for i := 0; i < aggregates; i++ {
		d.aggregates = append(d.aggregates, store.FrequentOpponent{
			SpeciesKey: fmt.Sprintf("90%02d-000", i), Score: float64(aggregates - i), Count: 1,
			LastCalculatedAt: f.now,
		})
	}
	d.favorites = favorites
}

// rowsLeft はその端末に残っている行数の合計。
func (f *fakeStore) rowsLeft(deviceID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.state(deviceID)
	return len(d.events) + len(d.aggregates) + d.favorites
}

// --- HTTP の呼び出しヘルパ ----------------------------------------------------

// headers は gateway が検証済みの正しいヘッダ。
func headers(deviceID string) http.Header {
	h := http.Header{}
	h.Set("X-Device-Id", deviceID)
	h.Set("X-Session-Id", sessionID)
	return h
}

// serve は1リクエストを処理して記録を返す。
func serve(t *testing.T, h http.Handler, method, path string, hdr http.Header, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeDeletion は 200 の本文を RecordDeletionResult として読む。
func decodeDeletion(t *testing.T, rec *httptest.ResponseRecorder) api.RecordDeletionResult {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got api.RecordDeletionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("RecordDeletionResult として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}

// assertErrorBody は Error 形式の本文が期待の code であることを確かめる。
func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode api.ErrorCode) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var got api.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Error として読めない: %v; body=%s", err, rec.Body.String())
	}
	if got.Code != wantCode {
		t.Errorf("code = %q, want %q; message=%q", got.Code, wantCode, got.Message)
	}
	if got.Message == "" {
		t.Error("message が空")
	}
}
