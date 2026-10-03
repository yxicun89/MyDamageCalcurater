package expire_test

// record-svc の失効ジョブ(ADR-0209 §4・ADR-0220 §3・§4)の手順を、偽ストアで検査する(make test。DB 不要)。
//
// **test-first(ADR-0003)**: このパッケージ(services/record/internal/expire)はまだ無い。実装者は次の形で作る:
//
//	type Policy struct {
//		CalcEventsRetention   time.Duration // RECORD_CALC_EVENTS_RETENTION_DAYS
//		FavoritesRetention    time.Duration // RECORD_FAVORITES_RETENTION_DAYS
//		DeviceRowExpiry       time.Duration // RECORD_DEVICE_ROW_EXPIRY_DAYS(JetStream max_age 7日より大きい)
//		PurgeJournalRetention time.Duration // RECORD_PURGE_JOURNAL_RETENTION_DAYS
//		BatchLimit            int           // RECORD_EXPIRE_BATCH_LIMIT(1回の実行で消す行数の上限)
//	}
//	func (p Policy) Validate() error
//
//	type Result struct {
//		CalcEvents, Aggregates, Favorites, PurgeJournal, Devices int
//		OrphansMarked, OrphansCleared                            int
//		Remaining                                                bool
//	}
//
//	// Store は失効専用の操作(全端末を横断する。internal/store の Store とは別。ADR-0220 §3)。
//	// Delete* は「判定時刻 < cutoff」の行を最大 limit 件消し、消した件数を返す(ちょうど cutoff の行は消さない)。
//	type Store interface {
//		DeleteCalcEvents(ctx context.Context, cutoff time.Time, limit int) (int, error)        // occurred_at
//		DeleteStaleAggregates(ctx context.Context, cutoff time.Time, limit int) (int, error)   // last_calculated_at
//		DeleteInactiveFavorites(ctx context.Context, cutoff time.Time, limit int) (int, error) // max(devices.last_seen_at, updated_at)
//		DeletePurgeJournal(ctx context.Context, cutoff time.Time, limit int) (int, error)      // requested_at
//		MarkOrphans(ctx context.Context, now time.Time, limit int) (marked, cleared int, err error)
//		DeleteOrphanDevices(ctx context.Context, cutoff time.Time, limit int) (int, error)
//	}
//
//	// Run は ADR-0220 §4 の順に1巡だけ実行し、終わりに件数をログに1行出す。
//	func Run(ctx context.Context, st Store, p Policy, now time.Time, log *slog.Logger) (Result, error)
//
//	// NewTiDB は Store の TiDB 実装(tidb_test.go が検査する)。
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

	"example.com/pokecalc/services/record/internal/expire"
)

const day = 24 * time.Hour

// ADR-0211 §7 の既定値(テストの独立した期待値)。
var defaultPolicy = expire.Policy{
	CalcEventsRetention:   90 * day,
	FavoritesRetention:    540 * day,
	DeviceRowExpiry:       30 * day,
	PurgeJournalRetention: 90 * day,
	BatchLimit:            1000,
}

// 架空の端末 ID。
const (
	deviceA = "00000000-0000-4000-8000-00000000000a"
	deviceB = "00000000-0000-4000-8000-00000000000b"
)

var fixedNow = time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)

// ---- 偽ストア ----
//
// Store の契約(「判定時刻 < cutoff の行を最大 limit 件消す」)どおりに振る舞う。Run が正しい cutoff を渡せば境界の行は残り、
// 1日でも・1秒でもずれた cutoff を渡せば境界の行の扱いが変わる、という形で Run の計算を検査する。
// devices の orphan の意味(SQL の条件)は tidb_test.go が検査する。ここでは呼ばれ方だけを見る。

type row struct {
	id     string
	device string
	ts     time.Time // occurred_at / last_calculated_at / requested_at
}

type favorite struct {
	id        string
	device    string
	updatedAt time.Time
}

type call struct {
	name   string
	cutoff time.Time
	limit  int
}

