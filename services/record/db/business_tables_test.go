package db

// P5-3 の業務テーブル(ADR-0209 §3 #1〜#3・§5.2 の削除対象)が migrations に入っていることの
// 静的テスト。DB は使わない(make test で走る)。
//
// **test-first(ADR-0003)**: P5-1 は `devices` と `purge_journal` の2表までで、
// `calc_events` / 集計 / `favorites` は P5-3 で追加した(migrations/000003〜000005)。
//
// テーブル名は、集計を返す API(`GET /api/record/frequent-opponents`)に合わせて
// `frequent_opponents` に固定する(契約の名前とスキーマの名前を揃えて追いやすくするため)。

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

// ADR-0209 §5.2 の削除順序 (2) calc_events → (3) 集計 → (4) favorites に対応する3表が存在すること。
func TestBusinessTablesExist(t *testing.T) {
	sql := allUpSQL(t)
	for _, table := range []string{"calc_events", "frequent_opponents", "favorites"} {
		t.Run(table, func(t *testing.T) {
			re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+` + table + `\b`)
			if !re.MatchString(sql) {
				t.Errorf("migrations に CREATE TABLE %s が無い(ADR-0209 §3・§5.2)", table)
			}
		})
	}
}

// ADR-0209 §6-1: すべての業務テーブルは device_id を持ち、device_id で引ける索引を持つ
// (端末で絞らないクエリを書けないようにするための最低条件)。
func TestBusinessTablesAreKeyedByDevice(t *testing.T) {
	sql := allUpSQL(t)
	for _, table := range []string{"calc_events", "frequent_opponents", "favorites"} {
		t.Run(table, func(t *testing.T) {
			body := tableBody(t, sql, table)
			if body == "" {
				t.Skipf("CREATE TABLE %s がまだ無い(TestBusinessTablesExist が報告する)", table)
			}
			if !regexp.MustCompile(`(?i)\bdevice_id\b`).MatchString(body) {
				t.Errorf("%s に device_id 列が無い(ADR-0209 §6-1)", table)
			}
			// PRIMARY KEY か KEY の先頭が device_id であること(端末内の検索が索引で閉じる)。
			if !regexp.MustCompile(`(?i)(PRIMARY\s+KEY|KEY)\s*[a-z0-9_]*\s*\(\s*` + "`?" + `device_id`).MatchString(body) {
				t.Errorf("%s に device_id を先頭に持つ索引が無い(ADR-0209 §6-1・§6-3)", table)
			}
		})
	}
}

// ADR-0209 §3 #3 の設計上の制約: favorites は calc_events を外部キーで参照しない
// (保持期間が 90日 < 540日 で矛盾するため。ピン留め時点のスナップショットを自分で持つ)。
func TestFavoritesDoNotReferenceCalcEvents(t *testing.T) {
	sql := allUpSQL(t)
	body := tableBody(t, sql, "favorites")
	if body == "" {
		t.Skip("CREATE TABLE favorites がまだ無い")
	}
	if regexp.MustCompile(`(?i)(FOREIGN\s+KEY|REFERENCES)`).MatchString(body) {
		t.Errorf("favorites が外部キーを持っている(ADR-0209 §3 #3: calc_events を参照しない):\n%s", body)
	}
}

// ADR-0212 §6 / 新設 AC-R7: calc_events は重複排除キーに一意制約を持つ
// (at-least-once 配送で同じイベントが2回届いても2行にならない)。
func TestCalcEventsHasUniqueEventID(t *testing.T) {
	sql := allUpSQL(t)
	body := tableBody(t, sql, "calc_events")
	if body == "" {
		t.Skip("CREATE TABLE calc_events がまだ無い")
	}
	if !regexp.MustCompile("(?i)(UNIQUE\\s+KEY|UNIQUE\\s*\\()[^\\n]*`?event_id").MatchString(body) &&
		!regexp.MustCompile("(?i)PRIMARY\\s+KEY\\s*\\([^)]*`?event_id").MatchString(body) {
		t.Errorf("calc_events に event_id の一意制約が無い(再配送で二重保存になる。ADR-0212 §6):\n%s", body)
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

// ADR-0227 §2(P5-3c): お気に入りの二重作成を DB で防ぐため、favorites は内容のハッシュ
// snapshot_hash(正規化した snapshot の SHA-256。16進64文字)を持ち、(device_id, snapshot_hash) に
// 一意制約を張る。同時に同じ要求が2つ来ても行は1つになる(アプリ側の「先に探してから入れる」だけでは
// 競合ですり抜ける)。既存の 000005 は書き換えず、新しい版の migration で足す(適用済みの版を変えない)。
func TestFavoritesHasUniqueSnapshotHashPerDevice(t *testing.T) {
	sql := allUpSQL(t)
	if !regexp.MustCompile(`(?is)ALTER\s+TABLE\s+favorites\b.*?\bsnapshot_hash\s+(CHAR|BINARY|VARCHAR)\s*\(\s*64\s*\)`).MatchString(sql) {
		t.Errorf("favorites に snapshot_hash CHAR(64) を足す migration が無い(ADR-0227 §2)")
	}
	if !regexp.MustCompile("(?is)UNIQUE\\s+(KEY|INDEX)\\s+`?[a-z0-9_]*`?\\s*\\(\\s*`?device_id`?\\s*,\\s*`?snapshot_hash`?\\s*\\)").MatchString(sql) {
		t.Errorf("favorites に UNIQUE (device_id, snapshot_hash) が無い(同じ内容の二重作成を DB で防ぐ。ADR-0227 §2)")
	}
	// 000005(P5-3 で適用済み)の CREATE TABLE favorites は書き換えない(適用済みの版を変えると
	// 既存の DB と migration の履歴がずれる)。
	if body := tableBody(t, sql, "favorites"); regexp.MustCompile(`(?i)snapshot_hash`).MatchString(body) {
		t.Errorf("snapshot_hash を 000005 の CREATE TABLE に直接足している(新しい版で ALTER TABLE する)")
	}
}
