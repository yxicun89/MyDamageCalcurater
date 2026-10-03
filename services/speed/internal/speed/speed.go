// Package speed は素早さ比較の純粋なコア(I/O なし)。実数値・ランクの式は engine を呼び、
// engine に無いこだわりスカーフの補正だけをここで持つ(ADR-0600 §3)。
package speed

import (
	"errors"

	"example.com/pokecalc/engine"
)

// 種族値・ランクの範囲(ADR-0600 §3)。
const (
	minBaseSpeed = 1
	maxBaseSpeed = 255
	minRank      = -6
	maxRank      = 6
)

// scarfSpeedModifier はこだわりスカーフの素早さ補正(×1.5)を engine.Modifier4096 基準で表した値。
// ランク補正の後に、この値を五捨五超入で掛ける(出典: ADR-0600 §3。Showdown の
// Pokemon.getStat → ModifySpe の chainModify(1.5) に相当)。
const scarfSpeedModifier = 6144

// tailwindSpeedModifier は追い風の素早さ補正(×2)を engine.Modifier4096 基準で表した値(ADR-0607 §2)。
const tailwindSpeedModifier = 8192

// まひの素早さ補正(ADR-0607 §3)。4096 基準の補正ではなく、連結・五捨五超入の後に
// floor(v × paralysisSpeedPercent / percentDenominator) の整数演算で掛ける(@smogon/calc 0.12.0 の
// Champions 世代は 50/100)。float の 0.5 は使わない。
const (
	paralysisSpeedPercent = 50
	percentDenominator    = 100
)

// NatureEffect は素早さに対する性格の補正(ADR-0600 §3)。
type NatureEffect string

const (
	// NatureMinus は素早さ下降の性格(上昇は攻撃)。
	NatureMinus NatureEffect = "minus"
	// NatureNeutral は補正なしの性格。
	NatureNeutral NatureEffect = "neutral"
	// NaturePlus は素早さ上昇の性格(下降は攻撃)。
	NaturePlus NatureEffect = "plus"
)

// 入力検証の sentinel エラー。検証はコアが持つドメインの不変条件(coding-rules §3)。
var (
	// ErrInvalidBaseSpeed は素早さ種族値が 1〜255 の外であることを表す。
	ErrInvalidBaseSpeed = errors.New("base speed must be between 1 and 255")
	// ErrInvalidSP は素早さ SP が 0〜engine.MaxSPPerStat の外であることを表す。
	ErrInvalidSP = errors.New("speed SP is out of range")
	// ErrInvalidNature は性格の補正が minus / neutral / plus 以外であることを表す。
	ErrInvalidNature = errors.New("nature effect must be minus, neutral or plus")
	// ErrInvalidRank は素早さのランクが -6〜+6 の外であることを表す。
	ErrInvalidRank = errors.New("speed rank must be between -6 and +6")
)

// Input は素早さ 1 通り分の入力(ADR-0600 §3)。
type Input struct {
	BaseSpeed int
	SP        int
	Nature    NatureEffect
	Rank      int
	Scarf     bool
	// Tailwind は追い風(その個体の側)。Paralysis はまひ(ADR-0607 §1)。
	Tailwind  bool
	Paralysis bool
}

// Speed は入力から戦闘中の素早さを返す。順序は 実数値(engine.RealStats)→ ランク(engine.EffectiveStat)
// → 追い風・こだわりスカーフ(4096 基準で連結して 1 回だけ五捨五超入)→ まひ(floor(v × 50 / 100))。入力が範囲外なら sentinel エラーを包んで返す。
func Speed(in Input) (int, error) {
	if in.BaseSpeed < minBaseSpeed || in.BaseSpeed > maxBaseSpeed {
		return 0, ErrInvalidBaseSpeed
	}
	if in.SP < 0 || in.SP > engine.MaxSPPerStat {
		return 0, ErrInvalidSP
	}
	if in.Rank < minRank || in.Rank > maxRank {
		return 0, ErrInvalidRank
	}
	nature, err := toEngineNature(in.Nature)
	if err != nil {
		return 0, err
	}

	individual := engine.Individual{
		Species: engine.Species{BaseStats: engine.Stats{Spe: in.BaseSpeed}},
		Nature:  nature,
		SP:      engine.Stats{Spe: in.SP},
		Ranks:   engine.Ranks{Spe: in.Rank},
	}
	v := engine.EffectiveStat(individual, engine.StatSpe)
	// 配列に積む順は追い風が先、こだわりスカーフが後(ADR-0607 §2)。
	var mods []int
	if in.Tailwind {
		mods = append(mods, tailwindSpeedModifier)
	}
	if in.Scarf {
		mods = append(mods, scarfSpeedModifier)
	}
	if len(mods) > 0 {
		v = applySpeedModifiers(v, mods)
	}
	if in.Paralysis {
		v = v * paralysisSpeedPercent / percentDenominator
	}
	return v, nil
}

// toEngineNature は素早さに対する性格の補正を engine.Nature に変換する(ADR-0600 §3)。
// minus/plus の「もう一方」(下降/上昇)は攻撃に割り当てる。engine の Nature は Plus/Minus の
// 組で1性格を表すため、素早さ以外のどちらかを埋める必要がある(値には影響しない)。
func toEngineNature(n NatureEffect) (engine.Nature, error) {
	switch n {
	case NatureMinus:
		return engine.Nature{Plus: engine.StatAtk, Minus: engine.StatSpe}, nil
	case NatureNeutral:
		return engine.NatureNeutral, nil
	case NaturePlus:
		return engine.Nature{Plus: engine.StatSpe, Minus: engine.StatAtk}, nil
	default:
		return engine.Nature{}, ErrInvalidNature
	}
}

// chainSpeedModifiers は素早さ補正(各値は engine.Modifier4096 基準)を 1 つに連結する
// (@smogon/calc 0.12.0 の chainMods。1 ステップ M = (M × mod + engine.Modifier4096/2) / engine.Modifier4096、
// 4096 から始める。ADR-0607 §2)。mods は原典が積む順(追い風 → 持ち物)で渡す。
func chainSpeedModifiers(mods []int) int {
	m := engine.Modifier4096
	for _, mod := range mods {
		m = (m*mod + engine.Modifier4096/2) / engine.Modifier4096
	}
	return m
}

// applySpeedModifiers はランク適用後の実数値に、連結した補正を五捨五超入で 1 回だけ掛ける
// (floor((v × M + engine.Modifier4096/2 − 1) / engine.Modifier4096))。
func applySpeedModifiers(v int, mods []int) int {
	m := chainSpeedModifiers(mods)
	return (v*m + engine.Modifier4096/2 - 1) / engine.Modifier4096
}
