package fetcher

import (
	"context"
	"sync"
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

	mu    sync.Mutex
	hosts map[string]*siteGate
}

// NewHostGate は HostGate を返す。clock が nil なら SystemClock。
func NewHostGate(interval time.Duration, hostIntervals map[string]time.Duration, clock Clock) *HostGate {
	if clock == nil {
		clock = SystemClock
	}
	return &HostGate{interval: interval, hostIntervals: hostIntervals, clock: clock, hosts: map[string]*siteGate{}}
}

// Do は host(小文字・ポートなしのホスト名。HostOf の値)への 1 回の取得 fn を、間隔を守って実行する。
func (g *HostGate) Do(ctx context.Context, host string, fn func(context.Context) error) error {
	g.mu.Lock()
	sg, ok := g.hosts[host]
	if !ok {
		sg = &siteGate{sem: make(chan struct{}, 1)}
		g.hosts[host] = sg
	}
	g.mu.Unlock()
	select {
	case sg.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-sg.sem }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !sg.last.IsZero() {
		iv := g.interval
		if d := g.hostIntervals[host]; d > iv {
			iv = d
		}
		if wait := iv - g.clock.Now().Sub(sg.last); wait > 0 {
			if err := g.clock.Sleep(ctx, wait); err != nil {
				return err
			}
		}
	}
	defer func() { sg.last = g.clock.Now() }()
	return fn(ctx)
}

// HostOf は URL(テンプレートでもよい)のホスト名を小文字・ポートなしで返す。読めなければ空。
func HostOf(rawURL string) string {
	return siteHost(Site{SearchURLTemplate: rawURL})
}

// Gate は NewRegistry が登録した Fetcher が使う HostGate(公式ページの取得と共有する)。NewRegistryWith の表では nil。
func (r *Registry) Gate() *HostGate {
	return r.gate
}

// siteGate はホストごとの直列化(容量 1 のチャネル。待ちを ctx で取り消せる)と、前回の取得が終わった時刻。
type siteGate struct {
	sem  chan struct{}
	last time.Time // 前回の取得が終わった時刻(初回はゼロ値)
}
