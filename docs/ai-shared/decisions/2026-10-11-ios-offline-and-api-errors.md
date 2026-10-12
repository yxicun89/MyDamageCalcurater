## 2026-10-11: iOS のオフライン時の挙動と、サーバーが使えないときの表示(調査と案。ec レーンの依頼 (A)(B)。実装はまだ)
Decision: 案の提示のみ(人間の選択待ち)。推奨は (A) 案 1(接続先が無いときにテスト用データで動かさず、「サーバーにつながらない」と原因を出す。端末に保存した構築だけ使える)を先に入れ、必要なら案 2(読み取りキャッシュ)を続ける。案 3(engine を iOS に載せる)は見送り。(B) は、原因の分類を 1 か所(`PokeCalcCore`)にまとめ、全画面で「接続できない / 時間切れ / サーバーの障害 / まだ使えない機能 / 応答を読み取れない / 入力の誤り」を出し分けて「もう一度」ボタンを付ける。
Reason: いまの iOS の「オフライン」は、接続先が空のときに架空データ(名前が「テスト」で始まる)で動くモックそのもので、利用者には使い道が無い。サーバーの失敗は、画面ごとに別の写像があり、5xx・契約外の応答・形の不一致が「接続できません」や「失敗しました」にまとまって原因が分からない箇所がある(下の監査で 13 件)。
Impact: iOS(`PokeCalcCore` のエラー写像・`AppConfiguration`・`AppEnvironment`・`RootView` と各画面のエラー表示)。API・engine・サーバーの変更なし。XCUITest は全 30 ファイルが `POKECALC_USE_MOCK=1` で起動しているので、モックは開発・テスト用として残せる。ADR-0500 §4・§5(空ならモック)を変えるので、実装するときは新しい ADR を足す。

### 1. いまの動作モード(調査結果)

| 経路 | 決まり方 | 画面に出るもの |
|---|---|---|
| モック | `PokeCalcAPIBaseURL` が空・無し(`ios/PokeCalc/Config/PokeCalc.xcconfig:9` の既定は空)、または起動時の環境変数 `POKECALC_USE_MOCK=1`(`ios/PokeCalcKit/Sources/PokeCalcCore/AppConfiguration.swift:46-59`) | ルートのバッジ「モックデータで動作中」(`ios/PokeCalc/AppEnvironment.swift:30`、`ios/PokeCalc/RootView.swift:126-134`)。計算・逆算・調整の画面にも同じバッジ(`CalcScreenView.swift:138-144`・`ReverseScreenView.swift:70-76`・`AdjustScreenView.swift:77-83`)。選べるポケモン・技・持ち物は「テスト〜」の架空データ(`PokeCalcCore/Resources/*.json`)。ダメージは計算せず、決まった結果を返す(ADR-0500 §4) |
| サーバー | 有効な `https://`(または ATS が通す `http://`)の URL | バッジ「サーバーに接続中(ホスト名)」(`AppEnvironment.swift:38`)。**実際につながるかは確かめない**固定の文言 |
| 設定エラー | URL が不正 | 「設定エラー: …」(`AppEnvironment.swift:49-51`)。「このアプリについて」だけ開ける(`RootView.swift:115-118`) |

- 「オフライン」に当たるのはモックの経路だけ。実利用で接続先を入れずにビルドすると、テスト用の名前と「モックデータで動作中」がそのまま出る(依頼 (A) の「テストデータしか無い」の原因)。
- サーバーに接続できない(機内モード・サーバー停止)ときに別の経路へ切り替える仕組みは無い。各画面が開いたときの読み込みで失敗し、画面ごとのエラーが出る(バッジは「接続中」のまま)。
- タイプバランスはモックを持たず、モック時は `UnavailableBalanceService`(`PokeCalc/Features/BalanceFeature.swift:14-16`)で常に「タイプバランスのサーバーに接続できません」になる(実際は接続先が無いだけ)。
- Web の WASM(この端末で計算)に当たる、iOS で本当にオフラインで計算する経路は無い。engine は Go だけで、Swift 版は作らない方針(ADR-0500 §4「engine の式を Swift に写すと単一の正が崩れる」)。端末に残るのは構築(`LocalTeamStore`。UserDefaults に JSON)だけで、メンバーは番号(`speciesKey`・`moveIds`)しか持たないので、名前の表示にはサーバーのデータが要る。
- 要件: `docs/requirements.md:71` のオフライン要件は「ブラウザ版は WASM でバックエンドなしでも計算可能」で、iOS のオフラインは要件に無い。Web は ADR-0313 で「架空データを利用者の画面に出さず、一度取得したデータを端末に保存してオフラインで使う」に変えている(iOS には未反映)。

