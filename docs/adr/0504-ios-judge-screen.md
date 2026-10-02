# ADR-0504: iOS の判定画面

- 状態: 採用(2026-10-03。P6-25。ユーザー決定「iOS に判定画面を作る」〈DECISIONS.md 2026-10-03〉の具体化。spec-writer の判断)
- 日付: 2026-10-03
- 関連: ADR-0503(契約ごとの別ターゲット生成・素早さ画面。この ADR の前提)、ADR-0501「P6-25 の受け入れ条件」、ADR-0700〜0708(判定。特に 0705 §3〜§8 の Web 画面・0708 の未対応の印)、
  ADR-0123(未対応の印)、ADR-0012(サービスローカル契約)、ADR-0802(エラー形)、docs/judge-design.md、requirements.md §2、services/judge/api/openapi.yaml

## 背景

判定(「想定した相手を抜いて、倒せるかを 1 回で確認」。requirements.md §2)は Web(`web/src/judge/JudgeScreen.tsx`)にだけあった。iOS にも同じ画面を作る。契約は root の `api/openapi.yaml` ではなく
サービスローカルの `services/judge/api/openapi.yaml`。実用の endpoint は `POST /api/judge/v1/outspeed-and-ko`(`outspeedAndKo`)の 1 本で、自分 1 体と相手候補(1〜6 件。候補ごとに撃ち返す技を持つ)から、
候補ごとの「素早さ・優先度・行動順・双方向の確定数」と、確定数に付く未対応の印(方向ごと)を返す。勝敗の真偽値は返さない(ADR-0700 §6-1)。

## 決定

### 1. 生成方式: ADR-0503 と同じ形で別ターゲット `PokeCalcJudgeAPI`

- `ios/scripts/openapi-gen.sh` の `targets` 配列に 1 行、`ios/tools/openapi-gen/openapi-generator-config.judge.yaml`(speed と同形。契約に `tags` が無いので `filter` なし)、`Package.swift` の target と依存(`PokeCalcCore`・テスト)。
  `make ios-gen`・`make ios-gen-check` の出力は 3 本になる。`PokeCalcAPI`・`PokeCalcSpeedAPI` の生成物は不変。
- `APIJudgeService.swift` だけが `PokeCalcJudgeAPI` を import する(3 つのターゲットは `Client`・`Components` が衝突する)。
- 接続先: judge は gateway ではなく自分の Ingress(`/api/judge` prefix。ADR-0700 §6-3)を持つが、ホストは他の API と同じなので `baseURL` と `ClientIdentity` を共有する(Web の `judgeClient` と同じ。ADR-0705 §3)。
- 契約に `default` 応答が無いので、契約外のステータスは生成クライアントが `undocumented` で返す。本文が `{code,message}` ならその `code`、読めなければ `client_unexpected_status`(ADR-0503 §3 と同じ)。
  `ErrorCode` の enum に無い code は `decode` になる(古いアプリの挙動。DECISIONS.md 2026-10-03 の連絡のとおり、増やす前に相談する)。

### 2. 契約との同期

- 生成された `Format`・`ErrorCode`(8 値)と、ドメイン・文言の対応表の一致を `JudgeContractSyncTests` が固定する。未対応の印の `target`・`reason` は契約で enum にしない(ADR-0215・ADR-0708 §4)ので、
  ドメインは知らない値を `.unknown` に写す(既存の `UnsupportedTarget`・`UnsupportedReason` を再利用)。
- 範囲の数値は専用の複製を作らない。能力ポイント(各 0〜32・合計 66)・ランク(±6)は `SPLimits`・`RankLimits` を使い、判定固有の 3 つ(候補数の最小・最大、技 ID の最大長)だけ `RequestLimits` に写しを足した
  (`maxJudgeDefenders`・`minJudgeDefenders`・`maxJudgeMoveIdLength`)。契約との一致は `ios/scripts/check-request-limits.sh` の判定の節が見る(`StatBlock` 6 項目・`RankBlock` 5 項目も全項目)。

### 3. 範囲: single 固定・`field` は送らない

