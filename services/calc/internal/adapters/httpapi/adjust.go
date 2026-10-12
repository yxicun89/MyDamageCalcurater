package httpapi

// 調整(plan.md AJ4・AJ10)の5操作。契約は api/openapi.yaml の /api/calc/adjust/* と ADR-0250。
//
// ここは 厳格デコードと、生成型 ⇄ app の型の変換・結果の写しだけを持つ。値域の検査(domain)→ 列挙の検証
// → ID 解決 → engine の手順は app.Service.Adjust* が持つ(ADR-0811・ADR-0252 追記 S3b)。
// 計算イベントは発行しない(ADR-0250 §7)。式は持たず engine の関数を呼ぶだけ(CLAUDE.md 絶対ルール2・3)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
)

// --- 生成型 → app -------------------------------------------------------------------

func ceilingFrom(c *api.AdjustCeiling) *domain.Ceiling {
	if c == nil {
		return nil
	}
	return &domain.Ceiling{HP: c.Hp, Atk: c.Atk, Def: c.Def, SpA: c.Spa, SpD: c.Spd, Spe: c.Spe}
}

func adjustSearchRequestFrom(req api.AdjustSearchRequest) app.AdjustSearchRequest {
	return app.AdjustSearchRequest{
		Format: formatFrom(req.Format), Attacker: individualFrom(req.Attacker), Defender: individualFrom(req.Defender),
		MoveID: req.MoveId, Field: fieldFrom(req.Field), Options: optionsFrom(req.Options),
		Hits: req.Hits, ThresholdPercent: req.ThresholdPercent,
	}
}

func adjustGoalsRequestFrom(req api.AdjustGoalsRequest) app.AdjustGoalsRequest {
	goals := make([]app.AdjustGoal, 0, len(req.Goals))
	for _, g := range req.Goals {
		goals = append(goals, app.AdjustGoal{
			Kind: domain.Enum{Value: string(g.Kind), Known: g.Kind.Valid()}, Opponent: individualFrom(g.Opponent),
			MoveID: g.MoveId, Hits: g.Hits, ThresholdPercent: g.ThresholdPercent,
		})
	}
	return app.AdjustGoalsRequest{Format: formatFrom(req.Format), Self: individualFrom(req.Self), Ceiling: ceilingFrom(req.Ceiling), Goals: goals}
}

func adjustAllocationRequestFrom(req api.AdjustAllocationRequest) app.AdjustAllocationRequest {
	out := app.AdjustAllocationRequest{
		Self: individualFrom(req.Self), Ceiling: ceilingFrom(req.Ceiling),
		Mode: domain.Enum{Value: string(req.Mode), Known: req.Mode.Valid()}, MinSpeed: req.MinSpeed,
	}
	if f := req.Focus; f != nil {
		out.Focus = &domain.Enum{Value: string(*f), Known: f.Valid()}
	}
	if c := req.OffenseCategory; c != nil {
		out.OffenseCategory = &domain.Enum{Value: string(*c), Known: c.Valid()}
	}
	if g := req.Goal; g != nil {
		out.Goal = &app.AdjustAllocGoal{
			Format: formatFrom(g.Format), Opponent: individualFrom(g.Opponent), MoveID: g.MoveId,
			Field: fieldFrom(g.Field), Options: optionsFrom(g.Options), Hits: g.Hits, ThresholdPercent: g.ThresholdPercent,
		}
	}
	return out
}

// --- 結果の写し --------------------------------------------------------------------

func hpLinePointFrom(p *engine.HPLinePoint) *api.HPLinePoint {
	if p == nil {
		return nil
	}
	return &api.HPLinePoint{Hp: p.HP, Sp: p.SP, SpDelta: p.SPDelta}
}

func hpLineReportFrom(r engine.HPLineReport) api.HPLineReport {
	current := api.HPLineKindNone
	switch r.Current {
	case engine.HPLine16n:
		current = api.HPLineKindN16n
	case engine.HPLine16nMinus1:
		current = api.HPLineKindN16n1
	}
	return api.HPLineReport{
		Hp: r.HP, Sp: r.SP, Current: current,
		Next16n: hpLinePointFrom(r.Next16n), Prev16n: hpLinePointFrom(r.Prev16n),
		Next16nMinus1: hpLinePointFrom(r.Next16nMinus1), Prev16nMinus1: hpLinePointFrom(r.Prev16nMinus1),
	}
}

func allocPlanFrom(p engine.AllocPlan) api.AdjustAllocPlan {
	return api.AdjustAllocPlan{
		Sp: statBlockFrom(p.SP), TotalSp: p.TotalSP, Stats: statBlockFrom(p.Real),
		PhysicalBulk: int64(p.PhysicalBulk), SpecialBulk: int64(p.SpecialBulk),
		SpeedMet: p.SpeedMet, GoalMet: p.GoalMet, ChancePercent: p.ChancePercent,
	}
}

