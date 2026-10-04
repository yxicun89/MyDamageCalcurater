// Package fetcher はサイト別の価格取得(apps/wishlist/CLAUDE.md §6)。受け入れ条件は docs/phase3-api-spec.md の AC-F*。
//
// 取得方式(sites.fetch_type)ごとの実装:
//   - api      : Yahoo!ショッピング(Yahoo。appid が無ければ使わない)
//   - scrape   : カードラッシュ・あみあみ・Yahoo!フリマ・駿河屋(検索 URL のホスト名で選ぶ。ForSite)。
//     駿河屋(www.suruga-ya.jp)は Crawl-delay 30 秒を守り、夜間の CronJob だけで取る(HostMinIntervals・NightlyOnlyHosts)。
//     ドラゴンスターは取得不可(docs/sites.md)
//   - headless : メルカリ(jp.mercari.com。chromedp。Config.Renderer があるときだけ。夜間の CronJob だけで取る)
//   - link_only: 取得しない
//
// 実サイトへのアクセスはテストからは行わない(fixture・httptest を使う。仕様 §13)。
package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

const (
	// MaxListings は 1 サイトあたりに取得する上位件数(仕様 §6)。
	MaxListings = 20
	// MinInterval は同じサイトへのリクエストの最低間隔(仕様 §6)。
	MinInterval = 5 * time.Second
	// UserAgent は取得時に送る User-Agent。偽装せず、目的が分かるように名乗る(個人情報・リポジトリ URL は入れない)。
	UserAgent = "wishlist-price-checker/0.1 (personal use; +https://github.com/)"

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

// Throttle は inner を包み、同じホスト(検索 URL テンプレートのホスト名。読めなければ Site.ID)へのリクエストを直列にし、前回の取得が終わってから
// interval 以上あけてから次を始める(待ちは clock.Sleep)。初回は待たない。違うホストは互いに待たない(同じホストの別サイト行も同じ待ち合わせ)。
// 待っている間に ctx が終わったら inner を呼ばずに ctx の err を返す。inner の失敗も「取得した」として間隔を数える。
func Throttle(inner Fetcher, interval time.Duration, clock Clock) Fetcher {
	return ThrottleWith(inner, interval, nil, clock)
}

// HostMinIntervals はホストごとの最小間隔(既定の MinInterval より長いもの)。robots.txt の Crawl-delay に合わせる。
// 駿河屋は Crawl-delay: 30(docs/sites.md。ユーザー決定 2026-10-04)。キーは小文字・ポートなしのホスト名。
var HostMinIntervals = map[string]time.Duration{
	"www.suruga-ya.jp": 30 * time.Second,
}

// NightlyOnlyHosts は夜間の CronJob だけで取るホスト(api の裏の更新・手動の更新では取らない)。
var NightlyOnlyHosts = map[string]bool{
	"www.suruga-ya.jp": true,
	mercariHost:        true,
}

// mercariHost は headless で取るメルカリのホスト。
const mercariHost = "jp.mercari.com"

// ThrottleWith は Throttle にホストごとの最小間隔 hostIntervals(nil 可)を足したもの。
// そのホストの間隔は interval と表の値の長いほう。
func ThrottleWith(inner Fetcher, interval time.Duration, hostIntervals map[string]time.Duration, clock Clock) Fetcher {
	return throttleOn(inner, NewHostGate(interval, hostIntervals, clock))
}

// throttleOn は inner を gate(他の Fetcher・公式ページの取得と共有してよい)で包む。
func throttleOn(inner Fetcher, gate *HostGate) Fetcher {
	return &throttled{inner: inner, gate: gate}
}

type throttled struct {
	inner Fetcher
	gate  *HostGate
}

// gateKey は待ち合わせのキー。検索 URL テンプレートのホスト名(小文字・ポートなし)。読めなければ Site.ID。
func gateKey(site Site) string {
	if h := siteHost(site); h != "" {
		return h
	}
	return "id:" + strconv.FormatInt(site.ID, 10)
}

// siteHost は検索 URL テンプレートのホスト名(小文字・ポートなし)。読めなければ空。
func siteHost(site Site) string {
	u, err := url.Parse(site.SearchURLTemplate)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func (t *throttled) Fetch(ctx context.Context, site Site, query string) ([]Listing, error) {
	var out []Listing
	err := t.gate.Do(ctx, gateKey(site), func(ctx context.Context) error {
		var err error
		out, err = t.inner.Fetch(ctx, site, query)
		return err
	})
	return out, err
}

// Config は NewRegistry の設定。
type Config struct {
	YahooAppID    string        // 空(空白だけを含む)なら api 型は「使えない」
	YahooEndpoint string        // 空なら YahooEndpoint
	Client        *http.Client  // nil なら netguard.NewClient(netguard.Options{})
	Clock         Clock         // nil なら SystemClock
	Interval      time.Duration // 0 なら MinInterval
	// Renderer は headless(メルカリ)の描画。nil なら headless は登録しない(Chromium が無い環境。refresher だけが渡す)。
	Renderer Renderer
}

// Registry は fetch_type から Fetcher を選ぶ表。
type Registry struct {
	m map[item.FetchType]Fetcher
	// scrape は検索 URL テンプレートのホスト名(小文字)ごとの Fetcher。NewRegistryWith では使わない。
	scrape map[string]Fetcher
	// headless は同じくホストごとの Fetcher(Config.Renderer があるときだけ)。
	headless map[string]Fetcher
	// gate は NewRegistry の Fetcher が共有する HostGate(NewRegistryWith では nil)。
	gate *HostGate
}

// NewRegistry は本番の表を作る。api は Yahoo(YahooAppID があるときだけ)。scrape(ホスト別)・headless(メルカリ。Renderer があるときだけ)、link_only は取得しない。
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
	gate := NewHostGate(interval, HostMinIntervals, clock)
	m := map[item.FetchType]Fetcher{}
	if appID := strings.TrimSpace(cfg.YahooAppID); appID != "" {
		y := NewYahoo(appID, cfg.Client)
		y.Endpoint = cfg.YahooEndpoint
		m[item.FetchAPI] = throttleOn(y, gate)
	}
	scrape := map[string]Fetcher{
		"www.cardrush-dm.jp":           throttleOn(NewCardrush(cfg.Client), gate),
		"slist.amiami.jp":              throttleOn(NewAmiami(cfg.Client), gate),
		"paypayfleamarket.yahoo.co.jp": throttleOn(NewYahooFurima(cfg.Client), gate),
		"www.suruga-ya.jp":             throttleOn(NewSurugaya(cfg.Client), gate),
	}
	reg := &Registry{m: m, scrape: scrape, gate: gate}
	if cfg.Renderer != nil {
		reg.headless = map[string]Fetcher{
			mercariHost: throttleOn(NewMercari(cfg.Renderer, clock), gate),
		}
	}
	return reg
}

// NightlyOnly は site が夜間の CronJob だけで取るサイトか(NightlyOnlyHosts。検索 URL テンプレートのホストで決める)。
func (r *Registry) NightlyOnly(site Site) bool { return NightlyOnlyHosts[siteHost(site)] }

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

// ForSite は site を取得する Fetcher を返す。scrape は fetch_type と検索 URL テンプレートのホスト名(完全一致・
// 大文字小文字とポートは無視)で選ぶ。それ以外は For(fetch_type)。NewRegistryWith の表はホストを見ない。
func (r *Registry) ForSite(site Site) (Fetcher, bool) {
	if site.FetchType == item.FetchScrape && r.scrape != nil {
		f, ok := r.scrape[siteHost(site)]
		return f, ok
	}
	if site.FetchType == item.FetchHeadless && r.headless != nil {
		f, ok := r.headless[siteHost(site)]
		return f, ok
	}
	return r.For(site.FetchType)
}
