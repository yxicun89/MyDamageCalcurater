package engine

// 逆算(P1-12 で再設計)の受け入れ条件をテストで固定する。ADR-0010(P1-12 改訂 §R)が定義の正。
//
// 仕様(ユーザー決定。docs/requirements.md §2「調整の推定(逆算)」、DECISIONS.md 2026-09-21 (5)):
//   - 固定プリセット/型からの選択はしない。防御側は H32(HP SP=32)を前提に B(D) の SP を 0..32 探索、
//     攻撃側は A(C) の SP を 0..32 探索。性格は「補正なし」「関連ステータス上昇」の2通り。
//   - 結果は「性格クラス × 持ち物」ごとに、観測を説明できる SP の範囲。区別できない候補は残す。
//   - 同じ相手の観測を複数入力すると絞り込める。
//   - 観測%の丸め規則は未確認(人間の確認待ち)なので、観測は「精度付きの値」として受け、
//     どの丸め規則(切り捨て/四捨五入/切り上げ)で作られた観測でも真値を落とさない区間で照合する。
//
// テストの「正解」は engine の関数からではなく、観測を作った手順そのもの(roundObserved)から
// 独立に導く(oracleMatches / bruteForceReverse)。engine の区間式を写さない。
//
// Recall の合格基準そのものは engine/reverse_recall_test.go(allspecies タグ)にある。
// 本ファイルの TestReverseRecallSmoke は小標本の早期検知。

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

// revDefenderSpecies は逆算される側の試験用種族(HP100 / B80 / D80 のエスパー単)。
func revDefenderSpecies() Species {
	return Species{
		Key:       "0997-000",
		NameJa:    "テストぎゃくさんぼう",
		Types:     []Type{TypePsychic},
		BaseStats: Stats{HP: 100, Atk: 50, Def: 80, SpA: 50, SpD: 80, Spe: 50},
	}
}

// revAttackerSpecies は攻撃側の試験用種族。
func revAttackerSpecies() Species {
	return Species{
		Key:       "0996-000",
		NameJa:    "テストぎゃくさんこう",
		Types:     []Type{TypeWater},
		BaseStats: Stats{HP: 100, Atk: 120, Def: 70, SpA: 120, SpD: 70, Spe: 100},
	}
}

// revFrailSpecies は HP バーが 100% に張り付く(確定1発になる)試験用の脆い種族。
func revFrailSpecies() Species {
	return Species{
		Key:       "0994-000",
		NameJa:    "テストもろい",
		Types:     []Type{TypeNormal},
		BaseStats: Stats{HP: 40, Atk: 40, Def: 30, SpA: 40, SpD: 30, Spe: 40},
	}
}

// revKnownAttacker は既知側(自分)の攻撃個体。A/C に振らない素の個体。
func revKnownAttacker() Individual {
	return Individual{
		Species: revAttackerSpecies(),
		Level:   DefaultLevel,
		Nature:  NatureNeutral,
		Status:  StatusNone,
	}
}

// revStrongAttacker は A32・A上昇の既知側攻撃個体(ダメージが大きく、ロールの間隔が 2 以上に開く)。
func revStrongAttacker() Individual {
	return Individual{
		Species: revAttackerSpecies(),
		Level:   DefaultLevel,
		Nature:  Nature{Plus: StatAtk, Minus: StatSpA},
		SP:      Stats{Atk: MaxSPPerStat},
		Status:  StatusNone,
	}
}

// revMove は威力100の水技(防御側エスパー単に対して等倍)。
func revMove(cat MoveCategory) Move {
	return Move{ID: "testmove", NameJa: "テストわざ", Type: TypeWater, Category: cat, Power: 100}
}

// revEviolite / revVest は持ち物候補の試験用(効果はマスタから解決済みの体で直接与える)。
func revEviolite() *Item {
	return &Item{ID: "eviolite", NameJa: "テストもちもの1",
		Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: 6144, StatSpD: 6144}}}
}

