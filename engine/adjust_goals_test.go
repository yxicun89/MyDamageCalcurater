package engine

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// 複数の目標をすべて満たす最小の振り方(ADR-0177。F-11 段階 B)の受け入れ条件。
//
// 正解は engine の実装からではなく、このファイルの総当たり(goalsOracle)から独立に導く。
// 候補(探索する能力が各 [下限, 上限]・それ以外は下限・合計 ≤ 66)を全部列挙し、各候補で目標を CalcDamage と
// EffectiveStat から直接判定して、ADR-0177 §5 の順を整数のキーの辞書式比較で選ぶ。確率は adjust_search_test.go の
// oracleKOCount(16^n 通りの数え上げ。n ≤ 3)で求める。実装の分解(§6)とは独立。
//
// 種族は adjust_search_test.go の架空種族を使う(coding-rules §1)。
//   - 自分: adjSelfAttackerSpecies(H90/A110/B80/C110/D80/S90。ノーマル)→ 無補正で H = 165 + h、A = 130 + a、
//     B = 100 + b、C = 130 + c、D = 100 + d、S = 110 + s
//   - 相手: adjFoeSpecies(H95/B90/D90/S70。みず)と adjSelfAttackerSpecies

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

var natureGoalsSpeUp = Nature{Plus: StatSpe, Minus: StatAtk} // S↑A↓

// goalsSelf は自分の個体(下限 floor・性格 nature)。
func goalsSelf(nature Nature, floor Stats) Individual {
	return Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: nature, SP: floor, Status: StatusNone}
}

// goalsFoe は相手の個体(adjFoeSpecies)。
func goalsFoe(nature Nature, sp Stats) Individual {
	return Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: nature, SP: sp, Status: StatusNone}
}

// goalsAttackerFoe は相手の攻撃側の個体(adjSelfAttackerSpecies。A・C 32)。
func goalsAttackerFoe() Individual {
	return Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{Atk: 32, SpA: 32}, Status: StatusNone}
}

// outspeedGoal は素早さの目標(相手 = adjFoeSpecies。S の SP と性格・ランクを指定)。
func outspeedGoal(nature Nature, speSP, rank, stage int) SPGoal {
	foe := goalsFoe(nature, Stats{Spe: speSP})
	foe.Ranks.Spe = rank
	return SPGoal{Kind: SPGoalOutspeed, Opponent: foe, SelfSpeedStage: stage}
}

// surviveGoal は耐える目標(相手 = A・C 32 の攻撃側)。
func surviveGoal(cat MoveCategory, power, hits int, threshold float64) SPGoal {
	return SPGoal{Kind: SPGoalSurvive, Opponent: goalsAttackerFoe(), Move: adjMove(cat, power), Hits: hits, ThresholdPercent: threshold}
}

// koGoal は倒す目標(相手 = H32 の adjFoeSpecies)。
func koGoal(cat MoveCategory, power, hits int, threshold float64) SPGoal {
	return SPGoal{Kind: SPGoalKO, Opponent: goalsFoe(NatureNeutral, Stats{HP: 32}), Move: adjMove(cat, power), Hits: hits, ThresholdPercent: threshold}
}

// goalsInput は入力を作る(相性表つき・場なし)。
func goalsInput(t *testing.T, self Individual, ceiling Stats, goals ...SPGoal) SPGoalsInput {
	t.Helper()
	return SPGoalsInput{
		Format: FormatSingle, Self: self, Ceiling: ceiling,
		Field: Field{Weather: WeatherNone, Terrain: TerrainNone}, TypeChart: mustTypeChart(t), Goals: goals,
	}
}

// statsAll は全能力を v にした Stats。
func statsAll(v int) Stats { return Stats{HP: v, Atk: v, Def: v, SpA: v, SpD: v, Spe: v} }

// ---------------------------------------------------------------------------
// 独立の総当たり(oracle)
// ---------------------------------------------------------------------------

// goalsOracleKeys は探索する能力(ADR-0177 §5。H, A, B, C, D, S の順)。
func goalsOracleKeys(goals []SPGoal) []StatKey {
	use := map[StatKey]bool{}
	for _, g := range goals {
		switch {
		case g.Kind == SPGoalOutspeed:
			use[StatSpe] = true
		case g.Kind == SPGoalKO && g.Move.Category == CategoryPhysical:
			use[StatAtk] = true
		case g.Kind == SPGoalKO:
			use[StatSpA] = true
		case g.Move.Category == CategoryPhysical:
			use[StatHP], use[StatDef] = true, true
		default:
			use[StatHP], use[StatSpD] = true, true
		}
	}
	var keys []StatKey
	for _, k := range AllStatKeys() {
		if use[k] {
			keys = append(keys, k)
		}
	}
	return keys
}

// goalsOracleCountCache は oracleKOCount の結果の使い回し(数え上げの重さを避けるだけ。値は同じ)。
type goalsOracleCountCache map[string][2]int

func (c goalsOracleCountCache) count(t *testing.T, res DamageResult, hits int) (ko, total int) {
	t.Helper()
	key := fmt.Sprint(res.Rolls, res.DefenderHP, hits)
	if v, ok := c[key]; ok {
		return v[0], v[1]
	}
	ko, total = oracleKOCount(t, res.Rolls, res.DefenderHP, hits)
	c[key] = [2]int{ko, total}
	return ko, total
}

// goalsOracleCand は候補1つの評価。
type goalsOracleCand struct {
	sp       Stats
	outcomes []SPGoalOutcome
	met      int
	progress []int // 満たした目標は goalsOracleSaturated
	bulk     []int
}

const goalsOracleSaturated = 1 << 62

