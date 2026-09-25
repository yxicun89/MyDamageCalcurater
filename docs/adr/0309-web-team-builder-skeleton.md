# ADR-0309: Web の構築ビルダー(PR-A1)— 骨格だけを先に作り、タブ・クライアント・削除確認の形を決める

- 状態: 採用(Web レーン、2026-09-26。P5-5a の受け入れ条件とテストの正)
- 日付: 2026-09-26
- 関連: docs/requirements.md §2「必須: 構築ビルダー」、docs/plan.md「P5-5」、
  ADR-0213(team-svc の CRUD 契約。`Team`/`TeamInput`/`TeamMember`)、ADR-0209 §5.3・§6(端末 ID の分離。
  他端末のリソース ID は 404 `not_found`)、ADR-0300 §1(ルーターも状態管理ライブラリも入れない)、
  ADR-0303 §1・§5・§6(balance のクライアントの設計)、ADR-0604 §2・§3(素早さのクライアント・画面)、
  ADR-0705 §1(判定のタブとレーン境界)、ADR-0304 追記6 / issue #308(マスタが読めないときのタブの扱い)、
  ADR-0308 / issue #218(訪れたタブだけ mount して入力を保つ)

## 背景

requirements.md §2 の必須要件「構築ビルダー(6体のパーティ、個体、Showdown 形式のインポート/エクスポート)」の
うち、サーバー側(team-svc の CRUD・端末 ID の分離・失効)は P5-4 で入り、契約は `api/openapi.yaml` にある。
Web 側はまだ何も無い。

一度に作ると PR が大きくなりすぎる(メンバー編集だけで種族検索・技/持ち物/特性の選択・SP のグリッドという
新規 UI が並ぶ)。そこで段階に割る。

- **PR-A1(このADR)**: 構築の一覧・新規作成(名前だけ・メンバーは空)・名前変更・削除
- **PR-A2**: メンバー編集(種族・技・持ち物・特性・性格・SP・テラスタイプ)
- Showdown 形式の入出力は判定レーンが `web/src/team/showdownFormat.ts` として別に担当する(分担合意済み)

PR-A1 で決めておかないと後戻りになる判断が3つある。

## 決定

### 1. タブは末尾に1件足し、`usesMaster: true` にする

`app/routes.ts` の `SCREEN_ROUTES` の末尾に `{ id: "team", segment: "team", label: appText.teamTabLabel,
usesMaster: true }` を足す(パスは `/team`、タブ名は「構築」)。レーンの持ち物は `web/src/team/` の中だけにし、
共有ファイルへの追記は **ルート表1件・`app/screens.tsx` 1件・`i18n/ja.ts` の文言・`App.tsx` の client の
受け渡し**に限る(ADR-0705 §1 が判定レーンで置いた作法をそのまま踏む)。

`usesMaster` は **`true`**。PR-A1 の一覧・作成・名前変更・削除自体はマスタを使わないので `false` でも動くが、
PR-A2 のメンバー編集では種族・技・持ち物・特性の名前解決にマスタが要る。いま `false` にすると、

