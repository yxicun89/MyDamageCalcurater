package db

// items.fling_power(なげつけるの威力。ADR-0144 §5)の静的な確認。DB は使わない。
//
//   - 000017: items.fling_power SMALLINT UNSIGNED NULL(既存の行がある DB に migrate できる。NULL = 投げられない・まだ取り込んでいない)・
//     CHECK (fling_power IS NULL OR fling_power > 0)。既定値で埋めない。down は CHECK を落としてから列を落とす。
//   - 既存の migration は書き換えない(ADR-0124)。
//   - 版は origin/main の最新(2026-10-10 時点で 000016)の次。マージ前に main を確かめ、先に別の版が入っていたら繰り下げる。
//   - 持ち物の投入(InsertItem)・一覧(ListItems。内部 API のマスタ)・検索(SearchItems。公開 API の Item)のクエリが fling_power を運ぶ。

import (
	"regexp"
	"strings"
	"testing"
)

const itemFlingPowerMigrationVersion = 17

func TestMigrationAddsItemFlingPower(t *testing.T) {
	_, up, down := migrationPairs(t)
	if up[itemFlingPowerMigrationVersion] == "" || down[itemFlingPowerMigrationVersion] == "" {
		t.Fatalf("migration %06d(items.fling_power。ADR-0144)が無い", itemFlingPowerMigrationVersion)
	}
	upSQL := readLower(t, up[itemFlingPowerMigrationVersion])
	if !regexp.MustCompile("alter\\s+table\\s+`?items`?\\s+add\\s+column\\s+`?fling_power`?\\s+smallint\\s+unsigned(\\s+null)?\\b").MatchString(upSQL) {
		t.Errorf("up に ALTER TABLE items ADD COLUMN fling_power SMALLINT UNSIGNED [NULL] が無い:\n%s", upSQL)
	}
	if regexp.MustCompile(`fling_power\s+smallint\s+unsigned[^,;]*not\s+null`).MatchString(upSQL) {
		t.Error("fling_power を NOT NULL にしない(既存の行がある DB で migrate が失敗する)")
	}
	if regexp.MustCompile(`fling_power\s+smallint\s+unsigned[^,;]*\bdefault\b`).MatchString(upSQL) {
		t.Error("fling_power に既定値を付けない(取り込み前の行が誤った威力を持つ)")
	}
	if !regexp.MustCompile(`check\s*\(\s*fling_power\s+is\s+null\s+or\s+fling_power\s*>\s*0\s*\)`).MatchString(upSQL) {
		t.Errorf("CHECK (fling_power IS NULL OR fling_power > 0) が無い:\n%s", upSQL)
	}
	downSQL := readLower(t, down[itemFlingPowerMigrationVersion])
	dropCheck := regexp.MustCompile(`drop\s+check\s+\w+`).FindStringIndex(downSQL)
	dropCol := regexp.MustCompile("drop\\s+column\\s+`?fling_power`?").FindStringIndex(downSQL)
	if dropCheck == nil || dropCol == nil || dropCheck[0] > dropCol[0] {
		t.Errorf("down は CHECK を落としてから fling_power の列を落とす:\n%s", downSQL)
	}
}

// TestItemFlingPowerMigrationIsNew は既存の migration を書き換えないこと(ADR-0124)。
func TestItemFlingPowerMigrationIsNew(t *testing.T) {
	versions, up, _ := migrationPairs(t)
	for _, v := range versions {
		if v >= itemFlingPowerMigrationVersion {
			continue
		}
		if strings.Contains(readLower(t, up[v]), "fling_power") {
			t.Errorf("既存の migration %06d に fling_power がある(新しい版で足すこと)", v)
		}
	}
}

func TestItemFlingPowerQueries(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	for _, name := range []string{"insertitem", "listitems", "searchitems"} {
		body := queryBody(t, raw, name)
		if !strings.Contains(body, "fling_power") {
			t.Errorf("クエリ %s が fling_power を運ばない:\n%s", name, body)
		}
	}
}
