package httpapi

// POST /api/calc/adjust/goals の受け入れテスト(ADR-0331 段階 B・ADR-0177 §9)。
//
//   - C1 素通し: 成功時は同じ入力を engine.SuggestSPForGoals に直接渡した結果の写し。ceiling の省略は 32、
//     outspeed の moveId は engine.GuaranteedSelfSpeedStage で段数にする。outspeed は chancePercent が null、
//     survive・ko は selfSpeed・opponentSpeed・selfSpeedRank が null(キーは省略しない)。unsupported は空なら []。
//   - C2 エラー語彙(新しい code を足さない)と検査順(値域 → 列挙 → 種類ごとの必須 → ID 解決)。
//   - C3 ヘッダ必須・計算イベントを発行しない・マスタ準備中は 503・応答は契約に照らして妥当。
//
// 期待値はすべて engine を直接呼んで作る(手計算しない)。WASM には出さない(ADR-0319 §1)のでパリティは見ない。

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

const pathAdjustGoals = "/api/calc/adjust/goals"

// 目標のテストだけで使う技(追加効果つき。fakeStore に足す)。
const (
	moveCharge      = "test-charge"       // fire / physical / 50。確率 100%・自分の素早さ +1(ニトロチャージ相当)
	moveMaybeCharge = "test-maybe-charge" // fire / physical / 50。確率 50%・自分の素早さ +1(掛けない)
	moveFoeSlow     = "test-foe-slow"     // water / special / 55。確率 100%・相手の素早さ -1(掛けない)
)

// newGoalsStore は fakeStore に追加効果つきの技を足したもの(ほかのテストの fakeStore は変えない)。
func newGoalsStore(t testing.TB) *fakeStore {
	t.Helper()
	f := newFakeStore(t)
	f.moves[moveCharge] = engine.Move{ID: moveCharge, NameJa: "テストチャージ", Type: engine.TypeFire, Category: engine.CategoryPhysical, Power: 50,
		Effect: &engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}}}
	f.moves[moveMaybeCharge] = engine.Move{ID: moveMaybeCharge, NameJa: "テストかもチャージ", Type: engine.TypeFire, Category: engine.CategoryPhysical, Power: 50,
		Effect: &engine.MoveEffect{Chance: 50, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}}}
	f.moves[moveFoeSlow] = engine.Move{ID: moveFoeSlow, NameJa: "テストこごえ", Type: engine.TypeWater, Category: engine.CategorySpecial, Power: 55,
		Effect: &engine.MoveEffect{Chance: 100, Target: engine.RankTargetOpponent, Stages: map[engine.StatKey]int{engine.StatSpe: -1}}}
	return f
}

// adjGoalCase は目標1件。
type adjGoalCase struct {
	kind      string
	opponent  indiv
	moveID    string // "" は省略
	hits      *int
	threshold *float64
}

func (g adjGoalCase) http() map[string]any {
	m := map[string]any{"kind": g.kind, "opponent": g.opponent.http()}
	if g.moveID != "" {
		m["moveId"] = g.moveID
	}
	if g.hits != nil {
		m["hits"] = *g.hits
	}
	if g.threshold != nil {
		m["thresholdPercent"] = *g.threshold
	}
	return m
}

// adjGoalsCase は要求1件。
type adjGoalsCase struct {
	name    string
	self    indiv
	ceiling map[string]int // nil は省略
	goals   []adjGoalCase
}

func (c adjGoalsCase) httpBody() map[string]any {
	goals := make([]any, 0, len(c.goals))
	for _, g := range c.goals {
		goals = append(goals, g.http())
	}
	b := map[string]any{"format": "single", "self": c.self.http(), "goals": goals}
	if c.ceiling != nil {
		ceiling := map[string]any{}
		for k, v := range c.ceiling {
			ceiling[k] = v
		}
		b["ceiling"] = ceiling
	}
	return b
}

