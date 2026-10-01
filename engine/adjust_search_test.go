package engine

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// 倒せる/耐える最小 SP の探索(plan.md AJ2・ADR-0150 §7)の受け入れ条件。
//
// 正解は engine の探索からではなく、このファイルの総当たり(oracleMinSPToKO / oracleMinSPToSurvive)から
// 独立に導く。n 発の確率は koProbability を使わず、16^n 通りのロールの組を数え上げて求める
// (n ≤ 3 に限る)。ダメージそのものは CalcDamage を正とする(探索が式を再実装しないことの裏返し)。
//
// 種族はすべて架空(数値だけの最小 fixture。coding-rules §1)。

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

// adjSelfAttackerSpecies は「自分」が攻撃側のときの試験用種族(ノーマル単。HP90/A110/B80/C110/D80/S90)。
func adjSelfAttackerSpecies() Species {
	return Species{
		Key: "0993-000", NameJa: "テストちょうせいこう", Types: []Type{TypeNormal},
		BaseStats: Stats{HP: 90, Atk: 110, Def: 80, SpA: 110, SpD: 80, Spe: 90},
	}
}

// adjFoeSpecies は相手の試験用種族(みず単。HP95/A80/B90/C80/D90/S70)。
// MinSPToSurvive では「自分」の防御側としても使う。
func adjFoeSpecies() Species {
	return Species{
		Key: "0992-000", NameJa: "テストちょうせいぼう", Types: []Type{TypeWater},
		BaseStats: Stats{HP: 95, Atk: 80, Def: 90, SpA: 80, SpD: 90, Spe: 70},
	}
}

// adjGhostSpecies はノーマル技が無効になる試験用種族(ゴースト単)。
func adjGhostSpecies() Species {
	return Species{
		Key: "0991-000", NameJa: "テストちょうせいれい", Types: []Type{TypeGhost},
		BaseStats: Stats{HP: 95, Atk: 80, Def: 90, SpA: 80, SpD: 90, Spe: 70},
	}
}

// adjMove はノーマルタイプの試験用の技(攻撃側の種族とタイプ一致)。
func adjMove(cat MoveCategory, power int) Move {
	return Move{ID: "adjtest", NameJa: "テストちょうせいわざ", Type: TypeNormal, Category: cat, Power: power}
}

var (
	natureAdjAtkUp = Nature{Plus: StatAtk, Minus: StatSpe} // A↑S↓
	natureAdjSpAUp = Nature{Plus: StatSpA, Minus: StatSpe} // C↑S↓
	natureAdjDefUp = Nature{Plus: StatDef, Minus: StatSpe} // B↑S↓
	natureAdjSpDUp = Nature{Plus: StatSpD, Minus: StatSpe} // D↑S↓
)

// koSearchInput は MinSPToKO の入力を作る。自分(攻撃側)の SP・性格と、相手(防御側)は H32 の固定個体。
func koSearchInput(t *testing.T, cat MoveCategory, power, hits int, threshold float64, selfSP Stats, selfNature Nature) AdjustSearchInput {
	t.Helper()
	return AdjustSearchInput{
		Format:    FormatSingle,
		Attacker:  Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: selfNature, SP: selfSP, Status: StatusNone},
		Defender:  Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{HP: 32}, Status: StatusNone},
		Move:      adjMove(cat, power),
		TypeChart: mustTypeChart(t),
		Hits:      hits, ThresholdPercent: threshold,
	}
}

// surviveSearchInput は MinSPToSurvive の入力を作る。相手(攻撃側)は A32・C32 の固定個体、
// 自分(防御側)の SP・性格を渡す。
func surviveSearchInput(t *testing.T, cat MoveCategory, power, hits int, threshold float64, selfSP Stats, selfNature Nature) AdjustSearchInput {
	t.Helper()
	return AdjustSearchInput{
		Format:    FormatSingle,
		Attacker:  Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{Atk: 32, SpA: 32}, Status: StatusNone},
		Defender:  Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: selfNature, SP: selfSP, Status: StatusNone},
		Move:      adjMove(cat, power),
		TypeChart: mustTypeChart(t),
		Hits:      hits, ThresholdPercent: threshold,
	}
}

// ---------------------------------------------------------------------------
// 独立の総当たり(oracle)
// ---------------------------------------------------------------------------

// oracleMaxHits は数え上げで扱う発数の上限(16^3 = 4096 通り)。
const oracleMaxHits = 3

// oracleKOCount は 16^hits 通りのロールの組を数え上げ、合計が hp 以上になる組の数と全組数を返す。
func oracleKOCount(t *testing.T, rolls [16]int, hp, hits int) (ko, total int) {
	t.Helper()
	if hits < 1 || hits > oracleMaxHits {
		t.Fatalf("oracle は 1..%d 発だけ数え上げる: %d", oracleMaxHits, hits)
	}
	var walk func(depth, sum int)
	walk = func(depth, sum int) {
		if depth == hits {
			total++
			if sum >= hp {
				ko++
			}
			return
		}
		for _, r := range rolls {
			walk(depth+1, sum+r)
		}
	}
	walk(0, 0)
	return ko, total
}

// oraclePercent は count/total を % にする(n ≤ 3 では 2 進で正確に表せる)。
func oraclePercent(count, total int) float64 {
	return float64(count) * 100 / float64(total)
}

// oracleThreshold は ThresholdPercent の既定(0 → 100)を解決する。
func oracleThreshold(v float64) float64 {
	if v == 0 {
		return 100
	}
	return v
}

// oracleDamage は CalcDamage を直接呼ぶ(探索側と同じ式を正とする)。
func oracleDamage(t *testing.T, in AdjustSearchInput, attacker, defender Individual) DamageResult {
	t.Helper()
	res, err := CalcDamage(DamageInput{
		Format: in.Format, Attacker: attacker, Defender: defender, Move: in.Move,
		Field: in.Field, Critical: in.Critical, TypeChart: in.TypeChart,
	})
	if err != nil {
		t.Fatalf("oracle の CalcDamage: %v", err)
	}
	return res
}

