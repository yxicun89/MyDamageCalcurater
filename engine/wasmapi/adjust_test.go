package wasmapi_test

// 調整(plan.md AJ4)の WASM 境界の受け入れテスト。契約は ADR-0250(engine の定義は ADR-0150)。
//
//   - AC-A1 素通し: 4関数の結果は engine(RealStats・FirepowerIndex・BulkIndex・HPLines・MinSPToKO・MinSPToSurvive・
//     SuggestSPAllocation)に同じ入力を直接渡した結果の写し。
//   - AC-A2 既定: 省略した modifier / damageModifier は 4096、thresholdPercent は 100、ceiling の各能力は 32、
//     moveId(move)の省略は firepowerIndex=null、goal の省略は minSp=null・unsupported=[]。
//   - AC-A3 探索する能力の SP は無視する(境界で個体を先に Validate しない)。
//   - AC-A4 エラー: 値域違反は invalid_input、未知の列挙は invalid_enum、engine.ErrInvalidAdjustInput は invalid_input。
//     値域(hits・thresholdPercent・ceiling・minSpeed・modifier・damageModifier)の検査は DTO の変換より前(ADR-0108 決定3)。
//
// 期待値はすべて engine を直接呼んで作る(手計算しない)。

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
)

// --- 架空の個体・技 ----------------------------------------------------------

func adjSpecies(key string, types []engine.Type, base engine.Stats) engine.Species {
	return engine.Species{Key: key, Types: types, BaseStats: base}
}

var (
	adjDragon = adjSpecies("testdragon", []engine.Type{engine.TypeDragon, engine.TypeGround},
		engine.Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102})
	adjWall = adjSpecies("testwall", []engine.Type{engine.TypeNormal},
		engine.Stats{HP: 160, Atk: 110, Def: 65, SpA: 65, SpD: 110, Spe: 30})
	adjFire = adjSpecies("testfire", []engine.Type{engine.TypeFire},
		engine.Stats{HP: 75, Atk: 95, Def: 67, SpA: 125, SpD: 95, Spe: 83})
	adjGhost = adjSpecies("testghost", []engine.Type{engine.TypeGhost},
		engine.Stats{HP: 60, Atk: 65, Def: 60, SpA: 130, SpD: 75, Spe: 110})

	adjQuake = engine.Move{ID: "testquake", NameJa: "テストじしん", Type: engine.TypeGround, Category: engine.CategoryPhysical, Power: 100}
	adjSlam  = engine.Move{ID: "testslam", NameJa: "テストのしかかり", Type: engine.TypeNormal, Category: engine.CategoryPhysical, Power: 85}
	adjFlame = engine.Move{ID: "testflame", NameJa: "テストかえん", Type: engine.TypeFire, Category: engine.CategorySpecial, Power: 90}
	adjGlare = engine.Move{ID: "testglare", NameJa: "テストにらみ", Type: engine.TypeNormal, Category: engine.CategoryStatus}
)

func adjIndiv(sp engine.Species, nature engine.Nature, s engine.Stats) engine.Individual {
	return engine.Individual{Species: sp, Level: engine.DefaultLevel, Nature: nature, SP: s, Status: engine.StatusNone}
}

var (
	natureAtkUp = engine.Nature{Plus: engine.StatAtk, Minus: engine.StatSpA}
	natureSpAUp = engine.Nature{Plus: engine.StatSpA, Minus: engine.StatAtk}
	natureSpeDn = engine.Nature{Plus: engine.StatAtk, Minus: engine.StatSpe}
)

// --- engine の値 → DTO(JSON の map)---------------------------------------

func adjStatsMap(s engine.Stats) map[string]any {
	return stats(s.HP, s.Atk, s.Def, s.SpA, s.SpD, s.Spe)
}

func adjIndivDTO(in engine.Individual) map[string]any {
	types := make([]any, 0, len(in.Species.Types))
	for _, ty := range in.Species.Types {
		types = append(types, string(ty))
	}
	return map[string]any{
		"species": map[string]any{"key": in.Species.Key, "types": types, "baseStats": adjStatsMap(in.Species.BaseStats)},
		"level":   engine.DefaultLevel,
		"nature":  map[string]any{"plus": string(in.Nature.Plus), "minus": string(in.Nature.Minus)},
		"sp":      adjStatsMap(in.SP),
		"status":  "none",
	}
}

func adjMoveDTO(m engine.Move) map[string]any {
	out := map[string]any{
		"id": m.ID, "nameJa": m.NameJa, "type": string(m.Type), "category": string(m.Category), "power": m.Power, "priority": m.Priority,
	}
	if len(m.Mechanisms) > 0 {
		ms := make([]any, 0, len(m.Mechanisms))
		for _, x := range m.Mechanisms {
			ms = append(ms, string(x))
		}
		out["mechanisms"] = ms
	}
	return out
}

func adjField() map[string]any { return map[string]any{"weather": "none", "terrain": "none"} }

// --- 応答の型(= 境界の契約。ADR-0250 §6。未知のキーは decodeEnvelope が拒否する)------

type hpLinePointView struct {
	HP      int `json:"hp"`
	SP      int `json:"sp"`
	SPDelta int `json:"spDelta"`
}

type hpLinesView struct {
	HP            int              `json:"hp"`
	SP            int              `json:"sp"`
	Current       string           `json:"current"`
	Next16n       *hpLinePointView `json:"next16n"`
	Prev16n       *hpLinePointView `json:"prev16n"`
	Next16nMinus1 *hpLinePointView `json:"next16nMinus1"`
	Prev16nMinus1 *hpLinePointView `json:"prev16nMinus1"`
}

type indicesView struct {
	Stats             statsView   `json:"stats"`
	FirepowerIndex    *int64      `json:"firepowerIndex"`
	PhysicalBulkIndex int64       `json:"physicalBulkIndex"`
	SpecialBulkIndex  int64       `json:"specialBulkIndex"`
	HPLines           hpLinesView `json:"hpLines"`
}

