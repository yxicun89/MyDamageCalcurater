package domain

// 「相手を選んで、上を取る最低ライン」と「自分の素早さの、これ以上 / これ以下」(I-speed-4。ADR-0610)。
//
// 素早さの値は Speed(= engine.FinalSpeed と同じ値。engine_parity_test.go)で求め、行動順は engine.CompareSpeed で決める。
// 新しい素早さの式をここに作らない(ADR-0179 §2・§3 の共有)。

import (
	"errors"
	"fmt"
	"sort"

	"example.com/pokecalc/engine"
)

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
	if in.OpponentSpeed < 1 {
		return nil, ErrInvalidOpponentSpeed
	}
	out := make([]NatureLine, 0, len(natureOrder))
	for _, nature := range natureOrder {
		line := NatureLine{Nature: nature}
		for sp := 0; sp <= engine.MaxSPPerStat; sp++ {
			v, err := Speed(Input{
				BaseSpeed: in.SelfBaseSpeed, SP: sp, Nature: nature, Rank: in.Self.Rank,
				Scarf: in.Self.Scarf, Tailwind: in.Self.Tailwind, Paralysis: in.Self.Paralysis,
			})
			if err != nil {
				return nil, err
			}
			// 素早さは SP について広義単調増加なので、該当する SP は連続する(区間を広げていけばよい)。
			switch engine.CompareSpeed(v, in.OpponentSpeed, in.TrickRoom) {
			case engine.TurnOrderSelfFirst:
				line.MovesFirst = extendSPRange(line.MovesFirst, sp)
			case engine.TurnOrderSpeedTie:
				line.Tie = extendSPRange(line.Tie, sp)
			}
		}
		out = append(out, line)
	}
	return out, nil
}

// natureOrder は結果に並べる性格の順(下降・なし・上昇)。
var natureOrder = []NatureEffect{NatureMinus, NatureNeutral, NaturePlus}

// extendSPRange は r に sp を加えた区間を返す(r が nil なら sp だけの区間)。sp は昇順に渡される。
func extendSPRange(r *SPRange, sp int) *SPRange {
	if r == nil {
		return &SPRange{Min: sp, Max: sp}
	}
	r.Max = sp
	return r
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
	return []OpponentAssumption{
		{Preset: PresetUninvested}, {Preset: PresetNeutralMax}, {Preset: PresetMax}, {Preset: PresetMaxScarf},
	}
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
	self, err := resolvePokemon(roster, req.SelfPokemonID)
	if err != nil {
		return MatchupResult{}, err
	}
	opponent, err := resolvePokemon(roster, req.OpponentPokemonID)
	if err != nil {
		return MatchupResult{}, err
	}
	assumptions := req.Assumptions
	if len(assumptions) == 0 {
		assumptions = DefaultOpponentAssumptions()
	}
	inputs, err := opponentInputs(opponent, assumptions)
	if err != nil {
		return MatchupResult{}, err
	}

	result := MatchupResult{Self: self, Opponent: opponent, Lines: make([]MatchupLine, 0, len(inputs))}
	if req.SelfCurrent != nil {
		result.SelfSpeed, err = Speed(Input{
			BaseSpeed: self.BaseSpeed, SP: req.SelfCurrent.SP, Nature: req.SelfCurrent.Nature, Rank: req.Self.Rank,
			Scarf: req.Self.Scarf, Tailwind: req.Self.Tailwind, Paralysis: req.Self.Paralysis,
		})
		if err != nil {
			return MatchupResult{}, err
		}
	}
	for i, in := range inputs {
		in.Tailwind = req.OpponentTailwind
		in.Paralysis = req.OpponentParalysis
		opponentSpeed, err := Speed(in)
		if err != nil {
			return MatchupResult{}, err
		}
		lines, err := MovesFirstLines(LineInput{
			SelfBaseSpeed: self.BaseSpeed, Self: req.Self, OpponentSpeed: opponentSpeed, TrickRoom: req.TrickRoom,
		})
		if err != nil {
			return MatchupResult{}, err
		}
		line := MatchupLine{Assumption: assumptions[i], OpponentSpeed: opponentSpeed, Lines: lines}
		if req.SelfCurrent != nil {
			order := engine.CompareSpeed(result.SelfSpeed, opponentSpeed, req.TrickRoom)
			line.Current = &order
		}
		result.Lines = append(result.Lines, line)
	}
	return result, nil
}