// engineInput は同じ入力を engine.SPGoalsInput にする(ceiling の省略は 32、場は parseField(nil) と同じ零値)。
func (c adjGoalsCase) engineInput(t *testing.T, f *fakeStore) engine.SPGoalsInput {
	t.Helper()
	ceiling := engine.Stats{HP: 32, Atk: 32, Def: 32, SpA: 32, SpD: 32, Spe: 32}
	for k, v := range c.ceiling {
		key := map[string]engine.StatKey{"hp": engine.StatHP, "atk": engine.StatAtk, "def": engine.StatDef,
			"spa": engine.StatSpA, "spd": engine.StatSpD, "spe": engine.StatSpe}[k]
		ceiling = ceiling.WithStat(key, v)
	}
	in := engine.SPGoalsInput{
		Format: engine.FormatSingle, Self: c.self.engine(t, f), Ceiling: ceiling,
		Field: engine.Field{}, TypeChart: f.TypeChart(),
	}
	for _, g := range c.goals {
		goal := engine.SPGoal{Kind: engine.SPGoalKind(g.kind), Opponent: g.opponent.engine(t, f)}
		if g.moveID != "" {
			mv, ok := f.moves[g.moveID]
			if !ok {
				t.Fatalf("fixture の技 %q が無い", g.moveID)
			}
			if g.kind == string(engine.SPGoalOutspeed) {
				goal.SelfSpeedStage = engine.GuaranteedSelfSpeedStage(mv)
			} else {
				goal.Move = mv
			}
		}
		if g.kind != string(engine.SPGoalOutspeed) {
			if g.hits != nil {
				goal.Hits = *g.hits
			}
			if g.threshold != nil {
				goal.ThresholdPercent = *g.threshold
			}
		}
		in.Goals = append(in.Goals, goal)
	}
	return in
}

// goalsResultOf は engine の結果を契約の形に写す(テスト側の独立した写し)。
func goalsResultOf(r engine.SPGoalsResult) api.AdjustGoalsResult {
	out := api.AdjustGoalsResult{
		Feasible: r.Feasible, Remaining: r.Remaining,
		Plan:  api.AdjustGoalsPlan{Sp: statBlockFrom(r.Plan.SP), TotalSp: r.Plan.TotalSP, Stats: statBlockFrom(r.Plan.Real)},
		Goals: []api.AdjustGoalOutcome{}, Unsupported: []api.UnsupportedMark{},
	}
	for _, o := range r.Goals {
		g := api.AdjustGoalOutcome{Kind: api.AdjustGoalKind(o.Kind), Met: o.Met}
		if o.Kind == engine.SPGoalOutspeed {
			self, foe, rank := o.SelfSpeed, o.OpponentSpeed, o.SelfSpeedRank
			g.SelfSpeed, g.OpponentSpeed, g.SelfSpeedRank = &self, &foe, &rank
		} else {
			chance := o.ChancePercent
			g.ChancePercent = &chance
		}
		out.Goals = append(out.Goals, g)
	}
	for _, m := range r.Unsupported {
		out.Unsupported = append(out.Unsupported, api.UnsupportedMark{Target: string(m.Target), Reason: string(m.Reason), Id: m.ID})
	}
	return out
}

func hitsOf(v int) *int { return &v }

