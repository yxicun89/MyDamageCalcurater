package engine

// ADR-0126 追記(計算量)の後続: 特性の候補を渡したときの逆算・一括計算の計算量を下げる。結果は変えない。
//
// 受け入れ条件(ADR-0126「追記(2026-10-02): 計算量の受け入れ条件」):
//   AC-1 等価性: CalcReverse / CalcBulk の結果(エラーを含む)が、最適化前の実装(このファイルの
//        refCalcReverse / refCalcBulk。2026-10-02 時点の本体を凍結した参照実装)と reflect.DeepEqual で一致する。
//        特性の候補 0〜3・まとまる/分かれる・最初/真ん中/最後の特性で違いが出る・未対応の印・無効・
//        受けたダメージの逆算(side=attacker)・持ち物候補あり/なし・MaxCandidates の切り取りを含む。
//   AC-2 逆算の決定的な予算: 最悪ケース(持ち物 64・観測 16・特性 3 つが全部違う結果)の CalcReverse 1 回が
//        確保するメモリが reverseWorstAbilitiesBudgetBytes 以下。特性なしの同じ入力は reverseWorstNoAbilityBudgetBytes 以下。
//        壁時計ではなく確保バイト数で見る(CI の負荷で揺れない。同じ Go の版なら決定的)。
//   AC-2b 一括計算の決定的な予算: 最悪ケース(プリセット 8・持ち物 64)で結果が同じ特性 3 つのとき、CalcBulk 1 回の
//        確保が bulkWorstSameAbilitiesBudgetBytes 以下(まとめた特性の KO を何度も作らない)。
//   AC-3 ベンチマーク: BenchmarkCalcReverseAbilityCandidatesWorst / BenchmarkCalcBulkAbilityCandidatesWorst で
//        特性なし・3つ同じ・3つ違う を並べて測れる(閾値は判定しない。ADR に実測を書く)。
//
// 実測(2026-10-02, Apple M5 Pro, go1.27.1, 最適化前。このファイルのベンチマーク):
//   逆算  特性なし 8.6ms / 20MB、3つ同じ 23.8ms / 60MB、3つ違う 43.7ms / 105MB
//   一括  特性なし 0.64ms / 2.3MB、3つ同じ 2.1ms / 6.3MB、3つ違う 3.5ms / 11.3MB
// 時間と確保のほとんどは reflect.DeepEqual ではなく、各 CalcDamage の確定数の確率(ComputeKO → koProbability)。
// ダメージが小さい(半減の特性・防御の補正など)ほど攻撃回数が増えて重くなるので、特性ごとに重さが違い、
// 「特性 3 つ = 3 倍」を超える。逆算の候補は KO を出力に使わない(KO は Rolls と DefenderHP だけの関数なので、
// KO を作らなくても特性のまとめの判定は変わらない)。

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// 参照実装(凍結)。本体の最適化で意味を変えてはならない。本体の helper のシグネチャを変えるときは、
// ここでは旧版を複製して参照実装の振る舞いを保つこと(参照を本体に合わせて書き換えない)。
// ---------------------------------------------------------------------------

func refGroupAbilities(n int, same func(i, j int) bool) [][]int {
	var groups [][]int
	for i := range n {
		placed := false
		for g := range groups {
			if same(groups[g][0], i) {
				groups[g] = append(groups[g], i)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, []int{i})
		}
	}
	return groups
}

