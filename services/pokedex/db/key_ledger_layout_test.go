package db

// species_key_ledger 表(配った種族 key と showdown_id の対応の台帳。issue #277・ADR-0131)の静的な確認。
// DB は使わない。効くことの確認は key_ledger_mysql_test.go(-tags mysql)。
//
// 台帳は species を全置換しても残る(消えた種族の key も覚えておき、後の再利用を止めるため)。
// そのため species への外部キーを持たない。importer の経路(sqlc のクエリ)からは追記と参照だけにする。

import (
	"regexp"
	"strings"
	"testing"
)

// keyLedgerMigrationVersion は台帳を足す migration の版(既存の 000001〜000008 は書き換えない。ADR-0100 §1)。
const keyLedgerMigrationVersion = 9

func keyLedgerDDL(t *testing.T) string {
	t.Helper()
	_, up, _ := migrationPairs(t)
	path, ok := up[keyLedgerMigrationVersion]
	if !ok {
		t.Fatalf("migration %06d が無い(台帳は新しい版で足す。ADR-0131)", keyLedgerMigrationVersion)
	}
	upSQL := readLower(t, path)
	start := strings.Index(upSQL, "create table species_key_ledger")
	if start < 0 {
		t.Fatalf("%s に CREATE TABLE species_key_ledger が無い", path)
	}
	end := strings.Index(upSQL[start:], ") engine")
	if end < 0 {
		t.Fatal("species_key_ledger の CREATE TABLE の終わりが見つからない")
	}
	return upSQL[start : start+end]
}

func TestMigrationsCreateKeyLedger(t *testing.T) {
	_, _, down := migrationPairs(t)
	ddl := keyLedgerDDL(t)
	downPath, ok := down[keyLedgerMigrationVersion]
	if !ok {
		t.Fatalf("migration %06d の down が無い", keyLedgerMigrationVersion)
	}
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?species_key_ledger`?\\s*;").MatchString(readLower(t, downPath)) {
		t.Error("down に DROP TABLE species_key_ledger が無い")
	}
	required := []struct{ name, pattern string }{
		// species.key と同じ型(CHAR(8) ascii_bin)。
		{"species_key 列", `species_key\s+char\(8\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		// species.showdown_id と同じ型。
		{"showdown_id 列", `showdown_id\s+varchar\(64\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"first_seen_at 列", `first_seen_at\s+datetime\s+not\s+null`},
		{"主キーは species_key", "primary\\s+key\\s*\\(\\s*`?species_key`?\\s*\\)"},
		// 1つの showdown_id に key は1つ(消えた種族が別の key で戻るのも止める)。
		{"showdown_id は一意", "unique\\s+key\\s+\\w+\\s*\\(\\s*`?showdown_id`?\\s*\\)"},
	}
	for _, r := range required {
		if !regexp.MustCompile(r.pattern).MatchString(ddl) {
			t.Errorf("species_key_ledger に %s が無い:\n%s", r.name, ddl)
		}
	}
	// species を全置換しても行を残すため、species(や他の表)への外部キーを持たない。
	if regexp.MustCompile(`foreign\s+key|references\s`).MatchString(ddl) {
		t.Errorf("species_key_ledger が外部キーを持っている(species の全置換で行を失う・削除を妨げる):\n%s", ddl)
	}
}

// TestKeyLedgerMigrationIsNew は既存の migration を書き換えず、新しい版で足すこと(ADR-0100 §1)。
func TestKeyLedgerMigrationIsNew(t *testing.T) {
	versions, up, down := migrationPairs(t)
	for _, v := range versions {
		if v >= keyLedgerMigrationVersion {
			continue
		}
		for _, p := range []string{up[v], down[v]} {
			if strings.Contains(readLower(t, p), "species_key_ledger") {
				t.Errorf("既存の migration %s に species_key_ledger がある(新しい版で足すこと)", p)
			}
		}
	}
}

// TestKeyLedgerQueriesAreAppendOnly は sqlc のクエリに台帳の参照と追記があり、削除・更新が無いこと
// (台帳から行が消えると、消えた key の再利用を止められなくなる)。
func TestKeyLedgerQueriesAreAppendOnly(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	if !regexp.MustCompile(`(?s)-- name: \w+ :many\s+select[^;]*from\s+species_key_ledger`).MatchString(raw) {
		t.Error("query/pokedex.sql に species_key_ledger を読むクエリが無い")
	}
	if !regexp.MustCompile(`(?s)-- name: \w+ :exec\s+insert[^;]*into\s+species_key_ledger`).MatchString(raw) {
		t.Error("query/pokedex.sql に species_key_ledger へ追記するクエリが無い")
	}
	for _, bad := range []*regexp.Regexp{
		regexp.MustCompile(`delete\s+from\s+species_key_ledger`),
		regexp.MustCompile(`update\s+species_key_ledger`),
		regexp.MustCompile(`truncate\s+(table\s+)?species_key_ledger`),
		regexp.MustCompile(`replace\s+into\s+species_key_ledger`),
		regexp.MustCompile(`on\s+duplicate\s+key\s+update[^;]*first_seen_at`),
	} {
		if bad.MatchString(raw) {
			t.Errorf("query/pokedex.sql に台帳を消す・書き換えるクエリがある: /%s/", bad)
		}
	}
}
