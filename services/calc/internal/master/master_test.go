package master

// マスタ境界(ADR-0200 §3 → ADR-0204 §2)の受け入れテスト。入力は MasterExport(pokedex-svc の内部 API の本文)。
// データはすべて架空(種族・技・持ち物・特性・性格の名前と ID)。相性表だけは testdata/golden/typechart.json
// (数値と英語 ID のみ。ADR-0002 / ADR-0015)を行の形にして使う。
//
// ADR-0204 で暫定スナップショット形式(LoadSnapshot / LoadTypeChart / New)を廃止した。旧テストの期待値
// (Lookup の中身・コピーを返すこと・NatureID の写像)は変えず、入力の作り方だけを MasterExport に移した。
// 技・持ち物・特性の ID は共通マスタの形式(^[a-z0-9]+$。ADR-0100 §2)に合わせてハイフンを除いた。

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	sharedmaster "example.com/pokecalc/services/internal/master"
)

const (
	sharedTypeChartPath = "../../../../testdata/golden/typechart.json"
	exampleMasterPath   = "../../testdata/master.example.json"
)

func typePtr(t api.PokeType) *api.PokeType { return &t }
func statPtr(s api.StatKey) *api.StatKey   { return &s }
func strPtr(s string) *string              { return &s }
func effect(m map[string]any) *api.MasterEffect {
	e := api.MasterEffect(m)
	return &e
}