### 2. (A) オフライン時の案

| 案 | 内容 | オフラインで使えるもの | コスト | 影響・撤去範囲 |
|---|---|---|---|---|
| **1(推奨・先に入れる)** 原因を出す+端末の保存分だけ | 接続先が空のときモックに落とさず、設定エラーと同じ扱いの案内「サーバーの接続先が設定されていません」にする(モックは `POKECALC_USE_MOCK=1` のときだけ)。サーバーにつながらないときは、各画面に「サーバーに接続できません」+「もう一度」(下の (B))。ルートのバッジは固定の「接続中」をやめ、起動時に 1 回、既存の軽い取得(性格の一覧など。新しい API は足さない)で確かめた結果「サーバーにつながっています / つながりません」を出す | 構築の一覧(名前・メンバー数)と、Showdown 形式の書き出し・読み込み(テキストのみ)。ポケモン名・技名はつながるまで番号のまま出さず「名前を読み込めません」と添える | 小(`AppConfiguration`・`AppEnvironment`・`RootView`・構築一覧の表示。1〜2 タスク) | 実利用の画面から「モックデータで動作中」とテスト用の名前が消える。`AppConfigurationTests` の「空 → モック」を「空 → 未設定」に変える。モック時のバッジは XCUITest が見ている(`AdjustScreenUITests.swift:79`・`CalcScreenUITests.swift:107` ほか)ので、`POKECALC_USE_MOCK=1` のときだけ「テスト用データで動作中」を出して残す。`make ios-run` は `IOS_RUN_MOCK=1` で今まで通りモックを使える。ADR-0500 §5 を新しい ADR で置き換える |
| 2 読み取りキャッシュ(案 1 の上に足す) | Web の ADR-0313 と同じ考え方で、一度取得したポケモン・技・持ち物・性格・特性のデータと、お気に入り・履歴の最後の取得結果を端末(アプリの Caches か Application Support に JSON)に保存し、つながらないときは保存分を読む。黙って切り替えず「保存したデータで表示しています(サーバーにつながりません)」と出す | 案 1 + 構築の編集(保存済みのポケモン・技から選ぶ)、構築・お気に入り・履歴の名前つき閲覧。計算・逆算・調整・素早さ・タイプバランスは使えない(サーバーで計算するため)と明示 | 中(保存口・版・壊れたときの破棄・テスト。3〜4 タスク) | 実データを端末に保存するだけで Git には入らない(ADR-0002 に反しない)。保存先の上限・版の扱いを ADR に書く。計算は相変わらずサーバー必須なので、利用者の「オフラインで計算したい」は満たさない |
| 3 engine を iOS に載せる | Go の engine(`engine/wasmapi` の境界)を gomobile で xcframework にし、Swift から呼ぶ。マスタは案 2 のキャッシュから渡す | 計算・逆算・調整(engine の範囲)まで端末で動く。素早さ・タイプバランスはサービス側のロジックなので別途 | 大(gomobile のビルドを CI・Makefile に追加、バイナリの大きさ、Swift 側の境界とゴールデンの一致テスト、ADR) | ビルドが Go に依存し、iOS レーン単独で回せなくなる。案 2 が前提。今は見送り |

