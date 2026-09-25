package importer_test

// 「正」が2か所にあるファイルの一致(issue #280・ADR-0118)。どれもコミット済みのファイル
// (英語 ID・整数・版の文字列だけ)を読む。実データ(data/generated)は読まない。
//
//   - 効果定義: data/importer/effects.json(本番の正。ADR-0101)と testdata/golden/effects.json
//     (ゴールデンの生成器と engine のゴールデンテストの入力)。キーの表記(ID と英語名)だけが違う。
//   - @smogon/calc の版: importer の取得元の版(data/importer/config.json)・取得スクリプトの固定版・
//     importer とゴールデン生成器の依存・ゴールデンの生成物に書かれた版。

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

// repoPath はリポジトリ直下からの相対パスを、このパッケージからのパスにする。
func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

func readRepoFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	path := repoPath(parts...)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s が読めない(スキップしない): %v", path, err)
	}
	return raw
}

// normalizeEffectKeys はキーを toID で正規化し、値を JSON の意味(空白・キー順を無視)で比べられる形にする。
// 正規化でキーが衝突したら失敗させる(同じ ID に2つの定義があると、どちらが使われるか決まらない)。
func normalizeEffectKeys(t *testing.T, label string, defs map[string]json.RawMessage) map[string]any {
	t.Helper()
	out := make(map[string]any, len(defs))
	for name, raw := range defs {
		id := testToID(name)
		if _, dup := out[id]; dup {
			t.Fatalf("%s: toID(%q) = %q が重複している", label, name, id)
		}
		var v any
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("%s: %q の値が JSON でない: %v", label, name, err)
		}
		out[id] = v
	}
	return out
}

// testToID は Showdown/calc の toID(小文字化して英数字以外を落とす)。importer の toID と同じ規則。
func testToID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// diffKeys は2つの定義の食い違い(片方だけにある ID・値が違う ID)を昇順で返す。
func diffKeys(a, b map[string]any) []string {
	var diffs []string
	for id, av := range a {
		bv, ok := b[id]
		switch {
		case !ok:
			diffs = append(diffs, id+"(本番だけにある)")
		case !reflect.DeepEqual(av, bv):
			diffs = append(diffs, id+"(値が違う)")
		}
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			diffs = append(diffs, id+"(ゴールデンだけにある)")
		}
	}
	sort.Strings(diffs)
	return diffs
}

// goldenEffectsFile は testdata/golden/effects.json の形(キーは @smogon/calc の英語名)。
type goldenEffectsFile struct {
	Note      string                     `json:"note"`
	Items     map[string]json.RawMessage `json:"items"`
	Abilities map[string]json.RawMessage `json:"abilities"`
}

// TestGoldenEffectsMatchImporterEffects は、ゴールデンテストが検証している効果定義と、本番の DB に
// 入る効果定義が同じであることを確かめる。片方だけを編集すると失敗する(正は data/importer/effects.json。
// ゴールデン側を合わせてから tools/golden で再生成する。ADR-0118)。
func TestGoldenEffectsMatchImporterEffects(t *testing.T) {
	prod, err := importer.DecodeEffectsFile(readRepoFile(t, "data", "importer", "effects.json"))
	if err != nil {
		t.Fatalf("data/importer/effects.json: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(readRepoFile(t, "testdata", "golden", "effects.json")))
	dec.DisallowUnknownFields()
	var golden goldenEffectsFile
	if err := dec.Decode(&golden); err != nil {
		t.Fatalf("testdata/golden/effects.json: %v", err)
	}

	for _, c := range []struct {
		label        string
		prod, golden map[string]json.RawMessage
	}{
		{"items", prod.Items, golden.Items},
		{"abilities", prod.Abilities, golden.Abilities},
	} {
		if len(c.prod) == 0 {
			t.Errorf("%s: 本番の効果定義が空(比較が空振りしていないか)", c.label)
		}
		p := normalizeEffectKeys(t, "data/importer/effects.json "+c.label, c.prod)
		g := normalizeEffectKeys(t, "testdata/golden/effects.json "+c.label, c.golden)
		if diffs := diffKeys(p, g); len(diffs) != 0 {
			t.Errorf("%s: 本番(data/importer/effects.json)とゴールデン(testdata/golden/effects.json)の効果定義が食い違う: %v", c.label, diffs)
		}
	}
}

// expectedVersionPattern は tools/importer/fetch-calc.mjs の固定版の宣言。
var expectedVersionPattern = regexp.MustCompile(`(?m)^const EXPECTED_VERSION = '([^']+)';$`)

// TestCalcVersionPinnedConsistently は @smogon/calc の版を固定している箇所がすべて同じ版であることを
// 確かめる。importer の取り込む相性表・効果の対象と、ゴールデンが照合した版が別の版にずれないため。
func TestCalcVersionPinnedConsistently(t *testing.T) {
	cfg, err := importer.DecodeConfig(readRepoFile(t, "data", "importer", "config.json"))
	if err != nil {
		t.Fatalf("data/importer/config.json: %v", err)
	}
	want := cfg.Sources["calc"]
	if want == "" {
		t.Fatal("data/importer/config.json の sources.calc が空")
	}

	got := map[string]string{}

	m := expectedVersionPattern.FindSubmatch(readRepoFile(t, "tools", "importer", "fetch-calc.mjs"))
	if m == nil {
		t.Fatal("tools/importer/fetch-calc.mjs に EXPECTED_VERSION の宣言が見つからない(書き方を変えたらこのテストも直す)")
	}
	got["tools/importer/fetch-calc.mjs EXPECTED_VERSION"] = string(m[1])

	for _, dir := range []string{"importer", "golden"} {
		var pkg struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if err := json.Unmarshal(readRepoFile(t, "tools", dir, "package.json"), &pkg); err != nil {
			t.Fatalf("tools/%s/package.json: %v", dir, err)
		}
		got["tools/"+dir+"/package.json"] = pkg.Dependencies["@smogon/calc"]

		var lock struct {
			Packages map[string]struct {
				Version string `json:"version"`
			} `json:"packages"`
		}
		if err := json.Unmarshal(readRepoFile(t, "tools", dir, "package-lock.json"), &lock); err != nil {
			t.Fatalf("tools/%s/package-lock.json: %v", dir, err)
		}
		got["tools/"+dir+"/package-lock.json"] = lock.Packages["node_modules/@smogon/calc"].Version
	}

	for _, name := range []string{"metadata.json", "typechart.json"} {
		var meta struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(readRepoFile(t, "testdata", "golden", name), &meta); err != nil {
			t.Fatalf("testdata/golden/%s: %v", name, err)
		}
		got["testdata/golden/"+name] = meta.Version
	}

	for where, v := range got {
		if v != want {
			t.Errorf("%s の @smogon/calc の版 = %q, want %q(data/importer/config.json の sources.calc と揃える)", where, v, want)
		}
	}
}
