package engine

import (
	"errors"
	"fmt"
)

// 素早さと行動順の式(ADR-0179 §2・§3。@smogon/calc 0.12.0 の getFinalSpeed と同じ)。

// ErrInvalidSpeedInput は素早さの入力が範囲外(素早さの種族値 1..255・SP・ランク・性格)であることを表す。
var ErrInvalidSpeedInput = errors.New("素早さの入力が不正")

const (
	minBaseSpeed = 1
	maxBaseSpeed = 255
	// tailwindSpeedModifier は追い風の素早さ倍率(2 倍。4096 基準)。
	tailwindSpeedModifier = 8192
	// paralysisSpeedPercent はまひの素早さ倍率(%)。
	paralysisSpeedPercent = 50
)

// speedModBounds は素早さの補正の連鎖の範囲(oracle の getFinalSpeed の chainMods)。
var speedModBounds = modBounds{lower: 410, upper: 131172}

// maxSpeed は素早さの上限(oracle)。正当な入力(種族値 ≤ 255・SP ≤ 32・ランク ≤ +6・補正の連鎖)では届かない防御的な分岐。
const maxSpeed = 10000

// SpeedSide はその個体の側にかかっている、素早さに効く場の効果。
type SpeedSide struct{ Tailwind bool }

// SpeedResult は FinalSpeed の結果。
type SpeedResult struct {
	Speed int
	// AbilityUndetermined / ItemUndetermined は、成立を判定できない条件(item_lost・語彙に無い条件)を、
	// 掛けた要素より前で(または掛けた要素が無いまま)飛ばしたこと。呼び出し側が未対応の印にする。
	AbilityUndetermined bool
	ItemUndetermined    bool
}

// speedCondHolds は素早さの補正の条件が成り立つか。判定できない条件(item_lost・語彙に無い条件)は decided=false で不成立。
func speedCondHolds(c SpeedCondition, ind Individual, f Field) (holds, decided bool) {
	switch c {
	case SpeedConditionAlways:
		return true, true
	case SpeedConditionWeatherSun:
		return f.Weather == WeatherSun, true
	case SpeedConditionWeatherRain:
		return f.Weather == WeatherRain, true
	case SpeedConditionWeatherSand:
		return f.Weather == WeatherSand, true
	case SpeedConditionWeatherSnow:
		return f.Weather == WeatherSnow, true
	case SpeedConditionTerrainElectric:
		return f.Terrain == TerrainElectric, true
	case SpeedConditionHasStatus:
		return ind.Status != StatusNone && ind.Status != "", true
	}
	return false, false
}

// firstSpeedMod は成立した最初の素早さの補正を返す。判定できない条件は飛ばして次へ進み、undetermined を立てる
// (oracle の既定「持ち物を失っていない」と同じ。成立した要素より前で飛ばしたときだけ立つ)。
func firstSpeedMod(mods []SpeedMod, ind Individual, f Field) (modifier int, ok, undetermined bool) {
	for _, m := range mods {
		holds, decided := speedCondHolds(m.Condition, ind, f)
		if !decided {
			undetermined = true
			continue
		}
		if holds {
			return m.Modifier, true, undetermined
		}
	}
	return 0, false, undetermined
}

// validateSpeedInput は素早さに関わる入力だけを確かめる(タイプ・素早さ以外の種族値は見ない)。
func validateSpeedInput(ind Individual) error {
	if b := ind.Species.BaseStats.Spe; b < minBaseSpeed || b > maxBaseSpeed {
		return fmt.Errorf("%w: 素早さの種族値 %d は %d..%d", ErrInvalidSpeedInput, b, minBaseSpeed, maxBaseSpeed)
	}
	for _, k := range AllStatKeys() {
		if v := ind.SP.Get(k); v < 0 || v > MaxSPPerStat {
			return fmt.Errorf("%w: %s の SP %d は 0..%d", ErrInvalidSpeedInput, k, v, MaxSPPerStat)
		}
	}
	if sum := ind.SP.Sum(); sum > MaxSPTotal {
		return fmt.Errorf("%w: SP の合計 %d は %d 以下", ErrInvalidSpeedInput, sum, MaxSPTotal)
	}
	r := ind.Ranks
	for _, v := range []int{r.Atk, r.Def, r.SpA, r.SpD, r.Spe} {
		if v < -6 || v > 6 {
			return fmt.Errorf("%w: ランク %d は -6..+6", ErrInvalidSpeedInput, v)
		}
	}
	if ind.Nature.Plus == StatHP || ind.Nature.Minus == StatHP {
		return fmt.Errorf("%w: 性格補正に HP は使えない", ErrInvalidSpeedInput)
	}
	return nil
}