推奨: 案 1 を今入れる(テスト用データ前提の表示をなくすのが依頼の主眼で、コストが小さい)。オフラインでの閲覧が欲しくなったら案 2。案 3 は「iOS でもオフラインで計算したい」という要望が出たら検討する。

### 3. (B) サーバーが使えないときの表示の監査

#### 3.1 通信層の写像(全画面の土台)

| 層 | 接続できない・時間切れ | 応答の形が違う(デコード失敗) | 契約に無いステータス(404 の経路なし・502・504・501 など) | 契約にあるエラー(400・500・503 の JSON) |
|---|---|---|---|---|
| 計算・逆算・調整・構築・お気に入り・履歴(`APIPokeCalcService.swift:249-268`) | `client_transport_error`(時間切れも同じ) | `client_decode_error` | 契約が `default` を持つので、本文が JSON ならその `code`。HTML の 502 などは `ClientError`(デコード失敗ではない)になり `client_transport_error` に入る見込み(要確認。テストが無い) | サーバーの `code` |
| タイプバランス(`APIBalanceService.swift:131-168`) | 同上 | 同上 | **すべて `client_transport_error`**(`undocumentedError`、166-168 行) | サーバーの `code` |
| 素早さ(`APISpeedService.swift:98-143`) | 同上 | 同上 | 本文が `{code,message}` ならその `code`、読めなければ `client_unexpected_status` | サーバーの `code` |
| 判定(`APIJudgeService.swift:66-111`) | 同上 | 同上 | 素早さと同じ | サーバーの `code` |

- 時間切れは URLSession の既定(60 秒)のまま(`timeoutInterval` の設定なし)。待たされたうえで「接続できません」と同じ文言になる。
- 失敗の HTTP ステータスは `PokeCalcError` に残らない(`code` と `message` だけ)。そのため画面側で「サーバーの障害(5xx)」と「経路が無い(404)」を分けられない。

#### 3.2 画面ごとの文言と再試行

| 画面 | 写像 | 接続できない | 形が違う | 5xx(JSON) | 契約外の応答 | 未対応(404・501) | 再試行 |
|---|---|---|---|---|---|---|---|
| 計算・逆算(`CalcScreenError.swift:12-45`、表示 `CalcScreenView.swift:81-82`・`ReverseScreenView.swift:40-41`) | 3 種類 | 「通信に失敗しました。接続を確認して…」 | 「応答の形が想定と違うため…」 | 「エラー(internal_error): <サーバーの英語>」 | 接続できないと同じ(見込み) | 「エラー(not_found): <英語>」 | ボタン無し。最初の読み込み(`CalcScreenView.swift:120-131` の `.task`)の失敗は画面を開き直すまで直らない。入力を変えると計算は送り直す |
| 構築(`TeamScreenError.swift:12-41`、`TeamListView.swift:41-42`・`TeamEditView.swift:58-59`) | 計算と同じ | 同上 | 同上 | 同上 | 同上 | 同上 | ボタン無し(一覧は `onAppear` で読み直す) |
| タイプバランス(`BalanceLabels.swift:160-215`、表示 `BalanceScreenView.swift:29-30`・`BalanceResultViews.swift:34-35,86-87`・`BalanceThreatsView.swift:35-36`・`BalanceMoveRangeView.swift:33-34`・`BalanceRecommendationsView.swift:19-27`) | code → 日本語 | 「タイプバランスのサーバーに接続できません」 | **接続できないと同じ** | `internal_error`「サーバーでエラーが起きました…」、`overloaded`・`master_unavailable` も個別 | **接続できないと同じ**(通信層で transport) | **接続できないと同じ** | 「再計算」はおすすめだけ。他は入力を変えたとき。ポケモン・技の一覧の失敗(`masterError`)は開き直すまで直らない |
| 素早さ(`SpeedLabels.swift:111-128`、`SpeedViewModel.swift:232-234`、表示 `SpeedScreenView.swift:102-104,205-206`・`SpeedTableSection.swift:94-95`) | code → 日本語 | 「素早さのサーバーに接続できません」 | **接続できないと同じ** | **「素早さの計算に失敗しました」(汎用)** | 汎用 | 汎用(`not_found` も汎用) | ボタン無し。表は条件を変えると取り直す。ポケモンの一覧は開き直すまで直らない |
| 調整(`AdjustText.swift:115-136,222-226`、`AdjustScreenView.swift:47`) | code → 日本語 | 「調整のサーバーに接続できません」 | **接続できないと同じ** | **「調整に失敗しました」(汎用)** | 汎用 | `not_found` は「見つかりませんでした。入力を確かめてください」(入力の誤りと区別できない)。目標方式だけは `not_found` を「機能が無い」として引っ込める(`AdjustGoalsDomainTypes.swift:122-131`) | 送信ボタンを押し直す。最初の読み込みは開き直すまで直らない |
| 判定(既定で非表示。`JudgeLabels.swift:53,158-176`、`JudgeScreenView.swift:40`・`JudgeResultSection.swift:25`) | code → 日本語 | **「判定の API に接続できません」(禁止語)** | 接続できないと同じ | 「判定に失敗しました」(汎用) | 接続できないと同じ | 汎用 | 送信ボタン。一覧の失敗は「性格・持ち物などの一覧を読み込めません」1 種類で、ボタン無し |
| お気に入り・履歴(`Favorites.swift:113-197`、`FavoritesScreenView.swift:77,159,231`) | 6 種類 | 「通信に失敗しました。接続を確認して…」 | 汎用「うまくいきませんでした…」 | 汎用(`store_unavailable`・`upstream_unavailable` だけ「計算はそのまま使えます」と個別) | 汎用 | 汎用 | 「再読み込み」あり(いちばん整っている) |

