package master_test

// DB の行 → engine の型の写像(ADR-0100 §6)。DB を使わない。架空データだけを使う。

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// testTypes は架空の小さなタイプ表(ADR-0100 §7。example_seed.sql と同じ形)。
func testTypes() []master.TypeRow {
	return []master.TypeRow{
		{ID: "grass", SortOrder: 3, NameJa: "テストくさ"},
		{ID: "fire", SortOrder: 1, NameJa: "テストほのお"},
		{ID: "normal", SortOrder: 4, NameJa: "テストふつう"},
		{ID: "water", SortOrder: 2, NameJa: "テストみず"},
	}
}

func testChartRows() []master.TypeChartRow {
	return []master.TypeChartRow{
		{AttackType: "fire", DefenseType: "grass", Code: 4},
		{AttackType: "fire", DefenseType: "water", Code: 1},
		{AttackType: "water", DefenseType: "fire", Code: 4},
		{AttackType: "grass", DefenseType: "fire", Code: 1},
		{AttackType: "normal", DefenseType: "normal", Code: 0},
		{AttackType: "normal", DefenseType: "fire", Code: 2}, // 等倍を明示した行も受ける
	}
}

func testChart(t *testing.T) engine.TypeChart {
	t.Helper()
	c, err := master.TypeChart(testTypes(), testChartRows())
	if err != nil {
		t.Fatalf("TypeChart: %v", err)
	}
	return c
}

// --- タイプ相性表 ------------------------------------------------------------

func TestTypeChartDataOrdersTypesBySortOrder(t *testing.T) {
	d, err := master.TypeChartData(testTypes(), testChartRows())
	if err != nil {
		t.Fatalf("TypeChartData: %v", err)
	}
	want := []engine.Type{"fire", "water", "grass", "normal"}
	if !reflect.DeepEqual(d.Types, want) {
		t.Fatalf("Types = %v, want %v(sort_order 順)", d.Types, want)
	}
	if got := d.Effectiveness["fire"]["grass"]; got != 4 {
		t.Fatalf("fire→grass = %d, want 4", got)
	}
}

func TestTypeChartFromRows(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		atk  engine.Type
		def  []engine.Type
		num  int
		den  int
		name string
	}{
		{"fire", []engine.Type{"grass"}, 4, 2, "抜群"},
		{"fire", []engine.Type{"water"}, 1, 2, "いまひとつ"},
		{"normal", []engine.Type{"normal"}, 0, 2, "無効"},
		{"water", []engine.Type{"grass"}, 2, 2, "表に無い組は等倍"},
		{"fire", []engine.Type{"grass", "water"}, 4, 4, "複合"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := c.Effectiveness(tc.atk, tc.def)
			if err != nil {
				t.Fatalf("Effectiveness: %v", err)
			}
			if e.Num != tc.num || e.Den != tc.den {
				t.Fatalf("got %d/%d, want %d/%d", e.Num, e.Den, tc.num, tc.den)
			}
		})
	}
}

