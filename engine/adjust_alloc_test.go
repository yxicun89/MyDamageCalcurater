package engine

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

// SP 配分の提案(plan.md AJ3・ADR-0150 §8)の受け入れ条件。
//
// 正解は engine の実装からではなく、このファイルの総当たり(allocOracle)から独立に導く。
// 候補(回す能力が各 [下限, 上限]・回さない能力は下限・合計 ≤ 66)を全部列挙し、§8 の順を整数のキーの
// 辞書式比較で選ぶ。目標の確率は adjust_search_test.go の oracleKOCount(16^n 通りの数え上げ)で求める(n ≤ 3)。
// ダメージそのものは CalcDamage を正とする。
//
// 種族は adjust_search_test.go の架空種族を使う(数値だけの最小 fixture。coding-rules §1)。
//   - 耐久側の自分: adjFoeSpecies(H95/B90/D90/S70)→ H = 170 + h、B = 110 + b、D = 110 + d(無補正)
//   - 攻撃側の自分: adjSelfAttackerSpecies(A110/C110/S90)→ A = 130 + a、S = 110 + s(無補正)

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

var (
	natureAllocDefDown = Nature{Plus: StatAtk, Minus: StatDef} // A↑B↓(B の実数値が SP 1 で上がらない段がある)
	natureAllocSpDDown = Nature{Plus: StatAtk, Minus: StatSpD} // A↑D↓
	natureAllocSpeUp   = Nature{Plus: StatSpe, Minus: StatAtk} // S↑A↓
)

// allocFullCeiling は全能力の上限を 32 にした上限。
var allocFullCeiling = Stats{HP: MaxSPPerStat, Atk: MaxSPPerStat, Def: MaxSPPerStat, SpA: MaxSPPerStat, SpD: MaxSPPerStat, Spe: MaxSPPerStat}

// bulkAllocInput は耐久側の入力を作る(目標なし)。
func bulkAllocInput(focus BulkFocus, nature Nature, floor, ceiling Stats) SPAllocInput {
	return SPAllocInput{
		Self:    Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: nature, SP: floor, Status: StatusNone},
		Ceiling: ceiling, Mode: AllocModeBulk, Focus: focus,
	}
}

// offenseAllocInput は攻撃側の入力を作る(目標なし)。
func offenseAllocInput(cat MoveCategory, nature Nature, floor, ceiling Stats, minSpeed int) SPAllocInput {
	return SPAllocInput{
		Self:    Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: nature, SP: floor, Status: StatusNone},
		Ceiling: ceiling, Mode: AllocModeOffense, OffenseCategory: cat, MinSpeed: minSpeed,
	}
}

// bulkAllocGoal は耐久側の目標(相手 = A32・C32 の攻撃側が自分を攻撃する)。
func bulkAllocGoal(t *testing.T, cat MoveCategory, power, hits int, threshold float64) *AllocGoal {
	t.Helper()
	return &AllocGoal{
		Format:    FormatSingle,
		Opponent:  Individual{Species: adjSelfAttackerSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{Atk: 32, SpA: 32}, Status: StatusNone},
		Move:      adjMove(cat, power),
		TypeChart: mustTypeChart(t),
		Hits:      hits, ThresholdPercent: threshold,
	}
}

// offenseAllocGoal は攻撃側の目標(相手 = H32 の防御側を自分が攻撃する)。
func offenseAllocGoal(t *testing.T, cat MoveCategory, power, hits int, threshold float64) *AllocGoal {
	t.Helper()
	return &AllocGoal{
		Format:    FormatSingle,
		Opponent:  Individual{Species: adjFoeSpecies(), Level: DefaultLevel, Nature: NatureNeutral, SP: Stats{HP: 32}, Status: StatusNone},
		Move:      adjMove(cat, power),
		TypeChart: mustTypeChart(t),
		Hits:      hits, ThresholdPercent: threshold,
	}
}

// withGoal は入力に目標を付けた写しを返す。
func withGoal(in SPAllocInput, goal *AllocGoal) SPAllocInput {
	in.Goal = goal
	return in
}

// ---------------------------------------------------------------------------
// 独立の総当たり(oracle)
// ---------------------------------------------------------------------------

// allocOracleKeys は残り SP を回す能力(ADR-0150 §8 の表)。
func allocOracleKeys(in SPAllocInput) []StatKey {
	if in.Mode == AllocModeBulk {
		return []StatKey{StatHP, StatDef, StatSpD}
	}
	if in.OffenseCategory == CategorySpecial {
		return []StatKey{StatSpA, StatSpe}
	}
	return []StatKey{StatAtk, StatSpe}
}

// allocOracleCand は候補1つの評価。
type allocOracleCand struct {
	sp         Stats
	total      int
	real       Stats
	phys, spec int // 等倍の耐久指数 H×B・H×D
	speedMet   bool
	goalMet    bool
	goalCount  int // 目標の事象の組数(耐久側は耐える組、攻撃側は倒す組)。16^Hits 通り中
	chance     float64
}

// offense は攻撃側で比べる攻撃実数値(物理 → A、特殊 → C)。
func (c allocOracleCand) offense(in SPAllocInput) int {
	if in.OffenseCategory == CategorySpecial {
		return c.real.SpA
	}
	return c.real.Atk
}

// goalBulk は目標の技の分類の耐久指数。
func (c allocOracleCand) goalBulk(in SPAllocInput) int {
	if in.Goal != nil && in.Goal.Move.Category == CategorySpecial {
		return c.spec
	}
	return c.phys
}

// lexi は §6 の辞書順(H, A, B, C, D, S が小さい方が前)を「大きい方が前」のキーにしたもの。
func (c allocOracleCand) lexi() []int {
	s := c.sp
	return []int{-s.HP, -s.Atk, -s.Def, -s.SpA, -s.SpD, -s.Spe}
}

// allocOracleEval は SP の組1つを評価する。
func allocOracleEval(t *testing.T, in SPAllocInput, sp Stats) allocOracleCand {
	t.Helper()
	self := in.Self
	self.SP = sp
	real := RealStats(self)
	c := allocOracleCand{
		sp: sp, total: sp.Sum(), real: real,
		phys: real.HP * real.Def, spec: real.HP * real.SpD,
		speedMet: in.Mode == AllocModeBulk || real.Spe >= in.MinSpeed,
	}
	if g := in.Goal; g != nil {
		attacker, defender := g.Opponent, self
		if in.Mode == AllocModeOffense {
			attacker, defender = self, g.Opponent
		}
		res, err := CalcDamage(DamageInput{
			Format: g.Format, Attacker: attacker, Defender: defender, Move: g.Move,
			Field: g.Field, Critical: g.Critical, TypeChart: g.TypeChart,
		})
		if err != nil {
			t.Fatalf("oracle の CalcDamage: %v", err)
		}
		ko, total := oracleKOCount(t, res.Rolls, res.DefenderHP, g.Hits)
		c.goalCount = ko
		if in.Mode == AllocModeBulk {
			c.goalCount = total - ko
		}
		c.chance = oraclePercent(c.goalCount, total)
		c.goalMet = c.chance >= oracleThreshold(g.ThresholdPercent)
	}
	return c
}

