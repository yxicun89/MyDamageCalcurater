package importer_test

// 性格(natures)の取得・変換のテスト(ADR-0105 §4)。入力は testdata/fictional の架空データだけ。
// 補正の正は Showdown の natures、calc の natures は突き合わせ(不一致は Blocker)、日本語名は
// override > PokeAPI(config の nameJaLanguages の順)> 英語名(fallback_en)で、他のテーブルと同じ規則。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func naturesByID(out importer.Output) map[string]importer.NatureRow {
	m := map[string]importer.NatureRow{}
	for _, r := range out.Natures {
		m[r.ID] = r
	}
	return m
}

// AC-N1: Showdown の natures が ID 順の NatureRow になる。補正は Showdown の plus/minus、無補正は両方とも空文字(NULL)。
func TestConvertNatures(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))

	want := []importer.NatureRow{
		{ID: "testbrave", NameJa: "テストゆうかん", NameJaSource: "pokeapi", NameEn: "Testbrave", Plus: "atk", Minus: "spe"},
		{ID: "testcalm", NameJa: "テストおだやか", NameJaSource: "pokeapi", NameEn: "Testcalm", Plus: "spd", Minus: "atk"},
		{ID: "testneutral", NameJa: "Testneutral", NameJaSource: "fallback_en", NameEn: "Testneutral", Plus: "", Minus: ""},
		{ID: "testquiet", NameJa: "テストれいせい", NameJaSource: "override", NameEn: "Testquiet", Plus: "spa", Minus: "spe"},
	}
	if !reflect.DeepEqual(out.Natures, want) {
		t.Fatalf("Natures =\n%+v\nwant\n%+v", out.Natures, want)
	}
}

// AC-N1: 日本語名の出どころの報告(英語名へのフォールバック・使われない override)は他のテーブルと同じ。
func TestConvertNaturesNameFindings(t *testing.T) {
	_, rep := convertOK(t, loadFixture(t))
	if !hasFinding(rep.Warnings, importer.KindNameFallback, "testneutral") {
		t.Errorf("性格の英語名へのフォールバック(testneutral)が Warnings に無い")
	}
	if !hasFinding(rep.Warnings, importer.KindOverrideUnused, "testunknownnature") {
		t.Errorf("どの性格にも当たらない override(testunknownnature)が Warnings に無い")
	}
}

// AC-N1: nameJaLanguages の順は性格にも効く(ja を先にすると ja の名前)。
func TestConvertNaturesNameLanguageOrder(t *testing.T) {
	in := loadFixture(t)
	in.Config.NameJaLanguages = []string{"ja", "ja-Hrkt"}
	out, _ := convertOK(t, in)
	if got := naturesByID(out)["testbrave"]; got.NameJa != "テスト勇敢" || got.NameJaSource != "pokeapi" {
		t.Errorf("nameJaLanguages を ja 優先にしたのに (%q, %q)", got.NameJa, got.NameJaSource)
	}
}

// AC-N2: calc の natures との突き合わせ。補正の違い・片方にだけある性格は Blocker(KindNatureMismatch)で、
// Output はゼロ値(部分的な結果を投入に回さない)。calc の無補正は plus == minus の形で表される。
func TestConvertBlocksOnNatureMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(in *importer.Input)
		id     string
	}{
		{"補正が違う", func(in *importer.Input) {
			for i := range in.Calc.Natures {
				if in.Calc.Natures[i].Name == "Testbrave" {
					in.Calc.Natures[i].Minus = "spa"
				}
			}
		}, "testbrave"},
		{"calc で補正あり・Showdown で無補正", func(in *importer.Input) {
			for i := range in.Calc.Natures {
				if in.Calc.Natures[i].Name == "Testneutral" {
					in.Calc.Natures[i].Minus = "def"
				}
			}
		}, "testneutral"},
		{"calc にだけある", func(in *importer.Input) {
			in.Calc.Natures = append(in.Calc.Natures, importer.CalcNature{Name: "Testextra", Plus: "def", Minus: "spe"})
		}, "testextra"},
		{"Showdown にだけある", func(in *importer.Input) {
			var kept []importer.CalcNature
			for _, n := range in.Calc.Natures {
				if n.Name != "Testcalm" {
					kept = append(kept, n)
				}
			}
			in.Calc.Natures = kept
		}, "testcalm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(&in)
			out, rep, err := importer.Convert(in)
			if !errors.Is(err, importer.ErrBlocked) {
				t.Fatalf("err = %v, want ErrBlocked", err)
			}
			if !hasFinding(rep.Blockers, importer.KindNatureMismatch, tt.id) {
				t.Errorf("Blockers に nature-mismatch(%s)が無い: %+v", tt.id, rep.Blockers)
			}
			if !reflect.DeepEqual(out, importer.Output{}) {
				t.Errorf("ErrBlocked なのに Output がゼロ値でない")
			}
		})
	}
}

