package httpapi

// POST /api/calc/combo(連続技。ADR-0252・ADR-0179 追記 CB-3)。
//
// 流れは /api/calc と同じ(パッケージ doc): 厳格デコード → 手の件数(ID の解決より前。ADR-0208 §3)
// → 列挙(format・opponentSpeed・status・teraType・天候・フィールド)→ ID 解決 → 個体の検証 → engine.CalcCombo → 写し。
// ランク変化の無い変化技の手・変化技の相手の技・多段の回数は engine の sentinel(invalid_input)に任せる。
// 独自の式・丸めを持たない(表示%は engine の tenths を 10 で割るだけ)。計算イベントは発行しない(絶対ルール5。ADR-0252 §4)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/engine"
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
	if n := len(req.Steps); n < 1 || n > engine.MaxComboSteps {
		return newError(api.InvalidInput, "steps は 1..%d 件でなければならない: %d 件", engine.MaxComboSteps, n)
	}
	format, err := parseFormat(req.Format)
	if err != nil {
		return err
	}
	oppSpeed, err := parseComboOpponentSpeed(req.OpponentSpeed)
	if err != nil {
		return err
	}
	field, err := parseField(req.Field)
	if err != nil {
		return err
	}
	attacker, err := s.resolveIndividual("attacker", req.Attacker)
	if err != nil {
		return err
	}
	defender, err := s.resolveIndividual("defender", req.Defender)
	if err != nil {
		return err
	}
	steps := make([]engine.ComboStep, 0, len(req.Steps))
	for _, st := range req.Steps {
		mv, err := s.resolveMove(st.MoveId)
		if err != nil {
			return err
		}
		steps = append(steps, engine.ComboStep{Move: mv, Critical: derefBool(st.Critical), Hits: derefInt(st.Hits)})
	}
	var oppMove *engine.Move
	if req.OpponentMoveId != nil {
		mv, err := s.resolveMove(*req.OpponentMoveId)
		if err != nil {
			return err
		}
		oppMove = &mv
	}
	if err := validateIndividual("攻撃側", attacker); err != nil {
		return err
	}
	if err := validateIndividual("防御側", defender); err != nil {
		return err
	}
	if err := s.checkMegaItem("攻撃側", attacker.Species, attacker.Item); err != nil {
		return err
	}
	if err := s.checkMegaItem("防御側", defender.Species, defender.Item); err != nil {
		return err
	}

	if err := checkDeadline(ctx.Request().Context()); err != nil {
		return err
	}
	res, err := engine.CalcCombo(engine.ComboInput{
		Format: format, Attacker: attacker, Defender: defender, Field: field, TypeChart: s.store.TypeChart(),
		Steps: steps, OpponentMove: oppMove, OpponentSpeed: oppSpeed, SpeedField: comboSpeedFieldFrom(req.SpeedField),
	})
	if err != nil {
		return errFromEngine(err)
	}
	return ctx.JSON(http.StatusOK, comboResultFrom(res))
}

// parseComboOpponentSpeed は相手の素早さの扱いを検証する(省略は unknown)。
func parseComboOpponentSpeed(m *api.ComboOpponentSpeed) (engine.OpponentSpeedMode, error) {
	if m == nil {
		return engine.OpponentSpeedUnknown, nil
	}
	if !m.Valid() {
		return "", newError(api.InvalidEnum, "opponentSpeed に未知の値 %q", string(*m))
	}
	return engine.OpponentSpeedMode(*m), nil
}

func comboSpeedFieldFrom(f *api.ComboSpeedField) engine.ComboSpeedField {
	if f == nil {
		return engine.ComboSpeedField{}
	}
	return engine.ComboSpeedField{
		TrickRoom: derefBool(f.TrickRoom), SelfTailwind: derefBool(f.SelfTailwind), OpponentTailwind: derefBool(f.OpponentTailwind),
	}
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

// comboResultFrom は engine.ComboResult を契約の ComboResult に写す(形は WASM の calcCombo と同じ。ADR-0252)。
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
