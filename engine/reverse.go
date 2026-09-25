package engine

// 逆算(調整推定、P1-8。P1-12 で再設計)。観測したダメージから相手の調整候補を返す。
//
// 要件(docs/requirements.md「調整の推定(逆算)」):
//   - 与えたダメージ(相手 HP の減少%)→ 相手の H/B(D) 配分・性格・持ち物の候補
//   - 受けたダメージ(自分 HP の減少量)→ 相手の A(C) 配分・性格・持ち物の候補
//   - 候補は一致度の高い順に表示。同じ相手の観測を複数入力すると候補を絞り込める
//   - 正確さより候補の提示を優先する(完全一致が無くても近い候補を返す)
//
// 設計は ADR-0010 §R(P1-12 改訂):
//   - CalcReverse は CalcDamage の合成にすぎない。独自のダメージ式を書いてはならない。
//   - 防御側は H32(HP SP=32)を前提に、関連ステータスの SP を 0..32 で総当たりする。
//     攻撃側は関連ステータスの SP を 0..32 で総当たりする(H は計算に使わない)。
//   - 性格は「補正なし」「関連ステータス上昇」の2通りだけ探索する(下降補正は探索しない)。
//   - 持ち物候補は解決済みの []*Item を引数で受け取る。engine に持ち物一覧は持ち込まない(ADR-0005)。
//   - 結果は候補(性格クラス × 持ち物)ごとの SP の範囲。観測を説明できる SP の集合そのものを返し、
//     区別できない候補は決め打ちせず残す。
//   - 順序は Mismatch 昇順 → Support 降順 → SPCount 降順 → 定義順 の全順序で、常に決定的。

import (
	"errors"
	"fmt"
	"sort"
)

// 逆算の入力不正。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidReverseSide は Side が defender / attacker のどちらでもない。
	ErrInvalidReverseSide = errors.New("逆算の対象側が不正")
	// ErrNoObservation は観測が1件も無い。
	ErrNoObservation = errors.New("観測が1件も無い")
	// ErrInvalidObservation は観測の指定が不正(Percent・PercentTenths・Damage のうち
	// ちょうど1つでない、または範囲外)。
	ErrInvalidObservation = errors.New("観測の指定が不正")
	// ErrTooManyItemCandidates は ItemCandidates の件数が MaxReverseItemCandidates を超えている
	// (issue #110。ADR-0208 §4・ADR-0108)。HTTP を経由しない直接呼び出し・WASM でも
	// 探索コスト(性格クラス × 持ち物 × 33 SP × 観測 × 16 ロール)を増幅させないための防御。
	ErrTooManyItemCandidates = errors.New("持ち物候補の件数が上限を超えている")
	// ErrTooManyObservations は Observations の件数が MaxReverseObservations を超えている
	// (issue #110。ADR-0208 §4・ADR-0108)。
	ErrTooManyObservations = errors.New("観測の件数が上限を超えている")
	// ErrInvalidMaxCandidates は MaxCandidates が 0(無制限)でも 1..MaxReverseMaxCandidates
	// の範囲内でもない(負・上限超過。issue #110。ADR-0208 §4・ADR-0108)。
	ErrInvalidMaxCandidates = errors.New("MaxCandidates の範囲が不正")
	// ErrMoveDealsNoDamage は技がダメージを与えられないため観測を説明できない(issue #317。ADR-0117 §3)。
	// 変化技・威力 0 以下の技は探索の前に、タイプ相性・特性の無効で全候補の全ロールが 0 のときは
	// 探索の後に返す。観測は必ず正の値(validateObservations)なので、ダメージ 0 では説明できない。
	ErrMoveDealsNoDamage = errors.New("この技ではダメージが出ないため逆算できない")
)

// 件数の上限(issue #110。ADR-0208 §1 の契約値と同じ。ADR-0108)。
const (
	// MaxReverseItemCandidates は ItemCandidates の件数上限。
	MaxReverseItemCandidates = 64
	// MaxReverseObservations は Observations の件数上限(minItems 1 は validateObservations が別に見る)。
	MaxReverseObservations = 16
	// MaxReverseMaxCandidates は MaxCandidates が 0(無制限)でないときに許される上限。
	MaxReverseMaxCandidates = 128
)

// ReverseSide はどちら側の調整を逆算するか。
type ReverseSide string

