package master

// 暫定マスタ境界(ADR-0018 §B)の受け入れテスト。データはすべて架空
// (種族・技・持ち物・特性・性格の名前と ID)。相性表だけは testdata/golden/typechart.json
// (数値と英語 ID のみ。ADR-0002 / ADR-0015)を読む。

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
)

const (
	sharedTypeChartPath = "../../../../testdata/golden/typechart.json"
	exampleMasterPath   = "../../testdata/master.example.json"
)

// baseSnapshot は最小の妥当なスナップショット。各ケースはこれを書き換えて不正を作る。
func baseSnapshot() map[string]any {
	return map[string]any{
		"schemaVersion": 1,
		"species": []any{
			map[string]any{
				"key": "9001-000", "dexNo": 9001, "form": 0, "nameJa": "テストモン",
				"types":     []any{"normal"},
				"baseStats": map[string]any{"hp": 80, "atk": 100, "def": 70, "spa": 60, "spd": 70, "spe": 90},
				"abilities": []any{"test-plain"},
			},
			map[string]any{
				"key": "9002-000", "dexNo": 9002, "form": 0, "nameJa": "テストガード",
				"types":     []any{"water", "steel"},
				"baseStats": map[string]any{"hp": 95, "atk": 60, "def": 90, "spa": 70, "spd": 85, "spe": 50},
				"abilities": []any{"test-plain"},
			},
		},
		"moves": []any{
			map[string]any{"id": "test-beam", "nameJa": "テストビーム", "type": "normal", "category": "physical", "power": 80, "priority": 0},
			map[string]any{"id": "test-wave", "nameJa": "テストウェーブ", "type": "grass", "category": "special", "power": 90, "priority": 0},
		},
		"items": []any{
			map[string]any{"id": "test-plain-item", "nameJa": "テストのいし", "effect": nil},
			map[string]any{"id": "test-orb", "nameJa": "テストのたま", "effect": map[string]any{
				"statMods": nil, "damageMod": 5324, "powerMod": 0, "powerCategory": "",
				"onlySuperEffective": false, "boostType": "", "boostTypeMod": 0, "resistBerryType": "",
			}},
			map[string]any{"id": "test-shell", "nameJa": "テストのから", "effect": map[string]any{
				"statMods": map[string]any{"def": 6144, "spd": 6144}, "damageMod": 0, "powerMod": 0, "powerCategory": "",
				"onlySuperEffective": false, "boostType": "", "boostTypeMod": 0, "resistBerryType": "",
			}},
		},
		"abilities": []any{
			map[string]any{"id": "test-plain", "nameJa": "テストとくせい", "effect": nil},
			map[string]any{"id": "test-thick", "nameJa": "テストぶあつい", "effect": map[string]any{
				"stabMod": 0, "offBoostType": "", "offBoostTypeMod": 0,
				"defResistType": map[string]any{"fire": 2048}, "reduceSuperEffective": 0, "ignoresBurn": false,
			}},
		},
		"natures": []any{
			// ファイルの並びと ID の昇順をわざとずらす(代表の選び方がファイル順に依存しないことを見る)。
			map[string]any{"id": "test-neutral-z", "nameJa": "テストむほせいZ", "plus": nil, "minus": nil},
			map[string]any{"id": "test-def-up", "nameJa": "テストかたい", "plus": "def", "minus": "atk"},
			map[string]any{"id": "test-neutral-b", "nameJa": "テストむほせいB", "plus": nil, "minus": nil},
			map[string]any{"id": "test-atk-up", "nameJa": "テストつよい", "plus": "atk", "minus": "spa"},
		},
	}
}

func encode(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("fixture の JSON 化に失敗: %v", err)
	}
	return bytes.NewReader(b)
}

// entry は baseSnapshot の配列 kind の i 番目の要素(書き換え用)。
func entry(m map[string]any, kind string, i int) map[string]any {
	return m[kind].([]any)[i].(map[string]any)
}

