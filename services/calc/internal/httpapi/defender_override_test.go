package httpapi

// issue #274/#272 の残り(ADR-0216): BulkCalcRequest.defenderOverride.ranks / status を
// engine.BulkInput.DefenderOverride に写し、全行の防御側に一律で当てることを固定する。
//
//   - 省略・空オブジェクト・ゼロ値は応答がバイト単位で従来と同じ
//   - 指定すると engine.CalcBulk(DefenderOverride 付き)と同じ行になる(abilityId とも独立に併用できる)
//   - ranks の -6..+6 外は 400 invalid_input、未知の status は 400 invalid_enum(individual.status と同じ)、
//     ranks.hp 等の契約外キーは 400 unknown_field、整数でない値は 400 invalid_json
//   - 件数上限(ADR-0208)の検査が上書きの検証より先
//   - 逆算(ReverseRequest)には足さない(ADR-0216 §4)

import (
	"bytes"
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

func bulkBodyWithOverride(moveID string, presets any, override any) map[string]any {
	body := bulkBody(moveID, presets, nil)
	if override != nil {
		body["defenderOverride"] = override
	}
	return body
}

// engineBulkWithOverride は calc-svc が engine に渡すはずの入力(既定の特性候補つき)を直接作って計算する。
func engineBulkWithOverride(t *testing.T, f *fakeStore, moveID string, keys []engine.PresetKey, items []*engine.Item,
	speciesKey string, abilities []engine.Ability, ov engine.DefenderOverride) engine.BulkResult {
	t.Helper()
	res, err := engine.CalcBulk(engine.BulkInput{
		Format:            engine.FormatSingle,
		Attacker:          bulkAttacker().engine(t, f),
		DefenderSpecies:   f.species[speciesKey],
		Move:              f.moves[moveID],
		Field:             engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone},
		TypeChart:         f.chart,
		PresetKeys:        keys,
		ItemVariants:      items,
		DefenderAbilities: abilities,
		DefenderOverride:  ov,
	})
	if err != nil {
		t.Fatalf("engine.CalcBulk = %v", err)
	}
	return res
}

func TestCalcBulkDefenderOverrideZeroValueIsByteIdentical(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	legacy := post(t, h, "/api/calc/bulk", mustJSON(t, bulkBody(movePhysical, nil, nil)), true)
	if legacy.Code != http.StatusOK {
		t.Fatalf("前提: 上書きなしが 200 でない: %d %s", legacy.Code, legacy.Body.String())
	}
	for name, ov := range map[string]any{
		"空オブジェクト":         map[string]any{},
		"ranks 空":         map[string]any{"ranks": map[string]any{}},
		"ranks 全0":        map[string]any{"ranks": ranksMap(engine.Ranks{})},
		"status none":     map[string]any{"status": "none"},
		"ranks 全0 と none": map[string]any{"ranks": ranksMap(engine.Ranks{}), "status": "none"},
	} {
		rec := post(t, h, "/api/calc/bulk", mustJSON(t, bulkBodyWithOverride(movePhysical, nil, ov)), true)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), legacy.Body.Bytes()) {
			t.Errorf("%s: 応答が従来と違う status=%d\n got=%s\nwant=%s", name, rec.Code, rec.Body.String(), legacy.Body.String())
		}
	}
}

func TestCalcBulkDefenderOverrideMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	plain := []engine.Ability{store.abilities[abilityPlain]} // speciesDefender の特性(既定の候補)
	shell := store.items[itemShell]
	tests := []struct {
		name    string
		moveID  string
		presets []any
		keys    []engine.PresetKey
		ranks   engine.Ranks
		status  engine.Status
	}{
		{"物理・防御+2・まひ", movePhysical, nil, nil, engine.Ranks{Def: 2}, engine.StatusParalysis},
		{"物理・防御-6(境界)", movePhysical, []any{"none", "hb_full"}, []engine.PresetKey{engine.PresetNone, engine.PresetHBFull}, engine.Ranks{Def: -6}, ""},
		{"特殊・特防+6(境界)・やけど", moveSpecial, nil, nil, engine.Ranks{SpD: 6}, engine.StatusBurn},
		{"特殊・全ランク", moveSpecial, nil, nil, engine.Ranks{Atk: 1, Def: -1, SpA: 2, SpD: -2, Spe: 3}, engine.StatusSleep},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ov := map[string]any{"ranks": ranksMap(tt.ranks)}
			if tt.status != "" {
				ov["status"] = string(tt.status)
			}
			body := bulkBodyWithOverride(tt.moveID, nil, ov)
			if tt.presets != nil {
				body["presets"] = tt.presets
			}
			body["itemVariants"] = []any{nil, itemShell}
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
			var got api.BulkCalcResult
			decodeInto(t, rec, &got)
			want := engineBulkWithOverride(t, store, tt.moveID, tt.keys, []*engine.Item{nil, &shell}, speciesDefender, plain,
				engine.DefenderOverride{Ranks: tt.ranks, Status: tt.status})
			assertBulkMatchesEngine(t, store, got, want)

			// 上書きが実際に効いている(防御/特防のランクが 0 でなければ、上書きなしと結果が違う)。
			base := engineBulkWithOverride(t, store, tt.moveID, tt.keys, []*engine.Item{nil, &shell}, speciesDefender, plain, engine.DefenderOverride{})
			if tt.ranks.Def != 0 && tt.moveID == movePhysical || tt.ranks.SpD != 0 && tt.moveID == moveSpecial {
				if got.Rows[0].Result.MaxDamage == base.Rows[0].Result.MaxDamage() {
					t.Errorf("ランクの上書きが結果に効いていない: maxDamage=%d", got.Rows[0].Result.MaxDamage)
				}
			}
		})
	}
}