// sharedTypeRows は共有の相性表(typechart.json)を types / type_chart の行の形にする(等倍は省略)。
// 実装(FromExport)とは独立に、JSON を直接読んで作る。
func sharedTypeRows(t *testing.T) ([]api.MasterType, []api.MasterTypeChartEntry) {
	t.Helper()
	b, err := os.ReadFile(sharedTypeChartPath)
	if err != nil {
		t.Fatalf("相性表を読めない: %v", err)
	}
	var raw struct {
		Types         []string                  `json:"types"`
		Effectiveness map[string]map[string]int `json:"effectiveness"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("相性表を解釈できない: %v", err)
	}
	types := make([]api.MasterType, 0, len(raw.Types))
	var chart []api.MasterTypeChartEntry
	for i, ty := range raw.Types {
		types = append(types, api.MasterType{Id: api.PokeType(ty), SortOrder: i + 1, NameJa: "テスト" + ty})
	}
	for _, atk := range raw.Types {
		for _, def := range raw.Types {
			code, ok := raw.Effectiveness[atk][def]
			if !ok || code == engine.TypeCodeNeutral {
				continue
			}
			chart = append(chart, api.MasterTypeChartEntry{
				AttackType: api.PokeType(atk), DefenseType: api.PokeType(def), Code: api.MasterTypeChartEntryCode(code),
			})
		}
	}
	return types, chart
}

func stats(hp, atk, def, spa, spd, spe int) api.StatBlock {
	return api.StatBlock{Hp: hp, Atk: atk, Def: def, Spa: spa, Spd: spd, Spe: spe}
}

// baseExport は最小の妥当なマスタ一式。各ケースはこれを書き換えて不正を作る。
func baseExport(t *testing.T) api.MasterExport {
	t.Helper()
	types, chart := sharedTypeRows(t)
	return api.MasterExport{
		SchemaVersion: 1,
		DataVersion:   "test-1",
		Types:         types,
		TypeChart:     chart,
		Species: []api.MasterSpecies{
			{Key: "9001-000", DexNo: 9001, Form: 0, ShowdownId: "testmon", NameJa: "テストモン",
				Type1: api.PokeTypeNormal, BaseStats: stats(80, 100, 70, 60, 70, 90),
				Abilities: []api.MasterSpeciesAbility{{Slot: 1, AbilityId: "testplain"}}},
			{Key: "9002-000", DexNo: 9002, Form: 0, ShowdownId: "testguard", NameJa: "テストガード",
				Type1: api.PokeTypeWater, Type2: typePtr(api.PokeTypeSteel), BaseStats: stats(95, 60, 90, 70, 85, 50),
				Abilities: []api.MasterSpeciesAbility{{Slot: 1, AbilityId: "testplain"}}},
			{Key: "9002-001", DexNo: 9002, Form: 1, ShowdownId: "testguardmega", NameJa: "テストガードメガ",
				Type1: api.PokeTypeWater, Type2: typePtr(api.PokeTypeSteel), BaseStats: stats(95, 80, 120, 90, 105, 50),
				IsMega: true, BaseSpeciesKey: strPtr("9002-000"), RequiredItemId: strPtr("testguardite"),
				Abilities: []api.MasterSpeciesAbility{{Slot: 1, AbilityId: "testthick"}}},
		},
		Moves: []api.MasterMove{
			{Id: "testbeam", NameJa: "テストビーム", Type: api.PokeTypeNormal, Category: api.Physical, Power: 80},
			{Id: "testwave", NameJa: "テストウェーブ", Type: api.PokeTypeGrass, Category: api.Special, Power: 90},
		},
		Items: []api.MasterItem{
			{Id: "testplainitem", NameJa: "テストのいし"},
			{Id: "testorb", NameJa: "テストのたま", Effect: effect(map[string]any{"DamageMod": 5324})},
			{Id: "testshell", NameJa: "テストのから", Effect: effect(map[string]any{
				"StatMods": map[string]any{"def": 6144, "spd": 6144}})},
			{Id: "testguardite", NameJa: "テストガードナイト"},
		},
		Abilities: []api.MasterAbility{
			{Id: "testplain", NameJa: "テストとくせい"},
			{Id: "testthick", NameJa: "テストぶあつい", Effect: effect(map[string]any{
				"DefResistType": map[string]any{"fire": 2048}})},
		},
		// ファイルの並びと ID の昇順をわざとずらす(代表の選び方が並びに依存しないことを見る)。
		// 性格の ID の形式は calc-svc では検査しない(旧テストの ID をそのまま使う)。
		Natures: []api.MasterNature{
			{Id: "test-neutral-z", NameJa: "テストむほせいZ"},
			{Id: "test-def-up", NameJa: "テストかたい", Plus: statPtr(api.StatKeyDef), Minus: statPtr(api.StatKeyAtk)},
			{Id: "test-neutral-b", NameJa: "テストむほせいB"},
			{Id: "test-atk-up", NameJa: "テストつよい", Plus: statPtr(api.StatKeyAtk), Minus: statPtr(api.StatKeySpa)},
		},
	}
}

func newStore(t *testing.T, export api.MasterExport) *MemoryStore {
	t.Helper()
	store, err := FromExport(export)
	if err != nil {
		t.Fatalf("FromExport = %v, want nil", err)
	}
	return store
}

// AC-M1: 妥当なマスタ一式を読み、ID で engine の型が引ける(写像は共通マスタ。ADR-0204 §2)。
func TestFromExportAndLookup(t *testing.T) {
	store := newStore(t, baseExport(t))

	sp, ok := store.Species("9002-000")
	if !ok {
		t.Fatal("Species(9002-000) が見つからない")
	}
	wantSp := engine.Species{
		Key: "9002-000", DexNo: 9002, Form: 0, NameJa: "テストガード",
		Types:     []engine.Type{engine.TypeWater, engine.TypeSteel},
		BaseStats: engine.Stats{HP: 95, Atk: 60, Def: 90, SpA: 70, SpD: 85, Spe: 50},
		Abilities: []string{"testplain"},
	}
	if !equalJSON(sp, wantSp) {
		t.Errorf("Species = %+v, want %+v", sp, wantSp)
	}
	single, ok := store.Species("9001-000")
	if !ok || len(single.Types) != 1 || single.Types[0] != engine.TypeNormal {
		t.Errorf("Species(9001-000) = %+v, %v, want 単タイプ normal(type2 null)", single, ok)
	}
	mega, ok := store.Species("9002-001")
	if !ok || mega.Form != 1 || mega.BaseStats.Def != 120 {
		t.Errorf("Species(9002-001) = %+v, %v, want メガの行(form 1・防御 120)", mega, ok)
	}

	mv, ok := store.Move("testwave")
	want := engine.Move{ID: "testwave", NameJa: "テストウェーブ", Type: engine.TypeGrass, Category: engine.CategorySpecial, Power: 90}
	if !ok || mv != want {
		t.Errorf("Move = %+v, %v, want %+v, true", mv, ok, want)
	}

	orb, ok := store.Item("testorb")
	if !ok || orb.Effect == nil || orb.Effect.DamageMod != 5324 || orb.NameJa != "テストのたま" {
		t.Errorf("Item(testorb) = %+v, %v, want DamageMod 5324", orb, ok)
	}
	shell, ok := store.Item("testshell")
	if !ok || shell.Effect == nil || shell.Effect.StatMods[engine.StatDef] != 6144 || shell.Effect.StatMods[engine.StatSpD] != 6144 {
		t.Errorf("Item(testshell) = %+v, %v, want StatMods def/spd 6144", shell, ok)
	}
	plain, ok := store.Item("testplainitem")
	if !ok || plain.Effect != nil {
		t.Errorf("Item(testplainitem) = %+v, %v, want Effect nil(効果なし)", plain, ok)
	}

	thick, ok := store.Ability("testthick")
	if !ok || thick.Effect == nil || thick.Effect.DefResistType[engine.TypeFire] != 2048 {
		t.Errorf("Ability(testthick) = %+v, %v, want DefResistType fire 2048", thick, ok)
	}
	plainAb, ok := store.Ability("testplain")
	if !ok || plainAb.Effect != nil {
		t.Errorf("Ability(testplain) = %+v, %v, want Effect nil", plainAb, ok)
	}

	n, ok := store.Nature("test-def-up")
	if !ok || n != (engine.Nature{Plus: engine.StatDef, Minus: engine.StatAtk}) {
		t.Errorf("Nature(test-def-up) = %+v, %v, want +def/-atk", n, ok)
	}
	n, ok = store.Nature("test-neutral-z")
	if !ok || n != engine.NatureNeutral {
		t.Errorf("Nature(test-neutral-z) = %+v, %v, want 無補正", n, ok)
	}

	if store.TypeChart().IsZero() {
		t.Error("TypeChart() がゼロ値")
	}
	if got := store.DataVersion(); got != "test-1" {
		t.Errorf("DataVersion() = %q, want %q", got, "test-1")
	}
}

// AC-M1: 相性表は MasterExport の types / typeChart から作る(等倍の省略は等倍。ADR-0013)。代表的な組を固定する。
func TestFromExportTypeChart(t *testing.T) {
	chart := newStore(t, baseExport(t)).TypeChart()
	if got := len(chart.Types()); got != 18 {
		t.Errorf("タイプ数 = %d, want 18", got)
	}
	cases := []struct {
		atk, def engine.Type
		want     int
	}{
		{engine.TypeFire, engine.TypeGrass, engine.TypeCodeSuperEffective},
		{engine.TypeNormal, engine.TypeGhost, engine.TypeCodeImmune},
		{engine.TypeWater, engine.TypeGrass, engine.TypeCodeNotVeryEffective},
		{engine.TypeNormal, engine.TypeNormal, engine.TypeCodeNeutral}, // 行が無い組は等倍
	}
	for _, c := range cases {
		got, err := chart.Code(c.atk, c.def)
		if err != nil || got != c.want {
			t.Errorf("Code(%s, %s) = %d, %v, want %d", c.atk, c.def, got, err, c.want)
		}
	}
}

// AC-M1: 未知の ID は false(panic しない)。
func TestLookupUnknownIDs(t *testing.T) {
	store := newStore(t, baseExport(t))
	if _, ok := store.Species("9999-000"); ok {
		t.Error("Species(未知) = true")
	}
	if _, ok := store.Move("testnothing"); ok {
		t.Error("Move(未知) = true")
	}
	if _, ok := store.Item("testnothing"); ok {
		t.Error("Item(未知) = true")
	}
	if _, ok := store.Ability("testnothing"); ok {
		t.Error("Ability(未知) = true")
	}
	if _, ok := store.Nature("test-nothing"); ok {
		t.Error("Nature(未知) = true")
	}
}

// Store の値を書き換えても次の参照に漏れない(並行に呼ばれるハンドラが共有するため)。
func TestLookupReturnsCopies(t *testing.T) {
	store := newStore(t, baseExport(t))
	sp, _ := store.Species("9002-000")
	sp.Types[0] = engine.TypeFire
	sp.Abilities[0] = "testchanged"
	again, _ := store.Species("9002-000")
	if again.Types[0] != engine.TypeWater || again.Abilities[0] != "testplain" {
		t.Errorf("返したスライスの書き換えが Store に漏れた: %+v", again)
	}
}

// critic 指摘 O2: Item / Ability の Effect(内部の map を含む)を書き換えても Store に漏れない。
func TestLookupReturnsCopiesOfEffects(t *testing.T) {
	store := newStore(t, baseExport(t))

	shell, _ := store.Item("testshell")
	shell.Effect.StatMods[engine.StatDef] = 9999
	shell.Effect.DamageMod = 9999
	again, _ := store.Item("testshell")
	if again.Effect.StatMods[engine.StatDef] != 6144 || again.Effect.DamageMod != 0 {
		t.Errorf("Item の Effect の書き換えが Store に漏れた: %+v", again.Effect)
	}

	thick, _ := store.Ability("testthick")
	thick.Effect.DefResistType[engine.TypeFire] = 9999
	thick.Effect.StabMod = 9999
	again2, _ := store.Ability("testthick")
	if again2.Effect.DefResistType[engine.TypeFire] != 2048 || again2.Effect.StabMod != 0 {
		t.Errorf("Ability の Effect の書き換えが Store に漏れた: %+v", again2.Effect)
	}
}

// 入力の MasterExport を FromExport の後で書き換えても Store に漏れない(effect の map を共有しない)。
func TestFromExportDoesNotAliasInput(t *testing.T) {
	export := baseExport(t)
	store := newStore(t, export)
	(*export.Items[1].Effect)["DamageMod"] = 9999
	export.Species[1].Abilities[0].AbilityId = "testchanged"
	orb, _ := store.Item("testorb")
	sp, _ := store.Species("9002-000")
	if orb.Effect.DamageMod != 5324 || sp.Abilities[0] != "testplain" {
		t.Errorf("入力の書き換えが Store に漏れた: orb=%+v species=%+v", orb.Effect, sp)
	}
}

// AC-M2: 不正なマスタ一式はロード時に ErrInvalidMaster。部分的な Store を返さない。
// 共通マスタ・engine が判別できる失敗は、その sentinel も errors.Is で見える(also)。
func TestFromExportRejectsInvalid(t *testing.T) {
	removeType := func(e *api.MasterExport, ty api.PokeType, alsoChartRows bool) {
		types := e.Types[:0:0]
		for _, row := range e.Types {
			if row.Id != ty {
				types = append(types, row)
			}
		}
		e.Types = types
		if !alsoChartRows {
			return
		}
		chart := e.TypeChart[:0:0]
		for _, row := range e.TypeChart {
			if row.AttackType != ty && row.DefenseType != ty {
				chart = append(chart, row)
			}
		}
		e.TypeChart = chart
	}

	tests := []struct {
		name   string
		mutate func(e *api.MasterExport)
		also   error // nil なら ErrInvalidMaster だけを見る
	}{
		{name: "schemaVersion が 2", mutate: func(e *api.MasterExport) { e.SchemaVersion = 2 }},
		{name: "schemaVersion が 0(欠落)", mutate: func(e *api.MasterExport) { e.SchemaVersion = 0 }},
		{name: "dataVersion が空", mutate: func(e *api.MasterExport) { e.DataVersion = "" }},

		// 相性表(ADR-0013。写像は共通マスタの TypeChart)
		{name: "types が空", mutate: func(e *api.MasterExport) { e.Types = nil; e.TypeChart = nil }},
		{name: "タイプ ID の重複", mutate: func(e *api.MasterExport) {
			e.Types = append(e.Types, api.MasterType{Id: api.PokeTypeFire, SortOrder: 99, NameJa: "テストfire2"})
		}, also: sharedmaster.ErrInvalidRow},
		{name: "sortOrder の重複", mutate: func(e *api.MasterExport) { e.Types[1].SortOrder = e.Types[0].SortOrder },
			also: sharedmaster.ErrInvalidRow},
		{name: "タイプの日本語名が空", mutate: func(e *api.MasterExport) { e.Types[0].NameJa = "" }, also: sharedmaster.ErrInvalidRow},
		{name: "未知のタイプ(列挙外)", mutate: func(e *api.MasterExport) {
			e.Types = append(e.Types, api.MasterType{Id: "cosmic", SortOrder: 99, NameJa: "テストcosmic"})
		}},
		{name: "相性の不正なコード 3", mutate: func(e *api.MasterExport) { e.TypeChart[0].Code = 3 }, also: engine.ErrInvalidTypeChart},
		{name: "相性の組の重複", mutate: func(e *api.MasterExport) { e.TypeChart = append(e.TypeChart, e.TypeChart[0]) },
			also: sharedmaster.ErrInvalidRow},
		{name: "相性の行のタイプが types に無い", mutate: func(e *api.MasterExport) { removeType(e, api.PokeTypeFairy, false) },
			also: engine.ErrInvalidTypeChart},

		// 種族(写像は共通マスタの Species)
		{name: "種族キーの重複", mutate: func(e *api.MasterExport) {
			dup := e.Species[0]
			dup.ShowdownId = "testmontwo"
			e.Species = append(e.Species, dup)
		}},
		{name: "種族の未知のタイプ(列挙外)", mutate: func(e *api.MasterExport) { e.Species[0].Type1 = "cosmic" }},
		{name: "種族のタイプが相性表に無い", mutate: func(e *api.MasterExport) { removeType(e, api.PokeTypeSteel, true) },
			also: sharedmaster.ErrInvalidRow},
		{name: "種族の type2 が type1 と同じ", mutate: func(e *api.MasterExport) { e.Species[1].Type2 = typePtr(api.PokeTypeWater) },
			also: sharedmaster.ErrInvalidRow},
		{name: "種族キーが dexNo/form と合わない", mutate: func(e *api.MasterExport) { e.Species[0].DexNo = 9005 },
			also: sharedmaster.ErrInvalidRow},
		{name: "showdownId の形式が不正", mutate: func(e *api.MasterExport) { e.Species[0].ShowdownId = "Test-Mon" },
			also: sharedmaster.ErrInvalidRow},
		{name: "種族値が範囲外", mutate: func(e *api.MasterExport) { e.Species[0].BaseStats.Spe = 0 }, also: sharedmaster.ErrInvalidRow},
		{name: "種族の特性が無い", mutate: func(e *api.MasterExport) { e.Species[0].Abilities = nil }, also: sharedmaster.ErrInvalidRow},
		{name: "種族の特性が特性一覧に無い", mutate: func(e *api.MasterExport) { e.Species[0].Abilities[0].AbilityId = "testnothing" }},
		{name: "メガなのに baseSpeciesKey が無い", mutate: func(e *api.MasterExport) { e.Species[2].BaseSpeciesKey = nil },
			also: sharedmaster.ErrInvalidRow},
		{name: "メガの baseSpeciesKey が種族一覧に無い", mutate: func(e *api.MasterExport) { e.Species[2].BaseSpeciesKey = strPtr("9009-000") }},
		{name: "メガの requiredItemId が持ち物一覧に無い", mutate: func(e *api.MasterExport) { e.Species[2].RequiredItemId = strPtr("testnothing") }},

		// 技(写像は共通マスタの Move)
		{name: "技 ID の重複", mutate: func(e *api.MasterExport) { e.Moves[1].Id = "testbeam" }},
		{name: "技 ID の形式が不正(ハイフン)", mutate: func(e *api.MasterExport) { e.Moves[0].Id = "test-beam" },
			also: sharedmaster.ErrInvalidRow},
		{name: "技のタイプが相性表に無い", mutate: func(e *api.MasterExport) { removeType(e, api.PokeTypeGrass, true) },
			also: sharedmaster.ErrInvalidRow},
		{name: "技の未知の分類", mutate: func(e *api.MasterExport) { e.Moves[0].Category = "magic" }},
		{name: "変化技なのに威力がある", mutate: func(e *api.MasterExport) { e.Moves[0].Category = api.Status },
			also: sharedmaster.ErrInvalidRow},

		// 持ち物・特性(効果定義は共通マスタの DecodeItemEffect / DecodeAbilityEffect。ADR-0005)
		{name: "持ち物 ID の重複", mutate: func(e *api.MasterExport) { e.Items[1].Id = "testplainitem" }},
		{name: "特性 ID の重複", mutate: func(e *api.MasterExport) { e.Abilities[1].Id = "testplain" }},
		{name: "持ち物効果の未知のキー(旧形式の camelCase)", mutate: func(e *api.MasterExport) {
			e.Items[1].Effect = effect(map[string]any{"damageMod": 5324})
		}, also: sharedmaster.ErrInvalidEffect},
		{name: "持ち物効果が空のオブジェクト", mutate: func(e *api.MasterExport) { e.Items[1].Effect = effect(map[string]any{}) },
			also: sharedmaster.ErrInvalidEffect},
		{name: "持ち物効果が 4096 基準の整数でない", mutate: func(e *api.MasterExport) {
			e.Items[1].Effect = effect(map[string]any{"DamageMod": 5324.5})
		}, also: sharedmaster.ErrInvalidEffect},
		{name: "持ち物効果の未知のステータスキー", mutate: func(e *api.MasterExport) {
			e.Items[2].Effect = effect(map[string]any{"StatMods": map[string]any{"luck": 6144}})
		}, also: sharedmaster.ErrInvalidEffect},
		{name: "特性効果の半減タイプが相性表に無い", mutate: func(e *api.MasterExport) {
			e.Abilities[1].Effect = effect(map[string]any{"DefResistType": map[string]any{"cosmic": 2048}})
		}, also: sharedmaster.ErrInvalidEffect},

		// 性格(calc-svc 側で検証する。ADR-0204 §2)
		{name: "性格 ID の重複", mutate: func(e *api.MasterExport) { e.Natures[1].Id = "test-neutral-z" }},
		{name: "性格 ID が空", mutate: func(e *api.MasterExport) { e.Natures[1].Id = "" }},
		{name: "性格の上昇が HP", mutate: func(e *api.MasterExport) { e.Natures[1].Plus = statPtr(api.StatKeyHp) }},
		{name: "性格の下降が HP", mutate: func(e *api.MasterExport) { e.Natures[1].Minus = statPtr(api.StatKeyHp) }},
		{name: "性格の未知のステータスキー", mutate: func(e *api.MasterExport) { e.Natures[1].Plus = statPtr("luck") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			export := baseExport(t)
			tt.mutate(&export)
			store, err := FromExport(export)
			if !errors.Is(err, ErrInvalidMaster) {
				t.Fatalf("FromExport err = %v, want ErrInvalidMaster", err)
			}
			if tt.also != nil && !errors.Is(err, tt.also) {
				t.Errorf("FromExport err = %v, want errors.Is(%v) も真(共通マスタ・engine のエラーを包むこと)", err, tt.also)
			}
			if store != nil {
				t.Errorf("失敗時に部分的な Store を返した: %+v", store)
			}
		})
	}
}

// AC-M5: NatureID の写像規則(ADR-0200 §2。ADR-0204 でも変えない)。
func TestNatureID(t *testing.T) {
	export := baseExport(t)
	// plus == minus は engine の IsNeutral で無補正。ID 昇順で最も小さいので、無補正の代表になる。
	export.Natures = append(export.Natures,
		api.MasterNature{Id: "test-neutral-a-same", NameJa: "テストおなじ", Plus: statPtr(api.StatKeySpe), Minus: statPtr(api.StatKeySpe)})
	store := newStore(t, export)

	tests := []struct {
		name   string
		nature engine.Nature
		wantID string
		wantOK bool
	}{
		{"無補正は ID 昇順の最初の無補正性格", engine.NatureNeutral, "test-neutral-a-same", true},
		{"plus==minus も無補正として同じ代表", engine.Nature{Plus: engine.StatAtk, Minus: engine.StatAtk}, "test-neutral-a-same", true},
		{"+B/-A は一致する性格", engine.Nature{Plus: engine.StatDef, Minus: engine.StatAtk}, "test-def-up", true},
		{"+A/-C は一致する性格", engine.Nature{Plus: engine.StatAtk, Minus: engine.StatSpA}, "test-atk-up", true},
		{"該当なし(+D/-A)", engine.Nature{Plus: engine.StatSpD, Minus: engine.StatAtk}, "", false},
		{"逆向き(+A/-B)は +B/-A と別物", engine.Nature{Plus: engine.StatAtk, Minus: engine.StatDef}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := store.NatureID(tt.nature)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("NatureID(%+v) = %q, %v, want %q, %v", tt.nature, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

// AC-M5: 無補正性格がマスタに1つも無ければ、無補正の写像は該当なし。
func TestNatureIDNoNeutralInMaster(t *testing.T) {
	export := baseExport(t)
	export.Natures = []api.MasterNature{
		{Id: "test-def-up", NameJa: "テストかたい", Plus: statPtr(api.StatKeyDef), Minus: statPtr(api.StatKeyAtk)},
	}
	store := newStore(t, export)
	if id, ok := store.NatureID(engine.NatureNeutral); ok || id != "" {
		t.Errorf("NatureID(無補正) = %q, %v, want \"\", false", id, ok)
	}
}

func equalJSON(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}