type koSearchView struct {
	Stat          string     `json:"stat"`
	SearchLimit   int        `json:"searchLimit"`
	Feasible      bool       `json:"feasible"`
	SP            int        `json:"sp"`
	ChancePercent float64    `json:"chancePercent"`
	Unsupported   []markView `json:"unsupported"`
}

type surviveSearchView struct {
	Stat          string     `json:"stat"`
	SearchLimit   int        `json:"searchLimit"`
	Feasible      bool       `json:"feasible"`
	HPSP          int        `json:"hpSp"`
	StatSP        int        `json:"statSp"`
	TotalSP       int        `json:"totalSp"`
	BulkIndex     int64      `json:"bulkIndex"`
	ChancePercent float64    `json:"chancePercent"`
	Unsupported   []markView `json:"unsupported"`
}

type allocPlanView struct {
	SP            statsView `json:"sp"`
	TotalSP       int       `json:"totalSp"`
	Stats         statsView `json:"stats"`
	PhysicalBulk  int64     `json:"physicalBulk"`
	SpecialBulk   int64     `json:"specialBulk"`
	SpeedMet      bool      `json:"speedMet"`
	GoalMet       bool      `json:"goalMet"`
	ChancePercent float64   `json:"chancePercent"`
}

type allocView struct {
	Remaining   int            `json:"remaining"`
	MaxIndex    allocPlanView  `json:"maxIndex"`
	MinSP       *allocPlanView `json:"minSp"`
	Unsupported []markView     `json:"unsupported"`
}

func hpLineKindView(k engine.HPLineKind) string {
	if k == engine.HPLineNone {
		return "none"
	}
	return string(k)
}

func hpPointView(p *engine.HPLinePoint) *hpLinePointView {
	if p == nil {
		return nil
	}
	return &hpLinePointView{HP: p.HP, SP: p.SP, SPDelta: p.SPDelta}
}

func hpLinesOf(r engine.HPLineReport) hpLinesView {
	return hpLinesView{
		HP: r.HP, SP: r.SP, Current: hpLineKindView(r.Current),
		Next16n: hpPointView(r.Next16n), Prev16n: hpPointView(r.Prev16n),
		Next16nMinus1: hpPointView(r.Next16nMinus1), Prev16nMinus1: hpPointView(r.Prev16nMinus1),
	}
}

func marksOf(ms []engine.UnsupportedMark) []markView {
	out := make([]markView, 0, len(ms))
	for _, m := range ms {
		out = append(out, markView{Target: string(m.Target), Reason: string(m.Reason), ID: m.ID})
	}
	return out
}

func planOf(p engine.AllocPlan) allocPlanView {
	return allocPlanView{
		SP: statsOf(p.SP), TotalSP: p.TotalSP, Stats: statsOf(p.Real),
		PhysicalBulk: int64(p.PhysicalBulk), SpecialBulk: int64(p.SpecialBulk),
		SpeedMet: p.SpeedMet, GoalMet: p.GoalMet, ChancePercent: p.ChancePercent,
	}
}

// assertRawHas は応答の生の JSON に want(キーと値の断片)が含まれることを確かめる
// (null・空配列が「キーごと省略」になっていないことを見るため)。
func assertRawHas(t *testing.T, resp, want string) {
	t.Helper()
	if !strings.Contains(resp, want) {
		t.Errorf("応答に %s が無い(キーを省略せず明示すること。ADR-0250 §6): %s", want, resp)
	}
}

// --- AC-A1 / AC-A2: adjustIndices ------------------------------------------

func TestAdjustIndicesMatchesEngine(t *testing.T) {
	cases := []struct {
		name           string
		self           engine.Individual
		move           *engine.Move
		modifier       *int // nil は省略
		damageModifier *int // nil は省略
	}{
		{"物理技・補正の省略は 4096", adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 20, Atk: 32, Spe: 14}), &adjQuake, nil, nil},
		{"特殊技・補正を明示", adjIndiv(adjFire, natureSpAUp, engine.Stats{HP: 4, SpA: 32, Spe: 30}), &adjFlame, ptr(6144), ptr(2048)},
		{"技なし(firepowerIndex は null)", adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 17, Def: 32, SpD: 17}), nil, nil, ptr(3072)},
		{"HP が 16n の SP(HP 種族値 75 + 75 + SP 10 = 160)", adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{HP: 10}), &adjFlame, nil, nil},
		{"HP の SP が 32(次のラインは無い)", adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 32}), nil, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := map[string]any{"individual": adjIndivDTO(c.self)}
			if c.move != nil {
				req["move"] = adjMoveDTO(*c.move)
			}
			if c.modifier != nil {
				req["modifier"] = *c.modifier
			}
			if c.damageModifier != nil {
				req["damageModifier"] = *c.damageModifier
			}
			resp := invoke(t, "adjustIndices", mustJSON(t, req))
			var got indicesView
			decodeEnvelope(t, resp, &got)

			mod, dmg := engine.Modifier4096, engine.Modifier4096
			if c.modifier != nil {
				mod = *c.modifier
			}
			if c.damageModifier != nil {
				dmg = *c.damageModifier
			}
			if want := statsOf(engine.RealStats(c.self)); got.Stats != want {
				t.Errorf("stats = %+v, want %+v(engine.RealStats)", got.Stats, want)
			}
			if c.move == nil {
				assertRawHas(t, resp, `"firepowerIndex":null`)
				if got.FirepowerIndex != nil {
					t.Errorf("firepowerIndex = %d, want null(move を省略)", *got.FirepowerIndex)
				}
			} else {
				want, err := engine.FirepowerIndex(c.self, c.move.Category, c.move.Power, mod)
				if err != nil {
					t.Fatalf("engine.FirepowerIndex = %v", err)
				}
				if got.FirepowerIndex == nil || *got.FirepowerIndex != int64(want) {
					t.Errorf("firepowerIndex = %v, want %d", got.FirepowerIndex, want)
				}
			}
			for _, b := range []struct {
				cat  engine.MoveCategory
				got  int64
				name string
			}{
				{engine.CategoryPhysical, got.PhysicalBulkIndex, "physicalBulkIndex"},
				{engine.CategorySpecial, got.SpecialBulkIndex, "specialBulkIndex"},
			} {
				want, err := engine.BulkIndex(c.self, b.cat, dmg)
				if err != nil {
					t.Fatalf("engine.BulkIndex = %v", err)
				}
				if b.got != int64(want) {
					t.Errorf("%s = %d, want %d", b.name, b.got, want)
				}
			}
			wantLines, err := engine.HPLines(c.self.Species.BaseStats.HP, c.self.SP.HP)
			if err != nil {
				t.Fatalf("engine.HPLines = %v", err)
			}
			if want := hpLinesOf(wantLines); !reflect.DeepEqual(got.HPLines, want) {
				t.Errorf("hpLines = %+v, want %+v", got.HPLines, want)
			}
			// 次・前のラインが無いときも null で明示する(キーを省略しない)。
			for _, p := range []struct {
				key string
				v   *engine.HPLinePoint
			}{
				{"next16n", wantLines.Next16n}, {"prev16n", wantLines.Prev16n},
				{"next16nMinus1", wantLines.Next16nMinus1}, {"prev16nMinus1", wantLines.Prev16nMinus1},
			} {
				if p.v == nil {
					assertRawHas(t, resp, `"`+p.key+`":null`)
				}
			}
		})
	}
}

