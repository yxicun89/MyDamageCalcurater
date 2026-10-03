# フェーズ2(iOS ネイティブ版)の受け入れ条件

対象: `apps/wishlist/ios/`。仕様は CLAUDE.md §3・§4・§5・§9・§9.5・§12(フェーズ2)、設計は docs/design.md、API 契約は api/openapi.yaml(変えない)。
「PWA と同じ挙動」の正は [phase1-web-spec.md](phase1-web-spec.md)。ダメ計の iOS(`ios/`)と同じ構成:ロジックは `WishlistKit`(`swift test`)、View は `Wishlist/`・`WishlistShare/`(`xcodebuild`)。

テスト: `make wishlist-ios-test`(= `cd apps/wishlist/ios/WishlistKit && swift test`。xcode-env を通して `DEVELOPER_DIR` を補う)。
実装前は失敗する(スタブは空・nil・`WishlistError.stub` を返す。クラッシュはしない)。`swift build --build-tests` と `xcodebuild build-for-testing`(スタブ View でプロジェクトがビルドできる)は通る。

## 決めたこと

- **構成**:`WishlistKit`(WishlistAPI = 生成物、WishlistCore = それ以外)。ダメ計の PokeCalcKit と同じく platforms は iOS 27・macOS 27、依存の版は完全に同じ。
- **ドメイン型を生成型から切り離す**(`Domain.swift`)。生成型 ↔ ドメインの写像は `APIWishlistService` に閉じる。ID は `Int`、日時は `Date`、画像パスは `Item.imageURLPath`。
- **PATCH の null(重要)**:生成型 `ItemUpdate` の nullable 項目は `String?` で、nil は JSON から**省略**される(null を送れない)。PWA の「空にしたら null」を守るため、ドメインは `FieldUpdate<T>`(`.keep` / `.set` / `.clear`)を持ち、
  `APIWishlistService` は `.clear` を JSON の `null` で送る。実装方法は自由(PATCH だけ `ClientMiddleware` でボディを組み直す、等)。`APIWishlistServiceTests` が送信ボディを固定する。openapi.yaml は変えない。
- **共通テストベクタはコピー**:`testdata/query-cases.json` を `Tests/WishlistCoreTests/Resources/query-cases.json` にコピーし(SwiftPM のリソースは Package 外を指せず、シンボリックリンクはシミュレータ実行で壊れうる)、`ios/scripts/sync-testdata.sh` で同期する。`make wishlist-ios-test` は `--check` を先に走らせる(元と違えば失敗)。
- **キャッシュ**:SwiftData(`SwiftDataWishlistCache`、macOS の `swift test` のインメモリで動くことを確認済みなので JSON への切り替えは不要)。種類(items・genres・sites)ごとに全件置き換え。画像は `ImageFileCache`(ファイル)。
  `WishlistRepository` が「成功したら保存して `stale=false`・失敗したら保存済みを `stale=true`・保存済みも無ければ元の例外」を持つ(web の `fetchWithCache` と同じ)。
- **設定**:`WishlistSettings`(API の URL と トークン)を `UserDefaults` の `wishlist.settings` に、PWA の localStorage と同じ JSON(`{"apiBaseUrl", "token"}`)の `Data` で保存(Keychain は将来)。
  Share Extension は本体とデータを共有しない(§9.5)ので、拡張は拡張自身の UserDefaults で同じ型(`WishlistSettings`・`UserDefaultsSettingsStore`・`ConnectionSettingsViewModel`)を使う。**拡張にも同じ設定画面を持たせる**。