func oracleOffenseStat(cat MoveCategory) StatKey {
	if cat == CategorySpecial {
		return StatSpA
	}
	return StatAtk
}

func oracleDefenseStat(cat MoveCategory) StatKey {
	if cat == CategorySpecial {
		return StatSpD
	}
	return StatDef
}

// oracleKOChanceAt は自分の探索能力の SP を sp にしたときの「Hits 発で倒す確率(%)」。
func oracleKOChanceAt(t *testing.T, in AdjustSearchInput, sp int) float64 {
	t.Helper()
	attacker := in.Attacker
	attacker.SP = attacker.SP.WithStat(oracleOffenseStat(in.Move.Category), sp)
	res := oracleDamage(t, in, attacker, in.Defender)
	return oraclePercent(oracleKOCount(t, res.Rolls, res.DefenderHP, in.Hits))
}

// oracleMinSPToKO は SP を 0 から上限まで全部試し、しきい値を満たす最小の SP を返す。
func oracleMinSPToKO(t *testing.T, in AdjustSearchInput) KOSearchResult {
	t.Helper()
	stat := oracleOffenseStat(in.Move.Category)
	fixed := in.Attacker.SP.WithStat(stat, 0).Sum()
	limit := min(MaxSPPerStat, MaxSPTotal-fixed)
	threshold := oracleThreshold(in.ThresholdPercent)
	for sp := 0; sp <= limit; sp++ {
		if chance := oracleKOChanceAt(t, in, sp); chance >= threshold {
			return KOSearchResult{Stat: stat, SearchLimit: limit, Feasible: true, SP: sp, ChancePercent: chance}
		}
	}
	return KOSearchResult{Stat: stat, SearchLimit: limit, Feasible: false, SP: limit, ChancePercent: oracleKOChanceAt(t, in, limit)}
}

// oracleSurviveCandidate は耐久側の組1つ。
type oracleSurviveCandidate struct {
	hpSP, statSP, index int
	chance              float64
}

// oracleBetterSurvive は ADR-0150 §6 の順(合計 SP 小 → 耐久指数大 → HPSP 小)で a が b より前かを返す。
func oracleBetterSurvive(a, b oracleSurviveCandidate) bool {
	if ta, tb := a.hpSP+a.statSP, b.hpSP+b.statSP; ta != tb {
		return ta < tb
	}
	if a.index != b.index {
		return a.index > b.index
	}
	return a.hpSP < b.hpSP
}

// oracleSurviveCandidates は H と B/D の SP の組を全部試す(各 0..32、合計 ≤ 上限)。
func oracleSurviveCandidates(t *testing.T, in AdjustSearchInput) (stat StatKey, limit int, cands []oracleSurviveCandidate) {
	t.Helper()
	stat = oracleDefenseStat(in.Move.Category)
	fixed := in.Defender.SP.WithStat(StatHP, 0).WithStat(stat, 0).Sum()
	limit = MaxSPTotal - fixed
	for h := 0; h <= MaxSPPerStat; h++ {
		for s := 0; s <= MaxSPPerStat && h+s <= limit; s++ {
			defender := in.Defender
			defender.SP = defender.SP.WithStat(StatHP, h).WithStat(stat, s)
			res := oracleDamage(t, in, in.Attacker, defender)
			ko, total := oracleKOCount(t, res.Rolls, res.DefenderHP, in.Hits)
			real := RealStats(defender)
			cands = append(cands, oracleSurviveCandidate{
				hpSP: h, statSP: s, index: real.HP * real.Get(stat), chance: oraclePercent(total-ko, total),
			})
		}
	}
	return stat, limit, cands
}

// oracleMinSPToSurvive はしきい値を満たす組のうち ADR-0150 §6 の順で最初のものを返す。
// 満たす組が無ければ、耐える確率が最大の組のうち同じ順で最初のもの。
func oracleMinSPToSurvive(t *testing.T, in AdjustSearchInput) SurviveSearchResult {
	t.Helper()
	stat, limit, cands := oracleSurviveCandidates(t, in)
	threshold := oracleThreshold(in.ThresholdPercent)
	var best *oracleSurviveCandidate
	for i := range cands {
		c := &cands[i]
		if c.chance >= threshold && (best == nil || oracleBetterSurvive(*c, *best)) {
			best = c
		}
	}
	feasible := best != nil
	if !feasible {
		for i := range cands {
			c := &cands[i]
			if best == nil || c.chance > best.chance || (c.chance == best.chance && oracleBetterSurvive(*c, *best)) {
				best = c
			}
		}
	}
	return SurviveSearchResult{
		Stat: stat, SearchLimit: limit, Feasible: feasible,
		HPSP: best.hpSP, StatSP: best.statSP, TotalSP: best.hpSP + best.statSP,
		BulkIndex: best.index, ChancePercent: best.chance,
	}
}

// chanceEqual は確率(%)の比較。n ≤ 3 では両者とも正確に表せるが、計算順の違いに備えて微小な差を許す。
func chanceEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func assertKOResult(t *testing.T, got, want KOSearchResult) {
	t.Helper()
	if got.Stat != want.Stat || got.SearchLimit != want.SearchLimit || got.Feasible != want.Feasible ||
		got.SP != want.SP || !chanceEqual(got.ChancePercent, want.ChancePercent) {
		t.Errorf("MinSPToKO=%+v\n want %+v", got, want)
	}
}

func assertSurviveResult(t *testing.T, got, want SurviveSearchResult) {
	t.Helper()
	if got.Stat != want.Stat || got.SearchLimit != want.SearchLimit || got.Feasible != want.Feasible ||
		got.HPSP != want.HPSP || got.StatSP != want.StatSP || got.TotalSP != want.TotalSP ||
		got.BulkIndex != want.BulkIndex || !chanceEqual(got.ChancePercent, want.ChancePercent) {
		t.Errorf("MinSPToSurvive=%+v\n want %+v", got, want)
	}
}

