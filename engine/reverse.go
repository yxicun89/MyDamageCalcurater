package engine

// 逆算(調整推定、P1-8)。観測したダメージから相手の調整候補を「型」にまとめて返す。
//
// 要件(docs/requirements.md「調整の推定(逆算)」):
//   - 与えたダメージ(相手 HP の減少%)→ 相手の H/B(D) 配分・性格・持ち物の候補
//   - 受けたダメージ(自分 HP の減少量)→ 相手の A(C) 配分・性格・持ち物の候補
//   - 候補は「無振り」「HB特化」などの名前付きの型にまとめ、一致度の高い順に表示
//   - 同じ相手の観測を複数入力すると候補を絞り込める
//   - 正確さより候補の提示を優先する(完全一致が無くても近い候補を返す)
//
// 設計は ADR-0010:
//   - CalcReverse は CalcDamage の合成にすぎない。独自のダメージ式を書いてはならない。
//   - 未知側の SP を全格子で総当たりする(CLAUDE.md ドメイン規約)。格子は小さいので WASM でも動く。
//   - 持ち物候補は解決済みの []*Item を引数で受け取る。engine に持ち物一覧は持ち込まない(ADR-0005)。
//   - 順序は MatchScore 降順 → 型の事前順位 → 持ち物候補の添字 の全順序で、常に決定的。

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// 逆算の入力不正。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidReverseSide は Side が defender / attacker のどちらでもない。
	ErrInvalidReverseSide = errors.New("逆算の対象側が不正")
	// ErrNoObservation は観測が1件も無い。
	ErrNoObservation = errors.New("観測が1件も無い")
	// ErrInvalidObservation は観測の指定が不正(Percent と Damage の同時指定・両方未指定・範囲外)。
	ErrInvalidObservation = errors.New("観測の指定が不正")
)

// ReverseSide はどちら側の調整を逆算するか。
type ReverseSide string

const (
	// SideDefender は「自分が与えたダメージ」から相手の防御側(H と B/D)を逆算する。
	SideDefender ReverseSide = "defender"
	// SideAttacker は「自分が受けたダメージ」から相手の攻撃側(A/C)を逆算する。
	SideAttacker ReverseSide = "attacker"
)

// SPBucket は SP を型に畳むときのバケット(ADR-0010 §5.1)。
type SPBucket string

const (
	SPBucketNone SPBucket = "none" // SP = 0(無振り)
	SPBucketMid  SPBucket = "mid"  // SP = 1..31(中間調整)
	SPBucketFull SPBucket = "full" // SP = 32(振り切り)
)

// NatureClass は関連ステータスに対する性格補正のクラス。
// 下降補正の置き場所は結果に影響しないため、3クラスに畳む(ADR-0010 §4)。
type NatureClass string

const (
	NatureClassPlus    NatureClass = "plus"    // 関連ステータスが +10%
	NatureClassNeutral NatureClass = "neutral" // 無補正
	NatureClassMinus   NatureClass = "minus"   // 関連ステータスが -10%
)

// ArchetypeKey は型の識別子。構造そのものを表す(ADR-0010 §5.2)。
// 例 "hfull-bnone-plus"(H振り + B補正)、"afull-plus"(A特化)。
// ADR-0009 のプリセット enum とは独立。表示名は Archetype.Label を使う。
type ArchetypeKey string

// Archetype は逆算候補の「型」。格子点を畳み込む単位で、どの格子点もちょうど1つの型に属する。
type Archetype struct {
	Key   ArchetypeKey
	Label string      // 表示名(例 "HB特化")。requirements の「名前付きの型」
	Side  ReverseSide // 攻撃側か防御側か
	Stat  StatKey     // 関連ステータス(def/spd/atk/spa)
	// HPBucket は H の SP バケット。Side=attacker では常に SPBucketNone(意味を持たない)。
	HPBucket    SPBucket
	StatBucket  SPBucket // 関連ステータスの SP バケット
	NatureClass NatureClass
}

// Observation は1発ぶんの観測(防御側の HP 減少)。Percent と Damage はどちらか一方だけを使う。
type Observation struct {
	// Percent はゲーム表示の整数%(1..100)。engine は float を持たない(ADR-0010 §3)。
	Percent int
	// Damage は HP の実点数(> 0)。自分が受けたダメージなど、点数で分かるとき。
	Damage int
	// Note は画面用のメモ。計算には使わない。
	Note string
}

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
	// MaxCandidates は返す候補数の上限。0 以下は無制限。
	MaxCandidates int
}

