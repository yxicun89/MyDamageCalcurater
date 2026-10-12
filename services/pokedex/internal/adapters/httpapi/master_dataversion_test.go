package httpapi_test

// GET /internal/pokedex/master の dataVersion の形のテスト(issue #281・issue #403 パッケージ D20・ADR-0128)。
// dataVersion は data_versions の各行を「source=version@checksum先頭8桁」にし、source 昇順に「,」で連結する。
// effects・regulations・importer-config・name-overrides の version は常に "local" なので、checksum を含めないと
// 中身の違うマスタを区別できない。応答の形(MasterExport の dataVersion は1文字以上の文字列)は変えないので、
// calc-svc(services/calc/internal/master.FromExport は空でないことだけを見る)はそのまま読める。

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func masterDataVersion(t *testing.T, q *storetest.Querier) string {
	t.Helper()
	rec := do(t, newHandler(t, q), http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	return decodeExport(t, rec.Body.Bytes()).DataVersion
}

// AC-V1: 形は source=version@checksum先頭8桁 の source 昇順の連結(DB の行の順によらない)。
func TestMasterExportDataVersionIncludesChecksumPrefix(t *testing.T) {
	q := storetest.New()
	// 行の順を変えても同じ値になること(source 昇順に並べ直す)。
	reverse(q.DataVersions)
	got := masterDataVersion(t, q)
	want := "calc=test-calc-1@22222222," +
		"pokeapi=cafef00dcafef00dcafef00dcafef00dcafef00d@33333333," +
		"showdown=abad1deaabad1deaabad1deaabad1deaabad1dea@11111111"
	if got != want {
		t.Errorf("dataVersion = %q, want %q", got, want)
	}
}

// AC-V2(#281 の回帰テスト): version が同じ("local" 固定の取得元)でも checksum が変われば dataVersion が変わる。
func TestMasterExportDataVersionChangesWithChecksumOnly(t *testing.T) {
	base := storetest.New()
	base.DataVersions = append(base.DataVersions, localVersion("effects", strings.Repeat("a", 64)))
	before := masterDataVersion(t, base)

	changed := storetest.New()
	changed.DataVersions = append(changed.DataVersions, localVersion("effects", strings.Repeat("b", 64)))
	after := masterDataVersion(t, changed)

	if before == after {
		t.Fatalf("effects の checksum だけを変えても dataVersion が変わらない: %q", before)
	}
	if !strings.Contains(before, "effects=local@aaaaaaaa") || !strings.Contains(after, "effects=local@bbbbbbbb") {
		t.Errorf("effects の項が source=version@checksum先頭8桁 になっていない\nbefore=%q\nafter=%q", before, after)
	}
}

// localVersion は version が "local" 固定の取得元(effects 等。importer の versions.go)の行。
func localVersion(source, checksum string) store.DataVersion {
	return store.DataVersion{Source: source, Version: "local", Checksum: checksum, ImportedAt: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)}
}

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
