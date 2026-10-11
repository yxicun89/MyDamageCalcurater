# ADR-0179: 連続技の土台 — 確率 100% のランク変化と、素早さ・行動順の式を engine に置く(CB-2)

- 状態: 採用(2026-10-11。実装済み)
- 日付: 2026-10-11
- レーン: データ(engine)
- 関連: [docs/plan/combo-calc.md](../plan/combo-calc.md) §3-1・§3-2・§4-1・§6 CB-2・§7、ADR-0107(技の追加効果 MoveEffect)、
  ADR-0139(素早さの補正の語彙)、ADR-0143 §1(`finalSpeed`。oracle の getFinalSpeed)、ADR-0177 §3・§4(`GuaranteedSelfSpeedStage`)、
  ADR-0701・ADR-0702・ADR-0704・ADR-0712・ADR-0714(判定の素早さ・追い風・トリックルーム・行動順・まひ・特性/持ち物)

## 背景

連続技(CB。docs/plan/combo-calc.md)は、手ごとに自分・相手のランクを変え、その手の行動順を出す。オフライン(WASM)でも動かすため、
計算は engine に置く(§4-1)。いま engine にあるのは次の 2 つだけで、どちらも連続技には足りない。

- `GuaranteedSelfSpeedStage(m Move) int`(ADR-0177 §4): 確率 100%・自分・**素早さだけ**。りゅうのまいの攻撃 +1・インファイトの防御 -1・
  がんせきふうじ(相手の素早さ -1)を返せない。
- 非公開の `finalSpeed(ind, f)`(ADR-0143 §1。ジャイロボール等の威力の式が使う): 追い風を持たず、行動順の比較も無い。

素早さ・行動順の完全な式は判定(`services/judge/internal/judge/speed.go`・`turnorder.go`)にある。判定は CB-7 で廃止する予定(§7 Q2)なので、
式を engine に移す。ADR-0177 §3「engine は素早さの補正を評価しない」はこの ADR で改める(`finalSpeed` が既に評価しているので、実態に合わせる)。

## 決定

### 1. 確率 100% のランク変化(`engine/move_stages.go`)

```go
// GuaranteedSelfStages は技の追加効果のうち、確率 100% で必ず起きる使用者自身のランク変化を返す(無ければゼロ値)。
func GuaranteedSelfStages(m Move) Ranks
// GuaranteedTargetStages は確率 100% で必ず起きる、技の対象(相手)のランク変化を返す(無ければゼロ値)。
func GuaranteedTargetStages(m Move) Ranks
// AddClamped は r に d を足し、各能力を -6..+6 に収めた値を返す。
func (r Ranks) AddClamped(d Ranks) Ranks
```

- `m.Effect != nil && m.Effect.Chance == 100 && m.Effect.Target == RankTargetSelf`(Target は `RankTargetOpponent`)のときだけ、
  `Stages` の atk/def/spa/spd/spe を `Ranks` に写す。確率 100% 未満は**常に返さない**(ユーザー決定 §7 Q4 (b)。印も出さない)。
  変化技か攻撃技かは見ない。下降(負の値)もそのまま返す。
- `GuaranteedSelfSpeedStage(m)` は `GuaranteedSelfStages(m).Spe` に置き換える(結果は変わらない。既存の `TestGuaranteedSelfSpeedStage` は不変)。
- **CB-1 との整合**: CB-1 は Showdown の変化技のトップレベル `boosts` を、`target` が自分なら `RankTargetSelf`・確率 100、相手なら
  `RankTargetOpponent`・確率 100 として move_effects に入れる。攻撃技の `self.boosts`(ニトロチャージ・インファイト)と `secondary`
  (確率つき)は今のまま。どれも `Move.Effect` 1 件(ADR-0107 決定3「1 技 1 エントリ」)なので、この 2 関数は `Move.Effect` だけを読めばよく、
  CB-1 の前後で関数は変わらない(CB-1 の後はりゅうのまい等も値を返すようになるだけ)。
  1 技が自分と相手の両方を変える取り込みが必要になったら、`Move.Effect` を配列にする別 ADR で扱う(この 2 関数の形は変えない)。
- 命中率は見ない(§3-6)。

### 2. 素早さ(`engine/speed_order.go`)

```go
// SpeedSide はその個体の側にかかっている、素早さに効く場の効果。
type SpeedSide struct{ Tailwind bool }

// SpeedResult は FinalSpeed の結果。
type SpeedResult struct {
	Speed int
	// AbilityUndetermined / ItemUndetermined は、成立を判定できない条件(item_lost・語彙に無い条件)を、
	// 掛けた要素より前で(または掛けた要素が無いまま)飛ばしたこと。呼び出し側が未対応の印にする(CB-3)。
	AbilityUndetermined bool
	ItemUndetermined    bool
}

// ErrInvalidSpeedInput は素早さの入力が範囲外(素早さの種族値 1..255・SP・ランク・性格)。
var ErrInvalidSpeedInput = errors.New(...)

func FinalSpeed(ind Individual, f Field, side SpeedSide) (SpeedResult, error)
```