func refCalcBulk(in BulkInput) (BulkResult, error) {
	if len(in.Presets) > MaxBulkPresets {
		return BulkResult{}, fmt.Errorf("%w: %d 件", ErrTooManyPresets, len(in.Presets))
	}
	if len(in.PresetKeys) > MaxBulkPresets {
		return BulkResult{}, fmt.Errorf("%w: %d 件", ErrTooManyPresets, len(in.PresetKeys))
	}
	if len(in.ItemVariants) > MaxBulkItemVariants {
		return BulkResult{}, fmt.Errorf("%w: %d 件", ErrTooManyItemVariants, len(in.ItemVariants))
	}
	if err := in.DefenderOverride.validate(); err != nil {
		return BulkResult{}, err
	}
	presets, err := selectPresets(in)
	if err != nil {
		return BulkResult{}, err
	}
	for _, p := range presets {
		if err := p.validate(); err != nil {
			return BulkResult{}, err
		}
	}
	abilities, err := abilityCandidates("DefenderAbilities", in.DefenderSpecies, in.DefenderAbilities)
	if err != nil {
		return BulkResult{}, err
	}
	variants := in.ItemVariants
	if len(variants) == 0 {
		variants = []*Item{nil}
	}
	results := make([][][]DamageResult, len(abilities))
	for a, ability := range abilities {
		results[a] = make([][]DamageResult, len(presets))
		for pi, p := range presets {
			results[a][pi] = make([]DamageResult, len(variants))
			for vi, item := range variants {
				def := p.Defender(in.DefenderSpecies, item)
				def.Ability = ability
				in.DefenderOverride.apply(&def)
				// 参照実装も公開の CalcDamage を呼ぶ(KO の値は自己参照の比較)。CalcDamage そのものの不変はゴールデンが担保する。
				res, err := CalcDamage(DamageInput{
					Format: in.Format, Attacker: in.Attacker, Defender: def, Move: in.Move,
					Field: in.Field, Critical: in.Critical, TypeChart: in.TypeChart,
				})
				if err != nil {
					return BulkResult{}, fmt.Errorf("防御側プリセット %q・特性 %q の計算: %w", p.Key, ability.ID, err)
				}
				results[a][pi][vi] = res
			}
		}
	}
	groups := refGroupAbilities(len(abilities), func(i, j int) bool {
		return reflect.DeepEqual(results[i], results[j])
	})
	rows := make([]BulkRow, 0, len(presets)*len(groups)*len(variants))
	for pi, p := range presets {
		label := p.Label
		if label == "" {
			label = string(p.Key)
		}
		for _, group := range groups {
			ability := abilities[group[0]]
			ids := groupAbilityIDs(abilities, group)
			for vi, item := range variants {
				def := p.Defender(in.DefenderSpecies, item)
				def.Ability = ability
				in.DefenderOverride.apply(&def)
				row := BulkRow{
					Preset: p.Key, PresetLabel: label, Item: item, Ability: ability,
					AbilityIDs: slices.Clone(ids), Defender: def, Result: results[group[0]][pi][vi],
				}
				if item != nil {
					row.ItemID = item.ID
				}
				rows = append(rows, row)
			}
		}
	}
	return BulkResult{DefenderSpeciesKey: in.DefenderSpecies.Key, Rows: rows}, nil
}

func refReverseCandidate(obs []Observation, rolls *[MaxSPPerStat + 1]DamageResult) ReverseCandidate {
	dist := make([]int, MaxSPPerStat+1)
	for x, res := range rolls {
		sum := 0
		for _, o := range obs {
			best := -1
			for _, r := range res.Rolls {
				d := o.Distance(r, res.DefenderHP)
				if best < 0 || d < best {
					best = d
				}
			}
			sum += best
		}
		dist[x] = sum
	}
	mismatch := slices.Min(dist)
	var sps []int
	for x, d := range dist {
		if d == mismatch {
			sps = append(sps, x)
		}
	}
	support, minT, maxT := 0, -1, -1
	for _, x := range sps {
		res := rolls[x]
		for _, o := range obs {
			for _, r := range res.Rolls {
				if o.Matches(r, res.DefenderHP) {
					support++
				}
			}
		}
		lo, hi := res.DisplayPercentRangeTenths()
		if minT < 0 || lo < minT {
			minT = lo
		}
		if hi > maxT {
			maxT = hi
		}
	}
	return ReverseCandidate{
		Ranges: collapseSPRanges(sps), SPCount: len(sps), Exact: mismatch == 0, Mismatch: mismatch,
		Support: support, MinPercentTenths: minT, MaxPercentTenths: maxT, Unsupported: rolls[0].Unsupported,
	}
}

