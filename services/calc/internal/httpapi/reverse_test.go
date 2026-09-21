package httpapi

// POST /api/calc/reverse の受け入れテスト(ADR-0018 AC-4、ADR-0010 §R)。
// 期待値は同じ入力を engine.CalcReverse に直接渡した結果と照合する。観測は engine.CalcDamage で
// 作った「真値」のダメージから作る(手計算しない)。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// reverseCase は /api/calc/reverse の1ケース。
type reverseCase struct {
	name           string
	side           engine.ReverseSide
	known          indiv
	unknownSpecies string
	moveID         string
	itemIDs        []string // "" は持ち物なし(null)。nil は省略
	observations   []engine.Observation
	maxCandidates  int
}

func (c reverseCase) httpBody() map[string]any {
	obs := make([]any, 0, len(c.observations))
	for _, o := range c.observations {
		m := map[string]any{}
		if o.Percent != 0 {
			m["percent"] = o.Percent
		}
		if o.PercentTenths != 0 {
			m["percentTenths"] = o.PercentTenths
		}
		if o.Damage != 0 {
			m["damage"] = o.Damage
		}
		if o.Note != "" {
			m["note"] = o.Note
		}
		obs = append(obs, m)
	}
	body := map[string]any{
		"format":            "single",
		"side":              string(c.side),
		"known":             c.known.http(),
		"unknownSpeciesKey": c.unknownSpecies,
		"moveId":            c.moveID,
		"observations":      obs,
	}
	if c.itemIDs != nil {
		items := make([]any, 0, len(c.itemIDs))
		for _, id := range c.itemIDs {
			if id == "" {
				items = append(items, nil)
			} else {
				items = append(items, id)
			}
		}
		body["itemCandidates"] = items
	}
	if c.maxCandidates != 0 {
		body["maxCandidates"] = c.maxCandidates
	}
	return body
}

func (c reverseCase) engineInput(t *testing.T, f *fakeStore) engine.ReverseInput {
	t.Helper()
	var items []*engine.Item
	for _, id := range c.itemIDs {
		if id == "" {
			items = append(items, nil)
			continue
		}
		it := f.items[id]
		items = append(items, &it)
	}
	return engine.ReverseInput{
		Format:         engine.FormatSingle,
		Side:           c.side,
		Known:          c.known.engine(t, f),
		UnknownSpecies: f.species[c.unknownSpecies],
		Move:           f.moves[c.moveID],
		Field:          engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone},
		TypeChart:      f.chart,
		ItemCandidates: items,
		Observations:   c.observations,
		MaxCandidates:  c.maxCandidates,
	}
}

// truthRolls は「真値」の相手で1発撃ったときの engine の結果(観測を作るため)。
func truthRolls(t *testing.T, f *fakeStore, in engine.DamageInput) engine.DamageResult {
	t.Helper()
	in.Format, in.TypeChart = engine.FormatSingle, f.chart
	in.Field = engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone}
	res, err := engine.CalcDamage(in)
	if err != nil {
		t.Fatalf("真値の engine.CalcDamage = %v", err)
	}
	return res
}

func reverseCases(t *testing.T, f *fakeStore) []reverseCase {
	t.Helper()
	atkKnown := indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{Atk: 32, Spe: 32}}
	defKnown := indiv{speciesKey: speciesDefender, natureID: natureNeutral, sp: engine.Stats{HP: 32, SpD: 32}}
	leafKnown := indiv{speciesKey: speciesLeaf, natureID: natureSpAUp, sp: engine.Stats{SpA: 32}}

	// defender 側の真値: H32 / B20 / 無補正 / 持ち物なし。
	defTruth := truthRolls(t, f, engine.DamageInput{
		Attacker: atkKnown.engine(t, f),
		Defender: engine.Individual{Species: f.species[speciesDefender], Level: engine.DefaultLevel,
			SP: engine.Stats{HP: 32, Def: 20}, Status: engine.StatusNone},
		Move: f.moves[movePhysical],
	})
	percent := defTruth.Rolls[8] * 100 / defTruth.DefenderHP
	// attacker 側の真値: A10 / A上昇 / 持ち物なし。
	atkTruth := truthRolls(t, f, engine.DamageInput{
		Attacker: engine.Individual{Species: f.species[speciesAttacker], Level: engine.DefaultLevel,
			Nature: engine.Nature{Plus: engine.StatAtk, Minus: engine.StatSpA}, SP: engine.Stats{Atk: 10}, Status: engine.StatusNone},
		Defender: defKnown.engine(t, f),
		Move:     f.moves[movePhysical],
	})
	// 特殊の defender 側の真値: H32 / D0。
	spTruth := truthRolls(t, f, engine.DamageInput{
		Attacker: leafKnown.engine(t, f),
		Defender: engine.Individual{Species: f.species[speciesDefender], Level: engine.DefaultLevel,
			SP: engine.Stats{HP: 32}, Status: engine.StatusNone},
		Move: f.moves[moveSpecial],
	})
	if percent < 1 || atkTruth.Rolls[3] < 1 || spTruth.Rolls[0] < 1 {
		t.Fatalf("fixture の真値のダメージが 0(観測を作れない): %d%%, %d, %d", percent, atkTruth.Rolls[3], spTruth.Rolls[0])
	}

	return []reverseCase{
		{
			name: "defender 側・整数%・持ち物候補2つ", side: engine.SideDefender, known: atkKnown,
			unknownSpecies: speciesDefender, moveID: movePhysical, itemIDs: []string{"", itemShell},
			observations: []engine.Observation{{Percent: percent, Note: "1発目"}},
		},
		{
			name: "defender 側・maxCandidates=1(exactCount は切る前の値)", side: engine.SideDefender, known: atkKnown,
			unknownSpecies: speciesDefender, moveID: movePhysical, itemIDs: []string{"", itemShell},
			observations: []engine.Observation{{Percent: percent}}, maxCandidates: 1,
		},
		{
			name: "attacker 側・実点数と 0.1% の2観測", side: engine.SideAttacker, known: defKnown,
			unknownSpecies: speciesAttacker, moveID: movePhysical, itemIDs: []string{"", itemOrb},
			observations: []engine.Observation{
				{Damage: atkTruth.Rolls[3]},
				{PercentTenths: atkTruth.Rolls[12] * 1000 / atkTruth.DefenderHP},
			},
		},
		{
			name: "defender 側・特殊(+D/-A の性格はマスタに無い → natureId null)・持ち物候補省略", side: engine.SideDefender,
			known: leafKnown, unknownSpecies: speciesDefender, moveID: moveSpecial,
			observations: []engine.Observation{{Damage: spTruth.Rolls[0]}},
		},
	}
}

