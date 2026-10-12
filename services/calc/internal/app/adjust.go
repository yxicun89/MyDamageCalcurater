package app

// 調整(plan.md AJ4・AJ10)の5操作(POST /api/calc/adjust/*。ADR-0250・ADR-0177 §9)。
//
// 順番は 値域(hits・thresholdPercent・ceiling・minSpeed・modifier・damageModifier。domain)→ 列挙
// (format・mode・focus・offenseCategory・kind・field・status・teraType)→ ID 解決 → engine。
// 自分の個体は Validate しない(探索する能力の SP は engine が上書きする。ADR-0250 §2)。
// 計算イベントは発行しない(ADR-0250 §7)。式は持たず engine の関数を呼ぶだけ(CLAUDE.md 絶対ルール2・3)。
// 締め切りの確認(Expired)は S3a 以前から調整に無かったので足さない(振る舞いを変えない。ADR-0252 追記 S3b)。

import (
	"fmt"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

// --- 入力 --------------------------------------------------------------------------

// AdjustIndicesRequest は指数(POST /api/calc/adjust/indices)の入力。
type AdjustIndicesRequest struct {
	Individual               Individual
	MoveID                   *string
	Modifier, DamageModifier *int
}

// AdjustSearchRequest は最小 SP の探索(min-sp-to-ko・min-sp-to-survive)の入力。
type AdjustSearchRequest struct {
	Format             domain.Enum
	Attacker, Defender Individual
	MoveID             string
	Field              *domain.Field
	Options            *Options
	Hits               int
	ThresholdPercent   *float64
}

// AdjustGoal は複数の目標の 1 件(Hits・MoveID は kind によって必須。ADR-0331)。
type AdjustGoal struct {
	Kind             domain.Enum
	Opponent         Individual
	MoveID           *string
	Hits             *int
	ThresholdPercent *float64
}

// AdjustGoalsRequest は複数の目標(POST /api/calc/adjust/goals)の入力。
type AdjustGoalsRequest struct {
	Format  domain.Enum
	Self    Individual
	Ceiling *domain.Ceiling
	Goals   []AdjustGoal
}

// AdjustAllocGoal は振り分けの最小 SP の組の目標。
type AdjustAllocGoal struct {
	Format           domain.Enum
	Opponent         Individual
	MoveID           string
	Field            *domain.Field
	Options          *Options
	Hits             int
	ThresholdPercent *float64
}

// AdjustAllocationRequest は振り分け(POST /api/calc/adjust/allocation)の入力。
type AdjustAllocationRequest struct {
	Self            Individual
	Ceiling         *domain.Ceiling
	Mode            domain.Enum
	Focus           *domain.Enum
	OffenseCategory *domain.Enum
	MinSpeed        *int
	Goal            *AdjustAllocGoal
}

// AdjustIndicesOutput は指数の結果。FirepowerIndex は技を指定したときだけ入る。
type AdjustIndicesOutput struct {
	FirepowerIndex                      *int
	PhysicalBulkIndex, SpecialBulkIndex int
	HPLines                             engine.HPLineReport
	Stats                               engine.Stats
}

// --- indices -----------------------------------------------------------------------

// AdjustIndices は火力指数・耐久指数・HP の線を求める。値域は ID 解決より前(ADR-0250 §5)。
func (s *Service) AdjustIndices(req AdjustIndicesRequest) (AdjustIndicesOutput, error) {
	modifier, err := domain.AdjustModifier("modifier", req.Modifier)
	if err != nil {
		return AdjustIndicesOutput{}, err
	}
	damageModifier, err := domain.AdjustModifier("damageModifier", req.DamageModifier)
	if err != nil {
		return AdjustIndicesOutput{}, err
	}
	self, err := s.ResolveIndividual("individual", req.Individual)
	if err != nil {
		return AdjustIndicesOutput{}, err
	}
	var move *engine.Move
	if req.MoveID != nil {
		mv, err := s.ResolveMove(*req.MoveID)
		if err != nil {
			return AdjustIndicesOutput{}, err
		}
		move = &mv
	}

	var out AdjustIndicesOutput
	if move != nil {
		fp, err := engine.FirepowerIndex(self, move.Category, move.Power, modifier)
		if err != nil {
			return AdjustIndicesOutput{}, domain.FromEngine(err)
		}
		out.FirepowerIndex = &fp
	}
	phys, err := engine.BulkIndex(self, engine.CategoryPhysical, damageModifier)
	if err != nil {
		return AdjustIndicesOutput{}, domain.FromEngine(err)
	}
	spec, err := engine.BulkIndex(self, engine.CategorySpecial, damageModifier)
	if err != nil {
		return AdjustIndicesOutput{}, domain.FromEngine(err)
	}
	lines, err := engine.HPLines(self.Species.BaseStats.HP, self.SP.HP)
	if err != nil {
		return AdjustIndicesOutput{}, domain.FromEngine(err)
	}
	out.Stats = engine.RealStats(self)
	out.PhysicalBulkIndex, out.SpecialBulkIndex = phys, spec
	out.HPLines = lines
	return out, nil
}

// --- min-sp-to-ko / min-sp-to-survive ------------------------------------------------

// adjustSearchInput は探索の入力を値域 → 列挙 → ID 解決の順で組み立てる。両側の個体は Validate しない
// (engine の検査に一本化。ADR-0250 §2)。
func (s *Service) adjustSearchInput(req AdjustSearchRequest) (engine.AdjustSearchInput, error) {
	if err := domain.CheckAdjustHits("hits", req.Hits); err != nil {
		return engine.AdjustSearchInput{}, err
	}
	threshold, err := domain.AdjustThreshold("thresholdPercent", req.ThresholdPercent)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	format, err := domain.ParseFormat(req.Format)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	field, err := domain.ParseField(req.Field)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	attacker, err := s.ResolveIndividual("attacker", req.Attacker)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	defender, err := s.ResolveIndividual("defender", req.Defender)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	move, err := s.ResolveMove(req.MoveID)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	return engine.AdjustSearchInput{
		Format: format, Attacker: attacker, Defender: defender, Move: move, Field: field,
		Critical: req.Options.critical(), TypeChart: s.master.TypeChart(),
		Hits: req.Hits, ThresholdPercent: threshold,
	}, nil
}

// AdjustMinSPToKO は攻撃側の最小 SP で倒せるかを探す。
func (s *Service) AdjustMinSPToKO(req AdjustSearchRequest) (engine.KOSearchResult, error) {
	in, err := s.adjustSearchInput(req)
	if err != nil {
		return engine.KOSearchResult{}, err
	}
	res, err := engine.MinSPToKO(in)
	if err != nil {
		return engine.KOSearchResult{}, domain.FromEngine(err)
	}
	return res, nil
}

// AdjustMinSPToSurvive は防御側の最小 SP で耐えられるかを探す。
func (s *Service) AdjustMinSPToSurvive(req AdjustSearchRequest) (engine.SurviveSearchResult, error) {
	in, err := s.adjustSearchInput(req)
	if err != nil {
		return engine.SurviveSearchResult{}, err
	}
	res, err := engine.MinSPToSurvive(in)
	if err != nil {
		return engine.SurviveSearchResult{}, domain.FromEngine(err)
	}
	return res, nil
}

// --- goals ---------------------------------------------------------------------------

// AdjustGoals は複数の目標を満たす SP を探す(ADR-0331 の段階 B・ADR-0177 §9)。
// 検査順は 値域 → 列挙 → 種類ごとの必須 → ID 解決(self → 目標の順に opponent → moveId)→ engine。
// ID 解決より前はマスタを引かない。場は契約に無いので零値(ParseField(nil) と同じ)を渡す。
func (s *Service) AdjustGoals(req AdjustGoalsRequest) (engine.SPGoalsResult, error) {
	// 値域(ID 解決・列挙より前。ADR-0250 §5)。hits・thresholdPercent は種類によらず、値があれば検査する。
	if err := domain.CheckAdjustGoalCount(len(req.Goals)); err != nil {
		return engine.SPGoalsResult{}, err
	}
	thresholds := make([]float64, len(req.Goals))
	for i, g := range req.Goals {
		if g.Hits != nil {
			if err := domain.CheckAdjustHits(fmt.Sprintf("goals[%d].hits", i), *g.Hits); err != nil {
				return engine.SPGoalsResult{}, err
			}
		}
		threshold, err := domain.AdjustThreshold(fmt.Sprintf("goals[%d].thresholdPercent", i), g.ThresholdPercent)
		if err != nil {
			return engine.SPGoalsResult{}, err
		}
		thresholds[i] = threshold
	}
	ceiling, err := domain.AdjustCeiling(req.Ceiling)
	if err != nil {
		return engine.SPGoalsResult{}, err
	}
	// 列挙。
	format, err := domain.ParseFormat(req.Format)
	if err != nil {
		return engine.SPGoalsResult{}, err
	}
	kinds := make([]engine.SPGoalKind, len(req.Goals))
	for i, g := range req.Goals {
		if kinds[i], err = domain.ParseSPGoalKind(fmt.Sprintf("goals[%d]", i), g.Kind); err != nil {
			return engine.SPGoalsResult{}, err
		}
	}
	// 種類ごとの必須(outspeed の moveId は任意)。
	for i, g := range req.Goals {
		if kinds[i] == engine.SPGoalOutspeed {
			continue
		}
		if g.MoveID == nil {
			return engine.SPGoalsResult{}, domain.Errorf(domain.CodeInvalidInput, "goals[%d](%s)の moveId は必須", i, kinds[i])
		}
		if g.Hits == nil {
			return engine.SPGoalsResult{}, domain.Errorf(domain.CodeInvalidInput, "goals[%d](%s)の hits は必須", i, kinds[i])
		}
	}
	// ID 解決。
	self, err := s.ResolveIndividual("self", req.Self)
	if err != nil {
		return engine.SPGoalsResult{}, err
	}
	goals := make([]engine.SPGoal, 0, len(req.Goals))
	for i, g := range req.Goals {
		opponent, err := s.ResolveIndividual(fmt.Sprintf("goals[%d].opponent", i), g.Opponent)
		if err != nil {
			return engine.SPGoalsResult{}, err
		}
		goal := engine.SPGoal{Kind: kinds[i], Opponent: opponent}
		if g.MoveID != nil {
			move, err := s.ResolveMove(*g.MoveID)
			if err != nil {
				return engine.SPGoalsResult{}, err
			}
			if kinds[i] == engine.SPGoalOutspeed {
				// 先に使う技は素早さのランク変化だけを使う(ダメージは計算しない。ADR-0177 §4)。
				goal.SelfSpeedStage = engine.GuaranteedSelfSpeedStage(move)
			} else {
				goal.Move = move
			}
		}
		if kinds[i] != engine.SPGoalOutspeed {
			goal.Hits, goal.ThresholdPercent = *g.Hits, thresholds[i]
		}
		goals = append(goals, goal)
	}

	res, err := engine.SuggestSPForGoals(engine.SPGoalsInput{
		Format: format, Self: self, Ceiling: ceiling, Field: engine.Field{}, TypeChart: s.master.TypeChart(), Goals: goals,
	})
	if err != nil {
		return engine.SPGoalsResult{}, domain.FromEngine(err)
	}
	return res, nil
}

// --- allocation ----------------------------------------------------------------------

// AdjustAllocation は残りの SP の振り分けを提案する。
func (s *Service) AdjustAllocation(req AdjustAllocationRequest) (engine.SPAllocResult, error) {
	// 値域(ID 解決・列挙の検証より前。ADR-0108 決定3・ADR-0208 §3)。
	ceiling, err := domain.AdjustCeiling(req.Ceiling)
	if err != nil {
		return engine.SPAllocResult{}, err
	}
	minSpeed, err := domain.AdjustMinSpeed(req.MinSpeed)
	if err != nil {
		return engine.SPAllocResult{}, err
	}
	var goalThreshold float64
	if g := req.Goal; g != nil {
		if err := domain.CheckAdjustHits("goal.hits", g.Hits); err != nil {
			return engine.SPAllocResult{}, err
		}
		if goalThreshold, err = domain.AdjustThreshold("goal.thresholdPercent", g.ThresholdPercent); err != nil {
			return engine.SPAllocResult{}, err
		}
	}

	// 列挙。mode は必須(欠落も invalid_enum)。focus / offenseCategory は値があれば列挙だけ検査し、
	// 使う側での欠落は engine が invalid_input にする(ADR-0250 §4)。
	mode, err := domain.ParseAllocMode(req.Mode)
	if err != nil {
		return engine.SPAllocResult{}, err
	}
	focus, err := domain.ParseBulkFocus(req.Focus)
	if err != nil {
		return engine.SPAllocResult{}, err
	}
	category, err := domain.ParseOffenseCategory(req.OffenseCategory)
	if err != nil {
		return engine.SPAllocResult{}, err
	}
	var goalFormat engine.Format
	var goalField engine.Field
	if g := req.Goal; g != nil {
		if goalFormat, err = domain.ParseFormat(g.Format); err != nil {
			return engine.SPAllocResult{}, err
		}
		if goalField, err = domain.ParseField(g.Field); err != nil {
			return engine.SPAllocResult{}, err
		}
	}

	self, err := s.ResolveIndividual("self", req.Self)
	if err != nil {
		return engine.SPAllocResult{}, err
	}
	in := engine.SPAllocInput{
		Self: self, Ceiling: ceiling, Mode: mode, Focus: focus, OffenseCategory: category, MinSpeed: minSpeed,
	}
	if g := req.Goal; g != nil {
		opponent, err := s.ResolveIndividual("goal.opponent", g.Opponent)
		if err != nil {
			return engine.SPAllocResult{}, err
		}
		move, err := s.ResolveMove(g.MoveID)
		if err != nil {
			return engine.SPAllocResult{}, err
		}
		in.Goal = &engine.AllocGoal{
			Format: goalFormat, Opponent: opponent, Move: move, Field: goalField, Critical: g.Options.critical(),
			TypeChart: s.master.TypeChart(), Hits: g.Hits, ThresholdPercent: goalThreshold,
		}
	}

	res, err := engine.SuggestSPAllocation(in)
	if err != nil {
		return engine.SPAllocResult{}, domain.FromEngine(err)
	}
	return res, nil
}
