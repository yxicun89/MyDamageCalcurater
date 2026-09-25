package db

// move_mechanisms 表(技の機構。ADR-0121)の静的な確認。DB は使わない。
// learnsets と同じ「親子の組を主キーにする」形: 主キー (move_id, mechanism)・moves への外部キー
// ON DELETE CASCADE・値の一覧を CHECK で固定(一覧は services/internal/master と一致させる)。

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/master"
)

func moveMechanismsDDL(t *testing.T) string {
	t.Helper()
	_, up, _ := migrationPairs(t)
	upSQL := readAll(t, up)
	start := strings.Index(upSQL, "create table move_mechanisms")
	if start < 0 {
		t.Fatal("up に CREATE TABLE move_mechanisms が無い(ADR-0121)")
	}
	end := strings.Index(upSQL[start:], ") engine")
	if end < 0 {
		t.Fatal("move_mechanisms の CREATE TABLE の終わりが見つからない")
	}
	return upSQL[start : start+end]
}

func TestMigrationsCreateMoveMechanisms(t *testing.T) {
	_, _, down := migrationPairs(t)
	ddl := moveMechanismsDDL(t)
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?move_mechanisms`?\\s*;").MatchString(readAll(t, down)) {
		t.Error("down に DROP TABLE move_mechanisms が無い")
	}
	required := []struct{ name, pattern string }{
		{"move_id 列", `move_id\s+varchar\(64\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"mechanism 列", `mechanism\s+varchar\(\d+\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"主キーは (move_id, mechanism)", "primary\\s+key\\s*\\(\\s*`?move_id`?\\s*,\\s*`?mechanism`?\\s*\\)"},
		{"moves への外部キー", "foreign\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)\\s*references\\s+`?moves`?\\s*\\(\\s*`?id`?\\s*\\)"},
		{"親が消えたら一緒に消す", `on\s+delete\s+cascade`},
	}
	for _, r := range required {
		if !regexp.MustCompile(r.pattern).MatchString(ddl) {
			t.Errorf("move_mechanisms に %s が無い:\n%s", r.name, ddl)
		}
	}
}

// TestMoveMechanismsCheckMatchesMaster は CHECK の値の一覧が services/internal/master の一覧と一致すること
// (片方だけ増やすと、importer が作る行を DB が拒否するか、DB に未知の値が入る)。
func TestMoveMechanismsCheckMatchesMaster(t *testing.T) {
	ddl := moveMechanismsDDL(t)
	m := regexp.MustCompile(`constraint\s+chk_move_mechanisms_mechanism\s+check\s*\(\s*mechanism\s+in\s*\(([^)]*)\)\s*\)`).FindStringSubmatch(ddl)
	if m == nil {
		t.Fatalf("chk_move_mechanisms_mechanism CHECK (mechanism IN (...)) が無い:\n%s", ddl)
	}
	var got []string
	for _, v := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(m[1], -1) {
		got = append(got, v[1])
	}
	sort.Strings(got)
	var want []string
	for _, v := range master.AllMoveMechanisms() {
		want = append(want, string(v))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CHECK の値 = %v,\nmaster の一覧 = %v", got, want)
	}
}

// TestMoveMechanismsMigrationIsNew は既存の migration を書き換えず、新しい版で足すこと(ADR-0100 §1)。
func TestMoveMechanismsMigrationIsNew(t *testing.T) {
	versions, up, _ := migrationPairs(t)
	for _, v := range versions {
		if v > 7 {
			continue
		}
		if strings.Contains(readLower(t, up[v]), "move_mechanisms") {
			t.Errorf("既存の migration %06d に move_mechanisms がある(新しい版で足すこと)", v)
		}
	}
}

// TestMoveMechanismsHasQueries は sqlc のクエリに一覧・削除・投入があること。
func TestMoveMechanismsHasQueries(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	for _, name := range []string{
		"-- name: listmovemechanisms :many",
		"-- name: deletemovemechanisms :exec",
		"-- name: insertmovemechanism :exec",
	} {
		if !strings.Contains(raw, name) {
			t.Errorf("query/pokedex.sql に %q が無い", name)
		}
	}
}
