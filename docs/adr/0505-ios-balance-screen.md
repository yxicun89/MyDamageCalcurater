# ADR-0505: iOS のタイプバランス画面

- 状態: 採用(2026-10-03。P6-26。ユーザー決定「iOS にタイプバランス画面を作る」〈DECISIONS.md 2026-10-03〉の具体化。spec-writer の判断)
- 日付: 2026-10-03
- 関連: ADR-0503(契約ごとの別ターゲット生成。この ADR の前提)、ADR-0504(判定画面。同形)、ADR-0501「P6-26 の受け入れ条件」、ADR-0014・0016・0017(balance の防御・攻撃範囲・特性)、
  ADR-0013(タイプ相性表の正本)、ADR-0303(Web のタイプバランス画面)、ADR-0802(エラー形)、ADR-0012(サービスローカル契約)、docs/type-balance-design.md、services/balance/api/openapi.yaml

## 背景

タイプバランスチェッカー(最大 6 体のパーティの防御相性・攻撃範囲を一覧にする。type-balance-design.md §1)は Web(`web/src/screens/BalanceScreen.tsx`)にだけあった(設計書 §11「iOS のタイプバランス画面: 未対応」)。
iOS にも同じ画面を作る。契約は root の `api/openapi.yaml` ではなくサービスローカルの `services/balance/api/openapi.yaml`(5 本の POST: analyze・coverage・threats・recommendations・move-range)。
入力は `pokemonId`(= 構築の `speciesKey`)・特性 ID・技 ID だけで、**タイプ・相性は balance が read model から引く**(ADR-0014 §1)。応答は倍率・分類・集計・攻撃範囲を既に判定済みで返す。

## 決定

### 1. 生成方式: ADR-0503 と同じ形で別ターゲット `PokeCalcBalanceAPI`

- `ios/scripts/openapi-gen.sh` の `targets` 配列に 1 行、`ios/tools/openapi-gen/openapi-generator-config.balance.yaml`(judge と同形。契約に `tags` が無いので `filter` なし)、`Package.swift` の product・target・依存(`PokeCalcCore`・テスト)。
  `make ios-gen`・`make ios-gen-check` の出力は 4 本になる。`PokeCalcAPI`・`PokeCalcSpeedAPI`・`PokeCalcJudgeAPI` の生成物は不変(spec の段階で生成して確認済み)。
