//go:build mysql

package db

// 技の逆引きのクエリ ListMoveLearners(AJ5・ADR-0251)を実 MySQL で確かめる。
// `make test-db` だけが実行する。POKEDEX_TEST_DSN が無ければ失敗する(mysql_test.go の testDSN)。
// example_seed.sql の架空データに、使用可能集合の外の種族・技と、図鑑番号の大きい種族を足して使う。

import (
	"context"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
)

func learnerKeysOf(rows []store.ListMoveLearnersRow) []string {
	keys := []string{}
	for _, r := range rows {
		keys = append(keys, r.Key)
	}
	return keys
}

// AC-L9: 種族・技の両方が既定のレギュレーションの使用可能集合にあるものだけを、dex_no・form の昇順で返す。
// LIMIT・OFFSET でページングできる。使用可能集合の外の技は 0 件。
func TestListMoveLearnersQuery(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	for _, stmt := range []string{
		speciesInsert("9004-000", 9004, 0, "testlate", "'fire'", "NULL", 0, "NULL", "NULL"),
		speciesInsert("9005-000", 9005, 0, "testoutside", "'water'", "NULL", 0, "NULL", "NULL"),
		`INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority) VALUES
		  ('testbanned', 'テストキンシ', 'override', 'Test Banned', 'water', 'special', 80, 100, 10, 0)`,
		`DELETE FROM learnsets WHERE move_id = 'testflame'`,
		// 挿入の順は図鑑番号の逆(ORDER BY が効いていることを確かめるため)。
		`INSERT INTO learnsets (species_key, move_id) VALUES
		  ('9005-000', 'testflame'), ('9004-000', 'testflame'), ('9001-001', 'testflame'), ('9001-000', 'testflame'),
		  ('9001-000', 'testbanned')`,
		`INSERT INTO regulation_species (regulation_id, species_key) VALUES ('test-a', '9004-000'), ('test-b', '9005-000')`,
		`INSERT INTO regulation_moves (regulation_id, move_id) VALUES ('test-b', 'testbanned'), ('test-b', 'testflame')`,
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

	list := func(moveID string, limit, offset int32) []store.ListMoveLearnersRow {
		t.Helper()
		rows, err := q.ListMoveLearners(ctx, store.ListMoveLearnersParams{RegulationID: reg.ID, MoveID: moveID, Limit: limit, Offset: offset})
		if err != nil {
			t.Fatalf("ListMoveLearners(%s, %d, %d): %v", moveID, limit, offset, err)
		}
		return rows
	}

	all := list("testflame", 200, 0)
	if got, want := learnerKeysOf(all), []string{"9001-000", "9001-001", "9004-000"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("testflame の逆引き = %v, want %v(9005-000 は集合の外。dex_no・form の昇順)", got, want)
	}
	mega := all[1]
	if mega.DexNo != 9001 || mega.Form != 1 || mega.Type1 != "fire" || !mega.Type2.Valid || mega.Type2.String != "water" || mega.NameJa == "" {
		t.Errorf("9001-001 の行 = %+v, want dex 9001・form 1・fire/water・名前あり", mega)
	}
	if got := learnerKeysOf(list("testflame", 2, 0)); !reflect.DeepEqual(got, []string{"9001-000", "9001-001"}) {
		t.Errorf("LIMIT 2 = %v", got)
	}
	if got := learnerKeysOf(list("testflame", 2, 2)); !reflect.DeepEqual(got, []string{"9004-000"}) {
		t.Errorf("LIMIT 2 OFFSET 2 = %v", got)
	}
	if got := list("testflame", 200, 3); len(got) != 0 {
		t.Errorf("末尾を超える OFFSET で %d 件", len(got))
	}
	if got := list("testbanned", 200, 0); len(got) != 0 {
		t.Errorf("使用可能集合の外の技 testbanned で %d 件(%v)", len(got), learnerKeysOf(got))
	}
	if got := list("testnosuchmove", 200, 0); len(got) != 0 {
		t.Errorf("マスタに無い技で %d 件", len(got))
	}

	// getSpecies の learnset(ListSpeciesLearnset)と同じ規則: 使用可能な種族 S・技 M について
	// 「S が M の逆引きに出る」⇔「ListSpeciesLearnset(S) に M がある」。
	speciesKeys, err := q.ListRegulationSpeciesKeys(ctx, reg.ID)
	if err != nil {
		t.Fatal(err)
	}
	moveIDs, err := q.ListMoveIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range moveIDs {
		listed := map[string]bool{}
		for _, r := range list(m, 200, 0) {
			listed[r.Key] = true
		}
		for _, s := range speciesKeys {
			ls, err := q.ListSpeciesLearnset(ctx, store.ListSpeciesLearnsetParams{SpeciesKey: s, RegulationID: reg.ID})
			if err != nil {
				t.Fatal(err)
			}
			if listed[s] != contains(ls, m) {
				t.Errorf("技 %s・種族 %s: 逆引き=%v, learnset=%v(一致すること)", m, s, listed[s], contains(ls, m))
			}
		}
	}
}

// AC-L10: learnsets に move_id を先頭の列とする索引がある(逆引きが全件走査にならない)。
// 000002 の FK fk_learnsets_move のために MySQL が自動で作る索引で足りるので、migration を足さない(ADR-0251)。
func TestLearnsetsHasMoveIDIndex(t *testing.T) {
	conn := freshDB(t)
	var n int
	err := conn.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'learnsets' AND column_name = 'move_id' AND seq_in_index = 1`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("learnsets に move_id を先頭の列とする索引が無い")
	}
}