func loadSharedTypeChart(t *testing.T) engine.TypeChart {
	t.Helper()
	f, err := os.Open(sharedTypeChartPath)
	if err != nil {
		t.Fatalf("相性表を開けない: %v", err)
	}
	defer f.Close()
	chart, err := LoadTypeChart(f)
	if err != nil {
		t.Fatalf("LoadTypeChart(%s) = %v, want nil", sharedTypeChartPath, err)
	}
	return chart
}

func newStore(t *testing.T, snap map[string]any) *MemoryStore {
	t.Helper()
	s, err := LoadSnapshot(encode(t, snap))
	if err != nil {
		t.Fatalf("LoadSnapshot = %v, want nil", err)
	}
	store, err := New(s, loadSharedTypeChart(t))
	if err != nil {
		t.Fatalf("New = %v, want nil", err)
	}
	return store
}

// AC-M1: 妥当なスナップショットを読み、ID で engine の型が引ける。
func TestLoadSnapshotAndLookup(t *testing.T) {
	store := newStore(t, baseSnapshot())

	sp, ok := store.Species("9002-000")
	if !ok {
		t.Fatal("Species(9002-000) が見つからない")
	}
	wantSp := engine.Species{
		Key: "9002-000", DexNo: 9002, Form: 0, NameJa: "テストガード",
		Types:     []engine.Type{engine.TypeWater, engine.TypeSteel},
		BaseStats: engine.Stats{HP: 95, Atk: 60, Def: 90, SpA: 70, SpD: 85, Spe: 50},
		Abilities: []string{"test-plain"},
	}
	if !equalJSON(sp, wantSp) {
		t.Errorf("Species = %+v, want %+v", sp, wantSp)
	}

	mv, ok := store.Move("test-wave")
	want := engine.Move{ID: "test-wave", NameJa: "テストウェーブ", Type: engine.TypeGrass, Category: engine.CategorySpecial, Power: 90}
	if !ok || mv != want {
		t.Errorf("Move = %+v, %v, want %+v, true", mv, ok, want)
	}

	orb, ok := store.Item("test-orb")
	if !ok || orb.Effect == nil || orb.Effect.DamageMod != 5324 || orb.NameJa != "テストのたま" {
		t.Errorf("Item(test-orb) = %+v, %v, want DamageMod 5324", orb, ok)
	}
	shell, ok := store.Item("test-shell")
	if !ok || shell.Effect == nil || shell.Effect.StatMods[engine.StatDef] != 6144 || shell.Effect.StatMods[engine.StatSpD] != 6144 {
		t.Errorf("Item(test-shell) = %+v, %v, want StatMods def/spd 6144", shell, ok)
	}
	plain, ok := store.Item("test-plain-item")
	if !ok || plain.Effect != nil {
		t.Errorf("Item(test-plain-item) = %+v, %v, want Effect nil(効果なし)", plain, ok)
	}

	thick, ok := store.Ability("test-thick")
	if !ok || thick.Effect == nil || thick.Effect.DefResistType[engine.TypeFire] != 2048 {
		t.Errorf("Ability(test-thick) = %+v, %v, want DefResistType fire 2048", thick, ok)
	}
	plainAb, ok := store.Ability("test-plain")
	if !ok || plainAb.Effect != nil {
		t.Errorf("Ability(test-plain) = %+v, %v, want Effect nil", plainAb, ok)
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
}

// AC-M1: 未知の ID は false(panic しない)。
func TestLookupUnknownIDs(t *testing.T) {
	store := newStore(t, baseSnapshot())
	if _, ok := store.Species("9999-000"); ok {
		t.Error("Species(未知) = true")
	}
	if _, ok := store.Move("test-nothing"); ok {
		t.Error("Move(未知) = true")
	}
	if _, ok := store.Item("test-nothing"); ok {
		t.Error("Item(未知) = true")
	}
	if _, ok := store.Ability("test-nothing"); ok {
		t.Error("Ability(未知) = true")
	}
	if _, ok := store.Nature("test-nothing"); ok {
		t.Error("Nature(未知) = true")
	}
}

// Store の値を書き換えても次の参照に漏れない(並行に呼ばれるハンドラが共有するため)。
func TestLookupReturnsCopies(t *testing.T) {
	store := newStore(t, baseSnapshot())
	sp, _ := store.Species("9002-000")
	sp.Types[0] = engine.TypeFire
	sp.Abilities[0] = "test-changed"
	again, _ := store.Species("9002-000")
	if again.Types[0] != engine.TypeWater || again.Abilities[0] != "test-plain" {
		t.Errorf("返したスライスの書き換えが Store に漏れた: %+v", again)
	}
}

// critic 指摘 O2: Item / Ability の Effect(内部の map を含む)を書き換えても Store に漏れない。
func TestLookupReturnsCopiesOfEffects(t *testing.T) {
	store := newStore(t, baseSnapshot())

	shell, _ := store.Item("test-shell")
	shell.Effect.StatMods[engine.StatDef] = 9999
	shell.Effect.DamageMod = 9999
	again, _ := store.Item("test-shell")
	if again.Effect.StatMods[engine.StatDef] != 6144 || again.Effect.DamageMod != 0 {
		t.Errorf("Item の Effect の書き換えが Store に漏れた: %+v", again.Effect)
	}

	thick, _ := store.Ability("test-thick")
	thick.Effect.DefResistType[engine.TypeFire] = 9999
	thick.Effect.StabMod = 9999
	again2, _ := store.Ability("test-thick")
	if again2.Effect.DefResistType[engine.TypeFire] != 2048 || again2.Effect.StabMod != 0 {
		t.Errorf("Ability の Effect の書き換えが Store に漏れた: %+v", again2.Effect)
	}
}

// 例のファイル(services/calc/testdata/master.example.json)は常に読めること(README の例が腐らない)。
func TestExampleSnapshotLoads(t *testing.T) {
	f, err := os.Open(exampleMasterPath)
	if err != nil {
		t.Fatalf("例のファイルを開けない: %v", err)
	}
	defer f.Close()
	s, err := LoadSnapshot(f)
	if err != nil {
		t.Fatalf("LoadSnapshot(example) = %v, want nil", err)
	}
	if _, err := New(s, loadSharedTypeChart(t)); err != nil {
		t.Fatalf("New(example) = %v, want nil", err)
	}
}

// AC-M2: スキーマ違反はロード時に ErrInvalidSnapshot。部分的なスナップショットを返さない。
func TestLoadSnapshotRejectsInvalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m map[string]any)
		raw    string // 空でなければ mutate の代わりにこの文字列を読む
	}{
		{name: "壊れた JSON", raw: `{"schemaVersion":1,`},
		{name: "JSON の後ろに余計なデータ", raw: `{"schemaVersion":1,"species":[],"moves":[],"items":[],"abilities":[],"natures":[]} {}`},
		{name: "未知のトップレベルフィールド", mutate: func(m map[string]any) { m["extra"] = true }},
		{name: "未知の種族フィールド", mutate: func(m map[string]any) { entry(m, "species", 0)["hiddenPower"] = 1 }},
		{name: "schemaVersion 欠落", mutate: func(m map[string]any) { delete(m, "schemaVersion") }},
		{name: "schemaVersion が 2", mutate: func(m map[string]any) { m["schemaVersion"] = 2 }},
		{name: "種族キーが空", mutate: func(m map[string]any) { entry(m, "species", 0)["key"] = "" }},
		{name: "種族キーの重複", mutate: func(m map[string]any) { entry(m, "species", 1)["key"] = "9001-000" }},
		{name: "技 ID の重複", mutate: func(m map[string]any) { entry(m, "moves", 1)["id"] = "test-beam" }},
		{name: "持ち物 ID の重複", mutate: func(m map[string]any) { entry(m, "items", 1)["id"] = "test-plain-item" }},
		{name: "特性 ID の重複", mutate: func(m map[string]any) { entry(m, "abilities", 1)["id"] = "test-plain" }},
		{name: "性格 ID の重複", mutate: func(m map[string]any) { entry(m, "natures", 1)["id"] = "test-neutral-z" }},
		{name: "種族の未知のタイプ", mutate: func(m map[string]any) { entry(m, "species", 0)["types"] = []any{"cosmic"} }},
		{name: "種族のタイプが0個", mutate: func(m map[string]any) { entry(m, "species", 0)["types"] = []any{} }},
		{name: "種族のタイプが3個", mutate: func(m map[string]any) {
			entry(m, "species", 0)["types"] = []any{"normal", "fire", "water"}
		}},
		{name: "種族値のステータスキーが未知", mutate: func(m map[string]any) {
			entry(m, "species", 0)["baseStats"].(map[string]any)["luck"] = 1
		}},
		{name: "技の未知のタイプ", mutate: func(m map[string]any) { entry(m, "moves", 0)["type"] = "cosmic" }},
		{name: "技の未知の分類", mutate: func(m map[string]any) { entry(m, "moves", 0)["category"] = "magic" }},
		{name: "技の分類が空", mutate: func(m map[string]any) { entry(m, "moves", 0)["category"] = "" }},
		{name: "持ち物効果の未知のステータスキー", mutate: func(m map[string]any) {
			entry(m, "items", 2)["effect"].(map[string]any)["statMods"] = map[string]any{"luck": 6144}
		}},
		{name: "持ち物効果の未知の強化タイプ", mutate: func(m map[string]any) {
			entry(m, "items", 1)["effect"].(map[string]any)["boostType"] = "cosmic"
		}},
		{name: "持ち物効果の未知の分類", mutate: func(m map[string]any) {
			entry(m, "items", 1)["effect"].(map[string]any)["powerCategory"] = "magic"
		}},
		{name: "特性効果の未知の半減タイプ", mutate: func(m map[string]any) {
			entry(m, "abilities", 1)["effect"].(map[string]any)["defResistType"] = map[string]any{"cosmic": 2048}
		}},
		{name: "性格の上昇が HP", mutate: func(m map[string]any) { entry(m, "natures", 1)["plus"] = "hp" }},
		{name: "性格の下降が HP", mutate: func(m map[string]any) { entry(m, "natures", 1)["minus"] = "hp" }},
		{name: "性格の未知のステータスキー", mutate: func(m map[string]any) { entry(m, "natures", 1)["plus"] = "luck" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r *bytes.Reader
			if tt.raw != "" {
				r = bytes.NewReader([]byte(tt.raw))
			} else {
				m := baseSnapshot()
				tt.mutate(m)
				r = encode(t, m)
			}
			s, err := LoadSnapshot(r)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("LoadSnapshot err = %v, want ErrInvalidSnapshot", err)
			}
			if s != nil {
				t.Errorf("失敗時に部分的なスナップショットを返した: %+v", s)
			}
		})
	}
}

