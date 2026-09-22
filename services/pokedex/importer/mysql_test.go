//go:build mysql

package importer_test

// 実 MySQL への投入のテスト(ADR-0101 §8・§9)。`make test-db` だけが実行する(`make test` には含めない)。
// POKEDEX_TEST_DSN が無い・DB に届かないときは**失敗**する(黙ってスキップしない)。
// 全テーブルを消すので、DB 名が _test で終わる DSN だけを受け付ける(services/pokedex/db の mysql_test.go と同じ規則)。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	pokedexdb "example.com/pokecalc/services/pokedex/db"
	"example.com/pokecalc/services/pokedex/importer"
)

// masterTables は投入で置き換えるテーブル(ADR-0100 §3。schema_migrations を除く全テーブル)。
var masterTables = []string{
	"types", "type_chart", "abilities", "items", "moves", "species", "species_abilities",
	"item_effects", "ability_effects", "learnsets",
	"regulations", "regulation_species", "regulation_moves", "regulation_items", "regulation_abilities",
	"data_versions",
}

func freshImportDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("POKEDEX_TEST_DSN")
	if dsn == "" {
		t.Fatal("POKEDEX_TEST_DSN が無い(make test-db は DB を前提にする。スキップしない)")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("POKEDEX_TEST_DSN を解釈できない: %v", err)
	}
	if !strings.HasSuffix(cfg.DBName, "_test") {
		t.Fatalf("DB 名は _test で終わること(全テーブルを消すため): %q", cfg.DBName)
	}
	if err := pokedexdb.DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll: %v", err)
	}
	if err := pokedexdb.Up(dsn); err != nil {
		t.Fatalf("Up: %v", err)
	}
	c := cfg.Clone()
	c.ParseTime = true
	conn, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}
	return conn
}

func fixtureOutput(t *testing.T) (importer.Output, []importer.SourceVersion) {
	t.Helper()
	in, versions, err := importer.LoadInput(fixtureRoot)
	if err != nil {
		t.Fatalf("LoadInput: %v", err)
	}
	out, _, err := importer.Convert(in)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return out, versions
}

