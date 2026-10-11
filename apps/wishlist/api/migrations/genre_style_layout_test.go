package migrations

import (
	"regexp"
	"strings"
	"testing"
)

// 000009(フェーズ5 5-1b・5-1c・5-1e。ジャンルの色・絵柄・URL の手がかり、METAL BUILD・ROBOT魂、Yahoo!ショッピングを scrape に)の
// SQL の形(MySQL なしで確かめる。内容は genre_style_mysql_test.go が実 DB で確かめる。docs/phase5-spec.md)。
//   - genres に color・icon・match_hosts・match_keywords を足す。icon は 6 つの列挙
//   - 既存の 4 ジャンルは名前で引いて UPDATE する(id を書かない)。色は仕様の値
//   - METAL BUILD・ROBOT魂 は id を書かず、同じ名前があれば足さない(NOT EXISTS)
//   - Yahoo!ショッピングは名前と URL の両方で特定して scrape にする
//   - up はテーブル・行を消さない。down は足した列を外し、Yahoo!ショッピングを api に戻し、足したジャンルだけを消す
func TestGenreStyle000009Shape(t *testing.T) {
	up, err := FS.ReadFile("000009_genre_style.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("000009_genre_style.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	noComments := func(s string) string { return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "") }
	u, d := noComments(string(up)), noComments(string(down))
	U, D := strings.ToUpper(u), strings.ToUpper(d)

	for _, col := range []string{"COLOR", "ICON", "MATCH_HOSTS", "MATCH_KEYWORDS"} {
		if !regexp.MustCompile(`ADD\s+COLUMN\s+` + col + `\b`).MatchString(U) {
			t.Errorf("up が genres.%s を足していない", strings.ToLower(col))
		}
		if !regexp.MustCompile(`DROP\s+COLUMN\s+` + col + `\b`).MatchString(D) {
			t.Errorf("down が genres.%s を外していない", strings.ToLower(col))
		}
	}
	if !strings.Contains(u, "'card', 'figure', 'robot', 'goods', 'star', 'box'") {
		t.Error("icon の列挙が card・figure・robot・goods・star・box でない")
	}
	if !strings.Contains(u, "'#1d5fd1'") {
		t.Error("color の既定値 #1d5fd1 が無い")
	}
	for _, bad := range []string{"DELETE ", "TRUNCATE", "DROP TABLE", "DROP COLUMN", "REPLACE INTO", "ON DUPLICATE KEY"} {
		if strings.Contains(U, bad) {
			t.Errorf("up に %q がある(既存の行を消さない)", strings.TrimSpace(bad))
		}
	}
	if regexp.MustCompile(`INSERT\s+INTO\s+GENRES\s*\(\s*ID\b`).MatchString(U) {
		t.Error("genres の INSERT が id を明示している(名前で引く)")
	}
	if !strings.Contains(U, "NOT EXISTS") {
		t.Error("up に、同じ名前のジャンルを足さない仕組み(NOT EXISTS)が無い")
	}
	if regexp.MustCompile(`WHERE\s+ID\s*=`).MatchString(U) {
		t.Error("up が id で行を引いている(名前で引く)")
	}
	styles := map[string][2]string{
		"デュエル・マスターズ":         {"#e0393e", "card"},
		"S.H.Figuarts":       {"#7c4dff", "figure"},
		"ガンプラ":               {"#1f6fe5", "robot"},
		"ポケモングッズ":            {"#f2b705", "goods"},
		"METAL BUILD・ROBOT魂": {"#5b6b7f", "robot"},
	}
	for name, st := range styles {
		for _, want := range []string{"'" + name + "'", "'" + st[0] + "'", "'" + st[1] + "'"} {
			if !strings.Contains(u, want) {
				t.Errorf("up に %s の %s が無い", name, want)
			}
		}
	}
	for _, kw := range []string{"'dm.takaratomy.co.jp'", "'bandai-hobby.net'", "'www.pokemoncenter-online.com'", "'デュエマ'", "'フィギュアーツ'", "'30MF'", "'30MM'", "'METAL ROBOT魂'"} {
		if !strings.Contains(u, kw) {
			t.Errorf("up に手がかり %s が無い", kw)
		}
	}
	yahoo := regexp.MustCompile(`(?s)UPDATE\s+sites\s+SET\s+fetch_type\s*=\s*'scrape'.*?name\s*=\s*'Yahoo!ショッピング'.*?search_url_template\s*=\s*'https://shopping\.yahoo\.co\.jp/search/\{q\}/0/\?X=2'`)
	if !yahoo.MatchString(u) {
		t.Error("up が Yahoo!ショッピングを名前と URL で特定して scrape にしていない")
	}
	yahooDown := regexp.MustCompile(`(?s)UPDATE\s+sites\s+SET\s+fetch_type\s*=\s*'api'.*?name\s*=\s*'Yahoo!ショッピング'.*?search_url_template\s*=\s*'https://shopping\.yahoo\.co\.jp/search/\{q\}/0/\?X=2'`)
	if !yahooDown.MatchString(d) {
		t.Error("down が Yahoo!ショッピングを api に戻していない")
	}
	for _, bad := range []string{"TRUNCATE", "DROP TABLE", "DELETE FROM SITES", "DELETE FROM GENRE_SITES"} {
		if strings.Contains(D, bad) {
			t.Errorf("down に %q がある(000009 で足したものだけを戻す)", bad)
		}
	}
	if !strings.Contains(d, "'METAL BUILD・ROBOT魂'") {
		t.Error("down が METAL BUILD・ROBOT魂 を消さない")
	}
}
