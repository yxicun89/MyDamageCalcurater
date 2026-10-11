package wasmapi

// 連続技 calcCombo の DTO・検証・結果の写し(ADR-0179 追記「CB-5」。docs/plan/combo-calc.md)。
// 攻撃側・防御側・場・技・相性表は calc の DTO を再利用する。計算は engine.CalcCombo の素通し。

import (
	"fmt"

	"example.com/pokecalc/engine"
)

type comboRequest struct {
	Format   string         `json:"format"`
	Attacker individualDTO  `json:"attacker"`
	Defender individualDTO  `json:"defender"`
	Field    fieldDTO       `json:"field"`
	Steps    []comboStepDTO `json:"steps"`
	// OpponentMove は相手が行動するときの技(攻撃技のみ)。省略・null は相手の攻撃を計算しない。
	OpponentMove *moveDTO `json:"opponentMove"`
	// OpponentSpeed は "unknown"(既定・"" も同じ)か "from_defender"。
	OpponentSpeed string        `json:"opponentSpeed"`
	SpeedField    speedFieldDTO `json:"speedField"`
	// TypeChart は必須。省略は type_chart_missing(ADR-0011 §13)。
	TypeChart typeChartDTO `json:"typeChart"`
}

type comboStepDTO struct {
	Move     moveDTO `json:"move"`
	Critical bool    `json:"critical"`
	// Hits は多段技の回数。指定した値は 1 以上(省略・null は既定。battleState.hits と同じ規則)。
	Hits *int `json:"hits"`
}

type speedFieldDTO struct {
	TrickRoom        bool `json:"trickRoom"`
	SelfTailwind     bool `json:"selfTailwind"`
	OpponentTailwind bool `json:"opponentTailwind"`
}

func (r *comboRequest) run() (comboResultDTO, error) {
	format, err := parseFormat(r.Format)
	if err != nil {
		return comboResultDTO{}, err
	}
	in := engine.ComboInput{Format: format, SpeedField: engine.ComboSpeedField(r.SpeedField)}
	if in.Attacker, err = r.Attacker.toEngine("attacker"); err != nil {
		return comboResultDTO{}, err
	}
	if in.Defender, err = r.Defender.toEngine("defender"); err != nil {
		return comboResultDTO{}, err
	}
	if in.Field, err = r.Field.toEngine(); err != nil {
		return comboResultDTO{}, err
	}
	if in.TypeChart, err = r.TypeChart.toEngine("typeChart"); err != nil {
		return comboResultDTO{}, err
	}
	switch s := engine.OpponentSpeedMode(r.OpponentSpeed); s {
	case "", engine.OpponentSpeedUnknown, engine.OpponentSpeedFromDefender:
		in.OpponentSpeed = s
	default:
		return comboResultDTO{}, enumError("opponentSpeed", r.OpponentSpeed)
	}
	// 手の数は DTO の変換より前に見る(多すぎる配列の変換を無駄にしない)。
	if n := len(r.Steps); n < 1 || n > engine.MaxComboSteps {
		return comboResultDTO{}, fail(CodeInvalidInput, "steps は 1..%d 手でなければならない: %d", engine.MaxComboSteps, n)
	}
	for i, s := range r.Steps {
		path := fmt.Sprintf("steps[%d]", i)
		move, err := s.Move.toEngine(path + ".move")
		if err != nil {
			return comboResultDTO{}, err
		}
		step := engine.ComboStep{Move: move, Critical: s.Critical}
		if s.Hits != nil {
			if *s.Hits < 1 {
				return comboResultDTO{}, fail(CodeInvalidInput, "%s.hits は 1 以上でなければならない: %d", path, *s.Hits)
			}
			step.Hits = *s.Hits
		}
		in.Steps = append(in.Steps, step)
	}
	if r.OpponentMove != nil {
		m, err := r.OpponentMove.toEngine("opponentMove")
		if err != nil {
			return comboResultDTO{}, err
		}
		in.OpponentMove = &m
	}
	if err := validateIndividual("攻撃側", in.Attacker); err != nil {
		return comboResultDTO{}, err
	}
	if err := validateIndividual("防御側", in.Defender); err != nil {
		return comboResultDTO{}, err
	}
	if err := r.Attacker.Species.checkMegaItem("攻撃側", r.Attacker.Item); err != nil {
		return comboResultDTO{}, err
	}
	if err := r.Defender.Species.checkMegaItem("防御側", r.Defender.Item); err != nil {
		return comboResultDTO{}, err
	}
	res, err := engine.CalcCombo(in)
	if err != nil {
		return comboResultDTO{}, err
	}
	return comboResultFrom(res), nil
}

