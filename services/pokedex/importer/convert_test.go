package importer_test

// 純粋な変換(スナップショット → ADR-0015 の行)のテスト(ADR-0101 §3〜§8)。
// DB もネットワークも使わない。入力は testdata/fictional の架空データだけ
// (図鑑番号 9001 以降、ID は test で始まる英小文字、日本語名は「テスト」で始まる。ADR-0015 §7)。

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

const fixtureRoot = "testdata/fictional"

// loadFixture は架空データの入力一式を読む(LoadInput 自体のテストは load_test.go)。
func loadFixture(t *testing.T) importer.Input {
	t.Helper()
	in, _, err := importer.LoadInput(fixtureRoot)
	if err != nil {
		t.Fatalf("LoadInput(%s): %v", fixtureRoot, err)
	}
	return in
}

func convertOK(t *testing.T, in importer.Input) (importer.Output, importer.Report) {
	t.Helper()
	out, rep, err := importer.Convert(in)
	if err != nil {
		t.Fatalf("Convert: %v\nreport: %+v", err, rep)
	}
	if len(rep.Blockers) != 0 {
		t.Fatalf("エラーなしなのに Blockers がある: %+v", rep.Blockers)
	}
	return out, rep
}

// hasFinding は指定の種類・ID の指摘があるか。
func hasFinding(findings []importer.Finding, kind importer.FindingKind, id string) bool {
	for _, f := range findings {
		if f.Kind == kind && f.ID == id {
			return true
		}
	}
	return false
}

func calcMove(t *testing.T, in *importer.Input, name string) *importer.CalcMove {
	t.Helper()
	for i := range in.Calc.Moves {
		if in.Calc.Moves[i].Name == name {
			return &in.Calc.Moves[i]
		}
	}
	t.Fatalf("calc の技 %q が fixture に無い", name)
	return nil
}

func showdownMove(t *testing.T, in *importer.Input, id string) *importer.ShowdownMove {
	t.Helper()
	for i := range in.Showdown.Moves {
		if in.Showdown.Moves[i].ID == id {
			return &in.Showdown.Moves[i]
		}
	}
	t.Fatalf("Showdown の技 %q が fixture に無い", id)
	return nil
}

func calcSpecies(t *testing.T, in *importer.Input, name string) *importer.CalcSpecies {
	t.Helper()
	for i := range in.Calc.Species {
		if in.Calc.Species[i].Name == name {
			return &in.Calc.Species[i]
		}
	}
	t.Fatalf("calc の種族 %q が fixture に無い", name)
	return nil
}

func showdownSpecies(t *testing.T, in *importer.Input, id string) *importer.ShowdownSpecies {
	t.Helper()
	for i := range in.Showdown.Species {
		if in.Showdown.Species[i].ID == id {
			return &in.Showdown.Species[i]
		}
	}
	t.Fatalf("Showdown の種族 %q が fixture に無い", id)
	return nil
}

func requireRegulation(t *testing.T, in *importer.Input) {
	t.Helper()
	if len(in.Regulations.Regulations) == 0 {
		t.Fatal("fixture のレギュレーション定義が読めていない")
	}
}

func movesByID(out importer.Output) map[string]importer.MoveRow {
	m := map[string]importer.MoveRow{}
	for _, r := range out.Moves {
		m[r.ID] = r
	}
	return m
}

func speciesByShowdownID(out importer.Output) map[string]importer.SpeciesRow {
	m := map[string]importer.SpeciesRow{}
	for _, r := range out.Species {
		m[r.ShowdownID] = r
	}
	return m
}

