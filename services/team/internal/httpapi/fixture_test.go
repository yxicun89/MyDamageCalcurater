package httpapi

// team-svc の httpapi テストの共通部品(record-svc の同名ファイルと同じ流儀)。
//
// 実装済み。`NewHandler` はこの形を固定する(record-svc の NewHandler と同じ形):
//
//	// NewHandler は team-svc の HTTP ハンドラ全体を組み立てる(record-svc の NewHandler と同じ形)。
//	//   GET    /api/team/teams            → ListTeams(生成ラッパ経由)
//	//   POST   /api/team/teams            → CreateTeam
//	//   GET    /api/team/teams/{teamId}   → GetTeam
//	//   PUT    /api/team/teams/{teamId}   → UpdateTeam
//	//   DELETE /api/team/teams/{teamId}   → DeleteTeam
//	//   DELETE /api/team/device-data      → DeleteTeamDeviceData
//	//   GET    /healthz                   → 常に 200(DB に触れない)
//	//   GET    /readyz                    → DB に届けば 200、届かなければ 503 store_unavailable
//	//   calc / pokedex / record / internal の操作 → ルートに登録せず 404 not_found
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
	"example.com/pokecalc/services/team/internal/store"
)

// 架空の端末 ID / セッション ID(版4の形。gateway が検証済みの値が来る前提)。
const (
	deviceA   = "00000000-0000-4000-8000-00000000000a"
	deviceB   = "00000000-0000-4000-8000-00000000000b"
	sessionID = "00000000-0000-4000-8000-000000000051"
)

// 架空のマスタ ID(pokedex のマスタには依存しない。team-svc は ID をそのまま保存するだけ。ADR-0213 §3)。
const (
	speciesGuard = "9002-000"
	speciesLeaf  = "9003-000"
	natureID     = "test-nature"
	itemID       = "test-item"
	abilityID    = "test-ability"
	moveIDA      = "test-move-a"
	moveIDB      = "test-move-b"
)

// パス。
const (
	pathTeams      = "/api/team/teams"
	pathDeviceData = "/api/team/device-data"
)

// deviceState は fakeStore が端末ごとに持つ状態。
type deviceState struct {
	lastSeenAt time.Time
	purgedAt   time.Time // ゼロ値なら墓石なし
	teams      []store.Team
}

// fakeStore は store.Store の架空実装。端末ごとに状態を分けて持ち、
// 「他端末のデータに触れていないこと」をテストから検査できるようにする。
type fakeStore struct {
	mu sync.Mutex

	devices map[string]*deviceState

	// now は fake の時計。呼ばれるたびに tick 進む(purgedAt・createdAt・updatedAt に使う)。
	now  time.Time
	tick time.Duration

	// nextID は CreateTeam が発行する ID の連番(UUID の形をした決定的な値)。
	nextID int

	// teamLimit は1端末が持てる構築の上限(既定は store.MaxTeamsPerDevice)。
	teamLimit int

	// purgeLimit は1回の PurgeDevice で消せる行数の上限(AC-P2 の partial を作るため)。
	purgeLimit int

	// unavailable が true なら全メソッドが store.ErrUnavailable を返す(store_unavailable)。
	unavailable bool
	// unavailableAfterTombstone が true なら、墓石を立てた**後**に ErrUnavailable を返す(AC-P6)。
	unavailableAfterTombstone bool

	// calls は呼ばれたメソッドと端末 ID の記録(越境の検知に使う)。
	calls []storeCall
}

