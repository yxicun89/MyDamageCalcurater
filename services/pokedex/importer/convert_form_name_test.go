package importer_test

// 種族の姿(地方の姿・時間帯・天気・性別など)の日本語名(issue #607・ADR-0141)。
// 優先順は 上書き > 上流の完全名(pokemon_name)> 上流の姿の名前(form_name)から「基本種名（姿の名前）」を生成 > 英語名。
// 名前の表は持たない(取得元の form_name から導く)。

import (
	"errors"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

// setFormNames は PokeAPI のフォームの完全名(names)と姿の名前(formNames)を差し替える。
func setFormNames(t *testing.T, in *importer.Input, slug string, names, formNames map[string]string) {
	t.Helper()
	for i := range in.PokeAPI.Forms {
		if in.PokeAPI.Forms[i].Slug == slug {
			in.PokeAPI.Forms[i].Names = names
			in.PokeAPI.Forms[i].FormNames = formNames
			return
		}
	}
	t.Fatalf("PokeAPI のフォーム %q が fixture に無い", slug)
}

func TestConvertGeneratesFormNameJa(t *testing.T) {
	empty := map[string]string{"ja-Hrkt": "", "ja": ""}
	tests := []struct {
		name       string
		mutate     func(t *testing.T, in *importer.Input)
		wantName   string
		wantSource string
	}{
		{"完全名が無く姿の名前だけ: 基本種名（姿の名前）", func(t *testing.T, in *importer.Input) {
			setFormNames(t, in, "testleaf-rain", empty, map[string]string{"ja": "あめのすがた", "ja-Hrkt": "あめのすがた"})
		}, "テストリーフ（あめのすがた）", "generated"},
		{"姿の名前は languages の優先順で選ぶ", func(t *testing.T, in *importer.Input) {
			setFormNames(t, in, "testleaf-rain", empty, map[string]string{"ja": "", "ja-Hrkt": "かな"})
		}, "テストリーフ（かな）", "generated"},
		{"上流の完全名がある: それを優先し生成しない", func(t *testing.T, in *importer.Input) {
			setFormNames(t, in, "testleaf-rain", map[string]string{"ja": "テストリーフ雨"}, map[string]string{"ja": "あめのすがた"})
		}, "テストリーフ雨", "pokeapi"},
		{"上書きがある: 生成より優先", func(t *testing.T, in *importer.Input) {
			setFormNames(t, in, "testleaf-rain", empty, map[string]string{"ja": "あめのすがた"})
			in.Overrides.Species["testleafrain"] = "オーバーライド"
		}, "オーバーライド", "override"},
		{"姿の名前が無い: 推測せず英語名", func(t *testing.T, in *importer.Input) {
			setFormNames(t, in, "testleaf-rain", empty, map[string]string{})
		}, "Testleaf-Rain", "fallback_en"},
		{"基本種の日本語名が無い: 生成せず英語名", func(t *testing.T, in *importer.Input) {
			setFormNames(t, in, "testleaf-rain", empty, map[string]string{"ja": "あめのすがた"})
			dropPokeAPIName(in, "testleaf")
		}, "Testleaf-Rain", "fallback_en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := reconcileInput(t)
			delete(in.Overrides.Species, "testleafrain")
			tt.mutate(t, &in)
			out, rec := reconcileOK(t, in)
			got := speciesByShowdownID(out)["testleafrain"]
			if got.NameJa != tt.wantName || got.NameJaSource != tt.wantSource {
				t.Errorf("(%q, %q), want (%q, %q)", got.NameJa, got.NameJaSource, tt.wantName, tt.wantSource)
			}
			if gen := hasFinding(rec.Report.Warnings, importer.KindNameGenerated, "testleafrain"); gen != (tt.wantSource == "generated") {
				t.Errorf("name-generated の警告の有無 = %v, source %q", gen, tt.wantSource)
			}
			if fb := containsString(rec.Names["species"].FallbackIDs, "testleafrain"); fb != (tt.wantSource == "fallback_en") {
				t.Errorf("fallbackIds への載り方 = %v, source %q", fb, tt.wantSource)
			}
			if gen := containsString(rec.Names["species"].GeneratedIDs, "testleafrain"); gen != (tt.wantSource == "generated") {
				t.Errorf("generatedIds への載り方 = %v, source %q", gen, tt.wantSource)
			}
		})
	}
}

// 生成の規則を上流の完全名で検証する: 完全名と姿の名前の両方がある姿で、規則の結果が完全名と違えば
// 警告 name-form-rule-mismatch(止めない。名前は完全名を使う)。同じなら警告しない。
func TestFormNameRuleIsCheckedAgainstUpstream(t *testing.T) {
	tests := []struct {
		name     string
		full     string
		wantWarn bool
	}{
		{"規則の結果と完全名が一致", "テストリーフ（あめのすがた）", false},
		{"規則の結果と完全名が違う", "テストリーフ(あめのすがた)", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := reconcileInput(t)
			delete(in.Overrides.Species, "testleafrain")
			setFormNames(t, &in, "testleaf-rain", map[string]string{"ja": tt.full}, map[string]string{"ja": "あめのすがた"})
			out, rec := reconcileOK(t, in)
			got := speciesByShowdownID(out)["testleafrain"]
			if got.NameJa != tt.full || got.NameJaSource != "pokeapi" {
				t.Errorf("(%q, %q), want 完全名・pokeapi", got.NameJa, got.NameJaSource)
			}
			if w := hasFinding(rec.Report.Warnings, importer.KindFormNameRuleMismatch, "testleafrain"); w != tt.wantWarn {
				t.Errorf("name-form-rule-mismatch の警告 = %v, want %v", w, tt.wantWarn)
			}
		})
	}
}

// formNames が無い古い PokeAPI スナップショットは、黙って読まず取り直しを案内して拒否する(ADR-0136・0140 と同じ作法)。
func TestDecodePokeAPISnapshotRejectsMissingFormNames(t *testing.T) {
	old := []byte(`{"schemaVersion":1,"source":"pokeapi","version":"v","species":[],"forms":[{"slug":"a-b","names":{"ja":"x"}}],
"moves":[],"items":[],"abilities":[],"types":[],"natures":[]}`)
	if _, err := importer.DecodePokeAPISnapshot(old); !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	ok := []byte(`{"schemaVersion":1,"source":"pokeapi","version":"v","species":[],"forms":[{"slug":"a-b","names":{"ja":"x"},"formNames":{}}],
"moves":[],"items":[],"abilities":[],"types":[],"natures":[]}`)
	if _, err := importer.DecodePokeAPISnapshot(ok); err != nil {
		t.Fatalf("err = %v", err)
	}
}
