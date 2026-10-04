# wishlist フェーズ3 iOS の受け入れ条件: 目安価格の表示

対象: `apps/wishlist/ios/`。「PWA と同じ挙動」の正は [phase3-web-spec.md](phase3-web-spec.md)(AC-EST-01〜12)、API は [phase3-api-spec.md](phase3-api-spec.md)、
iOS の構成・流儀は [phase2-ios-spec.md](phase2-ios-spec.md)。ロジックは `WishlistKit`(`make wishlist-ios-test`)、View は `Wishlist/`(`xcodebuild`)。
実装前は失敗する(スタブは空・nil・`WishlistError.stub`。クラッシュはしない)。`swift build --build-tests` と `xcodebuild build-for-testing` は通る。

## 決めたこと

- **ユーザー指示(2026-10-04)**: 検索して該当が無ければ金額を出さず「出品ないかも」だけを表示し、検索へのリンクは残す。推測の情報は出さない。
- **WishlistService に 2 メソッド**: `refreshEstimates(itemID:)`(POST。202 の本文を返す)と `listings(itemID:siteID:)`(GET。参考外を含む)。API 実装と Fake の両方。
  ドメイン型は `SiteEstimate`(`inStockCount` を追加。既定 0)・`ItemEstimates`(既存)・`Listing`・`SuspiciousReason`(新規)。
- **時計の差し替え**: `ItemDetailViewModel.init(…, now:, sleep:)`(既定は `Date()` と `Task.sleep`)。テストは `ManualSleeper`(要求を記録して `advance()` まで待たせる。キャンセルで `CancellationError`)。
- **ポーリングは VM が持つ `Task`**: `loadEstimates()` / `refresh()` は 1 回の通信が終わったら返り、`refreshing: true` なら**背後の Task が 5 秒ごとに最大 6 回** GET する(最初と合わせて最大 7 回)。
  `stopPolling()`(View はシートの `onDisappear` で呼ぶ)で取り消し、`waitForPolling()` はループの終了まで待つ(テスト用)。通信失敗では予約しない。
  定数は `EstimatePolling.interval`(5 秒)・`maxRefetches`(6)。
- **世代番号**: GET・POST を出すたびに番号を進め、応答は「自分が最新の番号のとき」だけ反映する(古い GET が新しい POST の結果を上書きしない)。
- **サマリ文言**(`summaryText`。1 つの文字列。`更新中…` は半角スペースで末尾に添える。PWA と同じ):
  `だいたい ¥A〜¥B で買えそう(M/D 時点)` / mid なしは `¥A〜` / 時点なしは括弧なし / 目安なし `まだ価格情報はありません` / すべて no_result `出品ないかも` / 通信できない(最初の取得)`オフライン`。
  `Summary` に `.priced(low:mid:at:)` と `.noListings` を足した。
- **PWA との差(既存テストを守った結果)**: 最初の取得が通信以外で失敗(401 など)したときの文言は、PWA の「価格情報を取得できませんでした」ではなく、
  フェーズ2の AC-IOS-VM-SHEET-04 のまま `まだ価格情報はありません`(「オフラインとは言わない」)。変えるなら既存テストの変更を伴うので、人が決める。
- **前回値の保持**: 一度でも estimates を取得できたら、更新(POST)・再取得(GET)が失敗しても値(サマリ・サイト行)を出し続け、`noticeText` に `オフライン(前回の値)`(通信失敗)/ `更新できませんでした`(それ以外)。次の成功で消える。
- **更新ボタン**: `canRefresh` = POST の応答待ちでなく・`isRefreshing` でなく・`summary != .offline`。条件を満たさなければ `refresh()` は何もしない。
- **サイト行**: `siteRows`(`links` と同じ並び・同じ件数。取得前も出る)。`SiteRow` の 4 項目 `estimateText`・`countText`・`stockText`・`noteText` と `accessibilityLabel`(サイト名 + あるものを半角スペースでつなぐ)。
  ok は `¥low〜¥mid` / `N件` / `在庫あり|在庫なし`(`inStockCount > 0`)。failed は前回値(low があれば同じ表示)+ `最終取得: N日前|今日`(JST の暦日の差)、前回値なしは `取得できませんでした` だけ。no_result は `出品ないかも` だけ。estimates に無い・ジャンルに無いサイトは出さない/リンクだけ。