式(判定の `Speed` と ADR-0143 の `finalSpeed` を 1 本にしたもの。@smogon/calc 0.12.0 の getFinalSpeed):

1. `applyStatStage(RealStats(ind).Spe, ind.Ranks.Spe)`
2. 補正を **追い風 8192 → 特性の SpeedMods(成立した最初の 1 つ)→ 持ち物の SpeedMods(同)** の順に積み、`chainMods(…, speedModBounds)` で連結
3. `pokeRound` を 1 回(五捨五超入)
4. `ind.Status == StatusParalysis` かつ特性が `IgnoresParalysisSpeedDrop` でなければ `floor(x × 50 / 100)`
5. 0..10000 に収める

- 条件は `Field.Weather`・`Field.Terrain`・`ind.Status`(none 以外が `has_status`)で評価する(既存の `speedCondHolds`)。
- **こだわりスカーフ**: engine は持ち物 ID を見ない。スカーフは「`SpeedMods: [{always, 6144}]` を持つ持ち物」として呼び出し側が渡す
  (Champions のマスタにスカーフは無い。判定の `Scarf` フラグ + 持ち物の効果データより優先、は「持ち物は 1 つ」なので同じ値になる)。
- **成立を判定できない条件**(`item_lost` など): 現行の `finalSpeed`(oracle の既定「持ち物を失っていない」)と同じく**不成立として次の要素へ進む**。
  そのうえで `*Undetermined` を立てる。判定(`ResolveSpeedMod`)は「そこで止めて何も掛けない」だったが、要素が 1 つの `item_lost`
  (実データの唯一の例 かるわざ)では両者は同じ値になる。`[item_lost, 他]` の並びでだけ違い、engine は oracle 側(ゴールデンと同じ)を採る。
- 検証は素早さに関わる入力だけ(種族値は素早さだけ見る。タイプは見ない)。範囲外は `ErrInvalidSpeedInput` を `%w` で包んで返す。
- 非公開の `finalSpeed(ind, f)` は `FinalSpeed(ind, f, SpeedSide{})` の `Speed` に置き換える(値は変えない。ゴールデン不変)。

### 3. 行動順(`engine/speed_order.go`)

```go
type TurnOrder string
const (
	TurnOrderSelfFirst      TurnOrder = "self_first"
	TurnOrderOpponentFirst  TurnOrder = "opponent_first"
	TurnOrderSpeedTie       TurnOrder = "speed_tie"
)
// CompareSpeed は素早さだけで行動順を決める。トリックルームなら遅い方が先。同速は TurnOrderSpeedTie(反転しない)。
func CompareSpeed(selfSpeed, opponentSpeed int, trickRoom bool) TurnOrder
// CompareTurnOrder は優先度が違えば高い方が先(素早さ・トリックルームを見ない)。同じなら bySpeed をそのまま返す。
func CompareTurnOrder(selfPriority, opponentPriority int, bySpeed TurnOrder) TurnOrder
// EffectivePriority は技の優先度に、技の処理の定義の PriorityBoost(使用者が接地していてそのフィールド)を足した値。
func EffectivePriority(m Move, user Individual, f Field) int
```

- 判定の `SpeedComparison{Outspeeds, SpeedTie}`・`TurnOrder{AttackerMovesFirst, Tie}` の 2 つの真偽値を 1 つの閉じた語彙にした
  (「先に動く」と「同速」が同時に真にならないことを型で保つ。値は combo-calc.md §2-2 の出力の語彙と同じ)。
  `depends_on_opponent_speed`(相手の素早さが分からない)は CB-3 の `CalcCombo` が出す値で、この関数は返さない。
- `EffectivePriority` は `applyRulePreconditions` の優先度の加算と同じ式。`applyRulePreconditions` はこれを呼ぶ形にする(値は変えない)。

### 4. 判定(judge)との対応

