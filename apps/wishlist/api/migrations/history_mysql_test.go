//go:build mysql

// 000007(価格の推移 price_history)を実 MySQL で確かめる。`make wishlist-test-mysql` で流す。docs/phase4-spec.md AC-H14・H15。
// 補助(newMigrator・mustMigrate・count)は seed_mysql_test.go のもの。
// 000006 は別の PR(#601)で入る。golang-migrate は版の欠けを飛ばして進むので、000006 が無くても 7 まで上げられる。
package migrations_test

import (
	"database/sql"
	"testing"
)

// historySetup は 000007 まで上げ、seed のジャンル・サイトに商品を 1 つ作って (db, item_id, site_id, 別の site_id) を返す。
func historySetup(t *testing.T) (*sql.DB, int64, int64, int64) {
	t.Helper()
	m, db := newMigrator(t)
	mustMigrate(t, m, 7)
	var genreID int64
	if err := db.QueryRow(`SELECT id FROM genres ORDER BY id LIMIT 1`).Scan(&genreID); err != nil {
		t.Fatal(err)
	}
	var s1, s2 int64
	rows, err := db.Query(`SELECT id FROM sites ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) != 2 {
		t.Fatalf("seed のサイトが 2 つ無い: %v", ids)
	}
	s1, s2 = ids[0], ids[1]
	res, err := db.Exec(`INSERT INTO items (genre_id, name, image_path) VALUES (?, 'グリス', '11111111-1111-4111-8111-111111111111.png')`, genreID)
	if err != nil {
		t.Fatal(err)
	}
	itemID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return db, itemID, s1, s2
}

func insertHistory(db *sql.DB, itemID, siteID int64, day string, low int, mid any) error {
	_, err := db.Exec(`INSERT INTO price_history (item_id, site_id, day, low, mid, count, recorded_at) VALUES (?, ?, ?, ?, ?, 3, '2026-10-03 03:00:00')`,
		itemID, siteID, day, low, mid)
	return err
}

func historyCount(t *testing.T, db *sql.DB, where string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM price_history WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// AC-H15: 主キー (item_id, site_id, day) で 1 日 1 行。mid は null を許し low は許さない。
// 存在しない商品・サイトは入らない。サイトを消すとそのサイトの推移が、商品を消すとその商品の推移が消える(CASCADE)。
func TestPriceHistory000007_Constraints(t *testing.T) {
	db, itemID, s1, s2 := historySetup(t)
	if err := insertHistory(db, itemID, s1, "2026-10-03", 3000, 4500); err != nil {
		t.Fatal(err)
	}
	if err := insertHistory(db, itemID, s1, "2026-10-03", 2900, nil); err == nil {
		t.Error("同じ商品・サイト・日に 2 行目が入った")
	}
	if err := insertHistory(db, itemID, s1, "2026-10-04", 2900, nil); err != nil {
		t.Errorf("mid が null の行が入らない: %v", err)
	}
	if err := insertHistory(db, itemID, s2, "2026-10-03", 3100, nil); err != nil {
		t.Errorf("別のサイトの同じ日が入らない: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO price_history (item_id, site_id, day, low, mid, count, recorded_at) VALUES (?, ?, '2026-10-05', NULL, NULL, 0, '2026-10-05 03:00:00')`, itemID, s1); err == nil {
		t.Error("low が null の行が入った")
	}
	if err := insertHistory(db, 99_999_999, s1, "2026-10-03", 1, nil); err == nil {
		t.Error("存在しない商品の推移が入った")
	}
	if err := insertHistory(db, itemID, 99_999_999, "2026-10-03", 1, nil); err == nil {
		t.Error("存在しないサイトの推移が入った")
	}

	if _, err := db.Exec(`DELETE FROM sites WHERE id = ?`, s2); err != nil {
		t.Fatal(err)
	}
	if n := historyCount(t, db, "site_id = ?", s2); n != 0 {
		t.Errorf("サイトを消しても推移が %d 行残っている", n)
	}
	if n := historyCount(t, db, "item_id = ?", itemID); n != 2 {
		t.Errorf("別のサイトの推移 = %d 行, want 2(消したサイトの分だけ消える)", n)
	}
	if _, err := db.Exec(`DELETE FROM items WHERE id = ?`, itemID); err != nil {
		t.Fatal(err)
	}
	if n := historyCount(t, db, "item_id = ?", itemID); n != 0 {
		t.Errorf("商品を消しても推移が %d 行残っている", n)
	}
}

// AC-H14: down は price_history だけを消す(商品・サイト・ジャンルは残る)。
func TestPriceHistory000007_Down(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 7)
	genres, sites := count(t, db, "genres"), count(t, db, "sites")
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'price_history'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("down のあとに price_history が残っている")
	}
	if g, s := count(t, db, "genres"), count(t, db, "sites"); g != genres || s != sites {
		t.Errorf("genres = %d・sites = %d, want %d・%d(変えない)", g, s, genres, sites)
	}
}
