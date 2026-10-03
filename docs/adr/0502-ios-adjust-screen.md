# ADR-0502: iOS の調整画面(AJ7。指数・16n・SP 配分・最小 SP・技を覚えるポケモン)

- 状態: 採用(実装済み。2026-10-02)
- 日付: 2026-10-02
- レーン: ダメージ計算(iOS 帯 `0500〜`。`0501` までは使用済みのため `0502`)
- 関連: plan.md「AJ: 調整」AJ7、ADR-0319(Web の調整タブ。機能の正)、ADR-0150(指数・16n・探索・配分の定義)、
  ADR-0250(調整 API の契約)、ADR-0251(技の逆引き API)、ADR-0500(iOS の構成)、ADR-0501(画面ごとの規約。
  issue #68 の検索シート・issue #110 の RequestLimits・issue #113 の LatestTaskRunner・P6-14/15 の AX5・P6-17 の未対応の印・
  P6-7 の別プロトコルの境界)、ADR-0411 §3(サーバーの message を出さない)

## 背景

AJ6(ADR-0319)で Web に「調整」タブができた。AJ7 は同じ機能を iOS に出す。API は AJ4(`/api/calc/adjust/*`)と
AJ5(`listMoveLearners`)で、iOS の生成物(`PokeCalcAPI/Generated`)は再生成済み。iOS は WASM を持たないので API 専用。
iOS レーン(`feat/ios-p6-20-showdown`)とタイプバランスの iOS(`feat/tb-ios-balance`)が並行して `RootView` /
`AppEnvironment` を触っているため、共有ファイルへの変更は最小にする。

## 決定

### 1. 入口: ルート画面のボタン「調整」を1件足す(タブにはしない)

- iOS はタブバーを持たず、ルート画面(`RootView`)のピルボタンから `NavigationStack` で各画面へ進む。同じ形で
  「構築」の後ろに `NavigationLink(value: AdjustScreenRoute())`(文字 `AdjustText.screenTitle` =「調整」、
  `accessibilityIdentifier("openAdjustScreen")`)を1件足す。既存のボタン・遷移・識別子は変えない。
- `AppEnvironment.ready` に `adjust: any AdjustService` を**末尾に**1つ足す(`deviceData` と同じ形)。モックは
  `MockAdjustService()`、API は `APIPokeCalcService`(同じ値を `service` と `adjust` に渡す。`deviceData` と同じ)。
  `feat/tb-ios-balance` も同じ enum に `balance` を足しているので、後から main に入る側がパターン(`case .ready(...)`)の
  数を合わせる(衝突は機械的に解ける)。
- 起動時に直接開く環境変数 `POKECALC_OPEN_ADJUST_SCREEN_AT_LAUNCH=1` を既存3つと同じ形で足す(スクリーンショット用。
  `ios/scripts/sim-run.sh` の `IOS_SCREEN=adjust` は任意)。

### 2. 画面の構成とモード(Web の ADR-0319 §2 と同じ)

上から1列の `ScrollView`。各領域は `glassCard()` + 見出し(`.accessibilityAddTraits(.isHeader)`)。

| 領域(見出し) | identifier | 中身 |
|---|---|---|
| 自分 | `adjustOwnCard` | ポケモン(検索シート)・性格・特性(任意)・持ち物(任意)・技(任意。learnset のダメージ技)+「覚えるポケモン」・固定する能力ポイント 6 欄と「合計 n / 66」 |
| 調整の内容 | `adjustModeCard` | モード5つ(縦に並ぶ選択肢)とモードごとの欄 |
| 相手 | `adjustOpponentCard` | ポケモン・調整(プリセット)・技(相手が攻撃する側のときだけ)+「覚えるポケモン」。`needsOpponent` のときだけ |
| 目標 | `adjustGoalCard` | 発数(1〜10)・確率(確定 / 90 / 75 / 50%)。`needsOpponent` のときだけ |
| (ボタン)調整する | `adjustSubmitButton` | |
| 調整の結果 | `adjustResultCard` | 指数と 16n + モードの結果 + 未対応の印 |
| この技を覚えるポケモン | `adjustLearnersCard` | 「覚えるポケモン」を押したときだけ |