| judge(services/judge/internal/judge) | engine | 備考 |
|---|---|---|
| `Individual{BaseSpeed, Nature, SP, Ranks}` | `Individual{Species.BaseStats.Spe, Nature, SP, Ranks}` | |
| `Individual.Tailwind` | `SpeedSide.Tailwind` | |
| `Individual.Scarf` | `Item{Effect: &ItemEffect{SpeedMods: [{always, 6144}]}}` | ID を engine に持ち込まない |
| `Individual.Paralysis` / `Env.HasStatus` | `Individual.Status`(paralysis は has_status も満たす) | |
| `Env.Weather` / `Env.Terrain`(文字列) | `Field.Weather` / `Field.Terrain` | |
| `Ability.Mods`・`IgnoresParalysisSpeedDrop` / `Item.Mods` | `Ability.Effect.SpeedMods`・同 / `Item.Effect.SpeedMods` | |
| `Speed` | `FinalSpeed(...).Speed` | 値は同じ(`[item_lost, 他]` の並びだけ §2 のとおり違う) |
| `ResolveSpeedMod` の undetermined | `SpeedResult.*Undetermined` | |
| `ErrInvalidBaseSpeed`・`ErrInvalidSP`・`ErrInvalidRank` | `ErrInvalidSpeedInput`(%w) | |
| `CompareSpeed(a, d, SpeedField{TrickRoom})` | `CompareSpeed(FinalSpeed(a), FinalSpeed(d), trickRoom)` | judge の `TestCompareSpeedTailwindPerSide`・`TestCompareSpeedTrickRoom`・`TestCompareSpeedZeroSpeedFieldMatchesJD1` は S9 で代用(比較は素早さの値だけを見る) |
| `CompareTurnOrder(pa, pd, SpeedComparison)` | `CompareTurnOrder(pa, pd, TurnOrder)` | |
| `AppliedSpeedFactors`・`IgnoredSpeedInputs*`・`IsChoiceScarf` | 移さない | 判定の契約(ADR-0710)専用。CB-7 で判定ごと消える |

judge のコードはこの ADR では変えない(CB-7 まで自前のまま)。

## 受け入れ条件と担当テスト(`engine/move_stages_test.go`・`engine/speed_order_test.go`)

| ID | 条件 | テスト |
|---|---|---|
| S1 | 確率 100%・自分の効果だけ全能力を `Ranks` で返す。確率 100% 未満・相手が対象・効果なしはゼロ値 | `TestGuaranteedSelfStages` |
| S2 | 確率 100%・相手が対象だけを返す | `TestGuaranteedTargetStages` |
| S3 | `GuaranteedSelfSpeedStage(m) == GuaranteedSelfStages(m).Spe`(ADR-0177 §4 の結果を変えない) | `TestGuaranteedSelfSpeedStageIsSelfStagesSpe` |
| S4 | `AddClamped` は -6..+6 に収める(+6 にさらに +2 で +6、-6 にさらに -1 で -6) | `TestRanksAddClamped` |
| S5 | judge の `TestSpeed`・`TestSpeedTailwind`・`TestSpeedTailwindAndChoiceScarfChainBeforeRounding`・`TestSpeedSingleModifiersUnchangedFromJD1`・`TestSpeedWithAbilityAndItemEffects`・`TestSpeedWithoutEffectsUnchanged`・`TestSpeedParalysis` の期待値を engine で同じ値にする | `TestFinalSpeedJudgeParity*` |
| S6 | 判定できない条件は飛ばして `*Undetermined` を立てる。前に成立した要素があれば立てない | `TestFinalSpeedUndetermined` |
| S7 | 範囲外の入力は `ErrInvalidSpeedInput`(judge の `TestSpeedRejectsOutOfRangeInput` の各ケース) | `TestFinalSpeedRejectsOutOfRange` |
| S8 | 追い風なしの `FinalSpeed` は既存の `finalSpeed` と全入力で一致(ゴールデン不変の前提) | `TestFinalSpeedMatchesInternalFinalSpeed` |
| S9 | 素早さの比較: 速い方が先・同速は speed_tie・トリックルームで反転(同速は反転しない) | `TestCompareSpeed` |
| S10 | 行動順: 優先度が違えば素早さ・トリックルームを見ない。同じなら素早さ(judge の turnorder_test の各ケース) | `TestCompareTurnOrder*` |
| S11 | `EffectivePriority`: PriorityBoost は接地・そのフィールドのときだけ足す | `TestEffectivePriority` |
| S12 | `make test-golden` 全件一致(既存ベクタを変えない) | 既存 |

## 実装の手順(implementer)

1. `engine/move_stages.go` に §1 の 3 関数。`adjust_goals.go` の `GuaranteedSelfSpeedStage` の本体を `return GuaranteedSelfStages(m).Spe` に。
2. `engine/speed_order.go` に §2・§3 の型と関数。`FinalSpeed` は `move_rule.go` の `finalSpeed`・`firstSpeedMod`・`speedCondHolds`・
   `speedModBounds`・`maxSpeed` を移して使い、`finalSpeed` は `FinalSpeed(ind, f, SpeedSide{}).Speed` を返す 1 行にする
   (`ruleBasePower` からの呼び出しは入力検証済みなのでエラーは起きない。エラーを握りつぶさず、起きたら現行と同じ式の値を返す形でよい)。
   「判定できない」の検出は `firstSpeedMod` に戻り値を 1 つ足す。
3. `applyRulePreconditions` の優先度の加算を `EffectivePriority` に置き換える。
4. `engine/speed_effect.go` の冒頭コメント「engine は素早さの補正を評価しない(ADR-0177 §3)」を「`FinalSpeed` が評価する(ADR-0179)」に直す。
5. `cd engine && go test ./...` → `make test-golden`(全件一致)→ `make lint`。
6. この ADR の状態を「採用」にし、docs/plan/improvements/data.md の I-data-10 を [x] にする。

## 却下した案

