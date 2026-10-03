# フェーズ1 Web(PWA)の受け入れ条件

対象: `apps/wishlist/web/`。仕様は CLAUDE.md §3・§4・§5・§10・§12、設計は docs/design.md §6(W-03・W-04・W-07)、API 契約は api/openapi.yaml と docs/phase1-api-spec.md。
テストは `cd apps/wishlist/web && npm test`(= `make wishlist-web-test`)。実装前は失敗する(スタブは `not implemented` を投げる / 空のコンポーネント)。

## 決めたこと

- ルーティングはライブラリなし。状態で画面を切り替える(ホーム / 設定。シート・登録・編集・確認はダイアログ)。
- 画面テストは `App` だけを入口にする(内部のコンポーネント構成は自由)。操作と検査は role・アクセシブルネームで行う(下の「画面の契約」)。
- 一覧のジャンル絞り込みはクライアント側(全件を取得して `genre_id` で絞る。オフラインでも効かせるため)。
- フェーズ1の「参考外」は 0 件なので、折りたたみ自体を出さない(「参考外」の文字も出さない)。
- サマリ欄(`role="status"`):詳細を開いたとき `GET /api/items/:id/estimates` を呼び、`sites` が空なら「まだ価格情報はありません」。通信できない(`ApiError.code === "network"` または `navigator.onLine === false`)なら「オフライン」。
- 検索ワードを空にして保存すると `query_override: null`(テンプレートへ戻す)。編集ダイアログも同様に、空にした任意項目は `null` で送る。PATCH は**変えた項目だけ**送る。
- トークン未設定で起動したら設定画面から始め、API を呼ばない。一覧の取得に失敗したら(401 を含む)保存済みの一覧を使い、保存済みも無ければ `role="alert"` を出す。
- SW のロジックは `public/sw-strategy.js`(classic script。`self.wishlistSw` に `classify` / `shouldCache` / `cacheNames` を定義)に分け、`public/sw.js` が `importScripts("sw-strategy.js")` で使う。テストは vm でこのファイルを読む。
- アイコンは単色の簡単な図形を自作(SVG と、iPhone 用の `public/apple-touch-icon.png` 180x180 PNG。外部素材なし。PNG は node の zlib などで生成してよい)。

## 画面の契約(アクセシブルネーム)

| 画面 | 要素 |
|---|---|
| ホーム | グリッド `data-testid="item-grid"`(中の各商品は `<button>`+`<img alt=商品名>`、グリッドの `textContent` は空)。ジャンルは `<nav aria-label="ジャンル">` の button(「すべて」+各ジャンル、`aria-pressed`)。右下 `button[aria-label=追加]`(表示は「+」)、`button[aria-label=設定]` |
| 長押しメニュー | `role="menu"` に `menuitem`「編集」「削除」。削除確認は `role="alertdialog"`(商品名を含む。「削除する」「キャンセル」) |
| 詳細シート | `role="dialog"` name「詳細」。「閉じる」、`button[name=検索ワードを編集]`(表示は現在の検索ワード)→ `textbox[name=検索ワード]` と「保存」、`role="status"`(サマリ)、サイト行は `<a>`(`role=link`) |
| 登録 | `dialog[name=登録]`。ファイル入力 label「写真」、`textbox[name=URL]`+「取得」、`textbox[name=名前]`、`combobox[name=ジャンル]`、「登録」「キャンセル」 |
| 編集 | `dialog[name=編集]`。`名前`・`オプション`・`検索ワード上書き`(textbox)、`ジャンル`(combobox)、`最低価格(円)`(spinbutton)、ファイル入力 label「画像を差し替え」、「保存」 |
| 設定 | `heading[name=設定]`、「戻る」。`textbox[name=APIのURL]`、`トークン`(`type=password`)、「保存」。`region[name=ジャンル]`(`listitem` が sort_order 順。「ジャンルを追加」「<名前>を編集」)と `dialog[name=ジャンル]`(`ジャンル名`・`検索ワードのテンプレート`、サイトごとの `checkbox`、チェック済みサイトの「<サイト名>を上へ」ボタン)。`region[name=サイト]`(「サイトを追加」「<名前>を編集」)と `dialog[name=サイト]`(`サイト名`・`検索URLのテンプレート`・`combobox[name=取得方式]`)。エラーは `role="alert"` |

