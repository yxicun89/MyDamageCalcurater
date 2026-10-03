package migrations

import (
	"regexp"
	"strings"
	"testing"
)

// フェーズ4-1 表記揺れの辞書の migration 000005(docs/phase4-spec.md AC-A12)。MySQL なしで SQL の形を確かめる
// (内容は aliases_mysql_test.go が実 DB で確かめる)。
//   - up は genre_aliases を CREATE TABLE IF NOT EXISTS で作り(もう一度流しても壊れない)、genre_id は genres を ON DELETE CASCADE で参照する
//   - 正規化後の語の一意制約があり、その列は utf8mb4_bin(DB 既定の ai_ci で濁点・かなの違いを同一視しない)
//   - seed はジャンル名で引き(id を書かない)、既存の行を変えない・ジャンルとサイトを足さない
//   - down は genre_aliases だけを消す
func TestGenreAliases000005Shape(t *testing.T) {
	up, err := FS.ReadFile("000005_genre_aliases.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("000005_genre_aliases.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	noComments := func(s string) string { return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "") }
	u, d := strings.ToUpper(noComments(string(up))), strings.ToUpper(noComments(string(down)))
	space := regexp.MustCompile(`\s+`)
	u, d = space.ReplaceAllString(u, " "), space.ReplaceAllString(d, " ")

	if !strings.Contains(u, "CREATE TABLE IF NOT EXISTS GENRE_ALIASES") {
		t.Error("up が genre_aliases を CREATE TABLE IF NOT EXISTS で作らない")
	}
	if !regexp.MustCompile(`FOREIGN KEY ?\( ?GENRE_ID ?\) REFERENCES GENRES ?\( ?ID ?\) ON DELETE CASCADE`).MatchString(u) {
		t.Error("genre_id が genres を ON DELETE CASCADE で参照しない")
	}
	if !regexp.MustCompile(`UNIQUE (KEY )?\w* ?\( ?GENRE_ID ?, ?NORMALIZED ?\)`).MatchString(u) {
		t.Error("(genre_id, normalized) の一意制約が無い")
	}
	if !regexp.MustCompile(`NORMALIZED VARCHAR\(\d+\)( CHARACTER SET UTF8MB4)? COLLATE UTF8MB4_BIN`).MatchString(u) {
		t.Error("normalized 列が utf8mb4_bin でない(濁点・ひらがなとカタカナを同一視してしまう)")
	}
	// ON DELETE CASCADE の DELETE は文ではないので、文の形(UPDATE x SET・DELETE FROM 等)で見る。
	for _, bad := range []string{`\bUPDATE \w+ SET\b`, `\bREPLACE (INTO )?\w`, `ON DUPLICATE KEY`, `\bDELETE (\w+ )?FROM\b`, `\bTRUNCATE\b`, `\bDROP\b`, `INTO GENRES\b`, `INTO SITES\b`, `INTO GENRE_SITES\b`} {
		if regexp.MustCompile(bad).MatchString(u) {
			t.Errorf("up に %s がある(既存の行を変えない・ジャンルとサイトを足さない)", bad)
		}
	}
	if !strings.Contains(u, "NOT EXISTS (") && !strings.Contains(u, "INSERT IGNORE") {
		t.Error("seed に、同じ語を足さない仕組み(NOT EXISTS 等)が無い")
	}
	for _, name := range []string{"S.H.Figuarts", "ガンプラ", "SHフィギュアーツ", "HG", "ハイグレード", "MG", "マスターグレード", "RG", "リアルグレード"} {
		if !strings.Contains(string(up), "'"+name+"'") {
			t.Errorf("up の seed に %s が無い", name)
		}
	}
	if !strings.Contains(d, "DROP TABLE IF EXISTS GENRE_ALIASES") {
		t.Error("down が genre_aliases を消さない")
	}
	for _, bad := range []string{"GENRES ", "GENRES;", "SITES", "GENRE_SITES", "ITEMS", "TRUNCATE"} {
		if strings.Contains(strings.ReplaceAll(d, "GENRE_ALIASES", ""), bad) {
			t.Errorf("down が genre_aliases 以外(%s)に触れている", strings.TrimSpace(bad))
		}
	}
}