- **judge の `Speed` をそのまま engine に写す(`Scarf bool` を持つ)**: 持ち物 ID の判定を engine に持ち込む。効果データ(SpeedMods)で表せる。
- **判定できない条件で止める(judge の規則)**: 既存の `finalSpeed`(oracle 準拠。ジャイロボール等のゴールデン)と食い違い、
  素早さの式が engine に 2 本になる。実データでは結果が同じなので oracle 側に揃え、判定できなかったことは結果のフラグで返す。
- **行動順を 2 つの真偽値で返す(judge と同じ形)**: 「先に動く」と「同速」が同時に真になる組を型で防げない。

## 結果

- engine に `GuaranteedSelfStages`・`GuaranteedTargetStages`・`Ranks.AddClamped`・`FinalSpeed`・`CompareSpeed`・`CompareTurnOrder`・
  `EffectivePriority` が増える。ダメージ式は変えない(ゴールデン不変)。
- CB-3 の `CalcCombo` はこれらを組み合わせる。CB-7 で judge を消すとき、素早さの式は engine に残る。

## 追記 CB-3(2026-10-11): `CalcCombo` と合計の畳み込み

- 状態: 採用(2026-10-11 実装済み。ゴールデン combo.jsonl.gz 9 件)
- 関連: docs/plan/combo-calc.md §2〜§5・§7(Q1〜Q3・Q5〜Q9 は推奨 (a)、Q4 は (b))、ADR-0142 §3(多段の 1 発ごとの 16 段階)、
  ADR-0144(BattleState)、ADR-0123(未対応の印)、ADR-0010 §3.4(確率の表示)

### 1. API(`engine/combo.go`)

API(CB-4)・WASM(CB-5)の DTO にそのまま写せるよう、出力の語彙は combo-calc.md §2-2 と同じ文字列にする。

```go
const MaxComboSteps = 3 // §7 Q1
var ErrInvalidComboInput = errors.New("連続技の入力が不正")

type OpponentSpeedMode string
const (
	OpponentSpeedUnknown      OpponentSpeedMode = "unknown" // "" も unknown
	OpponentSpeedFromDefender OpponentSpeedMode = "from_defender"
)
const TurnOrderDependsOnOpponentSpeed TurnOrder = "depends_on_opponent_speed"

type ComboSpeedField struct{ TrickRoom, SelfTailwind, OpponentTailwind bool }
type ComboStep struct {
	Move     Move
	Critical bool
	Hits     int // BattleState.Hits と同じ(0 は既定。範囲の多段だけ)
}
type ComboInput struct {
	Format        Format
	Attacker      Individual // Ranks は 1 手目の開始時
	Defender      Individual // 相手の振り方(UC-5)
	Field         Field      // 全手で同じ(§3-5)
	TypeChart     TypeChart
	Steps         []ComboStep
	OpponentMove  *Move // nil は相手の攻撃を計算しない(行動順は優先度 0 として出す)
	OpponentSpeed OpponentSpeedMode
	SpeedField    ComboSpeedField
}

type SpeedRange struct{ Min, Max int } // 両端を含む。Min > Max は空
type SpeedLine struct {
	SelfSpeed     int        // 同速の値
	SelfFirst     SpeedRange // 相手の素早さがこの範囲なら自分が先(TR なしは 0..S-1、TR は S+1..maxSpeed)
	OpponentFirst SpeedRange // TR なしは S+1..maxSpeed、TR は 0..S-1
}
type ComboStepResult struct {
	MoveID                               string
	SelfStartRanks, SelfEndRanks         Ranks
	OpponentStartRanks, OpponentEndRanks Ranks
	Damage                               *DamageResult // 変化技は nil。CalcDamage の結果そのもの(印・KO を含む)
	SelfSpeed                            int           // 手の開始時の FinalSpeed
	OpponentSpeed                        int           // from_defender のときだけ(unknown は 0)
	SelfPriority, OpponentPriority       int           // EffectivePriority(相手の技なしは 0)
	Order                                TurnOrder     // self_first / opponent_first / speed_tie / depends_on_opponent_speed
	Line                                 SpeedLine     // 常に出す
}
type ComboKOChance struct {
	ChancePercent float64 // 0..100
	Guaranteed    bool    // Σ最小 ≥ HP(このとき ChancePercent = 100)
}
func (k ComboKOChance) DisplayChancePercentTenths() int // 0 → 0、Guaranteed → 1000、他は四捨五入して 1..999(KOChance と同じ規則)
type ComboTotal struct {
	MinDamage, MaxDamage, DefenderHP int
	KO       ComboKOChance   // 全手
	KOByStep []ComboKOChance // i 手目までで倒せる確率(累積。長さ = 手の数。最後 = KO)
}
func (t ComboTotal) DisplayPercentRangeTenths() (minTenths, maxTenths int) // 最小は切り捨て・最大は四捨五入(ADR-0010 §3.2)
type ComboSpeed struct {
	FirstSelfSpeed, LastSelfSpeed int // 1 手目・最後の手の開始時
	OpponentSpeed                 int // 1 手目の開始時。from_defender のときだけ
	OpponentSpeedKnown            bool
}
type OpponentTiming string // "before_self" | "after_self"
type OpponentAction struct {
	Step                 int // 0 始まり
	Timing               OpponentTiming
	Damage               DamageResult // その時点の両側のランクで
	SurviveChancePercent float64      // ここまでの全行動を耐える確率
}
type OpponentScenario struct {
	Actions              []OpponentAction
	SurviveChancePercent float64 // 行動なしは 100
}
type OpponentAttack struct {
	SelfHP                 int
	Fast, Slow             OpponentScenario // 順番が決まらない手を「相手が先」/「自分が先」とした場合
	MinActions, MaxActions int              // = len(Slow.Actions) / len(Fast.Actions)
}
type ComboResult struct {
	Steps          []ComboStepResult
	Total          ComboTotal
	Speed          ComboSpeed
	OpponentAttack *OpponentAttack // OpponentMove が nil なら nil
	Unsupported    []UnsupportedMark
}
func CalcCombo(in ComboInput) (ComboResult, error)
```

