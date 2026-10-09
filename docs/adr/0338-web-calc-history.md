# ADR-0338: Web の計算履歴の一覧(お気に入りの画面の第2の節)— P5-5 続き

- 状態: 採用
- 関連: ADR-0230(計算履歴 API。「Web・iOS レーンへの依頼」)、ADR-0327(お気に入りの画面)、ADR-0333(お気に入りから計算を開く)、
  ADR-0317(recordClient)、ADR-0318(端末データの削除。`reloadToken`)、docs/glossary.md(やさしい言い換え)

## 決定

1. **置き場所**: お気に入りの画面(`favorites/FavoritesScreen.tsx`)の中に第2の節「計算履歴」(`favorites/CalcHistorySection.tsx`、
   region の名前は「計算履歴」、一覧の名前は「計算履歴の一覧」)を置く。iOS の「お気に入り・履歴」と対にする。新しいタブは作らない。
2. **recordClient**: `listCalcHistory(cursor?: string, signal?: AbortSignal): Promise<RecordResult<CalcHistoryPage>>`。
   `GET api/record/calc-history?limit=20`(`CALC_HISTORY_PAGE_LIMIT = 20` 固定・`RECORD_PATHS.calcHistory`)、cursor は解釈せず
   クエリに符号化して付けるだけ。エラーは他の呼び出しと同じ(Error 封筒はそのまま、通信・形の不正は `record_unavailable`)。
   成功本文は `items` が配列・`nextCursor` が文字列か null・各行が `calc` を持つ、を確かめる。
3. **計算に使う**: `restoreFavoriteCalc` は `CalcRequest` を直接受ける(ADR-0333)ので復元の本体は変えない。節は行を
   `historyEntryAsFavorite(entry, label)`(`favorites/calcHistoryFavorite.ts`。純粋)で `Favorite` 形に包み、画面の既存の
   `onUse(favorite)` → `App.openFavoriteInCalc` → `restoreRequest` の経路にそのまま流す。サーバーへ保存しない。
   label は「{攻撃側の key} → {防御側の key}」。名前の解決はしない(お気に入りの画面は `usesMaster: false`。key のまま表示する)。
4. **取得**: マウント時と `recordClient`/`reloadToken` の変化で先頭ページを1回。古い取得は abort し、古い応答は捨てる。ポーリングしない。
   「もっと見る」は `nextCursor != null` の間だけ。押すとその値を渡して末尾に足す。応答待ちの間は二重に呼ばない。
   続きが 400 なら先頭から1回だけ読み直す(読み直しも失敗したらエラー表示で止める)。
5. **失敗の閉じ込め**: エラーは節の中の `role="alert"`(見出し + サーバーの message)だけ。お気に入りの一覧・計算画面を塞がない
   (絶対ルール5)。続きの失敗は読み込み済みの行と「もっと見る」を残す。オフライン(`recordClient` なし)は節ごと出さない
   (お気に入り側の既存の案内だけ)。

## 受け入れ条件

- **AC-1** `listCalcHistory`: `GET {base}api/record/calc-history?limit=20`(cursor があれば同じ limit に足す。復号すると元の文字列)、
  ヘッダに端末 ID・セッション ID、本文なし、signal を渡す。成功は `{items, nextCursor}` をそのまま運び、Error 封筒はそのまま、
  通信失敗・JSON でない・形の不正は `record_unavailable`。例外を投げない(`recordClient.calcHistory.test.ts`)
- **AC-2** 一覧: 4状態(読み込み中・空・失敗・一覧)。行に攻撃側/防御側 key・技 ID・「min〜max%」・読みやすい日時(生の ISO 文字列を出さない)。
  サーバーの順。同じ内容の行が並んでも両方出る(キーは配列の位置)(`CalcHistorySection.test.tsx`)
- **AC-3** 続き: 「もっと見る」は `nextCursor` が null でない間だけ。cursor を加工せず渡し、末尾に足す。連打で二重に呼ばない。
  続きの失敗は行を残して alert。400 は cursor なしで1回だけ読み直す
- **AC-4** 失敗は節の中の alert のみで、お気に入りの一覧を塞がない(逆も同じ)。ポーリングなし。オフラインは節なし・呼び出しなし
  (`FavoritesScreen.history.test.tsx`)
- **AC-5** `reloadToken` で先頭から取り直し(古い取得は abort、古い応答・古い「もっと見る」の応答は無視)。
  アンマウント後に setState しない。端末データの削除後の `reloadToken` 更新で履歴も空になる
- **AC-6** `onUse` を渡したときだけ各行に「この計算を使う」。押すと行の `calc` を持つ `Favorite` 形を1回渡す。
  `historyEntryAsFavorite` は calc を同じ値で、individual は攻撃側、日時は occurredAt、入力を書き換えない(`calcHistoryFavorite.test.ts`)
- **AC-7** 文言は `i18n/favorites.ts` の `calcHistoryText`(`listLabel`・`loadingNotice`・`emptyNotice`・`errorHeading`・`useLabel`=「この計算を使う」、
  節の名前「計算履歴」、「もっと見る」)。docs/glossary.md の禁止語を含まない(`glossary.test.ts` が走査)。CSS はトークンと `.ui-*` のみ(色リテラルなし)

## 実装者への注意

- `RecordClient` に `listCalcHistory` が増えるため、既存テストの fake(`FavoritesScreen*.test.tsx` など `RecordClient` を作る所)は
  型エラーになり、`FavoritesScreen` が節を描くと実行時にも `listCalcHistory is not a function` になる。**fake に `listCalcHistory`
  を足して直す**(既存の期待は変えない。テストを弱めない)。App 系のテストの fake も同様に確認する
- 節は `usesMaster: false` の画面の中なので、名前の解決は後続(要望が出たら別 ADR)

## 補足(critic 指摘)

- `historyEntryAsFavorite` の `id`(`history:<occurredAt>`)は**一意ではない**(同時刻の2行が重複する)。履歴の行は React の key に配列の位置を使い、
  `onUse` は Favorite を丸ごと渡すだけで id で引かない。id を使う処理を足すときは、この前提を見直す。
