# ADR-0177: 複数の目標(素早さを上回る・耐える・倒す)をすべて満たす最小の振り方の探索(F-11 段階 B)

- 状態: 採用(2026-10-04。engine・calc-svc の実装と Web のフラグの有効化まで)
- 日付: 2026-10-04
- レーン: ダメージ計算(データ帯 `0100〜`。`0176` は PR #632(特性 段階1)が使うため `0177`)
- 関連: ADR-0331(F-11 段階 A。契約 `adjustGoals`・§3 相手の素早さはサーバー・§4 engine の拡張の提案)、
  ADR-0150 §6〜§8(最小 SP・同点規則・配分)、ADR-0250(調整 API の検査順・値域・エラー語彙・§7 イベントを発行しない)、
  ADR-0107(技の追加効果のランク変化)、ADR-0123(未対応の印)、ADR-0139(素早さの補正の語彙)、ADR-0319 §1(調整は API 専用)、
  ADR-0701 §2(judge の素早さ = `RealStats` → `EffectiveStat` → judge だけが持つ補正)

## 背景

ADR-0331 で、目標を「相手と種類(素早さを上回る・この技を耐える・この技で倒す)」で選ぶ契約 `POST /api/calc/adjust/goals` を
決めた(段階 A。Web は完了、calc-svc はスタブ)。段階 B では engine に複数目標の探索を足し、calc-svc の `AdjustGoals` を実装する。
本 ADR は engine の探索の定義(同点・満たせないときの組)、相手の素早さの式の置き場所、ニトロチャージの扱い、計算量、
calc-svc の境界を決める。

## 決定

### 1. engine の公開 API(`engine/adjust_goals.go`)

```go
const MaxSPGoals = 6                      // 契約の goals.maxItems と同じ

type SPGoalKind string                    // "outspeed" / "survive" / "ko"(契約 AdjustGoalKind と同じ値)

type SPGoal struct {
    Kind             SPGoalKind
    Opponent         Individual
    Move             Move     // survive: 相手の技 / ko: 自分の技 / outspeed: 読まない
    Hits             int      // survive・ko: 1..MaxAdjustHits。outspeed は読まない
    ThresholdPercent float64  // survive・ko: 0 は既定(100)。outspeed は読まない
    SelfSpeedStage   int      // outspeed: 先に使う技の自分の素早さのランク変化(-6..6。呼び出し側が解決)。他の種類は 0
}

type SPGoalsInput struct {
    Format    Format
    Self      Individual  // Self.SP は各能力の下限
    Ceiling   Stats       // 探索する能力の上限(下限 ≤ 上限 ≤ 32。0 は「下限より上に振らない」)
    Field     Field       // 契約には無い(calc-svc は場なしを渡す)。engine は CalcDamage に素通し
    TypeChart TypeChart
    Goals     []SPGoal    // 1..MaxSPGoals
}

type SPGoalsPlan    struct { SP Stats; TotalSP int; Real Stats }
type SPGoalOutcome  struct { Kind SPGoalKind; Met bool; ChancePercent float64; SelfSpeed, OpponentSpeed, SelfSpeedRank int }
type SPGoalsResult  struct { Feasible bool; Remaining int; Plan SPGoalsPlan; Goals []SPGoalOutcome; Unsupported []UnsupportedMark }

func SuggestSPForGoals(in SPGoalsInput) (SPGoalsResult, error)
func GuaranteedSelfSpeedStage(m Move) int
```

- `SPGoalOutcome` は目標の順。`ChancePercent` は survive が耐える確率・ko が倒す確率(ADR-0150 §7 と同じ生値)、outspeed は 0。
  `SelfSpeed`・`OpponentSpeed`・`SelfSpeedRank` は outspeed だけが値を持ち、他は 0(calc-svc が種類で null に写す)。
- `Remaining = MaxSPTotal − Plan.TotalSP`(契約 `remaining`)。`Plan.Real` は `RealStats`(ランク補正なし)。
- WASM には出さない(調整は API 専用。ADR-0319 §1・ADR-0331 §2)。`engine/wasmapi`・`engine/cmd/wasm` は変えない。

### 2. 判定(目標1件)

| 種類 | 満たす条件 | 使う能力 |
|---|---|---|
| outspeed | `SelfSpeed > OpponentSpeed`(同速は満たさない) | S |
| survive | `Opponent` が `Move` で自分を `Hits` 発攻撃して耐える確率 ≥ しきい値(ADR-0150 §7 と同じ。100 は最大ロールの整数比較) | H と B(物理)/ D(特殊) |
| ko | 自分が `Move` で `Opponent` を `Hits` 発で倒す確率 ≥ しきい値(同上。100 は最小ロールの整数比較) | A(物理)/ C(特殊) |

- **相手の素早さ** `OpponentSpeed = EffectiveStat(Opponent, StatSpe)`(実数値に `Opponent.Ranks.Spe`)。
- **自分の素早さ** `SelfSpeedRank = clamp(Self.Ranks.Spe + SelfSpeedStage, −6, 6)`、`SelfSpeed = 実数値 S にそのランクを掛けた値`
  (`applyStatStage`。`EffectiveStat` と同じ式)。契約の `selfSpeedRank` も同じ値(Web の `self.ranks.spe` は 0 なので技の変化そのもの)。
- 持ち物・特性・場・まひによる素早さの補正は含めない(ADR-0331 §3)。

### 3. 素早さの式の置き場所(新しい式を書かない)

- 素早さの実数値とランクは **engine の既存の `RealStats`・`EffectiveStat`(`applyStatStage`)だけ**を使う。新しい式は足さない。
- judge の `Speed`(services/judge/internal/judge/speed.go)は既に `engine.EffectiveStat` を呼び、追い風・スカーフ・特性・まひの
  補正だけを自分で持つ(ADR-0701 §2・ADR-0714)。よって「実数値 → ランク」の正は今も engine の1か所で、重複は無い。
  judge の補正(SpeedMods の評価)を engine に寄せることは本 ADR ではしない(調整は補正を含めない。必要になったら
  engine に素早さの補正を足す別 ADR で、judge と共通にする)。
- `speed_effect.go` の「engine は素早さを計算しない」は「素早さの**補正**(SpeedMods)を評価しない」の意味に読み替える
  (コメントを実装時に直す)。
- ゴールデン: ダメージ式は変えないので `make test-golden` は不変。素早さの実数値は `RealStats` が既に
  `stats_test.go`・`stats_allspecies_test.go` で照合済み(@smogon/calc の `calcStat` 相当の式)で、新しい照合は要らない。

### 4. ニトロチャージ(先に使う技)— `GuaranteedSelfSpeedStage`

- `Move.Effect` が `Chance == 100` かつ `Target == RankTargetSelf` で `Stages[StatSpe]` を持てばその値、それ以外は 0。
  確率 100% 未満・相手が対象・素早さを含まない効果・効果なし・変化技でも 0(エラーにしない。ADR-0331 §3)。下降(例: −1)もそのまま返す。
- 純粋関数として engine に置く(ADR-0107 の `MoveEffect` の意味を知るのは engine)。calc-svc はマスタの `Move.Effect`
  (公開 `Move` には無い)を解決してこの関数で段数を求め、`SPGoal.SelfSpeedStage` に入れる。

### 5. 探索の定義(候補と順序)

**探索する能力** R は目標から決まる: outspeed があれば S、物理の ko があれば A、特殊の ko があれば C、
物理の survive があれば H と B、特殊の survive があれば H と D。R 以外の能力は下限のまま。

**候補** = R の各能力が [下限, 上限]、R 以外は下限、合計 ≤ 66 の全組(ADR-0150 §8 と同じ。下限の合計がちょうど 66 なら下限の組1つ)。

**順序**(次のキーを大きい方が前として辞書式に比べ、最も前の候補を `Plan` にする。満たせるときも満たせないときも同じ1本の順):

1. 満たす目標の数 大
2. 目標ごとの「近さ」をリクエストの順に 大。満たした目標は上限値(それ以上は区別しない)、満たさない outspeed は `SelfSpeed`、
   満たさない survive / ko は事象の組数(ロール数^Hits 通り中。`goalEventCount`。確率の浮動小数の誤差で同点を崩さない)
3. 合計 SP 小
4. 耐久指数 大(survive が物理だけなら H×B、特殊だけなら H×D、両方なら min(H×B, H×D) → max(H×B, H×D)。survive が無ければ比べない)
5. 能力の固定順(H, A, B, C, D, S)で SP の列が辞書順に小さい方(ADR-0150 §6)

- すべて満たせる組があれば 1・2 は全候補で同じ値になり、「合計 SP 最小 → 耐久指数 → 辞書順」= ADR-0150 §6 の最小 SP になる。
- 満たせないとき(`Feasible=false`)は「満たす目標の数が最大の組のうち、前の目標ほど近づける組」。目標が1件なら
  ADR-0150 §7・§8 の代わりの組と同じ(survive は 耐える確率 大 → 合計 SP 小 → 指数 → 辞書順、outspeed は届く最大の素早さを最小の SP で)。
  **目標の順に意味がある**(予算が足りないとき前の目標を優先する)。Web は追加した順に送る(ADR-0331 §6)ので、画面の上の目標が優先される。
- 単一の目標での一致(性質テストで固定する): survive だけ(下限 0・上限 32)は `MinSPToSurvive` と同じ組・確率。
  ko だけで満たせるときは `MinSPToKO` と同じ SP。outspeed + ko(満たせる)は `SuggestSPAllocation`(offense・`MinSpeed = 相手の S + 1`)の
  `MinSP` と同じ組。

### 6. 計算量と実装(分解)

engine の `CalcDamage` は攻撃側の A/C(技の分類)と防御側の H・B/D(同)の実数値しか読まない(`attackDefenseStats`・`DefenderHP`)。
ボディプレス・イカサマ・サイコショックのような「別の能力で計算する技」も engine は通常の式で計算して未対応の印
(`alt_offense_stat`・`alt_defense_stat`)を付ける(ADR-0123)。よって**能力の組は目標の種類ごとに重ならず**、例外の技も拒否しない
(数値は通常の式・印は `Unsupported` で伝える。ほかの調整操作と同じ)。この前提は性質テスト(他の能力の SP を変えても
`CalcDamage` のロールが変わらない)で固定し、engine がその技を別の能力で計算するようになったら本 ADR を改める。

全候補の素朴な列挙は最悪 33⁶ ≈ 13 億で不可。順序の分解で次のように求める(結果は §5 の総当たりと一致すること):

1. **HBD の表**: H・B・D(R に含まれる分)を総当たり(最大 33³ = 35,937 組)。survive の `CalcDamage` は目標ごとに
   (H 実数値, B/D 実数値) で使い回す(最大 6 × 1,089 回)。HBD のコスト r(0..66)ごとに、§5 のキーを HBD の部分
   (HBD の目標の数・HBD の目標の近さ(順)・コスト小・耐久指数・H, B, D の辞書順)に絞った最良の組を求め、r の昇順に累積最良にする。
2. **SAC の総当たり**: S・A・C(R に含まれる分。最大 33³)を回し、予算 66 − (S+A+C) の累積最良の HBD と組み合わせて §5 の全キーで比べる。
   ko の `CalcDamage` は目標ごとに攻撃実数値で使い回す(最大 33 回)。
   - SAC を固定すると §5 のキーの SAC の成分は等しいので、比較は HBD の部分のキーに帰着する(辞書順も A・C・S が同じなら H, B, D の順)。
     コストが小さいほど候補の集合は包含で増えるので、累積最良で正しい。

最悪(6 件で全 6 能力を使う: outspeed・物理 ko・特殊 ko・物理 survive・特殊 survive・もう1件)で、`CalcDamage` 最大 約 6,600 回、
候補の比較 約 7.2 万回。allocation(bulk)の数倍以内。engine と calc-svc に上限ちょうどのベンチマークを置き、実測を本 ADR に追記する
(壁時計の閾値は判定しない。ADR-0250 §5)。

実測(2026-10-04。Apple M5 Pro・Go のネイティブ・`-benchtime 10x`・3回):

| ベンチマーク | 時間/回 | メモリ/回 | 割り当て/回 |
|---|---|---|---|
| `BenchmarkSuggestSPForGoalsAtLimit`(engine) | 約 14 ms(13.7〜15.3) | 約 21.6 MB | 約 22,800 |
| `BenchmarkAdjustGoalsAtLimit`(calc-svc の HTTP。デコード・ID 解決・写しを含む) | 約 15 ms(14.4〜15.8) | 約 16.4 MB | 約 20,200 |

最悪でも 1 要求あたり数十 ms 以内で、ほかの調整(allocation)と同じ guard(締め切り・同時実行の上限)の範囲に収まる。

### 7. 未対応の印

survive・ko の目標ごとに、下限の組での `CalcDamage` の印(SP によらない)を目標の順に連結し、完全に同じ印(target・reason・id)は
先に出たものだけ残す。印が無ければ nil(calc-svc は `[]`)。outspeed は印を持たない。survive では相手が攻撃側なので
`attacker_*` の印は相手の持ち物・特性を指す(既存の min-sp-to-survive と同じ意味)。

### 8. 入力検査(engine。すべて `ErrInvalidAdjustInput` で包む)

目標が 0 件・`MaxSPGoals` 超、未知の `Kind`、`Self.Validate` の失敗(下限の範囲・合計 66 超・ランク)、各 `Opponent.Validate` の失敗、
survive・ko の発数・しきい値・技(変化技・威力 0 以下。`validateAdjustSearch` と同じ)、outspeed の `SelfSpeedStage` が −6..6 の外、
survive・ko で `SelfSpeedStage` が 0 以外、R の能力の上限が下限未満・32 超(R 以外の上限は見ない。ADR-0150 §8 と同じ)。
相性表の不足は `CalcDamage` のエラーを包んで返す(outspeed だけなら相性表は要らない)。入力の `Goals` を書き換えない。

### 9. calc-svc(`services/calc/internal/httpapi/adjust.go` の `AdjustGoals`)

検査順は ADR-0250 §5(値域 → 列挙 → 必須 → ID 解決 → engine):

1. ヘッダ(`missing_header` / `invalid_header`)→ 厳格デコード(`invalid_json` / `unknown_field`)
2. **値域**(マスタ参照 0 回): `goals` の件数 1..6、各目標の `hits`(値があれば 1..10。種類によらず)・`thresholdPercent`(値があれば (0, 100])、
   `ceiling`(各 0..32。省略は 32)→ `invalid_input`
3. **列挙**: `format`、各目標の `kind` → `invalid_enum`
4. **種類ごとの必須**: survive・ko の `moveId`・`hits` の欠落 → `invalid_input`(outspeed の `moveId` は任意)
5. **ID 解決**: `self` → 目標の順に `opponent`・`moveId`(`unknown_species` / `unknown_nature` / `unknown_item` / `unknown_ability` / `unknown_move`。
   個体の `status`・`teraType` の列挙は既存どおり解決と同時)
6. outspeed の `moveId` があれば `GuaranteedSelfSpeedStage(move)` で段数を求める。engine を呼び、`ErrInvalidAdjustInput` は `invalid_input`
- 応答: outspeed は `chancePercent: null`、survive・ko は `selfSpeed`・`opponentSpeed`・`selfSpeedRank` を null(キーは省略せず null で出す)。
  `unsupported` は空なら `[]`。
- 新しいエラー code は足さない。計算イベントは発行しない(ADR-0250 §7)。マスタ準備中は 503 `master_unavailable`
  (`registerDeferredCalcRoutes` にも登録する)。ルートは `/api/calc/adjust/goals` に登録し、ほかの調整と同じ guard を掛ける。
- gateway は変えない(`/api/calc/*` の前方一致で calc-svc に届く。ADR-0202 §3)。テストでパスを固定する。

### 10. Web の有効化(段階 B のマージで同時に行う)

- `web/src/adjust/adjustGoals.ts` の `ADJUST_GOALS_ENABLED` を `true` にする。
- 仕様の変更として既存テストの期待値を改める(ADR-0331 §結果): `AdjustScreen.test.tsx` の「モードは…radio group」(先頭に `goals`、6 つ)、
  `adjustGoals.test.ts` の「段階 B が入るまでモードは出さない」(true)。本 ADR の spec でこの2件を先に書き換える。
- E2E(k3d のスモーク)に「目標(素早さ + 倒す)で 200」を1件足す案は ADR-0331 のとおり残す(本 ADR の範囲外)。

### 11. 契約の文言の更新(型は変えない)

`api/openapi.yaml` の `adjustGoals` の説明から「未実装(404)」を外し、満たせないときの組の選び方を本 ADR §5 に結ぶ。
`AdjustGoalOutcome.selfSpeedRank`・`selfSpeed` の説明に `self.ranks.spe` を含めることを書く(§2)。

## 却下した案

- **33⁶ の素朴な総当たり**: 最悪 13 億候補で不可。§6 の分解は同じ順序を保ったまま 33³ + 33³ に落とす。
- **満たせないときの組を「満たす数 → 合計 SP」だけで選ぶ**: 届かない目標に1ポイントも振らない組になり、ADR-0150 の
  「最も近い組」(素早さは届く最大、耐えるは確率最大)と食い違う。
- **満たせないときに目標の近さを合計・重み付けする**: 素早さの実数値と確率は単位が違い、重みが恣意的になる。順序(前の目標を優先)は
  利用者が並べ替えで制御でき、決定的。
- **ボディプレス等を含む目標を拒否する**: engine はその技も通常の式で計算し印を付ける方針(ADR-0123)。調整だけ拒否すると
  計算画面と食い違う。
- **素早さの式を engine に新しく書く・judge を engine に寄せる**: 実数値 → ランクは既に engine の1か所。補正は調整の範囲外。
- **WASM に出す**: 調整は API 専用(ADR-0319 §1)。iOS も HTTP。

## 受け入れ条件と担当テスト

| # | 受け入れ条件 | テスト |
|---|---|---|
| B1 | `SuggestSPForGoals` の結果が、§5 の候補を全部列挙して順に並べた独立の総当たりと一致する(下限・上限・性格・種類の組み合わせ・満たせない場合・順の入れ替え) | `engine/adjust_goals_test.go` `TestSPGoalsMatchOracle` |
| B2 | 単一の目標は既存の探索と一致する(survive = `MinSPToSurvive`、ko(満たせる)= `MinSPToKO`、outspeed + ko = `SuggestSPAllocation` の `MinSP`) | 同 `TestSPGoalsSingleGoalMatchesExistingSearch` |
| B3 | 境界: 同速は満たさない・SP 0 / 32 で届く・届かない・先に使う技の段数(+1 で S が減る・ランク 6 で頭打ち)・下限が目標を超える・下限の合計 66・上限 0・2つの survive で H を共有・素早さ + 倒す・予算が足りないとき前の目標を優先 | 同 `TestSPGoalsBoundaries` ほか |
| B4 | 不正入力は `ErrInvalidAdjustInput`、相性表の不足は別のエラー、outspeed だけなら相性表なしで動く、入力を書き換えない | 同 `TestSPGoalsRejectsInvalidInput` ほか |
| B5 | 未対応の印を目標の順に連結し重複を除く | 同 `TestSPGoalsCarriesUnsupportedMarks` |
| B6 | `GuaranteedSelfSpeedStage` は確率 100%・自分・素早さのときだけ段数、ほかは 0 | 同 `TestGuaranteedSelfSpeedStage` |
| B7 | 分解の前提: `CalcDamage` のロールは攻撃側の A/C・防御側の H・B/D 以外の SP で変わらない | 同 `TestSPGoalsDecompositionPremise` |
| C1 | HTTP の応答が同じ入力を engine に渡した結果の写し(ceiling の既定 32・先に使う技の段数・null の出し分け・`unsupported: []`) | `services/calc/internal/httpapi/adjust_goals_test.go` `TestAdjustGoalsHTTPMatchesEngine` ほか |
| C2 | エラー語彙(新しい code なし)と検査順(値域 → 列挙 → 必須 → ID 解決。値域・列挙・必須の段でマスタ参照 0 回) | 同 `TestAdjustGoalsHTTPErrors`・`TestAdjustGoalsCheckOrder` |
| C3 | ヘッダ必須・イベント非発行・マスタ準備中 503 → 準備後 200・契約に照らして妥当 | 同 |
| G1 | gateway 経由で calc の上流にだけ届き、実物の calc-svc で 200・代表的な 400 が契約どおり | `services/gateway/internal/httpapi/adjust_goals_routing_test.go` |
| W1 | `ADJUST_GOALS_ENABLED` が true、モードの radio は 6 つで先頭が goals | `web/src/adjust/adjustGoals.test.ts`・`AdjustScreen.test.tsx`(期待値の更新) |

## 結果

- engine に `SuggestSPForGoals`・`GuaranteedSelfSpeedStage` が増える。ダメージ式・素早さの式は変えない(ゴールデン不変)。
- calc-svc の `adjustGoals` が 200 を返し、Web の目標モードが有効になる。iOS は同じ契約を後で呼ぶ(ADR-0331 §8)。
- 満たせないときの組は目標の順に依存する。画面で順を入れ替える操作が要るかは使用感で判断する(要望が出たら Web の ADR)。