// allocOracleEnumerate は候補を全部列挙する(回す能力が各 [下限, 上限]、回さない能力は下限、合計 ≤ 66)。
func allocOracleEnumerate(t *testing.T, in SPAllocInput) []allocOracleCand {
	t.Helper()
	keys := allocOracleKeys(in)
	var out []allocOracleCand
	var walk func(i int, sp Stats)
	walk = func(i int, sp Stats) {
		if i == len(keys) {
			if sp.Sum() <= MaxSPTotal {
				out = append(out, allocOracleEval(t, in, sp))
			}
			return
		}
		k := keys[i]
		for v := in.Self.SP.Get(k); v <= in.Ceiling.Get(k); v++ {
			walk(i+1, sp.WithStat(k, v))
		}
	}
	walk(0, in.Self.SP)
	if len(out) == 0 {
		t.Fatal("oracle: 候補が無い(fixture の誤り)")
	}
	return out
}

// allocKeyGreater はキーを辞書式に比べ、a が b より前(大きい)かを返す。
func allocKeyGreater(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// allocOraclePick は filter を満たす候補のうち key が最大のものを返す(無ければ nil)。
// 同じキーの候補が複数あれば、辞書順まで含むキーが不完全なので t.Fatal にする。
func allocOraclePick(t *testing.T, cands []allocOracleCand, filter func(allocOracleCand) bool, key func(allocOracleCand) []int) *allocOracleCand {
	t.Helper()
	var best *allocOracleCand
	var bestKey []int
	for i := range cands {
		c := &cands[i]
		if !filter(*c) {
			continue
		}
		k := key(*c)
		if best != nil && !allocKeyGreater(k, bestKey) && !allocKeyGreater(bestKey, k) {
			t.Fatalf("oracle: キーが同点の候補が2つある(%+v と %+v)", best.sp, c.sp)
		}
		if best == nil || allocKeyGreater(k, bestKey) {
			best, bestKey = c, k
		}
	}
	return best
}

func allocAll(allocOracleCand) bool { return true }

// allocOracleMaxIndexKey は指数最大の順(ADR-0150 §8 の表 → 合計 SP 小 → 辞書順)。
// 攻撃側は speedFeasible(素早さ目標を満たす候補があるか)で並べ方を変える。
func allocOracleMaxIndexKey(in SPAllocInput, speedFeasible bool) func(allocOracleCand) []int {
	return func(c allocOracleCand) []int {
		var head []int
		switch {
		case in.Mode == AllocModeBulk && in.Focus == BulkFocusPhysical:
			head = []int{c.phys}
		case in.Mode == AllocModeBulk && in.Focus == BulkFocusSpecial:
			head = []int{c.spec}
		case in.Mode == AllocModeBulk:
			head = []int{min(c.phys, c.spec), max(c.phys, c.spec)}
		case speedFeasible:
			head = []int{c.offense(in), c.real.Spe}
		default:
			head = []int{c.real.Spe, c.offense(in)}
		}
		return append(append(head, -c.total), c.lexi()...)
	}
}

// allocOracleMinSPKey は最小 SP の順(合計 SP 小 → 指数大 → 辞書順)。
func allocOracleMinSPKey(in SPAllocInput) func(allocOracleCand) []int {
	return func(c allocOracleCand) []int {
		index := c.goalBulk(in)
		if in.Mode == AllocModeOffense {
			index = c.offense(in)
		}
		return append([]int{-c.total, index}, c.lexi()...)
	}
}

// allocOracleFallbackKey は最小 SP の組が無いときの順(ADR-0150 §8)。
// 耐久側: 耐える確率 大 → 最小 SP の順。攻撃側: min(S, MinSpeed) 大 → 倒す確率 大 → 最小 SP の順。
func allocOracleFallbackKey(in SPAllocInput) func(allocOracleCand) []int {
	minSP := allocOracleMinSPKey(in)
	return func(c allocOracleCand) []int {
		head := []int{c.goalCount}
		if in.Mode == AllocModeOffense {
			head = []int{min(c.real.Spe, in.MinSpeed), c.goalCount}
		}
		return append(head, minSP(c)...)
	}
}

func allocOraclePlan(c allocOracleCand) AllocPlan {
	return AllocPlan{
		SP: c.sp, TotalSP: c.total, Real: c.real, PhysicalBulk: c.phys, SpecialBulk: c.spec,
		SpeedMet: c.speedMet, GoalMet: c.goalMet, ChancePercent: c.chance,
	}
}

// allocOracle は SuggestSPAllocation の正解を総当たりで求める(Unsupported は別のテストで照合する)。
func allocOracle(t *testing.T, in SPAllocInput) SPAllocResult {
	t.Helper()
	cands := allocOracleEnumerate(t, in)
	speedMet := func(c allocOracleCand) bool { return c.speedMet }
	speedFeasible := allocOraclePick(t, cands, speedMet, allocOracleMaxIndexKey(in, true)) != nil

	maxFilter := allocAll
	if speedFeasible {
		maxFilter = speedMet
	}
	out := SPAllocResult{
		Remaining: MaxSPTotal - in.Self.SP.Sum(),
		MaxIndex:  allocOraclePlan(*allocOraclePick(t, cands, maxFilter, allocOracleMaxIndexKey(in, speedFeasible))),
	}
	if in.Goal != nil {
		satisfied := func(c allocOracleCand) bool { return c.goalMet && c.speedMet }
		best := allocOraclePick(t, cands, satisfied, allocOracleMinSPKey(in))
		if best == nil {
			best = allocOraclePick(t, cands, allocAll, allocOracleFallbackKey(in))
		}
		plan := allocOraclePlan(*best)
		out.MinSP = &plan
	}
	return out
}

// ---------------------------------------------------------------------------
// 照合の補助
// ---------------------------------------------------------------------------

func allocPlanEqual(a, b AllocPlan) bool {
	return a.SP == b.SP && a.TotalSP == b.TotalSP && a.Real == b.Real &&
		a.PhysicalBulk == b.PhysicalBulk && a.SpecialBulk == b.SpecialBulk &&
		a.SpeedMet == b.SpeedMet && a.GoalMet == b.GoalMet && chanceEqual(a.ChancePercent, b.ChancePercent)
}

// assertAllocResult は Unsupported 以外を照合する。
func assertAllocResult(t *testing.T, got, want SPAllocResult) {
	t.Helper()
	if got.Remaining != want.Remaining {
		t.Errorf("Remaining=%d want %d", got.Remaining, want.Remaining)
	}
	if !allocPlanEqual(got.MaxIndex, want.MaxIndex) {
		t.Errorf("MaxIndex=%+v\n want %+v", got.MaxIndex, want.MaxIndex)
	}
	switch {
	case (got.MinSP == nil) != (want.MinSP == nil):
		t.Errorf("MinSP=%v want %v", got.MinSP, want.MinSP)
	case got.MinSP != nil && !allocPlanEqual(*got.MinSP, *want.MinSP):
		t.Errorf("MinSP=%+v\n want %+v", *got.MinSP, *want.MinSP)
	}
}

// assertAllocPlanBounds は組が §8 の制約(下限以上・回す能力は上限以下・回さない能力は下限のまま・
// 各 32・合計 66 以内・TotalSP と Real の整合)を守ることを確かめる。
func assertAllocPlanBounds(t *testing.T, label string, in SPAllocInput, p AllocPlan) {
	t.Helper()
	allocatable := map[StatKey]bool{}
	for _, k := range allocOracleKeys(in) {
		allocatable[k] = true
	}
	for _, k := range AllStatKeys() {
		v, floor := p.SP.Get(k), in.Self.SP.Get(k)
		switch {
		case v < floor:
			t.Errorf("%s: %s=%d が下限 %d 未満", label, k, v, floor)
		case allocatable[k] && v > in.Ceiling.Get(k):
			t.Errorf("%s: %s=%d が上限 %d 超過", label, k, v, in.Ceiling.Get(k))
		case !allocatable[k] && v != floor:
			t.Errorf("%s: 回さない能力 %s=%d が下限 %d から動いた", label, k, v, floor)
		case v > MaxSPPerStat:
			t.Errorf("%s: %s=%d が 32 超過", label, k, v)
		}
	}
	if p.SP.Sum() > MaxSPTotal || p.TotalSP != p.SP.Sum() {
		t.Errorf("%s: 合計 %d(TotalSP=%d)が不正", label, p.SP.Sum(), p.TotalSP)
	}
	self := in.Self
	self.SP = p.SP
	if p.Real != RealStats(self) {
		t.Errorf("%s: Real=%+v が RealStats と不一致", label, p.Real)
	}
}

func mustSuggest(t *testing.T, in SPAllocInput) SPAllocResult {
	t.Helper()
	got, err := SuggestSPAllocation(in)
	if err != nil {
		t.Fatalf("SuggestSPAllocation err=%v want nil", err)
	}
	return got
}

// ---------------------------------------------------------------------------
// 指数最大の境界(手計算の期待値。式は各ケースのコメント)
// ---------------------------------------------------------------------------

type allocMaxCase struct {
	name         string
	in           SPAllocInput
	wantSP       Stats
	wantSpeedMet bool
}

func allocBulkMaxCases() []allocMaxCase {
	full := allocFullCeiling
	return []allocMaxCase{
		{"残り 0(下限の合計がちょうど 66)は下限の組そのもの(エラーにしない)",
			bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{HP: 32, Def: 32, Spe: 2}, full),
			Stats{HP: 32, Def: 32, Spe: 2}, true},
		// 残り 34。(170+h)(110+b) は B に振る方が伸びる: b=32,h=2 → 172×142=24424 > h=3,b=31 → 173×141=24393。
		{"S32 固定・物理: 残り 34 を B から振り合計ちょうど 66",
			bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Spe: 32}, full),
			Stats{HP: 2, Def: 32, Spe: 32}, true},
		{"S32 固定・特殊: D から振る(B は振らない)",
			bulkAllocInput(BulkFocusSpecial, NatureNeutral, Stats{Spe: 32}, full),
			Stats{HP: 2, SpD: 32, Spe: 32}, true},
		// 残り 2: b=2 → 170×112=19040 > h=1,b=1 → 171×111=18981 > h=2 → 172×110=18920。
		{"A32・S32 固定・物理: 残り 2 は B に",
			bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Atk: 32, Spe: 32}, full),
			Stats{Atk: 32, Def: 2, Spe: 32}, true},
		{"上限 0 は下限より上に振らない",
			bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Spe: 32}, Stats{}),
			Stats{Spe: 32}, true},
		// B↓: floor((110+b)×9/10) は b=0,1 で 99、b=2 で 100。同じ実数値なら合計 SP が小さい方。
		{"B の下降補正で実数値が上がらない SP 1 は振らない",
			bulkAllocInput(BulkFocusPhysical, natureAllocDefDown, Stats{}, Stats{Def: 1}),
			Stats{}, true},
		{"B の下降補正でも SP 2 なら実数値が上がるので振る",
			bulkAllocInput(BulkFocusPhysical, natureAllocDefDown, Stats{}, Stats{Def: 2}),
			Stats{Def: 2}, true},
		{"H・B が上限なら物理は D を振らず余らせる(合計 20)",
			bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, Stats{HP: 10, Def: 10, SpD: 32}),
			Stats{HP: 10, Def: 10}, true},
		// 残り 34: h=32,b=1,d=1 → min=max=202×111=22422。h=30,b=d=2 → 200×112=22400。h=32,b=2,d=0 → min 202×110。
		{"両方: H を先に、残りを B と D に均等",
			bulkAllocInput(BulkFocusBoth, NatureNeutral, Stats{Spe: 32}, full),
			Stats{HP: 32, Def: 1, SpD: 1, Spe: 32}, true},
		// H は振らない(上限 0)。残り 35 を B・D に: (18,17) と (17,18) は min=170×127・max=170×128 で同点。
		// 辞書順(H, A, B, C, D, S が小さい方)で B=17 を選ぶ。
		{"両方で指数も合計も同点なら辞書順(B が小さい方)",
			bulkAllocInput(BulkFocusBoth, NatureNeutral, Stats{Spe: 31}, Stats{HP: 0, Def: 32, SpD: 32}),
			Stats{Def: 17, SpD: 18, Spe: 31}, true},
		// 残り 14、H は 20 以上: (190+h')(110+b) は b=14 → 190×124=23560 > h'=1,b=13 → 191×123=23493。
		{"下限 H20 は結果でも 20 以上(固定の下限を尊重)",
			bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{HP: 20, Spe: 32}, full),
			Stats{HP: 20, Def: 14, Spe: 32}, true},
	}
}

