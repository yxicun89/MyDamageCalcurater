package wasmapi

// 調整(plan.md AJ4)の WASM 境界。契約は ADR-0250(engine の定義は ADR-0150)。
//
// 各 run は「値域の検査 → DTO の変換(列挙の検証)→ engine 呼び出し → 結果の写し」だけを行う。
// 値域(hits・thresholdPercent・ceiling・minSpeed・modifier・damageModifier)は DTO の変換より前に見る(ADR-0108 決定3・ADR-0208 §3)。
// 自分の個体は境界で Validate しない(探索する能力の SP は engine が上書きする。ADR-0250 §2)。
// 省略と明示の 0 を区別する項目はポインタで受け、既定(4096・100・32)は境界が補う(ADR-0250 §3)。

import (
	"example.com/pokecalc/engine"
)

// AdjustIndices は指数(火力・耐久)と HP の 16n / 16n-1 ラインを返す(ADR-0250 §1)。
func AdjustIndices(requestJSON string) string {
	return handle(requestJSON, (*adjustIndicesRequest).run)
}

// AdjustMinSPToKO は相手を n 発で倒せる最小の A / C の SP を返す(engine.MinSPToKO の素通し)。
func AdjustMinSPToKO(requestJSON string) string {
	return handle(requestJSON, (*adjustSearchRequest).runKO)
}

// AdjustMinSPToSurvive は相手の技を n 発耐える最小の H と B / D の SP の組を返す(engine.MinSPToSurvive の素通し)。
func AdjustMinSPToSurvive(requestJSON string) string {
	return handle(requestJSON, (*adjustSearchRequest).runSurvive)
}

// AdjustAllocation は SP 配分の提案を返す(engine.SuggestSPAllocation の素通し。ceiling の既定 32 は境界が補う)。
func AdjustAllocation(requestJSON string) string {
	return handle(requestJSON, (*adjustAllocationRequest).run)
}

// --- 値域の検査(DTO の変換より前)---------------------------------------------

// checkAdjustHits は発数 1..engine.MaxAdjustHits を確かめる(欠落 = 0 も拒否する)。
func checkAdjustHits(path string, hits int) error {
	if hits < 1 || hits > engine.MaxAdjustHits {
		return fail(CodeInvalidInput, "%s は 1..%d でなければならない: %d", path, engine.MaxAdjustHits, hits)
	}
	return nil
}

// adjustThreshold はしきい値を確かめて engine に渡す値にする。省略は 0(engine の既定 = 100)、
// 明示した値は (0, 100] でなければならない(明示の 0 を「既定」と読ませない。ADR-0250 §3)。
func adjustThreshold(path string, p *float64) (float64, error) {
	if p == nil {
		return 0, nil
	}
	if !(*p > 0 && *p <= engine.DefaultAdjustThresholdPercent) {
		return 0, fail(CodeInvalidInput, "%s は (0, %v] でなければならない: %v", path, engine.DefaultAdjustThresholdPercent, *p)
	}
	return *p, nil
}

// --- adjustIndices ------------------------------------------------------------

type adjustIndicesRequest struct {
	Individual individualDTO `json:"individual"`
	// Move は火力指数に使う技。省略すると firepowerIndex は null。
	Move *moveDTO `json:"move"`
	// Modifier / DamageModifier は 4096 基準の補正。省略は 4096、明示の値は境界が値域を検査する。
	Modifier       *int `json:"modifier"`
	DamageModifier *int `json:"damageModifier"`
}

type hpLinePointDTO struct {
	HP      int `json:"hp"`
	SP      int `json:"sp"`
	SPDelta int `json:"spDelta"`
}

type hpLineReportDTO struct {
	HP            int             `json:"hp"`
	SP            int             `json:"sp"`
	Current       string          `json:"current"`
	Next16n       *hpLinePointDTO `json:"next16n"`
	Prev16n       *hpLinePointDTO `json:"prev16n"`
	Next16nMinus1 *hpLinePointDTO `json:"next16nMinus1"`
	Prev16nMinus1 *hpLinePointDTO `json:"prev16nMinus1"`
}

type adjustIndicesResultDTO struct {
	Stats             statsDTO        `json:"stats"`
	FirepowerIndex    *int64          `json:"firepowerIndex"`
	PhysicalBulkIndex int64           `json:"physicalBulkIndex"`
	SpecialBulkIndex  int64           `json:"specialBulkIndex"`
	HPLines           hpLineReportDTO `json:"hpLines"`
}

// hpLineKindNone は engine の HPLineNone(空文字)の契約上の表記(ADR-0250 §6)。
const hpLineKindNone = "none"

func hpLinePointFrom(p *engine.HPLinePoint) *hpLinePointDTO {
	if p == nil {
		return nil
	}
	return &hpLinePointDTO{HP: p.HP, SP: p.SP, SPDelta: p.SPDelta}
}

