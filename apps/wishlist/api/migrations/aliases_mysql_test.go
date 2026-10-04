//go:build mysql

// 000005(表記揺れの辞書 genre_aliases)を実 MySQL で確かめる。`make wishlist-test-mysql` で流す。docs/phase4-spec.md AC-A12・A13。
// 補助(newMigrator・mustMigrate・count)は seed_mysql_test.go のもの。
package migrations_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/golang-migrate/migrate/v4"

	"example.com/pokecalc/apps/wishlist/api/internal/query"
	"example.com/pokecalc/apps/wishlist/api/migrations"
)

// aliasRows はジャンル名ごとの別名グループ(group_no 昇順、グループ内は id 昇順)を返す。
// あわせて normalized 列が query.Normalize(alias) と一致することを確かめる。
func aliasRows(t *testing.T, db *sql.DB) map[string][][]string {
	t.Helper()
	rows, err := db.Query(`SELECT g.name, a.group_no, a.alias, a.normalized FROM genre_aliases a
		JOIN genres g ON g.id = a.genre_id ORDER BY g.id, a.group_no, a.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][][]string{}
	last := map[string]int{}
	for rows.Next() {
		var genre, alias, norm string
		var no int
		if err := rows.Scan(&genre, &no, &alias, &norm); err != nil {
			t.Fatal(err)
		}
		if want := query.Normalize(alias); norm != want {
			t.Errorf("%s の %q の normalized = %q, want %q", genre, alias, norm, want)
		}
		groups := out[genre]
		if len(groups) == 0 || last[genre] != no {
			groups = append(groups, nil)
			last[genre] = no
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], alias)
		out[genre] = groups
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

var wantSeedAliases = map[string][][]string{
	"S.H.Figuarts": {{"S.H.Figuarts", "SHフィギュアーツ"}},
	"ガンプラ":         {{"HG", "ハイグレード"}, {"MG", "マスターグレード"}, {"RG", "リアルグレード"}},
}

func eqGroups(a, b [][]string) bool { return fmt.Sprintf("%q", a) == fmt.Sprintf("%q", b) }

// AC-A12: 空の DB に 000005 まで適用すると、既定の辞書が入る(S.H.Figuarts に 1 グループ、ガンプラに 3 グループ。他のジャンルは無し)。
// normalized は query.Normalize と同じ値。
func TestGenreAliases000005_Fresh(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 5)
	got := aliasRows(t, db)
	if len(got) != len(wantSeedAliases) {
		t.Errorf("辞書のあるジャンル = %q, want %q", got, wantSeedAliases)
	}
	for g, want := range wantSeedAliases {
		if !eqGroups(got[g], want) {
			t.Errorf("%s の辞書 = %q, want %q", g, got[g], want)
		}
	}
}

// AC-A12: 2 回適用しない。up の SQL をもう一度流しても失敗せず、行は増えない。
func TestGenreAliases000005_NotAppliedTwice(t *testing.T) {
	m, db := newMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	n := count(t, db, "genre_aliases")
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Errorf("2 回目の Up = %v, want migrate.ErrNoChange", err)
	}
	b, err := migrations.FS.ReadFile("000005_genre_aliases.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("up の SQL をもう一度流すと失敗した: %v", err)
	}
	if got := count(t, db, "genre_aliases"); got != n {
		t.Errorf("genre_aliases = %d 行, want %d", got, n)
	}
}

// AC-A12: 使われている DB(ガンプラを消し、S.H.Figuarts を残した DB)に流しても失敗せず、あるジャンルにだけ入る。
func TestGenreAliases000005_MissingGenre(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 4)
	if _, err := db.Exec(`DELETE FROM genres WHERE name = 'ガンプラ'`); err != nil {
		t.Fatal(err)
	}
	mustMigrate(t, m, 5)
	got := aliasRows(t, db)
	if len(got) != 1 || !eqGroups(got["S.H.Figuarts"], wantSeedAliases["S.H.Figuarts"]) {
		t.Errorf("辞書 = %q, want S.H.Figuarts だけ", got)
	}
}

// AC-A12: down は genre_aliases だけを消し、000004 の状態(ジャンル 4・サイト 7)に戻る。
func TestGenreAliases000005_Down(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 5)
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'genre_aliases'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("down のあとに genre_aliases が残っている")
	}
	if g, s := count(t, db, "genres"), count(t, db, "sites"); g != 4 || s != 7 {
		t.Errorf("genres = %d・sites = %d, want 4・7", g, s)
	}
}

// AC-A13: ジャンルを消すと、その辞書も消える(CASCADE)。同じジャンル内で normalized は一意。
// normalized は utf8mb4_bin で比べる(濁点・ひらがなとカタカナの違いは別の語)。
func TestGenreAliases000005_Constraints(t *testing.T) {
	m, db := newMigrator(t)
	mustMigrate(t, m, 5)
	var gid int64
	if err := db.QueryRow(`SELECT id FROM genres WHERE name = 'ガンプラ'`).Scan(&gid); err != nil {
		t.Fatal(err)
	}
	ins := func(alias string) error {
		_, err := db.Exec(`INSERT INTO genre_aliases (genre_id, group_no, alias, normalized) VALUES (?, 9, ?, ?)`, gid, alias, query.Normalize(alias))
		return err
	}
	if err := ins("ｈｇ"); err == nil {
		t.Error("同じジャンルに正規化後の重複(hg)が入った")
	}
	for _, a := range []string{"カンダム", "ガンダム", "はいぐれーど"} {
		if err := ins(a); err != nil {
			t.Errorf("%s(正規化後は別の語)が入らない: %v", a, err)
		}
	}
	if _, err := db.Exec(`DELETE FROM genres WHERE id = ?`, gid); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM genre_aliases WHERE genre_id = ?`, gid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("ジャンルを消しても辞書が %d 行残っている", n)
	}
}