type storeCall struct {
	method   string
	deviceID string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		devices:    map[string]*deviceState{},
		now:        time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
		tick:       time.Second,
		teamLimit:  store.MaxTeamsPerDevice,
		purgeLimit: 1000,
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

func (f *fakeStore) unavailableErr() error {
	return fmt.Errorf("fake: %w", store.ErrUnavailable)
}

func (f *fakeStore) TouchDevice(ctx context.Context, deviceID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("TouchDevice", deviceID)
	if f.unavailable {
		return f.unavailableErr()
	}
	d := f.state(deviceID)
	if now.Sub(d.lastSeenAt) < 24*time.Hour {
		return nil // 24時間規則(AC-R5)。fake 側も同じ規則で書かない。
	}
	d.lastSeenAt = now
	return nil
}

func (f *fakeStore) TouchDeviceFromEvent(ctx context.Context, deviceID string, occurredAt time.Time) (store.TouchOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("TouchDeviceFromEvent", deviceID)
	if f.unavailable {
		return 0, f.unavailableErr()
	}
	d := f.state(deviceID)
	if !d.purgedAt.IsZero() && !occurredAt.After(d.purgedAt) {
		return store.Tombstoned, nil
	}
	if occurredAt.Sub(d.lastSeenAt) < 24*time.Hour {
		return store.Skipped, nil
	}
	d.lastSeenAt = occurredAt
	return store.Touched, nil
}

func (f *fakeStore) ListTeams(ctx context.Context, deviceID string) ([]store.Team, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ListTeams", deviceID)
	if f.unavailable {
		return nil, f.unavailableErr()
	}
	out := append([]store.Team(nil), f.state(deviceID).teams...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) GetTeam(ctx context.Context, deviceID, teamID string) (store.Team, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("GetTeam", deviceID)
	if f.unavailable {
		return store.Team{}, f.unavailableErr()
	}
	for _, t := range f.state(deviceID).teams {
		if t.ID == teamID {
			return t, nil
		}
	}
	return store.Team{}, store.ErrNotFound
}

func (f *fakeStore) CreateTeam(ctx context.Context, deviceID string, t store.Team, now time.Time) (store.Team, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("CreateTeam", deviceID)
	if f.unavailable {
		return store.Team{}, f.unavailableErr()
	}
	d := f.state(deviceID)
	if len(d.teams) >= f.teamLimit {
		return store.Team{}, store.ErrTeamLimitReached
	}
	at := f.advance()
	f.nextID++
	t.ID = fakeTeamID(f.nextID)
	t.CreatedAt, t.UpdatedAt = at, at
	d.teams = append(d.teams, t)
	return t, nil
}

func (f *fakeStore) UpdateTeam(ctx context.Context, deviceID, teamID string, t store.Team, now time.Time) (store.Team, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("UpdateTeam", deviceID)
	if f.unavailable {
		return store.Team{}, f.unavailableErr()
	}
	d := f.state(deviceID)
	for i := range d.teams {
		if d.teams[i].ID != teamID {
			continue
		}
		at := f.advance()
		d.teams[i].Name = t.Name
		d.teams[i].Members = t.Members
		d.teams[i].UpdatedAt = at
		return d.teams[i], nil
	}
	return store.Team{}, store.ErrNotFound
}

func (f *fakeStore) DeleteTeam(ctx context.Context, deviceID, teamID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("DeleteTeam", deviceID)
	if f.unavailable {
		return f.unavailableErr()
	}
	d := f.state(deviceID)
	for i := range d.teams {
		if d.teams[i].ID == teamID {
			d.teams = append(d.teams[:i], d.teams[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) PurgeDevice(ctx context.Context, deviceID string, now time.Time) (store.PurgeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("PurgeDevice", deviceID)
	if f.unavailable {
		return store.PurgeResult{}, f.unavailableErr()
	}
	d := f.state(deviceID)

	// (1) 墓石 + journal(ADR-0209 §5.2 の順序。呼ばれるたびに現在時刻へ更新する)。
	d.purgedAt = f.advance()
	if f.unavailableAfterTombstone {
		return store.PurgeResult{}, f.unavailableErr()
	}

	budget := f.purgeLimit
	var del store.Deleted
	// (2) team_members → (3) teams の順に上限まで消す。
	for i := range d.teams {
		for len(d.teams[i].Members) > 0 && budget > 0 {
			d.teams[i].Members = d.teams[i].Members[1:]
			del.TeamMembers++
			budget--
		}
	}
	var kept []store.Team
	for _, t := range d.teams {
		if len(t.Members) == 0 && budget > 0 {
			del.Teams++
			budget--
			continue
		}
		kept = append(kept, t)
	}
	d.teams = kept

	return store.PurgeResult{PurgedAt: d.purgedAt, Deleted: del, Remaining: len(kept) > 0}, nil
}

// fakeTeamID は決定的な「UUID の形をした」ID(契約の TeamId のパターンに合わせる)。
func fakeTeamID(n int) string {
	return fmt.Sprintf("11111111-2222-4333-8444-%012d", n)
}

// seed は端末に構築を積む(名前とメンバー数だけを決め、中身は問わない)。戻り値は作った ID。
func (f *fakeStore) seed(deviceID string, teams, membersPerTeam int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.state(deviceID)
	var ids []string
	for i := 0; i < teams; i++ {
		f.nextID++
		at := f.advance()
		t := store.Team{
			ID:        fakeTeamID(f.nextID),
			Name:      fmt.Sprintf("構築%d", i+1),
			CreatedAt: at,
			UpdatedAt: at,
		}
		for j := 0; j < membersPerTeam; j++ {
			t.Members = append(t.Members, store.Member{
				SpeciesKey: speciesGuard, NatureID: natureID, MoveIDs: []string{moveIDA},
			})
		}
		d.teams = append(d.teams, t)
		ids = append(ids, t.ID)
	}
	return ids
}

// rowsLeft はその端末に残っている行数の合計(teams + team_members)。
func (f *fakeStore) rowsLeft(deviceID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, t := range f.state(deviceID).teams {
		n += 1 + len(t.Members)
	}
	return n
}

// --- リクエスト本文の組み立て ------------------------------------------------

// memberJSON は契約に合う最小のメンバー(必須項目だけ)。
func memberJSON() map[string]any {
	return map[string]any{
		"speciesKey": speciesGuard,
		"natureId":   natureID,
		"sp":         spJSON(0, 0, 0, 0, 0, 0),
	}
}

func spJSON(hp, atk, def, spa, spd, spe int) map[string]any {
	return map[string]any{"hp": hp, "atk": atk, "def": def, "spa": spa, "spd": spd, "spe": spe}
}

// teamJSON は TeamInput の本文を組み立てる。
func teamJSON(name string, members ...map[string]any) map[string]any {
	if members == nil {
		members = []map[string]any{}
	}
	return map[string]any{"name": name, "members": members}
}

// body は JSON に直す(テストの中で組み立てた map をそのまま送るため)。
func body(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("リクエスト本文を組み立てられない: %v", err)
	}
	return b
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
func serve(t *testing.T, h http.Handler, method, path string, hdr http.Header, reqBody []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if reqBody == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewReader(reqBody))
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

// decodeTeam は 200 / 201 の本文を Team として読む。
func decodeTeam(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) api.Team {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var got api.Team
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Team として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}

// decodeTeams は 200 の本文を Team の配列として読む。
func decodeTeams(t *testing.T, rec *httptest.ResponseRecorder) []api.Team {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got []api.Team
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Team の配列として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}

// decodeDeletion は 200 の本文を TeamDeletionResult として読む。
func decodeDeletion(t *testing.T, rec *httptest.ResponseRecorder) api.TeamDeletionResult {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got api.TeamDeletionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("TeamDeletionResult として読めない: %v; body=%s", err, rec.Body.String())
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