func assertReverseMatchesEngine(t *testing.T, f *fakeStore, got api.ReverseResult, want engine.ReverseResult) {
	t.Helper()
	if string(got.Side) != string(want.Side) || string(got.Stat) != string(want.Stat) {
		t.Errorf("side/stat = %q/%q, want %q/%q", got.Side, got.Stat, want.Side, want.Stat)
	}
	if got.AssumedHpSp != want.AssumedHPSP {
		t.Errorf("assumedHpSp = %d, want %d", got.AssumedHpSp, want.AssumedHPSP)
	}
	if got.ExactCount != want.ExactCount {
		t.Errorf("exactCount = %d, want %d", got.ExactCount, want.ExactCount)
	}
	if len(got.Candidates) != len(want.Candidates) {
		t.Fatalf("候補数 = %d, want %d", len(got.Candidates), len(want.Candidates))
	}
	for i, w := range want.Candidates {
		g := got.Candidates[i]
		if string(g.NatureClass) != string(w.NatureClass) {
			t.Errorf("candidates[%d].natureClass = %q, want %q(順序は ADR-0010 §R4)", i, g.NatureClass, w.NatureClass)
		}
		if !equalStatKeyPtr(g.Nature.Plus, statKeyPtr(w.Nature.Plus)) || !equalStatKeyPtr(g.Nature.Minus, statKeyPtr(w.Nature.Minus)) {
			t.Errorf("candidates[%d].nature = %+v, want %+v", i, g.Nature, w.Nature)
		}
		wantID, ok := f.NatureID(w.Nature)
		switch {
		case !ok && g.NatureId != nil:
			t.Errorf("candidates[%d].natureId = %q, want null", i, *g.NatureId)
		case ok && (g.NatureId == nil || *g.NatureId != wantID):
			t.Errorf("candidates[%d].natureId = %s, want %q", i, strPtrString(g.NatureId), wantID)
		}
		switch {
		case w.ItemID == "" && g.ItemId != nil:
			t.Errorf("candidates[%d].itemId = %q, want null", i, *g.ItemId)
		case w.ItemID != "" && (g.ItemId == nil || *g.ItemId != w.ItemID):
			t.Errorf("candidates[%d].itemId = %s, want %q", i, strPtrString(g.ItemId), w.ItemID)
		}
		if len(g.Ranges) != len(w.Ranges) {
			t.Errorf("candidates[%d].ranges = %+v, want %+v", i, g.Ranges, w.Ranges)
		} else {
			for j, r := range w.Ranges {
				if g.Ranges[j].Min != r.Min || g.Ranges[j].Max != r.Max {
					t.Errorf("candidates[%d].ranges[%d] = %+v, want %+v", i, j, g.Ranges[j], r)
				}
			}
		}
		if g.SpCount != w.SPCount || g.Exact != w.Exact || g.Mismatch != w.Mismatch || g.Support != w.Support {
			t.Errorf("candidates[%d] spCount/exact/mismatch/support = %d/%v/%d/%d, want %d/%v/%d/%d",
				i, g.SpCount, g.Exact, g.Mismatch, g.Support, w.SPCount, w.Exact, w.Mismatch, w.Support)
		}
		if g.MinPercent != tenths(w.MinPercentTenths) || g.MaxPercent != tenths(w.MaxPercentTenths) {
			t.Errorf("candidates[%d] min/maxPercent = %v/%v, want %v/%v", i, g.MinPercent, g.MaxPercent,
				tenths(w.MinPercentTenths), tenths(w.MaxPercentTenths))
		}
	}
}