// ReverseCandidate は候補1件(型 × 持ち物)。
type ReverseCandidate struct {
	Archetype Archetype
	Item      *Item  // 渡された *Item をそのまま保持する(nil は持ち物なし)
	ItemID    string // Item.ID。Item が nil なら空文字

	// SP / Nature は型を代表する格子点(ADR-0010 §6.2)。
	SP     Stats
	Nature Nature

	// MatchScore は観測との一致度 (0,1]。1.0 は「全観測に完全一致する格子点がある」と同値。
	MatchScore float64
	// Exact は MatchScore == 1.0 と同値。画面と絞り込みの判定に使う。
	Exact bool

	// MinPercentTenths / MaxPercentTenths は代表点での想定ダメージ幅(表示%。0.1% 単位。
	// 最小側は切り捨て・最大側は四捨五入。ADR-0010 §3.3)。照合に使う観測%とは別物。
	MinPercentTenths int
	MaxPercentTenths int

	// Points は型に属する格子点数、ExactPoints は全観測に完全一致した格子点数。
	// 画面の参考値であり、順序には使わない(ADR-0010 §6.4)。
	Points      int
	ExactPoints int
}

// ReverseResult は逆算の結果。Candidates は ADR-0010 §6.3 の全順序で並ぶ。
type ReverseResult struct {
	Side ReverseSide
	// Stat は逆算した関連ステータス(技の分類から決まる)。
	Stat StatKey
	// Candidates は MatchScore 降順 → 型の事前順位 → 持ち物候補の添字 の順。
	Candidates []ReverseCandidate
	// ExactCount は Exact な候補の数。観測を足すと単調に非増加になる。
	ExactCount int
}

// ObservedPercent はダメージを観測%(ゲーム内表示の整数%)に丸める(ADR-0010 §3.1)。
// 逆算の入力との照合に使う値であり、アプリが画面に出す表示%(DisplayPercentTenths*)ではない。
//
// 丸めは round-half-up と仮定している = floor(100*damage/maxHP + 0.5)。
// float を使わず整数演算で行う(CLAUDE.md ドメイン規約)。
// 実機の丸めが確認できたら、この関数と ADR-0010 §3.1 だけを差し替える。
func ObservedPercent(damage, maxHP int) int {
	if maxHP <= 0 {
		return 0
	}
	return (damage*200 + maxHP) / (2 * maxHP)
}

// ---------------------------------------------------------------------------
// 型カタログ(ADR-0010 §5)
// ---------------------------------------------------------------------------

// reverseStat は side と技の分類から逆算する関連ステータスを決める(ADR-0010 §2)。
// 変化技・未知の分類は物理と同じ扱い。
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

// statLetter は型キー・ラベルに使う統計文字。
func statLetter(stat StatKey) string {
	switch stat {
	case StatAtk:
		return "A"
	case StatDef:
		return "B"
	case StatSpA:
		return "C"
	}
	return "D"
}

// spBucketOf は SP をバケットに畳む。範囲外は ok=false。
func spBucketOf(sp int) (SPBucket, bool) {
	switch {
	case sp < 0 || sp > MaxSPPerStat:
		return "", false
	case sp == 0:
		return SPBucketNone, true
	case sp == MaxSPPerStat:
		return SPBucketFull, true
	}
	return SPBucketMid, true
}

// バケット・性格クラスを添字にするための番号(型の逆引き表に使う)。
func spBucketIndex(b SPBucket) int {
	switch b {
	case SPBucketNone:
		return 0
	case SPBucketMid:
		return 1
	}
	return 2
}

func natureClassIndex(c NatureClass) int {
	switch c {
	case NatureClassPlus:
		return 0
	case NatureClassNeutral:
		return 1
	}
	return 2
}

// reverseNatureClasses は格子・カタログで使う性格クラスの走査順。
var reverseNatureClasses = [3]NatureClass{NatureClassPlus, NatureClassNeutral, NatureClassMinus}

// natureForClass は関連ステータス stat に対するクラスの代表 Nature を返す(ADR-0010 §4)。
// 下降側・上昇側の相手は、この計算で使われない atk / spa に置く。
func natureForClass(stat StatKey, c NatureClass) Nature {
	other := StatAtk
	if stat == StatAtk {
		other = StatSpA
	}
	switch c {
	case NatureClassPlus:
		return Nature{Plus: stat, Minus: other}
	case NatureClassMinus:
		return Nature{Plus: other, Minus: stat}
	}
	return NatureNeutral
}

// natureClassOf は Nature が関連ステータス stat に与える補正のクラスを返す。
func natureClassOf(stat StatKey, n Nature) NatureClass {
	switch stat {
	case n.Plus:
		return NatureClassPlus
	case n.Minus:
		return NatureClassMinus
	}
	return NatureClassNeutral
}

