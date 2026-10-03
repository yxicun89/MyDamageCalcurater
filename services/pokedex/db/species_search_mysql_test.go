//go:build mysql

package db

// 種族検索 SearchSpecies の規則(ADR-0324・issue #515)を実 MySQL で確かめる。`make test-db` だけが実行する。
//   - メガ種族の名前(メガテストモンX)で検索するとメガ種族が返る
//   - 「メガ」で検索すると全メガ種族が返る
//   - メガを除いた基本種名(テストモン)で検索すると、基本種とメガ種族の両方が返る(図鑑番号・フォーム番号の昇順)
//   - メガでない種族は「メガ + q」では当たらない

import (
	"context"
	"os"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/store"
)

func TestSearchSpeciesMegaRule(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	// seed のメガ(9001-001)の名前を、importer が生成する形(メガ + 基本種名 + 識別子)にする。
	if _, err := conn.Exec("UPDATE species SET name_ja = 'メガテストモンX', name_ja_source = 'generated' WHERE `key` = '9001-001'"); err != nil {
		t.Fatal(err)
	}
	// 名前が「メガ」で始まる非メガ種族(実データのメガニウム・メガヤンマ相当。架空名)。is_mega ガードの回帰:
	// 規則 1(名前の前方一致)でだけ当たり、「メガ + q」の規則 2 では当たらない。
	for _, stmt := range []string{
		speciesInsert("9201-000", 9201, 0, "testmegaso", "'grass'", "NULL", 0, "NULL", "NULL"),
		"UPDATE species SET name_ja = 'メガテストそう' WHERE `key` = '9201-000'",
		"INSERT INTO regulation_species (regulation_id, species_key) VALUES ('test-a', '9201-000')",
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("追加の行を入れられない: %v\n%s", err, stmt)
		}
	}
	q := store.New(conn)
	ctx := context.Background()
	reg, err := q.GetDefaultRegulation(ctx)
	if err != nil {
		t.Fatalf("GetDefaultRegulation: %v", err)
	}
	search := func(query string) []string {
		t.Helper()
		pattern, megaPattern := httpapi.SpeciesSearchPatterns(query)
		rows, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: reg.ID, Pattern: pattern, MegaPattern: megaPattern, Limit: 200})
		if err != nil {
			t.Fatalf("SearchSpecies(%q): %v", query, err)
		}
		keys := []string{}
		for _, r := range rows {
			keys = append(keys, r.Key)
		}
		return keys
	}

	tests := []struct {
		query string
		want  []string
	}{
		{"メガテストモン", []string{"9001-001"}},
		{"メガテストモンX", []string{"9001-001"}},
		{"メガ", []string{"9001-001", "9201-000"}}, // 規則 1: 名前が「メガ」で始まる非メガも出る
		{"テストそう", []string{}},                    // 非メガは「メガ + q」では当たらない
		{"メガテストそう", []string{"9201-000"}},
		{"テストモン", []string{"9001-000", "9001-001"}},
		{"テストモンX", []string{"9001-001"}},
		{"メガテストリーフ", []string{}},
	}
	for _, tt := range tests {
		if got := search(tt.query); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("q=%q: %v, want %v", tt.query, got, tt.want)
		}
	}
}

// migration 000011 の down は、generated の行がある状態でも通り、行を fallback_en に寄せて CHECK を戻す。
// 戻したあとは up を流し直す(後続のテストの DownAll・Up を壊さない)。
func TestMigration000011DownWithGeneratedRows(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	if _, err := conn.Exec("UPDATE species SET name_ja_source = 'generated' WHERE `key` = '9001-001'"); err != nil {
		t.Fatalf("generated を入れられない(up の CHECK): %v", err)
	}
	runDownFile(t, conn, speciesNameSourceMigrationVersion)
	var source, name string
	if err := conn.QueryRow("SELECT name_ja_source, name_ja FROM species WHERE `key` = '9001-001'").Scan(&source, &name); err != nil {
		t.Fatal(err)
	}
	if source != "fallback_en" || name == "" {
		t.Errorf("down の後 = (%q, %q), want (fallback_en, 名前はそのまま)", source, name)
	}
	if _, err := conn.Exec("UPDATE species SET name_ja_source = 'generated' WHERE `key` = '9001-001'"); err == nil {
		t.Error("down の後も generated を入れられてしまう(CHECK が戻っていない)")
	}
	_, up, _ := migrationPairs(t)
	raw, err := os.ReadFile(up[speciesNameSourceMigrationVersion])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(raw)); err != nil {
		t.Fatalf("up の流し直し: %v", err)
	}
}
