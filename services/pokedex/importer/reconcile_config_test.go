package importer_test

// config.json の照合の設定(reconcile)の検証と、コミットされた data/importer の版の固定の検査
// (ADR-0103 §5・§9・§10)。ネットワークには触らない(コミットされた設定ファイルを読むだけ)。

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

const validReconcileConfig = `{"schemaVersion":1,
 "sources":{"calc":"test-calc-1","showdown":"abad1deaabad1deaabad1deaabad1deaabad1dea"},
 "excludeCalcSpecies":[],"excludeTypes":[],"nameJaLanguages":["ja"],
 "reconcile":{
  "effectHooks":["onBasePower","onModifyDamage"],
  "verdicts":{
   "basis":{"calc":"test-calc-1","showdown":"abad1deaabad1deaabad1deaabad1deaabad1dea"},
   "moves":{
    "calcOnlyExcluded":{"count":2,"idsSha256":"1111111111111111111111111111111111111111111111111111111111111111"},
    "showdownOnlyIncluded":{"count":1,"idsSha256":"2222222222222222222222222222222222222222222222222222222222222222"},
    "statusTypeMismatch":{"count":1,"idsSha256":"3333333333333333333333333333333333333333333333333333333333333333"}}}}}`

func TestDecodeConfigReconcileSection(t *testing.T) {
	cfg, err := importer.DecodeConfig([]byte(validReconcileConfig))
	if err != nil {
		t.Fatalf("妥当な reconcile を拒否した: %v", err)
	}
	if cfg.Reconcile == nil {
		t.Fatal("reconcile が読めていない")
	}
	r := cfg.Reconcile
	if len(r.EffectHooks) != 2 || r.EffectHooks[0] != "onBasePower" {
		t.Errorf("EffectHooks = %v", r.EffectHooks)
	}
	if r.Verdicts.Basis["showdown"] != "abad1deaabad1deaabad1deaabad1deaabad1dea" {
		t.Errorf("Basis = %v", r.Verdicts.Basis)
	}
	m := r.Verdicts.Moves
	if m.CalcOnlyExcluded.Count != 2 || m.ShowdownOnlyIncluded.Count != 1 || m.StatusTypeMismatch.Count != 1 ||
		!strings.HasPrefix(m.CalcOnlyExcluded.IDsSHA256, "1111") || !strings.HasPrefix(m.StatusTypeMismatch.IDsSHA256, "3333") {
		t.Errorf("Moves = %+v", m)
	}

	bad := map[string]string{
		"count が負": strings.Replace(validReconcileConfig, `{"count":2,`, `{"count":-1,`, 1),
		"idsSha256 が64桁でない": strings.Replace(validReconcileConfig,
			`"1111111111111111111111111111111111111111111111111111111111111111"`, `"1111"`, 1),
		"idsSha256 が大文字": strings.Replace(validReconcileConfig,
			`"1111111111111111111111111111111111111111111111111111111111111111"`,
			`"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`, 1),
		"effectHooks が空":             strings.Replace(validReconcileConfig, `["onBasePower","onModifyDamage"]`, `[]`, 1),
		"effectHooks の形式(on で始まらない)": strings.Replace(validReconcileConfig, `"onBasePower"`, `"basePower"`, 1),
		"verdicts に未知のフィールド":         strings.Replace(validReconcileConfig, `"verdicts":{`, `"verdicts":{"extra":1,`, 1),
		"moves の区分が欠けている(改名で黙ってゼロ値にしない)": strings.Replace(validReconcileConfig,
			`"statusTypeMismatch":{"count":1,"idsSha256":"3333333333333333333333333333333333333333333333333333333333333333"}`,
			`"statusTypeMismatchX":{"count":1,"idsSha256":"3333333333333333333333333333333333333333333333333333333333333333"}`, 1),
		"moves の区分が無い(ゼロ値の count 0・空のハッシュを通さない)": strings.Replace(validReconcileConfig,
			`,
    "statusTypeMismatch":{"count":1,"idsSha256":"3333333333333333333333333333333333333333333333333333333333333333"}`, ``, 1),
		"basis のキーが sources に無い": strings.Replace(validReconcileConfig, `"basis":{"calc"`, `"basis":{"calcx"`, 1),
		"basis の版が空":             strings.Replace(validReconcileConfig, `"basis":{"calc":"test-calc-1"`, `"basis":{"calc":""`, 1),
		"basis の版が未固定のプレースホルダ":   strings.Replace(validReconcileConfig, `"basis":{"calc":"test-calc-1"`, `"basis":{"calc":"PENDING-PIN"`, 1),
	}
	for label, raw := range bad {
		if raw == validReconcileConfig {
			t.Fatalf("%s: 置換が効いていない(テストの不備)", label)
		}
		if _, err := importer.DecodeConfig([]byte(raw)); !errors.Is(err, importer.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", label, err)
		}
	}
}

