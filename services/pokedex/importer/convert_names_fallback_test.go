package importer_test

// 上流(PokeAPI の ja・ja-Hrkt)に日本語名が無い持ち物・特性・地方の姿の扱い(issue #607・ADR-0140)。データは架空。
//
//   - 推測で名前を作らない: 英語名のまま(name_ja_source = fallback_en)にし、report の names.<種類>.fallbackIds に ID を出す。
//   - 上書き設定(data/local/name_ja_overrides.json → Input.Overrides)があればそれを使い(source = override)、
//     fallbackIds から外れる。使われた上書きは override-unused にならない。
//   - 地方の姿(Alola など)は、上流に同じ接尾辞の姿の日本語名が1件も無く規則を検証できないので生成しない
//     (メガの名前の生成 ADR-0324 はメガ種族だけ)。

import (
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

// dropPokeAPINamed は PokeAPI の名前一覧(持ち物・特性)から slug の日本語名を空にする。
func dropPokeAPINamed(t *testing.T, entries []importer.PokeAPIName, slug string) {
	t.Helper()
	for i := range entries {
		if entries[i].Slug == slug {
			entries[i].Names = map[string]string{"ja-Hrkt": "", "ja": ""}
			return
		}
	}
	t.Fatalf("前提: PokeAPI に %q が無い", slug)
}

func namedRowByID(t *testing.T, rows []importer.NamedRow, id string) importer.NamedRow {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("%q が変換結果に無い", id)
	return importer.NamedRow{}
}

// AC-6: 日本語名の無い持ち物・特性は英語名のまま fallbackIds に出し、上書きで補える。
func TestNameFallbackAndOverrideForItemsAndAbilities(t *testing.T) {
	type pick func(out importer.Output) []importer.NamedRow
	tests := []struct {
		name     string
		category string // report の names のキー
		id, slug string
		drop     func(t *testing.T, in *importer.Input, slug string)
		override func(in *importer.Input) map[string]string
		rows     pick
	}{
		{"持ち物", "items", "testorb", "test-orb",
			func(t *testing.T, in *importer.Input, slug string) { dropPokeAPINamed(t, in.PokeAPI.Items, slug) },
			func(in *importer.Input) map[string]string {
				if in.Overrides.Items == nil {
					in.Overrides.Items = map[string]string{}
				}
				return in.Overrides.Items
			},
			func(out importer.Output) []importer.NamedRow { return itemNamedRows(out) }},
		{"特性", "abilities", "testguard", "test-guard",
			func(t *testing.T, in *importer.Input, slug string) { dropPokeAPINamed(t, in.PokeAPI.Abilities, slug) },
			func(in *importer.Input) map[string]string {
				if in.Overrides.Abilities == nil {
					in.Overrides.Abilities = map[string]string{}
				}
				return in.Overrides.Abilities
			},
			func(out importer.Output) []importer.NamedRow { return out.Abilities }},
	}
	for _, tt := range tests {
		t.Run(tt.name+": 上書きなし", func(t *testing.T) {
			in := reconcileInput(t)
			tt.drop(t, &in, tt.slug)
			out, rec := reconcileOK(t, in)
			row := namedRowByID(t, tt.rows(out), tt.id)
			if row.NameJaSource != "fallback_en" || row.NameJa != row.NameEn {
				t.Errorf("(%q, %q), want 英語名・fallback_en(推測しない)", row.NameJa, row.NameJaSource)
			}
			if !containsString(rec.Names[tt.category].FallbackIDs, tt.id) {
				t.Errorf("names.%s.fallbackIds に %s が無い: %v", tt.category, tt.id, rec.Names[tt.category].FallbackIDs)
			}
			if !hasFinding(rec.Report.Warnings, importer.KindNameFallback, tt.id) {
				t.Errorf("name-fallback の警告(%s)が無い", tt.id)
			}
		})
		t.Run(tt.name+": 上書きあり", func(t *testing.T) {
			in := reconcileInput(t)
			tt.drop(t, &in, tt.slug)
			tt.override(&in)[tt.id] = "テストうわがき"
			out, rec := reconcileOK(t, in)
			row := namedRowByID(t, tt.rows(out), tt.id)
			if row.NameJa != "テストうわがき" || row.NameJaSource != "override" {
				t.Errorf("(%q, %q), want 上書きの名前・override", row.NameJa, row.NameJaSource)
			}
			if containsString(rec.Names[tt.category].FallbackIDs, tt.id) {
				t.Errorf("上書きした %s が fallbackIds に残っている", tt.id)
			}
			if hasFinding(rec.Report.Warnings, importer.KindOverrideUnused, tt.id) {
				t.Errorf("使った上書き %s が override-unused になった", tt.id)
			}
		})
	}
}

// itemNamedRows は持ち物の行を名前の比較用に NamedRow へ寄せる(持ち物の行の型が NamedRow を埋め込む前提にしない)。
func itemNamedRows(out importer.Output) []importer.NamedRow {
	rows := make([]importer.NamedRow, 0, len(out.Items))
	for _, it := range out.Items {
		rows = append(rows, importer.NamedRow{ID: it.ID, NameJa: it.NameJa, NameJaSource: it.NameJaSource, NameEn: it.NameEn})
	}
	return rows
}

// AC-7: 地方の姿は、上流に日本語名が無ければ「基本種名(○○のすがた)」を生成しない(英語名のまま fallbackIds)。
// 上書きがあればそれを使う。
func TestRegionalFormNameIsNotGenerated(t *testing.T) {
	for _, forme := range []string{"Alola", "Galar", "Hisui", "Paldea-Aqua"} {
		t.Run(forme, func(t *testing.T) {
			in := reconcileInput(t)
			showdownSpecies(t, &in, "testleafrain").Forme = forme
			delete(in.Overrides.Species, "testleafrain")
			dropPokeAPIName(&in, "testleaf-rain")
			out, rec := reconcileOK(t, in)
			got := speciesByShowdownID(out)["testleafrain"]
			if got.NameJaSource != "fallback_en" || got.NameJa != got.NameEn {
				t.Errorf("(%q, %q), want 英語名・fallback_en(推測しない)", got.NameJa, got.NameJaSource)
			}
			if !containsString(rec.Names["species"].FallbackIDs, "testleafrain") {
				t.Errorf("species.fallbackIds に testleafrain が無い: %v", rec.Names["species"].FallbackIDs)
			}
			if hasFinding(rec.Report.Warnings, importer.KindNameGenerated, "testleafrain") {
				t.Error("地方の姿の名前を生成した")
			}
		})
	}
	t.Run("上書きあり", func(t *testing.T) {
		in := reconcileInput(t)
		showdownSpecies(t, &in, "testleafrain").Forme = "Alola"
		dropPokeAPIName(&in, "testleaf-rain")
		in.Overrides.Species["testleafrain"] = "テストリーフ(テストのすがた)"
		out, _ := reconcileOK(t, in)
		got := speciesByShowdownID(out)["testleafrain"]
		if got.NameJa != "テストリーフ(テストのすがた)" || got.NameJaSource != "override" {
			t.Errorf("(%q, %q), want 上書きの名前・override", got.NameJa, got.NameJaSource)
		}
	})
}