効果データに 2 項目を足す(CalcDamage は読まない。ItemRoles〈ADR-0175〉も変えない):

- `ItemEffect.LocksMove bool`: こだわり系(§3-3。ID を見ずに `choice_locked` を出すため)
- `ItemEffect.FullHPOnly bool`・`AbilityEffect.FullHPOnly bool`: 満タンのときだけ効く(きあいのタスキ・ばけのかわ・マルチスケイル型。§7 Q6 は印だけ)

未対応の印の理由を 3 つ足す: `choice_locked`・`defender_full_hp_assumed`・`speed_undetermined`(FinalSpeed の `*Undetermined`)。
`chance_effect_not_applied` は作らない(§7 Q4 (b)。確率 100% 未満は計算にも印にも出さない)。

### 2. 手の進め方(1 手ごと)

`cur`(自分のランク)= `Attacker.Ranks`、`oppCur` = `Defender.Ranks` から始め、手 i ごとに:

1. `self` / `opp` = 入力の個体のランクを `cur` / `oppCur` にしたもの(入力は書き換えない)。
2. `SelfSpeed = FinalSpeed(self, Field, {SelfTailwind})`。from_defender なら `OpponentSpeed = FinalSpeed(opp, Field, {OpponentTailwind})`。
3. `SelfPriority = EffectivePriority(step.Move, self, Field)`、`OpponentPriority` は相手の技があれば同じ式、無ければ 0。
4. `bySpeed` = from_defender なら `CompareSpeed(SelfSpeed, OpponentSpeed, TrickRoom)`、unknown なら `depends_on_opponent_speed`。
   `Order = CompareTurnOrder(SelfPriority, OpponentPriority, bySpeed)`(優先度が違えば素早さを見ない)。`Line` は `SelfSpeed` と TrickRoom から。
5. 相手の技があれば、シナリオごとに「相手が先か」を決める: Fast は `opponent_first`・`speed_tie`・`depends_on_opponent_speed`、Slow は `opponent_first` だけ。
   相手が先なら、ここで相手の行動を 1 つ足す(`before_self`。ランクは `cur`・`oppCur`)。
6. 攻撃技なら `CalcDamage({Format, self, opp, Move, Field, Critical, TypeChart, State{Hits}})`。変化技は nil。
7. 技が当たったとき(変化技、または `Effectiveness != 0` かつ `Nullified == ""`)だけ
   `cur = cur.AddClamped(GuaranteedSelfStages(m))`・`oppCur = oppCur.AddClamped(GuaranteedTargetStages(m))`。
8. 5. で相手が後で、かつ最後の手でなければ、相手の行動を足す(`after_self`。ランクは 7. の後の値)。最後の手の後の相手の行動は数えない
   (連続技を撃ち終わるまでに受ける攻撃だけを見る)。

相手の攻撃は `DamageInput{Attacker: opp, Defender: self, Move: *OpponentMove, Field: 壁を入れ替えた Field(AttackerScreens ↔ DefenderScreens)}`。
急所なし・多段は既定の回数。相手のランクは相手の技の追加効果では変えない(§3-1 の 4.)。

### 3. 畳み込みと計算量

- **倒せる確率**: 長さ HP+1 の分布(HP 以上は 1 つのバケットに吸収)に、手の順に「発」(単発は `Rolls`、多段は `HitRolls` の各行)を
  1 つずつ畳み込む(`koProbabilitySeq` と同じ方式を「同じ技の繰り返し」から「違う発の列」に広げた関数。既存の `koProbabilitySeq` は変えない)。
  各手の最後の発の後の吸収バケットが `KOByStep[i]`。変化技の手は前の値のまま。
