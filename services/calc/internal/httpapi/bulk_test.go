package httpapi

// POST /api/calc/bulk の受け入れテスト(ADR-0016 AC-3、ADR-0009)。
// 期待値は同じ入力を engine.CalcBulk に直接渡した結果と照合する。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// bulkAttacker は一括計算で固定する攻撃側。
func bulkAttacker() indiv {
	return indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, abilityID: abilityPlain, sp: engine.Stats{Atk: 32, Spe: 32}}
}

func bulkBody(moveID string, presets any, itemVariants any) map[string]any {
	body := map[string]any{
		"format":             "single",
		"attacker":           bulkAttacker().http(),
		"defenderSpeciesKey": speciesDefender,
		"moveId":             moveID,
	}
	if presets != nil {
		body["presets"] = presets
	}
	if itemVariants != nil {
		body["itemVariants"] = itemVariants
	}
	return body
}

func engineBulk(t *testing.T, f *fakeStore, moveID string, keys []engine.PresetKey, items []*engine.Item) engine.BulkResult {
	t.Helper()
	res, err := engine.CalcBulk(engine.BulkInput{
		Format:          engine.FormatSingle,
		Attacker:        bulkAttacker().engine(t, f),
		DefenderSpecies: f.species[speciesDefender],
		Move:            f.moves[moveID],
		Field:           engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone},
		TypeChart:       f.chart,
		PresetKeys:      keys,
		ItemVariants:    items,
	})
	if err != nil {
		t.Fatalf("engine.CalcBulk = %v", err)
	}
	return res
}

func statBlock(s engine.Stats) api.StatBlock {
	return api.StatBlock{Hp: s.HP, Atk: s.Atk, Def: s.Def, Spa: s.SpA, Spd: s.SpD, Spe: s.Spe}
}

func statKeyPtr(k engine.StatKey) *api.StatKey {
	if k == "" {
		return nil
	}
	v := api.StatKey(k)
	return &v
}

func equalStatKeyPtr(a, b *api.StatKey) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func strPtrString(p *string) string {
	if p == nil {
		return "<null>"
	}
	return *p
}

// assertBulkMatchesEngine は行の数・順序・各行の中身が engine の結果の写しであることを確かめる。
func assertBulkMatchesEngine(t *testing.T, f *fakeStore, got api.BulkCalcResult, want engine.BulkResult) {
	t.Helper()
	if got.DefenderSpeciesKey != want.DefenderSpeciesKey {
		t.Errorf("defenderSpeciesKey = %q, want %q", got.DefenderSpeciesKey, want.DefenderSpeciesKey)
	}
	if len(got.Rows) != len(want.Rows) {
		t.Fatalf("行数 = %d, want %d", len(got.Rows), len(want.Rows))
	}
	for i, w := range want.Rows {
		g := got.Rows[i]
		if string(g.Preset) != string(w.Preset) || g.PresetLabel != w.PresetLabel {
			t.Errorf("rows[%d] preset = %q %q, want %q %q", i, g.Preset, g.PresetLabel, w.Preset, w.PresetLabel)
		}
		// 持ち物なしは null(engine の空文字を契約の nullable に写す)。
		switch {
		case w.ItemID == "" && g.ItemId != nil:
			t.Errorf("rows[%d] itemId = %q, want null", i, *g.ItemId)
		case w.ItemID != "" && (g.ItemId == nil || *g.ItemId != w.ItemID):
			t.Errorf("rows[%d] itemId = %s, want %q", i, strPtrString(g.ItemId), w.ItemID)
		}
		if g.Defender.Sp != statBlock(w.Defender.SP) {
			t.Errorf("rows[%d] defender.sp = %+v, want %+v", i, g.Defender.Sp, statBlock(w.Defender.SP))
		}
		if g.Defender.Stats != statBlock(engine.RealStats(w.Defender)) {
			t.Errorf("rows[%d] defender.stats = %+v, want %+v(engine.RealStats)", i, g.Defender.Stats, statBlock(engine.RealStats(w.Defender)))
		}
		if !equalStatKeyPtr(g.Defender.Nature.Plus, statKeyPtr(w.Defender.Nature.Plus)) ||
			!equalStatKeyPtr(g.Defender.Nature.Minus, statKeyPtr(w.Defender.Nature.Minus)) {
			t.Errorf("rows[%d] defender.nature = %+v, want %+v", i, g.Defender.Nature, w.Defender.Nature)
		}
		wantID, ok := f.NatureID(w.Defender.Nature)
		switch {
		case !ok && g.Defender.NatureId != nil:
			t.Errorf("rows[%d] defender.natureId = %q, want null(マスタに該当なし)", i, *g.Defender.NatureId)
		case ok && (g.Defender.NatureId == nil || *g.Defender.NatureId != wantID):
			t.Errorf("rows[%d] defender.natureId = %s, want %q", i, strPtrString(g.Defender.NatureId), wantID)
		}
		assertCalcResultMatchesEngine(t, g.Result, w.Result)
	}
}