- **参考外**: `suspiciousTotal`(`suspicious_count` の合計)が 0 なら欄も取得も無し。1 以上なら `suspiciousTitle`=`参考外 N件` の折りたたみ(`DisclosureGroup`)。**開いたときに** `loadSuspiciousListings()`(`siteID` なし)→ 理由が空でない出品だけ、API の並びのまま。
  `SuspiciousListingRow`(タイトル・`¥300`・日本語の理由・画像 URL・リンク URL)。画像・リンクは `Deeplink.isHTTPURL` を通った URL だけ(`javascript:`・`data:` は nil)。取得失敗は `.failed("参考外の出品を取得できませんでした")` を折りたたみの中に出す(シート全体は壊さない)。
- **整形は純関数** `PriceFormat`(`yen`・`range`・`jstDate`・`age`・`reasonLabel`)。JST・ja-JP を明示し、実行環境のロケール・タイムゾーンに依存させない。
- **常時動くアニメーションは入れない**(更新中は文字だけ。スピナー・`ProgressView` を使わない)。
- **モック起動**: `WISHLIST_USE_FAKE=estimates`(`1` と同じ + 目安価格・参考外)。`1` は従来どおり価格情報なし(フェーズ2の XCUITest が依存)。
  `WishlistFixtures.estimates`(商品 12 = メルカリ ok ¥3,000〜¥4,500・5 件・在庫 4・参考外 1 / Amazon no_result(架空)、商品 11 = 両方 no_result)・`listings`(出品 102 が参考外)・`makeServiceWithEstimates()`。
  `AppModel` の分岐は spec-writer が足した(配線だけ)。

## テスト支援

- `FakeWishlistService`: `setEstimatesSequence([…])`(GET を順に返し、最後は返し続ける)・`setRefreshResponse(_)`(未設定なら先頭 + `refreshing: true`)・`setListings(_)`・`gate`(estimates / refreshEstimates / listings の応答を返す直前に呼ぶ。応答の保留用)。Call に `.refreshEstimates`・`.listings` を追加。既存の振る舞いは不変。
- `Tests/WishlistCoreTests/Support/ManualClock.swift`: `ManualSleeper`・`Gate`・`iso(_:)`。

## 受け入れ条件とテスト

テストは `ios/WishlistKit/Tests/WishlistCoreTests/`。XC は XCUITest(`ios/WishlistUITests/WishlistUITests.swift`。シミュレータのみ)。

