//go:build mysql

// 000008(公式サイトの販売状況 official_status・items.watch_official)を実 MySQL で確かめる。`make wishlist-test-mysql` で流す。
// docs/phase4-spec.md AC-O29・O30。補助(newMigrator・mustMigrate・count)は seed_mysql_test.go のもの。
package migrations_test

import (
	"database/sql"
	"testing"
)

// officialSetup は 000008 まで上げ、seed のジャンルに商品を 1 つ作って (db, item_id) を返す。
func officialSetup(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	m, db := newMigrator(t)
	mustMigrate(t, m, 8)
	var genreID int64
	if err := db.QueryRow(`SELECT id FROM genres ORDER BY id LIMIT 1`).Scan(&genreID); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO items (genre_id, name, image_path) VALUES (?, 'グリス', '11111111-1111-4111-8111-111111111111.png')`, genreID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return db, id
}

func insertOfficial(db *sql.DB, itemID int64, status string, prev any) error {
	_, err := db.Exec(`INSERT INTO official_status (item_id, status, evidence, checked_at, changed_at, previous_status, last_result, last_attempt_at)
		VALUES (?, ?, '["販売中"]', '2026-10-03 03:00:00', NULL, ?, ?, '2026-10-03 03:00:00')`, itemID, status, prev, status)
	return err
}

// AC-O30: items.watch_official の既定は false。official_status は 1 商品 1 行、status は 8 つ以外を入れられない、
// previous_status は null を許す。存在しない商品は入らない。商品を消すと状態も消える。
func TestOfficialStatus000008_Constraints(t *testing.T) {
	db, itemID := officialSetup(t)
	var watch bool
	if err := db.QueryRow(`SELECT watch_official FROM items WHERE id = ?`, itemID).Scan(&watch); err != nil {
		t.Fatal(err)
	}
	if watch {
		t.Error("watch_official の既定が true")
	}
	if err := insertOfficial(db, itemID, "available", nil); err != nil {
		t.Fatalf("previous_status が null の行が入らない: %v", err)
	}
	if err := insertOfficial(db, itemID, "soldout", "available"); err == nil {
		t.Error("同じ商品に 2 行目が入った")
	}
	if _, err := db.Exec(`UPDATE official_status SET status = 'sold_out' WHERE item_id = ?`, itemID); err == nil {
		t.Error("8 つ以外の status が入った")
	}
	if _, err := db.Exec(`UPDATE official_status SET status = 'ambiguous', previous_status = 'blocked', last_result = 'failed', changed_at = '2026-10-04 03:00:00' WHERE item_id = ?`, itemID); err != nil {
		t.Errorf("8 つの値・changed_at が入らない: %v", err)
	}
	if err := insertOfficial(db, 99_999_999, "available", nil); err == nil {
		t.Error("存在しない商品の状態が入った")
	}
	if _, err := db.Exec(`DELETE FROM items WHERE id = ?`, itemID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "official_status"); n != 0 {
		t.Errorf("商品を消しても状態が %d 行残っている", n)
	}
}

// AC-O29: down は official_status を消し、items.watch_official を外す(商品・ジャンル・サイトの行は残る)。
func TestOfficialStatus000008_Down(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 8)
	var genreID int64
	if err := db.QueryRow(`SELECT id FROM genres ORDER BY id LIMIT 1`).Scan(&genreID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO items (genre_id, name, image_path, watch_official) VALUES (?, 'グリス', '11111111-1111-4111-8111-111111111111.png', TRUE)`, genreID); err != nil {
		t.Fatal(err)
	}
	items, genres, sites := count(t, db, "items"), count(t, db, "genres"), count(t, db, "sites")
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'official_status'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("down のあとに official_status が残っている")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'items' AND column_name = 'watch_official'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("down のあとに items.watch_official が残っている")
	}
	if i, g, s := count(t, db, "items"), count(t, db, "genres"), count(t, db, "sites"); i != items || g != genres || s != sites {
		t.Errorf("items・genres・sites = %d・%d・%d, want %d・%d・%d(変えない)", i, g, s, items, genres, sites)
	}
}