func namedByID(rows []importer.NamedRow) map[string]importer.NamedRow {
	m := map[string]importer.NamedRow{}
	for _, r := range rows {
		m[r.ID] = r
	}
	return m
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- 技(ADR-0002 追記 P2-1c の規則 1〜4。ADR-0101 §4) --------------------------

func TestConvertMovesFollowP21cRules(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))
	got := movesByID(out)

	// 規則1: 両方にある技だけ。規則2: Showdown だけにある使用可の技は Showdown の値で補う。
	// calc の type の無い断片で Showdown が使用可(testrevived)は「calc に無い」扱い → 規則2。
	want := map[string]importer.MoveRow{
		"testflame":   {ID: "testflame", NameEn: "Test Flame", Type: "fire", Category: "special", Power: 90, Accuracy: 100, PP: 15, Priority: 0},
		"teststrike":  {ID: "teststrike", NameEn: "Test Strike", Type: "normal", Category: "physical", Power: 40, Accuracy: 0, PP: 30, Priority: 1},
		"testglare":   {ID: "testglare", NameEn: "Test Glare", Type: "grass", Category: "status", Power: 0, Accuracy: 100, PP: 20, Priority: 0},
		"testrevived": {ID: "testrevived", NameEn: "Test Revived", Type: "water", Category: "special", Power: 70, Accuracy: 90, PP: 10, Priority: 0},
		"testsplash":  {ID: "testsplash", NameEn: "Test Splash", Type: "water", Category: "special", Power: 60, Accuracy: 100, PP: 20, Priority: 0},
	}
	if !reflect.DeepEqual(keysOf(got), keysOf(want)) {
		t.Fatalf("技の集合 = %v, want %v", keysOf(got), keysOf(want))
	}
	for id, w := range want {
		g := got[id]
		// 日本語名は TestConvertNamesJa で見る。ここでは数値と分類・タイプだけ。
		g.NameJa, g.NameJaSource = "", ""
		if g != w {
			t.Errorf("技 %s = %+v, want %+v", id, g, w)
		}
	}

	// 除外: calc だけ(番兵を含む)・calc の断片で Showdown が使用不可・Showdown で isNonstandard が null でない。
	for _, id := range []string{"nomove", "testold", "testbanned"} {
		if !hasFinding(rep.Warnings, importer.KindMoveExcluded, id) {
			t.Errorf("除外した技 %s が Warnings(%s)に無い", id, importer.KindMoveExcluded)
		}
	}
	for _, id := range []string{"testsplash", "testrevived"} {
		if !hasFinding(rep.Warnings, importer.KindMoveShowdownOnly, id) {
			t.Errorf("Showdown の値で補った技 %s が Warnings(%s)に無い", id, importer.KindMoveShowdownOnly)
		}
	}
	// 規則3: 変化技のタイプの食い違いは警告だけ(Showdown のタイプを採る)。
	if !hasFinding(rep.Warnings, importer.KindMoveTypeMismatch, "testglare") {
		t.Errorf("変化技のタイプの食い違い(testglare)が Warnings に無い")
	}
	// 規則4: calc の category 省略は Status。Showdown の Status と食い違いにしない。
	if hasFinding(rep.Warnings, importer.KindMoveValueMismatch, "testglare") || hasFinding(rep.Blockers, importer.KindMoveValueMismatch, "testglare") {
		t.Errorf("calc の category 省略を Status と扱っていない(testglare が値の食い違いになった)")
	}
}

// 持ち物は技と同じ規則(ADR-0101 §5)。calc だけの持ち物は除外の警告、Showdown だけで
// 使用可の持ち物は補完の警告(どちらも取り込む集合には規則どおり反映される)。
func TestConvertItemsFollowSameRuleAsMoves(t *testing.T) {
	in := loadFixture(t)
	in.Calc.Items = append(in.Calc.Items, "Test Onlycalc")
	for i := range in.Showdown.Items {
		if in.Showdown.Items[i].ID == "testrelic" {
			in.Showdown.Items[i].IsNonstandard = nil
		}
	}

	out, rep := convertOK(t, in)
	items := namedByID(out.Items)
	if _, ok := items["testonlycalc"]; ok {
		t.Errorf("calc だけの持ち物 testonlycalc が取り込まれた")
	}
	if !hasFinding(rep.Warnings, importer.KindItemExcluded, "testonlycalc") {
		t.Errorf("calc だけの持ち物 testonlycalc が除外の記録(%s)に無い", importer.KindItemExcluded)
	}
	if _, ok := items["testrelic"]; !ok {
		t.Errorf("Showdown だけで使用可になった testrelic が取り込まれていない")
	}
	if !hasFinding(rep.Warnings, importer.KindItemShowdownOnly, "testrelic") {
		t.Errorf("Showdown だけで補った持ち物 testrelic が記録(%s)に無い", importer.KindItemShowdownOnly)
	}
}