func revVest() *Item {
	return &Item{ID: "assaultvest", NameJa: "テストもちもの2",
		Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
}

// revBerry は半減きのみの試験用。t タイプの抜群技だけを半減する。等倍の技には効かないので、
// 「持ち物なし」と観測上まったく区別できない候補を作るのに使う。
func revBerry(t Type) *Item {
	return &Item{ID: "berry-" + string(t), NameJa: "テストきのみ",
		Effect: &ItemEffect{ResistBerryType: t}}
}

// revDefender は逆算対象の防御側個体を組み立てる。
func revDefender(sp Stats, n Nature, item *Item) Individual {
	return Individual{
		Species: revDefenderSpecies(),
		Level:   DefaultLevel,
		Nature:  n,
		SP:      sp,
		Item:    item,
		Status:  StatusNone,
	}
}

// revBigHPSpecies は HP が大きい既知側の試験用種族。
// 点数差 1〜2 が「差*1000/HP」でも小さくなることを利用して、距離の下限 1 が効いているかを検出する。
func revBigHPSpecies() Species {
	return Species{
		Key:       "0995-000",
		NameJa:    "テストだいたいりょく",
		Types:     []Type{TypeNormal},
		BaseStats: Stats{HP: 255, Atk: 10, Def: 60, SpA: 10, SpD: 60, Spe: 55},
	}
}

// ---------------------------------------------------------------------------
// 観測の作り方(テスト側の独立な定義)
//
// 実機の丸め規則は未確認(plan.md ブロッカー)。テストは3つの規則のどれで観測を作っても
// engine が真値を落とさないことを見る。engine の区間式(ADR-0010 §R2)はここに写さない。
// ---------------------------------------------------------------------------

type obsRounding int

const (
	roundFloor obsRounding = iota
	roundHalfUp
	roundCeil
)

var allObsRoundings = []obsRounding{roundFloor, roundHalfUp, roundCeil}

func (r obsRounding) String() string {
	switch r {
	case roundFloor:
		return "切り捨て"
	case roundHalfUp:
		return "四捨五入"
	}
	return "切り上げ"
}

// roundObserved は実機画面の観測値を作る。scale=100 は整数%、scale=1000 は 0.1% 単位。
// HP バーは 100% で頭打ちなので、scale を超えた値は scale にする。
func roundObserved(damage, maxHP, scale int, r obsRounding) int {
	var v int
	switch r {
	case roundFloor:
		v = scale * damage / maxHP
	case roundHalfUp:
		v = (2*scale*damage + maxHP) / (2 * maxHP)
	default:
		v = (scale*damage + maxHP - 1) / maxHP
	}
	return min(v, scale)
}

// oracleMatches は「観測 o が、ダメージ damage をいずれかの丸め規則で観測したものでありうるか」。
// engine の Observation.Matches と同値であるべき(ADR-0010 §R2)が、定義は独立に書く。
func oracleMatches(o Observation, damage, maxHP int) bool {
	scale, v := 0, 0
	switch {
	case o.Damage != 0:
		return damage == o.Damage
	case o.PercentTenths != 0:
		scale, v = 1000, o.PercentTenths
	default:
		scale, v = 100, o.Percent
	}
	if maxHP <= 0 {
		return false
	}
	for _, r := range allObsRoundings {
		if roundObserved(damage, maxHP, scale, r) == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 総当たりの正解(テスト側)。engine の CalcReverse と同じ候補を CalcDamage から直接組み立てる。
// ---------------------------------------------------------------------------

// revClasses は候補の性格クラス(定義順)。ADR-0010 §R1。
var revClasses = []NatureClass{NatureClassNeutral, NatureClassPlus}

// revStatFor は逆算する関連ステータス(ADR-0010 §2 と同じ規則をテスト側に書く)。
func revStatFor(side ReverseSide, cat MoveCategory) StatKey {
	switch {
	case side == SideAttacker && cat == CategorySpecial:
		return StatSpA
	case side == SideAttacker:
		return StatAtk
	case cat == CategorySpecial:
		return StatSpD
	}
	return StatDef
}

// revNatureFor は関連ステータスに対する性格クラスの代表 Nature(ADR-0010 §R1)。
// 上昇補正の相手(下降側)は、この計算で使われない atk / spa に置く。
func revNatureFor(stat StatKey, c NatureClass) Nature {
	if c != NatureClassPlus {
		return NatureNeutral
	}
	other := StatAtk
	if stat == StatAtk {
		other = StatSpA
	}
	return Nature{Plus: stat, Minus: other}
}

func revItems(in ReverseInput) []*Item {
	if len(in.ItemCandidates) == 0 {
		return []*Item{nil}
	}
	return in.ItemCandidates
}

func revItemID(it *Item) string {
	if it == nil {
		return ""
	}
	return it.ID
}

// revRolls は候補 (class, item) の SP=x での CalcDamage。防御側は H32 固定(ADR-0010 §R1)。
func revRolls(t *testing.T, in ReverseInput, class NatureClass, item *Item, x int) DamageResult {
	t.Helper()
	stat := revStatFor(in.Side, in.Move.Category)
	sp := Stats{}.WithStat(stat, x)
	if in.Side == SideDefender {
		sp.HP = MaxSPPerStat
	}
	unknown := Individual{
		Species: in.UnknownSpecies, Level: DefaultLevel, Nature: revNatureFor(stat, class),
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
	res, err := calcDamage(dmg)
	if err != nil {
		t.Fatalf("正解の CalcDamage(class=%s item=%q SP=%d): %v", class, revItemID(item), x, err)
	}
	return res
}

type revKey struct {
	class  NatureClass
	itemID string
}

// oracleCand は候補1件の正解。
type oracleCand struct {
	defIndex int   // 定義順(性格クラス → 持ち物添字)
	exact    []int // 全観測を説明できる SP(独立な oracleMatches による)
	mismatch int   // 最良の SP での距離の和(0 ⇔ exact が空でない)
	ranges   []int // 候補が返すべき SP 集合(exact が空でなければ exact、空なら距離最小の SP)
	support  int   // ranges の各 SP で、各観測を説明できるロールの延べ数
	minT     int   // ranges 全体での表示%の最小(切り捨て、0.1%)
	maxT     int   // ranges 全体での表示%の最大(四捨五入、0.1%)
}

// bruteForceReverse は ADR-0010 §R の定義どおりに、全候補の正解を総当たりで作る。
// 「説明できるか」は独立な oracleMatches で決める。完全一致が無い候補の距離だけは
// engine の Observation.Distance(TestObservationDistance で手計算の表に固定済み)を部品として使う。
func bruteForceReverse(t *testing.T, in ReverseInput) map[revKey]oracleCand {
	t.Helper()
	items := revItems(in)
	out := map[revKey]oracleCand{}
	for ci, class := range revClasses {
		for ii, item := range items {
			var rolls [MaxSPPerStat + 1]DamageResult
			dist := make([]int, MaxSPPerStat+1)
			var exact []int
			for x := 0; x <= MaxSPPerStat; x++ {
				res := revRolls(t, in, class, item, x)
				rolls[x] = res
				all := true
				for _, o := range in.Observations {
					best, hit := -1, false
					for _, r := range res.Rolls {
						if oracleMatches(o, r, res.DefenderHP) {
							hit = true
						}
						d := o.Distance(r, res.DefenderHP)
						if best < 0 || d < best {
							best = d
						}
					}
					if !hit {
						all = false
					}
					dist[x] += best
				}
				if all {
					exact = append(exact, x)
				}
			}
			mism := slices.Min(dist)
			if (mism == 0) != (len(exact) > 0) {
				t.Fatalf("正解の自己矛盾: 距離0 と 独立な照合 が食い違う(class=%s item=%q)。Observation.Distance を確認",
					class, revItemID(item))
			}
			oc := oracleCand{defIndex: ci*len(items) + ii, exact: exact, mismatch: mism}
			if len(exact) > 0 {
				oc.ranges = exact
			} else {
				for x, d := range dist {
					if d == mism {
						oc.ranges = append(oc.ranges, x)
					}
				}
			}
			oc.minT, oc.maxT = -1, -1
			for _, x := range oc.ranges {
				res := rolls[x]
				for _, o := range in.Observations {
					for _, r := range res.Rolls {
						if oracleMatches(o, r, res.DefenderHP) {
							oc.support++
						}
					}
				}
				lo, hi := res.DisplayPercentRangeTenths()
				if oc.minT < 0 || lo < oc.minT {
					oc.minT = lo
				}
				if hi > oc.maxT {
					oc.maxT = hi
				}
			}
			out[revKey{class, revItemID(item)}] = oc
		}
	}
	return out
}

// spsOf は範囲の列を SP の昇順の列に展開する。
func spsOf(rs []SPRange) []int {
	var out []int
	for _, r := range rs {
		for x := r.Min; x <= r.Max; x++ {
			out = append(out, x)
		}
	}
	return out
}

// assertCanonicalRanges は範囲の列が「昇順・互いに素・隣接しない(極大連続区間)・0..32 内・空でない」ことを見る。
func assertCanonicalRanges(t *testing.T, label string, rs []SPRange) {
	t.Helper()
	if len(rs) == 0 {
		t.Errorf("%s: Ranges が空(候補は必ず SP を1つ以上持つ。ADR-0010 §R3)", label)
		return
	}
	for i, r := range rs {
		if r.Min < 0 || r.Max > MaxSPPerStat || r.Min > r.Max {
			t.Errorf("%s: Ranges[%d] = %+v が 0..%d の閉区間になっていない", label, i, r, MaxSPPerStat)
		}
		if i > 0 && r.Min <= rs[i-1].Max+1 {
			t.Errorf("%s: Ranges[%d]=%+v が前の %+v と重なるか隣接している(極大連続区間に畳むこと)",
				label, i, r, rs[i-1])
		}
	}
}

// findCand は候補列から (性格クラス, 持ち物 ID) の候補を探し、その順位を返す。
func findCand(cands []ReverseCandidate, class NatureClass, itemID string) (int, *ReverseCandidate) {
	for i := range cands {
		if cands[i].NatureClass == class && cands[i].ItemID == itemID {
			return i, &cands[i]
		}
	}
	return -1, nil
}

// assertMatchesOracle は CalcReverse の全候補が総当たりの正解と一致することを見る(厳密性)。
func assertMatchesOracle(t *testing.T, in ReverseInput, res ReverseResult) {
	t.Helper()
	want := bruteForceReverse(t, in)
	if len(res.Candidates) != len(want) {
		t.Fatalf("候補数 = %d, want %d(性格2 × 持ち物 %d)", len(res.Candidates), len(want), len(revItems(in)))
	}
	exactCount := 0
	for _, c := range res.Candidates {
		k := revKey{c.NatureClass, c.ItemID}
		w, ok := want[k]
		if !ok {
			t.Fatalf("想定外の候補 %+v", k)
		}
		label := string(c.NatureClass) + "/" + c.ItemID
		assertCanonicalRanges(t, label, c.Ranges)
		if got := spsOf(c.Ranges); !slices.Equal(got, w.ranges) {
			t.Errorf("%s: SP = %v, want %v(exact=%v)", label, got, w.ranges, len(w.exact) > 0)
		}
		if c.Exact != (len(w.exact) > 0) {
			t.Errorf("%s: Exact = %v, want %v", label, c.Exact, len(w.exact) > 0)
		}
		if c.Mismatch != w.mismatch {
			t.Errorf("%s: Mismatch = %d, want %d", label, c.Mismatch, w.mismatch)
		}
		if c.Exact != (c.Mismatch == 0) {
			t.Errorf("%s: Exact=%v と Mismatch=%d が矛盾", label, c.Exact, c.Mismatch)
		}
		if c.SPCount != len(w.ranges) {
			t.Errorf("%s: SPCount = %d, want %d", label, c.SPCount, len(w.ranges))
		}
		if c.Support != w.support {
			t.Errorf("%s: Support = %d, want %d", label, c.Support, w.support)
		}
		if c.MinPercentTenths != w.minT || c.MaxPercentTenths != w.maxT {
			t.Errorf("%s: 想定ダメージ幅 = [%d,%d], want [%d,%d](0.1%%単位。Ranges 全体の表示%%)",
				label, c.MinPercentTenths, c.MaxPercentTenths, w.minT, w.maxT)
		}
		if c.Exact {
			exactCount++
		}
	}
	if res.ExactCount != exactCount {
		t.Errorf("ExactCount = %d, want %d", res.ExactCount, exactCount)
	}
}

// observeTruth は真値の個体への1発を、指定ロール・丸め規則・精度で観測にする。
func observeTruth(t *testing.T, in DamageInput, rollIdx int, r obsRounding, tenths bool) Observation {
	t.Helper()
	res, err := calcDamage(in)
	if err != nil {
		t.Fatalf("真値の CalcDamage が失敗した: %v", err)
	}
	if tenths {
		v := roundObserved(res.Rolls[rollIdx], res.DefenderHP, 1000, r)
		if v <= 0 {
			t.Fatalf("フィクスチャ不正: 観測が 0.0%%")
		}
		return Observation{PercentTenths: v}
	}
	v := roundObserved(res.Rolls[rollIdx], res.DefenderHP, 100, r)
	if v <= 0 {
		t.Fatalf("フィクスチャ不正: 観測が 0%%")
	}
	return Observation{Percent: v}
}

// ---------------------------------------------------------------------------
// AC-1: 観測の区間モデル(丸め規則に依存しない照合。ADR-0010 §R2)
// ---------------------------------------------------------------------------

func TestObservationMatchesTable(t *testing.T) {
	tests := []struct {
		name   string
		obs    Observation
		damage int
		maxHP  int
		want   bool
	}{
		// 整数% 45(HP200): 真の% p が (44, 46) に入るダメージだけ。88=44.0% と 92=46.0% は、
		// どの丸め規則でも 45 にならないので含めない。89=44.5%(四捨五入・切り上げで45)、91=45.5%(切り捨てで45)。
		{"45%: 88 は 44.0% で外", Observation{Percent: 45}, 88, 200, false},
		{"45%: 89 は 44.5%(切り上げ・四捨五入で45)", Observation{Percent: 45}, 89, 200, true},
		{"45%: 90 はちょうど45.0%", Observation{Percent: 45}, 90, 200, true},
		{"45%: 91 は 45.5%(切り捨てで45)", Observation{Percent: 45}, 91, 200, true},
		{"45%: 92 は 46.0% で外", Observation{Percent: 45}, 92, 200, false},
		// 整数% 1: (0, 2) の範囲。
		{"1%: 1 ダメージ(0.5%)", Observation{Percent: 1}, 1, 200, true},
		{"1%: 3 ダメージ(1.5%)", Observation{Percent: 1}, 3, 200, true},
		{"1%: 4 ダメージ(2.0%)は外", Observation{Percent: 1}, 4, 200, false},
		// 100% は HP バーの頭打ち。99% を超えるダメージ(瀕死・過剰打点を含む)すべてと両立する。
		{"100%: 198 は 99.0% で外", Observation{Percent: 100}, 198, 200, false},
		{"100%: 199 は 99.5%", Observation{Percent: 100}, 199, 200, true},
		{"100%: 200 はちょうど100%", Observation{Percent: 100}, 200, 200, true},
		{"100%: 250 は過剰打点(125%)でも両立", Observation{Percent: 100}, 250, 200, true},
		// 0.1% 精度。HP175 の 128 ダメージ = 73.142857...% → 切り捨て・四捨五入 73.1、切り上げ 73.2。
		{"73.1%: HP175 の 128", Observation{PercentTenths: 731}, 128, 175, true},
		{"73.2%: HP175 の 128", Observation{PercentTenths: 732}, 128, 175, true},
		{"73.0%: HP175 の 128 は外", Observation{PercentTenths: 730}, 128, 175, false},
		{"73.3%: HP175 の 128 は外", Observation{PercentTenths: 733}, 128, 175, false},
		// 73.4% は HP175 ではどの整数ダメージからも作れない(128=73.14 / 129=73.71)。
		{"73.4%: HP175 の 128 は外", Observation{PercentTenths: 734}, 128, 175, false},
		{"73.4%: HP175 の 129 は外", Observation{PercentTenths: 734}, 129, 175, false},
		// 100.0% は頭打ち。99.9% を超えるダメージと両立。HP175 の 174 = 99.43% は外。
		{"100.0%: HP175 の 174 は外", Observation{PercentTenths: 1000}, 174, 175, false},
		{"100.0%: HP175 の 175", Observation{PercentTenths: 1000}, 175, 175, true},
		{"100.0%: HP175 の 300(過剰打点)", Observation{PercentTenths: 1000}, 300, 175, true},
		// 実点数は完全一致。
		{"Damage 100: 100", Observation{Damage: 100}, 100, 207, true},
		{"Damage 100: 101", Observation{Damage: 100}, 101, 207, false},
		{"Damage 100: 99", Observation{Damage: 100}, 99, 207, false},
		// maxHP <= 0 は照合できない(ゼロ除算しない。panic しない)。
		{"maxHP=0 は常に外", Observation{Percent: 45}, 10, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.obs.Matches(tt.damage, tt.maxHP); got != tt.want {
				t.Errorf("%+v.Matches(%d, %d) = %v, want %v", tt.obs, tt.damage, tt.maxHP, got, tt.want)
			}
			// 独立な定義(いずれかの丸め規則で作れるか)とも一致すること。
			if got := oracleMatches(tt.obs, tt.damage, tt.maxHP); got != tt.want {
				t.Fatalf("テスト表の誤り: oracleMatches = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestObservationMatchesAnyRounding は、区間モデルが「どの丸め規則で作った観測でも真値を落とさない」
// こと(被覆)と、「どの丸め規則でも作れない値は受け入れない」こと(最小性)の両方を性質として見る。
// つまり Matches は「切り捨て・四捨五入・切り上げのいずれかで作れる」と同値でなければならない。
func TestObservationMatchesAnyRounding(t *testing.T) {
	hps := []int{76, 100, 147, 175, 207, 341, 500, 704}
	for _, hp := range hps {
		for d := 1; d <= hp+hp/5; d++ {
			for _, scale := range []int{100, 1000} {
				mk := func(v int) Observation {
					if scale == 100 {
						return Observation{Percent: v}
					}
					return Observation{PercentTenths: v}
				}
				for _, r := range allObsRoundings {
					v := roundObserved(d, hp, scale, r)
					if v <= 0 {
						continue
					}
					if o := mk(v); !o.Matches(d, hp) {
						t.Fatalf("被覆の破れ: HP%d の %d ダメージを%sで観測した %+v を Matches が落とした", hp, d, r, o)
					}
				}
				// 近傍の値すべてで、独立な定義と同値であること(区間が広すぎない)。
				center := roundObserved(d, hp, scale, roundHalfUp)
				for v := max(1, center-3); v <= min(scale, center+3); v++ {
					o := mk(v)
					got, want := o.Matches(d, hp), oracleMatches(o, d, hp)
					if got != want {
						t.Fatalf("HP%d の %d ダメージと %+v: Matches = %v, want %v(いずれかの丸め規則で作れる ⇔ 両立)",
							hp, d, o, got, want)
					}
					if (o.Distance(d, hp) == 0) != got {
						t.Fatalf("HP%d の %d ダメージと %+v: Distance=%d と Matches=%v が矛盾(0 ⇔ 両立)",
							hp, d, o, o.Distance(d, hp), got)
					}
				}
			}
		}
	}
}

// TestObservationDistance は、説明できない観測との距離(0.1% 単位の整数。ADR-0010 §R2)を手計算の表で固定する。
//
//	Percent       : x = 100*d  - v*HP、Distance = (|x| / HP) * 10   (両立なら 0)
//	PercentTenths : x = 1000*d - v*HP、Distance = (|x| / HP)        (両立なら 0)
//	Damage        : d == D なら 0、それ以外は max(1, 1000*|d-D| / HP)
func TestObservationDistance(t *testing.T) {
	tests := []struct {
		name   string
		obs    Observation
		damage int
		maxHP  int
		want   int
	}{
		{"45%: 両立は 0", Observation{Percent: 45}, 90, 200, 0},
		{"45%: 92 は |x|=200 → 1 → 10", Observation{Percent: 45}, 92, 200, 10},
		{"45%: 93 は |x|=300 → 1 → 10", Observation{Percent: 45}, 93, 200, 10},
		{"45%: 94 は |x|=400 → 2 → 20", Observation{Percent: 45}, 94, 200, 20},
		{"45%: 87 は |x|=300 → 1 → 10", Observation{Percent: 45}, 87, 200, 10},
		{"100%: 過剰打点は 0", Observation{Percent: 100}, 250, 200, 0},
		{"100%: 198 は |x|=200 → 10", Observation{Percent: 100}, 198, 200, 10},
		{"100%: 150 は |x|=5000 → 25 → 250", Observation{Percent: 100}, 150, 200, 250},
		{"73.1%: 両立は 0", Observation{PercentTenths: 731}, 128, 175, 0},
		{"73.0%: HP175 の 128 は |x|=250 → 1", Observation{PercentTenths: 730}, 128, 175, 1},
		{"73.3%: HP175 の 128 は |x|=275 → 1", Observation{PercentTenths: 733}, 128, 175, 1},
		{"73.4%: HP175 の 128 は |x|=450 → 2", Observation{PercentTenths: 734}, 128, 175, 2},
		{"100.0%: HP175 の 174 は |x|=1000 → 5", Observation{PercentTenths: 1000}, 174, 175, 5},
		{"Damage: 一致は 0", Observation{Damage: 100}, 100, 207, 0},
		{"Damage: 差1・HP207 は 1000/207=4", Observation{Damage: 100}, 101, 207, 4},
		{"Damage: 差1(下側)・HP207 は 4", Observation{Damage: 100}, 99, 207, 4},
		{"Damage: 差10・HP207 は 10000/207=48", Observation{Damage: 100}, 110, 207, 48},
		{"Damage: 差1・HP500 は 2", Observation{Damage: 100}, 101, 500, 2},
		{"Damage: 差1・HP2000 は 0 に落ちず下限 1", Observation{Damage: 100}, 101, 2000, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.obs.Distance(tt.damage, tt.maxHP); got != tt.want {
				t.Errorf("%+v.Distance(%d, %d) = %d, want %d", tt.obs, tt.damage, tt.maxHP, got, tt.want)
			}
		})
	}
	t.Run("maxHP=0 でも panic せず 1 以上", func(t *testing.T) {
		for _, o := range []Observation{{Percent: 45}, {PercentTenths: 450}, {Damage: 10}} {
			if got := o.Distance(10, 0); got < 1 {
				t.Errorf("%+v.Distance(10, 0) = %d, want >= 1(照合できないものを完全一致にしない)", o, got)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AC-2: 候補の格子 = 性格クラス{補正なし, 上昇} × 持ち物。防御側は H32 前提(ADR-0010 §R1)
// ---------------------------------------------------------------------------

func TestReverseCandidateGrid(t *testing.T) {
	tests := []struct {
		name       string
		side       ReverseSide
		category   MoveCategory
		wantStat   StatKey
		wantHPSP   int
		wantPlusNa Nature
	}{
		{"防御側・物理は B", SideDefender, CategoryPhysical, StatDef, MaxSPPerStat, Nature{Plus: StatDef, Minus: StatAtk}},
		{"防御側・特殊は D", SideDefender, CategorySpecial, StatSpD, MaxSPPerStat, Nature{Plus: StatSpD, Minus: StatAtk}},
		{"防御側・変化技は物理と同じ B", SideDefender, CategoryStatus, StatDef, MaxSPPerStat, Nature{Plus: StatDef, Minus: StatAtk}},
		// 攻撃側の HP は計算に使わないので 0(H の仮定を持たない)。上昇補正の相手は spa。
		{"攻撃側・物理は A", SideAttacker, CategoryPhysical, StatAtk, 0, Nature{Plus: StatAtk, Minus: StatSpA}},
		{"攻撃側・特殊は C", SideAttacker, CategorySpecial, StatSpA, 0, Nature{Plus: StatSpA, Minus: StatAtk}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := []*Item{nil, revEviolite(), revBerry(TypeWater)}
			in := ReverseInput{
				Format: FormatSingle, Side: tt.side, Known: revKnownAttacker(),
				UnknownSpecies: revDefenderSpecies(), Move: revMove(tt.category),
				ItemCandidates: items, Observations: []Observation{{Percent: 40}},
			}
			if tt.side == SideAttacker {
				in.Known = revDefender(Stats{HP: 32, Def: 32}, NatureNeutral, nil)
				in.UnknownSpecies = revAttackerSpecies()
			}
			res, err := calcReverse(in)
			if err != nil {
				t.Fatalf("CalcReverse: %v", err)
			}
			if res.Side != tt.side || res.Stat != tt.wantStat {
				t.Errorf("Side/Stat = %q/%q, want %q/%q", res.Side, res.Stat, tt.side, tt.wantStat)
			}
			if res.AssumedHPSP != tt.wantHPSP {
				t.Errorf("AssumedHPSP = %d, want %d(防御側は H32 前提、攻撃側は 0)", res.AssumedHPSP, tt.wantHPSP)
			}
			if len(res.Candidates) != len(revClasses)*len(items) {
				t.Fatalf("候補数 = %d, want %d(性格2 × 持ち物%d)", len(res.Candidates), len(revClasses)*len(items), len(items))
			}
			seen := map[revKey]bool{}
			for _, c := range res.Candidates {
				k := revKey{c.NatureClass, c.ItemID}
				if seen[k] {
					t.Errorf("候補が重複: %+v", k)
				}
				seen[k] = true
				switch c.NatureClass {
				case NatureClassNeutral:
					if c.Nature != NatureNeutral {
						t.Errorf("補正なしの代表性格 = %+v, want 無補正", c.Nature)
					}
				case NatureClassPlus:
					if c.Nature != tt.wantPlusNa {
						t.Errorf("上昇の代表性格 = %+v, want %+v", c.Nature, tt.wantPlusNa)
					}
				default:
					t.Errorf("性格クラス %q は候補にしない(補正なし・上昇の2通りだけ)", c.NatureClass)
				}
				idx := slices.IndexFunc(items, func(it *Item) bool { return revItemID(it) == c.ItemID })
				if idx < 0 {
					t.Fatalf("持ち物 %q は候補に無い", c.ItemID)
				}
				if c.Item != items[idx] {
					t.Errorf("候補は渡された *Item をそのまま保持すること(%q)", c.ItemID)
				}
			}
			for _, class := range revClasses {
				for _, it := range items {
					if !seen[revKey{class, revItemID(it)}] {
						t.Errorf("候補 (%s, %q) が無い", class, revItemID(it))
					}
				}
			}
		})
	}

	t.Run("持ち物候補を省略すると持ち物なしの1通り(候補は2件)", func(t *testing.T) {
		res, err := calcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: revMove(CategoryPhysical),
			Observations: []Observation{{Percent: 40}},
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		if len(res.Candidates) != 2 {
			t.Fatalf("候補数 = %d, want 2", len(res.Candidates))
		}
		for _, c := range res.Candidates {
			if c.Item != nil || c.ItemID != "" {
				t.Errorf("持ち物候補を省略したのに持ち物付きの候補: %q", c.ItemID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AC-3: 範囲は厳密(観測を説明できる SP の集合そのもの。広すぎも狭すぎもしない。ADR-0010 §R3)
// ---------------------------------------------------------------------------

func TestReverseRangesMatchBruteForce(t *testing.T) {
	phys, spec := revMove(CategoryPhysical), revMove(CategorySpecial)
	truthB19 := revDefender(Stats{HP: 32, Def: 19}, NatureNeutral, nil)
	truthD5 := revDefender(Stats{HP: 32, SpD: 5}, Nature{Plus: StatSpD, Minus: StatAtk}, revVest())
	knownWall := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	atkTruth := Individual{Species: revAttackerSpecies(), Level: DefaultLevel,
		Nature: Nature{Plus: StatAtk, Minus: StatSpA}, SP: Stats{Atk: 11}, Status: StatusNone}
	frailTruth := Individual{Species: revFrailSpecies(), Level: DefaultLevel,
		SP: Stats{HP: 32}, Status: StatusNone}
	bigMove := Move{ID: "big", NameJa: "テストおおわざ", Type: TypeWater, Category: CategoryPhysical, Power: 120}

	defIn := func(move Move, items []*Item, obs ...Observation) ReverseInput {
		return ReverseInput{Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move, ItemCandidates: items, Observations: obs}
	}

	tests := []struct {
		name string
		in   func(t *testing.T) ReverseInput
	}{
		{"防御側・整数%1件(四捨五入)", func(t *testing.T) ReverseInput {
			o := observeTruth(t, DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truthB19, Move: phys}, 7, roundHalfUp, false)
			return defIn(phys, []*Item{nil, revEviolite()}, o)
		}},
		{"防御側・整数%2件(切り捨て/切り上げ)・区別できない持ち物(等倍へのきのみ)を含む", func(t *testing.T) ReverseInput {
			in := DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truthD5, Move: spec}
			return defIn(spec, []*Item{nil, revVest(), revBerry(TypeWater)},
				observeTruth(t, in, 2, roundFloor, false), observeTruth(t, in, 13, roundCeil, false))
		}},
		{"防御側・0.1%精度1件", func(t *testing.T) ReverseInput {
			o := observeTruth(t, DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truthB19, Move: phys}, 10, roundFloor, true)
			return defIn(phys, []*Item{nil, revEviolite()}, o)
		}},
		{"防御側・100%頭打ち(確定1発)", func(t *testing.T) ReverseInput {
			res, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: revStrongAttacker(), Defender: frailTruth, Move: bigMove})
			if err != nil {
				t.Fatal(err)
			}
			if res.Rolls[0] < res.DefenderHP {
				t.Fatalf("フィクスチャ不正: 最低ロール %d < HP %d(瀕死にならない)", res.Rolls[0], res.DefenderHP)
			}
			return ReverseInput{Format: FormatSingle, Side: SideDefender, Known: revStrongAttacker(),
				UnknownSpecies: revFrailSpecies(), Move: bigMove, ItemCandidates: []*Item{nil, revEviolite()},
				Observations: []Observation{{Percent: roundObserved(res.Rolls[0], res.DefenderHP, 100, roundFloor)}}}
		}},
		{"攻撃側・実点数1件", func(t *testing.T) ReverseInput {
			res, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: atkTruth, Defender: knownWall, Move: phys})
			if err != nil {
				t.Fatal(err)
			}
			return ReverseInput{Format: FormatSingle, Side: SideAttacker, Known: knownWall,
				UnknownSpecies: revAttackerSpecies(), Move: phys, Observations: []Observation{{Damage: res.Rolls[6]}}}
		}},
		{"防御側・どの SP でも説明できない観測(近い候補を返す)", func(t *testing.T) ReverseInput {
			return defIn(phys, []*Item{nil, revEviolite()}, Observation{Percent: 1})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)
			res, err := calcReverse(in)
			if err != nil {
				t.Fatalf("CalcReverse: %v", err)
			}
			assertMatchesOracle(t, in, res)
		})
	}
}

// TestReverseNonContiguousRangesKept は、説明できる SP が飛び飛びになる観測で、
// engine が1区間に丸めず複数の区間として返すことを見る(情報を捨てない。ADR-0010 §R3)。
func TestReverseNonContiguousRangesKept(t *testing.T) {
	// A32・A上昇の威力150はロールの間隔が 2 以上に開き、ある実点数を出せる B の SP が飛び飛びになる。
	move := Move{ID: "gap", NameJa: "テストすきま", Type: TypeWater, Category: CategoryPhysical, Power: 150}
	base := ReverseInput{Format: FormatSingle, Side: SideDefender, Known: revStrongAttacker(),
		UnknownSpecies: revDefenderSpecies(), Move: move}

	// テスト側で、飛び飛びになる実点数を決定的に探す(小さい順で最初のもの)。
	byDamage := map[int][]int{}
	lo, hi := 1<<30, 0
	for x := 0; x <= MaxSPPerStat; x++ {
		res := revRolls(t, base, NatureClassNeutral, nil, x)
		seen := map[int]bool{}
		for _, d := range res.Rolls {
			lo, hi = min(lo, d), max(hi, d)
			if !seen[d] {
				seen[d] = true
				byDamage[d] = append(byDamage[d], x)
			}
		}
	}
	gapDamage := 0
	for d := lo; d <= hi && gapDamage == 0; d++ {
		s := byDamage[d]
		for i := 1; i < len(s); i++ {
			if s[i] != s[i-1]+1 {
				gapDamage = d
				break
			}
		}
	}
	if gapDamage == 0 {
		t.Fatal("フィクスチャ不正: 飛び飛びになる実点数が見つからない")
	}

	in := base
	in.Observations = []Observation{{Damage: gapDamage}}
	res, err := calcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	_, c := findCand(res.Candidates, NatureClassNeutral, "")
	if c == nil {
		t.Fatal("補正なし・持ち物なしの候補が無い")
	}
	if len(c.Ranges) < 2 {
		t.Errorf("Damage=%d で SP %v が飛び飛びなのに Ranges = %+v(1区間に丸めて情報を捨てている)",
			gapDamage, byDamage[gapDamage], c.Ranges)
	}
	if got := spsOf(c.Ranges); !slices.Equal(got, byDamage[gapDamage]) {
		t.Errorf("Damage=%d の SP = %v, want %v", gapDamage, got, byDamage[gapDamage])
	}
	assertMatchesOracle(t, in, res)
}

// ---------------------------------------------------------------------------
// AC-4: 真値は必ず範囲に入る(どの丸め規則・どの SP でも。被覆)
// ---------------------------------------------------------------------------

func TestReverseTruthInRange(t *testing.T) {
	sps := []int{0, 1, 7, 16, 19, 31, 32}
	for _, side := range []ReverseSide{SideDefender, SideAttacker} {
		for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
			stat := revStatFor(side, cat)
			items := []*Item{nil, revEviolite()}
			if side == SideAttacker {
				items = []*Item{nil, {ID: "dmgup", NameJa: "テストもちもの5", Effect: &ItemEffect{DamageMod: 5324}}}
			}
			for i, x := range sps {
				for _, class := range revClasses {
					for _, r := range allObsRoundings {
						item := items[i%len(items)]
						truthSP := Stats{}.WithStat(stat, x)
						known := revKnownAttacker()
						unknownSpecies := revDefenderSpecies()
						if side == SideDefender {
							truthSP.HP = MaxSPPerStat
						} else {
							known = revDefender(Stats{HP: 32, Def: 32, SpD: 2}, NatureNeutral, nil)
							unknownSpecies = revAttackerSpecies()
						}
						truth := Individual{Species: unknownSpecies, Level: DefaultLevel,
							Nature: revNatureFor(stat, class), SP: truthSP, Item: item, Status: StatusNone}
						dmg := DamageInput{Format: FormatSingle, Move: revMove(cat)}
						if side == SideDefender {
							dmg.Attacker, dmg.Defender = known, truth
						} else {
							dmg.Attacker, dmg.Defender = truth, known
						}
						obs := observeTruth(t, dmg, (x*5+int(r))%16, r, false)
						res, err := calcReverse(ReverseInput{
							Format: FormatSingle, Side: side, Known: known, UnknownSpecies: unknownSpecies,
							Move: revMove(cat), ItemCandidates: items, Observations: []Observation{obs},
						})
						if err != nil {
							t.Fatalf("CalcReverse: %v", err)
						}
						_, c := findCand(res.Candidates, class, revItemID(item))
						if c == nil {
							t.Fatalf("%s/%s SP=%d %s: 真値の候補が無い", side, cat, x, class)
						}
						if !c.Exact || !slices.Contains(spsOf(c.Ranges), x) {
							t.Errorf("%s/%s 真値 SP=%d %s 持ち物%q 観測%+v(%s): Exact=%v Ranges=%+v に真値が入らない",
								side, cat, x, class, revItemID(item), obs, r, c.Exact, c.Ranges)
						}
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC-5: 観測を足すと絞り込める(範囲は部分集合になり、真値は残る)
// ---------------------------------------------------------------------------

func TestReverseMultipleObservationsNarrow(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil, revEviolite()}
	truthX := 20
	truth := revDefender(Stats{HP: 32, Def: truthX}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	dmg := DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move}
	o1 := observeTruth(t, dmg, 0, roundHalfUp, false)
	o2 := observeTruth(t, dmg, 15, roundFloor, false)
	o3 := observeTruth(t, dmg, 8, roundCeil, false)

	call := func(obs ...Observation) ReverseResult {
		t.Helper()
		res, err := calcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move, ItemCandidates: items, Observations: obs,
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		return res
	}
	rs := []ReverseResult{call(o1), call(o1, o2), call(o1, o2, o3)}

	for i, r := range rs {
		_, c := findCand(r.Candidates, NatureClassPlus, "")
		if c == nil || !c.Exact || !slices.Contains(spsOf(c.Ranges), truthX) {
			t.Errorf("観測 %d 件で真値(上昇・B%d・持ち物なし)が範囲から消えた: %+v", i+1, truthX, c)
		}
		if len(r.Candidates) != len(rs[0].Candidates) {
			t.Errorf("候補の総数が観測数で変わった: %d -> %d", len(rs[0].Candidates), len(r.Candidates))
		}
	}
	for i := 1; i < len(rs); i++ {
		if rs[i].ExactCount > rs[i-1].ExactCount {
			t.Errorf("ExactCount が増えた: 観測%d件 %d -> 観測%d件 %d", i, rs[i-1].ExactCount, i+1, rs[i].ExactCount)
		}
		for _, c := range rs[i].Candidates {
			if !c.Exact {
				continue
			}
			_, prev := findCand(rs[i-1].Candidates, c.NatureClass, c.ItemID)
			if prev == nil || !prev.Exact {
				t.Errorf("観測%d件目で (%s,%q) が新たに説明可能になった(部分集合でない)", i+1, c.NatureClass, c.ItemID)
				continue
			}
			prevSet := spsOf(prev.Ranges)
			for _, x := range spsOf(c.Ranges) {
				if !slices.Contains(prevSet, x) {
					t.Errorf("観測%d件目で (%s,%q) の SP %d が増えた(範囲は部分集合でなければならない)",
						i+1, c.NatureClass, c.ItemID, x)
				}
			}
		}
	}
	// 3件の観測で、真値の範囲は1件のときより狭い(このフィクスチャでは実際に絞れる)。
	_, first := findCand(rs[0].Candidates, NatureClassPlus, "")
	_, last := findCand(rs[2].Candidates, NatureClassPlus, "")
	if first != nil && last != nil && last.SPCount >= first.SPCount {
		t.Errorf("観測3件でも真値の範囲が狭まらない: %d -> %d(2件目以降の観測を無視していないか)", first.SPCount, last.SPCount)
	}
	all := []Observation{o1, o2, o3}
	for i, r := range rs {
		assertMatchesOracle(t, ReverseInput{Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move, ItemCandidates: items,
			Observations: all[:i+1]}, r)
	}
}

// 到達不能な2件目の観測を足すと、1件目で説明できていた候補がすべて説明できなくなる
// (「2件目以降の観測を無視する」実装の検出)。
func TestReverseUnreachableSecondObservationRemovesExact(t *testing.T) {
	move := revMove(CategoryPhysical)
	truth := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	o1 := observeTruth(t, DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move}, 0, roundHalfUp, false)

	call := func(obs ...Observation) ReverseResult {
		t.Helper()
		res, err := calcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move, Observations: obs,
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		return res
	}
	one := call(o1)
	if one.ExactCount == 0 || !one.Candidates[0].Exact {
		t.Fatalf("1件目だけなら説明できる候補がある: ExactCount=%d", one.ExactCount)
	}
	// 1% はこの技のどの調整でも到達しない(ダメージは 29〜46% 程度)。
	two := call(o1, Observation{Percent: 1})
	if two.ExactCount != 0 {
		t.Errorf("到達不能な2件目を足したのに ExactCount = %d, want 0", two.ExactCount)
	}
	for i, c := range two.Candidates {
		if c.Exact || c.Mismatch <= 0 {
			t.Errorf("候補 %d が説明可能のまま: Exact=%v Mismatch=%d", i, c.Exact, c.Mismatch)
		}
		if len(c.Ranges) == 0 {
			t.Errorf("候補 %d の Ranges が空(近い SP を返すこと)", i)
		}
	}
}

// ---------------------------------------------------------------------------
// AC-6: 説明できなくても近い候補を返す(正確さより候補の提示を優先)
// ---------------------------------------------------------------------------

func TestReverseNoExactStillReturnsCandidates(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil, revEviolite()}
	// どの調整でも到達しない観測(1% と 100%)。100% は「99% 超」と両立だが、この技は 46% 程度まで。
	for _, pct := range []int{1, 100} {
		in := ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			ItemCandidates: items, Observations: []Observation{{Percent: pct}},
		}
		res, err := calcReverse(in)
		if err != nil {
			t.Fatalf("observed=%d%%: CalcReverse: %v", pct, err)
		}
		if len(res.Candidates) != 4 {
			t.Fatalf("observed=%d%%: 候補数 = %d, want 4(説明できなくても全候補を返す)", pct, len(res.Candidates))
		}
		if res.ExactCount != 0 {
			t.Errorf("observed=%d%%: ExactCount = %d, want 0", pct, res.ExactCount)
		}
		for i := 1; i < len(res.Candidates); i++ {
			if res.Candidates[i-1].Mismatch > res.Candidates[i].Mismatch {
				t.Errorf("observed=%d%%: Mismatch が昇順でない(%d 番目)", pct, i)
			}
		}
		// 範囲は「距離が最小の SP」そのもの(総当たりの正解と一致)。
		assertMatchesOracle(t, in, res)
	}
}

// ロールに存在しない近傍の実点数観測は説明可能にならない(距離の下限 1。HP が大きくても 0 に落ちない)。
func TestReverseDamageObservationNearMissIsNotExact(t *testing.T) {
	move := revMove(CategoryPhysical)
	known := Individual{Species: revBigHPSpecies(), Level: DefaultLevel, Nature: NatureNeutral, Status: StatusNone}
	hp := RealStats(known).HP
	if hp <= 200 {
		t.Fatalf("フィクスチャ不正: HP=%d", hp)
	}
	base := ReverseInput{Format: FormatSingle, Side: SideAttacker, Known: known,
		UnknownSpecies: revAttackerSpecies(), Move: move}

	// 攻撃側の全候補(33 × 性格2)のロールの集合を、テスト側で独立に作る。
	rolls := map[int]bool{}
	maxRoll, minRoll := 0, 1<<30
	for _, class := range revClasses {
		for x := 0; x <= MaxSPPerStat; x++ {
			for _, r := range revRolls(t, base, class, nil, x).Rolls {
				rolls[r] = true
				maxRoll, minRoll = max(maxRoll, r), min(minRoll, r)
			}
		}
	}
	call := func(d int) ReverseResult {
		t.Helper()
		in := base
		in.Observations = []Observation{{Damage: d}}
		res, err := calcReverse(in)
		if err != nil {
			t.Fatalf("Damage=%d: CalcReverse: %v", d, err)
		}
		return res
	}
	if res := call(maxRoll); res.ExactCount == 0 {
		t.Fatalf("Damage=%d(実在するロール)が説明可能にならない", maxRoll)
	}
	misses := []int{maxRoll + 1, maxRoll + 2}
	if minRoll-2 >= 1 {
		misses = append(misses, minRoll-1, minRoll-2)
	}
	for _, d := range misses {
		if rolls[d] {
			t.Fatalf("フィクスチャ不正: Damage=%d は実在するロール", d)
		}
		res := call(d)
		if res.ExactCount != 0 {
			t.Errorf("Damage=%d(実在しない近傍)で ExactCount = %d, want 0", d, res.ExactCount)
		}
		for _, c := range res.Candidates {
			if c.Exact || c.Mismatch < 1 {
				t.Errorf("Damage=%d(実在しない近傍)で (%s,%q) が説明可能扱い: Exact=%v Mismatch=%d",
					d, c.NatureClass, c.ItemID, c.Exact, c.Mismatch)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC-7: 順序は決定的な全順序(ADR-0010 §R4)
//   Mismatch 昇順 → Support 降順 → SPCount 降順 → 定義順(性格クラス 補正なし→上昇、持ち物添字)
// ---------------------------------------------------------------------------

func revDefIndex(c ReverseCandidate, items []*Item) int {
	ci := slices.Index(revClasses, c.NatureClass)
	ii := slices.IndexFunc(items, func(it *Item) bool { return revItemID(it) == c.ItemID })
	return ci*len(items) + ii
}

func TestReverseOrderDeterministic(t *testing.T) {
	move := revMove(CategoryPhysical)
	// 等倍の水技にはきのみが効かないので、(補正なし, なし) と (補正なし, きのみ) は完全に同点になる。
	items := []*Item{nil, revEviolite(), revBerry(TypeWater)}
	obs := []Observation{{Percent: 42}, {Percent: 38}}
	build := func() ReverseInput {
		return ReverseInput{Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move, ItemCandidates: items, Observations: obs}
	}
	first, err := calcReverse(build())
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := calcReverse(build())
		if err != nil {
			t.Fatalf("CalcReverse(%d): %v", i, err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("同じ入力で結果が変わった(%d 回目)", i)
		}
	}
	for i := 1; i < len(first.Candidates); i++ {
		a, b := first.Candidates[i-1], first.Candidates[i]
		ok := false
		switch {
		case a.Mismatch != b.Mismatch:
			ok = a.Mismatch < b.Mismatch
		case a.Support != b.Support:
			ok = a.Support > b.Support
		case a.SPCount != b.SPCount:
			ok = a.SPCount > b.SPCount
		default:
			ok = revDefIndex(a, items) < revDefIndex(b, items)
		}
		if !ok {
			t.Errorf("順序違反: %d 番目 (%s,%q M=%d S=%d N=%d) の後に (%s,%q M=%d S=%d N=%d)", i,
				a.NatureClass, a.ItemID, a.Mismatch, a.Support, a.SPCount,
				b.NatureClass, b.ItemID, b.Mismatch, b.Support, b.SPCount)
		}
	}
	// 同点の2件は定義順(持ち物添字)で並ぶ。
	iNone, cNone := findCand(first.Candidates, NatureClassNeutral, "")
	iBerry, cBerry := findCand(first.Candidates, NatureClassNeutral, revBerry(TypeWater).ID)
	if cNone == nil || cBerry == nil {
		t.Fatal("候補が無い")
	}
	if !reflect.DeepEqual(cNone.Ranges, cBerry.Ranges) || cNone.Support != cBerry.Support || cNone.Mismatch != cBerry.Mismatch {
		t.Fatalf("フィクスチャ不正: 等倍へのきのみが持ち物なしと同点になっていない")
	}
	if iNone > iBerry {
		t.Errorf("同点の (補正なし, なし)=%d が (補正なし, きのみ)=%d より後(定義順は持ち物添字の昇順)", iNone, iBerry)
	}

	// MaxCandidates は上位から切り取るだけで、順序も ExactCount も変えない。
	in := build()
	in.MaxCandidates = 3
	limited, err := calcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse(MaxCandidates=3): %v", err)
	}
	if len(limited.Candidates) != 3 {
		t.Fatalf("MaxCandidates=3 のとき候補数 = %d, want 3", len(limited.Candidates))
	}
	for i := range limited.Candidates {
		if !reflect.DeepEqual(limited.Candidates[i], first.Candidates[i]) {
			t.Errorf("MaxCandidates で %d 番目の候補が変わった", i)
		}
	}
	if limited.ExactCount != first.ExactCount {
		t.Errorf("MaxCandidates=3 の ExactCount = %d, want %d(切り取り前の値)", limited.ExactCount, first.ExactCount)
	}
	// MaxCandidates が負は 0(無制限)ではなく不正として拒否する(issue #110。ADR-0208 §4・ADR-0108。
	// 2026-09-23 に「負は無制限」から変更: HTTP 契約(ADR-0208 §1)が負を invalid_input にしており、
	// 直接呼び出し・WASM でも同じ結論にする)。
	in.MaxCandidates = -1
	if _, err := calcReverse(in); !errors.Is(err, ErrInvalidMaxCandidates) {
		t.Errorf("MaxCandidates=-1: err = %v, want ErrInvalidMaxCandidates", err)
	}
}

// 変化技・無効相性はダメージ 0。エラーにせず、全候補が同点(距離は手計算できる)で定義順に並ぶ。
func TestReverseZeroDamageMovesKeepDefinitionOrder(t *testing.T) {
	items := []*Item{nil, revEviolite(), revVest()}
	tests := []struct {
		name    string
		species Species
		move    Move
	}{
		{"変化技", revDefenderSpecies(), Move{ID: "status", NameJa: "テストへんか", Type: TypeWater, Category: CategoryStatus}},
		{"無効相性", revBigHPSpecies(), Move{ID: "immune", NameJa: "テストむこう", Type: TypeGhost, Category: CategoryPhysical, Power: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := calcReverse(ReverseInput{
				Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
				UnknownSpecies: tt.species, Move: tt.move,
				ItemCandidates: items, Observations: []Observation{{Percent: 30}},
			})
			if err != nil {
				t.Fatalf("ダメージ 0 の技でエラーになった: %v", err)
			}
			if len(res.Candidates) != len(revClasses)*len(items) {
				t.Fatalf("候補数 = %d, want %d", len(res.Candidates), len(revClasses)*len(items))
			}
			if res.ExactCount != 0 {
				t.Errorf("ExactCount = %d, want 0", res.ExactCount)
			}
			for i, c := range res.Candidates {
				// ダメージ 0 と 30% の距離: x = 100*0 - 30*HP → |x|/HP = 30 → ×10 = 300(0.1% 単位)。
				if c.Mismatch != 300 {
					t.Errorf("候補 %d の Mismatch = %d, want 300", i, c.Mismatch)
				}
				// 全 SP が同じ距離なので範囲は 0..32 の1区間、説明できるロールは無い。
				if !reflect.DeepEqual(c.Ranges, []SPRange{{Min: 0, Max: MaxSPPerStat}}) || c.SPCount != MaxSPPerStat+1 || c.Support != 0 {
					t.Errorf("候補 %d: Ranges=%+v SPCount=%d Support=%d, want [{0 32}] 33 0", i, c.Ranges, c.SPCount, c.Support)
				}
				wantClass := revClasses[i/len(items)]
				wantItem := revItemID(items[i%len(items)])
				if c.NatureClass != wantClass || c.ItemID != wantItem {
					t.Fatalf("候補 %d = (%s,%q), want (%s,%q)(同点は定義順: 性格クラス → 持ち物添字)",
						i, c.NatureClass, c.ItemID, wantClass, wantItem)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-8: 候補は CalcDamage の合成(場・急所・持ち物・観測の種類が効く)
// ---------------------------------------------------------------------------

func TestReverseCriticalAndFieldAreApplied(t *testing.T) {
	move := revMove(CategoryPhysical) // 水技
	truthX := 20
	truth := revDefender(Stats{HP: 32, Def: truthX}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	tests := []struct {
		name     string
		critical bool
		field    Field
	}{
		{"急所", true, Field{}},
		{"雨 + 水技", false, Field{Weather: WeatherRain}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := observeTruth(t, DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth,
				Move: move, Field: tt.field, Critical: tt.critical}, 8, roundHalfUp, false)
			call := func(critical bool, field Field) *ReverseCandidate {
				t.Helper()
				res, err := calcReverse(ReverseInput{
					Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
					UnknownSpecies: revDefenderSpecies(), Move: move,
					Field: field, Critical: critical, Observations: []Observation{obs},
				})
				if err != nil {
					t.Fatalf("CalcReverse: %v", err)
				}
				_, c := findCand(res.Candidates, NatureClassPlus, "")
				if c == nil {
					t.Fatal("真値の候補が無い")
				}
				return c
			}
			if c := call(tt.critical, tt.field); !c.Exact || !slices.Contains(spsOf(c.Ranges), truthX) {
				t.Errorf("同じ Critical/Field を渡したのに真値 B%d が範囲に入らない: %+v", truthX, c.Ranges)
			}
			// 1.5 倍のダメージは、同じ SP の ±1% の区間に入らない。
			if c := call(false, Field{}); c.Exact && slices.Contains(spsOf(c.Ranges), truthX) {
				t.Errorf("Critical/Field を渡さないのに真値 B%d が説明可能のまま: %+v", truthX, c.Ranges)
			}
		})
	}
}

func TestReverseItemCandidates(t *testing.T) {
	move := revMove(CategorySpecial)
	vest := revVest()
	truthX := 10
	truth := revDefender(Stats{HP: 32, SpD: truthX}, NatureNeutral, vest)
	obs := observeTruth(t, DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move}, 7, roundHalfUp, false)
	in := ReverseInput{Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
		UnknownSpecies: revDefenderSpecies(), Move: move,
		ItemCandidates: []*Item{nil, vest}, Observations: []Observation{obs}}
	res, err := calcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	_, c := findCand(res.Candidates, NatureClassNeutral, vest.ID)
	if c == nil || !c.Exact || !slices.Contains(spsOf(c.Ranges), truthX) {
		t.Fatalf("持ち物付きの真値(補正なし・D%d・%s)が範囲に入らない: %+v", truthX, vest.ID, c)
	}
	if c.Item != vest {
		t.Error("候補は渡された *Item をそのまま保持すること")
	}
	assertMatchesOracle(t, in, res)
}

// Percent / PercentTenths / Damage の観測の混在は許容され、真値は範囲に残る。
func TestReverseMixedObservationKinds(t *testing.T) {
	move := revMove(CategoryPhysical)
	known := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	truth := Individual{Species: revAttackerSpecies(), Level: DefaultLevel,
		Nature: Nature{Plus: StatAtk, Minus: StatSpA}, SP: Stats{Atk: 32}, Status: StatusNone}
	dmg := DamageInput{Format: FormatSingle, Attacker: truth, Defender: known, Move: move}
	base, err := calcDamage(dmg)
	if err != nil {
		t.Fatalf("真値の CalcDamage: %v", err)
	}
	in := ReverseInput{
		Format: FormatSingle, Side: SideAttacker, Known: known,
		UnknownSpecies: revAttackerSpecies(), Move: move,
		Observations: []Observation{
			observeTruth(t, dmg, 3, roundHalfUp, false),
			{Damage: base.Rolls[9]},
			observeTruth(t, dmg, 12, roundFloor, true),
		},
	}
	res, err := calcReverse(in)
	if err != nil {
		t.Fatalf("観測の種類の混在でエラー: %v", err)
	}
	if _, c := findCand(res.Candidates, NatureClassPlus, ""); c == nil || !c.Exact || !slices.Contains(spsOf(c.Ranges), 32) {
		t.Errorf("混在観測で真値(上昇・A32)が範囲に入らない: %+v", c)
	}
	assertMatchesOracle(t, in, res)
}

// ---------------------------------------------------------------------------
// AC-9: 入力検証(errors.Is で判別できる sentinel)
// ---------------------------------------------------------------------------

func TestReverseValidationErrors(t *testing.T) {
	valid := func() ReverseInput {
		return ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: revMove(CategoryPhysical),
			Observations: []Observation{{Percent: 40}},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*ReverseInput)
		wantErr error
	}{
		{"Side が空", func(in *ReverseInput) { in.Side = "" }, ErrInvalidReverseSide},
		{"Side が未知の値", func(in *ReverseInput) { in.Side = "both" }, ErrInvalidReverseSide},
		{"観測が nil", func(in *ReverseInput) { in.Observations = nil }, ErrNoObservation},
		{"観測が空", func(in *ReverseInput) { in.Observations = []Observation{} }, ErrNoObservation},
		{"Percent と Damage の同時指定", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 40, Damage: 80}}
		}, ErrInvalidObservation},
		{"Percent と PercentTenths の同時指定", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 40, PercentTenths: 400}}
		}, ErrInvalidObservation},
		{"PercentTenths と Damage の同時指定", func(in *ReverseInput) {
			in.Observations = []Observation{{PercentTenths: 400, Damage: 80}}
		}, ErrInvalidObservation},
		{"何も指定しない", func(in *ReverseInput) {
			in.Observations = []Observation{{Note: "めも"}}
		}, ErrInvalidObservation},
		{"Percent=0 は未指定扱い", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 0}}
		}, ErrInvalidObservation},
		{"Percent が負", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: -1}}
		}, ErrInvalidObservation},
		{"Percent が 100 超", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 101}}
		}, ErrInvalidObservation},
		{"PercentTenths が負", func(in *ReverseInput) {
			in.Observations = []Observation{{PercentTenths: -1}}
		}, ErrInvalidObservation},
		{"PercentTenths が 1000 超", func(in *ReverseInput) {
			in.Observations = []Observation{{PercentTenths: 1001}}
		}, ErrInvalidObservation},
		{"Damage が負", func(in *ReverseInput) {
			in.Observations = []Observation{{Damage: -5}}
		}, ErrInvalidObservation},
		{"2件目だけ不正", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 40}, {Percent: 0}}
		}, ErrInvalidObservation},
		// 件数・範囲の上限(issue #110。ADR-0208 §4・ADR-0108)。
		{"ItemCandidates が上限超過", func(in *ReverseInput) {
			in.ItemCandidates = make([]*Item, MaxReverseItemCandidates+1)
		}, ErrTooManyItemCandidates},
		{"Observations が上限超過(内容はすべて不正でも件数エラーが先)", func(in *ReverseInput) {
			// 中身をすべて不正(Percent=0 は未指定扱い)にすることで、件数の検査が
			// validateObservations(観測1件ごとの内容検証)より前にあることを確かめる。
			// 中身が正しい観測だけでは、検査の順序に依らずどちらのエラーにもなりうる。
			obs := make([]Observation, MaxReverseObservations+1)
			for i := range obs {
				obs[i] = Observation{Percent: 0}
			}
			in.Observations = obs
		}, ErrTooManyObservations},
		{"MaxCandidates が負", func(in *ReverseInput) { in.MaxCandidates = -1 }, ErrInvalidMaxCandidates},
		{"MaxCandidates が上限超過", func(in *ReverseInput) { in.MaxCandidates = MaxReverseMaxCandidates + 1 }, ErrInvalidMaxCandidates},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.mutate(&in)
			_, err := calcReverse(in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is(..., %v)", err, tt.wantErr)
			}
		})
	}

	t.Run("上限ちょうどの観測は受け付ける", func(t *testing.T) {
		for _, o := range []Observation{{Percent: 100}, {PercentTenths: 1000}, {Percent: 1}, {PercentTenths: 1}} {
			in := valid()
			in.Observations = []Observation{o}
			if _, err := calcReverse(in); err != nil {
				t.Errorf("%+v を拒否した: %v", o, err)
			}
		}
	})
	t.Run("上限ちょうどの件数・範囲は受け付ける(issue #110。ADR-0108)", func(t *testing.T) {
		in := valid()
		in.ItemCandidates = make([]*Item, MaxReverseItemCandidates)
		for i := range in.ItemCandidates {
			in.ItemCandidates[i] = &Item{ID: fmt.Sprintf("item%d", i)}
		}
		obs := make([]Observation, MaxReverseObservations)
		for i := range obs {
			obs[i] = Observation{Percent: 40}
		}
		in.Observations = obs
		in.MaxCandidates = MaxReverseMaxCandidates
		res, err := calcReverse(in)
		if err != nil {
			t.Fatalf("上限ちょうどの入力が失敗した: %v", err)
		}
		// 性格クラス2 × 持ち物候補64 = 128 件がちょうど MaxCandidates(128)と一致するので、
		// 上限で切り取られても件数は変わらない。
		want := len(reverseClasses) * MaxReverseItemCandidates
		if want != MaxReverseMaxCandidates {
			t.Fatalf("テストの前提が崩れている: 性格クラス×持ち物候補 = %d, MaxReverseMaxCandidates = %d", want, MaxReverseMaxCandidates)
		}
		if len(res.Candidates) != want {
			t.Errorf("候補数 = %d, want %d", len(res.Candidates), want)
		}
	})
	t.Run("既知側の個体が不正なら CalcDamage と同じエラー", func(t *testing.T) {
		in := valid()
		in.Known.SP = Stats{Atk: MaxSPPerStat + 1}
		if _, err := calcReverse(in); err == nil {
			t.Fatal("SP 上限超えの既知側を受け入れてしまった")
		}
	})
	t.Run("未知側の種族が不正ならエラー", func(t *testing.T) {
		in := valid()
		in.UnknownSpecies.Types = nil
		if _, err := calcReverse(in); err == nil {
			t.Fatal("タイプなしの種族を受け入れてしまった")
		}
	})
}

