package speed

// 「相手を選んで、上を取る最低ライン」と「自分の素早さの、これ以上 / これ以下」(I-speed-4。ADR-0610)。
//
// このファイルは spec-writer が置いた型と関数の形だけ(スタブ)。本体は implementer が書く。
// 素早さの値は Speed(= engine.FinalSpeed と同じ値。engine_parity_test.go)で求め、行動順は engine.CompareSpeed で決める。
// 新しい素早さの式をここに作らない(ADR-0179 §2・§3 の共有)。

import (
	"errors"

	"example.com/pokecalc/engine"
)

// errNotImplemented はスタブの印。implementer が本体を書いたら消す。
var errNotImplemented = errors.New("not implemented (I-speed-4 stub)")

// 入力検証の sentinel エラー(ADR-0610 §2)。
var (
	// ErrInvalidOpponentSpeed は相手の素早さ(比べる相手の値)が 1 未満であることを表す。
	ErrInvalidOpponentSpeed = errors.New("opponent speed must be at least 1")
	// ErrInvalidSpeedValue は自分の素早さの値が 1 未満であることを表す(Reach)。
	ErrInvalidSpeedValue = errors.New("speed value must be at least 1")
	// ErrInvalidAssumption は相手の想定が preset と custom の両方を持つ、またはどちらも持たないことを表す。
	ErrInvalidAssumption = errors.New("an opponent assumption needs exactly one of preset or custom")
)

// Condition は SP・性格以外の、その個体の素早さに乗る条件(ADR-0610 §1。ランク・こだわりスカーフ・その側の追い風・まひ)。
type Condition struct {
	Rank      int
	Scarf     bool
	Tailwind  bool
	Paralysis bool
}

// Investment は素早さの SP と性格の補正の組(「振り」)。
type Investment struct {
	SP     int
	Nature NatureEffect
}

// SPRange は SP の閉区間 [Min, Max](0 ≦ Min ≦ Max ≦ engine.MaxSPPerStat)。
type SPRange struct {
	Min int
	Max int
}

// NatureLine は性格の補正 1 通りについての「上を取る SP の範囲」と「同速になる SP の範囲」(ADR-0610 §1)。
// 素早さは SP について広義単調増加なので、どちらも区間になる。
//   - トリックルームなし: MovesFirst = {SP | 自分 > 相手} = [k, 32]。Min が「上を取る最低ライン」。
//   - トリックルーム中:   MovesFirst = {SP | 自分 < 相手} = [0, k]。Max が「下を取れる上限」。
//
// nil はその性格ではどの SP でも成り立たないこと。同速(Tie)は「上を取る」に含めない。
type NatureLine struct {
	Nature     NatureEffect
	MovesFirst *SPRange
	Tie        *SPRange
}

// LineInput は MovesFirstLines の入力: 自分の種族値と条件、比べる相手の素早さ(戦闘中の最終値)、トリックルーム。
type LineInput struct {
	SelfBaseSpeed int
	Self          Condition
	OpponentSpeed int
	TrickRoom     bool
}

// MovesFirstLines は性格 3 通り(minus, neutral, plus の順)について、SP 0〜engine.MaxSPPerStat のうち
// 行動順が self_first になる範囲と speed_tie になる範囲を返す(ADR-0610 §1)。入力が範囲外なら sentinel エラー。
func MovesFirstLines(in LineInput) ([]NatureLine, error) {
	_ = in
	_ = engine.MaxSPPerStat
	return nil, errNotImplemented
}

// OpponentAssumption は相手の振りの想定 1 つ。Preset(表の 6 行のどれか)か Custom(SP・性格・ランク・スカーフ)の
// どちらか一方だけを持つ。相手側の追い風・まひは MatchupRequest が全想定に掛ける。
type OpponentAssumption struct {
	Preset PresetID
	Custom *OpponentCustom
}

// OpponentCustom は相手の振りの自由入力。
type OpponentCustom struct {
	SP     int
	Nature NatureEffect
	Rank   int
	Scarf  bool
}

// DefaultOpponentAssumptions は想定を省略したときの相手の想定(無振り・準速・最速・最速スカーフ。ADR-0610 §1)。
// ランクの 2 行(max-plus1・max-plus2)は「相手が積んだ後」なので既定に含めない(指定すれば使える)。
func DefaultOpponentAssumptions() []OpponentAssumption {
	return nil
}

// MatchupRequest は「自分 × 相手」の入力(ADR-0610 §2)。
type MatchupRequest struct {
	SelfPokemonID string
	Self          Condition
	// SelfCurrent は今の自分の振り(任意)。あれば各想定に今の行動順(Current)を付ける。
	SelfCurrent *Investment

	OpponentPokemonID string
	// Assumptions は相手の想定(nil は DefaultOpponentAssumptions)。preset の重複は ErrDuplicatePreset。
	Assumptions       []OpponentAssumption
	OpponentTailwind  bool
	OpponentParalysis bool

	TrickRoom bool
}

// MatchupLine は相手の想定 1 つに対する結果。
type MatchupLine struct {
	Assumption    OpponentAssumption
	OpponentSpeed int
	Lines         []NatureLine
	// Current は SelfCurrent があるときの今の行動順(engine.CompareSpeed の値)。無ければ nil。
	Current *engine.TurnOrder
}

// MatchupResult は Matchup の結果。SelfSpeed は SelfCurrent があるときの今の自分の素早さ(無ければ 0)。
type MatchupResult struct {
	Self      Pokemon
	Opponent  Pokemon
	SelfSpeed int
	Lines     []MatchupLine
}

// Matchup は自分と相手を roster から引き、相手の想定ごとに MovesFirstLines を返す(ADR-0610 §2)。
// 未知の pokemonId は ErrUnknownPokemon。
func Matchup(roster Roster, req MatchupRequest) (MatchupResult, error) {
	_ = roster
	_ = req
	return MatchupResult{}, errNotImplemented
}

// BaseRange は素早さ種族値の閉区間 [Min, Max](1 ≦ Min ≦ Max ≦ 255)。
type BaseRange struct {
	Min int
	Max int
}

// ReachLine は表の行(プリセット)1 つについて、自分の素早さ v と比べた種族値の区切り(ADR-0610 §1「族」)。
// 値は素早さの比較(ADR-0607 §4 と同じ。トリックルームで入れ替えない)。
//   - Slower: そのプリセットの素早さが v より遅い種族値(= v で抜ける)。[1, k]
//   - Tie:    v と同速になる種族値
//   - Faster: v より速い種族値(= 抜かれる)。[k', 255]
//
// nil はその区分に入る種族値が無いこと。Count と例は roster(使用可能なポケモン)で数える。
type ReachLine struct {
	Preset PresetID
	Slower *BaseRange
	Tie    *BaseRange
	Faster *BaseRange

	SlowerCount int
	TieCount    int
	FasterCount int
	// FastestSlower は roster のうち v より遅い中で最も速いポケモン(種族値が同じなら pokemonId の昇順の先頭)。
	// SlowestFaster は v より速い中で最も遅いポケモン(同上)。無ければ nil。
	FastestSlower *Pokemon
	SlowestFaster *Pokemon
}

// Reach は自分の素早さ v について、表の 6 プリセット(ADR-0601 §2 の順)ごとの ReachLine を返す(ADR-0610 §1)。
// opponentTailwind は相手側(表の行)の追い風。v が 1 未満なら ErrInvalidSpeedValue。
func Reach(roster Roster, v int, opponentTailwind bool) ([]ReachLine, error) {
	_ = roster
	_ = v
	_ = opponentTailwind
	return nil, errNotImplemented
}