func hpLineReportFrom(r engine.HPLineReport) hpLineReportDTO {
	current := string(r.Current)
	if r.Current == engine.HPLineNone {
		current = hpLineKindNone
	}
	return hpLineReportDTO{
		HP: r.HP, SP: r.SP, Current: current,
		Next16n: hpLinePointFrom(r.Next16n), Prev16n: hpLinePointFrom(r.Prev16n),
		Next16nMinus1: hpLinePointFrom(r.Next16nMinus1), Prev16nMinus1: hpLinePointFrom(r.Prev16nMinus1),
	}
}

// adjustModifier は補正を確かめて engine に渡す値にする。省略は 4096、明示した値は
// engine.MinEffectModifier..engine.MaxEffectModifier でなければならない(技の有無にかかわらず検査する。ADR-0250 §3・§5)。
func adjustModifier(path string, p *int) (int, error) {
	if p == nil {
		return engine.Modifier4096, nil
	}
	if *p < engine.MinEffectModifier || *p > engine.MaxEffectModifier {
		return 0, fail(CodeInvalidInput, "%s は %d..%d でなければならない: %d", path, engine.MinEffectModifier, engine.MaxEffectModifier, *p)
	}
	return *p, nil
}

func (r *adjustIndicesRequest) run() (adjustIndicesResultDTO, error) {
	// 値域は DTO の変換(列挙の検証・ID の解決)より前(ADR-0250 §5)。
	modifier, err := adjustModifier("modifier", r.Modifier)
	if err != nil {
		return adjustIndicesResultDTO{}, err
	}
	damageModifier, err := adjustModifier("damageModifier", r.DamageModifier)
	if err != nil {
		return adjustIndicesResultDTO{}, err
	}
	self, err := r.Individual.toEngine("individual")
	if err != nil {
		return adjustIndicesResultDTO{}, err
	}
	var move *engine.Move
	if r.Move != nil {
		m, err := r.Move.toEngine("move")
		if err != nil {
			return adjustIndicesResultDTO{}, err
		}
		move = &m
	}

	var out adjustIndicesResultDTO
	if move != nil {
		fp, err := engine.FirepowerIndex(self, move.Category, move.Power, modifier)
		if err != nil {
			return adjustIndicesResultDTO{}, err
		}
		v := int64(fp)
		out.FirepowerIndex = &v
	}
	phys, err := engine.BulkIndex(self, engine.CategoryPhysical, damageModifier)
	if err != nil {
		return adjustIndicesResultDTO{}, err
	}
	spec, err := engine.BulkIndex(self, engine.CategorySpecial, damageModifier)
	if err != nil {
		return adjustIndicesResultDTO{}, err
	}
	lines, err := engine.HPLines(self.Species.BaseStats.HP, self.SP.HP)
	if err != nil {
		return adjustIndicesResultDTO{}, err
	}
	out.Stats = statsFrom(engine.RealStats(self))
	out.PhysicalBulkIndex = int64(phys)
	out.SpecialBulkIndex = int64(spec)
	out.HPLines = hpLineReportFrom(lines)
	return out, nil
}

// --- adjustMinSpToKo / adjustMinSpToSurvive -----------------------------------

type adjustSearchRequest struct {
	Format   string        `json:"format"`
	Attacker individualDTO `json:"attacker"`
	Defender individualDTO `json:"defender"`
	Move     moveDTO       `json:"move"`
	Field    fieldDTO      `json:"field"`
	Critical bool          `json:"critical"`
	// Hits は必須(1..engine.MaxAdjustHits)。
	Hits int `json:"hits"`
	// ThresholdPercent は省略で 100(確定)。明示の値は (0, 100]。
	ThresholdPercent *float64 `json:"thresholdPercent"`
	// TypeChart は必須。省略は type_chart_missing(ADR-0011 §13)。
	TypeChart typeChartDTO `json:"typeChart"`
}

type adjustKOResultDTO struct {
	Stat          string               `json:"stat"`
	SearchLimit   int                  `json:"searchLimit"`
	Feasible      bool                 `json:"feasible"`
	SP            int                  `json:"sp"`
	ChancePercent float64              `json:"chancePercent"`
	Unsupported   []unsupportedMarkDTO `json:"unsupported"`
}

type adjustSurviveResultDTO struct {
	Stat          string               `json:"stat"`
	SearchLimit   int                  `json:"searchLimit"`
	Feasible      bool                 `json:"feasible"`
	HPSP          int                  `json:"hpSp"`
	StatSP        int                  `json:"statSp"`
	TotalSP       int                  `json:"totalSp"`
	BulkIndex     int64                `json:"bulkIndex"`
	ChancePercent float64              `json:"chancePercent"`
	Unsupported   []unsupportedMarkDTO `json:"unsupported"`
}

