package httpapi

// issue 272(ADR-0126・ADR-0214)の API レーン担当分: defenderOverride.abilityId / unknownAbilityId が
// engine の特性候補(BulkInput.DefenderAbilities・ReverseInput.UnknownAbilities)に正しく写ること、
// 省略時に種族の特性(最大3件。4件目は落とす)を解決して渡すことを固定する。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
)

const (
	speciesFourAbilities  = "9099-000" // 特性を4つ持つ架空種族(スロット1〜3は無効果、4件目だけ防御効果を持つ)
	speciesSingleNoEffect = "9097-000" // 特性を1つだけ持つ架空種族(無効果。対照用)
	speciesSingleResist   = "9098-000" // 特性を1つだけ持つ架空種族(ノーマル技半減。既定でも必ず効く)
	speciesTwoAbilities   = "9096-000" // 特性を2つ持つ架空種族(1件目は無効果、2件目はノーマル技半減)
	abilityNoEffectA      = "test-ability-none-a"
	abilityNoEffectB      = "test-ability-none-b"
	abilityNoEffectC      = "test-ability-none-c"
	abilityResistNormal   = "test-ability-resist-normal" // ノーマル技半減(スロット4。既定では使われない)
	abilityElsewhere      = "test-ability-elsewhere"     // 別の種族の特性(speciesFourAbilities は持たない)
)

// newStoreWithFourAbilitySpecies は特性を4つ持つ架空種族を追加した fakeStore を作る。
// 4件目(abilityResistNormal)だけがノーマル技を半減する(DefResistType)。1〜3件目は無効果で
// 結果が完全に同じになるため、既定(上書き無し)では1行/1候補にまとまり、4件目は既定では使われない
// (ADR-0105 §5 と同じ「スロット4を落とす」判断。ADR-0126・ADR-0214)。
func newStoreWithFourAbilitySpecies(t *testing.T) *fakeStore {
	t.Helper()
	store := newFakeStore(t)
	store.abilities[abilityNoEffectA] = engine.Ability{ID: abilityNoEffectA, NameJa: "テストとくせいA"}
	store.abilities[abilityNoEffectB] = engine.Ability{ID: abilityNoEffectB, NameJa: "テストとくせいB"}
	store.abilities[abilityNoEffectC] = engine.Ability{ID: abilityNoEffectC, NameJa: "テストとくせいC"}
	store.abilities[abilityResistNormal] = engine.Ability{
		ID: abilityResistNormal, NameJa: "テストはんげん",
		Effect: &engine.AbilityEffect{DefResistType: map[engine.Type]int{engine.TypeNormal: 2048}},
	}
	store.abilities[abilityElsewhere] = engine.Ability{ID: abilityElsewhere, NameJa: "テストよそのとくせい"}
	store.species[speciesFourAbilities] = engine.Species{
		Key: speciesFourAbilities, DexNo: 9099, NameJa: "テストよんとくせい",
		Types:     []engine.Type{engine.TypeWater},
		BaseStats: engine.Stats{HP: 100, Atk: 80, Def: 80, SpA: 80, SpD: 80, Spe: 80},
		Abilities: []string{abilityNoEffectA, abilityNoEffectB, abilityNoEffectC, abilityResistNormal},
	}
	// speciesSingleNoEffect と speciesSingleResist は同じ種族値・タイプで特性だけが違う対照ペア。
	// 特性を1件だけ持つので既定(上書き無し)でも必ずその1件が使われ、Effect が実際に計算へ
	// 届くこと(critic 指摘。省略時に解決した特性の効果が engine まで伝わることの確認)を固定できる。
	baseStats := engine.Stats{HP: 100, Atk: 80, Def: 80, SpA: 80, SpD: 80, Spe: 80}
	store.species[speciesSingleNoEffect] = engine.Species{
		Key: speciesSingleNoEffect, DexNo: 9097, NameJa: "テストむこうかたいしょう",
		Types: []engine.Type{engine.TypeWater}, BaseStats: baseStats, Abilities: []string{abilityNoEffectA},
	}
	store.species[speciesSingleResist] = engine.Species{
		Key: speciesSingleResist, DexNo: 9098, NameJa: "テストはんげんたいしょう",
		Types: []engine.Type{engine.TypeWater}, BaseStats: baseStats, Abilities: []string{abilityResistNormal},
	}
	// speciesTwoAbilities は2件のうち1件だけ効果を持つ。既定で行/候補が2つに分かれることを固定する。
	store.species[speciesTwoAbilities] = engine.Species{
		Key: speciesTwoAbilities, DexNo: 9096, NameJa: "テストにとくせい",
		Types: []engine.Type{engine.TypeWater}, BaseStats: baseStats,
		Abilities: []string{abilityNoEffectA, abilityResistNormal},
	}
	return store
}