func TestTypeChartRejectsInvalidRows(t *testing.T) {
	cases := []struct {
		name   string
		types  []master.TypeRow
		chart  []master.TypeChartRow
		engine bool // engine.ErrInvalidTypeChart でも判別できること
	}{
		{name: "倍率コード3", types: testTypes(), chart: []master.TypeChartRow{{AttackType: "fire", DefenseType: "grass", Code: 3}}, engine: true},
		{name: "負のコード", types: testTypes(), chart: []master.TypeChartRow{{AttackType: "fire", DefenseType: "grass", Code: -1}}, engine: true},
		{name: "未知の攻撃タイプ", types: testTypes(), chart: []master.TypeChartRow{{AttackType: "testunknown", DefenseType: "grass", Code: 4}}, engine: true},
		{name: "未知の防御タイプ", types: testTypes(), chart: []master.TypeChartRow{{AttackType: "fire", DefenseType: "testunknown", Code: 4}}, engine: true},
		{name: "タイプが空", types: nil, chart: nil, engine: true},
		{name: "同じ組の重複", types: testTypes(), chart: []master.TypeChartRow{
			{AttackType: "fire", DefenseType: "grass", Code: 4},
			{AttackType: "fire", DefenseType: "grass", Code: 1},
		}},
		{name: "タイプ ID の重複", types: append(testTypes(), master.TypeRow{ID: "fire", SortOrder: 9, NameJa: "テストにせほのお"})},
		{name: "sort_order の重複", types: append(testTypes(), master.TypeRow{ID: "testextra", SortOrder: 1, NameJa: "テストついか"})},
		{name: "タイプ ID が大文字", types: append(testTypes(), master.TypeRow{ID: "Fire", SortOrder: 9, NameJa: "テストおおもじ"})},
		{name: "タイプ ID が空", types: append(testTypes(), master.TypeRow{ID: "", SortOrder: 9, NameJa: "テストから"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := master.TypeChart(tc.types, tc.chart)
			if err == nil {
				t.Fatal("エラーにならなかった")
			}
			if !errors.Is(err, master.ErrInvalidRow) && !errors.Is(err, engine.ErrInvalidTypeChart) {
				t.Fatalf("ErrInvalidRow / engine.ErrInvalidTypeChart で判別できない: %v", err)
			}
			if tc.engine && !errors.Is(err, engine.ErrInvalidTypeChart) {
				t.Fatalf("engine.ErrInvalidTypeChart で判別できない: %v", err)
			}
		})
	}
}

// --- 種族 --------------------------------------------------------------------

func baseSpeciesRow() master.SpeciesRow {
	return master.SpeciesRow{
		Key: "9001-000", DexNo: 9001, Form: 0, ShowdownID: "testmon",
		NameJa: "テストモン", NameEn: "Testmon",
		Type1: "fire", Type2: "",
		BaseHP: 80, BaseAtk: 90, BaseDef: 70, BaseSpA: 100, BaseSpD: 75, BaseSpe: 85,
	}
}

func megaSpeciesRow() master.SpeciesRow {
	return master.SpeciesRow{
		Key: "9001-001", DexNo: 9001, Form: 1, ShowdownID: "testmonmega",
		NameJa: "テストモン(メガ)", NameEn: "Testmon-Mega",
		Type1: "fire", Type2: "water",
		BaseHP: 80, BaseAtk: 110, BaseDef: 90, BaseSpA: 130, BaseSpD: 95, BaseSpe: 95,
		IsMega: true, BaseSpeciesKey: "9001-000", RequiredItemID: "teststone",
	}
}

func TestSpeciesMapsAllEngineFields(t *testing.T) {
	c := testChart(t)
	abilities := []master.SpeciesAbilityRow{{Slot: 3, AbilityID: "testhidden"}, {Slot: 1, AbilityID: "testability"}}
	got, err := master.Species(baseSpeciesRow(), abilities, c)
	if err != nil {
		t.Fatalf("Species: %v", err)
	}
	want := engine.Species{
		Key: "9001-000", DexNo: 9001, Form: 0, NameJa: "テストモン",
		Types:     []engine.Type{"fire"},
		BaseStats: engine.Stats{HP: 80, Atk: 90, Def: 70, SpA: 100, SpD: 75, Spe: 85},
		Abilities: []string{"testability", "testhidden"}, // slot 順
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	assertAllFieldsSet(t, "engine.Species", got)
}

func TestSpeciesMapsShowdownSpecialAbilitySlot(t *testing.T) {
	abilities := []master.SpeciesAbilityRow{
		{Slot: 1, AbilityID: "testone"},
		{Slot: 2, AbilityID: "testtwo"},
		{Slot: 3, AbilityID: "testhidden"},
		{Slot: 4, AbilityID: "testspecial"},
	}
	got, err := master.Species(baseSpeciesRow(), abilities, testChart(t))
	if err != nil {
		t.Fatalf("Species: %v", err)
	}
	want := []string{"testone", "testtwo", "testhidden", "testspecial"}
	if !reflect.DeepEqual(got.Abilities, want) {
		t.Fatalf("Abilities = %v, want %v", got.Abilities, want)
	}
}

func TestSpeciesMegaAndDualType(t *testing.T) {
	c := testChart(t)
	got, err := master.Species(megaSpeciesRow(), []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testguard"}}, c)
	if err != nil {
		t.Fatalf("Species: %v", err)
	}
	if want := []engine.Type{"fire", "water"}; !reflect.DeepEqual(got.Types, want) {
		t.Fatalf("Types = %v, want %v", got.Types, want)
	}
	if got.Key != "9001-001" || got.Form != 1 {
		t.Fatalf("Key/Form = %s/%d", got.Key, got.Form)
	}
}

func TestSpeciesRejectsInvalidRows(t *testing.T) {
	c := testChart(t)
	oneAbility := []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testability"}}
	cases := []struct {
		name      string
		mutate    func(*master.SpeciesRow)
		abilities []master.SpeciesAbilityRow
	}{
		{name: "key の形式", mutate: func(r *master.SpeciesRow) { r.Key = "9001-0" }},
		{name: "key と図鑑番号の不一致", mutate: func(r *master.SpeciesRow) { r.DexNo = 9002 }},
		{name: "key とフォルムの不一致", mutate: func(r *master.SpeciesRow) { r.Form = 2 }},
		{name: "未知のタイプ1", mutate: func(r *master.SpeciesRow) { r.Type1 = "testunknown" }},
		{name: "未知のタイプ2", mutate: func(r *master.SpeciesRow) { r.Type2 = "testunknown" }},
		{name: "タイプ1が空", mutate: func(r *master.SpeciesRow) { r.Type1 = "" }},
		{name: "タイプ1とタイプ2が同じ", mutate: func(r *master.SpeciesRow) { r.Type2 = "fire" }},
		{name: "種族値0", mutate: func(r *master.SpeciesRow) { r.BaseSpe = 0 }},
		{name: "種族値256", mutate: func(r *master.SpeciesRow) { r.BaseHP = 256 }},
		{name: "日本語名が空", mutate: func(r *master.SpeciesRow) { r.NameJa = "" }},
		{name: "図鑑番号が0", mutate: func(r *master.SpeciesRow) { r.DexNo, r.Form, r.Key = 0, 0, "0000-000" }},
		{name: "図鑑番号が10000", mutate: func(r *master.SpeciesRow) { r.DexNo, r.Key = 10000, "10000-000" }},
		{name: "フォルムが1000", mutate: func(r *master.SpeciesRow) { r.Form, r.Key = 1000, "9001-1000" }},
		{name: "showdown_id に大文字", mutate: func(r *master.SpeciesRow) { r.ShowdownID = "TestMon" }},
		{name: "showdown_id が空", mutate: func(r *master.SpeciesRow) { r.ShowdownID = "" }},
		{name: "base_species_key の形式", mutate: func(r *master.SpeciesRow) {
			r.IsMega = true
			r.BaseSpeciesKey = "900-000"
			r.RequiredItemID = "teststone"
		}},
		{name: "required_item_id の形式", mutate: func(r *master.SpeciesRow) {
			r.IsMega = true
			r.BaseSpeciesKey = "9000-000"
			r.RequiredItemID = "TestStone"
		}},
		{name: "メガなのに持ち物が無い", mutate: func(r *master.SpeciesRow) { r.IsMega = true; r.BaseSpeciesKey = "9000-000" }},
		{name: "メガなのに元の種族が無い", mutate: func(r *master.SpeciesRow) { r.IsMega = true; r.RequiredItemID = "teststone" }},
		{name: "メガでないのに持ち物がある", mutate: func(r *master.SpeciesRow) { r.RequiredItemID = "teststone" }},
		{name: "メガでないのに元の種族がある", mutate: func(r *master.SpeciesRow) { r.BaseSpeciesKey = "9000-000" }},
		{name: "元の種族が自分自身", mutate: func(r *master.SpeciesRow) {
			r.IsMega = true
			r.BaseSpeciesKey = r.Key
			r.RequiredItemID = "teststone"
		}},
		{name: "特性が無い", abilities: []master.SpeciesAbilityRow{}},
		{name: "特性スロット0", abilities: []master.SpeciesAbilityRow{{Slot: 0, AbilityID: "testability"}}},
		{name: "特性スロット5", abilities: []master.SpeciesAbilityRow{{Slot: 5, AbilityID: "testability"}}},
		{name: "特性スロットの重複", abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testability"}, {Slot: 1, AbilityID: "testguard"}}},
		{name: "同じ特性が2スロット", abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testability"}, {Slot: 2, AbilityID: "testability"}}},
		{name: "特性 ID が空", abilities: []master.SpeciesAbilityRow{{Slot: 1, AbilityID: ""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := baseSpeciesRow()
			if tc.mutate != nil {
				tc.mutate(&row)
			}
			abilities := oneAbility
			if tc.abilities != nil {
				abilities = tc.abilities
			}
			_, err := master.Species(row, abilities, c)
			if !errors.Is(err, master.ErrInvalidRow) {
				t.Fatalf("ErrInvalidRow にならない: %v", err)
			}
		})
	}
}

func TestSpeciesRequiresTypeChart(t *testing.T) {
	_, err := master.Species(baseSpeciesRow(), []master.SpeciesAbilityRow{{Slot: 1, AbilityID: "testability"}}, engine.TypeChart{})
	if err == nil {
		t.Fatal("表がゼロ値でもエラーにならなかった(黙ってタイプを通さない)")
	}
}

// --- 技 ----------------------------------------------------------------------

func TestMoveMapsAllEngineFields(t *testing.T) {
	c := testChart(t)
	got, err := master.Move(master.MoveRow{ID: "testflame", NameJa: "テストフレイム", Type: "fire", Category: "special", Power: 90, Priority: 1,
		Mechanisms: []string{"variable_power", "multi_hit"}}, c)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	// 機構は昇順に並べて engine.Move に載せる(ADR-0123)。
	want := engine.Move{ID: "testflame", NameJa: "テストフレイム", Type: "fire", Category: engine.CategorySpecial, Power: 90, Priority: 1,
		Mechanisms: []engine.MoveMechanism{engine.MechanismMultiHit, engine.MechanismVariablePower}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	assertAllFieldsSet(t, "engine.Move", got)
}

func TestMoveRejectsInvalidRows(t *testing.T) {
	c := testChart(t)
	base := master.MoveRow{ID: "testflame", NameJa: "テストフレイム", Type: "fire", Category: "special", Power: 90}
	cases := []struct {
		name   string
		mutate func(*master.MoveRow)
	}{
		{"ID に大文字", func(r *master.MoveRow) { r.ID = "TestFlame" }},
		{"ID に記号", func(r *master.MoveRow) { r.ID = "test-flame" }},
		{"ID が空", func(r *master.MoveRow) { r.ID = "" }},
		{"未知のタイプ", func(r *master.MoveRow) { r.Type = "testunknown" }},
		{"タイプが空", func(r *master.MoveRow) { r.Type = "" }},
		{"未知の分類", func(r *master.MoveRow) { r.Category = "testcategory" }},
		{"負の威力", func(r *master.MoveRow) { r.Power = -1 }},
		{"変化技に威力", func(r *master.MoveRow) { r.Category = "status"; r.Power = 10 }},
		{"優先度+6", func(r *master.MoveRow) { r.Priority = 6 }},
		{"優先度-8", func(r *master.MoveRow) { r.Priority = -8 }},
		{"日本語名が空", func(r *master.MoveRow) { r.NameJa = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.mutate(&row)
			if _, err := master.Move(row, c); !errors.Is(err, master.ErrInvalidRow) {
				t.Fatalf("ErrInvalidRow にならない: %v", err)
			}
		})
	}
}

// --- 持ち物・特性 -------------------------------------------------------------

func TestItemWithoutEffectRowHasNilEffect(t *testing.T) {
	got, err := master.Item(master.ItemRow{ID: "testplain", NameJa: "テストただのもの"}, testChart(t))
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if got.Effect != nil || got.ID != "testplain" || got.NameJa != "テストただのもの" {
		t.Fatalf("got %+v", got)
	}
}

func TestItemAndAbilityDecodeEffect(t *testing.T) {
	c := testChart(t)
	item, err := master.Item(master.ItemRow{ID: "testorb", NameJa: "テストオーブ", Effect: []byte(`{"DamageMod":5324}`)}, c)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if item.Effect == nil || item.Effect.DamageMod != 5324 {
		t.Fatalf("Item.Effect = %+v", item.Effect)
	}
	ab, err := master.Ability(master.AbilityRow{ID: "testguard", NameJa: "テストガード", Effect: []byte(`{"DefResistType":{"fire":2048,"water":2048}}`)}, c)
	if err != nil {
		t.Fatalf("Ability: %v", err)
	}
	want := map[engine.Type]int{"fire": 2048, "water": 2048}
	if ab.Effect == nil || !reflect.DeepEqual(ab.Effect.DefResistType, want) {
		t.Fatalf("Ability.Effect = %+v", ab.Effect)
	}
}

func TestItemAndAbilityRejectInvalidRows(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		run  func() error
		want error
	}{
		{"持ち物 ID に大文字", func() error {
			_, err := master.Item(master.ItemRow{ID: "TestOrb", NameJa: "テストオーブ"}, c)
			return err
		}, master.ErrInvalidRow},
		{"持ち物の日本語名が空", func() error { _, err := master.Item(master.ItemRow{ID: "testorb"}, c); return err }, master.ErrInvalidRow},
		{"特性 ID が空", func() error {
			_, err := master.Ability(master.AbilityRow{NameJa: "テストとくせい"}, c)
			return err
		}, master.ErrInvalidRow},
		{"持ち物の効果が壊れた JSON", func() error {
			_, err := master.Item(master.ItemRow{ID: "testorb", NameJa: "テストオーブ", Effect: []byte(`{"DamageMod":`)}, c)
			return err
		}, master.ErrInvalidEffect},
		{"特性の効果に未知のフィールド", func() error {
			_, err := master.Ability(master.AbilityRow{ID: "testability", NameJa: "テストとくせい", Effect: []byte(`{"StabMod":8192,"TestUnknown":1}`)}, c)
			return err
		}, master.ErrInvalidEffect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, tc.want) {
				t.Fatalf("%v にならない: %v", tc.want, err)
			}
		})
	}
}

// --- 効果定義の JSON(ADR-0005 / ADR-0100 §6) --------------------------------

// fullItemEffect は ItemEffect の全フィールドを埋めた値。engine にフィールドが増えたら
// TestEffectFixturesCoverAllFields が落ちて、写像とこの fixture の更新を強制する。
func fullItemEffect() engine.ItemEffect {
	return engine.ItemEffect{
		StatMods:           map[engine.StatKey]int{engine.StatAtk: 6144, engine.StatSpD: 6144},
		DamageMod:          5324,
		PowerMod:           4505,
		PowerCategory:      engine.CategoryPhysical,
		OnlySuperEffective: true,
		BoostType:          "fire",
		BoostTypeMod:       4915,
		ResistBerryType:    "water",
		// 未対応の印(ADR-0123)。本番のデータでは補正と混ぜないが(生成器が確かめる)、形としては往復できる。
		UnsupportedAttacker: true,
		UnsupportedDefender: true,
	}
}

func fullAbilityEffect() engine.AbilityEffect {
	return engine.AbilityEffect{
		StabMod:              8192,
		OffBoostType:         "grass",
		OffBoostTypeMod:      6144,
		DefResistType:        map[engine.Type]int{"fire": 2048, "normal": 2048},
		DefImmuneTypes:       []engine.Type{"grass"},
		DefAbsorbTypes:       map[engine.Type]engine.AbsorbEffect{"water": {HealNumerator: 1, HealDenominator: 4}},
		ReduceSuperEffective: 3072,
		IgnoresBurn:          true,
		Airborne:             true,
		UnsupportedAttacker:  true,
		UnsupportedDefender:  true,
	}
}

func TestEffectFixturesCoverAllFields(t *testing.T) {
	assertAllFieldsSet(t, "engine.ItemEffect", fullItemEffect())
	assertAllFieldsSet(t, "engine.AbilityEffect", fullAbilityEffect())
}

func TestItemEffectRoundTrip(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		e    engine.ItemEffect
	}{
		{"全フィールド", fullItemEffect()},
		{"最終ダメージ倍率だけ", engine.ItemEffect{DamageMod: 5324}},
		{"実数値倍率だけ", engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatDef: 6144, engine.StatSpD: 6144}}},
		{"半減きのみ", engine.ItemEffect{ResistBerryType: "grass"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := master.EncodeItemEffect(tc.e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := master.DecodeItemEffect(raw, c)
			if err != nil {
				t.Fatalf("Decode(%s): %v", raw, err)
			}
			if got == nil || !reflect.DeepEqual(*got, tc.e) {
				t.Fatalf("往復で変わった: %s → %+v, want %+v", raw, got, tc.e)
			}
		})
	}
}

func TestAbilityEffectRoundTrip(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		e    engine.AbilityEffect
	}{
		{"全フィールド", fullAbilityEffect()},
		{"タイプ一致補正だけ", engine.AbilityEffect{StabMod: 8192}},
		{"やけど無視だけ", engine.AbilityEffect{IgnoresBurn: true}},
		{"浮いているだけ(ADR-0116)", engine.AbilityEffect{Airborne: true}},
		{"無効と浮いているの組合せ(ふゆうの形。表にあるタイプで)", engine.AbilityEffect{DefImmuneTypes: []engine.Type{"grass"}, Airborne: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := master.EncodeAbilityEffect(tc.e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := master.DecodeAbilityEffect(raw, c)
			if err != nil {
				t.Fatalf("Decode(%s): %v", raw, err)
			}
			if got == nil || !reflect.DeepEqual(*got, tc.e) {
				t.Fatalf("往復で変わった: %s → %+v, want %+v", raw, got, tc.e)
			}
		})
	}
}

func TestEncodeEffectIsCanonical(t *testing.T) {
	cases := []struct {
		name string
		run  func() ([]byte, error)
		want string
	}{
		{"ゼロ値を省く", func() ([]byte, error) { return master.EncodeItemEffect(engine.ItemEffect{DamageMod: 5324}) }, `{"DamageMod":5324}`},
		{"map のキーは昇順", func() ([]byte, error) {
			return master.EncodeItemEffect(engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatSpD: 6144, engine.StatDef: 6144}})
		}, `{"StatMods":{"def":6144,"spd":6144}}`},
		{"フィールドは定義順", func() ([]byte, error) {
			return master.EncodeItemEffect(engine.ItemEffect{BoostType: "fire", BoostTypeMod: 4915})
		}, `{"BoostType":"fire","BoostTypeMod":4915}`},
		{"特性", func() ([]byte, error) {
			return master.EncodeAbilityEffect(engine.AbilityEffect{DefResistType: map[engine.Type]int{"water": 2048, "fire": 2048}})
		}, `{"DefResistType":{"fire":2048,"water":2048}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.run()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEncodeEmptyEffectIsRejected(t *testing.T) {
	if _, err := master.EncodeItemEffect(engine.ItemEffect{}); !errors.Is(err, master.ErrInvalidEffect) {
		t.Fatalf("空の ItemEffect: %v", err)
	}
	if _, err := master.EncodeAbilityEffect(engine.AbilityEffect{}); !errors.Is(err, master.ErrInvalidEffect) {
		t.Fatalf("空の AbilityEffect: %v", err)
	}
}

func TestDecodeItemEffectRejectsInvalidJSON(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
	}{
		{"未知のフィールド", `{"DamageMod":5324,"TestUnknown":1}`},
		{"フィールド名の大文字小文字違い", `{"damageMod":5324}`},
		{"オブジェクトでない", `[5324]`},
		{"null", `null`},
		{"空のオブジェクト", `{}`},
		{"ゼロ値だけ", `{"DamageMod":0}`},
		{"後続のデータ", `{"DamageMod":5324} {"DamageMod":1}`},
		{"小数", `{"DamageMod":1.3}`},
		{"小数表記の整数", `{"DamageMod":5324.0}`},
		{"文字列の数値", `{"DamageMod":"5324"}`},
		{"負の倍率", `{"DamageMod":-5324}`},
		{"負の実数値倍率", `{"StatMods":{"atk":-6144}}`},
		{"実数値倍率のキーが hp", `{"StatMods":{"hp":6144}}`},
		{"実数値倍率のキーが未知", `{"StatMods":{"testunknown":6144}}`},
		{"実数値倍率が空", `{"StatMods":{}}`},
		{"タイプ強化のタイプが表に無い", `{"BoostType":"testunknown","BoostTypeMod":4915}`},
		{"タイプ強化の倍率だけ", `{"BoostTypeMod":4915}`},
		{"タイプ強化のタイプだけ", `{"BoostType":"fire"}`},
		{"半減きのみのタイプが表に無い", `{"ResistBerryType":"testunknown"}`},
		{"威力分類が変化", `{"PowerMod":4505,"PowerCategory":"status"}`},
		{"威力分類が未知", `{"PowerMod":4505,"PowerCategory":"testcategory"}`},
		{"威力分類だけ", `{"PowerCategory":"physical"}`},
		{"抜群限定だけ", `{"OnlySuperEffective":true}`},
		{"壊れた JSON", `{"DamageMod":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.DecodeItemEffect([]byte(tc.raw), c)
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Fatalf("ErrInvalidEffect にならない: got=%+v err=%v", got, err)
			}
		})
	}
}

