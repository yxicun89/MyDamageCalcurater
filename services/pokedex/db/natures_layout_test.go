package db

// natures(000006)と P2-3 のクエリの静的検査(ADR-0105 §4・§2)。ファイルを読むだけで DB は使わない(make test で走る)。
// 制約が実際に効くことは natures_mysql_test.go(-tags mysql)で確かめる。

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const naturesUp = "migrations/000006_create_natures.up.sql"
const naturesDown = "migrations/000006_create_natures.down.sql"

func readLower(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s が読めない: %v", path, err)
	}
	return strings.ToLower(string(raw))
}

// AC-N4: natures は 000006 の新しい migration で作る(取り込み済みの 000001〜000005 を書き換えない)。
// down は natures だけを DROP する。
func TestNaturesMigrationIsNewPair(t *testing.T) {
	up := readLower(t, naturesUp)
	if !regexp.MustCompile("create\\s+table\\s+`?natures`?\\s*\\(").MatchString(up) {
		t.Fatalf("%s に CREATE TABLE natures が無い", naturesUp)
	}
	down := readLower(t, naturesDown)
	if !regexp.MustCompile("drop\\s+table\\s+(if\\s+exists\\s+)?`?natures`?\\s*;").MatchString(down) {
		t.Fatalf("%s に DROP TABLE natures が無い", naturesDown)
	}
	if n := len(regexp.MustCompile(`drop\s+table`).FindAllString(down, -1)); n != 1 {
		t.Errorf("%s の DROP TABLE が %d 個(natures の1つだけ)", naturesDown, n)
	}
	for _, old := range []string{
		"migrations/000001_create_types.up.sql",
		"migrations/000002_create_master.up.sql",
		"migrations/000003_create_regulations.up.sql",
		"migrations/000004_create_data_versions.up.sql",
		"migrations/000005_widen_species_ability_slot.up.sql",
	} {
		if strings.Contains(readLower(t, old), "natures") {
			t.Errorf("%s に natures がある(取り込み済みの migration は書き換えない。000006 に置く)", old)
		}
	}
}

