package db

// move_rules 表(技の処理の定義。ADR-0143 §5)と species.weight_hg(種族の重さ)の静的な確認。DB は使わない。
//
//   - 000015: move_rules(move_id 主キー・moves への外部キー ON DELETE CASCADE・rule JSON NOT NULL・JSON_TYPE(rule) = 'OBJECT')。
//     item_effects と同じ形(定義は正規化せず JSON。検証は services/internal/master.DecodeMoveRule)。down は DROP TABLE。
//   - 000016: species.weight_hg SMALLINT UNSIGNED NULL(既存の行がある DB に migrate できる。NULL = まだ取り込んでいない)・
//     CHECK (weight_hg IS NULL OR weight_hg > 0)。既定値で埋めない。down は列を落とす。
//   - 既存の migration は書き換えない(ADR-0124)。
//   - 版は origin/main の最新(2026-10-09 時点で 000014)の次。マージ前に main を確かめ、先に別の版が入っていたら繰り下げる。

import (
	"regexp"
	"strings"
	"testing"
)

const (
	moveRulesMigrationVersion     = 15
	speciesWeightMigrationVersion = 16
)

func TestMigrationCreatesMoveRules(t *testing.T) {
	_, up, down := migrationPairs(t)
	if up[moveRulesMigrationVersion] == "" || down[moveRulesMigrationVersion] == "" {
		t.Fatalf("migration %06d(move_rules。ADR-0143)が無い", moveRulesMigrationVersion)
	}
	upSQL := readLower(t, up[moveRulesMigrationVersion])
	start := strings.Index(upSQL, "create table move_rules")
	if start < 0 {
		t.Fatalf("migration %06d に CREATE TABLE move_rules が無い", moveRulesMigrationVersion)
	}
	end := strings.Index(upSQL[start:], ") engine")
	if end < 0 {
		t.Fatal("move_rules の CREATE TABLE の終わりが見つからない")
	}
	ddl := upSQL[start : start+end]
	for _, r := range []struct{ name, pattern string }{
		{"move_id 列", `move_id\s+varchar\(64\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"rule 列", `rule\s+json\s+not\s+null`},
		{"主キーは move_id", "primary\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)"},
		{"moves への外部キー", "foreign\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)\\s*references\\s+`?moves`?\\s*\\(\\s*`?id`?\\s*\\)"},
		{"親が消えたら一緒に消す", `on\s+delete\s+cascade`},
		{"JSON はオブジェクト", `check\s*\(\s*json_type\(\s*rule\s*\)\s*=\s*'object'\s*\)`},
	} {
		if !regexp.MustCompile(r.pattern).MatchString(ddl) {
			t.Errorf("move_rules に %s が無い:\n%s", r.name, ddl)
		}
	}
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?move_rules`?\\s*;").MatchString(readLower(t, down[moveRulesMigrationVersion])) {
		t.Error("down に DROP TABLE move_rules が無い")
	}
}

func TestMigrationAddsSpeciesWeight(t *testing.T) {
	_, up, down := migrationPairs(t)
	if up[speciesWeightMigrationVersion] == "" || down[speciesWeightMigrationVersion] == "" {
		t.Fatalf("migration %06d(species.weight_hg。ADR-0143)が無い", speciesWeightMigrationVersion)
	}
	upSQL := readLower(t, up[speciesWeightMigrationVersion])
	if !regexp.MustCompile("alter\\s+table\\s+`?species`?\\s+add\\s+column\\s+`?weight_hg`?\\s+smallint\\s+unsigned(\\s+null)?\\b").MatchString(upSQL) {
		t.Errorf("up に ALTER TABLE species ADD COLUMN weight_hg SMALLINT UNSIGNED [NULL] が無い:\n%s", upSQL)
	}
	if regexp.MustCompile(`weight_hg\s+smallint\s+unsigned[^,;]*not\s+null`).MatchString(upSQL) {
		t.Error("weight_hg を NOT NULL にしない(既存の行がある DB で migrate が失敗する)")
	}
	if regexp.MustCompile(`weight_hg\s+smallint\s+unsigned[^,;]*\bdefault\b`).MatchString(upSQL) {
		t.Error("weight_hg に既定値を付けない(取り込み前の行が誤った重さを持つ)")
	}
	if !regexp.MustCompile(`check\s*\(\s*weight_hg\s+is\s+null\s+or\s+weight_hg\s*>\s*0\s*\)`).MatchString(upSQL) {
		t.Errorf("CHECK (weight_hg IS NULL OR weight_hg > 0) が無い:\n%s", upSQL)
	}
	if !regexp.MustCompile("drop\\s+column\\s+`?weight_hg`?").MatchString(readLower(t, down[speciesWeightMigrationVersion])) {
		t.Error("down に DROP COLUMN weight_hg が無い")
	}
}

// TestMoveRulesAndWeightMigrationsAreNew は既存の migration を書き換えないこと(ADR-0124)。
func TestMoveRulesAndWeightMigrationsAreNew(t *testing.T) {
	versions, up, _ := migrationPairs(t)
	for _, v := range versions {
		if v >= moveRulesMigrationVersion {
			continue
		}
		raw := readLower(t, up[v])
		if strings.Contains(raw, "move_rules") || strings.Contains(raw, "weight_hg") {
			t.Errorf("既存の migration %06d に move_rules / weight_hg がある(新しい版で足すこと)", v)
		}
	}
}

// TestMoveRulesAndWeightQueries は sqlc のクエリに move_rules の一覧・削除・投入があり、種族の投入・一覧が weight_hg を運ぶこと。
func TestMoveRulesAndWeightQueries(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	for _, name := range []string{
		"-- name: listmoverules :many",
		"-- name: deletemoverules :exec",
		"-- name: insertmoverule :exec",
	} {
		if !strings.Contains(raw, name) {
			t.Errorf("query/pokedex.sql に %q が無い", name)
		}
	}
	for _, name := range []string{"insertspecies", "listspecies", "getspeciesbykey"} {
		body := queryBody(t, raw, name) // natures_layout_test.go の補助(小文字化した本文と名前で引く)
		if !strings.Contains(body, "weight_hg") {
			t.Errorf("クエリ %s が weight_hg を運ばない:\n%s", name, body)
		}
	}
}