func goalsOracleEval(t *testing.T, in SPGoalsInput, sp Stats, cache goalsOracleCountCache) goalsOracleCand {
	t.Helper()
	self := in.Self
	self.SP = sp
	c := goalsOracleCand{sp: sp}
	hasPhys, hasSpec := false, false
	for _, g := range in.Goals {
		var o SPGoalOutcome
		var progress int
		o.Kind = g.Kind
		switch g.Kind {
		case SPGoalOutspeed:
			rank := min(6, max(-6, in.Self.Ranks.Spe+g.SelfSpeedStage))
			boosted := self
			boosted.Ranks.Spe = rank
			o.SelfSpeedRank = rank
			o.SelfSpeed = EffectiveStat(boosted, StatSpe)
			o.OpponentSpeed = EffectiveStat(g.Opponent, StatSpe)
			o.Met = o.SelfSpeed > o.OpponentSpeed
			progress = o.SelfSpeed
		default:
			attacker, defender := g.Opponent, self
			if g.Kind == SPGoalKO {
				attacker, defender = self, g.Opponent
			} else if g.Move.Category == CategoryPhysical {
				hasPhys = true
			} else {
				hasSpec = true
			}
			res, err := CalcDamage(DamageInput{
				Format: in.Format, Attacker: attacker, Defender: defender, Move: g.Move,
				Field: in.Field, TypeChart: in.TypeChart,
			})
			if err != nil {
				t.Fatalf("oracle の CalcDamage: %v", err)
			}
			ko, total := cache.count(t, res, g.Hits)
			count := ko
			if g.Kind == SPGoalSurvive {
				count = total - ko
			}
			o.ChancePercent = oraclePercent(count, total)
			o.Met = o.ChancePercent >= oracleThreshold(g.ThresholdPercent)
			progress = count
		}
		if o.Met {
			c.met++
			progress = goalsOracleSaturated
		}
		c.outcomes = append(c.outcomes, o)
		c.progress = append(c.progress, progress)
	}
	real := RealStats(self)
	phys, spec := real.HP*real.Def, real.HP*real.SpD
	switch {
	case hasPhys && hasSpec:
		c.bulk = []int{min(phys, spec), max(phys, spec)}
	case hasPhys:
		c.bulk = []int{phys}
	case hasSpec:
		c.bulk = []int{spec}
	}
	return c
}

// key は ADR-0177 §5 の順のキー(大きい方が前)。辞書順(H, A, B, C, D, S が小さい方)まで含む。
func (c goalsOracleCand) key() []int {
	k := []int{c.met}
	k = append(k, c.progress...)
	k = append(k, -c.sp.Sum())
	k = append(k, c.bulk...)
	for _, s := range AllStatKeys() {
		k = append(k, -c.sp.Get(s))
	}
	return k
}

// goalsOracle は SuggestSPForGoals の正解を総当たりで求める。
func goalsOracle(t *testing.T, in SPGoalsInput) SPGoalsResult {
	t.Helper()
	keys := goalsOracleKeys(in.Goals)
	cache := goalsOracleCountCache{}
	var best *goalsOracleCand
	var walk func(i int, sp Stats)
	walk = func(i int, sp Stats) {
		if i == len(keys) {
			if sp.Sum() > MaxSPTotal {
				return
			}
			c := goalsOracleEval(t, in, sp, cache)
			if best == nil || allocKeyGreater(c.key(), best.key()) {
				best = &c
			}
			return
		}
		k := keys[i]
		for v := in.Self.SP.Get(k); v <= in.Ceiling.Get(k); v++ {
			walk(i+1, sp.WithStat(k, v))
		}
	}
	walk(0, in.Self.SP)
	if best == nil {
		t.Fatal("oracle: 候補が無い(fixture の誤り)")
	}
	self := in.Self
	self.SP = best.sp
	return SPGoalsResult{
		Feasible:    best.met == len(in.Goals),
		Remaining:   MaxSPTotal - best.sp.Sum(),
		Plan:        SPGoalsPlan{SP: best.sp, TotalSP: best.sp.Sum(), Real: RealStats(self)},
		Goals:       best.outcomes,
		Unsupported: goalsOracleMarks(t, in),
	}
}