// toEngine は値域を検査してから DTO を変換する。両側の個体は Validate しない(engine の検査に一本化。ADR-0250 §2)。
func (r *adjustSearchRequest) toEngine() (engine.AdjustSearchInput, error) {
	if err := checkAdjustHits("hits", r.Hits); err != nil {
		return engine.AdjustSearchInput{}, err
	}
	threshold, err := adjustThreshold("thresholdPercent", r.ThresholdPercent)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	format, err := parseFormat(r.Format)
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	attacker, err := r.Attacker.toEngine("attacker")
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	defender, err := r.Defender.toEngine("defender")
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	move, err := r.Move.toEngine("move")
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	field, err := r.Field.toEngine()
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	chart, err := r.TypeChart.toEngine("typeChart")
	if err != nil {
		return engine.AdjustSearchInput{}, err
	}
	return engine.AdjustSearchInput{
		Format: format, Attacker: attacker, Defender: defender, Move: move, Field: field, Critical: r.Critical,
		TypeChart: chart, Hits: r.Hits, ThresholdPercent: threshold,
	}, nil
}

func (r *adjustSearchRequest) runKO() (adjustKOResultDTO, error) {
	in, err := r.toEngine()
	if err != nil {
		return adjustKOResultDTO{}, err
	}
	res, err := engine.MinSPToKO(in)
	if err != nil {
		return adjustKOResultDTO{}, err
	}
	return adjustKOResultDTO{
		Stat: string(res.Stat), SearchLimit: res.SearchLimit, Feasible: res.Feasible, SP: res.SP,
		ChancePercent: res.ChancePercent, Unsupported: unsupportedFrom(res.Unsupported),
	}, nil
}

func (r *adjustSearchRequest) runSurvive() (adjustSurviveResultDTO, error) {
	in, err := r.toEngine()
	if err != nil {
		return adjustSurviveResultDTO{}, err
	}
	res, err := engine.MinSPToSurvive(in)
	if err != nil {
		return adjustSurviveResultDTO{}, err
	}
	return adjustSurviveResultDTO{
		Stat: string(res.Stat), SearchLimit: res.SearchLimit, Feasible: res.Feasible,
		HPSP: res.HPSP, StatSP: res.StatSP, TotalSP: res.TotalSP, BulkIndex: int64(res.BulkIndex),
		ChancePercent: res.ChancePercent, Unsupported: unsupportedFrom(res.Unsupported),
	}, nil
}

// --- adjustAllocation ---------------------------------------------------------

// adjustCeilingDTO は能力ごとの上限。省略した能力は engine.MaxSPPerStat(ADR-0250 §3)。
type adjustCeilingDTO struct {
	HP  *int `json:"hp"`
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	SpA *int `json:"spa"`
	SpD *int `json:"spd"`
	Spe *int `json:"spe"`
}

// toEngine は値域 0..MaxSPPerStat を確かめ、省略を MaxSPPerStat で補う。
func (c adjustCeilingDTO) toEngine() (engine.Stats, error) {
	var out engine.Stats
	for _, f := range []struct {
		key engine.StatKey
		v   *int
	}{
		{engine.StatHP, c.HP}, {engine.StatAtk, c.Atk}, {engine.StatDef, c.Def},
		{engine.StatSpA, c.SpA}, {engine.StatSpD, c.SpD}, {engine.StatSpe, c.Spe},
	} {
		v := engine.MaxSPPerStat
		if f.v != nil {
			v = *f.v
		}
		if v < 0 || v > engine.MaxSPPerStat {
			return engine.Stats{}, fail(CodeInvalidInput, "ceiling.%s は 0..%d でなければならない: %d", f.key, engine.MaxSPPerStat, v)
		}
		out = out.WithStat(f.key, v)
	}
	return out, nil
}

type adjustAllocGoalDTO struct {
	Format           string        `json:"format"`
	Opponent         individualDTO `json:"opponent"`
	Move             moveDTO       `json:"move"`
	Field            fieldDTO      `json:"field"`
	Critical         bool          `json:"critical"`
	Hits             int           `json:"hits"`
	ThresholdPercent *float64      `json:"thresholdPercent"`
}

type adjustAllocationRequest struct {
	Self            individualDTO       `json:"self"`
	Ceiling         adjustCeilingDTO    `json:"ceiling"`
	Mode            string              `json:"mode"`
	Focus           string              `json:"focus"`
	OffenseCategory string              `json:"offenseCategory"`
	MinSpeed        int                 `json:"minSpeed"`
	Goal            *adjustAllocGoalDTO `json:"goal"`
	// TypeChart は goal があるときだけ必須(goal が無ければ読まない。ADR-0250 §4)。
	TypeChart typeChartDTO `json:"typeChart"`
}