| ID | 条件 | テスト |
|---|---|---|
| AC-IOS-EST-01 | サマリの文言(値あり・mid なし・時点なし・目安なし・すべて no_result・failed だけ・混在)。no_result のサマリに金額なし。`refreshing` はどの形にも ` 更新中…` を添える。最初の取得が通信失敗なら `オフライン`(注記なし・リンクあり・更新不可・POST しない) | `ItemDetailEstimatesTests.testSummaryTextTable`・`testNoListingsSummaryHasNoAmount`・`testRefreshingAppendsUpdatingToEveryForm`・`testFirstFetchNetworkFailureIsOfflineAndDisablesRefresh` |
| AC-IOS-EST-02 | サイト行: ok / mid なし / 在庫 0 / failed(前回値・2 日前・同じ日)/ failed 前回値なし / no_result(金額・件数・在庫なし)。ジャンルの順・取得前でも出る・目安なしはリンクだけ・ジャンル外は出さない・`accessibilityLabel` | `testSiteRowsTable`・`testSiteRowsFollowGenreOrderAndKeepLinkOnlyRows`・`testSiteRowAccessibilityLabelJoinsNameAndParts` |
| AC-IOS-EST-03 | 再取得: refreshing:false なら予約なし / 5 秒後に GET し false で止まる / 最大 6 回で打ち切り `更新中…` を消し値は残す(GET は 7 回)/ 閉じたら予約を取り消す / 再取得の失敗は前回値を残して止まる | `testNoPollingWhenNotRefreshing`・`testPollsEveryFiveSecondsAndStopsWhenRefreshingIsFalse`・`testGivesUpAfterSixRefetchesAndKeepsTheValue`・`testStopPollingCancelsThePendingWait`・`testPollFailureKeepsThePreviousValueAndStops` |
| AC-IOS-EST-04 | 「更新」: POST → `更新中…` → 5 秒後に GET → 止まる。応答待ち・refreshing 中・オフラインは押せず、連打しても POST は重ならない | `testRefreshPostsShowsUpdatingThenPollsOnce`・`testRefreshDoesNotOverlap`・`testRefreshIsDisabledWhileRefreshingEvenWithoutPressing` |
| AC-IOS-EST-05 | 更新が失敗しても前回値を残し、注記(通信失敗 / それ以外)を出して再取得を予約しない。次の成功で注記が消える | `testRefreshFailureKeepsThePreviousValue`・`testNoticeClearsOnTheNextSuccess` |
| AC-IOS-EST-06 | 古い GET が遅れて返っても、新しい POST の結果を上書きしない | `testStaleGetDoesNotOverwriteTheRefreshResult` |
| AC-IOS-EST-07 | 参考外: 合計 0 なら欄も listings も無し / `参考外 N件`(合計)・開くまで取らない / 開くと GET listings(siteID なし)・理由のある出品だけ・並びそのまま・日本語の理由・http(s) 以外は画像もリンクも nil / 失敗は折りたたみ内の文で、目安は壊さない | `testSuspiciousIsHiddenWhenTheTotalIsZero`・`testSuspiciousTitleSumsAndDoesNotFetchUntilOpened`・`testLoadSuspiciousFiltersMapsAndSanitizes`・`testLoadSuspiciousFailureKeepsEstimates` |
| AC-IOS-EST-08 | 整形の純関数(桁区切り・range・JST の M/D(年またぎ・UTC との日付差)・N日前(JST の暦日・未来)・理由の文言) | `PriceFormatTests`(テーブル駆動 5 件) |
| AC-IOS-EST-09 | `APIWishlistService`: estimates の写像(`in_stock_count`・status 3 種・null)/ `refreshEstimates` は POST・202 の本文・Bearer・失敗は `WishlistError` / `listings` は GET・`site_id` クエリ・全項目・理由・null の画像・通信失敗は `.network` | `APIWishlistEstimatesTests` 全件 |
| AC-IOS-EST-10 | モック用フィクスチャ(商品 12 = ok + no_result・参考外 1、商品 11 = 全 no_result、参照整合、`makeServiceWithEstimates`) | `WishlistFixturesEstimatesTests`(spec-writer が実装済みなので最初から通る) |
| AC-IOS-EST-UI-01 | **XC** 商品 12: サマリ `だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)`、メルカリ行に金額・件数・在庫、Amazon 行は `出品ないかも` だけ(`¥`・`件`・`在庫` なし)、`refreshButton` が押せる、`suspiciousDisclosure`(`参考外 1件`)は開くまで出品なし、開くと `suspiciousListing-102`(¥300・理由 2 つ)だけ | `WishlistUITests.testEstimateRowsShowPricesAndNoListingsAndSuspiciousDisclosure` |
| AC-IOS-EST-UI-02 | **XC** 商品 11: サマリは `出品ないかも` だけ、両サイト行が残り、`suspiciousDisclosure` は無い | `WishlistUITests.testAllNoResultShowsNoListingsSummaryAndKeepsLinks` |