// goalsOracleMarks は survive・ko の下限での印を目標の順に連結し、同じ印を除く(ADR-0177 §7)。
func goalsOracleMarks(t *testing.T, in SPGoalsInput) []UnsupportedMark {
	t.Helper()
	var out []UnsupportedMark
	for _, g := range in.Goals {
		if g.Kind == SPGoalOutspeed {
			continue
		}
		attacker, defender := g.Opponent, in.Self
		if g.Kind == SPGoalKO {
			attacker, defender = in.Self, g.Opponent
		}
		res, err := CalcDamage(DamageInput{Format: in.Format, Attacker: attacker, Defender: defender, Move: g.Move, Field: in.Field, TypeChart: in.TypeChart})
		if err != nil {
			t.Fatalf("oracle の CalcDamage: %v", err)
		}
		for _, m := range res.Unsupported {
			if !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 比較
// ---------------------------------------------------------------------------

func mustSuggestGoals(t *testing.T, in SPGoalsInput) SPGoalsResult {
	t.Helper()
	got, err := SuggestSPForGoals(in)
	if err != nil {
		t.Fatalf("SuggestSPForGoals: %v", err)
	}
	return got
}

func assertGoalsResult(t *testing.T, got, want SPGoalsResult) {
	t.Helper()
	if got.Feasible != want.Feasible || got.Remaining != want.Remaining || got.Plan != want.Plan {
		t.Errorf("feasible/remaining/plan = %v/%d/%+v\n want %v/%d/%+v", got.Feasible, got.Remaining, got.Plan, want.Feasible, want.Remaining, want.Plan)
	}
	if len(got.Goals) != len(want.Goals) {
		t.Fatalf("goals の件数 = %d, want %d", len(got.Goals), len(want.Goals))
	}
	for i := range want.Goals {
		g, w := got.Goals[i], want.Goals[i]
		if g.Kind != w.Kind || g.Met != w.Met || !chanceEqual(g.ChancePercent, w.ChancePercent) ||
			g.SelfSpeed != w.SelfSpeed || g.OpponentSpeed != w.OpponentSpeed || g.SelfSpeedRank != w.SelfSpeedRank {
			t.Errorf("goals[%d] = %+v, want %+v", i, g, w)
		}
	}
	if !reflect.DeepEqual(got.Unsupported, want.Unsupported) {
		t.Errorf("unsupported = %+v, want %+v", got.Unsupported, want.Unsupported)
	}
}

// ---------------------------------------------------------------------------
// B1: 独立の総当たりと一致する
// ---------------------------------------------------------------------------

type goalsCase struct {
	name string
	in   func(t *testing.T) SPGoalsInput
	// feasible は fixture の前提(oracle で確かめる)。nil は問わない。
	feasible *bool
}

func boolPtr(v bool) *bool { return &v }

func goalsCases() []goalsCase {
	full := statsAll(MaxSPPerStat)
	return []goalsCase{
		{"素早さだけ(最速の相手)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
		}, boolPtr(true)},
		{"素早さだけ(S↑性格の自分)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(natureGoalsSpeUp, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
		}, boolPtr(true)},
		{"素早さだけ(相手 +2 で届かない)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 2, 0))
		}, boolPtr(false)},
		{"素早さだけ(先に使う技で +1)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 1, 1))
		}, nil},
		{"素早さ2件(遅い相手と速い相手)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full,
				outspeedGoal(NatureNeutral, 0, 0, 0), outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
		}, boolPtr(true)},
		{"耐える(物理)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, surviveGoal(CategoryPhysical, 200, 1, 0))
		}, nil},
		{"耐える(物理・50%)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, surviveGoal(CategoryPhysical, 200, 1, 50))
		}, nil},
		{"耐える(届かない)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, surviveGoal(CategoryPhysical, 250, 2, 0))
		}, boolPtr(false)},
		{"耐える2件(物理と特殊で H を共有)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 4, Def: 4, SpD: 4}), Stats{HP: 22, Def: 20, SpD: 20},
				surviveGoal(CategoryPhysical, 200, 1, 0), surviveGoal(CategorySpecial, 200, 1, 0))
		}, nil},
		{"倒す(物理・2発)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, koGoal(CategoryPhysical, 130, 2, 0))
		}, boolPtr(true)},
		{"素早さ + 倒す(ニトロチャージの判定レーンの要望)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full,
				outspeedGoal(natureGoalsSpeUp, 32, 0, 1), koGoal(CategoryPhysical, 130, 2, 0))
		}, boolPtr(true)},
		{"素早さ + 倒す + 耐える(範囲を絞る)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{Spe: 20}), Stats{HP: 12, Atk: 12, SpD: 12, Spe: 30},
				outspeedGoal(natureGoalsSpeUp, 32, 0, 0), koGoal(CategoryPhysical, 130, 2, 0), surviveGoal(CategorySpecial, 200, 1, 0))
		}, nil},
		{"予算が足りない(素早さが先)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{SpA: 32}), Stats{HP: 32, Def: 32, Spe: 32},
				outspeedGoal(natureGoalsSpeUp, 32, 0, 0), surviveGoal(CategoryPhysical, 200, 1, 0))
		}, boolPtr(false)},
		{"予算が足りない(耐えるが先)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{SpA: 32}), Stats{HP: 32, Def: 32, Spe: 32},
				surviveGoal(CategoryPhysical, 200, 1, 0), outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
		}, boolPtr(false)},
		{"6件(全能力・範囲を絞る)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 8, Atk: 8, Def: 8, SpA: 8, SpD: 8, Spe: 18}),
				Stats{HP: 12, Atk: 12, Def: 12, SpA: 12, SpD: 12, Spe: 22},
				outspeedGoal(NatureNeutral, 32, 0, 0), outspeedGoal(natureGoalsSpeUp, 32, 0, 1),
				koGoal(CategoryPhysical, 130, 2, 50), koGoal(CategorySpecial, 130, 2, 50),
				surviveGoal(CategoryPhysical, 160, 1, 0), surviveGoal(CategorySpecial, 160, 1, 0))
		}, nil},
		{"下限が目標を超える", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{Spe: 32}), full, outspeedGoal(NatureNeutral, 0, 0, 0))
		}, boolPtr(true)},
		{"下限の合計ちょうど 66(候補は1つ)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 32, Def: 32, Spe: 2}), full,
				outspeedGoal(natureGoalsSpeUp, 32, 0, 0), surviveGoal(CategoryPhysical, 200, 1, 0))
		}, nil},
		{"上限 0(下限より上に振らない)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), Stats{}, outspeedGoal(NatureNeutral, 30, 0, 0))
		}, boolPtr(false)},
		// critic I-1(a): 最上位のキー「満たす目標の数」。予算 34 では耐えるは届かず素早さ(S 25)は届く。目標の順(耐えるが先)より
		// 満たす数が優先なので S 25 になる(満たす数を比べないと、前の目標の近さで H と B に振ってしまう)。
		{"満たす数が最上位(耐えるが先でも届かないなら素早さを満たす)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{SpA: 32}), Stats{HP: 32, Def: 32, Spe: 32},
				surviveGoal(CategoryPhysical, 220, 1, 0), outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
		}, boolPtr(false)},
		// 同じく耐久側の中(表の側のキー): 前の「耐える」(物理・50%)は予算 32 では届かず、後ろの「耐える」(特殊)は H に振れば届く。
		// 満たす数が先なので後ろを満たす組(H11 B21)になる(満たす数を比べないと、前の目標の近さで H5 B27 になる)。
		{"満たす数が最上位(耐久側の2件)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{Atk: 32, Spe: 2}), statsAll(MaxSPPerStat),
				surviveGoal(CategoryPhysical, 220, 1, 50), surviveGoal(CategorySpecial, 160, 1, 0))
		}, boolPtr(false)},
		// critic C-1: 攻撃側・素早さの表で、下限の合計に上限までの値を足すと 66 を超える組を CalcDamage に渡さない(500 の回帰)。
		{"下限 H32・B32 で倒す(上限は既定の 32)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 32, Def: 32}), full, koGoal(CategoryPhysical, 130, 2, 0))
		}, nil},
		{"下限の合計ちょうど 66 で倒す", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 32, Def: 32, Atk: 2}), full, koGoal(CategoryPhysical, 130, 2, 0))
		}, nil},
		{"下限の合計ちょうど 66 で素早さ + 倒す(特殊)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 32, SpD: 32, Spe: 2}), full,
				outspeedGoal(natureGoalsSpeUp, 32, 0, 0), koGoal(CategorySpecial, 130, 2, 0))
		}, boolPtr(false)},
		{"下限 A32・C32 で耐える(耐久側の予算は 2)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{Atk: 32, SpA: 32}), full, surviveGoal(CategoryPhysical, 200, 1, 0))
		}, nil},
	}
}

func TestSPGoalsFixturePremises(t *testing.T) {
	for _, tt := range goalsCases() {
		if tt.feasible == nil {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			if got := goalsOracle(t, tt.in(t)).Feasible; got != *tt.feasible {
				t.Errorf("oracle の feasible = %v, fixture の前提 %v(fixture の誤り)", got, *tt.feasible)
			}
		})
	}
}