func presetKeysOf(rows []api.BulkCalcRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, string(r.Preset))
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AC-3: presets の省略と [] はどちらも技の分類に応じた既定セット。変化技は none/hp の2件のみ。
func TestCalcBulkDefaultPresets(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	tests := []struct {
		name     string
		moveID   string
		presets  any
		wantKeys []string
	}{
		{"物理・省略", movePhysical, nil, []string{"none", "hp", "hb_boost", "hb", "hb_full"}},
		{"物理・空配列", movePhysical, []any{}, []string{"none", "hp", "hb_boost", "hb", "hb_full"}},
		{"特殊・省略", moveSpecial, nil, []string{"none", "hp", "hd_boost", "hd", "hd_full"}},
		{"特殊・空配列", moveSpecial, []any{}, []string{"none", "hp", "hd_boost", "hd", "hd_full"}},
		{"変化技・省略は none/hp の2件のみ", moveStatus, nil, []string{"none", "hp"}},
		{"変化技・空配列は none/hp の2件のみ", moveStatus, []any{}, []string{"none", "hp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, bulkBody(tt.moveID, tt.presets, nil)), true)
			var got api.BulkCalcResult
			decodeInto(t, rec, &got)
			if keys := presetKeysOf(got.Rows); !equalStrings(keys, tt.wantKeys) {
				t.Fatalf("行の preset = %v, want %v", keys, tt.wantKeys)
			}
			assertBulkMatchesEngine(t, store, got, engineBulk(t, store, tt.moveID, nil, nil))
		})
	}
}

// AC-3: presets 指定時はその順、行はプリセット優先(presets × itemVariants)。itemVariants の null は持ち物なし。
func TestCalcBulkRowOrderIsPresetMajor(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	body := bulkBody(movePhysical, []any{"hb_full", "none", "hd"}, []any{nil, itemShell, itemOrb})
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
	var got api.BulkCalcResult
	decodeInto(t, rec, &got)

	shell, orb := store.items[itemShell], store.items[itemOrb]
	want := engineBulk(t, store, movePhysical,
		[]engine.PresetKey{engine.PresetHBFull, engine.PresetNone, engine.PresetHD},
		[]*engine.Item{nil, &shell, &orb})
	wantOrder := []string{"hb_full", "hb_full", "hb_full", "none", "none", "none", "hd", "hd", "hd"}
	if keys := presetKeysOf(got.Rows); !equalStrings(keys, wantOrder) {
		t.Fatalf("行の preset = %v, want %v(preset-major)", keys, wantOrder)
	}
	assertBulkMatchesEngine(t, store, got, want)
}

// AC-3: 性格 ID の写像。無補正 → 代表 ID、+B/-A → 一致する ID、マスタに無い +D/-A → null。
func TestCalcBulkNatureIDMapping(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	body := bulkBody(moveSpecial, []any{"none", "hb_boost", "hd_boost"}, nil)
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
	var got api.BulkCalcResult
	decodeInto(t, rec, &got)
	want := []*string{ptr(natureNeutral), ptr(natureDefUp), nil}
	if len(got.Rows) != len(want) {
		t.Fatalf("行数 = %d, want %d", len(got.Rows), len(want))
	}
	for i, w := range want {
		g := got.Rows[i].Defender.NatureId
		if (w == nil) != (g == nil) || (w != nil && *w != *g) {
			t.Errorf("rows[%d](%s) natureId = %s, want %s", i, got.Rows[i].Preset, strPtrString(g), strPtrString(w))
		}
	}
}

// AC-3: 不正な presets / itemVariants / ID はエラー(部分成功にしない。ADR-0009 §5)。
func TestCalcBulkErrors(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{"重複した preset", bulkBody(movePhysical, []any{"hp", "none", "hp"}, nil), http.StatusBadRequest, "duplicate_preset"},
		// presets の未知の値は WASM の presetKeys と同じく engine の sentinel に一本化する(ADR-0016)。
		{"未知の preset", bulkBody(movePhysical, []any{"hx"}, nil), http.StatusBadRequest, "unknown_preset"},
		{"未知の持ち物(itemVariants)", bulkBody(movePhysical, nil, []any{nil, "test-nothing"}), http.StatusBadRequest, "unknown_item"},
		{"未知の防御側種族", func() map[string]any {
			b := bulkBody(movePhysical, nil, nil)
			b["defenderSpeciesKey"] = speciesUnknown
			return b
		}(), http.StatusBadRequest, "unknown_species"},
		{"未知の技", bulkBody("test-nothing", nil, nil), http.StatusBadRequest, "unknown_move"},
		{"攻撃側の SP 合計 67", func() map[string]any {
			b := bulkBody(movePhysical, nil, nil)
			b["attacker"].(map[string]any)["sp"] = statsMap(engine.Stats{Atk: 32, Def: 32, Spe: 3})
			return b
		}(), http.StatusBadRequest, "invalid_input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, tt.body), false)
			assertError(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}

func ptr[T any](v T) *T { return &v }