func allocOffenseMaxCases() []allocMaxCase {
	full := allocFullCeiling
	return []allocMaxCase{
		{"目標なし: A と S を上限まで(H・B・D は回さない。合計 64)",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, full, 0),
			Stats{Atk: 32, Spe: 32}, true},
		{"H32 固定・目標なし: A を先に、余りを S(合計 66)",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, full, 0),
			Stats{HP: 32, Atk: 32, Spe: 2}, true},
		// S = 110 + s ≥ 120 → s=10。残り 34 − 10 = 24 を A。
		{"素早さ 120: S は届く最小の 10、残りを A",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, full, 120),
			Stats{HP: 32, Atk: 24, Spe: 10}, true},
		{"素早さ 142 は S32 でちょうど届く",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, full, 142),
			Stats{HP: 32, Atk: 2, Spe: 32}, true},
		{"素早さ 143 は届かない: SpeedMet=false で S を最大、残りを A",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, full, 143),
			Stats{HP: 32, Atk: 2, Spe: 32}, false},
		// S↓: floor((110+s)×9/10) は s=0,1 で 99、s=2 で 100。残り 32。
		{"S の下降補正: 100 には SP 2 が要る(1 では 99 のまま)",
			offenseAllocInput(CategoryPhysical, natureAdjAtkUp, Stats{HP: 32, SpD: 2}, full, 100),
			Stats{HP: 32, Atk: 30, SpD: 2, Spe: 2}, true},
		{"S の下降補正: 99 は SP 0 で届く(同じ実数値なら SP を減らす)",
			offenseAllocInput(CategoryPhysical, natureAdjAtkUp, Stats{HP: 32, SpD: 2}, full, 99),
			Stats{HP: 32, Atk: 32, SpD: 2}, true},
		{"余りの 1 が S の実数値を上げないなら振らない(合計 65)",
			offenseAllocInput(CategoryPhysical, natureAdjAtkUp, Stats{HP: 32, Def: 1}, full, 0),
			Stats{HP: 32, Atk: 32, Def: 1}, true},
		{"特殊は C と S",
			offenseAllocInput(CategorySpecial, NatureNeutral, Stats{HP: 32}, full, 120),
			Stats{HP: 32, SpA: 24, Spe: 10}, true},
		{"A の上限 20 なら余りは S へ",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, Stats{Atk: 20, Spe: 32}, 0),
			Stats{Atk: 20, Spe: 32}, true},
		// S = 110 + 2 = 112 < 120。
		{"残り 0 で素早さが届かない: 下限の組で SpeedMet=false",
			offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32, Def: 32, Spe: 2}, full, 120),
			Stats{HP: 32, Def: 32, Spe: 2}, false},
	}
}