localStorage の契約: `wishlist.settings`=`{"apiBaseUrl": string|null, "token": string}`、`wishlist.cache.items|genres|sites`=該当する配列の JSON。

## 受け入れ条件とテスト

| ID | 条件 | テスト |
|---|---|---|
| AC-LIB-01 | `buildQuery` が testdata/query-cases.json の build を全件満たす(空 option の空白詰め・優先順位 site_query > query_override > テンプレート) | `src/lib/query.test.ts` |
| AC-LIB-02 | `buildDeeplink` が deeplink を全件満たす(encodeURIComponent、再置換しない) | `src/lib/deeplink.test.ts` |
| AC-LIB-03 | `isValidSearchTemplate` は http(s) で `{q}` を含むものだけ true | `src/lib/deeplink.test.ts` |
| AC-LIB-04 | `resolveSiteLinks`: ジャンルの site_ids 順・enabled=false 除外・サイト別 query 優先・未知のサイト ID と未知のジャンルの扱い | `src/lib/links.test.ts` |
| AC-LIB-06 | settings の保存・読み出し。壊れた値・localStorage 例外でも落ちない | `src/lib/settings.test.ts` |
| AC-LIB-07 | cache の保存・読み出し・`fetchWithCache`(失敗時は保存済み・stale=true、無ければ元の例外、失敗で上書きしない) | `src/lib/cache.test.ts` |
| AC-API-01〜06 | ベース URL 基準・Bearer・JSON/multipart・204・エラー応答の型付け(ApiError.code/status)・通信失敗は `network`(status 0)・fetch は呼び出し時点の globalThis.fetch | `src/lib/api.test.ts` |
| AC-API-07 | 画像 URL は `new URL(image_url, baseUrl)`、`defaultBaseUrl`(document.baseURI のディレクトリ)、`normalizeBaseUrl` | `src/lib/api.test.ts` |
| AC-HOME-01〜09 | 画像のみのグリッド(文字なし・src はベース基準)/ Bearer / ジャンルチップ(sort_order 順・絞り込み)/「+」/ 0 件でも落ちない / 長押し 500ms でメニュー(離してもシートは開かない)/ 短いタップでシート / 途中で離す・キャンセルでは成立しない / 設定への出入り | `src/App.home.test.tsx` |
| AC-SHEET-01〜09 | シートの構成 / サイト行 `<a target=_blank rel=noopener>`・site_ids 順 / option の空白詰め / enabled=false・サイト別 query / サマリ「まだ価格情報はありません」/ 参考外を出さない / その場編集は一時的(保存まで PATCH しない・閉じれば破棄)/ 保存で PATCH `{query_override}`(空は null)/ 保存失敗は alert で編集値を保持 | `src/App.sheet.test.tsx` |
| AC-REG-01〜06 | 写真で登録(multipart)/ URL から下書き → 確認 → JSON 登録 / チップのジャンルが初期値 / 必須が揃うまで「登録」不可 / 失敗は alert で入力保持 / キャンセル | `src/App.register.test.tsx` |
| AC-EDIT-01〜05 | 編集は変えた項目だけ PATCH / 空にした任意項目は null / 画像差し替え PUT / 削除は確認のあとだけ DELETE / 失敗は alert でグリッド不変 | `src/App.edit.test.tsx` |
| AC-SET-01〜07 | トークン未設定は設定から・API を呼ばない / URL とトークンの保存と利用 / ジャンルの一覧・追加(サイトの選択と順序)・編集(site_ids 全件置き換え)/ サイトの追加(`{q}` 必須の検証)・編集 | `src/App.settings.test.tsx` |
| AC-OFF-01〜05 | API が落ちていても保存済みの一覧・チップ・リンクが出る / サマリは「オフライン」/ 401 でも保存済みを使う / 保存済みも無ければ alert(白画面にしない)/ オンラインに戻ると最新に置き換わる | `src/App.offline.test.tsx` |
| AC-PWA-01 | manifest(名前「欲しいもの」・standalone・start_url と scope が `/wishlist/`・アイコンが public に存在・外部 URL なし・apple-touch-icon 180x180 PNG) | `src/pwa/pwaFiles.test.ts` |
| AC-PWA-02 | index.html(viewport-fit=cover・manifest・apple-touch-icon・タイトル)・`env(safe-area-inset-top/bottom)` の CSS・`infinite` アニメーション無し | `src/pwa/pwaFiles.test.ts` |
| AC-PWA-03 | SW の戦略: `/api/*`・healthz・sw.js は network-only(キャッシュしない)/ `images/*` は cache-first / navigate は network-first-shell / その他の同一オリジン静的ファイルは stale-while-revalidate / 非 GET・他オリジン・scope 外は ignore / 200 の basic 応答だけキャッシュ | `src/pwa/swStrategy.test.ts` |
| AC-PWA-04 | `public/sw.js` が sw-strategy.js を読み、install・activate・fetch を扱い、`/api/` をキャッシュに入れる処理を直接持たない | `src/pwa/swStrategy.test.ts` |
| AC-PWA-05 | `registerServiceWorker`: `${base}sw.js` を scope `${base}` で登録。開発では登録しない。未対応・失敗でも例外なし | `src/lib/registerSw.test.ts` |
| AC-PWA-06 | `vite.config.ts` の base は `/wishlist/` | `src/pwa/pwaFiles.test.ts` |
| AC-DEP-01 | `web/Dockerfile` と `web/nginx.conf`: 非 root・8080・`/healthz`・SPA フォールバック(`try_files ... /index.html`)・sw.js と index.html は no-cache | `src/pwa/pwaFiles.test.ts`(中身の検査のみ。イメージのビルド・起動は implementer が `make wishlist-docker-build` 相当で確かめる) |
| AC-BUILD-01 | `make wishlist-web-deps/test/lint/build/gen` があり、`wishlist-test/lint/build/gen` の前提になっている | `make -n` で確認(Makefile 実装済み) |

