package httpapi_test

// DB を使う操作の締め切り(issue #323・#299 の pokedex 分・ADR-0129 §2)のテスト。
// DB が固まった(応答しない)ときも、ハンドラは締め切りの context を DB の呼び出しに渡し、
// 締め切りで 503 master_unavailable を返す(http.Server の WriteTimeout はハンドラを止めないため。Go の仕様)。
//
// 実装者向け: httpapi に次を用意する(ADR-0129 §2・§4)。
//
//	const DefaultRequestTimeout   time.Duration // DB を使う操作1回の締め切り(5秒)
//	const DefaultReadinessTimeout time.Duration // GET /readyz の DB 確認の締め切り(2秒)
//	type Option func(*...)
//	func WithRequestTimeout(d time.Duration) Option   // テストが短い締め切りを渡すため
//	func WithReadinessTimeout(d time.Duration) Option
//	func NewHandler(q readtx.DB, opts ...Option) http.Handler // 省略時は Default* を使う(本番は省略)

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readtx"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// testDeadline はテストで渡す短い締め切り。stuckCap は「締め切りが無いまま固まった」ときにテストを
// 無期限に止めないための上限(これに達したらその呼び出しは失敗し、経過時間の検査で落ちる)。
const (
	testDeadline = 100 * time.Millisecond
	stuckCap     = 3 * time.Second
	// deadlineSlack は締め切りから応答までの許容(goroutine の切り替え・JSON の書き出し)。
	deadlineSlack = time.Second
)

// errNoDeadline は締め切りの無い context で固まった呼び出しが stuckCap 後に返す値。
var errNoDeadline = errors.New("test: 締め切りの無い context で DB の呼び出しが固まった")

// stuckDB は DB が固まった状態を模す。DB を使う全ての入口(autocommit の読み出しと BeginTx)で、
// 実 MySQL ドライバと同じく ctx が終わるまで待ち、ctx.Err() を返す。受け取った ctx の締め切りを記録する。
// 上書きしていないメソッドは storetest.Querier に委ねる(想定外の書き込み等は panic で気づく)。
type stuckDB struct {
	*storetest.Querier

	mu        sync.Mutex
	deadlines []time.Time // 受け取った ctx の締め切り(締め切りが無ければ time.Time{})
}

func newStuckDB() *stuckDB { return &stuckDB{Querier: storetest.New()} }

func (s *stuckDB) block(ctx context.Context) error {
	d, _ := ctx.Deadline()
	s.mu.Lock()
	s.deadlines = append(s.deadlines, d)
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(stuckCap):
		return errNoDeadline
	}
}

func (s *stuckDB) calls() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.deadlines...)
}

func (s *stuckDB) BeginTx(ctx context.Context, _ *sql.TxOptions) (readtx.Tx, error) {
	return nil, s.block(ctx)
}
func (s *stuckDB) ListDataVersions(ctx context.Context) ([]store.DataVersion, error) {
	return nil, s.block(ctx)
}
func (s *stuckDB) ListTypes(ctx context.Context) ([]store.Type, error) { return nil, s.block(ctx) }
func (s *stuckDB) ListNatures(ctx context.Context) ([]store.Nature, error) {
	return nil, s.block(ctx)
}
func (s *stuckDB) GetDefaultRegulation(ctx context.Context) (store.GetDefaultRegulationRow, error) {
	return store.GetDefaultRegulationRow{}, s.block(ctx)
}
func (s *stuckDB) GetSpeciesByKey(ctx context.Context, _ string) (store.Species, error) {
	return store.Species{}, s.block(ctx)
}
func (s *stuckDB) GetMove(ctx context.Context, _ string) (store.GetMoveRow, error) {
	return store.GetMoveRow{}, s.block(ctx)
}
func (s *stuckDB) GetMovesByIDs(ctx context.Context, _ []string) ([]store.GetMovesByIDsRow, error) {
	return nil, s.block(ctx)
}
func (s *stuckDB) SearchSpecies(ctx context.Context, _ store.SearchSpeciesParams) ([]store.SearchSpeciesRow, error) {
	return nil, s.block(ctx)
}
func (s *stuckDB) SearchMoves(ctx context.Context, _ store.SearchMovesParams) ([]store.SearchMovesRow, error) {
	return nil, s.block(ctx)
}
func (s *stuckDB) SearchItems(ctx context.Context, _ store.SearchItemsParams) ([]store.SearchItemsRow, error) {
	return nil, s.block(ctx)
}