func TestDecodeAbilityEffectRejectsInvalidJSON(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
	}{
		{"未知のフィールド", `{"StabMod":8192,"TestUnknown":true}`},
		{"空のオブジェクト", `{}`},
		{"小数", `{"StabMod":8192.5}`},
		{"負の倍率", `{"ReduceSuperEffective":-3072}`},
		{"相手の補正のタイプが表に無い", `{"DefResistType":{"testunknown":2048}}`},
		{"相手の補正が負", `{"DefResistType":{"fire":-2048}}`},
		{"攻撃強化のタイプだけ", `{"OffBoostType":"fire"}`},
		{"攻撃強化の倍率だけ", `{"OffBoostTypeMod":6144}`},
		{"攻撃強化のタイプが表に無い", `{"OffBoostType":"testunknown","OffBoostTypeMod":6144}`},
		{"後続のデータ", `{"StabMod":8192}x`},
		{"浮いているが false(省略で表す)", `{"Airborne":false}`},
		{"浮いているが真偽値でない", `{"Airborne":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.DecodeAbilityEffect([]byte(tc.raw), c)
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Fatalf("ErrInvalidEffect にならない: got=%+v err=%v", got, err)
			}
		})
	}
}

// TestDecodeAcceptsGoldenEffectsFormat は DB の効果定義が ADR-0005 の effects.json と同じ形であることを確かめる。
// effects.json はテスト専用のアダプタ(名前は英語、値は数値と英語識別子だけ)。表は effects.json に現れるタイプで作る。
func TestDecodeAcceptsGoldenEffectsFormat(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/golden/effects.json")
	if err != nil {
		t.Fatalf("effects.json が読めない(スキップしない): %v", err)
	}
	var file struct {
		Items     map[string]json.RawMessage `json:"items"`
		Abilities map[string]json.RawMessage `json:"abilities"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("effects.json: %v", err)
	}
	if len(file.Items) == 0 || len(file.Abilities) == 0 {
		t.Fatal("effects.json の items / abilities が空")
	}
	chart := chartOfTypesIn(t, raw)
	for _, name := range sortedKeys(file.Items) {
		if _, err := master.DecodeItemEffect(file.Items[name], chart); err != nil {
			t.Errorf("items[%q]: %v", name, err)
		}
	}
	for _, name := range sortedKeys(file.Abilities) {
		if _, err := master.DecodeAbilityEffect(file.Abilities[name], chart); err != nil {
			t.Errorf("abilities[%q]: %v", name, err)
		}
	}
}