// ---------------------------------------------------------------------------
// 前提の確認(fixture が意図した境界を作っていること。探索の実装前から通る)
// ---------------------------------------------------------------------------

// TestAdjustSearchFixturePremises は境界テストの fixture が意図どおりの状況を作ることを oracle で確かめる。
// これが落ちたら、境界テストの失敗は探索の誤りではなく fixture の誤り。
func TestAdjustSearchFixturePremises(t *testing.T) {
	t.Run("攻撃: 威力150・2発は SP0 で既に確定", func(t *testing.T) {
		want := oracleMinSPToKO(t, koSearchInput(t, CategoryPhysical, 150, 2, 0, Stats{}, NatureNeutral))
		if !want.Feasible || want.SP != 0 {
			t.Fatalf("前提が崩れた: %+v", want)
		}
	})
	t.Run("攻撃: 威力20・1発は 32 でも倒せない", func(t *testing.T) {
		want := oracleMinSPToKO(t, koSearchInput(t, CategoryPhysical, 20, 1, 0, Stats{}, NatureNeutral))
		if want.Feasible || want.ChancePercent != 0 {
			t.Fatalf("前提が崩れた: %+v", want)
		}
	})
	t.Run("攻撃: 威力100・確定3発は 0 と 32 の間", func(t *testing.T) {
		want := oracleMinSPToKO(t, koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, NatureNeutral))
		if !want.Feasible || want.SP == 0 || want.SP >= MaxSPPerStat {
			t.Fatalf("前提が崩れた: %+v", want)
		}
	})
	t.Run("攻撃: 威力250・1発は SP0 で 0% から乱数1発を経て確定1発になる", func(t *testing.T) {
		in := koSearchInput(t, CategoryPhysical, 250, 1, 0, Stats{}, NatureNeutral)
		if c := oracleKOChanceAt(t, in, 0); c != 0 {
			t.Fatalf("SP0 で既に %v%%", c)
		}
		want := oracleMinSPToKO(t, in)
		if !want.Feasible || want.SP == 0 || want.SP >= MaxSPPerStat {
			t.Fatalf("前提が崩れた: %+v", want)
		}
		partial := false
		for sp := 0; sp <= MaxSPPerStat; sp++ {
			if c := oracleKOChanceAt(t, in, sp); c > 0 && c < 100 {
				partial = true
			}
		}
		if !partial {
			t.Fatal("乱数1発(0% < 確率 < 100%)の SP が無い")
		}
	})
	t.Run("耐久: 威力40・1発は SP0 で既に確定で耐える", func(t *testing.T) {
		want := oracleMinSPToSurvive(t, surviveSearchInput(t, CategoryPhysical, 40, 1, 0, Stats{}, NatureNeutral))
		if !want.Feasible || want.TotalSP != 0 {
			t.Fatalf("前提が崩れた: %+v", want)
		}
	})
	t.Run("耐久: 威力250・急所・1発はどう振っても耐えられない", func(t *testing.T) {
		in := surviveSearchInput(t, CategoryPhysical, 250, 1, 0, Stats{}, NatureNeutral)
		in.Critical = true
		want := oracleMinSPToSurvive(t, in)
		if want.Feasible {
			t.Fatalf("前提が崩れた: %+v", want)
		}
	})
	t.Run("耐久: 威力200・確定で耐えるには 0 より多く上限より少なく振る", func(t *testing.T) {
		want := oracleMinSPToSurvive(t, surviveSearchInput(t, CategoryPhysical, 200, 1, 0, Stats{}, NatureNeutral))
		if !want.Feasible || want.TotalSP == 0 || want.TotalSP >= 2*MaxSPPerStat {
			t.Fatalf("前提が崩れた: %+v", want)
		}
	})
	t.Run("耐久の総当たり表で、合計 SP 最小の組が複数ある(同点の規則が効く)ケースがある", func(t *testing.T) {
		ties := 0
		for _, in := range surviveGrid(t) {
			_, _, cands := oracleSurviveCandidates(t, in)
			threshold := oracleThreshold(in.ThresholdPercent)
			minTotal, count := math.MaxInt, 0
			for _, c := range cands {
				if c.chance < threshold {
					continue
				}
				switch total := c.hpSP + c.statSP; {
				case total < minTotal:
					minTotal, count = total, 1
				case total == minTotal:
					count++
				}
			}
			if count >= 2 {
				ties++
			}
		}
		if ties == 0 {
			t.Fatal("同点の規則を検査できるケースが無い")
		}
	})
	t.Run("性質テストの表が「SP0 で満たす」「途中」「不可」をすべて含む", func(t *testing.T) {
		var koZero, koMid, koNone int
		for _, in := range koGrid(t) {
			switch w := oracleMinSPToKO(t, in); {
			case !w.Feasible:
				koNone++
			case w.SP == 0:
				koZero++
			default:
				koMid++
			}
		}
		var svZero, svMid, svNone int
		for _, in := range surviveGrid(t) {
			switch w := oracleMinSPToSurvive(t, in); {
			case !w.Feasible:
				svNone++
			case w.TotalSP == 0:
				svZero++
			default:
				svMid++
			}
		}
		if koZero == 0 || koMid == 0 || koNone == 0 || svZero == 0 || svMid == 0 || svNone == 0 {
			t.Fatalf("偏った表: KO(0=%d 途中=%d 不可=%d) 耐久(0=%d 途中=%d 不可=%d)", koZero, koMid, koNone, svZero, svMid, svNone)
		}
		t.Logf("KO(0=%d 途中=%d 不可=%d) 耐久(0=%d 途中=%d 不可=%d)", koZero, koMid, koNone, svZero, svMid, svNone)
	})
}

