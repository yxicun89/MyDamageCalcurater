package expire_test

// team-svc の失効ジョブ(ADR-0209 §4・ADR-0211 §7・ADR-0213・ADR-0220 §3・§4)の手順を、偽ストアで検査する(make test)。
//
// **test-first(ADR-0003)**: このパッケージ(services/team/internal/expire)はまだ無い。実装者は次の形で作る:
//
//	type Policy struct {
//		Retention             time.Duration // TEAM_RETENTION_DAYS
//		DeviceRowExpiry       time.Duration // TEAM_DEVICE_ROW_EXPIRY_DAYS(JetStream max_age 7日より大きい)
//		PurgeJournalRetention time.Duration // TEAM_PURGE_JOURNAL_RETENTION_DAYS
//		BatchLimit            int           // TEAM_EXPIRE_BATCH_LIMIT(構築の件数で数える)
//	}
//	func (p Policy) Validate() error
//
//	type Result struct {
//		Teams, TeamMembers, PurgeJournal, Devices int
//		OrphansMarked, OrphansCleared             int
//		Remaining                                 bool
//	}
//
//	type Store interface {
//		// max(devices.last_seen_at, teams.updated_at) < cutoff の構築を最大 limit 件、そのメンバーと一緒に消す
//		// (メンバーを先に・同じトランザクションで)。消した構築の数とメンバーの数を返す。
//		DeleteInactiveTeams(ctx context.Context, cutoff time.Time, limit int) (teams, members int, err error)
//		DeletePurgeJournal(ctx context.Context, cutoff time.Time, limit int) (int, error)
//		MarkOrphans(ctx context.Context, now time.Time, limit int) (marked, cleared int, err error)
//		DeleteOrphanDevices(ctx context.Context, cutoff time.Time, limit int) (int, error)
//	}
//
//	func Run(ctx context.Context, st Store, p Policy, now time.Time, log *slog.Logger) (Result, error)
//	func NewTiDB(db *sql.DB) Store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/team/internal/expire"
)

const day = 24 * time.Hour

// ADR-0211 §7 の既定値。
var defaultPolicy = expire.Policy{
	Retention:             540 * day,
	DeviceRowExpiry:       30 * day,
	PurgeJournalRetention: 90 * day,
	BatchLimit:            1000,
}

const (
	deviceA = "00000000-0000-4000-8000-00000000000a"
	deviceB = "00000000-0000-4000-8000-00000000000b"
)

var fixedNow = time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)

type team struct {
	id        string
	device    string
	updatedAt time.Time
	members   int
}

type row struct {
	id     string
	device string
	ts     time.Time
}

type call struct {
	name   string
	cutoff time.Time
	limit  int
}

// fakeStore は Store の契約どおり(判定時刻 < cutoff を最大 limit 件)に振る舞う。
type fakeStore struct {
	teams    []team
	journal  []row
	lastSeen map[string]time.Time
	calls    []call
	failOn   string
}

var errBoom = errors.New("boom: TiDB に届かない")

func newFake() *fakeStore { return &fakeStore{lastSeen: map[string]time.Time{}} }

func (f *fakeStore) note(name string, cutoff time.Time, limit int) error {
	f.calls = append(f.calls, call{name, cutoff, limit})
	if limit <= 0 {
		return errors.New("fake: limit <= 0 で呼ばれた(上限が尽きた段は呼ばないこと)")
	}
	if f.failOn == name {
		return errBoom
	}
	return nil
}

func (f *fakeStore) DeleteInactiveTeams(_ context.Context, cutoff time.Time, limit int) (int, int, error) {
	if err := f.note("DeleteInactiveTeams", cutoff, limit); err != nil {
		return 0, 0, err
	}
	var kept []team
	n, members := 0, 0
	for _, tm := range f.teams {
		basis := tm.updatedAt
		if seen, ok := f.lastSeen[tm.device]; ok && seen.After(basis) {
			basis = seen
		}
		if n < limit && basis.Before(cutoff) {
			n++
			members += tm.members
			continue
		}
		kept = append(kept, tm)
	}
	f.teams = kept
	return n, members, nil
}

func (f *fakeStore) DeletePurgeJournal(_ context.Context, cutoff time.Time, limit int) (int, error) {
	if err := f.note("DeletePurgeJournal", cutoff, limit); err != nil {
		return 0, err
	}
	var kept []row
	n := 0
	for _, r := range f.journal {
		if n < limit && r.ts.Before(cutoff) {
			n++
			continue
		}
		kept = append(kept, r)
	}
	f.journal = kept
	return n, nil
}

func (f *fakeStore) MarkOrphans(_ context.Context, now time.Time, limit int) (int, int, error) {
	return 0, 0, f.note("MarkOrphans", now, limit)
}

