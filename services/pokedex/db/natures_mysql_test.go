//go:build mysql

package db

// natures(000006)の制約と、P2-3 の検索クエリの照合順序を実 MySQL で確かめる(ADR-0105 §3・§4)。
// `make test-db` だけが実行する。POKEDEX_TEST_DSN が無ければ失敗する(mysql_test.go の testDSN)。

import (
	"context"
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/pokedex/internal/store"
)

func insertNature(id, nameJa, plus, minus string) string {
	lit := func(s string) string {
		if s == "" {
			return "NULL"
		}
		return "'" + s + "'"
	}
	return `INSERT INTO natures (id, name_ja, name_ja_source, name_en, plus, minus) VALUES ('` +
		id + `', '` + nameJa + `', 'override', 'Test', ` + lit(plus) + `, ` + lit(minus) + `)`
}

// AC-N4: 正しい行は入り、無補正は複数あってよい。
func TestNaturesAcceptValidRows(t *testing.T) {
	conn := freshDB(t)
	for _, stmt := range []string{
		insertNature("testbrave", "テストゆうかん", "atk", "spe"),
		insertNature("testneutrala", "テストむほせいA", "", ""),
		insertNature("testneutralb", "テストむほせいB", "", ""),
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("正しい行が入らない: %v\n%s", err, stmt)
		}
	}
	rows, err := store.New(conn).ListNatures(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].ID != "testbrave" || rows[1].Plus.Valid || rows[1].Minus.Valid {
		t.Fatalf("ListNatures = %+v", rows)
	}
}

// AC-N4: DB の CHECK・UNIQUE が不正な性格を拒否する。
func TestNaturesRejectInvalidRows(t *testing.T) {
	conn := freshDB(t)
	if _, err := conn.Exec(insertNature("testbrave", "テストゆうかん", "atk", "spe")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		stmt  string
		codes []uint16
	}{
		{"plus が HP", insertNature("testhp", "テストえいち", "hp", "atk"), []uint16{errCheckViolated}},
		{"minus が未知", insertNature("testunk", "テストみち", "atk", "speed"), []uint16{errCheckViolated}},
		{"plus だけ", insertNature("testhalf", "テストかたほう", "atk", ""), []uint16{errCheckViolated}},
		{"plus と minus が同じ", insertNature("testsame", "テストおなじ", "atk", "atk"), []uint16{errCheckViolated}},
		{"ID に大文字", insertNature("TestBig", "テストおおもじ", "def", "atk"), []uint16{errCheckViolated}},
		{"日本語名が空", insertNature("testempty", "", "def", "atk"), []uint16{errCheckViolated}},
		{"同じ補正の組", insertNature("testbrave2", "テストゆうかん2", "atk", "spe"), []uint16{errDupEntry}},
		{"ID の重複", insertNature("testbrave", "テストゆうかん3", "def", "spe"), []uint16{errDupEntry}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conn.Exec(tc.stmt)
			var me *mysql.MySQLError
			if !errors.As(err, &me) {
				t.Fatalf("DB が拒否しなかった(err=%v)", err)
			}
			for _, c := range tc.codes {
				if me.Number == c {
					return
				}
			}
			t.Fatalf("エラー番号 %d(%s), want %v", me.Number, me.Message, tc.codes)
		})
	}
}

