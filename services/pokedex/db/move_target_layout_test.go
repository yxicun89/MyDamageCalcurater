package db

// moves.target(技の対象。ADR-0136・issue #288)の静的な確認。DB は使わない。
//
// 1つの技の対象は1つなので、move_mechanisms のような子表ではなく moves の列にする。
// 既存の行がある DB にも migrate できるよう NULL を許す(NULL = まだ importer が入れていない)。
// 既定値で埋めない(「単体」等の既定値は誤ったデータを黙って作る)。importer は必ず値を入れる
// (-tags mysql のテストで確かめる)。値の一覧は CHECK で固定し、services/internal/master の
// AllMoveTargets と一致させる。値は Showdown の文字列のまま(大文字小文字を区別するので、
// 小文字化する readAll/readLower ではなく元のファイルを読む)。

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/master"
)

// moveTargetMigrationVersion は moves.target を足す migration の版(000009 の次)。
const moveTargetMigrationVersion = 10

func readRaw(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s が読めない: %v", path, err)
	}
	return string(raw)
}

func moveTargetMigration(t *testing.T) (upSQL, downSQL string) {
	t.Helper()
	_, up, down := migrationPairs(t)
	if up[moveTargetMigrationVersion] == "" || down[moveTargetMigrationVersion] == "" {
		t.Fatalf("migration %06d(moves.target を足す。ADR-0136)が無い", moveTargetMigrationVersion)
	}
	return readRaw(t, up[moveTargetMigrationVersion]), readRaw(t, down[moveTargetMigrationVersion])
}

func TestMigrationAddsMoveTargetColumn(t *testing.T) {
	upSQL, downSQL := moveTargetMigration(t)
	// 列: ID 列と同じ ascii_bin(大文字小文字を区別する)。NULL を許す(既存の行がある DB への migrate を通す)。
	column := regexp.MustCompile(`(?i)alter\s+table\s+` + "`?moves`?" + `\s+add\s+column\s+` + "`?target`?" +
		`\s+varchar\(\d+\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin(\s+null)?(\s+after\s+\S+)?\s*[,;]`)
	if !column.MatchString(upSQL) {
		t.Errorf("up に ALTER TABLE moves ADD COLUMN target VARCHAR(n) CHARACTER SET ascii COLLATE ascii_bin [NULL] が無い:\n%s", upSQL)
	}
	if regexp.MustCompile(`(?i)\btarget\s+varchar\(\d+\)[^,;]*not\s+null`).MatchString(upSQL) {
		t.Error("target を NOT NULL にしない(既存の行がある DB で migrate が失敗する。ADR-0136 §1)")
	}
	if regexp.MustCompile(`(?i)\btarget\s+varchar\(\d+\)[^,;]*\bdefault\b`).MatchString(upSQL) {
		t.Error("target に既定値を付けない(取り込み前の行が誤った対象を持つ。ADR-0136 §1)")
	}
	if !regexp.MustCompile(`(?i)drop\s+column\s+` + "`?target`?").MatchString(downSQL) {
		t.Errorf("down に DROP COLUMN target が無い:\n%s", downSQL)
	}
}

// TestMoveTargetCheckMatchesMaster は CHECK の値の一覧が services/internal/master の一覧と一致すること
// (片方だけ増やすと、importer が作る行を DB が拒否するか、DB に未知の値が入る)。
func TestMoveTargetCheckMatchesMaster(t *testing.T) {
	upSQL, _ := moveTargetMigration(t)
	m := regexp.MustCompile(`(?is)constraint\s+chk_moves_target\s+check\s*\(\s*target\s+in\s*\(([^)]*)\)\s*\)`).FindStringSubmatch(upSQL)
	if m == nil {
		t.Fatalf("CONSTRAINT chk_moves_target CHECK (target IN (...)) が無い:\n%s", upSQL)
	}
	var got []string
	for _, v := range regexp.MustCompile(`'([A-Za-z]+)'`).FindAllStringSubmatch(m[1], -1) {
		got = append(got, v[1])
	}
	sort.Strings(got)
	var want []string
	for _, v := range master.AllMoveTargets() {
		want = append(want, string(v))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CHECK の値 = %v,\nmaster の一覧 = %v", got, want)
	}
}

// TestMoveTargetMigrationIsNew は既存の migration を書き換えず、新しい版で足すこと(ADR-0100 §1・ADR-0124)。
func TestMoveTargetMigrationIsNew(t *testing.T) {
	versions, up, down := migrationPairs(t)
	for _, v := range versions {
		if v >= moveTargetMigrationVersion {
			continue
		}
		for _, p := range []string{up[v], down[v]} {
			if strings.Contains(readLower(t, p), "chk_moves_target") ||
				regexp.MustCompile(`(?i)add\s+column\s+`+"`?target`?").MatchString(readRaw(t, p)) {
				t.Errorf("既存の migration %s に moves.target がある(新しい版で足すこと)", p)
			}
		}
	}
}

// TestMoveTargetQueries は sqlc のクエリが対象を読み書きすること。
//   - InsertMove は target を入れる(importer の投入)
//   - ListMoves は target を返す(内部 API の export・read model が使う。列が moves の全列と一致しないと
//     sqlc が store.Move でなく別の行型を生成し、storetest・readmodel・httpapi が壊れる)
//
// 公開 API の技の応答(GetMove・GetMovesByIDs・SearchMoves)に出すかは契約の変更で、API レーンが決める。
func TestMoveTargetQueries(t *testing.T) {
	raw := readLower(t, "query/pokedex.sql")
	for _, name := range []string{"listmoves", "insertmove"} {
		header := "-- name: " + name + " "
		start := strings.Index(raw, header)
		if start < 0 {
			t.Fatalf("query/pokedex.sql に %q が無い", header)
		}
		end := strings.Index(raw[start:], ";")
		if end < 0 {
			t.Fatalf("%s が ; で終わっていない", name)
		}
		if !regexp.MustCompile(`\btarget\b`).MatchString(raw[start : start+end]) {
			t.Errorf("%s が target 列を扱っていない:\n%s", name, raw[start:start+end])
		}
	}
}
