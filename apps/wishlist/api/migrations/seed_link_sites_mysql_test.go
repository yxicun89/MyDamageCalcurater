//go:build mysql

// 000006(リンク用サイト 3 つ)を実 MySQL で確かめる(`make wishlist-test-mysql`。ヘルパーは seed_mysql_test.go)。docs/phase3-api-spec.md の AC-D5。
// 000005 は別の作業が使う予定。000005 が無くても動くよう「000004 まで → 000006 まで」で書く(Migrate(6) は存在する版だけを順に流す)。
// 000005 が sites・genre_sites の行数を変える場合は、そのときこのテストの行数(10 サイト・26 行)を見直す。
package migrations_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/golang-migrate/migrate/v4"

	"example.com/pokecalc/apps/wishlist/api/migrations"
)

var wantLinkSites = map[string]siteRow{
	"魂ウェブ": {"魂ウェブ", "https://tamashiiweb.com/item/?wo={q}", "link_only", false},
	"ポケモンセンターオンライン": {"ポケモンセンターオンライン", "https://www.pokemoncenter-online.com/search/?q={q}", "link_only", false},
	"プレバン": {"プレバン", "https://p-bandai.jp/search_bst/?q={q}", "link_only", false},
}

// 紐づけ(既定案): 各ジャンルの末尾(Amazon の後)に置く。プレバン 25・魂ウェブ 30(S.H.Figuarts)、プレバン 25(ガンプラ)、ポケセン 25(ポケモングッズ)。
// デュエマには足さない。既存の sort_order(あみあみ 2〜Amazon 20)は変えない。
var wantLinkOrder = map[string][]string{
	"デュエル・マスターズ":   {"カードラッシュ", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon"},
	"S.H.Figuarts": {"あみあみ", "駿河屋", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon", "プレバン", "魂ウェブ"},
	"ガンプラ":         {"あみあみ", "駿河屋", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon", "プレバン"},
	"ポケモングッズ":      {"駿河屋", "Yahoo!フリマ", "メルカリ", "Yahoo!ショッピング", "Amazon", "ポケモンセンターオンライン"},
}

// AC-D5: 空の DB に 000006 まで適用すると、3 サイトが link_only・is_reference=false で入り(合計 10 サイト)、紐づけは上の既定案になる。
// 000004 までの行(sort_order 含む)は変わらない。
func TestSeed000006_Fresh(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 4)
	before := links(t, db)
	mustMigrate(t, m, 6)

	sites := sitesByName(t, db)
	if len(sites) != 10 {
		t.Errorf("サイト数 = %d, want 10(%v)", len(sites), sites)
	}
	for name, want := range wantLinkSites {
		if got := sites[name]; len(got) != 1 || got[0] != want {
			t.Errorf("サイト %s = %+v, want %+v", name, got, want)
		}
	}
	after := links(t, db)
	for _, g := range genreNames {
		if got := siteNames(after, g); !slices.Equal(got, wantLinkOrder[g]) {
			t.Errorf("%s の表示順 = %v, want %v", g, got, wantLinkOrder[g])
		}
	}
	for _, b := range before {
		if !slices.Contains(after, b) {
			t.Errorf("000004 までの紐づけが変わった・消えた: %+v", b)
		}
	}
	if n := count(t, db, "genre_sites"); n != 22+4 {
		t.Errorf("genre_sites = %d 行, want 26", n)
	}
}

// AC-D5: SQL をもう一度流しても行が増えない。Up は ErrNoChange。
func TestSeed000006_NotAppliedTwice(t *testing.T) {
	m, db := newMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	sites, gs := count(t, db, "sites"), count(t, db, "genre_sites")
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Errorf("2 回目の Up = %v, want migrate.ErrNoChange", err)
	}
	b, err := migrations.FS.ReadFile("000006_seed_link_sites.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("up の SQL をもう一度流すと失敗した: %v", err)
	}
	if got := count(t, db, "sites"); got != sites {
		t.Errorf("sites = %d 行, want %d", got, sites)
	}
	if got := count(t, db, "genre_sites"); got != gs {
		t.Errorf("genre_sites = %d 行, want %d", got, gs)
	}
}

// AC-D5: 使われている DB(ユーザーが同じ名前「魂ウェブ」を別の URL で登録済み・S.H.Figuarts に紐づけ済み)。
// 同名のサイトは足さず(URL を変えない)、既存の紐づけ(sort_order)は変えず、紐づけの重複も作らない。足りない分だけ増える。
func TestSeed000006_ExistingDB(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 4)
	if _, err := db.Exec(`INSERT INTO sites (name, search_url_template, fetch_type, is_reference)
		VALUES ('魂ウェブ', 'https://user.example/tw?q={q}', 'link_only', FALSE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO genre_sites (genre_id, site_id, sort_order)
		SELECT g.id, s.id, 3 FROM genres g, sites s WHERE g.name = 'S.H.Figuarts' AND s.name = '魂ウェブ'`); err != nil {
		t.Fatal(err)
	}
	beforeLinks := links(t, db)
	mustMigrate(t, m, 6)

	sites := sitesByName(t, db)
	user := siteRow{"魂ウェブ", "https://user.example/tw?q={q}", "link_only", false}
	if got := sites["魂ウェブ"]; len(got) != 1 || got[0] != user {
		t.Errorf("既存の魂ウェブ = %+v, want %+v(変えない・重ねない)", got, user)
	}
	for _, name := range []string{"ポケモンセンターオンライン", "プレバン"} {
		if got := sites[name]; len(got) != 1 || got[0] != wantLinkSites[name] {
			t.Errorf("サイト %s = %+v", name, got)
		}
	}
	after := links(t, db)
	for _, b := range beforeLinks {
		if !slices.Contains(after, b) {
			t.Errorf("既存の紐づけが変わった・消えた: %+v", b)
		}
	}
	if shf := siteNames(after, "S.H.Figuarts"); countOf(shf, "魂ウェブ") != 1 || slices.Index(shf, "魂ウェブ") != slices.Index(shf, "あみあみ")+1 {
		t.Errorf("S.H.Figuarts の表示順 = %v(既存の sort_order 3 の魂ウェブが 1 つだけで、あみあみ〈2〉の直後のまま)", shf)
	}
}

