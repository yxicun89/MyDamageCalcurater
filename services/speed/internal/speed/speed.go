// Package speed は素早さ比較の純粋なコア(I/O なし)。実数値・ランクの式は engine を呼び、
// engine に無いこだわりスカーフの補正だけをここで持つ(ADR-0600 §3)。
package speed

import "errors"

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
	panic("unimplemented")
}