// opponentInputs は相手の想定ごとの素早さの入力(相手側の追い風・まひを除く)を作る。
// preset と custom の片方だけを持たない想定は ErrInvalidAssumption、未知の preset は ErrUnknownPreset、
// preset の重複は ErrDuplicatePreset。
func opponentInputs(opponent Pokemon, assumptions []OpponentAssumption) ([]Input, error) {
	seen := make(map[PresetID]bool, len(assumptions))
	inputs := make([]Input, 0, len(assumptions))
	for _, a := range assumptions {
		hasPreset := a.Preset != ""
		if hasPreset == (a.Custom != nil) {
			return nil, ErrInvalidAssumption
		}
		if a.Custom != nil {
			c := a.Custom
			inputs = append(inputs, Input{BaseSpeed: opponent.BaseSpeed, SP: c.SP, Nature: c.Nature, Rank: c.Rank, Scarf: c.Scarf})
			continue
		}
		def, ok := presetByID(a.Preset)
		if !ok {
			return nil, fmt.Errorf("%q: %w", a.Preset, ErrUnknownPreset)
		}
		if seen[a.Preset] {
			return nil, fmt.Errorf("%q: %w", a.Preset, ErrDuplicatePreset)
		}
		seen[a.Preset] = true
		inputs = append(inputs, Input{BaseSpeed: opponent.BaseSpeed, SP: def.SP, Nature: def.Nature, Rank: def.Rank, Scarf: def.Scarf})
	}
	return inputs, nil
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
	if v < 1 {
		return nil, ErrInvalidSpeedValue
	}
	defs := Presets()
	out := make([]ReachLine, 0, len(defs))
	for _, def := range defs {
		// 種族値 1〜255 ごとの比較。素早さは種族値について広義単調増加なので、各区分は連続する。
		cmp := make([]int, maxBaseSpeed+1) // 添字 = 種族値。-1 = 遅い・0 = 同速・1 = 速い
		line := ReachLine{Preset: def.ID}
		for b := minBaseSpeed; b <= maxBaseSpeed; b++ {
			s, err := Speed(Input{BaseSpeed: b, SP: def.SP, Nature: def.Nature, Rank: def.Rank, Scarf: def.Scarf, Tailwind: opponentTailwind})
			if err != nil {
				return nil, err
			}
			switch {
			case s < v:
				cmp[b] = -1
				line.Slower = extendBaseRange(line.Slower, b)
			case s == v:
				cmp[b] = 0
				line.Tie = extendBaseRange(line.Tie, b)
			default:
				cmp[b] = 1
				line.Faster = extendBaseRange(line.Faster, b)
			}
		}
		fillReachExamples(&line, roster, cmp)
		out = append(out, line)
	}
	return out, nil
}

// extendBaseRange は r に base を加えた区間を返す(r が nil なら base だけの区間)。base は昇順に渡される。
func extendBaseRange(r *BaseRange, base int) *BaseRange {
	if r == nil {
		return &BaseRange{Min: base, Max: base}
	}
	r.Max = base
	return r
}

// fillReachExamples は roster を数え、遅い側で最も速い・速い側で最も遅いポケモンを line に入れる。
// 種族値が同じなら pokemonId の昇順の先頭。cmp は種族値ごとの比較(-1 遅い・0 同速・1 速い)。
func fillReachExamples(line *ReachLine, roster Roster, cmp []int) {
	pokemon := make([]Pokemon, len(roster.Pokemon))
	copy(pokemon, roster.Pokemon)
	sort.Slice(pokemon, func(i, j int) bool { return pokemon[i].PokemonID < pokemon[j].PokemonID })
	for _, p := range pokemon {
		if p.BaseSpeed < minBaseSpeed || p.BaseSpeed > maxBaseSpeed {
			continue
		}
		p := p
		switch cmp[p.BaseSpeed] {
		case -1:
			line.SlowerCount++
			if line.FastestSlower == nil || p.BaseSpeed > line.FastestSlower.BaseSpeed {
				line.FastestSlower = &p
			}
		case 0:
			line.TieCount++
		default:
			line.FasterCount++
			if line.SlowestFaster == nil || p.BaseSpeed < line.SlowestFaster.BaseSpeed {
				line.SlowestFaster = &p
			}
		}
	}
}