// TestAdjustAllocExpectationsMatchOracle は手計算の期待値が総当たりと一致することを確かめる(fixture の前提)。
// 実装の前から通る。これが落ちたら境界テストの失敗は実装ではなく期待値の誤り。
func TestAdjustAllocExpectationsMatchOracle(t *testing.T) {
	for _, tt := range append(allocBulkMaxCases(), allocOffenseMaxCases()...) {
		t.Run(tt.name, func(t *testing.T) {
			want := allocOracle(t, tt.in).MaxIndex
			if want.SP != tt.wantSP || want.SpeedMet != tt.wantSpeedMet {
				t.Errorf("oracle SP=%+v SpeedMet=%v, 手計算 %+v %v", want.SP, want.SpeedMet, tt.wantSP, tt.wantSpeedMet)
			}
		})
	}
}

// TestAdjustAllocFixturePremises は境界の前提(性格の下降補正で実数値が上がらない段・素早さの上限)を確かめる。
func TestAdjustAllocFixturePremises(t *testing.T) {
	real := func(species Species, n Nature, sp Stats) Stats {
		return RealStats(Individual{Species: species, Level: DefaultLevel, Nature: n, SP: sp})
	}
	if a, b, c := real(adjFoeSpecies(), natureAllocDefDown, Stats{}).Def,
		real(adjFoeSpecies(), natureAllocDefDown, Stats{Def: 1}).Def,
		real(adjFoeSpecies(), natureAllocDefDown, Stats{Def: 2}).Def; a != 99 || b != 99 || c != 100 {
		t.Errorf("B↓ の B 実数値 = %d, %d, %d want 99, 99, 100", a, b, c)
	}
	if a, b, c := real(adjSelfAttackerSpecies(), natureAdjAtkUp, Stats{}).Spe,
		real(adjSelfAttackerSpecies(), natureAdjAtkUp, Stats{Spe: 1}).Spe,
		real(adjSelfAttackerSpecies(), natureAdjAtkUp, Stats{Spe: 2}).Spe; a != 99 || b != 99 || c != 100 {
		t.Errorf("S↓ の S 実数値 = %d, %d, %d want 99, 99, 100", a, b, c)
	}
	if s := real(adjSelfAttackerSpecies(), NatureNeutral, Stats{Spe: MaxSPPerStat}).Spe; s != 142 {
		t.Errorf("無補正 S32 の S 実数値 = %d want 142", s)
	}
}

// TestAdjustAllocMaxIndexBoundaries は指数最大の組の境界(残り 0・合計ちょうど 66・上限・性格の段・
// 素早さ目標の届く/届かない/ちょうど・辞書順の同点・下限の尊重)を確かめる。
func TestAdjustAllocMaxIndexBoundaries(t *testing.T) {
	for _, tt := range append(allocBulkMaxCases(), allocOffenseMaxCases()...) {
		t.Run(tt.name, func(t *testing.T) {
			got := mustSuggest(t, tt.in)
			if got.MaxIndex.SP != tt.wantSP || got.MaxIndex.SpeedMet != tt.wantSpeedMet {
				t.Errorf("MaxIndex SP=%+v SpeedMet=%v want %+v %v", got.MaxIndex.SP, got.MaxIndex.SpeedMet, tt.wantSP, tt.wantSpeedMet)
			}
			if got.MinSP != nil {
				t.Errorf("目標なしで MinSP=%+v want nil", *got.MinSP)
			}
			if got.Unsupported != nil {
				t.Errorf("目標なしで Unsupported=%+v want nil", got.Unsupported)
			}
			assertAllocPlanBounds(t, "MaxIndex", tt.in, got.MaxIndex)
			assertAllocResult(t, got, allocOracle(t, tt.in))
		})
	}
}

// TestAdjustAllocTotalExactly66 は「余りを使い切れるなら合計ちょうど 66」を明示する(ADR-0150 §8)。
func TestAdjustAllocTotalExactly66(t *testing.T) {
	ins := []SPAllocInput{
		bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Spe: 32}, allocFullCeiling),
		bulkAllocInput(BulkFocusSpecial, NatureNeutral, Stats{Atk: 10}, allocFullCeiling),
		bulkAllocInput(BulkFocusBoth, NatureNeutral, Stats{}, allocFullCeiling),
		offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 2}, allocFullCeiling, 0),
		offenseAllocInput(CategorySpecial, NatureNeutral, Stats{SpD: 20}, allocFullCeiling, 130),
	}
	for _, in := range ins {
		got := mustSuggest(t, in)
		if got.MaxIndex.TotalSP != MaxSPTotal || got.MaxIndex.SP.Sum() != MaxSPTotal {
			t.Errorf("%s/%s 下限 %+v: TotalSP=%d want %d", in.Mode, in.Focus, in.Self.SP, got.MaxIndex.TotalSP, MaxSPTotal)
		}
	}
}