## implementer への注意

- テストを変えて通さない。契約が不都合なら、先に spec-writer へ戻す。
- `src/lib/*.ts` と `App.tsx`、`public/sw-strategy.js`・`public/sw.js` はスタブ。`src/api/schema.d.ts` は生成物(`make wishlist-web-gen`。手で直さない)。`src/api/types.ts` はそこからの別名。
- 画面の中身(コンポーネント分割・CSS)は自由。ただし上の契約(role・名前・`data-testid="item-grid"`)は守る。CSS は `src/` 内の `.css` に書く(テストが `src/**/*.css` を走査する)。`@keyframes` を使うのは操作時のみ。
- `createApiClient` は `fetch` を生成時に捕まえない(`vi.stubGlobal("fetch")` で差し替える)。画像 `<img src>` は `resolveImageUrl(item.image_url, baseUrl)`(`baseUrl` は設定値 or `defaultBaseUrl()`)。
- `fetchWithCache` の保存は成功時だけ。`fakeApi`(`src/test/fakeApi.ts`)は Bearer が `test-token` でないと 401 を返す。
- 長押しは pointer イベント(`pointerdown` で 500ms タイマー、`pointerup`・`pointercancel`・`pointerleave` で解除。成立後の `click` は無視)。`contextmenu` も抑止すること(iOS の長押しメニュー対策)。
- 未決: 価格の状態表示(在庫あり等)・参考外の折りたたみはフェーズ3(docs/design.md §7)。
- `.node-version`・`engines` は 26.10.0。npm install は web/ 内で行い、`package-lock.json` をコミットする。依存はすべて固定(`^` なし)。
