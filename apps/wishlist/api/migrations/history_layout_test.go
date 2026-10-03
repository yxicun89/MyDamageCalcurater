package migrations

import (
	"regexp"
	"strings"
	"testing"
)

// フェーズ4-2 価格の推移の migration 000007(docs/phase4-spec.md AC-H14)。MySQL なしで SQL の形を確かめる
// (制約の効き目は history_mysql_test.go が実 DB で確かめる)。
//   - up は price_history を CREATE TABLE IF NOT EXISTS で作る。主キーは (item_id, site_id, day)、day は DATE
//   - item_id は items、site_id は sites を ON DELETE CASCADE で参照する(商品・サイトを消すと推移も消える)
//   - low は NOT NULL(点を作るのは low がある日だけ)、mid は NULL を許す
//   - up は既存の表の行を変えない・足さない(seed を持たない)
//   - down は price_history だけを消す
//
// 000006 は別の PR(#601)で入る。それが main に入るまでは TestLayout が「版 000006 の up が無い」で落ちる(想定内)。
func TestPriceHistory000007Shape(t *testing.T) {
	up, err := FS.ReadFile("000007_price_history.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("000007_price_history.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	noComments := func(s string) string { return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "") }
	space := regexp.MustCompile(`\s+`)
	u := space.ReplaceAllString(strings.ToUpper(noComments(string(up))), " ")
	d := space.ReplaceAllString(strings.ToUpper(noComments(string(down))), " ")

	if !strings.Contains(u, "CREATE TABLE IF NOT EXISTS PRICE_HISTORY") {
		t.Error("up が price_history を CREATE TABLE IF NOT EXISTS で作らない")
	}
	if !regexp.MustCompile(`PRIMARY KEY ?\( ?ITEM_ID ?, ?SITE_ID ?, ?DAY ?\)`).MatchString(u) {
		t.Error("主キーが (item_id, site_id, day) でない(1 商品×1 サイト×1 日 1 行)")
	}
	if !regexp.MustCompile(`\bDAY DATE NOT NULL\b`).MatchString(u) {
		t.Error("day が DATE NOT NULL でない")
	}
	if !regexp.MustCompile(`\bLOW INT NOT NULL\b`).MatchString(u) {
		t.Error("low が INT NOT NULL でない(点を作るのは low がある日だけ)")
	}
	if !regexp.MustCompile(`\bMID INT( NULL)?,`).MatchString(u) || regexp.MustCompile(`\bMID INT NOT NULL\b`).MatchString(u) {
		t.Error("mid が NULL を許す INT でない(件数 3 未満の日は null)")
	}
	for _, col := range []string{`\bCOUNT INT NOT NULL\b`, `\bRECORDED_AT DATETIME NOT NULL\b`} {
		if !regexp.MustCompile(col).MatchString(u) {
			t.Errorf("列 %s が無い", col)
		}
	}
	if !regexp.MustCompile(`FOREIGN KEY ?\( ?ITEM_ID ?\) REFERENCES ITEMS ?\( ?ID ?\) ON DELETE CASCADE`).MatchString(u) {
		t.Error("item_id が items を ON DELETE CASCADE で参照しない")
	}
	if !regexp.MustCompile(`FOREIGN KEY ?\( ?SITE_ID ?\) REFERENCES SITES ?\( ?ID ?\) ON DELETE CASCADE`).MatchString(u) {
		t.Error("site_id が sites を ON DELETE CASCADE で参照しない")
	}
	for _, bad := range []string{`\bINSERT\b`, `\bUPDATE \w+ SET\b`, `\bREPLACE\b`, `\bDELETE (\w+ )?FROM\b`, `\bTRUNCATE\b`, `\bDROP\b`, `\bALTER TABLE (ITEMS|SITES|ESTIMATES|LISTINGS|GENRES)\b`} {
		if regexp.MustCompile(bad).MatchString(u) {
			t.Errorf("up に %s がある(price_history を作るだけ)", bad)
		}
	}
	if !strings.Contains(d, "DROP TABLE IF EXISTS PRICE_HISTORY") {
		t.Error("down が price_history を消さない")
	}
	rest := strings.ReplaceAll(d, "PRICE_HISTORY", "")
	for _, bad := range []string{"ITEMS", "SITES", "ESTIMATES", "LISTINGS", "GENRES", "TRUNCATE", "DELETE"} {
		if strings.Contains(rest, bad) {
			t.Errorf("down が price_history 以外(%s)に触れている", bad)
		}
	}
}
