package importer_test

// メガ種族の日本語名の生成(issue #515・ADR-0324)。上流(上書き・PokeAPI)に日本語名があればそれを使い、
// 無いときだけ「メガ + 基本種の日本語名 + フォーム識別子(Mega-X の X)」から機械的に作る。名前の表は持たない。

import (
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

// dropPokeAPIName は PokeAPI の種族・フォームの名前を空にする(上流に日本語名が無い状態にする)。
func dropPokeAPIName(in *importer.Input, slug string) {
	for i := range in.PokeAPI.Species {
		if in.PokeAPI.Species[i].Slug == slug {
			in.PokeAPI.Species[i].Names = map[string]string{"ja-Hrkt": "", "ja": ""}
		}
	}
	for i := range in.PokeAPI.Forms {
		if in.PokeAPI.Forms[i].Slug == slug {
			in.PokeAPI.Forms[i].Names = map[string]string{"ja-Hrkt": "", "ja": ""}
		}
	}
}

func TestConvertGeneratesMegaNameJa(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(t *testing.T, in *importer.Input)
		wantName   string
		wantSource string
		wantGen    bool
	}{
		{"上流に名前が無い: メガ + 基本種名", func(t *testing.T, in *importer.Input) { dropPokeAPIName(in, "testmon-mega") },
			"メガテストモン", "generated", true},
		{"2形態の X", func(t *testing.T, in *importer.Input) {
			dropPokeAPIName(in, "testmon-mega")
			showdownSpecies(t, in, "testmonmega").Forme = "Mega-X"
		}, "メガテストモンX", "generated", true},
		{"2形態の Y", func(t *testing.T, in *importer.Input) {
			dropPokeAPIName(in, "testmon-mega")
			showdownSpecies(t, in, "testmonmega").Forme = "Mega-Y"
		}, "メガテストモンY", "generated", true},
		{"3形態目の Z", func(t *testing.T, in *importer.Input) {
			dropPokeAPIName(in, "testmon-mega")
			showdownSpecies(t, in, "testmonmega").Forme = "Mega-Z"
		}, "メガテストモンZ", "generated", true},
		{"上流(PokeAPI)に名前がある: それを優先し生成しない", func(t *testing.T, in *importer.Input) {
			showdownSpecies(t, in, "testmonmega").Forme = "Mega-X"
		}, "テストメガモン", "pokeapi", false},
		{"上書き設定がある: 生成より優先", func(t *testing.T, in *importer.Input) {
			dropPokeAPIName(in, "testmon-mega")
			in.Overrides.Species["testmonmega"] = "オーバーライドメガ"
		}, "オーバーライドメガ", "override", false},
		{"基本種の日本語名が無い: 生成せず英語名のまま", func(t *testing.T, in *importer.Input) {
			dropPokeAPIName(in, "testmon-mega")
			dropPokeAPIName(in, "testmon")
		}, "Testmon-Mega", "fallback_en", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := reconcileInput(t)
			if in.Overrides.Species == nil {
				in.Overrides.Species = map[string]string{}
			}
			tt.mutate(t, &in)
			out, rec := reconcileOK(t, in)
			got := speciesByShowdownID(out)["testmonmega"]
			if got.NameJa != tt.wantName || got.NameJaSource != tt.wantSource {
				t.Errorf("(%q, %q), want (%q, %q)", got.NameJa, got.NameJaSource, tt.wantName, tt.wantSource)
			}
			if hasFinding(rec.Report.Warnings, importer.KindNameGenerated, "testmonmega") != tt.wantGen {
				t.Errorf("name-generated の警告の有無が違う(want %v)", tt.wantGen)
			}
			wantCount := 0
			if tt.wantGen {
				wantCount = 1
			}
			if g := rec.Names["species"].Generated; g != wantCount {
				t.Errorf("report の species.generated = %d, want %d", g, wantCount)
			}
		})
	}
}

// メガでない種族は、名前が無くても生成しない(生成はメガ種族だけ)。
func TestConvertDoesNotGenerateNameForNonMega(t *testing.T) {
	in := reconcileInput(t)
	dropPokeAPIName(&in, "testleaf")
	out, rec := reconcileOK(t, in)
	if got := speciesByShowdownID(out)["testleaf"]; got.NameJaSource != "fallback_en" {
		t.Errorf("testleaf の source = %q, want fallback_en", got.NameJaSource)
	}
	if rec.Names["species"].Generated != 0 {
		t.Errorf("生成件数 = %d, want 0", rec.Names["species"].Generated)
	}
}
