package db

// move_flags 表(技のフラグ。ADR-0178)の静的な確認。DB は使わない。
// move_mechanisms(ADR-0121)と同じ「親子の組を主キーにする」形: 主キー (move_id, flag)・moves への外部キー
// ON DELETE CASCADE・値の一覧を CHECK で固定(一覧は services/internal/master と一致させる)。

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/master"
)

func moveFlagsDDL(t *testing.T) string {
	t.Helper()
	_, up, _ := migrationPairs(t)
	upSQL := readAll(t, up)
	start := strings.Index(upSQL, "create table move_flags")
	if start < 0 {
		t.Fatal("up に CREATE TABLE move_flags が無い(ADR-0178)")
	}
	end := strings.Index(upSQL[start:], ") engine")
	if end < 0 {
		t.Fatal("move_flags の CREATE TABLE の終わりが見つからない")
	}
	return upSQL[start : start+end]
}

func TestMigrationsCreateMoveFlags(t *testing.T) {
	_, _, down := migrationPairs(t)
	ddl := moveFlagsDDL(t)
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?move_flags`?\\s*;").MatchString(readAll(t, down)) {
		t.Error("down に DROP TABLE move_flags が無い")
	}
	required := []struct{ name, pattern string }{
		{"move_id 列", `move_id\s+varchar\(64\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"flag 列", `flag\s+varchar\(\d+\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"主キーは (move_id, flag)", "primary\\s+key\\s*\\(\\s*`?move_id`?\\s*,\\s*`?flag`?\\s*\\)"},
		{"moves への外部キー", "foreign\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)\\s*references\\s+`?moves`?\\s*\\(\\s*`?id`?\\s*\\)"},
		{"親が消えたら一緒に消す", `on\s+delete\s+cascade`},
	}
	for _, r := range required {
		if !regexp.MustCompile(r.pattern).MatchString(ddl) {
			t.Errorf("move_flags に %s が無い:\n%s", r.name, ddl)
		}
	}
}

// TestMoveFlagsCheckMatchesMaster は CHECK の値の一覧が services/internal/master(= engine)の一覧と一致すること。
func TestMoveFlagsCheckMatchesMaster(t *testing.T) {
	ddl := moveFlagsDDL(t)
	m := regexp.MustCompile(`constraint\s+chk_move_flags_flag\s+check\s*\(\s*flag\s+in\s*\(([^)]*)\)\s*\)`).FindStringSubmatch(ddl)
	if m == nil {
		t.Fatalf("chk_move_flags_flag CHECK (flag IN (...)) が無い:\n%s", ddl)
	}
	var got []string
	for _, v := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(m[1], -1) {
		got = append(got, v[1])
	}
	sort.Strings(got)
	var want []string
	for _, v := range master.AllMoveFlags() {
		want = append(want, string(v))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CHECK の値 = %v,\nmaster の一覧 = %v", got, want)
	}
}

// TestMoveFlagsMigrationIsNew は既存の migration を書き換えず、新しい版(000013)で足すこと(ADR-0124)。
func TestMoveFlagsMigrationIsNew(t *testing.T) {
	versions, up, _ := migrationPairs(t)
	for _, v := range versions {
		if v > 12 {
			continue
		}
		if strings.Contains(readLower(t, up[v]), "move_flags") {
			t.Errorf("既存の migration %06d に move_flags がある(新しい版で足すこと)", v)
		}
	}
}

// TestMoveFlagsHasQueries は sqlc のクエリに一覧・技の ID で絞る一覧・有無・削除・投入があること。
func TestMoveFlagsHasQueries(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	for _, name := range []string{
		"-- name: listmoveflags :many",
		"-- name: listmoveflagsbymoveids :many",
		"-- name: hasmoveflags :one",
		"-- name: deletemoveflags :exec",
		"-- name: insertmoveflag :exec",
	} {
		if !strings.Contains(raw, name) {
			t.Errorf("query/pokedex.sql に %q が無い", name)
		}
	}
}
