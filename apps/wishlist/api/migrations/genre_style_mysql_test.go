//go:build mysql

// 000009(フェーズ5 5-1b・5-1c・5-1e)を実 MySQL で確かめる。`make wishlist-test-mysql` で流す。
// docs/phase5-spec.md。補助(newMigrator・mustMigrate・count・links・siteNames・sitesByName)は seed_mysql_test.go のもの。
package migrations_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/golang-migrate/migrate/v4"
)

type genreStyle struct {
	Color, Icon             string
	MatchHosts, MatchKwords []string
}

// genreStyles は genres の色・絵柄・手がかりを名前で返す。
func genreStyles(t *testing.T, db *sql.DB) map[string]genreStyle {
	t.Helper()
	rows, err := db.Query(`SELECT name, color, icon, match_hosts, match_keywords FROM genres ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]genreStyle{}
	for rows.Next() {
		var name, hosts, kws string
		var g genreStyle
		if err := rows.Scan(&name, &g.Color, &g.Icon, &hosts, &kws); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(hosts), &g.MatchHosts); err != nil {
			t.Fatalf("%s の match_hosts が JSON の配列でない: %q", name, hosts)
		}
		if err := json.Unmarshal([]byte(kws), &g.MatchKwords); err != nil {
			t.Fatalf("%s の match_keywords が JSON の配列でない: %q", name, kws)
		}
		out[name] = g
	}
	return out
}

func columnExists(t *testing.T, db *sql.DB, table, col string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, col).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

const metalBuild = "METAL BUILD・ROBOT魂"

var wantStyles = map[string]genreStyle{
	"デュエル・マスターズ":   {"#e0393e", "card", []string{"dm.takaratomy.co.jp"}, []string{"デュエル・マスターズ", "デュエマ"}},
	"S.H.Figuarts": {"#7c4dff", "figure", []string{}, []string{"S.H.Figuarts", "フィギュアーツ"}},
	"ガンプラ":         {"#1f6fe5", "robot", []string{"bandai-hobby.net"}, []string{"ガンプラ", "HG", "MG", "RG", "PG", "30MF", "30MM"}},
	"ポケモングッズ":      {"#f2b705", "goods", []string{"www.pokemoncenter-online.com"}, []string{}},
	metalBuild:     {"#5b6b7f", "robot", []string{}, []string{"METAL BUILD", "ROBOT魂", "METAL ROBOT魂"}},
}

func sameStyle(a, b genreStyle) bool {
	return a.Color == b.Color && a.Icon == b.Icon && slices.Equal(a.MatchHosts, b.MatchHosts) && slices.Equal(a.MatchKwords, b.MatchKwords)
}

// AC-5-1b-3・4・5-1c-5・5-1e-3: 空の DB に 000009 まで適用すると、4 ジャンルに色・絵柄・手がかり、METAL BUILD・ROBOT魂 が足され
// (表示するサイトは S.H.Figuarts と同じ・同じ順)、Yahoo!ショッピングは scrape(URL・基準サイトは変えない)。ほかの行は変えない。
func TestGenreStyle000009_Fresh(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 8)
	beforeLinks, beforeSites := links(t, db), sitesByName(t, db)
	mustMigrate(t, m, 9)

	styles := genreStyles(t, db)
	if len(styles) != 5 {
		t.Errorf("ジャンル = %d, want 5(%v)", len(styles), styles)
	}
	for name, want := range wantStyles {
		if got, ok := styles[name]; !ok || !sameStyle(got, want) {
			t.Errorf("%s = %+v, want %+v", name, got, want)
		}
	}
	var sortOrder int
	if err := db.QueryRow(`SELECT sort_order FROM genres WHERE name = ?`, metalBuild).Scan(&sortOrder); err != nil {
		t.Fatal(err)
	}
	var maxOther int
	if err := db.QueryRow(`SELECT MAX(sort_order) FROM genres WHERE name <> ?`, metalBuild).Scan(&maxOther); err != nil {
		t.Fatal(err)
	}
	if sortOrder <= maxOther {
		t.Errorf("METAL BUILD・ROBOT魂 の sort_order = %d, want 既存のジャンル(最大 %d)より後", sortOrder, maxOther)
	}

	after := links(t, db)
	shf, mb := []string{}, []string{}
	shfOrder, mbOrder := []int{}, []int{}
	for _, l := range after {
		switch l.Genre {
		case "S.H.Figuarts":
			shf, shfOrder = append(shf, l.Site), append(shfOrder, l.SortOrder)
		case metalBuild:
			mb, mbOrder = append(mb, l.Site), append(mbOrder, l.SortOrder)
		}
	}
	if len(mb) == 0 || !slices.Equal(mb, shf) || !slices.Equal(mbOrder, shfOrder) {
		t.Errorf("METAL BUILD・ROBOT魂 のサイト = %v %v, want S.H.Figuarts と同じ %v %v", mb, mbOrder, shf, shfOrder)
	}
	for _, b := range beforeLinks {
		if !slices.Contains(after, b) {
			t.Errorf("既存の紐づけが変わった・消えた: %+v", b)
		}
	}

	sites := sitesByName(t, db)
	for name, rows := range beforeSites {
		want := rows[0]
		if name == "Yahoo!ショッピング" {
			want.FetchType = "scrape"
		}
		if got := sites[name]; len(got) != 1 || got[0] != want {
			t.Errorf("サイト %s = %+v, want %+v", name, got, want)
		}
	}
	if len(sites) != len(beforeSites) {
		t.Errorf("サイト数 = %d, want %d(足さない)", len(sites), len(beforeSites))
	}
}

// AC-5-1b-1・2: 列の既定値と制約。INSERT で省略したら #1d5fd1・box・[]・[]。色は小文字の #rrggbb だけ、絵柄は 6 つだけ。
func TestGenreStyle000009_Constraints(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 9)
	if _, err := db.Exec(`INSERT INTO genres (name) VALUES ('既定値の確認')`); err != nil {
		t.Fatal(err)
	}
	got := genreStyles(t, db)["既定値の確認"]
	if !sameStyle(got, genreStyle{"#1d5fd1", "box", []string{}, []string{}}) {
		t.Errorf("既定値 = %+v", got)
	}
	for _, c := range []string{"#ZZZZZZ", "#1D5FD1", "1d5fd1", "#1d5fd"} {
		if _, err := db.Exec(`UPDATE genres SET color = ? WHERE name = '既定値の確認'`, c); err == nil {
			t.Errorf("color %q が入った", c)
		}
	}
	for _, ic := range []string{"heart", ""} {
		if _, err := db.Exec(`UPDATE genres SET icon = ? WHERE name = '既定値の確認'`, ic); err == nil {
			t.Errorf("icon %q が入った", ic)
		}
	}
	for _, ic := range []string{"card", "figure", "robot", "goods", "star", "box"} {
		if _, err := db.Exec(`UPDATE genres SET icon = ? WHERE name = '既定値の確認'`, ic); err != nil {
			t.Errorf("icon %q が入らない: %v", ic, err)
		}
	}
}

// AC-5-1b-3・4: 使われている DB。ユーザーが同じ名前の METAL BUILD・ROBOT魂 を作っていたら足さず、その紐づけも変えない。
// 初期のジャンルが無い(名前を変えた・消した)ときは何もしない(失敗しない)。Yahoo!ショッピングをユーザーが変えていたら変えない。
func TestGenreStyle000009_ExistingDB(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 8)
	if _, err := db.Exec(`INSERT INTO genres (name, query_template, sort_order) VALUES (?, '{name}', 7)`, metalBuild); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO genre_sites (genre_id, site_id, sort_order)
		SELECT g.id, s.id, 3 FROM genres g, sites s WHERE g.name = ? AND s.name = 'Amazon'`, metalBuild); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE genres SET name = 'ポケモン' WHERE name = 'ポケモングッズ'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE sites SET fetch_type = 'link_only' WHERE name = 'Yahoo!ショッピング'`); err != nil {
		t.Fatal(err)
	}
	genres := count(t, db, "genres")
	mustMigrate(t, m, 9)

	if n := count(t, db, "genres"); n != genres {
		t.Errorf("genres = %d, want %d(同名を足さない)", n, genres)
	}
	if got := siteNames(links(t, db), metalBuild); !slices.Equal(got, []string{"Amazon"}) {
		t.Errorf("ユーザーの METAL BUILD・ROBOT魂 のサイト = %v, want [Amazon](変えない)", got)
	}
	styles := genreStyles(t, db)
	if got := styles["ポケモン"]; !sameStyle(got, genreStyle{"#1d5fd1", "box", []string{}, []string{}}) {
		t.Errorf("名前を変えたジャンル = %+v, want 既定値のまま", got)
	}
	if got := styles["デュエル・マスターズ"]; !sameStyle(got, wantStyles["デュエル・マスターズ"]) {
		t.Errorf("デュエル・マスターズ = %+v", got)
	}
	if got := sitesByName(t, db)["Yahoo!ショッピング"]; len(got) != 1 || got[0].FetchType != "link_only" {
		t.Errorf("ユーザーが変えた Yahoo!ショッピング = %+v, want link_only のまま", got)
	}
}

// AC-5-1b: 2 回適用しない(Up をもう一度呼んでも ErrNoChange)。
func TestGenreStyle000009_NotAppliedTwice(t *testing.T) {
	m, db := newMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	genres, gs := count(t, db, "genres"), count(t, db, "genre_sites")
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Errorf("2 回目の Up = %v, want migrate.ErrNoChange", err)
	}
	if g, l := count(t, db, "genres"), count(t, db, "genre_sites"); g != genres || l != gs {
		t.Errorf("genres・genre_sites = %d・%d, want %d・%d", g, l, genres, gs)
	}
}

// AC-5-1b・5-1e: down は列を外し、Yahoo!ショッピングを api に戻し、METAL BUILD・ROBOT魂(商品が無いとき)とその紐づけを消す。
// 空の DB なら 000008 の状態に戻る。METAL BUILD・ROBOT魂 に商品があれば消さない(down は失敗しない)。
func TestGenreStyle000009_Down(t *testing.T) {
	t.Run("空の DB は 000008 の状態に戻る", func(t *testing.T) {
		m, db := newMigrator(t)
		mustMigrate(t, m, 8)
		beforeLinks, beforeSites, genres := links(t, db), sitesByName(t, db), count(t, db, "genres")
		mustMigrate(t, m, 9)
		if err := m.Steps(-1); err != nil {
			t.Fatal(err)
		}
		for _, col := range []string{"color", "icon", "match_hosts", "match_keywords"} {
			if columnExists(t, db, "genres", col) {
				t.Errorf("down のあとに genres.%s が残っている", col)
			}
		}
		if n := count(t, db, "genres"); n != genres {
			t.Errorf("genres = %d, want %d", n, genres)
		}
		if got := links(t, db); !slices.Equal(got, beforeLinks) {
			t.Errorf("紐づけ = %+v, want %+v", got, beforeLinks)
		}
		if got := sitesByName(t, db); fmt.Sprint(got) != fmt.Sprint(beforeSites) {
			t.Errorf("サイト = %v, want %v(Yahoo!ショッピングは api に戻る)", got, beforeSites)
		}
	})
	t.Run("商品のある METAL BUILD・ROBOT魂 は消さない", func(t *testing.T) {
		m, db := newMigrator(t)
		mustMigrate(t, m, 9)
		if _, err := db.Exec(`INSERT INTO items (genre_id, name, image_path) SELECT id, 'METAL BUILD 架空', '11111111-1111-4111-8111-111111111111.png' FROM genres WHERE name = ?`, metalBuild); err != nil {
			t.Fatal(err)
		}
		if err := m.Steps(-1); err != nil {
			t.Fatalf("down が失敗した: %v", err)
		}
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM genres WHERE name = ?`, metalBuild).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("商品のある METAL BUILD・ROBOT魂 = %d 行, want 1(消さない)", n)
		}
	})
}
