//go:build mysql

package db

// 種族検索 SearchSpecies の規則(ADR-0324・issue #515)を実 MySQL で確かめる。`make test-db` だけが実行する。
//   - メガ種族の名前(メガテストモンX)で検索するとメガ種族が返る
//   - 「メガ」で検索すると全メガ種族が返る
//   - メガを除いた基本種名(テストモン)で検索すると、基本種とメガ種族の両方が返る(図鑑番号・フォーム番号の昇順)
//   - メガでない種族は「メガ + q」では当たらない

import (
	"context"
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
		{"メガ", []string{"9001-001"}},
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
