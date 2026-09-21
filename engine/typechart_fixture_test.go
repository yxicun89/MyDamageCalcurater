package engine

// テストがタイプ相性表(ADR-0013)を受け取るための共通 fixture とヘルパー。
//
// 方針(ADR-0013 §P1-13.5):
//   - engine のテストは表を**データから読む**。engine 側に「正しい表」を書き戻さない。
//   - 正の表は oracle(@smogon/calc)が生成する testdata/golden/typechart.json ただ1つ。
//     単体テスト(make test)もゴールデン(-tags golden)も同じファイルを読む。
//   - 読み込み時に metadata.json の sha256 と突き合わせる。手書きで差し替えると落ちる
//     = 生成器(tools/golden)を通すことが強制される。
//   - fixture が無いときは**スキップせず落とす**(TestTypeChartFixtureIsAvailable)。
//     「前提が足りないテストを黙ってスキップしない」(docs/coding-rules.md §5)。
//
// 引き方(ルール)そのものを見るテストは、この表ではなく小さな独立 fixture
// (smallTypeChartData)で書く。実装の写しにしないため。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const (
	goldenDir          = "../testdata/golden"
	typeChartFixture   = "typechart.json"
	typeChartTypeCount = 18 // 第9世代 / チャンピオンズの18タイプ
)

// typeChartFixtureFile は testdata/golden/typechart.json の形(ADR-0013 §P1-13.5)。
type typeChartFixtureFile struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Source        string                `json:"source"`
	Version       string                `json:"version"`
	Generation    int                   `json:"generation"`
	Types         []Type                `json:"types"`
	Effectiveness map[Type]map[Type]int `json:"effectiveness"`
}

