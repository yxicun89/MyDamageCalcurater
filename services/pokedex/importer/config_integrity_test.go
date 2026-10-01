package importer_test

// issue #222・#301(D19): config.json の `integrity`(取得物の期待ハッシュ)の形。
// ハッシュの照合そのものは Node(tools/importer の integrity.mjs)が行い、Go は config.json を
// 厳格にデコードするだけなので、`integrity` を未知のフィールドとして拒否しないこと・形(64桁の16進)を
// 検証することだけをここで固定する(ADR-0101 追記)。値は架空(実際の期待ハッシュは実データから計算して入れる)。

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

const (
	fakeCommit = "0123456789abcdef0123456789abcdef01234567"
	fakeHexA   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fakeHexB   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func configWithIntegrity(integrity string) []byte {
	return []byte(fmt.Sprintf(`{
  "schemaVersion": 1,
  "sources": {"calc": "0.12.0", "showdown": %q, "pokeapi": %q},
  "nameJaLanguages": ["ja"],
  "integrity": %s
}`, fakeCommit, fakeCommit, integrity))
}

func TestDecodeConfigAcceptsIntegrity(t *testing.T) {
	cfg, err := importer.DecodeConfig(configWithIntegrity(fmt.Sprintf(
		`{"showdown": {"treeSha256": %q}, "pokeapi": {"csvSha256": {"moves.csv": %q, "items.csv": %q}}}`,
		fakeHexA, fakeHexA, fakeHexB)))
	if err != nil {
		t.Fatalf("integrity を持つ config を読めること(未知のフィールドとして拒否しない): %v", err)
	}
	if cfg.Integrity == nil {
		t.Fatal("cfg.Integrity が nil")
	}
	if got := cfg.Integrity.Showdown.TreeSha256; got != fakeHexA {
		t.Errorf("Integrity.Showdown.TreeSha256 = %q, want %q", got, fakeHexA)
	}
	if got := cfg.Integrity.PokeAPI.CSVSha256["items.csv"]; got != fakeHexB {
		t.Errorf("Integrity.PokeAPI.CSVSha256[items.csv] = %q, want %q", got, fakeHexB)
	}
}

func TestDecodeConfigRejectsMalformedIntegrity(t *testing.T) {
	cases := map[string]string{
		"短い treeSha256":     `{"showdown": {"treeSha256": "abc"}}`,
		"大文字の treeSha256":   fmt.Sprintf(`{"showdown": {"treeSha256": %q}}`, strings.ToUpper(fakeHexA)),
		"16進でない treeSha256": fmt.Sprintf(`{"showdown": {"treeSha256": %q}}`, strings.Repeat("z", 64)),
		"csvSha256 の値が不正":   `{"pokeapi": {"csvSha256": {"moves.csv": "xyz"}}}`,
		"未知のキー":             fmt.Sprintf(`{"showdown": {"treeSha256": %q, "tarballSha256": %q}}`, fakeHexA, fakeHexA),
	}
	for name, integrity := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := importer.DecodeConfig(configWithIntegrity(integrity))
			if !errors.Is(err, importer.ErrInvalidInput) {
				t.Errorf("ErrInvalidInput を返すこと: %v", err)
			}
		})
	}
}

// 実物の config.json は、Showdown の展開後の内容ハッシュと、取得する PokeAPI の CSV ごとの sha256 を持つ
// (fail closed の前提。無いと Node 側の取得が終了コード 3 で止まる)。CSV の一覧は fetch-pokeapi.mjs の
// fetchCSV('...') の呼び出しから読む(二重管理しない)。
func TestRepoConfigPinsFetchIntegrity(t *testing.T) {
	raw, err := os.ReadFile(repoDataPath("config.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := importer.DecodeConfig(raw)
	if err != nil {
		t.Fatalf("data/importer/config.json: %v", err)
	}
	if cfg.Integrity == nil {
		t.Fatal("config.json に integrity が無い(Showdown の treeSha256 と PokeAPI の csvSha256 を pin する。ADR-0101 追記)")
	}
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !hex64.MatchString(cfg.Integrity.Showdown.TreeSha256) {
		t.Errorf("integrity.showdown.treeSha256 が64桁の16進でない: %q", cfg.Integrity.Showdown.TreeSha256)
	}
	src := readRepo(t, "tools/importer/fetch-pokeapi.mjs")
	names := regexp.MustCompile(`fetchCSV\('([a-z_]+\.csv)'\)`).FindAllStringSubmatch(src, -1)
	if len(names) == 0 {
		t.Fatal("fetch-pokeapi.mjs から取得する CSV の一覧を読めない")
	}
	for _, m := range names {
		if !hex64.MatchString(cfg.Integrity.PokeAPI.CSVSha256[m[1]]) {
			t.Errorf("integrity.pokeapi.csvSha256[%s] が無い・64桁の16進でない", m[1])
		}
	}
}