func (f *fakeStore) DeleteOrphanDevices(_ context.Context, cutoff time.Time, limit int) (int, error) {
	return 0, f.note("DeleteOrphanDevices", cutoff, limit)
}

func (f *fakeStore) callNames() []string {
	var names []string
	for _, c := range f.calls {
		names = append(names, c.name)
	}
	return names
}

func (f *fakeStore) callOf(t *testing.T, name string) call {
	t.Helper()
	for _, c := range f.calls {
		if c.name == name {
			return c
		}
	}
	t.Fatalf("%s が呼ばれていない(呼ばれた順: %v)", name, f.callNames())
	return call{}
}

func teamIDs(ts []team) []string {
	var out []string
	for _, tm := range ts {
		out = append(out, tm.id)
	}
	slices.Sort(out)
	return out
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

func run(t *testing.T, f *fakeStore, p expire.Policy, now time.Time) expire.Result {
	t.Helper()
	res, err := expire.Run(context.Background(), f, p, now, discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

// AC-E1
func TestRunPassesExactCutoffs(t *testing.T) {
	f := newFake()
	run(t, f, defaultPolicy, fixedNow)
	want := []string{"DeleteInactiveTeams", "DeletePurgeJournal", "MarkOrphans", "DeleteOrphanDevices"}
	if got := f.callNames(); !slices.Equal(got, want) {
		t.Fatalf("呼ばれた順 = %v, want %v", got, want)
	}
	cases := map[string]time.Time{
		"DeleteInactiveTeams": fixedNow.Add(-defaultPolicy.Retention),
		"DeletePurgeJournal":  fixedNow.Add(-defaultPolicy.PurgeJournalRetention),
		"MarkOrphans":         fixedNow,
		"DeleteOrphanDevices": fixedNow.Add(-defaultPolicy.DeviceRowExpiry),
	}
	for name, want := range cases {
		if got := f.callOf(t, name).cutoff; !got.Equal(want) {
			t.Errorf("%s の cutoff = %s, want %s", name, got, want)
		}
	}
}

// AC-E2・AC-E3(AC-R2・AC-R2b・AC-R2d)
func TestRunTeamsBoundaryUsesMaxOfLastSeenAndUpdatedAt(t *testing.T) {
	const (
		devOld    = "00000000-0000-4000-8000-0000000000c1"
		devActive = "00000000-0000-4000-8000-0000000000c2" // 計算イベントの購読で last_seen が新しい(AC-R2d)
		devNone   = "00000000-0000-4000-8000-0000000000c3" // devices 行が無い
	)
	ago := func(d time.Duration) time.Time { return fixedNow.Add(-d) }
	f := newFake()
	f.lastSeen[devOld] = ago(541 * day)
	f.lastSeen[devActive] = ago(time.Hour)
	f.teams = []team{
		{"old/541d", devOld, ago(541 * day), 6},
		{"old/539d", devOld, ago(539 * day), 3},
		{"old/540d", devOld, ago(540 * day), 1},
		{"old/540d+1s", devOld, ago(540*day + time.Second), 2},
		{"active/900d", devActive, ago(900 * day), 6},
		{"none/541d", devNone, ago(541 * day), 0},
		{"none/540d", devNone, ago(540 * day), 0},
	}
	res := run(t, f, defaultPolicy, fixedNow)

	want := []string{"active/900d", "none/540d", "old/539d", "old/540d"}
	if got := teamIDs(f.teams); !slices.Equal(got, want) {
		t.Errorf("残った構築 = %v, want %v", got, want)
	}
	if res.Teams != 3 || res.TeamMembers != 8 {
		t.Errorf("Result = %+v, want Teams 3・TeamMembers 8", res)
	}
}

func TestRunPurgeJournalBoundary(t *testing.T) {
	f := newFake()
	f.journal = []row{{"90d", deviceA, fixedNow.Add(-90 * day)}, {"90d+1s", deviceB, fixedNow.Add(-90*day - time.Second)}}
	res := run(t, f, defaultPolicy, fixedNow)
	if len(f.journal) != 1 || f.journal[0].id != "90d" || res.PurgeJournal != 1 {
		t.Errorf("残った journal = %+v・Result = %+v, want 90d だけ残り PurgeJournal 1", f.journal, res)
	}
}

// AC-E4: 上限は構築の件数で数え、段をまたいで共有する。
func TestRunBatchLimitThenIdempotent(t *testing.T) {
	f := newFake()
	for i := range 5 {
		f.teams = append(f.teams, team{"t" + strconv.Itoa(i), deviceA, fixedNow.Add(-1000 * day), 6})
	}
	f.journal = []row{{"j", deviceA, fixedNow.Add(-1000 * day)}}
	p := defaultPolicy
	p.BatchLimit = 3

	first := run(t, f, p, fixedNow)
	if first.Teams != 3 || first.TeamMembers != 18 || first.PurgeJournal != 0 || !first.Remaining {
		t.Fatalf("1回目 = %+v, want Teams 3・TeamMembers 18(メンバーは上限に数えない)・PurgeJournal 0・Remaining true", first)
	}
	for _, c := range f.calls {
		if c.name == "DeletePurgeJournal" || c.name == "DeleteOrphanDevices" {
			t.Errorf("上限が尽きた後に %s が呼ばれた", c.name)
		}
	}
	second := run(t, f, p, fixedNow)
	// 2回目: 構築 2 件で上限の残りは 1、journal がちょうど 1 件で上限を使い切る → 保守的に Remaining true(ADR-0220 §4)。
	if second.Teams != 2 || second.PurgeJournal != 1 || !second.Remaining {
		t.Fatalf("2回目 = %+v, want Teams 2・PurgeJournal 1・Remaining true(上限を使い切った)", second)
	}
	if third := run(t, f, p, fixedNow); third != (expire.Result{}) {
		t.Fatalf("3回目 = %+v, want 全て 0(冪等)", third)
	}
}

// AC-E7
func TestRunStopsAtFirstStoreError(t *testing.T) {
	f := newFake()
	f.failOn = "DeleteInactiveTeams"
	if _, err := expire.Run(context.Background(), f, defaultPolicy, fixedNow, discardLogger()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom を包んだエラー", err)
	}
	if got := f.callNames(); !slices.Equal(got, []string{"DeleteInactiveTeams"}) {
		t.Errorf("失敗後に後の段が呼ばれた: %v", got)
	}
}

// AC-E8
func TestRunLogsCountsWithoutDeviceIDs(t *testing.T) {
	f := newFake()
	f.teams = []team{{"t", deviceA, fixedNow.Add(-1000 * day), 2}}
	var buf bytes.Buffer
	if _, err := expire.Run(context.Background(), f, defaultPolicy, fixedNow, slog.New(slog.NewJSONHandler(&buf, nil))); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), deviceA) {
		t.Errorf("ログに端末 ID が出ている: %s", buf.String())
	}
	var done map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("ログが JSON でない: %q", line)
		}
		if m["msg"] == "expire done" {
			done = m
		}
	}
	if done == nil {
		t.Fatalf(`msg="expire done" のログが無い: %s`, buf.String())
	}
	for _, key := range []string{"teams", "team_members", "purge_journal", "devices", "orphans_marked", "orphans_cleared", "remaining", "duration"} {
		if _, ok := done[key]; !ok {
			t.Errorf("expire done のログに %q が無い: %v", key, done)
		}
	}
	if done["teams"] != float64(1) || done["team_members"] != float64(2) {
		t.Errorf("teams・team_members = %v・%v, want 1・2", done["teams"], done["team_members"])
	}
}

