package db

// P5-4 の業務テーブル(ADR-0209 §3 #4・§5.2 の削除対象、ADR-0213 §3)が migrations に
// 入っていることの静的テスト。DB は使わない(make test で走る)。
//
// **test-first(ADR-0003)**: P5-1 は `devices` と `purge_journal` の2表までで、
// `teams` / `team_members` は P5-4 で追加する(migrations/000003〜)。
//
// テーブル名は requirements.md §6 と ADR-0209 §3 #4 の `teams` / `team_members` に固定する。

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// allUpSQL は migrations/*.up.sql を1つにつないで返す(順序は版の昇順)。
func allUpSQL(t *testing.T) string {
	t.Helper()
	versions, up, _ := migrationPairs(t)
	var b strings.Builder
	for _, v := range versions {
		data, err := os.ReadFile(up[v])
		if err != nil {
			t.Fatalf("%s を読めない: %v", up[v], err)
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}

// ADR-0209 §5.2 の削除順序 (2) team_members → (3) teams に対応する2表が存在すること。
func TestBusinessTablesExist(t *testing.T) {
	sql := allUpSQL(t)
	for _, table := range []string{"teams", "team_members"} {
		t.Run(table, func(t *testing.T) {
			re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+` + table + `\b`)
			if !re.MatchString(sql) {
				t.Errorf("migrations に CREATE TABLE %s が無い(ADR-0209 §3 #4・§5.2)", table)
			}
		})
	}
}

// ADR-0209 §6-1: すべての業務テーブルは device_id を持ち、device_id で引ける索引を持つ
// (端末で絞らないクエリを書けないようにするための最低条件)。
// `team_members` も device_id を持つ(teams 経由の JOIN でしか絞れない形にすると、
// §6-1 の「すべての読み書きが WHERE device_id を持つ」を保てない)。
func TestBusinessTablesAreKeyedByDevice(t *testing.T) {
	sql := allUpSQL(t)
	for _, table := range []string{"teams", "team_members"} {
		t.Run(table, func(t *testing.T) {
			body := tableBody(t, sql, table)
			if body == "" {
				t.Skipf("CREATE TABLE %s がまだ無い(TestBusinessTablesExist が報告する)", table)
			}
			if !regexp.MustCompile(`(?i)\bdevice_id\b`).MatchString(body) {
				t.Errorf("%s に device_id 列が無い(ADR-0209 §6-1)", table)
			}
			if !regexp.MustCompile(`(?i)(PRIMARY\s+KEY|KEY)\s*[a-z0-9_]*\s*\(\s*` + "`?" + `device_id`).MatchString(body) {
				t.Errorf("%s に device_id を先頭に持つ索引が無い(ADR-0209 §6-1・§6-3)", table)
			}
		})
	}
}

// ADR-0209 §4: 失効判定は `max(devices.last_seen_at, 行.updated_at)` で行うので、
// teams は行自体の updated_at を持つ(契約の Team.updatedAt と同じ値)。
func TestTeamsHaveTimestamps(t *testing.T) {
	sql := allUpSQL(t)
	body := tableBody(t, sql, "teams")
	if body == "" {
		t.Skip("CREATE TABLE teams がまだ無い")
	}
	for _, col := range []string{"created_at", "updated_at"} {
		if !regexp.MustCompile(`(?i)\b` + col + `\s+datetime`).MatchString(body) {
			t.Errorf("teams に %s(DATETIME)が無い(ADR-0209 §4 の失効判定・契約の Team)", col)
		}
	}
}

// ADR-0213 §3: メンバーはパーティの何番目か(slot)を持ち、1つの構築の中で重複しない。
// 上限6体は API 側で検証するが、DB にも「同じ slot が2行ある」状態を作らせない。
func TestTeamMembersHaveUniqueSlot(t *testing.T) {
	sql := allUpSQL(t)
	body := tableBody(t, sql, "team_members")
	if body == "" {
		t.Skip("CREATE TABLE team_members がまだ無い")
	}
	if !regexp.MustCompile(`(?i)\bslot\b`).MatchString(body) {
		t.Errorf("team_members に slot 列が無い(並び順を配列の添字から決める。ADR-0213 §3):\n%s", body)
	}
	if !regexp.MustCompile("(?i)(UNIQUE\\s+KEY|UNIQUE\\s*\\()[^\\n]*`?slot").MatchString(body) &&
		!regexp.MustCompile("(?i)PRIMARY\\s+KEY\\s*\\([^)]*`?slot").MatchString(body) {
		t.Errorf("team_members に (team_id, slot) の一意制約が無い(同じ枠の二重登録を DB でも防ぐ):\n%s", body)
	}
}

// tableBody は CREATE TABLE <name> ( ... ); の中身を返す(見つからなければ "")。
func tableBody(t *testing.T, sql, table string) string {
	t.Helper()
	re := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+` + table + `\s*\((.*?)\n\)\s*;`)
	m := re.FindStringSubmatch(sql)
	if m == nil {
		return ""
	}
	return m[1]
}
