//go:build mysql

package db

// 実 MySQL を使うテスト(ADR-0100 §5)。`make test-db` だけが実行する(`make test` には含めない)。
// POKEDEX_TEST_DSN が無い・DB に届かないときは**失敗**する(黙ってスキップして成功扱いにしない)。
// 全テーブルを消すので、DB 名が _test で終わる DSN だけを受け付ける。

import (
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// MySQL のエラー番号。
const (
	errDupEntry        = 1062
	errBadNull         = 1048
	errNoReferencedRow = 1452
	errCheckViolated   = 3819
	errInvalidJSON     = 3140
	errGeneratedColumn = 3105 // 生成列に値を入れた
)

// testDSN は POKEDEX_TEST_DSN を検証して返す。
func testDSN(t *testing.T) (dsn string, cfg *mysql.Config) {
	t.Helper()
	dsn = os.Getenv("POKEDEX_TEST_DSN")
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
	return dsn, cfg
}

// freshDB はスキーマを作り直した DB を返す(DownAll → Up)。
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn, cfg := testDSN(t)
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll: %v", err)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("Up: %v", err)
	}
	c := cfg.Clone()
	c.MultiStatements = true
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

func seed(t *testing.T, conn *sql.DB) {
	t.Helper()
	raw, err := os.ReadFile("testdata/example_seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(raw)); err != nil {
		t.Fatalf("example_seed.sql を流し込めない: %v", err)
	}
}

func userTables(t *testing.T, conn *sql.DB) []string {
	t.Helper()
	rows, err := conn.Query(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name <> 'schema_migrations' ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	return names
}

func TestMigrateUpDownUp(t *testing.T) {
	conn := freshDB(t)
	dsn, cfg := testDSN(t)

	versions, _, _ := migrationPairs(t)
	var version int
	var dirty bool
	if err := conn.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if version != versions[len(versions)-1] || dirty {
		t.Fatalf("version=%d dirty=%v, want %d / false", version, dirty, versions[len(versions)-1])
	}
	got := userTables(t, conn)
	for _, want := range requiredTables {
		if !contains(got, want) {
			t.Errorf("up の後にテーブル %s が無い: %v", want, got)
		}
	}

	if err := Up(dsn); err != nil {
		t.Fatalf("2回目の Up(変更なし)が失敗: %v", err)
	}
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll: %v", err)
	}
	if left := userTables(t, conn); len(left) != 0 {
		t.Fatalf("down の後に残ったテーブル: %v", left)
	}
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("スキーマが無い DB への DownAll は何もせず nil: %v", err)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("down の後の Up: %v", err)
	}
	seed(t, conn) // up → down → up の後もスキーマが元どおり
}

// TestMoveMechanismsDownWithRows は機構の行が入った DB でも down が通り、技を消すと機構も消えること
// (ADR-0121。行が入った状態の down が失敗した前例 #278 があるため、空の DB だけで確かめない)。
func TestMoveMechanismsDownWithRows(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM move_mechanisms WHERE move_id = 'testsplash'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("seed の機構の行数 = %d, err=%v, want 2", n, err)
	}

	tx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`DELETE FROM regulation_moves WHERE move_id = 'testsplash'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`DELETE FROM moves WHERE id = 'testsplash'`); err != nil {
		t.Fatalf("機構を持つ技を消せない: %v", err)
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM move_mechanisms WHERE move_id = 'testsplash'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("技を消した後の機構の行数 = %d, err=%v, want 0(ON DELETE CASCADE)", n, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	dsn, cfg := testDSN(t)
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("機構の行がある DB の DownAll: %v", err)
	}
	if left := userTables(t, conn); len(left) != 0 {
		t.Fatalf("down の後に残ったテーブル: %v", left)
	}
}

// TestDownAllWithSlot4Abilities は slot 4 の特性(Showdown の "S"。ADR-0103 §12。実データにある)が
// 入った DB でも DownAll が最後まで通り、dirty で止まらないこと(issue #278・ADR-0124)。
func TestDownAllWithSlot4Abilities(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	if _, err := conn.Exec(`INSERT INTO species_abilities (species_key, slot, ability_id) VALUES ('9001-000', 4, 'testguard')`); err != nil {
		t.Fatalf("slot 4 の行を入れられない: %v", err)
	}

	dsn, cfg := testDSN(t)
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("slot 4 の行がある DB の DownAll: %v", err)
	}
	if _, dirty, ok, err := Version(dsn); err != nil || ok || dirty {
		t.Fatalf("DownAll の後の版: ok=%v dirty=%v err=%v, want 未適用", ok, dirty, err)
	}
	if left := userTables(t, conn); len(left) != 0 {
		t.Fatalf("down の後に残ったテーブル: %v", left)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("down の後の Up: %v", err)
	}
}

func TestExampleSeedLoads(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM species`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("species の件数 %d, err=%v", n, err)
	}
}

