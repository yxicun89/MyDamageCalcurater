package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

// fakeClock は実際には待たない時計。Sleep は待った時間を記録して現在時刻を進める。
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sleeps = append(c.sleeps, d)
	if d > 0 {
		c.now = c.now.Add(d)
	}
	return nil
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// takeSleeps は記録した待ち(0 以下は「待っていない」として除く)を返して消す。
func (c *fakeClock) takeSleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []time.Duration
	for _, d := range c.sleeps {
		if d > 0 {
			out = append(out, d)
		}
	}
	c.sleeps = nil
	return out
}

// countingFetcher は呼ばれた回数を数え、同時に 2 つ以上走ったら overlap を立てる。
type countingFetcher struct {
	calls    atomic.Int64
	inFlight atomic.Int64
	overlap  atomic.Bool
	err      error
	during   func() // 取得中に呼ぶ(時計を進める・少し待つ)
}

func (f *countingFetcher) Fetch(ctx context.Context, site fetcher.Site, q string) ([]fetcher.Listing, error) {
	if f.inFlight.Add(1) > 1 {
		f.overlap.Store(true)
	}
	defer f.inFlight.Add(-1)
	f.calls.Add(1)
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return nil, f.err
	}
	return []fetcher.Listing{{Title: "x", Price: 100, URL: "https://example.com/x", InStock: true}}, nil
}

var (
	siteA = fetcher.Site{ID: 1, Name: "A", FetchType: item.FetchAPI}
	siteB = fetcher.Site{ID: 2, Name: "B", FetchType: item.FetchAPI}
)

func mustFetch(t *testing.T, f fetcher.Fetcher, s fetcher.Site) {
	t.Helper()
	if _, err := f.Fetch(context.Background(), s, "q"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
}

func eqDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AC-F1: 同じサイトへのリクエストは前回の取得が終わってから最低 5 秒あける(初回は待たない)。違うサイトは待たない。
func TestThrottle_Interval(t *testing.T) {
	clock := newFakeClock()
	inner := &countingFetcher{}
	f := fetcher.Throttle(inner, fetcher.MinInterval, clock)

	mustFetch(t, f, siteA)
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("初回で待った: %v", s)
	}
	mustFetch(t, f, siteA)
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{5 * time.Second}) {
		t.Errorf("直後の 2 回目の待ち = %v, want [5s]", s)
	}
	mustFetch(t, f, siteB)
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("違うサイトで待った: %v", s)
	}
	clock.Advance(7 * time.Second)
	mustFetch(t, f, siteA)
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("7 秒後に待った: %v", s)
	}
	clock.Advance(3 * time.Second)
	mustFetch(t, f, siteA)
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{2 * time.Second}) {
		t.Errorf("3 秒後の待ち = %v, want [2s]", s)
	}
	if n := inner.calls.Load(); n != 5 {
		t.Errorf("inner の呼び出し = %d, want 5", n)
	}
}

// AC-F1: 間隔は前回の取得が「終わってから」数える。失敗した取得も数える。
func TestThrottle_FromEndAndFailures(t *testing.T) {
	clock := newFakeClock()
	inner := &countingFetcher{during: func() { clock.Advance(2 * time.Second) }}
	f := fetcher.Throttle(inner, fetcher.MinInterval, clock)
	mustFetch(t, f, siteA)
	mustFetch(t, f, siteA)
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{5 * time.Second}) {
		t.Errorf("取得に 2 秒かかったときの待ち = %v, want [5s](終わってから数える)", s)
	}

	clock2 := newFakeClock()
	failing := &countingFetcher{err: errors.New("boom")}
	g := fetcher.Throttle(failing, fetcher.MinInterval, clock2)
	if _, err := g.Fetch(context.Background(), siteA, "q"); err == nil {
		t.Fatal("inner のエラーが返らない")
	}
	g.Fetch(context.Background(), siteA, "q")
	if s := clock2.takeSleeps(); !eqDurations(s, []time.Duration{5 * time.Second}) {
		t.Errorf("失敗の後の待ち = %v, want [5s]", s)
	}
}

// AC-F1: 待っている間に ctx が終わったら inner を呼ばずに ctx の err を返す。
func TestThrottle_CanceledWhileWaiting(t *testing.T) {
	clock := newFakeClock()
	inner := &countingFetcher{}
	f := fetcher.Throttle(inner, fetcher.MinInterval, clock)
	mustFetch(t, f, siteA)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Fetch(ctx, siteA, "q"); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if n := inner.calls.Load(); n != 1 {
		t.Errorf("inner の呼び出し = %d, want 1(取り消し後は呼ばない)", n)
	}
}