// ---------------------------------------------------------------------------
// AC-10: 純粋性(入力を変更しない)
// ---------------------------------------------------------------------------

func TestReverseDoesNotMutateInput(t *testing.T) {
	vest := revVest()
	items := []*Item{nil, vest}
	obs := []Observation{{Percent: 40}, {Percent: 45}}
	in := ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
		UnknownSpecies: revDefenderSpecies(), Move: revMove(CategoryPhysical),
		ItemCandidates: items, Observations: obs,
	}
	before := ReverseInput{
		Format: in.Format, Side: in.Side, Known: in.Known,
		UnknownSpecies: in.UnknownSpecies, Move: in.Move,
		ItemCandidates: append([]*Item(nil), items...),
		Observations:   append([]Observation(nil), obs...),
	}
	beforeVest := *vest

	if _, err := calcReverse(in); err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	if !reflect.DeepEqual(in.Known, before.Known) {
		t.Error("CalcReverse が Known を変更した")
	}
	if !reflect.DeepEqual(in.UnknownSpecies, before.UnknownSpecies) {
		t.Error("CalcReverse が UnknownSpecies を変更した")
	}
	if !reflect.DeepEqual(in.ItemCandidates, before.ItemCandidates) {
		t.Error("CalcReverse が ItemCandidates を変更した")
	}
	if !reflect.DeepEqual(in.Observations, before.Observations) {
		t.Error("CalcReverse が Observations を変更した")
	}
	if !reflect.DeepEqual(*vest, beforeVest) {
		t.Error("CalcReverse が渡された *Item の中身を変更した")
	}
}