#### 3.3 原因が分からない・隠れる箇所(13 件)

1. タイプバランスの契約外のステータス(経路が無い 404・502・504)が「接続できません」になる(`APIBalanceService.swift:166-168`)。サーバー側の障害が端末の通信の問題に見える。**今回の balance の失敗はここに当たる可能性が高い。**
2. 応答の形が違う(サーバーとアプリの版のずれ)を「接続できません」にしている: `BalanceLabels.swift:209`・`SpeedLabels.swift:125`・`AdjustText.swift:224`・`JudgeLabels.swift:174`。
3. サーバーの障害(5xx の `internal_error` など)が汎用の「失敗しました」に埋もれる: 素早さ `SpeedLabels.swift:126`・調整 `AdjustText.swift:225`・判定 `JudgeLabels.swift:175`・お気に入り `Favorites.swift:158-159`。
4. 素早さの `client_unexpected_status`(HTML の 502 など)が汎用の文言になる(`SpeedLabels.swift:126`)。
5. 計算・逆算・構築が `code` とサーバーの英語の `message` をそのまま出す(`CalcScreenError.swift:37-38`・`TeamScreenError.swift:38-39`)。利用者は原因を読めず、`client_nature_unavailable` のような内部の語も出る。
6. タイプバランスのポケモン・技の一覧(pokedex)の失敗を「タイプバランスのサーバーに接続できません」と出す(`BalanceViewModel.swift:159,215,311` が `BalanceScreenError` を使う)。原因のサーバーを取り違える。
7. 未対応(経路が無い 404・501)と入力の誤りが区別できない: 調整「見つかりませんでした。入力を確かめてください」(`AdjustText.swift:131`)、計算「エラー(not_found)」。
8. 最初の読み込みの失敗に「もう一度」が無い: 計算・逆算・調整・構築の編集・タイプバランスの一覧・素早さのポケモン一覧・判定の一覧。画面を開き直すしかない。
9. 判定の文言が禁止語を使う:「判定の API に接続できません」「このポケモンはマスタにありません」ほか 2 件(`JudgeLabels.swift:159,167-169`)。`GlossaryTests.swift:10` が判定を対象外にしているので検査で見つからない。
10. ルートのバッジ「サーバーに接続中(ホスト名)」は確かめていない固定の文言で、つながらないときも出る(`AppEnvironment.swift:38`・`RootView.swift:126-134`)。
11. 時間切れと接続できないを区別しない(どちらも `client_transport_error`)。待ち時間は既定の 60 秒。
12. モックのときタイプバランスが「サーバーに接続できません」と出る(`BalanceFeature.swift:14-16`)。実際は接続先が無いだけ。案 1 で「接続先が設定されていません」にそろう。
13. HTML 本文の 5xx(ingress が返す 502・503 など)が契約にある経路でどのコードになるか、テストが無い(判定だけ `APIJudgeServiceTests.swift:294` にある)。計算側・タイプバランス側で「接続できません」になっている見込み。