// ---------------------------------------------------------------------------
// 最小 SP の組(目標あり)
// ---------------------------------------------------------------------------

// TestAdjustAllocMinSPBoundaries は目標を満たす最小 SP の組の境界と、指数最大の組に付く目標の判定を確かめる。
func TestAdjustAllocMinSPBoundaries(t *testing.T) {
	full := allocFullCeiling
	tests := []struct {
		name   string
		in     func(t *testing.T) SPAllocInput
		assert func(t *testing.T, got SPAllocResult)
	}{
		{"耐久: 下限で既に耐えるなら下限の組(下限は最小 SP にも効く)",
			func(t *testing.T) SPAllocInput {
				return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{HP: 20, Spe: 32}, full),
					bulkAllocGoal(t, CategoryPhysical, 40, 1, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.SP != (Stats{HP: 20, Spe: 32}) || !got.MinSP.GoalMet || got.MinSP.ChancePercent != 100 {
					t.Errorf("MinSP=%+v want 下限の組・確定で耐える", *got.MinSP)
				}
			}},
		{"耐久: 物理の目標に D は振らない",
			func(t *testing.T) SPAllocInput {
				return withGoal(bulkAllocInput(BulkFocusBoth, NatureNeutral, Stats{}, full),
					bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if !got.MinSP.GoalMet || got.MinSP.SP.SpD != 0 || got.MinSP.TotalSP == 0 {
					t.Errorf("MinSP=%+v want 耐える・D=0・合計 > 0", *got.MinSP)
				}
			}},
		{"耐久: 特殊の目標に B は振らない",
			func(t *testing.T) SPAllocInput {
				return withGoal(bulkAllocInput(BulkFocusBoth, NatureNeutral, Stats{}, full),
					bulkAllocGoal(t, CategorySpecial, 200, 1, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.SP.Def != 0 {
					t.Errorf("MinSP=%+v want B=0", *got.MinSP)
				}
			}},
		{"耐久: 届かない(急所・威力250)ならエラーにせず GoalMet=false",
			func(t *testing.T) SPAllocInput {
				goal := bulkAllocGoal(t, CategoryPhysical, 250, 1, 0)
				goal.Critical = true
				return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, full), goal)
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.GoalMet || got.MaxIndex.GoalMet {
					t.Errorf("MinSP=%+v MaxIndex=%+v want GoalMet=false", *got.MinSP, got.MaxIndex)
				}
			}},
		{"耐久: 残り 2 では届かない(代わりに耐える確率が最大の組)",
			func(t *testing.T) SPAllocInput {
				return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Atk: 32, Spe: 32}, full),
					bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
			},
			func(t *testing.T, got SPAllocResult) {}},
		{"耐久: 乱数で耐える(しきい値 50%)",
			func(t *testing.T) SPAllocInput {
				return withGoal(bulkAllocInput(BulkFocusPhysical, natureAllocDefDown, Stats{Spe: 32}, full),
					bulkAllocGoal(t, CategoryPhysical, 200, 1, 50))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.GoalMet && got.MinSP.ChancePercent < 50 {
					t.Errorf("確率 %v%% がしきい値 50%% 未満", got.MinSP.ChancePercent)
				}
			}},
		// S = 110 + s ≥ 120 → s=10 が最小。A は確定3発に届く最小。
		{"攻撃: 素早さと倒す目標の両方を満たす最小",
			func(t *testing.T) SPAllocInput {
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, full, 120),
					offenseAllocGoal(t, CategoryPhysical, 100, 3, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if !got.MinSP.GoalMet || !got.MinSP.SpeedMet || got.MinSP.SP.Spe != 10 {
					t.Errorf("MinSP=%+v want 両方満たす・S=10", *got.MinSP)
				}
			}},
		{"攻撃: 素早さが届かないなら SpeedMet=false(S は最大へ寄せる)",
			func(t *testing.T) SPAllocInput {
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, full, 143),
					offenseAllocGoal(t, CategoryPhysical, 100, 3, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.SpeedMet || got.MinSP.SP.Spe != MaxSPPerStat {
					t.Errorf("MinSP=%+v want SpeedMet=false・S=32", *got.MinSP)
				}
			}},
		{"攻撃: 倒せない(威力20・1発)なら GoalMet=false",
			func(t *testing.T) SPAllocInput {
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, full, 0),
					offenseAllocGoal(t, CategoryPhysical, 20, 1, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.GoalMet {
					t.Errorf("MinSP=%+v want GoalMet=false", *got.MinSP)
				}
			}},
		{"攻撃: 下限が目標より多いなら下限の組",
			func(t *testing.T) SPAllocInput {
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{Atk: 32, Spe: 32}, full, 0),
					offenseAllocGoal(t, CategoryPhysical, 150, 2, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.SP != (Stats{Atk: 32, Spe: 32}) || !got.MinSP.GoalMet {
					t.Errorf("MinSP=%+v want 下限の組で満たす", *got.MinSP)
				}
			}},
		{"攻撃: 特殊は C を探す",
			func(t *testing.T) SPAllocInput {
				return withGoal(offenseAllocInput(CategorySpecial, natureAdjSpAUp, Stats{HP: 32}, full, 0),
					offenseAllocGoal(t, CategorySpecial, 100, 3, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.SP.Atk != 0 {
					t.Errorf("MinSP=%+v want A=0", *got.MinSP)
				}
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			got := mustSuggest(t, in)
			if got.MinSP == nil {
				t.Fatal("目標ありで MinSP=nil")
			}
			assertAllocPlanBounds(t, "MaxIndex", in, got.MaxIndex)
			assertAllocPlanBounds(t, "MinSP", in, *got.MinSP)
			if got.MinSP.TotalSP > got.MaxIndex.TotalSP && got.MinSP.GoalMet && got.MaxIndex.GoalMet && got.MinSP.SpeedMet && got.MaxIndex.SpeedMet {
				t.Errorf("最小 SP の組(%d)が指数最大の組(%d)より多い", got.MinSP.TotalSP, got.MaxIndex.TotalSP)
			}
			assertAllocResult(t, got, allocOracle(t, in))
			tt.assert(t, got)
		})
	}
}

// ---------------------------------------------------------------------------
// 最適性の性質テスト(独立の総当たりと全件照合)
// ---------------------------------------------------------------------------

// allocCeilingFor は下限以上に揃えた上限を作る(回す能力の上限 < 下限は不正入力なので避ける)。
func allocCeilingFor(floor, ceiling Stats) Stats {
	out := ceiling
	for _, k := range AllStatKeys() {
		out = out.WithStat(k, max(floor.Get(k), ceiling.Get(k)))
	}
	return out
}

// TestAdjustAllocBulkMaxIndexMatchesBruteForce は耐久側の指数最大の組が総当たりと全件一致することを確かめる。
func TestAdjustAllocBulkMaxIndexMatchesBruteForce(t *testing.T) {
	floors := []Stats{{}, {Spe: 32}, {HP: 20, Spe: 32}, {Atk: 32, SpA: 32}, {HP: 32, Def: 32, Spe: 2}, {Def: 10, SpD: 5, Spe: 20}, {Spe: 31}}
	ceilings := []Stats{allocFullCeiling, {HP: 15, Def: 32, SpD: 32}, {HP: 32, Def: 7}, {Def: 32, SpD: 32}}
	natures := []Nature{NatureNeutral, natureAdjDefUp, natureAllocDefDown, natureAllocSpDDown}
	for _, focus := range []BulkFocus{BulkFocusPhysical, BulkFocusSpecial, BulkFocusBoth} {
		for _, n := range natures {
			for _, floor := range floors {
				for _, c := range ceilings {
					in := bulkAllocInput(focus, n, floor, allocCeilingFor(floor, c))
					got := mustSuggest(t, in)
					assertAllocPlanBounds(t, "MaxIndex", in, got.MaxIndex)
					assertAllocResult(t, got, allocOracle(t, in))
				}
			}
		}
	}
}

// TestAdjustAllocOffenseMaxIndexMatchesBruteForce は攻撃側の指数最大の組が総当たりと全件一致することを確かめる。
func TestAdjustAllocOffenseMaxIndexMatchesBruteForce(t *testing.T) {
	floors := []Stats{{}, {HP: 32}, {HP: 32, SpD: 2}, {HP: 32, Def: 1}, {HP: 20, Def: 20}, {Atk: 10, SpA: 10, Spe: 5}}
	ceilings := []Stats{allocFullCeiling, {Atk: 20, SpA: 20, Spe: 32}, {Atk: 32, SpA: 32, Spe: 8}}
	natures := []Nature{NatureNeutral, natureAdjAtkUp, natureAdjSpAUp, natureAllocSpeUp}
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		for _, n := range natures {
			for _, floor := range floors {
				for _, c := range ceilings {
					for _, minSpeed := range []int{0, 99, 100, 120, 142, 143, 160} {
						in := offenseAllocInput(cat, n, floor, allocCeilingFor(floor, c), minSpeed)
						got := mustSuggest(t, in)
						assertAllocPlanBounds(t, "MaxIndex", in, got.MaxIndex)
						assertAllocResult(t, got, allocOracle(t, in))
					}
				}
			}
		}
	}
}