// ---------------------------------------------------------------------------
// AC-11: 旧仕様(型・格子)の公開 API を残さない / ダメージ%に float を使わない
// ---------------------------------------------------------------------------

func TestReverseLegacyAPIRemoved(t *testing.T) {
	removed := map[string]bool{
		"Archetype": true, "ArchetypeKey": true, "ArchetypeOf": true, "ReverseArchetypes": true,
		"SPBucket": true, "SPBucketNone": true, "SPBucketMid": true, "SPBucketFull": true,
		"NatureClassMinus": true,
		// 観測%を round-half-up の整数に丸める仮定の関数。区間モデル(Observation.Matches)に置き換える。
		"ObservedPercent": true,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && removed[d.Name.Name] {
					t.Errorf("%s: 旧仕様の関数 %s が残っている(ADR-0010 §R6)", f, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						if removed[s.Name.Name] {
							t.Errorf("%s: 旧仕様の型 %s が残っている(ADR-0010 §R6)", f, s.Name.Name)
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if removed[n.Name] {
								t.Errorf("%s: 旧仕様の定数 %s が残っている(ADR-0010 §R6)", f, n.Name)
							}
						}
					}
				}
			}
		}
	}

	for _, name := range []string{"Archetype", "MatchScore", "Points", "ExactPoints", "SP"} {
		if _, ok := reflect.TypeOf(ReverseCandidate{}).FieldByName(name); ok {
			t.Errorf("ReverseCandidate に旧仕様のフィールド %s が残っている", name)
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(ReverseCandidate{}), reflect.TypeOf(Observation{}), reflect.TypeOf(SPRange{})} {
		for i := 0; i < typ.NumField(); i++ {
			if k := typ.Field(i).Type.Kind(); k == reflect.Float32 || k == reflect.Float64 {
				t.Errorf("%s.%s が float(ダメージ%%・一致度は整数で持つ。CLAUDE.md ドメイン規約)", typ.Name(), typ.Field(i).Name)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC-12: Recall(小標本の早期検知)
//
// ★ 合格基準そのものではない。合格基準は engine/reverse_recall_test.go の
//    TestAllSpeciesReverseRecall(allspecies タグ、全種族・固定シード 1,000 ケース)。
//    被覆と厳密性は決定的な性質なので、小標本でも 100% を要求する(標本誤差の余地が無い)。
// ---------------------------------------------------------------------------

func TestReverseRecallSmoke(t *testing.T) {
	const (
		cases     = 60
		fixedSeed = 0x50314238 // "P1B8"
	)
	for _, side := range []ReverseSide{SideDefender, SideAttacker} {
		for _, nObs := range []int{1, 2} {
			for _, rule := range allObsRoundings {
				st := reverseRecall(t, side, smokeSpecies(), cases, nObs, fixedSeed, rule)
				t.Logf("side=%s 観測%d件 %s: %s", side, nObs, rule, st)
				if st.hit != st.total || st.covered != st.total || st.tightViolations != 0 || st.narrowViolations != 0 {
					t.Errorf("side=%s 観測%d件 %s: %s(被覆・厳密性・絞り込みはすべて 100%% でなければならない)",
						side, nObs, rule, st)
				}
			}
		}
	}
}

// smokeSpecies は小標本テスト用の種族(耐久・タイプを散らした12件)。
func smokeSpecies() []Species {
	return []Species{
		{Key: "s01", Types: []Type{TypeNormal}, BaseStats: Stats{HP: 55, Atk: 81, Def: 60, SpA: 50, SpD: 70, Spe: 97}},
		{Key: "s02", Types: []Type{TypeWater, TypeGround}, BaseStats: Stats{HP: 100, Atk: 100, Def: 90, SpA: 85, SpD: 90, Spe: 60}},
		{Key: "s03", Types: []Type{TypeSteel, TypeFairy}, BaseStats: Stats{HP: 80, Atk: 50, Def: 115, SpA: 80, SpD: 95, Spe: 30}},
		{Key: "s04", Types: []Type{TypeFire}, BaseStats: Stats{HP: 78, Atk: 84, Def: 78, SpA: 109, SpD: 85, Spe: 100}},
		{Key: "s05", Types: []Type{TypeDragon, TypeGround}, BaseStats: Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102}},
		{Key: "s06", Types: []Type{TypeGhost}, BaseStats: Stats{HP: 45, Atk: 30, Def: 35, SpA: 20, SpD: 20, Spe: 45}},
		{Key: "s07", Types: []Type{TypeRock, TypeIce}, BaseStats: Stats{HP: 95, Atk: 95, Def: 180, SpA: 95, SpD: 45, Spe: 70}},
		{Key: "s08", Types: []Type{TypePsychic}, BaseStats: Stats{HP: 190, Atk: 33, Def: 58, SpA: 33, SpD: 58, Spe: 33}},
		{Key: "s09", Types: []Type{TypeDark, TypeFlying}, BaseStats: Stats{HP: 100, Atk: 125, Def: 52, SpA: 105, SpD: 52, Spe: 71}},
		{Key: "s10", Types: []Type{TypeGrass, TypePoison}, BaseStats: Stats{HP: 80, Atk: 82, Def: 83, SpA: 100, SpD: 100, Spe: 80}},
		{Key: "s11", Types: []Type{TypeElectric}, BaseStats: Stats{HP: 35, Atk: 55, Def: 40, SpA: 50, SpD: 50, Spe: 90}},
		{Key: "s12", Types: []Type{TypeFighting, TypeBug}, BaseStats: Stats{HP: 70, Atk: 110, Def: 80, SpA: 55, SpD: 80, Spe: 105}},
	}
}

// ---------------------------------------------------------------------------
// Recall テストの共通ヘルパー(ADR-0010 §R5)
//
// TestReverseRecallSmoke(小標本・タグなし)と TestAllSpeciesReverseRecall(全種族・allspecies)が
// 同じ手順を共有するため、タグの付いていないこのファイルに置く。
// ---------------------------------------------------------------------------

// revRNG は xorshift64。固定シードの再現性を Go の math/rand の実装に依存させない。
type revRNG struct{ s uint64 }

func newRevRNG(seed uint64) *revRNG {
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15
	}
	return &revRNG{s: seed}
}

func (r *revRNG) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func (r *revRNG) intn(n int) int { return int(r.next() % uint64(n)) }

// revRecallTypes は代表技のタイプ18種。
var revRecallTypes = []Type{
	TypeNormal, TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce,
	TypeFighting, TypePoison, TypeGround, TypeFlying, TypePsychic, TypeBug,
	TypeRock, TypeGhost, TypeDragon, TypeDark, TypeSteel, TypeFairy,
}

// revRecallPowers は代表技の威力。
var revRecallPowers = []int{60, 80, 90, 100, 120}

// revRecallItems は逆算に渡す持ち物候補。真値もこの中から選ぶ。
// requirements.md「逆算の持ち物候補」の分類ごとに1つずつ(効果はテスト側で組み立てる。ADR-0005):
//
//	防御側: なし / 防御・特防を上げる持ち物 / 半減きのみ(技のタイプ)
//	攻撃側: なし / 最終ダメージを上げる / 分類の威力を上げる / タイプ強化(技のタイプ) / 抜群時だけ上げる
//
// 攻撃側の持ち物は倍率が近く、SP・性格と観測上ほとんど区別できない。これを「決め打ちせず残す」ことが
// 新仕様の要点なので、あえて全分類を入れる。
func revRecallItems(side ReverseSide, cat MoveCategory, moveType Type) []*Item {
	if side == SideDefender {
		return []*Item{nil,
			{ID: "defboost", NameJa: "テストもちもの1", Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: 6144, StatSpD: 6144}}},
			{ID: "berry", NameJa: "テストきのみ", Effect: &ItemEffect{ResistBerryType: moveType}},
		}
	}
	return []*Item{nil,
		{ID: "dmgup", NameJa: "テストもちもの5", Effect: &ItemEffect{DamageMod: 5324}},
		{ID: "powup", NameJa: "テストもちもの6", Effect: &ItemEffect{PowerMod: 4505, PowerCategory: cat}},
		{ID: "typeup", NameJa: "テストもちもの7", Effect: &ItemEffect{BoostType: moveType, BoostTypeMod: 4915}},
		{ID: "seup", NameJa: "テストもちもの8", Effect: &ItemEffect{DamageMod: 4915, OnlySuperEffective: true}},
	}
}

