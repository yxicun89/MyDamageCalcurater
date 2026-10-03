package official

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

const (
	// MaxPageBytes は取得する公式ページの本文の上限(超えたら failed)。
	MaxPageBytes = 2 << 20
	// MaxRobotsBytes は取得する robots.txt の上限(超えたら取らない = failed)。
	MaxRobotsBytes = 512 << 10
)

// Config は NewChecker の設定。
type Config struct {
	// Client は取得に使う(nil なら netguard.NewClient(netguard.Options{}))。別のホストへのリダイレクトは追わない(Checker が包む)。
	Client *http.Client
	// Gate はホスト単位の間隔(価格の取得と共有する。refresher は fetcher.Registry.Gate() を渡す)。
	// nil なら fetcher.NewHostGate(fetcher.MinInterval, fetcher.HostMinIntervals, Clock)。
	Gate *fetcher.HostGate
	// Clock は Gate が nil のときに使う(nil なら fetcher.SystemClock)。
	Clock fetcher.Clock
}

// Checker は公式ページを取得して判定する。robots.txt はホストごとに Checker の寿命の間だけ覚える
// (refresher は 1 回の実行で 1 つ作る = 1 回の実行の中でホストごとに 1 回だけ取る)。並行に呼んでもよい。
type Checker struct {
	client *http.Client
	gate   *fetcher.HostGate

	mu      sync.Mutex
	origins map[string]*robotsEntry
}

// robotsEntry はオリジンごとの robots.txt の結果(1 回だけ取る)。
type robotsEntry struct {
	mu     sync.Mutex
	done   bool
	robots Robots
	fail   item.OfficialState // 空なら取れた(robots を使う)。blocked・failed なら取れなかった結果
}

// NewChecker は Checker を返す。
func NewChecker(cfg Config) *Checker {
	client := cfg.Client
	if client == nil {
		client = netguard.NewClient(netguard.Options{})
	}
	gate := cfg.Gate
	if gate == nil {
		gate = fetcher.NewHostGate(fetcher.MinInterval, fetcher.HostMinIntervals, cfg.Clock)
	}
	return &Checker{client: client, gate: gate, origins: map[string]*robotsEntry{}}
}

var (
	errOtherHost       = errors.New("official: redirect to another host")
	errRedirectBlocked = errors.New("official: redirect target disallowed by robots.txt")
)

func finished(state item.OfficialState) Result {
	return Result{State: state, Evidence: []string{}}
}

// get は u を Gate を通して取得し、(状態コード, 本文)を返す。リダイレクトは同じ scheme・ホスト(ポート込み)だけ追い、
// allowed があれば移動先がそれを満たすかも確かめる(満たさなければ errRedirectBlocked)。
func (c *Checker) get(ctx context.Context, u *url.URL, limit int64, allowed func(*url.URL) bool) (status int, body []byte, ctype string, err error) {
	client := *c.client
	orig := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if orig != nil {
			if err := orig(req, via); err != nil {
				return err
			}
		} else if len(via) >= 10 {
			return errors.New("official: too many redirects")
		}
		if req.URL.Scheme != u.Scheme || req.URL.Host != u.Host {
			return errOtherHost
		}
		if allowed != nil && !allowed(req.URL) {
			return errRedirectBlocked
		}
		return nil
	}
	err = c.gate.Do(ctx, fetcher.HostOf(u.String()), func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", fetcher.UserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		status = resp.StatusCode
		ctype = resp.Header.Get("Content-Type")
		if status < 200 || status > 299 {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			return nil
		}
		body, err = netguard.ReadLimited(resp.Body, limit)
		return err
	})
	return status, body, ctype, err
}

// robotsFor はオリジンの robots.txt を(まだなら)取得して返す。取れなかったときの状態(blocked・failed)は fail に入る。
func (c *Checker) robotsFor(ctx context.Context, u *url.URL) (Robots, item.OfficialState) {
	origin := u.Scheme + "://" + u.Host
	c.mu.Lock()
	e, ok := c.origins[origin]
	if !ok {
		e = &robotsEntry{}
		c.origins[origin] = e
	}
	c.mu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.done {
		return e.robots, e.fail
	}
	ru := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/robots.txt"}
	status, body, _, err := c.get(ctx, ru, MaxRobotsBytes, nil)
	switch {
	case ctx.Err() != nil:
		return Robots{}, item.OfficialFailed // 取り消しは覚えない
	case err != nil:
		e.fail = item.OfficialFailed
	case status >= 200 && status <= 299:
		e.robots = ParseRobots(string(body), RobotsProduct)
	case status == http.StatusNotFound || status == http.StatusGone:
		e.robots = AllowAll()
	case status >= 400 && status <= 499:
		e.fail = item.OfficialBlocked
	default:
		e.fail = item.OfficialFailed
	}
	e.done = true
	return e.robots, e.fail
}

// Check は rawURL の公式ページの状態を返す(エラーは返さず、状態で表す)。
//
//  1. rawURL が http(s) の絶対 URL でなければ failed(取得しない)
//  2. そのホストの「scheme://host[:port]/robots.txt」を(まだなら)取得する。Gate を通し、User-Agent は fetcher.UserAgent
//     - 2xx → ParseRobots(本文, RobotsProduct)。404・410 → 全部許す。それ以外の 4xx → blocked。5xx・通信失敗・MaxRobotsBytes 超え → failed
//     (取れなかったことも覚える。同じ実行の中で同じホストは取り直さない)
//  3. Allowed(パス + クエリ) が false なら blocked(ページは取得しない)
//  4. ページを Gate を通して取得する(User-Agent は fetcher.UserAgent)。2xx 以外・通信失敗・MaxPageBytes 超え・別のホストへのリダイレクト → failed
//  5. Judge(ExtractText(本文))
//
// ctx が終わったら failed を返す(呼び出し側が ctx.Err() で止める)。
func (c *Checker) Check(ctx context.Context, rawURL string) Result {
	u, err := netguard.ValidateURL(rawURL)
	if err != nil || ctx.Err() != nil {
		return finished(item.OfficialFailed)
	}
	robots, fail := c.robotsFor(ctx, u)
	if fail != "" {
		return finished(fail)
	}
	if !robots.Allowed(u.RequestURI()) {
		return finished(item.OfficialBlocked)
	}
	status, body, ctype, err := c.get(ctx, u, MaxPageBytes, func(r *url.URL) bool { return robots.Allowed(r.RequestURI()) })
	switch {
	case errors.Is(err, errRedirectBlocked):
		return finished(item.OfficialBlocked)
	case err != nil || status < 200 || status > 299:
		return finished(item.OfficialFailed)
	}
	return Judge(extractText(body, ctype))
}