const (
	// SideDefender は「自分が与えたダメージ」から相手の防御側(H と B/D)を逆算する。
	SideDefender ReverseSide = "defender"
	// SideAttacker は「自分が受けたダメージ」から相手の攻撃側(A/C)を逆算する。
	SideAttacker ReverseSide = "attacker"
)

// NatureClass は関連ステータスに対する性格補正のクラス。
// 下降補正は探索しない(ユーザー決定。ADR-0010 §R1)。
type NatureClass string

const (
	NatureClassNeutral NatureClass = "neutral" // 無補正
	NatureClassPlus    NatureClass = "plus"    // 関連ステータスが +10%
)

// reverseClasses は候補の性格クラス(定義順)。ADR-0010 §R1。
var reverseClasses = []NatureClass{NatureClassNeutral, NatureClassPlus}

// Observation は1発ぶんの観測。Percent / PercentTenths / Damage の**ちょうど1つ**を指定する
// (ADR-0010 §R2)。float は使わない(CLAUDE.md ドメイン規約)。
type Observation struct {
	// Percent は整数%の観測(1..100)。精度 1%。
	Percent int
	// PercentTenths は小数第1位の観測(1..1000。0.1% 単位の整数)。精度 0.1%。
	PercentTenths int
	// Damage は HP の実点数(> 0)。精度 1 点。
	Damage int
	// Note は画面用のメモ。計算には使わない。
	Note string
}

// Matches は damage(実点数)が maxHP に対してこの観測を説明できるかを返す(ADR-0010 §R2)。
//
// 真の値 p(観測と同じ単位、S = 100 or 1000)が開区間 (v-1, v+1) に入るかで判定する。
// これは「切り捨て・四捨五入・切り上げのどの丸め規則で観測 v を作っても真値が入る」最小の区間と同値。
// v が精度の最大値(100% / 100.0%)のときは、HP バーが頭打ちになるので上側を開ける
// (99%超のダメージすべてと両立する)。
// maxHP <= 0 は照合できない(false。panic しない)。
func (o Observation) Matches(damage, maxHP int) bool {
	if maxHP <= 0 {
		return false
	}
	if o.Damage != 0 {
		return damage == o.Damage
	}
	scale, v := 100, o.Percent
	if o.PercentTenths != 0 {
		scale, v = 1000, o.PercentTenths
	}
	x := scale*damage - v*maxHP
	ax := x
	if ax < 0 {
		ax = -ax
	}
	if ax < maxHP {
		return true
	}
	return v == scale && x >= 0
}

// distanceUnreachable は maxHP <= 0(照合できない)ときの距離。0 にはしない
// (「0 ⇔ 説明できる」を守るため。ADR-0010 §R2)。
const distanceUnreachable = 1 << 30

// Distance は Matches が false のときの「近さ」を返す(0.1% 単位の整数。0 ⇔ Matches)。
// ADR-0010 §R2。
func (o Observation) Distance(damage, maxHP int) int {
	if maxHP <= 0 {
		return distanceUnreachable
	}
	if o.Damage != 0 {
		if damage == o.Damage {
			return 0
		}
		d := damage - o.Damage
		if d < 0 {
			d = -d
		}
		return max(1, 1000*d/maxHP)
	}
	if o.Matches(damage, maxHP) {
		return 0
	}
	scale, v, unit := 100, o.Percent, 10
	if o.PercentTenths != 0 {
		scale, v, unit = 1000, o.PercentTenths, 1
	}
	x := scale*damage - v*maxHP
	if x < 0 {
		x = -x
	}
	return (x / maxHP) * unit
}

// SPRange は候補が返す SP の1区間(両端を含む)。
type SPRange struct{ Min, Max int }

// ReverseInput は逆算の入力。場・急所・技・既知側は全観測で共通(ADR-0010 §8)。
type ReverseInput struct {
	Format Format
	Side   ReverseSide
	// Known は既知側の個体。Side=defender なら攻撃側(自分)、Side=attacker なら防御側(自分)。
	Known Individual
	// UnknownSpecies は逆算する相手の種族。SP・性格・持ち物は探索対象なので渡さない。
	UnknownSpecies Species
	Move           Move
	Field          Field
	Critical       bool
	// TypeChart はタイプ相性表(ADR-0013)。CalcReverse は解釈せず DamageInput へ素通しする。
	TypeChart TypeChart
	// ItemCandidates は探索する持ち物(解決済み)。nil 要素は「持ち物なし」。
	// nil / 空スライスは []*Item{nil} と同じ(持ち物なしの1通り)。
	ItemCandidates []*Item
	// Observations は1件以上。すべて同じ技・同じ場・同じ既知側に対する別々の1発。
	Observations []Observation
	// MaxCandidates は返す候補数の上限。0 は無制限。1..MaxReverseMaxCandidates は上限として使う。
	// 負・MaxReverseMaxCandidates 超過は ErrInvalidMaxCandidates(issue #110。ADR-0108 決定4)。
	MaxCandidates int
}

