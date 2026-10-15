# ADR-0535: iOS の計算画面に「連続技」の節を足す(CB-8)

- 状態: 実装済み(2026-10-12。受け入れ条件とテストは ADR-0501「連続技(CB-8)」章)
- レーン: iOS(I-ios-20)
- 関連: docs/plan/combo-calc.md(§4-3 案 A・§5・§7)、ADR-0253(`POST /api/calc/combo`・公開 `Move.effect`)、
  ADR-0359(Web の連続技。段階 B・C・I-web-32)、docs/ai-shared/decisions/2026-10-12-web-combo-screen.md、
  ADR-0531(タブ・画面の持ち方)、ADR-0533(ランクのメーター)、ADR-0534(失敗の分類 `ServiceFailureKind`。PR #773)

## 決めたこと

1. **置き場所は案 A**。`CalcScreenView` の結果(`ResultsSectionView`)の下に「連続技」の節(`ComboSectionView`)。
   手がメインの 1 つだけのときは「次の技」ボタンだけにし、押したときだけ広げる。構成・語・並びは Web(ADR-0359・CB-6)と同じ。
2. **iOS は API だけ**(WASM は無い。ADR-0500 §4)。`POST /api/calc/combo` を生成クライアント(`make ios-gen` の `calcCombo`)で呼ぶ。
   サービスは `PokeCalcService` に足さず、別のプロトコル **`ComboCalcService`**(`calcCombo(_:)`)にする。
   既存のテスト用の偽物(`StubPokeCalcService`・`ConcurrencyProbeService`)を変えずに済み、連続技の失敗が単発の計算に混ざらない(絶対ルール 5)。
   実装は `APIPokeCalcService`(同じ gateway・同じ端末 ID/セッション ID)と `MockPokeCalcService`(架空データの決め打ち。計算はしない)。
3. **入力は二重に持たない(UC-5)**。`CalcViewModel` が `comboContext`(攻撃側の個体〈ランク・やけど・特性・持ち物込み〉・メインの技・急所・場・
   防御側の振りの選択肢・攻撃側の覚える技・防御側の種族・性格の一覧)を出し、`ComboViewModel`(PokeCalcCore)がそれを受け取る。
   防御側の振りは一括計算の行(`BulkCalcRow`。振り × 持ち物 × 特性)から作る(`ComboDefenderOptions`)。キーは結果の行の ID と同じ。
   メインの急所は計算画面の「急所」と同じ値(節で切り替えると `CalcViewModel.setCritical` を呼ぶ)。
4. **手**は「メイン + 追加の手(技 ID・急所)」の並び。最大 `RequestLimits.maxComboSteps`(= 契約 `ComboRequest.steps.maxItems` = 3。
   `check-request-limits.sh` で照合)。メインは外せない。追加の手は外せる・上へ/下へで並べ替えられる(メインより前にも置ける)。
   追加の手の候補は攻撃側の覚える技を `moves(ids:)`(getMovesByIds)で引き、攻撃技かランクを変える変化技(`Move.effect`)に絞ってタイプ順。
   相手の技の候補は防御側の `species(key:)` の覚える技のうち攻撃技。攻撃側を変えて覚えなくなった追加の手は、状態を消さずに表示と要求から外す。
5. **公開 `Move.effect`** をドメインの `Move.effect: MoveRankEffect?`(target = 自分/相手、stages = `RankBlock`)に写す。PascalCase
   (`Chance`・`Target`・`Stages`)を読み、確率が 100 でない・語彙外・形の違う効果は「ランクが変わらない」(nil)として扱う(応答全体を落とさない)。
6. **計算する条件**は Web と同じ(手が 2 件以上、または相手の技を選んだとき)。入力が変わるたびに計算し直し、最新の要求の応答だけを出す
   (世代の番号。キャンセルは表示しない)。相手の素早さは既定「分からない」(`unknown`。要求では省く)、切り替えたときだけ `from_defender`。
   素早さの場(トリックルーム・追い風〈自分〉・追い風〈相手〉)は折りたたみ。全部 false なら要求で省く。
7. **自分の素早さ**(I-web-32 相当)を節に持つ: 素早さの SP(未入力は個体の値。0〜min(32, 66 − 素早さ以外の SP))・性格補正
   (計算画面の性格のまま / 上がる / 補正なし / 下がる。Web と同じ置き換え規則)・素早さのランク(`RankMeterView`。field `comboSelfSpeed`)。
   iOS の個体は性格を ID で持つので、置き換えた (上昇, 下降) の組をマスタの性格から ID の昇順で最初に引く(ADR-0200 §2 と同じ規則)。引けなければ計算しない(案内を出す)。
8. **結果の表示**は API の値の書式化だけ(`ComboResultDisplay`。ADR-0300 §8 と同じ考え方)。倒す確率は `total.ko.displayChancePercent`
   (確定は `guaranteed`、表示 0 は「倒せない」)、耐える確率は `opponentAttack.fast`(相手が先とした悪い方)を主にし 0/100 を予約する丸め。
   素早さは `speed.firstSelfSpeed → lastSelfSpeed`、相手の素早さが分からないときは最後の手の `line`。
9. **素早さの基本値の例**(Web 段階 C1 相当): 相手の素早さが分からず、トリックルームでないとき、最後の手の自分の素早さで
   `SpeedService.reach`(raw)を引き、準速(`neutral-max`)の行だけを 1〜3 行出す。失敗・素早さのサービスが無いときは何も出さない(注記も出さない)。
10. **印**: 連続技だけの理由(`choice_locked`・`defender_full_hp_assumed`・`speed_undetermined`)は `UnsupportedReason` に足さず
    (契約の 15 種と照合するテストがあるため。Web の ADR-0359 §7 と同じ判断)、`ComboMark.combo` として Web と同じ語で出す。
11. **失敗**: 連続技の節の中だけに案内(`comboError`)と「もう一度」(`comboRetry`)を出し、単発の結果・エラー表示は変えない(絶対ルール 5)。
    文言は「連続技を計算できませんでした(<計算画面の失敗の文言>)」。原因の文言は `CalcScreenError(error).message` をそのまま使う
    (PR #773〈ADR-0534〉が入ると `ServiceFailureKind` の分類の文言になる。連続技の側で分類を持たない)。
12. **見た目**: パレット K のトークンだけ(上昇チップ = brandPrimary、下降 = warning、合計のバー = brandPrimary)。タップ領域 36pt 以上・
    `lineLimit` を付けない・常時アニメーション無し・画像は使わない(画像無しで成立)。AX5 では手の操作ボタンを手の名前の下の行へ折り返す。

## 今回は対象外(段階に分ける)

- **素早さ画面 → 計算タブの導線**(Web 段階 C2「この相手で連続技を計算」)。iOS にはタブをまたいで画面の入力を渡す仕組みが無い
  (`AppNavigationModel` はタブの選択だけを持ち、お気に入り・履歴からの計算は「積む画面」で開く。ADR-0531 §2)。素早さ画面
  (speed レーンが再設計済み)の「自分 / 相手」をダメージタブの計算画面へ入れるには、ルートの View に要求の受け渡し(token)を足し、
  計算画面の保持(`.retained`)と入力の上書きの規則を決める必要がある。連続技の節そのものは導線なしで使えるので、**I-ios-21** に分ける。
  そのとき「素早さ画面の SP・性格・ランクを自分の素早さへ入れる(スカーフは反映できない旨の注記)」も一緒に入れる。
  (追記 2026-10-15: ADR-0542 で入れる。受け入れ条件は ADR-0501「素早さ画面から連続技へ(I-ios-21)」。)
- こだわりスカーフの反映(Web と同じく API に項目が無い)。
- 計算履歴・お気に入りへの保存(record は連続技のイベントを持たない。ADR-0253)。

## 影響

- API・engine・サービスの変更なし(`api/openapi.yaml` は不変。生成物は `make ios-gen` が既に `calcCombo` を含む)。
- `Move` に `effect` を足す(既定 nil。既存の `Move(...)` の呼び出しは変えない)。
- モックのフィクスチャ: `moves.json` の `test-move-status-a` に確率 100% の効果(自分の攻撃 +1・素早さ +1)を足し、`combo-results.json` を新設。
  架空データのまま(名前は「テスト」で始まる。ADR-0002)。