- **ベース URL**:iOS には「既定の URL」が無いので、URL とトークンの**両方**が入って初めて `isConfigured`(PWA はトークンだけ)。未設定なら設定画面から始め、API を呼ばない。
- **フェーズ2は価格を取得しない**:サマリは「まだ価格情報はありません」か、通信できなければ「オフライン」(PWA と同じ。参考外は出さない)。
- **登録のジャンル初期値**:ホームのチップで選んでいるジャンル。無ければ sortOrder 昇順の先頭(PWA は未規定。iOS は選択式 UI のため先頭を選んでおく)。
- **共有された URL の取り出し**:`SharedURL.extract`(Safari は URL、他のアプリはテキストで渡してくる)。OGP に画像が無い下書きは登録できない(API は画像が必須)。
- **WishlistKit にデザイントークンを置かない(判断)**:ダメ計の PokeCalcDesign は独自の色・間隔の体系(docs/design.md)を持つが、wishlist は「画像だけのグリッド」で文字・色の語彙が少なく、Liquid Glass(`glassEffect`)とシステムの意味色(`.primary`・`.secondary`・`AccentColor`)で足りる。
  トークンのターゲットを足すと SwiftUI 依存のテスト(色の同期・コントラスト)も要るため、フェーズ2では持たない。**数値で固定したい UI 定数だけ** `HomeLayout`(3 列・長押し 0.5 秒)に置いた。見た目の語彙が増えたら `WishlistDesign` ターゲットを足す。
- **FakeWishlistService / InMemoryWishlistCache は WishlistCore に置く**(spec-writer が書いた完成品)。ViewModel のテストだけでなく、モック起動(XCUITest・プレビュー)でアプリも使う。
- **Bundle ID**:ダメ計(`com.example.pokecalc`)の命名に合わせ、`com.example.wishlist`・`com.example.wishlist.share`・`com.example.wishlist.WishlistUITests`。署名チームは空。
- **常時動くアニメーションは入れない**(演出は操作時のみ)。

## 受け入れ条件とテスト

テストは `ios/WishlistKit/Tests/WishlistCoreTests/`。「XC」は xcodebuild(シミュレータ)でだけ動く XCUITest(`ios/WishlistUITests/WishlistUITests.swift`)。