// ReverseCandidate は候補1件(性格クラス × 持ち物。ADR-0010 §R3)。
type ReverseCandidate struct {
	NatureClass NatureClass
	// Nature は性格クラスの代表 Nature(構造値)。
	Nature Nature
	Item   *Item  // 渡された *Item をそのまま保持する(nil は持ち物なし)
	ItemID string // Item.ID。Item が nil なら空文字

	// Ranges は観測を説明できる SP の集合(昇順・互いに素・隣接しない極大連続区間)。
	// 空にならない。説明できる SP が無ければ、距離が最小の SP を返す。
	Ranges  []SPRange
	SPCount int // Ranges に含まれる SP の数

	// Exact は Mismatch == 0 と同値。
	Exact bool
	// Mismatch は min_SP Σ_観測 min_ロール Distance(0.1% 単位)。
	Mismatch int
	// Support は Ranges の各 SP で、各観測を説明できるロールの延べ数。
	Support int

	// MinPercentTenths / MaxPercentTenths は Ranges 全体での想定ダメージ幅(表示%。0.1% 単位)。
	MinPercentTenths, MaxPercentTenths int
}

// ReverseResult は逆算の結果。Candidates は ADR-0010 §R4 の全順序で並ぶ。
type ReverseResult struct {
	Side ReverseSide
	// Stat は逆算した関連ステータス(技の分類から決まる)。
	Stat StatKey
	// AssumedHPSP は仮定した H の SP(defender=32、attacker=0。ADR-0010 §R1)。
	AssumedHPSP int
	// Candidates は Mismatch 昇順 → Support 降順 → SPCount 降順 → 定義順 の順。
	Candidates []ReverseCandidate
	// ExactCount は Exact な候補の数(切り取り前。MaxCandidates で変わらない)。
	ExactCount int
}

// reverseStat は side と技の分類から逆算する関連ステータスを決める(ADR-0010 §2)。
// 未知の分類は物理と同じ扱い(変化技は CalcReverse が探索の前に ErrMoveDealsNoDamage で拒否する。#317)。
func reverseStat(side ReverseSide, category MoveCategory) StatKey {
	special := category == CategorySpecial
	switch {
	case side == SideAttacker && special:
		return StatSpA
	case side == SideAttacker:
		return StatAtk
	case special:
		return StatSpD
	}
	return StatDef
}

// natureForClass は関連ステータス stat に対する性格クラスの代表 Nature を返す(ADR-0010 §R1)。
// 上昇補正の相手(下降側)は、この計算で使われない atk / spa に置く。
func natureForClass(stat StatKey, c NatureClass) Nature {
	if c != NatureClassPlus {
		return NatureNeutral
	}
	other := StatAtk
	if stat == StatAtk {
		other = StatSpA
	}
	return Nature{Plus: stat, Minus: other}
}

// validateObservations は観測列を検証する(ADR-0010 §R2)。
func validateObservations(obs []Observation) error {
	if len(obs) == 0 {
		return ErrNoObservation
	}
	for i, o := range obs {
		n := 0
		if o.Percent != 0 {
			n++
		}
		if o.PercentTenths != 0 {
			n++
		}
		if o.Damage != 0 {
			n++
		}
		switch {
		case n != 1:
			return fmt.Errorf("%w: 観測 %d は Percent・PercentTenths・Damage のうちちょうど1つを指定すること", ErrInvalidObservation, i+1)
		case o.Percent < 0 || o.Percent > 100:
			return fmt.Errorf("%w: 観測 %d の Percent は 1..100 の範囲外: %d", ErrInvalidObservation, i+1, o.Percent)
		case o.PercentTenths < 0 || o.PercentTenths > 1000:
			return fmt.Errorf("%w: 観測 %d の PercentTenths は 1..1000 の範囲外: %d", ErrInvalidObservation, i+1, o.PercentTenths)
		case o.Damage < 0:
			return fmt.Errorf("%w: 観測 %d の Damage は正でなければならない: %d", ErrInvalidObservation, i+1, o.Damage)
		}
	}
	return nil
}

