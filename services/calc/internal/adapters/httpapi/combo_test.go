package httpapi

// POST /api/calc/combo の受け入れテスト(ADR-0253。docs/plan/combo-calc.md §5 の AC-13・AC-16・AC-17)。
//
//   - K1 素通し: 成功時は同じ入力を engine.CalcCombo に直接渡した結果の写し(テスト側の独立した写し comboResultOf と一致)。
//     opponentSpeed の省略は unknown、speedField の省略は全部 false、critical の省略は false、hits の省略は 0(既定の回数)。
//   - K2 null の出し分け: 変化技の手の damage・相手の技なしの opponentAttack は null(キーは省かない)。unknown の opponentSpeed は 0 で
//     speed.opponentSpeedKnown が false(WASM の calcCombo と同じ形)。印なしの unsupported は [](null にしない)。
//   - K3(AC-13)各手の damage は、その手の開始時のランク(自分 = selfStartRanks・相手 = opponentStartRanks)を入れた
//     POST /api/calc の応答(CalcResult)と同じ。
//   - K4(AC-16)入力の検査: 手 0 件・4 件は ID の解決より前に 400 invalid_input(マスタを 1 回も引かない)。
//     ランク変化の無い変化技の手(確率 100% 未満の変化だけの技も)・変化技の相手の技は 400 invalid_input。
//     未知の技(手・相手の技)は 400 unknown_move(HTTP の語彙。/api/calc と同じ。422 は契約に無い)。
//     opponentSpeed の未知の値は 400 invalid_enum。エラーの形は /api/calc と同じ(Error 形式・契約どおり)。
//   - K5(AC-17)計算イベントを発行しない(record・team・NATS に依存しない)。publisher なしでも成功。
//     マスタ準備中は 503 master_unavailable、準備後は 200(registerDeferredCalcRoutes にも登録する)。
//   - K6 ヘッダ必須(missing_header)。契約に操作があり、検証ヘルパーが空振りしない。
//
// 期待値はすべて engine を直接呼んで作る(手計算しない)。

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

const pathCombo = "/api/calc/combo"

// 連続技のテストだけで使う技(newComboStore で fakeStore に足す)。
const (
	moveDance       = "test-dance"        // normal / status。確率 100%・自分の攻撃 +1・素早さ +1(りゅうのまい相当)
	moveMaybeDance  = "test-maybe-dance"  // normal / status。確率 50%・自分の攻撃 +1(確率 100% 未満だけ = 手に置けない)
	moveSneak       = "test-sneak"        // ghost / physical / 40 / 優先度 +1(かげうち相当)
	moveSelfDrop    = "test-self-drop"    // fighting / physical / 120。確率 100%・自分の防御 -1・特防 -1(インファイト相当)
	moveBulletMulti = "test-bullet-multi" // grass / physical / 25。2〜5 回の多段(タネマシンガン相当)
)

// newComboStore は目標のテストの技(test-charge・test-foe-slow 等)に、連続技のテストの技を足したもの。
func newComboStore(t testing.TB) *fakeStore {
	t.Helper()
	f := newGoalsStore(t)
	f.moves[moveDance] = engine.Move{ID: moveDance, NameJa: "テストのまい", Type: engine.TypeNormal, Category: engine.CategoryStatus,
		Effect: &engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatAtk: 1, engine.StatSpe: 1}}}
	f.moves[moveMaybeDance] = engine.Move{ID: moveMaybeDance, NameJa: "テストかもまい", Type: engine.TypeNormal, Category: engine.CategoryStatus,
		Effect: &engine.MoveEffect{Chance: 50, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatAtk: 1}}}
	f.moves[moveSneak] = engine.Move{ID: moveSneak, NameJa: "テストかげうち", Type: engine.TypeGhost, Category: engine.CategoryPhysical, Power: 40, Priority: 1}
	f.moves[moveSelfDrop] = engine.Move{ID: moveSelfDrop, NameJa: "テストインファイト", Type: engine.TypeFighting, Category: engine.CategoryPhysical, Power: 120,
		Effect: &engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatDef: -1, engine.StatSpD: -1}}}
	f.moves[moveBulletMulti] = engine.Move{ID: moveBulletMulti, NameJa: "テストマシンガン", Type: engine.TypeGrass, Category: engine.CategoryPhysical, Power: 25,
		Mechanisms: []engine.MoveMechanism{engine.MechanismMultiHit},
		Params:     engine.MechanismParams{MultiHit: &engine.MultiHit{Min: 2, Max: 5}}}
	return f
}