- モード(`AdjustMode`: indices / bulk / offense / minKo / minSurvive。既定 indices)、呼ぶ API、必須、欄は ADR-0319 §2 の表と同じ。
- 相手の SP・性格は数値入力させずプリセットから選ぶ。相手が攻撃する側(minSurvive・bulk の目標)は `AttackerPreset`
  (無振り / A(C)特化 / A(C)振り。`AttackerPreset.build`)、相手が受ける側(minKo・offense の目標)は
  **`KnownDefenderPreset`**(無振り / HB(HD)振り / HB(HD)特化。`KnownDefenderPreset.build`)。Web は ADR-0009 の8種の
  防御側プリセットを出すが、iOS は逆算画面と同じ3種にする(既存の型と性格の選び方〈ID を直書きしない〉をそのまま使い、
  AX5 で縦に並べても長くならない)。分類は minKo / offense の目標では自分の技、minSurvive / bulk の目標では相手の技の分類。
- 自分・相手の既定は「未選択」(Web と同じ)。モードを切り替えても入力は消さない。場・急所・相手の持ち物/特性・ダブルは出さない。
- 固定 SP と素早さの目標は数字キーボードの `TextField`(文字で持ち、送信時に検査)。上限は 0〜32 の `Picker`、発数・確率は `Picker`。

### 3. 境界: `AdjustService`(`PokeCalcService` には混ぜない)

- `AdjustService` プロトコル(`adjustIndices` / `adjustMinSpToKo` / `adjustMinSpToSurvive` / `adjustAllocation` /
  `moveLearners(moveId:limit:offset:)`)を新設し、`DeviceDataService` と同じく `extension APIPokeCalcService: AdjustService`
  (`APIPokeCalcService+Adjust.swift`)と `MockAdjustService` で実装する。`PokeCalcService` に足すと、全準拠型(テストの
  `StubPokeCalcService` 931 行を含む)を変えることになり、並行する iOS レーンとの衝突面が広がる。
- ドメインの型は `AdjustDomainTypes.swift`(生成型の写し。省略可の項目は Optional で、nil は「キーを送らない」)。
  契約のキー `self` は `selfIndividual` と呼ぶ。指数は `Int64`。`HPLineKind` は `none` / `line16n`(`"16n"`)/ `line16nMinus1`(`"16n-1"`)。
- 写像の規則: ドメインの nil → 生成型の nil(キーを出さない)。`minSpeed` は生成型では任意だが、Web と同じく常に送る。`format` は single。
  エラーは既存の `send` / `domainError` / `domainErrorFromSchema` と同じ(400/404/500/503/default → `PokeCalcError(code:)`、
  通信失敗 `client_transport_error`、デコード失敗 `client_decode_error`、取り消しは `CancellationError` のまま)。
- 技の逆引きは `GET /api/pokedex/moves/{key}/learners?limit=&offset=`(pokedex だが調整画面だけが使うので `AdjustService` に置く。
  Web の `adjustClient.moveLearners` と同じ判断)。

### 4. モック(`MockAdjustService`)

- 計算しない(ADR-0500 §4)。指数・最小 SP・配分は新しい架空データ `Resources/adjust-results.json` の決め打ちの値を返し、
  要求の形だけを反映する: 技なし → `firepowerIndex: nil`、探索する能力は技の分類から(物理 atk/def・特殊 spa/spd)、
  `goal` なし → `minSp: nil`、技の機構(`moves.json` の `mechanisms`)から未対応の印(`MockPokeCalcService.moveMarks` と同じ規則)。
- 逆引きは `species.json` の learnset を逆に引き、図鑑番号・フォルム番号の昇順に `limit` / `offset` を効かせる。マスタに無い技は `not_found`。
- `adjust-results.json` は `MockFixtures.resourceNames` に足し、既存の「架空データだけ」の検査(`MockPokeCalcServiceTests`)の対象にする。

### 5. ViewModel(`AdjustViewModel`。`@MainActor @Observable`)

