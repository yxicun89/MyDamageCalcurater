package readmodel_test

// pokedex export の type-chart.json(DB の type_chart から作るタイプ相性表。issue #259-a・issue #403 パッケージ D20・
// ADR-0128)のテスト。形は services/balance/schema/type-chart.schema.json(タイプバランスレーンの持ち物。読むだけ)。
// balance の読み込み(BALANCE_TYPE_CHART_PATH)はタイプバランスレーンが後で足す。ここでは export の出力だけを確かめる。

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// goldenTypeChartPath は @smogon/calc 0.12.0 の Champions 世代から生成した相性表(testdata/golden。ADR-0002 の例外で
// コミット済み)。balance が埋め込んでいる表と同じバイト列。
const goldenTypeChartPath = "../../../../testdata/golden/typechart.json"

// typeChartFile は type-chart.schema.json の形(未知のフィールドは strict で拒否する)。
type typeChartFile struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Source        string                    `json:"source"`
	Version       string                    `json:"version"`
	Generation    *int                      `json:"generation"`
	Note          string                    `json:"note"`
	ExcludedTypes []string                  `json:"excludedTypes"`
	Types         []string                  `json:"types"`
	Effectiveness map[string]map[string]int `json:"effectiveness"`
}

// AC-T1: 架空データ(4タイプ)の表。types は ID 昇順、effectiveness は types × types の全組(DB に無い組は等倍 2)。
// source は "pokedex"、version は metadata.json と同じ dataVersion、excludedTypes は空配列(null にしない)。
func TestExportTypeChart(t *testing.T) {
	files, _ := export(t, storetest.New())
	var f typeChartFile
	strict(t, files.TypeChart, &f)
	if f.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", f.SchemaVersion)
	}
	if f.Source != "pokedex" {
		t.Errorf("source = %q, want pokedex", f.Source)
	}
	if f.Version != fixtureDataVersion {
		t.Errorf("version = %q, want dataVersion %q", f.Version, fixtureDataVersion)
	}
	if f.Generation == nil {
		t.Error("generation が無い(schema の必須)")
	}
	if f.Note == "" {
		t.Error("note が空(コードの意味 0/1/2/4 を書く)")
	}
	if f.ExcludedTypes == nil || len(f.ExcludedTypes) != 0 {
		t.Errorf("excludedTypes = %#v, want [](DB には除外タイプが入らない)", f.ExcludedTypes)
	}
	if bytes.Contains(files.TypeChart, []byte(`"excludedTypes":null`)) {
		t.Error("excludedTypes が null")
	}
	if want := []string{"fire", "grass", "normal", "water"}; !reflect.DeepEqual(f.Types, want) {
		t.Errorf("types = %v, want %v(ID 昇順)", f.Types, want)
	}
	want := map[string]map[string]int{
		"fire":   {"fire": 2, "grass": 4, "normal": 2, "water": 1},
		"grass":  {"fire": 1, "grass": 2, "normal": 2, "water": 4},
		"normal": {"fire": 2, "grass": 2, "normal": 2, "water": 2},
		"water":  {"fire": 4, "grass": 2, "normal": 2, "water": 2},
	}
	if !reflect.DeepEqual(f.Effectiveness, want) {
		t.Errorf("effectiveness = %v, want %v(全組。DB に無い組は等倍 2)", f.Effectiveness, want)
	}
}

// goldenTypeChart は testdata/golden/typechart.json を読む。
func goldenTypeChart(t *testing.T) typeChartFile {
	t.Helper()
	raw, err := os.ReadFile(goldenTypeChartPath)
	if err != nil {
		t.Fatalf("golden の相性表を読めない: %v", err)
	}
	var g typeChartFile
	strict(t, raw, &g)
	return g
}

// querierFromGolden は golden の表を、importer と同じく等倍の組を省いた type_chart の行にした偽の DB を返す
// (types・type_chart 以外は storetest の架空データのまま)。行の順はわざと golden と逆にする。
func querierFromGolden(t *testing.T, g typeChartFile) *storetest.Querier {
	t.Helper()
	q := storetest.New()
	q.Types = nil
	for i, id := range g.Types {
		q.Types = append(q.Types, store.Type{ID: id, SortOrder: uint16(i + 1), NameJa: "テスト" + id, NameJaSource: "override"})
	}
	reverse(q.Types)
	q.TypeChart = nil
	attacks := make([]string, 0, len(g.Effectiveness))
	for a := range g.Effectiveness {
		attacks = append(attacks, a)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(attacks)))
	for _, a := range attacks {
		defenses := make([]string, 0, len(g.Effectiveness[a]))
		for d := range g.Effectiveness[a] {
			defenses = append(defenses, d)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(defenses)))
		for _, d := range defenses {
			code := g.Effectiveness[a][d]
			if code == 2 {
				continue // importer は等倍の組を省く(services/pokedex/importer/convert_types.go)
			}
			q.TypeChart = append(q.TypeChart, store.TypeChart{AttackType: a, DefenseType: d, Code: uint8(code)})
		}
	}
	return q
}

// AC-T2: golden と同じ表を DB に入れて export すると、types と effectiveness が golden と完全に一致し、
// balance の type-chart.schema.json に合う(balance の埋め込みと export の表を同じものとして突き合わせられる。issue #259)。
func TestExportTypeChartMatchesGolden(t *testing.T) {
	g := goldenTypeChart(t)
	files, _ := export(t, querierFromGolden(t, g))
	if err := validate(t, compileSchema(t, "type-chart.schema.json"), files.TypeChart); err != nil {
		t.Fatalf("type-chart.schema.json に合わない: %v\n%s", err, files.TypeChart)
	}
	var f typeChartFile
	strict(t, files.TypeChart, &f)
	if !reflect.DeepEqual(f.Types, g.Types) {
		t.Errorf("types = %v, golden = %v", f.Types, g.Types)
	}
	if !reflect.DeepEqual(f.Effectiveness, g.Effectiveness) {
		for a, row := range g.Effectiveness {
			for d, code := range row {
				if got := f.Effectiveness[a][d]; got != code {
					t.Errorf("%s→%s = %d, golden = %d", a, d, got, code)
				}
			}
		}
		t.Fatalf("effectiveness が golden と一致しない")
	}
	if f.SchemaVersion != g.SchemaVersion {
		t.Errorf("schemaVersion = %d, golden = %d", f.SchemaVersion, g.SchemaVersion)
	}
}

// AC-T2 の空振り防止: schema が壊れた表(コード 3)を実際に拒否する。
func TestTypeChartSchemaCheckIsNotVacuous(t *testing.T) {
	g := goldenTypeChart(t)
	files, _ := export(t, querierFromGolden(t, g))
	var doc map[string]any
	if err := json.Unmarshal(files.TypeChart, &doc); err != nil {
		t.Fatalf("type-chart.json が JSON として読めない: %v\n%s", err, files.TypeChart)
	}
	eff, ok := doc["effectiveness"].(map[string]any)
	if !ok {
		t.Fatalf("effectiveness が無い\n%s", files.TypeChart)
	}
	row, ok := eff["fire"].(map[string]any)
	if !ok {
		t.Fatalf("effectiveness.fire が無い")
	}
	row["grass"] = 3
	broken, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(t, compileSchema(t, "type-chart.schema.json"), broken); err == nil {
		t.Fatal("コード 3 が schema を通った(検証が空振りしている)")
	}
}