// 規則3: 攻撃技のタイプの食い違いは取り込みを止める。分類・威力の食い違いも止める(ダメージに効く)。
func TestConvertBlocksOnDamagingMoveConflict(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
		kind   importer.FindingKind
		id     string
	}{
		{"攻撃技のタイプの食い違い", func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Flame").Type = "Water" },
			importer.KindMoveTypeMismatch, "testflame"},
		{"威力の食い違い", func(t *testing.T, in *importer.Input) { showdownMove(t, in, "testflame").BasePower = 95 },
			importer.KindMoveValueMismatch, "testflame"},
		{"分類の食い違い", func(t *testing.T, in *importer.Input) { showdownMove(t, in, "teststrike").Category = "Special" },
			importer.KindMoveValueMismatch, "teststrike"},
		{"calc の category 省略(=Status)と Showdown の攻撃技", func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Strike").Category = "" },
			importer.KindMoveValueMismatch, "teststrike"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(t, &in)
			out, rep, err := importer.Convert(in)
			if !errors.Is(err, importer.ErrBlocked) {
				t.Fatalf("err = %v, want ErrBlocked", err)
			}
			if !hasFinding(rep.Blockers, tt.kind, tt.id) {
				t.Errorf("Blockers に %s/%s が無い: %+v", tt.kind, tt.id, rep.Blockers)
			}
			if !reflect.DeepEqual(out, importer.Output{}) {
				t.Errorf("止めたのに Output が空でない(部分的な結果を投入に回さない)")
			}
		})
	}
}

// --- 種族・フォーム・メガ(ADR-0101 §5) ---------------------------------------

