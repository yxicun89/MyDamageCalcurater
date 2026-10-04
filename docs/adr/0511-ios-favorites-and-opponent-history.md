# ADR-0511: iOS のお気に入りと計算履歴(よく計算する相手)の画面

- 状態: 提案(spec 段階。実装前)
- 日付: 2026-10-04
- 関連: ADR-0227(お気に入り API)、ADR-0209 §3・§4・§5・§6(お気に入り=手動ピン留め・540日・生イベント90日・端末境界)、
  ADR-0507(画面レジストリ)、ADR-0501「P6-23」(よく使う相手)・「P6-7」(端末データ削除)、requirements.md §2、
  docs/ai-shared/decisions/2026-10-03-api-p5-3c-favorites.md

## 背景

API レーンが P5-3c でお気に入りの3操作(`listFavorites` / `createFavorite` / `deleteFavorite`)を契約に入れた(`make ios-gen` 済み)。
計算履歴は、生のイベントを返す API が契約に無く、`GET /api/record/frequent-opponents`(P6-23 で iOS は種族ピッカーに実装済み)だけがある。
Web にはお気に入り・履歴の画面がまだ無い(契約の型テストのみ)ので、契約と要件だけを根拠にする。

## 決定

1. **1画面「お気に入り・履歴」**(`FavoritesFeature`。ルートのピル `openFavoritesScreen`、order 800、画面 id `favorites`。
   起動時に開く環境変数 `POKECALC_OPEN_FAVORITES_SCREEN_AT_LAUNCH`)。上にお気に入り、下によく計算する相手の2セクション。
   2つは独立に読み込み、片方の失敗がもう片方を巻き込まない(絶対ルール5)。
2. **サービスは別プロトコル**: `FavoritesService`(`APIPokeCalcService` の extension + `MockFavoritesService`)。`PokeCalcService` に混ぜない。
   `FeatureServices` へ `registerServices`(`AboutFeature` の `DeviceDataService` が見本)。計算画面の追加ボタンは `FeatureServices` から
   **任意で**引く(無ければボタンを出さない。`CalcFeature` の `requiredServices` には足さない)。
3. **追加の導線は計算画面の最小限**: 攻撃側・防御側の個体それぞれに「お気に入りに追加」ボタン(ラベル入力は持たない=ラベル無し)。
   `FavoriteLabel.normalize`(前後空白除去・空は未設定・30コードポイントに切り詰め)は API 層が送る前に必ず通す(将来の名前入力のため)。
   同じ内容は 200 `alreadyPinned`(「すでに追加済み」)、新規は 201 `created`。
4. **お気に入りを計算に読み込む導線は今回やらない**(要件は「手動ピン留め」のみで、読み込みは Web・iOS 共通の別タスク。Web レーンの結果を見て揃える)。
5. **計算履歴**: 契約に生イベントの一覧が無いので、**「よく計算する相手」の一覧画面**(`OpponentHistoryViewModel`。件数・最終計算時刻つき、
   名前を引けない相手も「不明なポケモン」で残す〈ピッカーは省くが、こちらは件数情報があるので残す〉)までとし、
   「計算の履歴そのものの一覧は、サーバーの対応待ち」という注記を出す。**推測で API を足さない**(`api/openapi.yaml` は触らない)。
   取得は既存の `FrequentOpponentsService`(`limit` = 50 = 契約の最大)。
6. **削除**は確認ダイアログ無しで即時(ピンを外すだけ。付け直せる)。404 は「すでに無い」として成功扱いで一覧から除く。
   外した直後に、削除前に始まった読み込みの古い応答が来ても復活させない。
7. **上限**: `RequestLimits.maxFavorites = 100`(listFavorites 応答の maxItems)・`maxFavoriteLabelLength = 30`(FavoriteInput.label.maxLength)を
   `check-request-limits.sh` が契約と照合する(spec 段階で追加済み)。上限到達の作成は契約上 400 `invalid_input`
   なので、追加の `invalidInput` の文言は上限に触れる。一覧が100件のときは上限の注記を出す。
8. **エラー**: `RecordScreenError`(code → 種類 → `FavoritesLabels` の日本語)。サーバーの英語 `message` と `code` は画面に出さない。
   503(`store_unavailable`/`upstream_unavailable`)の文言は必ず「計算はそのまま使えます」を含める。
9. **モック**: `MockFavoritesService` は環境変数 `POKECALC_MOCK_FAVORITES`(未設定=**空のストアで追加・削除が動く**。`list`/`fail`/`unavailable`/`full`)。
   既存の XCUITest は `openFavoritesScreen` を使わないので影響しない。

## 結果と未決

- Web レーンが同じ画面を作るときは、ここの文言・上限・冪等性の扱い(200/201)を揃える。
- 生の履歴 API が入ったら、`OpponentHistory` の注記を履歴一覧に置き換える(別 ADR)。
