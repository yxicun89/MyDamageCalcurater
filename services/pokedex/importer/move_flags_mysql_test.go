//go:build mysql

package importer_test

// 技のフラグ(move_flags。ADR-0178)の投入を実 MySQL で確かめる。`make test-db` だけが実行する。
//   - Apply が Output.MoveFlags を move_flags にそのまま入れる(行の集合が一致する)
//   - 入れ直し(2回目の Apply)で古い行が残らない(技とフラグを同じトランザクションで入れ直す。ADR-0178 §3)

import (
	"context"
	"database/sql"
	"reflect"
	"sort"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

func TestApplyWritesMoveFlags(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	if len(out.MoveFlags) == 0 {
		t.Fatal("fixture の変換結果に move_flags が無い")
	}
	at := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(context.Background(), conn, out, versions, at); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := selectMoveFlags(t, conn); !reflect.DeepEqual(got, out.MoveFlags) {
		t.Fatalf("move_flags = %+v,\nwant %+v", got, out.MoveFlags)
	}

	// 1行減らして入れ直すと、その行は残らない。
	changed := out
	changed.MoveFlags = append([]importer.MoveFlagRow(nil), out.MoveFlags[1:]...)
	if err := importer.Apply(context.Background(), conn, changed, versions, at.Add(time.Hour)); err != nil {
		t.Fatalf("Apply(2回目): %v", err)
	}
	if got := selectMoveFlags(t, conn); !reflect.DeepEqual(got, changed.MoveFlags) {
		t.Fatalf("入れ直し後の move_flags = %+v,\nwant %+v", got, changed.MoveFlags)
	}
}

func selectMoveFlags(t *testing.T, conn *sql.DB) []importer.MoveFlagRow {
	t.Helper()
	rows, err := conn.Query(`SELECT move_id, flag FROM move_flags`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []importer.MoveFlagRow
	for rows.Next() {
		var r importer.MoveFlagRow
		if err := rows.Scan(&r.MoveID, &r.Flag); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MoveID != out[j].MoveID {
			return out[i].MoveID < out[j].MoveID
		}
		return out[i].Flag < out[j].Flag
	})
	return out
}