// TestAdjustAllocBulkMinSPMatchesBruteForce は耐久側の最小 SP の組が総当たりと全件一致することを確かめる。
// D(物理)/ B(特殊)の上限を下限に揃えて 2 能力に絞った表と、3 能力すべてを回す1件で照合する。
func TestAdjustAllocBulkMinSPMatchesBruteForce(t *testing.T) {
	floors := []Stats{{}, {Spe: 32}, {HP: 20, Spe: 32}}
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		ceiling := Stats{HP: 32, Def: 32}
		if cat == CategorySpecial {
			ceiling = Stats{HP: 32, SpD: 32}
		}
		for _, power := range []int{40, 120, 200} {
			for _, hits := range []int{1, 2} {
				for _, threshold := range []float64{0, 50} {
					for _, n := range []Nature{NatureNeutral, natureAdjDefUp} {
						for _, floor := range floors {
							in := withGoal(bulkAllocInput(BulkFocusBoth, n, floor, allocCeilingFor(floor, ceiling)),
								bulkAllocGoal(t, cat, power, hits, threshold))
							got := mustSuggest(t, in)
							if got.MinSP == nil {
								t.Fatalf("目標ありで MinSP=nil: %+v", in)
							}
							assertAllocPlanBounds(t, "MinSP", in, *got.MinSP)
							assertAllocResult(t, got, allocOracle(t, in))
						}
					}
				}
			}
		}
	}
	t.Run("H・B・D の3能力すべてを回す", func(t *testing.T) {
		in := withGoal(bulkAllocInput(BulkFocusBoth, NatureNeutral, Stats{}, allocFullCeiling),
			bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
		assertAllocResult(t, mustSuggest(t, in), allocOracle(t, in))
	})
}

// TestAdjustAllocOffenseMinSPMatchesBruteForce は攻撃側の最小 SP の組が総当たりと全件一致することを確かめる。
func TestAdjustAllocOffenseMinSPMatchesBruteForce(t *testing.T) {
	floors := []Stats{{}, {HP: 32}, {HP: 32, Def: 20}}
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		for _, power := range []int{60, 100, 150} {
			for _, hits := range []int{1, 2} {
				for _, threshold := range []float64{0, 50} {
					for _, minSpeed := range []int{0, 120, 143} {
						for _, floor := range floors {
							in := withGoal(offenseAllocInput(cat, NatureNeutral, floor, allocFullCeiling, minSpeed),
								offenseAllocGoal(t, cat, power, hits, threshold))
							got := mustSuggest(t, in)
							if got.MinSP == nil {
								t.Fatalf("目標ありで MinSP=nil: %+v", in)
							}
							assertAllocPlanBounds(t, "MinSP", in, *got.MinSP)
							assertAllocResult(t, got, allocOracle(t, in))
						}
					}
				}
			}
		}
	}
	t.Run("確定3発", func(t *testing.T) {
		in := withGoal(offenseAllocInput(CategoryPhysical, natureAdjAtkUp, Stats{HP: 32}, allocFullCeiling, 100),
			offenseAllocGoal(t, CategoryPhysical, 100, 3, 0))
		assertAllocResult(t, mustSuggest(t, in), allocOracle(t, in))
	})
}