func TestConvertSpeciesFormsAndMega(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))
	got := speciesByShowdownID(out)

	type want struct {
		key, nameEn, type1, type2 string
		dex, form                 int
		stats                     [6]int
		isMega                    bool
		baseKey, item             string
		abilities                 []master.SpeciesAbilityRow
	}
	wants := map[string]want{
		// フォルム番号 = 基本種の formeOrder の添字(見た目違いの番号も欠番として数える)。
		"testmon": {key: "9001-000", nameEn: "Testmon", type1: "fire", dex: 9001, form: 0, stats: [6]int{80, 100, 70, 60, 70, 90},
			abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testblaze"}, {Slot: 3, AbilityID: "testguard"}}},
		"testmonmega": {key: "9001-001", nameEn: "Testmon-Mega", type1: "fire", type2: "normal", dex: 9001, form: 1,
			stats: [6]int{80, 130, 90, 80, 90, 110}, isMega: true, baseKey: "9001-000", item: "testmonite",
			abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testguard"}}},
		"testleaf": {key: "9002-000", nameEn: "Testleaf", type1: "grass", dex: 9002, form: 0, stats: [6]int{70, 60, 80, 90, 80, 60},
			abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testleaf"}}},
		"testleafrain": {key: "9002-002", nameEn: "Testleaf-Rain", type1: "water", dex: 9002, form: 2, stats: [6]int{70, 60, 80, 90, 80, 60},
			abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testleaf"}}},
		// calc の "Testshield-Shield" は Showdown の baseForme で "Testshield" と対応づける。
		"testshield": {key: "9003-000", nameEn: "Testshield", type1: "normal", dex: 9003, form: 0, stats: [6]int{60, 50, 140, 50, 140, 60},
			abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "teststance"}}},
	}
	if !reflect.DeepEqual(keysOf(got), keysOf(wants)) {
		t.Fatalf("種族の集合(showdown_id) = %v, want %v", keysOf(got), keysOf(wants))
	}
	for id, w := range wants {
		g := got[id]
		stats := [6]int{g.BaseHP, g.BaseAtk, g.BaseDef, g.BaseSpA, g.BaseSpD, g.BaseSpe}
		if g.Key != w.key || g.DexNo != w.dex || g.Form != w.form || g.NameEn != w.nameEn ||
			g.Type1 != w.type1 || g.Type2 != w.type2 || stats != w.stats ||
			g.IsMega != w.isMega || g.BaseSpeciesKey != w.baseKey || g.RequiredItemID != w.item {
			t.Errorf("種族 %s = %+v, want %+v", id, g.SpeciesRow, w)
		}
		if !reflect.DeepEqual(g.Abilities, w.abilities) {
			t.Errorf("種族 %s の特性 = %+v, want %+v", id, g.Abilities, w.abilities)
		}
	}

	// 見た目だけ違うフォームは代表(フォルム番号が最小)に畳む。
	if !hasFinding(rep.Warnings, importer.KindFormFolded, "testleafbloom") {
		t.Errorf("見た目違いの testleafbloom が畳まれた記録(%s)に無い", importer.KindFormFolded)
	}
	// Showdown だけにあり性能が違うフォームは取り込まず警告(P2-2c で照合)。
	if !hasFinding(rep.Warnings, importer.KindSpeciesShowdownOnly, "testshieldblade") {
		t.Errorf("Showdown だけの testshieldblade が警告(%s)に無い", importer.KindSpeciesShowdownOnly)
	}
	// HP 種族値 1 と、設定で除外した calc の内部フォームは取り込まない(P2-1b と同じ種族集合)。
	if !hasFinding(rep.Warnings, importer.KindSpeciesExcluded, "testbug") {
		t.Errorf("HP 種族値 1 の testbug が除外の記録(%s)に無い", importer.KindSpeciesExcluded)
	}
	if !hasFinding(rep.Warnings, importer.KindSpeciesExcluded, "testshieldboth") {
		t.Errorf("設定で除外した Testshield-Both が除外の記録(%s)に無い", importer.KindSpeciesExcluded)
	}
}

// calc 側に見た目違いのフォームが両方あっても、性能が同じなら1件に畳む。
func TestConvertFoldsCosmeticFormPresentInBothSources(t *testing.T) {
	in := loadFixture(t)
	base := *calcSpecies(t, &in, "Testleaf")
	base.Name = "Testleaf-Bloom"
	in.Calc.Species = append(in.Calc.Species, base)

	out, rep := convertOK(t, in)
	if _, ok := speciesByShowdownID(out)["testleafbloom"]; ok {
		t.Errorf("性能が同じ testleafbloom が別の行になった")
	}
	if !hasFinding(rep.Warnings, importer.KindFormFolded, "testleafbloom") {
		t.Errorf("testleafbloom が畳まれた記録に無い")
	}
}

// calc 由来の種族が、フォルム番号がより小さい Showdown だけの見た目違いフォームと性能が
// 同じ場合でも、その calc 由来の種族(の性能)は必ず取り込まれる(ADR-0101 §5「calc にあるか
// どうかに依らない」)。calc の名前を改名して Testleaf(form0)を Showdown だけの候補に変え、
// Testleaf-Bloom(form1)を calc 由来にする(calc の名前が変わっても対応する Showdown 側の
// フォームが消えるわけではない実データの畳み込みを模す)。
func TestConvertKeepsCalcConfirmedFormAsFoldRepresentative(t *testing.T) {
	in := loadFixture(t)
	calcSpecies(t, &in, "Testleaf").Name = "Testleaf-Bloom"

	out, rep := convertOK(t, in)
	species := speciesByShowdownID(out)
	if _, ok := species["testleaf"]; !ok {
		t.Fatalf("calc 由来の性能を持つ testleaf が畳み込みで消えた: %+v", keysOf(species))
	}
	if _, ok := species["testleafbloom"]; ok {
		t.Errorf("testleafbloom が別行として残った(畳まれていない)")
	}
	if hasFinding(rep.Warnings, importer.KindSpeciesShowdownOnly, "testleaf") {
		t.Errorf("calc 由来の性能を確認できた testleaf が species-showdown-only になった")
	}
	if !hasFinding(rep.Warnings, importer.KindFormFolded, "testleafbloom") {
		t.Errorf("testleafbloom が畳まれた記録(%s)に無い", importer.KindFormFolded)
	}
}

func TestConvertBlocksOnSpeciesMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{"種族値の食い違い", func(t *testing.T, in *importer.Input) { showdownSpecies(t, in, "testmon").BaseStats.Atk = 101 }},
		{"タイプの食い違い", func(t *testing.T, in *importer.Input) { showdownSpecies(t, in, "testmon").Types = []string{"Water"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(t, &in)
			_, rep, err := importer.Convert(in)
			if !errors.Is(err, importer.ErrBlocked) {
				t.Fatalf("err = %v, want ErrBlocked", err)
			}
			if !hasFinding(rep.Blockers, importer.KindSpeciesMismatch, "testmon") {
				t.Errorf("Blockers に %s/testmon が無い: %+v", importer.KindSpeciesMismatch, rep.Blockers)
			}
		})
	}
}