func adjGoalsCases() []adjGoalsCase {
	fastLeaf := indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{Spe: 32}} // 85 + 20 + 32 = 137
	bulkyFoe := indiv{speciesKey: speciesDefender, natureID: natureNeutral, sp: engine.Stats{HP: 32}}
	hitter := indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{Atk: 32}}
	caster := indiv{speciesKey: speciesLeaf, natureID: natureSpAUp, sp: engine.Stats{SpA: 32}}
	return []adjGoalsCase{
		{name: "素早さだけ", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
			goals: []adjGoalCase{{kind: "outspeed", opponent: fastLeaf}}},
		{name: "素早さ(ニトロチャージ)+ 倒す", self: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp},
			goals: []adjGoalCase{
				{kind: "outspeed", opponent: fastLeaf, moveID: moveCharge},
				{kind: "ko", opponent: bulkyFoe, moveID: movePhysical, hits: hitsOf(3)},
			}},
		{name: "耐える2件(物理と特殊)", self: indiv{speciesKey: speciesDefender, natureID: natureNeutral},
			goals: []adjGoalCase{
				{kind: "survive", opponent: hitter, moveID: movePhysical, hits: hitsOf(2)},
				{kind: "survive", opponent: caster, moveID: moveSpecial, hits: hitsOf(2), threshold: half()},
			}},
		{name: "満たせない(相手 +6)", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
			goals: []adjGoalCase{{kind: "outspeed", opponent: indiv{speciesKey: speciesLeaf, natureID: natureNeutral,
				sp: engine.Stats{Spe: 32}, ranks: engine.Ranks{Spe: 6}}}}},
		{name: "ceiling と下限を明示", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{Spe: 10, SpA: 4}},
			ceiling: map[string]int{"spe": 20, "atk": 12},
			goals: []adjGoalCase{
				{kind: "outspeed", opponent: fastLeaf},
				{kind: "ko", opponent: bulkyFoe, moveID: movePhysical, hits: hitsOf(3)},
			}},
		{name: "未対応の印(テラスタイプ)", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, tera: engine.TypeFire},
			goals: []adjGoalCase{{kind: "ko", opponent: bulkyFoe, moveID: movePhysical, hits: hitsOf(3)}}},
		{name: "6件", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
			goals: []adjGoalCase{
				{kind: "outspeed", opponent: fastLeaf}, {kind: "outspeed", opponent: fastLeaf, moveID: moveCharge},
				{kind: "ko", opponent: bulkyFoe, moveID: movePhysical, hits: hitsOf(3)},
				{kind: "ko", opponent: bulkyFoe, moveID: moveSpecial, hits: hitsOf(3)},
				{kind: "survive", opponent: hitter, moveID: movePhysical, hits: hitsOf(1)},
				{kind: "survive", opponent: caster, moveID: moveSpecial, hits: hitsOf(1)},
			}},
		// critic C-1: 下限が大きく ceiling を省略(32)しても 500 にしない(66 を超える組を計算に渡さない)。
		{name: "下限 H32・B32 で倒す(ceiling 省略)", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{HP: 32, Def: 32}},
			goals: []adjGoalCase{{kind: "ko", opponent: bulkyFoe, moveID: movePhysical, hits: hitsOf(3)}}},
		{name: "下限の合計 66 で素早さ + 倒す", self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral, sp: engine.Stats{HP: 32, Def: 32, Spe: 2}},
			goals: []adjGoalCase{
				{kind: "outspeed", opponent: fastLeaf},
				{kind: "ko", opponent: bulkyFoe, moveID: movePhysical, hits: hitsOf(3)},
			}},
	}
}

// C1: HTTP の応答は engine の結果の写し。
func TestAdjustGoalsHTTPMatchesEngine(t *testing.T) {
	store := newGoalsStore(t)
	h := NewHandler(store, nil)
	for _, c := range adjGoalsCases() {
		t.Run(c.name, func(t *testing.T) {
			rec := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
			var got api.AdjustGoalsResult
			decodeInto(t, rec, &got)
			res, err := engine.SuggestSPForGoals(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.SuggestSPForGoals: %v", err)
			}
			if want := goalsResultOf(res); !reflect.DeepEqual(got, want) {
				t.Errorf("応答 = %+v\nwant %+v", got, want)
			}
		})
	}
}

