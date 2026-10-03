package official

import (
	"context"
	"net/http"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
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
	cfg Config
}

// NewChecker は Checker を返す。
func NewChecker(cfg Config) *Checker {
	return &Checker{cfg: cfg}
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
	return Result{} // TODO(implementer)
}
