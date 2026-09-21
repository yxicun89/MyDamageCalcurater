//go:build golden

package engine

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// UnmarshalJSON は input(engine に渡す DamageInput)だけを未知フィールド拒否で読む。
// case 側の oracle / expected.smogonKO は生成器の診断情報で engine の入力ではないため、そこは緩く読む。
// フィールド改名で入力が黙ってゼロ値になり「別の入力」と照合する事故を防ぐ(P1-6 改善要望。golden_scope_test.go)。
func (c *goldenCase) UnmarshalJSON(b []byte) error {
	var raw struct {
		ID       string          `json:"id"`
		Input    json.RawMessage `json:"input"`
		Expected json.RawMessage `json:"expected"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	c.ID = raw.ID
	if err := decodeStrictGolden(raw.Input, &c.Input); err != nil {
		return fmt.Errorf("%s: input: %w", raw.ID, err)
	}
	if len(raw.Expected) == 0 {
		return fmt.Errorf("%s: expected が無い", raw.ID)
	}
	return json.Unmarshal(raw.Expected, &c.Expected)
}

// goldenMetadata は testdata/golden/metadata.json(schemaVersion 2。P2-1b / ADR-0002 §決定 5 の追記)。
//
// oracle は2つ。どちらも同じ pin した @smogon/calc(0.12.0)で、世代だけが違う:
//   - champions: 主のゴールデン(種族網羅・ランダム・固定・実数値・相性表)。SP は直接渡す。
//   - gen9: Champions の mechanics に無い効果(持ち物・特性)の計算式を検証する legacy-effects。SP は max(0,8×SP−4) に換算。
type goldenMetadata struct {
	SchemaVersion int               `json:"schemaVersion"`
	Source        string            `json:"source"`
	Version       string            `json:"version"`
	Generation    string            `json:"generation"`
	Seed          uint32            `json:"seed"`
	SpeciesScope  string            `json:"speciesScope"`
	SpeciesCount  int               `json:"speciesCount"`
	Exclusions    []goldenExclusion `json:"exclusions"`
	Oracles       []goldenOracle    `json:"oracles"`
	Files         map[string]struct {
		Count  int    `json:"count"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

type goldenExclusion struct {
	Scope  string   `json:"scope"`
	Names  []string `json:"names"`
	Reason string   `json:"reason"`
}

// goldenOracle は1つの oracle(世代)と、その oracle が生成したファイル。
type goldenOracle struct {
	ID            string   `json:"id"`            // "champions" / "gen9-legacy-effects"
	Source        string   `json:"source"`        // "@smogon/calc"
	Version       string   `json:"version"`       // "0.12.0"(両方同じ pin)
	Generation    string   `json:"generation"`    // "champions" / "gen9"
	GenerationNum int      `json:"generationNum"` // Generations.get(n) の n。champions は 0
	SPInput       string   `json:"spInput"`       // "direct" / "max(0,8*SP-4)"
	Seed          *uint32  `json:"seed,omitempty"`
	Files         []string `json:"files"`
	// LegacyEffects は Champions 世代に存在しない効果(effects.json の名前)。gen9 oracle だけが持つ。
	LegacyEffects *struct {
		Items     []string `json:"items"`
		Abilities []string `json:"abilities"`
	} `json:"legacyEffects,omitempty"`
	Reason string `json:"reason,omitempty"`
}

const (
	goldenOracleVersion     = "0.12.0"
	goldenChampionsOracleID = "champions"
	goldenLegacyOracleID    = "gen9-legacy-effects"
	goldenLegacyEffectsFile = "legacy-effects.jsonl.gz"
	goldenSPToEVFormula     = "max(0,8*SP-4)"
	goldenChampionsSPDirect = "direct"
	goldenChampionsGenNum   = 0
	goldenGen9GenNum        = 9
)

func (m goldenMetadata) oracle(t *testing.T, id string) goldenOracle {
	t.Helper()
	for _, o := range m.Oracles {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("metadata.json の oracles に %q が無い(P2-1b: champions と gen9-legacy-effects の2つを記録する)", id)
	return goldenOracle{}
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
	// P2-1b: oracle を @smogon/calc 0.12.0 の Champions 世代へ切り替え(ADR-0002 §決定 5 の追記)。
	// 旧値(schemaVersion 1 / 0.10.0 / gen9)からの変更理由: 種族集合と SP の渡し方が Champions の定義になるため。
	// speciesCount は 0 でないことだけでなく現実的な範囲で守る(golden_scope_test.go の定数の注記)。
	if meta.SchemaVersion != 2 || meta.Source != "@smogon/calc" || meta.Version != goldenOracleVersion || meta.Generation != goldenChampionsOracleID || meta.Seed != 0x504f4b45 {
		t.Fatalf("unexpected oracle metadata: schemaVersion=%d source=%q version=%q generation=%q seed=%#x", meta.SchemaVersion, meta.Source, meta.Version, meta.Generation, meta.Seed)
	}
	if meta.SpeciesCount < goldenMinSpeciesCount || meta.SpeciesCount > goldenMaxSpeciesCount {
		t.Fatalf("speciesCount=%d は Champions 集合として範囲外 [%d, %d](基底種だけ・空・gen9 参考集合への逆戻りを疑う)", meta.SpeciesCount, goldenMinSpeciesCount, goldenMaxSpeciesCount)
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
	//
	// 固定ケースの下限 200 は「fixed.json 単独」から「fixed.json + legacy-effects の固定部分」に変えた(P2-1b)。
	// Champions に無い効果(こだわり系・チョッキ・しんかのきせき・はがねつかい)の固定シナリオは削除せず、
	// gen9 で照合する legacy-effects へ移すため(ADR-0002 §決定 7)。シナリオが消えていないことは
	// TestGoldenFixedScenariosPreserved がラベル単位で守る。
	legacyFixed := countGoldenFixedInLegacy(t, meta)
	if meta.Files["random.jsonl.gz"].Count != 10000 || meta.Files["fixed.json"].Count+legacyFixed < 200 || meta.Files["attack-species.jsonl.gz"].Count != meta.SpeciesCount*20 || meta.Files["defense-species.jsonl.gz"].Count != meta.SpeciesCount*40 {
		t.Fatal("golden coverage incomplete")
	}
	if meta.Files[goldenLegacyEffectsFile].Count == 0 {
		t.Fatalf("%s が空(Champions に無い効果の計算式が検証されていない)", goldenLegacyEffectsFile)
	}
	// legacy-effects も全件一致を要求する(known_diffs には何も足さない。P2-1b 決定 5)。
	for _, name := range []string{"fixed.json", "random.jsonl.gz", "attack-species.jsonl.gz", "defense-species.jsonl.gz", goldenLegacyEffectsFile} {
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
	// P2-1b: 相性表は Champions 世代(Generations.get(0))から出す。gen9 と同一であることは第1段階で確認済み。
	champions := meta.oracle(t, goldenChampionsOracleID)
	if f.Source != "@smogon/calc" || f.Version != meta.Version || f.Generation != champions.GenerationNum || f.Generation != goldenChampionsGenNum {
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
		// individual も未知フィールド拒否で読み直す(SP のキー改名でゼロ値のまま照合しないため)。
		var raw struct {
			Individual json.RawMessage `json:"individual"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			t.Fatal(err)
		}
		if err := decodeStrictGolden(raw.Individual, &v.Individual); err != nil {
			t.Fatalf("%s: individual: %v", v.ID, err)
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