- **`format` は single 固定**(ユーザー決定でダブルは対象外)。契約上は必須なので要求の型には持ち、画面は選ばせない。
- **`field`(天候・地形・壁)は送らない**(Web の JD5 と同じ。ADR-0705 §6)。判定の価値は素早さと確定数で、場の効果は `speedField`(トリックルーム・自分の追い風・相手の追い風)に絞る。`speedField` は全候補に共通
  (候補ごとに変えられない旨を画面に出す。ADR-0703 §5)。天候などを足すときは別の ADR で、`JudgeRequest` に `field` を足す。

### 4. サービス境界と ViewModel

- 画面が依存するのは新しいプロトコル `JudgeService`(`outspeedAndKo(_:)` の 1 本)。`PokeCalcService`・`SpeedService` には混ぜない(絶対ルール 5: 判定の失敗が計算・構築に影響しない)。
  実装は `APIJudgeService` と `MockJudgeService`。ドメインの型は生成型に依存しない。
- `JudgeViewModel`(`PokeCalcCore`・`@MainActor @Observable`)は、入力(自分 1 体・候補 1〜6 件・場の効果)から要求を作り、応答を表示用に整える。計算は持たない。
  マスタ(性格・持ち物・技・種族・特性)と構築は、判定とは別に `PokeCalcService`・`TeamStore` から読む(互いの失敗に巻き込まれない。`load()` は throw しない)。
- **送信は「判定する」ボタンを押したときだけ**(入力のたびに送らない。debounce もしない)。1 リクエストが上流を最大 27 回逐次で叩くため、打鍵ごとの呼び出しは重すぎる(ADR-0705 §7)。
  送信のたびに世代を進めて先行を cancel し、最新の世代の応答だけ反映する(cancel を無視する古い応答も捨てる)。判定中は `resultState = .loading`。
- 送信前に契約の範囲を検査し、違反なら呼ばずに理由を出す(誰の入力かを先頭に付ける)。能力ポイントの 1 ステータスごとの範囲・ランクの範囲は setter が収めるので検査の対象は
  必須(種族・性格・技)・技 ID の長さ・能力ポイント合計だけ。違反では直前の結果を消さない。
- 入力はマスタから選ぶ(**ID の自由入力にしない**。Web は ADR-0304 §3 の「ID から技を引く API が無い」制約で技を ID 入力にしたが、iOS は `searchMoves` が使える)。種族・技は既存の検索シート、性格・特性・持ち物は
  新しい選択シート(`Menu` は使わない)。性格は `load()` が「補正なしの最初」を既定として入れる(必須の入力を 1 つ減らす)。特性は種族の `species(key:)` の候補から選び、種族を変えたら新しい種族に無い特性は外す(任意)。

### 5. 要求の省略規則(Web と同じ。最小の要求を送る)

ランクは 5 項目すべて 0 なら送らない(非 nil なら 5 項目すべて)・特性/持ち物は未選択なら欄ごと送らない(`null` を送らない)・`speedField` は 3 つすべて false なら送らない(非 nil なら 3 項目すべて)。
省略の判断は ViewModel(`makeRequest()`)が持ち、`JudgeIndividual.ranks`・`speedField` などを nil にして渡す。`APIJudgeService` はその値をそのまま写す(判断を二重に持たない)。

### 6. 結果の見せ方: 丸めない・取り違えない・印は方向ごと

- 行は応答の `defenderIndex` で要求の候補と対応づける(位置ではなく index。行を並べ替えて表示しても取り違えない)。種族名は judge が返さないので**送信時点の要求**から引く(判定中に入力を変えても変わらない)。
- 素早さ・優先度・行動順・確定数は応答のまま(同速・行動順が決まらないを真偽値 1 つに丸めない。確定数は「倒せない / 確定 n 発 / 乱数 n 発(x.x%)」)。「勝ち」「負け」の語を持たない。
- **未対応の印は iOS が先に出す**(ADR-0708)。方向ごとに分けて混ぜない: 順方向(`attackerKoUnsupported` = 自分の技 → その候補)と逆方向(`defenderKoUnsupported` = その候補の技 → 自分)を別々に既存の
  `UnsupportedPlacement` へ渡し、全候補に共通する印は結果の上に 1 回(方向ごと)、一部の候補だけの印はその行に出す。**その方向の確定数を確定として見せない添え書き**は、置き場所(共通・個別)によらず印のある行の
  その方向だけに付ける(順方向の印が逆方向の確定数を疑わしく見せない)。