// AC-A2: 補正の省略と 4096 の明示は同じ応答(バイト単位)。
func TestAdjustIndicesModifierOmittedEquals4096(t *testing.T) {
	self := adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 20, Atk: 32, Spe: 14})
	omitted := map[string]any{"individual": adjIndivDTO(self), "move": adjMoveDTO(adjQuake)}
	explicit := map[string]any{"individual": adjIndivDTO(self), "move": adjMoveDTO(adjQuake), "modifier": 4096, "damageModifier": 4096}
	a := invoke(t, "adjustIndices", mustJSON(t, omitted))
	b := invoke(t, "adjustIndices", mustJSON(t, explicit))
	var v indicesView
	decodeEnvelope(t, a, &v)
	if a != b {
		t.Errorf("補正の省略と 4096 の明示で応答が違う\nomitted : %s\nexplicit: %s", a, b)
	}
}

// --- AC-A1 / AC-A2: adjustMinSpToKo / adjustMinSpToSurvive ------------------

type adjSearchCase struct {
	name      string
	attacker  engine.Individual
	defender  engine.Individual
	move      engine.Move
	hits      int
	threshold *float64 // nil は省略(= 100)
}

func (c adjSearchCase) request(t *testing.T) map[string]any {
	t.Helper()
	req := map[string]any{
		"format":    "single",
		"attacker":  adjIndivDTO(c.attacker),
		"defender":  adjIndivDTO(c.defender),
		"move":      adjMoveDTO(c.move),
		"field":     adjField(),
		"critical":  false,
		"hits":      c.hits,
		"typeChart": typeChartRequestValue(),
	}
	if c.threshold != nil {
		req["thresholdPercent"] = *c.threshold
	}
	return req
}

func (c adjSearchCase) engineInput(t *testing.T) engine.AdjustSearchInput {
	t.Helper()
	in := engine.AdjustSearchInput{
		Format: engine.FormatSingle, Attacker: c.attacker, Defender: c.defender, Move: c.move,
		Field: engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone}, TypeChart: sharedTypeChart(t),
		Hits: c.hits,
	}
	if c.threshold != nil {
		in.ThresholdPercent = *c.threshold
	}
	return in
}

func koSearchCases() []adjSearchCase {
	multiHit := adjSlam
	multiHit.Mechanisms = []engine.MoveMechanism{engine.MechanismMultiHit}
	return []adjSearchCase{
		{"しきい値の省略は確定(100)", adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 32, Spe: 32}),
			adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{HP: 32}), adjQuake, 1, nil},
		{"しきい値 50%・3発", adjIndiv(adjWall, natureAtkUp, engine.Stats{HP: 32}),
			adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 32, Def: 32}), adjSlam, 3, ptrF(50)},
		{"特殊技(C を探索)", adjIndiv(adjFire, natureSpAUp, engine.Stats{Spe: 32}),
			adjIndiv(adjDragon, engine.NatureNeutral, engine.Stats{}), adjFlame, 2, nil},
		{"届かない(feasible=false・上限の確率)", adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{}),
			adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 32, Def: 32}), adjSlam, 1, nil},
		{"無効相性はエラーにしない", adjIndiv(adjWall, natureAtkUp, engine.Stats{}),
			adjIndiv(adjGhost, engine.NatureNeutral, engine.Stats{}), adjSlam, 1, nil},
		{"固定 SP の合計が 66(上限 0)", adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 32, Def: 2, Spe: 32}),
			adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{}), adjQuake, 2, nil},
		{"未対応の印(技の機構)", adjIndiv(adjWall, natureAtkUp, engine.Stats{}),
			adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{}), multiHit, 2, nil},
	}
}

func TestAdjustMinSPToKOMatchesEngine(t *testing.T) {
	for _, c := range koSearchCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.MinSPToKO(c.engineInput(t))
			if err != nil {
				t.Fatalf("engine.MinSPToKO = %v", err)
			}
			resp := invoke(t, "adjustMinSpToKo", mustJSON(t, c.request(t)))
			var got koSearchView
			decodeEnvelope(t, resp, &got)
			assertRawHas(t, resp, `"unsupported":[`)
			wantView := koSearchView{
				Stat: string(want.Stat), SearchLimit: want.SearchLimit, Feasible: want.Feasible, SP: want.SP,
				ChancePercent: want.ChancePercent, Unsupported: marksOf(want.Unsupported),
			}
			if !reflect.DeepEqual(got, wantView) {
				t.Errorf("結果が engine と違う\n got %+v\nwant %+v", got, wantView)
			}
		})
	}
}