// readTypeChartFixture は fixture を読み、metadata.json の sha256 と突き合わせて返す。
func readTypeChartFixture() (typeChartFixtureFile, error) {
	var out typeChartFixtureFile

	metaRaw, err := os.ReadFile(filepath.Join(goldenDir, "metadata.json"))
	if err != nil {
		return out, fmt.Errorf("golden の metadata.json を読めない(cd tools/golden && npm ci && npm run generate): %w", err)
	}
	var meta struct {
		Files map[string]struct {
			Count  int    `json:"count"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return out, err
	}
	spec, ok := meta.Files[typeChartFixture]
	if !ok {
		return out, fmt.Errorf("%s が metadata.json の files に無い(tools/golden が相性表を出力していない)", typeChartFixture)
	}

	raw, err := os.ReadFile(filepath.Join(goldenDir, typeChartFixture))
	if err != nil {
		return out, fmt.Errorf("%s を読めない(cd tools/golden && npm ci && npm run generate): %w", typeChartFixture, err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != spec.SHA256 {
		return out, fmt.Errorf("%s の sha256 が metadata.json と違う(表と manifest は同時に再生成すること)", typeChartFixture)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("%s の JSON が壊れている: %w", typeChartFixture, err)
	}
	if n := len(out.Types) * len(out.Types); n != spec.Count {
		return out, fmt.Errorf("%s の組数 %d が metadata.json の count %d と違う", typeChartFixture, n, spec.Count)
	}
	return out, nil
}

// loadGoldenTypeChart は fixture から検証済みの表を作る。結果はプロセス内で1度だけ計算する。
var loadGoldenTypeChart = sync.OnceValues(func() (TypeChart, error) {
	f, err := readTypeChartFixture()
	if err != nil {
		return TypeChart{}, err
	}
	return NewTypeChart(TypeChartData{Types: f.Types, Effectiveness: f.Effectiveness})
})

// typeChartForTests は既存テストが入力に載せる相性表を返す。
// 読めないときはゼロ値を返す(ここでは落とさず、TestTypeChartFixtureIsAvailable が
// 理由付きで落とす。ヘルパーから t を持ち回らずに済ませるため)。
func typeChartForTests() TypeChart {
	c, _ := loadGoldenTypeChart()
	return c
}

// mustTypeChart は fixture の表を返し、読めなければそのテストを落とす。
func mustTypeChart(t *testing.T) TypeChart {
	t.Helper()
	c, err := loadGoldenTypeChart()
	if err != nil {
		t.Fatalf("相性表の fixture を読めない: %v", err)
	}
	return c
}

// --- 表を補う呼び出しラッパ(表の指定を1か所に集約する)----------------------
//
// 既存のテストは入力に表を持っていないので、ここで「未設定なら fixture の表を補う」。
// 表そのものを検証するテスト(未設定・未知タイプ・別の表)は、ラッパではなく
// CalcDamage / CalcBulk / CalcReverse を直接呼ぶ。

func calcDamage(in DamageInput) (DamageResult, error) {
	if in.TypeChart.IsZero() {
		in.TypeChart = typeChartForTests()
	}
	return CalcDamage(in)
}

func calcBulk(in BulkInput) (BulkResult, error) {
	if in.TypeChart.IsZero() {
		in.TypeChart = typeChartForTests()
	}
	return CalcBulk(in)
}

func calcReverse(in ReverseInput) (ReverseResult, error) {
	if in.TypeChart.IsZero() {
		in.TypeChart = typeChartForTests()
	}
	return CalcReverse(in)
}

// --- 小さな独立 fixture(引き方のルールを見る)-------------------------------

// 独立 fixture のタイプ ID。実在のタイプ名を使わないのは、
// engine が「18 タイプ」を知らないこと(表は完全に入力であること)を示すため。
const (
	typeAlpha Type = "alpha"
	typeBeta  Type = "beta"
	typeGamma Type = "gamma"
	typeDelta Type = "delta"
	// typeEpsilon は表に載せない未知タイプ。
	typeEpsilon Type = "epsilon"
)

// smallTypeChartData は引き方の検証に使う独立 fixture(実装の写しではない)。
//
//	        → beta   gamma  delta  alpha
//	alpha     抜群   抜群   無効   (省略=等倍)
//	beta      半減   半減   (省略) (省略)
//	gamma     行ごと省略(すべて等倍)
//	delta     抜群   (省略) (省略) 半減
func smallTypeChartData() TypeChartData {
	return TypeChartData{
		Types: []Type{typeAlpha, typeBeta, typeGamma, typeDelta},
		Effectiveness: map[Type]map[Type]int{
			typeAlpha: {typeBeta: TypeCodeSuperEffective, typeGamma: TypeCodeSuperEffective, typeDelta: TypeCodeImmune},
			typeBeta:  {typeBeta: TypeCodeNotVeryEffective, typeGamma: TypeCodeNotVeryEffective},
			typeDelta: {typeBeta: TypeCodeSuperEffective, typeAlpha: TypeCodeNotVeryEffective},
		},
	}
}

// smallTypeChart は smallTypeChartData から検証済みの表を作る。
func smallTypeChart(t *testing.T) TypeChart {
	t.Helper()
	c, err := NewTypeChart(smallTypeChartData())
	if err != nil {
		t.Fatalf("独立 fixture の表を作れない: %v", err)
	}
	return c
}

// TestTypeChartFixtureIsAvailable は相性表の fixture が使えることを明示的に確かめる。
// 他のテストはヘルパー経由で静かにゼロ値を受け取るため、ここで理由付きで落とす。
func TestTypeChartFixtureIsAvailable(t *testing.T) {
	f, err := readTypeChartFixture()
	if err != nil {
		t.Fatalf("%v", err)
	}
	// P2-1b(ADR-0002 §決定5 の追記): 相性表は @smogon/calc 0.12.0 の Champions 世代(Generations.get(0))
	// から生成する。gen9 と一致することは生成器(tools/golden/generate.mjs)が確認済み。
	if f.SchemaVersion != 1 || f.Source != "@smogon/calc" || f.Version != "0.12.0" || f.Generation != 0 {
		t.Fatalf("相性表の出どころが想定と違う: %+v", struct {
			SchemaVersion   int
			Source, Version string
			Generation      int
		}{f.SchemaVersion, f.Source, f.Version, f.Generation})
	}
	if len(f.Types) != typeChartTypeCount {
		t.Fatalf("タイプ数 = %d, want %d", len(f.Types), typeChartTypeCount)
	}
	if _, err := loadGoldenTypeChart(); err != nil {
		t.Fatalf("fixture から相性表を作れない: %v", err)
	}
}
