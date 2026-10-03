package httpapi

// POST /api/calc/adjust/* の受け入れテスト(plan.md AJ4。契約は api/openapi.yaml と ADR-0250)。
//
//   - AC-H1 素通し: 成功時は同じ入力を engine(RealStats・FirepowerIndex・BulkIndex・HPLines・MinSPToKO・
//     MinSPToSurvive・SuggestSPAllocation)に直接渡した結果の写し。ID はマスタ(fake の Store)で解決する。
//   - AC-H2 既定: modifier / damageModifier の省略は 4096、thresholdPercent は 100、ceiling の各能力は 32、
//     moveId の省略は firepowerIndex=null、goal の省略は minSp=null・unsupported=[]。
//   - AC-H3 エラー語彙: 既存の code だけを使う(新しい code を足さない。ADR-0250 §4)。
//   - AC-H4 検査順: hits・thresholdPercent・ceiling・minSpeed・modifier・damageModifier の値域は ID の解決より前(マスタ参照 0 回)。
//   - AC-H5 パリティ: 同じ入力は engine/wasmapi と同じ応答(JSON として等しい)・同じ失敗は同じ code。
//   - AC-H6 ステートレス: 計算イベントを発行しない。マスタ準備中は 503 master_unavailable。
//
// 期待値はすべて engine を直接呼んで作る(手計算しない)。応答は post が契約(kin-openapi)に照らす。

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
	"example.com/pokecalc/services/calc/internal/master"
	"example.com/pokecalc/services/internal/api"
)

const (
	pathAdjustIndices    = "/api/calc/adjust/indices"
	pathAdjustMinSPKO    = "/api/calc/adjust/min-sp-to-ko"
	pathAdjustMinSPSurv  = "/api/calc/adjust/min-sp-to-survive"
	pathAdjustAllocation = "/api/calc/adjust/allocation"
)

// adjustPaths は調整の4操作のパス。
var adjustPaths = []string{pathAdjustIndices, pathAdjustMinSPKO, pathAdjustMinSPSurv, pathAdjustAllocation}

func ptrInt(v int) *int { return &v }

func engineField() engine.Field {
	return engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone}
}

// --- indices -----------------------------------------------------------------

type adjIndicesCase struct {
	name           string
	self           indiv
	moveID         string // "" は省略
	modifier       *int
	damageModifier *int
}

func (c adjIndicesCase) httpBody() map[string]any {
	b := map[string]any{"individual": c.self.http()}
	if c.moveID != "" {
		b["moveId"] = c.moveID
	}
	if c.modifier != nil {
		b["modifier"] = *c.modifier
	}
	if c.damageModifier != nil {
		b["damageModifier"] = *c.damageModifier
	}
	return b
}

func (c adjIndicesCase) wasmBody(t *testing.T, f *fakeStore) map[string]any {
	b := map[string]any{"individual": c.self.wasm(t, f)}
	if c.moveID != "" {
		b["move"] = wasmMove(f.moves[c.moveID])
	}
	if c.modifier != nil {
		b["modifier"] = *c.modifier
	}
	if c.damageModifier != nil {
		b["damageModifier"] = *c.damageModifier
	}
	return b
}

func adjIndicesCases() []adjIndicesCase {
	return []adjIndicesCase{
		{name: "物理技・補正の省略は 4096", self: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{HP: 20, Atk: 32, Spe: 14}},
			moveID: movePhysical},
		{name: "特殊技・補正を明示・持ち物と特性は指数に使わない", self: indiv{speciesKey: speciesLeaf, natureID: natureSpAUp,
			abilityID: abilityBoost, itemID: itemShell, sp: engine.Stats{HP: 4, SpA: 32, Spe: 30}},
			moveID: moveSpecial, modifier: ptrInt(6144), damageModifier: ptrInt(2048)},
		{name: "技なし(firepowerIndex は null)", self: indiv{speciesKey: speciesDefender, natureID: natureDefUp, sp: engine.Stats{HP: 32, Def: 32}}},
		{name: "ランクは指数に含めない", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{Atk: 32},
			ranks: engine.Ranks{Atk: 6, Def: -6}}, moveID: movePhysical},
	}
}

func TestAdjustIndicesHTTPMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	for _, c := range adjIndicesCases() {
		t.Run(c.name, func(t *testing.T) {
			rec := post(t, h, pathAdjustIndices, mustJSON(t, c.httpBody()), true)
			var got api.AdjustIndicesResult
			decodeInto(t, rec, &got)

			self := c.self.engine(t, store)
			mod, dmg := engine.Modifier4096, engine.Modifier4096
			if c.modifier != nil {
				mod = *c.modifier
			}
			if c.damageModifier != nil {
				dmg = *c.damageModifier
			}
			if want := statBlockFrom(engine.RealStats(self)); got.Stats != want {
				t.Errorf("stats = %+v, want %+v", got.Stats, want)
			}
			if c.moveID == "" {
				if got.FirepowerIndex != nil {
					t.Errorf("firepowerIndex = %d, want null", *got.FirepowerIndex)
				}
				if !strings.Contains(rec.Body.String(), `"firepowerIndex":null`) {
					t.Errorf("firepowerIndex のキーが省略されている(null で明示する): %s", rec.Body.String())
				}
			} else {
				mv := store.moves[c.moveID]
				want, err := engine.FirepowerIndex(self, mv.Category, mv.Power, mod)
				if err != nil {
					t.Fatalf("engine.FirepowerIndex = %v", err)
				}
				if got.FirepowerIndex == nil || *got.FirepowerIndex != int64(want) {
					t.Errorf("firepowerIndex = %v, want %d", got.FirepowerIndex, want)
				}
			}
			wantPhys, err := engine.BulkIndex(self, engine.CategoryPhysical, dmg)
			if err != nil {
				t.Fatalf("engine.BulkIndex = %v", err)
			}
			wantSpec, err := engine.BulkIndex(self, engine.CategorySpecial, dmg)
			if err != nil {
				t.Fatalf("engine.BulkIndex = %v", err)
			}
			if got.PhysicalBulkIndex != int64(wantPhys) || got.SpecialBulkIndex != int64(wantSpec) {
				t.Errorf("physical/specialBulkIndex = %d/%d, want %d/%d", got.PhysicalBulkIndex, got.SpecialBulkIndex, wantPhys, wantSpec)
			}
			lines, err := engine.HPLines(self.Species.BaseStats.HP, self.SP.HP)
			if err != nil {
				t.Fatalf("engine.HPLines = %v", err)
			}
			if want := hpLineReportOf(lines); !reflect.DeepEqual(got.HpLines, want) {
				t.Errorf("hpLines = %+v, want %+v", got.HpLines, want)
			}
		})
	}
}