func surviveSearchCases() []adjSearchCase {
	return []adjSearchCase{
		{"物理・2発・確定", adjIndiv(adjDragon, natureAtkUp, engine.Stats{Atk: 32, Spe: 32}),
			adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{Atk: 32}), adjQuake, 2, nil},
		{"特殊・1発・しきい値 75%", adjIndiv(adjFire, natureSpAUp, engine.Stats{SpA: 32, Spe: 32}),
			adjIndiv(adjDragon, engine.NatureNeutral, engine.Stats{Atk: 32, Spe: 2}), adjFlame, 1, ptrF(75)},
		{"届かない(最も耐える組)", adjIndiv(adjDragon, natureAtkUp, engine.Stats{Atk: 32}),
			adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{SpA: 32, Spe: 32}), adjQuake, 3, nil},
		{"無効相性は (0, 0) で 100%", adjIndiv(adjWall, natureAtkUp, engine.Stats{Atk: 32}),
			adjIndiv(adjGhost, engine.NatureNeutral, engine.Stats{}), adjSlam, 10, nil},
	}
}

func TestAdjustMinSPToSurviveMatchesEngine(t *testing.T) {
	for _, c := range surviveSearchCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.MinSPToSurvive(c.engineInput(t))
			if err != nil {
				t.Fatalf("engine.MinSPToSurvive = %v", err)
			}
			resp := invoke(t, "adjustMinSpToSurvive", mustJSON(t, c.request(t)))
			var got surviveSearchView
			decodeEnvelope(t, resp, &got)
			assertRawHas(t, resp, `"unsupported":[`)
			wantView := surviveSearchView{
				Stat: string(want.Stat), SearchLimit: want.SearchLimit, Feasible: want.Feasible,
				HPSP: want.HPSP, StatSP: want.StatSP, TotalSP: want.TotalSP, BulkIndex: int64(want.BulkIndex),
				ChancePercent: want.ChancePercent, Unsupported: marksOf(want.Unsupported),
			}
			if !reflect.DeepEqual(got, wantView) {
				t.Errorf("結果が engine と違う\n got %+v\nwant %+v", got, wantView)
			}
		})
	}
}

// AC-A2: しきい値の省略と 100 の明示は同じ応答。
func TestAdjustSearchThresholdOmittedEquals100(t *testing.T) {
	for _, fn := range []string{"adjustMinSpToKo", "adjustMinSpToSurvive"} {
		t.Run(fn, func(t *testing.T) {
			c := koSearchCases()[0]
			omitted := c.request(t)
			explicit := c.request(t)
			explicit["thresholdPercent"] = 100
			a, b := invoke(t, fn, mustJSON(t, omitted)), invoke(t, fn, mustJSON(t, explicit))
			var env map[string]json.RawMessage
			if err := json.Unmarshal([]byte(a), &env); err != nil || env["result"] == nil {
				t.Fatalf("成功するはず: %s", a)
			}
			if a != b {
				t.Errorf("しきい値の省略と 100 で応答が違う\nomitted : %s\nexplicit: %s", a, b)
			}
		})
	}
}

// AC-A3: 探索する能力の SP は無視する。境界が個体を先に Validate すると合計 66 超で落ちてしまう。
func TestAdjustSearchIgnoresSearchedStatSP(t *testing.T) {
	t.Run("ko: attacker.sp.atk は無視", func(t *testing.T) {
		c := koSearchCases()[5] // 固定 SP(atk 以外)の合計がちょうど 66
		base := invoke(t, "adjustMinSpToKo", mustJSON(t, c.request(t)))
		c.attacker.SP.Atk = 32 // 合計 98 だが atk は探索で上書きされる
		got := invoke(t, "adjustMinSpToKo", mustJSON(t, c.request(t)))
		var v koSearchView
		decodeEnvelope(t, got, &v)
		if got != base {
			t.Errorf("探索する能力の SP で応答が変わった\nbase: %s\ngot : %s", base, got)
		}
	})
	t.Run("survive: defender.sp.hp と def は無視", func(t *testing.T) {
		c := surviveSearchCases()[0]
		c.defender.SP = engine.Stats{Atk: 32, SpA: 32, Spe: 2} // hp・def 以外で 66
		base := invoke(t, "adjustMinSpToSurvive", mustJSON(t, c.request(t)))
		c.defender.SP.HP, c.defender.SP.Def = 32, 32
		got := invoke(t, "adjustMinSpToSurvive", mustJSON(t, c.request(t)))
		var v surviveSearchView
		decodeEnvelope(t, got, &v)
		if got != base {
			t.Errorf("探索する能力の SP で応答が変わった\nbase: %s\ngot : %s", base, got)
		}
	})
}

// --- AC-A1 / AC-A2: adjustAllocation ----------------------------------------

type adjAllocCase struct {
	name     string
	self     engine.Individual
	ceiling  map[string]any // nil は省略。値は各能力の上限(省略した能力は 32)
	mode     engine.AllocMode
	focus    engine.BulkFocus
	category engine.MoveCategory
	minSpeed int
	goal     *engine.AllocGoal // TypeChart は engineInput が入れる
}