func refCalcReverse(in ReverseInput) (ReverseResult, error) {
	if in.Side != SideDefender && in.Side != SideAttacker {
		return ReverseResult{}, fmt.Errorf("%w: %q", ErrInvalidReverseSide, in.Side)
	}
	if len(in.ItemCandidates) > MaxReverseItemCandidates {
		return ReverseResult{}, fmt.Errorf("%w: %d 件", ErrTooManyItemCandidates, len(in.ItemCandidates))
	}
	if len(in.Observations) > MaxReverseObservations {
		return ReverseResult{}, fmt.Errorf("%w: %d 件", ErrTooManyObservations, len(in.Observations))
	}
	if in.MaxCandidates < 0 || in.MaxCandidates > MaxReverseMaxCandidates {
		return ReverseResult{}, fmt.Errorf("%w: %d", ErrInvalidMaxCandidates, in.MaxCandidates)
	}
	if err := validateObservations(in.Observations); err != nil {
		return ReverseResult{}, err
	}
	if in.Move.Category == CategoryStatus || in.Move.Power <= 0 {
		return ReverseResult{}, fmt.Errorf("%w: 技 %q は変化技か威力 0(分類=%s, 威力=%d)",
			ErrMoveDealsNoDamage, in.Move.ID, in.Move.Category, in.Move.Power)
	}
	stat := reverseStat(in.Side, in.Move.Category)
	items := in.ItemCandidates
	if len(items) == 0 {
		items = []*Item{nil}
	}
	assumedHPSP := 0
	if in.Side == SideDefender {
		assumedHPSP = MaxSPPerStat
	}
	abilities, err := abilityCandidates("UnknownAbilities", in.UnknownSpecies, in.UnknownAbilities)
	if err != nil {
		return ReverseResult{}, err
	}
	rolls := make([][][][MaxSPPerStat + 1]DamageResult, len(abilities))
	dealsDamage := false
	for a, ability := range abilities {
		rolls[a] = make([][][MaxSPPerStat + 1]DamageResult, len(reverseClasses))
		for ci, class := range reverseClasses {
			nature := natureForClass(stat, class)
			rolls[a][ci] = make([][MaxSPPerStat + 1]DamageResult, len(items))
			for ii, item := range items {
				for x := 0; x <= MaxSPPerStat; x++ {
					sp := Stats{}.WithStat(stat, x)
					if in.Side == SideDefender {
						sp.HP = MaxSPPerStat
					}
					unknown := Individual{
						Species: in.UnknownSpecies, Level: DefaultLevel, Nature: nature, Ability: ability,
						SP: sp, Item: item, Status: StatusNone,
					}
					dmg := DamageInput{
						Format: in.Format, Move: in.Move, Field: in.Field, Critical: in.Critical, TypeChart: in.TypeChart,
					}
					if in.Side == SideDefender {
						dmg.Attacker, dmg.Defender = in.Known, unknown
					} else {
						dmg.Attacker, dmg.Defender = unknown, in.Known
					}
					res, err := CalcDamage(dmg)
					if err != nil {
						return ReverseResult{}, fmt.Errorf("逆算の候補(性格クラス=%s, 特性=%q, 持ち物=%q, SP=%d)の計算: %w",
							class, ability.ID, itemID(item), x, err)
					}
					rolls[a][ci][ii][x] = res
					if res.Rolls[len(res.Rolls)-1] > 0 {
						dealsDamage = true
					}
				}
			}
		}
	}
	groups := refGroupAbilities(len(abilities), func(i, j int) bool {
		return reflect.DeepEqual(rolls[i], rolls[j])
	})
	cands := make([]ReverseCandidate, 0, len(reverseClasses)*len(groups)*len(items))
	for ci, class := range reverseClasses {
		nature := natureForClass(stat, class)
		for _, group := range groups {
			ids := groupAbilityIDs(abilities, group)
			for ii, item := range items {
				c := refReverseCandidate(in.Observations, &rolls[group[0]][ci][ii])
				c.NatureClass, c.Nature, c.Item = class, nature, item
				c.Ability, c.AbilityIDs = abilities[group[0]], slices.Clone(ids)
				if item != nil {
					c.ItemID = item.ID
				}
				cands = append(cands, c)
			}
		}
	}
	if !dealsDamage {
		return ReverseResult{}, fmt.Errorf("%w: 技 %q はどの候補にもダメージが 0(タイプ相性・特性の無効)",
			ErrMoveDealsNoDamage, in.Move.ID)
	}
	exactCount := 0
	for _, c := range cands {
		if c.Exact {
			exactCount++
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Mismatch != b.Mismatch {
			return a.Mismatch < b.Mismatch
		}
		if a.Support != b.Support {
			return a.Support > b.Support
		}
		return a.SPCount > b.SPCount
	})
	n := len(cands)
	if in.MaxCandidates > 0 && in.MaxCandidates < n {
		n = in.MaxCandidates
	}
	return ReverseResult{
		Side: in.Side, Stat: stat, AssumedHPSP: assumedHPSP,
		Candidates: cands[:n], ExactCount: exactCount,
	}, nil
}