| ID | 条件 | テスト |
|---|---|---|
| AC-IOS-LIB-01 | `SearchQuery.build` が testdata/query-cases.json の build を全件満たす(空 option の空白詰め・優先順位 siteQuery > queryOverride > テンプレート・1 回の走査で置換) | `SearchQueryTests.testBuildMatchesAllSharedVectors`・`testNilOptionBehavesLikeEmpty`・`testSpecTableExamples` |
| AC-IOS-LIB-02 | `Deeplink.build` が deeplink を全件満たす(encodeURIComponent と同じエスケープ集合・`{q}` を再置換しない) | `DeeplinkTests.testBuildMatchesAllSharedVectors`・`testQueryWithReplacementPatternCharactersIsLiteral` |
| AC-IOS-LIB-03 | `isValidSearchTemplate` は http(s) で `{q}` を含むものだけ true。`isHTTPURL` は http(s) の絶対 URL だけ | `DeeplinkTests.testIsValidSearchTemplate`・`testIsHTTPURL` |
| AC-IOS-LIB-04 | `SiteLinks.resolve`:ジャンルの siteIDs 順・enabled=false 除外・サイト別 query > queryOverride > テンプレート・http(s) 以外の URL を除く・未知のサイト ID と genre なし | `SiteLinksTests` 全件 |
| AC-IOS-SVC-01 | `APIWishlistService`:全リクエストに `Authorization: Bearer <token>`、パス・メソッド・JSON/multipart ボディが API 契約どおり。**`.clear` は JSON の null、`.keep` は省略** | `APIWishlistServiceTests`(`testListItemsSendsBearerTokenAndMapsFields`・`testUpdateItem*`・`testCreateItem*`・`testReplaceItemImageSendsMultipartPut`・`testDraftFromURLSendsURLAndGenre`・`testCreateSite*`・`testCreateGenreOmitsNilFields`・`testUpdateGenre*`・`testUpdateSite*`・`testDeleteItemAcceptsNoContent`) |
| AC-IOS-SVC-02 | 応答の写像:snake_case → ドメイン、日時、`site_ids` の順。エラーは Error スキーマの code・status・message を `WishlistError` に(JSON でない応答はステータスから)。通信できなければ `.network`(status 0)。ベース URL の末尾 `/` の有無によらず前置 `/wishlist` が残る | `APIWishlistServiceTests`(`testBaseURLPrefixIsKeptWithAndWithoutTrailingSlash`・`testErrorBodyMapsCodeStatusAndMessage`・`testNonJSONErrorMapsFromStatus`・`testTransportFailureMapsToNetworkError`・`testEstimatesMapsEmptySites`・`testListGenresKeepsSiteIDOrder`・`testListSitesMapsFetchType`) |
| AC-IOS-SVC-03 | 画像 URL は `images/<name>` をベース URL(末尾 `/` を保証)基準で解決する(前置が抜けない)。絶対 URL はそのまま | `WishlistURLsTests` |
| AC-IOS-SET-01 | `WishlistSettings.baseURL` は http(s) だけ。`isConfigured` は URL が使えてトークンが空白だけでない | `WishlistSettingsTests` |
| AC-IOS-SET-02 | `UserDefaultsSettingsStore`:保存・読み出し(別インスタンスでも)、PWA と同じ JSON の形、壊れた値・型違いは既定値、http(s) 以外の URL は捨ててトークンは残す | `UserDefaultsSettingsStoreTests` |
| AC-IOS-SET-03 | `ConnectionSettingsViewModel`(本体・拡張共通):初期化・trim して保存・http(s) 以外は保存せず `errorMessage`・空 URL は未設定で保存・ストアの失敗を報告 | `ConnectionSettingsViewModelTests` |
| AC-IOS-CACHE-01 | `SwiftDataWishlistCache`(インメモリ):保存が無ければ nil、items・genres・sites の往復(全項目・並び・site_ids の順)、種類ごとの全件置き換え、空配列は nil と区別、種類が独立 | `SwiftDataWishlistCacheTests` |
| AC-IOS-CACHE-02 | ファイルのストアは、同じ URL で作り直しても残る | `SwiftDataWishlistCacheTests.testFileStoreSurvivesANewInstance` |
| AC-IOS-CACHE-03 | `WishlistRepository`:成功で保存・`stale=false`、失敗(通信・401)でキャッシュを `stale=true` で返す、保存済みが無ければ元の例外、失敗でキャッシュを上書きしない、空配列の保存済みも返す、復帰で最新に置き換わる | `WishlistRepositoryTests` |
| AC-IOS-CACHE-04 | `ImageFileCache`:2 回目はファイルから(loader を呼ばない)・作り直しても通信せず返す・失敗して保存済みが無ければ投げて途中のファイルを残さない・URL ごとに別 | `ImageFileCacheTests` |
| AC-IOS-CACHE-05 | Mac が落ちていても、保存済みの一覧・サイト行(ディープリンク)・画像が使える。サマリは「オフライン」。復帰して取得できたキャッシュは次回に効く | `OfflineScenarioTests` |
| AC-IOS-VM-HOME-01 | ホーム:`refresh` が 3 つを取得してキャッシュへ保存、`loadCached` は通信せずキャッシュで埋める | `HomeViewModelTests.testRefreshLoadsAllThreeListsAndSavesThem`・`testLoadCached*` |
| AC-IOS-VM-HOME-02 | 失敗(通信・401)しても保存済みを出し `isStale`、保存済みも無ければ `loadError`(白画面にしない)、復帰で置き換わり stale が消える | `HomeViewModelTests.testRefreshFailure*`・`testBackOnline*` |
| AC-IOS-VM-HOME-03 | 設定が無い(URL かトークン)なら `needsSettings`、API を呼ばない | `HomeViewModelTests.testNeedsSettings*`・`testRefreshDoesNotCallTheServiceWhenNotConfigured` |
| AC-IOS-VM-HOME-04 | ジャンルで絞り込み(nil はすべて)・商品は sortOrder 昇順(同順は id 降順)・チップは sortOrder 昇順(同順は id 昇順)・0 件でも落ちない | `HomeViewModelTests.testVisibleItems*`・`testChipGenres*`・`testEmptyListsAreFine` |
| AC-IOS-VM-HOME-05 | `upsert`(置き換え/追加)・`setGenres`/`setSites`(キャッシュへ保存。選択中のジャンルが消えたら「すべて」へ) | `HomeViewModelTests.testUpsert*`・`testSetGenresAndSites*`・`testSelectedGenreResets*` |
| AC-IOS-VM-HOME-06 | 削除:成功で一覧とキャッシュから消す。失敗したら一覧は変えず `errorMessage` | `HomeViewModelTests.testDelete*` |
| AC-IOS-VM-SHEET-01 | 詳細シート:サイト行はジャンルの siteIDs 順・option 空の空白詰め・通信なしで常に出る | `ItemDetailViewModelTests.testLinks*`・`testOptionless*` |
| AC-IOS-VM-SHEET-02 | 検索ワードのその場編集は一時的:開始で `displayQuery` で初期化、編集中はリンクが入力値で変わる、空ならテンプレートに戻る、`cancel` で破棄、保存まで API を呼ばない | `ItemDetailViewModelTests.testBeginEditing*`・`testLinksReflectTheDraft*`・`testEmptyDraft*`・`testCancel*` |
| AC-IOS-VM-SHEET-03 | 「保存」で `PATCH {query_override}` のみ(trim。空・空白だけは `.clear` = null)、成功で `item` 更新・編集終了、失敗は `errorMessage` で編集中の値とリンクを残す | `ItemDetailViewModelTests.testSave*`・`testSaving*` |
| AC-IOS-VM-SHEET-04 | サマリ:取得中は空、`sites` が空なら「まだ価格情報はありません」、通信できなければ「オフライン」、401 などはオフラインと言わない | `ItemDetailViewModelTests.testSummary*` |
| AC-IOS-VM-REG-01 | 登録:ジャンルの初期値(チップ → 先頭)、「登録」できる条件(名前・ジャンル・画像)、条件を満たさなければ API を呼ばない | `RegisterViewModelTests.testInitialGenre*`・`testCanRegister*`・`testRegisterDoesNothingWhenNotReady` |
| AC-IOS-VM-REG-02 | 写真は multipart、URL は from-url → 名前とジャンルが入る → 確認(名前を直せる)→ JSON(image_url)で登録。写真と下書きの両方なら写真(source_url は下書きの)。画像の無い下書きは写真が要る | `RegisterViewModelTests.testRegisterWithPhoto*`・`testFetchDraft*`・`testRegisterFromDraft*`・`testDraftWithoutImage*`・`testPhotoWins*` |
| AC-IOS-VM-REG-03 | 取得・登録に失敗したら `errorMessage` を出し、入力は残して再試行できる | `RegisterViewModelTests.testFetchDraftFailure*`・`testRegisterFailure*` |
| AC-IOS-VM-EDIT-01 | 編集:元の値でフォームを初期化、変えた項目だけ PATCH、前後の空白だけの変更は無視、何も変えなければ API を呼ばない | `EditItemViewModelTests`(初期化・差分) |
| AC-IOS-VM-EDIT-02 | 空にした option・検索ワード上書き・最低価格は元が値ありのときだけ `.clear`(null)。元も空なら送らない | `EditItemViewModelTests.testClearing*`・`testStayingEmpty*` |
| AC-IOS-VM-EDIT-03 | 検証:名前が空・最低価格が 0 以上の整数でない(abc・負・小数)なら送らず `errorMessage` | `EditItemViewModelTests.testInvalid*`・`testZeroMinPriceIsValid` |
| AC-IOS-VM-EDIT-04 | 画像の差し替えは PUT。PATCH と両方なら PATCH → PUT。失敗は入力を残す | `EditItemViewModelTests.testReplacing*`・`testPatchThenPut*`・`testFailure*` |
| AC-IOS-VM-SET-01 | 設定:`movingUp`(上へ)・`sortedGenres` | `SettingsListViewModelTests.testMovingUp`・`testSortedGenres*` |
| AC-IOS-VM-SET-02 | サイトの追加・編集:`{q}` を含む http(s) だけ(違えば API を呼ばず `{q}` を含むメッセージ)。編集は変えた項目だけ・変更なしは呼ばない。追加は全項目を明示して送る | `SettingsListViewModelTests.testAddSite*`・`testUpdateSite*` |
| AC-IOS-VM-SET-03 | ジャンルの追加・編集:名前が空なら呼ばない、空のテンプレートは送らない(既定)、`siteIDs` は順序込みで違うときだけ全件置き換えで送る、変更なしは呼ばない | `SettingsListViewModelTests.testAddGenre*`・`testUpdateGenre*` |
| AC-IOS-VM-SET-04 | API の失敗は `errorMessage` で一覧は変えない | `SettingsListViewModelTests.testAPIFailure*` |
| AC-IOS-SHARE-01 | `SharedURL.extract`:テキストから最初の http(s) URL(括弧・句読点・改行を含めない) | `SharedURLTests` |
| AC-IOS-SHARE-02 | `ShareViewModel`:設定が無ければ `.needsSettings`(API を呼ばない)。URL が無い・http(s) でなければ `.failed`(API を呼ばない) | `ShareViewModelTests.testNeedsSettings*`・`testFailsWithoutAnAPICall*` |
| AC-IOS-SHARE-03 | `load` で `listGenres` と `draftFromURL` を呼び、名前・ジャンル(下書き → 先頭)・プレビュー画像(http(s) だけ)を持って `.ready`。失敗は `.failed` | `ShareViewModelTests.testLoad*`・`testDraftGenre*`・`testDraftFetchFailure*`・`testOffline*`・`testNonHTTPPreviewImage*` |
| AC-IOS-SHARE-04 | `save`:下書きの image_url を JSON で登録(編集した名前・ジャンルを使う)→ `.saved`。画像の無い下書き・空の名前は保存できない。失敗は入力を残して再保存できる | `ShareViewModelTests.testSave*`・`testEdited*`・`testDraftWithoutImage*`・`testBlankName*` |
| AC-IOS-FIX-01 | `WishlistFixtures`(モック起動用):サイト 2(メルカリ=headless・Amazon。仕様 §5 の確認済み URL だけ)、ジャンル 2、商品 2(グリス・ボルシャック)。参照整合・画像パスの形式。グリスのリンクがメルカリと Amazon の検索 URL(§12 の完了条件) | `WishlistFixturesTests` |
| AC-IOS-UI-00 | 数値:グリッド 3 列、長押し 0.5 秒、サマリ文言 | `WishlistURLsTests.testUIConstants` |
| AC-IOS-UI-01〜09 | **XC** 画像のみのグリッド / ジャンルチップで絞り込み / タップで詳細シート(サイト行がジャンルの順・サマリ)/ 検索ワードの一時編集(閉じれば破棄)/ 長押しで編集・削除メニュー(シートは開かない)/ 削除は確認のあと / 「+」で登録シート / 機内モード相当で一覧とサイト行 / トークン未設定は設定画面から | `WishlistUITests`(下記の取り決め) |
| AC-IOS-BUILD-01 | `make wishlist-ios-gen`・`wishlist-ios-gen-check`・`wishlist-ios-test`・`wishlist-ios-xcode-test`・`wishlist-ios-sync-testdata` がある。`wishlist-ios-xcode-test` は Xcode が無ければ「Xcode が要る」を表示して終了コード 2。`wishlist-test` に iOS を含めない | `make -n`・`DEVELOPER_DIR=/Library/Developer/CommandLineTools make wishlist-ios-xcode-test`(spec-writer が確認済み) |