func (c adjAllocCase) request(t *testing.T) map[string]any {
	t.Helper()
	req := map[string]any{
		"self":      adjIndivDTO(c.self),
		"mode":      string(c.mode),
		"typeChart": typeChartRequestValue(),
	}
	if c.ceiling != nil {
		req["ceiling"] = c.ceiling
	}
	if c.focus != "" {
		req["focus"] = string(c.focus)
	}
	if c.category != "" {
		req["offenseCategory"] = string(c.category)
	}
	if c.minSpeed != 0 {
		req["minSpeed"] = c.minSpeed
	}
	if g := c.goal; g != nil {
		goal := map[string]any{
			"format": "single", "opponent": adjIndivDTO(g.Opponent), "move": adjMoveDTO(g.Move),
			"field": adjField(), "critical": g.Critical, "hits": g.Hits,
		}
		if g.ThresholdPercent != 0 {
			goal["thresholdPercent"] = g.ThresholdPercent
		}
		req["goal"] = goal
	}
	return req
}

// engineCeiling は ceiling の map を engine の Stats にする。省略した能力は 32(ADR-0250 §3)。
func (c adjAllocCase) engineCeiling() engine.Stats {
	out := engine.Stats{HP: 32, Atk: 32, Def: 32, SpA: 32, SpD: 32, Spe: 32}
	for k, v := range c.ceiling {
		out = out.WithStat(engine.StatKey(k), v.(int))
	}
	return out
}

func (c adjAllocCase) engineInput(t *testing.T) engine.SPAllocInput {
	t.Helper()
	in := engine.SPAllocInput{
		Self: c.self, Ceiling: c.engineCeiling(), Mode: c.mode, Focus: c.focus,
		OffenseCategory: c.category, MinSpeed: c.minSpeed,
	}
	if c.goal != nil {
		g := *c.goal
		g.Format = engine.FormatSingle
		g.Field = engine.Field{Weather: engine.WeatherNone, Terrain: engine.TerrainNone}
		g.TypeChart = sharedTypeChart(t)
		in.Goal = &g
	}
	return in
}

func allocCases() []adjAllocCase {
	foe := adjIndiv(adjDragon, natureAtkUp, engine.Stats{Atk: 32, Spe: 32})
	return []adjAllocCase{
		{name: "耐久側・both・目標なし・ceiling 省略", self: adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{Atk: 32}),
			mode: engine.AllocModeBulk, focus: engine.BulkFocusBoth},
		{name: "耐久側・physical・目標あり・H の上限 28", self: adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{Atk: 32}),
			ceiling: map[string]any{"hp": 28}, mode: engine.AllocModeBulk, focus: engine.BulkFocusPhysical,
			goal: &engine.AllocGoal{Opponent: foe, Move: adjQuake, Hits: 2}},
		{name: "耐久側・D の上限 0 は「振らない」", self: adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{}),
			ceiling: map[string]any{"spd": 0}, mode: engine.AllocModeBulk, focus: engine.BulkFocusBoth},
		{name: "攻撃側・素早さ目標・しきい値 50%", self: adjIndiv(adjDragon, natureAtkUp, engine.Stats{}),
			mode: engine.AllocModeOffense, category: engine.CategoryPhysical, minSpeed: 150,
			goal: &engine.AllocGoal{Opponent: adjIndiv(adjFire, engine.NatureNeutral, engine.Stats{HP: 32}), Move: adjQuake, Hits: 1, ThresholdPercent: 50}},
		{name: "攻撃側・性格の下降補正(S)・届かない素早さ", self: adjIndiv(adjDragon, natureSpeDn, engine.Stats{HP: 2}),
			mode: engine.AllocModeOffense, category: engine.CategoryPhysical, minSpeed: 400},
		{name: "下限の合計が 66(残り 0)", self: adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 32, Atk: 32, Spe: 2}),
			mode: engine.AllocModeBulk, focus: engine.BulkFocusSpecial,
			goal: &engine.AllocGoal{Opponent: foe, Move: adjQuake, Hits: 1}},
	}
}

func TestAdjustAllocationMatchesEngine(t *testing.T) {
	for _, c := range allocCases() {
		t.Run(c.name, func(t *testing.T) {
			want, err := engine.SuggestSPAllocation(c.engineInput(t))
			if err != nil {
				t.Fatalf("engine.SuggestSPAllocation = %v", err)
			}
			resp := invoke(t, "adjustAllocation", mustJSON(t, c.request(t)))
			var got allocView
			decodeEnvelope(t, resp, &got)
			assertRawHas(t, resp, `"unsupported":[`)
			wantView := allocView{Remaining: want.Remaining, MaxIndex: planOf(want.MaxIndex), Unsupported: marksOf(want.Unsupported)}
			if want.MinSP != nil {
				p := planOf(*want.MinSP)
				wantView.MinSP = &p
			} else {
				assertRawHas(t, resp, `"minSp":null`)
			}
			if !reflect.DeepEqual(got, wantView) {
				t.Errorf("結果が engine と違う\n got %+v\nwant %+v", got, wantView)
			}
		})
	}
}

// AC-A2: ceiling の省略と全能力 32 の明示は同じ応答。ゼロ値(振らない)をそのまま engine に渡していないこと。
func TestAdjustAllocationCeilingOmittedEquals32(t *testing.T) {
	c := allocCases()[0]
	omitted := c.request(t)
	explicit := c.request(t)
	explicit["ceiling"] = map[string]any{"hp": 32, "atk": 32, "def": 32, "spa": 32, "spd": 32, "spe": 32}
	partial := c.request(t)
	partial["ceiling"] = map[string]any{}
	a := invoke(t, "adjustAllocation", mustJSON(t, omitted))
	var v allocView
	decodeEnvelope(t, a, &v)
	if v.MaxIndex.TotalSP == c.self.SP.Sum() {
		t.Errorf("ceiling の省略で残り SP が1つも振られていない(ゼロ値を「振らない」のまま渡している): %s", a)
	}
	for name, req := range map[string]map[string]any{"全能力 32": explicit, "空オブジェクト": partial} {
		if b := invoke(t, "adjustAllocation", mustJSON(t, req)); a != b {
			t.Errorf("ceiling の省略と %s で応答が違う\nomitted: %s\n%s: %s", name, a, name, b)
		}
	}
}

