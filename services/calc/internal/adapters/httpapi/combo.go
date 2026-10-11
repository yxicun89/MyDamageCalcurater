package httpapi

// POST /api/calc/combo(連続技。ADR-0253・ADR-0179 追記 CB-3)。
//
// ここは 厳格デコード → 手の件数(ID の解決より前。ADR-0208 §3)→ 各手の hits の下限(指定したら 1 以上。
// battleStateFrom と同じ規則。0 を engine の「既定」として通さない)と、生成型 ⇄ app の型の変換・結果の写しだけを持つ。
// 列挙の検証・ID 解決・個体の検証・engine.CalcCombo の手順は app.Service.Combo が持つ(ADR-0811)。
// 独自の式・丸めを持たない(表示%は engine の tenths を 10 で割るだけ)。計算イベントは発行しない(絶対ルール5。ADR-0253 §4)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
)

// CalcCombo は POST /api/calc/combo。
func (s *Server) CalcCombo(ctx *echo.Context, params api.CalcComboParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.ComboRequest
	if err := decodeStrict(limitedBody(ctx), &req, "attacker", "defender"); err != nil {
		return err
	}
	if err := checkComboLimits(req); err != nil {
		return err
	}
	res, err := s.app.Combo(ctx.Request().Context(), comboRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, comboResultFrom(res))
}

// checkComboLimits は手の件数と各手の hits の下限を見る(列挙の検証・ID 解決より前)。
func checkComboLimits(req api.ComboRequest) error {
	if n := len(req.Steps); n < 1 || n > engine.MaxComboSteps {
		return newError(api.InvalidInput, "steps は 1..%d 件でなければならない: %d 件", engine.MaxComboSteps, n)
	}
	for i, st := range req.Steps {
		if st.Hits != nil && *st.Hits < 1 {
			return newError(api.InvalidInput, "steps[%d].hits は 1 以上でなければならない: %d", i, *st.Hits)
		}
	}
	return nil
}

// --- 生成型 → app -------------------------------------------------------------

func comboRequestFrom(req api.ComboRequest) app.ComboRequest {
	steps := make([]app.ComboStep, 0, len(req.Steps))
	for _, st := range req.Steps {
		steps = append(steps, app.ComboStep{MoveID: st.MoveId, Critical: st.Critical, Hits: st.Hits})
	}
	out := app.ComboRequest{
		Format: formatFrom(req.Format), Attacker: individualFrom(req.Attacker), Defender: individualFrom(req.Defender),
		Steps: steps, OpponentMoveID: req.OpponentMoveId, Field: fieldFrom(req.Field),
	}
	if m := req.OpponentSpeed; m != nil {
		out.OpponentSpeed = &domain.Enum{Value: string(*m), Known: m.Valid()}
	}
	if f := req.SpeedField; f != nil {
		out.SpeedField = &app.ComboSpeedField{TrickRoom: f.TrickRoom, SelfTailwind: f.SelfTailwind, OpponentTailwind: f.OpponentTailwind}
	}
	return out
}

// --- 結果の写し ---------------------------------------------------------------

func comboRanksFrom(r engine.Ranks) api.ComboRanks {
	return api.ComboRanks{Atk: r.Atk, Def: r.Def, Spa: r.SpA, Spd: r.SpD, Spe: r.Spe}
}

func comboSpeedRangeFrom(r engine.SpeedRange) api.ComboSpeedRange {
	return api.ComboSpeedRange{Min: r.Min, Max: r.Max}
}

func comboKOChanceFrom(k engine.ComboKOChance) api.ComboKOChance {
	return api.ComboKOChance{
		ChancePercent: k.ChancePercent, Guaranteed: k.Guaranteed,
		DisplayChancePercent: percentFromTenths(k.DisplayChancePercentTenths()),
	}
}

func comboScenarioFrom(sc engine.OpponentScenario) api.ComboOpponentScenario {
	acts := make([]api.ComboOpponentAction, 0, len(sc.Actions))
	for _, a := range sc.Actions {
		acts = append(acts, api.ComboOpponentAction{
			Step: a.Step, Timing: api.ComboOpponentTiming(a.Timing), Damage: calcResultFrom(a.Damage),
			SurviveChancePercent: a.SurviveChancePercent,
		})
	}
	return api.ComboOpponentScenario{Actions: acts, SurviveChancePercent: sc.SurviveChancePercent}
}

// comboResultFrom は engine.ComboResult を契約の ComboResult に写す(形は WASM の calcCombo と同じ。ADR-0253)。
// opponentSpeed は unknown のとき 0(opponentSpeedKnown で区別)。変化技の手の damage・相手の技なしの opponentAttack は null。
// 印なしは空配列。
func comboResultFrom(r engine.ComboResult) api.ComboResult {
	steps := make([]api.ComboStepResult, 0, len(r.Steps))
	for _, s := range r.Steps {
		st := api.ComboStepResult{
			MoveId:         s.MoveID,
			SelfStartRanks: comboRanksFrom(s.SelfStartRanks), SelfEndRanks: comboRanksFrom(s.SelfEndRanks),
			OpponentStartRanks: comboRanksFrom(s.OpponentStartRanks), OpponentEndRanks: comboRanksFrom(s.OpponentEndRanks),
			SelfSpeed: s.SelfSpeed, OpponentSpeed: s.OpponentSpeed, SelfPriority: s.SelfPriority, OpponentPriority: s.OpponentPriority,
			Order: api.ComboTurnOrder(s.Order),
			Line: api.ComboSpeedLine{SelfSpeed: s.Line.SelfSpeed,
				SelfFirst: comboSpeedRangeFrom(s.Line.SelfFirst), OpponentFirst: comboSpeedRangeFrom(s.Line.OpponentFirst)},
		}
		if s.Damage != nil {
			d := calcResultFrom(*s.Damage)
			st.Damage = &d
		}
		steps = append(steps, st)
	}

	minT, maxT := r.Total.DisplayPercentRangeTenths()
	byStep := make([]api.ComboKOChance, 0, len(r.Total.KOByStep))
	for _, k := range r.Total.KOByStep {
		byStep = append(byStep, comboKOChanceFrom(k))
	}
	out := api.ComboResult{
		Steps: steps,
		Total: api.ComboTotal{
			MinDamage: r.Total.MinDamage, MaxDamage: r.Total.MaxDamage, DefenderHP: r.Total.DefenderHP,
			MinPercent: percentFromTenths(minT), MaxPercent: percentFromTenths(maxT),
			Ko: comboKOChanceFrom(r.Total.KO), KoByStep: byStep,
		},
		Speed: api.ComboSpeed{FirstSelfSpeed: r.Speed.FirstSelfSpeed, LastSelfSpeed: r.Speed.LastSelfSpeed,
			OpponentSpeed: r.Speed.OpponentSpeed, OpponentSpeedKnown: r.Speed.OpponentSpeedKnown},
		Unsupported: unsupportedFrom(r.Unsupported),
	}
	if a := r.OpponentAttack; a != nil {
		out.OpponentAttack = &api.ComboOpponentAttack{
			SelfHp: a.SelfHP, MinActions: a.MinActions, MaxActions: a.MaxActions,
			Fast: comboScenarioFrom(a.Fast), Slow: comboScenarioFrom(a.Slow),
		}
	}
	return out
}