- `APIBalanceService.swift` だけが `PokeCalcBalanceAPI` を import する(4 つのターゲットは `Client`・`Components` が衝突する)。
- 接続先: balance は gateway の `/api/balance/*`(issue #284)から届く。ホストは他の API と同じなので `baseURL` と `ClientIdentity` を共有する。gateway の `GATEWAY_BALANCE_URL` が未設定なら gateway が 503 を返す
  (本文が `{code,message}` ならその code、読めなければ `client_unexpected_status` →「接続できません」)。直結の Ingress の撤去と URL の配線は balance レーンの別タスク(設計書 §11)で、iOS はどちらでも同じパスを叩く。
- 契約に `default` 応答が無いので、契約外のステータスは生成クライアントが `undocumented` で返す。本文が `{code,message}` ならその `code`、読めなければ `client_unexpected_status`(ADR-0503 §3・ADR-0504 §1 と同じ)。
  `ErrorCode` の enum に無い code は `decode` になる(古いアプリの挙動。DECISIONS.md 2026-10-03 の連絡のとおり、増やす前に相談する)。

### 2. 契約との同期

- 生成された `ErrorCode`(10 値)・`TypeId`(18 値・正準順)・`DefenseCategory`・`EffectSource`・`DefenseEffect`・`CoverageMultiplier` と、ドメイン・文言の対応表の一致を `BalanceContractSyncTests` が固定する。
  `TypeId` は `PokeType` と値も順序も同じで、iOS はタイプの一覧・相性を持たず応答のタイプを `PokeType` に写すだけ。
- 範囲の数値は `RequestLimits` に写しを足した(`maxBalanceMembers` 6・`minBalanceMembers` 1・`maxBalanceMovesPerMember` 4・`maxBalanceMoveIdLength` 40)。
  メンバー数と技の件数は構築の `TeamLimits` と同じ値で、1 つの構築をそのまま送れる(単体テストが両者の一致を固定する)。契約との一致は `ios/scripts/check-request-limits.sh` の balance の節が見る
  (`AnalyzeRequest`・`CoverageRequest` の `members` の min/maxItems・`CoverageRequestMember.moveIds.maxItems`・`MoveId.maxLength`。1 つずらすと失敗することを確認済み)。判定の `MoveId` は 64 文字・balance は 40 文字で別の契約なので別の定数。

### 3. 範囲: analyze(防御相性)と coverage(攻撃範囲)の 2 本

- 使うのは `analyzeTeamBalance`(`POST /api/balance/v1/team-balance/analyze`)と `analyzeTeamCoverage`(`.../coverage`)。どちらも「構築のメンバー(最大 6 体)」から作れる入力で、構築から選ぶ入口(§7)に素直に合う。
  結果は契約が返す分類(弱点・耐性・無効・等倍・4 倍・特性による変化)と集計(弱点数・うち 4 倍・耐性数・無効数・等倍数)、攻撃範囲(最大倍率・有効・抜群・技が無い)をそのまま表示する。
- **後続(別タスク)**: `threats`(仮想敵。入力に仮想敵の最大 6 体が要る)・`recommendations`(おすすめタイプ。待たせず 503 `overloaded` を返す同時実行上限がある。ADR-0409。結果にポケモン名の一覧が付き大きい)・
  `move-range`(技構成 1〜4 件の攻撃範囲と受けられる実在ポケモン。構築と別の入力)。契約・生成物は 5 本分ができるので、足すときは `BalanceService` に操作を足すだけでよい。画面のスクリーンショットの一覧性を保つため、v1 は 2 本に絞る。
- 総合点・ランキング・独自スコアは作らない(type-balance-design.md §1)。弱点が多いかどうかの判定・おすすめの文言も iOS に持たない(集計の数を並べるだけ)。

### 4. サービス境界と ViewModel

- 画面が依存するのは新しいプロトコル `BalanceService`(`analyzeTeamBalance(_:)`・`analyzeTeamCoverage(_:)`)。`PokeCalcService`・`SpeedService`・`JudgeService` には混ぜない(絶対ルール 5: balance の失敗が計算・構築に影響しない)。
  実装は `APIBalanceService` と `MockBalanceService`。ドメインの型は生成型に依存しない。
- `BalanceViewModel`(`PokeCalcCore`・`@MainActor @Observable`)は、構築(`TeamStore`)の一覧を読み、選ばれた構築から要求を作り(`BalanceRequestBuilder`)、応答を表示用に整える。計算は持たない。
- **構築を選んだ時点で送る**(1 回の選択 = analyze と coverage の 1 組の呼び出し。判定のような「送信ボタン」は無い: 入力が選択だけで、打鍵ごとの送信が起きず、1 回の呼び出しは軽い)。
  失敗後・構築の編集後は「もう一度解析する」(`reanalyze()`。構築の一覧を読み直し、選択中の構築の最新の内容で解析し直す)。
- **2 つの呼び出しは独立**(Web の ADR-0303 §9 と同じ): それぞれの状態 `.idle / .loading / .loaded / .failed / .skipped` を持ち、片方の失敗・遅延がもう片方の表示を消さない。
  送るたびに世代を進めて先行を cancel し、最新の世代の応答だけ反映する(cancel を無視する古い応答・古い失敗も捨てる)。`CancellationError` は失敗にしない。画面を離れるときは進行中を cancel して `.loading` を `.idle` に戻す。
- 名前の引き当て(ニックネーム → 種族名 → speciesKey。特性名)はマスタ(`PokeCalcService.species(key:)`)から行い、**失敗しても解析は止まらない**(名前が ID に落ちるだけ)。構築の読み込みの失敗(`teamLoadFailed`)・balance の失敗・マスタの失敗は互いに影響しない。`load()` は throw しない。
- 結果の名前は**送信時点の構築**から作る(index で引く。同じ種族が 2 体いても取り違えない)。

### 5. 要求の規則(最小の要求を送る)

- `pokemonId` は構築の `speciesKey` をそのまま使う(Web も同じ)。メンバーは構築の順で先頭から 6 体まで(`TeamValidator` を通らない壊れた保存データの 7 体以上でも契約の上限を超えて送らない)。
- analyze: `abilityId` は nil・空文字なら欄ごと送らない(`null` を送らない)。coverage: `moveIds` は構築の順のまま重複を除き(契約の `uniqueItems`)先頭から 4 件まで。**技の無いメンバーも空配列で載せる**(`moveIds` は必須。契約上 0〜4 件)。
  技を 1 つも持たないメンバーしかいない構築では coverage を呼ばない(`.skipped`。画面は案内)。変化技かどうかは iOS が判断せず、技を持つメンバーを全員載せてサーバーに任せる(変化技だけのメンバーは `attackTypes` が空・「攻撃技なし」で返る)。
- メンバーが 0 体の構築を選んだときは、どちらも呼ばず `.skipped`(画面は「この構築にはポケモンがいません」)。タイプ・相性・倍率は要求に含めない(単体テストが固定する)。

### 6. 結果の見せ方: 相性を計算しない・サーバーの値のまま

- 倍率は応答の**文字列のまま**(`"4"`・`"1/4"`・`"3/4"`)。float にしない。語は応答の `category` から選び(`quad_weak` も `weak` も「弱点」。×4 は倍率に出る)、値の範囲を iOS で判定し直さない。分類と倍率が食い違う応答でも応答のまま出す(単体テストが固定する)。
- 集計(弱点・うち×4・耐性・無効・等倍)・有効/抜群の人数・有効/抜群の真偽は**応答の数のまま**。メンバーから数え直さない・並べ替えない(正準順 normal … fairy のまま)・「偏り」の閾値・おすすめの語を持たない。
- 防御の欄は「×2 弱点」のように**色だけで表さず語も出す**。タイプはタイプ色のエンブレム(`TypeBadgeView`。ink トークン。P6-21)とタイプ名(アクセシビリティのラベルにも)。特性が倍率を変えた欄(`source == ability`)は添え書き
  (「特性で無効」「特性で吸収」「特性で倍率が変わる」。`effect` から選ぶ)を付ける。攻撃範囲は「×2 抜群」・技が無いメンバーは「攻撃技なし」(`bestMultiplier` が null)。
- 画像は必須にしない(タイプ色エンブレムで成立)。常時動くアニメーションは入れない。色は design.md のトークンだけ・`lineLimit`・`minimumScaleFactor` なし・AX5 で折り返す。**タイプ相性表をコードに持たない**
  (ドメイン規約・ADR-0013。正本は balance が持つ `typechart.json` の複製と pokedex の export。iOS は応答のタイプを写すだけ)。
- エラーは `code` から日本語にする(サーバーの英語 message は出さない)。`unknown_pokemon` などは構築の入力なので「構築を見直してください」(§9)。エラーでも構築の一覧・選択は消さず、「もう一度解析する」で再試行できる。

### 7. 入力は構築から選ぶだけ(手入力なし)

- Web はメンバー・特性・技を画面で組む(ADR-0303)。iOS は端末内の構築(`LocalTeamStore`)があるので、**構築を選べば 1 回の操作で解析**できる(iOS の強み。判定・計算・逆算の「構築から呼び出す」と同じ発想の最小形)。
  ポケモン・特性・技を画面で組む入力は作らない(必要になったら別タスク。仮想敵・技範囲を足すときに入力の形を決める)。
- 構築を読むだけでストアには書かない。構築のメンバーに `abilityId`・`moveIds` が入っていれば、そのまま送る(特性で防御が変わる・技で攻撃範囲が決まる)。構築が無い間は案内(「まだ構築がありません。構築ビルダーで作ると解析できます」)。

### 8. モック

`MockBalanceService`(架空データ。環境変数 `POKECALC_MOCK_BALANCE` でシナリオを切り替える。`error` / `coverage-error`、なし・未知は正常)。**メンバーごとに違う値を返す**(行の取り違えを画面・テストが検出できるように)。
UI テストと `MockBalanceServiceTests` が頼る固定の事実(i = メンバーの位置・t = タイプの位置。normal = 0・fire = 1・water = 2・electric = 3 … `PokeType.allCases` 順):

- 防御: メンバー i のタイプは `[allCases[(2 × i) % 18]]`。欄 (i, t) は `(i + t) % 6` で 0 → ×4 弱点 / 1 → ×2 弱点 / 2 → ×1 等倍 / 3 → ×1/2 耐性 / 4 → ×1/4 耐性 / 5 → ×0 無効(出どころ type・効果 none)。
  特性を送ったメンバーだけ t = 0 が ×3/4 耐性(出どころ ability・効果 multiplier)。集計は応答のメンバーから数えた値(不変条件: 弱点 + 耐性 + 無効 + 等倍 = メンバー数)。
- 攻撃範囲: 技が 0 件のメンバーは攻撃タイプなし・全欄 nil(攻撃技なし)。技を持つメンバー i の攻撃タイプは、技 k の `allCases[(i + 5 × k) % 18]`(重複なし・正準順)。倍率は `[×0, ×1/2, ×1, ×2][(i + t) % 4]`、有効 = 等倍以上・抜群 = ×2。
  チームの行は、技を持つメンバーの最大倍率(0 < 1/2 < 1 < 2)と有効・抜群の人数(誰も技を持たなければ nil・0・0)。
- `error` は analyze も coverage も `master_unavailable`、`coverage-error` は coverage だけ `internal_error`(analyze は成功)。メンバーが 0 件・7 件以上は `invalid_request`。
- モックの式は本物を写したものではない(タイプ相性の正しさは保証しない。相性表を iOS に持ち込まない)。

### 9. 文言

`BalanceLabels`(Core)に集約し、Web の `balanceScreenText`・`balanceLabelText`・`balanceErrorText` と同じ語にそろえる。違いは次の 4 点だけ:

1. 入力は構築から選ぶので、エラー文の「選び直してください」は「構築を見直してください」(`unknown_pokemon`・`unknown_move`・`unknown_ability`・`invalid_request`・`request_too_large`)。
2. 「ページを開き直してください」は「アプリを開き直してください」(`missing_request_context`)。
3. 特性が倍率を変えた旨の添え書き(Web にはまだ無い。iOS は色だけにしない)。
4. 技が 1 つも無い構築・メンバーが 0 体の構築・構築を読み込めないときの案内(Web には無い iOS 固有の入口)。

契約外の 404/405 が本文つきで届いたとき(`not_found`)は汎用の文言(「タイプバランスを計算できませんでした。…」)。通信できない・応答が読めない・契約外のステータスは「タイプバランスの API に接続できません」(Web の `balance_unavailable` と同じ)。

### 10. 識別子(XCUITest が使う名前)

`BalanceScreenUITests` の冒頭に一覧がある(ルート `openBalanceScreen`、画面 `balanceScreen`、構築 `balanceTeamList`・`balanceTeam-<n>`・`balanceTeamEmptyMessage`・`balanceTeamLoadError`、状態 `balanceSelectPrompt`・`balanceLoading`・
`balanceNoMembers`・`balanceRetry`、防御 `balanceDefenseRegion`・`balanceDefenseMember-<i>`・`balanceDefenseCell-<i>-<type>`・`balanceAbilityNote-<i>-<type>`・`balanceSummaryRegion`・`balanceSummary-<type>`・`balanceDefenseError`、
攻撃範囲 `balanceCoverageRegion`・`balanceCoverageMember-<i>`・`balanceCoverageCell-<i>-<type>`・`balanceCoverageTeam-<type>`・`balanceCoverageNoMoves`・`balanceCoverageError`)。

## 却下した案

- **Web と同じ手入力(メンバー・特性・技を画面で組む)**: iOS は構築があり、構築を選ぶだけのほうが手数が少ない(§7)。入力画面は仮想敵・技範囲を足すときに必要なら作る。
- **5 本すべてを v1 で出す**: threats(仮想敵入力)・recommendations(同時実行上限・大きな応答)・move-range(構築と別の入力)は入力と見せ方が別の設計になる。2 本で「構築を選ぶと弱点・耐性・無効と攻撃範囲が分かる」が完結する(§3)。
- **iOS 側で弱点の偏り・穴・おすすめを計算する**: 相性表を iOS に持ち込むことになる(ドメイン規約・ADR-0013 違反)。穴・おすすめは recommendations(サーバー)の責務で、後続で結果をそのまま表示する。
- **解析を「解析する」ボタンにする**: 入力が構築の選択だけで、打鍵ごとの送信が無い。選択 = 実行のほうが操作が 1 つ少ない。再試行の入口は別に持つ。
- **analyze と coverage を 1 つの状態にまとめる**: 片方の失敗が全体を消す。Web の ADR-0303 §9 に反する(§4)。
- **balance を `PokeCalcService` に混ぜる**: balance の失敗・遅延が計算・構築に波及する(絶対ルール 5)。

## 結果

- root の API・Web・balance の契約は変えない。iOS が `/api/balance/v1/team-balance/{analyze,coverage}` を使い始める(契約を変える側への連絡は DECISIONS.md 2026-10-03「iOS にタイプバランス画面を作る」)。
- 後続で threats・recommendations・move-range を足すときは、`BalanceService` に操作を足す・状態を足す・識別子を足す(生成物・設定は不変)。
- 追加するテスト数・失敗数と実装者への注意は ADR-0501「P6-26 の受け入れ条件」。