func adjustIndicesResultFrom(o app.AdjustIndicesOutput) api.AdjustIndicesResult {
	var result api.AdjustIndicesResult
	if o.FirepowerIndex != nil {
		v := int64(*o.FirepowerIndex)
		result.FirepowerIndex = &v
	}
	result.Stats = statBlockFrom(o.Stats)
	result.PhysicalBulkIndex = int64(o.PhysicalBulkIndex)
	result.SpecialBulkIndex = int64(o.SpecialBulkIndex)
	result.HpLines = hpLineReportFrom(o.HPLines)
	return result
}

// adjustGoalsResultFrom は engine の結果を契約の形に写す。outspeed は chancePercent を、survive・ko は
// 素早さの3項目を null にする(キーは省略しない)。
func adjustGoalsResultFrom(r engine.SPGoalsResult) api.AdjustGoalsResult {
	out := api.AdjustGoalsResult{
		Feasible: r.Feasible, Remaining: r.Remaining,
		Plan:        api.AdjustGoalsPlan{Sp: statBlockFrom(r.Plan.SP), TotalSp: r.Plan.TotalSP, Stats: statBlockFrom(r.Plan.Real)},
		Goals:       make([]api.AdjustGoalOutcome, 0, len(r.Goals)),
		Unsupported: unsupportedFrom(r.Unsupported),
	}
	for _, o := range r.Goals {
		g := api.AdjustGoalOutcome{Kind: api.AdjustGoalKind(o.Kind), Met: o.Met}
		if o.Kind == engine.SPGoalOutspeed {
			selfSpeed, opponentSpeed, rank := o.SelfSpeed, o.OpponentSpeed, o.SelfSpeedRank
			g.SelfSpeed, g.OpponentSpeed, g.SelfSpeedRank = &selfSpeed, &opponentSpeed, &rank
		} else {
			chance := o.ChancePercent
			g.ChancePercent = &chance
		}
		out.Goals = append(out.Goals, g)
	}
	return out
}

func adjustAllocationResultFrom(res engine.SPAllocResult) api.AdjustAllocationResult {
	result := api.AdjustAllocationResult{
		Remaining: res.Remaining, MaxIndex: allocPlanFrom(res.MaxIndex), Unsupported: unsupportedFrom(res.Unsupported),
	}
	if res.MinSP != nil {
		p := allocPlanFrom(*res.MinSP)
		result.MinSp = &p
	}
	return result
}

// --- ハンドラ ----------------------------------------------------------------------

// AdjustIndices は POST /api/calc/adjust/indices。
func (s *Server) AdjustIndices(ctx *echo.Context, params api.AdjustIndicesParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustIndicesRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	out, err := s.app.AdjustIndices(app.AdjustIndicesRequest{
		Individual: individualFrom(req.Individual), MoveID: req.MoveId, Modifier: req.Modifier, DamageModifier: req.DamageModifier,
	})
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, adjustIndicesResultFrom(out))
}

// AdjustMinSpToKo は POST /api/calc/adjust/min-sp-to-ko。
func (s *Server) AdjustMinSpToKo(ctx *echo.Context, params api.AdjustMinSpToKoParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustSearchRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	res, err := s.app.AdjustMinSPToKO(adjustSearchRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, api.AdjustKOResult{
		Stat: api.StatKey(res.Stat), SearchLimit: res.SearchLimit, Feasible: res.Feasible, Sp: res.SP,
		ChancePercent: res.ChancePercent, Unsupported: unsupportedFrom(res.Unsupported),
	})
}

// AdjustMinSpToSurvive は POST /api/calc/adjust/min-sp-to-survive。
func (s *Server) AdjustMinSpToSurvive(ctx *echo.Context, params api.AdjustMinSpToSurviveParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustSearchRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	res, err := s.app.AdjustMinSPToSurvive(adjustSearchRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, api.AdjustSurviveResult{
		Stat: api.StatKey(res.Stat), SearchLimit: res.SearchLimit, Feasible: res.Feasible,
		HpSp: res.HPSP, StatSp: res.StatSP, TotalSp: res.TotalSP, BulkIndex: int64(res.BulkIndex),
		ChancePercent: res.ChancePercent, Unsupported: unsupportedFrom(res.Unsupported),
	})
}

// AdjustGoals は POST /api/calc/adjust/goals(ADR-0331 の段階 B・ADR-0177 §9)。
func (s *Server) AdjustGoals(ctx *echo.Context, params api.AdjustGoalsParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustGoalsRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	res, err := s.app.AdjustGoals(adjustGoalsRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, adjustGoalsResultFrom(res))
}

// AdjustAllocation は POST /api/calc/adjust/allocation。
func (s *Server) AdjustAllocation(ctx *echo.Context, params api.AdjustAllocationParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustAllocationRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	res, err := s.app.AdjustAllocation(adjustAllocationRequestFrom(req))
	if err != nil {
		return httpErrorFrom(err)
	}
	return ctx.JSON(http.StatusOK, adjustAllocationResultFrom(res))
}