// collapseSPRanges は昇順の SP 列を、隣接する値をまとめた極大連続区間の列にする。
// 空を渡すと nil を返す。
func collapseSPRanges(xs []int) []SPRange {
	if len(xs) == 0 {
		return nil
	}
	out := make([]SPRange, 0, len(xs))
	start, prev := xs[0], xs[0]
	for _, x := range xs[1:] {
		if x == prev+1 {
			prev = x
			continue
		}
		out = append(out, SPRange{Min: start, Max: prev})
		start, prev = x, x
	}
	out = append(out, SPRange{Min: start, Max: prev})
	return out
}

// CalcReverse は観測ダメージから相手の調整候補を返す。
// CalcDamage の合成のみで行い、独自のダメージ式は書かない(ADR-0010 §1)。
// 候補(性格クラス × 持ち物)ごとに CalcDamage を SP 0..32 の33回だけ呼び、
// 16ロールを全観測で使い回す。エラー時は部分的な結果を返さない。
func CalcReverse(in ReverseInput) (ReverseResult, error) {
	if in.Side != SideDefender && in.Side != SideAttacker {
		return ReverseResult{}, fmt.Errorf("%w: %q", ErrInvalidReverseSide, in.Side)
	}
	// 件数・範囲の上限は、観測の中身の検証より前に見る(issue #110。ADR-0208 §4・ADR-0108)。
	// 巨大な入力に対して以降の一切の追加の仕事をしないため。
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

	cands := make([]ReverseCandidate, 0, len(reverseClasses)*len(items))
	// dealsDamage はどれか1つの候補でダメージが出たか(全候補で 0 なら ErrMoveDealsNoDamage)。
	dealsDamage := false
	for _, class := range reverseClasses {
		nature := natureForClass(stat, class)
		for _, item := range items {
			var rolls [MaxSPPerStat + 1]DamageResult
			dist := make([]int, MaxSPPerStat+1)

			for x := 0; x <= MaxSPPerStat; x++ {
				sp := Stats{}.WithStat(stat, x)
				if in.Side == SideDefender {
					sp.HP = MaxSPPerStat
				}
				unknown := Individual{
					Species: in.UnknownSpecies, Level: DefaultLevel, Nature: nature,
					SP: sp, Item: item, Status: StatusNone,
				}
				dmg := DamageInput{
					Format: in.Format, Move: in.Move, Field: in.Field, Critical: in.Critical,
					TypeChart: in.TypeChart,
				}
				if in.Side == SideDefender {
					dmg.Attacker, dmg.Defender = in.Known, unknown
				} else {
					dmg.Attacker, dmg.Defender = unknown, in.Known
				}
				res, err := CalcDamage(dmg)
				if err != nil {
					return ReverseResult{}, fmt.Errorf("逆算の候補(性格クラス=%s, 持ち物=%q, SP=%d)の計算: %w",
						class, itemID(item), x, err)
				}
				rolls[x] = res
				if res.Rolls[len(res.Rolls)-1] > 0 {
					dealsDamage = true
				}

				sum := 0
				for _, o := range in.Observations {
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

			mismatch := dist[0]
			for _, d := range dist[1:] {
				if d < mismatch {
					mismatch = d
				}
			}
			var sps []int
			for x, d := range dist {
				if d == mismatch {
					sps = append(sps, x)
				}
			}

			support, minT, maxT := 0, -1, -1
			for _, x := range sps {
				res := rolls[x]
				for _, o := range in.Observations {
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

			c := ReverseCandidate{
				NatureClass:      class,
				Nature:           nature,
				Item:             item,
				Ranges:           collapseSPRanges(sps),
				SPCount:          len(sps),
				Exact:            mismatch == 0,
				Mismatch:         mismatch,
				Support:          support,
				MinPercentTenths: minT,
				MaxPercentTenths: maxT,
			}
			if item != nil {
				c.ItemID = item.ID
			}
			cands = append(cands, c)
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

	// 全順序: Mismatch 昇順 → Support 降順 → SPCount 降順 → 定義順(ADR-0010 §R4)。
	// cands は既に定義順(性格クラス → 持ち物添字)で並んでいるので、安定ソートで
	// 同点はそのまま定義順に残る。
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

// itemID は Item.ID を返す(nil は空文字)。
func itemID(it *Item) string {
	if it == nil {
		return ""
	}
	return it.ID
}