// AC-A2: goal を省略したら typeChart は読まない(省略してよい)。
func TestAdjustAllocationWithoutGoalDoesNotNeedTypeChart(t *testing.T) {
	c := allocCases()[0]
	with := c.request(t)
	without := c.request(t)
	delete(without, "typeChart")
	a, b := invoke(t, "adjustAllocation", mustJSON(t, with)), invoke(t, "adjustAllocation", mustJSON(t, without))
	var v allocView
	decodeEnvelope(t, b, &v)
	if a != b {
		t.Errorf("goal なしで typeChart の有無により応答が違う\nwith   : %s\nwithout: %s", a, b)
	}
}

// --- 成功封筒のキー ------------------------------------------------------------

func TestAdjustSuccessEnvelopeShape(t *testing.T) {
	cases := []struct {
		fn   string
		req  map[string]any
		keys []string
	}{
		{"adjustIndices", map[string]any{"individual": adjIndivDTO(adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{}))},
			[]string{"stats", "firepowerIndex", "physicalBulkIndex", "specialBulkIndex", "hpLines"}},
		{"adjustMinSpToKo", koSearchCases()[0].request(t),
			[]string{"stat", "searchLimit", "feasible", "sp", "chancePercent", "unsupported"}},
		{"adjustMinSpToSurvive", surviveSearchCases()[0].request(t),
			[]string{"stat", "searchLimit", "feasible", "hpSp", "statSp", "totalSp", "bulkIndex", "chancePercent", "unsupported"}},
		{"adjustAllocation", allocCases()[0].request(t), []string{"remaining", "maxIndex", "minSp", "unsupported"}},
	}
	for _, c := range cases {
		t.Run(c.fn, func(t *testing.T) {
			resp := invoke(t, c.fn, mustJSON(t, c.req))
			if strings.HasSuffix(resp, "\n") {
				t.Error("レスポンスが改行で終わっている(Go/WASM のバイト一致比較が壊れる)")
			}
			var env map[string]json.RawMessage
			if err := json.Unmarshal([]byte(resp), &env); err != nil {
				t.Fatalf("JSON でない: %v\n%s", err, resp)
			}
			if _, bad := env["error"]; bad || len(env) != 1 {
				t.Fatalf("成功封筒(result だけ)になっていない: %s", resp)
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(env["result"], &result); err != nil {
				t.Fatalf("result が object でない: %v", err)
			}
			if got := keysOf(result); !sameSet(got, c.keys) {
				t.Errorf("result のキー集合が契約と違う\n got %v\nwant %v", got, c.keys)
			}
		})
	}
}

// --- AC-A4: エラー ----------------------------------------------------------

