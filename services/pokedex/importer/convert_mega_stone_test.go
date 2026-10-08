package importer_test

// メガストーンの判定を取得元(Showdown の持ち物データの megaStone)からも導く・メガのフォーム判定の漏れ・
// 名前を推測しないこと(issue #607・ADR-0140)。データは架空(testdata/fictional)。
//
//   - Showdown のメガ判定は「フォーム名に Mega を含む」(sim/dex-species.ts の isMega)。"M-Mega" のように
//     Mega が先頭に来ないフォームもメガとして取り込む(is_mega・base_species_key・required_item_id を持つ)。
//   - 持ち物の行の IsMegaStone は「Showdown の megaStone が空でない」または「取り込んだメガ種族の
//     required_item_id に現れる」(どちらか)。メガ種族が取り込まれない(レギュレーション外)ストーンも true。
//   - 2つの根拠が食い違う(メガ種族が要求するのに Showdown の megaStone が空)ときは警告 item-mega-stone-mismatch。
//   - スナップショットの持ち物に megaStone のキーが無い(古い取得物)ときはデコードで拒否する(取り直しを促す)。
//   - 名前の表を持たない: "M-Mega" のようにフォーム識別子が規則に合わないメガ、地方の姿(Alola など)は
//     上流に日本語名が無ければ生成せず英語名のまま(fallbackIds に出し、上書き設定で補う)。

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

// itemMegaStone は変換結果の持ち物の IsMegaStone を ID ごとに返す。
func itemMegaStone(out importer.Output) map[string]bool {
	m := map[string]bool{}
	for _, it := range out.Items {
		m[it.ID] = it.IsMegaStone
	}
	return m
}

// AC-1: フォーム名の先頭が Mega でなくても(M-Mega・F-Mega)、Mega を含めばメガとして取り込む。
// そのメガが要求する持ち物はメガストーンになる(Meowsticite の漏れの再現。実名は使わない)。
func TestConvertTreatsInfixMegaFormeAsMega(t *testing.T) {
	for _, forme := range []string{"Mega", "Mega-X", "M-Mega", "F-Mega"} {
		t.Run(forme, func(t *testing.T) {
			in := reconcileInput(t)
			showdownSpecies(t, &in, "testmonmega").Forme = forme
			out, _ := reconcileOK(t, in)
			sp, ok := speciesByShowdownID(out)["testmonmega"]
			if !ok {
				t.Fatal("testmonmega が取り込まれていない")
			}
			if !sp.IsMega || sp.RequiredItemID != "testmonite" || sp.BaseSpeciesKey == "" {
				t.Errorf("IsMega=%v RequiredItemID=%q BaseSpeciesKey=%q, want メガ・testmonite・基本種のキー",
					sp.IsMega, sp.RequiredItemID, sp.BaseSpeciesKey)
			}
			if !itemMegaStone(out)["testmonite"] {
				t.Error("testmonite がメガストーンと判定されていない")
			}
		})
	}
}