func TestSPGoalsMatchOracle(t *testing.T) {
	for _, tt := range goalsCases() {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			assertGoalsResult(t, mustSuggestGoals(t, in), goalsOracle(t, in))
		})
	}
}

// 性格・下限・上限・しきい値の表で、素早さ + 倒す + 耐える の組み合わせを総当たりと照合する(範囲を絞って oracle を回せる大きさにする)。
func TestSPGoalsMatchOracleGrid(t *testing.T) {
	natures := []Nature{NatureNeutral, natureGoalsSpeUp, natureAllocDefDown, natureAdjSpAUp}
	thresholds := []float64{0, 50}
	for _, n := range natures {
		for _, th := range thresholds {
			for _, floorS := range []int{16, 24} {
				name := fmt.Sprintf("性格%+v/しきい値%v/S下限%d", n, th, floorS)
				t.Run(name, func(t *testing.T) {
					in := goalsInput(t, goalsSelf(n, Stats{Spe: floorS, HP: 6, Def: 6}), Stats{HP: 14, Def: 14, SpA: 12, Spe: 28},
						outspeedGoal(natureGoalsSpeUp, 32, 0, 0), koGoal(CategorySpecial, 130, 2, th), surviveGoal(CategoryPhysical, 200, 1, th))
					assertGoalsResult(t, mustSuggestGoals(t, in), goalsOracle(t, in))
				})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// B2: 単一の目標は既存の探索と一致する
// ---------------------------------------------------------------------------

func TestSPGoalsSingleGoalMatchesExistingSearch(t *testing.T) {
	t.Run("survive = MinSPToSurvive", func(t *testing.T) {
		for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
			for _, power := range []int{120, 200, 250} {
				for _, th := range []float64{0, 50} {
					in := goalsInput(t, goalsSelf(NatureNeutral, Stats{Atk: 10}), statsAll(MaxSPPerStat), surviveGoal(cat, power, 1, th))
					got := mustSuggestGoals(t, in)
					want, err := MinSPToSurvive(AdjustSearchInput{
						Format: FormatSingle, Attacker: in.Goals[0].Opponent, Defender: in.Self, Move: in.Goals[0].Move,
						Field: in.Field, TypeChart: in.TypeChart, Hits: 1, ThresholdPercent: th,
					})
					if err != nil {
						t.Fatalf("MinSPToSurvive: %v", err)
					}
					stat := reverseStat(SideDefender, cat)
					if got.Feasible != want.Feasible || got.Plan.SP.HP != want.HPSP || got.Plan.SP.Get(stat) != want.StatSP ||
						!chanceEqual(got.Goals[0].ChancePercent, want.ChancePercent) {
						t.Errorf("%s/%d/%v: goals=(%v, H%d, %s%d, %v) survive=(%v, H%d, %d, %v)", cat, power, th,
							got.Feasible, got.Plan.SP.HP, stat, got.Plan.SP.Get(stat), got.Goals[0].ChancePercent,
							want.Feasible, want.HPSP, want.StatSP, want.ChancePercent)
					}
				}
			}
		}
	})
	t.Run("ko(満たせる)= MinSPToKO", func(t *testing.T) {
		for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
			for _, power := range []int{130, 150, 200} {
				in := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), koGoal(cat, power, 2, 0))
				want, err := MinSPToKO(AdjustSearchInput{
					Format: FormatSingle, Attacker: in.Self, Defender: in.Goals[0].Opponent, Move: in.Goals[0].Move,
					Field: in.Field, TypeChart: in.TypeChart, Hits: 2,
				})
				if err != nil {
					t.Fatalf("MinSPToKO: %v", err)
				}
				if !want.Feasible {
					continue
				}
				got := mustSuggestGoals(t, in)
				if !got.Feasible || got.Plan.SP.Get(want.Stat) != want.SP || got.Plan.TotalSP != want.SP {
					t.Errorf("%s/%d: goals=(%v, %+v) ko=(%d)", cat, power, got.Feasible, got.Plan.SP, want.SP)
				}
			}
		}
	})
	t.Run("outspeed + ko(満たせる)= SuggestSPAllocation の MinSP", func(t *testing.T) {
		for _, speSP := range []int{0, 20, 32} {
			og := outspeedGoal(natureGoalsSpeUp, speSP, 0, 0)
			kg := koGoal(CategoryPhysical, 130, 2, 0)
			in := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), og, kg)
			alloc, err := SuggestSPAllocation(SPAllocInput{
				Self: in.Self, Ceiling: in.Ceiling, Mode: AllocModeOffense, OffenseCategory: CategoryPhysical,
				MinSpeed: EffectiveStat(og.Opponent, StatSpe) + 1,
				Goal:     &AllocGoal{Format: FormatSingle, Opponent: kg.Opponent, Move: kg.Move, Field: in.Field, TypeChart: in.TypeChart, Hits: 2},
			})
			if err != nil {
				t.Fatalf("SuggestSPAllocation: %v", err)
			}
			if !alloc.MinSP.GoalMet || !alloc.MinSP.SpeedMet {
				t.Fatalf("前提が崩れた: 配分で満たせない(S%d)", speSP)
			}
			got := mustSuggestGoals(t, in)
			if !got.Feasible || got.Plan.SP != alloc.MinSP.SP {
				t.Errorf("S%d: goals=(%v, %+v) alloc=%+v", speSP, got.Feasible, got.Plan.SP, alloc.MinSP.SP)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// B3: 境界
// ---------------------------------------------------------------------------

func TestSPGoalsBoundaries(t *testing.T) {
	full := statsAll(MaxSPPerStat)
	tests := []struct {
		name      string
		in        func(t *testing.T) SPGoalsInput
		wantSP    Stats
		feasible  bool
		check     func(t *testing.T, got SPGoalsResult)
		wantSpeed [3]int // outspeed の目標1件目の (SelfSpeed, OpponentSpeed, SelfSpeedRank)。0,0,0 は見ない
	}{
		{
			// 相手 = 無補正 S30 → 90 + 30 = 120。自分 110 + s > 120 → s = 11(s = 10 は同速で満たさない)。
			name: "同速は満たさない(S 11)",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(NatureNeutral, 30, 0, 0))
			},
			wantSP: Stats{Spe: 11}, feasible: true, wantSpeed: [3]int{121, 120, 0},
		},
		{
			// 相手 = S↑ S32 → floor(122 × 1.1) = 134。自分 110 + s > 134 → s = 25。
			name: "最速の相手を抜く(S 25)",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
			},
			wantSP: Stats{Spe: 25}, feasible: true, wantSpeed: [3]int{135, 134, 0},
		},
		{
			// 先に使う技で +1: floor((110 + s) × 3 / 2) > 134 → s = 0(165)。
			name: "ニトロチャージで +1(S 0)",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 0, 1))
			},
			wantSP: Stats{}, feasible: true, wantSpeed: [3]int{165, 134, 1},
		},
		{
			// 相手 +1: floor(134 × 3 / 2) = 201。自分 +1: floor((110 + s) × 3 / 2) > 201 → 110 + s ≥ 135 → s = 25。
			name: "両方 +1(相手のランクと先に使う技)",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 1, 1))
			},
			wantSP: Stats{Spe: 25}, feasible: true, wantSpeed: [3]int{202, 201, 1},
		},
		{
			// 自分のランク 6 + 先に使う技 +1 → 6 で頭打ち(×4)。相手 +6 は 134 × 4 = 536。(110 + s) × 4 > 536 → s = 25(540)。
			name: "ランクは 6 で頭打ち",
			in: func(t *testing.T) SPGoalsInput {
				self := goalsSelf(NatureNeutral, Stats{})
				self.Ranks.Spe = 6
				return goalsInput(t, self, full, outspeedGoal(natureGoalsSpeUp, 32, 6, 1))
			},
			wantSP: Stats{Spe: 25}, feasible: true, wantSpeed: [3]int{540, 536, 6},
		},
		{
			// 相手 +2: 134 × 2 = 268。自分の最大 142 では届かない → 最も近い = S 32(142)。
			name: "届かない素早さは最大まで寄せる",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 2, 0))
			},
			wantSP: Stats{Spe: 32}, feasible: false, wantSpeed: [3]int{142, 268, 0},
		},
		{
			// 自分 S↑: floor((110 + s) × 1.1) > 134 → 110 + s ≥ 123 → s = 13。下降補正の A は探索しない。
			name: "性格の上昇補正で S が減る",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(natureGoalsSpeUp, Stats{}), full, outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
			},
			wantSP: Stats{Spe: 13}, feasible: true, wantSpeed: [3]int{135, 134, 0},
		},
		{
			// 相手の種族値 S を 89 にした無補正 S32 = 89 + 20 + 32 = 141。自分 110 + s > 141 → s = 32(142)。
			name: "SP 32 でちょうど届く",
			in: func(t *testing.T) SPGoalsInput {
				foe := outspeedGoal(NatureNeutral, 32, 0, 0) // 122
				foe.Opponent.Species.BaseStats.Spe = 89      // 89 + 20 + 32 = 141
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, foe)
			},
			wantSP: Stats{Spe: 32}, feasible: true, wantSpeed: [3]int{142, 141, 0},
		},
		{
			name: "下限が目標を超えるなら下限の組",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{Spe: 32, Atk: 5}), full, outspeedGoal(NatureNeutral, 0, 0, 0))
			},
			wantSP: Stats{Spe: 32, Atk: 5}, feasible: true, wantSpeed: [3]int{142, 90, 0},
		},
		{
			name: "下限の合計ちょうど 66 はエラーにしない",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 32, Atk: 32, Spe: 2}), full, outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
			},
			wantSP: Stats{HP: 32, Atk: 32, Spe: 2}, feasible: false, wantSpeed: [3]int{112, 134, 0},
		},
		{
			name: "上限 0 の能力は下限のまま",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), Stats{HP: 32, Def: 32}, outspeedGoal(NatureNeutral, 0, 0, 0),
					surviveGoal(CategoryPhysical, 200, 1, 0))
			},
			feasible: true,
			check: func(t *testing.T, got SPGoalsResult) {
				if got.Plan.SP.Spe != 0 {
					t.Errorf("上限 0 の S = %d, want 0", got.Plan.SP.Spe)
				}
			},
		},
		{
			name: "探索しない能力は下限のまま・remaining = 66 − 合計",
			in: func(t *testing.T) SPGoalsInput {
				return goalsInput(t, goalsSelf(NatureNeutral, Stats{SpA: 7, SpD: 3}), full, koGoal(CategoryPhysical, 130, 2, 0))
			},
			feasible: true,
			check: func(t *testing.T, got SPGoalsResult) {
				if got.Plan.SP.SpA != 7 || got.Plan.SP.SpD != 3 || got.Plan.SP.HP != 0 || got.Plan.SP.Def != 0 || got.Plan.SP.Spe != 0 {
					t.Errorf("探索しない能力が動いた: %+v", got.Plan.SP)
				}
				if got.Remaining != MaxSPTotal-got.Plan.TotalSP || got.Plan.TotalSP != got.Plan.SP.Sum() {
					t.Errorf("remaining=%d totalSp=%d sum=%d", got.Remaining, got.Plan.TotalSP, got.Plan.SP.Sum())
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			got := mustSuggestGoals(t, in)
			assertGoalsResult(t, got, goalsOracle(t, in))
			if got.Feasible != tt.feasible {
				t.Errorf("feasible = %v, want %v", got.Feasible, tt.feasible)
			}
			if tt.wantSP != (Stats{}) || tt.check == nil {
				if got.Plan.SP != tt.wantSP {
					t.Errorf("SP = %+v, want %+v", got.Plan.SP, tt.wantSP)
				}
			}
			if tt.wantSpeed != [3]int{} {
				o := got.Goals[0]
				if [3]int{o.SelfSpeed, o.OpponentSpeed, o.SelfSpeedRank} != tt.wantSpeed {
					t.Errorf("(selfSpeed, opponentSpeed, rank) = (%d, %d, %d), want %v", o.SelfSpeed, o.OpponentSpeed, o.SelfSpeedRank, tt.wantSpeed)
				}
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// 2つの「耐える」(物理と特殊)は H を共有するので、別々の最小を足した合計より少なくて済む(ADR-0331 背景の表)。
func TestSPGoalsSharedHPForTwoSurviveGoals(t *testing.T) {
	phys, spec := surviveGoal(CategoryPhysical, 200, 1, 0), surviveGoal(CategorySpecial, 200, 1, 0)
	full := statsAll(MaxSPPerStat)
	both := mustSuggestGoals(t, goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, phys, spec))
	onlyPhys := mustSuggestGoals(t, goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, phys))
	onlySpec := mustSuggestGoals(t, goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, spec))
	if !both.Feasible || !onlyPhys.Feasible || !onlySpec.Feasible {
		t.Fatalf("前提: すべて満たせる fixture(both=%v phys=%v spec=%v)", both.Feasible, onlyPhys.Feasible, onlySpec.Feasible)
	}
	if both.Plan.TotalSP >= onlyPhys.Plan.TotalSP+onlySpec.Plan.TotalSP {
		t.Errorf("H を共有していない: both=%d, phys+spec=%d", both.Plan.TotalSP, onlyPhys.Plan.TotalSP+onlySpec.Plan.TotalSP)
	}
	for i, o := range both.Goals {
		if !o.Met || o.Kind != SPGoalSurvive {
			t.Errorf("goals[%d] = %+v, want survive・満たす", i, o)
		}
	}
}

// 満たす目標の数が最上位のキー(critic I-1(a))。前の目標(耐える)が届かないときは、後ろの目標(素早さ)を満たす組を選ぶ。
func TestSPGoalsMetCountComesFirst(t *testing.T) {
	in := goalsInput(t, goalsSelf(NatureNeutral, Stats{SpA: 32}), Stats{HP: 32, Def: 32, Spe: 32},
		surviveGoal(CategoryPhysical, 220, 1, 0), outspeedGoal(natureGoalsSpeUp, 32, 0, 0))
	got := mustSuggestGoals(t, in)
	if got.Goals[0].Met || !got.Goals[1].Met || got.Plan.SP != (Stats{SpA: 32, Spe: 25}) {
		t.Errorf("goals=%+v SP=%+v, want 耐える✗・素早さ✓・S25", got.Goals, got.Plan.SP)
	}
}

// 耐久側の同点は H, B, D の辞書順(小さい方)で決める(critic I-1(b)。ADR-0150 §6)。
// H の実数値 = 100 + h、B の実数値 = 100 + b になる種族では、(h, b) と (b, h) が合計 SP・耐久指数・満たすかで同点になる。
func TestSPGoalsBulkTieBrokenByLexOrder(t *testing.T) {
	sp := adjSelfAttackerSpecies()
	sp.BaseStats.HP = sp.BaseStats.Def - 55 // H 実数値 = 100 + h、B 実数値 = 100 + b
	self := Individual{Species: sp, Level: DefaultLevel, Nature: NatureNeutral, Status: StatusNone}
	in := goalsInput(t, self, statsAll(MaxSPPerStat), surviveGoal(CategoryPhysical, 120, 1, 0))
	cache := goalsOracleCountCache{}
	low, high := goalsOracleEval(t, in, Stats{HP: 13, Def: 16}, cache), goalsOracleEval(t, in, Stats{HP: 16, Def: 13}, cache)
	if low.met != 1 || high.met != 1 || !slices.Equal(low.bulk, high.bulk) {
		t.Fatalf("前提が崩れた: (H13, B16) と (H16, B13) が同点でない: %+v / %+v", low, high)
	}
	got := mustSuggestGoals(t, in)
	assertGoalsResult(t, got, goalsOracle(t, in))
	if got.Plan.SP != (Stats{HP: 13, Def: 16}) {
		t.Errorf("SP = %+v, want H13 B16(同点は H が小さい方)", got.Plan.SP)
	}
}

// 予算が足りないときは前の目標を優先する(ADR-0177 §5。順に意味がある)。
func TestSPGoalsOrderDecidesFallback(t *testing.T) {
	self := goalsSelf(NatureNeutral, Stats{SpA: 32}) // 予算 34: 素早さだけ(S 25)・耐えるだけ(約 32)は届くが両方は届かない
	ceiling := Stats{HP: 32, Def: 32, Spe: 32}
	speed := outspeedGoal(natureGoalsSpeUp, 32, 0, 0)
	bulk := surviveGoal(CategoryPhysical, 200, 1, 0)

	speedFirst := mustSuggestGoals(t, goalsInput(t, self, ceiling, speed, bulk))
	bulkFirst := mustSuggestGoals(t, goalsInput(t, self, ceiling, bulk, speed))
	if speedFirst.Feasible || bulkFirst.Feasible {
		t.Fatalf("前提: 予算 34 では両方は満たせない(%v, %v)", speedFirst.Feasible, bulkFirst.Feasible)
	}
	if !speedFirst.Goals[0].Met {
		t.Errorf("素早さが先なら素早さを満たす: %+v", speedFirst.Goals)
	}
	if !bulkFirst.Goals[0].Met {
		t.Errorf("耐えるが先なら耐えるを満たす: %+v", bulkFirst.Goals)
	}
	if speedFirst.Plan.SP == bulkFirst.Plan.SP {
		t.Errorf("順を入れ替えても同じ組: %+v", speedFirst.Plan.SP)
	}
}

// ---------------------------------------------------------------------------
// B4: 不正入力・相性表・純粋性
// ---------------------------------------------------------------------------

func TestSPGoalsRejectsInvalidInput(t *testing.T) {
	full := statsAll(MaxSPPerStat)
	base := func(t *testing.T, goals ...SPGoal) SPGoalsInput {
		return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, goals...)
	}
	speed := outspeedGoal(NatureNeutral, 0, 0, 0)
	seven := []SPGoal{speed, speed, speed, speed, speed, speed, speed}
	tests := []struct {
		name string
		in   func(t *testing.T) SPGoalsInput
	}{
		{"目標が 0 件", func(t *testing.T) SPGoalsInput { return base(t) }},
		{"目標が 7 件", func(t *testing.T) SPGoalsInput { return base(t, seven...) }},
		{"未知の種類", func(t *testing.T) SPGoalsInput { g := speed; g.Kind = "faster"; return base(t, g) }},
		{"survive の hits 0", func(t *testing.T) SPGoalsInput { return base(t, surviveGoal(CategoryPhysical, 100, 0, 0)) }},
		{"ko の hits 11", func(t *testing.T) SPGoalsInput { return base(t, koGoal(CategoryPhysical, 100, MaxAdjustHits+1, 0)) }},
		{"ko のしきい値 100 超", func(t *testing.T) SPGoalsInput { return base(t, koGoal(CategoryPhysical, 100, 1, 100.5)) }},
		{"survive のしきい値が負", func(t *testing.T) SPGoalsInput { return base(t, surviveGoal(CategoryPhysical, 100, 1, -1)) }},
		{"ko が変化技", func(t *testing.T) SPGoalsInput { return base(t, koGoal(CategoryStatus, 0, 1, 0)) }},
		{"survive の威力 0", func(t *testing.T) SPGoalsInput { return base(t, surviveGoal(CategorySpecial, 0, 1, 0)) }},
		{"outspeed の段数 7", func(t *testing.T) SPGoalsInput { return base(t, outspeedGoal(NatureNeutral, 0, 0, 7)) }},
		{"outspeed の段数 -7", func(t *testing.T) SPGoalsInput { return base(t, outspeedGoal(NatureNeutral, 0, 0, -7)) }},
		{"survive に段数", func(t *testing.T) SPGoalsInput {
			g := surviveGoal(CategoryPhysical, 100, 1, 0)
			g.SelfSpeedStage = 1
			return base(t, g)
		}},
		{"自分の下限の合計 67", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 32, Atk: 32, Spe: 3}), full, speed)
		}},
		{"自分のランク 7", func(t *testing.T) SPGoalsInput {
			self := goalsSelf(NatureNeutral, Stats{})
			self.Ranks.Spe = 7
			return goalsInput(t, self, full, speed)
		}},
		{"相手が不正(SP 33)", func(t *testing.T) SPGoalsInput { return base(t, outspeedGoal(NatureNeutral, 33, 0, 0)) }},
		{"探索する能力の上限が下限未満", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{Spe: 10}), Stats{Spe: 9}, speed)
		}},
		{"探索する能力の上限 33", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), Stats{Spe: 33}, speed)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SuggestSPForGoals(tt.in(t))
			if !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err = %v, want ErrInvalidAdjustInput", err)
			}
		})
	}
}

