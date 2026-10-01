//go:build mysql

package importer_test

// 実 MySQL で、取得元の checksum だけが変わる2回目の import(effects.json だけを直した、など。version は "local" 固定)の
// 後に、内部 API(/internal/pokedex/master)の dataVersion と `pokedex export` の metadata.json の dataVersion が
// 両方とも変わり、互いに一致し続けることを確かめる(issue #281・issue #108-a・ADR-0128)。`make test-db` だけが実行する。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/readtx"
)

// readDataVersions は内部 API と export の dataVersion を読む。
func readDataVersions(t *testing.T, db readtx.DB) (master, export string) {
	t.Helper()
	var ex struct {
		DataVersion string `json:"dataVersion"`
	}
	if err := json.Unmarshal(getMaster(t, httpapi.NewHandler(db)), &ex); err != nil {
		t.Fatalf("内部 API の本文を読めない: %v", err)
	}
	files, _, err := readmodel.Export(context.Background(), db)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var md struct {
		DataVersion string `json:"dataVersion"`
	}
	if err := json.Unmarshal(files.Metadata, &md); err != nil {
		t.Fatalf("metadata.json を読めない: %v\n%s", err, files.Metadata)
	}
	return ex.DataVersion, md.DataVersion
}

func TestDataVersionFollowsChecksumOnlyReimport(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(ctx, conn, out, versions, now); err != nil {
		t.Fatalf("Apply(1回目): %v", err)
	}
	db := readtx.NewDB(conn)
	masterBefore, exportBefore := readDataVersions(t, db)
	if masterBefore == "" || masterBefore != exportBefore {
		t.Fatalf("1回目: 内部 API の dataVersion = %q, export = %q(空でなく一致すること)", masterBefore, exportBefore)
	}

	// version はそのまま、checksum だけを変える("local" 固定の取得元があればそれを選ぶ)。
	changed := append([]importer.SourceVersion(nil), versions...)
	target := 0
	for i, v := range changed {
		if v.Version == "local" {
			target = i
			break
		}
	}
	replacement := strings.Repeat("e", 64)
	if changed[target].Checksum == replacement {
		replacement = strings.Repeat("d", 64)
	}
	changed[target].Checksum = replacement
	if err := importer.Apply(ctx, conn, out, changed, now.Add(time.Hour)); err != nil {
		t.Fatalf("Apply(2回目。checksum だけ違う): %v", err)
	}
	masterAfter, exportAfter := readDataVersions(t, db)
	if masterAfter == masterBefore {
		t.Errorf("%s の checksum だけを変えた import の後も内部 API の dataVersion が変わらない: %q", changed[target].Source, masterAfter)
	}
	if exportAfter != masterAfter {
		t.Errorf("2回目: 内部 API の dataVersion = %q, export = %q(一致すること)", masterAfter, exportAfter)
	}
	if want := changed[target].Source + "=" + changed[target].Version + "@" + replacement[:8]; !strings.Contains(masterAfter, want) {
		t.Errorf("dataVersion = %q, want %q を含む", masterAfter, want)
	}
}