// AC-F1: 同じサイトへは並列で叩かない(同時に呼ばれても直列にし、間隔も守る)。
func TestThrottle_NoParallelSameSite(t *testing.T) {
	clock := newFakeClock()
	inner := &countingFetcher{during: func() { time.Sleep(5 * time.Millisecond) }}
	f := fetcher.Throttle(inner, fetcher.MinInterval, clock)
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() { f.Fetch(context.Background(), siteA, "q") })
	}
	wg.Wait()
	if inner.overlap.Load() {
		t.Error("同じサイトへの取得が並列に走った")
	}
	if n := inner.calls.Load(); n != 5 {
		t.Errorf("inner の呼び出し = %d, want 5", n)
	}
	var total time.Duration
	for _, d := range clock.takeSleeps() {
		total += d
	}
	if total < 20*time.Second {
		t.Errorf("待ちの合計 = %v, want 20s 以上(5 回で間隔 4 つ)", total)
	}
}

// AC-F2: fetch_type から Fetcher を選ぶ。link_only は取得しない。scrape・headless は未実装なので使えない。
// api は appid があるときだけ使える。
func TestNewRegistry(t *testing.T) {
	all := []item.FetchType{item.FetchAPI, item.FetchScrape, item.FetchHeadless, item.FetchLinkOnly}
	cases := []struct {
		name  string
		appid string
		want  map[item.FetchType]bool
	}{
		{"appid なし", "", map[item.FetchType]bool{}},
		{"appid 空白だけ", "  ", map[item.FetchType]bool{}},
		{"appid あり", "unit-test-appid", map[item.FetchType]bool{item.FetchAPI: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := fetcher.NewRegistry(fetcher.Config{YahooAppID: c.appid})
			for _, ft := range all {
				f, ok := r.For(ft)
				if ok != c.want[ft] || (ok && f == nil) {
					t.Errorf("For(%s) = %v, %v; want ok=%v", ft, f, ok, c.want[ft])
				}
			}
		})
	}
}

// AC-F2: テスト用の表は渡したものをそのまま使う(link_only は渡しても使わない)。
func TestNewRegistryWith(t *testing.T) {
	a, s, l := &countingFetcher{}, &countingFetcher{}, &countingFetcher{}
	r := fetcher.NewRegistryWith(map[item.FetchType]fetcher.Fetcher{item.FetchAPI: a, item.FetchScrape: s, item.FetchLinkOnly: l})
	if f, ok := r.For(item.FetchAPI); !ok || f != fetcher.Fetcher(a) {
		t.Errorf("For(api) = %v, %v", f, ok)
	}
	if f, ok := r.For(item.FetchScrape); !ok || f != fetcher.Fetcher(s) {
		t.Errorf("For(scrape) = %v, %v", f, ok)
	}
	if _, ok := r.For(item.FetchHeadless); ok {
		t.Error("For(headless) が使える")
	}
	if _, ok := r.For(item.FetchLinkOnly); ok {
		t.Error("For(link_only) が使える")
	}
}

// AC-F2・AC-F1: 本番の表の Yahoo は Endpoint・Client の設定を使い、Throttle(Clock・Interval)で包まれている。
func TestNewRegistry_YahooIsThrottled(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hits":[]}`))
	}))
	defer srv.Close()
	clock := newFakeClock()
	r := fetcher.NewRegistry(fetcher.Config{
		YahooAppID:    "unit-test-appid",
		YahooEndpoint: srv.URL + "/ShoppingWebService/V3/itemSearch",
		Client:        netguard.NewClient(netguard.Options{AllowAddr: func(netip.Addr) bool { return true }}),
		Clock:         clock,
	})
	f, ok := r.For(item.FetchAPI)
	if !ok {
		t.Fatal("For(api) が使えない")
	}
	mustFetch(t, f, siteA)
	mustFetch(t, f, siteA)
	if n := hits.Load(); n != 2 {
		t.Errorf("サーバーへのリクエスト = %d, want 2", n)
	}
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{fetcher.MinInterval}) {
		t.Errorf("待ち = %v, want [5s]", s)
	}
}
