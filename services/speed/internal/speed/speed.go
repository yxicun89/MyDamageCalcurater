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
}

// Speed は入力から戦闘中の素早さを返す。順序は 実数値(engine.RealStats)→ ランク(engine.EffectiveStat)
// → こだわりスカーフ(×6144/4096 の五捨五超入)。入力が範囲外なら sentinel エラーを包んで返す。
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
	if in.Scarf {
		v = applyScarf(v)
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

// applyScarf はランク適用後の実数値にこだわりスカーフの補正を五捨五超入で掛ける
// (floor((v × scarfSpeedModifier + engine.Modifier4096/2 − 1) / engine.Modifier4096))。
func applyScarf(v int) int {
	return (v*scarfSpeedModifier + engine.Modifier4096/2 - 1) / engine.Modifier4096
}