// catalogSlot は型カタログの1要素の構造(H バケット・関連ステータスのバケット・性格クラス)。
type catalogSlot struct {
	h, x  SPBucket
	class NatureClass
}

// 事前順位の先頭(現実的な調整)。ADR-0010 §5.3。ラベルの {X} は統計文字に置き換える。
var (
	reverseDefenderHead = []struct {
		slot  catalogSlot
		label string
	}{
		{catalogSlot{SPBucketNone, SPBucketNone, NatureClassNeutral}, "無振り"},
		{catalogSlot{SPBucketFull, SPBucketNone, NatureClassNeutral}, "H振り"},
		{catalogSlot{SPBucketFull, SPBucketNone, NatureClassPlus}, "H振り+{X}補正"},
		{catalogSlot{SPBucketFull, SPBucketFull, NatureClassPlus}, "H{X}特化"},
		{catalogSlot{SPBucketFull, SPBucketFull, NatureClassNeutral}, "H{X}振り(無補正)"},
		{catalogSlot{SPBucketNone, SPBucketFull, NatureClassPlus}, "{X}特化(H無振り)"},
		{catalogSlot{SPBucketNone, SPBucketFull, NatureClassNeutral}, "{X}振り(無補正)"},
		{catalogSlot{SPBucketNone, SPBucketNone, NatureClassPlus}, "{X}補正のみ"},
	}
	reverseAttackerHead = []struct {
		slot  catalogSlot
		label string
	}{
		{catalogSlot{SPBucketNone, SPBucketNone, NatureClassNeutral}, "無振り"},
		{catalogSlot{SPBucketNone, SPBucketFull, NatureClassPlus}, "{X}特化"},
		{catalogSlot{SPBucketNone, SPBucketFull, NatureClassNeutral}, "{X}振り(無補正)"},
		{catalogSlot{SPBucketNone, SPBucketNone, NatureClassPlus}, "{X}補正のみ"},
	}
	// 事前順位の先頭以降を並べる入れ子順。
	reverseTailBuckets = [3]SPBucket{SPBucketFull, SPBucketNone, SPBucketMid}
)

// generatedLabel は固定表に無い型のラベルを生成規則(ADR-0010 §5.2)で作る。
func generatedLabel(side ReverseSide, s catalogSlot, letter string) string {
	var parts []string
	if side == SideDefender {
		switch s.h {
		case SPBucketMid:
			parts = append(parts, "H中間")
		case SPBucketFull:
			parts = append(parts, "H振り")
		}
	}
	switch s.x {
	case SPBucketMid:
		parts = append(parts, letter+"中間")
	case SPBucketFull:
		parts = append(parts, letter+"振り")
	}
	switch s.class {
	case NatureClassPlus:
		parts = append(parts, letter+"補正")
	case NatureClassMinus:
		parts = append(parts, letter+"下降")
	}
	if len(parts) == 0 {
		return "無振り"
	}
	return strings.Join(parts, "+")
}

// newArchetype は構造から Archetype を組み立てる(キーは構造そのもの、ADR-0010 §5.2)。
func newArchetype(side ReverseSide, stat StatKey, s catalogSlot, label string) Archetype {
	letter := strings.ToLower(statLetter(stat))
	key := letter + string(s.x) + "-" + string(s.class)
	hb := s.h
	if side == SideDefender {
		key = "h" + string(s.h) + "-" + key
	} else {
		hb = SPBucketNone
	}
	if label == "" {
		label = generatedLabel(side, s, statLetter(stat))
	}
	return Archetype{
		Key:         ArchetypeKey(key),
		Label:       label,
		Side:        side,
		Stat:        stat,
		HPBucket:    hb,
		StatBucket:  s.x,
		NatureClass: s.class,
	}
}

// buildCatalog は型カタログを事前順位の順で作る(ADR-0010 §5.3)。
func buildCatalog(side ReverseSide, stat StatKey) []Archetype {
	letter := statLetter(stat)
	head := reverseDefenderHead
	hBuckets := reverseTailBuckets[:]
	if side == SideAttacker {
		head = reverseAttackerHead
		hBuckets = []SPBucket{SPBucketNone} // H の次元を持たない
	}

	out := make([]Archetype, 0, 27)
	seen := make(map[ArchetypeKey]bool, 27)
	add := func(s catalogSlot, label string) {
		a := newArchetype(side, stat, s, label)
		if seen[a.Key] {
			return
		}
		seen[a.Key] = true
		out = append(out, a)
	}
	for _, h := range head {
		add(h.slot, strings.ReplaceAll(h.label, "{X}", letter))
	}
	for _, hb := range hBuckets {
		for _, xb := range reverseTailBuckets {
			for _, c := range reverseNatureClasses {
				add(catalogSlot{hb, xb, c}, "")
			}
		}
	}
	return out
}