// ---------------------------------------------------------------------------
// フィクスチャ(効果だけを書く。特性名・持ち物名はコードに書かない)
// ---------------------------------------------------------------------------

// perfResist は水技の攻撃実数値を mod/4096 にする防御側の特性(半減系)。
func perfResist(id string, mod int) Ability {
	return Ability{ID: id, NameJa: "テストとくせい", Effect: &AbilityEffect{DefResistType: map[Type]int{TypeWater: mod}}}
}

func perfWaterImmune() Ability {
	return Ability{ID: "perf-water-immune", NameJa: "テストとくせい", Effect: &AbilityEffect{DefImmuneTypes: []Type{TypeWater}}}
}

// perfUnsupportedDef は効果が同じでも ID が印に載るので、ID が違えば結果が違う(まとめてはならない)。
func perfUnsupportedDef(id string) Ability {
	return Ability{ID: id, NameJa: "テストとくせい", Effect: &AbilityEffect{UnsupportedDefender: true}}
}

// perfWaterBoost は攻撃側の水技の攻撃実数値を上げる特性(受けたダメージの逆算用)。
func perfWaterBoost() Ability {
	return Ability{ID: "perf-water-boost", NameJa: "テストとくせい", Effect: &AbilityEffect{OffBoostType: TypeWater, OffBoostTypeMod: 6144}}
}

// perfItems は n 件の持ち物候補(先頭は持ち物なし)。防御の補正をずらし、ロールが互いに違うようにする
// (同じロールの使い回し・メモ化だけで予算を満たせないようにするため)。
func perfItems(n int) []*Item {
	items := make([]*Item, n)
	for i := 1; i < n; i++ {
		items[i] = &Item{ID: fmt.Sprintf("perf-item-%02d", i), NameJa: "テストもちもの",
			Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: Modifier4096 + 64*i}}}
	}
	return items
}

// perfObservations は真の個体(B に spDef を振った防御側)の実ロールから観測を作る。
func perfObservations(t testing.TB, known Individual, truth Individual, move Move, n int, attackerSide bool) []Observation {
	t.Helper()
	in := DamageInput{Format: FormatSingle, Attacker: known, Defender: truth, Move: move, TypeChart: typeChartForTests()}
	if attackerSide {
		in.Attacker, in.Defender = truth, known
	}
	res, err := CalcDamage(in)
	if err != nil {
		t.Fatalf("観測の生成: %v", err)
	}
	obs := make([]Observation, n)
	for i := range obs {
		if i%2 == 0 {
			obs[i] = Observation{Damage: res.Rolls[(i*5)%16]}
		} else {
			obs[i] = Observation{Percent: 10 + i}
		}
	}
	return obs
}

func perfReverseInput(t testing.TB, abilities []Ability, items []*Item, nObs int) ReverseInput {
	t.Helper()
	known := revStrongAttacker()
	move := revMove(CategoryPhysical)
	truth := revDefender(Stats{HP: MaxSPPerStat, Def: 12}, NatureNeutral, nil)
	return ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: known, UnknownSpecies: revDefenderSpecies(),
		Move: move, ItemCandidates: items, UnknownAbilities: abilities, TypeChart: typeChartForTests(),
		Observations: perfObservations(t, known, truth, move, nObs, false),
	}
}

// ---------------------------------------------------------------------------
// AC-1 等価性
// ---------------------------------------------------------------------------

type abilityEquivCase struct {
	name      string
	abilities []Ability
	// groups は参照実装で特性がいくつのまとまりに分かれるか(フィクスチャが意図どおり「まとまる/分かれる」を
	// 作れているかの前提確認。違えばテストの組み立てが壊れている)。
	groups int
}

// abilityGroupCount は結果に現れる特性のまとまりの数(AbilityIDs の種類数)。
func abilityGroupCount[T any](xs []T, ids func(T) []string) int {
	seen := map[string]bool{}
	for _, x := range xs {
		seen[fmt.Sprint(ids(x))] = true
	}
	return len(seen)
}