// TestConstraintsRejectInvalidRows は ADR-0100 §3 の制約を DB が拒否すること。
// 各ケースは example_seed.sql を流した状態に対する1文。
func TestConstraintsRejectInvalidRows(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	cases := []struct {
		name  string
		stmt  string
		codes []uint16 // どれか1つに一致すればよい
	}{
		// type_chart
		{"倍率コード3", `INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES ('testice', 9, 'テストこおり', 'override');
			INSERT INTO type_chart (attack_type, defense_type, code) VALUES ('testice','fire',3)`, []uint16{errCheckViolated}},
		{"未知の攻撃タイプ", `INSERT INTO type_chart (attack_type, defense_type, code) VALUES ('testunknown','fire',2)`, []uint16{errNoReferencedRow}},
		{"組の重複", `INSERT INTO type_chart (attack_type, defense_type, code) VALUES ('fire','grass',2)`, []uint16{errDupEntry}},
		// types
		{"タイプ ID が大文字", `INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES ('Testbig', 10, 'テストおおもじ', 'override')`, []uint16{errCheckViolated}},
		{"タイプの sort_order 重複", `INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES ('testdup', 1, 'テストじゅうふく', 'override')`, []uint16{errDupEntry}},
		{"日本語名が空", `INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES ('testempty', 11, '', 'override')`, []uint16{errCheckViolated}},
		{"日本語名の出どころが未知", `INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES ('testsrc', 12, 'テストでどころ', 'testsource')`, []uint16{errCheckViolated}},
		// species
		{"未知のタイプ", speciesInsert("9003-000", 9003, 0, "testbad", "'testunknown'", "NULL", 0, "NULL", "NULL"), []uint16{errNoReferencedRow}},
		{"タイプ1とタイプ2が同じ", speciesInsert("9003-000", 9003, 0, "testbad", "'fire'", "'fire'", 0, "NULL", "NULL"), []uint16{errCheckViolated}},
		{"key と図鑑番号の不一致", speciesInsert("9003-000", 9004, 0, "testbad", "'fire'", "NULL", 0, "NULL", "NULL"), []uint16{errCheckViolated}},
		{"key の形式", speciesInsert("93-0", 93, 0, "testbad", "'fire'", "NULL", 0, "NULL", "NULL"), []uint16{errCheckViolated}},
		{"メガなのに持ち物が無い", speciesInsert("9001-002", 9001, 2, "testbad", "'fire'", "NULL", 1, "'9001-000'", "NULL"), []uint16{errCheckViolated}},
		{"メガなのに元の種族が無い", speciesInsert("9001-002", 9001, 2, "testbad", "'fire'", "NULL", 1, "NULL", "'teststone'"), []uint16{errCheckViolated}},
		{"メガでないのに持ち物がある", speciesInsert("9001-002", 9001, 2, "testbad", "'fire'", "NULL", 0, "NULL", "'teststone'"), []uint16{errCheckViolated}},
		{"メガの持ち物が未知", speciesInsert("9001-002", 9001, 2, "testbad", "'fire'", "NULL", 1, "'9001-000'", "'testnoitem'"), []uint16{errNoReferencedRow}},
		{"メガの元の種族が未知", speciesInsert("9001-002", 9001, 2, "testbad", "'fire'", "NULL", 1, "'9099-000'", "'teststone'"), []uint16{errNoReferencedRow}},
		{"メガの元の種族が自分", speciesInsert("9001-002", 9001, 2, "testbad", "'fire'", "NULL", 1, "'9001-002'", "'teststone'"), []uint16{errCheckViolated, errNoReferencedRow}},
		{"showdown_id の重複", speciesInsert("9003-000", 9003, 0, "testmon", "'fire'", "NULL", 0, "NULL", "NULL"), []uint16{errDupEntry}},
		{"種族値0", `INSERT INTO species (` + "`key`" + `, dex_no, form, showdown_id, name_ja, name_ja_source, name_en, type1, type2,
			base_hp, base_atk, base_def, base_spa, base_spd, base_spe, is_mega, base_species_key, required_item_id)
			VALUES ('9003-000', 9003, 0, 'testzero', 'テストゼロ', 'override', 'Testzero', 'fire', NULL, 0, 1, 1, 1, 1, 1, 0, NULL, NULL)`, []uint16{errCheckViolated}},
		{"特性スロット5", `INSERT INTO species_abilities (species_key, slot, ability_id) VALUES ('9002-000', 5, 'testhidden')`, []uint16{errCheckViolated}},
		{"同じ特性が2スロット", `INSERT INTO species_abilities (species_key, slot, ability_id) VALUES ('9002-000', 3, 'testguard')`, []uint16{errDupEntry}},
		// moves
		{"技 ID に大文字", `INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
			VALUES ('TestBig', 'テストおおもじ', 'override', 'Test Big', 'fire', 'special', 90, 100, 10, 0)`, []uint16{errCheckViolated}},
		{"技の分類が未知", `INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
			VALUES ('testcat', 'テストぶんるい', 'override', 'Test Cat', 'fire', 'testcategory', 90, 100, 10, 0)`, []uint16{errCheckViolated}},
		{"変化技に威力", `INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
			VALUES ('teststatus', 'テストへんか', 'override', 'Test Status', 'normal', 'status', 40, NULL, 10, 0)`, []uint16{errCheckViolated}},
		{"技のタイプが未知", `INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
			VALUES ('testtype', 'テストタイプ', 'override', 'Test Type', 'testunknown', 'special', 90, 100, 10, 0)`, []uint16{errNoReferencedRow}},
		{"優先度+6", `INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
			VALUES ('testprio', 'テストゆうせん', 'override', 'Test Prio', 'fire', 'special', 90, 100, 10, 6)`, []uint16{errCheckViolated}},
		// effects
		{"効果が壊れた JSON", `INSERT INTO item_effects (item_id, effect) VALUES ('testplain', '{"DamageMod":')`, []uint16{errInvalidJSON}},
		{"効果がオブジェクトでない", `INSERT INTO item_effects (item_id, effect) VALUES ('testplain', '[1]')`, []uint16{errCheckViolated}},
		{"効果の持ち物が未知", `INSERT INTO item_effects (item_id, effect) VALUES ('testnoitem', '{"DamageMod":5324}')`, []uint16{errNoReferencedRow}},
		// move_mechanisms(ADR-0121)
		{"機構の値が未知", `INSERT INTO move_mechanisms (move_id, mechanism) VALUES ('testflame', 'teleport')`, []uint16{errCheckViolated}},
		{"機構の値が大文字", `INSERT INTO move_mechanisms (move_id, mechanism) VALUES ('testflame', 'MULTI_HIT')`, []uint16{errCheckViolated}},
		{"機構の技が未知", `INSERT INTO move_mechanisms (move_id, mechanism) VALUES ('testnomove', 'multi_hit')`, []uint16{errNoReferencedRow}},
		{"同じ技に同じ機構が2行", `INSERT INTO move_mechanisms (move_id, mechanism) VALUES ('testsplash', 'multi_hit')`, []uint16{errDupEntry}},
		// regulations
		{"既定のレギュレーションが2件", `INSERT INTO regulations (id, name_ja, is_default) VALUES ('test-c', 'テストレギュレーションC', 1)`, []uint16{errDupEntry}},
		{"レギュレーション ID が大文字", `INSERT INTO regulations (id, name_ja, is_default) VALUES ('Test-D', 'テストレギュレーションD', 0)`, []uint16{errCheckViolated}},
		{"既定の印を直接入れる", `INSERT INTO regulations (id, name_ja, is_default, default_marker) VALUES ('test-e', 'テストレギュレーションE', 0, 1)`, []uint16{errGeneratedColumn}},
		{"集合に未知の種族", `INSERT INTO regulation_species (regulation_id, species_key) VALUES ('test-b', '9099-000')`, []uint16{errNoReferencedRow}},
		{"集合に未知のレギュレーション", `INSERT INTO regulation_items (regulation_id, item_id) VALUES ('test-z', 'testorb')`, []uint16{errNoReferencedRow}},
		// data_versions
		{"checksum が16進でない", `INSERT INTO data_versions (source, version, checksum, imported_at)
			VALUES ('testsource', 'v1', 'zz00000000000000000000000000000000000000000000000000000000000000', '2000-01-01 00:00:00')`, []uint16{errCheckViolated}},
		{"版が空", `INSERT INTO data_versions (source, version, checksum, imported_at)
			VALUES ('testsource', '', '0000000000000000000000000000000000000000000000000000000000000000', '2000-01-01 00:00:00')`, []uint16{errCheckViolated}},
		{"取得元の名前に大文字", `INSERT INTO data_versions (source, version, checksum, imported_at)
			VALUES ('TestSource', 'v1', '0000000000000000000000000000000000000000000000000000000000000000', '2000-01-01 00:00:00')`, []uint16{errCheckViolated}},
		{"取込日時が NULL", `INSERT INTO data_versions (source, version, checksum, imported_at)
			VALUES ('testnull', 'v1', '0000000000000000000000000000000000000000000000000000000000000000', NULL)`, []uint16{errBadNull}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := conn.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback() //nolint:errcheck // 失敗させるための文なので元に戻す
			// 前提の文(";\n" 区切り)は成功し、最後の1文だけが拒否されること
			stmts := strings.Split(tc.stmt, ";\n")
			for _, pre := range stmts[:len(stmts)-1] {
				if _, err := tx.Exec(pre); err != nil {
					t.Fatalf("前提の文が失敗: %v", err)
				}
			}
			_, err = tx.Exec(stmts[len(stmts)-1])
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

// TestSpeciesAbilitiesSlot4RoundTrip は species_abilities.slot=4(Showdown の "S"。ADR-0103 §12)の
// 挿入成功と読み戻しを検査する(issue #76)。TestConstraintsRejectInvalidRows のスロット5拒否・
// 特性重複拒否は負方向だけで、migration 000005 が広げた slot 4 の正方向を検査していなかった。
func TestSpeciesAbilitiesSlot4RoundTrip(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)

	// トランザクション内で確認しロールバックする(他のテストに slot=4 の行を残さない。
	// slot=4 の行が残った DB の DownAll は TestDownAllWithSlot4Abilities で確かめる)。
	tx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck // 確認用トランザクションなので必ず戻す

	// 9001-000 は seed で slot 1(testability)・slot 3(testhidden)しか使っていない。
	// 既存の特性 testguard を slot 4 として追加できること。
	if _, err := tx.Exec(`INSERT INTO species_abilities (species_key, slot, ability_id) VALUES ('9001-000', 4, 'testguard')`); err != nil {
		t.Fatalf("slot 4 の挿入が失敗(1..4 を許容する CHECK のはず): %v", err)
	}

	rows, err := tx.Query(`SELECT slot, ability_id FROM species_abilities WHERE species_key = '9001-000' ORDER BY slot`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []master.SpeciesAbilityRow
	for rows.Next() {
		var a master.SpeciesAbilityRow
		if err := rows.Scan(&a.Slot, &a.AbilityID); err != nil {
			t.Fatal(err)
		}
		got = append(got, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []master.SpeciesAbilityRow{
		{Slot: 1, AbilityID: "testability"},
		{Slot: 3, AbilityID: "testhidden"},
		{Slot: 4, AbilityID: "testguard"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("読み戻し = %+v, want %+v", got, want)
	}
}

// TestMegaItemCannotBeDeletedWhileReferenced はメガの持ち物が参照中は消せないこと(FK は RESTRICT。ADR-0100 §2)。
func TestMegaItemCannotBeDeletedWhileReferenced(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	_, err := conn.Exec(`DELETE FROM items WHERE id = 'teststone'`)
	var me *mysql.MySQLError
	if !errors.As(err, &me) || (me.Number != 1451 && me.Number != 3819) {
		t.Fatalf("参照中のメガストーンが消せてしまう: %v", err)
	}
}

// TestSeedMapsToEngineTypes は DB の行 → engine の型までを通す(ADR-0100 §4・§6)。
func TestSeedMapsToEngineTypes(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)

	var types []master.TypeRow
	rows, err := conn.Query(`SELECT id, sort_order, name_ja FROM types`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r master.TypeRow
		if err := rows.Scan(&r.ID, &r.SortOrder, &r.NameJa); err != nil {
			t.Fatal(err)
		}
		types = append(types, r)
	}
	rows.Close()
	var chartRows []master.TypeChartRow
	rows, err = conn.Query(`SELECT attack_type, defense_type, code FROM type_chart`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r master.TypeChartRow
		if err := rows.Scan(&r.AttackType, &r.DefenseType, &r.Code); err != nil {
			t.Fatal(err)
		}
		chartRows = append(chartRows, r)
	}
	rows.Close()
	chart, err := master.TypeChart(types, chartRows)
	if err != nil {
		t.Fatalf("TypeChart: %v", err)
	}
	if got := chart.Types(); !reflect.DeepEqual(got, []engine.Type{"fire", "water", "grass", "normal"}) {
		t.Fatalf("Types = %v(sort_order 順)", got)
	}

	var sr master.SpeciesRow
	var type2, baseKey, reqItem sql.NullString
	err = conn.QueryRow(`SELECT `+"`key`"+`, dex_no, form, showdown_id, name_ja, name_en, type1, type2,
		base_hp, base_atk, base_def, base_spa, base_spd, base_spe, is_mega, base_species_key, required_item_id
		FROM species WHERE `+"`key`"+` = '9001-001'`).Scan(&sr.Key, &sr.DexNo, &sr.Form, &sr.ShowdownID, &sr.NameJa, &sr.NameEn,
		&sr.Type1, &type2, &sr.BaseHP, &sr.BaseAtk, &sr.BaseDef, &sr.BaseSpA, &sr.BaseSpD, &sr.BaseSpe, &sr.IsMega, &baseKey, &reqItem)
	if err != nil {
		t.Fatal(err)
	}
	sr.Type2, sr.BaseSpeciesKey, sr.RequiredItemID = type2.String, baseKey.String, reqItem.String
	var abilities []master.SpeciesAbilityRow
	rows, err = conn.Query(`SELECT slot, ability_id FROM species_abilities WHERE species_key = '9001-001'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var a master.SpeciesAbilityRow
		if err := rows.Scan(&a.Slot, &a.AbilityID); err != nil {
			t.Fatal(err)
		}
		abilities = append(abilities, a)
	}
	rows.Close()
	sp, err := master.Species(sr, abilities, chart)
	if err != nil {
		t.Fatalf("Species: %v", err)
	}
	want := engine.Species{
		Key: "9001-001", DexNo: 9001, Form: 1, NameJa: "テストモン(メガ)",
		Types:     []engine.Type{"fire", "water"},
		BaseStats: engine.Stats{HP: 80, Atk: 110, Def: 90, SpA: 130, SpD: 95, Spe: 95},
		Abilities: []string{"testguard"},
	}
	if !reflect.DeepEqual(sp, want) {
		t.Fatalf("got %+v\nwant %+v", sp, want)
	}

	var effect []byte
	if err := conn.QueryRow(`SELECT effect FROM item_effects WHERE item_id = 'testorb'`).Scan(&effect); err != nil {
		t.Fatal(err)
	}
	item, err := master.Item(master.ItemRow{ID: "testorb", NameJa: "テストオーブ", Effect: effect}, chart)
	if err != nil || item.Effect == nil || item.Effect.DamageMod != 5324 {
		t.Fatalf("Item: %+v, err=%v(JSON 列の往復で 4096 基準の整数が変わらないこと)", item, err)
	}
}

func speciesInsert(key string, dex, form int, showdown, type1, type2 string, isMega int, baseKey, reqItem string) string {
	return `INSERT INTO species (` + "`key`" + `, dex_no, form, showdown_id, name_ja, name_ja_source, name_en, type1, type2,
		base_hp, base_atk, base_def, base_spa, base_spd, base_spe, is_mega, base_species_key, required_item_id) VALUES ('` +
		key + `', ` + strconv.Itoa(dex) + `, ` + strconv.Itoa(form) + `, '` + showdown + `', 'テストふせい', 'override', 'Testbad', ` +
		type1 + `, ` + type2 + `, 50, 50, 50, 50, 50, 50, ` + strconv.Itoa(isMega) + `, ` + baseKey + `, ` + reqItem + `)`
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
