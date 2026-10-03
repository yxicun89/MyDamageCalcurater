package httpapi

// 調整(plan.md AJ4)の4操作。契約は api/openapi.yaml の /api/calc/adjust/* と ADR-0250。
//
// 流れは既存の calc と同じ: 厳格デコード → 値域の検査(hits・thresholdPercent・ceiling・minSpeed・modifier・damageModifier。ID 解決より前)
// → 列挙(format・mode・focus・offenseCategory・field・status・teraType)→ ID 解決 → engine → 生成型への写し。
// 自分の個体は境界で Validate しない(探索する能力の SP は engine が上書きする。ADR-0250 §2)。
// 計算イベントは発行しない(ADR-0250 §7)。式は持たず engine の関数を呼ぶだけ(CLAUDE.md 絶対ルール2・3)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// --- 値域の検査(ID 解決より前)--------------------------------------------------

// checkAdjustHits は発数 1..engine.MaxAdjustHits を確かめる(欠落 = 0 も拒否する)。
func checkAdjustHits(label string, hits int) error {
	if hits < 1 || hits > engine.MaxAdjustHits {
		return newError(api.InvalidInput, "%s は 1..%d でなければならない: %d", label, engine.MaxAdjustHits, hits)
	}
	return nil
}

// adjustThreshold はしきい値を確かめて engine に渡す値にする。省略は 0(engine の既定 = 100)、
// 明示した値は (0, 100] でなければならない(ADR-0250 §3)。
func adjustThreshold(label string, p *float64) (float64, error) {
	if p == nil {
		return 0, nil
	}
	if !(*p > 0 && *p <= engine.DefaultAdjustThresholdPercent) {
		return 0, newError(api.InvalidInput, "%s は (0, %v] でなければならない: %v", label, engine.DefaultAdjustThresholdPercent, *p)
	}
	return *p, nil
}

// adjustCeiling は能力ごとの上限を確かめ、省略を engine.MaxSPPerStat で補う(ADR-0250 §3)。
func adjustCeiling(c *api.AdjustCeiling) (engine.Stats, error) {
	var in api.AdjustCeiling
	if c != nil {
		in = *c
	}
	var out engine.Stats
	for _, f := range []struct {
		key engine.StatKey
		v   *int
	}{
		{engine.StatHP, in.Hp}, {engine.StatAtk, in.Atk}, {engine.StatDef, in.Def},
		{engine.StatSpA, in.Spa}, {engine.StatSpD, in.Spd}, {engine.StatSpe, in.Spe},
	} {
		v := engine.MaxSPPerStat
		if f.v != nil {
			v = *f.v
		}
		if v < 0 || v > engine.MaxSPPerStat {
			return engine.Stats{}, newError(api.InvalidInput, "ceiling.%s は 0..%d でなければならない: %d", f.key, engine.MaxSPPerStat, v)
		}
		out = out.WithStat(f.key, v)
	}
	return out, nil
}

// adjustModifier は補正を確かめて engine に渡す値にする。省略は 4096、明示した値は
// engine.MinEffectModifier..engine.MaxEffectModifier でなければならない(技の有無にかかわらず検査する。ADR-0250 §3・§5)。
func adjustModifier(label string, p *api.AdjustModifier) (int, error) {
	if p == nil {
		return engine.Modifier4096, nil
	}
	if *p < engine.MinEffectModifier || *p > engine.MaxEffectModifier {
		return 0, newError(api.InvalidInput, "%s は %d..%d でなければならない: %d", label, engine.MinEffectModifier, engine.MaxEffectModifier, *p)
	}
	return *p, nil
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

// --- indices -----------------------------------------------------------------------

// AdjustIndices は POST /api/calc/adjust/indices。
func (s *Server) AdjustIndices(ctx *echo.Context, params api.AdjustIndicesParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustIndicesRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	// 値域は ID 解決より前(ADR-0250 §5)。
	modifier, err := adjustModifier("modifier", req.Modifier)
	if err != nil {
		return err
	}
	damageModifier, err := adjustModifier("damageModifier", req.DamageModifier)
	if err != nil {
		return err
	}
	self, err := s.resolveIndividual("individual", req.Individual)
	if err != nil {
		return err
	}
	var move *engine.Move
	if req.MoveId != nil {
		mv, err := s.resolveMove(*req.MoveId)
		if err != nil {
			return err
		}
		move = &mv
	}

	var result api.AdjustIndicesResult
	if move != nil {
		fp, err := engine.FirepowerIndex(self, move.Category, move.Power, modifier)
		if err != nil {
			return errFromEngine(err)
		}
		v := int64(fp)
		result.FirepowerIndex = &v
	}
	phys, err := engine.BulkIndex(self, engine.CategoryPhysical, damageModifier)
	if err != nil {
		return errFromEngine(err)
	}
	spec, err := engine.BulkIndex(self, engine.CategorySpecial, damageModifier)
	if err != nil {
		return errFromEngine(err)
	}
	lines, err := engine.HPLines(self.Species.BaseStats.HP, self.SP.HP)
	if err != nil {
		return errFromEngine(err)
	}
	result.Stats = statBlockFrom(engine.RealStats(self))
	result.PhysicalBulkIndex = int64(phys)
	result.SpecialBulkIndex = int64(spec)
	result.HpLines = hpLineReportFrom(lines)
	return ctx.JSON(http.StatusOK, result)
}

// --- min-sp-to-ko / min-sp-to-survive ------------------------------------------------

// adjustSearchInput は探索の入力を値域 → 列挙 → ID 解決の順で組み立てる。両側の個体は Validate しない
// (engine の検査に一本化。ADR-0250 §2)。
func (s *Server) adjustSearchInput(ctx *echo.Context) (engine.AdjustSearchInput, error) {
	var req api.AdjustSearchRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return engine.AdjustSearchInput{}, err
	}
	if err := checkAdjustHits("hits", req.Hits); err != nil {
		return engine.AdjustSearchInput{}, err
	}
	threshold, err := adjustThreshold("thresholdPercent", req.ThresholdPercent)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	format, err := parseFormat(req.Format)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	field, err := parseField(req.Field)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	attacker, err := s.resolveIndividual("attacker", req.Attacker)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	defender, err := s.resolveIndividual("defender", req.Defender)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	move, err := s.resolveMove(req.MoveId)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	return engine.AdjustSearchInput{
		Format: format, Attacker: attacker, Defender: defender, Move: move, Field: field,
		Critical: criticalFrom(req.Options), TypeChart: s.store.TypeChart(),
		Hits: req.Hits, ThresholdPercent: threshold,
	}, nil
}