// ---------------------------------------------------------------------------
// MinSPToKO
// ---------------------------------------------------------------------------

// TestAdjustMinSPToKOBoundaries は攻撃側の境界(0 で満たす・途中・32 でも不可・分類)を確かめる。
func TestAdjustMinSPToKOBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		in     func(t *testing.T) AdjustSearchInput
		assert func(t *testing.T, got KOSearchResult)
	}{
		{"SP0 で既に確定2発なら 0 を返す(振らない)",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategoryPhysical, 150, 2, 0, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got KOSearchResult) {
				if !got.Feasible || got.SP != 0 || got.ChancePercent != 100 {
					t.Errorf("got %+v want Feasible SP=0 100%%", got)
				}
			}},
		{"32 でも倒せないなら不可(エラーにしない)。SP は上限、確率は上限での値",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategoryPhysical, 20, 1, 0, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got KOSearchResult) {
				if got.Feasible || got.SP != MaxSPPerStat || got.SearchLimit != MaxSPPerStat {
					t.Errorf("got %+v want 不可・SP=上限", got)
				}
			}},
		{"確定3発に必要なだけ振り、最大振りにしない",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got KOSearchResult) {
				if !got.Feasible || got.SP == 0 || got.SP >= MaxSPPerStat {
					t.Errorf("got %+v want 0 < SP < 32", got)
				}
			}},
		{"ThresholdPercent 100 を明示しても既定と同じ",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategoryPhysical, 100, 3, 100, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got KOSearchResult) {}},
		{"特殊技は C を探索し、A の SP は結果に効かない(A を 0 にした入力と同じ結果)",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategorySpecial, 100, 3, 0, Stats{Atk: 32}, natureAdjSpAUp)
			},
			func(t *testing.T, got KOSearchResult) {
				if got.Stat != StatSpA {
					t.Errorf("Stat=%q want %q", got.Stat, StatSpA)
				}
				// A32 は固定の合計に入るが、C の上限は min(32, 66-32) = 32 のままなので結果は完全に一致するはず。
				zero, err := MinSPToKO(koSearchInput(t, CategorySpecial, 100, 3, 0, Stats{}, natureAdjSpAUp))
				if err != nil {
					t.Fatalf("err=%v", err)
				}
				if got.Feasible != zero.Feasible || got.SP != zero.SP || !chanceEqual(got.ChancePercent, zero.ChancePercent) {
					t.Errorf("A32=%+v A0=%+v(A の SP が結果に効いた)", got, zero)
				}
			}},
		{"性格で A が上がると必要な SP が減る(A↑)",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, natureAdjAtkUp)
			},
			func(t *testing.T, got KOSearchResult) {
				neutral := oracleMinSPToKO(t, koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, NatureNeutral))
				if got.SP > neutral.SP {
					t.Errorf("A↑ の SP=%d が無補正の %d より多い", got.SP, neutral.SP)
				}
			}},
		{"n 発の確率は最小の確定数ではなく、指定の Hits で求める(確定2発の火力で Hits=3)",
			func(t *testing.T) AdjustSearchInput {
				return koSearchInput(t, CategoryPhysical, 150, 3, 0, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got KOSearchResult) {
				if !got.Feasible || got.SP != 0 || got.ChancePercent != 100 {
					t.Errorf("got %+v want SP=0 100%%", got)
				}
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			got, err := MinSPToKO(in)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			assertKOResult(t, got, oracleMinSPToKO(t, in))
			tt.assert(t, got)
		})
	}
}

// TestAdjustMinSPToKOThresholdIsInclusive は「確率 >= しきい値」(ちょうど等しいときは満たす)を確かめる。
// 威力250・1発は SP を上げると 0% → 乱数1発 → 確定1発 と変わる(前提は TestAdjustSearchFixturePremises)。
// 乱数1発になった最初の SP の確率をしきい値にすると、その SP がちょうど閾値で最小になる。
func TestAdjustMinSPToKOThresholdIsInclusive(t *testing.T) {
	base := koSearchInput(t, CategoryPhysical, 250, 1, 0, Stats{}, NatureNeutral)
	firstPartial, threshold := -1, 0.0
	for sp := 0; sp <= MaxSPPerStat; sp++ {
		if c := oracleKOChanceAt(t, base, sp); c > 0 && c < 100 {
			firstPartial, threshold = sp, c
			break
		}
	}
	if firstPartial < 1 {
		t.Fatalf("乱数1発の SP が見つからない(fixture の誤り): %d", firstPartial)
	}

	t.Run("ちょうど閾値の確率なら満たす", func(t *testing.T) {
		in := base
		in.ThresholdPercent = threshold
		got, err := MinSPToKO(in)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !got.Feasible || got.SP != firstPartial || got.ChancePercent != threshold {
			t.Errorf("got %+v want SP=%d 確率=%v", got, firstPartial, threshold)
		}
		assertKOResult(t, got, oracleMinSPToKO(t, in))
	})
	t.Run("閾値をわずかに上げると次の段階まで振る", func(t *testing.T) {
		in := base
		in.ThresholdPercent = math.Nextafter(threshold, math.Inf(1))
		got, err := MinSPToKO(in)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got.SP <= firstPartial {
			t.Errorf("SP=%d は %d より大きいはず", got.SP, firstPartial)
		}
		assertKOResult(t, got, oracleMinSPToKO(t, in))
	})
	t.Run("しきい値を下げると必要な SP は増えない(確定 ≥ 乱数)", func(t *testing.T) {
		guaranteed, err := MinSPToKO(base)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		in := base
		in.ThresholdPercent = threshold
		relaxed, err := MinSPToKO(in)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if relaxed.SP > guaranteed.SP {
			t.Errorf("しきい値 %v%% の SP=%d、確定の SP=%d(%v%%)", threshold, relaxed.SP, guaranteed.SP, guaranteed.ChancePercent)
		}
	})
}