// --- ヘルパー ----------------------------------------------------------------

// chartOfTypesIn は effects.json に現れる小文字のタイプ ID(値と map のキー・配列の要素)だけで
// 表を作る。タイプの一覧をテストに書かないため、ファイルの内容から集める。
func chartOfTypesIn(t *testing.T, raw []byte) engine.TypeChart {
	t.Helper()
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	typeKeys := map[string]bool{
		"BoostType": true, "ResistBerryType": true, "OffBoostType": true, "DefImmuneTypes": true,
	}
	mapKeyTypeParents := map[string]bool{"DefResistType": true, "DefAbsorbTypes": true}
	seen := map[string]bool{}
	var walk func(parent string, v any)
	walk = func(parent string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if mapKeyTypeParents[parent] {
					seen[k] = true
				}
				walk(k, child)
			}
		case []any:
			for _, child := range x {
				walk(parent, child)
			}
		case string:
			if typeKeys[parent] {
				seen[x] = true
			}
		}
	}
	walk("", doc)
	rows := make([]master.TypeRow, 0, len(seen))
	for i, id := range sortedKeys(seen) {
		rows = append(rows, master.TypeRow{ID: id, SortOrder: i + 1, NameJa: "テスト" + id})
	}
	c, err := master.TypeChart(rows, nil)
	if err != nil {
		t.Fatalf("TypeChart: %v", err)
	}
	return c
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// assertAllFieldsSet は struct の全フィールドがゼロ値でないことを確かめる(写像で情報が落ちないことの前提)。
func assertAllFieldsSet(t *testing.T, name string, v any) {
	t.Helper()
	rv := reflect.ValueOf(v)
	var zero []string
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		if rv.Field(i).IsZero() && !zeroAllowed(name, f.Name) {
			zero = append(zero, f.Name)
		}
	}
	if len(zero) > 0 {
		t.Fatalf("%s のフィールドがゼロ値: %s(fixture を埋めるか、写像を更新する)", name, strings.Join(zero, ", "))
	}
}

// zeroAllowed はゼロ値が正しい値であるフィールド(フォルム0 = 基本の姿、追加効果なしの技)。
func zeroAllowed(structName, field string) bool {
	if structName == "engine.Species" && field == "Form" {
		return true
	}
	if structName == "engine.Move" && field == "Effect" {
		return true
	}
	return false
}