// TestAdjustAllocSpecialCasesMatchBruteForce は無効相性・ランク ±6・下降補正の性格の最小 SP を総当たりと照合する。
func TestAdjustAllocSpecialCasesMatchBruteForce(t *testing.T) {
	tests := []struct {
		name   string
		in     func(t *testing.T) SPAllocInput
		assert func(t *testing.T, got SPAllocResult)
	}{
		{"攻撃: 無効相性(ゴースト)は倒せない",
			func(t *testing.T) SPAllocInput {
				g := offenseAllocGoal(t, CategoryPhysical, 150, 3, 0)
				g.Opponent.Species = adjGhostSpecies()
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, allocFullCeiling, 0), g)
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.GoalMet || got.MaxIndex.GoalMet || got.MinSP.ChancePercent != 0 {
					t.Errorf("MinSP=%+v MaxIndex=%+v want 倒せない(GoalMet=false・確率 0)", *got.MinSP, got.MaxIndex)
				}
			}},
		{"耐久: 無効相性(ゴースト)は必ず耐える",
			func(t *testing.T) SPAllocInput {
				in := bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, allocFullCeiling)
				in.Self.Species = adjGhostSpecies()
				return withGoal(in, bulkAllocGoal(t, CategoryPhysical, 250, 1, 100))
			},
			func(t *testing.T, got SPAllocResult) {
				if !got.MinSP.GoalMet || got.MinSP.TotalSP != 0 || got.MinSP.ChancePercent != 100 {
					t.Errorf("MinSP=%+v want SP 0 で確定で耐える", *got.MinSP)
				}
			}},
		{"攻撃: 自分の A +6",
			func(t *testing.T) SPAllocInput {
				in := offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, allocFullCeiling, 0)
				in.Self.Ranks.Atk = 6
				return withGoal(in, offenseAllocGoal(t, CategoryPhysical, 100, 1, 0))
			}, nil},
		{"攻撃: 自分の A -6",
			func(t *testing.T) SPAllocInput {
				in := offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, allocFullCeiling, 0)
				in.Self.Ranks.Atk = -6
				return withGoal(in, offenseAllocGoal(t, CategoryPhysical, 150, 3, 0))
			}, nil},
		{"攻撃: 相手の B +6",
			func(t *testing.T) SPAllocInput {
				g := offenseAllocGoal(t, CategoryPhysical, 150, 3, 0)
				g.Opponent.Ranks.Def = 6
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, allocFullCeiling, 0), g)
			}, nil},
		{"攻撃: 相手の B -6",
			func(t *testing.T) SPAllocInput {
				g := offenseAllocGoal(t, CategoryPhysical, 40, 1, 0)
				g.Opponent.Ranks.Def = -6
				return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, allocFullCeiling, 0), g)
			}, nil},
		{"耐久: 自分の B +6",
			func(t *testing.T) SPAllocInput {
				in := bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, allocFullCeiling)
				in.Self.Ranks.Def = 6
				return withGoal(in, bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
			}, nil},
		{"耐久: 自分の B -6",
			func(t *testing.T) SPAllocInput {
				in := bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, allocFullCeiling)
				in.Self.Ranks.Def = -6
				return withGoal(in, bulkAllocGoal(t, CategoryPhysical, 60, 1, 0))
			}, nil},
		{"耐久: 相手の A +6",
			func(t *testing.T) SPAllocInput {
				g := bulkAllocGoal(t, CategoryPhysical, 80, 1, 0)
				g.Opponent.Ranks.Atk = 6
				return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, allocFullCeiling), g)
			}, nil},
		{"耐久: 相手の A -6",
			func(t *testing.T) SPAllocInput {
				g := bulkAllocGoal(t, CategoryPhysical, 250, 1, 0)
				g.Opponent.Ranks.Atk = -6
				return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, allocFullCeiling), g)
			}, nil},
		{"耐久: 下降補正の性格(B↓)の最小 SP",
			func(t *testing.T) SPAllocInput {
				return withGoal(bulkAllocInput(BulkFocusPhysical, natureAllocDefDown, Stats{Spe: 32}, allocFullCeiling),
					bulkAllocGoal(t, CategoryPhysical, 120, 1, 0))
			}, nil},
		{"攻撃: 下降補正の性格(S↓)の最小 SP(素早さ 100 は SP 2 が要る)",
			func(t *testing.T) SPAllocInput {
				return withGoal(offenseAllocInput(CategoryPhysical, natureAdjAtkUp, Stats{HP: 32}, allocFullCeiling, 100),
					offenseAllocGoal(t, CategoryPhysical, 100, 3, 0))
			},
			func(t *testing.T, got SPAllocResult) {
				if got.MinSP.SpeedMet && got.MinSP.SP.Spe < 2 {
					t.Errorf("MinSP=%+v want 素早さ 100 を満たすなら S>=2", *got.MinSP)
				}
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			got := mustSuggest(t, in)
			if got.MinSP == nil {
				t.Fatal("目標ありで MinSP=nil")
			}
			assertAllocPlanBounds(t, "MaxIndex", in, got.MaxIndex)
			assertAllocPlanBounds(t, "MinSP", in, *got.MinSP)
			assertAllocResult(t, got, allocOracle(t, in))
			if tt.assert != nil {
				tt.assert(t, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 不正入力・値域の端・相性表・未対応の印
// ---------------------------------------------------------------------------

// TestAdjustAllocRejectsInvalidInput は不正入力を ErrInvalidAdjustInput で拒否することを確かめる。
func TestAdjustAllocRejectsInvalidInput(t *testing.T) {
	full := allocFullCeiling
	bulk := func(t *testing.T) SPAllocInput {
		return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, full), bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
	}
	offense := func(t *testing.T) SPAllocInput {
		return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, full, 0), offenseAllocGoal(t, CategoryPhysical, 100, 3, 0))
	}
	tests := []struct {
		name string
		base func(t *testing.T) SPAllocInput
		f    func(in *SPAllocInput)
	}{
		{"Mode が空", bulk, func(in *SPAllocInput) { in.Mode = "" }},
		{"Mode が未知", bulk, func(in *SPAllocInput) { in.Mode = "speed" }},
		{"耐久側の Focus が空", bulk, func(in *SPAllocInput) { in.Focus = "" }},
		{"耐久側の Focus が未知", bulk, func(in *SPAllocInput) { in.Focus = "total" }},
		{"耐久側で MinSpeed が 0 以外(素早さは見ない)", bulk, func(in *SPAllocInput) { in.MinSpeed = 100 }},
		{"攻撃側の分類が変化技", offense, func(in *SPAllocInput) { in.OffenseCategory = CategoryStatus }},
		{"攻撃側の分類が空", offense, func(in *SPAllocInput) { in.OffenseCategory = "" }},
		{"攻撃側の MinSpeed が負", offense, func(in *SPAllocInput) { in.MinSpeed = -1 }},
		{"下限の合計が 66 超過", bulk, func(in *SPAllocInput) { in.Self.SP = Stats{Atk: 32, SpA: 32, Spe: 3} }},
		{"下限の1能力が 32 超過", bulk, func(in *SPAllocInput) { in.Self.SP = Stats{Spe: MaxSPPerStat + 1} }},
		{"下限が負", bulk, func(in *SPAllocInput) { in.Self.SP = Stats{HP: -1} }},
		{"自分のレベルが 50 以外", bulk, func(in *SPAllocInput) { in.Self.Level = 49 }},
		{"耐久側: H の上限が下限未満", bulk, func(in *SPAllocInput) {
			in.Self.SP = Stats{HP: 20}
			in.Ceiling = full.WithStat(StatHP, 10)
		}},
		{"耐久側: D の上限が 32 超過", bulk, func(in *SPAllocInput) { in.Ceiling = full.WithStat(StatSpD, MaxSPPerStat+1) }},
		{"耐久側: B の上限が負", bulk, func(in *SPAllocInput) { in.Ceiling = full.WithStat(StatDef, -1) }},
		{"攻撃側: A の上限が下限未満", offense, func(in *SPAllocInput) {
			in.Self.SP = Stats{Atk: 20}
			in.Ceiling = full.WithStat(StatAtk, 19)
		}},
		{"攻撃側: S の上限が 32 超過", offense, func(in *SPAllocInput) { in.Ceiling = full.WithStat(StatSpe, MaxSPPerStat+1) }},
		{"攻撃側: 技の分類が OffenseCategory と違う", offense, func(in *SPAllocInput) { in.Goal.Move.Category = CategorySpecial }},
		{"目標の Hits 0", bulk, func(in *SPAllocInput) { in.Goal.Hits = 0 }},
		{"目標の Hits が上限超過", offense, func(in *SPAllocInput) { in.Goal.Hits = MaxAdjustHits + 1 }},
		{"目標のしきい値が負", bulk, func(in *SPAllocInput) { in.Goal.ThresholdPercent = -1 }},
		{"目標のしきい値が 100 超過", offense, func(in *SPAllocInput) { in.Goal.ThresholdPercent = 100.5 }},
		{"目標のしきい値が NaN", bulk, func(in *SPAllocInput) { in.Goal.ThresholdPercent = math.NaN() }},
		{"目標のしきい値が +Inf", offense, func(in *SPAllocInput) { in.Goal.ThresholdPercent = math.Inf(1) }},
		{"目標の技が変化技", bulk, func(in *SPAllocInput) { in.Goal.Move.Category = CategoryStatus; in.Goal.Move.Power = 0 }},
		{"目標の技の威力 0", offense, func(in *SPAllocInput) { in.Goal.Move.Power = 0 }},
		{"相手のランクが範囲外", bulk, func(in *SPAllocInput) { in.Goal.Opponent.Ranks.Atk = 7 }},
		{"相手の SP 合計が 66 超過", offense, func(in *SPAllocInput) { in.Goal.Opponent.SP = Stats{HP: 32, Def: 32, SpD: 3} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.base(t)
			tt.f(&in)
			if _, err := SuggestSPAllocation(in); !errors.Is(err, ErrInvalidAdjustInput) {
				t.Errorf("err=%v want ErrInvalidAdjustInput", err)
			}
		})
	}
}

// TestAdjustAllocAcceptsEdgeInput は値域の端と「回さない能力の上限は見ない」を受け付けることを確かめる。
func TestAdjustAllocAcceptsEdgeInput(t *testing.T) {
	tests := []struct {
		name string
		in   func(t *testing.T) SPAllocInput
	}{
		{"耐久側: A32 を固定し A の上限は 0 のまま(回さない能力の上限は見ない)", func(t *testing.T) SPAllocInput {
			return bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Atk: 32}, Stats{HP: 32, Def: 32, SpD: 32})
		}},
		{"攻撃側: H32 を固定し H の上限は 0 のまま", func(t *testing.T) SPAllocInput {
			return offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, Stats{Atk: 32, Spe: 32}, 0)
		}},
		{"攻撃側: 物理なら C の上限は見ない", func(t *testing.T) SPAllocInput {
			return offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{SpA: 10}, Stats{Atk: 32, Spe: 32}, 0)
		}},
		{"目標の Hits = MaxAdjustHits", func(t *testing.T) SPAllocInput {
			return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, allocFullCeiling, 0),
				offenseAllocGoal(t, CategoryPhysical, 100, MaxAdjustHits, 0))
		}},
		{"目標のしきい値 100 を明示", func(t *testing.T) SPAllocInput {
			return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, Stats{HP: 32, Def: 32}),
				bulkAllocGoal(t, CategoryPhysical, 200, 1, 100))
		}},
		{"攻撃側: 素早さの目標がとても大きい(届かないだけでエラーではない)", func(t *testing.T) SPAllocInput {
			return offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, allocFullCeiling, 1000)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SuggestSPAllocation(tt.in(t)); err != nil {
				t.Errorf("err=%v want nil", err)
			}
		})
	}
}