- **耐える確率**: 同じ関数を自分の HP と相手の行動の発の列で回し、`100 − 吸収バケット`。
- 計算量: 発の数 D(≤ 3 × 多段の最大回数)× 16 × (HP+1)。3 手・多段 10 発でも 約 30 × 16 × 700 ≈ 3.4×10^5 回。16^手数 の総当たりはしない
  (総当たりはテストだけ)。ダメージは整数のまま、確率だけ float64 で足す(1/16 の累乗なので発 13 までは厳密)。
- `Guaranteed` は整数で決める(Σ最小 ≥ HP)。そのとき `ChancePercent = 100`。Σ最大 < HP なら 0。

### 4. 入力の検査(AC-16。この順。ID の解決は API 側)

1. `len(Steps)` が 1..`MaxComboSteps` でなければ `ErrInvalidComboInput`(個体の検査より前)
2. `OpponentSpeed` が `""`・unknown・from_defender 以外 → `ErrInvalidComboInput`
3. 変化技の手で `GuaranteedSelfStages`・`GuaranteedTargetStages` がどちらもゼロ値 → `ErrInvalidComboInput`(確率 100% 未満だけの変化技も)
4. 相手の技が変化技 → `ErrInvalidComboInput`
5. `TypeChart` 未設定 → `ErrTypeChartMissing`。両側の `Individual.Validate`(変化技だけの手でも検査が抜けないように明示的に呼ぶ)
6. 以降は `FinalSpeed`(`ErrInvalidSpeedInput`)・`CalcDamage`(`ErrInvalidBattleState` 等)のエラーをそのまま(`%w`)返す

### 5. 印(`ComboResult.Unsupported`)の順

1. 各手の `Damage.Unsupported` を手の順に連結し、重複を除く(各手の `Damage.Unsupported` はそのまま残す)
2. `speed_undetermined`: 攻撃側の特性 → 攻撃側の持ち物 → (from_defender のときだけ)防御側の特性 → 防御側の持ち物。どれかの手で立てば 1 つ
3. `choice_locked`(対象 `attacker_item`): 攻撃側の持ち物が `LocksMove` で、手の `Move.ID` が 2 種類以上
4. `defender_full_hp_assumed`: 2 手目以降に攻撃技があるとき、防御側の持ち物の `FullHPOnly`(`defender_item`)→ 特性の `FullHPOnly`(`defender_ability`)→
   2 手目以降の攻撃技で残り HP で威力・ダメージが変わる定義(`Rule.PowerFormula == defender_hp_ratio`・`FixedDamageFormula` が
   `defender_current_hp_half` / `defender_minus_attacker_hp`)の技(`move`、手の順・重複なし)

印が 1 つも無ければ nil。

### 6. 受け入れ条件とテスト(`engine/combo_test.go`。AC は combo-calc.md §5)

| AC | テスト |
|---|---|
| AC-1 | `TestCalcComboUC3PlayRoughShadowSneak`(16 段階・169〜201・51.5625%・表示 516・累積・合計の表示%) |
| AC-2 | `TestCalcComboUC3BulkyDefenderCannotKO` |
| AC-3 | `TestCalcComboDragonDanceOutrage` |
| AC-4 | `TestCalcComboSpeedFromDefender` |
| AC-5 | `TestCalcComboSpeedLineUnknown`(TR で逆・追い風の両側) |
| AC-6・AC-7 | `TestCalcComboOpponentAttackSurvives`・`TestCalcComboOpponentAttackFaints`・`TestCalcComboOpponentActionCount`(最小・最大の回数) |
| AC-8 | `TestCalcComboTurnOrderPriority`・`TestTurnOrderDependsOnOpponentSpeedValue` |
| AC-9 | `TestCalcComboFlameChargeSpeedTiming` |
| AC-10 | `TestCalcComboRanksClamp` |
| AC-11 | `TestCalcComboOpponentDamageAfterSelfDefenseDrop`・`TestCalcComboOpponentAttackUsesSelfSideScreens`・`TestCalcComboTargetStagesAffectLaterDamage` |
| AC-12(§7 Q4 (b)) | `TestCalcComboChanceEffectNotApplied` |
| AC-13 | `TestCalcComboStepDamageMatchesCalcDamage` |
| AC-14 | `TestCalcComboKOMatchesBruteForce`(固定の種で 60 件。3 手・多段・相手の攻撃の耐える確率も総当たり)・`TestComboKOChanceDisplay` |
| AC-15 | `TestCalcComboChoiceLocked` |
| AC-16(engine 側) | `TestCalcComboRejectsInvalidInput` |
| §7 Q5 | `TestCalcComboRockTombChangesTurnOrder` |
| §3-4・§7 Q6 | `TestCalcComboDefenderFullHPAssumed`・`TestCalcComboUnsupportedMarksConcatenated`・`TestCalcComboSpeedUndeterminedMark` |
| §2 の 7. | `TestCalcComboNoStagesWhenMoveDoesNotHit`・`TestCalcComboDoesNotMutateInput` |
| AC-1〜AC-4・AC-9・AC-20 | `engine/golden_combo_test.go` の `TestGoldenCombo`(`-tags golden`。`goldenChampionsFiles` に `combo.jsonl.gz` を足した) |

