package migrations

import (
	"regexp"
	"strings"
	"testing"
)

// フェーズ4-3 公式サイトの販売状況の migration 000008(docs/phase4-spec.md AC-O29)。MySQL なしで SQL の形を確かめる
// (制約の効き目は official_mysql_test.go が実 DB で確かめる)。
//   - up は items に watch_official BOOLEAN NOT NULL DEFAULT FALSE を足す
//   - up は official_status を CREATE TABLE IF NOT EXISTS で作る。主キーは item_id、items を ON DELETE CASCADE で参照する
//   - status・last_result は 8 つの ENUM(NOT NULL)、previous_status は同じ ENUM で NULL を許す。evidence は JSON NOT NULL
//   - checked_at・last_attempt_at は DATETIME NOT NULL、changed_at は DATETIME で NULL を許す
//   - up は行を足さない・変えない(seed を持たない)
//   - down は official_status を消し、items から watch_official を外す(他に触れない)
func TestOfficialStatus000008Shape(t *testing.T) {
	up, err := FS.ReadFile("000008_official_status.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("000008_official_status.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	noComments := func(s string) string { return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "") }
	space := regexp.MustCompile(`\s+`)
	u := space.ReplaceAllString(strings.ToUpper(noComments(string(up))), " ")
	d := space.ReplaceAllString(strings.ToUpper(noComments(string(down))), " ")

	if !regexp.MustCompile(`ALTER TABLE ITEMS ADD (COLUMN )?WATCH_OFFICIAL BOOLEAN NOT NULL DEFAULT FALSE\b`).MatchString(u) {
		t.Error("items に watch_official BOOLEAN NOT NULL DEFAULT FALSE を足さない")
	}
	if !strings.Contains(u, "CREATE TABLE IF NOT EXISTS OFFICIAL_STATUS") {
		t.Error("up が official_status を CREATE TABLE IF NOT EXISTS で作らない")
	}
	if !regexp.MustCompile(`PRIMARY KEY ?\( ?ITEM_ID ?\)`).MatchString(u) {
		t.Error("主キーが item_id でない(1 商品 1 行)")
	}
	if !regexp.MustCompile(`FOREIGN KEY ?\( ?ITEM_ID ?\) REFERENCES ITEMS ?\( ?ID ?\) ON DELETE CASCADE`).MatchString(u) {
		t.Error("item_id が items を ON DELETE CASCADE で参照しない")
	}
	enum := `ENUM ?\( ?'AVAILABLE' ?, ?'PREORDER' ?, ?'SOLDOUT' ?, ?'ENDED' ?, ?'UNKNOWN' ?, ?'AMBIGUOUS' ?, ?'BLOCKED' ?, ?'FAILED' ?\)`
	for _, col := range []string{`\bSTATUS ` + enum + ` NOT NULL\b`, `\bLAST_RESULT ` + enum + ` NOT NULL\b`} {
		if !regexp.MustCompile(col).MatchString(u) {
			t.Errorf("列 %s が無い(8 つの ENUM・NOT NULL)", col)
		}
	}
	if !regexp.MustCompile(`\bPREVIOUS_STATUS ` + enum + `( NULL)?,`).MatchString(u) {
		t.Error("previous_status が NULL を許す 8 つの ENUM でない")
	}
	for _, col := range []string{`\bEVIDENCE JSON NOT NULL\b`, `\bCHECKED_AT DATETIME NOT NULL\b`, `\bLAST_ATTEMPT_AT DATETIME NOT NULL\b`} {
		if !regexp.MustCompile(col).MatchString(u) {
			t.Errorf("列 %s が無い", col)
		}
	}
	if !regexp.MustCompile(`\bCHANGED_AT DATETIME( NULL)?,`).MatchString(u) || regexp.MustCompile(`\bCHANGED_AT DATETIME NOT NULL\b`).MatchString(u) {
		t.Error("changed_at が NULL を許す DATETIME でない")
	}
	for _, bad := range []string{`\bINSERT\b`, `\bUPDATE \w+ SET\b`, `\bREPLACE\b`, `\bDELETE (\w+ )?FROM\b`, `\bTRUNCATE\b`, `\bDROP\b`, `\bALTER TABLE (SITES|ESTIMATES|LISTINGS|GENRES|PRICE_HISTORY)\b`} {
		if regexp.MustCompile(bad).MatchString(u) {
			t.Errorf("up に %s がある(列と表を足すだけ)", bad)
		}
	}
	if !strings.Contains(d, "DROP TABLE IF EXISTS OFFICIAL_STATUS") {
		t.Error("down が official_status を消さない")
	}
	if !regexp.MustCompile(`ALTER TABLE ITEMS DROP (COLUMN )?WATCH_OFFICIAL\b`).MatchString(d) {
		t.Error("down が items から watch_official を外さない")
	}
	rest := strings.NewReplacer("OFFICIAL_STATUS", "", "WATCH_OFFICIAL", "", "ALTER TABLE ITEMS", "").Replace(d)
	for _, bad := range []string{"ITEMS", "SITES", "ESTIMATES", "LISTINGS", "GENRES", "PRICE_HISTORY", "TRUNCATE", "DELETE"} {
		if strings.Contains(rest, bad) {
			t.Errorf("down が official_status・watch_official 以外(%s)に触れている", bad)
		}
	}
}