// TestAdjustMinSPToKOFixedSP は固定側の SP と合計 66 の制約を確かめる。
func TestAdjustMinSPToKOFixedSP(t *testing.T) {
	tests := []struct {
		name      string
		selfSP    Stats
		wantLimit int
	}{
		{"固定なしなら上限は 32", Stats{}, MaxSPPerStat},
		{"固定 H32・B32 なら残りは 2", Stats{HP: 32, Def: 32}, MaxSPTotal - 64},
		{"探索する A の入力値は無視する(A32 を渡しても固定 64 → 残り 2)", Stats{HP: 32, Def: 32, Atk: 32}, MaxSPTotal - 64},
		{"固定がちょうど 66 なら上限 0(エラーにしない)", Stats{HP: 32, Def: 32, Spe: 2}, 0},
		{"固定 35 なら上限は 31(各 32 と、合計 66 の残りの小さい方)", Stats{HP: 32, Spe: 3}, MaxSPTotal - 35},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := koSearchInput(t, CategoryPhysical, 100, 3, 0, tt.selfSP, NatureNeutral)
			got, err := MinSPToKO(in)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			if got.SearchLimit != tt.wantLimit {
				t.Errorf("SearchLimit=%d want %d", got.SearchLimit, tt.wantLimit)
			}
			if got.SP > got.SearchLimit {
				t.Errorf("SP=%d が上限 %d を超えた", got.SP, got.SearchLimit)
			}
			assertKOResult(t, got, oracleMinSPToKO(t, in))
		})
	}
}

// TestAdjustMinSPToKOImmune はタイプ相性で無効なら「不可」(エラーにしない)を返すことを確かめる。
func TestAdjustMinSPToKOImmune(t *testing.T) {
	in := koSearchInput(t, CategoryPhysical, 150, 1, 0, Stats{}, NatureNeutral)
	in.Defender.Species = adjGhostSpecies()
	got, err := MinSPToKO(in)
	if err != nil {
		t.Fatalf("err=%v want nil(不可はエラーにしない)", err)
	}
	if got.Feasible || got.ChancePercent != 0 {
		t.Errorf("got %+v want 不可・0%%", got)
	}
}

// ---------------------------------------------------------------------------
// MinSPToSurvive
// ---------------------------------------------------------------------------

// TestAdjustMinSPToSurviveBoundaries は耐久側の境界(0 で満たす・途中・どう振っても不可・分類)を確かめる。
func TestAdjustMinSPToSurviveBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		in     func(t *testing.T) AdjustSearchInput
		assert func(t *testing.T, got SurviveSearchResult)
	}{
		{"SP0 で既に確定で耐えるなら (0, 0)",
			func(t *testing.T) AdjustSearchInput {
				return surviveSearchInput(t, CategoryPhysical, 40, 1, 0, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got SurviveSearchResult) {
				if !got.Feasible || got.HPSP != 0 || got.StatSP != 0 || got.ChancePercent != 100 {
					t.Errorf("got %+v want (0,0) 100%%", got)
				}
			}},
		{"確定で耐えるのに必要なだけ振り、H・B を最大振りにしない",
			func(t *testing.T) AdjustSearchInput {
				return surviveSearchInput(t, CategoryPhysical, 200, 1, 0, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got SurviveSearchResult) {
				if !got.Feasible || got.TotalSP == 0 || got.TotalSP >= 2*MaxSPPerStat {
					t.Errorf("got %+v want 0 < 合計 < 64", got)
				}
			}},
		{"どう振っても耐えられないなら不可(エラーにしない)。組は耐える確率が最大のもの",
			func(t *testing.T) AdjustSearchInput {
				in := surviveSearchInput(t, CategoryPhysical, 250, 1, 0, Stats{}, NatureNeutral)
				in.Critical = true
				return in
			},
			func(t *testing.T, got SurviveSearchResult) {
				if got.Feasible {
					t.Errorf("got %+v want 不可", got)
				}
			}},
		{"特殊技は H と D を探索する",
			func(t *testing.T) AdjustSearchInput {
				return surviveSearchInput(t, CategorySpecial, 200, 1, 0, Stats{}, natureAdjSpDUp)
			},
			func(t *testing.T, got SurviveSearchResult) {
				if got.Stat != StatSpD {
					t.Errorf("Stat=%q want %q", got.Stat, StatSpD)
				}
			}},
		{"2発耐える(Hits=2)",
			func(t *testing.T) AdjustSearchInput {
				return surviveSearchInput(t, CategoryPhysical, 120, 2, 0, Stats{}, natureAdjDefUp)
			},
			func(t *testing.T, got SurviveSearchResult) {}},
		{"乱数で耐える(しきい値 50%)",
			func(t *testing.T) AdjustSearchInput {
				return surviveSearchInput(t, CategoryPhysical, 200, 1, 50, Stats{}, NatureNeutral)
			},
			func(t *testing.T, got SurviveSearchResult) {
				if got.Feasible && got.ChancePercent < 50 {
					t.Errorf("確率 %v%% がしきい値 50%% 未満", got.ChancePercent)
				}
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			got, err := MinSPToSurvive(in)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			assertSurviveResult(t, got, oracleMinSPToSurvive(t, in))
			tt.assert(t, got)
		})
	}
}

// TestAdjustMinSPToSurviveFixedSP は固定側の SP と合計 66 の制約を確かめる。
func TestAdjustMinSPToSurviveFixedSP(t *testing.T) {
	tests := []struct {
		name      string
		selfSP    Stats
		wantLimit int
	}{
		{"固定なしなら H+B の上限は 66(各 32 は別に守る)", Stats{}, MaxSPTotal},
		{"固定 A32・C30 なら H+B は 4 まで", Stats{Atk: 32, SpA: 30}, MaxSPTotal - 62},
		{"探索する H・B の入力値は無視する", Stats{Atk: 32, SpA: 30, HP: 32, Def: 32}, MaxSPTotal - 62},
		{"固定がちょうど 66 なら (0, 0) だけを試す", Stats{Atk: 32, SpA: 32, Spe: 2}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := surviveSearchInput(t, CategoryPhysical, 200, 1, 0, tt.selfSP, NatureNeutral)
			got, err := MinSPToSurvive(in)
			if err != nil {
				t.Fatalf("err=%v want nil", err)
			}
			if got.SearchLimit != tt.wantLimit {
				t.Errorf("SearchLimit=%d want %d", got.SearchLimit, tt.wantLimit)
			}
			if got.TotalSP > got.SearchLimit || got.HPSP > MaxSPPerStat || got.StatSP > MaxSPPerStat {
				t.Errorf("組 (%d, %d) が上限を超えた", got.HPSP, got.StatSP)
			}
			assertSurviveResult(t, got, oracleMinSPToSurvive(t, in))
		})
	}
}