// deadlineCapturingDB は DB を固めずに、受け取った ctx の締め切りだけを記録する(既定の締め切りが
// 本番の NewHandler(q) で掛かっていることを、5秒待たずに確かめるため)。BeginTx の Tx の中の読み出しは
// storetest に委ねる(BeginTx の ctx を記録すれば足りる)。
type deadlineCapturingDB struct {
	*storetest.Querier

	mu        sync.Mutex
	deadlines []time.Time
}

func (d *deadlineCapturingDB) capture(ctx context.Context) {
	dl, _ := ctx.Deadline()
	d.mu.Lock()
	d.deadlines = append(d.deadlines, dl)
	d.mu.Unlock()
}

func (d *deadlineCapturingDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (readtx.Tx, error) {
	d.capture(ctx)
	return d.Querier.BeginTx(ctx, opts)
}
func (d *deadlineCapturingDB) ListDataVersions(ctx context.Context) ([]store.DataVersion, error) {
	d.capture(ctx)
	return d.Querier.ListDataVersions(ctx)
}
func (d *deadlineCapturingDB) ListTypes(ctx context.Context) ([]store.Type, error) {
	d.capture(ctx)
	return d.Querier.ListTypes(ctx)
}
func (d *deadlineCapturingDB) ListNatures(ctx context.Context) ([]store.Nature, error) {
	d.capture(ctx)
	return d.Querier.ListNatures(ctx)
}
func (d *deadlineCapturingDB) GetDefaultRegulation(ctx context.Context) (store.GetDefaultRegulationRow, error) {
	d.capture(ctx)
	return d.Querier.GetDefaultRegulation(ctx)
}

func (d *deadlineCapturingDB) captured() []time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]time.Time(nil), d.deadlines...)
}

// dbRoutes は DB を使う全ての操作(公開の検索7 + 内部 API 1)。withHeaders は公開操作の必須ヘッダ。
var dbRoutes = []struct {
	name        string
	target      string
	withHeaders bool
}{
	{"種族の検索", "/api/pokedex/species?q=a", true},
	{"種族の詳細", "/api/pokedex/species/9001-000", true},
	{"技の検索", "/api/pokedex/moves?q=a", true},
	{"技のまとめ取り", "/api/pokedex/moves/batch?ids=teststrike", true},
	{"技の詳細", "/api/pokedex/moves/teststrike", true},
	{"持ち物の検索", "/api/pokedex/items?q=a", true},
	{"性格の一覧", "/api/pokedex/natures", true},
	{"内部 API(マスタ全体)", masterPath, false},
}

