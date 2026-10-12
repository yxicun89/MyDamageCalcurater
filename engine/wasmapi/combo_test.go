package wasmapi_test

// CB-5(ADR-0179 追記「CB-5」。docs/plan/combo-calc.md §5 AC-18): 連続技 calcCombo の境界。
// 計算は engine.CalcCombo の素通しで、ここは DTO・検証・エラー写像・出力の写しを固定する。

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
)

func danceMove() map[string]any {
	return map[string]any{"id": "testdance", "type": "normal", "category": "status", "priority": 0,
		"effect": map[string]any{"chance": 100, "target": "self", "stages": map[string]any{"atk": 1, "spe": 1}}}
}

func baseCombo() map[string]any {
	return map[string]any{
		"format":   "single",
		"attacker": attackerIndividual(),
		"defender": defenderIndividual(),
		"field":    map[string]any{"weather": "none", "terrain": "none"},
		"steps": []any{
			map[string]any{"move": danceMove()},
			map[string]any{"move": bodySlam(), "critical": true},
		},
		"opponentSpeed": "from_defender",
		"typeChart":     typeChartRequestValue(),
	}
}

func callCombo(t *testing.T, req map[string]any) string {
	t.Helper()
	return invoke(t, "calcCombo", mustJSON(t, req))
}

type comboRanksView struct{ Atk, Def, SpA, SpD, Spe int }

type comboView struct {
	Steps []struct {
		MoveID           string          `json:"moveId"`
		SelfStartRanks   comboRanksView  `json:"selfStartRanks"`
		SelfEndRanks     comboRanksView  `json:"selfEndRanks"`
		OpponentEndRanks comboRanksView  `json:"opponentEndRanks"`
		Damage           json.RawMessage `json:"damage"`
		SelfSpeed        int             `json:"selfSpeed"`
		OpponentSpeed    int             `json:"opponentSpeed"`
		Order            string          `json:"order"`
		SelfPriority     int             `json:"selfPriority"`
		OpponentPriority int             `json:"opponentPriority"`
		Line             struct {
			SelfSpeed     int                    `json:"selfSpeed"`
			SelfFirst     struct{ Min, Max int } `json:"selfFirst"`
			OpponentFirst struct{ Min, Max int } `json:"opponentFirst"`
		} `json:"line"`
	} `json:"steps"`
	Total struct {
		MinDamage  int         `json:"minDamage"`
		MaxDamage  int         `json:"maxDamage"`
		DefenderHP int         `json:"defenderHP"`
		MinPercent json.Number `json:"minPercent"`
		MaxPercent json.Number `json:"maxPercent"`
		KO         struct {
			ChancePercent        float64     `json:"chancePercent"`
			Guaranteed           bool        `json:"guaranteed"`
			DisplayChancePercent json.Number `json:"displayChancePercent"`
		} `json:"ko"`
		KOByStep []struct {
			ChancePercent float64 `json:"chancePercent"`
			Guaranteed    bool    `json:"guaranteed"`
		} `json:"koByStep"`
	} `json:"total"`
	Speed struct {
		FirstSelfSpeed     int  `json:"firstSelfSpeed"`
		LastSelfSpeed      int  `json:"lastSelfSpeed"`
		OpponentSpeed      int  `json:"opponentSpeed"`
		OpponentSpeedKnown bool `json:"opponentSpeedKnown"`
	} `json:"speed"`
	OpponentAttack *struct {
		SelfHP     int `json:"selfHp"`
		MinActions int `json:"minActions"`
		MaxActions int `json:"maxActions"`
		Fast, Slow struct {
			Actions []struct {
				Step                 int             `json:"step"`
				Timing               string          `json:"timing"`
				Damage               json.RawMessage `json:"damage"`
				SurviveChancePercent float64         `json:"surviveChancePercent"`
			} `json:"actions"`
			SurviveChancePercent float64 `json:"surviveChancePercent"`
		}
	} `json:"opponentAttack"`
	Unsupported []markView `json:"unsupported"`
}

func decodeCombo(t *testing.T, resp string) comboView {
	t.Helper()
	var v comboView
	if err := json.Unmarshal(decodeSuccess(t, resp), &v); err != nil {
		t.Fatalf("結果を読めない: %v\n%s", err, resp)
	}
	return v
}

