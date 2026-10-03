# ADR-0327: お気に入り(手動ピン留め)の Web 画面(P5-3c)

- 状態: 採用
- 日付: 2026-10-04
- 関連: ADR-0227(お気に入り API)、ADR-0209(端末 ID 境界・保持)、ADR-0317(record チップ・record が落ちても計算は影響されない)、
  ADR-0309(構築画面の2段階削除・hasWrittenRef)、ADR-0308(訪問済みタブの保持)、ADR-0323(画面の登録)、
  requirements.md §2「お気に入り(手動ピン留め)」(あれば便利)

## 背景

API(一覧・作成・削除)は完了済み。保存するのは計算 API の `Individual`(種族・性格・SP・持ち物・特性・ランク等)に
任意の `label`。Web の画面を決める。

## 決定

1. **画面は新タブ「お気に入り」**(`web/src/favorites/favorites.screen.tsx`、segment `favorites`、order 650、`usesMaster: false`)。
   一覧(更新の新しい順のままサーバーの順)・件数「n/100件」・空の案内・削除(2段階。ADR-0309 §5 と同じ。404 は「もう無い」=成功扱い)。
   種族名の解決にマスタを使わない(マスタ読み込み失敗でも開ける・オンライン限定の API 画面なのでマスタ依存を増やさない)。
   行の見出しは `label`、null なら `speciesKey`。名前解決は後続(要望が出てから)。
2. **作成は計算画面だけ**: 「攻撃側をお気に入りに追加」ボタン。`label` は攻撃側の種族の日本語名(30 コードポイントを超えれば切る)。
   一覧画面に作成フォームは置かない(`Individual` の入力 UI を二重に持たない)。冪等(200)は「すでに入っています」、201 は「追加しました」。
3. **反映(お気に入り → 計算画面に入れる)は今回やらない**。契約は `Individual` を持つので技術的には可能だが、タブ間の状態受け渡し
   (App の状態・マスタ解決・メガの持ち物整合〈domain/mega.ts〉・攻撃側プリセットとの対応)が大きく、要件は「ピン留め」までで
   「呼び出す」を求めていない。後続タスク(P5-3d 候補)に回す。ADR-0227 §2 が「そのまま計算に使える」形にしてあるので、
   後から足せる。
4. **オンライン限定**: `recordClient` はオンラインのときだけ App が渡す(ADR-0317)。オフラインでは API を呼ばず、タブは
   `role=status` の案内を出す。計算画面のボタンも出さない。record が落ちても計算は影響されない(絶対ルール5):
   追加の失敗・pending・reject は追加ボタン周りの alert だけにし、計算結果に触れない。
5. **一覧の鮮度**: 画面は ADR-0308 で保持されるため、計算画面で追加した後にお気に入りタブを開いたとき古い一覧が残らないこと。
   方式: 追加成功で App の `favoritesReloadToken` を進める(`ScreenEnvironment.favoritesReloadToken` と `onFavoriteAdded`。構築の `reloadToken`
   は共有しない〈構築一覧を無駄に取り直さない〉)。端末データの削除後にも同じトークンを進める。画面はトークンの変化で読み込み中に戻して取り直す。
6. **RecordClient の拡張**: `listFavorites(signal?)` / `createFavorite(input)`(`{favorite, created}`。201=true・200=false)/
   `deleteFavorite(id)`(204 は本文なしで成功)。例外を投げず `RecordResult`。成功本文の形が違えば `record_unavailable`。
   定数 `MAX_FAVORITES_PER_DEVICE = 100`、`RECORD_PATHS.favorites`。
7. **エラー表示**: サーバーの `message` を日本語の見出しに続けて出す(`role=alert`)。上限超過は 400 `invalid_input` の message をそのまま。
   文言は `web/src/i18n/favorites.ts`(`favoritesScreenText`・`favoritesCalcText`)にだけ足す。

8. **実装の詳細**: 計算画面のボタンは `favorites/AddFavoriteButton.tsx`(送信中は ref で二重押しを弾き、reject も握って alert だけに出す=絶対ルール5)。
   性格 ID は `resolveNatureId`(api/apiEngine.ts を export)で攻撃側プリセットの (plus, minus) から引く。本文は `favorites/favoriteInput.ts`。
   削除の失敗は確認を閉じ、行の下に alert を出して再度削除できる。タブは登録ファイルのみで増やした(order 650 は 構築 600 と 調整 700 の間)。
   マスタの読み込み失敗中も開けるよう、マスタを使わない画面の描画にも `recordClient` を渡す。

## 結果・トレードオフ

- 利点: 最小の UI で要件(ピン留め)を満たし、API の契約だけに依存する。タブは独立しており、record 障害は他画面に波及しない。
- 欠点: ピンを計算に呼び出せない(当面は「保存して見る・外す」のみ)。種族名が出ない(key 表示)。ラベルの編集もできない(API に更新が無い)。