- `target` の `attacker_*`/`defender_*` は**その calc から見た役割**(ADR-0708 §5)。書き換えず、既存の `UnsupportedMarkLabel`(「攻撃側の持ち物」等)を使い、方向の見出し(「自分の技の確定数」「相手の技の確定数」)の下に出す。
  逆方向では「攻撃側 = その候補」だが、見出しが方向を言うので読み違えない(自分の持ち物と読み替えない)。名前の辞書は既存の `UnsupportedMarkNames`(技・持ち物・特性。無ければ ID)。
- エラーは `code` から日本語にする(サーバーの英語 message は出さない)。サーバーの message の `defenders[<index>]`(ADR-0703 §3)が候補数の範囲内なら、「相手候補 N の入力で失敗しました」を添える
  (index だけを読む。message の他の部分は使わない。`attacker` の失敗には付けない。ADR-0706 §4)。エラーでも入力は消さない。

### 7. 構築から呼び出す(iOS の強み。最小の範囲)

- 自分側と各候補側を、構築のメンバー 1 体で埋められる(`applyTeamMember(teamID:memberID:to:)`)。種族・性格・SP・特性・持ち物・技を写し、ランクは構築に無いので 0 に戻す。
  技は `TeamMemberConverter` の規則(最初のダメージ技 → 無ければ最初の技 → 無ければ未選択)。候補は技が必須なので、技が無ければ送信前の検査が止める。
- 写したあとも各欄は手で直せる(スナップショット。構築側の以後の編集には追従しない)。無効な ID・範囲外の対象は何もしない。ストアには書かない(読むだけ)。
- 入口は既存の `TeamSourceMenuRow`(計算・逆算と同じ部品)を再利用する(`identifierPrefix` = `judgeAttackerTeam`/`judgeCandidate<n>Team`)。
- やらないこと: 構築の全メンバーを候補に一括で入れる・構築全体を相手にする(最小の範囲を超える。要望が出たら別タスク)。

### 8. モック

`MockJudgeService`(架空データ。環境変数 `POKECALC_MOCK_JUDGE` でシナリオを切り替える。`error` / `candidate-error` / `marks`、なし・未知は正常)。**候補ごとに違う値を返す**
(行の取り違えを画面・テストが検出できるように。ADR-0705 §8)。UI テストと `MockJudgeServiceTests` が頼る固定の事実(i = 候補の index。0 始まり):

- 素早さ: 自分 150(自分の追い風で 300)。候補 i は 100 + 25 × i(相手の追い風で 2 倍)。同速は i = 2 の追い風なしのとき。トリックルームは `outspeeds` の向きだけを変える(実数値・同速は変えない)。
- 優先度: 自分 0。候補は 0、ただし i = 1 だけ 1(素早さで抜いていても先に動けない行)。行動順は優先度が違えば優先度、同じなら `outspeeds`、同速なら `turnOrderTie`。
- 自分の技の確定数: hits = i + 1。i が偶数なら確定(100.0)、奇数なら乱数(10.0 + 5.0 × i)。候補の技の確定数: i = 0 は倒せない(hits 0)、それ以外は hits = 6 - i。i >= 4 は確定、それ以外は乱数(12.5 × i)。
- 印: 既定は空。`marks` では順方向は全行に [技 multi_hit(自分の技 ID)]、逆方向は i = 1 の行だけ [技 variable_power(その候補の技 ID)]。
- `error` は `upstream_unavailable`、`candidate-error` は候補が 2 件以上のとき `invalid_request`(message が `defenders[1]` を示す)。候補が 0 件・上限超は `invalid_request`。
- モックの式は本物を写したものではない(判定の正しさは保証しない)。