// 境界の結果が engine.CalcCombo と一致する(素通し)。相手の技・素早さの場も通す。
func TestCalcComboMatchesEngine(t *testing.T) {
	req := baseCombo()
	req["opponentMove"] = bodySlam()
	req["speedField"] = map[string]any{"trickRoom": false, "selfTailwind": true, "opponentTailwind": false}
	got := decodeCombo(t, callCombo(t, req))

	snorlax := engine.Species{Key: "snorlax", Types: []engine.Type{engine.TypeNormal}, BaseStats: engine.Stats{HP: 160, Atk: 110, Def: 65, SpA: 65, SpD: 110, Spe: 30}}
	blissey := engine.Species{Key: "blissey", Types: []engine.Type{engine.TypeNormal}, BaseStats: engine.Stats{HP: 255, Atk: 10, Def: 10, SpA: 75, SpD: 135, Spe: 55}}
	slam := engine.Move{ID: "bodyslam", NameJa: "テストわざ1", Type: engine.TypeNormal, Category: engine.CategoryPhysical, Power: 85}
	dance := engine.Move{ID: "testdance", Type: engine.TypeNormal, Category: engine.CategoryStatus,
		Effect: &engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatAtk: 1, engine.StatSpe: 1}}}
	want, err := engine.CalcCombo(engine.ComboInput{
		Format:       engine.FormatSingle,
		Attacker:     engine.Individual{Species: snorlax, Level: 50, Nature: engine.Nature{Plus: engine.StatAtk, Minus: engine.StatSpA}, SP: engine.Stats{Atk: 32}, Status: engine.StatusNone},
		Defender:     engine.Individual{Species: blissey, Level: 50, Status: engine.StatusNone},
		TypeChart:    sharedTypeChart(t),
		Steps:        []engine.ComboStep{{Move: dance}, {Move: slam, Critical: true}},
		OpponentMove: &slam, OpponentSpeed: engine.OpponentSpeedFromDefender,
		SpeedField: engine.ComboSpeedField{SelfTailwind: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 2 || got.Steps[0].MoveID != "testdance" {
		t.Fatalf("手の列が違う: %+v", got.Steps)
	}
	if string(got.Steps[0].Damage) != "null" {
		t.Errorf("変化技の damage は null: %s", got.Steps[0].Damage)
	}
	if got.Steps[0].SelfEndRanks != (comboRanksView{Atk: 1, Spe: 1}) || got.Steps[1].SelfStartRanks != (comboRanksView{Atk: 1, Spe: 1}) {
		t.Errorf("ランクの累積が違う: %+v", got.Steps)
	}
	for i, s := range got.Steps {
		w := want.Steps[i]
		if s.SelfSpeed != w.SelfSpeed || s.OpponentSpeed != w.OpponentSpeed || s.Order != string(w.Order) ||
			s.Line.SelfSpeed != w.Line.SelfSpeed || s.Line.SelfFirst.Min != w.Line.SelfFirst.Min || s.Line.OpponentFirst.Max != w.Line.OpponentFirst.Max {
			t.Errorf("step %d の素早さ・行動順が engine と違う: got=%+v want=%+v", i, s, w)
		}
	}
	if got.Total.MinDamage != want.Total.MinDamage || got.Total.MaxDamage != want.Total.MaxDamage || got.Total.DefenderHP != want.Total.DefenderHP {
		t.Errorf("合計が engine と違う: got=%+v want=%+v", got.Total, want.Total)
	}
	minT, maxT := want.Total.DisplayPercentRangeTenths()
	if tenths(t, "minPercent", got.Total.MinPercent) != minT || tenths(t, "maxPercent", got.Total.MaxPercent) != maxT {
		t.Errorf("表示%%が engine と違う: %v %v want %d %d", got.Total.MinPercent, got.Total.MaxPercent, minT, maxT)
	}
	if got.Total.KO.ChancePercent != want.Total.KO.ChancePercent || got.Total.KO.Guaranteed != want.Total.KO.Guaranteed ||
		tenths(t, "displayChancePercent", got.Total.KO.DisplayChancePercent) != want.Total.KO.DisplayChancePercentTenths() ||
		len(got.Total.KOByStep) != len(want.Total.KOByStep) {
		t.Errorf("倒せる確率が engine と違う: got=%+v want=%+v", got.Total.KO, want.Total.KO)
	}
	if !got.Speed.OpponentSpeedKnown || got.Speed.OpponentSpeed != want.Speed.OpponentSpeed || got.Speed.FirstSelfSpeed != want.Speed.FirstSelfSpeed || got.Speed.LastSelfSpeed != want.Speed.LastSelfSpeed {
		t.Errorf("素早さの要約が違う: got=%+v want=%+v", got.Speed, want.Speed)
	}
	oa := got.OpponentAttack
	if oa == nil || want.OpponentAttack == nil {
		t.Fatalf("opponentAttack が無い: %+v", oa)
	}
	if oa.SelfHP != want.OpponentAttack.SelfHP || oa.MinActions != want.OpponentAttack.MinActions || oa.MaxActions != want.OpponentAttack.MaxActions ||
		len(oa.Fast.Actions) != len(want.OpponentAttack.Fast.Actions) || len(oa.Slow.Actions) != len(want.OpponentAttack.Slow.Actions) ||
		oa.Fast.SurviveChancePercent != want.OpponentAttack.Fast.SurviveChancePercent || oa.Slow.SurviveChancePercent != want.OpponentAttack.Slow.SurviveChancePercent {
		t.Errorf("相手の攻撃が engine と違う: got=%+v want=%+v", oa, want.OpponentAttack)
	}
	if len(oa.Fast.Actions) > 0 && (oa.Fast.Actions[0].Timing == "" || string(oa.Fast.Actions[0].Damage) == "null") {
		t.Errorf("行動の形が違う: %+v", oa.Fast.Actions[0])
	}
}

func TestCalcComboDefaultsAndOmissions(t *testing.T) {
	req := baseCombo()
	delete(req, "opponentSpeed")
	got := decodeCombo(t, callCombo(t, req))
	if got.OpponentAttack != nil {
		t.Errorf("opponentMove が無いのに opponentAttack がある: %+v", got.OpponentAttack)
	}
	if got.Speed.OpponentSpeedKnown || got.Steps[0].Order != "depends_on_opponent_speed" {
		t.Errorf("opponentSpeed 省略は unknown: %+v", got.Speed)
	}
	if got.Unsupported == nil {
		t.Error("unsupported は空配列(null にしない)")
	}
	raw := callCombo(t, req)
	for _, key := range []string{`"opponentAttack":null`, `"unsupported":[`} {
		if !strings.Contains(raw, key) {
			t.Errorf("%s が無い: %s", key, raw)
		}
	}
}

func TestCalcComboEmptyOpponentSpeedMeansUnknown(t *testing.T) {
	a, b := baseCombo(), baseCombo()
	a["opponentSpeed"] = ""
	b["opponentSpeed"] = "unknown"
	delete(b, "format")
	if x, y := callCombo(t, a), callCombo(t, b); x != y {
		t.Errorf("'' と unknown と format 省略は同じ結果:\n%s\n%s", x, y)
	}
}

func TestCalcComboMarks(t *testing.T) {
	t.Run("choice_locked は LocksMove の持ち物で別の技を並べたとき", func(t *testing.T) {
		req := baseCombo()
		req["steps"] = []any{map[string]any{"move": bodySlam()}, map[string]any{"move": map[string]any{"id": "other", "type": "normal", "category": "physical", "power": 60}}}
		sub(t, req, "attacker")["item"] = map[string]any{"id": "testband", "effect": map[string]any{"locksMove": true}}
		got := decodeCombo(t, callCombo(t, req))
		found := false
		for _, m := range got.Unsupported {
			found = found || m.Reason == "choice_locked"
		}
		if !found {
			t.Errorf("choice_locked が無い: %+v", got.Unsupported)
		}
	})
	t.Run("defender_full_hp_assumed は FullHPOnly の持ち物", func(t *testing.T) {
		req := baseCombo()
		req["steps"] = []any{map[string]any{"move": bodySlam()}, map[string]any{"move": bodySlam()}}
		sub(t, req, "defender")["item"] = map[string]any{"id": "testsash", "effect": map[string]any{"fullHPOnly": true}}
		sub(t, req, "defender")["ability"] = map[string]any{"id": "testscale", "effect": map[string]any{"fullHPOnly": true}}
		got := decodeCombo(t, callCombo(t, req))
		found := false
		for _, m := range got.Unsupported {
			found = found || m.Reason == "defender_full_hp_assumed"
		}
		if !found {
			t.Errorf("defender_full_hp_assumed が無い: %+v", got.Unsupported)
		}
	})
}

func TestCalcComboErrors(t *testing.T) {
	step := func(m map[string]any) map[string]any { return map[string]any{"move": m} }
	cases := []struct {
		name string
		edit func(req map[string]any)
		code string
	}{
		{"手が 0 件", func(r map[string]any) { r["steps"] = []any{} }, "invalid_input"},
		{"手が省略", func(r map[string]any) { delete(r, "steps") }, "invalid_input"},
		{"手が 4 件", func(r map[string]any) {
			r["steps"] = []any{step(bodySlam()), step(bodySlam()), step(bodySlam()), step(bodySlam())}
		}, "invalid_input"},
		{"ランク変化の無い変化技", func(r map[string]any) {
			r["steps"] = []any{step(map[string]any{"id": "noop", "type": "normal", "category": "status"})}
		}, "invalid_input"},
		{"相手の技が変化技", func(r map[string]any) { r["opponentMove"] = danceMove() }, "invalid_input"},
		{"opponentSpeed が語彙外", func(r map[string]any) { r["opponentSpeed"] = "fromDefender" }, "invalid_enum"},
		{"hits が 0", func(r map[string]any) {
			r["steps"] = []any{map[string]any{"move": bodySlam(), "hits": 0}}
		}, "invalid_input"},
		{"hits が範囲外(単発技)", func(r map[string]any) {
			r["steps"] = []any{map[string]any{"move": bodySlam(), "hits": 99}}
		}, "invalid_input"},
		{"手の未知のキー", func(r map[string]any) {
			r["steps"] = []any{map[string]any{"move": bodySlam(), "boosts": map[string]any{}}}
		}, "unknown_field"},
		{"speedField の未知のキー", func(r map[string]any) { r["speedField"] = map[string]any{"sun": true} }, "unknown_field"},
		{"トップレベルの未知のキー", func(r map[string]any) { r["move"] = bodySlam() }, "unknown_field"},
		{"effect の target が語彙外", func(r map[string]any) {
			m := danceMove()
			m["effect"].(map[string]any)["target"] = "ally"
			r["steps"] = []any{step(m)}
		}, "invalid_enum"},
		{"effect の stages のキーが語彙外", func(r map[string]any) {
			m := danceMove()
			m["effect"].(map[string]any)["stages"] = map[string]any{"acc": 1}
			r["steps"] = []any{step(m)}
		}, "invalid_enum"},
		{"effect の変化量が 0", func(r map[string]any) {
			m := danceMove()
			m["effect"].(map[string]any)["stages"] = map[string]any{"atk": 0}
			r["steps"] = []any{step(m)}
		}, "invalid_input"},
		{"effect の確率が範囲外", func(r map[string]any) {
			m := danceMove()
			m["effect"].(map[string]any)["chance"] = 101
			r["steps"] = []any{step(m)}
		}, "invalid_input"},
		{"typeChart が無い", func(r map[string]any) { delete(r, "typeChart") }, "type_chart_missing"},
		{"攻撃側の SP が範囲外", func(r map[string]any) { sub(t, r, "attacker")["sp"] = stats(0, 33, 0, 0, 0, 0) }, "invalid_input"},
		{"format が語彙外", func(r map[string]any) { r["format"] = "triple" }, "invalid_enum"},
		{"相手の技のタイプが語彙外", func(r map[string]any) {
			m := bodySlam()
			m["type"] = "sound"
			r["opponentMove"] = m
		}, "invalid_enum"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := baseCombo()
			c.edit(req)
			if e := decodeError(t, callCombo(t, req)); e.Code != c.code {
				t.Errorf("code = %q, want %q (%s)", e.Code, c.code, e.Message)
			}
		})
	}
	if e := decodeError(t, invoke(t, "calcCombo", "[]")); e.Code != "invalid_json" {
		t.Errorf("配列は invalid_json: %q", e.Code)
	}
}

// 3 手ちょうどは成功し、effect を calc に付けても結果は変わらない(Web は同じ技の JSON を渡す)。
func TestCalcComboThreeStepsAndEffectIgnoredByCalc(t *testing.T) {
	req := baseCombo()
	req["steps"] = []any{map[string]any{"move": danceMove()}, map[string]any{"move": danceMove()}, map[string]any{"move": bodySlam()}}
	got := decodeCombo(t, callCombo(t, req))
	if len(got.Steps) != 3 || got.Steps[2].SelfStartRanks != (comboRanksView{Atk: 2, Spe: 2}) {
		t.Errorf("3 手・ランク累積: %+v", got.Steps)
	}

	plain, withEffect := baseCalc(), baseCalc()
	m := bodySlam()
	m["effect"] = map[string]any{"chance": 10, "target": "self", "stages": map[string]any{"atk": 1}}
	withEffect["move"] = m
	if a, b := invoke(t, "calc", mustJSON(t, plain)), invoke(t, "calc", mustJSON(t, withEffect)); a != b {
		t.Errorf("calc は effect を読まない:\n%s\n%s", a, b)
	}
	// 持ち物・特性の locksMove / fullHPOnly も calc は読まない。
	e1, e2 := baseCalc(), baseCalc()
	sub(t, e2, "attacker")["item"] = map[string]any{"id": "x", "effect": map[string]any{"locksMove": true, "fullHPOnly": true}}
	sub(t, e1, "attacker")["item"] = map[string]any{"id": "x", "effect": map[string]any{}}
	if a, b := invoke(t, "calc", mustJSON(t, e1)), invoke(t, "calc", mustJSON(t, e2)); a != b {
		t.Errorf("calc は locksMove/fullHPOnly を読まない")
	}
}

func TestCalcComboDeterministicNoTrailingNewline(t *testing.T) {
	req := baseCombo()
	req["opponentMove"] = bodySlam()
	a, b := callCombo(t, req), callCombo(t, req)
	if a != b || strings.HasSuffix(a, "\n") {
		t.Error("同じ入力は同じバイト列で、改行で終わらない")
	}
	var anyv map[string]any
	if err := json.Unmarshal([]byte(a), &anyv); err != nil || !reflect.DeepEqual(reflect.TypeOf(anyv["result"]).Kind().String(), "map") {
		t.Errorf("result は object: %v", err)
	}
}

// 公開 API(pokedex)の Move.effect は PascalCase({"Chance":100,"Target":"self","Stages":{...}})で返る。
// それをそのまま技 DTO の effect に渡して受理され、camelCase で渡したときと同じ結果になる(ADR-0253。CB-4b)。
func TestCalcComboAcceptsPublicPascalCaseMoveEffect(t *testing.T) {
	camel := baseCombo()
	pascal := baseCombo()
	// 公開 API の JSON をそのまま JSON 文字列として差し込む(キーの大文字小文字だけが違う)。
	var publicEffect map[string]any
	if err := json.Unmarshal([]byte(`{"Chance":100,"Target":"self","Stages":{"atk":1,"spe":1}}`), &publicEffect); err != nil {
		t.Fatal(err)
	}
	m := danceMove()
	m["effect"] = publicEffect
	pascal["steps"].([]any)[0].(map[string]any)["move"] = m

	want := callCombo(t, camel)
	got := callCombo(t, pascal)
	if strings.Contains(got, `"error":{`) {
		t.Fatalf("PascalCase の effect が受理されない: %s", got)
	}
	if got != want {
		t.Errorf("PascalCase と camelCase で結果が違う:\n%s\n%s", got, want)
	}
	// 効果が実際に読まれている(ランクが累積している)ことで、無視されて一致した場合を除く。
	v := decodeCombo(t, got)
	if v.Steps[0].SelfEndRanks != (comboRanksView{Atk: 1, Spe: 1}) {
		t.Errorf("effect が読まれていない: %+v", v.Steps[0].SelfEndRanks)
	}
	// 相手側の技(opponentMove)の effect も同じ。ランクを下げる技(Target=target)を PascalCase で。
	pascalFoe := baseCombo()
	foe := danceMove()
	foe["effect"] = map[string]any{"Chance": 100, "Target": "target", "Stages": map[string]any{"spe": -1}}
	pascalFoe["steps"].([]any)[0].(map[string]any)["move"] = foe
	camelFoe := baseCombo()
	cf := danceMove()
	cf["effect"] = map[string]any{"chance": 100, "target": "target", "stages": map[string]any{"spe": -1}}
	camelFoe["steps"].([]any)[0].(map[string]any)["move"] = cf
	if a, b := callCombo(t, pascalFoe), callCombo(t, camelFoe); a != b || strings.Contains(a, `"error":{`) {
		t.Errorf("target 効果の PascalCase が camelCase と違う:\n%s\n%s", a, b)
	}
}
