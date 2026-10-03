package migrations

import (
	"fmt"
	"io/fs"
	"regexp"
	"testing"
)

var fileRe = regexp.MustCompile(`^(\d{6})_[a-z0-9_]+\.(up|down)\.sql$`)

// 各版に up と down がそろい、版番号が 1 から欠けずに続くこと。
func TestLayout(t *testing.T) {
	entries, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("migration が 1 つも埋め込まれていない")
	}
	ups := map[string]bool{}
	downs := map[string]bool{}
	for _, name := range entries {
		m := fileRe.FindStringSubmatch(name)
		if m == nil {
			t.Errorf("命名規則に合わない: %s", name)
			continue
		}
		if m[2] == "up" {
			ups[m[1]] = true
		} else {
			downs[m[1]] = true
		}
	}
	for i := 1; i <= len(ups); i++ {
		v := fmt.Sprintf("%06d", i)
		if !ups[v] {
			t.Errorf("版 %s の up が無い(番号が欠けている)", v)
		}
		if !downs[v] {
			t.Errorf("版 %s の down が無い", v)
		}
	}
	if len(ups) != len(downs) {
		t.Errorf("up %d 件と down %d 件が一致しない", len(ups), len(downs))
	}
}

// 初期データのサイトは、仕様で確認済みの URL だけであること(推測で URL を足さない。CLAUDE.md §5)。
func TestSeedSitesAreConfirmedOnly(t *testing.T) {
	b, err := FS.ReadFile("000002_seed.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	confirmed := map[string]bool{
		"https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc": true,
		"https://www.amazon.co.jp/s?k={q}&s=price-asc-rank":                             true,
	}
	urls := regexp.MustCompile(`'(https?://[^']+)'`).FindAllStringSubmatch(string(b), -1)
	if len(urls) == 0 {
		t.Fatal("seed にサイトの URL が無い")
	}
	for _, u := range urls {
		if !confirmed[u[1]] {
			t.Errorf("確認済みでない URL が seed にある: %s", u[1])
		}
	}
}
