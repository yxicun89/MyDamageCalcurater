package db

// ファイルを読むだけの静的テスト(ADR-0211 §5・§6)。DB は使わない(make test で走る)。

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var migrationName = regexp.MustCompile(`^(\d{6})_([a-z0-9]+(?:_[a-z0-9]+)*)\.(up|down)\.sql$`)

// migrationPairs は migrations/ のファイルを版ごとにまとめる。規則に合わない名前は失敗にする。
func migrationPairs(t *testing.T) (versions []int, up, down map[int]string) {
	t.Helper()
	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatalf("migrations/ が読めない: %v", err)
	}
	up, down = map[int]string{}, map[int]string{}
	titles := map[int]string{}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("migrations/ にディレクトリがある: %s", e.Name())
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("ファイル名が NNNNNN_<snake>.(up|down).sql でない: %s", e.Name())
			continue
		}
		v, _ := strconv.Atoi(m[1])
		if prev, ok := titles[v]; ok && prev != m[2] {
			t.Errorf("版 %d の up と down の題名が違う: %s / %s", v, prev, m[2])
		}
		titles[v] = m[2]
		target := up
		if m[3] == "down" {
			target = down
		}
		if _, dup := target[v]; dup {
			t.Errorf("版 %d の %s が重複している", v, m[3])
		}
		target[v] = filepath.Join("migrations", e.Name())
	}
	for v := range titles {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions, up, down
}

func TestMigrationsAreNumberedPairs(t *testing.T) {
	versions, up, down := migrationPairs(t)
	if len(versions) == 0 {
		t.Fatal("migration が1つも無い")
	}
	for i, v := range versions {
		if v != i+1 {
			t.Fatalf("版は 1 から隙間なく並ぶこと: %v", versions)
		}
		if up[v] == "" {
			t.Errorf("版 %d に up が無い", v)
		}
		if down[v] == "" {
			t.Errorf("版 %d に down が無い(down の無い migration を作らない)", v)
		}
	}
}

func readAll(t *testing.T, paths map[int]string) string {
	t.Helper()
	var b strings.Builder
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return strings.ToLower(b.String())
}

// requiredTables は ADR-0211 §6(ADR-0209 §3 #5・#5b)のテーブル。業務テーブルは P5-3 で追加する。
var requiredTables = []string{"devices", "purge_journal"}

func TestMigrationsCreateAndDropRequiredTables(t *testing.T) {
	_, up, down := migrationPairs(t)
	upSQL, downSQL := readAll(t, up), readAll(t, down)
	for _, table := range requiredTables {
		create := regexp.MustCompile(`create\s+table\s+(if\s+not\s+exists\s+)?` + "`?" + table + "`?" + `\s*\(`)
		if !create.MatchString(upSQL) {
			t.Errorf("up に CREATE TABLE %s が無い", table)
		}
		drop := regexp.MustCompile(`drop\s+table\s+(if\s+exists\s+)?` + "`?" + table + "`?" + `\s*;`)
		if !drop.MatchString(downSQL) {
			t.Errorf("down に DROP TABLE %s が無い", table)
		}
	}
}

// TestMigrationsHaveNoData は migrations に実データを入れないこと。
func TestMigrationsHaveNoData(t *testing.T) {
	_, up, down := migrationPairs(t)
	for _, paths := range []map[int]string{up, down} {
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if regexp.MustCompile(`(?i)\b(insert|replace)\s+into\b`).Match(raw) {
				t.Errorf("%s: migrations に INSERT/REPLACE を書かない", p)
			}
		}
	}
}

// TestMigrationsDeclareKeyConstraints は ADR-0211 §6 の要の列・索引が up にあることを確かめる
// (効くことの確認は -tags tidb のテスト)。
func TestMigrationsDeclareKeyConstraints(t *testing.T) {
	_, up, _ := migrationPairs(t)
	upSQL := readAll(t, up)
	cases := []struct {
		name string
		re   string
	}{
		{"devices の主キーは device_id", `primary\s+key\s*\(\s*` + "`?" + `device_id`},
		{"devices は失効起点の orphaned_since を持つ(ADR-0209 §3 #5)", `orphaned_since\s+datetime`},
		{"devices.last_seen_at に索引", `idx_devices_last_seen_at`},
		{"devices.orphaned_since に索引", `idx_devices_orphaned_since`},
		{"purge_journal.device_id に索引(復元時の再適用用)", `idx_purge_journal_device_id`},
		{"purge_journal.requested_at に索引(期限切れ削除の範囲スキャン用)", `idx_purge_journal_requested_at`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !regexp.MustCompile(tc.re).MatchString(upSQL) {
				t.Fatalf("up に見当たらない: /%s/", tc.re)
			}
		})
	}
}

// TestMigrationsAreEmbedded は migrate が使う embed.FS が migrations/ と同じ集合であること。
func TestMigrationsAreEmbedded(t *testing.T) {
	_, up, down := migrationPairs(t)
	var onDisk []string
	for _, m := range []map[int]string{up, down} {
		for _, p := range m {
			onDisk = append(onDisk, filepath.Base(p))
		}
	}
	sort.Strings(onDisk)
	embedded, err := fs.Glob(Migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := range embedded {
		embedded[i] = filepath.Base(embedded[i])
	}
	sort.Strings(embedded)
	if strings.Join(embedded, ",") != strings.Join(onDisk, ",") {
		t.Fatalf("embed の集合 %v と migrations/ の集合 %v が違う", embedded, onDisk)
	}
}

// TestDownAllRequiresConfirmation は DB 名の確認が無い down を接続前に拒否すること。
// 到達できないアドレスを使い、接続を試みたら別のエラーになって落ちるようにしている。
func TestDownAllRequiresConfirmation(t *testing.T) {
	const dsn = "testuser:testpass@tcp(127.0.0.1:1)/team_test"
	// DB 名を持たない DSN(スキーム全体への接続になり、全データベースが対象になり得る)。
	const dsnWithoutDBName = "testuser:testpass@tcp(127.0.0.1:1)/"
	cases := []struct {
		name    string
		dsn     string
		confirm string
	}{
		{name: "確認なし", confirm: ""},
		{name: "別の DB 名", confirm: "team"},
		{name: "大文字小文字違い", confirm: "TEAM_TEST"},
		{name: "前方一致", confirm: "team_tes"},
		// DSN 側に DB 名が無いと「確認なし(空文字)」同士が一致してしまいかねない。
		{name: "DSN に DB 名が無い×確認なし", dsn: dsnWithoutDBName, confirm: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.dsn
			if d == "" {
				d = dsn
			}
			if err := DownAll(d, tc.confirm); !errors.Is(err, ErrDownNotConfirmed) {
				t.Fatalf("ErrDownNotConfirmed にならない: %v", err)
			}
		})
	}
}