func TestSPGoalsAcceptsEdgeInput(t *testing.T) {
	speed := outspeedGoal(NatureNeutral, 0, 0, 0)
	tests := []struct {
		name string
		in   func(t *testing.T) SPGoalsInput
	}{
		{"目標 6 件", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), speed, speed, speed, speed, speed, speed)
		}},
		{"探索しない能力の上限は見ない(下限未満でもよい)", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{HP: 10}), Stats{Spe: 32}, speed)
		}},
		{"outspeed の hits・しきい値・技は読まない", func(t *testing.T) SPGoalsInput {
			g := speed
			g.Hits, g.ThresholdPercent, g.Move = 99, 500, adjMove(CategoryStatus, 0)
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), g)
		}},
		{"outspeed の段数 ±6", func(t *testing.T) SPGoalsInput {
			return goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), outspeedGoal(NatureNeutral, 0, 0, 6), outspeedGoal(NatureNeutral, 0, 0, -6))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SuggestSPForGoals(tt.in(t)); err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

func TestSPGoalsTypeChart(t *testing.T) {
	t.Run("outspeed だけなら相性表なしで動く", func(t *testing.T) {
		in := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), outspeedGoal(NatureNeutral, 0, 0, 0))
		in.TypeChart = TypeChart{}
		if _, err := SuggestSPForGoals(in); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
	t.Run("survive に相性表が無ければ CalcDamage のエラー(入力不正とは区別する)", func(t *testing.T) {
		in := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat), surviveGoal(CategoryPhysical, 100, 1, 0))
		in.TypeChart = TypeChart{}
		_, err := SuggestSPForGoals(in)
		if err == nil || errors.Is(err, ErrInvalidAdjustInput) {
			t.Errorf("err = %v, want 相性表のエラー(ErrInvalidAdjustInput ではない)", err)
		}
	})
}