// AC-P2: 検索クエリは使用可能集合で絞り、日本語名の前方一致はひらがなとカタカナを区別しない
// (utf8mb4_ja_0900_as_cs。ADR-0100 §2)。example_seed.sql の架空データを使う。
func TestSearchSpeciesPrefixAndRegulation(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	q := store.New(conn)
	ctx := context.Background()

	reg, err := q.GetDefaultRegulation(ctx)
	if err != nil {
		t.Fatalf("GetDefaultRegulation: %v(example_seed.sql に既定のレギュレーションがあること)", err)
	}
	keys, err := q.ListRegulationSpeciesKeys(ctx, reg.ID)
	if err != nil || len(keys) == 0 {
		t.Fatalf("ListRegulationSpeciesKeys = %v, %v", keys, err)
	}
	all, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: reg.ID, Pattern: "%", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(keys) {
		t.Fatalf("空の前方一致の件数 %d, want 既定のレギュレーションの種族数 %d", len(all), len(keys))
	}
	// 「テスト」はカタカナ。ひらがなの「てすと」でも一致する(照合順序が kana-insensitive)。
	hira, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: reg.ID, Pattern: "てすと%", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(hira) != len(all) {
		t.Errorf("ひらがなの前方一致 %d 件, want %d 件(ひらがなとカタカナを区別しない)", len(hira), len(all))
	}
	limited, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: reg.ID, Pattern: "%", Limit: 1})
	if err != nil || len(limited) != 1 {
		t.Errorf("LIMIT 1 の件数 %d, err=%v", len(limited), err)
	}
	none, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: "test-none", Pattern: "%", Limit: 200})
	if err != nil || len(none) != 0 {
		t.Errorf("未知のレギュレーションで %d 件, err=%v", len(none), err)
	}
}

// AC-P2: LIKE の特殊文字(%)がエスケープされ、リテラルとして一致すること。
// MySQL 既定のエスケープ文字 \ に依存している(クエリに ESCAPE 句が無い)ので、
// NO_BACKSLASH_ESCAPES が有効な環境では壊れる。それを実 MySQL で確かめる(critic の軽微指摘)。
// httpapi.likeEscaper と同じ変換を手元で行い、DB 層(LIKE そのもの)だけを見る。
//
// 2件の名前を用意する: 「literal」は名前に文字どおりの % を含み、「gap」は同じ位置に
// 別の文字(丸)を置く(% を含まない)。「% off」をエスケープした前方一致は literal だけに
// 当たり、エスケープしないままだと % がワイルドカードになって literal と gap の両方に当たる。
func TestSearchSpeciesEscapesLikeSpecialChars(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	insertSpecies := "INSERT INTO species (`key`, dex_no, form, showdown_id, name_ja, name_ja_source, name_en, type1, base_hp, base_atk, base_def, base_spa, base_spd, base_spe, is_mega) VALUES " +
		"('9003-000', 9003, 0, 'testliteral', 'テスト100%off', 'override', 'TestLiteral', 'normal', 1, 1, 1, 1, 1, 1, 0), " +
		"('9003-001', 9003, 1, 'testgap', 'テスト100丸off', 'override', 'TestGap', 'normal', 1, 1, 1, 1, 1, 1, 0)"
	if _, err := conn.Exec(insertSpecies); err != nil {
		t.Fatalf("%%を含む名前の行を入れられない: %v", err)
	}
	q := store.New(conn)
	ctx := context.Background()
	reg, err := q.GetDefaultRegulation(ctx)
	if err != nil {
		t.Fatalf("GetDefaultRegulation: %v", err)
	}
	if _, err := conn.Exec(
		"INSERT INTO regulation_species (regulation_id, species_key) VALUES (?, '9003-000'), (?, '9003-001')", reg.ID, reg.ID,
	); err != nil {
		t.Fatalf("既定のレギュレーションに追加できない: %v", err)
	}

	// "%" をエスケープした前方一致は、"テスト100%off" で始まる literal だけに当たる。
	escaped := "テスト100\\%off" + "%"
	got, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: reg.ID, Pattern: escaped, Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "9003-000" {
		t.Fatalf("エスケープした %% の前方一致 = %v(len=%d), want [9003-000] のみ", got, len(got))
	}

	// エスケープしないままだと "%" がワイルドカードになり、literal・gap の両方に当たる。
	unescaped, err := q.SearchSpecies(ctx, store.SearchSpeciesParams{RegulationID: reg.ID, Pattern: "テスト100%off%", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(unescaped) != 2 {
		t.Fatalf("未エスケープの %% がワイルドカードとして働いていない(%d 件, want 2): NO_BACKSLASH_ESCAPES が有効かも", len(unescaped))
	}
}