// ReverseArchetypes は side と技の分類に対する型カタログを、事前順位の順で返す
// (ADR-0010 §5.3)。返すスライスの添字がそのまま事前順位になる。
// 呼び出しごとに新しいスライスを返す。side が不正なら nil。
func ReverseArchetypes(side ReverseSide, category MoveCategory) []Archetype {
	if side != SideDefender && side != SideAttacker {
		return nil
	}
	return buildCatalog(side, reverseStat(side, category))
}

// ArchetypeOf は SP と性格が属する型を返す(ADR-0010 §5.1)。
// どの格子点もちょうど1つの型に属するので、格子内なら ok は true。
// Recall テストの「正解」の定義はこの写像そのもの(ADR-0010 §7)。
// SP が 0..32 の範囲外、または side が不正なときは ok=false。
func ArchetypeOf(side ReverseSide, category MoveCategory, sp Stats, nature Nature) (Archetype, bool) {
	if side != SideDefender && side != SideAttacker {
		return Archetype{}, false
	}
	stat := reverseStat(side, category)
	xb, ok := spBucketOf(sp.Get(stat))
	if !ok {
		return Archetype{}, false
	}
	hb := SPBucketNone
	if side == SideDefender {
		if hb, ok = spBucketOf(sp.HP); !ok {
			return Archetype{}, false
		}
	}
	class := natureClassOf(stat, nature)
	// 固定表に載る型はラベルもそちらに合わせる。カタログから引くのが確実。
	for _, a := range buildCatalog(side, stat) {
		if a.HPBucket == hb && a.StatBucket == xb && a.NatureClass == class {
			return a, true
		}
	}
	return Archetype{}, false
}

// ---------------------------------------------------------------------------
// 逆算本体(ADR-0010 §4, §6)
// ---------------------------------------------------------------------------

// validateObservations は観測列を検証する(ADR-0010 §3)。
func validateObservations(obs []Observation) error {
	if len(obs) == 0 {
		return ErrNoObservation
	}
	for i, o := range obs {
		hasPercent, hasDamage := o.Percent != 0, o.Damage != 0
		switch {
		case hasPercent && hasDamage:
			return fmt.Errorf("%w: 観測 %d は Percent と Damage を同時に指定している", ErrInvalidObservation, i+1)
		case !hasPercent && !hasDamage:
			return fmt.Errorf("%w: 観測 %d は Percent も Damage も指定していない", ErrInvalidObservation, i+1)
		case o.Percent < 0 || o.Percent > 100:
			return fmt.Errorf("%w: 観測 %d の Percent は 1..100 の範囲外: %d", ErrInvalidObservation, i+1, o.Percent)
		case o.Damage < 0:
			return fmt.Errorf("%w: 観測 %d の Damage は正でなければならない: %d", ErrInvalidObservation, i+1, o.Damage)
		}
	}
	return nil
}

// observationDistance は格子点の16ロールと観測1件の距離(表示%ポイント)を返す(ADR-0010 §6.1)。
// 0 は完全一致のときだけ。disp は rolls の表示%。
func observationDistance(o Observation, rolls *[16]int, disp *[16]int, hp int) int {
	best := -1
	if o.Percent != 0 {
		for _, d := range disp {
			diff := d - o.Percent
			if diff < 0 {
				diff = -diff
			}
			if best < 0 || diff < best {
				best = diff
			}
		}
		return best
	}
	for _, r := range rolls {
		diff := r - o.Damage
		if diff < 0 {
			diff = -diff
		}
		if best < 0 || diff < best {
			best = diff
		}
	}
	if best == 0 {
		return 0
	}
	// 実点数の距離は%ポイントに換算する。ただし完全一致でないものは 0 にしない
	// (near = 1.0 は完全一致のときだけ、という不変条件を守る)。
	return max(1, best*100/hp)
}

// reverseState は (型 × 持ち物) ごとの集計。
type reverseState struct {
	seen                 bool
	score                float64
	sp                   Stats
	nature               Nature
	minTenths, maxTenths int // 代表点の表示%(0.1% 単位)
	points               int
	exactPts             int
}