### 9. 文言

`JudgeLabels`(Core)に集約し、Web の `judgeScreenText`・`judgeErrorText` と同じ文言にそろえる。違いは次の 4 点だけ:

1. 技は ID の自由入力ではなくマスタから選ぶので、ラベルは「技」(Web は「技の ID」。ヒントの文言も不要)。
2. 能力ポイント・ランクのラベルのステータス名は日本語名(既存の `StatKeyLabel`。Web は英字 1 文字)。
3. 確定数の確率は小数第 1 位固定(`BulkRowDisplay.koText` と同じ。Web は数値そのまま)。
4. 未対応の印の添え書きと、方向ごとの注記(Web にはまだ無い。ADR-0708 の画面側)。

エラーの対応は `judgeErrorText` に `not_found`(Web は未知の code の扱い)を足した: 契約外の 404/405 が本文つきで届いたとき、`判定に失敗しました`(汎用)にする。通信できない・応答が読めない・契約外のステータスは
「判定の API に接続できません」(Web の `judge_unavailable` と同じ)。

### 10. 識別子(XCUITest が使う名前)

`JudgeScreenUITests` の冒頭に一覧がある(ルート `openJudgeScreen`、画面 `judgeScreen`、各体 `judge<Attacker|Candidate<n>>` + `SpeciesButton`・`NatureButton`・`AbilityButton`・`ItemButton`・`MoveButton`・`SPValue-<stat>`・
`SPIncrement-<stat>`・`SPDecrement-<stat>`・`SPTotal`・`RankValue-<stat>`・`RankIncrement-<stat>`・`RankDecrement-<stat>`・`TeamSourceButton`・`TeamEmptyMessage`、選択シート `judgeOptionSheet`・`judgeOption-<id>`・`judgeOptionNone`、
候補 `judgeAddCandidate`・`judgeRemoveCandidate-<n>`・`judgeCandidateLimitNotice`、場の効果 `judgeTrickRoom`・`judgeAttackerTailwind`・`judgeDefenderTailwind`・`judgeDefenderTailwindNotice`、送信 `judgeSubmit`・
`judgeValidationError`・`judgeLoading`・`judgeEmptyResult`、結果 `judgeResult`・`judgeRow-<i>` と行の各要素・`judgeAttackerUnsupportedSummary`・`judgeDefenderUnsupportedSummary`、失敗 `judgeError`・`judgeErrorCandidate`・`judgeMasterError`)。

## 却下した案

- **技を ID の自由入力にする(Web と同じ)**: iOS は技の検索・解決の API(`searchMoves`・`getMove`)を使え、ID の打ち間違い(`unknown_move`)を画面で避けられる。入力の手間も小さい。
- **入力のたびに送る**: 1 リクエストが上流を最大 27 回叩く(ADR-0705 §7)。送信ボタンだけにする。
- **`format` を選ばせる・`field` を送る**: ユーザー決定でダブルは対象外。`field` は Web の JD5 も送らない。必要になったら別の ADR。
- **順方向と逆方向の印を 1 つの配列にまとめて 1 回だけ出す**: どちらの確定数が疑わしいか、`attacker_item` が誰の持ち物か読めなくなる(ADR-0708 §4・§5)。
- **`defenderKoUnsupported` の `target` を自分・相手の語に読み替える**: 契約の値を iOS が再解釈すると正が 2 つになる(ADR-0708 §4 の byte-for-byte と同じ立場)。方向の見出しで読み違いを防ぐ。
- **構築のメンバー全員を候補へ一括で入れる**: 最小の範囲を超える(§7)。

## 結果

- root の API・Web・judge の契約は変えない。iOS が `/api/judge/v1/outspeed-and-ko` を使い始める(契約を変える側への連絡は DECISIONS.md 2026-10-03「iOS に判定画面を作る」)。
- タイプバランス(P6-26)も同じ形(配列に 1 行 + 設定ファイル + Package.swift の target)で足せる。
- 追加するテスト数・失敗数と実装者への注意は ADR-0501「P6-25 の受け入れ条件」。