- 依存は `PokeCalcService`(性格・種族の検索と詳細・持ち物・`moves(ids:)`)と `AdjustService`。種族は
  `MasterSpeciesSearchProviding` に準拠し、計算・逆算と同じ `MasterSearchSheet` で選ぶ(自分・相手で共用)。
- 技の選択肢は選んだ種族の `species(key:)` の learnset を `moves(ids:)`(64 件ずつの分割は既存の実装)で引き、変化技を除いた
  learnset の順。技の検索欄(issue #68 の交差)は使わない(一覧が learnset に閉じていて、変化技を除くと短いため)。
- 呼ぶのは `submit()` / `scheduleSubmit()` と learners の操作だけ。`load()`・入力の変更では調整 API を呼ばない。
- 1回の送信で indices とモードの操作を並行に呼び(`async let`)、両方そろってから `outcome` を出す。どちらかが失敗したら
  `outcome = nil` にして `alertMessage` を出す(前の結果も残さない)。入力は消さない。
- 送り直し・画面を閉じたとき: `LatestTaskRunner` で前の Task を cancel し、世代番号でも古い応答を捨てる。`CancellationError` は
  エラーにしない。`cancelPendingWork()` は送信と learners の両方の Task を止め、`isLoading` を解く。
- 送信前の検査(違反なら API を呼ばず `alertMessage`。先に当たった1つだけ):
  1. 自分のポケモンと性格(`ownRequired`)
  2. 固定 SP: 各欄が空(= 0)か 0〜32 の整数(前後の空白も不可)(`spRangeInvalid`)
  3. 固定 SP の合計 ≤ 66(`spTotalExceeded`)
  4. bulk・offense(配分): 回す能力の上限が 0〜32 かつ固定 SP 以上(bulk: H・B・D、offense: A か C と S)(`ceilingBelowFixed`)
  5. offense: 素早さの目標が空か 0 以上の整数(`minSpeedInvalid`)
  6. 相手・技の必須: minKo = 自分の技 → 相手のポケモン、minSurvive = 相手のポケモン → 相手の技、
     bulk + 目標 = 相手のポケモン → 相手の技、offense + 目標 = 自分の技 → 攻撃の分類 = 自分の技の分類(`categoryMismatch`)→ 相手のポケモン
  7. 相手のプリセットに合う性格がマスタにある(`natureNotFound`。別の性格で代えない)
  順は Web の `AdjustScreen.tsx` と同じ(上限 → 素早さ → 目標の必須 → 分類 → 性格)。複数の違反が重なるときも Web と同じ文が出る。
- 要求の組み立て(ADR-0319 §5 と同じ): 個体は `speciesKey`・`natureId`・`sp`(固定 SP)・選んだときだけ `abilityId` / `itemId`。
  `thresholdPercent` は 100 なら nil、`modifier` はタイプ一致(技のタイプ ∈ 自分の種族のタイプ)なら 6144、そうでなければ nil、
  `damageModifier` は送らない。`ceiling` は回す能力だけ(bulk: hp/def/spd、offense: atk か spa と spe)。`goal` は「目標を指定する」のときだけ。
- 未対応の印: モードの結果の `unsupported` を `UnsupportedNoticeText.summary` で1回だけ(名前は見た技・持ち物・特性から引く)。数値は変えない。

### 6. 文言とエラー(`AdjustText`)

- 文言はすべて `AdjustText`(PokeCalcCore)に置き、Web の `adjustScreenText` / `adjustErrorText` と同じ語にする。View に日本語を直書きしない。
  例外: `missing_header` / `invalid_header` は Web の「ページを読み込み直して」を iOS では「アプリを開き直して」にする。
- エラーは code から `AdjustText.errorMessages` を引き、未知の code は「調整に失敗しました」、通信・デコード失敗は
  「調整の API に接続できません」。サーバーの `message` は出さない(既存の `CalcScreenError.message` の「エラー(code): message」方式は採らない)。
- 確率は engine の生値を 0.1% 単位で切り捨てて出す(99.99% → 「99.9%」。確定に見せない)。

### 7. 技を覚えるポケモン(機能 1)とページング・RequestLimits

- 自分の技・相手の技の横に「覚えるポケモン」(`adjustOwnLearnersButton` / `adjustOpponentLearnersButton`。accessibilityLabel は
  「自分の技を覚えるポケモン」/「相手の技を覚えるポケモン」)。技を選ぶまで無効。押すと `learners` を置き換えて先頭ページを読む。
- 1ページ `RequestLimits.moveLearnersPageSize`(= 契約の `limit` の既定 50)。返った件数がちょうど 50 なら「続きを読み込む」
  (offset = 読んだ件数)。一致なしは「この技を覚えるポケモンはいません」。エラーは一覧の中に出し、続きの失敗では一覧を残してもう一度押せる。
  別の技で開き直したら前の読み込みを cancel する。行は種族名だけ(自分に設定する操作は持たない。Web と同じ)。
- `RequestLimits` に `moveLearnersPageSize = 50` と `maxAdjustHits = 10`(`AdjustHits.maximum`)を足し、`ios/scripts/check-request-limits.sh`
  (`make ios-check-request-limits`)が契約の `listMoveLearners` の `limit` の `default` と `AdjustHits.maximum` と照合する。
  調整の要求は配列を持たないので `maxItems` の写しは増えない(ADR-0250 §5)。

### 8. a11y・見た目・識別子

- 見出しは見える語(§2 の表)で `.isHeader`。欄の accessibilityLabel は「<領域>の<ラベル>」(例「自分のポケモン」)で見える語を含む。
  未選択の欄は「ポケモンを選ぶ」「性格を選ぶ」「技を選ぶ」を出す。
- モードは縦に並ぶ選択肢(segmented にしない。5つは AX5 の幅に収まらない)。選択中は `.isSelected` trait。
  固定 SP と素早さの目標の欄は説明文を `accessibilityHint` にする。
- AX5 で横にはみ出さない(長い結果の行は折り返す)。アニメーションは入れない(結果は差し替えるだけ)。画像は使わない。
- エラーは `adjustAlert`(`Text`。VoiceOver に読ませるため `AccessibilityNotification.Announcement` を送る)。送信後のフォーカスは動かさない。
- 識別子(XCUITest が参照する): `adjustScreen`・`adjustBackendModeBadge`・§2 の表のカード・`adjustOwnSpeciesButton`・
  `adjustOwnNaturePicker`・`adjustOwnAbilityPicker`・`adjustOwnItemPicker`・`adjustOwnMovePicker`(Menu の Picker。項目は見える名前)・
  `adjustOwnLearnersButton`・`adjustFixedSP-<stat>`・`adjustFixedSPTotal`・`adjustMode-<mode>`・`adjustBulkFocus-<focus>`・
  `adjustCeiling-<stat>`・`adjustOffenseCategory-<category>`・`adjustMinSpeed`・`adjustUseGoalToggle`・`adjustOpponentSpeciesButton`・
  `adjustOpponentMovePicker`・`adjustOpponentLearnersButton`・`adjustOpponentAttackerPreset-<case>`・`adjustOpponentDefenderPreset-<case>`・
  `adjustHitsPicker`・`adjustThresholdPicker`・`adjustAlert`・`adjustLoading`・`adjustEmptyResult`・`adjustStatsLine`・
  `adjustFirepowerIndex`・`adjustPhysicalBulk`・`adjustSpecialBulk`・`adjustIndexNote`・`adjustHPCurrent`・`adjustHPLine-<0..3>`・
  `adjustModeResult`・`adjustRemaining`・`adjustMaxIndexPlan`・`adjustMinSpPlan`・`adjustMinSpNotRequested`・`adjustUnsupportedNotice`・
  `adjustLearnersHeading`・`adjustLearnerRow-<speciesKey>`・`adjustLearnersMore`・`adjustLearnersEmpty`・`adjustLearnersError`・`adjustLearnersClose`。
  `Text` を内に持つカードには `.accessibilityElement(children: .contain)` を付ける(ADR-0501 P6-14 §2 の申し送り)。

### 9. XCUITest の範囲(モック強制)

主要な操作が画面でつながることだけ: ルートから開く・空の結果・送信前の検査の文・indices の結果・minKo で相手と目標が出て結果が出る・
bulk の目標なしの案内・未対応の印・覚えるポケモンの一覧。数値は固定しない。AX5 の横はみ出しは送信後の結果の行まで確かめる。
既存の `LargeTextLayoutUITests.swift` は並行レーンが触るので触らず、`AdjustLargeTextLayoutUITests.swift` に補助関数を写す。

## 却下した案

- **`PokeCalcService` に5操作を足す**: §3 の理由(全準拠型の変更・並行レーンとの衝突)。
- **相手の防御側プリセットを Web と同じ8種にする**: iOS に ADR-0009 のカタログの型(性格の選び方を含む)を新しく持つことになる。
  3種で調整の用途は足り、逆算画面と同じ操作感になる。要望が出たら足す。
- **技の選択を検索シート(issue #68)にする**: learnset に閉じた短い一覧なので Menu で足りる。検索は件数が多い種族だけに使う。
- **`CalcScreenError` の文言を使う**: サーバーの message を出してしまう(ADR-0411 §3)。

## 受け入れ条件と担当テスト

| # | 受け入れ条件 | テスト |
|---|---|---|
| AC1 | `APIPokeCalcService` が契約どおりのパス・メソッド・ヘッダー・本文で呼び(nil は送らない・`self`・`minSpeed` 常時)、応答(int64・null・HPLineKind・未対応の印)を写し、エラーの code・通信失敗・デコード失敗・取り消しを区別する | `APIAdjustServiceTests` |
| AC2 | 開いただけ・入力を変えただけでは調整 API を呼ばない。既定は未選択・indices・上限 32・発数 1・確定 | `AdjustViewModelTests`(load・input)、`AdjustScreenUITests.testOpensFromRoot…` |
| AC3 | モードごとに契約どおりの要求(固定 SP・プリセット・タイプ一致・上限・目標・省略)を組み立て、indices と並行に呼ぶ | `AdjustViewModelTests` |
| AC4 | 送信前の検査の違反を日本語で止め、API を呼ばない | `AdjustViewModelValidationTests`、`AdjustScreenUITests.testSubmitWithoutOwn…`・`testFixedSPOverLimit…` |
| AC5 | 両方そろってから結果を出す。未対応の印を1回出す。確率は切り捨て | `AdjustViewModelSubmitTests`、`AdjustTextTests` |
| AC6 | エラーは code から日本語で、サーバーの message を出さない。失敗時は結果を出さず入力を残す | `AdjustViewModelSubmitTests`、`AdjustTextTests` |
| AC7 | 送り直し・画面を閉じたときに前の呼び出しを cancel し、古い応答で上書きせず、取り消しをエラーにしない | `AdjustViewModelSubmitTests`(resubmit・cancelPendingWork) |
| AC8 | 技を覚えるポケモン: 50 件ずつ・続き・空・エラー(一覧を残す)・開き直しで cancel | `AdjustViewModelLearnersTests`、`AdjustScreenUITests.testLearnersPanel…` |
| AC9 | モックは計算せず要求の形だけを反映し、逆引きは架空の learnset から引く | `MockAdjustServiceTests` |
| AC10 | `RequestLimits` の写しが契約と一致 | `make ios-check-request-limits` |
| AC11 | 既定・AX5 の文字サイズで主要な要素(結果の行を含む)が横にはみ出さず、モードは縦に並ぶ | `AdjustLargeTextLayoutUITests` |

## 結果

- iOS も計算しない(指数・探索・配分は calc-svc の engine)。補正はタイプ一致だけで Web と同じ規則。
- `AppEnvironment.ready` の関連値が1つ増える(タイプバランスの iOS と同じ箇所。後から入る側が合わせる)。
- 実装後(2026-10-02。実測): XCTest 684 件・XCUITest 64 件がすべて成功(`make ios-test-unit` / `make ios-test-ui`。
  iPhone 17e シミュレータ)。うち調整画面の XCUITest は `AdjustScreenUITests`・`AdjustLargeTextLayoutUITests`。
- 結果の「素早さの目標を満たします」の表示は送信時の値(`AdjustModeResult.allocation` の `speedTargetRequested`)で決め、
  送信後に入力を書き換えても結果は変わらない。送り直しを始めたら前の結果を消す(Web と同じ)。