// recallStats は Recall テストの集計。
type recallStats struct {
	total int
	// hit: 真値の候補(性格クラス・持ち物)が説明可能で、真値の SP がその範囲に入り、
	// かつその範囲が総当たりの正解と完全に一致する(広すぎない)ケース数。= Recall の分子(ADR-0010 §R5)
	hit int
	// covered: 真値の候補が説明可能で、真値の SP が範囲に入ったケース数(被覆)。
	covered int
	// tightViolations: 総当たりの正解と食い違った候補の延べ数(全候補が対象)。
	tightViolations int
	// narrowViolations: 観測を足したのに説明可能な SP が増えた候補の延べ数(nObs >= 2 のとき)。
	narrowViolations int
	// 情報量(参考。基準ではない): 真値の候補の SP 数の合計、説明可能な候補数の合計、真値の候補の順位。
	truthSPSum, exactSum, top1, top5 int
}

func (s recallStats) String() string {
	if s.total == 0 {
		return "ケース 0 件"
	}
	pct := func(n int) int { return 100 * n / s.total }
	avg := func(n int) float64 { return float64(n) / float64(s.total) }
	return fmt.Sprintf("Recall=%d%% (%d/%d) 被覆=%d 厳密性違反=%d 絞り込み違反=%d "+
		"真値の範囲の平均SP数=%.1f 説明可能な候補の平均数=%.1f 真値の順位 1位=%d%% / 5位以内=%d%%(参考)",
		pct(s.hit), s.hit, s.total, s.covered, s.tightViolations, s.narrowViolations,
		avg(s.truthSPSum), avg(s.exactSum), pct(s.top1), pct(s.top5))
}