type fakeStore struct {
	events     []row
	aggregates []row
	favorites  []favorite
	journal    []row
	lastSeen   map[string]time.Time

	calls  []call
	failOn string // この名前のメソッドで ErrBoom を返す
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

func deleteRows(rows []row, cutoff time.Time, limit int) ([]row, int) {
	var kept []row
	n := 0
	for _, r := range rows {
		if n < limit && r.ts.Before(cutoff) {
			n++
			continue
		}
		kept = append(kept, r)
	}
	return kept, n
}

func (f *fakeStore) DeleteCalcEvents(_ context.Context, cutoff time.Time, limit int) (int, error) {
	if err := f.note("DeleteCalcEvents", cutoff, limit); err != nil {
		return 0, err
	}
	var n int
	f.events, n = deleteRows(f.events, cutoff, limit)
	return n, nil
}

func (f *fakeStore) DeleteStaleAggregates(_ context.Context, cutoff time.Time, limit int) (int, error) {
	if err := f.note("DeleteStaleAggregates", cutoff, limit); err != nil {
		return 0, err
	}
	var n int
	f.aggregates, n = deleteRows(f.aggregates, cutoff, limit)
	return n, nil
}

func (f *fakeStore) DeleteInactiveFavorites(_ context.Context, cutoff time.Time, limit int) (int, error) {
	if err := f.note("DeleteInactiveFavorites", cutoff, limit); err != nil {
		return 0, err
	}
	var kept []favorite
	n := 0
	for _, fav := range f.favorites {
		basis := fav.updatedAt
		if seen, ok := f.lastSeen[fav.device]; ok && seen.After(basis) {
			basis = seen
		}
		if n < limit && basis.Before(cutoff) {
			n++
			continue
		}
		kept = append(kept, fav)
	}
	f.favorites = kept
	return n, nil
}

func (f *fakeStore) DeletePurgeJournal(_ context.Context, cutoff time.Time, limit int) (int, error) {
	if err := f.note("DeletePurgeJournal", cutoff, limit); err != nil {
		return 0, err
	}
	var n int
	f.journal, n = deleteRows(f.journal, cutoff, limit)
	return n, nil
}

func (f *fakeStore) MarkOrphans(_ context.Context, now time.Time, limit int) (int, int, error) {
	if err := f.note("MarkOrphans", now, limit); err != nil {
		return 0, 0, err
	}
	return 0, 0, nil
}

func (f *fakeStore) DeleteOrphanDevices(_ context.Context, cutoff time.Time, limit int) (int, error) {
	if err := f.note("DeleteOrphanDevices", cutoff, limit); err != nil {
		return 0, err
	}
	return 0, nil
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

func ids[T any](items []T, id func(T) string) []string {
	var out []string
	for _, it := range items {
		out = append(out, id(it))
	}
	slices.Sort(out)
	return out
}

func rowIDs(rs []row) []string { return ids(rs, func(r row) string { return r.id }) }

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

func run(t *testing.T, f *fakeStore, p expire.Policy, now time.Time) expire.Result {
	t.Helper()
	res, err := expire.Run(context.Background(), f, p, now, discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

// ---- AC-E1: cutoff はそのまま now − 保持期間 ----

func TestRunPassesExactCutoffs(t *testing.T) {
	f := newFake()
	run(t, f, defaultPolicy, fixedNow)

	want := []string{
		"DeleteCalcEvents", "DeleteStaleAggregates", "DeleteInactiveFavorites", "DeletePurgeJournal",
		"MarkOrphans", "DeleteOrphanDevices",
	}
	if got := f.callNames(); !slices.Equal(got, want) {
		t.Fatalf("呼ばれた順 = %v, want %v(ADR-0220 §4 の順。orphan の印付けは devices 行の削除より先)", got, want)
	}
	cases := []struct {
		name string
		want time.Time
	}{
		{"DeleteCalcEvents", fixedNow.Add(-defaultPolicy.CalcEventsRetention)},
		// 集計は生イベントと同時に失効する(ADR-0209 §3 #2。独自の保持期間を持たない)。
		{"DeleteStaleAggregates", fixedNow.Add(-defaultPolicy.CalcEventsRetention)},
		{"DeleteInactiveFavorites", fixedNow.Add(-defaultPolicy.FavoritesRetention)},
		{"DeletePurgeJournal", fixedNow.Add(-defaultPolicy.PurgeJournalRetention)},
		{"MarkOrphans", fixedNow},
		{"DeleteOrphanDevices", fixedNow.Add(-defaultPolicy.DeviceRowExpiry)},
	}
	for _, tc := range cases {
		if got := f.callOf(t, tc.name).cutoff; !got.Equal(tc.want) {
			t.Errorf("%s の cutoff = %s, want %s", tc.name, got, tc.want)
		}
	}
	if got := f.callOf(t, "MarkOrphans").limit; got != defaultPolicy.BatchLimit {
		t.Errorf("MarkOrphans の limit = %d, want %d(削除の上限とは別に、同じ値を上限にする)", got, defaultPolicy.BatchLimit)
	}
}

// ---- AC-E2: 境界(ちょうど保持期間は残る。超えたら消える) ----

func TestRunCalcEventsBoundary(t *testing.T) {
	f := newFake()
	ago := func(d time.Duration) time.Time { return fixedNow.Add(-d) }
	f.events = []row{
		{"89d", deviceA, ago(89 * day)},
		{"90d", deviceA, ago(90 * day)}, // ちょうど 2160 時間は残る(AC-R1)
		{"90d+1s", deviceA, ago(90*day + time.Second)},
		{"91d", deviceA, ago(91 * day)},
		{"B-1d", deviceB, ago(day)},
	}
	res := run(t, f, defaultPolicy, fixedNow)

	if got, want := rowIDs(f.events), []string{"89d", "90d", "B-1d"}; !slices.Equal(got, want) {
		t.Errorf("残ったイベント = %v, want %v", got, want)
	}
	if res.CalcEvents != 2 {
		t.Errorf("Result.CalcEvents = %d, want 2", res.CalcEvents)
	}
	if res.Remaining {
		t.Error("上限に届いていないのに Remaining = true")
	}
}

func TestRunAggregatesFollowCalcEventsRetention(t *testing.T) {
	f := newFake()
	f.aggregates = []row{
		{"A-x-90d", deviceA, fixedNow.Add(-90 * day)},
		{"A-y-91d", deviceA, fixedNow.Add(-91 * day)},
		{"B-x-1d", deviceB, fixedNow.Add(-day)},
	}
	res := run(t, f, defaultPolicy, fixedNow)
	if got, want := rowIDs(f.aggregates), []string{"A-x-90d", "B-x-1d"}; !slices.Equal(got, want) {
		t.Errorf("残った集計 = %v, want %v", got, want)
	}
	if res.Aggregates != 1 {
		t.Errorf("Result.Aggregates = %d, want 1", res.Aggregates)
	}
}

// AC-R2・AC-R2b: max(devices.last_seen_at, 行.updated_at) から540日。
func TestRunFavoritesBoundaryUsesMaxOfLastSeenAndUpdatedAt(t *testing.T) {
	const (
		devOld      = "00000000-0000-4000-8000-0000000000c1" // last_seen 541日前
		devRecent   = "00000000-0000-4000-8000-0000000000c2" // last_seen 1日前
		devNoDevice = "00000000-0000-4000-8000-0000000000c3" // devices 行が無い
	)
	f := newFake()
	ago := func(d time.Duration) time.Time { return fixedNow.Add(-d) }
	f.lastSeen[devOld] = ago(541 * day)
	f.lastSeen[devRecent] = ago(day)
	f.favorites = []favorite{
		{"old-seen/updated-541d", devOld, ago(541 * day)},       // 消える
		{"old-seen/updated-539d", devOld, ago(539 * day)},       // 行が最近更新されたので残る(AC-R2b)
		{"old-seen/updated-540d", devOld, ago(540 * day)},       // ちょうど540日は残る
		{"recent-seen/updated-900d", devRecent, ago(900 * day)}, // 端末が使われているので残る
		{"no-device/updated-541d", devNoDevice, ago(541 * day)}, // devices 行が無ければ行の更新時刻だけで判定
		{"no-device/updated-540d", devNoDevice, ago(540 * day)},
	}
	res := run(t, f, defaultPolicy, fixedNow)

	got := ids(f.favorites, func(x favorite) string { return x.id })
	want := []string{"no-device/updated-540d", "old-seen/updated-539d", "old-seen/updated-540d", "recent-seen/updated-900d"}
	if !slices.Equal(got, want) {
		t.Errorf("残ったお気に入り = %v, want %v", got, want)
	}
	if res.Favorites != 2 {
		t.Errorf("Result.Favorites = %d, want 2", res.Favorites)
	}
}

func TestRunPurgeJournalBoundary(t *testing.T) {
	f := newFake()
	f.journal = []row{
		{"90d", deviceA, fixedNow.Add(-90 * day)},
		{"90d+1s", deviceA, fixedNow.Add(-90*day - time.Second)},
	}
	res := run(t, f, defaultPolicy, fixedNow)
	if got, want := rowIDs(f.journal), []string{"90d"}; !slices.Equal(got, want) {
		t.Errorf("残った purge journal = %v, want %v", got, want)
	}
	if res.PurgeJournal != 1 {
		t.Errorf("Result.PurgeJournal = %d, want 1", res.PurgeJournal)
	}
}

// ---- AC-E4: 上限・残りあり・冪等 ----

func TestRunBatchLimitThenIdempotent(t *testing.T) {
	f := newFake()
	for i := range 5 {
		f.events = append(f.events, row{"old-" + strconv.Itoa(i), deviceA, fixedNow.Add(-100 * day)})
	}
	p := defaultPolicy
	p.BatchLimit = 3

	first := run(t, f, p, fixedNow)
	if first.CalcEvents != 3 || !first.Remaining {
		t.Fatalf("1回目 = %+v, want CalcEvents 3・Remaining true", first)
	}
	second := run(t, f, p, fixedNow)
	if second.CalcEvents != 2 || second.Remaining {
		t.Fatalf("2回目 = %+v, want CalcEvents 2・Remaining false", second)
	}
	third := run(t, f, p, fixedNow)
	if third != (expire.Result{}) {
		t.Fatalf("3回目 = %+v, want 全て 0・Remaining false(冪等。AC-R3)", third)
	}
	if len(f.events) != 0 {
		t.Errorf("期限切れのイベントが残っている: %v", rowIDs(f.events))
	}
}

// 上限は段をまたいで共有する。尽きたら後の削除の段は呼ばない(limit 0 で呼ばない)。
func TestRunBatchLimitIsSharedAcrossSteps(t *testing.T) {
	f := newFake()
	old := fixedNow.Add(-1000 * day)
	f.events = []row{{"e1", deviceA, old}, {"e2", deviceA, old}}
	f.favorites = []favorite{{"f1", deviceA, old}, {"f2", deviceA, old}}
	f.journal = []row{{"j1", deviceA, old}}
	p := defaultPolicy
	p.BatchLimit = 3

	res := run(t, f, p, fixedNow)
	if res.CalcEvents != 2 || res.Favorites != 1 || res.PurgeJournal != 0 || !res.Remaining {
		t.Fatalf("Result = %+v, want CalcEvents 2・Favorites 1・PurgeJournal 0・Remaining true", res)
	}
	if got := f.callOf(t, "DeleteInactiveFavorites").limit; got != 1 {
		t.Errorf("DeleteInactiveFavorites の limit = %d, want 1(残りの上限)", got)
	}
	for _, c := range f.calls {
		if c.name == "DeletePurgeJournal" || c.name == "DeleteOrphanDevices" {
			t.Errorf("上限が尽きた後に %s が呼ばれた(limit %d)", c.name, c.limit)
		}
	}
	// 次回が続きを消す。
	res2 := run(t, f, p, fixedNow)
	if res2.Favorites != 1 || res2.PurgeJournal != 1 || res2.Remaining {
		t.Fatalf("2回目 = %+v, want Favorites 1・PurgeJournal 1・Remaining false", res2)
	}
}

// ---- AC-E7: 途中の失敗 ----

func TestRunStopsAtFirstStoreError(t *testing.T) {
	f := newFake()
	f.events = []row{{"e1", deviceA, fixedNow.Add(-100 * day)}}
	f.failOn = "DeleteInactiveFavorites"

	res, err := expire.Run(context.Background(), f, defaultPolicy, fixedNow, discardLogger())
	if !errors.Is(err, errBoom) {
		t.Fatalf("Run の err = %v, want errBoom を包んだエラー", err)
	}
	if res.CalcEvents != 1 {
		t.Errorf("失敗前に消した件数 = %d, want 1(結果はログに出す)", res.CalcEvents)
	}
	for _, name := range []string{"DeletePurgeJournal", "MarkOrphans", "DeleteOrphanDevices"} {
		for _, c := range f.calls {
			if c.name == name {
				t.Errorf("失敗した段より後の %s が呼ばれた", name)
			}
		}
	}
}

// ---- AC-E8: ログ ----

func TestRunLogsCountsWithoutDeviceIDs(t *testing.T) {
	f := newFake()
	f.events = []row{{"e1", deviceA, fixedNow.Add(-100 * day)}}
	var buf bytes.Buffer
	if _, err := expire.Run(context.Background(), f, defaultPolicy, fixedNow, slog.New(slog.NewJSONHandler(&buf, nil))); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(buf.String(), deviceA) {
		t.Errorf("ログに端末 ID が出ている(件数だけを出す): %s", buf.String())
	}
	var done map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("ログが JSON でない: %q", line)
		}
		if m["msg"] == "expire done" {
			if done != nil {
				t.Fatal(`msg="expire done" が2行ある(1回の実行で1行)`)
			}
			done = m
		}
	}
	if done == nil {
		t.Fatalf(`msg="expire done" のログが無い: %s`, buf.String())
	}
	for _, key := range []string{
		"calc_events", "aggregates", "favorites", "purge_journal", "devices",
		"orphans_marked", "orphans_cleared", "remaining", "duration",
	} {
		if _, ok := done[key]; !ok {
			t.Errorf("expire done のログに %q が無い: %v", key, done)
		}
	}
	if done["calc_events"] != float64(1) {
		t.Errorf("calc_events = %v, want 1", done["calc_events"])
	}
}

// ---- Policy の検証 ----

func TestPolicyValidate(t *testing.T) {
	if err := defaultPolicy.Validate(); err != nil {
		t.Fatalf("既定値の Policy が不正とされた: %v", err)
	}
	cases := []struct {
		name string
		mod  func(*expire.Policy)
	}{
		{"CalcEventsRetention 0", func(p *expire.Policy) { p.CalcEventsRetention = 0 }},
		{"FavoritesRetention 負", func(p *expire.Policy) { p.FavoritesRetention = -day }},
		{"PurgeJournalRetention 0", func(p *expire.Policy) { p.PurgeJournalRetention = 0 }},
		{"DeviceRowExpiry が JetStream max_age 7日ちょうど", func(p *expire.Policy) { p.DeviceRowExpiry = 7 * day }},
		{"BatchLimit 0", func(p *expire.Policy) { p.BatchLimit = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := defaultPolicy
			tc.mod(&p)
			if err := p.Validate(); err == nil {
				t.Error("Validate が nil(不正な Policy を受け付けた)")
			}
			// 不正な Policy では DB に触れない。
			f := newFake()
			if _, err := expire.Run(context.Background(), f, p, fixedNow, discardLogger()); err == nil {
				t.Error("不正な Policy で Run が成功した")
			}
			if len(f.calls) != 0 {
				t.Errorf("不正な Policy で store が呼ばれた: %v", f.callNames())
			}
		})
	}
}

// ---- AC-E9: 境界(HTTP・NATS・他サービスに依存しない) ----

func TestExpirePackageImportsStayInsideRecordDB(t *testing.T) {
	forbidden := []string{
		"net/http", "github.com/nats-io/", "github.com/labstack/echo",
		"example.com/pokecalc/services/team/", "example.com/pokecalc/services/calc/",
		"example.com/pokecalc/services/gateway/", "example.com/pokecalc/services/pokedex/",
		"example.com/pokecalc/services/record/internal/httpapi", "example.com/pokecalc/services/record/internal/events",
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
					t.Errorf("%s が %s を import している(失効ジョブは record DB だけに触る。絶対ルール4・5)", file, path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("expire パッケージの本体ファイルが無い(検査が空振りしている)")
	}
}