// TestAdjustMinSPToSurviveImmune はタイプ相性で無効なら (0, 0) で確定で耐えることを確かめる。
func TestAdjustMinSPToSurviveImmune(t *testing.T) {
	in := surviveSearchInput(t, CategoryPhysical, 250, 1, 0, Stats{}, NatureNeutral)
	in.Defender.Species = adjGhostSpecies()
	got, err := MinSPToSurvive(in)
	if err != nil {
		t.Fatalf("err=%v want nil", err)
	}
	if !got.Feasible || got.TotalSP != 0 || got.ChancePercent != 100 {
		t.Errorf("got %+v want (0,0) 100%%", got)
	}
}

// ---------------------------------------------------------------------------
// 不正入力
// ---------------------------------------------------------------------------

// TestAdjustSearchRejectsInvalidInput は不正入力を ErrInvalidAdjustInput で拒否することを両方の探索で確かめる。
func TestAdjustSearchRejectsInvalidInput(t *testing.T) {
	type mutate func(in *AdjustSearchInput)
	common := []struct {
		name string
		f    mutate
	}{
		{"Hits 0", func(in *AdjustSearchInput) { in.Hits = 0 }},
		{"Hits が負", func(in *AdjustSearchInput) { in.Hits = -1 }},
		{"Hits が上限超過", func(in *AdjustSearchInput) { in.Hits = MaxAdjustHits + 1 }},
		{"しきい値が負", func(in *AdjustSearchInput) { in.ThresholdPercent = -1 }},
		{"しきい値が 100 超過", func(in *AdjustSearchInput) { in.ThresholdPercent = 100.5 }},
		{"しきい値が NaN", func(in *AdjustSearchInput) { in.ThresholdPercent = math.NaN() }},
		{"しきい値が +Inf", func(in *AdjustSearchInput) { in.ThresholdPercent = math.Inf(1) }},
		{"変化技", func(in *AdjustSearchInput) { in.Move.Category = CategoryStatus; in.Move.Power = 0 }},
		{"威力 0", func(in *AdjustSearchInput) { in.Move.Power = 0 }},
		{"威力が負", func(in *AdjustSearchInput) { in.Move.Power = -10 }},
		{"攻撃側のランクが範囲外", func(in *AdjustSearchInput) { in.Attacker.Ranks.Atk = 7 }},
		{"防御側のレベルが 50 以外", func(in *AdjustSearchInput) { in.Defender.Level = 49 }},
		{"攻撃側の種族値が範囲外", func(in *AdjustSearchInput) { in.Attacker.Species.BaseStats.Spe = 0 }},
	}
	for _, tt := range common {
		t.Run("KO/"+tt.name, func(t *testing.T) {
			in := koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, NatureNeutral)
			tt.f(&in)
			if _, err := MinSPToKO(in); !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
		t.Run("Survive/"+tt.name, func(t *testing.T) {
			in := surviveSearchInput(t, CategoryPhysical, 200, 1, 0, Stats{}, NatureNeutral)
			tt.f(&in)
			if _, err := MinSPToSurvive(in); !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}

	// 固定側の SP(探索しない能力)の不正。
	// defenderSP が nil なら koSearchInput の既定(相手は H32)のまま。
	koFixed := []struct {
		name       string
		sp         Stats
		defenderSP *Stats
	}{
		{"固定の合計が 66 超過(H32+B32+S3=67)", Stats{HP: 32, Def: 32, Spe: 3}, nil},
		{"固定の1能力が 32 超過", Stats{Spe: MaxSPPerStat + 1}, nil},
		{"固定の SP が負", Stats{HP: -1}, nil},
		{"相手側(防御側)の SP 合計が 66 超過", Stats{}, &Stats{HP: 32, Def: 32, SpD: 3}},
	}
	for _, tt := range koFixed {
		t.Run("KO/"+tt.name, func(t *testing.T) {
			in := koSearchInput(t, CategoryPhysical, 100, 3, 0, tt.sp, NatureNeutral)
			if tt.defenderSP != nil {
				in.Defender.SP = *tt.defenderSP
			}
			if _, err := MinSPToKO(in); !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}
	surviveFixed := []struct {
		name string
		sp   Stats
	}{
		{"固定の合計が 66 超過(A32+C32+S3=67)", Stats{Atk: 32, SpA: 32, Spe: 3}},
		{"固定の1能力が 32 超過", Stats{SpA: MaxSPPerStat + 1}},
		{"固定の SP が負", Stats{Spe: -1}},
	}
	for _, tt := range surviveFixed {
		t.Run("Survive/"+tt.name, func(t *testing.T) {
			in := surviveSearchInput(t, CategoryPhysical, 200, 1, 0, tt.sp, NatureNeutral)
			if _, err := MinSPToSurvive(in); !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}
}

// TestAdjustSearchAcceptsEdgeInput は値域の端(Hits の上限・しきい値 100 の明示・小さい正のしきい値)を受け付けることを確かめる。
func TestAdjustSearchAcceptsEdgeInput(t *testing.T) {
	tests := []struct {
		name      string
		hits      int
		threshold float64
	}{
		{"Hits = MaxAdjustHits", MaxAdjustHits, 0},
		{"Hits = 1", 1, 0},
		{"しきい値 100 を明示", 2, 100},
		{"しきい値が小さい正の値", 2, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := MinSPToKO(koSearchInput(t, CategoryPhysical, 100, tt.hits, tt.threshold, Stats{}, NatureNeutral)); err != nil {
				t.Errorf("MinSPToKO err=%v want nil", err)
			}
			if _, err := MinSPToSurvive(surviveSearchInput(t, CategoryPhysical, 200, tt.hits, tt.threshold, Stats{}, NatureNeutral)); err != nil {
				t.Errorf("MinSPToSurvive err=%v want nil", err)
			}
		})
	}
}

// TestAdjustSearchRequiresTypeChart は相性表が無ければ CalcDamage と同じ ErrTypeChartMissing を返すことを確かめる。
func TestAdjustSearchRequiresTypeChart(t *testing.T) {
	ko := koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, NatureNeutral)
	ko.TypeChart = TypeChart{}
	if _, err := MinSPToKO(ko); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("MinSPToKO err=%v want ErrTypeChartMissing", err)
	}
	sv := surviveSearchInput(t, CategoryPhysical, 200, 1, 0, Stats{}, NatureNeutral)
	sv.TypeChart = TypeChart{}
	if _, err := MinSPToSurvive(sv); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("MinSPToSurvive err=%v want ErrTypeChartMissing", err)
	}
}

// ---------------------------------------------------------------------------
// 最小性の性質テスト(独立の総当たりと全件照合)
// ---------------------------------------------------------------------------

// koGrid は攻撃側の性質テストの入力の組み合わせ。
func koGrid(t *testing.T) []AdjustSearchInput {
	t.Helper()
	var out []AdjustSearchInput
	for _, power := range []int{60, 80, 100, 120, 150, 250} {
		for _, hits := range []int{1, 2, 3} {
			for _, threshold := range []float64{0, 50, 93.75, 6.25} {
				for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
					for _, up := range []bool{false, true} {
						for _, fixed := range []Stats{{}, {HP: 32, Def: 32}} {
							nature := NatureNeutral
							if up && cat == CategoryPhysical {
								nature = natureAdjAtkUp
							} else if up {
								nature = natureAdjSpAUp
							}
							out = append(out, koSearchInput(t, cat, power, hits, threshold, fixed, nature))
						}
					}
				}
			}
		}
	}
	return out
}

// TestAdjustMinSPToKOMatchesBruteForce は威力・発数・しきい値・性格・分類・固定 SP の組み合わせで、
// 探索の結果が総当たりと一致することを確かめる(最小性・不可の判定・確率)。
func TestAdjustMinSPToKOMatchesBruteForce(t *testing.T) {
	for i, in := range koGrid(t) {
		name := fmt.Sprintf("#%d %s/威力%d/%d発/%v%%/固定%d", i, in.Move.Category, in.Move.Power, in.Hits, in.ThresholdPercent, in.Attacker.SP.Sum())
		got, err := MinSPToKO(in)
		if err != nil {
			t.Fatalf("%s: err=%v", name, err)
		}
		want := oracleMinSPToKO(t, in)
		if got.Stat != want.Stat || got.SearchLimit != want.SearchLimit || got.Feasible != want.Feasible ||
			got.SP != want.SP || !chanceEqual(got.ChancePercent, want.ChancePercent) {
			t.Errorf("%s: got %+v want %+v", name, got, want)
		}
	}
}

// surviveGrid は耐久側の性質テストの入力の組み合わせ。
func surviveGrid(t *testing.T) []AdjustSearchInput {
	t.Helper()
	var out []AdjustSearchInput
	for _, power := range []int{100, 140, 180, 200, 220, 250} {
		for _, hits := range []int{1, 2} {
			for _, threshold := range []float64{0, 50, 6.25} {
				for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
					for _, up := range []bool{false, true} {
						for _, fixed := range []Stats{{}, {Atk: 32, SpA: 30}} {
							nature := NatureNeutral
							if up && cat == CategoryPhysical {
								nature = natureAdjDefUp
							} else if up {
								nature = natureAdjSpDUp
							}
							out = append(out, surviveSearchInput(t, cat, power, hits, threshold, fixed, nature))
						}
					}
				}
			}
		}
	}
	return out
}

// TestAdjustMinSPToSurviveMatchesBruteForce は耐久側の結果が総当たり(合計 SP 最小 → 耐久指数最大 → H が小さい方)
// と一致し、制約(各 32・合計 ≤ 上限)を守ることを確かめる。
func TestAdjustMinSPToSurviveMatchesBruteForce(t *testing.T) {
	for i, in := range surviveGrid(t) {
		name := fmt.Sprintf("#%d %s/威力%d/%d発/%v%%/固定%d", i, in.Move.Category, in.Move.Power, in.Hits, in.ThresholdPercent, in.Defender.SP.Sum())
		got, err := MinSPToSurvive(in)
		if err != nil {
			t.Fatalf("%s: err=%v", name, err)
		}
		want := oracleMinSPToSurvive(t, in)
		if got.Stat != want.Stat || got.SearchLimit != want.SearchLimit || got.Feasible != want.Feasible ||
			got.HPSP != want.HPSP || got.StatSP != want.StatSP || got.TotalSP != want.TotalSP ||
			got.BulkIndex != want.BulkIndex || !chanceEqual(got.ChancePercent, want.ChancePercent) {
			t.Errorf("%s: got %+v want %+v", name, got, want)
		}
		if got.TotalSP != got.HPSP+got.StatSP || got.HPSP > MaxSPPerStat || got.StatSP > MaxSPPerStat || got.TotalSP > got.SearchLimit {
			t.Errorf("%s: 制約違反 %+v", name, got)
		}
	}
}

// TestAdjustSearchRankEdges はランクの端(攻撃側 +6・防御側 -6)を受け付け、総当たりと一致することを確かめる。
func TestAdjustSearchRankEdges(t *testing.T) {
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		t.Run(string(cat), func(t *testing.T) {
			ko := koSearchInput(t, cat, 60, 2, 0, Stats{}, NatureNeutral)
			ko.Attacker.Ranks = Ranks{Atk: 6, SpA: 6}
			ko.Defender.Ranks = Ranks{Def: -6, SpD: -6}
			gotKO, err := MinSPToKO(ko)
			if err != nil {
				t.Fatalf("MinSPToKO err=%v want nil", err)
			}
			assertKOResult(t, gotKO, oracleMinSPToKO(t, ko))

			sv := surviveSearchInput(t, cat, 40, 2, 0, Stats{}, NatureNeutral)
			sv.Attacker.Ranks = Ranks{Atk: 6, SpA: 6}
			sv.Defender.Ranks = Ranks{Def: -6, SpD: -6}
			gotSV, err := MinSPToSurvive(sv)
			if err != nil {
				t.Fatalf("MinSPToSurvive err=%v want nil", err)
			}
			assertSurviveResult(t, gotSV, oracleMinSPToSurvive(t, sv))
		})
	}
}

