## 2026-10-03: お気に入り(手動ピン留め)の API 契約を追加(API レーンから Web・iOS レーンへ。P5-3c・ADR-0227)
Decision: `api/openapi.yaml` に record-svc のお気に入り API を追加した(契約先行。record-svc の実装も同じブランチで済み。critic PASS・ADR-0227 採用)。
- `GET /api/record/favorites`(`listFavorites`): この端末のピンを `updatedAt` 降順(同時刻は `id` 降順)で全件。最大100件・ページングなし。無ければ空配列。
- `POST /api/record/favorites`(`createFavorite`): 本文 `FavoriteInput = { label?: string|null(30文字まで), individual: Individual }`。新規は 201、**同じ内容(label + 既定値を補った individual)がすでにあれば 200 で既存を返す**(`updatedAt` が進み一覧の先頭へ)。上限100件を超える作成は 400 `invalid_input`。
- `DELETE /api/record/favorites/{favoriteId}`(`deleteFavorite`): 204。持っていない・他端末・形式違いの ID は 404 `not_found`。
- 更新(PUT/PATCH)と1件取得は無い(ラベルを変えるときは外して付け直す)。`favoriteId` は10進の文字列。
- 応答の `Favorite.individual` は正規化済み(`level` 50・`ranks` 6キー・`status` あり、未指定の abilityId/itemId/teraType は省略)。
- ヘッダは既存と同じ `X-Device-Id`・`X-Session-Id`。エラーは既存の語彙だけ(503 `store_unavailable`/`upstream_unavailable` は「保存できないが計算はできる」)。
Reason: requirements.md §2「お気に入り(手動ピン留め)」と iOS レーンの依頼(decisions/2026-10-03-210-ios-4-ios-api-web.md の 3)。ユーザー指示「全レーンを100%にしてアプリを使えるようにする」。
Impact: **Web レーンへ**: `recordClient` に3操作を足し、計算画面から攻撃側・防御側の個体をピン留め・一覧から読み込む画面を作ってほしい(生成型は `make gen-ts` 済み。`web/src/record/favoritesContract.test.ts` が型の前提を固定)。**iOS レーンへ**: 生成クライアントの3操作と `Components.Schemas.Favorite`/`FavoriteInput` を使える(`make ios-gen` 済み。`FavoritesContractTests.swift`)。計算履歴(生のイベント)の取得 API はまだ無い(別タスク)。サーバーの実装(record-svc)は同じブランチにあり、main に入って k3d に反映されるまで実環境では 404 になる。