// abilityEquivCases は防御側(水の物理技を受ける側)の特性の候補の組。
func abilityEquivCases() []abilityEquivCase {
	plain1, plain2, plain3 := candNoEffect("perf-plain-1"), candNoEffect("perf-plain-2"), candNoEffect("perf-plain-3")
	ground := candGroundImmune() // 水技には効かない(plain とまとまる)
	half, threeQ := perfResist("perf-resist-half", 2048), perfResist("perf-resist-3q", 3072)
	return []abilityEquivCase{
		{"特性なし", nil, 1},
		{"空スライス", []Ability{}, 1},
		{"1つ(効く)", []Ability{half}, 1},
		{"1つ(効かない)", []Ability{plain1}, 1},
		{"2つ同じ", []Ability{plain1, ground}, 1},
		{"2つ違う", []Ability{plain1, half}, 2},
		{"3つ同じ", []Ability{plain1, ground, plain2}, 1},
		{"最初の特性で違いが出る", []Ability{half, plain1, plain2}, 2},
		{"真ん中の特性で違いが出る", []Ability{plain1, half, plain2}, 2},
		{"最後の特性で違いが出る", []Ability{plain1, plain2, half}, 2},
		{"3つ全部違う", []Ability{half, threeQ, plain1}, 3},
		{"1つ目と3つ目が同じ", []Ability{half, plain1, perfResist("perf-resist-half-2", 2048)}, 2},
		{"無効を含む", []Ability{perfWaterImmune(), plain1, half}, 3},
		{"未対応の印は ID で分かれる", []Ability{perfUnsupportedDef("perf-unsup-1"), perfUnsupportedDef("perf-unsup-2"), plain3}, 3},
	}
}

func abilityEquivItems() []struct {
	name  string
	items []*Item
} {
	return []struct {
		name  string
		items []*Item
	}{
		{"持ち物なし", nil},
		{"持ち物候補あり", []*Item{nil, revEviolite(), revVest(), revBerry(TypeWater), perfItems(3)[2]}},
	}
}

func sameErr(t *testing.T, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("err=%v want %v", got, want)
	}
	if got != nil && got.Error() != want.Error() {
		t.Fatalf("err=%q want %q(参照実装と違う)", got, want)
	}
}

// AC-1(逆算): 特性の候補の組 × 持ち物候補 × MaxCandidates で、参照実装と完全に一致する。
func TestCalcReverseAbilityCandidatesMatchReference(t *testing.T) {
	for _, ac := range abilityEquivCases() {
		for _, ic := range abilityEquivItems() {
			for _, maxC := range []int{0, 5} {
				t.Run(fmt.Sprintf("%s/%s/max=%d", ac.name, ic.name, maxC), func(t *testing.T) {
					in := perfReverseInput(t, ac.abilities, ic.items, 3)
					in.MaxCandidates = maxC
					want, wantErr := refCalcReverse(in)
					if n := abilityGroupCount(want.Candidates, func(c ReverseCandidate) []string { return c.AbilityIDs }); maxC == 0 && n != ac.groups {
						t.Fatalf("前提: 参照実装の特性のまとまり=%d want %d", n, ac.groups)
					}
					got, err := CalcReverse(in)
					sameErr(t, err, wantErr)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("参照実装と違う\n got=%+v\nwant=%+v", got, want)
					}
				})
			}
		}
	}
}

// AC-1(逆算・受けたダメージ側): 相手 = 攻撃側の特性が違いを出す位置ごとに一致する。
func TestCalcReverseAttackerSideAbilityCandidatesMatchReference(t *testing.T) {
	boost, plain1, plain2 := perfWaterBoost(), candNoEffect("perf-plain-1"), candNoEffect("perf-plain-2")
	cases := []abilityEquivCase{
		{"特性なし", nil, 1},
		{"3つ同じ", []Ability{plain1, plain2, candGroundImmune()}, 1},
		{"最初の特性で違いが出る", []Ability{boost, plain1, plain2}, 2},
		{"最後の特性で違いが出る", []Ability{plain1, plain2, boost}, 2},
	}
	for _, ac := range cases {
		for _, ic := range abilityEquivItems() {
			t.Run(ac.name+"/"+ic.name, func(t *testing.T) {
				known := revDefender(Stats{HP: MaxSPPerStat}, NatureNeutral, nil)
				truth := Individual{Species: revAttackerSpecies(), Level: DefaultLevel, SP: Stats{Atk: 20}, Status: StatusNone}
				move := revMove(CategoryPhysical)
				in := ReverseInput{
					Format: FormatSingle, Side: SideAttacker, Known: known, UnknownSpecies: revAttackerSpecies(),
					Move: move, ItemCandidates: ic.items, UnknownAbilities: ac.abilities, TypeChart: typeChartForTests(),
					Observations: perfObservations(t, known, truth, move, 2, true),
				}
				want, wantErr := refCalcReverse(in)
				if n := abilityGroupCount(want.Candidates, func(c ReverseCandidate) []string { return c.AbilityIDs }); n != ac.groups {
					t.Fatalf("前提: 参照実装の特性のまとまり=%d want %d", n, ac.groups)
				}
				got, err := CalcReverse(in)
				sameErr(t, err, wantErr)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("参照実装と違う\n got=%+v\nwant=%+v", got, want)
				}
			})
		}
	}
}