// TestAdjustMinSPToKOHitsMatchComputeKO は Hits = 4..MaxAdjustHits の確定判定が ComputeKO と矛盾しないことを確かめる。
// 固定 SP を 66 にして探索を SP0 の1点に絞り、ComputeKO の Hits(倒すのに必要な最小の発数)n について
// Hits=n の結果が「確定 ⇔ Feasible」「確率は ComputeKO と同じ(確定なら 100)」、Hits=n-1 は 0% になることを見る。
func TestAdjustMinSPToKOHitsMatchComputeKO(t *testing.T) {
	fixed := Stats{HP: 32, Def: 32, Spe: 2}
	covered, random := 0, 0
	for power := 10; power <= 80; power += 5 {
		base := koSearchInput(t, CategoryPhysical, power, 1, 0, fixed, NatureNeutral)
		res := oracleDamage(t, base, base.Attacker, base.Defender)
		ko := ComputeKO(res.Rolls, res.DefenderHP)
		if ko.Hits < 4 || ko.Hits > MaxAdjustHits {
			continue
		}
		covered++
		wantChance := 100.0
		if !ko.Guaranteed {
			random++
			wantChance = ko.ChancePercent
		}
		in := base
		in.Hits = ko.Hits
		got, err := MinSPToKO(in)
		if err != nil {
			t.Fatalf("威力%d: err=%v", power, err)
		}
		if got.SearchLimit != 0 || got.SP != 0 || got.Feasible != ko.Guaranteed || !chanceEqual(got.ChancePercent, wantChance) {
			t.Errorf("威力%d Hits=%d: got %+v want Feasible=%v 確率=%v(ComputeKO=%+v)", power, ko.Hits, got, ko.Guaranteed, wantChance, ko)
		}
		in.Hits = ko.Hits - 1
		prev, err := MinSPToKO(in)
		if err != nil {
			t.Fatalf("威力%d: err=%v", power, err)
		}
		if prev.Feasible || prev.ChancePercent != 0 {
			t.Errorf("威力%d Hits=%d: got %+v want 不可・0%%(ComputeKO の最小発数より少ない)", power, ko.Hits-1, prev)
		}
	}
	if covered == 0 || random == 0 {
		t.Fatalf("Hits 4..%d の検査ケースが偏った: 全 %d 件・乱数 %d 件", MaxAdjustHits, covered, random)
	}
}