// AdjustMinSpToKo は POST /api/calc/adjust/min-sp-to-ko。
func (s *Server) AdjustMinSpToKo(ctx *echo.Context, params api.AdjustMinSpToKoParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	in, err := s.adjustSearchInput(ctx)
	if err != nil {
		return err
	}
	res, err := engine.MinSPToKO(in)
	if err != nil {
		return errFromEngine(err)
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
	in, err := s.adjustSearchInput(ctx)
	if err != nil {
		return err
	}
	res, err := engine.MinSPToSurvive(in)
	if err != nil {
		return errFromEngine(err)
	}
	return ctx.JSON(http.StatusOK, api.AdjustSurviveResult{
		Stat: api.StatKey(res.Stat), SearchLimit: res.SearchLimit, Feasible: res.Feasible,
		HpSp: res.HPSP, StatSp: res.StatSP, TotalSp: res.TotalSP, BulkIndex: int64(res.BulkIndex),
		ChancePercent: res.ChancePercent, Unsupported: unsupportedFrom(res.Unsupported),
	})
}

// --- allocation ----------------------------------------------------------------------

// AdjustAllocation は POST /api/calc/adjust/allocation。
func (s *Server) AdjustAllocation(ctx *echo.Context, params api.AdjustAllocationParams) error {
	if err := checkHeaders(params.XDeviceId, params.XSessionId); err != nil {
		return err
	}
	var req api.AdjustAllocationRequest
	if err := decodeStrict(limitedBody(ctx), &req); err != nil {
		return err
	}
	// 値域(ID 解決・列挙の検証より前。ADR-0108 決定3・ADR-0208 §3)。
	ceiling, err := adjustCeiling(req.Ceiling)
	if err != nil {
		return err
	}
	minSpeed := derefInt(req.MinSpeed)
	if minSpeed < 0 {
		return newError(api.InvalidInput, "minSpeed は 0 以上でなければならない: %d", minSpeed)
	}
	var goalThreshold float64
	if g := req.Goal; g != nil {
		if err := checkAdjustHits("goal.hits", g.Hits); err != nil {
			return err
		}
		if goalThreshold, err = adjustThreshold("goal.thresholdPercent", g.ThresholdPercent); err != nil {
			return err
		}
	}

	// 列挙。mode は必須(欠落も invalid_enum)。focus / offenseCategory は値があれば列挙だけ検査し、
	// 使う側での欠落は engine が invalid_input にする(ADR-0250 §4)。
	if !req.Mode.Valid() {
		return newError(api.InvalidEnum, "mode に未知の値 %q", string(req.Mode))
	}
	var focus engine.BulkFocus
	if req.Focus != nil {
		if !req.Focus.Valid() {
			return newError(api.InvalidEnum, "focus に未知の値 %q", string(*req.Focus))
		}
		focus = engine.BulkFocus(*req.Focus)
	}
	var category engine.MoveCategory
	if req.OffenseCategory != nil {
		if !req.OffenseCategory.Valid() {
			return newError(api.InvalidEnum, "offenseCategory に未知の値 %q", string(*req.OffenseCategory))
		}
		category = engine.MoveCategory(*req.OffenseCategory)
	}
	var goalFormat engine.Format
	var goalField engine.Field
	if g := req.Goal; g != nil {
		if goalFormat, err = parseFormat(g.Format); err != nil {
			return err
		}
		if goalField, err = parseField(g.Field); err != nil {
			return err
		}
	}

	self, err := s.resolveIndividual("self", req.Self)
	if err != nil {
		return err
	}
	in := engine.SPAllocInput{
		Self: self, Ceiling: ceiling, Mode: engine.AllocMode(req.Mode), Focus: focus,
		OffenseCategory: category, MinSpeed: minSpeed,
	}
	if g := req.Goal; g != nil {
		opponent, err := s.resolveIndividual("goal.opponent", g.Opponent)
		if err != nil {
			return err
		}
		move, err := s.resolveMove(g.MoveId)
		if err != nil {
			return err
		}
		in.Goal = &engine.AllocGoal{
			Format: goalFormat, Opponent: opponent, Move: move, Field: goalField, Critical: criticalFrom(g.Options),
			TypeChart: s.store.TypeChart(), Hits: g.Hits, ThresholdPercent: goalThreshold,
		}
	}

	res, err := engine.SuggestSPAllocation(in)
	if err != nil {
		return errFromEngine(err)
	}
	result := api.AdjustAllocationResult{
		Remaining: res.Remaining, MaxIndex: allocPlanFrom(res.MaxIndex), Unsupported: unsupportedFrom(res.Unsupported),
	}
	if res.MinSP != nil {
		p := allocPlanFrom(*res.MinSP)
		result.MinSp = &p
	}
	return ctx.JSON(http.StatusOK, result)
}