// AC-2: 持ち物の IsMegaStone は Showdown の megaStone からも導く。メガ種族が取り込まれないストーンも true。
func TestConvertMarksMegaStoneFromShowdownItemData(t *testing.T) {
	t.Run("既存の持ち物: ストーンだけ true", func(t *testing.T) {
		out, _ := reconcileOK(t, reconcileInput(t))
		want := map[string]bool{"testmonite": true, "testorb": false, "testberry": false}
		if got := itemMegaStone(out); !reflect.DeepEqual(got, want) {
			t.Errorf("IsMegaStone = %v, want %v", got, want)
		}
	})
	t.Run("対応するメガ種族が取り込まれないストーンも true", func(t *testing.T) {
		in := reconcileInput(t)
		// Showdown にだけある使用可の持ち物(item-showdown-only で取り込まれる)。対応するメガ種族は fixture に無い。
		in.Showdown.Items = append(in.Showdown.Items, importer.ShowdownItem{
			ID: "testoutite", Name: "Testoutite", MegaStone: map[string]string{"Testouter": "Testouter-Mega"},
		})
		out, rec := reconcileOK(t, in)
		got, ok := itemMegaStone(out)["testoutite"]
		if !ok {
			t.Fatal("前提: testoutite が取り込まれていない(Showdown だけの使用可の持ち物)")
		}
		if !got {
			t.Error("メガ種族が取り込まれないストーン testoutite がメガストーンと判定されていない")
		}
		for _, sp := range out.Species {
			if sp.RequiredItemID == "testoutite" {
				t.Fatalf("前提: testoutite を要求する種族がある: %s", sp.ShowdownID)
			}
		}
		if hasFinding(rec.Report.Warnings, importer.KindItemMegaStoneMismatch, "testoutite") {
			t.Error("レギュレーション外のメガのストーンは食い違いではない(警告しない)")
		}
	})
	t.Run("megaStone が空の持ち物は false", func(t *testing.T) {
		in := reconcileInput(t)
		in.Showdown.Items = append(in.Showdown.Items, importer.ShowdownItem{ID: "testplainitem", Name: "Test Plain Item", MegaStone: map[string]string{}})
		out, _ := reconcileOK(t, in)
		if got, ok := itemMegaStone(out)["testplainitem"]; !ok || got {
			t.Errorf("testplainitem: 取り込み=%v IsMegaStone=%v, want 取り込み・false", ok, got)
		}
	})
	t.Run("使用不可のストーンは従来どおり取り込まない", func(t *testing.T) {
		in := reconcileInput(t)
		past := "Past"
		in.Showdown.Items = append(in.Showdown.Items, importer.ShowdownItem{
			ID: "testpastite", Name: "Testpastite", IsNonstandard: &past, MegaStone: map[string]string{"Testpast": "Testpast-Mega"},
		})
		out, _ := reconcileOK(t, in)
		if _, ok := itemMegaStone(out)["testpastite"]; ok {
			t.Error("使用不可(isNonstandard)のストーンを取り込んだ")
		}
	})
}

// AC-3: メガ種族が要求する持ち物なのに Showdown の megaStone が空なら、IsMegaStone は true のまま
// (どちらかの根拠で判定する)にし、警告 item-mega-stone-mismatch を出す(取り込みは止めない)。
func TestConvertWarnsMegaStoneMismatch(t *testing.T) {
	in := reconcileInput(t)
	sdItem(t, &in, "testmonite").MegaStone = map[string]string{}
	out, rec := reconcileOK(t, in)
	if !itemMegaStone(out)["testmonite"] {
		t.Error("メガ種族が要求する testmonite がメガストーンでなくなった")
	}
	if !hasFinding(rec.Report.Warnings, importer.KindItemMegaStoneMismatch, "testmonite") {
		t.Errorf("警告 %s(testmonite)が無い: %+v", importer.KindItemMegaStoneMismatch, rec.Report.Warnings)
	}
	// 基準状態(根拠が揃っている)では出ない。
	_, base := reconcileOK(t, reconcileInput(t))
	if hasKind(base.Report.Warnings, importer.KindItemMegaStoneMismatch) {
		t.Errorf("根拠が揃っているのに %s が出た", importer.KindItemMegaStoneMismatch)
	}
}

// showdownSnapshotWithItems は持ち物だけを差し替えた最小の Showdown スナップショット。
func showdownSnapshotWithItems(items string) []byte {
	return []byte(`{"schemaVersion":1,"source":"showdown","version":"abad1deaabad1deaabad1deaabad1deaabad1dea","mod":"testmod",` +
		`"species":[],"moves":[],"items":[` + items + `],"abilities":[],"learnsets":{},"natures":[]}`)
}

