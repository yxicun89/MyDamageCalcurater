package importer_test

// 入力の読み込み(ディレクトリの配置・厳格なデコード・取得元ごとの版とチェックサム)のテスト(ADR-0101 §1・§9)。

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fixture の Showdown・PokeAPI の版(commit)。40桁の16進(ADR-0101 §3)を満たす架空の値。
const (
	fixtureShowdownVersion = "abad1deaabad1deaabad1deaabad1deaabad1dea"
	fixturePokeAPIVersion  = "cafef00dcafef00dcafef00dcafef00dcafef00d"
)

func readFixture(t *testing.T, rel string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// copyFixture は架空データ一式を一時ディレクトリへ複製する(書き換えるテスト用)。
func copyFixture(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(fixtureRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixtureRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func versionsBySource(vs []importer.SourceVersion) map[string]importer.SourceVersion {
	m := map[string]importer.SourceVersion{}
	for _, v := range vs {
		m[v.Source] = v
	}
	return m
}

func TestLoadInputReadsLayoutAndVersions(t *testing.T) {
	in, versions, err := importer.LoadInput(fixtureRoot)
	if err != nil {
		t.Fatalf("LoadInput: %v", err)
	}
	if in.Calc.Version != "test-calc-1" || in.Showdown.Version != fixtureShowdownVersion || in.PokeAPI.Version != fixturePokeAPIVersion {
		t.Errorf("スナップショットの版 = %q / %q / %q", in.Calc.Version, in.Showdown.Version, in.PokeAPI.Version)
	}
	if in.Showdown.Mod != "testmod" {
		t.Errorf("Showdown の mod = %q", in.Showdown.Mod)
	}
	if got := in.Overrides.Species["testleafrain"]; got != "テストリーフあめ" {
		t.Errorf("override が読めていない: %q", got)
	}

	got := versionsBySource(versions)
	// data_versions の source は ADR-0101 §9 の7つ。checksum はファイルのバイト列の sha256。
	want := map[string]importer.SourceVersion{
		"calc":            {Source: "calc", Version: "test-calc-1", Checksum: sha256Hex(readFixture(t, "generated/calc/test-calc-1/snapshot.json"))},
		"showdown":        {Source: "showdown", Version: fixtureShowdownVersion, Checksum: sha256Hex(readFixture(t, "generated/showdown/"+fixtureShowdownVersion+"/snapshot.json"))},
		"pokeapi":         {Source: "pokeapi", Version: fixturePokeAPIVersion, Checksum: sha256Hex(readFixture(t, "generated/pokeapi/"+fixturePokeAPIVersion+"/snapshot.json"))},
		"importer-config": {Source: "importer-config", Version: "local", Checksum: sha256Hex(readFixture(t, "importer/config.json"))},
		"effects":         {Source: "effects", Version: "local", Checksum: sha256Hex(readFixture(t, "importer/effects.json"))},
		"regulations":     {Source: "regulations", Version: "local", Checksum: sha256Hex(readFixture(t, "importer/regulations.json"))},
		"name-overrides":  {Source: "name-overrides", Version: "local", Checksum: sha256Hex(readFixture(t, "local/name_ja_overrides.json"))},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("versions = %+v\nwant %+v", got, want)
	}
	if len(versions) != len(want) {
		t.Errorf("versions の件数 = %d(重複がある)", len(versions))
	}
	if !sort.SliceIsSorted(versions, func(i, j int) bool { return versions[i].Source < versions[j].Source }) {
		t.Errorf("versions が source 順でない")
	}
}

// override のファイルは任意(利用者が用意する実データで Git に入れない)。無ければ空として扱う。
func TestLoadInputWithoutOverrides(t *testing.T) {
	root := copyFixture(t)
	if err := os.Remove(filepath.Join(root, "local", "name_ja_overrides.json")); err != nil {
		t.Fatal(err)
	}
	in, versions, err := importer.LoadInput(root)
	if err != nil {
		t.Fatalf("LoadInput: %v", err)
	}
	if len(in.Overrides.Species)+len(in.Overrides.Moves)+len(in.Overrides.Items)+len(in.Overrides.Abilities)+len(in.Overrides.Types) != 0 {
		t.Errorf("override のファイルが無いのに中身がある: %+v", in.Overrides)
	}
	v := versionsBySource(versions)["name-overrides"]
	if v.Version != "none" || v.Checksum != sha256Hex(nil) {
		t.Errorf("name-overrides = %+v, want version none / sha256(空)", v)
	}
	if _, _, err := importer.Convert(in); err != nil {
		t.Errorf("override なしでも変換できること: %v", err)
	}
}

func TestLoadInputRejectsBrokenLayout(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, root string)
	}{
		{"設定の版のスナップショットが無い", func(t *testing.T, root string) {
			missing := "cafef00dcafef00dcafef00dcafef00dcafef00e" // 末尾だけ違う(有効な形式だがディレクトリが無い)
			writeFile(t, root, "importer/config.json", strings.Replace(string(readFixture(t, "importer/config.json")), fixturePokeAPIVersion, missing, 1))
		}},
		{"スナップショットの version がディレクトリ(設定の版)と違う", func(t *testing.T, root string) {
			src := filepath.Join(root, "generated/calc/test-calc-1/snapshot.json")
			raw, _ := os.ReadFile(src)
			writeFile(t, root, "generated/calc/test-calc-2/snapshot.json", string(raw))
			writeFile(t, root, "importer/config.json", strings.Replace(string(readFixture(t, "importer/config.json")), "test-calc-1", "test-calc-2", 1))
		}},
		{"版に使えない文字(パスを外へ出さない)", func(t *testing.T, root string) {
			writeFile(t, root, "importer/config.json", strings.Replace(string(readFixture(t, "importer/config.json")), "test-calc-1", "../calc", 1))
		}},
		{"効果定義のファイルが無い", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "importer/effects.json")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyFixture(t)
			tt.mutate(t, root)
			if _, _, err := importer.LoadInput(root); err == nil {
				t.Fatal("壊れた配置を受け付けた")
			}
		})
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 各デコーダは未知のフィールド・schemaVersion・source の食い違い・後続データを拒否する
// (フィールドの改名で値が黙ってゼロ値にならないように)。
func TestDecodersAreStrict(t *testing.T) {
	type decoder func([]byte) error
	decoders := map[string]decoder{
		"calc":        func(b []byte) error { _, err := importer.DecodeCalcSnapshot(b); return err },
		"showdown":    func(b []byte) error { _, err := importer.DecodeShowdownSnapshot(b); return err },
		"pokeapi":     func(b []byte) error { _, err := importer.DecodePokeAPISnapshot(b); return err },
		"overrides":   func(b []byte) error { _, err := importer.DecodeNameOverrides(b); return err },
		"effects":     func(b []byte) error { _, err := importer.DecodeEffectsFile(b); return err },
		"regulations": func(b []byte) error { _, err := importer.DecodeRegulationsFile(b); return err },
		"config":      func(b []byte) error { _, err := importer.DecodeConfig(b); return err },
	}
	valid := map[string]string{
		"calc":        `{"schemaVersion":1,"source":"calc","version":"v","generation":0,"types":[],"typeChart":{},"species":[],"moves":[],"items":[],"abilities":[]}`,
		"showdown":    `{"schemaVersion":1,"source":"showdown","version":"v","mod":"m","species":[],"moves":[],"items":[],"abilities":[],"learnsets":{}}`,
		"pokeapi":     `{"schemaVersion":1,"source":"pokeapi","version":"v","species":[],"forms":[],"moves":[],"items":[],"abilities":[],"types":[]}`,
		"overrides":   `{"schemaVersion":1,"species":{},"moves":{},"items":{},"abilities":{},"types":{}}`,
		"effects":     `{"schemaVersion":1,"items":{},"abilities":{}}`,
		"regulations": `{"schemaVersion":1,"regulations":[]}`,
		"config":      `{"schemaVersion":1,"sources":{},"excludeCalcSpecies":[],"excludeTypes":[],"nameJaLanguages":["ja"]}`,
	}
	for name, dec := range decoders {
		t.Run(name, func(t *testing.T) {
			base := valid[name]
			if err := dec([]byte(base)); err != nil {
				t.Fatalf("妥当な入力を拒否した: %v", err)
			}
			bad := map[string]string{
				"未知のフィールド":          strings.Replace(base, `{"schemaVersion":1`, `{"schemaVersion":1,"extra":true`, 1),
				"schemaVersion が違う": strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":2`, 1),
				"後続のデータ":            base + `{}`,
				"JSON でない":          `not json`,
			}
			if strings.Contains(base, `"source":"`) {
				bad["source が違う"] = strings.Replace(base, `"source":"`+name+`"`, `"source":"other"`, 1)
			}
			for label, raw := range bad {
				err := dec([]byte(raw))
				if !errors.Is(err, importer.ErrInvalidInput) {
					t.Errorf("%s: err = %v, want ErrInvalidInput", label, err)
				}
			}
		})
	}
}

func TestDecodeRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"override の値が空", func() error {
			_, err := importer.DecodeNameOverrides([]byte(`{"schemaVersion":1,"species":{"testmon":""},"moves":{},"items":{},"abilities":{},"types":{}}`))
			return err
		}()},
		{"レギュレーション ID の形式", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"Test_Reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"","endsOn":"","showdownMod":"m"}]}`))
			return err
		}()},
		{"レギュレーションの日付の形式", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"test-reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"2026/01/01","endsOn":"","showdownMod":"m"}]}`))
			return err
		}()},
		{"レギュレーションの開始が終了より後", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"test-reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"2026-03-01","endsOn":"2026-01-01","showdownMod":"m"}]}`))
			return err
		}()},
		{"nameJaLanguages が空", func() error {
			_, err := importer.DecodeConfig([]byte(`{"schemaVersion":1,"sources":{},"excludeCalcSpecies":[],"excludeTypes":[],"nameJaLanguages":[]}`))
			return err
		}()},
		{"レギュレーションの showdownMod が未固定のプレースホルダ(版を明示的に止める)", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"test-reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"","endsOn":"","showdownMod":"PENDING-PIN-MOD"}]}`))
			return err
		}()},
		{"config の showdown の版が commit(40桁16進)の形式でない", func() error {
			_, err := importer.DecodeConfig([]byte(`{"schemaVersion":1,"sources":{"showdown":"not-a-commit"},"excludeCalcSpecies":[],"excludeTypes":[],"nameJaLanguages":["ja"]}`))
			return err
		}()},
		{"config の pokeapi の版が未固定のプレースホルダ(版を明示的に止める)", func() error {
			_, err := importer.DecodeConfig([]byte(`{"schemaVersion":1,"sources":{"pokeapi":"PENDING-PIN-COMMIT"},"excludeCalcSpecies":[],"excludeTypes":[],"nameJaLanguages":["ja"]}`))
			return err
		}()},
		{"レギュレーションの minSourceGen が0(ADR-0103 §7・§9)", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"test-reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"","endsOn":"","showdownMod":"m","minSourceGen":0}]}`))
			return err
		}()},
		{"レギュレーションの minSourceGen が無い(欠落を黙って0のまま通さない。ADR-0103 §7・§9)", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"test-reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"","endsOn":"","showdownMod":"m"}]}`))
			return err
		}()},
		{"レギュレーションの minSourceGen が負(ADR-0103 §7・§9)", func() error {
			_, err := importer.DecodeRegulationsFile([]byte(`{"schemaVersion":1,"regulations":[{"id":"test-reg","nameJa":"テストレギュ","isDefault":true,"startsOn":"","endsOn":"","showdownMod":"m","minSourceGen":-1}]}`))
			return err
		}()},
	}
	for _, tt := range tests {
		if !errors.Is(tt.err, importer.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", tt.name, tt.err)
		}
	}
}