func TestConvertRejectsInconsistentData(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{"calc の種族に Showdown の対応が無く、除外の設定にも無い", func(t *testing.T, in *importer.Input) {
			in.Config.ExcludeCalcSpecies = nil
		}},
		{"除外の設定の名前が calc に実在しない(改名を黙って素通りしない)", func(t *testing.T, in *importer.Input) {
			in.Config.ExcludeCalcSpecies = append(in.Config.ExcludeCalcSpecies, "Testnothing")
		}},
		{"除外するタイプの名前が calc に実在しない", func(t *testing.T, in *importer.Input) {
			in.Config.ExcludeTypes = append(in.Config.ExcludeTypes, "Testnone")
		}},
		{"フォームが基本種の formeOrder に無い(フォルム番号を採番できない)", func(t *testing.T, in *importer.Input) {
			showdownSpecies(t, in, "testmon").FormeOrder = []string{"Testmon"}
		}},
		{"メガなのに requiredItem が無い", func(t *testing.T, in *importer.Input) {
			showdownSpecies(t, in, "testmonmega").RequiredItem = ""
		}},
		{"メガの requiredItem が取り込む持ち物に無い", func(t *testing.T, in *importer.Input) {
			kept := in.Showdown.Items[:0]
			for _, it := range in.Showdown.Items {
				if it.ID != "testmonite" {
					kept = append(kept, it)
				}
			}
			in.Showdown.Items = kept
		}},
		{"種族が除外したタイプを使う", func(t *testing.T, in *importer.Input) {
			calcSpecies(t, in, "Testleaf").Types = []string{"Stellar"}
			showdownSpecies(t, in, "testleaf").Types = []string{"Stellar"}
		}},
		{"技(両方にある)が除外したタイプを使う", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "testflame").Type = "Stellar"
		}},
		{"技(Showdown だけにある)が除外したタイプを使う", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "testrevived").Type = "Stellar"
		}},
		{"レギュレーションの showdownMod がスナップショットの mod と違う", func(t *testing.T, in *importer.Input) {
			requireRegulation(t, in)
			in.Regulations.Regulations[0].ShowdownMod = "othermod"
		}},
		{"既定のレギュレーションが2件", func(t *testing.T, in *importer.Input) {
			requireRegulation(t, in)
			second := in.Regulations.Regulations[0]
			second.ID = "test-reg-two"
			in.Regulations.Regulations = append(in.Regulations.Regulations, second)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(t, &in)
			_, _, err := importer.Convert(in)
			if !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

// --- 日本語名(ADR-0101 §6) -----------------------------------------------------

func TestConvertNamesJa(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))
	species := speciesByShowdownID(out)
	moves := movesByID(out)
	items := namedByID(out.Items)
	abilities := namedByID(out.Abilities)
	types := map[string]importer.TypeRow{}
	for _, r := range out.Types {
		types[r.ID] = r
	}

	tests := []struct {
		name               string
		gotName, gotSource string
		wantName, wantSrc  string
	}{
		// 優先順位: override > PokeAPI(config の nameJaLanguages の順。既定 ja-Hrkt → ja)> 英語名(fallback_en)
		{"種族: PokeAPI の ja-Hrkt を ja より優先", species["testmon"].NameJa, species["testmon"].NameJaSource, "テストモン", "pokeapi"},
		{"種族: ja-Hrkt が無ければ ja", species["testleaf"].NameJa, species["testleaf"].NameJaSource, "テストリーフ", "pokeapi"},
		{"メガ: PokeAPI のフォーム名", species["testmonmega"].NameJa, species["testmonmega"].NameJaSource, "テストメガモン", "pokeapi"},
		{"フォーム: override は PokeAPI より優先", species["testleafrain"].NameJa, species["testleafrain"].NameJaSource, "テストリーフあめ", "override"},
		{"種族: どこにも無ければ英語名", species["testshield"].NameJa, species["testshield"].NameJaSource, "Testshield", "fallback_en"},
		{"技: PokeAPI", moves["testflame"].NameJa, moves["testflame"].NameJaSource, "テストほのおわざ", "pokeapi"},
		{"技: override は PokeAPI より優先", moves["teststrike"].NameJa, moves["teststrike"].NameJaSource, "テストうわがき", "override"},
		{"技: 英語名へフォールバック", moves["testsplash"].NameJa, moves["testsplash"].NameJaSource, "Test Splash", "fallback_en"},
		{"持ち物: PokeAPI", items["testorb"].NameJa, items["testorb"].NameJaSource, "テストだま", "pokeapi"},
		{"持ち物: override で補完", items["testberry"].NameJa, items["testberry"].NameJaSource, "テストのみ", "override"},
		{"特性: PokeAPI", abilities["testguard"].NameJa, abilities["testguard"].NameJaSource, "テストまもり", "pokeapi"},
		{"タイプ: PokeAPI", types["fire"].NameJa, types["fire"].NameJaSource, "テストほのお", "pokeapi"},
		{"タイプ: 英語名(calc の型名)へフォールバック", types["grass"].NameJa, types["grass"].NameJaSource, "Grass", "fallback_en"},
	}
	for _, tt := range tests {
		if tt.gotName != tt.wantName || tt.gotSource != tt.wantSrc {
			t.Errorf("%s: (%q, %q), want (%q, %q)", tt.name, tt.gotName, tt.gotSource, tt.wantName, tt.wantSrc)
		}
	}
	for _, r := range out.Moves {
		if r.NameEn == "" {
			t.Errorf("技 %s の name_en が空", r.ID)
		}
	}

	if !hasFinding(rep.Warnings, importer.KindNameFallback, "testshield") {
		t.Errorf("英語名へのフォールバック(testshield)が Warnings に無い(欠落件数の報告に使う)")
	}
	if !hasFinding(rep.Warnings, importer.KindOverrideUnused, "testghost") {
		t.Errorf("どの行にも当たらない override(testghost)が Warnings に無い")
	}
}

