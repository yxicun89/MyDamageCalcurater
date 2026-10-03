//go:build mysql

package importer_test

// 技の対象(moves.target。ADR-0136)の投入を実 MySQL で確かめる。`make test-db` だけが実行する。
// 列は NULL を許す(migration 直後の既存の行のため)が、importer の投入後は全行が値を持つこと。

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

func TestApplyWritesMoveTargets(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	if err := importer.Apply(context.Background(), conn, out, versions, time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var nulls int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM moves WHERE target IS NULL`).Scan(&nulls); err != nil {
		t.Fatal(err)
	}
	if nulls != 0 {
		t.Errorf("投入後に target が NULL の技が %d 件ある(importer は必ず対象を入れる)", nulls)
	}
	for _, m := range out.Moves {
		var got sql.NullString
		if err := conn.QueryRow(`SELECT target FROM moves WHERE id = ?`, m.ID).Scan(&got); err != nil {
			t.Fatalf("%s: %v", m.ID, err)
		}
		if !got.Valid || got.String != m.Target {
			t.Errorf("%s の target = %+v, want %q", m.ID, got, m.Target)
		}
	}
}
