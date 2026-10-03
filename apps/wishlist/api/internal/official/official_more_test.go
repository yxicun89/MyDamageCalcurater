package official_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/japanese"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
)

// レビュー指摘(robots の計算量・BOM・誤判定の語・文字コード・上限ちょうど)の追加テスト。

func TestRobots_NoExponentialBlowup(t *testing.T) {
	r := official.ParseRobots("User-agent: *\nDisallow: /*a*a*a*a*a*a*a*a*a*a*b\n", official.RobotsProduct)
	path := "/" + strings.Repeat("a", 5000)
	start := time.Now()
	if !r.Allowed(path) {
		t.Error("b が無いので許可されるはず")
	}
	if r.Allowed(path+"b") {
		t.Error("b で終わるパスは禁止のはず")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("遅すぎる: %v", d)
	}
}

func TestRobots_BOM(t *testing.T) {
	r := official.ParseRobots("\uFEFFUser-agent: *\nDisallow: /item/\n", official.RobotsProduct)
	if r.Allowed("/item/1") {
		t.Error("BOM つきでも最初の User-agent を読む")
	}
}

func TestJudge_EndedPhrases(t *testing.T) {
	cases := []struct {
		text  string
		state item.OfficialState
		ev    []string
	}{
		{"予約受付は終了しました", item.OfficialEnded, []string{"予約受付は終了"}},
		{"予約受付を終了しました", item.OfficialEnded, []string{"予約受付を終了"}},
		{"販売は終了しました", item.OfficialEnded, []string{"販売は終了"}},
		{"受付を終了しました", item.OfficialUnknown, []string{}},
		{"受付は終了しました", item.OfficialUnknown, []string{}},
		{"予約受付は終了しました カートに入れる", item.OfficialAmbiguous, []string{"予約受付は終了", "カートに入れる"}},
	}
	for _, c := range cases {
		got := official.Judge(c.text)
		if got.State != c.state || fmt.Sprintf("%q", got.Evidence) != fmt.Sprintf("%q", c.ev) {
			t.Errorf("Judge(%q) = %s %q, want %s %q", c.text, got.State, got.Evidence, c.state, c.ev)
		}
	}
}

func sjis(t *testing.T, s string) string {
	t.Helper()
	b, err := japanese.ShiftJIS.NewEncoder().String(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExtractText_ShiftJIS(t *testing.T) {
	meta := sjis(t, `<html><head><meta charset="Shift_JIS"></head><body>予約受付中</body></html>`)
	if got := official.ExtractText([]byte(meta)); got != "予約受付中" {
		t.Errorf("meta の文字コード: %q", got)
	}
	// Content-Type の charset(meta なし)は Checker を通して確かめる
	s := newSite(t)
	s.robots(http.StatusNotFound, "")
	s.text("/item/1/", http.StatusOK, "text/html; charset=Shift_JIS", sjis(t, "<html><body>販売終了</body></html>"))
	got := newChecker(newFakeClock()).Check(context.Background(), s.srv.URL+"/item/1/")
	if got.State != item.OfficialEnded {
		t.Errorf("Content-Type の文字コード: %s", resultString(got))
	}
}

// 上限ちょうどは通る(robots.txt 512 KiB・ページ 2 MiB)。
func TestChecker_ExactLimitsPass(t *testing.T) {
	s := newSite(t)
	robots := "User-agent: *\nDisallow: /private/\n"
	s.robots(http.StatusOK, robots+strings.Repeat("#", official.MaxRobotsBytes-len(robots)))
	page := "<body>販売中</body>"
	s.text("/item/1/", http.StatusOK, "text/html; charset=utf-8", page+strings.Repeat(" ", official.MaxPageBytes-len(page)))
	got := newChecker(newFakeClock()).Check(context.Background(), s.srv.URL+"/item/1/")
	if got.State != item.OfficialAvailable {
		t.Errorf("上限ちょうど = %s, want available", resultString(got))
	}
}
