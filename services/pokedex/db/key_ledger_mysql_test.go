//go:build mysql

package db

// species_key_ledger(ADR-0131)の制約と権限を実 MySQL で確かめる。`make test-db` だけが実行する。

import (
	"testing"
)

const insertLedger = "INSERT INTO species_key_ledger (species_key, showdown_id, first_seen_at) VALUES "

// 1つの key に showdown_id は1つ、1つの showdown_id に key は1つ。species を消しても台帳は残る。
func TestKeyLedgerConstraints(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	if _, err := conn.Exec(insertLedger + `('9001-000', 'testmon', '2026-10-01 00:00:00'), ('9002-000', 'testleaf', '2026-10-01 00:00:00')`); err != nil {
		t.Fatalf("正しい行を入れられない: %v", err)
	}
	cases := []struct {
		name string
		sql  string
		want uint16
	}{
		{"同じ key を別の showdown_id に", insertLedger + `('9001-000', 'testother', '2026-10-01 00:00:00')`, errDupEntry},
		{"同じ showdown_id を別の key に", insertLedger + `('9001-009', 'testmon', '2026-10-01 00:00:00')`, errDupEntry},
		{"first_seen_at が無い", `INSERT INTO species_key_ledger (species_key, showdown_id, first_seen_at) VALUES ('9003-000', 'testnew', NULL)`, errBadNull},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conn.Exec(tc.sql)
			if got := mysqlErrNumber(err); got != tc.want {
				t.Fatalf("err = %v(番号 %d), want %d", err, got, tc.want)
			}
		})
	}

	// species に無い key も持てる(消えた種族を覚えておくため。外部キーが無い)。
	if _, err := conn.Exec(insertLedger + `('9009-000', 'testgone', '2026-10-01 00:00:00')`); err != nil {
		t.Fatalf("species に無い key を入れられない(外部キーがある?): %v", err)
	}
	// species を全部消しても(importer の全置換と同じ)台帳の行は残る。子の表の行を先に消すのは、
	// 孤児の行が残ると、次のテストの DownAll(migration 000005 の down が外部キーを張り直す)が失敗するため。
	if _, err := conn.Exec(`DELETE FROM species_abilities; DELETE FROM learnsets; DELETE FROM regulation_species; SET FOREIGN_KEY_CHECKS = 0; DELETE FROM species; SET FOREIGN_KEY_CHECKS = 1`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM species_key_ledger`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("species を消した後の台帳 = %d 行, want 3", n)
	}
}

// 台帳は migrate up の後の Provision で importer が追記・参照でき、reader は参照だけできる
// (ADR-0110・ADR-0125。cmd/migrate が up の後に importer の権限を付け直す)。
func TestKeyLedgerGrants(t *testing.T) {
	admin := freshDB(t)
	dropTestUsers(t, admin)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	im := mustOpenAs(t, roles.importer)
	_, err := im.Exec(insertLedger + `('9001-000', 'testmon', '2026-10-01 00:00:00')`)
	expectOK(t, "importer の台帳への INSERT", err)
	var n int
	expectOK(t, "importer の台帳の SELECT", im.QueryRow(`SELECT COUNT(*) FROM species_key_ledger`).Scan(&n))
	if n != 1 {
		t.Errorf("importer が入れた台帳の行が見えない: %d", n)
	}

	r := mustOpenAs(t, roles.reader)
	expectOK(t, "reader の台帳の SELECT", r.QueryRow(`SELECT COUNT(*) FROM species_key_ledger`).Scan(&n))
	_, err = r.Exec(insertLedger + `('9002-000', 'testleaf', '2026-10-01 00:00:00')`)
	expectDenied(t, "reader の台帳への INSERT", err)
	_, err = r.Exec(`DELETE FROM species_key_ledger`)
	expectDenied(t, "reader の台帳の DELETE", err)
	assertImporterGrants(t, admin, cfg.DBName)
}