// FinalSpeed は戦闘中の素早さ: ランク → [追い風 → 特性 → 持ち物の補正(特性・持ち物は成立した最初の 1 つ)] の chainMods →
// pokeRound(1 回)→ まひ(IgnoresParalysisSpeedDrop が無ければ ×50/100 floor)→ 0..10000。
// こだわりスカーフは「SpeedMods: [{always, 6144}] を持つ持ち物」として渡す(engine は持ち物 ID を見ない)。
// 入力が範囲外なら ErrInvalidSpeedInput を %w で包んで返す。
func FinalSpeed(ind Individual, f Field, side SpeedSide) (SpeedResult, error) {
	if err := validateSpeedInput(ind); err != nil {
		return SpeedResult{}, err
	}
	return finalSpeedValue(ind, f, side), nil
}

// finalSpeedValue は検証なしの FinalSpeed(威力の式の finalSpeed と共有する)。
func finalSpeedValue(ind Individual, f Field, side SpeedSide) SpeedResult {
	var res SpeedResult
	speed := applyStatStage(RealStats(ind).Spe, ind.Ranks.Spe)
	var mods []int
	if side.Tailwind {
		mods = append(mods, tailwindSpeedModifier)
	}
	ae := ind.Ability.Effect
	if ae != nil {
		m, ok, und := firstSpeedMod(ae.SpeedMods, ind, f)
		res.AbilityUndetermined = und
		if ok {
			mods = append(mods, m)
		}
	}
	if e := itemEffect(ind.Item); e != nil {
		m, ok, und := firstSpeedMod(e.SpeedMods, ind, f)
		res.ItemUndetermined = und
		if ok {
			mods = append(mods, m)
		}
	}
	speed = pokeRound(speed, chainMods(mods, speedModBounds))
	if ind.Status == StatusParalysis && !(ae != nil && ae.IgnoresParalysisSpeedDrop) {
		speed = speed * paralysisSpeedPercent / 100
	}
	res.Speed = max(0, min(speed, maxSpeed))
	return res
}

// TurnOrder は行動順(閉じた語彙)。
type TurnOrder string

const (
	TurnOrderSelfFirst     TurnOrder = "self_first"
	TurnOrderOpponentFirst TurnOrder = "opponent_first"
	TurnOrderSpeedTie      TurnOrder = "speed_tie"
)

// CompareSpeed は素早さだけで行動順を決める。トリックルームなら遅い方が先。同速は TurnOrderSpeedTie(反転しない)。
func CompareSpeed(selfSpeed, opponentSpeed int, trickRoom bool) TurnOrder {
	switch {
	case selfSpeed == opponentSpeed:
		return TurnOrderSpeedTie
	case (selfSpeed > opponentSpeed) != trickRoom:
		return TurnOrderSelfFirst
	}
	return TurnOrderOpponentFirst
}

// CompareTurnOrder は優先度が違えば高い方が先(素早さ・トリックルームを見ない)。同じなら bySpeed をそのまま返す。
func CompareTurnOrder(selfPriority, opponentPriority int, bySpeed TurnOrder) TurnOrder {
	switch {
	case selfPriority > opponentPriority:
		return TurnOrderSelfFirst
	case selfPriority < opponentPriority:
		return TurnOrderOpponentFirst
	}
	return bySpeed
}

// EffectivePriority は技の優先度に、技の処理の定義の PriorityBoost(使用者が接地していてそのフィールド)を足した値。
func EffectivePriority(m Move, user Individual, f Field) int {
	p := m.Priority
	if r := m.Rule; r != nil {
		if pb := r.PriorityBoost; pb != nil && isGrounded(user) && f.Terrain == pb.Terrain {
			p += pb.Delta
		}
	}
	return p
}