func hpLineReportOf(r engine.HPLineReport) api.HPLineReport {
	point := func(p *engine.HPLinePoint) *api.HPLinePoint {
		if p == nil {
			return nil
		}
		return &api.HPLinePoint{Hp: p.HP, Sp: p.SP, SpDelta: p.SPDelta}
	}
	current := api.HPLineKindNone
	switch r.Current {
	case engine.HPLine16n:
		current = api.HPLineKindN16n
	case engine.HPLine16nMinus1:
		current = api.HPLineKindN16n1
	}
	return api.HPLineReport{
		Hp: r.HP, Sp: r.SP, Current: current,
		Next16n: point(r.Next16n), Prev16n: point(r.Prev16n),
		Next16nMinus1: point(r.Next16nMinus1), Prev16nMinus1: point(r.Prev16nMinus1),
	}
}

// --- min-sp-to-ko / min-sp-to-survive -----------------------------------------

type adjSearchCase struct {
	name      string
	attacker  indiv
	defender  indiv
	moveID    string
	hits      int
	threshold *float64
	critical  bool
}

func (c adjSearchCase) httpBody() map[string]any {
	b := map[string]any{
		"format": "single", "attacker": c.attacker.http(), "defender": c.defender.http(),
		"moveId": c.moveID, "hits": c.hits,
	}
	if c.threshold != nil {
		b["thresholdPercent"] = *c.threshold
	}
	if c.critical {
		b["options"] = map[string]any{"critical": true}
	}
	return b
}

func (c adjSearchCase) wasmBody(t *testing.T, f *fakeStore) map[string]any {
	b := map[string]any{
		"format": "single", "attacker": c.attacker.wasm(t, f), "defender": c.defender.wasm(t, f),
		"move": wasmMove(f.moves[c.moveID]), "critical": c.critical, "hits": c.hits, "typeChart": wasmTypeChart(t),
	}
	if c.threshold != nil {
		b["thresholdPercent"] = *c.threshold
	}
	return b
}

func (c adjSearchCase) engineInput(t *testing.T, f *fakeStore) engine.AdjustSearchInput {
	in := engine.AdjustSearchInput{
		Format: engine.FormatSingle, Attacker: c.attacker.engine(t, f), Defender: c.defender.engine(t, f),
		Move: f.moves[c.moveID], Field: engineField(), Critical: c.critical, TypeChart: f.chart, Hits: c.hits,
	}
	if c.threshold != nil {
		in.ThresholdPercent = *c.threshold
	}
	return in
}

func half() *float64 { v := 50.0; return &v }

func adjKOCases() []adjSearchCase {
	return []adjSearchCase{
		{name: "確定(しきい値の省略)・2発", attacker: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{HP: 32, Spe: 32}},
			defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{HP: 32}}, moveID: movePhysical, hits: 2},
		{name: "しきい値 50%・特殊・特性と持ち物はマスタから解決", attacker: indiv{speciesKey: speciesLeaf, natureID: natureSpAUp, abilityID: abilityBoost},
			defender: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, itemID: itemShell, sp: engine.Stats{HP: 32, SpD: 32}},
			moveID:   moveSpecial, hits: 2, threshold: half()},
		{name: "無効相性はエラーにしない(feasible=false)", attacker: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp},
			defender: indiv{speciesKey: speciesGhost, natureID: natureNeutral}, moveID: movePhysical, hits: 1},
		{name: "急所", attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
			defender: indiv{speciesKey: speciesDefender, natureID: natureDefUp, sp: engine.Stats{HP: 32, Def: 32}}, moveID: moveFire, hits: 3, critical: true},
	}
}

func adjSurviveCases() []adjSearchCase {
	return []adjSearchCase{
		{name: "物理・2発・確定", attacker: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, itemID: itemOrb, sp: engine.Stats{Atk: 32}},
			defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{SpA: 32}}, moveID: movePhysical, hits: 2},
		{name: "特殊・1発・しきい値 50%", attacker: indiv{speciesKey: speciesLeaf, natureID: natureSpAUp, abilityID: abilityBoost, sp: engine.Stats{SpA: 32}},
			defender: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{Atk: 32, Spe: 32}}, moveID: moveSpecial, hits: 1, threshold: half()},
		{name: "届かない(最も耐える組)", attacker: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, itemID: itemOrb, sp: engine.Stats{Atk: 32}},
			defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{SpA: 32, Spe: 32}}, moveID: moveFire, hits: 10},
	}
}

func koResultOf(r engine.KOSearchResult) api.AdjustKOResult {
	return api.AdjustKOResult{
		Stat: api.StatKey(r.Stat), SearchLimit: r.SearchLimit, Feasible: r.Feasible, Sp: r.SP,
		ChancePercent: r.ChancePercent, Unsupported: unsupportedFrom(r.Unsupported),
	}
}

func surviveResultOf(r engine.SurviveSearchResult) api.AdjustSurviveResult {
	return api.AdjustSurviveResult{
		Stat: api.StatKey(r.Stat), SearchLimit: r.SearchLimit, Feasible: r.Feasible, HpSp: r.HPSP, StatSp: r.StatSP,
		TotalSp: r.TotalSP, BulkIndex: int64(r.BulkIndex), ChancePercent: r.ChancePercent, Unsupported: unsupportedFrom(r.Unsupported),
	}
}

func TestAdjustMinSpToKoHTTPMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	for _, c := range adjKOCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.MinSPToKO(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.MinSPToKO = %v", err)
			}
			rec := post(t, h, pathAdjustMinSPKO, mustJSON(t, c.httpBody()), true)
			var got api.AdjustKOResult
			decodeInto(t, rec, &got)
			if w := koResultOf(want); !reflect.DeepEqual(got, w) {
				t.Errorf("結果が engine と違う\n got %+v\nwant %+v", got, w)
			}
		})
	}
}

func TestAdjustMinSpToSurviveHTTPMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	for _, c := range adjSurviveCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.MinSPToSurvive(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.MinSPToSurvive = %v", err)
			}
			rec := post(t, h, pathAdjustMinSPSurv, mustJSON(t, c.httpBody()), true)
			var got api.AdjustSurviveResult
			decodeInto(t, rec, &got)
			if w := surviveResultOf(want); !reflect.DeepEqual(got, w) {
				t.Errorf("結果が engine と違う\n got %+v\nwant %+v", got, w)
			}
		})
	}
}

// AC-H1(ADR-0250 §2): 探索する能力の SP は無視する。calc-svc が個体を先に Validate すると合計 66 超で落ちる。
func TestAdjustSearchHTTPIgnoresSearchedStatSP(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)

	ko := adjKOCases()[0]
	ko.attacker.sp = engine.Stats{HP: 32, Def: 2, Spe: 32} // atk 以外で 66
	base := post(t, h, pathAdjustMinSPKO, mustJSON(t, ko.httpBody()), true)
	ko.attacker.sp.Atk = 32
	got := post(t, h, pathAdjustMinSPKO, mustJSON(t, ko.httpBody()), false)
	if base.Code != http.StatusOK || got.Code != http.StatusOK || base.Body.String() != got.Body.String() {
		t.Errorf("ko: 探索する能力(atk)の SP で応答が変わった\nbase %d %s\ngot  %d %s", base.Code, base.Body, got.Code, got.Body)
	}

	sv := adjSurviveCases()[0]
	sv.defender.sp = engine.Stats{Atk: 32, SpA: 32, Spe: 2} // hp・def 以外で 66
	base = post(t, h, pathAdjustMinSPSurv, mustJSON(t, sv.httpBody()), true)
	sv.defender.sp.HP, sv.defender.sp.Def = 32, 32
	got = post(t, h, pathAdjustMinSPSurv, mustJSON(t, sv.httpBody()), false)
	if base.Code != http.StatusOK || got.Code != http.StatusOK || base.Body.String() != got.Body.String() {
		t.Errorf("survive: 探索する能力(hp・def)の SP で応答が変わった\nbase %d %s\ngot  %d %s", base.Code, base.Body, got.Code, got.Body)
	}
}

// --- allocation ------------------------------------------------------------------

type adjAllocCase struct {
	name     string
	self     indiv
	ceiling  map[string]any // nil は省略
	mode     engine.AllocMode
	focus    engine.BulkFocus
	category engine.MoveCategory
	minSpeed int
	goal     *adjAllocGoalCase
}

type adjAllocGoalCase struct {
	opponent  indiv
	moveID    string
	hits      int
	threshold *float64
}

func (c adjAllocCase) httpBody() map[string]any {
	b := map[string]any{"self": c.self.http(), "mode": string(c.mode)}
	if c.ceiling != nil {
		b["ceiling"] = c.ceiling
	}
	if c.focus != "" {
		b["focus"] = string(c.focus)
	}
	if c.category != "" {
		b["offenseCategory"] = string(c.category)
	}
	if c.minSpeed != 0 {
		b["minSpeed"] = c.minSpeed
	}
	if g := c.goal; g != nil {
		goal := map[string]any{"format": "single", "opponent": g.opponent.http(), "moveId": g.moveID, "hits": g.hits}
		if g.threshold != nil {
			goal["thresholdPercent"] = *g.threshold
		}
		b["goal"] = goal
	}
	return b
}

func (c adjAllocCase) wasmBody(t *testing.T, f *fakeStore) map[string]any {
	b := map[string]any{"self": c.self.wasm(t, f), "mode": string(c.mode), "typeChart": wasmTypeChart(t)}
	if c.ceiling != nil {
		b["ceiling"] = c.ceiling
	}
	if c.focus != "" {
		b["focus"] = string(c.focus)
	}
	if c.category != "" {
		b["offenseCategory"] = string(c.category)
	}
	if c.minSpeed != 0 {
		b["minSpeed"] = c.minSpeed
	}
	if g := c.goal; g != nil {
		goal := map[string]any{"format": "single", "opponent": g.opponent.wasm(t, f), "move": wasmMove(f.moves[g.moveID]),
			"critical": false, "hits": g.hits}
		if g.threshold != nil {
			goal["thresholdPercent"] = *g.threshold
		}
		b["goal"] = goal
	}
	return b
}

func (c adjAllocCase) engineInput(t *testing.T, f *fakeStore) engine.SPAllocInput {
	ceiling := engine.Stats{HP: 32, Atk: 32, Def: 32, SpA: 32, SpD: 32, Spe: 32} // 省略は 32(ADR-0250 §3)
	for k, v := range c.ceiling {
		ceiling = ceiling.WithStat(engine.StatKey(k), v.(int))
	}
	in := engine.SPAllocInput{
		Self: c.self.engine(t, f), Ceiling: ceiling, Mode: c.mode, Focus: c.focus, OffenseCategory: c.category, MinSpeed: c.minSpeed,
	}
	if g := c.goal; g != nil {
		goal := &engine.AllocGoal{
			Format: engine.FormatSingle, Opponent: g.opponent.engine(t, f), Move: f.moves[g.moveID], Field: engineField(),
			TypeChart: f.chart, Hits: g.hits,
		}
		if g.threshold != nil {
			goal.ThresholdPercent = *g.threshold
		}
		in.Goal = goal
	}
	return in
}