既存の `testSummaryWithoutPriceInfo`・`testSummaryForNonNetworkFailureIsNotOffline` などフェーズ2のテストは変えていない(estimates が空なら従来どおり)。

## accessibilityIdentifier の取り決め(View が付ける。フェーズ2の分に追加)

- `summaryText`(既存。ラベル = `summaryText`)/ `siteRow-<siteID>`(既存。ラベル = `SiteRow.accessibilityLabel`。`.accessibilityElement(children: .combine)` などで 1 要素にし、タップは今までどおり `UIApplication.shared.open`)
- `refreshButton`(ラベル「更新」。`canRefresh` が false なら `disabled`)
- `suspiciousDisclosure`(ラベルに `参考外 N件` を含む。`DisclosureGroup` のラベル。`suspiciousTotal == 0` なら存在しない。開いたら `loadSuspiciousListings()`)
- `suspiciousListing-<出品 ID>`(ラベル = タイトル・価格・理由を半角スペースでつないだもの。画像・「開く」リンクは子。リンクは `linkURL` があるときだけ)

## implementer への注意

- テスト・フィクスチャの値を変えて通さない。契約が不都合なら spec-writer へ戻す。
- スタブ: `PriceFormat`(空文字)、`ItemDetailViewModel` の新規メンバ(`canRefresh`・`siteRows`・`suspiciousTotal`・`suspiciousTitle`・`refresh`・`loadSuspiciousListings`・`stopPolling`・`waitForPolling`・`summaryText` の `.priced`/`.noListings`)、
  `APIWishlistService.refreshEstimates`・`listings`(`WishlistError.stub` を投げる)。`APIWishlistService.estimates` は `inStockCount` を写していない(足す)。`loadEstimates()` は今は結果を捨てている(置き換える)。
- 生成クライアントの操作: `refreshItemEstimates`(`.accepted`)・`listItemListings`(query `siteId`)。`SuspiciousReason` は生成型と同じ 3 値。未知の理由・status は落とさない方針(既存の `EstimateStatus(rawValue:) ?? .failed` に倣う)。
- `loadEstimates()` の通信失敗は `WishlistError.isNetwork` で分ける。値を取得済みなら `summary` は変えず `noticeText` だけ立てる。
- ポーリングの `sleep` は `Task.sleep` 以外を直接呼ばない(`ManualSleeper` が効かなくなる)。`self` を強く保持し続けないこと(`[weak self]`)。シートを閉じたら `stopPolling()`。
- View はシートの `.task` で `loadEstimates()`、`.onDisappear` で `stopPolling()`。更新中は文字だけ(`ProgressView`・`repeatForever` 不可)。
- 実行確認: `make wishlist-ios-test`。XCUITest は共有シミュレータで流さず、専用に複製して `WISHLIST_IOS_SIMULATOR` で指定し、終わったら削除する(spec-writer は XCUITest を実行していない。`xcodebuild build-for-testing` が通ることだけ確認した)。

## 既知の差・許容(critic の推奨。2026-10-04)

- 最初の取得が通信以外(401 等)で失敗したときのサマリは、フェーズ2の AC-IOS-VM-SHEET-04 に合わせ「まだ価格情報はありません」。
  PWA(phase3-web-spec.md)は「価格情報を取得できませんでした」で、ここだけ文言が違う。揃えるなら既存テストの変更が要る(人の判断)。
- 参考外の一覧は、開いたときに 1 回だけ取得する。開いている間に「更新」で件数が変わっても再取得しない(折りたたみを閉じて開き直すと取り直す)。
- 参考外の画像は AsyncImage(トークンなし)。ATS で読めない外部の http 画像は灰色のプレースホルダーのまま。リンクは http(s) を `UIApplication.shared.open` で開くので影響なし。