// TestCalcBulkDefaultsToAllSpeciesAbilitiesTruncatedToThree: 上書き無しでは種族の特性(最大3件。
// 4件目は落とす)を候補にする。1〜3件目は無効果で結果が同じなので1行にまとまり、abilityIds に3件とも
// 入るが、4件目(ノーマル半減)は使われないのでダメージは半減されない。
func TestCalcBulkDefaultsToAllSpeciesAbilitiesTruncatedToThree(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	body := map[string]any{
		"format":             "single",
		"attacker":           indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesFourAbilities,
		"moveId":             movePhysical, // test-beam: normal タイプ
		"presets":            []any{"none"},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)

	var got struct {
		Rows []struct {
			AbilityId  string   `json:"abilityId"`
			AbilityIds []string `json:"abilityIds"`
			Result     struct {
				MinDamage int `json:"minDamage"`
			} `json:"result"`
		} `json:"rows"`
	}
	decodeInto(t, rec, &got)

	if len(got.Rows) != 1 {
		t.Fatalf("rows の件数 = %d, want 1(1〜3件目は無効果で結果が同じなので1行にまとまる): %+v", len(got.Rows), got.Rows)
	}
	row := got.Rows[0]
	if row.AbilityId != abilityNoEffectA {
		t.Errorf("abilityId = %q, want %q(先頭が代表)", row.AbilityId, abilityNoEffectA)
	}
	wantIDs := []string{abilityNoEffectA, abilityNoEffectB, abilityNoEffectC}
	if len(row.AbilityIds) != len(wantIDs) {
		t.Fatalf("abilityIds = %v, want %v(4件目〈%s〉は既定では使わない)", row.AbilityIds, wantIDs, abilityResistNormal)
	}
	for i, id := range wantIDs {
		if row.AbilityIds[i] != id {
			t.Errorf("abilityIds[%d] = %q, want %q", i, row.AbilityIds[i], id)
		}
	}

	// 参考値: 4件目(半減)を明示指定した場合と比べ、既定のダメージの方が大きい(半減されていない)ことを確認する。
	overrideBody := map[string]any{
		"format": "single", "attacker": indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesFourAbilities, "moveId": movePhysical, "presets": []any{"none"},
		"defenderOverride": map[string]any{"abilityId": abilityResistNormal},
	}
	overrideRec := post(t, h, "/api/calc/bulk", mustJSON(t, overrideBody), true)
	var overrideGot struct {
		Rows []struct {
			Result struct {
				MinDamage int `json:"minDamage"`
			} `json:"result"`
		} `json:"rows"`
	}
	decodeInto(t, overrideRec, &overrideGot)
	if len(overrideGot.Rows) != 1 {
		t.Fatalf("override 側の rows = %d, want 1", len(overrideGot.Rows))
	}
	if overrideGot.Rows[0].Result.MinDamage >= row.Result.MinDamage {
		t.Errorf("4件目(半減)を明示指定したダメージ %d が既定のダメージ %d 以上(半減が効いていない)",
			overrideGot.Rows[0].Result.MinDamage, row.Result.MinDamage)
	}
}