// CalcReverse は観測ダメージから相手の調整候補を一致度の高い順に返す。
// CalcDamage の合成のみで行い、独自のダメージ式は書かない(ADR-0010 §1)。
// 格子点 × 持ち物ごとに CalcDamage を1回だけ呼び、16ロールを全観測で使い回す。
// エラー時は部分的な結果を返さない。
func CalcReverse(in ReverseInput) (ReverseResult, error) {
	if in.Side != SideDefender && in.Side != SideAttacker {
		return ReverseResult{}, fmt.Errorf("%w: %q", ErrInvalidReverseSide, in.Side)
	}
	if err := validateObservations(in.Observations); err != nil {
		return ReverseResult{}, err
	}

	stat := reverseStat(in.Side, in.Move.Category)
	catalog := buildCatalog(in.Side, stat)
	// (H バケット, 関連ステータスのバケット, 性格クラス) -> 事前順位
	var prio [3][3][3]int
	for i, a := range catalog {
		prio[spBucketIndex(a.HPBucket)][spBucketIndex(a.StatBucket)][natureClassIndex(a.NatureClass)] = i
	}

	// 持ち物なしの素の1通り。渡されたスライスは書き換えない。
	items := in.ItemCandidates
	if len(items) == 0 {
		items = []*Item{nil}
	}
	states := make([]reverseState, len(catalog)*len(items))

	hMax := MaxSPPerStat
	if in.Side == SideAttacker {
		hMax = 0 // H の次元を持たない
	}

	for h := 0; h <= hMax; h++ {
		hb, _ := spBucketOf(h)
		if in.Side == SideAttacker {
			hb = SPBucketNone
		}
		for x := 0; x <= MaxSPPerStat; x++ {
			xb, _ := spBucketOf(x)
			sp := Stats{}.WithStat(stat, x)
			if in.Side == SideDefender {
				sp.HP = h
			}
			for _, class := range reverseNatureClasses {
				nature := natureForClass(stat, class)
				base := prio[spBucketIndex(hb)][spBucketIndex(xb)][natureClassIndex(class)] * len(items)
				for ii, item := range items {
					unknown := Individual{
						Species: in.UnknownSpecies,
						Level:   DefaultLevel,
						Nature:  nature,
						SP:      sp,
						Item:    item,
						Status:  StatusNone,
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
						return ReverseResult{}, fmt.Errorf("逆算の格子点(SP=%+v, 性格クラス=%s)の計算: %w", sp, class, err)
					}

					var disp [16]int
					for i, r := range res.Rolls {
						disp[i] = ObservedPercent(r, res.DefenderHP)
					}
					sum, exact := 0.0, true
					for _, o := range in.Observations {
						d := observationDistance(o, &res.Rolls, &disp, res.DefenderHP)
						if d != 0 {
							exact = false
						}
						sum += 1 / (1 + float64(d))
					}
					score := 1.0
					if !exact {
						score = sum / float64(len(in.Observations))
					}

					st := &states[base+ii]
					st.points++
					if exact {
						st.exactPts++
					}
					// 走査順で最初に最大を達成した点を代表にする(strict >)。
					if !st.seen || score > st.score {
						st.seen = true
						st.score = score
						st.sp = sp
						st.nature = nature
						st.minTenths, st.maxTenths = res.DisplayPercentRangeTenths()
					}
				}
			}
		}
	}

	type ranked struct {
		cand   ReverseCandidate
		prio   int
		itemIx int
	}
	all := make([]ranked, 0, len(states))
	exactCount := 0
	for pi, a := range catalog {
		for ii, item := range items {
			st := states[pi*len(items)+ii]
			c := ReverseCandidate{
				Archetype:        a,
				Item:             item,
				SP:               st.sp,
				Nature:           st.nature,
				MatchScore:       st.score,
				Exact:            st.score == 1.0,
				MinPercentTenths: st.minTenths,
				MaxPercentTenths: st.maxTenths,
				Points:           st.points,
				ExactPoints:      st.exactPts,
			}
			if item != nil {
				c.ItemID = item.ID
			}
			if c.Exact {
				exactCount++
			}
			all = append(all, ranked{cand: c, prio: pi, itemIx: ii})
		}
	}
	// 全順序: MatchScore 降順 → 型の事前順位 → 持ち物候補の添字(ADR-0010 §6.3)。
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.cand.MatchScore != b.cand.MatchScore {
			return a.cand.MatchScore > b.cand.MatchScore
		}
		if a.prio != b.prio {
			return a.prio < b.prio
		}
		return a.itemIx < b.itemIx
	})

	n := len(all)
	if in.MaxCandidates > 0 && in.MaxCandidates < n {
		n = in.MaxCandidates
	}
	cands := make([]ReverseCandidate, n)
	for i := range cands {
		cands[i] = all[i].cand
	}
	return ReverseResult{Side: in.Side, Stat: stat, Candidates: cands, ExactCount: exactCount}, nil
}