// C1: null の出し分け(キーを省略しない)と unsupported の空配列。
func TestAdjustGoalsHTTPNullFields(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	c := adjGoalsCases()[1] // outspeed + ko
	rec := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	var raw struct {
		Goals       []map[string]json.RawMessage `json:"goals"`
		Unsupported json.RawMessage              `json:"unsupported"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if len(raw.Goals) != 2 {
		t.Fatalf("goals の件数 = %d, want 2", len(raw.Goals))
	}
	for _, k := range []string{"chancePercent"} {
		if v, ok := raw.Goals[0][k]; !ok || string(v) != "null" {
			t.Errorf("outspeed の %s = %s(want null で明示)", k, v)
		}
	}
	for _, k := range []string{"selfSpeed", "opponentSpeed", "selfSpeedRank"} {
		if v, ok := raw.Goals[1][k]; !ok || string(v) != "null" {
			t.Errorf("ko の %s = %s(want null で明示)", k, v)
		}
		if v := raw.Goals[0][k]; string(v) == "null" || len(v) == 0 {
			t.Errorf("outspeed の %s が null/欠落", k)
		}
	}
	if string(raw.Unsupported) != "[]" {
		t.Errorf("unsupported = %s, want []", raw.Unsupported)
	}
}

// C1: 先に使う技の段数は「確率 100%・自分・素早さ」だけ。効果の無い技・変化技でもエラーにしない。
func TestAdjustGoalsHTTPSelfSpeedRankFromMoveEffect(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	foe := indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{Spe: 32}}
	tests := []struct {
		name   string
		self   indiv
		moveID string
		want   int
	}{
		{"技なし", indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, "", 0},
		{"確率 100% の自分の素早さ +1", indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, moveCharge, 1},
		{"確率 50% は掛けない", indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, moveMaybeCharge, 0},
		{"相手の素早さを下げる技は掛けない", indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, moveFoeSlow, 0},
		{"効果の無い攻撃技", indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, movePhysical, 0},
		{"変化技もエラーにしない", indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, moveStatus, 0},
		{"自分のランクに足す", indiv{speciesKey: speciesAttacker, natureID: natureNeutral, ranks: engine.Ranks{Spe: 2}}, moveCharge, 3},
		{"6 で頭打ち", indiv{speciesKey: speciesAttacker, natureID: natureNeutral, ranks: engine.Ranks{Spe: 6}}, moveCharge, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := adjGoalsCase{self: tt.self, goals: []adjGoalCase{{kind: "outspeed", opponent: foe, moveID: tt.moveID}}}
			rec := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
			var got api.AdjustGoalsResult
			decodeInto(t, rec, &got)
			if len(got.Goals) != 1 || got.Goals[0].SelfSpeedRank == nil || *got.Goals[0].SelfSpeedRank != tt.want {
				t.Errorf("goals = %+v, want selfSpeedRank %d", got.Goals, tt.want)
			}
		})
	}
}

// C1: ceiling の省略は全能力 32(明示した 32 と同じ応答)。
func TestAdjustGoalsHTTPCeilingDefault(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	c := adjGoalsCases()[1]
	omitted := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
	c.ceiling = map[string]int{"hp": 32, "atk": 32, "def": 32, "spa": 32, "spd": 32, "spe": 32}
	explicit := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
	if omitted.Code != http.StatusOK || omitted.Body.String() != explicit.Body.String() {
		t.Errorf("省略 %d %s\n明示 %d %s", omitted.Code, omitted.Body, explicit.Code, explicit.Body)
	}
}

// C2: エラー語彙。
func TestAdjustGoalsHTTPErrors(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	base := func() map[string]any { return adjGoalsCases()[1].httpBody() } // [outspeed(charge), ko(physical, 3)]
	set := func(m map[string]any, k string, v any) map[string]any { m[k] = v; return m }
	goal := func(m map[string]any, i int, k string, v any) map[string]any {
		m["goals"].([]any)[i].(map[string]any)[k] = v
		return m
	}
	delGoal := func(m map[string]any, i int, k string) map[string]any {
		delete(m["goals"].([]any)[i].(map[string]any), k)
		return m
	}
	speed := adjGoalCase{kind: "outspeed", opponent: indiv{speciesKey: speciesLeaf, natureID: natureNeutral}}.http()
	seven := []any{speed, speed, speed, speed, speed, speed, speed}
	unknown := indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()

	tests := []struct {
		name string
		body any
		want string
	}{
		{"壊れた JSON", `{"goals":`, "invalid_json"},
		{"契約に無いフィールド(目標の field)", goal(base(), 1, "field", map[string]any{}), "unknown_field"},
		{"契約に無いフィールド(options)", set(base(), "options", map[string]any{}), "unknown_field"},
		{"目標が 0 件", set(base(), "goals", []any{}), "invalid_input"},
		{"goals の欠落", func() map[string]any { m := base(); delete(m, "goals"); return m }(), "invalid_input"},
		{"目標が 7 件", set(base(), "goals", seven), "invalid_input"},
		{"未知の kind", goal(base(), 0, "kind", "faster"), "invalid_enum"},
		{"kind の欠落", delGoal(base(), 0, "kind"), "invalid_enum"},
		{"未知の format", set(base(), "format", "triple"), "invalid_enum"},
		{"format の欠落", func() map[string]any { m := base(); delete(m, "format"); return m }(), "invalid_enum"},
		{"ko の moveId の欠落", delGoal(base(), 1, "moveId"), "invalid_input"},
		{"ko の hits の欠落", delGoal(base(), 1, "hits"), "invalid_input"},
		{"survive の moveId の欠落", delGoal(goal(base(), 1, "kind", "survive"), 1, "moveId"), "invalid_input"},
		{"ko の hits 0", goal(base(), 1, "hits", 0), "invalid_input"},
		{"ko の hits 11", goal(base(), 1, "hits", engine.MaxAdjustHits+1), "invalid_input"},
		{"ko のしきい値 0 の明示", goal(base(), 1, "thresholdPercent", 0), "invalid_input"},
		{"ko のしきい値 100 超", goal(base(), 1, "thresholdPercent", 100.5), "invalid_input"},
		{"outspeed でも hits の値域は検査する", goal(base(), 0, "hits", 0), "invalid_input"},
		{"outspeed でもしきい値の値域は検査する", goal(base(), 0, "thresholdPercent", 101), "invalid_input"},
		{"ceiling 33", set(base(), "ceiling", map[string]any{"spe": 33}), "invalid_input"},
		{"ceiling が負", set(base(), "ceiling", map[string]any{"hp": -1}), "invalid_input"},
		{"ceiling に契約外の能力", set(base(), "ceiling", map[string]any{"luck": 1}), "unknown_field"},
		{"探索する能力の ceiling が下限未満", set(set(base(), "self", indiv{speciesKey: speciesAttacker, natureID: natureNeutral,
			sp: engine.Stats{Spe: 20}}.http()), "ceiling", map[string]any{"spe": 10}), "invalid_input"},
		{"ko が変化技", goal(base(), 1, "moveId", moveStatus), "invalid_input"},
		{"自分の下限の合計 67", set(base(), "self", indiv{speciesKey: speciesAttacker, natureID: natureNeutral,
			sp: engine.Stats{HP: 32, Atk: 32, Spe: 3}}.http()), "invalid_input"},
		{"相手の SP 33", goal(base(), 0, "opponent", indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{Spe: 33}}.http()), "invalid_input"},
		{"自分の未知の種族", set(base(), "self", unknown), "unknown_species"},
		{"相手の未知の種族", goal(base(), 1, "opponent", unknown), "unknown_species"},
		{"相手の未知の性格", goal(base(), 0, "opponent", indiv{speciesKey: speciesLeaf, natureID: "test-nothing"}.http()), "unknown_nature"},
		{"相手の未知の持ち物", goal(base(), 0, "opponent", indiv{speciesKey: speciesLeaf, natureID: natureNeutral, itemID: "test-nothing"}.http()), "unknown_item"},
		{"ko の未知の技", goal(base(), 1, "moveId", "test-nothing"), "unknown_move"},
		{"outspeed の未知の先に使う技", goal(base(), 0, "moveId", "test-nothing"), "unknown_move"},
		{"相手の未知の状態異常", goal(base(), 0, "opponent", indiv{speciesKey: speciesLeaf, natureID: natureNeutral, status: "confused"}.http()), "invalid_enum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, ok := tt.body.(string)
			raw := []byte(body)
			if !ok {
				raw = mustJSON(t, tt.body)
			}
			rec := post(t, h, pathAdjustGoals, raw, false)
			assertError(t, rec, http.StatusBadRequest, tt.want)
		})
	}
}

// C2: 検査順(値域 → 列挙 → 種類ごとの必須 → ID 解決)。値域・列挙・必須の段ではマスタを引かない。
func TestAdjustGoalsCheckOrder(t *testing.T) {
	unknownIndiv := indiv{speciesKey: speciesUnknown, natureID: "test-nothing"}.http()
	// すべての ID をマスタに無いものにした本文(どの段で止まっても ID 解決には届かないことを見る)。
	unknownBody := func() map[string]any {
		return map[string]any{
			"format": "single", "self": unknownIndiv,
			"goals": []any{
				map[string]any{"kind": "outspeed", "opponent": unknownIndiv, "moveId": "test-nothing"},
				map[string]any{"kind": "ko", "opponent": unknownIndiv, "moveId": "test-nothing", "hits": 2},
			},
		}
	}
	goal := func(m map[string]any, i int, k string, v any) map[string]any {
		m["goals"].([]any)[i].(map[string]any)[k] = v
		return m
	}
	tests := []struct {
		name string
		body map[string]any
		want string
	}{
		{"目標 0 件", func() map[string]any { m := unknownBody(); m["goals"] = []any{}; return m }(), "invalid_input"},
		{"目標 7 件", func() map[string]any {
			m := unknownBody()
			g := m["goals"].([]any)[0]
			m["goals"] = []any{g, g, g, g, g, g, g}
			return m
		}(), "invalid_input"},
		{"hits 11", goal(unknownBody(), 1, "hits", engine.MaxAdjustHits+1), "invalid_input"},
		{"しきい値 0", goal(unknownBody(), 1, "thresholdPercent", 0), "invalid_input"},
		{"outspeed の hits 0", goal(unknownBody(), 0, "hits", 0), "invalid_input"},
		{"ceiling 33", func() map[string]any { m := unknownBody(); m["ceiling"] = map[string]any{"hp": 33}; return m }(), "invalid_input"},
		{"値域は列挙より前(hits 11 と未知の kind)", goal(goal(unknownBody(), 1, "hits", engine.MaxAdjustHits+1), 0, "kind", "faster"), "invalid_input"},
		{"未知の kind", goal(unknownBody(), 0, "kind", "faster"), "invalid_enum"},
		{"未知の format", func() map[string]any { m := unknownBody(); m["format"] = "triple"; return m }(), "invalid_enum"},
		{"列挙は必須より前(未知の format と moveId の欠落)", func() map[string]any {
			m := unknownBody()
			m["format"] = "triple"
			delete(m["goals"].([]any)[1].(map[string]any), "moveId")
			return m
		}(), "invalid_enum"},
		{"ko の moveId の欠落", func() map[string]any {
			m := unknownBody()
			delete(m["goals"].([]any)[1].(map[string]any), "moveId")
			return m
		}(), "invalid_input"},
		{"survive の hits の欠落", func() map[string]any {
			m := unknownBody()
			g := m["goals"].([]any)[1].(map[string]any)
			g["kind"] = "survive"
			delete(g, "hits")
			return m
		}(), "invalid_input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &countingStore{inner: newGoalsStore(t)}
			h := NewHandler(store, nil)
			rec := post(t, h, pathAdjustGoals, mustJSON(t, tt.body), false)
			assertError(t, rec, http.StatusBadRequest, tt.want)
			if store.lookups != 0 || store.chartGet != 0 {
				t.Errorf("マスタ参照 %d 回・相性表 %d 回(want 0。ID 解決より前に止まる)", store.lookups, store.chartGet)
			}
		})
	}
}

// C2: ID 解決は self → 目標の順(opponent → moveId)。先に出た未知の ID の code になる。
func TestAdjustGoalsResolveOrder(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	body := adjGoalsCases()[1].httpBody()
	body["self"] = indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()
	goals := body["goals"].([]any)
	goals[0].(map[string]any)["moveId"] = "test-nothing"
	rec := post(t, h, pathAdjustGoals, mustJSON(t, body), false)
	assertError(t, rec, http.StatusBadRequest, "unknown_species")

	body = adjGoalsCases()[1].httpBody()
	goals = body["goals"].([]any)
	goals[0].(map[string]any)["moveId"] = "test-nothing"
	goals[1].(map[string]any)["opponent"] = indiv{speciesKey: speciesUnknown, natureID: natureNeutral}.http()
	rec = post(t, h, pathAdjustGoals, mustJSON(t, body), false)
	assertError(t, rec, http.StatusBadRequest, "unknown_move")
}

// C3: 必須ヘッダ。
func TestAdjustGoalsMissingHeaders(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	header := validHeaders()
	header.Del("X-Device-Id")
	rec := serve(t, h, http.MethodPost, pathAdjustGoals, header, mustJSON(t, adjGoalsCases()[0].httpBody()))
	assertError(t, rec, http.StatusBadRequest, "missing_header")
}

// C3: 計算イベントを発行しない(ADR-0250 §7)。
func TestAdjustGoalsDoesNotPublishEvents(t *testing.T) {
	pub := &fakePublisher{}
	h := NewHandler(newGoalsStore(t), pub)
	for _, c := range adjGoalsCases() {
		rec := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d; body=%s", c.name, rec.Code, rec.Body)
		}
	}
	if len(pub.calls) != 0 {
		t.Errorf("計算イベントを %d 件発行した(want 0)", len(pub.calls))
	}
}

// C3: マスタ準備中は 503 master_unavailable、準備後は 200。
func TestAdjustGoalsDeferredHandler(t *testing.T) {
	sw := &switchableStore{store: newGoalsStore(t)}
	h := NewDeferredHandler(sw.current, nil)
	body := mustJSON(t, adjGoalsCases()[1].httpBody())
	assertError(t, post(t, h, pathAdjustGoals, body, true), http.StatusServiceUnavailable, "master_unavailable")
	sw.ready.Store(true)
	assertStatusOK(t, post(t, h, pathAdjustGoals, body, true), pathAdjustGoals)
}

// C3: 契約の検証ヘルパーが goals の応答で空振りしない。
func TestAdjustGoalsContractHelperIsNotVacuous(t *testing.T) {
	const plan = `"plan":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":28},"totalSp":28,` +
		`"stats":{"hp":155,"atk":120,"def":90,"spa":80,"spd":90,"spe":138}}`
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"妥当な結果", `{"feasible":true,"remaining":38,` + plan + `,"goals":[{"kind":"outspeed","met":true,"chancePercent":null,` +
			`"selfSpeed":138,"opponentSpeed":137,"selfSpeedRank":0}],"unsupported":[]}`, false},
		{"selfSpeedRank が 7", `{"feasible":true,"remaining":38,` + plan + `,"goals":[{"kind":"outspeed","met":true,"chancePercent":null,` +
			`"selfSpeed":138,"opponentSpeed":137,"selfSpeedRank":7}],"unsupported":[]}`, true},
		{"goals の結果のキー欠落", `{"feasible":true,"remaining":38,` + plan + `,"goals":[{"kind":"outspeed","met":true}],"unsupported":[]}`, true},
		{"unsupported 欠落", `{"feasible":true,"remaining":38,` + plan + `,"goals":[]}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgainstContract(t, http.MethodPost, pathAdjustGoals, validHeaders(), []byte(`{}`),
				http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, []byte(tt.body), false)
			if err == errNoRoute {
				t.Fatalf("契約に %s が無い", pathAdjustGoals)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("検証エラー = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// C1: 応答の目標の順はリクエストの順(種類で並べ替えない)。
func TestAdjustGoalsHTTPKeepsGoalOrder(t *testing.T) {
	h := NewHandler(newGoalsStore(t), nil)
	c := adjGoalsCases()[6] // 6件
	rec := post(t, h, pathAdjustGoals, mustJSON(t, c.httpBody()), true)
	var got api.AdjustGoalsResult
	decodeInto(t, rec, &got)
	var kinds []string
	for _, g := range got.Goals {
		kinds = append(kinds, string(g.Kind))
	}
	want := []string{"outspeed", "outspeed", "ko", "ko", "survive", "survive"}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Errorf("goals の順 = %v, want %v", kinds, want)
	}
}

// --- ベンチマーク(上限ちょうど: 6 件・全能力・hits 10・しきい値 50%。壁時計の閾値は判定しない) -----------

func BenchmarkAdjustGoalsAtLimit(b *testing.B) {
	h := NewHandler(newGoalsStore(b), nil)
	hits, th := engine.MaxAdjustHits, half()
	fast := indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{Spe: 32}}
	foe := indiv{speciesKey: speciesDefender, natureID: natureNeutral, sp: engine.Stats{HP: 32}}
	hitter := indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{Atk: 32}}
	c := adjGoalsCase{self: indiv{speciesKey: speciesAttacker, natureID: natureNeutral}, goals: []adjGoalCase{
		{kind: "outspeed", opponent: fast, moveID: moveCharge}, {kind: "outspeed", opponent: fast},
		{kind: "ko", opponent: foe, moveID: movePhysical, hits: &hits, threshold: th},
		{kind: "ko", opponent: foe, moveID: moveSpecial, hits: &hits, threshold: th},
		{kind: "survive", opponent: hitter, moveID: movePhysical, hits: &hits, threshold: th},
		{kind: "survive", opponent: hitter, moveID: moveSpecial, hits: &hits, threshold: th},
	}}
	body := mustJSON(b, c.httpBody())
	header := validHeaders()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := serve(b, h, http.MethodPost, pathAdjustGoals, header, body)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
	}
}