type adjustAllocPlanDTO struct {
	SP            statsDTO `json:"sp"`
	TotalSP       int      `json:"totalSp"`
	Stats         statsDTO `json:"stats"`
	PhysicalBulk  int64    `json:"physicalBulk"`
	SpecialBulk   int64    `json:"specialBulk"`
	SpeedMet      bool     `json:"speedMet"`
	GoalMet       bool     `json:"goalMet"`
	ChancePercent float64  `json:"chancePercent"`
}

type adjustAllocationResultDTO struct {
	Remaining   int                  `json:"remaining"`
	MaxIndex    adjustAllocPlanDTO   `json:"maxIndex"`
	MinSP       *adjustAllocPlanDTO  `json:"minSp"`
	Unsupported []unsupportedMarkDTO `json:"unsupported"`
}

var validAllocModes = map[engine.AllocMode]bool{engine.AllocModeBulk: true, engine.AllocModeOffense: true}

var validBulkFocuses = map[engine.BulkFocus]bool{
	engine.BulkFocusPhysical: true, engine.BulkFocusSpecial: true, engine.BulkFocusBoth: true,
}

func allocPlanFrom(p engine.AllocPlan) adjustAllocPlanDTO {
	return adjustAllocPlanDTO{
		SP: statsFrom(p.SP), TotalSP: p.TotalSP, Stats: statsFrom(p.Real),
		PhysicalBulk: int64(p.PhysicalBulk), SpecialBulk: int64(p.SpecialBulk),
		SpeedMet: p.SpeedMet, GoalMet: p.GoalMet, ChancePercent: p.ChancePercent,
	}
}

func (r *adjustAllocationRequest) run() (adjustAllocationResultDTO, error) {
	// 値域は DTO の変換(列挙の検証)より前(ADR-0108 決定3)。
	ceiling, err := r.Ceiling.toEngine()
	if err != nil {
		return adjustAllocationResultDTO{}, err
	}
	if r.MinSpeed < 0 {
		return adjustAllocationResultDTO{}, fail(CodeInvalidInput, "minSpeed は 0 以上でなければならない: %d", r.MinSpeed)
	}
	var goalThreshold float64
	if r.Goal != nil {
		if err := checkAdjustHits("goal.hits", r.Goal.Hits); err != nil {
			return adjustAllocationResultDTO{}, err
		}
		if goalThreshold, err = adjustThreshold("goal.thresholdPercent", r.Goal.ThresholdPercent); err != nil {
			return adjustAllocationResultDTO{}, err
		}
	}

	// mode は必須の列挙(欠落も invalid_enum)。focus / offenseCategory は値があれば列挙だけ検査する。
	mode := engine.AllocMode(r.Mode)
	if !validAllocModes[mode] {
		return adjustAllocationResultDTO{}, enumError("mode", r.Mode)
	}
	focus := engine.BulkFocus(r.Focus)
	if r.Focus != "" && !validBulkFocuses[focus] {
		return adjustAllocationResultDTO{}, enumError("focus", r.Focus)
	}
	category, err := parseCategory("offenseCategory", r.OffenseCategory, true)
	if err != nil {
		return adjustAllocationResultDTO{}, err
	}
	self, err := r.Self.toEngine("self")
	if err != nil {
		return adjustAllocationResultDTO{}, err
	}

	in := engine.SPAllocInput{
		Self: self, Ceiling: ceiling, Mode: mode, Focus: focus, OffenseCategory: category, MinSpeed: r.MinSpeed,
	}
	if g := r.Goal; g != nil {
		format, err := parseFormat(g.Format)
		if err != nil {
			return adjustAllocationResultDTO{}, err
		}
		opponent, err := g.Opponent.toEngine("goal.opponent")
		if err != nil {
			return adjustAllocationResultDTO{}, err
		}
		move, err := g.Move.toEngine("goal.move")
		if err != nil {
			return adjustAllocationResultDTO{}, err
		}
		field, err := g.Field.toEngine()
		if err != nil {
			return adjustAllocationResultDTO{}, err
		}
		chart, err := r.TypeChart.toEngine("typeChart")
		if err != nil {
			return adjustAllocationResultDTO{}, err
		}
		in.Goal = &engine.AllocGoal{
			Format: format, Opponent: opponent, Move: move, Field: field, Critical: g.Critical,
			TypeChart: chart, Hits: g.Hits, ThresholdPercent: goalThreshold,
		}
	}

	res, err := engine.SuggestSPAllocation(in)
	if err != nil {
		return adjustAllocationResultDTO{}, err
	}
	out := adjustAllocationResultDTO{
		Remaining: res.Remaining, MaxIndex: allocPlanFrom(res.MaxIndex), Unsupported: unsupportedFrom(res.Unsupported),
	}
	if res.MinSP != nil {
		p := allocPlanFrom(*res.MinSP)
		out.MinSP = &p
	}
	return out, nil
}