// AC-1(逆算・エラー): 全特性で無効なら両方とも ErrMoveDealsNoDamage(同じ文言)。
func TestCalcReverseAllImmuneAbilitiesMatchReference(t *testing.T) {
	in := perfReverseInput(t, []Ability{perfWaterImmune()}, perfItems(4), 2)
	_, wantErr := refCalcReverse(in)
	_, err := CalcReverse(in)
	if !errors.Is(wantErr, ErrMoveDealsNoDamage) {
		t.Fatalf("前提: 参照実装の err=%v want ErrMoveDealsNoDamage", wantErr)
	}
	sameErr(t, err, wantErr)
}

// AC-1(一括計算): 特性の候補の組 × 持ち物 × プリセット(既定・全 8 つ)で、参照実装と完全に一致する。
func TestCalcBulkAbilityCandidatesMatchReference(t *testing.T) {
	allKeys := []PresetKey{PresetNone, PresetHP, PresetHBBoost, PresetHB, PresetHBFull, PresetHDBoost, PresetHD, PresetHDFull}
	cases := append(abilityEquivCases(), abilityEquivCase{"全特性で無効", []Ability{perfWaterImmune()}, 1})
	for _, ac := range cases {
		for _, ic := range abilityEquivItems() {
			for _, keys := range [][]PresetKey{nil, allKeys} {
				t.Run(fmt.Sprintf("%s/%s/presets=%d", ac.name, ic.name, len(keys)), func(t *testing.T) {
					in := bulkInput(CategoryPhysical, TypeWater)
					in.TypeChart = typeChartForTests()
					in.DefenderAbilities = ac.abilities
					in.ItemVariants = ic.items
					in.PresetKeys = keys
					want, wantErr := refCalcBulk(in)
					if n := abilityGroupCount(want.Rows, func(r BulkRow) []string { return r.AbilityIDs }); n != ac.groups {
						t.Fatalf("前提: 参照実装の特性のまとまり=%d want %d", n, ac.groups)
					}
					got, err := CalcBulk(in)
					sameErr(t, err, wantErr)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("参照実装と違う\n got=%+v\nwant=%+v", got, want)
					}
				})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC-2 逆算の決定的な予算(確保バイト数)
// ---------------------------------------------------------------------------

const (
	// reverseWorstAbilitiesBudgetBytes は最悪ケース(持ち物 64・観測 16・特性 3 つが全部違う)の CalcReverse 1 回の
	// 確保バイト数の上限。最適化前は約 105MB。特性ごとの SP 0..32 の結果(DamageResult 12,672 個 ≒ 3MB)を
	// 持つ実装でも収まり、確定数の確率の分布(koProbability)を逆算で作ると超える値にする。
	reverseWorstAbilitiesBudgetBytes = 12 << 20
	// reverseWorstNoAbilityBudgetBytes は同じ入力で特性なしのときの上限。最適化前は約 20MB。
	reverseWorstNoAbilityBudgetBytes = 4 << 20
	// bulkWorstSameAbilitiesBudgetBytes は一括計算の最悪ケースで結果が同じ特性 3 つのときの上限。最適化前は約 6.3MB
	// (特性なしは約 2.3MB で、その大半が 512 行ぶんの KO)。KO を代表の特性の行だけで作れば収まる値にする。
	bulkWorstSameAbilitiesBudgetBytes = 4 << 20
)

// allocatedBytesPerCall は f を runs 回呼んだときの1回あたりの確保バイト数(runtime.MemStats.TotalAlloc の差)。
// 壁時計と違い、同じ Go の版・同じ入力なら負荷によらず決まる。並列テストは無いので他のテストの確保は混ざらない。
// 上限テストが落ちたら、まず Go の版の差(map・スライスの伸び方)を疑う(一括計算の予算は実測の約 1.6 倍で一番きつい)。
func allocatedBytesPerCall(runs int, f func()) uint64 {
	f() // 初回だけの確保(sync.Once 等)を除く
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range runs {
		f()
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / uint64(runs)
}

func reverseWorstAbilities() []Ability {
	return []Ability{perfResist("perf-resist-half", 2048), perfResist("perf-resist-3q", 3072), candNoEffect("perf-plain-1")}
}

// AC-2: 逆算は確定数(KO)を出力に使わないので、最悪ケースでも確保量が予算内に収まる。
// 最適化前の実装ではここが失敗する(特性 3 つ: 約 105MB > 12MB、特性なし: 約 20MB > 4MB)。
func TestCalcReverseAbilityCandidatesAllocBudget(t *testing.T) {
	cases := []struct {
		name      string
		abilities []Ability
		budget    uint64
	}{
		{"特性なし", nil, reverseWorstNoAbilityBudgetBytes},
		{"特性3つが全部違う", reverseWorstAbilities(), reverseWorstAbilitiesBudgetBytes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := perfReverseInput(t, tc.abilities, perfItems(MaxReverseItemCandidates), MaxReverseObservations)
			if _, err := CalcReverse(in); err != nil {
				t.Fatalf("CalcReverse: %v", err)
			}
			got := allocatedBytesPerCall(3, func() { _, _ = CalcReverse(in) })
			if got > tc.budget {
				t.Errorf("CalcReverse 1 回の確保 = %d bytes(%.1f MiB)> 予算 %d bytes(%.1f MiB)",
					got, float64(got)/(1<<20), tc.budget, float64(tc.budget)/(1<<20))
			}
		})
	}
}

func bulkWorstInput(abilities []Ability) BulkInput {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.TypeChart = typeChartForTests()
	in.PresetKeys = []PresetKey{PresetNone, PresetHP, PresetHBBoost, PresetHB, PresetHBFull, PresetHDBoost, PresetHD, PresetHDFull}
	in.ItemVariants = perfItems(MaxBulkItemVariants)
	in.DefenderAbilities = abilities
	return in
}

func sameAbilities3() []Ability {
	return []Ability{candNoEffect("perf-plain-1"), candGroundImmune(), candNoEffect("perf-plain-2")}
}

// AC-2b: 一括計算は KO を出力に含むので作らないわけにはいかないが、結果が同じでまとめる特性の KO は要らない。
// 最適化前の実装ではここが失敗する(約 6.3MB > 4MB)。
func TestCalcBulkSameAbilityCandidatesAllocBudget(t *testing.T) {
	in := bulkWorstInput(sameAbilities3())
	res, err := CalcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if want := MaxBulkPresets * MaxBulkItemVariants; len(res.Rows) != want {
		t.Fatalf("前提: 行数=%d want %d(特性が1つにまとまっていない)", len(res.Rows), want)
	}
	got := allocatedBytesPerCall(3, func() { _, _ = CalcBulk(in) })
	if got > bulkWorstSameAbilitiesBudgetBytes {
		t.Errorf("CalcBulk 1 回の確保 = %d bytes(%.1f MiB)> 予算 %d bytes(%.1f MiB)",
			got, float64(got)/(1<<20), bulkWorstSameAbilitiesBudgetBytes, float64(bulkWorstSameAbilitiesBudgetBytes)/(1<<20))
	}
}

// ---------------------------------------------------------------------------
// AC-3 ベンチマーク(閾値は判定しない)
//
//	cd engine && go test -run '^$' -bench 'AbilityCandidatesWorst' -benchmem -count 5 .
// ---------------------------------------------------------------------------

func BenchmarkCalcReverseAbilityCandidatesWorst(b *testing.B) {
	sets := []struct {
		name      string
		abilities []Ability
	}{
		{"none", nil},
		{"same3", sameAbilities3()},
		{"diff3", reverseWorstAbilities()},
	}
	for _, s := range sets {
		b.Run(s.name, func(b *testing.B) {
			in := perfReverseInput(b, s.abilities, perfItems(MaxReverseItemCandidates), MaxReverseObservations)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := CalcReverse(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCalcBulkAbilityCandidatesWorst(b *testing.B) {
	sets := []struct {
		name      string
		abilities []Ability
	}{
		{"none", nil},
		{"same3", sameAbilities3()},
		{"diff3", reverseWorstAbilities()},
	}
	for _, s := range sets {
		b.Run(s.name, func(b *testing.B) {
			in := bulkWorstInput(s.abilities)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := CalcBulk(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