// --- 結果 -------------------------------------------------------------------

type comboResultDTO struct {
	Steps []comboStepResultDTO `json:"steps"`
	Total comboTotalDTO        `json:"total"`
	Speed comboSpeedDTO        `json:"speed"`
	// OpponentAttack は opponentMove があるときだけ(無いときは null)。
	OpponentAttack *opponentAttackDTO `json:"opponentAttack"`
	// Unsupported は印なしで空配列(null にしない)。
	Unsupported []unsupportedMarkDTO `json:"unsupported"`
}

type comboStepResultDTO struct {
	MoveID             string         `json:"moveId"`
	SelfStartRanks     ranksDTO       `json:"selfStartRanks"`
	SelfEndRanks       ranksDTO       `json:"selfEndRanks"`
	OpponentStartRanks ranksDTO       `json:"opponentStartRanks"`
	OpponentEndRanks   ranksDTO       `json:"opponentEndRanks"`
	Damage             *calcResultDTO `json:"damage"` // 変化技は null
	SelfSpeed          int            `json:"selfSpeed"`
	OpponentSpeed      int            `json:"opponentSpeed"` // from_defender のときだけ(unknown は 0)
	SelfPriority       int            `json:"selfPriority"`
	OpponentPriority   int            `json:"opponentPriority"`
	Order              string         `json:"order"`
	Line               speedLineDTO   `json:"line"`
}

type speedRangeDTO struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type speedLineDTO struct {
	SelfSpeed     int           `json:"selfSpeed"`
	SelfFirst     speedRangeDTO `json:"selfFirst"`
	OpponentFirst speedRangeDTO `json:"opponentFirst"`
}

type comboKOChanceDTO struct {
	ChancePercent        float64      `json:"chancePercent"`
	Guaranteed           bool         `json:"guaranteed"`
	DisplayChancePercent tenthPercent `json:"displayChancePercent"`
}

type comboTotalDTO struct {
	MinDamage  int                `json:"minDamage"`
	MaxDamage  int                `json:"maxDamage"`
	DefenderHP int                `json:"defenderHP"`
	MinPercent tenthPercent       `json:"minPercent"`
	MaxPercent tenthPercent       `json:"maxPercent"`
	KO         comboKOChanceDTO   `json:"ko"`
	KOByStep   []comboKOChanceDTO `json:"koByStep"`
}

type comboSpeedDTO struct {
	FirstSelfSpeed     int  `json:"firstSelfSpeed"`
	LastSelfSpeed      int  `json:"lastSelfSpeed"`
	OpponentSpeed      int  `json:"opponentSpeed"`
	OpponentSpeedKnown bool `json:"opponentSpeedKnown"`
}

type opponentActionDTO struct {
	Step                 int           `json:"step"`
	Timing               string        `json:"timing"`
	Damage               calcResultDTO `json:"damage"`
	SurviveChancePercent float64       `json:"surviveChancePercent"`
}

type opponentScenarioDTO struct {
	Actions              []opponentActionDTO `json:"actions"`
	SurviveChancePercent float64             `json:"surviveChancePercent"`
}