// comboStepCase は手 1 件。
type comboStepCase struct {
	moveID   string
	critical *bool
	hits     *int
}

func (s comboStepCase) http() map[string]any {
	m := map[string]any{"moveId": s.moveID}
	if s.critical != nil {
		m["critical"] = *s.critical
	}
	if s.hits != nil {
		m["hits"] = *s.hits
	}
	return m
}

// comboCase は要求 1 件。
type comboCase struct {
	name          string
	attacker      indiv
	defender      indiv
	field         map[string]any // nil は省略(engine では零値の Field)
	engineField   engine.Field
	steps         []comboStepCase
	opponentMove  string // "" は省略
	opponentSpeed string // "" は省略(unknown)
	speedField    *engine.ComboSpeedField
}

func (c comboCase) httpBody() map[string]any {
	steps := make([]any, 0, len(c.steps))
	for _, s := range c.steps {
		steps = append(steps, s.http())
	}
	b := map[string]any{"format": "single", "attacker": c.attacker.http(), "defender": c.defender.http(), "steps": steps}
	if c.field != nil {
		b["field"] = c.field
	}
	if c.opponentMove != "" {
		b["opponentMoveId"] = c.opponentMove
	}
	if c.opponentSpeed != "" {
		b["opponentSpeed"] = c.opponentSpeed
	}
	if sf := c.speedField; sf != nil {
		b["speedField"] = map[string]any{"trickRoom": sf.TrickRoom, "selfTailwind": sf.SelfTailwind, "opponentTailwind": sf.OpponentTailwind}
	}
	return b
}

// engineInput は同じ入力を engine.ComboInput にする。
func (c comboCase) engineInput(t *testing.T, f *fakeStore) engine.ComboInput {
	t.Helper()
	move := func(id string) engine.Move {
		mv, ok := f.moves[id]
		if !ok {
			t.Fatalf("fixture の技 %q が無い", id)
		}
		return mv
	}
	in := engine.ComboInput{
		Format: engine.FormatSingle, Attacker: c.attacker.engine(t, f), Defender: c.defender.engine(t, f),
		Field: c.engineField, TypeChart: f.TypeChart(), OpponentSpeed: engine.OpponentSpeedUnknown,
	}
	if c.opponentSpeed != "" {
		in.OpponentSpeed = engine.OpponentSpeedMode(c.opponentSpeed)
	}
	if c.speedField != nil {
		in.SpeedField = *c.speedField
	}
	for _, s := range c.steps {
		st := engine.ComboStep{Move: move(s.moveID)}
		if s.critical != nil {
			st.Critical = *s.critical
		}
		if s.hits != nil {
			st.Hits = *s.hits
		}
		in.Steps = append(in.Steps, st)
	}
	if c.opponentMove != "" {
		mv := move(c.opponentMove)
		in.OpponentMove = &mv
	}
	return in
}

// --- テスト側の独立した写し(engine → 契約の形) ---------------------------------------

func comboRanksOf(r engine.Ranks) api.ComboRanks {
	return api.ComboRanks{Atk: r.Atk, Def: r.Def, Spa: r.SpA, Spd: r.SpD, Spe: r.Spe}
}

func comboCalcResultOf(d engine.DamageResult) api.CalcResult {
	minT, maxT := d.DisplayPercentRangeTenths()
	chance := d.KO.ChancePercent
	marks := []api.UnsupportedMark{}
	for _, m := range d.Unsupported {
		marks = append(marks, api.UnsupportedMark{Target: string(m.Target), Reason: string(m.Reason), Id: m.ID})
	}
	return api.CalcResult{
		Rolls: append([]int(nil), d.Rolls[:]...), MinDamage: d.MinDamage(), MaxDamage: d.MaxDamage(),
		MinPercent: tenths(minT), MaxPercent: tenths(maxT), DefenderHP: d.DefenderHP,
		Effectiveness: d.Effectiveness, Stab: d.STAB, Category: api.MoveCategory(d.Category),
		Ko: api.KOChance{Hits: d.KO.Hits, Guaranteed: d.KO.Guaranteed, ChancePercent: &chance,
			DisplayChancePercent: tenths(d.KO.DisplayChancePercentTenths())},
		Unsupported: marks,
	}
}

func comboKOOf(k engine.ComboKOChance) api.ComboKOChance {
	return api.ComboKOChance{ChancePercent: k.ChancePercent, Guaranteed: k.Guaranteed, DisplayChancePercent: tenths(k.DisplayChancePercentTenths())}
}