### XCUITest の取り決め(View が守る)

- 起動:`WISHLIST_USE_FAKE=1`(通信しない。`WishlistFixtures` を入れた `FakeWishlistService` + `InMemoryWishlistCache`、設定済みの接続設定)/ `WISHLIST_USE_FAKE=offline`(SwiftData のキャッシュに `WishlistFixtures` があり、Service は通信できない)/ 環境変数なし(本物の設定。未設定なら設定画面から)。
- `accessibilityIdentifier`:`itemGrid` / `item-<id>` / `genreChip-all`・`genreChip-<id>` / `addButton` / `settingsButton` / `menuEdit`・`menuDelete` / `deleteConfirm` / `detailSheet`・`closeSheet`・`queryEditButton`(ラベル = 現在の検索ワード)・`queryField`・`querySaveButton`・`summaryText`(ラベル = サマリの文言)・`siteRow-<siteID>` / `registerSheet` / `settingsScreen`・`connectionURLField`・`connectionTokenField`。
- フィクスチャ:ジャンル 1 S.H.Figuarts(サイト [1 メルカリ, 2 Amazon])・ジャンル 2 デュエマ(サイト [2, 1])、商品 12 グリス(ジャンル 1)・11 ボルシャック(ジャンル 2・option 銀トレジャー)。
- 画像のみ:グリッドの各商品は画像だけ(文字を出さない。VoiceOver 用の accessibilityLabel に名前を入れるのは可)。

