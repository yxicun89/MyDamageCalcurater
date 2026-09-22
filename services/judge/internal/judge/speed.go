// Package judge is judge's own pure core (no I/O): the one calculation JD1 does itself is
// comparing battle speed, reusing engine.RealStats/EffectiveStat for the real stat and rank
// steps and adding only the choice scarf modifier, which engine doesn't have (ADR-0701 §2,
// same posture as services/speed/internal/speed and ADR-0600 §3).
package judge

import (
	"errors"

	"example.com/pokecalc/engine"
)

// 種族値・ランクの範囲(ADR-0701 §2。services/speed/internal/speed と同じ範囲)。
const (
	minBaseSpeed = 1
	maxBaseSpeed = 255
	minRank      = -6
	maxRank      = 6
)

// scarfSpeedModifier はこだわりスカーフの素早さ補正(×1.5)を engine.Modifier4096 基準で表した値
// (ADR-0701 §2。services/speed/internal/speed/speed.go と同一の出典・式)。
const scarfSpeedModifier = 6144

// DefaultChoiceScarfItemID は pokedex-svc の持ち物 ID の命名規則(Showdown ID。小文字英数のみ)
// に沿った既定値(ADR-0701 §3)。cmd/api が環境変数で上書きできる。
const DefaultChoiceScarfItemID = "choicescarf"

// 入力検証の sentinel エラー。検証はコアが持つドメインの不変条件(coding-rules §3)。
var (
	// ErrInvalidBaseSpeed は素早さ種族値が 1〜255 の外であることを表す。
	ErrInvalidBaseSpeed = errors.New("base speed must be between 1 and 255")
	// ErrInvalidSP は SP が 0〜engine.MaxSPPerStat の外、または合計が engine.MaxSPTotal 超であることを表す。
	ErrInvalidSP = errors.New("SP is out of range")
	// ErrInvalidRank はランクが -6〜+6 の外であることを表す。
	ErrInvalidRank = errors.New("rank must be between -6 and +6")
)

// Individual は JD1 の素早さ計算に使う個体(ADR-0701 §2)。状態異常(status)・テラスタルは
// JD1 では扱わないため持たない。
type Individual struct {
	BaseSpeed int
	Nature    engine.Nature
	SP        engine.Stats
	Ranks     engine.Ranks
	Scarf     bool
}

// SpeedComparison は attacker/defender の戦闘中の素早さの比較結果(ADR-0700 §6-1)。
type SpeedComparison struct {
	AttackerSpeed int
	DefenderSpeed int
	Outspeeds     bool
	SpeedTie      bool
}

// Speed は 実数値(engine.RealStats)→ ランク補正(engine.EffectiveStat)→ こだわりスカーフ
// (×6144/4096・五捨五超入)の順で戦闘中の素早さを求める(ADR-0701 §2)。judge は式を複製せず、
// engine に無いこだわりスカーフの補正だけを自分で持つ。入力が範囲外なら sentinel エラーを返す。
func Speed(in Individual) (int, error) {
	if in.BaseSpeed < minBaseSpeed || in.BaseSpeed > maxBaseSpeed {
		return 0, ErrInvalidBaseSpeed
	}
	if err := validateSP(in.SP); err != nil {
		return 0, err
	}
	if err := validateRanks(in.Ranks); err != nil {
		return 0, err
	}

	individual := engine.Individual{
		Species: engine.Species{BaseStats: engine.Stats{Spe: in.BaseSpeed}},
		Nature:  in.Nature,
		SP:      in.SP,
		Ranks:   in.Ranks,
	}
	v := engine.EffectiveStat(individual, engine.StatSpe)
	if in.Scarf {
		v = applyChoiceScarf(v)
	}
	return v, nil
}

// applyChoiceScarf はランク適用後の実数値にこだわりスカーフの補正を五捨五超入で掛ける
// (floor((v × scarfSpeedModifier + engine.Modifier4096/2 − 1) / engine.Modifier4096)。
// ADR-0701 §2・services/speed/internal/speed/speed.go の applyScarf と同一の式)。
func applyChoiceScarf(v int) int {
	return (v*scarfSpeedModifier + engine.Modifier4096/2 - 1) / engine.Modifier4096
}

// validateSP は SP の各ステータスが 0..MaxSPPerStat、合計が MaxSPTotal 以下であることを確かめる
// (CLAUDE.md のドメイン規約)。
func validateSP(sp engine.Stats) error {
	for _, k := range engine.AllStatKeys() {
		v := sp.Get(k)
		if v < 0 || v > engine.MaxSPPerStat {
			return ErrInvalidSP
		}
	}
	if sp.Sum() > engine.MaxSPTotal {
		return ErrInvalidSP
	}
	return nil
}

// validateRanks はランクが -6..+6 の範囲であることを確かめる。
func validateRanks(ranks engine.Ranks) error {
	for _, v := range []int{ranks.Atk, ranks.Def, ranks.SpA, ranks.SpD, ranks.Spe} {
		if v < minRank || v > maxRank {
			return ErrInvalidRank
		}
	}
	return nil
}

// CompareSpeed は attacker と defender の戦闘中の素早さを求めて比較する。outspeeds は厳密な
// >、speedTie は ==(ADR-0700 §6-1。同速を真偽値 1 つに丸めない)。どちらかが範囲外ならその
// sentinel エラーを返す。
func CompareSpeed(attacker, defender Individual) (SpeedComparison, error) {
	attackerSpeed, err := Speed(attacker)
	if err != nil {
		return SpeedComparison{}, err
	}
	defenderSpeed, err := Speed(defender)
	if err != nil {
		return SpeedComparison{}, err
	}
	return SpeedComparison{
		AttackerSpeed: attackerSpeed,
		DefenderSpeed: defenderSpeed,
		Outspeeds:     attackerSpeed > defenderSpeed,
		SpeedTie:      attackerSpeed == defenderSpeed,
	}, nil
}

// IsChoiceScarf は itemID が既知のこだわりスカーフ ID と一致するかを返す(ADR-0701 §3)。
// scarfItemID が空なら DefaultChoiceScarfItemID を使う。judge は持ち物の一覧を持たず、
// 設定 1 つの ID との一致だけで判定する。
func IsChoiceScarf(itemID, scarfItemID string) bool {
	if itemID == "" {
		return false
	}
	if scarfItemID == "" {
		scarfItemID = DefaultChoiceScarfItemID
	}
	return itemID == scarfItemID
}
