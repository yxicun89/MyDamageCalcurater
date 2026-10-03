//go:build mysql

// 000004(確認済みサイトの初期データ)を実 MySQL で確かめる。`make wishlist-test-mysql` か WISHLIST_TEST_DSN で流す
// (CREATE/DROP DATABASE ができるユーザー。DB 名は無視して使い捨ての DB を作る)。docs/phase3-api-spec.md の AC-D*。
package migrations_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"example.com/pokecalc/apps/wishlist/api/migrations"
)

var dbSeq atomic.Int64

// newMigrator は使い捨ての DB を作り、migrate と接続を返す(まだ 1 つも適用していない)。
func newMigrator(t *testing.T) (*migrate.Migrate, *sql.DB) {
	t.Helper()
	raw := os.Getenv("WISHLIST_TEST_DSN")
	if raw == "" {
		t.Fatal("WISHLIST_TEST_DSN が無い(make wishlist-test-mysql で流す。スキップはしない)")
	}
	base, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("wishlist_seed_%d_%d", time.Now().UnixNano()%1_000_000, dbSeq.Add(1))
	admin := base.Clone()
	admin.DBName = ""
	adminDB, err := sql.Open("mysql", admin.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminDB.Close() })
	if _, err := adminDB.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminDB.Exec("DROP DATABASE IF EXISTS " + name) })

	cfg := base.Clone()
	cfg.DBName = name
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	drv, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		t.Fatal(err)
	}
	return m, db
}

func mustMigrate(t *testing.T, m *migrate.Migrate, version uint) {
	t.Helper()
	if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("Migrate(%d): %v", version, err)
	}
}

type siteRow struct {
	Name, Template, FetchType string
	IsReference               bool
}

