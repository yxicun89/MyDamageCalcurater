package db

// docs/runbooks/data.md の日本語名の上書きの手順(issue #607・ADR-0140)の静的検査。DB は使わない。
// 場所・形・残りの一覧の見方・k3d への反映が書かれていること。実名を載せない(例は架空の test で始まる ID だけ)。

import (
	"regexp"
	"strings"
	"testing"
)

func TestRunbookDataDocumentsNameOverrides(t *testing.T) {
	s := readRepoFile(t, "docs/runbooks/data.md")
	for _, want := range []string{
		"data/local/name_ja_overrides.json", // 場所
		"schemaVersion",                     // 形
		"fallbackIds",                       // 残りの一覧(report)
		"override-unused",                   // 当たらない上書きの警告
		"pokedex-name-overrides",            // k3d の ConfigMap
		"make import-dry-run",               // 一覧を作る手順
	} {
		if !strings.Contains(s, want) {
			t.Errorf("docs/runbooks/data.md に %q が無い(日本語名の上書きの手順)", want)
		}
	}
	// 例の JSON の ID は架空(test で始まる)だけにする(実データの名前をコミットしない。ADR-0002)。
	block := regexp.MustCompile("(?s)```json\\n(.*?)```").FindAllStringSubmatch(s, -1)
	if len(block) == 0 {
		t.Fatal("上書きファイルの例(```json)が無い")
	}
	key := regexp.MustCompile(`"([a-z0-9]+)":\s*"`)
	for _, b := range block {
		if !strings.Contains(b[1], "schemaVersion") {
			continue
		}
		for _, m := range key.FindAllStringSubmatch(b[1], -1) {
			if !strings.HasPrefix(m[1], "test") {
				t.Errorf("上書きの例に架空でない ID %q がある", m[1])
			}
		}
	}
}