func TestPolicyValidate(t *testing.T) {
	if err := defaultPolicy.Validate(); err != nil {
		t.Fatalf("既定値の Policy が不正とされた: %v", err)
	}
	for name, mod := range map[string]func(*expire.Policy){
		"Retention 0":                 func(p *expire.Policy) { p.Retention = 0 },
		"PurgeJournalRetention 負":     func(p *expire.Policy) { p.PurgeJournalRetention = -day },
		"DeviceRowExpiry 7日(max_age)": func(p *expire.Policy) { p.DeviceRowExpiry = 7 * day },
		"BatchLimit 0":                func(p *expire.Policy) { p.BatchLimit = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := defaultPolicy
			mod(&p)
			if p.Validate() == nil {
				t.Error("Validate が nil")
			}
			f := newFake()
			if _, err := expire.Run(context.Background(), f, p, fixedNow, discardLogger()); err == nil || len(f.calls) != 0 {
				t.Errorf("不正な Policy で Run: err=%v calls=%v(DB に触れずに失敗すること)", err, f.callNames())
			}
		})
	}
}

// AC-E9
func TestExpirePackageImportsStayInsideTeamDB(t *testing.T) {
	forbidden := []string{
		"net/http", "github.com/nats-io/", "github.com/labstack/echo",
		"example.com/pokecalc/services/record/", "example.com/pokecalc/services/calc/",
		"example.com/pokecalc/services/gateway/", "example.com/pokecalc/services/pokedex/",
		"example.com/pokecalc/services/team/internal/httpapi", "example.com/pokecalc/services/team/internal/events",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		checked++
		for _, imp := range parsed.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if strings.HasPrefix(path, bad) {
					t.Errorf("%s が %s を import している(失効ジョブは team DB だけに触る)", file, path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("expire パッケージの本体ファイルが無い(検査が空振りしている)")
	}
}