// AC-N3: 取得元の natures として不正なものは ErrInvalidData(DB の CHECK・UNIQUE と同じ規則を投入前に持つ)。
func TestConvertRejectsInvalidNatures(t *testing.T) {
	setShowdown := func(id string, f func(n *importer.ShowdownNature)) func(in *importer.Input) {
		return func(in *importer.Input) {
			for i := range in.Showdown.Natures {
				if in.Showdown.Natures[i].ID == id {
					f(&in.Showdown.Natures[i])
				}
			}
		}
	}
	tests := []struct {
		name   string
		mutate func(in *importer.Input)
	}{
		{"Showdown の natures が空(古いスナップショット。取得し直す)", func(in *importer.Input) {
			in.Showdown.Natures = nil
			in.Calc.Natures = nil
		}},
		{"plus が HP", setShowdown("testbrave", func(n *importer.ShowdownNature) { n.Plus = "hp" })},
		{"minus が未知のステータス", setShowdown("testbrave", func(n *importer.ShowdownNature) { n.Minus = "speed" })},
		{"plus だけある", setShowdown("testneutral", func(n *importer.ShowdownNature) { n.Plus = "atk" })},
		{"plus と minus が同じ", setShowdown("testbrave", func(n *importer.ShowdownNature) { n.Minus = "atk" })},
		{"ID の形式が違う", setShowdown("testbrave", func(n *importer.ShowdownNature) { n.ID = "Test-Brave" })},
		{"ID が重複", func(in *importer.Input) {
			in.Showdown.Natures = append(in.Showdown.Natures, importer.ShowdownNature{ID: "testcalm", Name: "Testcalm", Plus: "def", Minus: "spe"})
		}},
		{"同じ補正の組が2つ(calc-svc の NatureID が引けなくなる)", func(in *importer.Input) {
			in.Showdown.Natures = append(in.Showdown.Natures, importer.ShowdownNature{ID: "testbrave2", Name: "Testbrave2", Plus: "atk", Minus: "spe"})
			in.Calc.Natures = append(in.Calc.Natures, importer.CalcNature{Name: "Testbrave2", Plus: "atk", Minus: "spe"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(&in)
			out, _, err := importer.Convert(in)
			if !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
			if !reflect.DeepEqual(out, importer.Output{}) {
				t.Errorf("失敗なのに Output がゼロ値でない")
			}
		})
	}
}

// AC-N1: 性格の件数(25)をコードに持たない。架空データの 4 件でも通る(上の TestConvertNatures)し、
// 1 件でも通る(無補正だけ)。
func TestConvertNaturesDoNotAssumeCount(t *testing.T) {
	in := loadFixture(t)
	in.Showdown.Natures = []importer.ShowdownNature{{ID: "testneutral", Name: "Testneutral"}}
	in.Calc.Natures = []importer.CalcNature{{Name: "Testneutral", Plus: "spe", Minus: "spe"}}
	out, _ := convertOK(t, in)
	if len(out.Natures) != 1 || out.Natures[0].Plus != "" || out.Natures[0].Minus != "" {
		t.Fatalf("Natures = %+v, want 無補正の1件", out.Natures)
	}
}