// TestAdjustAllocRequiresTypeChart は目標があるのに相性表が無ければ ErrTypeChartMissing を返すことを確かめる。
func TestAdjustAllocRequiresTypeChart(t *testing.T) {
	bulk := withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{}, allocFullCeiling), bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
	bulk.Goal.TypeChart = TypeChart{}
	if _, err := SuggestSPAllocation(bulk); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("耐久側 err=%v want ErrTypeChartMissing", err)
	}
	offense := withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{}, allocFullCeiling, 0), offenseAllocGoal(t, CategoryPhysical, 100, 3, 0))
	offense.Goal.TypeChart = TypeChart{}
	if _, err := SuggestSPAllocation(offense); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("攻撃側 err=%v want ErrTypeChartMissing", err)
	}
}

// TestAdjustAllocCarriesUnsupportedMarks は目標の CalcDamage の「未対応」の印が結果に付き、
// 数値は印のない技の場合と同じことを確かめる(ADR-0150 §8・ADR-0123)。
func TestAdjustAllocCarriesUnsupportedMarks(t *testing.T) {
	cases := []struct {
		name string
		in   func(t *testing.T) SPAllocInput
	}{
		{"耐久側", func(t *testing.T) SPAllocInput {
			return withGoal(bulkAllocInput(BulkFocusPhysical, NatureNeutral, Stats{Spe: 32}, Stats{HP: 32, Def: 32}),
				bulkAllocGoal(t, CategoryPhysical, 200, 1, 0))
		}},
		{"攻撃側", func(t *testing.T) SPAllocInput {
			return withGoal(offenseAllocInput(CategoryPhysical, NatureNeutral, Stats{HP: 32}, allocFullCeiling, 120),
				offenseAllocGoal(t, CategoryPhysical, 100, 2, 0))
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			plain := tt.in(t)
			want := mustSuggest(t, plain)
			if want.Unsupported != nil {
				t.Errorf("印なしの技に印: %+v", want.Unsupported)
			}

			marked := tt.in(t)
			marked.Goal.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
			attacker, defender := marked.Goal.Opponent, marked.Self
			if marked.Mode == AllocModeOffense {
				attacker, defender = marked.Self, marked.Goal.Opponent
			}
			ref, err := CalcDamage(DamageInput{
				Format: marked.Goal.Format, Attacker: attacker, Defender: defender, Move: marked.Goal.Move,
				TypeChart: marked.Goal.TypeChart,
			})
			if err != nil {
				t.Fatalf("CalcDamage: %v", err)
			}
			if len(ref.Unsupported) == 0 {
				t.Fatal("前提が崩れた: 複数回当たる技に印が付かない")
			}
			got := mustSuggest(t, marked)
			if !reflect.DeepEqual(got.Unsupported, ref.Unsupported) {
				t.Errorf("Unsupported=%+v want %+v", got.Unsupported, ref.Unsupported)
			}
			assertAllocResult(t, got, want)
		})
	}
}