// AC-4: 成功時は engine.CalcReverse の結果の写し(順序・範囲・一致度・表示%・性格 ID の写像)。
func TestCalcReverseMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	for _, c := range reverseCases(t, store) {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.CalcReverse(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.CalcReverse = %v", err)
			}
			rec := post(t, h, "/api/calc/reverse", mustJSON(t, c.httpBody()), true)
			var got api.ReverseResult
			decodeInto(t, rec, &got)
			assertReverseMatchesEngine(t, store, got, want)
		})
	}
}

// AC-4: 性格クラス → 性格 ID の写像を具体値で固定する(defender 物理: plus = +B/-A、attacker 物理: plus = +A/-C)。
func TestCalcReverseNatureIDs(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	cases := reverseCases(t, store)
	want := map[string]map[api.NatureClass]string{
		cases[0].name: {api.Neutral: natureNeutral, api.Plus: natureDefUp},
		cases[2].name: {api.Neutral: natureNeutral, api.Plus: natureAtkUp},
	}
	for _, c := range []reverseCase{cases[0], cases[2]} {
		t.Run(c.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/reverse", mustJSON(t, c.httpBody()), true)
			var got api.ReverseResult
			decodeInto(t, rec, &got)
			if len(got.Candidates) == 0 {
				t.Fatal("候補が空")
			}
			for _, cand := range got.Candidates {
				if cand.NatureId == nil || *cand.NatureId != want[c.name][cand.NatureClass] {
					t.Errorf("natureClass %q の natureId = %s, want %q", cand.NatureClass, strPtrString(cand.NatureId), want[c.name][cand.NatureClass])
				}
			}
		})
	}
}

// AC-4: 観測・side・ID の不正(ADR-0018 の code 対応)。
func TestCalcReverseErrors(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	base := func() map[string]any { return reverseCases(t, store)[0].httpBody() }
	with := func(mutate func(b map[string]any)) []byte {
		b := base()
		mutate(b)
		return mustJSON(t, b)
	}
	tests := []struct {
		name     string
		body     []byte
		wantCode string
	}{
		{"観測 0 件", with(func(b map[string]any) { b["observations"] = []any{} }), "no_observation"},
		{"観測の欠落", with(func(b map[string]any) { delete(b, "observations") }), "no_observation"},
		{"percent と damage の両方", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"percent": 40, "damage": 50}}
		}), "invalid_observation"},
		{"どれも指定しない観測", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"note": "メモだけ"}}
		}), "invalid_observation"},
		// HTTP ではキーの有無で数える: percent:0 は「指定したが範囲外」(ADR-0018)。
		{"percent 0 と damage", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"percent": 0, "damage": 50}}
		}), "invalid_observation"},
		{"percent が 101", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"percent": 101}}
		}), "invalid_observation"},
		{"percentTenths が 1001", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"percentTenths": 1001}}
		}), "invalid_observation"},
		{"damage が負", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"damage": -3}}
		}), "invalid_observation"},
		{"percent に小数 12.5", []byte(`{"format":"single","side":"defender","known":{"speciesKey":"9001-000","natureId":"test-atk-up",` +
			`"sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}},"unknownSpeciesKey":"9002-000","moveId":"test-beam",` +
			`"observations":[{"percent":12.5}]}`), "invalid_json"},
		// side は WASM と同じく engine の sentinel に一本化する(列挙で先に弾くと invalid_enum になり食い違う。ADR-0018)。
		{"side が未知", with(func(b map[string]any) { b["side"] = "sideways" }), "invalid_reverse_side"},
		{"side の欠落", with(func(b map[string]any) { delete(b, "side") }), "invalid_reverse_side"},
		{"未知の相手種族", with(func(b map[string]any) { b["unknownSpeciesKey"] = speciesUnknown }), "unknown_species"},
		{"未知の持ち物候補", with(func(b map[string]any) { b["itemCandidates"] = []any{nil, "test-nothing"} }), "unknown_item"},
		{"未知の技", with(func(b map[string]any) { b["moveId"] = "test-nothing" }), "unknown_move"},
		{"既知側の性格が未知", with(func(b map[string]any) { b["known"].(map[string]any)["natureId"] = "test-nothing" }), "unknown_nature"},
		{"旧フィールド attacker", with(func(b map[string]any) { b["attacker"] = b["known"] }), "unknown_field"},
		{"旧フィールド observedPercent", with(func(b map[string]any) {
			b["observations"] = []any{map[string]any{"observedPercent": 40}}
		}), "unknown_field"},
		{"maxCandidates が負", with(func(b map[string]any) { b["maxCandidates"] = -1 }), "invalid_input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/reverse", tt.body, false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}
