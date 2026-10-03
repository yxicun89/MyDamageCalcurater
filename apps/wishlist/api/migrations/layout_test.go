package migrations

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
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

// confirmedURLs は、確認済みの検索 URL テンプレート(docs/sites.md の一覧と一致させる。CLAUDE.md §5)。
// 初期データ(000002・000004)に入れてよい URL はここにあるものだけ。足すときは sites.md の確認結果を先に書く。
var confirmedURLs = []string{
	"https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc",
	"https://www.amazon.co.jp/s?k={q}&s=price-asc-rank",
	"https://paypayfleamarket.yahoo.co.jp/search/{q}",
	"https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20",
	"https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea",
	"https://shopping.yahoo.co.jp/search/{q}/0/?X=2",
	"https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On",
}

// 初期データのサイトは、仕様で確認済みの URL だけであること(推測で URL を足さない。CLAUDE.md §5)。
// 000002 に加えて、サイトを足す 000004 も対象(対象を足しただけで、検査は弱めていない)。
func TestSeedSitesAreConfirmedOnly(t *testing.T) {
	confirmed := map[string]bool{}
	for _, u := range confirmedURLs {
		confirmed[u] = true
	}
	for _, name := range []string{"000002_seed.up.sql", "000004_seed_sites.up.sql"} {
		t.Run(name, func(t *testing.T) {
			b, err := FS.ReadFile(name)
			if err != nil {
				t.Fatal(err)
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
		})
	}
}

// 確認済みの一覧は docs/sites.md と一致させる(sites.md に書かれていない URL を確認済みにしない)。
// 人が登録するための候補(プレバン・魂ウェブ・ポケセン)は未確認の点があるので、確認済みに入れない。
func TestConfirmedURLsMatchSitesDoc(t *testing.T) {
	b, err := os.ReadFile("../../docs/sites.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, u := range confirmedURLs {
		if !strings.Contains(doc, u) {
			t.Errorf("確認済みの URL が docs/sites.md に無い: %s", u)
		}
	}
	for _, u := range []string{"https://p-bandai.jp/search_bst/?q={q}", "https://tamashiiweb.com/item/?wo={q}", "https://www.pokemoncenter-online.com/?word={q}&main_page=search_result"} {
		for _, c := range confirmedURLs {
			if c == u {
				t.Errorf("未確認の点があるサイトが確認済みに入っている: %s", u)
			}
		}
	}
}

// 000004 の SQL の形(MySQL なしで確かめる。内容は seed_mysql_test.go が実 DB で確かめる)。
// 使われている DB に流しても壊れないこと:
//   - sites は id を明示せず、同じ名前があれば足さない(up の INSERT は WHERE NOT EXISTS 等で名前を見る)
//   - genre_sites は紐づけの既存行を変えない(UPDATE・REPLACE・ON DUPLICATE KEY UPDATE・DELETE を up に書かない)
//   - up はジャンルを足さない・消さない
//   - down は DROP・TRUNCATE をしない。消す対象は 000004 で足した 5 サイト(名前と URL で特定)とその紐づけだけ
func TestSeedSites000004Shape(t *testing.T) {
	up, err := FS.ReadFile("000004_seed_sites.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("000004_seed_sites.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	noComments := func(s string) string { return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "") }
	u, d := strings.ToUpper(noComments(string(up))), strings.ToUpper(noComments(string(down)))

	for _, bad := range []string{"UPDATE ", "REPLACE ", "ON DUPLICATE KEY", "DELETE ", "TRUNCATE", "DROP ", "INTO GENRES"} {
		if strings.Contains(u, bad) {
			t.Errorf("up に %q がある(既存の行を変えない・消さない)", strings.TrimSpace(bad))
		}
	}
	if regexp.MustCompile(`INSERT\s+(IGNORE\s+)?INTO\s+SITES\s*\(\s*ID\b`).MatchString(u) {
		t.Error("sites の INSERT が id を明示している(名前で紐づける)")
	}
	if !strings.Contains(u, "NOT EXISTS") && !strings.Contains(u, "INSERT IGNORE") {
		t.Error("up に、同じ名前のサイトを足さない仕組み(NOT EXISTS 等)が無い")
	}
	for _, bad := range []string{"DROP ", "TRUNCATE", "DELETE FROM GENRES", "DELETE FROM SITES;"} {
		if strings.Contains(d, bad) {
			t.Errorf("down に %q がある(000004 で足した行だけを消す)", strings.TrimSpace(bad))
		}
	}
	for _, name := range []string{"Yahoo!フリマ", "カードラッシュ", "あみあみ", "Yahoo!ショッピング", "駿河屋"} {
		if !strings.Contains(string(up), "'"+name+"'") {
			t.Errorf("up にサイト %s が無い", name)
		}
		if !strings.Contains(string(down), "'"+name+"'") {
			t.Errorf("down がサイト %s を消さない", name)
		}
	}
	for _, name := range []string{"プレミアムバンダイ", "プレバン", "魂ウェブ", "ポケモンセンターオンライン"} {
		if strings.Contains(string(up), name) {
			t.Errorf("up に未確認のサイト %s がある(sites.md に人が登録する候補として残す)", name)
		}
	}
}