func comboScenarioOf(s engine.OpponentScenario) api.ComboOpponentScenario {
	out := api.ComboOpponentScenario{Actions: []api.ComboOpponentAction{}, SurviveChancePercent: s.SurviveChancePercent}
	for _, a := range s.Actions {
		out.Actions = append(out.Actions, api.ComboOpponentAction{Step: a.Step, Timing: api.ComboOpponentTiming(a.Timing),
			Damage: comboCalcResultOf(a.Damage), SurviveChancePercent: a.SurviveChancePercent})
	}
	return out
}

func comboResultOf(r engine.ComboResult) api.ComboResult {
	out := api.ComboResult{Steps: []api.ComboStepResult{}, Unsupported: []api.UnsupportedMark{}}
	for _, s := range r.Steps {
		st := api.ComboStepResult{
			MoveId: s.MoveID, SelfStartRanks: comboRanksOf(s.SelfStartRanks), SelfEndRanks: comboRanksOf(s.SelfEndRanks),
			OpponentStartRanks: comboRanksOf(s.OpponentStartRanks), OpponentEndRanks: comboRanksOf(s.OpponentEndRanks),
			SelfSpeed: s.SelfSpeed, OpponentSpeed: s.OpponentSpeed, SelfPriority: s.SelfPriority, OpponentPriority: s.OpponentPriority,
			Order: api.ComboTurnOrder(s.Order),
			Line: api.ComboSpeedLine{SelfSpeed: s.Line.SelfSpeed,
				SelfFirst:     api.ComboSpeedRange{Min: s.Line.SelfFirst.Min, Max: s.Line.SelfFirst.Max},
				OpponentFirst: api.ComboSpeedRange{Min: s.Line.OpponentFirst.Min, Max: s.Line.OpponentFirst.Max}},
		}
		if s.Damage != nil {
			d := comboCalcResultOf(*s.Damage)
			st.Damage = &d
		}
		out.Steps = append(out.Steps, st)
	}
	minT, maxT := r.Total.DisplayPercentRangeTenths()
	out.Total = api.ComboTotal{MinDamage: r.Total.MinDamage, MaxDamage: r.Total.MaxDamage, DefenderHP: r.Total.DefenderHP,
		MinPercent: tenths(minT), MaxPercent: tenths(maxT), Ko: comboKOOf(r.Total.KO), KoByStep: []api.ComboKOChance{}}
	for _, k := range r.Total.KOByStep {
		out.Total.KoByStep = append(out.Total.KoByStep, comboKOOf(k))
	}
	out.Speed = api.ComboSpeed{FirstSelfSpeed: r.Speed.FirstSelfSpeed, LastSelfSpeed: r.Speed.LastSelfSpeed,
		OpponentSpeed: r.Speed.OpponentSpeed, OpponentSpeedKnown: r.Speed.OpponentSpeedKnown}
	if a := r.OpponentAttack; a != nil {
		out.OpponentAttack = &api.ComboOpponentAttack{SelfHp: a.SelfHP, MinActions: a.MinActions, MaxActions: a.MaxActions,
			Fast: comboScenarioOf(a.Fast), Slow: comboScenarioOf(a.Slow)}
	}
	for _, m := range r.Unsupported {
		out.Unsupported = append(out.Unsupported, api.UnsupportedMark{Target: string(m.Target), Reason: string(m.Reason), Id: m.ID})
	}
	return out
}

func boolOf(v bool) *bool { return &v }