// abilityId(ADR-0214)と ranks・status は独立に併用でき、全行に両方が当たる。
func TestCalcBulkDefenderOverrideComposesWithAbilityID(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)
	body := map[string]any{
		"format":             "single",
		"attacker":           indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesTwoAbilities,
		"moveId":             movePhysical,
		"defenderOverride": map[string]any{
			"abilityId": abilityResistNormal,
			"ranks":     map[string]any{"def": 2},
			"status":    "poison",
		},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
	var got api.BulkCalcResult
	decodeInto(t, rec, &got)
	res, err := engine.CalcBulk(engine.BulkInput{
		Format:            engine.FormatSingle,
		Attacker:          indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.engine(t, store),
		DefenderSpecies:   store.species[speciesTwoAbilities],
		Move:              store.moves[movePhysical],
		Field:             engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone},
		TypeChart:         store.chart,
		DefenderAbilities: []engine.Ability{store.abilities[abilityResistNormal]},
		DefenderOverride:  engine.DefenderOverride{Ranks: engine.Ranks{Def: 2}, Status: engine.StatusPoison},
	})
	if err != nil {
		t.Fatalf("engine.CalcBulk = %v", err)
	}
	assertBulkMatchesEngine(t, store, got, res)
	for i, row := range got.Rows {
		if row.AbilityId != abilityResistNormal {
			t.Errorf("rows[%d].abilityId = %q, want %q", i, row.AbilityId, abilityResistNormal)
		}
	}
}

func TestCalcBulkDefenderOverrideErrors(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	tests := []struct {
		name     string
		override any
		wantCode string
	}{
		{"防御+7", map[string]any{"ranks": map[string]any{"def": 7}}, string(api.InvalidInput)},
		{"特防-7", map[string]any{"ranks": map[string]any{"spd": -7}}, string(api.InvalidInput)},
		{"攻撃+7(使わない側でも拒否)", map[string]any{"ranks": map[string]any{"atk": 7}}, string(api.InvalidInput)},
		{"素早さ-7", map[string]any{"ranks": map[string]any{"spe": -7}}, string(api.InvalidInput)},
		{"未知の状態異常", map[string]any{"status": "confusion"}, string(api.InvalidEnum)},
		{"大文字の状態異常", map[string]any{"status": "BURN"}, string(api.InvalidEnum)},
		{"ranks.hp は契約に無い", map[string]any{"ranks": map[string]any{"hp": 1}}, string(api.UnknownField)},
		{"defenderOverride に未知のキー", map[string]any{"teraType": "fire"}, string(api.UnknownField)},
		{"ランクが小数", map[string]any{"ranks": map[string]any{"def": 1.5}}, string(api.InvalidJson)},
		{"ランクが文字列", map[string]any{"ranks": map[string]any{"def": "+1"}}, string(api.InvalidJson)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, bulkBodyWithOverride(movePhysical, nil, tt.override)), false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

// 件数上限(ADR-0208)の検査は上書きの検証より先(engine・WASM と同じ順)。
func TestCalcBulkLimitsCheckedBeforeDefenderOverride(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	presets := []any{"none", "hp", "hb_boost", "hb", "hb_full", "hd_boost", "hd", "hd_full", "none"}
	body := bulkBodyWithOverride(movePhysical, presets, map[string]any{"status": "confusion", "ranks": map[string]any{"def": 9}})
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), false)
	assertError(t, rec, http.StatusBadRequest, string(api.InvalidInput))
	if e := decodeErrorBody(t, rec); !bytes.Contains([]byte(e.Message), []byte("presets")) {
		t.Errorf("件数上限より先に上書きの検証が走った: %q", e.Message)
	}
}

// 逆算には defenderOverride を足さない(ADR-0216 §4)。既知の側のランク・状態は known.ranks / known.status で渡す。
func TestCalcReverseDoesNotAcceptDefenderOverride(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	body := reverseCases(t, store)[0].httpBody()
	body["defenderOverride"] = map[string]any{"ranks": map[string]any{"def": 1}}
	rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), false)
	assertError(t, rec, http.StatusBadRequest, string(api.UnknownField))
}