// AC-N4: natures の列と制約(ADR-0105 §4)。ID は ascii_bin、日本語名は前方一致用の照合順序、
// plus/minus は HP を含まない5能力か NULL、無補正は両方 NULL・補正ありは plus <> minus、同じ補正の組は1つ。
// 性格の件数・名前を CHECK に列挙しない(マスタのデータ)。
func TestNaturesMigrationDeclaresConstraints(t *testing.T) {
	up := readLower(t, naturesUp)
	cases := []struct {
		name string
		re   string
	}{
		{"id は ascii_bin の主キー", `id\s+varchar\(\d+\)\s+character\s+set\s+ascii\s+collate\s+ascii_bin\s+not\s+null`},
		{"主キー", `primary\s+key\s*\(\s*id\s*\)`},
		{"id の形式", `id\s+regexp\s+'\^\[a-z0-9\]\+\$'`},
		{"name_ja の照合順序", `name_ja\s+varchar\(64\)\s+character\s+set\s+utf8mb4\s+collate\s+utf8mb4_ja_0900_as_cs\s+not\s+null`},
		{"name_ja が空でない", `char_length\(name_ja\)\s*>\s*0`},
		{"name_ja_source の値", `name_ja_source\s+in\s*\(\s*'pokeapi'\s*,\s*'override'\s*,\s*'fallback_en'\s*\)`},
		{"name_en", `name_en\s+varchar\(64\)\s+not\s+null`},
		{"plus は NULL 可", `plus\s+varchar\(\d+\)[^,]*\bnull\b`},
		{"minus は NULL 可", `minus\s+varchar\(\d+\)[^,]*\bnull\b`},
		{"plus は HP を含まない5能力", `plus\s+is\s+null\s+or\s+plus\s+in\s*\(\s*'atk'\s*,\s*'def'\s*,\s*'spa'\s*,\s*'spd'\s*,\s*'spe'\s*\)`},
		{"minus は HP を含まない5能力", `minus\s+is\s+null\s+or\s+minus\s+in\s*\(\s*'atk'\s*,\s*'def'\s*,\s*'spa'\s*,\s*'spd'\s*,\s*'spe'\s*\)`},
		{"無補正は両方 NULL・補正ありは異なる2能力", `plus\s+is\s+null\s+and\s+minus\s+is\s+null\s*\)\s*or\s*\(\s*plus\s+is\s+not\s+null\s+and\s+minus\s+is\s+not\s+null\s+and\s+plus\s*<>\s*minus`},
		{"同じ補正の組は1つ", `unique\s+key\s+\w+\s*\(\s*plus\s*,\s*minus\s*\)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !regexp.MustCompile(tc.re).MatchString(up) {
				t.Fatalf("%s に見当たらない: /%s/", naturesUp, tc.re)
			}
		})
	}
	if regexp.MustCompile(`'hp'`).MatchString(up) {
		t.Errorf("%s に 'hp' がある(性格は HP を補正しない)", naturesUp)
	}
	for _, name := range []string{"adamant", "hardy", "いじっぱり"} {
		if strings.Contains(up, name) {
			t.Errorf("%s に実在の性格名 %q がある(性格の一覧はマスタのデータ。migration に書かない)", naturesUp, name)
		}
	}
}

// AC-N4: sqlc のクエリに P2-3 が使う読み出し・投入がある(ADR-0105 §2・§3・§4)。
// 生成物(internal/store)との一致は make gen の差分検査が見る。
func TestQueriesDeclareP23Reads(t *testing.T) {
	raw, err := os.ReadFile("query/pokedex.sql")
	if err != nil {
		t.Fatal(err)
	}
	q := string(raw)
	for _, name := range []string{
		"ListNatures :many", "DeleteNatures :exec", "InsertNature :exec",
		"ListSpecies :many", "ListAllSpeciesAbilities :many", "ListItems :many", "ListItemEffects :many",
		"ListAbilities :many", "ListAbilityEffects :many", "GetDefaultRegulation :one",
		"ListRegulationSpeciesKeys :many", "ListRegulationMoveIDs :many", "ListRegulationAbilityIDs :many",
		"SearchSpecies :many", "SearchMoves :many", "SearchItems :many",
		"ListSpeciesAbilityNames :many", "ListSpeciesLearnset :many",
	} {
		if !strings.Contains(q, "-- name: "+name) {
			t.Errorf("query/pokedex.sql に -- name: %s が無い", name)
		}
	}
	// 検索は前方一致のパターンを呼び出し側から受ける(LIKE ? の形。CONCAT で % を足さない: エスケープの責任を1か所にする)。
	for _, name := range []string{"SearchSpecies", "SearchMoves", "SearchItems"} {
		body := queryBody(t, q, name)
		if !regexp.MustCompile(`(?i)name_ja\s+like\s+sqlc\.arg\(pattern\)`).MatchString(body) {
			t.Errorf("%s が name_ja LIKE sqlc.arg(pattern) でない: %s", name, body)
		}
		if !regexp.MustCompile(`(?i)regulation_id\s*=\s*sqlc\.arg\(regulation_id\)`).MatchString(body) {
			t.Errorf("%s が使用可能集合(regulation_id)で絞っていない: %s", name, body)
		}
		if !regexp.MustCompile(`(?i)\blimit\s+\?`).MatchString(body) {
			t.Errorf("%s に LIMIT ? が無い: %s", name, body)
		}
	}
}

// queryBody は -- name: <name> から次の -- name: までを返す。
func queryBody(t *testing.T, q, name string) string {
	t.Helper()
	i := strings.Index(q, "-- name: "+name+" ")
	if i < 0 {
		t.Fatalf("クエリ %s が無い", name)
	}
	rest := q[i+len("-- name: "):]
	if j := strings.Index(rest, "-- name: "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
