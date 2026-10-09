# ADR-0519: iOS の計算履歴一覧(お気に入り・履歴画面の「契約待ち」注記の解除)

- 状態: 採用(2026-10-09)
- 日付: 2026-10-09
- 関連: ADR-0230(計算履歴 API。「Web・iOS レーンへの依頼」)、ADR-0511(お気に入り・履歴画面)、ADR-0513(お気に入りの読み込み)、
  ADR-0501「お気に入り・計算履歴」、ADR-0507(画面レジストリ)、ADR-0228(`calc` = `CalcRequest`)、CLAUDE.md 絶対ルール5

## 背景

ADR-0230 の `GET /api/record/calc-history`(`listCalcHistory`)が main に入った。iOS の「お気に入り・履歴」画面は
「計算の履歴そのものの一覧は、サーバーの対応待ちです。」と注記していた(ADR-0511)。この注記を外し、新しい順の「計算履歴」を出す。

## 決定

1. **サービス**: `CalcHistoryService`(`calcHistory(limit:cursor:)`)を `FavoritesService` と同じ流儀で別プロトコルにする
   (`PokeCalcService` に混ぜない。絶対ルール5)。実装は `APIPokeCalcService+CalcHistory.swift` と `MockCalcHistoryService`。
   `FavoritesFeature` が登録し、`requiredServices` に足す(`CoreServices` は変えない)。
2. **経路**: 画面を開いたとき `limit=20`(`CalcHistory.pageSize`)の先頭ページ。「もっと見る」は `nextCursor` を**そのまま**
   `cursor` に渡して末尾に足す(加工・解釈しない)。`nextCursor` が null なら「もっと見る」を出さない。ポーリングしない。
   1ページ上限(50)は `RequestLimits` に持たせない(クライアントは 20 固定で範囲外の値を作らず、サーバーが 400 で守る。
   `check-request-limits.sh` の照合対象を増やす利点が無い)。
3. **ViewModel(`CalcHistoryViewModel`)**: `rows` は取得済みの `entries` と名前辞書から組み立てる。行の ID は
   「並びの位置-`occurredAt`(秒)」で、ページを足しても既存の行の ID は変わらない。最新の `load()` だけを反映する世代カウンタ
   (`FavoritesViewModel` と同じ)に `loadMore()` も従い、読み直しの後に届いた続きの応答は捨てる。続きの 400(`invalid_input`)は
   先頭から読み直す。続きのそれ以外の失敗(503 等)は `loadMoreError` を立て、取得済みの行と同じカーソルを保つ(「もっと見る」が再試行)。
   先頭ページの失敗は `loadState = .failed` で、前回の行は消さない。名前は種族 = `SpeciesNameResolver`、技 = `moves(ids:)` の
   まとめ取り1回で引き、引けないものは「不明なポケモン」「不明な技」で行を残す。
4. **画面**: 「計算履歴」の節を「お気に入り」と「よく計算する相手」(`OpponentHistory`。残す)の間に置く。エラー(`ErrorBannerView`・
   再読み込みボタン)は履歴の節の中だけで、画面全体・計算画面は塞がない。「契約待ち」の注記(`pendingHistoryNote`)は削除した。
   「この端末のデータを削除」(情報画面)の後に履歴が空になる件は、この画面を開くたびに先頭から読み直す(`.task`)ので追加の配線は要らない
   (お気に入りと同じ。削除はこの画面の外の別画面で行う)。個別削除は契約に無いので持たない。
5. **行から計算画面へ**: 行は `Button`。タップで `CalcScreenView(restoring:)` をルートの `NavigationStack` に push し、
   `CalcViewModel.loadHistoryCalc(_:)` で入力を復元して計算を**ちょうど1回**出し直す(`.task` で1回だけ)。画面は `FavoritesFeature` が
   渡すクロージャで計算画面を作る(履歴画面は計算画面の部品を知らない)。
   復元する範囲は、画面が表せる入力に限る:
   攻撃側の個体(種族・性格・SP・特性・持ち物・ランク・やけど・テラスタイプ。構築の個体と同じ `.team` の出どころ。`teamID = "history"`)、
   技(learnset に無い技でも履歴のものを黙って別の技に置き換えない)、防御側の種族・特性(種族が持たなければ落とす)・ランク、
   天候・フィールド・防御側の壁・急所。
   **表せないもの**: 防御側の性格・SP・持ち物(計算画面は防御側を種族と代表調整の一括で計算し、履歴の行の防御側の調整は使わない)、
   攻撃側の壁(シングルでは効かない)、`format`(画面はシングルのみ)。したがって復元後の結果は、履歴の行の `result` と一致するとは限らない
   (防御側の調整が違えば%が変わる)。防御側の「比較する持ち物」は空に戻す。
   マスタに無い種族・技は、画面の入力を書き換えず、計算の失敗として表示する(`handleInputFailure`。サーバーが `unknown_move` 等で
   400 を返す場合も同じ既存の失敗経路)。お気に入りの読み込み(ADR-0513)のように「読めた分だけ設定して案内する」ことはしない:
   履歴は「その計算をもう一度出す」ための入力なので、一部だけ違う計算を黙って出すより失敗にする。
6. **モック**: 環境変数 `POKECALC_MOCK_CALC_HISTORY`(`MockCalcHistoryService`)。未設定=3件を1ページ / `paged`(4件を**1ページ2件**。
   契約の `limit` とは無関係に固定し、UI で「もっと見る」を確かめる) / `paged-fail-more`(続きだけ 503) / `empty` / `fail` / `unavailable`。
   架空の key(9001〜9004)・`test-move-*`・`test-nature-*` だけを使う。カーソルはモック専用の不透明な文字列で、読めない値は 400 `invalid_input`。

## 却下した案

- `CalcRequest` の domain 型に場(`field`)を足して流用する: 逆算・調整・判定が使う `CalcRequest` の呼び出しをすべて触る。履歴専用の
  `CalcHistoryCalc` を別に置いた。
- 復元で先に既定の入力を計算してから上書きする(現行の `load()` → 復元)ことを避けて `load` に初期値を渡す: `load()` の改修範囲が大きい。
  復元の前に既定の計算が1回走るが、結果は復元の計算で置き換わる(許容。`loadHistoryCalc` 自身は計算を1回だけ呼ぶ)。
- 画面に戻るたびの読み直しを避ける: 「画面を開いたときに読み直す」(ADR-0230)に従う。計算画面から戻ると先頭ページに戻る(許容)。

## 結果

- 追加: `CalcHistory.swift`(型・プロトコル・文言・ViewModel)、`APIPokeCalcService+CalcHistory.swift`、`MockCalcHistoryService.swift`、
  `CalcViewModel.loadHistoryCalc`、`FavoritesScreenView` の節、`CalcScreenView(restoring:)`。
- テスト: `CalcHistoryViewModelTests`・`CalcViewModelHistoryLoadTests`・`APICalcHistoryServiceTests`・`MockCalcHistoryServiceTests`、
  XCUITest `CalcHistoryUITests`、`FavoritesScreenUITests` の変更(ADR-0501「計算履歴の一覧の接続」に前後を記録)。
