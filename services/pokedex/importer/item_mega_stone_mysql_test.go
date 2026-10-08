//go:build mysql

package importer_test

// importer が items.is_mega_stone を書くこと(ADR-0140・issue #607)。実 MySQL(test-db のターゲットだけが実行する)。

import (
	"context"
	"reflect"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

func TestApplyWritesItemIsMegaStone(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	if err := importer.Apply(context.Background(), conn, out, versions, time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	rows, err := conn.Query("SELECT id, is_mega_stone FROM items ORDER BY id")
	if err != nil {
		t.Fatalf("items を読めない: %v", err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var id string
		var stone bool
		if err := rows.Scan(&id, &stone); err != nil {
			t.Fatal(err)
		}
		got[id] = stone
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := itemMegaStone(out)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DB の is_mega_stone = %v, want 変換結果と同じ %v", got, want)
	}
	if !got["testmonite"] {
		t.Error("testmonite が is_mega_stone = 1 で入っていない")
	}
}