// AC-M3: 共有の相性表(testdata/golden/typechart.json)を読める。代表的な組を固定する。
func TestLoadTypeChartShared(t *testing.T) {
	chart := loadSharedTypeChart(t)
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
		{engine.TypeNormal, engine.TypeNormal, engine.TypeCodeNeutral},
	}
	for _, c := range cases {
		got, err := chart.Code(c.atk, c.def)
		if err != nil || got != c.want {
			t.Errorf("Code(%s, %s) = %d, %v, want %d", c.atk, c.def, got, err, c.want)
		}
	}
}

// AC-M3: 相性表の schema 違反・表として不正なものは ErrInvalidTypeChart(静かに等倍にしない)。
func TestLoadTypeChartRejectsInvalid(t *testing.T) {
	const head = `{"schemaVersion":1,"source":"test","version":"0","generation":0,"note":"","excludedTypes":[],`
	tests := []struct{ name, raw string }{
		{"壊れた JSON", `{"schemaVersion":1,`},
		{"JSON の後ろに余計なデータ", head + `"types":["normal"],"effectiveness":{"normal":{"normal":2}}} {}`},
		{"未知のフィールド", head + `"types":["normal"],"effectiveness":{"normal":{"normal":2}},"extra":1}`},
		{"schemaVersion 欠落", `{"types":["normal"],"effectiveness":{"normal":{"normal":2}}}`},
		{"schemaVersion が 2", `{"schemaVersion":2,"types":["normal"],"effectiveness":{"normal":{"normal":2}}}`},
		{"types が空", head + `"types":[],"effectiveness":{"normal":{"normal":2}}}`},
		{"未知のタイプ ID", head + `"types":["normal","cosmic"],"effectiveness":{"normal":{"normal":2}}}`},
		{"タイプの重複", head + `"types":["normal","normal"],"effectiveness":{"normal":{"normal":2}}}`},
		{"不正なコード 3", head + `"types":["normal","ghost"],"effectiveness":{"normal":{"ghost":3}}}`},
		{"effectiveness のキーが types に無い", head + `"types":["normal"],"effectiveness":{"fire":{"normal":2}}}`},
		{"effectiveness が欠落", head + `"types":["normal"]}`},
		{"effectiveness が空", head + `"types":["normal"],"effectiveness":{}}`},
		{"null 行", head + `"types":["normal"],"effectiveness":{"normal":null}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chart, err := LoadTypeChart(strings.NewReader(tt.raw))
			if !errors.Is(err, ErrInvalidTypeChart) {
				t.Fatalf("LoadTypeChart err = %v, want ErrInvalidTypeChart", err)
			}
			if !chart.IsZero() {
				t.Error("失敗時にゼロ値でない表を返した")
			}
		})
	}
}

// AC-M4: New は相性表のゼロ値と、相性表に無いタイプを起動時に拒否する。
func TestNewRejectsInconsistentMaster(t *testing.T) {
	s, err := LoadSnapshot(encode(t, baseSnapshot()))
	if err != nil {
		t.Fatalf("LoadSnapshot = %v", err)
	}
	if _, err := New(s, engine.TypeChart{}); !errors.Is(err, ErrInvalidTypeChart) {
		t.Errorf("New(ゼロ値の表) err = %v, want ErrInvalidTypeChart", err)
	}

	// normal だけの表: 種族 9002-000 の water/steel、技 test-wave の grass が表に無い。
	small, err := LoadTypeChart(strings.NewReader(
		`{"schemaVersion":1,"source":"test","version":"0","generation":0,"note":"","excludedTypes":[],` +
			`"types":["normal"],"effectiveness":{"normal":{"normal":2}}}`))
	if err != nil {
		t.Fatalf("LoadTypeChart(小さい表) = %v", err)
	}
	if _, err := New(s, small); !errors.Is(err, engine.ErrUnknownType) {
		t.Errorf("New(タイプが欠けた表) err = %v, want engine.ErrUnknownType", err)
	}
}

// AC-M5: NatureID の写像規則(ADR-0018)。
func TestNatureID(t *testing.T) {
	m := baseSnapshot()
	// plus == minus は engine の IsNeutral で無補正。ID 昇順で最も小さいので、無補正の代表になる。
	m["natures"] = append(m["natures"].([]any),
		map[string]any{"id": "test-neutral-a-same", "nameJa": "テストおなじ", "plus": "spe", "minus": "spe"})
	store := newStore(t, m)

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
	m := baseSnapshot()
	m["natures"] = []any{map[string]any{"id": "test-def-up", "nameJa": "テストかたい", "plus": "def", "minus": "atk"}}
	store := newStore(t, m)
	if id, ok := store.NatureID(engine.NatureNeutral); ok || id != "" {
		t.Errorf("NatureID(無補正) = %q, %v, want \"\", false", id, ok)
	}
}

func equalJSON(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}