func TestSPGoalsIsPureAndDeterministic(t *testing.T) {
	in := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), statsAll(MaxSPPerStat),
		outspeedGoal(natureGoalsSpeUp, 32, 0, 1), koGoal(CategoryPhysical, 130, 2, 0), surviveGoal(CategorySpecial, 200, 1, 0))
	before := slices.Clone(in.Goals)
	first := mustSuggestGoals(t, in)
	second := mustSuggestGoals(t, in)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("同じ入力で結果が違う:\n%+v\n%+v", first, second)
	}
	if !reflect.DeepEqual(in.Goals, before) {
		t.Errorf("入力の Goals を書き換えた")
	}
	if len(first.Goals) != len(in.Goals) {
		t.Errorf("goals の件数 = %d, want %d", len(first.Goals), len(in.Goals))
	}
	for i, o := range first.Goals {
		if o.Kind != in.Goals[i].Kind {
			t.Errorf("goals[%d].Kind = %q, want %q(順を保つ)", i, o.Kind, in.Goals[i].Kind)
		}
		if o.Kind != SPGoalOutspeed && (o.SelfSpeed != 0 || o.OpponentSpeed != 0 || o.SelfSpeedRank != 0) {
			t.Errorf("goals[%d] の素早さの欄は outspeed だけ: %+v", i, o)
		}
		if o.Kind == SPGoalOutspeed && o.ChancePercent != 0 {
			t.Errorf("goals[%d] の確率は outspeed では 0: %+v", i, o)
		}
	}
}

