package db

// species.name_ja_source の 'generated'(メガ種族の名前の生成。ADR-0324・issue #515)の静的な確認。DB は使わない。

import (
	"regexp"
	"testing"
)

// speciesNameSourceMigrationVersion は 'generated' を許す migration の版(000010 の次)。
const speciesNameSourceMigrationVersion = 11

func TestMigrationAllowsGeneratedNameSourceOnSpecies(t *testing.T) {
	_, up, down := migrationPairs(t)
	if up[speciesNameSourceMigrationVersion] == "" || down[speciesNameSourceMigrationVersion] == "" {
		t.Fatalf("migration %06d(species.name_ja_source に generated を足す。ADR-0324)が無い", speciesNameSourceMigrationVersion)
	}
	upSQL := readRaw(t, up[speciesNameSourceMigrationVersion])
	downSQL := readRaw(t, down[speciesNameSourceMigrationVersion])

	check := regexp.MustCompile(`(?is)constraint\s+chk_species_name_ja_source\s+check\s*\(\s*name_ja_source\s+in\s*\(([^)]*)\)\s*\)`)
	m := check.FindStringSubmatch(upSQL)
	if m == nil {
		t.Fatalf("up に CHECK chk_species_name_ja_source の再定義が無い:\n%s", upSQL)
	}
	for _, want := range []string{"'pokeapi'", "'override'", "'fallback_en'", "'generated'"} {
		if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(m[1]) {
			t.Errorf("up の CHECK に %s が無い: %s", want, m[1])
		}
	}
	dm := check.FindStringSubmatch(downSQL)
	if dm == nil {
		t.Fatalf("down に CHECK を戻す定義が無い:\n%s", downSQL)
	}
	if regexp.MustCompile(`generated`).MatchString(dm[1]) {
		t.Errorf("down の CHECK に generated が残っている: %s", dm[1])
	}
	// generated の行が残っていると CHECK を戻せないので、先に別の値へ寄せる。
	if !regexp.MustCompile(`(?is)update\s+species\s+set\s+name_ja_source\s*=\s*'fallback_en'\s+where\s+name_ja_source\s*=\s*'generated'`).MatchString(downSQL) {
		t.Errorf("down に generated の行を fallback_en へ寄せる UPDATE が無い:\n%s", downSQL)
	}
}
