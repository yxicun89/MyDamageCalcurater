# ADR-0534: iOS は接続先が無いときにモックへ落とさず、サーバーの失敗を原因別に伝える

- 状態: 採用(2026-10-12。ec レーンの依頼 (A)(B) の「既定案」。人間の選択待ちの間は既定案で進める)
- 関連: ADR-0500 §4・§5(空ならモック。本 ADR の 1 で置き換え)、ADR-0501 追記章「接続先なし・サーバーの失敗の表示」、
  `docs/ai-shared/decisions/2026-10-11-ios-offline-and-api-errors.md`(調査と案。13 件の監査)、plan `I-ios-19`
- 担当: iOS(`PokeCalcCore`・`AppConfiguration`・`AppEnvironment`・`RootView`・各画面のエラー表示)

## 決定

1. **接続先が空・無しのとき、モックに落とさない**。`Backend` に `.unconfigured` を足し、`AppEnvironment` は設定エラーと同じ 1 画面
   (「サーバーの接続先が設定されていません」+「このアプリについて」)を出す。モックは `POKECALC_USE_MOCK=1` のときだけ
   (XCUITest・`make ios-run` の `IOS_RUN_MOCK=1`)。モック時のバッジ「モックデータで動作中」は今の文言のまま残す(UITest が見ている)。
   接続先なしのタイプバランス(`UnavailableBalanceService`)の文言は「サーバーの接続先が設定されていません」にそろえる。
2. **失敗の分類を `PokeCalcCore` の 1 か所に**: `ServiceFailureKind`(offline / timeout / serverFault / unsupported / unreadable / overloaded /
   other(code:))と `ServiceFailureText`(固定文)。全画面の写像(`CalcScreenError`・`TeamScreenError`・`BalanceScreenError`・
   `SpeedFailure`・`AdjustText.errorMessage`・`RecordScreenError`)はこの分類を前段に使い、画面別の入力の誤りの文言だけを残す。
3. **`PokeCalcError` に `statusCode: Int?` を足す**(既定 nil)。契約外の応答(HTML の 502・504、経路の無い 404 など)は
   `client_unexpected_status` + 番号。タイプバランスの契約外ステータスも `client_transport_error` ではなくこの形にする。
   時間切れ(`URLError.timedOut`)は新コード `client_timeout`。接続できない(`notConnectedToInternet`・`cannotFindHost`・
   `cannotConnectToHost`・`networkConnectionLost`)は従来の `client_transport_error`。
4. **番号・コード・サーバーの英語は画面に出さない**(決定案では「エラー番号を出すか」が人間の判断待ちだったが、既定は出さない。
   番号は `PokeCalcError.statusCode` に残り、分類にだけ使う)。計算・逆算・構築の「エラー(code): 英語」は廃止。
5. **分類**: 接続できない / 時間切れ / サーバーの障害(5xx・`internal_error`・`master_unavailable`・`upstream_unavailable`・
   `store_unavailable` ・契約外の 5xx)/ まだ使えない機能(契約外の 404・501。入力の誤りの `not_found` とは別)/
   応答を読み取れない(デコード失敗・番号の無い契約外・PokeCalcError 以外)/ 混み合い(`overloaded`)/ その他(画面別の文言)。
6. **「もう一度」**: 最初の読み込みの失敗が画面を開き直すまで直らない画面(計算・逆算・調整・構築の編集・タイプバランスの一覧・
   素早さのポケモン一覧)のエラーの帯に、`ServiceFailureText.retryButtonLabel`「もう一度」のボタン(36pt 以上)を付ける。
   ViewModel は `canRetryLoad` と `retryLoad()`。既に再読み込みのボタンがある画面(お気に入り・履歴)、送信ボタンで押し直せる
   画面には足さない。判定は撤去済みなので対象外。
7. 画面の文言は docs/glossary.md の禁止語(API・マスタ・リクエスト・観測・プリセット など)を使わない。`lineLimit` を付けない。常時アニメなし。
8. 時間切れの秒数は定数 1 か所(`ClientFailureMapping.requestTimeoutSeconds` = **30 秒**。URLSession の既定 60 秒より短くし、3 つの通信層が共通の transport を使う)。
9. 契約の列挙に無い code(例: 計算・マスタの契約は `internal`、素早さ・タイプバランスは `internal_error`)や HTML 本文でデコードできなかった応答は `client_decode_error` + `statusCode` とし、分類は番号で行う(5xx → 障害、404・501 → まだ使えない)。サーバー側の 5xx で画面別の文言が無い code も障害に含める。
10. 原因がより具体的な既存の文言(`master_unavailable`・`upstream_unavailable` の調整、`master_unavailable` の素早さ・タイプバランス)は、障害の汎用文より優先して残す。

## 影響

iOS だけ。API・engine・サーバー・Web は変えない。既存テストの文言変更は ADR-0501 の追記章の表。
案 2(読み取りキャッシュ)・案 3(engine を iOS に載せる)は見送り。