// ---------------------------------------------------------------------------
// B5: 未対応の印
// ---------------------------------------------------------------------------

func TestSPGoalsCarriesUnsupportedMarks(t *testing.T) {
	full := statsAll(MaxSPPerStat)
	multi := func(g SPGoal) SPGoal {
		g.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		return g
	}
	t.Run("印なし・outspeed だけは nil", func(t *testing.T) {
		for _, in := range []SPGoalsInput{
			goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, outspeedGoal(NatureNeutral, 0, 0, 0)),
			goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, koGoal(CategoryPhysical, 130, 2, 0)),
		} {
			if got := mustSuggestGoals(t, in).Unsupported; got != nil {
				t.Errorf("印 = %+v, want nil", got)
			}
		}
	})
	t.Run("目標の順に連結し、同じ印は1つ", func(t *testing.T) {
		in := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full,
			outspeedGoal(NatureNeutral, 0, 0, 0), multi(koGoal(CategoryPhysical, 130, 2, 0)), multi(surviveGoal(CategorySpecial, 200, 1, 0)))
		want := goalsOracleMarks(t, in)
		if len(want) == 0 {
			t.Fatal("前提が崩れた: 複数回当たる技に印が付かない")
		}
		if got := mustSuggestGoals(t, in).Unsupported; !reflect.DeepEqual(got, want) {
			t.Errorf("印 = %+v, want %+v", got, want)
		}
	})
	t.Run("印は数値を変えない", func(t *testing.T) {
		plain := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, koGoal(CategoryPhysical, 130, 2, 0))
		marked := goalsInput(t, goalsSelf(NatureNeutral, Stats{}), full, multi(koGoal(CategoryPhysical, 130, 2, 0)))
		a, b := mustSuggestGoals(t, plain), mustSuggestGoals(t, marked)
		if a.Plan != b.Plan || a.Feasible != b.Feasible {
			t.Errorf("印で組が変わった: %+v / %+v", a.Plan, b.Plan)
		}
	})
}