// AC-4: 持ち物の megaStone は必須のキー(空のオブジェクト可)。無い古い取得物は ErrInvalidInput で拒否し、
// 取り直し(make import-fetch)を案内する。黙って「ストーンでない」と読まない。
func TestDecodeShowdownSnapshotRequiresItemMegaStone(t *testing.T) {
	t.Run("キーが無い", func(t *testing.T) {
		_, err := importer.DecodeShowdownSnapshot(showdownSnapshotWithItems(`{"id":"testorb","name":"Test Orb","isNonstandard":null}`))
		if !errors.Is(err, importer.ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
		if !strings.Contains(err.Error(), "megaStone") || !strings.Contains(err.Error(), "make import-fetch") {
			t.Errorf("エラーに megaStone と取り直しの案内が無い: %v", err)
		}
	})
	t.Run("空のオブジェクトは通る", func(t *testing.T) {
		s, err := importer.DecodeShowdownSnapshot(showdownSnapshotWithItems(`{"id":"testorb","name":"Test Orb","isNonstandard":null,"megaStone":{}}`))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(s.Items) != 1 || len(s.Items[0].MegaStone) != 0 {
			t.Errorf("Items = %+v", s.Items)
		}
	})
	t.Run("基本種からメガへの対応を読む", func(t *testing.T) {
		s, err := importer.DecodeShowdownSnapshot(showdownSnapshotWithItems(
			`{"id":"testmonite","name":"Testmonite","isNonstandard":null,"megaStone":{"Testmon":"Testmon-M-Mega","Testmon-F":"Testmon-F-Mega"}}`))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		want := map[string]string{"Testmon": "Testmon-M-Mega", "Testmon-F": "Testmon-F-Mega"}
		if len(s.Items) != 1 || !reflect.DeepEqual(s.Items[0].MegaStone, want) {
			t.Errorf("MegaStone = %+v, want %v", s.Items, want)
		}
	})
}

// AC-5: フォーム識別子が「Mega」「Mega-<識別子>」の形でないメガ(M-Mega など)は、上流に日本語名が無くても
// 名前を推測しない(英語名のまま fallbackIds に出す)。上書き設定があればそれを使う。
func TestConvertDoesNotGuessNameForInfixMegaForme(t *testing.T) {
	t.Run("上書きなし: 英語名のまま", func(t *testing.T) {
		in := reconcileInput(t)
		dropPokeAPIName(&in, "testmon-mega")
		showdownSpecies(t, &in, "testmonmega").Forme = "M-Mega"
		out, rec := reconcileOK(t, in)
		got := speciesByShowdownID(out)["testmonmega"]
		if got.NameJaSource != "fallback_en" || got.NameJa != got.NameEn {
			t.Errorf("(%q, %q), want 英語名・fallback_en", got.NameJa, got.NameJaSource)
		}
		if !containsString(rec.Names["species"].FallbackIDs, "testmonmega") {
			t.Errorf("species.fallbackIds に testmonmega が無い: %v", rec.Names["species"].FallbackIDs)
		}
		if rec.Names["species"].Generated != 0 {
			t.Errorf("generated = %d, want 0(推測しない)", rec.Names["species"].Generated)
		}
	})
	t.Run("上書きあり", func(t *testing.T) {
		in := reconcileInput(t)
		dropPokeAPIName(&in, "testmon-mega")
		showdownSpecies(t, &in, "testmonmega").Forme = "M-Mega"
		if in.Overrides.Species == nil {
			in.Overrides.Species = map[string]string{}
		}
		in.Overrides.Species["testmonmega"] = "テストうわがきメガ"
		out, rec := reconcileOK(t, in)
		got := speciesByShowdownID(out)["testmonmega"]
		if got.NameJa != "テストうわがきメガ" || got.NameJaSource != "override" {
			t.Errorf("(%q, %q), want 上書きの名前・override", got.NameJa, got.NameJaSource)
		}
		if containsString(rec.Names["species"].FallbackIDs, "testmonmega") {
			t.Error("上書きした種族が fallbackIds に残っている")
		}
	})
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
