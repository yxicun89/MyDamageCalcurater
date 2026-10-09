package db

// move_mechanism_params 表(技の機構の中身。ADR-0142 §7)の静的な確認。DB は使わない。
// 1技1行(主キー move_id)・moves への外部キー ON DELETE CASCADE・列はすべて NULL 可(中身の無い項目)・
// 能力値と攻撃に使うポケモンの値は CHECK で固定する。既存の migration は書き換えず 000014 で足す。

import (
	"regexp"
	"strings"
	"testing"
)

// moveMechanismParamsMigrationVersion は move_mechanism_params を作る migration の版(000013 move_flags の次)。
const moveMechanismParamsMigrationVersion = 14

func moveMechanismParamsDDL(t *testing.T) (ddl, down string) {
	t.Helper()
	_, up, downs := migrationPairs(t)
	if up[moveMechanismParamsMigrationVersion] == "" || downs[moveMechanismParamsMigrationVersion] == "" {
		t.Fatalf("migration %06d(move_mechanism_params。ADR-0142)が無い", moveMechanismParamsMigrationVersion)
	}
	upSQL := readLower(t, up[moveMechanismParamsMigrationVersion])
	start := strings.Index(upSQL, "create table move_mechanism_params")
	if start < 0 {
		t.Fatalf("migration %06d に CREATE TABLE move_mechanism_params が無い", moveMechanismParamsMigrationVersion)
	}
	end := strings.Index(upSQL[start:], ") engine")
	if end < 0 {
		t.Fatal("move_mechanism_params の CREATE TABLE の終わりが見つからない")
	}
	return upSQL[start : start+end], readLower(t, downs[moveMechanismParamsMigrationVersion])
}

func TestMigrationCreatesMoveMechanismParams(t *testing.T) {
	ddl, down := moveMechanismParamsDDL(t)
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?move_mechanism_params`?\\s*;").MatchString(down) {
		t.Error("down に DROP TABLE move_mechanism_params が無い")
	}
	required := []struct{ name, pattern string }{
		{"move_id 列", `move_id\s+varchar\(64\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"主キーは move_id", "primary\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)"},
		{"moves への外部キー", "foreign\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)\\s*references\\s+`?moves`?\\s*\\(\\s*`?id`?\\s*\\)"},
		{"親が消えたら一緒に消す", `on\s+delete\s+cascade`},
		{"多段の最小", `multi_hit_min\s+tinyint`},
		{"多段の最大", `multi_hit_max\s+tinyint`},
		{"固定ダメージ(レベル)", `fixed_damage_level\s+(bool|boolean|tinyint\(1\))`},
		{"固定ダメージ(数値)", `fixed_damage_value\s+smallint`},
		{"一撃必殺", `ohko\s+(bool|boolean|tinyint\(1\))`},
		{"一撃必殺が効かないタイプ", `ohko_immune_type\s+varchar\(\d+\)`},
		{"攻撃に使う能力値", `offense_stat\s+varchar\(\d+\)`},
		{"攻撃に使うポケモン", `offense_pokemon\s+varchar\(\d+\)`},
		{"防御に使う能力値", `defense_stat\s+varchar\(\d+\)`},
	}
	for _, r := range required {
		if !regexp.MustCompile(r.pattern).MatchString(ddl) {
			t.Errorf("move_mechanism_params に %s が無い:\n%s", r.name, ddl)
		}
	}
	// 能力値は HP を含まない5種、攻撃に使うポケモンは attacker / defender(engine の語彙。ADR-0142 §2)。
	checks := []struct{ col, values string }{
		{"offense_stat", `'atk',\s*'def',\s*'spa',\s*'spd',\s*'spe'`},
		{"defense_stat", `'atk',\s*'def',\s*'spa',\s*'spd',\s*'spe'`},
		{"offense_pokemon", `'attacker',\s*'defender'`},
	}
	for _, c := range checks {
		if !regexp.MustCompile(`check\s*\(\s*` + c.col + `\s+in\s*\(\s*` + c.values + `\s*\)\s*\)`).MatchString(ddl) {
			t.Errorf("%s の CHECK (%s IN (...)) が無い:\n%s", c.col, c.col, ddl)
		}
	}
}

// TestMoveMechanismParamsMigrationIsNew は既存の migration を書き換えないこと(ADR-0100 §1)。
func TestMoveMechanismParamsMigrationIsNew(t *testing.T) {
	versions, up, _ := migrationPairs(t)
	for _, v := range versions {
		if v >= moveMechanismParamsMigrationVersion {
			continue
		}
		if strings.Contains(readLower(t, up[v]), "move_mechanism_params") {
			t.Errorf("既存の migration %06d に move_mechanism_params がある(新しい版で足すこと)", v)
		}
	}
}

// TestMoveMechanismParamsHasQueries は sqlc のクエリに一覧・削除・投入があること。
func TestMoveMechanismParamsHasQueries(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	for _, name := range []string{
		"-- name: listmovemechanismparams :many",
		"-- name: deletemovemechanismparams :exec",
		"-- name: insertmovemechanismparams :exec",
	} {
		if !strings.Contains(raw, name) {
			t.Errorf("query/pokedex.sql に %q が無い", name)
		}
	}
}
