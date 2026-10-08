package master

import (
	"fmt"

	"example.com/pokecalc/engine"
)

// MoveRow は moves テーブルの行(engine が使わない accuracy・pp・name_en は含まない)。
// Effect は move_effects に行が無ければ nil(空スライスも「効果なし」として扱う)。
// Mechanisms は move_mechanisms の機構(ADR-0121)。行が無ければ空(通常の技)。
type MoveRow struct {
	ID       string
	NameJa   string
	Type     string
	Category string
	Power    int
	Priority int
	Effect   []byte
	// Mechanisms は Move で検証し、昇順に並べて engine.Move.Mechanisms に載せる(未対応の印。ADR-0123)。
	Mechanisms []string
	// Target は技の対象(Showdown の文字列。ADR-0136)。空は不明(取り込み前の行・内部 API がまだ運ばない経路)。
	// Move で検証し、engine.Move.Target(single/spread)に分類して載せる(ADR-0223)。
	Target string
	// Params は move_mechanism_params の行(ADR-0142)。行が無ければ nil(中身なし)。Move で機構との対応・値域を検証し、
	// engine.Move.Params に写す。
	Params *MoveMechanismParamsRow
}

// MoveMechanismParamsRow は move_mechanism_params の行(1技1行。ADR-0142 §7)。ゼロ値は「その項目なし」。
type MoveMechanismParamsRow struct {
	MultiHitMin      int // 多段の最小回数(Min == Max は固定回数)。0 は多段の中身なし
	MultiHitMax      int
	FixedDamageLevel bool // 固定ダメージが攻撃側のレベルと同じ
	FixedDamageValue int  // 固定ダメージの数値。0 は数値なし
	OHKO             bool // 一撃必殺の中身あり
	OHKOImmuneType   string
	OffenseStat      string // 攻撃に使う能力値(atk / def / spa / spd / spe)。"" は分類どおり
	OffensePokemon   string // attacker / defender。"" は攻撃側
	DefenseStat      string // 防御に使う能力値。"" は分類どおり
}

// mechanismParams は行を engine.MechanismParams に写す。何も中身が無い行・OHKO でないのに効かないタイプがある行は不正。
func (r MoveMechanismParamsRow) mechanismParams() (engine.MechanismParams, error) {
	var p engine.MechanismParams
	if r.MultiHitMin != 0 || r.MultiHitMax != 0 {
		p.MultiHit = &engine.MultiHit{Min: r.MultiHitMin, Max: r.MultiHitMax}
	}
	if r.FixedDamageLevel || r.FixedDamageValue != 0 {
		p.FixedDamage = &engine.FixedDamage{Level: r.FixedDamageLevel, Value: r.FixedDamageValue}
	}
	if r.OHKO {
		p.OHKO = &engine.OHKO{ImmuneType: engine.Type(r.OHKOImmuneType)}
	} else if r.OHKOImmuneType != "" {
		return engine.MechanismParams{}, fmt.Errorf("%w: 一撃必殺でないのに効かないタイプがある: %q", ErrInvalidRow, r.OHKOImmuneType)
	}
	p.OffenseStat = engine.StatKey(r.OffenseStat)
	p.OffensePokemon = engine.OffensePokemon(r.OffensePokemon)
	p.DefenseStat = engine.StatKey(r.DefenseStat)
	if p.MultiHit == nil && p.FixedDamage == nil && p.OHKO == nil &&
		p.OffenseStat == "" && p.OffensePokemon == "" && p.DefenseStat == "" {
		return engine.MechanismParams{}, fmt.Errorf("%w: 機構の中身が空の行", ErrInvalidRow)
	}
	return p, nil
}

// moveCategories は moves.category として許される値(ADR-0100 §3)。
var moveCategories = map[string]engine.MoveCategory{
	"physical": engine.CategoryPhysical,
	"special":  engine.CategorySpecial,
	"status":   engine.CategoryStatus,
}

// Move は技の行を engine.Move に写像する。
func Move(row MoveRow, chart engine.TypeChart) (engine.Move, error) {
	if chart.IsZero() {
		return engine.Move{}, fmt.Errorf("%w: タイプ相性表が未設定", ErrInvalidRow)
	}
	if !codeIDPattern.MatchString(row.ID) {
		return engine.Move{}, fmt.Errorf("%w: 技 ID の形式が不正: %q", ErrInvalidRow, row.ID)
	}
	if row.NameJa == "" {
		return engine.Move{}, fmt.Errorf("%w: 日本語名が空", ErrInvalidRow)
	}
	if row.Type == "" || !chart.Has(engine.Type(row.Type)) {
		return engine.Move{}, fmt.Errorf("%w: タイプが不正: %q", ErrInvalidRow, row.Type)
	}
	category, ok := moveCategories[row.Category]
	if !ok {
		return engine.Move{}, fmt.Errorf("%w: 分類が不正: %q", ErrInvalidRow, row.Category)
	}
	if row.Power < 0 || row.Power > 999 {
		return engine.Move{}, fmt.Errorf("%w: 威力が範囲外(0..999): %d", ErrInvalidRow, row.Power)
	}
	if category == engine.CategoryStatus && row.Power != 0 {
		return engine.Move{}, fmt.Errorf("%w: 変化技なのに威力がある: %d", ErrInvalidRow, row.Power)
	}
	if row.Priority < -7 || row.Priority > 5 {
		return engine.Move{}, fmt.Errorf("%w: 優先度が範囲外(-7..5): %d", ErrInvalidRow, row.Priority)
	}
	mechanisms, err := MoveMechanismsOf(row)
	if err != nil {
		return engine.Move{}, err
	}
	target, err := MoveTargetOf(row)
	if err != nil {
		return engine.Move{}, err
	}
	var effect *engine.MoveEffect
	if len(row.Effect) > 0 {
		e, err := DecodeMoveEffect(row.Effect)
		if err != nil {
			return engine.Move{}, err
		}
		effect = e
	}
	var params engine.MechanismParams
	if row.Params != nil {
		if params, err = row.Params.mechanismParams(); err != nil {
			return engine.Move{}, err
		}
	}
	move := engine.Move{
		ID:         row.ID,
		NameJa:     row.NameJa,
		Type:       engine.Type(row.Type),
		Category:   category,
		Power:      row.Power,
		Priority:   row.Priority,
		Effect:     effect,
		Mechanisms: mechanisms,
		Params:     params,
		Target:     target.Engine(),
	}
	if err := move.ValidateParams(chart); err != nil {
		return engine.Move{}, fmt.Errorf("%w: %v", ErrInvalidRow, err)
	}
	return move, nil
}
