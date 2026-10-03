// Package fetcher はサイト別の価格取得(apps/wishlist/CLAUDE.md §6)。受け入れ条件は docs/phase3-api-spec.md の AC-F*。
//
// 取得方式(sites.fetch_type)ごとの実装:
//   - api      : Yahoo!ショッピング(Yahoo。appid が無ければ使わない)
//   - scrape   : 未実装(カードラッシュ・ドラゴンスター・あみあみ・駿河屋)。TODO: 実サイトの HTML を確認して fixture を保存してから作る
//   - headless : 未実装(メルカリ・Yahoo!フリマ。chromedp)。TODO: 同上
//   - link_only: 取得しない
//
// 実サイトへのアクセスはテストからは行わない(fixture・httptest を使う。仕様 §13)。
package fetcher

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

const (
	// MaxListings は 1 サイトあたりに取得する上位件数(仕様 §6)。
	MaxListings = 20
	// MinInterval は同じサイトへのリクエストの最低間隔(仕様 §6)。
	MinInterval = 5 * time.Second
	// MaxResponseBytes は取得する応答本文の上限。超えたら netguard.ErrTooLarge。
	MaxResponseBytes = 2 << 20
)

var (
	// ErrUpstreamStatus は取得先が 2xx 以外を返したこと。
	ErrUpstreamStatus = errors.New("fetcher: upstream returned non-2xx status")
	// ErrUnavailable はこの Fetcher が使えないこと(appid が無い等)。
	ErrUnavailable = errors.New("fetcher: unavailable")
)

// Site は取得対象のサイト(item.Site と同じ)。
type Site = item.Site

// Listing は取得した 1 件の出品(仕様 §6)。
type Listing struct {
	Title    string
	Price    int // 円。送料は含めない
	URL      string
	ImageURL string // 無ければ空
	InStock  bool
}

// Fetcher はサイトを query で検索し、出品を返す。
type Fetcher interface {
	Fetch(ctx context.Context, site Site, query string) ([]Listing, error)
}

// Clock は時刻と待ち。テストでは実際には待たない偽物に差し替える。
type Clock interface {
	Now() time.Time
	// Sleep は d だけ待つ。ctx が終わればその err を返す。
	Sleep(ctx context.Context, d time.Duration) error
}

// SystemClock は実際の時計。
var SystemClock Clock = systemClock{}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Throttle は inner を包み、同じサイト(Site.ID)へのリクエストを直列にし、前回の取得が終わってから
// interval 以上あけてから次を始める(待ちは clock.Sleep)。初回は待たない。違うサイトは互いに待たない。
// 待っている間に ctx が終わったら inner を呼ばずに ctx の err を返す。inner の失敗も「取得した」として間隔を数える。
func Throttle(inner Fetcher, interval time.Duration, clock Clock) Fetcher {
	return &throttled{inner: inner, interval: interval, clock: clock, sites: map[int64]*siteGate{}}
}

type throttled struct {
	inner    Fetcher
	interval time.Duration
	clock    Clock

	mu    sync.Mutex
	sites map[int64]*siteGate
}

// siteGate はサイトごとの直列化(容量 1 のチャネル。待ちを ctx で取り消せる)と、前回の取得が終わった時刻。
type siteGate struct {
	sem  chan struct{}
	last time.Time // 前回の取得が終わった時刻(初回はゼロ値)
}

func (t *throttled) gate(id int64) *siteGate {
	t.mu.Lock()
	defer t.mu.Unlock()
	g, ok := t.sites[id]
	if !ok {
		g = &siteGate{sem: make(chan struct{}, 1)}
		t.sites[id] = g
	}
	return g
}

func (t *throttled) Fetch(ctx context.Context, site Site, query string) ([]Listing, error) {
	g := t.gate(site.ID)
	select {
	case g.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-g.sem }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !g.last.IsZero() {
		if wait := t.interval - t.clock.Now().Sub(g.last); wait > 0 {
			if err := t.clock.Sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
	}
	defer func() { g.last = t.clock.Now() }()
	return t.inner.Fetch(ctx, site, query)
}

// Config は NewRegistry の設定。
type Config struct {
	YahooAppID    string        // 空(空白だけを含む)なら api 型は「使えない」
	YahooEndpoint string        // 空なら YahooEndpoint
	Client        *http.Client  // nil なら netguard.NewClient(netguard.Options{})
	Clock         Clock         // nil なら SystemClock
	Interval      time.Duration // 0 なら MinInterval
}

// Registry は fetch_type から Fetcher を選ぶ表。
type Registry struct {
	m map[item.FetchType]Fetcher
}

// NewRegistry は本番の表を作る。api は Yahoo(YahooAppID があるときだけ)。scrape・headless は未実装、link_only は取得しない。
// 登録する Fetcher はすべて Throttle(Interval・Clock)で包む(同じサイトの間隔は Fetcher をまたいで守る)。
func NewRegistry(cfg Config) *Registry {
	interval := cfg.Interval
	if interval == 0 {
		interval = MinInterval
	}
	clock := cfg.Clock
	if clock == nil {
		clock = SystemClock
	}
	m := map[item.FetchType]Fetcher{}
	if appID := strings.TrimSpace(cfg.YahooAppID); appID != "" {
		y := NewYahoo(appID, cfg.Client)
		y.Endpoint = cfg.YahooEndpoint
		m[item.FetchAPI] = Throttle(y, interval, clock)
	}
	return &Registry{m: m}
}

// NewRegistryWith は m をそのまま使う表(テスト用。Throttle で包まない)。link_only は m にあっても使わない。
func NewRegistryWith(m map[item.FetchType]Fetcher) *Registry {
	c := make(map[item.FetchType]Fetcher, len(m))
	for t, f := range m {
		if t != item.FetchLinkOnly && f != nil {
			c[t] = f
		}
	}
	return &Registry{m: c}
}

// For は fetch_type の Fetcher を返す。取得しない・使えないなら false。
func (r *Registry) For(t item.FetchType) (Fetcher, bool) {
	f, ok := r.m[t]
	return f, ok
}
