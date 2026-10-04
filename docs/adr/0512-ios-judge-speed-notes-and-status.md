# ADR-0512: iOS の判定画面に「素早さの反映/無視」の表示と状態異常の入力を足し、契約の未知の値で落ちないようにする

- 状態: 採用(受け入れ条件とテストまで。実装は未着手)
- 日付: 2026-10-04
- レーン: iOS
- 関連: ADR-0504(判定画面。契約 v0.2.0 への追従章)、ADR-0710(素早さの反映/無視の欄)、ADR-0712(まひ)、ADR-0714(特性・持ち物の素早さ効果。
  契約 v0.3.0 で SpeedFactor に ability・item が増えた)、ADR-0705(Web の判定画面)、ADR-0708(印は方向別)、ADR-0501「判定の素早さ反映/無視と状態異常の受け入れ条件」
- 番号: iOS 帯 0500〜。0509 が 2 つ重複しているため 0510 以降。open PR との衝突を避けて 0512 を使う
- 対象外(ユーザー決定 2026-10-03): ダブル・テラスタル。`format: double` は送らない。`field`(天候・地形・壁)も送らない(ADR-0504 §3)

## 背景

1. 応答の `Matchup` は、素早さに実際に効かせた補正(`attackerSpeedApplied`/`defenderSpeedApplied`。値 rank・tailwind・ability・choiceScarf・item・paralysis)と、
   指定されたのに反映しなかった入力(`attackerSpeedIgnored`/`defenderSpeedIgnored`。値 abilityId・itemId・fieldWeather)を必須配列で返す。
   iOS は `JudgeMatchup` に文字列で運んでいるが画面には出していない。ユーザーが指定した特性・持ち物が素早さに効いていないのに、何も言わないのは「黙って無視」になる。
2. 要求の `Individual`/`DefenderCandidate` に `status`(省略可)がある。judge は paralysis だけ素早さに反映(×0.5)し、calc-svc には全部転送する。iOS の判定画面には入力が無い。
3. 生成型の `SpeedFactor`・`SpeedIgnoredInput` は閉じた enum(`@frozen`)。契約に値が増えると、古いアプリは応答ごと decode に失敗する(今回 v0.3.0 で ability・item が実際に増えた。
   判定レーンからの依頼: 未知値を許容し、落ちずに文字列のまま運ぶ)。

## 決定

### 1. 表示(行ごと・方向ごと。空なら出さない)

- 各候補の行に最大4つの文を足す。`attacker*` は**自分**、`defender*` はその行の**相手候補**(契約の説明。attackerSpeed はどの行でも同じ値になるが、行を自己完結させるために各行に付く)。
  - 「自分の素早さに反映: ランク補正・追い風」(`attackerSpeedApplied`)/「相手の素早さに反映: こだわりスカーフ・まひ」(`defenderSpeedApplied`)
  - 「自分の素早さに特性は反映していません」(`attackerSpeedIgnored`)/「相手の素早さに持ち物・天候は反映していません」(`defenderSpeedIgnored`)
- **空配列の文は出さない**(識別子も付けない)。並びは応答のまま(契約が計算の連鎖順・abilityId → itemId → fieldWeather で固定しているので、並べ替えない)。区切りは「・」。
- 文言は Web の `judgeScreenText`(web/src/i18n/judge.ts。`speedAppliedNote`・`speedIgnoredNote`・`speedFactorLabel`・`speedIgnoredLabel`・`speedSideSelf/Opponent`)と同じ語にする。
  ランクは「ランク補正」(Web に合わせる。「ランク」ではない)。文言は `JudgeLabels` に集約する。
- 「反映していません」は契約の意味どおり「指定されたが素早さへの効き方を確定できなかった」。効果が無いと確定したものや条件が不成立のものは API が返さないので、画面は出ているものだけを文にする(推測で足さない)。
- 位置: 素早さの行(`judgeRowSpeed-<i>`)の近く。行の識別子は §6。

### 2. 状態異常の入力(足す。Web と同じ語・同じ順)

- Web の判定画面には状態異常の select がある(`statusLabel`「状態異常」・none/burn/paralysis/poison/badly_poison/sleep/freeze = なし/やけど/まひ/どく/もうどく/ねむり/こおり。issue 235)。iOS も自分と各候補に足す。
- 選択は**シート**(既存の `JudgeOptionSheet` の kind に `status` を足す)。`Menu` は使わない(中のボタンに identifier が付かない。ADR-0504 §4)。
  性格・特性・持ち物のボタンと同じ並び(持ち物の次)に「状態異常」ボタン。シートの行は7値で、「なし」が実の選択肢なので「未選択に戻す」行は出さない。
- **送信規則(ranks と同じ省略の流儀)**: ViewModel の `JudgeDraft.status` は既定 `.none`。要求の `JudgeIndividual.status` は `.none` なら nil(欄ごと送らない。null も送らない)、
  それ以外は選んだ値だけ契約の値(`badly_poison` はスネークケース)のまま送る。契約は「省略と none は同じ」なので、none を送る必要が無く、従来の要求本文は1バイトも変わらない。
- **リセット**: 種族を変えても状態異常は残す(ランクと同じ。ユーザーが選んだ戦況の仮定)。構築のメンバーを呼び出したときは `JudgeDraft` ごと作り直すので `.none` に戻る(構築に状態異常は無い。ランクと同じ)。
  候補の削除では状態異常も候補について行く(位置に貼り付かない)。入力の編集では judge を呼ばない(送信ボタンのみ。ADR-0705 §7)。
- 反映は judge が決める(まひだけ素早さ×0.5・特性の「状態異常のとき」の条件に none 以外を使う。ADR-0712・0714)。iOS は素早さの計算をしない(行の素早さは応答のまま)。