## View の要件(自動テストしにくいもの。implementer が守り、人が目視する)

- ホーム:画像のみの 3 列グリッド、ジャンルのチップ(スクロールで隠れてよい)、長押しで編集・削除、右下に「+」、設定への入口。下から出る詳細シート(`presentationDetents`)。
- サイト行は `UIApplication.shared.open(url)`(ユニバーサルリンク。メルカリ・Amazon のアプリが入っていればアプリ側の検索が開く)。`Deeplink.isHTTPURL` を通った URL だけ。
- 見た目は Liquid Glass 系(`glassEffect` など)。ダメ計の `ios/PokeCalc` の View の書き方(View は描くだけ・ViewModel を `@State`・Preview)を参考にする。常時動くアニメーションを入れない(演出は操作時のみ)。
- 画像は `ImageFileCache` 経由で出す(取れなければ色付きのプレースホルダー。画像が無くても成立すること)。
- 一覧データはキャッシュ(SwiftData)を先に出し、取得できたら置き換える(起動時に空白を見せない)。
- Share Extension:`SharedURL.extract` → `ShareViewModel`。OGP のプレビュー(名前・画像)・ジャンル選択・保存、トークン未設定なら設定画面(`ConnectionSettingsViewModel`)。

## Xcode が要る未検証項目(spec-writer は検証していない)

テストのうち `xcodebuild` が必要なのは XCUITest だけで、スタブ View に対して「全件失敗する」ことまで確認済み(`xcodebuild test`、シミュレータ iPhone 17e)。次は実装後に人が確認する。

| 項目 | 確認方法 |
|---|---|
| 署名(Personal Team)・実機インストール | README の手順 1〜2 |
| Share Extension が共有シートに出る・Safari の公式ページから登録できる | README の手順 5(`NSExtensionActivationRule`: Web URL 1 件とテキスト) |
| 拡張から API へ直接 POST できる(ATS: `NSAllowsLocalNetworking`。Tailscale の IP・MagicDNS 名) | 実機で登録。http の MagicDNS 名で ATS に弾かれるなら Info.plist の例外を見直す |
| ユニバーサルリンクでメルカリ・Amazon のアプリが開く | 実機でサイト行をタップ |
| Liquid Glass の見た目・ダークモード・文字サイズ・セーフエリア | シミュレータと実機の目視 |
| 機内モード | 実機で起動して一覧・画像・サイト行 |
| `make wishlist-ios-xcode-test` が全件成功 | Xcode 27 のある Mac で実行 |