// TestAdjustSearchCarriesUnsupportedMarks は探索中の CalcDamage の「未対応」の印が両方の結果に付き、
// 数値は印のない技の場合と同じことを確かめる(ADR-0150 §7・ADR-0123)。
func TestAdjustSearchCarriesUnsupportedMarks(t *testing.T) {
	mark := func(in AdjustSearchInput) AdjustSearchInput {
		in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
		return in
	}
	wantMarks := func(t *testing.T, in AdjustSearchInput) []UnsupportedMark {
		t.Helper()
		marks := oracleDamage(t, in, in.Attacker, in.Defender).Unsupported
		if len(marks) == 0 {
			t.Fatal("前提が崩れた: 複数回当たる技に印が付かない")
		}
		return marks
	}

	t.Run("KO", func(t *testing.T) {
		plain := koSearchInput(t, CategoryPhysical, 100, 3, 0, Stats{}, NatureNeutral)
		want, err := MinSPToKO(plain)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if want.Unsupported != nil {
			t.Errorf("印なしの技に印: %+v", want.Unsupported)
		}
		marked := mark(plain)
		got, err := MinSPToKO(marked)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !reflect.DeepEqual(got.Unsupported, wantMarks(t, marked)) {
			t.Errorf("Unsupported=%+v want %+v", got.Unsupported, wantMarks(t, marked))
		}
		got.Unsupported = nil
		assertKOResult(t, got, want)
	})
	t.Run("Survive", func(t *testing.T) {
		plain := surviveSearchInput(t, CategoryPhysical, 200, 1, 0, Stats{}, NatureNeutral)
		want, err := MinSPToSurvive(plain)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if want.Unsupported != nil {
			t.Errorf("印なしの技に印: %+v", want.Unsupported)
		}
		marked := mark(plain)
		got, err := MinSPToSurvive(marked)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !reflect.DeepEqual(got.Unsupported, wantMarks(t, marked)) {
			t.Errorf("Unsupported=%+v want %+v", got.Unsupported, wantMarks(t, marked))
		}
		assertSurviveResult(t, got, want)
	})
}