// TestRepoConfigIsPinned はコミットされた data/importer の設定が、取得元の版を固定し、
// ADR-0002 追記 P2-1c の裁定の件数(calc だけ 11・Showdown だけ 1・変化技のタイプの食い違い 1)を
// 照合の期待値として持っていることを確かめる(ADR-0103 §5・§10)。版は英語 ID と16進だけで、実データは読まない。
func TestRepoConfigIsPinned(t *testing.T) {
	repoData := filepath.Join("..", "..", "..", "data", "importer")
	rawCfg, err := os.ReadFile(filepath.Join(repoData, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := importer.DecodeConfig(rawCfg) // PENDING の版はここで ErrInvalidInput になる
	if err != nil {
		t.Fatalf("data/importer/config.json: %v(取得元の版を固定すること。ADR-0103 §10)", err)
	}
	commit := regexp.MustCompile(`^[0-9a-f]{40}$`)
	if cfg.Sources["calc"] != "0.12.0" {
		t.Errorf("sources.calc = %q, want 0.12.0(ADR-0101 §1)", cfg.Sources["calc"])
	}
	for _, src := range []string{"showdown", "pokeapi"} {
		if !commit.MatchString(cfg.Sources[src]) {
			t.Errorf("sources.%s = %q, want 40桁の commit", src, cfg.Sources[src])
		}
	}

	rawReg, err := os.ReadFile(filepath.Join(repoData, "regulations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importer.DecodeRegulationsFile(rawReg); err != nil {
		t.Errorf("data/importer/regulations.json: %v(showdownMod を固定すること)", err)
	}

	if cfg.Reconcile == nil {
		t.Fatal("config.json に reconcile(照合の設定)が無い")
	}
	if len(cfg.Reconcile.EffectHooks) == 0 {
		t.Error("reconcile.effectHooks が空")
	}
	basis := cfg.Reconcile.Verdicts.Basis
	if basis["calc"] != "0.12.0" {
		t.Errorf("verdicts.basis.calc = %q, want 0.12.0(P2-1c の裁定を行った版)", basis["calc"])
	}
	// P2-1c の裁定は smogon/pokemon-showdown の commit f10d679(2026-09-20)で行った(ADR-0002 追記 P2-1c)。
	if !commit.MatchString(basis["showdown"]) || !strings.HasPrefix(basis["showdown"], "f10d679") {
		t.Errorf("verdicts.basis.showdown = %q, want f10d679 で始まる40桁の commit", basis["showdown"])
	}
	zeros := strings.Repeat("0", 64)
	for _, v := range []struct {
		key  string
		got  importer.VerdictCount
		want int
	}{
		{"calcOnlyExcluded", cfg.Reconcile.Verdicts.Moves.CalcOnlyExcluded, 11},
		{"showdownOnlyIncluded", cfg.Reconcile.Verdicts.Moves.ShowdownOnlyIncluded, 1},
		{"statusTypeMismatch", cfg.Reconcile.Verdicts.Moves.StatusTypeMismatch, 1},
	} {
		if v.got.Count != v.want {
			t.Errorf("verdicts.moves.%s.count = %d, want %d(ADR-0002 追記 P2-1c の結論)", v.key, v.got.Count, v.want)
		}
		if v.got.IDsSHA256 == zeros {
			t.Errorf("verdicts.moves.%s.idsSha256 が仮置きの 0 のまま(実データの報告から写すこと。ADR-0103 §10 手順4)", v.key)
		}
	}
}