func adjErrorCode(t *testing.T, resp string) string {
	t.Helper()
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *errorView      `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("JSON でない: %v\n%s", err, resp)
	}
	if env.Error == nil {
		t.Fatalf("失敗するはずが成功した: %s", resp)
	}
	if env.Error.Message == "" {
		t.Errorf("message が空: %s", resp)
	}
	return env.Error.Code
}

func TestAdjustInvalidInput(t *testing.T) {
	indices := func() map[string]any {
		return map[string]any{"individual": adjIndivDTO(adjIndiv(adjDragon, natureAtkUp, engine.Stats{Atk: 32})), "move": adjMoveDTO(adjQuake)}
	}
	ko := func() map[string]any { return koSearchCases()[0].request(t) }
	survive := func() map[string]any { return surviveSearchCases()[0].request(t) }
	bulk := func() map[string]any { return allocCases()[1].request(t) }
	offense := func() map[string]any { return allocCases()[3].request(t) }
	set := func(m map[string]any, kv ...any) map[string]any {
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	del := func(m map[string]any, keys ...string) map[string]any {
		for _, k := range keys {
			delete(m, k)
		}
		return m
	}
	goalOf := func(m map[string]any) map[string]any { return sub(t, m, "goal") }
	noMove := func() map[string]any { return del(indices(), "move") } // move を省略

	tests := []struct {
		name string
		fn   string
		req  any // map または生の JSON 文字列
		want string
	}{
		// indices
		{"indices: modifier 0", "adjustIndices", set(indices(), "modifier", 0), CodeInvalidInputForTest},
		{"indices: modifier が上限超え", "adjustIndices", set(indices(), "modifier", engine.MaxEffectModifier+1), CodeInvalidInputForTest},
		{"indices: damageModifier が負", "adjustIndices", set(indices(), "damageModifier", -1), CodeInvalidInputForTest},
		// move を省略しても modifier の値域は検査する(契約 AdjustModifier・ADR-0250 §3・§5)。
		{"indices: 技なしで modifier 0", "adjustIndices", set(noMove(), "modifier", 0), CodeInvalidInputForTest},
		{"indices: 技なしで modifier が上限超え", "adjustIndices", set(noMove(), "modifier", engine.MaxEffectModifier+1), CodeInvalidInputForTest},
		{"indices: 技なしで damageModifier 0", "adjustIndices", set(noMove(), "damageModifier", 0), CodeInvalidInputForTest},
		{"indices: 変化技", "adjustIndices", set(indices(), "move", adjMoveDTO(adjGlare)), CodeInvalidInputForTest},
		{"indices: SP の合計 66 超", "adjustIndices", set(indices(), "individual",
			adjIndivDTO(adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 32, Atk: 32, Spe: 32}))), CodeInvalidInputForTest},
		{"indices: typeChart は契約に無い", "adjustIndices", set(indices(), "typeChart", typeChartRequestValue()), "unknown_field"},
		{"indices: 契約に無いフィールド", "adjustIndices", set(indices(), "power", 100), "unknown_field"},
		// 探索(ko / survive 共通)
		{"ko: hits 0", "adjustMinSpToKo", set(ko(), "hits", 0), CodeInvalidInputForTest},
		{"ko: hits の欠落", "adjustMinSpToKo", del(ko(), "hits"), CodeInvalidInputForTest},
		{"ko: hits 11", "adjustMinSpToKo", set(ko(), "hits", engine.MaxAdjustHits+1), CodeInvalidInputForTest},
		{"ko: hits が小数", "adjustMinSpToKo", set(ko(), "hits", 1.5), "invalid_json"},
		{"ko: しきい値 0 の明示", "adjustMinSpToKo", set(ko(), "thresholdPercent", 0), CodeInvalidInputForTest},
		{"ko: しきい値が負", "adjustMinSpToKo", set(ko(), "thresholdPercent", -5), CodeInvalidInputForTest},
		{"ko: しきい値 100 超", "adjustMinSpToKo", set(ko(), "thresholdPercent", 100.1), CodeInvalidInputForTest},
		{"ko: 変化技", "adjustMinSpToKo", set(ko(), "move", adjMoveDTO(adjGlare)), CodeInvalidInputForTest},
		{"ko: 固定 SP(atk 以外)の合計 66 超", "adjustMinSpToKo", set(ko(), "attacker",
			adjIndivDTO(adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 32, Def: 32, Spe: 32}))), CodeInvalidInputForTest},
		{"ko: typeChart の欠落", "adjustMinSpToKo", del(ko(), "typeChart"), "type_chart_missing"},
		{"ko: 未知の format", "adjustMinSpToKo", set(ko(), "format", "triple"), "invalid_enum"},
		{"survive: hits 11", "adjustMinSpToSurvive", set(survive(), "hits", engine.MaxAdjustHits+1), CodeInvalidInputForTest},
		{"survive: 変化技", "adjustMinSpToSurvive", set(survive(), "move", adjMoveDTO(adjGlare)), CodeInvalidInputForTest},
		{"survive: 相手の SP の合計 66 超", "adjustMinSpToSurvive", set(survive(), "attacker",
			adjIndivDTO(adjIndiv(adjDragon, natureAtkUp, engine.Stats{HP: 32, Atk: 32, Spe: 32}))), CodeInvalidInputForTest},
		// 配分
		{"alloc: 未知の mode", "adjustAllocation", set(bulk(), "mode", "balanced"), "invalid_enum"},
		{"alloc: mode の欠落(format と同じく必須の列挙)", "adjustAllocation", del(bulk(), "mode"), "invalid_enum"},
		{"alloc: 未知の focus", "adjustAllocation", set(bulk(), "focus", "mixed"), "invalid_enum"},
		{"alloc: bulk で focus の欠落", "adjustAllocation", del(bulk(), "focus"), CodeInvalidInputForTest},
		{"alloc: bulk で minSpeed", "adjustAllocation", set(bulk(), "minSpeed", 100), CodeInvalidInputForTest},
		{"alloc: 未知の offenseCategory", "adjustAllocation", set(offense(), "offenseCategory", "melee"), "invalid_enum"},
		{"alloc: offense で offenseCategory の欠落", "adjustAllocation", del(offense(), "offenseCategory"), CodeInvalidInputForTest},
		{"alloc: offenseCategory が status", "adjustAllocation", set(offense(), "offenseCategory", "status"), CodeInvalidInputForTest},
		{"alloc: minSpeed が負", "adjustAllocation", set(offense(), "minSpeed", -1), CodeInvalidInputForTest},
		{"alloc: ceiling が 33", "adjustAllocation", set(bulk(), "ceiling", map[string]any{"def": 33}), CodeInvalidInputForTest},
		{"alloc: ceiling が負", "adjustAllocation", set(bulk(), "ceiling", map[string]any{"def": -1}), CodeInvalidInputForTest},
		{"alloc: ceiling が下限未満(engine の検査)", "adjustAllocation", set(set(bulk(), "self",
			adjIndivDTO(adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 20}))), "ceiling", map[string]any{"hp": 10}), CodeInvalidInputForTest},
		{"alloc: 下限の合計 66 超", "adjustAllocation", set(bulk(), "self",
			adjIndivDTO(adjIndiv(adjWall, engine.NatureNeutral, engine.Stats{HP: 32, Atk: 32, Spe: 32}))), CodeInvalidInputForTest},
		{"alloc: ceiling に契約外の能力", "adjustAllocation", set(bulk(), "ceiling", map[string]any{"luck": 1}), "unknown_field"},
		{"alloc: goal の hits 0", "adjustAllocation", func() map[string]any { m := bulk(); goalOf(m)["hits"] = 0; return m }(), CodeInvalidInputForTest},
		{"alloc: goal のしきい値 0 の明示", "adjustAllocation", func() map[string]any { m := bulk(); goalOf(m)["thresholdPercent"] = 0; return m }(), CodeInvalidInputForTest},
		{"alloc: goal の技の分類が offenseCategory と違う", "adjustAllocation", func() map[string]any {
			m := offense()
			goalOf(m)["move"] = adjMoveDTO(adjFlame)
			return m
		}(), CodeInvalidInputForTest},
		{"alloc: goal があるのに typeChart の欠落", "adjustAllocation", del(bulk(), "typeChart"), "type_chart_missing"},
		// 共通
		{"壊れた JSON", "adjustAllocation", `{"self":`, "invalid_json"},
		{"配列", "adjustIndices", `[]`, "invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, ok := tt.req.(string)
			if !ok {
				req = mustJSON(t, tt.req)
			}
			if got := adjErrorCode(t, invoke(t, tt.fn, req)); got != tt.want {
				t.Errorf("code = %q, want %q", got, tt.want)
			}
		})
	}
}

// AC-A2: 値域の端(modifier / damageModifier の 1 と MaxEffectModifier、thresholdPercent の明示 100)は受け付ける。
func TestAdjustAcceptsBoundaryValues(t *testing.T) {
	self := adjIndivDTO(adjIndiv(adjDragon, natureAtkUp, engine.Stats{Atk: 32}))
	indices := func(withMove bool, mod int) map[string]any {
		m := map[string]any{"individual": self, "modifier": mod, "damageModifier": mod}
		if withMove {
			m["move"] = adjMoveDTO(adjQuake)
		}
		return m
	}
	threshold := func(c adjSearchCase) map[string]any {
		m := c.request(t)
		m["thresholdPercent"] = 100
		return m
	}
	tests := []struct {
		name string
		fn   string
		req  map[string]any
	}{
		{"indices: modifier 1", "adjustIndices", indices(true, engine.MinEffectModifier)},
		{"indices: modifier 2097152", "adjustIndices", indices(true, engine.MaxEffectModifier)},
		{"indices: 技なしで modifier 1", "adjustIndices", indices(false, engine.MinEffectModifier)},
		{"indices: 技なしで modifier 2097152", "adjustIndices", indices(false, engine.MaxEffectModifier)},
		{"ko: thresholdPercent 100 の明示", "adjustMinSpToKo", threshold(koSearchCases()[0])},
		{"survive: thresholdPercent 100 の明示", "adjustMinSpToSurvive", threshold(surviveSearchCases()[0])},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out map[string]any
			decodeEnvelope(t, invoke(t, tt.fn, mustJSON(t, tt.req)), &out)
		})
	}
}

// CodeInvalidInputForTest は invalid_input(wasmapi.CodeInvalidInput と同じ文字列。表を読みやすくするための別名)。
const CodeInvalidInputForTest = "invalid_input"

// AC-A4: 値域(hits・thresholdPercent・ceiling・minSpeed・modifier・damageModifier)の検査は DTO の変換(列挙の検証)より前。
// 両方の違反が重なったら invalid_input が先に出る(ADR-0108 決定3・ADR-0208 §3。HTTP と同じ順)。
func TestAdjustRangeChecksRunBeforeConversion(t *testing.T) {
	badField := map[string]any{"weather": "meteor", "terrain": "none"}
	tests := []struct {
		name string
		fn   string
		req  map[string]any
	}{
		{"ko: hits 11 と未知の weather", "adjustMinSpToKo", func() map[string]any {
			m := koSearchCases()[0].request(t)
			m["hits"], m["field"] = engine.MaxAdjustHits+1, badField
			return m
		}()},
		{"survive: しきい値 101 と未知の format", "adjustMinSpToSurvive", func() map[string]any {
			m := surviveSearchCases()[0].request(t)
			m["thresholdPercent"], m["format"] = 101, "triple"
			return m
		}()},
		{"alloc: ceiling 33 と未知の mode", "adjustAllocation", func() map[string]any {
			m := allocCases()[0].request(t)
			m["ceiling"], m["mode"] = map[string]any{"hp": 33}, "balanced"
			return m
		}()},
		{"alloc: minSpeed -1 と未知の offenseCategory", "adjustAllocation", func() map[string]any {
			m := allocCases()[3].request(t)
			m["minSpeed"], m["offenseCategory"] = -1, "melee"
			return m
		}()},
		{"alloc: goal の hits 11 と goal の未知の weather", "adjustAllocation", func() map[string]any {
			m := allocCases()[1].request(t)
			g := sub(t, m, "goal")
			g["hits"], g["field"] = engine.MaxAdjustHits+1, badField
			return m
		}()},
		{"indices: 技なしで modifier 0 と個体の未知の status", "adjustIndices", func() map[string]any {
			indiv := adjIndivDTO(adjIndiv(adjDragon, natureAtkUp, engine.Stats{Atk: 32}))
			indiv["status"] = "confused"
			return map[string]any{"individual": indiv, "modifier": 0}
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adjErrorCode(t, invoke(t, tt.fn, mustJSON(t, tt.req))); got != CodeInvalidInputForTest {
				t.Errorf("code = %q, want invalid_input(値域の検査が列挙より前)", got)
			}
		})
	}
}

// --- ベクタ(Go/WASM 一致テストの入力)---------------------------------------

// adjustVectorRequest はベクタのリクエストを組み立てる。adjustIndices は相性表を使わないので注入しない
// (注入すると unknown_field。ADR-0250 §4)。それ以外は先頭の typeChart を注入する(ADR-0011 §13)。
func adjustVectorRequest(t *testing.T, v vector) string {
	t.Helper()
	if v.Fn == "adjustIndices" {
		return string(v.Request)
	}
	return requestWithTypeChart(t, v.Request)
}

// ベクタは調整の4関数をそれぞれ1件以上含み、どれもネイティブ Go で成功する(期待値がエラー封筒だと一致テストが空振りする)。
func TestAdjustVectorsSucceed(t *testing.T) {
	for _, fn := range []string{"adjustIndices", "adjustMinSpToKo", "adjustMinSpToSurvive", "adjustAllocation"} {
		for _, v := range vectorsFor(t, fn) {
			t.Run(v.Name, func(t *testing.T) {
				resp := invoke(t, fn, adjustVectorRequest(t, v))
				var env map[string]json.RawMessage
				if err := json.Unmarshal([]byte(resp), &env); err != nil {
					t.Fatalf("JSON でない: %v", err)
				}
				if env["result"] == nil {
					t.Fatalf("ベクタが成功しない: %s", resp)
				}
				// 同じ入力を2回呼んでも同じバイト列(状態を持たない)。
				if again := invoke(t, fn, adjustVectorRequest(t, v)); !bytes.Equal([]byte(again), []byte(resp)) {
					t.Errorf("2回目の応答が違う")
				}
			})
		}
	}
}

func ptr(v int) *int          { return &v }
func ptrF(v float64) *float64 { return &v }