func adjAllocCases() []adjAllocCase {
	foe := indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, itemID: itemOrb, sp: engine.Stats{Atk: 32, Spe: 32}}
	return []adjAllocCase{
		{name: "耐久側・both・目標なし・ceiling 省略", self: indiv{speciesKey: speciesDefender, natureID: natureNeutral, sp: engine.Stats{SpA: 32}},
			mode: engine.AllocModeBulk, focus: engine.BulkFocusBoth},
		{name: "耐久側・physical・目標あり・H の上限 28", self: indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{SpA: 32}},
			ceiling: map[string]any{"hp": 28}, mode: engine.AllocModeBulk, focus: engine.BulkFocusPhysical,
			goal: &adjAllocGoalCase{opponent: foe, moveID: movePhysical, hits: 2}},
		{name: "攻撃側・素早さ目標・しきい値 50%", self: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp},
			mode: engine.AllocModeOffense, category: engine.CategoryPhysical, minSpeed: 130,
			goal: &adjAllocGoalCase{opponent: indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{HP: 32}},
				moveID: moveFire, hits: 1, threshold: half()}},
		{name: "攻撃側・特殊・D の上限 0 は回さない能力なので無関係", self: indiv{speciesKey: speciesLeaf, natureID: natureSpAUp},
			ceiling: map[string]any{"spd": 0, "spe": 10}, mode: engine.AllocModeOffense, category: engine.CategorySpecial},
	}
}

func allocPlanOf(p engine.AllocPlan) api.AdjustAllocPlan {
	return api.AdjustAllocPlan{
		Sp: statBlockFrom(p.SP), TotalSp: p.TotalSP, Stats: statBlockFrom(p.Real),
		PhysicalBulk: int64(p.PhysicalBulk), SpecialBulk: int64(p.SpecialBulk),
		SpeedMet: p.SpeedMet, GoalMet: p.GoalMet, ChancePercent: p.ChancePercent,
	}
}