func TestConvertNameLanguageOrderComesFromConfig(t *testing.T) {
	in := loadFixture(t)
	in.Config.NameJaLanguages = []string{"ja", "ja-Hrkt"}
	out, _ := convertOK(t, in)
	if got := speciesByShowdownID(out)["testmon"]; got.NameJa != "テスト門" || got.NameJaSource != "pokeapi" {
		t.Errorf("nameJaLanguages を ja 優先にしたのに (%q, %q)", got.NameJa, got.NameJaSource)
	}
}

// --- タイプ・相性表 ---------------------------------------------------------------

func TestConvertTypesAndChart(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	var ids []string
	for _, r := range out.Types {
		ids = append(ids, r.ID)
	}
	// 除外するタイプ(設定の excludeTypes)を除き、calc の並び順を sort_order(1 始まり)にする。
	if want := []string{"normal", "fire", "water", "grass"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("タイプ = %v, want %v", ids, want)
	}
	for i, r := range out.Types {
		if r.SortOrder != i+1 {
			t.Errorf("タイプ %s の sort_order = %d, want %d", r.ID, r.SortOrder, i+1)
		}
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	for _, c := range out.TypeChart {
		if !known[c.AttackType] || !known[c.DefenseType] {
			t.Errorf("除外したタイプの組が相性表に残っている: %+v", c)
		}
	}
	chart := engineChart(t, out)
	tests := []struct {
		atk, def engine.Type
		want     int
	}{
		{"fire", "grass", 4},
		{"fire", "water", 1},
		{"water", "fire", 4},
		{"normal", "fire", 2}, // calc の表に無い組は等倍
	}
	for _, tt := range tests {
		got, err := chart.Code(tt.atk, tt.def)
		if err != nil {
			t.Fatalf("Code(%s, %s): %v", tt.atk, tt.def, err)
		}
		if got != tt.want {
			t.Errorf("Code(%s, %s) = %d, want %d", tt.atk, tt.def, got, tt.want)
		}
	}
}

// --- レギュレーション(ADR-0101 §7) ----------------------------------------------

func TestConvertRegulationSets(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	want := []importer.RegulationRow{{ID: "test-reg", NameJa: "テストレギュ", IsDefault: true, StartsOn: "2026-01-01", EndsOn: "2026-03-31"}}
	if !reflect.DeepEqual(out.Regulations, want) {
		t.Fatalf("Regulations = %+v, want %+v", out.Regulations, want)
	}
	members := func(rows []importer.RegulationMemberRow) []string {
		var ids []string
		for _, r := range rows {
			if r.RegulationID != "test-reg" {
				t.Errorf("想定外のレギュレーション ID: %+v", r)
			}
			ids = append(ids, r.MemberID)
		}
		sort.Strings(ids)
		return ids
	}
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		// v1: 定義の showdownMod のスナップショットから取り込んだ行がそのレギュレーションの集合。
		{"species", members(out.RegulationSpecies), []string{"9001-000", "9001-001", "9002-000", "9002-002", "9003-000"}},
		{"moves", members(out.RegulationMoves), []string{"testflame", "testglare", "testrevived", "testsplash", "teststrike"}},
		{"items", members(out.RegulationItems), []string{"testberry", "testmonite", "testorb"}},
		// 特性は集合の種族の特性スロットから導く。
		{"abilities", members(out.RegulationAbilities), []string{"testblaze", "testguard", "testleaf", "teststance"}},
	}
	for _, tt := range tests {
		if !reflect.DeepEqual(tt.got, tt.want) {
			t.Errorf("regulation_%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
	// 持ち物: calc と Showdown の両方にあり Showdown で使用可のもの(testrelic は Past)。
	if got := keysOf(namedByID(out.Items)); !reflect.DeepEqual(got, []string{"testberry", "testmonite", "testorb"}) {
		t.Errorf("items = %v", got)
	}
}

// --- 習得技 -----------------------------------------------------------------------

func TestConvertLearnsets(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	got := map[string][]string{}
	for _, r := range out.Learnsets {
		got[r.SpeciesKey] = append(got[r.SpeciesKey], r.MoveID)
	}
	for k := range got {
		sort.Strings(got[k])
	}
	want := map[string][]string{
		// 取り込まない技(testold / testbanned)は落とす。
		"9001-000": {"testflame", "testglare", "teststrike"},
		// 自分の習得技が無いフォーム(メガ・別フォーム)は baseSpecies の習得技を使う。
		"9001-001": {"testflame", "testglare", "teststrike"},
		"9002-000": {"testglare", "testsplash"},
		"9002-002": {"testglare", "testsplash"},
		"9003-000": {"testrevived", "teststrike"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("learnsets = %v, want %v", got, want)
	}
}

// --- 効果定義(ADR-0101 §2) -------------------------------------------------------

func TestConvertEffects(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))
	gotItems := map[string]string{}
	for _, r := range out.ItemEffects {
		gotItems[r.ID] = string(r.Effect)
	}
	// 値は master.EncodeItemEffect の正準形。
	wantItems := map[string]string{"testorb": `{"DamageMod":5324}`, "testberry": `{"ResistBerryType":"fire"}`}
	if !reflect.DeepEqual(gotItems, wantItems) {
		t.Errorf("item_effects = %v, want %v", gotItems, wantItems)
	}
	gotAbilities := map[string]string{}
	for _, r := range out.AbilityEffects {
		gotAbilities[r.ID] = string(r.Effect)
	}
	if want := map[string]string{"testguard": `{"DefResistType":{"fire":2048}}`}; !reflect.DeepEqual(gotAbilities, want) {
		t.Errorf("ability_effects = %v, want %v", gotAbilities, want)
	}
	// 取り込む持ち物に無い ID の効果は投入しない(将来のレギュレーションで復活できるよう定義は残してよい)。
	if !hasFinding(rep.Warnings, importer.KindEffectUnused, "testunused") {
		t.Errorf("取り込まない持ち物の効果(testunused)が Warnings に無い")
	}
}

func TestConvertRejectsInvalidEffects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"4096 基準の整数でない(小数)", `{"DamageMod":5324.5}`},
		{"未知のフィールド", `{"DamageMod":5324,"Note":"x"}`},
		{"表に無いタイプ", `{"ResistBerryType":"testdark"}`},
		{"除外したタイプ", `{"ResistBerryType":"stellar"}`},
		{"空(補正なしと区別できない)", `{}`},
		{"負の値", `{"DamageMod":-1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			in.Effects.Items["testorb"] = []byte(tt.raw)
			_, _, err := importer.Convert(in)
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Fatalf("err = %v, want master.ErrInvalidEffect", err)
			}
		})
	}
}

// --- engine の型まで通る(ADR-0015 §6) ---------------------------------------------

func engineChart(t *testing.T, out importer.Output) engine.TypeChart {
	t.Helper()
	rows := make([]master.TypeRow, 0, len(out.Types))
	for _, r := range out.Types {
		rows = append(rows, r.TypeRow)
	}
	chart, err := master.TypeChart(rows, out.TypeChart)
	if err != nil {
		t.Fatalf("master.TypeChart: %v", err)
	}
	return chart
}

func TestConvertOutputPassesMasterMapping(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	chart := engineChart(t, out)

	for _, r := range out.Species {
		sp, err := master.Species(r.SpeciesRow, r.Abilities, chart)
		if err != nil {
			t.Errorf("種族 %s: %v", r.Key, err)
			continue
		}
		if sp.Key != r.Key || sp.NameJa != r.NameJa {
			t.Errorf("種族 %s の写像結果が行と違う: %+v", r.Key, sp)
		}
	}
	for _, r := range out.Moves {
		if _, err := master.Move(master.MoveRow{ID: r.ID, NameJa: r.NameJa, Type: r.Type, Category: r.Category, Power: r.Power, Priority: r.Priority}, chart); err != nil {
			t.Errorf("技 %s: %v", r.ID, err)
		}
	}
	itemEffects := map[string][]byte{}
	for _, e := range out.ItemEffects {
		itemEffects[e.ID] = e.Effect
	}
	for _, r := range out.Items {
		it, err := master.Item(master.ItemRow{ID: r.ID, NameJa: r.NameJa, Effect: itemEffects[r.ID]}, chart)
		if err != nil {
			t.Errorf("持ち物 %s: %v", r.ID, err)
		}
		if (itemEffects[r.ID] != nil) != (it.Effect != nil) {
			t.Errorf("持ち物 %s の効果の有無が写像で変わった", r.ID)
		}
	}
	abilityEffects := map[string][]byte{}
	for _, e := range out.AbilityEffects {
		abilityEffects[e.ID] = e.Effect
	}
	for _, r := range out.Abilities {
		if _, err := master.Ability(master.AbilityRow{ID: r.ID, NameJa: r.NameJa, Effect: abilityEffects[r.ID]}, chart); err != nil {
			t.Errorf("特性 %s: %v", r.ID, err)
		}
	}
	// 行の参照が閉じている(DB の外部キーと同じ規則を投入前に満たす)。
	speciesKeys := map[string]bool{}
	for _, r := range out.Species {
		speciesKeys[r.Key] = true
	}
	items := namedByID(out.Items)
	for _, r := range out.Species {
		if r.IsMega && (!speciesKeys[r.BaseSpeciesKey] || items[r.RequiredItemID].ID == "") {
			t.Errorf("メガ %s の参照先が無い: base=%q item=%q", r.Key, r.BaseSpeciesKey, r.RequiredItemID)
		}
	}
	moves := movesByID(out)
	for _, l := range out.Learnsets {
		if !speciesKeys[l.SpeciesKey] || moves[l.MoveID].ID == "" {
			t.Errorf("習得技の参照先が無い: %+v", l)
		}
	}
}

// --- 決定的であること(冪等な投入の前提) ---------------------------------------------

func TestConvertIsDeterministic(t *testing.T) {
	in := loadFixture(t)
	first, firstRep := convertOK(t, in)
	for i := 0; i < 5; i++ {
		again, rep := convertOK(t, loadFixture(t))
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("同じ入力で Output が変わった(%d 回目)", i+2)
		}
		if !reflect.DeepEqual(firstRep, rep) {
			t.Fatalf("同じ入力で Report が変わった(%d 回目)", i+2)
		}
	}
	if !sort.SliceIsSorted(first.Species, func(i, j int) bool { return first.Species[i].Key < first.Species[j].Key }) {
		t.Errorf("Species が key 順でない")
	}
	if !sort.SliceIsSorted(first.Moves, func(i, j int) bool { return first.Moves[i].ID < first.Moves[j].ID }) {
		t.Errorf("Moves が ID 順でない")
	}
}
