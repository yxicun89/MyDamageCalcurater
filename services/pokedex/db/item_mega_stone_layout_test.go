package db

// items.is_mega_stone(取得元の持ち物データから導いたメガストーンの判定。ADR-0140・issue #607)の静的な確認。DB は使わない。
//
// 既存の migration は書き換えず、新しい版で列を足す(ADR-0124)。既存の行がある DB にも migrate できるよう
// NOT NULL DEFAULT 0(偽)にする。0 のままでも、メガ種族の required_item_id に現れる持ち物は SearchItems の
// EXISTS で従来どおり true になる(判定は「列が真 OR メガ種族が要求する」。ADR-0175 §2 の追記)。

import (
	"regexp"
	"testing"
)

// itemMegaStoneMigrationVersion は items.is_mega_stone を足す migration の版(000011 の次)。
const itemMegaStoneMigrationVersion = 12

func TestMigrationAddsItemIsMegaStoneColumn(t *testing.T) {
	_, up, down := migrationPairs(t)
	if up[itemMegaStoneMigrationVersion] == "" || down[itemMegaStoneMigrationVersion] == "" {
		t.Fatalf("migration %06d(items.is_mega_stone を足す。ADR-0140)が無い", itemMegaStoneMigrationVersion)
	}
	upSQL := readRaw(t, up[itemMegaStoneMigrationVersion])
	downSQL := readRaw(t, down[itemMegaStoneMigrationVersion])

	column := regexp.MustCompile(`(?is)alter\s+table\s+` + "`?items`?" + `\s+add\s+column\s+` + "`?is_mega_stone`?" +
		`\s+(tinyint\(1\)|tinyint|boolean|bool)\s+not\s+null\s+default\s+(0|false)\b`)
	if !column.MatchString(upSQL) {
		t.Errorf("up に ALTER TABLE items ADD COLUMN is_mega_stone TINYINT(1) NOT NULL DEFAULT 0 が無い:\n%s", upSQL)
	}
	if !regexp.MustCompile(`(?is)alter\s+table\s+` + "`?items`?" + `\s+drop\s+column\s+` + "`?is_mega_stone`?").MatchString(downSQL) {
		t.Errorf("down に ALTER TABLE items DROP COLUMN is_mega_stone が無い:\n%s", downSQL)
	}
	// migration にデータを書かない(行は importer が書く。ADR-0100 §2)。
	if regexp.MustCompile(`(?is)\b(insert|update)\b`).MatchString(upSQL) {
		t.Errorf("up に INSERT/UPDATE がある(行は importer が書く):\n%s", upSQL)
	}
}

// SearchItems の is_mega_stone は「列が真 OR メガ種族が要求する」の1つの式(pokedex.sql が唯一の判定の場所)。
func TestSearchItemsQueryReadsItemIsMegaStoneColumn(t *testing.T) {
	sql := readRaw(t, "query/pokedex.sql")
	m := regexp.MustCompile(`(?s)-- name: SearchItems :many\n(.*?);`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatal("query/pokedex.sql に SearchItems が無い")
	}
	q := m[1]
	if !regexp.MustCompile(`(?is)i\.is_mega_stone\b[^,]*\bor\s+exists\s*\(\s*select\s+1\s+from\s+species`).MatchString(q) {
		t.Errorf("SearchItems の is_mega_stone が「i.is_mega_stone OR EXISTS (species …)」になっていない:\n%s", q)
	}
}
