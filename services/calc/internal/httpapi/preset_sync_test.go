package httpapi

// issue #73: api/openapi.yaml の DefenderPreset enum と engine.DefenderPresetCatalog() は
// 1対1対応が前提(presetKeysFrom が変換テーブルを持たず、契約の列挙値をそのまま engine.PresetKey に
// 型変換するだけのため。convert.go の presetKeysFrom 参照)。片方だけにプリセットを追加・削除しても
// 通常の生成・ユニットテストでは検出できないため、契約(埋め込まれた spec を kin-openapi で読む。
// vocabulary_test.go の TestWasmCodesAreValidErrorCodes と同様にハードコードした一覧を使う手もあるが、
// 手で列挙すると「足し忘れ」自体を検出できない。ここでは契約の enum を直接読み、ハードコードを避ける)。

import (
	"testing"

	"example.com/pokecalc/engine"
)

// AC: OpenAPI の DefenderPreset enum と engine.DefenderPresetCatalog() のキー集合・順序が一致する。
func TestDefenderPresetEnumMatchesEngineCatalog(t *testing.T) {
	doc, _ := loadContract(t)
	schema, ok := doc.Components.Schemas["DefenderPreset"]
	if !ok || schema.Value == nil {
		t.Fatal("契約に components/schemas/DefenderPreset が無い")
	}
	if len(schema.Value.Enum) == 0 {
		t.Fatal("DefenderPreset の enum が空(検査が空振りしている)")
	}
	var enumKeys []string
	for _, v := range schema.Value.Enum {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("DefenderPreset の enum に文字列でない値がある: %v", v)
		}
		enumKeys = append(enumKeys, s)
	}

	catalog := engine.DefenderPresetCatalog()
	if len(catalog) == 0 {
		t.Fatal("engine.DefenderPresetCatalog() が空(検査が空振りしている)")
	}
	var catalogKeys []string
	for _, p := range catalog {
		catalogKeys = append(catalogKeys, string(p.Key))
	}

	// 集合の一致(片方だけの追加・削除を検出)。
	enumSet := map[string]bool{}
	for _, k := range enumKeys {
		enumSet[k] = true
	}
	catalogSet := map[string]bool{}
	for _, k := range catalogKeys {
		catalogSet[k] = true
	}
	for _, k := range enumKeys {
		if !catalogSet[k] {
			t.Errorf("api/openapi.yaml の DefenderPreset に %q があるが、engine.DefenderPresetCatalog() に無い"+
				"(API は受け付けるが engine が解決できない値になる)", k)
		}
	}
	for _, k := range catalogKeys {
		if !enumSet[k] {
			t.Errorf("engine.DefenderPresetCatalog() に %q があるが、api/openapi.yaml の DefenderPreset に無い"+
				"(engine のプリセットを API から指定できない)", k)
		}
	}

	// 順序の一致(契約の description が「耐久が上がる順」と明記しているため。ADR-0009 §1 は8件・
	// 順序も規定している)。件数不一致を黙って skip すると、片方に重複がある(集合としては一致するが
	// 列としては崩れている)ケースを見逃す。
	if len(enumKeys) != len(catalogKeys) {
		t.Errorf("件数が不一致: openapi=%d件%v, engine=%d件%v。集合が一致していてもここで落ちる場合は"+
			"重複がある(ADR-0009 §1 は8件・順序も規定)", len(enumKeys), enumKeys, len(catalogKeys), catalogKeys)
	} else {
		for i := range enumKeys {
			if enumKeys[i] != catalogKeys[i] {
				t.Errorf("順序が不一致(%d番目): openapi=%q, engine=%q。契約は「耐久が上がる順」と明記(ADR-0009 §1)",
					i, enumKeys[i], catalogKeys[i])
				break
			}
		}
	}
}