- `MasterlessScreenProps` / `MASTERLESS_SCREEN_COMPONENTS` への登録が要り、PR-A2 でそれを外すことになる
- 何より、**マスタが読めない間に使えていたタブが、PR-A2 で使えなくなる**(issue #308 の観点からは退行に見える)

`true` にしておけば答えが1つで済む。代償は「マスタが読めない間、構築の一覧も見られない」ことだが、この状態では
計算・逆算・タイプバランス・判定も同じく使えず、画面には「再試行」と(オンラインなら)「オフラインに切り替える」が
出ている(issue #308)。マスタ不要のままにするのは素早さの画面だけに留める。

### 2. クライアントはルートの生成型をそのまま使い、例外を投げない

`web/src/team/teamClient.ts` は `speed/speedClient.ts` と同じ形にする(ADR-0303 §1・§5・§6 の踏襲)。

- `TeamResult<T> = {ok: true, value: T} | {ok: false, error: TeamError}` の判別 union を返し、**例外を投げない**
- 通信できない・応答が JSON でない・エラー本文が `{code, message}` の形でないときは、Web 側のコード
  `team_unavailable` にする(`SPEED_UNAVAILABLE_CODE`・`JUDGE_UNAVAILABLE_CODE` と同じ作法)。
  サーバーの `Error` 封筒はコードも文言もそのまま運ぶ(`not_found` を含む)
- 型は **ルートの `web/src/api/openapi.gen.ts`**(`components["schemas"]`)を使う。balance/speed/judge は別サービス・
  別 `openapi.yaml` なのでレーンごとの `*.gen.ts` を持つが、team・record はルートの契約に同居し、gateway 経由で
  計算 API と同じ基点 URL(`api/config.ts` の `apiBaseUrl()`)を使うため、レーン専用の生成物を増やさない
- 削除は 204(本文なし)なので `remove(teamId): Promise<TeamResult<void>>`。`delete` は予約語なので `remove`。
  **204 の本文を読もうとして `team_unavailable` に落ちない**ことをテストで固定する
- 端末 ID・セッション ID は全リクエストのヘッダー(`X-Device-Id`・`X-Session-Id`)。パスには入れない
  (ADR-0209 §6: 端末 ID は分割キーであって認証ではない)

### 3. 削除の確認は `window.confirm` を使わず、その行の2段階ボタンにする

`[削除]` を押すとその行に「「<名前>」を削除します。取り消せません」と `[削除を確定] [削除をやめる]` が出る。
確定するまで API を呼ばない。`window.confirm` を使わないのは、

- ブラウザのダイアログは文言・見た目・フォーカスの扱いを制御できず、デザイントークン(docs/design.md)から外れる
- jsdom では既定で未実装に近く、テストが「モックしたグローバル」への依存になる(この repo の既存テストは
  グローバルの差し替えを fetch 以外ではしていない)
- iOS レーンの確認シートと文言を揃えやすい

モーダルダイアログ(`role="dialog"` + フォーカストラップ)も候補だったが、PR-A1 の画面はメンバー一覧より単純で、
フォーカス管理を足すだけの理由が無い。ロービングフォーカス等の複雑なパターンも持たせない。

### 4. 一覧の状態と、書き込み後の扱い

- `list()` はマウント時に1回だけ呼ぶ(`cancelled` フラグで古い応答を捨てる。BalanceScreen/SpeedScreen と同じ)。
  訪れるまで mount しない(ADR-0308 決定1)ので、タブを開くまで構築 API は呼ばない
- 状態は **読み込み中 / 空(案内)/ 失敗(`role="alert"`)/ 一覧** の4つを取り違えずに出す。
  **一覧が読めなくても新規作成のフォームは使える**(素早さの画面が左の表を待たずに右の入力を使えるのと同じ)
- `create()`/`update()`/`remove()` が成功したら、**応答の `Team` で手元の一覧を書き換える**(`list()` を呼び直さない)。
  作ったものは先頭に足し、名前を変えても行は動かさない(操作中に並びが変わると見失う)。並び直しは次に開いたとき
- 送信前の検査は契約と同じ範囲だけ(`name` は前後の空白を除いて1〜50文字)。範囲外は API を呼ばずに理由を出す
- `update` は**全置換**なので、名前だけを変えるときも `members` を一緒に送る(PR-A1 では常に空配列)

## 影響

- API は変えない(`api/openapi.yaml` は P5-4 のまま。`make gen` 不要)
- 共有ファイルの差分: `app/routes.ts` 1件・`app/screens.tsx` 1件・`i18n/ja.ts` の文言・`App.tsx` の client 1本
- タブが5→6件になるので、タブの並びを数えている既存テスト(`app/routes.test.ts`・`App.test.tsx` の End キー・
  `App.masterSources.test.tsx` のタブ一覧・`e2e/a11y.spec.ts`)は期待値を1件足す形で更新する(弱めない)
- PR-A2 で `TeamScreenProps` に `master`(と `masterSearch`)が加わる。ルート表の `usesMaster` は動かさない

## 却下した案

- **`usesMaster: false` で始め、PR-A2 で true にする**: マスタ失敗中に使えていたタブが使えなくなる退行に見える。
  `MASTERLESS_SCREEN_COMPONENTS` の出し入れも増える
- **team 用に `web/src/team/team.gen.ts` を生成する**: team はルートの `api/openapi.yaml` に同居しており、
  同じ契約から2つの生成物ができて食い違う余地を作るだけ
- **`window.confirm` での削除確認**: 上記 §3
- **PR-A1 でメンバー編集まで作る**: 1 PR が大きくなりすぎ、レビュー(critic)の粒度が落ちる