// TestCalcBulkDefenderOverrideAbilityId: defenderOverride.abilityId を指定すると、その1件だけを
// 候補にする(1行・abilityIds はその1件だけ)。
func TestCalcBulkDefenderOverrideAbilityId(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	body := map[string]any{
		"format": "single", "attacker": indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesFourAbilities, "moveId": movePhysical, "presets": []any{"none"},
		"defenderOverride": map[string]any{"abilityId": abilityResistNormal},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)

	var got struct {
		Rows []struct {
			AbilityId  string   `json:"abilityId"`
			AbilityIds []string `json:"abilityIds"`
		} `json:"rows"`
	}
	decodeInto(t, rec, &got)
	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(got.Rows))
	}
	if got.Rows[0].AbilityId != abilityResistNormal {
		t.Errorf("abilityId = %q, want %q", got.Rows[0].AbilityId, abilityResistNormal)
	}
	if len(got.Rows[0].AbilityIds) != 1 || got.Rows[0].AbilityIds[0] != abilityResistNormal {
		t.Errorf("abilityIds = %v, want [%q]", got.Rows[0].AbilityIds, abilityResistNormal)
	}
}

// TestCalcBulkDefenderOverrideAbilityNotOnSpecies: 種族が持たない特性を指定すると 400 invalid_input。
func TestCalcBulkDefenderOverrideAbilityNotOnSpecies(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	body := map[string]any{
		"format": "single", "attacker": indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesFourAbilities, "moveId": movePhysical,
		"defenderOverride": map[string]any{"abilityId": abilityElsewhere},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), false)
	assertError(t, rec, 400, "invalid_input")
}

// TestCalcBulkDefenderOverrideUnknownAbility: マスタに無い特性IDを指定すると 400 unknown_ability。
func TestCalcBulkDefenderOverrideUnknownAbility(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	body := map[string]any{
		"format": "single", "attacker": indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesFourAbilities, "moveId": movePhysical,
		"defenderOverride": map[string]any{"abilityId": "test-ability-does-not-exist"},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), false)
	assertError(t, rec, 400, "unknown_ability")
}

// TestCalcReverseUnknownAbilityId: 逆算でも同じ既定・上書き・エラーの規則が働く(相手の特性候補)。
func TestCalcReverseUnknownAbilityId(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	base := map[string]any{
		"format": "single", "side": "defender",
		"known":             indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"unknownSpeciesKey": speciesFourAbilities,
		"moveId":            movePhysical,
		"observations":      []map[string]any{{"percent": 20}},
	}

	t.Run("既定は種族の特性(最大3件。4件目は落とす)", func(t *testing.T) {
		rec := post(t, h, "/api/calc/reverse", mustJSON(t, base), true)
		var got struct {
			Candidates []struct {
				AbilityId  string   `json:"abilityId"`
				AbilityIds []string `json:"abilityIds"`
			} `json:"candidates"`
		}
		decodeInto(t, rec, &got)
		if len(got.Candidates) == 0 {
			t.Fatal("candidates が空")
		}
		wantIDs := []string{abilityNoEffectA, abilityNoEffectB, abilityNoEffectC}
		for i, c := range got.Candidates {
			if len(c.AbilityIds) != len(wantIDs) {
				t.Errorf("candidates[%d].abilityIds = %v, want %v", i, c.AbilityIds, wantIDs)
			}
		}
	})

	t.Run("unknownAbilityId で1件に固定", func(t *testing.T) {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body["unknownAbilityId"] = abilityResistNormal
		rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), true)
		var got struct {
			Candidates []struct {
				AbilityId  string   `json:"abilityId"`
				AbilityIds []string `json:"abilityIds"`
			} `json:"candidates"`
		}
		decodeInto(t, rec, &got)
		if len(got.Candidates) == 0 {
			t.Fatal("candidates が空")
		}
		for i, c := range got.Candidates {
			if c.AbilityId != abilityResistNormal || len(c.AbilityIds) != 1 || c.AbilityIds[0] != abilityResistNormal {
				t.Errorf("candidates[%d] abilityId=%q abilityIds=%v, want %q/[%q]", i, c.AbilityId, c.AbilityIds, abilityResistNormal, abilityResistNormal)
			}
		}
	})

	t.Run("種族が持たない特性は invalid_input", func(t *testing.T) {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body["unknownAbilityId"] = abilityElsewhere
		rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), false)
		assertError(t, rec, 400, "invalid_input")
	})

	t.Run("マスタに無い特性は unknown_ability", func(t *testing.T) {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body["unknownAbilityId"] = "test-ability-does-not-exist"
		rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), false)
		assertError(t, rec, 400, "unknown_ability")
	})
}

