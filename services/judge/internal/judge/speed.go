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

// tailwindSpeedModifier は追い風の素早さ補正(×2)を engine.Modifier4096 基準で表した値
// (ADR-0702 §2。出典は @smogon/calc 0.12.0 の dist/mechanics/util.js の getFinalSpeed)。
const tailwindSpeedModifier = 8192

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
	// Tailwind は追い風がこの個体の側にかかっているか(ADR-0702 §4)。Scarf と同じ「その個体の
	// 素早さに乗る補正」なので、CompareSpeed の引数ではなく Individual に置く。
	Tailwind bool
}

// SpeedField は CompareSpeed の第3引数で、比較そのものに効く場の効果を表す(ADR-0702 §4)。
// 追い風は片側の値を変えるだけなので Individual.Tailwind に置き、ここには持たせない。
type SpeedField struct {
	// TrickRoom はトリックルームがかかっているか。実数値は変えず、outspeeds の向きだけを
	// 反転する(ADR-0702 §3)。
	TrickRoom bool
}

// SpeedComparison は attacker/defender の戦闘中の素早さの比較結果(ADR-0700 §6-1)。
type SpeedComparison struct {
	AttackerSpeed int
	DefenderSpeed int
	Outspeeds     bool
	SpeedTie      bool
}

// Speed は 実数値(engine.RealStats)→ ランク補正(engine.EffectiveStat)→ 素早さ補正(追い風・
// こだわりスカーフ)の順で戦闘中の素早さを求める(ADR-0701 §2・ADR-0702 §2)。効いている補正は
// 4096 基準で 1 つに連結してから 1 回だけ五捨五超入する(各補正ごとに丸めない)。judge は式を
// 複製せず、engine に無い素早さ補正だけを自分で持つ。入力が範囲外なら sentinel エラーを返す。
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

	// 配列に積む順は追い風が先、こだわりスカーフが後(ADR-0702 §2 の出典どおり)。
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
	return v, nil
}

// chainSpeedModifiers merges speed modifiers (each expressed in engine.Modifier4096 units) into
// a single modifier, replicating @smogon/calc 0.12.0's chainMods: each step rounds up
// (ADR-0702 §2. `M = (M*mod + engine.Modifier4096/2) / engine.Modifier4096`, starting from the
// identity modifier engine.Modifier4096). mods must be in the order @smogon/calc pushes them
// (tailwind before item).
func chainSpeedModifiers(mods []int) int {
	m := engine.Modifier4096
	for _, mod := range mods {
		m = (m*mod + engine.Modifier4096/2) / engine.Modifier4096
	}
	return m
}

// applySpeedModifiers はランク適用後の実数値に、連結済みの素早さ補正を五捨五超入で 1 回だけ掛ける
// (floor((v × M + engine.Modifier4096/2 − 1) / engine.Modifier4096)。ADR-0701 §2・ADR-0702 §2。
// 丸めの向きが chainSpeedModifiers の 1 ステップと違うのは @smogon/calc の原典どおり)。
func applySpeedModifiers(v int, mods []int) int {
	m := chainSpeedModifiers(mods)
	return (v*m + engine.Modifier4096/2 - 1) / engine.Modifier4096
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

// CompareSpeed は attacker と defender の戦闘中の素早さを求めて比較する。outspeeds は
// トリックルームが無ければ厳密な >、あれば < に反転する(ADR-0702 §3)。speedTie は常に ==
// (ADR-0700 §6-1。同速を真偽値 1 つに丸めない。トリックルームでも反転しない)。どちらかが
// 範囲外ならその sentinel エラーを返す。
func CompareSpeed(attacker, defender Individual, field SpeedField) (SpeedComparison, error) {
	attackerSpeed, err := Speed(attacker)
	if err != nil {
		return SpeedComparison{}, err
	}
	defenderSpeed, err := Speed(defender)
	if err != nil {
		return SpeedComparison{}, err
	}
	outspeeds := attackerSpeed > defenderSpeed
	if field.TrickRoom {
		outspeeds = attackerSpeed < defenderSpeed
	}
	return SpeedComparison{
		AttackerSpeed: attackerSpeed,
		DefenderSpeed: defenderSpeed,
		Outspeeds:     outspeeds,
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