func sitesByName(t *testing.T, db *sql.DB) map[string][]siteRow {
	t.Helper()
	rows, err := db.Query("SELECT name, search_url_template, fetch_type, is_reference FROM sites ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][]siteRow{}
	for rows.Next() {
		var r siteRow
		if err := rows.Scan(&r.Name, &r.Template, &r.FetchType, &r.IsReference); err != nil {
			t.Fatal(err)
		}
		out[r.Name] = append(out[r.Name], r)
	}
	return out
}

type link struct {
	Genre, Site string
	SortOrder   int
}

// links は genre_sites を (ジャンル名, サイト名, sort_order) で返す。ジャンル → sort_order の順。
func links(t *testing.T, db *sql.DB) []link {
	t.Helper()
	rows, err := db.Query(`SELECT g.name, s.name, gs.sort_order FROM genre_sites gs
		JOIN genres g ON g.id = gs.genre_id JOIN sites s ON s.id = gs.site_id ORDER BY g.id, gs.sort_order, s.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []link
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.Genre, &l.Site, &l.SortOrder); err != nil {
			t.Fatal(err)
		}
		out = append(out, l)
	}
	return out
}

func siteNames(ls []link, genre string) []string {
	var out []string
	for _, l := range ls {
		if l.Genre == genre {
			out = append(out, l.Site)
		}
	}
	return out
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var wantSites = map[string]siteRow{
	"Yahoo!フリマ":    {"Yahoo!フリマ", "https://paypayfleamarket.yahoo.co.jp/search/{q}", "scrape", false},
	"カードラッシュ":      {"カードラッシュ", "https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20", "scrape", true},
	"あみあみ":         {"あみあみ", "https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea", "scrape", true},
	"Yahoo!ショッピング": {"Yahoo!ショッピング", "https://shopping.yahoo.co.jp/search/{q}/0/?X=2", "api", true},
	"駿河屋":          {"駿河屋", "https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On", "link_only", false},
}

var wantOrder = map[string][]string{
	"デュエル・マスターズ":   {"カードラッシュ", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon"},
	"S.H.Figuarts": {"あみあみ", "駿河屋", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon"},
	"ガンプラ":         {"あみあみ", "駿河屋", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon"},
	"ポケモングッズ":      {"駿河屋", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon"},
}

var genreNames = []string{"デュエル・マスターズ", "S.H.Figuarts", "ガンプラ", "ポケモングッズ"}

// AC-D1: 空の DB に 000004 まで適用すると、確認済みの 5 サイトが仕様の fetch_type・is_reference・URL で入り(合計 7 サイト)、
// 各ジャンルの表示順が仕様の既定案になる(sort_order は各ジャンル内で重ならない)。000002 の行(メルカリ 10・Amazon 20)は変わらない。
func TestSeed000004_Fresh(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 4)

	sites := sitesByName(t, db)
	if len(sites) != 7 {
		t.Errorf("サイト数 = %d, want 7(%v)", len(sites), sites)
	}
	for name, want := range wantSites {
		got := sites[name]
		if len(got) != 1 || got[0] != want {
			t.Errorf("サイト %s = %+v, want %+v", name, got, want)
		}
	}
	ls := links(t, db)
	for _, g := range genreNames {
		if got := siteNames(ls, g); !slices.Equal(got, wantOrder[g]) {
			t.Errorf("%s の表示順 = %v, want %v", g, got, wantOrder[g])
		}
		prev := -1 << 31
		for _, l := range ls {
			if l.Genre != g {
				continue
			}
			if l.SortOrder <= prev {
				t.Errorf("%s の sort_order が昇順で重ならない並びでない: %+v", g, ls)
				break
			}
			prev = l.SortOrder
			if (l.Site == "メルカリ" && l.SortOrder != 10) || (l.Site == "Amazon" && l.SortOrder != 20) {
				t.Errorf("000002 の行が変わった: %+v", l)
			}
		}
	}
	if n := count(t, db, "genre_sites"); n != 5+6+6+5 {
		t.Errorf("genre_sites = %d 行, want 22", n)
	}
}

// AC-D2: 2 回適用しない。Up をもう一度呼んでも変更なし(ErrNoChange)で、行は増えない。
// SQL 自体も、もう一度流して重ならない(同じ名前のサイト・同じ紐づけを足さない)。
func TestSeed000004_NotAppliedTwice(t *testing.T) {
	m, db := newMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	sites, gs := count(t, db, "sites"), count(t, db, "genre_sites")
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Errorf("2 回目の Up = %v, want migrate.ErrNoChange", err)
	}
	b, err := migrations.FS.ReadFile("000004_seed_sites.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("up の SQL をもう一度流すと失敗した: %v", err)
	}
	if got := count(t, db, "sites"); got != sites {
		t.Errorf("sites = %d 行, want %d(同じ名前を足している)", got, sites)
	}
	if got := count(t, db, "genre_sites"); got != gs {
		t.Errorf("genre_sites = %d 行, want %d", got, gs)
	}
}

// userSetup は 000003 まで適用した「使われている DB」を作る:
// ユーザーが同じ名前「カードラッシュ」を別の URL・is_reference=false で登録済みで、デュエマの先頭(sort_order 5)に紐づけている。
func userSetup(t *testing.T) (*migrate.Migrate, *sql.DB) {
	t.Helper()
	m, db := newMigrator(t)
	mustMigrate(t, m, 3)
	if _, err := db.Exec(`INSERT INTO sites (name, search_url_template, fetch_type, is_reference)
		VALUES ('カードラッシュ', 'https://user.example/cr?q={q}', 'scrape', FALSE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO genre_sites (genre_id, site_id, sort_order)
		SELECT 1, id, 5 FROM sites WHERE name = 'カードラッシュ'`); err != nil {
		t.Fatal(err)
	}
	return m, db
}

var userCardrush = siteRow{"カードラッシュ", "https://user.example/cr?q={q}", "scrape", false}

// AC-D3: 使われている DB に流しても壊れない。同じ名前のサイトは足さず(既存の URL・方式を変えない)、
// 既存の genre_sites の行(sort_order を含む)は変えない。足りない紐づけとサイトだけが増える。
func TestSeed000004_ExistingDB(t *testing.T) {
	m, db := userSetup(t)
	beforeSites := sitesByName(t, db)
	beforeLinks := links(t, db)
	mustMigrate(t, m, 4)

	sites := sitesByName(t, db)
	if got := sites["カードラッシュ"]; len(got) != 1 || got[0] != userCardrush {
		t.Errorf("既存のカードラッシュ = %+v, want %+v(変えない・重ねない)", got, userCardrush)
	}
	for _, name := range []string{"メルカリ", "Amazon"} {
		if !slices.Equal(sites[name], beforeSites[name]) {
			t.Errorf("既存のサイト %s が変わった: %+v", name, sites[name])
		}
	}
	for _, name := range []string{"Yahoo!フリマ", "あみあみ", "Yahoo!ショッピング", "駿河屋"} {
		if got := sites[name]; len(got) != 1 || got[0] != wantSites[name] {
			t.Errorf("サイト %s = %+v, want %+v", name, got, wantSites[name])
		}
	}
	after := links(t, db)
	for _, b := range beforeLinks {
		if !slices.Contains(after, b) {
			t.Errorf("既存の紐づけが変わった・消えた: %+v", b)
		}
	}
	dm := siteNames(after, "デュエル・マスターズ")
	if slices.Index(dm, "カードラッシュ") != 0 || countOf(dm, "カードラッシュ") != 1 {
		t.Errorf("デュエマの表示順 = %v(既存の sort_order 5 のカードラッシュが先頭に 1 つだけ)", dm)
	}
	for _, name := range []string{"Yahoo!フリマ", "Yahoo!ショッピング"} {
		if !slices.Contains(dm, name) {
			t.Errorf("デュエマに %s が紐づいていない: %v", name, dm)
		}
	}
	if got := siteNames(after, "S.H.Figuarts"); !slices.Equal(got, wantOrder["S.H.Figuarts"]) {
		t.Errorf("S.H.Figuarts の表示順 = %v, want %v", got, wantOrder["S.H.Figuarts"])
	}
}

// AC-D4: down は 000004 で足した行だけを消す。空の DB なら 000002 の状態(メルカリ・Amazon の 2 サイトと
// 8 行の紐づけ、sort_order 10・20)に戻る。
func TestSeed000004_DownFresh(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 4)
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	sites := sitesByName(t, db)
	if len(sites) != 2 || len(sites["メルカリ"]) != 1 || len(sites["Amazon"]) != 1 {
		t.Errorf("サイト = %v, want メルカリ・Amazon だけ", sites)
	}
	ls := links(t, db)
	if len(ls) != 8 {
		t.Fatalf("genre_sites = %+v, want 8 行", ls)
	}
	for _, l := range ls {
		if (l.Site == "メルカリ" && l.SortOrder != 10) || (l.Site == "Amazon" && l.SortOrder != 20) {
			t.Errorf("紐づけが戻っていない: %+v", l)
		}
	}
	if n := count(t, db, "genres"); n != 4 {
		t.Errorf("genres = %d, want 4(ジャンルは消さない)", n)
	}
}

// AC-D4: ユーザーが先に登録していた同名のサイト(URL が違う)と、その既存の紐づけは down で消さない。
// 000004 が足した他の 4 サイトとその紐づけは消える。
func TestSeed000004_DownKeepsUserRows(t *testing.T) {
	m, db := userSetup(t)
	beforeLinks := links(t, db)
	mustMigrate(t, m, 4)
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	sites := sitesByName(t, db)
	if got := sites["カードラッシュ"]; len(got) != 1 || got[0] != userCardrush {
		t.Errorf("ユーザーのカードラッシュ = %+v, want %+v", got, userCardrush)
	}
	for _, name := range []string{"Yahoo!フリマ", "あみあみ", "Yahoo!ショッピング", "駿河屋"} {
		if len(sites[name]) != 0 {
			t.Errorf("down のあとにサイト %s が残っている", name)
		}
	}
	if after := links(t, db); !slices.Equal(after, beforeLinks) {
		t.Errorf("紐づけ = %+v, want 適用前の %+v", after, beforeLinks)
	}
}

func countOf(xs []string, x string) int {
	n := 0
	for _, v := range xs {
		if v == x {
			n++
		}
	}
	return n
}
