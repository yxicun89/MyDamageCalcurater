//go:build mysql

package db

// items.is_mega_stone(取得元の持ち物データからの判定。ADR-0140・issue #607)を実 MySQL で確かめる(test-db のターゲットだけが実行する)。
//   - 列が真の持ち物は、どのメガ種族にも要求されていなくても SearchItems の is_mega_stone が true
//   - 列の既定値は偽(列を指定しない INSERT・migrate 前からある行)
//   - 列が偽でも、メガ種族の required_item_id に現れる持ち物は従来どおり true(TestSearchItemsIsMegaStone)

import (
	"context"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
)

func TestSearchItemsIsMegaStoneFromColumn(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	for _, stmt := range []string{
		// メガ種族の無いストーン(対応するメガがレギュレーション外で取り込まれない)。
		"INSERT INTO items (id, name_ja, name_ja_source, name_en, is_mega_stone) VALUES ('teststonelone', 'テストストーンひとり', 'fallback_en', 'Test Stone Lone', 1)",
		"INSERT INTO regulation_items (regulation_id, item_id) VALUES ('test-a', 'teststonelone')",
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("追加の行を入れられない: %v\n%s", err, stmt)
		}
	}
	var def int
	if err := conn.QueryRow("SELECT is_mega_stone FROM items WHERE id = 'testplain'").Scan(&def); err != nil {
		t.Fatalf("is_mega_stone を読めない: %v", err)
	}
	if def != 0 {
		t.Errorf("列を指定しない行の is_mega_stone = %d, want 0", def)
	}

	q := store.New(conn)
	ctx := context.Background()
	reg, err := q.GetDefaultRegulation(ctx)
	if err != nil {
		t.Fatalf("GetDefaultRegulation: %v", err)
	}
	rows, err := q.SearchItems(ctx, store.SearchItemsParams{RegulationID: reg.ID, Pattern: "%", Limit: 200})
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.ID] = r.IsMegaStone
	}
	want := map[string]bool{"teststone": true, "teststonelone": true, "testorb": false, "testplain": false}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("is_mega_stone = %v, want %v", got, want)
	}
}