type opponentAttackDTO struct {
	SelfHP     int                 `json:"selfHp"`
	Fast       opponentScenarioDTO `json:"fast"`
	Slow       opponentScenarioDTO `json:"slow"`
	MinActions int                 `json:"minActions"`
	MaxActions int                 `json:"maxActions"`
}

func ranksFrom(r engine.Ranks) ranksDTO {
	return ranksDTO{Atk: r.Atk, Def: r.Def, SpA: r.SpA, SpD: r.SpD, Spe: r.Spe}
}

func comboKOFrom(k engine.ComboKOChance) comboKOChanceDTO {
	return comboKOChanceDTO{ChancePercent: k.ChancePercent, Guaranteed: k.Guaranteed,
		DisplayChancePercent: tenthPercent(k.DisplayChancePercentTenths())}
}

func scenarioFrom(s engine.OpponentScenario) opponentScenarioDTO {
	out := opponentScenarioDTO{Actions: make([]opponentActionDTO, 0, len(s.Actions)), SurviveChancePercent: s.SurviveChancePercent}
	for _, a := range s.Actions {
		out.Actions = append(out.Actions, opponentActionDTO{Step: a.Step, Timing: string(a.Timing),
			Damage: calcResultFrom(a.Damage), SurviveChancePercent: a.SurviveChancePercent})
	}
	return out
}

func comboResultFrom(r engine.ComboResult) comboResultDTO {
	out := comboResultDTO{Steps: make([]comboStepResultDTO, 0, len(r.Steps)), Unsupported: unsupportedFrom(r.Unsupported)}
	for _, s := range r.Steps {
		d := comboStepResultDTO{
			MoveID: s.MoveID, SelfStartRanks: ranksFrom(s.SelfStartRanks), SelfEndRanks: ranksFrom(s.SelfEndRanks),
			OpponentStartRanks: ranksFrom(s.OpponentStartRanks), OpponentEndRanks: ranksFrom(s.OpponentEndRanks),
			SelfSpeed: s.SelfSpeed, OpponentSpeed: s.OpponentSpeed,
			SelfPriority: s.SelfPriority, OpponentPriority: s.OpponentPriority, Order: string(s.Order),
			Line: speedLineDTO{SelfSpeed: s.Line.SelfSpeed,
				SelfFirst:     speedRangeDTO{Min: s.Line.SelfFirst.Min, Max: s.Line.SelfFirst.Max},
				OpponentFirst: speedRangeDTO{Min: s.Line.OpponentFirst.Min, Max: s.Line.OpponentFirst.Max}},
		}
		if s.Damage != nil {
			c := calcResultFrom(*s.Damage)
			d.Damage = &c
		}
		out.Steps = append(out.Steps, d)
	}
	minTenths, maxTenths := r.Total.DisplayPercentRangeTenths()
	out.Total = comboTotalDTO{MinDamage: r.Total.MinDamage, MaxDamage: r.Total.MaxDamage, DefenderHP: r.Total.DefenderHP,
		MinPercent: tenthPercent(minTenths), MaxPercent: tenthPercent(maxTenths), KO: comboKOFrom(r.Total.KO),
		KOByStep: make([]comboKOChanceDTO, 0, len(r.Total.KOByStep))}
	for _, k := range r.Total.KOByStep {
		out.Total.KOByStep = append(out.Total.KOByStep, comboKOFrom(k))
	}
	out.Speed = comboSpeedDTO{FirstSelfSpeed: r.Speed.FirstSelfSpeed, LastSelfSpeed: r.Speed.LastSelfSpeed,
		OpponentSpeed: r.Speed.OpponentSpeed, OpponentSpeedKnown: r.Speed.OpponentSpeedKnown}
	if oa := r.OpponentAttack; oa != nil {
		out.OpponentAttack = &opponentAttackDTO{SelfHP: oa.SelfHP, Fast: scenarioFrom(oa.Fast), Slow: scenarioFrom(oa.Slow),
			MinActions: oa.MinActions, MaxActions: oa.MaxActions}
	}
	return out
}