func TestAdjustAllocationHTTPMatchesEngine(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	for _, c := range adjAllocCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.SuggestSPAllocation(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.SuggestSPAllocation = %v", err)
			}
			rec := post(t, h, pathAdjustAllocation, mustJSON(t, c.httpBody()), true)
			var got api.AdjustAllocationResult
			decodeInto(t, rec, &got)
			w := api.AdjustAllocationResult{
				Remaining: want.Remaining, MaxIndex: allocPlanOf(want.MaxIndex), Unsupported: unsupportedFrom(want.Unsupported),
			}
			if want.MinSP != nil {
				p := allocPlanOf(*want.MinSP)
				w.MinSp = &p
			} else if !strings.Contains(rec.Body.String(), `"minSp":null`) {
				t.Errorf("goal なしの minSp は null で明示する: %s", rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"unsupported":[`) {
				t.Errorf("unsupported は常に配列(null・省略にしない): %s", rec.Body.String())
			}
			if !reflect.DeepEqual(got, w) {
				t.Errorf("結果が engine と違う\n got %+v\nwant %+v", got, w)
			}
		})
	}
}

// AC-H2: ceiling の省略・空オブジェクト・全能力 32 の明示は同じ応答。
func TestAdjustAllocationHTTPCeilingDefault(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	c := adjAllocCases()[0]
	omitted := post(t, h, pathAdjustAllocation, mustJSON(t, c.httpBody()), true)
	if omitted.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", omitted.Code, omitted.Body)
	}
	for name, ceiling := range map[string]map[string]any{
		"空オブジェクト": {},
		"全能力 32":  {"hp": 32, "atk": 32, "def": 32, "spa": 32, "spd": 32, "spe": 32},
	} {
		c := c
		c.ceiling = ceiling
		rec := post(t, h, pathAdjustAllocation, mustJSON(t, c.httpBody()), true)
		if rec.Body.String() != omitted.Body.String() {
			t.Errorf("ceiling の省略と %s で応答が違う\nomitted: %s\n%s: %s", name, omitted.Body, name, rec.Body)
		}
	}
}

// --- エラー --------------------------------------------------------------------

func TestAdjustHTTPErrors(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	set := func(m map[string]any, k string, v any) map[string]any { m[k] = v; return m }
	del := func(m map[string]any, k string) map[string]any { delete(m, k); return m }
	indices := func() map[string]any { return adjIndicesCases()[0].httpBody() }
	ko := func() map[string]any { return adjKOCases()[0].httpBody() }
	survive := func() map[string]any { return adjSurviveCases()[0].httpBody() }
	bulk := func() map[string]any { return adjAllocCases()[1].httpBody() }
	offense := func() map[string]any { return adjAllocCases()[2].httpBody() }
	goal := func(m map[string]any, k string, v any) map[string]any { m["goal"].(map[string]any)[k] = v; return m }
	noMove := func() map[string]any { return adjIndicesCases()[2].httpBody() } // moveId を省略

	tests := []struct {
		name string
		path string
		body any // map または生の本文
		want string
	}{
		// 共通
		{"壊れた JSON", pathAdjustIndices, `{"individual":`, "invalid_json"},
		{"配列", pathAdjustAllocation, `[]`, "invalid_json"},
		{"契約に無いフィールド", pathAdjustMinSPKO, set(ko(), "typeChart", map[string]any{}), "unknown_field"},
		// indices
		{"indices: 未知の種族", pathAdjustIndices, set(indices(), "individual", indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()), "unknown_species"},
		{"indices: 未知の性格", pathAdjustIndices, set(indices(), "individual", indiv{speciesKey: speciesAttacker, natureID: "test-nothing"}.http()), "unknown_nature"},
		{"indices: 未知の持ち物(指数に使わなくても解決する)", pathAdjustIndices,
			set(indices(), "individual", indiv{speciesKey: speciesAttacker, natureID: natureNeutral, itemID: "test-nothing"}.http()), "unknown_item"},
		{"indices: 未知の技", pathAdjustIndices, set(indices(), "moveId", "test-nothing"), "unknown_move"},
		{"indices: 変化技", pathAdjustIndices, set(indices(), "moveId", moveStatus), "invalid_input"},
		{"indices: modifier 0", pathAdjustIndices, set(indices(), "modifier", 0), "invalid_input"},
		{"indices: damageModifier が上限超え", pathAdjustIndices, set(indices(), "damageModifier", engine.MaxEffectModifier+1), "invalid_input"},
		// moveId を省略しても modifier の値域は検査する(契約 AdjustModifier・ADR-0250 §3・§5)。
		{"indices: 技なしで modifier 0", pathAdjustIndices, set(noMove(), "modifier", 0), "invalid_input"},
		{"indices: 技なしで modifier が上限超え", pathAdjustIndices, set(noMove(), "modifier", engine.MaxEffectModifier+1), "invalid_input"},
		{"indices: 技なしで damageModifier 0", pathAdjustIndices, set(noMove(), "damageModifier", 0), "invalid_input"},
		{"indices: SP の合計 67", pathAdjustIndices, set(indices(), "individual",
			indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{HP: 32, Atk: 32, Spe: 3}}.http()), "invalid_input"},
		{"indices: 未知の状態異常", pathAdjustIndices, set(indices(), "individual",
			indiv{speciesKey: speciesAttacker, natureID: natureNeutral, status: "confused"}.http()), "invalid_enum"},
		// 探索
		{"ko: hits 0", pathAdjustMinSPKO, set(ko(), "hits", 0), "invalid_input"},
		{"ko: hits の欠落", pathAdjustMinSPKO, del(ko(), "hits"), "invalid_input"},
		{"ko: hits 11", pathAdjustMinSPKO, set(ko(), "hits", engine.MaxAdjustHits+1), "invalid_input"},
		{"ko: hits が小数", pathAdjustMinSPKO, set(ko(), "hits", 1.5), "invalid_json"},
		{"ko: しきい値 0 の明示", pathAdjustMinSPKO, set(ko(), "thresholdPercent", 0), "invalid_input"},
		{"ko: しきい値 100 超", pathAdjustMinSPKO, set(ko(), "thresholdPercent", 100.5), "invalid_input"},
		{"ko: 変化技", pathAdjustMinSPKO, set(ko(), "moveId", moveStatus), "invalid_input"},
		{"ko: 未知の技", pathAdjustMinSPKO, set(ko(), "moveId", "test-nothing"), "unknown_move"},
		{"ko: 未知の相手の種族", pathAdjustMinSPKO, set(ko(), "defender", indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()), "unknown_species"},
		{"ko: 未知の format", pathAdjustMinSPKO, set(ko(), "format", "triple"), "invalid_enum"},
		{"ko: 未知の天候", pathAdjustMinSPKO, set(ko(), "field", map[string]any{"weather": "meteor"}), "invalid_enum"},
		{"ko: 固定 SP(atk 以外)の合計 66 超", pathAdjustMinSPKO, set(ko(), "attacker",
			indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{HP: 32, Def: 32, Spe: 32}}.http()), "invalid_input"},
		{"survive: hits 11", pathAdjustMinSPSurv, set(survive(), "hits", engine.MaxAdjustHits+1), "invalid_input"},
		{"survive: 変化技", pathAdjustMinSPSurv, set(survive(), "moveId", moveStatus), "invalid_input"},
		{"survive: 未知の特性", pathAdjustMinSPSurv, set(survive(), "attacker",
			indiv{speciesKey: speciesAttacker, natureID: natureNeutral, abilityID: "test-nothing"}.http()), "unknown_ability"},
		// 配分
		{"alloc: 未知の mode", pathAdjustAllocation, set(bulk(), "mode", "balanced"), "invalid_enum"},
		{"alloc: mode の欠落", pathAdjustAllocation, del(bulk(), "mode"), "invalid_enum"},
		{"alloc: 未知の focus", pathAdjustAllocation, set(bulk(), "focus", "mixed"), "invalid_enum"},
		{"alloc: bulk で focus の欠落", pathAdjustAllocation, del(bulk(), "focus"), "invalid_input"},
		{"alloc: bulk で minSpeed", pathAdjustAllocation, set(bulk(), "minSpeed", 100), "invalid_input"},
		{"alloc: 未知の offenseCategory", pathAdjustAllocation, set(offense(), "offenseCategory", "melee"), "invalid_enum"},
		{"alloc: offense で offenseCategory の欠落", pathAdjustAllocation, del(offense(), "offenseCategory"), "invalid_input"},
		{"alloc: offenseCategory が status", pathAdjustAllocation, set(offense(), "offenseCategory", "status"), "invalid_input"},
		{"alloc: minSpeed が負", pathAdjustAllocation, set(offense(), "minSpeed", -1), "invalid_input"},
		{"alloc: ceiling 33", pathAdjustAllocation, set(bulk(), "ceiling", map[string]any{"def": 33}), "invalid_input"},
		{"alloc: ceiling が負", pathAdjustAllocation, set(bulk(), "ceiling", map[string]any{"def": -1}), "invalid_input"},
		{"alloc: ceiling に契約外の能力", pathAdjustAllocation, set(bulk(), "ceiling", map[string]any{"luck": 1}), "unknown_field"},
		{"alloc: ceiling が下限未満", pathAdjustAllocation, set(set(bulk(), "self",
			indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{HP: 20}}.http()), "ceiling", map[string]any{"hp": 10}), "invalid_input"},
		{"alloc: 下限の合計 67", pathAdjustAllocation, set(bulk(), "self",
			indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{HP: 32, Atk: 32, Spe: 3}}.http()), "invalid_input"},
		{"alloc: goal の hits 0", pathAdjustAllocation, goal(bulk(), "hits", 0), "invalid_input"},
		{"alloc: goal のしきい値 0 の明示", pathAdjustAllocation, goal(bulk(), "thresholdPercent", 0), "invalid_input"},
		{"alloc: goal の未知の技", pathAdjustAllocation, goal(bulk(), "moveId", "test-nothing"), "unknown_move"},
		{"alloc: goal の未知の相手", pathAdjustAllocation, goal(bulk(), "opponent", indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()), "unknown_species"},
		{"alloc: goal の技の分類が offenseCategory と違う", pathAdjustAllocation, goal(offense(), "moveId", moveSpecial), "invalid_input"},
		{"alloc: 自分の未知の種族", pathAdjustAllocation, set(bulk(), "self", indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()), "unknown_species"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, ok := tt.body.(string)
			raw := []byte(body)
			if !ok {
				raw = mustJSON(t, tt.body)
			}
			rec := post(t, h, tt.path, raw, false)
			assertError(t, rec, http.StatusBadRequest, tt.want)
		})
	}
}

// AC-H3: engine.ErrInvalidAdjustInput は invalid_input(wasmapi と同じ語彙。ADR-0250 §4)。
func TestAdjustSentinelMapsToInvalidInput(t *testing.T) {
	got, ok := errFromEngine(engine.ErrInvalidAdjustInput).(*httpError)
	if !ok {
		t.Fatal("errFromEngine の型が *httpError でない")
	}
	if string(got.code) != wasmapi.CodeInvalidInput || got.status != http.StatusBadRequest {
		t.Errorf("ErrInvalidAdjustInput → %s/%d, want invalid_input/400", got.code, got.status)
	}
}

// AC-H3: 必須ヘッダの欠落は missing_header(生成ラッパ経由で登録していること)。
func TestAdjustMissingHeaders(t *testing.T) {
	h := NewHandler(newFakeStore(t), nil)
	for _, path := range adjustPaths {
		t.Run(path, func(t *testing.T) {
			header := validHeaders()
			header.Del("X-Session-Id")
			rec := serve(t, h, http.MethodPost, path, header, []byte(`{}`))
			assertError(t, rec, http.StatusBadRequest, "missing_header")
		})
	}
}

// AC-H4: 値域の検査は ID の解決・engine の呼び出しより前(マスタ参照 0 回)。ID はすべてマスタに無いものを混ぜる。
func TestAdjustRangeChecksRunBeforeStoreLookup(t *testing.T) {
	unknownIndiv := indiv{speciesKey: speciesUnknown, natureID: "test-nothing"}.http()
	ko := adjKOCases()[0].httpBody()
	ko["attacker"], ko["moveId"], ko["hits"] = unknownIndiv, "test-nothing", engine.MaxAdjustHits+1

	survive := adjSurviveCases()[0].httpBody()
	survive["defender"], survive["thresholdPercent"] = unknownIndiv, 0

	ceiling := adjAllocCases()[0].httpBody()
	ceiling["self"], ceiling["ceiling"] = unknownIndiv, map[string]any{"hp": 33}

	minSpeed := adjAllocCases()[2].httpBody()
	minSpeed["self"], minSpeed["minSpeed"] = unknownIndiv, -1

	goalHits := adjAllocCases()[1].httpBody()
	goalHits["self"] = unknownIndiv
	goalHits["goal"].(map[string]any)["hits"] = 0

	modifier := adjIndicesCases()[2].httpBody() // moveId を省略
	modifier["individual"], modifier["modifier"] = unknownIndiv, 0

	tests := []struct {
		name string
		path string
		body map[string]any
	}{
		{"ko: hits 11(未知の個体・技を含む)", pathAdjustMinSPKO, ko},
		{"survive: しきい値 0(未知の個体を含む)", pathAdjustMinSPSurv, survive},
		{"alloc: ceiling 33(未知の個体を含む)", pathAdjustAllocation, ceiling},
		{"alloc: minSpeed -1(未知の個体を含む)", pathAdjustAllocation, minSpeed},
		{"alloc: goal の hits 0(未知の個体を含む)", pathAdjustAllocation, goalHits},
		{"indices: 技なしで modifier 0(未知の個体を含む)", pathAdjustIndices, modifier},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &countingStore{inner: newFakeStore(t)}
			h := NewHandler(store, nil)
			rec := post(t, h, tt.path, mustJSON(t, tt.body), false)
			assertError(t, rec, http.StatusBadRequest, "invalid_input")
			if store.lookups != 0 || store.chartGet != 0 {
				t.Errorf("マスタ参照 %d 回・相性表 %d 回(want 0。値域の検査は ID 解決より前)", store.lookups, store.chartGet)
			}
		})
	}
}

// AC-H2: 値域の端(modifier / damageModifier の 1 と MaxEffectModifier、thresholdPercent の明示 100)は受け付ける。
func TestAdjustHTTPAcceptsBoundaryValues(t *testing.T) {
	h := NewHandler(newFakeStore(t), nil)
	set := func(m map[string]any, kv ...any) map[string]any {
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	withMove := func() map[string]any { return adjIndicesCases()[0].httpBody() }
	noMove := func() map[string]any { return adjIndicesCases()[2].httpBody() }
	tests := []struct {
		name string
		path string
		body map[string]any
	}{
		{"indices: modifier 1", pathAdjustIndices, set(withMove(), "modifier", engine.MinEffectModifier, "damageModifier", engine.MinEffectModifier)},
		{"indices: modifier 2097152", pathAdjustIndices, set(withMove(), "modifier", engine.MaxEffectModifier, "damageModifier", engine.MaxEffectModifier)},
		{"indices: 技なしで modifier 1", pathAdjustIndices, set(noMove(), "modifier", engine.MinEffectModifier, "damageModifier", engine.MinEffectModifier)},
		{"indices: 技なしで modifier 2097152", pathAdjustIndices, set(noMove(), "modifier", engine.MaxEffectModifier, "damageModifier", engine.MaxEffectModifier)},
		{"ko: thresholdPercent 100 の明示", pathAdjustMinSPKO, set(adjKOCases()[0].httpBody(), "thresholdPercent", 100)},
		{"survive: thresholdPercent 100 の明示", pathAdjustMinSPSurv, set(adjSurviveCases()[0].httpBody(), "thresholdPercent", 100)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, tt.path, mustJSON(t, tt.body), true)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200; body=%s", rec.Code, rec.Body)
			}
		})
	}
}

// --- パリティ(HTTP と WASM) -----------------------------------------------------

// genericJSON は JSON を汎用の値にする(HTTP と WASM の応答を「JSON として等しいか」で比べるため)。
func genericJSON(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("JSON でない: %v; %s", err, b)
	}
	return v
}

// AC-H5: 同じ入力は HTTP と engine/wasmapi で JSON として等しい応答(ADR-0250 §6。調整の応答は natureId を持たないので
// 形も同じ)。
func TestAdjustParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	type pair struct {
		name     string
		path     string
		wasm     func(string) string
		httpBody map[string]any
		wasmBody map[string]any
	}
	var pairs []pair
	for _, c := range adjIndicesCases() {
		pairs = append(pairs, pair{"indices: " + c.name, pathAdjustIndices, wasmapi.AdjustIndices, c.httpBody(), c.wasmBody(t, store)})
	}
	for _, c := range adjKOCases() {
		pairs = append(pairs, pair{"ko: " + c.name, pathAdjustMinSPKO, wasmapi.AdjustMinSPToKO, c.httpBody(), c.wasmBody(t, store)})
	}
	for _, c := range adjSurviveCases() {
		pairs = append(pairs, pair{"survive: " + c.name, pathAdjustMinSPSurv, wasmapi.AdjustMinSPToSurvive, c.httpBody(), c.wasmBody(t, store)})
	}
	for _, c := range adjAllocCases() {
		pairs = append(pairs, pair{"alloc: " + c.name, pathAdjustAllocation, wasmapi.AdjustAllocation, c.httpBody(), c.wasmBody(t, store)})
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			rec := post(t, h, p.path, mustJSON(t, p.httpBody), true)
			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d; body=%s", rec.Code, rec.Body)
			}
			wasmOut := wasmResult(t, p.wasm(string(mustJSON(t, p.wasmBody))))
			if got := genericJSON(t, rec.Body.Bytes()); !reflect.DeepEqual(got, wasmOut) {
				t.Errorf("HTTP と WASM の応答が違う\nhttp: %s\nwasm: %v", rec.Body, wasmOut)
			}
		})
	}
}

// AC-H5: 同じ失敗は HTTP と WASM で同じ code。
func TestAdjustErrorCodeParityWithWasm(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store, nil)
	ko, alloc, noMove := adjKOCases()[0], adjAllocCases()[2], adjIndicesCases()[2]
	tests := []struct {
		name   string
		path   string
		wasm   func(string) string
		mutate func(m map[string]any)
		http   map[string]any
		w      map[string]any
		want   string
	}{
		{"hits 11", pathAdjustMinSPKO, wasmapi.AdjustMinSPToKO, func(m map[string]any) { m["hits"] = 11 },
			ko.httpBody(), ko.wasmBody(t, store), wasmapi.CodeInvalidInput},
		{"しきい値 0", pathAdjustMinSPKO, wasmapi.AdjustMinSPToKO, func(m map[string]any) { m["thresholdPercent"] = 0 },
			ko.httpBody(), ko.wasmBody(t, store), wasmapi.CodeInvalidInput},
		{"技なし・modifier 0", pathAdjustIndices, wasmapi.AdjustIndices, func(m map[string]any) { m["modifier"] = 0 },
			noMove.httpBody(), noMove.wasmBody(t, store), wasmapi.CodeInvalidInput},
		{"未知の format", pathAdjustMinSPKO, wasmapi.AdjustMinSPToKO, func(m map[string]any) { m["format"] = "triple" },
			ko.httpBody(), ko.wasmBody(t, store), wasmapi.CodeInvalidEnum},
		{"未知のフィールド", pathAdjustMinSPKO, wasmapi.AdjustMinSPToKO, func(m map[string]any) { m["bogus"] = 1 },
			ko.httpBody(), ko.wasmBody(t, store), wasmapi.CodeUnknownField},
		{"未知の mode", pathAdjustAllocation, wasmapi.AdjustAllocation, func(m map[string]any) { m["mode"] = "balanced" },
			alloc.httpBody(), alloc.wasmBody(t, store), wasmapi.CodeInvalidEnum},
		{"ceiling 33 と未知の mode(値域が先)", pathAdjustAllocation, wasmapi.AdjustAllocation, func(m map[string]any) {
			m["mode"], m["ceiling"] = "balanced", map[string]any{"atk": 33}
		}, alloc.httpBody(), alloc.wasmBody(t, store), wasmapi.CodeInvalidInput},
		{"offenseCategory が status", pathAdjustAllocation, wasmapi.AdjustAllocation, func(m map[string]any) { m["offenseCategory"] = "status" },
			alloc.httpBody(), alloc.wasmBody(t, store), wasmapi.CodeInvalidInput},
		{"bulk で focus の欠落", pathAdjustAllocation, wasmapi.AdjustAllocation, func(m map[string]any) {
			m["mode"] = "bulk"
			delete(m, "offenseCategory")
			delete(m, "minSpeed")
			delete(m, "goal")
		}, alloc.httpBody(), alloc.wasmBody(t, store), wasmapi.CodeInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mutate(tt.http)
			tt.mutate(tt.w)
			rec := post(t, h, tt.path, mustJSON(t, tt.http), false)
			assertError(t, rec, http.StatusBadRequest, tt.want)
			if got := wasmError(t, tt.wasm(string(mustJSON(t, tt.w)))); got != tt.want {
				t.Errorf("WASM の code = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- ステートレス -------------------------------------------------------------------

// AC-H6: 調整は計算イベントを発行しない(ADR-0250 §7)。
func TestAdjustDoesNotPublishEvents(t *testing.T) {
	store := newFakeStore(t)
	pub := &fakePublisher{}
	h := NewHandler(store, pub)
	bodies := map[string]map[string]any{
		pathAdjustIndices:    adjIndicesCases()[0].httpBody(),
		pathAdjustMinSPKO:    adjKOCases()[0].httpBody(),
		pathAdjustMinSPSurv:  adjSurviveCases()[0].httpBody(),
		pathAdjustAllocation: adjAllocCases()[1].httpBody(),
	}
	for path, body := range bodies {
		rec := post(t, h, path, mustJSON(t, body), true)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d; body=%s", path, rec.Code, rec.Body)
		}
	}
	if len(pub.calls) != 0 {
		t.Errorf("計算イベントを %d 件発行した(want 0。ADR-0250 §7)", len(pub.calls))
	}
}

// AC-H6: マスタの準備中は 503 master_unavailable、準備後は 200(ADR-0204 §3。registerDeferredCalcRoutes にも登録する)。
func TestAdjustDeferredHandler(t *testing.T) {
	sw := &switchableStore{store: newFakeStore(t)}
	h := NewDeferredHandler(sw.current, nil)
	bodies := map[string]map[string]any{
		pathAdjustIndices:    adjIndicesCases()[0].httpBody(),
		pathAdjustMinSPKO:    adjKOCases()[0].httpBody(),
		pathAdjustMinSPSurv:  adjSurviveCases()[0].httpBody(),
		pathAdjustAllocation: adjAllocCases()[0].httpBody(),
	}
	for path, body := range bodies {
		rec := post(t, h, path, mustJSON(t, body), true)
		assertError(t, rec, http.StatusServiceUnavailable, "master_unavailable")
	}
	sw.ready.Store(true)
	for path, body := range bodies {
		assertStatusOK(t, post(t, h, path, mustJSON(t, body), true), path)
	}
}

var _ master.Store = (*countingStore)(nil)

// --- 契約 -----------------------------------------------------------------------

// 契約に4操作があり、検証ヘルパーが空振りしない(妥当な本文は通り、必須の欠けた本文は落ちる)。
func TestAdjustContractHelperIsNotVacuous(t *testing.T) {
	const plan = `{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"totalSp":0,` +
		`"stats":{"hp":170,"atk":80,"def":110,"spa":90,"spd":105,"spe":70},"physicalBulk":18700,"specialBulk":17850,` +
		`"speedMet":true,"goalMet":false,"chancePercent":0}`
	tests := []struct {
		name    string
		path    string
		body    string
		wantErr bool
	}{
		{"indices の妥当な結果", pathAdjustIndices, `{"stats":{"hp":170,"atk":80,"def":110,"spa":90,"spd":105,"spe":70},` +
			`"firepowerIndex":null,"physicalBulkIndex":18700,"specialBulkIndex":17850,"hpLines":{"hp":170,"sp":0,"current":"none",` +
			`"next16n":{"hp":176,"sp":6,"spDelta":6},"prev16n":null,"next16nMinus1":{"hp":175,"sp":5,"spDelta":5},"prev16nMinus1":null}}`, false},
		{"indices の hpLines.current が語彙外", pathAdjustIndices, `{"stats":{"hp":170,"atk":80,"def":110,"spa":90,"spd":105,"spe":70},` +
			`"firepowerIndex":null,"physicalBulkIndex":1,"specialBulkIndex":1,"hpLines":{"hp":170,"sp":0,"current":"8n",` +
			`"next16n":null,"prev16n":null,"next16nMinus1":null,"prev16nMinus1":null}}`, true},
		{"ko の妥当な結果", pathAdjustMinSPKO, `{"stat":"atk","searchLimit":32,"feasible":true,"sp":12,"chancePercent":100,"unsupported":[]}`, false},
		{"ko の unsupported 欠落", pathAdjustMinSPKO, `{"stat":"atk","searchLimit":32,"feasible":true,"sp":12,"chancePercent":100}`, true},
		{"survive の妥当な結果", pathAdjustMinSPSurv, `{"stat":"def","searchLimit":66,"feasible":false,"hpSp":32,"statSp":32,` +
			`"totalSp":64,"bulkIndex":40000,"chancePercent":12.5,"unsupported":[]}`, false},
		{"alloc の妥当な結果(minSp null)", pathAdjustAllocation, `{"remaining":66,"maxIndex":` + plan + `,"minSp":null,"unsupported":[]}`, false},
		{"alloc の minSp 欠落", pathAdjustAllocation, `{"remaining":66,"maxIndex":` + plan + `,"unsupported":[]}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgainstContract(t, http.MethodPost, tt.path, validHeaders(), []byte(`{}`),
				http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, []byte(tt.body), false)
			if err == errNoRoute {
				t.Fatalf("契約に %s が無い", tt.path)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("検証エラー = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- ベンチマーク(上限内の最大の仕事量。壁時計の閾値は判定しない。ADR-0250 §5) -----------

func BenchmarkAdjustAllocationAtLimit(b *testing.B) {
	store := newFakeStore(b)
	h := NewHandler(store, nil)
	c := adjAllocCase{
		self: indiv{speciesKey: speciesDefender, natureID: natureNeutral}, mode: engine.AllocModeBulk, focus: engine.BulkFocusBoth,
		goal: &adjAllocGoalCase{opponent: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{Atk: 32}},
			moveID: movePhysical, hits: engine.MaxAdjustHits, threshold: half()},
	}
	body := mustJSON(b, c.httpBody())
	header := validHeaders()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := serve(b, h, http.MethodPost, pathAdjustAllocation, header, body)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
	}
}