#### 3.4 直す案

- **原因の分類を 1 か所に**: `PokeCalcCore` に `ServiceFailureKind`(仮)を作り、全画面の写像(`CalcScreenError`・`TeamScreenError`・`BalanceScreenError`・`SpeedFailure`・`AdjustText.errorMessage`・`JudgeFailure`・`RecordScreenError`)の前段で使う。
  - `PokeCalcError` に HTTP ステータス(あれば)を足し、`client_transport_error` を「接続できない」(`URLError` の notConnectedToInternet・cannotFindHost・cannotConnectToHost・networkConnectionLost)と「時間切れ」(`timedOut`)に分ける。
  - タイプバランスの `undocumentedError` を素早さ・判定と同じ形(本文の `code`、読めなければ `client_unexpected_status`+ステータス)にする。
  - 分類: 接続できない / 時間切れ / サーバーの障害(5xx・`internal_error`・`overloaded`・`master_unavailable`・`upstream_unavailable`・`store_unavailable`)/ まだ使えない機能(経路の 404・501)/ 応答を読み取れない(デコード失敗)/ 入力の誤り(400 系の既知の `code`。いまの画面別の文言を使う)/ 端末の情報(`missing_header` など)。
- **文言案**(禁止語を使わない。`{機能}` は「計算」「タイプバランス」「素早さ」「調整」「判定」「お気に入り」など):
  - 接続できない: 「サーバーに接続できません。インターネットの接続を確かめて、もう一度お試しください」
  - 時間切れ: 「サーバーから時間内に返事がありませんでした。しばらくしてからもう一度お試しください」
  - サーバーの障害: 「{機能}のサーバーで問題が起きています。しばらくしてからもう一度お試しください(エラー番号 502)」(番号はステータスがあるときだけ。原因を伝える手がかりとして出す)
  - まだ使えない機能: 「{機能}は、いまつないでいるサーバーではまだ使えません」
  - 応答を読み取れない: 「サーバーの返事を読み取れませんでした。アプリかサーバーの更新が必要かもしれません」
  - 混み合い(`overloaded`): 「サーバーが混み合っています。しばらくしてからもう一度お試しください」(タイプバランスの既存文と同じ)
  - 計算・逆算・構築の「エラー(code): 英語」は廃止し、上の分類の文言と画面別の入力の誤りの文言にする。`code` は画面に出さない(開発者向けのログに残す)。
- **再試行**: エラーの帯に「もう一度」ボタンを付け、最初の読み込み(`load()`)を呼び直す。入力から送る計算は今まで通り入力の変更でも送り直す。
- **検査**: `GlossaryTests` に判定の文言を入れる(判定は既定で非表示でも、文言は禁止語を避ける)。各分類の写像はテーブル駆動のテストにし、HTML 本文の 502・503・504、経路なしの 404、時間切れ(`URLError(.timedOut)`)を偽の transport で確かめる。
- 時間切れの秒数は設定値(定数 1 か所)にして、既定の 60 秒より短く(例 15 秒)する案。値は実装時の ADR で決める。

### 4. 人間に決めてほしいこと
- (A) 案 1 で進めてよいか(接続先が空のビルドはテスト用データで動かさず「接続先が設定されていません」にする)。案 2 を続けて入れるか。
- (B) 文言案と、エラー番号(502 など)を画面に出すか。