### 7. ゴールデン `testdata/golden/combo.jsonl.gz`(tools/golden/generate.mjs の変更点)

新しいファイルだけを足す(既存のファイルのバイト列・乱数列は変えない。乱数は使わない)。1 行 1 ケース:

```json
{"id":"uc3/mimikyu-adamant/garchomp-jolly",
 "oracle":{"attacker":"Mimikyu","defender":"Garchomp","moves":["Play Rough","Shadow Sneak"]},
 "input":{"Format":"single","Attacker":{…individual() の input…},"Defender":{…},"Field":{"Weather":"none","Terrain":"none"},
          "Steps":[{"Move":{"ID":"playrough",…,"Effect":null},"Critical":false,"Hits":0},…],
          "OpponentSpeed":"from_defender","SpeedField":{"TrickRoom":false,"SelfTailwind":false,"OpponentTailwind":false}},
 "expected":{"steps":[{"boosts":{},"defenderBoosts":{},"rolls":[…16…],"selfSpeed":148,"defenderSpeed":169}, …],
             "defenderHp":185,"total":[169,201],"koCount":132,"koDenominator":256}}
```

- 手の開始時のランクは **JS 側で独立に** 累積する(`self` / `target` の確率 100 の変化だけを足し、-6..+6 に収める。当たらない技は足さない)。
  各手は `individual(genC, name, {..., ranks: その手の boosts})` の Pokemon で `calculate()`。変化技の手は `rolls: null`。
- 技の追加効果(`Move.Effect`)は oracle の技データに無い(りゅうのまい・ニトロチャージは boosts を持たない)ので、generate.mjs に
  シナリオで使う技だけの表(`comboEffects`)を置き、入力の `Move.Effect` と JS の累積の両方に使う。oracle の技データに `self.boosts`
  がある技(インファイト等)は表と一致することを assert する。
- 素早さは `require('@smogon/calc/dist/mechanics/util').getFinalSpeed(genC, pokemon, field, side)`(手の開始時の boosts)。
- `koCount` / `koDenominator` は oracle の各手の 16 段階(多段は 1 発ごと)から 16^発数 通りを **総当たり**で数える(発数は 6 以下に保つ)。
- ラベル(`goldenComboLabels`。各 1 件以上): `uc3`(AC-1。ID は `uc3/mimikyu-adamant/garchomp-jolly` を必ず含む)・`uc3-bulky`(AC-2)・
  `uc2`(AC-3。りゅうのまい → げきりん)・`flame-charge`(AC-9)・`close-combat`(自分の防御 -1 → 次の手)・`rock-tomb`(相手の素早さ -1)・
  `multi-hit`(タネマシンガン `Hits` 2〜3 を含む手)・`three-steps`(3 手)・`clamp`(攻撃 +6 で つるぎのまい)。
- `save('combo.jsonl.gz', comboCases, true)`、`championsFiles` に足す。metadata の `exclusions` の moves に「combo.jsonl.gz は各手を照合し、
  合計と確率は oracle の 16 段階の総当たり」を 1 文足す。

### 7 の 2. 実装の手順(implementer)

1. `engine/modifiers.go` に `LocksMove`・`FullHPOnly`(2 か所)、`engine/unsupported.go` に印の理由 3 つ、`engine/speed_order.go` に
   `TurnOrderDependsOnOpponentSpeed`。
2. `engine/combo.go` に §1 の型と `CalcCombo`(§2・§4・§5)。畳み込みは `engine/ko.go` に発の列を受ける関数を足す(§3。`koProbabilitySeq` は変えない)。
3. `cd engine && go test ./...`(combo_test.go が全部通る)。
4. tools/golden/generate.mjs を §7 のとおり変え、`make golden-generate` → `make test-golden`(既存のファイルの sha256 が変わらないこと・
   `TestGoldenCombo` を含め全件一致)。
5. `make lint`・`make test`。docs/plan/improvements/data.md の I-data-12 を [x]、この追記の状態を「採用」に。

### 互換・デプロイ順(CB-3。ADR-0144 §8・ADR-0145 にならう)

`ItemEffect.LocksMove`・`FullHPOnly`・`AbilityEffect.FullHPOnly` は効果定義の JSON に新しいキーを足す(`services/internal/master` が往復する)。
古い版の厳格なデコードは未知のキーを拒否する(calc-svc は 503、pokedex-svc の read model の書き出しは `ErrInvalidExport`)。したがって:

1. **先に** 新しい calc-svc・pokedex-svc(read model の書き出しを含む)と WASM をデプロイする。
2. **その後で** `LocksMove`・`FullHPOnly` を含む効果を取り込む。先に取り込むと、古い版が拒否する。

