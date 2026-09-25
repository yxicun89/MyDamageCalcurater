package importer_test

// 相性表の照合(issue #280・ADR-0118)。importer が calc スナップショットから作る相性表(DB の
// types/type_chart の正)と、ゴールデンテスト・balance・Web が使う testdata/golden/typechart.json
// (参照の相性表)が食い違ったまま取り込まれないことを確かめる。
//
// 実データは読まない。比較器は架空データ(testdata/fictional と testdata/fictional-reference)で確かめ、
// 形式の検証だけはコミット済みの testdata/golden/typechart.json(英語のタイプ ID と整数だけ)を読む。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

// fictionalReferenceTypeChartPath は架空データの参照の相性表(testdata/fictional の calc と一致する)。
var fictionalReferenceTypeChartPath = filepath.Join("testdata", "fictional-reference", "typechart.json")

// goldenTypeChartPath はコミット済みのゴールデンの相性表(本番の import が照合する参照)。
var goldenTypeChartPath = filepath.Join("..", "..", "..", "testdata", "golden", "typechart.json")

func loadFictionalReference(t *testing.T) *importer.ReferenceTypeChart {
	t.Helper()
	ref, err := importer.LoadReferenceTypeChart(fictionalReferenceTypeChartPath)
	if err != nil {
		t.Fatalf("LoadReferenceTypeChart(%s): %v", fictionalReferenceTypeChartPath, err)
	}
	return ref
}

// fixtureTypeChart は架空データを変換した相性表(types と等倍を省いた行)。
func fixtureTypeChart(t *testing.T) ([]master.TypeRow, []master.TypeChartRow) {
	t.Helper()
	out, _ := convertOK(t, loadFixture(t))
	types := make([]master.TypeRow, 0, len(out.Types))
	for _, r := range out.Types {
		types = append(types, r.TypeRow)
	}
	return types, out.TypeChart
}

func TestDecodeReferenceTypeChartAcceptsGolden(t *testing.T) {
	raw, err := os.ReadFile(goldenTypeChartPath)
	if err != nil {
		t.Fatalf("ゴールデンの相性表が読めない(スキップしない): %v", err)
	}
	ref, err := importer.DecodeReferenceTypeChart(raw)
	if err != nil {
		t.Fatalf("DecodeReferenceTypeChart(golden): %v", err)
	}
	if len(ref.Types) == 0 || len(ref.Effectiveness) != len(ref.Types) {
		t.Errorf("types = %d, effectiveness = %d(全タイプの行が要る)", len(ref.Types), len(ref.Effectiveness))
	}
}