### 3. 未知の SpeedFactor/SpeedIgnoredInput で decode を落とさない

選択肢と判断:

- (a) 判定の API 写像だけ生の JSON として読む(生成の `Client` を使わず手で decode): ヘッダー・エラー分類・ステータス処理を作り直すことになり、写像テストの資産と二重になる。却下。
- (b) 生成の設定で enum を開く(openapi-generator の設定・スキーマ): 生成物・スキーマに手を入れるのは範囲外(契約は判定レーンのもの。iOS から変えない)。swift-openapi-generator に「未知値を許す enum」の設定は無い。却下。
- (c) **`ClientMiddleware` で成功応答の本文を前処理する(採用)**: `Client(serverURL:transport:middlewares:)` に判定専用のミドルウェアを付ける。
  200 の JSON 応答から `matchups[i].{attackerSpeedApplied,defenderSpeedApplied,attackerSpeedIgnored,defenderSpeedIgnored}` の4配列を取り出し、**呼び出しごとの入れ物**(`@TaskLocal` に置く参照型。
  共有の可変状態を持たない=並行呼び出しで混ざらない)へ生の文字列として退避し、生成型には4欄とも `[]` にした本文を渡す(これで未知の値は enum に触れない)。
  `APIJudgeService.domainMatchup` は、退避した生の文字列で `JudgeMatchup` の4欄を作る。生成物は触らない・他の欄の decode・エラーの分類は変えない。
- 厳格さは保つ: 欄が無い・配列でない・文字列でない要素を含む応答は、ミドルウェアは触らず(そのまま渡す)、従来どおり decode エラーになる(必須欄の欠落を「空」と読まない。ADR-0708 §3 と同じ立場)。
  本文が JSON として読めない場合も同じ。
- 画面の文言は未知の値を**汎用の日本語**(「その他の補正」「その他の入力」)にする(契約の英語の値を画面に出さない。`JudgeLabels.errorMessage` の未知の code → 汎用文言と同じ流儀)。ドメインは文字列のまま運ぶ。
- **ErrorCode が増えたとき**は従来どおり(ADR-0504 の追従章): 生成型の ErrorCode は閉じた enum のまま、エラー応答の decode が失敗して `PokeCalcError.Code.decode`(= 画面は「判定の API に接続できません」)になる。
  文言の既知の値は `JudgeContractSyncTests` が契約と照合し、増えたら文言を足す合図になる。今回は SpeedFactor/SpeedIgnoredInput だけ未知値を許容する(成功応答の中の表示用の補足で、落とすと本体の判定結果まで失う)。
  ErrorCode まで広げるかは、増える頻度が上がったら別 ADR。
- 契約同期テストは「既知の値の文言を足し忘れない」合図として残す(`JudgeContractSyncSpeedTests`)。落ちないことの保証は `APIJudgeServiceUnknownSpeedValuesTests`。

### 4. モック

`POKECALC_MOCK_JUDGE=speed-notes` を足す(既定・既存のシナリオは4欄とも空のまま=既存のテスト・画面を変えない)。固定値は ADR-0501 の章と `MockJudgeServiceSpeedNotesTests`。
要求の状態異常がまひなら、その側の Applied の末尾に paralysis を足す(状態異常の選択 → 要求 → 表示の一連を XCUITest で確かめるため。式は本物ではない)。

### 5. 判定は計算に影響しない(絶対ルール5)

表示の追加・状態異常の入力は、判定画面の中だけで完結する。計算・構築の画面・サービスには触れない。

### 6. 識別子(追加のみ。既存は変えない)

- `judgeAttackerStatusButton`・`judgeCandidate<n>StatusButton`(選択シートは既存の `judgeOptionSheet`。行は `judgeOption-<none|burn|paralysis|poison|badly_poison|sleep|freeze>`)
- `judgeRowAttackerSpeedApplied-<i>`・`judgeRowDefenderSpeedApplied-<i>`・`judgeRowAttackerSpeedIgnored-<i>`・`judgeRowDefenderSpeedIgnored-<i>`(i = defenderIndex。空なら存在しない)
- 子が1つだけの `.contain` は識別子を畳むので、結果の枠(`judgeResult`)は見出しなど子を2つ以上持たせる既存の作りを保つ。行の文は `.contain` の中の個別の Text に識別子を付ける(畳まれない)。

## 却下した案

- **反映していない入力を非表示にする / 開発者向けの補足にする**: 指定したのに黙って無視する状態になる。契約の説明(「画面は空でないとき『素早さは特性・持ち物・天候を反映していない』旨を添える」)にも反する。
- **none を明示的に送る**: 従来の本文が変わる。契約上は省略と同じ。
- **状態異常を種族変更でリセットする**: ユーザーが選んだ戦況の仮定を勝手に消す(ランクも消さない)。
- **未知値を無視して配列から落とす**: 「空配列は素早さに影響する要素が無い保証」(契約)を破る。文字列のまま運ぶ。
- **ErrorCode も未知値を許容する**: 範囲外。上記。

## 結果

- root の API・Web・judge の契約は変えない。iOS が `status` を送り、`*SpeedApplied/Ignored` を表示する。判定レーンへ: SpeedFactor/SpeedIgnoredInput に値を足すのは iOS を壊さない(未知値を許容)。ErrorCode を増やすときは従来どおり相談(docs/ai-shared/decisions/2026-10-04-ios-judge-speed-factors.md)。
- 追加するテスト数・失敗数と実装者への注意は ADR-0501「判定の素早さ反映/無視と状態異常の受け入れ条件」。