// AC-D5: down は 000006 で足した行(名前と URL が一致するもの)とその紐づけだけを消す。空の DB なら 000004 の状態に戻る。
// ユーザーが登録した同名(別 URL)のサイトとその紐づけは消さない。
func TestSeed000006_Down(t *testing.T) {
	t.Run("空の DB は 000004 の状態に戻る", func(t *testing.T) {
		m, db := newMigrator(t)
		mustMigrate(t, m, 4)
		before, beforeSites := links(t, db), sitesByName(t, db)
		mustMigrate(t, m, 6)
		mustMigrate(t, m, 4) // 000006(と、あれば 000005)を戻す
		if got := links(t, db); !slices.Equal(got, before) {
			t.Errorf("紐づけ = %+v, want %+v", got, before)
		}
		if got := sitesByName(t, db); len(got) != len(beforeSites) {
			t.Errorf("サイト = %v, want 000004 の %d サイト", got, len(beforeSites))
		}
		if n := count(t, db, "genres"); n != 4 {
			t.Errorf("genres = %d, want 4(ジャンルは消さない)", n)
		}
	})
	t.Run("ユーザーの同名サイトは消さない", func(t *testing.T) {
		m, db := newMigrator(t)
		mustMigrate(t, m, 4)
		if _, err := db.Exec(`INSERT INTO sites (name, search_url_template, fetch_type, is_reference)
			VALUES ('魂ウェブ', 'https://user.example/tw?q={q}', 'link_only', FALSE)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO genre_sites (genre_id, site_id, sort_order)
			SELECT g.id, s.id, 3 FROM genres g, sites s WHERE g.name = 'S.H.Figuarts' AND s.name = '魂ウェブ'`); err != nil {
			t.Fatal(err)
		}
		before := links(t, db)
		mustMigrate(t, m, 6)
		mustMigrate(t, m, 4) // 000006(と、あれば 000005)を戻す
		user := siteRow{"魂ウェブ", "https://user.example/tw?q={q}", "link_only", false}
		sites := sitesByName(t, db)
		if got := sites["魂ウェブ"]; len(got) != 1 || got[0] != user {
			t.Errorf("ユーザーの魂ウェブ = %+v", got)
		}
		for _, name := range []string{"ポケモンセンターオンライン", "プレバン"} {
			if len(sites[name]) != 0 {
				t.Errorf("down のあとにサイト %s が残っている", name)
			}
		}
		if got := links(t, db); !slices.Equal(got, before) {
			t.Errorf("紐づけ = %+v, want 適用前の %+v", got, before)
		}
	})
}
