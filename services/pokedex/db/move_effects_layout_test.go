package db

// move_effects 表(ADR-0107 決定5)の静的な確認。DB は使わない。
// item_effects / ability_effects とまったく同じ形にすること(主キー1列・JSON NOT NULL・
// 親への外部キー ON DELETE CASCADE・JSON_TYPE の CHECK)。

import (
	"regexp"
	"strings"
	"testing"
)

// readLower(1ファイルを小文字で読む)は natures_layout_test.go の同名ヘルパを使う。

func TestMigrationsCreateMoveEffects(t *testing.T) {
	_, up, down := migrationPairs(t)
	upSQL, downSQL := readAll(t, up), readAll(t, down)

	if !regexp.MustCompile("create\\s+table\\s+(if\\s+not\\s+exists\\s+)?`?move_effects`?\\s*\\(").MatchString(upSQL) {
		t.Fatal("up に CREATE TABLE move_effects が無い(ADR-0107 決定5)")
	}
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?move_effects`?\\s*;").MatchString(downSQL) {
		t.Error("down に DROP TABLE move_effects が無い")
	}
}

// TestMoveEffectsTableMirrorsItemEffects は move_effects が既存の効果表と同じ作りであることを見る。
func TestMoveEffectsTableMirrorsItemEffects(t *testing.T) {
	_, up, _ := migrationPairs(t)
	upSQL := readAll(t, up)

	start := strings.Index(upSQL, "create table move_effects")
	if start < 0 {
		t.Skip("move_effects がまだ無い(TestMigrationsCreateMoveEffects が先に失敗する)")
	}
	end := strings.Index(upSQL[start:], ";")
	if end < 0 {
		t.Fatal("move_effects の CREATE TABLE が ; で終わっていない")
	}
	ddl := upSQL[start : start+end]

	required := []struct{ name, pattern string }{
		{"move_id 列", `move_id\s+varchar`},
		{"effect 列は JSON NOT NULL", `effect\s+json\s+not\s+null`},
		{"主キーは move_id", "primary\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)"},
		{"moves への外部キー", "foreign\\s+key\\s*\\(\\s*`?move_id`?\\s*\\)\\s*references\\s+`?moves`?"},
		{"親が消えたら一緒に消す", `on\s+delete\s+cascade`},
		{"effect はオブジェクトだけ", `json_type\s*\(\s*effect\s*\)\s*=\s*'object'`},
	}
	for _, r := range required {
		if !regexp.MustCompile(r.pattern).MatchString(ddl) {
			t.Errorf("move_effects に %s が無い:\n%s", r.name, ddl)
		}
	}
}

// TestMoveEffectsMigrationIsNew は既存の migration を書き換えず、新しい版で足すこと(ADR-0100 §1)。
func TestMoveEffectsMigrationIsNew(t *testing.T) {
	versions, up, _ := migrationPairs(t)
	for _, v := range versions {
		if v > 6 {
			continue
		}
		if strings.Contains(readLower(t, up[v]), "move_effects") {
			t.Errorf("既存の migration %06d に move_effects がある(新しい版で足すこと)", v)
		}
	}
}

// TestMoveEffectsHasListQuery は sqlc のクエリに ListMoveEffects があること
// (ListItemEffects / ListAbilityEffects と同じ。export が引くため)。
func TestMoveEffectsHasListQuery(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	if !strings.Contains(raw, "-- name: listmoveeffects :many") {
		t.Error("query/pokedex.sql に ListMoveEffects :many が無い")
	}
	if !strings.Contains(raw, "from move_effects") {
		t.Error("query/pokedex.sql が move_effects を引いていない")
	}
}