// bulkMinDamage は POST /api/calc/bulk(常に1プリセット "none")を呼び、その行の minDamage を返す。
func bulkMinDamage(t *testing.T, h http.Handler, defenderSpeciesKey string) int {
	t.Helper()
	body := map[string]any{
		"format": "single", "attacker": indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": defenderSpeciesKey, "moveId": movePhysical, "presets": []any{"none"},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
	var got struct {
		Rows []struct {
			Result struct {
				MinDamage int `json:"minDamage"`
			} `json:"result"`
		} `json:"rows"`
	}
	decodeInto(t, rec, &got)
	if len(got.Rows) == 0 {
		t.Fatal("rows が空")
	}
	return got.Rows[0].Result.MinDamage
}

// TestCalcBulkDefaultResolvedAbilityEffectReachesEngine(critic 指摘。重要): defenderOverride を
// 渡さずに種族の特性(1件だけ)を既定で解決したとき、その特性の効果(DefResistType)が実際に
// engine.CalcDamage まで届いてダメージを変えることを固定する。特性を渡すだけで効果を落として
// ID だけ渡す退行(engine.Ability{ID: a.ID} 相当)を検知できる。
func TestCalcBulkDefaultResolvedAbilityEffectReachesEngine(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	noEffect := bulkMinDamage(t, h, speciesSingleNoEffect)
	resisted := bulkMinDamage(t, h, speciesSingleResist)
	if resisted >= noEffect {
		t.Errorf("特性を1件だけ持つ種族の既定ダメージ: 半減特性=%d, 無効果=%d(半減が効いていない)", resisted, noEffect)
	}
}

// TestCalcBulkDefaultSplitsRowsWhenAbilitiesDiffer(critic 指摘): 2件のうち1件だけ効果を持つ種族では、
// 既定(上書き無し)で結果が違う特性ごとに行が分かれる(ADR-0126 の「結果が同じ特性は1行にまとめ、
// 違うときだけ分ける」を1プリセットの中で確認する)。
func TestCalcBulkDefaultSplitsRowsWhenAbilitiesDiffer(t *testing.T) {
	store := newStoreWithFourAbilitySpecies(t)
	h := NewHandler(store, nil)

	body := map[string]any{
		"format": "single", "attacker": indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
		"defenderSpeciesKey": speciesTwoAbilities, "moveId": movePhysical, "presets": []any{"none"},
	}
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
	var got struct {
		Rows []struct {
			AbilityId  string   `json:"abilityId"`
			AbilityIds []string `json:"abilityIds"`
			Result     struct {
				MinDamage int `json:"minDamage"`
			} `json:"result"`
		} `json:"rows"`
	}
	decodeInto(t, rec, &got)

	if len(got.Rows) != 2 {
		t.Fatalf("rows の件数 = %d, want 2(1件目〈無効果〉と2件目〈半減〉で結果が違うので行が分かれる): %+v", len(got.Rows), got.Rows)
	}
	if got.Rows[0].AbilityId != abilityNoEffectA || len(got.Rows[0].AbilityIds) != 1 || got.Rows[0].AbilityIds[0] != abilityNoEffectA {
		t.Errorf("rows[0] = %+v, want abilityId/abilityIds が %q だけ", got.Rows[0], abilityNoEffectA)
	}
	if got.Rows[1].AbilityId != abilityResistNormal || len(got.Rows[1].AbilityIds) != 1 || got.Rows[1].AbilityIds[0] != abilityResistNormal {
		t.Errorf("rows[1] = %+v, want abilityId/abilityIds が %q だけ", got.Rows[1], abilityResistNormal)
	}
	if got.Rows[1].Result.MinDamage >= got.Rows[0].Result.MinDamage {
		t.Errorf("2件目(半減)のダメージ %d が1件目(無効果)%d 以上", got.Rows[1].Result.MinDamage, got.Rows[0].Result.MinDamage)
	}
}
