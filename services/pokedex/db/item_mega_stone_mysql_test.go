//go:build mysql

package db

// 持ち物の検索 SearchItems の is_mega_stone(ADR-0175 §2)を実 MySQL で確かめる。`make test-db` だけが実行する。
//   - いずれかのメガ種族の required_item_id に現れる持ち物が true(seed の teststone)
//   - 使用可能集合の外のメガ種族のストーンも true(レギュレーションで絞らない)
//   - 非メガ種族に紐づく持ち物は無い(CHECK で禁止)ので、それ以外はすべて false
//   - 列を足しても並び(name_ja, id)・使用可能集合の絞り込みは変わらない

import (
	"context"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
)

func TestSearchItemsIsMegaStone(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	// 使用可能集合の外のメガ種族(9301-001)と、そのストーン(teststonetwo。持ち物は集合の中)を足す。
	for _, stmt := range []string{
		"INSERT INTO items (id, name_ja, name_ja_source, name_en) VALUES ('teststonetwo', 'テストストーン2', 'fallback_en', 'Test Stone Two')",
		"INSERT INTO regulation_items (regulation_id, item_id) VALUES ('test-a', 'teststonetwo')",
		speciesInsert("9301-000", 9301, 0, "testouter", "'water'", "NULL", 0, "NULL", "NULL"),
		speciesInsert("9301-001", 9301, 1, "testoutermega", "'water'", "NULL", 1, "'9301-000'", "'teststonetwo'"),
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
	rows, err := q.SearchItems(ctx, store.SearchItemsParams{RegulationID: reg.ID, Pattern: "%", Limit: 200})
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	got := map[string]bool{}
	var order []string
	for _, r := range rows {
		got[r.ID] = r.IsMegaStone
		order = append(order, r.ID)
	}
	want := map[string]bool{"teststone": true, "teststonetwo": true, "testorb": false, "testplain": false}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("is_mega_stone = %v, want %v", got, want)
	}
	// 並びは name_ja, id(テストオーブ < テストストーン < テストストーン2 < テストただのもの。照合順序の昇順)。
	wantOrder := []string{"testorb", "teststone", "teststonetwo", "testplain"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Errorf("並び = %v, want %v", order, wantOrder)
	}
}