// AC-D1(#323): DB が固まったとき、DB を使う全ての操作が締め切り(+ 許容)以内に 503 master_unavailable を返す。
// 応答は契約の Error に合い、DB の内部エラー(context deadline exceeded 等)の文言を出さない。
// DB の呼び出しには締め切り付きの context が渡る(締め切りの無い context で固まらない)。
func TestDBRoutesReturnUnavailableWithinRequestDeadline(t *testing.T) {
	for _, tt := range dbRoutes {
		t.Run(tt.name, func(t *testing.T) {
			db := newStuckDB()
			h := httpapi.NewHandler(db, httpapi.WithRequestTimeout(testDeadline))

			start := time.Now()
			rec := do(t, h, http.MethodGet, tt.target, tt.withHeaders)
			elapsed := time.Since(start)

			if elapsed > testDeadline+deadlineSlack {
				t.Fatalf("応答まで %v かかった(締め切り %v + 許容 %v を超えた。DB の呼び出しに締め切りが渡っていない)",
					elapsed, testDeadline, deadlineSlack)
			}
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			validateAgainstContract(t, http.MethodGet, tt.target, tt.withHeaders, rec)
			for _, leak := range []string{context.DeadlineExceeded.Error(), "context", errNoDeadline.Error()} {
				if containsFold(rec.Body.String(), leak) {
					t.Errorf("内部エラーの文言 %q を応答に出している: %s", leak, rec.Body.String())
				}
			}
			calls := db.calls()
			if len(calls) == 0 {
				t.Fatal("DB の呼び出しが1回も無い(検査が空振りしている)")
			}
			for i, d := range calls {
				if d.IsZero() {
					t.Errorf("DB の呼び出し %d に締め切りの無い context が渡った", i)
				}
			}
		})
	}
}

// AC-D2(#323): 本番の組み立て(NewHandler(q)。オプション無し)でも、DB を使う操作は
// DefaultRequestTimeout の締め切り付きの context で DB を呼ぶ。締め切りは DefaultRequestTimeout より先に来ない
// ほど長すぎず(受け取った時点から DefaultRequestTimeout 以内)、無期限でもない。
func TestDefaultRequestDeadlineIsApplied(t *testing.T) {
	if httpapi.DefaultRequestTimeout <= 0 {
		t.Fatalf("DefaultRequestTimeout = %v, want 正の値", httpapi.DefaultRequestTimeout)
	}
	for _, tt := range []struct {
		name   string
		target string
		hdr    bool
	}{
		{"種族の検索", "/api/pokedex/species?q=a", true},
		{"性格の一覧", "/api/pokedex/natures", true},
		{"内部 API(マスタ全体)", masterPath, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := &deadlineCapturingDB{Querier: storetest.New()}
			h := httpapi.NewHandler(db)
			start := time.Now()
			rec := do(t, h, http.MethodGet, tt.target, tt.hdr)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
			}
			calls := db.captured()
			if len(calls) == 0 {
				t.Fatal("DB の呼び出しが記録されていない(検査が空振りしている)")
			}
			for i, d := range calls {
				if d.IsZero() {
					t.Fatalf("DB の呼び出し %d に締め切りの無い context が渡った", i)
				}
				if limit := start.Add(httpapi.DefaultRequestTimeout + deadlineSlack); d.After(limit) {
					t.Errorf("DB の呼び出し %d の締め切り %v が DefaultRequestTimeout(%v)より遠い", i, d.Sub(start), httpapi.DefaultRequestTimeout)
				}
			}
		})
	}
}

// AC-D3(#323): 締め切りは DB が固まったときだけ効き、正常系は変わらない(短い締め切りでも、速い DB なら 200)。
// GET /healthz は DB にも締め切りにも関係なく 200(liveness を DB に連動させない。#107)。
func TestRequestDeadlineDoesNotAffectHealthyRequestsOrHealthz(t *testing.T) {
	h := httpapi.NewHandler(storetest.New(), httpapi.WithRequestTimeout(testDeadline))
	for _, tt := range dbRoutes {
		if rec := do(t, h, http.MethodGet, tt.target, tt.withHeaders); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200\nbody=%s", tt.name, rec.Code, rec.Body.String())
		}
	}

	stuck := newStuckDB()
	hs := httpapi.NewHandler(stuck, httpapi.WithRequestTimeout(testDeadline))
	start := time.Now()
	rec := do(t, hs, http.MethodGet, "/healthz", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("DB が固まっているとき /healthz = %d, want 200", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > deadlineSlack {
		t.Errorf("/healthz が %v かかった(DB を待っている)", elapsed)
	}
	if n := len(stuck.calls()); n != 0 {
		t.Errorf("/healthz が DB を %d 回呼んだ", n)
	}
}

// containsFold は大文字小文字を区別せずに部分一致を見る。
func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