// reverseRecall は ADR-0010 §R5 の手順で cases 件の Recall を測る。
//
// 真値の作り方:
//  1. 種族は species から一様に2体(既知側・未知側)
//  2. 技は タイプ18 × 分類2 × 威力5 の代表技から一様に
//  3. 真値(未知側): 関連ステータスの SP は 0..32 から一様、性格クラスは {補正なし, 上昇} から一様、
//     持ち物は revRecallItems から一様。防御側の HP は SP 32(新仕様の前提)
//  4. 既知側は現実的な調整(無振り / 関連ステータス32 + 上昇補正)
//  5. ダメージ0・無効相性・観測0% は観測にならないので引き直す。100% 超は 100% に丸めて残す
//  6. 16ロールから一様に nObs 段階を独立に選び、丸め規則 rule(整数%)で観測化する
func reverseRecall(t *testing.T, side ReverseSide, species []Species, cases, nObs int, seed uint64, rule obsRounding) recallStats {
	t.Helper()
	if len(species) == 0 {
		t.Fatal("種族集合が空")
	}
	r := newRevRNG(seed)
	const maxAttempts = 1 << 16
	attempts := 0
	var st recallStats

	for st.total < cases {
		attempts++
		if attempts > maxAttempts {
			t.Fatalf("観測を作れるケースが %d 回引いても %d 件しか集まらない", maxAttempts, st.total)
		}

		knownSpecies := species[r.intn(len(species))]
		unknownSpecies := species[r.intn(len(species))]
		cat := CategoryPhysical
		if r.intn(2) == 0 {
			cat = CategorySpecial
		}
		move := Move{
			ID: "recall", Type: revRecallTypes[r.intn(len(revRecallTypes))],
			Category: cat, Power: revRecallPowers[r.intn(len(revRecallPowers))],
		}
		atkStat, defStat := StatAtk, StatDef
		if cat == CategorySpecial {
			atkStat, defStat = StatSpA, StatSpD
		}
		items := revRecallItems(side, cat, move.Type)

		// --- 真値(未知側) ---
		xStat := revStatFor(side, cat)
		truthX := r.intn(MaxSPPerStat + 1)
		truthClass := revClasses[r.intn(len(revClasses))]
		itemIdx := r.intn(len(items))
		truthSP := Stats{}.WithStat(xStat, truthX)
		if side == SideDefender {
			truthSP.HP = MaxSPPerStat
		}
		truth := Individual{Species: unknownSpecies, Level: DefaultLevel, Nature: revNatureFor(xStat, truthClass),
			SP: truthSP, Item: items[itemIdx], Status: StatusNone}

		// --- 既知側(自分): 現実的な調整 ---
		known := Individual{Species: knownSpecies, Level: DefaultLevel, Status: StatusNone}
		if r.intn(2) == 0 {
			if side == SideDefender {
				known.SP = Stats{}.WithStat(atkStat, MaxSPPerStat)
				known.Nature = revNatureFor(atkStat, NatureClassPlus)
			} else {
				known.SP = Stats{HP: MaxSPPerStat}.WithStat(defStat, MaxSPPerStat)
				known.Nature = revNatureFor(defStat, NatureClassPlus)
			}
		}

		// --- 観測を作る ---
		dmgIn := DamageInput{Format: FormatSingle, Move: move}
		if side == SideDefender {
			dmgIn.Attacker, dmgIn.Defender = known, truth
		} else {
			dmgIn.Attacker, dmgIn.Defender = truth, known
		}
		res, err := calcDamage(dmgIn)
		if err != nil {
			continue
		}
		if res.Effectiveness == 0 || res.MinDamage() == 0 || res.DefenderHP <= 0 {
			continue
		}
		obs := make([]Observation, 0, nObs)
		ok := true
		for k := 0; k < nObs; k++ {
			v := roundObserved(res.Rolls[r.intn(16)], res.DefenderHP, 100, rule)
			if v <= 0 {
				ok = false
				break
			}
			obs = append(obs, Observation{Percent: v})
		}
		if !ok {
			continue
		}
		st.total++

		in := ReverseInput{Format: FormatSingle, Side: side, Known: known, UnknownSpecies: unknownSpecies,
			Move: move, ItemCandidates: items, Observations: obs}
		out, err := calcReverse(in)
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		oracle := bruteForceReverse(t, in)

		truthKey := revKey{truthClass, revItemID(items[itemIdx])}
		tight := func(c ReverseCandidate) bool {
			w := oracle[revKey{c.NatureClass, c.ItemID}]
			return c.Exact == (len(w.exact) > 0) && slices.Equal(spsOf(c.Ranges), w.ranges)
		}
		for i, c := range out.Candidates {
			if !tight(c) {
				st.tightViolations++
			}
			if c.Exact {
				st.exactSum++
			}
			if (revKey{c.NatureClass, c.ItemID}) != truthKey {
				continue
			}
			in := c.Exact && slices.Contains(spsOf(c.Ranges), truthX)
			if in {
				st.covered++
				if tight(c) {
					st.hit++
				}
			}
			st.truthSPSum += c.SPCount
			if i == 0 {
				st.top1++
			}
			if i < 5 {
				st.top5++
			}
		}
		if len(out.Candidates) != len(oracle) {
			st.tightViolations += len(oracle)
		}

		// --- 絞り込み: 1件目だけの結果に対して、説明可能な SP が増えていないこと ---
		if nObs >= 2 {
			in1 := in
			in1.Observations = obs[:1]
			out1, err := calcReverse(in1)
			if err != nil {
				t.Fatalf("CalcReverse(1件): %v", err)
			}
			for _, c := range out.Candidates {
				if !c.Exact {
					continue
				}
				_, p := findCand(out1.Candidates, c.NatureClass, c.ItemID)
				if p == nil || !p.Exact {
					st.narrowViolations++
					continue
				}
				prev := spsOf(p.Ranges)
				for _, x := range spsOf(c.Ranges) {
					if !slices.Contains(prev, x) {
						st.narrowViolations++
						break
					}
				}
			}
		}
	}
	return st
}