// dumpTables は全テーブルの中身を、行を文字列にして並べ替えた形で返す(比較用)。
// exclude に "table.column" を渡すと、その列を比較から外す。
func dumpTables(t *testing.T, conn *sql.DB, exclude ...string) map[string][]string {
	t.Helper()
	skip := map[string]bool{}
	for _, e := range exclude {
		skip[e] = true
	}
	out := map[string][]string{}
	for _, table := range masterTables {
		rows, err := conn.Query("SELECT * FROM `" + table + "`")
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for rows.Next() {
			vals := make([]sql.RawBytes, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			for i, c := range cols {
				if skip[table+"."+c] {
					continue
				}
				if vals[i] == nil {
					fmt.Fprintf(&b, "%s=NULL;", c)
				} else {
					fmt.Fprintf(&b, "%s=%s;", c, vals[i])
				}
			}
			lines = append(lines, b.String())
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		sort.Strings(lines)
		out[table] = lines
	}
	return out
}

func TestApplyWritesOutput(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(context.Background(), conn, out, versions, now); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	dump := dumpTables(t, conn)
	counts := map[string]int{
		"types": len(out.Types), "type_chart": len(out.TypeChart), "abilities": len(out.Abilities), "items": len(out.Items),
		"moves": len(out.Moves), "species": len(out.Species), "item_effects": len(out.ItemEffects),
		"ability_effects": len(out.AbilityEffects), "learnsets": len(out.Learnsets), "regulations": len(out.Regulations),
		"regulation_species": len(out.RegulationSpecies), "regulation_moves": len(out.RegulationMoves),
		"regulation_items": len(out.RegulationItems), "regulation_abilities": len(out.RegulationAbilities),
		"data_versions": len(versions),
	}
	for table, want := range counts {
		if got := len(dump[table]); got != want {
			t.Errorf("%s の行数 = %d, want %d", table, got, want)
		}
	}
	applied, err := importer.AppliedVersions(context.Background(), conn)
	if err != nil {
		t.Fatalf("AppliedVersions: %v", err)
	}
	if !reflect.DeepEqual(versionsBySource(applied), versionsBySource(versions)) {
		t.Errorf("data_versions = %+v, want %+v", applied, versions)
	}
	// 必中の技は accuracy が NULL(Output の Accuracy 0)。
	var acc sql.NullInt64
	if err := conn.QueryRow("SELECT accuracy FROM moves WHERE id = 'teststrike'").Scan(&acc); err != nil {
		t.Fatal(err)
	}
	if acc.Valid {
		t.Errorf("必中の技の accuracy = %d, want NULL", acc.Int64)
	}
}

// 同じ入力で2回投入しても行が増えない・変わらない(imported_at も同じ now なら同じ)。
func TestApplyIsIdempotent(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, now); err != nil {
		t.Fatalf("Apply 1回目: %v", err)
	}
	first := dumpTables(t, conn)
	if err := importer.Apply(ctx, conn, out, versions, now); err != nil {
		t.Fatalf("Apply 2回目: %v", err)
	}
	if second := dumpTables(t, conn); !reflect.DeepEqual(first, second) {
		t.Errorf("2回目の投入で中身が変わった\n1回目: %v\n2回目: %v", first, second)
	}
}

// 版に変化が無ければ Run は投入しない(imported_at も変えない)。版が変われば投入する。
func TestRunSkipsWhenVersionsUnchanged(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(7 * 24 * time.Hour)

	applied, err := importer.Run(ctx, conn, out, versions, t1, false)
	if err != nil || !applied {
		t.Fatalf("初回の Run = (%v, %v), want (true, nil)", applied, err)
	}
	before := dumpTables(t, conn)

	applied, err = importer.Run(ctx, conn, out, versions, t2, false)
	if err != nil {
		t.Fatalf("2回目の Run: %v", err)
	}
	if applied {
		t.Errorf("版に変化が無いのに投入した")
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("スキップしたのに中身(imported_at を含む)が変わった")
	}

	// force なら版が同じでも投入する(中身は imported_at 以外同じ。imported_at は新しい時刻)。
	beforeForce := dumpTables(t, conn, "data_versions.imported_at")
	applied, err = importer.Run(ctx, conn, out, versions, t2, true)
	if err != nil || !applied {
		t.Fatalf("force の Run = (%v, %v), want (true, nil)", applied, err)
	}
	if afterForce := dumpTables(t, conn, "data_versions.imported_at"); !reflect.DeepEqual(beforeForce, afterForce) {
		t.Errorf("force の再投入で imported_at 以外の中身が変わった")
	}
	var importedAt time.Time
	if err := conn.QueryRow("SELECT imported_at FROM data_versions WHERE source = 'calc'").Scan(&importedAt); err != nil {
		t.Fatal(err)
	}
	if !importedAt.Equal(t2) {
		t.Errorf("force 後の imported_at = %v, want %v", importedAt, t2)
	}

	changed := append([]importer.SourceVersion(nil), versions...)
	for i := range changed {
		if changed[i].Source == "effects" {
			changed[i].Checksum = strings.Repeat("0", 64)
		}
	}
	applied, err = importer.Run(ctx, conn, out, changed, t2, false)
	if err != nil || !applied {
		t.Fatalf("チェックサムが変わった Run = (%v, %v), want (true, nil)", applied, err)
	}
}

// 既存の showdown_id の key が変わる投入は止める(team-svc などが保存した key を壊さない)。DB は変えない。
func TestApplyRejectsSpeciesKeyChange(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(ctx, conn, out, versions, now); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := dumpTables(t, conn)

	rekeyed := rekey(out, "9002-002", "9002-001", 1)
	err := importer.Apply(ctx, conn, rekeyed, versions, now.Add(time.Hour))
	if !errors.Is(err, importer.ErrKeyChanged) {
		t.Fatalf("err = %v, want ErrKeyChanged", err)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("止めたのに DB が変わった")
	}
}

// 既存の key を別の showdown_id が引き継ぐ投入も止める(逆向きの検査。key の指す種族が
// 入れ替わって team-svc 等の保存済みデータが別の種族を指してしまうのを防ぐ)。DB は変えない。
func TestApplyRejectsSpeciesKeyHijack(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(ctx, conn, out, versions, now); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := dumpTables(t, conn)

	// "9002-000" は testleaf の key。testleafrain がそれを乗っ取る形に変える
	// (testleaf 自身の key は変えない = 前方向の検査には引っかからない)。
	hijacked := out
	hijacked.Species = append([]importer.SpeciesRow(nil), out.Species...)
	for i := range hijacked.Species {
		if hijacked.Species[i].ShowdownID == "testleafrain" {
			hijacked.Species[i].Key = "9002-000"
		}
	}
	err := importer.Apply(ctx, conn, hijacked, versions, now.Add(time.Hour))
	if !errors.Is(err, importer.ErrKeyChanged) {
		t.Fatalf("err = %v, want ErrKeyChanged", err)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("止めたのに DB が変わった")
	}
}

// 投入の途中で失敗したらトランザクションごと戻す(前の版のまま)。
func TestApplyRollsBackOnFailure(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(ctx, conn, out, versions, now); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := dumpTables(t, conn)

	broken := out
	broken.Learnsets = append(append([]importer.LearnsetRow(nil), out.Learnsets...), importer.LearnsetRow{SpeciesKey: "9001-000", MoveID: "testmissing"})
	changed := append([]importer.SourceVersion(nil), versions...)
	changed[0].Checksum = strings.Repeat("f", 64)
	if err := importer.Apply(ctx, conn, broken, changed, now.Add(time.Hour)); err == nil {
		t.Fatal("存在しない技を参照する習得技を受け付けた")
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("失敗したのに DB が変わった(ロールバックされていない)")
	}
}

// rekey は種族の key を付け替える(参照もすべて付け替える)。
func rekey(out importer.Output, from, to string, form int) importer.Output {
	next := out
	next.Species = append([]importer.SpeciesRow(nil), out.Species...)
	for i := range next.Species {
		if next.Species[i].Key == from {
			next.Species[i].Key = to
			next.Species[i].Form = form
		}
		if next.Species[i].BaseSpeciesKey == from {
			next.Species[i].BaseSpeciesKey = to
		}
	}
	next.Learnsets = append([]importer.LearnsetRow(nil), out.Learnsets...)
	for i := range next.Learnsets {
		if next.Learnsets[i].SpeciesKey == from {
			next.Learnsets[i].SpeciesKey = to
		}
	}
	next.RegulationSpecies = append([]importer.RegulationMemberRow(nil), out.RegulationSpecies...)
	for i := range next.RegulationSpecies {
		if next.RegulationSpecies[i].MemberID == from {
			next.RegulationSpecies[i].MemberID = to
		}
	}
	return next
}

// TestRunSchemaNotReady は migrate が済んでいない DB(data_versions が無い)で、Run / NewSQLStore が
// ErrSchemaNotReady を返し、テーブルを作らずに何も書かないこと(ADR-0104 §9)。
func TestRunSchemaNotReady(t *testing.T) {
	conn := freshImportDB(t) // Up まで済んだ DB。ここから全部戻して「migrate 前」にする
	dsn := os.Getenv("POKEDEX_TEST_DSN")
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := pokedexdb.DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll: %v", err)
	}
	out, versions := fixtureOutput(t)
	ctx := context.Background()

	if _, err := importer.NewSQLStore(conn).AppliedVersions(ctx); !errors.Is(err, importer.ErrSchemaNotReady) {
		t.Fatalf("AppliedVersions: err = %v, want ErrSchemaNotReady", err)
	}
	applied, err := importer.Run(ctx, conn, out, versions, time.Now().UTC(), true)
	if !errors.Is(err, importer.ErrSchemaNotReady) {
		t.Fatalf("Run: err = %v, want ErrSchemaNotReady", err)
	}
	if applied {
		t.Error("migrate 前の DB に投入したと返した")
	}
	var n int
	if err := conn.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name <> 'schema_migrations'",
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("migrate 前の DB にテーブルが %d 個できた(importer はテーブルを作らない)", n)
	}
}