いまの `effects.json` にこの 2 項目は無い(Champions にこだわり系も無い)ので、既存のイメージは壊れない。balance・speed の read model は
防御側の相性の効果だけを出すので、新しいキーは出ない。WASM の DTO はこの 2 項目を持たない(CB-5 で連続技の DTO を足すときに決める)。

### 却下した案(CB-3)

- **16^手数 の総当たりを本体で回す**: 多段 3 手で 16^15 を超える。畳み込みは HP に比例するだけ。
- **相手の行動回数を 1 つの値で返す**: unknown のとき最後の手の行動順が決まらず、最小と最大が分かれる(§2-3 の 4.)。
  ランク(インファイトの防御 -1)で 1 回ごとのダメージも変わるので、回数ではなく行動の列をシナリオごとに返す。
- **確率 100% 未満の効果を「未適用」として出す**: §7 Q4 (b)(ユーザー決定)で出さない。
- **こだわり・タスキを持ち物 ID で判定する**: ID の直書きになる。効果データの 2 項目で表す(値はデータレーンが取り込む。Champions のマスタに
  こだわり系は無いので、v1 では実データでは立たない)。

## 追記 CB-5(2026-10-12): WASM の `calcCombo`(`engine/wasmapi`)

状態: 採用。範囲は WASM の境界だけ(engine 本体・API 契約・DB は変えない)。受け入れ条件は combo-calc.md §5 の AC-18。

### 1. 境界関数と DTO の形

`wasmapi.CalcCombo(requestJSON) string` を足し、`engine/cmd/wasm` が `globalThis.pokecalc.calcCombo` に登録する(ADR-0011 の封筒・
厳格デコード・panic を外へ出さない・改行で終わらない、をそのまま使う)。計算は `engine.CalcCombo` の素通しで、独自の式・丸めを持たない。

- リクエスト: calc の `format`・`attacker`・`defender`・`field`・`typeChart`(DTO を再利用)に、`steps[]`(`{ move, critical?, hits? }`。1〜3 件)、
  任意 `opponentMove`(攻撃技)、任意 `opponentSpeed`(`unknown` 既定。`""` も同じ。`from_defender`)、任意 `speedField`
  (`trickRoom`・`selfTailwind`・`opponentTailwind`)を足す。`hits` は battleState.hits と同じ規則(指定したら 1 以上、範囲は engine が見る)。
  技・持ち物・特性は ID ではなく解決済みの値で渡す(マスタ解決は Web 側。ADR-0011)。公開 API(CB-4)の構造と名前をそろえる。
- 結果: engine の `ComboResult` を camelCase で写す。`steps[].damage` は変化技だけ null、他は calc の結果と同じ形。パーセントは 0.1% 単位の
  表示%(`tenthPercent`)、倒せる確率は `chancePercent`・`guaranteed`・`displayChancePercent`。`opponentAttack` は `opponentMove` が無いと null。
  `unsupported` は印なしで空配列。

### 2. 効果 DTO の互換(CB-3 の互換節の決定)

- 技 DTO に任意 `effect`(`{ chance, target: self|target, stages: { atk|def|spa|spd|spe } }`)を足す。ランク変化は `Move.Effect` から
  取るので、これが無いと変化技の手を置けない。`engine.MoveEffect.Validate` の失敗は `invalid_input`、target / stages のキーの語彙外は `invalid_enum`。
- `ItemEffect.LocksMove`・`FullHPOnly`、`AbilityEffect.FullHPOnly` を受ける(`locksMove`・`fullHPOnly`)。受けないと印 choice_locked /
  defender_full_hp_assumed がオフラインで出ず、API と結果が食い違う(AC-18)。calc / bulk / reverse は読まない(結果は不変)。
  CB-3 の「WASM の DTO はこの 2 項目を持たない」を、CB-5 で持つ形に改める。
- デプロイ順は CB-3 と同じ(先に WASM / calc-svc / pokedex-svc、後でこの 2 項目を含む効果の取り込み)。

### 3. エラー

新しい code は足さない。`engine.ErrInvalidComboInput` は `invalid_input`。手の数(0・4 件)・hits の範囲・変化技の相手の技・ランク変化の無い
変化技 → `invalid_input`、`opponentSpeed` の語彙外 → `invalid_enum`、未知のキー → `unknown_field`、`typeChart` なし → `type_chart_missing`。

### 4. テストと一致

- `engine/wasmapi/combo_test.go`(テーブル駆動: engine との一致・省略時の既定・印・エラー code・3 手の境界・calc が effect を読まないこと・決定性)。
- `testdata/vectors.json` に `calcCombo/*` を 4 件(成功 3・エラー 1)。`engine/cmd/wasmexpect` が期待値を生成し、`make test-wasm` で Go と WASM がバイト一致。

### 却下した案(CB-5)

- **WASM 用に別の入力形(ID 参照)を作る**: マスタを持たない方針(ADR-0011)に反する。Web が解決した値を渡す。
- **効果の 2 項目を WASM では無視する**: オフラインだけ印が欠け、API と結果が違う。
