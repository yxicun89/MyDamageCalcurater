package httpapi

// POST /api/calc の受け入れテスト(ADR-0200 AC-2)。期待値は手計算せず、同じ入力を
// engine.CalcDamage に直接渡した結果と照合する(HTTP 境界が engine の素通しであること)。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// calcCase は /api/calc の1ケース。HTTP のリクエストと engine の入力を同じ fixture から作る。
type calcCase struct {
	name     string
	attacker indiv
	defender indiv
	moveID   string
	field    map[string]any // HTTP の field(nil は省略)
	engField engine.Field
	critical bool
}

func (c calcCase) httpBody() map[string]any {
	body := map[string]any{
		"format":   "single",
		"attacker": c.attacker.http(),
		"defender": c.defender.http(),
		"moveId":   c.moveID,
	}
	if c.field != nil {
		body["field"] = c.field
	}
	if c.critical {
		body["options"] = map[string]any{"critical": true}
	}
	return body
}

func (c calcCase) engineInput(t *testing.T, f *fakeStore) engine.DamageInput {
	t.Helper()
	field := c.engField
	if field.Weather == "" {
		field.Weather = engine.WeatherNone
	}
	if field.Terrain == "" {
		field.Terrain = engine.TerrainNone
	}
	return engine.DamageInput{
		Format:    engine.FormatSingle,
		Attacker:  c.attacker.engine(t, f),
		Defender:  c.defender.engine(t, f),
		Move:      f.moves[c.moveID],
		Field:     field,
		Critical:  c.critical,
		TypeChart: f.chart,
	}
}

// assertCalcResultMatchesEngine は HTTP の CalcResult が engine の結果の写しであることを確かめる。
// 表示%は engine の tenths を 10 で割った値と一致すること(float で近似し直していないこと)。
func assertCalcResultMatchesEngine(t *testing.T, got api.CalcResult, want engine.DamageResult) {
	t.Helper()
	if len(got.Rolls) != len(want.Rolls) {
		t.Fatalf("rolls の数 = %d, want %d", len(got.Rolls), len(want.Rolls))
	}
	for i := range want.Rolls {
		if got.Rolls[i] != want.Rolls[i] {
			t.Errorf("rolls[%d] = %d, want %d", i, got.Rolls[i], want.Rolls[i])
		}
	}
	if got.MinDamage != want.MinDamage() || got.MaxDamage != want.MaxDamage() {
		t.Errorf("min/maxDamage = %d/%d, want %d/%d", got.MinDamage, got.MaxDamage, want.MinDamage(), want.MaxDamage())
	}
	minT, maxT := want.DisplayPercentRangeTenths()
	if got.MinPercent != tenths(minT) || got.MaxPercent != tenths(maxT) {
		t.Errorf("min/maxPercent = %v/%v, want %v/%v(tenths %d/%d ÷ 10)", got.MinPercent, got.MaxPercent, tenths(minT), tenths(maxT), minT, maxT)
	}
	if got.DefenderHP != want.DefenderHP {
		t.Errorf("defenderHP = %d, want %d", got.DefenderHP, want.DefenderHP)
	}
	if got.Effectiveness != want.Effectiveness {
		t.Errorf("effectiveness = %v, want %v", got.Effectiveness, want.Effectiveness)
	}
	if got.Stab != want.STAB {
		t.Errorf("stab = %v, want %v", got.Stab, want.STAB)
	}
	if string(got.Category) != string(want.Category) {
		t.Errorf("category = %q, want %q", got.Category, want.Category)
	}
	if got.Ko.Hits != want.KO.Hits || got.Ko.Guaranteed != want.KO.Guaranteed {
		t.Errorf("ko = hits %d guaranteed %v, want hits %d guaranteed %v", got.Ko.Hits, got.Ko.Guaranteed, want.KO.Hits, want.KO.Guaranteed)
	}
	// chancePercent は engine の生値を素通しする(WASM 境界と同じく常に返す。ADR-0006)。
	if got.Ko.ChancePercent == nil || *got.Ko.ChancePercent != want.KO.ChancePercent {
		t.Errorf("ko.chancePercent = %v, want %v", got.Ko.ChancePercent, want.KO.ChancePercent)
	}
	if got.Ko.DisplayChancePercent != tenths(want.KO.DisplayChancePercentTenths()) {
		t.Errorf("ko.displayChancePercent = %v, want %v", got.Ko.DisplayChancePercent, tenths(want.KO.DisplayChancePercentTenths()))
	}
}

func calcCases() []calcCase {
	return []calcCase{
		{
			name: "物理・タイプ一致・持ち物と特性あり・今ひとつ",
			attacker: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, abilityID: abilityBoost, itemID: itemOrb,
				sp: engine.Stats{Atk: 32, Spe: 32}},
			defender: indiv{speciesKey: speciesDefender, natureID: natureDefUp, itemID: itemShell,
				sp: engine.Stats{HP: 32, Def: 32}},
			moveID: movePhysical,
		},
		{
			name:     "特殊・等倍(抜群×今ひとつ)・天候・フィールド・急所",
			attacker: indiv{speciesKey: speciesLeaf, natureID: natureSpAUp, abilityID: abilityBoost, sp: engine.Stats{SpA: 32}},
			defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral, sp: engine.Stats{HP: 32}},
			moveID:   moveSpecial,
			field:    map[string]any{"weather": "rain", "terrain": "grassy"},
			engField: engine.Field{Weather: engine.WeatherRain, Terrain: engine.TerrainGrassy},
			critical: true,
		},
		{
			name: "ランク・テラスタル・やけど・壁",
			attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{Atk: 20},
				ranks: engine.Ranks{Atk: 2}, tera: engine.TypeFire, status: engine.StatusBurn},
			defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral, ranks: engine.Ranks{Def: -1}},
			moveID:   moveFire,
			field:    map[string]any{"defenderScreens": map[string]any{"reflect": true}},
			engField: engine.Field{DefenderScreens: engine.Screens{Reflect: true}},
		},
		{
			name:     "変化技(0 ダメージ・倒せない)",
			attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
			defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral},
			moveID:   moveStatus,
		},
	}
}

// AC-2: 成功時のレスポンスは engine.CalcDamage の結果の写しで、契約どおり。
func TestCalcDamageMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	for _, c := range calcCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.CalcDamage(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.CalcDamage = %v", err)
			}
			rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
			var got api.CalcResult
			decodeInto(t, rec, &got)
			assertCalcResultMatchesEngine(t, got, want)
		})
	}
}

// AC-2: 本文の moveId が attacker.moveId より優先される(契約の CalcRequest.moveId)。
func TestCalcDamageMoveIDTakesPrecedence(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	c := calcCases()[0]
	c.attacker.moveID = moveSpecial // 本文の moveId(物理)が勝つ
	want, err := engine.CalcDamage(c.engineInput(t, store))
	if err != nil {
		t.Fatalf("engine.CalcDamage = %v", err)
	}
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
	var got api.CalcResult
	decodeInto(t, rec, &got)
	if got.Category != api.MoveCategory(engine.CategoryPhysical) {
		t.Fatalf("category = %q, want physical(attacker.moveId ではなく本文の moveId を使う)", got.Category)
	}
	assertCalcResultMatchesEngine(t, got, want)
}

// AC-2: 計算はイベント保存に依存しない(絶対ルール5)。record/team/NATS が無くても 200。
// ここでは Store 以外の依存を何も渡さずに計算が成功することを固定する。
func TestCalcDamageNeedsOnlyStore(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	rec := post(t, h, "/api/calc", mustJSON(t, calcCases()[0].httpBody()), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