func comboCases() []comboCase {
	hitter := indiv{speciesKey: speciesAttacker, natureID: natureAtkUp, sp: engine.Stats{Atk: 32, Spe: 32}}       // 素早さ 90+20+32 = 142
	slowFoe := indiv{speciesKey: speciesDefender, natureID: natureNeutral, sp: engine.Stats{HP: 32, Def: 32}}     // 素早さ 70
	fastFoe := indiv{speciesKey: speciesLeaf, natureID: natureNeutral, sp: engine.Stats{HP: 2, Atk: 32, Spe: 32}} // 素早さ 137
	return []comboCase{
		{name: "攻撃技 → 先制技(相手の素早さは防御側から)", attacker: hitter, defender: fastFoe,
			steps: []comboStepCase{{moveID: movePhysical}, {moveID: moveSneak}}, opponentSpeed: "from_defender"},
		{name: "変化技 → 攻撃技(相手の素早さは分からない・既定)", attacker: hitter, defender: slowFoe,
			steps: []comboStepCase{{moveID: moveDance}, {moveID: movePhysical}}},
		{name: "unknown を明示", attacker: hitter, defender: slowFoe,
			steps: []comboStepCase{{moveID: moveDance}, {moveID: movePhysical}}, opponentSpeed: "unknown"},
		{name: "相手の技あり・3 手・素早さ +1 の攻撃技", attacker: hitter, defender: fastFoe,
			steps:        []comboStepCase{{moveID: moveCharge}, {moveID: movePhysical}, {moveID: moveSneak}},
			opponentMove: moveSpecial, opponentSpeed: "from_defender"},
		{name: "相手の技あり・unknown(行動回数が分かれる)", attacker: hitter, defender: fastFoe,
			steps: []comboStepCase{{moveID: moveSelfDrop}, {moveID: movePhysical}}, opponentMove: movePhysical},
		{name: "相手の素早さを下げる技 → 攻撃技", attacker: hitter, defender: fastFoe,
			steps: []comboStepCase{{moveID: moveFoeSlow}, {moveID: movePhysical}}, opponentSpeed: "from_defender"},
		{name: "トリックルーム・追い風(両側)", attacker: hitter, defender: fastFoe,
			steps: []comboStepCase{{moveID: movePhysical}, {moveID: movePhysical}}, opponentSpeed: "from_defender",
			speedField: &engine.ComboSpeedField{TrickRoom: true, SelfTailwind: true, OpponentTailwind: true}},
		{name: "急所・多段の回数・場", attacker: hitter, defender: slowFoe,
			field:       map[string]any{"weather": "sun", "defenderScreens": map[string]any{"reflect": true}},
			engineField: engine.Field{Weather: engine.WeatherSun, DefenderScreens: engine.Screens{Reflect: true}},
			steps:       []comboStepCase{{moveID: movePhysical, critical: boolOf(true)}, {moveID: moveBulletMulti, hits: hitsOf(4)}}},
		{name: "1 手だけ", attacker: hitter, defender: slowFoe, steps: []comboStepCase{{moveID: movePhysical}}},
		{name: "未対応の印(テラスタイプ)", attacker: hitter,
			defender: indiv{speciesKey: speciesDefender, natureID: natureNeutral, tera: engine.TypeFire},
			steps:    []comboStepCase{{moveID: movePhysical}, {moveID: movePhysical}}},
		{name: "1 手目の開始時のランクと状態異常", attacker: indiv{speciesKey: speciesAttacker, natureID: natureAtkUp,
			sp: engine.Stats{Atk: 32}, ranks: engine.Ranks{Atk: 5, Spe: -1}, status: engine.StatusParalysis},
			defender: slowFoe, steps: []comboStepCase{{moveID: moveDance}, {moveID: moveDance}, {moveID: movePhysical}}},
	}
}

// K1: HTTP の応答は engine.CalcCombo の結果の写し。
func TestCalcComboHTTPMatchesEngine(t *testing.T) {
	store := newComboStore(t)
	h := NewHandler(store, nil)
	for _, c := range comboCases() {
		t.Run(c.name, func(t *testing.T) {
			rec := post(t, h, pathCombo, mustJSON(t, c.httpBody()), true)
			var got api.ComboResult
			decodeInto(t, rec, &got)
			res, err := engine.CalcCombo(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.CalcCombo: %v", err)
			}
			if want := comboResultOf(res); !reflect.DeepEqual(got, want) {
				t.Errorf("応答 = %+v\nwant %+v", got, want)
			}
		})
	}
}

// K1: 既定値(opponentSpeed・speedField・critical・hits の省略)は明示した既定と同じ結果。
func TestCalcComboHTTPDefaults(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	c := comboCases()[1]
	omitted := post(t, h, pathCombo, mustJSON(t, c.httpBody()), true)
	explicit := c.httpBody()
	explicit["opponentSpeed"] = "unknown"
	explicit["speedField"] = map[string]any{"trickRoom": false, "selfTailwind": false, "opponentTailwind": false}
	explicit["steps"] = []any{map[string]any{"moveId": moveDance, "critical": false}, map[string]any{"moveId": movePhysical, "critical": false}}
	explicitRec := post(t, h, pathCombo, mustJSON(t, explicit), true)
	if omitted.Code != http.StatusOK || explicitRec.Code != http.StatusOK {
		t.Fatalf("status = %d / %d; body=%s / %s", omitted.Code, explicitRec.Code, omitted.Body, explicitRec.Body)
	}
	if omitted.Body.String() != explicitRec.Body.String() {
		t.Errorf("省略と明示の既定で応答が違う:\n省略 %s\n明示 %s", omitted.Body, explicitRec.Body)
	}
}

