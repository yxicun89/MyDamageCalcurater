package migrations

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// 000006(リンク用サイト 3 つ。魂ウェブ・ポケモンセンターオンライン・プレバン)の SQL の形(MySQL なしで確かめる。docs/phase3-api-spec.md の AC-D5)。
// 000004 と同じ作り方:
//   - sites は id を明示せず、同じ名前があれば足さない(WHERE NOT EXISTS)。すべて link_only・is_reference=FALSE
//   - genre_sites は名前で引き、無い組だけ足す。up にジャンルの追加・既存行の UPDATE・DELETE などを書かない
//   - down は DROP・TRUNCATE をしない。消す対象は名前と URL の両方が一致する行とその紐づけだけ
//   - 確認済みの URL は confirmedURLs(layout_test.go)。up に書く URL はその 3 つ
func TestSeedLinkSites000006Shape(t *testing.T) {
	up, err := FS.ReadFile("000006_seed_link_sites.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("000006_seed_link_sites.down.sql")
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
			t.Errorf("down に %q がある(000006 で足した行だけを消す)", strings.TrimSpace(bad))
		}
	}
	for _, name := range []string{"魂ウェブ", "ポケモンセンターオンライン", "プレバン"} {
		if !strings.Contains(string(up), "'"+name+"'") {
			t.Errorf("up にサイト %s が無い", name)
		}
		if !strings.Contains(string(down), "'"+name+"'") {
			t.Errorf("down がサイト %s を消さない", name)
		}
	}
	for _, url := range []string{"https://tamashiiweb.com/item/?wo={q}", "https://www.pokemoncenter-online.com/search/?q={q}", "https://p-bandai.jp/search_bst/?q={q}"} {
		if !strings.Contains(string(up), "'"+url+"'") {
			t.Errorf("up に URL %s が無い", url)
		}
		if !strings.Contains(string(down), "'"+url+"'") {
			t.Errorf("down が URL %s を条件にしていない(名前と URL の両方で特定)", url)
		}
	}
	if strings.Count(u, "'LINK_ONLY'") < 3 || strings.Contains(u, "'SCRAPE'") || strings.Contains(u, "'HEADLESS'") || strings.Contains(u, "'API'") {
		t.Error("3 サイトはすべて link_only のはず")
	}
	if strings.Contains(u, "TRUE") {
		t.Error("is_reference に TRUE がある(リンク用は基準サイトにしない)")
	}
}

// sites.md には、プレバンの検索語の文字コードが未確認であること、ドラゴンスターが取得不可(回避しない)であることを書く(AC-D5・AC-H12)。
func TestSitesDocNotes(t *testing.T) {
	b, err := os.ReadFile("../../docs/sites.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	// プレバン: 検索語の文字コードが未確認であること(プレバンの節に書く)。
	if !regexp.MustCompile(`(?s)### プレバン.{0,800}文字コード[^\n]*未確認|### プレバン.{0,800}未確認[^\n]*文字コード`).MatchString(doc) {
		t.Error("docs/sites.md の「### プレバン」の節に、検索語の文字コードが未確認と書かれていない")
	}
	// ドラゴンスター: 取得不可(回避しない)。
	if !regexp.MustCompile(`(?s)### ドラゴンスター.{0,600}取得不可\(回避しない\)`).MatchString(doc) {
		t.Error("docs/sites.md の「### ドラゴンスター」の節に「取得不可(回避しない)」が無い")
	}
}
