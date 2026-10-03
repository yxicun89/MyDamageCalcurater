package fetcher

import (
	"context"
	"time"
)

// フェーズ4-3: ホスト単位の間隔を、価格の Fetcher と公式ページの取得(internal/official)で共有する(docs/phase4-spec.md AC-O20・O21)。

// HostGate はホストごとの直列化と最小間隔(Throttle の中身を共有できるようにしたもの)。
// 同じホストへの取得は直列にし、前回の取得が終わってから interval(hostIntervals にあればその長いほう)以上あけてから次を始める。
// 初回は待たない。違うホストは互いに待たない。待っている間に ctx が終わったら fn を呼ばずに ctx の err を返す。fn の失敗も「取得した」と数える。
type HostGate struct {
	interval      time.Duration
	hostIntervals map[string]time.Duration
	clock         Clock
}

// NewHostGate は HostGate を返す。clock が nil なら SystemClock。
func NewHostGate(interval time.Duration, hostIntervals map[string]time.Duration, clock Clock) *HostGate {
	if clock == nil {
		clock = SystemClock
	}
	return &HostGate{interval: interval, hostIntervals: hostIntervals, clock: clock}
}

// Do は host(小文字・ポートなしのホスト名。HostOf の値)への 1 回の取得 fn を、間隔を守って実行する。
func (g *HostGate) Do(ctx context.Context, host string, fn func(context.Context) error) error {
	return fn(ctx) // TODO(implementer): 直列化と間隔(Throttle と同じ規則。Throttle もこれを使うようにする)
}

// HostOf は URL(テンプレートでもよい)のホスト名を小文字・ポートなしで返す。読めなければ空。
func HostOf(rawURL string) string {
	return siteHost(Site{SearchURLTemplate: rawURL})
}

// Gate は NewRegistry が登録した Fetcher が使う HostGate(公式ページの取得と共有する)。NewRegistryWith の表では nil。
func (r *Registry) Gate() *HostGate {
	return nil // TODO(implementer): NewRegistry で 1 つ作り、すべての Throttle と共有する
}