// ---------------------------------------------------------------------------
// B6: 先に使う技の段数
// ---------------------------------------------------------------------------

func TestGuaranteedSelfSpeedStage(t *testing.T) {
	effect := func(chance int, target RankTarget, stages map[StatKey]int) *MoveEffect {
		return &MoveEffect{Chance: chance, Target: target, Stages: stages}
	}
	tests := []struct {
		name string
		move Move
		want int
	}{
		{"効果なし", Move{Category: CategoryPhysical, Power: 50}, 0},
		{"確率 100%・自分の素早さ +1(ニトロチャージ)", Move{Category: CategoryPhysical, Power: 50, Effect: effect(100, RankTargetSelf, map[StatKey]int{StatSpe: 1})}, 1},
		{"確率 100%・自分の素早さ +2(変化技でもよい)", Move{Category: CategoryStatus, Effect: effect(100, RankTargetSelf, map[StatKey]int{StatSpe: 2})}, 2},
		{"確率 100%・自分の素早さ -1(下降もそのまま)", Move{Category: CategoryPhysical, Power: 100, Effect: effect(100, RankTargetSelf, map[StatKey]int{StatSpe: -1})}, -1},
		{"確率 100%・自分の攻撃と素早さ", Move{Category: CategoryStatus, Effect: effect(100, RankTargetSelf, map[StatKey]int{StatAtk: 1, StatSpe: 1})}, 1},
		{"確率 100% 未満は掛けない", Move{Category: CategoryPhysical, Power: 50, Effect: effect(99, RankTargetSelf, map[StatKey]int{StatSpe: 1})}, 0},
		{"相手が対象は掛けない", Move{Category: CategorySpecial, Power: 55, Effect: effect(100, RankTargetOpponent, map[StatKey]int{StatSpe: -1})}, 0},
		{"素早さを含まない", Move{Category: CategoryPhysical, Power: 120, Effect: effect(100, RankTargetSelf, map[StatKey]int{StatDef: -1, StatSpD: -1})}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuaranteedSelfSpeedStage(tt.move); got != tt.want {
				t.Errorf("GuaranteedSelfSpeedStage = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// B7: 分解の前提(ADR-0177 §6)
// ---------------------------------------------------------------------------

// CalcDamage のロールは、攻撃側の A/C(技の分類)と防御側の H・B/D(同)以外の SP で変わらない。
// この前提が崩れたら(例: ボディプレスを防御で計算するようになったら)ADR-0177 の分解を見直す。
func TestSPGoalsDecompositionPremise(t *testing.T) {
	chart := mustTypeChart(t)
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		for _, mech := range [][]MoveMechanism{nil, {MechanismAltOffenseStat}, {MechanismAltDefenseStat}} {
			move := adjMove(cat, 100)
			move.Mechanisms = mech
			atkKey, defKey := reverseStat(SideAttacker, cat), reverseStat(SideDefender, cat)
			calc := func(attackerSP, defenderSP Stats) DamageResult {
				res, err := CalcDamage(DamageInput{
					Format: FormatSingle, Move: move, TypeChart: chart,
					Attacker: Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: attackerSP, Status: StatusNone},
					Defender: Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: defenderSP, Status: StatusNone},
				})
				if err != nil {
					t.Fatalf("CalcDamage: %v", err)
				}
				return res
			}
			base := calc(Stats{}.WithStat(atkKey, 10), Stats{HP: 10}.WithStat(defKey, 10))
			for _, k := range AllStatKeys() {
				if k != atkKey {
					if got := calc(Stats{}.WithStat(atkKey, 10).WithStat(k, 20), Stats{HP: 10}.WithStat(defKey, 10)); got.Rolls != base.Rolls {
						t.Errorf("%s/%v: 攻撃側の %s の SP でロールが変わった", cat, mech, k)
					}
				}
				if k != StatHP && k != defKey {
					if got := calc(Stats{}.WithStat(atkKey, 10), Stats{HP: 10}.WithStat(defKey, 10).WithStat(k, 20)); got.Rolls != base.Rolls || got.DefenderHP != base.DefenderHP {
						t.Errorf("%s/%v: 防御側の %s の SP でロールが変わった", cat, mech, k)
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ベンチマーク(上限ちょうどの最悪: 6 件で全 6 能力・全範囲・hits 10・しきい値 50%)
// ---------------------------------------------------------------------------

func BenchmarkSuggestSPForGoalsAtLimit(b *testing.B) {
	chart, err := loadGoldenTypeChart()
	if err != nil {
		b.Fatalf("相性表: %v", err)
	}
	in := SPGoalsInput{
		Format: FormatSingle, Self: goalsSelf(NatureNeutral, Stats{}), Ceiling: statsAll(MaxSPPerStat),
		Field: Field{Weather: WeatherNone, Terrain: TerrainNone}, TypeChart: chart,
		Goals: []SPGoal{
			outspeedGoal(natureGoalsSpeUp, 32, 0, 1),
			koGoal(CategoryPhysical, 80, MaxAdjustHits, 50), koGoal(CategorySpecial, 80, MaxAdjustHits, 50),
			surviveGoal(CategoryPhysical, 40, MaxAdjustHits, 50), surviveGoal(CategorySpecial, 40, MaxAdjustHits, 50),
			outspeedGoal(NatureNeutral, 32, 1, 0),
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := SuggestSPForGoals(in); err != nil {
			b.Fatalf("SuggestSPForGoals: %v", err)
		}
	}
}
