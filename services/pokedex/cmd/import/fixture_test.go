package main

// CLI のテスト用の架空データ一式(ADR-0100 §7 の架空データの規約)。importer の testdata/fictional を
// 一時ディレクトリへ複製し、config.json に照合の設定(fixture の裁定の件数・ハッシュ。ADR-0103 §5)を足す。
// fixture そのものは変えない(P2-2b/c のテストの前提を保つ)。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const fixtureRoot = "../../importer/testdata/fictional"

// fixture の固定版(testdata/fictional/importer/config.json の sources)。
const (
	fixtureCalcVersion     = "test-calc-1"
	fixtureShowdownVersion = "abad1deaabad1deaabad1deaabad1deaabad1dea"
	fixturePokeAPIVersion  = "cafef00dcafef00dcafef00dcafef00dcafef00d"
)

func verdictHash(ids ...string) string {
	s := append([]string(nil), ids...)
	sort.Strings(s)
	sum := sha256.Sum256([]byte(strings.Join(s, "\n")))
	return hex.EncodeToString(sum[:])
}

// fixtureReferenceTypeChart は fixture の calc と一致する架空の参照の相性表(issue #280・ADR-0118)。
const fixtureReferenceTypeChart = "../../importer/testdata/fictional-reference/typechart.json"

// copyFixtureData は fixture を複製し、照合の設定を足した data ディレクトリを返す。
// calcOnlyExcludedCount を fixture の実際(2)と変えると、裁定の照合が Blocker になる。
// リポジトリと同じ配置(<root>/data と <root>/testdata/golden/typechart.json)にし、
// -typechart を省いたときの既定の場所で参照の相性表が見つかるようにする。
func copyFixtureData(t *testing.T, calcOnlyExcludedCount int) string {
	t.Helper()
	root := t.TempDir()
	refRaw, err := os.ReadFile(fixtureReferenceTypeChart)
	if err != nil {
		t.Fatal(err)
	}
	refDir := filepath.Join(root, "testdata", "golden")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "typechart.json"), refRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "data")
	err = filepath.WalkDir(fixtureRoot, func(path string, d os.DirEntry, err error) error {
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

	cfgPath := filepath.Join(dst, "importer", "config.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg["reconcile"] = map[string]any{
		"effectHooks": []string{"onBasePower", "onModifyAtk", "onModifySpA", "onModifyDamage", "onSourceModifyDamage"},
		"verdicts": map[string]any{
			"basis": map[string]string{"calc": fixtureCalcVersion, "showdown": fixtureShowdownVersion},
			"moves": map[string]any{
				"calcOnlyExcluded":     map[string]any{"count": calcOnlyExcludedCount, "idsSha256": verdictHash("testbanned", "testold")},
				"showdownOnlyIncluded": map[string]any{"count": 1, "idsSha256": verdictHash("testsplash")},
				"statusTypeMismatch":   map[string]any{"count": 1, "idsSha256": verdictHash("testglare")},
			},
		},
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}
