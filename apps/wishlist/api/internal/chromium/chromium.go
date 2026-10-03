// Package chromium は fetcher.Renderer の本番実装(chromedp。ローカルの headless-shell を ExecAllocator で起動する)。
// UA は既定のまま(偽装しない。docs/sites-headless.md)。実際のブラウザはテストから起動しない。
package chromium

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
)

// ErrLaunch は Chromium を起動できなかったこと(実行ファイルが無い等)。
var ErrLaunch = errors.New("chromium: cannot start the browser")

// launchTimeout はブラウザの起動(最初の接続)を待つ上限。
const launchTimeout = 30 * time.Second

// shutdownTimeout はブラウザの終了を待つ上限(超えたら強制的に取り消して先へ進む)。
const shutdownTimeout = 10 * time.Second

// Options は起動の設定。
type Options struct {
	// NoSandbox は --no-sandbox を付ける(コンテナの制限でサンドボックスが使えないとき)。
	NoSandbox bool
}

// New は execPath の headless-shell を使う Renderer を返す。作るだけでは起動しない(OpenPage で初めて起動する)。
func New(execPath string, opts ...Options) fetcher.Renderer {
	r := renderer{execPath: execPath}
	if len(opts) > 0 {
		r.opts = opts[0]
	}
	return r
}

type renderer struct {
	execPath string
	opts     Options
}

func (r renderer) OpenPage(ctx context.Context) (fetcher.Page, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// user-data-dir はページごとの一時ディレクトリ(TMPDIR。Close で消す)。
	dir, err := os.MkdirTemp("", "wishlist-chromium-")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLaunch, err)
	}
	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocOpts = append(allocOpts, chromedp.ExecPath(r.execPath), chromedp.UserDataDir(dir))
	if r.opts.NoSandbox {
		allocOpts = append(allocOpts, chromedp.NoSandbox)
	}
	actx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	bctx, cancelBrowser := chromedp.NewContext(actx)
	started := false
	p := &page{ctx: bctx, cleanup: func() {
		// 起動できたときは、Chromium の終了を待ってから user-data-dir を消す(動いている間に消すと書き込み中のファイルが残る)。
		// 起動に失敗したときはプロセスが無い(Cancel は待ち続けるので呼ばない)。
		if started {
			done := make(chan struct{})
			go func() {
				_ = chromedp.Cancel(bctx)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(shutdownTimeout):
			}
		}
		cancelBrowser()
		cancelAlloc()
		_ = os.RemoveAll(dir)
	}}

	// 最初の Run の ctx がブラウザの寿命になるので、期限付きの子 ctx は使わず、タイマーで取り消す。
	timer := time.AfterFunc(launchTimeout, cancelBrowser)
	err = chromedp.Run(bctx) // 何もせず実行すると、ブラウザを起動して接続する
	timer.Stop()
	if err != nil {
		p.cleanup()
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, fmt.Errorf("%w: %w", ErrLaunch, err)
	}
	started = true
	return p, nil
}

type page struct {
	ctx     context.Context // chromedp のブラウザ(タブ)の ctx
	cleanup func()
}

// run は呼び出しの ctx(取り消し・期限)を効かせて actions を実行する。
func (p *page) run(ctx context.Context, actions ...chromedp.Action) error {
	c, cancel := context.WithCancel(p.ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	err := chromedp.Run(c, actions...)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
	}
	return err
}

func (p *page) Navigate(ctx context.Context, rawURL string) error {
	return p.run(ctx, chromedp.Navigate(rawURL))
}

func (p *page) WaitVisible(ctx context.Context, selector string) error {
	return p.run(ctx, chromedp.WaitVisible(selector, chromedp.ByQuery))
}

func (p *page) CountVisible(ctx context.Context, selector string) (int, error) {
	var nodes int
	err := p.run(ctx, chromedp.Evaluate(
		fmt.Sprintf("document.querySelectorAll(%q).length", selector), &nodes))
	return nodes, err
}

func (p *page) ScrollToBottom(ctx context.Context) error {
	return p.run(ctx, chromedp.Evaluate("window.scrollTo(0, document.body.scrollHeight)", nil))
}

func (p *page) HTML(ctx context.Context) (string, error) {
	var html string
	err := p.run(ctx, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
	return html, err
}

func (p *page) Close() error {
	p.cleanup()
	return nil
}