func TestDecodeReferenceTypeChartRejectsInvalid(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"schemaVersion": 1,
			"source":        "test-calc",
			"version":       "test-calc-1",
			"generation":    0,
			"note":          "n",
			"excludedTypes": []string{"???"},
			"types":         []string{"fire", "water"},
			"effectiveness": map[string]map[string]int{
				"fire":  {"fire": 1, "water": 1},
				"water": {"fire": 4, "water": 1},
			},
		}
	}
	tests := []struct {
		name   string
		mutate func(m map[string]any)
	}{
		{"schemaVersion が違う", func(m map[string]any) { m["schemaVersion"] = 2 }},
		{"未知のフィールド", func(m map[string]any) { m["extra"] = true }},
		{"version が空", func(m map[string]any) { m["version"] = "" }},
		{"types が空", func(m map[string]any) {
			m["types"] = []string{}
			m["effectiveness"] = map[string]map[string]int{}
		}},
		{"types の重複", func(m map[string]any) { m["types"] = []string{"fire", "fire"} }},
		{"攻撃側の行が無い", func(m map[string]any) {
			m["effectiveness"] = map[string]map[string]int{"fire": {"fire": 1, "water": 1}}
		}},
		{"攻撃側に types に無いタイプ", func(m map[string]any) {
			e := m["effectiveness"].(map[string]map[string]int)
			e["grass"] = map[string]int{"fire": 2, "water": 2}
		}},
		{"防御側の欠け", func(m map[string]any) {
			e := m["effectiveness"].(map[string]map[string]int)
			e["water"] = map[string]int{"fire": 4}
		}},
		{"防御側に types に無いタイプ", func(m map[string]any) {
			e := m["effectiveness"].(map[string]map[string]int)
			e["water"] = map[string]int{"fire": 4, "water": 1, "grass": 1}
		}},
		{"倍率コードが範囲外", func(m map[string]any) {
			e := m["effectiveness"].(map[string]map[string]int)
			e["water"]["fire"] = 3
		}},
	}
	// 基準の形が通ることを先に確かめる(誤って全部を拒否していないこと)。
	baseRaw, err := json.Marshal(base())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importer.DecodeReferenceTypeChart(baseRaw); err != nil {
		t.Fatalf("基準の形を拒否した: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.mutate(m)
			raw, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := importer.DecodeReferenceTypeChart(raw); !errors.Is(err, importer.ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLoadReferenceTypeChartMissingFile(t *testing.T) {
	_, err := importer.LoadReferenceTypeChart(filepath.Join(t.TempDir(), "typechart.json"))
	if !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput(参照が無いまま照合を飛ばさない)", err)
	}
}

func TestCompareReferenceTypeChart(t *testing.T) {
	types, chart := fixtureTypeChart(t)

	t.Run("一致すれば指摘なし", func(t *testing.T) {
		ref := loadFictionalReference(t)
		if got := importer.CompareReferenceTypeChart(*ref, "test-calc-1", types, chart); len(got) != 0 {
			t.Fatalf("指摘 = %+v, want なし", got)
		}
	})

	t.Run("倍率の食い違い(非等倍の組)", func(t *testing.T) {
		ref := loadFictionalReference(t)
		ref.Effectiveness["fire"]["grass"] = 2
		got := importer.CompareReferenceTypeChart(*ref, "test-calc-1", types, chart)
		if !hasFinding(got, importer.KindTypeChartReferenceMismatch, "fire>grass") || len(got) != 1 {
			t.Fatalf("指摘 = %+v, want fire>grass の1件", got)
		}
	})

	t.Run("倍率の食い違い(importer 側で省いた等倍の組)", func(t *testing.T) {
		ref := loadFictionalReference(t)
		ref.Effectiveness["normal"]["water"] = 1
		got := importer.CompareReferenceTypeChart(*ref, "test-calc-1", types, chart)
		if !hasFinding(got, importer.KindTypeChartReferenceMismatch, "normal>water") || len(got) != 1 {
			t.Fatalf("指摘 = %+v, want normal>water の1件", got)
		}
	})

	t.Run("タイプの集合の食い違い", func(t *testing.T) {
		ref := loadFictionalReference(t)
		ref.Types = []string{"fire", "grass", "water"}
		delete(ref.Effectiveness, "normal")
		for _, row := range ref.Effectiveness {
			delete(row, "normal")
		}
		got := importer.CompareReferenceTypeChart(*ref, "test-calc-1", types, chart)
		if !hasFinding(got, importer.KindTypeChartReferenceMismatch, "types") {
			t.Fatalf("指摘 = %+v, want types", got)
		}
	})

	t.Run("calc の版の食い違い", func(t *testing.T) {
		ref := loadFictionalReference(t)
		got := importer.CompareReferenceTypeChart(*ref, "test-calc-2", types, chart)
		if !hasFinding(got, importer.KindTypeChartReferenceMismatch, "version") || len(got) != 1 {
			t.Fatalf("指摘 = %+v, want version の1件", got)
		}
	})
}

func TestReconcileRequiresReferenceTypeChart(t *testing.T) {
	in := reconcileInput(t)
	in.ReferenceTypeChart = nil
	if _, _, err := importer.Reconcile(in); !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput(参照の相性表が無いまま照合を飛ばさない)", err)
	}
}

func TestReconcileBlocksOnReferenceTypeChartMismatch(t *testing.T) {
	in := reconcileInput(t)
	in.ReferenceTypeChart.Effectiveness["water"]["fire"] = 2
	out, rec, err := importer.Reconcile(in)
	if !errors.Is(err, importer.ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if !rec.Partial {
		t.Error("Partial = false, want true")
	}
	if !hasFinding(rec.Report.Blockers, importer.KindTypeChartReferenceMismatch, "water>fire") {
		t.Errorf("Blockers = %+v, want water>fire", rec.Report.Blockers)
	}
	if rec.Summary.BlockerCounts[importer.KindTypeChartReferenceMismatch] != 1 {
		t.Errorf("BlockerCounts = %+v", rec.Summary.BlockerCounts)
	}
	if len(out.TypeChart) != 0 {
		t.Error("止めたのに Output が空でない(部分的な結果を投入に回さない)")
	}
}
