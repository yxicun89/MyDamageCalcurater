//go:build golden

package engine

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type goldenCase struct {
	ID       string      `json:"id"`
	Input    DamageInput `json:"input"`
	Expected struct {
		Rolls         [16]int  `json:"rolls"`
		AttackerStats Stats    `json:"attackerStats"`
		DefenderStats Stats    `json:"defenderStats"`
		KO            KOChance `json:"ko"`
	} `json:"expected"`
}

type goldenMetadata struct {
	SchemaVersion int    `json:"schemaVersion"`
	Source        string `json:"source"`
	Version       string `json:"version"`
	Seed          uint32 `json:"seed"`
	SpeciesCount  int    `json:"speciesCount"`
	Files         map[string]struct {
		Count  int    `json:"count"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

func readGoldenMetadata(t *testing.T) goldenMetadata {
	t.Helper()
	data, err := os.ReadFile("../testdata/golden/metadata.json")
	if err != nil {
		t.Fatalf("golden fixtures required (cd tools/golden && npm ci && npm run generate): %v", err)
	}
	var meta goldenMetadata
	if err = json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.SchemaVersion != 1 || meta.Source != "@smogon/calc" || meta.Version != "0.10.0" || meta.Seed != 0x504f4b45 || meta.SpeciesCount == 0 {
		t.Fatalf("unexpected oracle metadata: %+v", meta)
	}
	return meta
}

func goldenFile(t *testing.T, meta goldenMetadata, name string) []byte {
	t.Helper()
	spec, ok := meta.Files[name]
	if !ok {
		t.Fatalf("fixture %s missing from manifest", name)
	}
	data, err := os.ReadFile(filepath.Join("../testdata/golden", name))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != spec.SHA256 {
		t.Fatalf("%s checksum differs; regenerate fixtures and manifest together", name)
	}
	return data
}

// Stream the compressed fixtures to keep the exhaustive external reference suite small on disk.
func eachGoldenLine(t *testing.T, meta goldenMetadata, name string, consume func([]byte)) {
	t.Helper()
	goldenFile(t, meta, name)
	f, err := os.Open(filepath.Join("../testdata/golden", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scanner.Scan() {
		consume(scanner.Bytes())
		count++
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != meta.Files[name].Count {
		t.Fatalf("%s count=%d want=%d", name, count, meta.Files[name].Count)
	}
}

func TestGoldenDamage(t *testing.T) {
	meta := readGoldenMetadata(t)
	// タイプ相性表は oracle が出す fixture(testdata/golden/typechart.json)から入力に注入する。
	// ベクタ1件ずつに 324 件の表を書かないため(ADR-0013 §P1-13.5)。
	chart := mustTypeChart(t)
	// defense-species は P1-10 の防御プリセット再定義でグループあたり 4 → 8 件になった
	// (種族 × 攻撃側アンカー5 × プリセット8)。ADR-0009 §6。
	if meta.Files["random.jsonl.gz"].Count != 10000 || meta.Files["fixed.json"].Count < 200 || meta.Files["attack-species.jsonl.gz"].Count != meta.SpeciesCount*20 || meta.Files["defense-species.jsonl.gz"].Count != meta.SpeciesCount*40 {
		t.Fatal("golden coverage incomplete")
	}
	for _, name := range []string{"fixed.json", "random.jsonl.gz", "attack-species.jsonl.gz", "defense-species.jsonl.gz"} {
		t.Run(name, func(t *testing.T) {
			count, failures := 0, 0
			seen := map[string]bool{}
			check := func(v goldenCase) {
				count++
				if v.ID == "" || seen[v.ID] {
					t.Fatalf("missing or duplicate case ID: %q", v.ID)
				}
				seen[v.ID] = true
				v.Input.TypeChart = chart
				result, err := CalcDamage(v.Input)
				a, d := RealStats(v.Input.Attacker), RealStats(v.Input.Defender)
				ko := v.Expected.KO
				mismatch := err != nil || result.Rolls != v.Expected.Rolls || a != v.Expected.AttackerStats || d != v.Expected.DefenderStats || result.DefenderHP != d.HP || result.KO.Hits != ko.Hits || result.KO.Guaranteed != ko.Guaranteed || math.Abs(result.KO.ChancePercent-ko.ChancePercent) > 1e-9
				if mismatch {
					failures++
					// Bound diagnostics only; every fixture is still evaluated and counted as a failure.
					if failures <= 12 {
						t.Logf("%s: error=%v rolls=%v want=%v KO=%+v want=%+v stats=%+v/%+v want=%+v/%+v", v.ID, err, result.Rolls, v.Expected.Rolls, result.KO, ko, a, d, v.Expected.AttackerStats, v.Expected.DefenderStats)
					}
				}
			}
			if name == "fixed.json" {
				var cases []goldenCase
				if err := json.Unmarshal(goldenFile(t, meta, name), &cases); err != nil {
					t.Fatal(err)
				}
				for _, v := range cases {
					check(v)
				}
			} else {
				eachGoldenLine(t, meta, name, func(line []byte) {
					var v goldenCase
					if err := json.Unmarshal(line, &v); err != nil {
						t.Fatal(err)
					}
					check(v)
				})
			}
			if count != meta.Files[name].Count {
				t.Fatalf("%s count=%d want=%d", name, count, meta.Files[name].Count)
			}
			if failures > 0 {
				t.Errorf("%d/%d external oracle cases disagree", failures, count)
			}
		})
	}
}

// TestGoldenTypeChart は oracle が出した相性表(testdata/golden/typechart.json)が
// 生成物として完全であり、engine がその値どおりに引くことを確かめる(ADR-0013 §P1-13.5)。
// 表の中身の正しさは oracle の責務なので、ここでは「取りこぼしなく渡って引ける」ことだけを見る。
func TestGoldenTypeChart(t *testing.T) {
	meta := readGoldenMetadata(t)
	goldenFile(t, meta, typeChartFixture) // sha256 の照合
	f, err := readTypeChartFixture()
	if err != nil {
		t.Fatal(err)
	}
	if f.Source != "@smogon/calc" || f.Version != meta.Version || f.Generation != 9 {
		t.Fatalf("相性表の出どころが他の fixture と違う: source=%q version=%q generation=%d", f.Source, f.Version, f.Generation)
	}
	if len(f.Types) != typeChartTypeCount {
		t.Fatalf("タイプ数=%d want %d(oracle の ??? / Stellar は除外する)", len(f.Types), typeChartTypeCount)
	}
	if n := meta.Files[typeChartFixture].Count; n != typeChartTypeCount*typeChartTypeCount {
		t.Fatalf("metadata の count=%d want %d(18×18)", n, typeChartTypeCount*typeChartTypeCount)
	}
	if !sort.SliceIsSorted(f.Types, func(i, j int) bool { return f.Types[i] < f.Types[j] }) {
		t.Errorf("types は ID 昇順で出力すること(生成を決定的にするため): %v", f.Types)
	}

	chart := mustTypeChart(t)
	valid := map[int]bool{TypeCodeImmune: true, TypeCodeNotVeryEffective: true, TypeCodeNeutral: true, TypeCodeSuperEffective: true}
	pairs := 0
	for _, atk := range f.Types {
		row, ok := f.Effectiveness[atk]
		if !ok {
			t.Fatalf("生成物なのに %q の行が無い(等倍の省略は手書き fixture だけ)", atk)
		}
		for _, def := range f.Types {
			code, ok := row[def]
			if !ok {
				t.Fatalf("生成物なのに %q → %q の組が無い", atk, def)
			}
			if !valid[code] {
				t.Fatalf("%q → %q のコード %d は 0/1/2/4 以外", atk, def, code)
			}
			got, err := chart.Code(atk, def)
			if err != nil {
				t.Fatalf("Code(%q, %q): %v", atk, def, err)
			}
			if got != code {
				t.Errorf("Code(%q, %q) = %d, fixture = %d", atk, def, got, code)
			}
			pairs++
		}
	}
	if pairs != typeChartTypeCount*typeChartTypeCount {
		t.Fatalf("照合した組数=%d want %d", pairs, typeChartTypeCount*typeChartTypeCount)
	}
}

func TestGoldenSpeciesStats(t *testing.T) {
	meta := readGoldenMetadata(t)
	if meta.Files["stats-species.jsonl.gz"].Count != meta.SpeciesCount*6*4*3 {
		t.Fatal("SP/nature boundary coverage incomplete")
	}
	failures := 0
	eachGoldenLine(t, meta, "stats-species.jsonl.gz", func(line []byte) {
		var v struct {
			ID         string     `json:"id"`
			Individual Individual `json:"individual"`
			Expected   Stats      `json:"expected"`
		}
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatal(err)
		}
		if err := v.Individual.Validate(); err != nil {
			t.Fatalf("%s invalid fixture: %v", v.ID, err)
		}
		if got := RealStats(v.Individual); got != v.Expected {
			failures++
			if failures <= 12 {
				t.Logf("%s stats=%+v want=%+v", v.ID, got, v.Expected)
			}
		}
	})
	if failures > 0 {
		t.Errorf("%d external stat oracle cases disagree", failures)
	}
}
