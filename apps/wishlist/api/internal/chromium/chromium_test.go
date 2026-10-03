package chromium_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/chromium"
	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
)

// AC-H11: 本番の Renderer(chromedp)。実際のブラウザは起動しない(テストは起動前の振る舞いだけ)。
//   - New は起動しない(存在しないパスでも nil でなく、エラーも出ない)
//   - 実行ファイルを起動できなければ OpenPage は chromium.ErrLaunch を包んだ error を返す(panic しない・待たされない)
//   - ctx が取り消し済みなら context.Canceled
//
// 実際の描画の確認は手動(docker build 後に refresher イメージで 1 回。docs/sites-headless.md)。
func TestNew_DoesNotLaunch(t *testing.T) {
	var r fetcher.Renderer = chromium.New("/nonexistent/headless-shell")
	if r == nil {
		t.Fatal("New が nil")
	}
}

func TestOpenPage_MissingBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := chromium.New("/nonexistent/headless-shell").OpenPage(ctx)
	if err == nil {
		if p != nil {
			p.Close()
		}
		t.Fatal("存在しない実行ファイルなのにエラーにならない")
	}
	if !errors.Is(err, chromium.ErrLaunch) {
		t.Errorf("err = %v, want chromium.ErrLaunch を包む", err)
	}
	if p != nil {
		t.Errorf("エラーなのに Page を返した: %v", p)
	}
}

func TestOpenPage_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := chromium.New("/nonexistent/headless-shell").OpenPage(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// OpenPage が失敗しても、user-data-dir(TMPDIR の wishlist-chromium-*)を残さない。
func TestOpenPage_FailureLeavesNoTempDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if _, err := chromium.New("/nonexistent/headless-shell").OpenPage(context.Background()); err == nil {
		t.Fatal("存在しない実行ファイルなのにエラーにならない")
	}
	left, err := filepath.Glob(filepath.Join(tmp, "wishlist-chromium-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		entries, _ := os.ReadDir(tmp)
		t.Errorf("一時ディレクトリが残っている: %v", entries)
	}
}
