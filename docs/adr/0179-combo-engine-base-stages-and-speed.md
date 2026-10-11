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