// K2: null の出し分け(キーを省略しない)と unsupported の空配列。
func TestCalcComboHTTPNullFields(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	c := comboCases()[1] // 変化技 → 攻撃技・unknown・相手の技なし
	rec := post(t, h, pathCombo, mustJSON(t, c.httpBody()), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	var raw struct {
		Steps          []map[string]json.RawMessage `json:"steps"`
		Speed          map[string]json.RawMessage   `json:"speed"`
		OpponentAttack *json.RawMessage             `json:"opponentAttack"`
		Unsupported    json.RawMessage              `json:"unsupported"`
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if v, ok := keys["opponentAttack"]; !ok || string(v) != "null" {
		t.Errorf("opponentAttack = %s(ある=%v), want null(キーは省かない)", v, ok)
	}
	if string(raw.Unsupported) != "[]" {
		t.Errorf("unsupported = %s, want []", raw.Unsupported)
	}
	if len(raw.Steps) != 2 {
		t.Fatalf("steps の件数 = %d, want 2", len(raw.Steps))
	}
	if v, ok := raw.Steps[0]["damage"]; !ok || string(v) != "null" {
		t.Errorf("変化技の手の damage = %s(ある=%v), want null", v, ok)
	}
	if v, ok := raw.Steps[1]["damage"]; !ok || string(v) == "null" {
		t.Errorf("攻撃技の手の damage = %s(ある=%v), want CalcResult", v, ok)
	}
	for i, s := range raw.Steps {
		if v, ok := s["opponentSpeed"]; !ok || string(v) != "0" {
			t.Errorf("steps[%d].opponentSpeed = %s(ある=%v), want 0(unknown。WASM と同じ形)", i, v, ok)
		}
	}
	if v, ok := raw.Speed["opponentSpeed"]; !ok || string(v) != "0" {
		t.Errorf("speed.opponentSpeed = %s(ある=%v), want 0(unknown)", v, ok)
	}
	if v, ok := raw.Speed["opponentSpeedKnown"]; !ok || string(v) != "false" {
		t.Errorf("speed.opponentSpeedKnown = %s(ある=%v), want false(unknown)", v, ok)
	}
}

// K3(AC-13): 各手の damage は、その手の開始時のランクを入れた POST /api/calc の応答と同じ。
func TestCalcComboStepsMatchCalcDamage(t *testing.T) {
	store := newComboStore(t)
	h := NewHandler(store, nil)
	for _, c := range comboCases() {
		t.Run(c.name, func(t *testing.T) {
			rec := post(t, h, pathCombo, mustJSON(t, c.httpBody()), true)
			var combo api.ComboResult
			decodeInto(t, rec, &combo)
			if len(combo.Steps) != len(c.steps) {
				t.Fatalf("steps の件数 = %d, want %d", len(combo.Steps), len(c.steps))
			}
			for i, step := range combo.Steps {
				if step.Damage == nil {
					if store.moves[c.steps[i].moveID].Category != engine.CategoryStatus {
						t.Errorf("%d 手目(攻撃技 %s)の damage が null", i+1, step.MoveId)
					}
					continue
				}
				att, def := c.attacker, c.defender
				att.ranks = engine.Ranks{Atk: step.SelfStartRanks.Atk, Def: step.SelfStartRanks.Def, SpA: step.SelfStartRanks.Spa,
					SpD: step.SelfStartRanks.Spd, Spe: step.SelfStartRanks.Spe}
				def.ranks = engine.Ranks{Atk: step.OpponentStartRanks.Atk, Def: step.OpponentStartRanks.Def, SpA: step.OpponentStartRanks.Spa,
					SpD: step.OpponentStartRanks.Spd, Spe: step.OpponentStartRanks.Spe}
				body := map[string]any{"format": "single", "attacker": att.http(), "defender": def.http(), "moveId": c.steps[i].moveID}
				if c.field != nil {
					body["field"] = c.field
				}
				if cr := c.steps[i].critical; cr != nil {
					body["options"] = map[string]any{"critical": *cr}
				}
				if n := c.steps[i].hits; n != nil {
					body["battleState"] = map[string]any{"hits": *n}
				}
				var single api.CalcResult
				decodeInto(t, post(t, h, "/api/calc", mustJSON(t, body), true), &single)
				if !reflect.DeepEqual(single.Rolls, step.Damage.Rolls) || !reflect.DeepEqual(single.Ko, step.Damage.Ko) {
					t.Errorf("%d 手目 %s: combo の rolls・ko = %v・%+v, /api/calc = %v・%+v",
						i+1, step.MoveId, step.Damage.Rolls, step.Damage.Ko, single.Rolls, single.Ko)
				}
				if !reflect.DeepEqual(single, *step.Damage) {
					t.Errorf("%d 手目 %s: combo の damage = %+v, /api/calc = %+v", i+1, step.MoveId, *step.Damage, single)
				}
			}
		})
	}
}

// K4(AC-16): 入力の検査とエラーの語彙(/api/calc と同じ形。新しい code を足さない)。
func TestCalcComboHTTPErrors(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	base := comboCases()[0]
	with := func(edit func(b map[string]any)) map[string]any {
		b := base.httpBody()
		edit(b)
		return b
	}
	stepsOf := func(ids ...string) []any {
		out := make([]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, map[string]any{"moveId": id})
		}
		return out
	}
	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{"手 0 件", with(func(b map[string]any) { b["steps"] = []any{} }), http.StatusBadRequest, "invalid_input"},
		{"手の省略", with(func(b map[string]any) { delete(b, "steps") }), http.StatusBadRequest, "invalid_input"},
		{"手 4 件(上限 3)", with(func(b map[string]any) {
			b["steps"] = stepsOf(movePhysical, movePhysical, movePhysical, movePhysical)
		}), http.StatusBadRequest, "invalid_input"},
		{"ランク変化の無い変化技の手", with(func(b map[string]any) { b["steps"] = stepsOf(moveStatus, movePhysical) }),
			http.StatusBadRequest, "invalid_input"},
		{"確率 100% 未満の変化だけの変化技の手", with(func(b map[string]any) { b["steps"] = stepsOf(moveMaybeDance, movePhysical) }),
			http.StatusBadRequest, "invalid_input"},
		{"変化技の相手の技", with(func(b map[string]any) { b["opponentMoveId"] = moveDance }), http.StatusBadRequest, "invalid_input"},
		{"範囲の多段でない技に回数", with(func(b map[string]any) {
			b["steps"] = []any{map[string]any{"moveId": movePhysical, "hits": 2}}
		}), http.StatusBadRequest, "invalid_input"},
		{"多段の回数が範囲外", with(func(b map[string]any) {
			b["steps"] = []any{map[string]any{"moveId": moveBulletMulti, "hits": 6}}
		}), http.StatusBadRequest, "invalid_input"},
		{"多段の回数 0(既定として通さない)", with(func(b map[string]any) {
			b["steps"] = []any{map[string]any{"moveId": moveBulletMulti, "hits": 0}}
		}), http.StatusBadRequest, "invalid_input"},
		{"多段の回数 -1", with(func(b map[string]any) {
			b["steps"] = []any{map[string]any{"moveId": moveBulletMulti, "hits": -1}}
		}), http.StatusBadRequest, "invalid_input"},
		{"SP の欠落", with(func(b map[string]any) { delete(b["attacker"].(map[string]any), "sp") }), http.StatusBadRequest, "invalid_input"},
		{"未知の技(手)", with(func(b map[string]any) { b["steps"] = stepsOf(movePhysical, "test-nothing") }),
			http.StatusBadRequest, "unknown_move"},
		{"未知の技(相手の技)", with(func(b map[string]any) { b["opponentMoveId"] = "test-nothing" }), http.StatusBadRequest, "unknown_move"},
		{"未知の種族", with(func(b map[string]any) { b["defender"].(map[string]any)["speciesKey"] = speciesUnknown }),
			http.StatusBadRequest, "unknown_species"},
		{"未知の opponentSpeed", with(func(b map[string]any) { b["opponentSpeed"] = "fast" }), http.StatusBadRequest, "invalid_enum"},
		{"未知の format", with(func(b map[string]any) { b["format"] = "triple" }), http.StatusBadRequest, "invalid_enum"},
		{"未知の天候", with(func(b map[string]any) { b["field"] = map[string]any{"weather": "fog"} }), http.StatusBadRequest, "invalid_enum"},
		{"手の未知のフィールド", with(func(b map[string]any) { b["steps"] = []any{map[string]any{"moveId": movePhysical, "boosts": 1}} }),
			http.StatusBadRequest, "unknown_field"},
		{"speedField の未知のフィールド", with(func(b map[string]any) { b["speedField"] = map[string]any{"gravity": true} }),
			http.StatusBadRequest, "unknown_field"},
		{"壊れた JSON", nil, http.StatusBadRequest, "invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"format":`)
			if tt.body != nil {
				body = mustJSON(t, tt.body)
			}
			rec := post(t, h, pathCombo, body, false)
			assertError(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}

// K4(AC-16): 手の件数の範囲外は ID の解決より前(未知の個体・技を含んでも invalid_input で、マスタを引かない)。
func TestCalcComboStepCountCheckedBeforeStoreLookup(t *testing.T) {
	unknown := indiv{speciesKey: speciesUnknown, natureID: "test-nothing"}.http()
	many := make([]any, 0, engine.MaxComboSteps+1)
	for range engine.MaxComboSteps + 1 {
		many = append(many, map[string]any{"moveId": "test-nothing"})
	}
	tests := []struct {
		name  string
		steps []any
	}{
		{"0 件", []any{}},
		{"上限 + 1 件", many},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &countingStore{inner: newComboStore(t)}
			h := NewHandler(store, nil)
			body := map[string]any{"format": "single", "attacker": unknown, "defender": unknown, "steps": tt.steps,
				"opponentMoveId": "test-nothing"}
			rec := post(t, h, pathCombo, mustJSON(t, body), false)
			assertError(t, rec, http.StatusBadRequest, "invalid_input")
			if store.lookups != 0 || store.chartGet != 0 {
				t.Errorf("マスタ参照 %d 回・相性表 %d 回(want 0。手の件数の検査は ID 解決より前)", store.lookups, store.chartGet)
			}
		})
	}
}

// K4: 上限ちょうど(3 手)は受け付ける。
func TestCalcComboAcceptsMaxSteps(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	c := comboCases()[0]
	c.steps = nil
	for range engine.MaxComboSteps {
		c.steps = append(c.steps, comboStepCase{moveID: movePhysical})
	}
	assertStatusOK(t, post(t, h, pathCombo, mustJSON(t, c.httpBody()), true), "3 手")
}

// K5(AC-17): 計算イベントを発行しない(record・team・NATS が止まっていても計算は成功する)。
func TestCalcComboDoesNotPublishEvents(t *testing.T) {
	pub := &fakePublisher{}
	h := NewHandler(newComboStore(t), pub)
	for _, c := range comboCases() {
		assertStatusOK(t, post(t, h, pathCombo, mustJSON(t, c.httpBody()), true), c.name)
	}
	if len(pub.calls) != 0 {
		t.Errorf("計算イベントを %d 件発行した(want 0。ADR-0253)", len(pub.calls))
	}
}

// K5(AC-17): publisher なし(NATS 未接続)でも成功する。
func TestCalcComboSucceedsWithoutPublisher(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	assertStatusOK(t, post(t, h, pathCombo, mustJSON(t, comboCases()[3].httpBody()), true), "publisher なし")
}

// K5: マスタの準備中は 503 master_unavailable、準備後は 200(ADR-0204 §3)。
func TestCalcComboDeferredHandler(t *testing.T) {
	sw := &switchableStore{store: newComboStore(t)}
	h := NewDeferredHandler(sw.current, nil)
	body := mustJSON(t, comboCases()[0].httpBody())
	assertError(t, post(t, h, pathCombo, body, true), http.StatusServiceUnavailable, "master_unavailable")
	sw.ready.Store(true)
	assertStatusOK(t, post(t, h, pathCombo, body, true), "準備後")
}

// K6: ヘッダ必須。
func TestCalcComboMissingHeaders(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	header := validHeaders()
	header.Del("X-Session-Id")
	rec := serve(t, h, http.MethodPost, pathCombo, header, mustJSON(t, comboCases()[0].httpBody()))
	assertError(t, rec, http.StatusBadRequest, "missing_header")
}

// K6: 契約に操作があり、検証ヘルパーが空振りしない(妥当な本文は通り、必須の欠けた本文・語彙外の値は落ちる)。
func TestCalcComboContractHelperIsNotVacuous(t *testing.T) {
	const line = `"line":{"selfSpeed":142,"selfFirst":{"min":0,"max":141},"opponentFirst":{"min":143,"max":10000}}`
	const ranks = `{"atk":0,"def":0,"spa":0,"spd":0,"spe":0}`
	step := `{"moveId":"test-dance","selfStartRanks":` + ranks + `,"selfEndRanks":` + ranks + `,"opponentStartRanks":` + ranks +
		`,"opponentEndRanks":` + ranks + `,"damage":null,"selfSpeed":142,"opponentSpeed":0,"selfPriority":0,"opponentPriority":0,` +
		`"order":"depends_on_opponent_speed",` + line + `}`
	total := `"total":{"minDamage":0,"maxDamage":0,"minPercent":0,"maxPercent":0,"defenderHP":200,` +
		`"ko":{"chancePercent":0,"guaranteed":false,"displayChancePercent":0},"koByStep":[{"chancePercent":0,"guaranteed":false,"displayChancePercent":0}]}`
	speed := `"speed":{"firstSelfSpeed":142,"lastSelfSpeed":142,"opponentSpeed":0,"opponentSpeedKnown":false}`
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"妥当な結果", `{"steps":[` + step + `],` + total + `,` + speed + `,"opponentAttack":null,"unsupported":[]}`, false},
		{"opponentAttack の欠落", `{"steps":[` + step + `],` + total + `,` + speed + `,"unsupported":[]}`, true},
		{"order が語彙外", `{"steps":[` + `{"moveId":"x","selfStartRanks":` + ranks + `,"selfEndRanks":` + ranks + `,"opponentStartRanks":` + ranks +
			`,"opponentEndRanks":` + ranks + `,"damage":null,"selfSpeed":1,"opponentSpeed":0,"selfPriority":0,"opponentPriority":0,` +
			`"order":"later",` + line + `}],` + total + `,` + speed + `,"opponentAttack":null,"unsupported":[]}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgainstContract(t, http.MethodPost, pathCombo, validHeaders(), []byte(`{}`),
				http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, []byte(tt.body), false)
			if err == errNoRoute {
				t.Fatalf("契約に %s が無い", pathCombo)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("検証エラー = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// AC-18 の補助(ADR-0253): 応答のキーの集合が WASM の calcCombo の DTO と同じであることを固定する。
// 値まで比べる本体は combo_parity_test.go。
func TestCalcComboResponseKeysMatchWasmShape(t *testing.T) {
	h := NewHandler(newComboStore(t), nil)
	rec := post(t, h, pathCombo, mustJSON(t, comboCases()[3].httpBody()), true) // 相手の技あり・from_defender・3 手
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	object := func(raw json.RawMessage) map[string]json.RawMessage {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("オブジェクトでない: %s", raw)
		}
		return m
	}
	keysOf := func(raw json.RawMessage) []string {
		var out []string
		for k := range object(raw) {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	first := func(raw json.RawMessage) json.RawMessage {
		var a []json.RawMessage
		if err := json.Unmarshal(raw, &a); err != nil || len(a) == 0 {
			t.Fatalf("空でない配列でない: %s", raw)
		}
		return a[0]
	}
	sorted := func(ks ...string) []string { sort.Strings(ks); return ks }
	top := object(rec.Body.Bytes())
	step := first(top["steps"])
	oa := top["opponentAttack"]
	fast := object(oa)["fast"]
	tests := []struct {
		name string
		raw  json.RawMessage
		want []string
	}{
		{"ComboResult", rec.Body.Bytes(), sorted("steps", "total", "speed", "opponentAttack", "unsupported")},
		{"steps[]", step, sorted("moveId", "selfStartRanks", "selfEndRanks", "opponentStartRanks", "opponentEndRanks", "damage",
			"selfSpeed", "opponentSpeed", "selfPriority", "opponentPriority", "order", "line")},
		{"steps[].line", object(step)["line"], sorted("selfSpeed", "selfFirst", "opponentFirst")},
		{"steps[].selfStartRanks", object(step)["selfStartRanks"], sorted("atk", "def", "spa", "spd", "spe")},
		{"total", top["total"], sorted("minDamage", "maxDamage", "defenderHP", "minPercent", "maxPercent", "ko", "koByStep")},
		{"total.ko", object(top["total"])["ko"], sorted("chancePercent", "guaranteed", "displayChancePercent")},
		{"speed", top["speed"], sorted("firstSelfSpeed", "lastSelfSpeed", "opponentSpeed", "opponentSpeedKnown")},
		{"opponentAttack", oa, sorted("selfHp", "fast", "slow", "minActions", "maxActions")},
		{"opponentAttack.fast", fast, sorted("actions", "surviveChancePercent")},
		{"opponentAttack.fast.actions[]", first(object(fast)["actions"]), sorted("step", "timing", "damage", "surviveChancePercent")},
	}
	for _, tt := range tests {
		if got := keysOf(tt.raw); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s のキー = %v, want %v(WASM の calcCombo と同じ形)", tt.name, got, tt.want)
		}
	}
}
