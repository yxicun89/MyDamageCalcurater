import XCTest

/// フェーズ2の XCUITest(docs/phase2-ios-spec.md の AC-IOS-UI-*)。シミュレータでだけ動く(Xcode が要る)。
///
/// 起動の取り決め:
/// - `WISHLIST_USE_FAKE=1`: 通信しない。WishlistFixtures を入れた FakeWishlistService と InMemoryWishlistCache、設定済みの接続設定で起動する。
/// - `WISHLIST_USE_FAKE=offline`: 保存済み(SwiftData のキャッシュに WishlistFixtures)があり、Service は通信できない状態で起動する(機内モード相当)。
/// - 環境変数なし: Info.plist(ビルド設定)の接続先を使う。接続先が無ければ保存済みだけで動く(「オフライン」の札)。
///
/// accessibilityIdentifier の取り決め(View はこれを付ける):
/// `itemGrid`(グリッド)/ `item-<id>`(商品)/ `genreChip-all`・`genreChip-<id>` / `addButton` / `settingsButton` /
/// `menuEdit`・`menuDelete`(長押しメニュー)/ `deleteConfirm`(削除確認の「削除する」)/
/// `detailSheet`・`closeSheet`・`queryEditButton`(ラベルは現在の検索ワード)・`queryField`・`querySaveButton`・`summaryText`・`siteRow-<siteID>`(サイトのタイル)/
/// `registerSheet` / `settingsScreen` / `offlineBadge`(ホームの「オフライン」の札)。
/// フィクスチャ: ジャンル 1 S.H.Figuarts(サイト [1 メルカリ, 2 Amazon])と 2 デュエマ(サイト [2, 1])、商品 12 グリス(ジャンル 1)と 11 ボルシャック(ジャンル 2)。
@MainActor
final class WishlistUITests: XCTestCase {
    private static let timeout: TimeInterval = 5

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(fake: String?) -> XCUIApplication {
        let app = XCUIApplication()
        if let fake { app.launchEnvironment["WISHLIST_USE_FAKE"] = fake }
        app.launch()
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func requireExists(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) -> XCUIElement {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.timeout), "\(identifier) が無い", file: file, line: line)
        return target
    }

    /// 一覧(Form)は遅延して作られるので、画面の外にある項目は上へスクロールして出す(5-3: ジャンル編集の項目が増えた)。
    private func revealByScrolling(_ app: XCUIApplication, _ identifier: String) {
        for _ in 0..<8 where !element(app, identifier).exists || !element(app, identifier).isHittable {
            app.swipeUp()
        }
    }

    /// AC-IOS-UI-01: 画像のグリッドに全商品が出る。
    func testGridShowsEveryItem() {
        let app = launch(fake: "1")
        _ = requireExists(app, "itemGrid")
        _ = requireExists(app, "item-12")
        _ = requireExists(app, "item-11")
        _ = requireExists(app, "addButton")
        _ = requireExists(app, "settingsButton")
    }

    /// AC-IOS-UI-02: ジャンルのチップで絞り込める。
    func testGenreChipFiltersTheGrid() {
        let app = launch(fake: "1")
        _ = requireExists(app, "item-12")
        requireExists(app, "genreChip-2").tap()
        _ = requireExists(app, "item-11")
        XCTAssertFalse(element(app, "item-12").exists, "デュエマに絞ると S.H.Figuarts の商品は出ない")
        requireExists(app, "genreChip-all").tap()
        _ = requireExists(app, "item-12")
    }

    /// AC-6-2-3: 選ばれているジャンルのボタンだけ名前が見える(`genreName-<id>` / `genreName-all`)。選び直すと移る。
    func testOnlySelectedGenreChipShowsItsName() {
        let app = launch(fake: "1")
        _ = requireExists(app, "item-12")
        XCTAssertTrue(element(app, "genreName-all").exists, "最初は「すべて」だけ名前が出る")
        XCTAssertFalse(element(app, "genreName-2").exists)
        XCTAssertFalse(element(app, "genreName-1").exists)
        requireExists(app, "genreChip-2").tap()
        _ = requireExists(app, "genreName-2")
        XCTAssertFalse(element(app, "genreName-all").exists, "選び直したら前の名前は消える")
        XCTAssertFalse(element(app, "genreName-1").exists)
        XCTAssertEqual(element(app, "genreChip-2").label, "デュエマ", "名前が出ても accessibilityLabel はジャンル名のまま")
        XCTAssertEqual(element(app, "genreChip-1").label, "S.H.Figuarts", "名前が出ていないボタンにも accessibilityLabel はある")
    }

    /// AC-IOS-UI-03: タップで詳細シートが出て、サイトのタイルがジャンルの順に並ぶ。サマリは「まだ価格情報はありません」。
    func testTapOpensDetailSheetWithSiteRowsInGenreOrder() {
        let app = launch(fake: "1")
        requireExists(app, "item-12").tap()
        _ = requireExists(app, "detailSheet")
        let mercari = requireExists(app, "siteRow-1")
        let amazon = requireExists(app, "siteRow-2")
        // 5-3(phase5-spec): サイトのタイルは 3 列のグリッドなので、同じ行で左から メルカリ → Amazon の順(旧: 縦に並ぶ行の minY)
        XCTAssertLessThan(mercari.frame.minX, amazon.frame.minX, "S.H.Figuarts のサイトはメルカリ → Amazon の順")
        XCTAssertEqual(requireExists(app, "summaryText").label, "まだ価格情報はありません")
        XCTAssertTrue(requireExists(app, "queryEditButton").label.contains("S.H.Figuarts グリス"))
    }

    /// AC-IOS-UI-04: 検索ワードのその場編集は、「保存」を押すまでは一時的(閉じれば破棄される)。
    func testQueryEditIsDiscardedWhenTheSheetIsClosedWithoutSaving() {
        let app = launch(fake: "1")
        requireExists(app, "item-12").tap()
        requireExists(app, "queryEditButton").tap()
        let field = requireExists(app, "queryField")
        field.tap()
        field.typeText(" 変身ベルト")
        requireExists(app, "closeSheet").tap()
        XCTAssertFalse(element(app, "detailSheet").waitForExistence(timeout: 1))

        requireExists(app, "item-12").tap()
        XCTAssertTrue(requireExists(app, "queryEditButton").label.contains("S.H.Figuarts グリス"))
        XCTAssertFalse(requireExists(app, "queryEditButton").label.contains("変身ベルト"))
    }

    /// AC-IOS-UI-05: 長押し(0.5 秒)で編集・削除メニューが出る。離してもシートは開かない。
    func testLongPressShowsEditAndDeleteWithoutOpeningTheSheet() {
        let app = launch(fake: "1")
        requireExists(app, "item-12").press(forDuration: 0.8)
        // 負荷が高いときにメニューの表示が遅れて落ちたので、ここだけ待ち時間を長くする(5-3)
        XCTAssertTrue(element(app, "menuEdit").waitForExistence(timeout: 20), "menuEdit が無い")
        XCTAssertTrue(element(app, "menuDelete").waitForExistence(timeout: 20), "menuDelete が無い")
        XCTAssertFalse(element(app, "detailSheet").exists)
    }

    /// AC-IOS-UI-06: 削除は確認のあとだけ。
    func testDeleteNeedsConfirmation() {
        let app = launch(fake: "1")
        requireExists(app, "item-12").press(forDuration: 0.8)
        requireExists(app, "menuDelete").tap()
        requireExists(app, "deleteConfirm").tap()
        XCTAssertTrue(waitForDisappearance(element(app, "item-12")))
        _ = requireExists(app, "item-11")
    }

    /// AC-IOS-UI-07: 右下の「+」で登録シートが出る。
    func testAddButtonOpensTheRegisterSheet() {
        let app = launch(fake: "1")
        requireExists(app, "addButton").tap()
        _ = requireExists(app, "registerSheet")
    }

    /// AC-IOS-UI-08(機内モード相当): 通信できなくても保存済みの一覧が出て、サイト行(検索ボタン)は出る。サマリは「オフライン」。
    func testOfflineLaunchShowsSavedListAndLinks() {
        let app = launch(fake: "offline")
        requireExists(app, "item-12").tap()
        _ = requireExists(app, "siteRow-1")
        _ = requireExists(app, "siteRow-2")
        XCTAssertEqual(requireExists(app, "summaryText").label, "オフライン")
    }

    /// AC-IOS-UI-09(置換。ユーザー決定 2026-10-11 / phase5-spec 5-3): 接続先(ビルド設定)が無くても設定画面を挟まずホームが出る。
    /// 保存済みだけで動き、「オフライン」の札が出る。設定に「接続」の欄は無い。
    /// (旧: トークン未設定なら設定画面から始まり、接続の URL・トークンの欄があった。)
    func testLaunchWithoutBaseURLShowsHomeWithOfflineBadgeAndNoConnectionFields() {
        let app = launch(fake: nil)
        _ = requireExists(app, "itemGrid")
        _ = requireExists(app, "offlineBadge")
        XCTAssertFalse(element(app, "settingsScreen").exists, "設定画面から始めない")
        requireExists(app, "settingsButton").tap()
        _ = requireExists(app, "settingsScreen")
        XCTAssertFalse(element(app, "connectionURLField").exists, "接続の URL の欄は無い")
        XCTAssertFalse(element(app, "connectionTokenField").exists, "トークンの欄は無い")
    }

    // MARK: - フェーズ3(目安価格。docs/phase3-ios-spec.md の AC-IOS-EST-UI-*)
    // `WISHLIST_USE_FAKE=estimates`(目安価格つき。商品 12 はメルカリ = ok・Amazon = no_result、商品 11 は両方 no_result)。
    // 追加の accessibilityIdentifier: `refreshButton`(ラベル「更新」)/ `suspiciousDisclosure`(ラベル「あやしい出品 N」。5-3 で「参考外 N件」から置換)/
    // `suspiciousListing-<出品 ID>`(ラベルにタイトル・価格・理由)。`siteRow-<siteID>` のラベルは `SiteRow.accessibilityLabel`
    // (サイト名 + 目安・件数・在庫・注記)。

    /// AC-IOS-EST-UI-01: 目安のあるサイトは金額・件数・在庫、no_result のサイトは「出品ないかも」だけ(金額なし・リンクは残る)。
    /// あやしい出品(旧「参考外」)はボタンで、押すと同じシートの中身が理由つきの出品の一覧に替わる。
    func testEstimateRowsShowPricesAndNoListingsAndSuspiciousDisclosure() {
        let app = launch(fake: "estimates")
        requireExists(app, "item-12").tap()
        XCTAssertEqual(
            requireExists(app, "summaryText").label, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)")
        let mercari = requireExists(app, "siteRow-1").label
        XCTAssertTrue(mercari.contains("¥3,000〜¥4,500"), mercari)
        XCTAssertTrue(mercari.contains("5件"), mercari)
        XCTAssertTrue(mercari.contains("在庫あり"), mercari)
        let amazon = requireExists(app, "siteRow-2").label
        XCTAssertTrue(amazon.contains("出品ないかも"), amazon)
        XCTAssertFalse(amazon.contains("¥"), "no_result は金額を出さない: \(amazon)")
        XCTAssertFalse(amazon.contains("件"), "no_result は件数を出さない: \(amazon)")
        XCTAssertFalse(amazon.contains("在庫"), "no_result は在庫を出さない: \(amazon)")
        XCTAssertTrue(requireExists(app, "refreshButton").isEnabled)

        let disclosure = requireExists(app, "suspiciousDisclosure")
        XCTAssertTrue(disclosure.label.contains("あやしい出品 1"), disclosure.label)  // 5-3: 旧「参考外 1件」
        XCTAssertFalse(element(app, "suspiciousListing-102").exists, "押すまでは出品を出さない")
        disclosure.tap()
        let listing = requireExists(app, "suspiciousListing-102")
        XCTAssertTrue(listing.label.contains("¥300"), listing.label)
        XCTAssertTrue(listing.label.contains("名前が違う"), listing.label)
        XCTAssertTrue(listing.label.contains("安すぎ"), listing.label)
        XCTAssertFalse(element(app, "suspiciousListing-101").exists, "参考にしている出品は出さない")
    }

    /// AC-IOS-EST-UI-02: すべて no_result なら、サマリは「出品ないかも」だけ(金額なし)。検索へのリンクは全サイト残り、参考外の欄は出ない。
    func testAllNoResultShowsNoListingsSummaryAndKeepsLinks() {
        let app = launch(fake: "estimates")
        requireExists(app, "item-11").tap()
        XCTAssertEqual(requireExists(app, "summaryText").label, "出品ないかも")
        XCTAssertTrue(requireExists(app, "siteRow-1").label.contains("出品ないかも"))
        XCTAssertTrue(requireExists(app, "siteRow-2").label.contains("出品ないかも"))
        XCTAssertFalse(element(app, "suspiciousDisclosure").exists, "あやしい出品の合計が 0 なら出さない")
    }

    // MARK: - フェーズ4-2(価格の推移。docs/phase4-spec.md の AC-IOS-HIS-UI-01)
    // 追加の accessibilityIdentifier: `historyDisclosure`(ラベル「価格の推移」)/ `priceHistoryChart`(Swift Charts。ラベルは要約)/
    // `historySite-<siteID>`(凡例のボタン。ラベルはサイト名、値は「表示中」/「非表示」)/ `historyEmpty`(ラベル「推移はまだありません」)。
    // フィクスチャ: 商品 12 はメルカリの 3 日分(10/1 ¥3,200・10/2 ¥3,000・10/3 ¥3,000)、商品 11 は推移なし。

    /// AC-IOS-HIS-UI-01: 折りたたみを開くと推移のグラフ(要約つき)と凡例が出る。凡例でサイトの線を切り替えられる。推移の無い商品は「推移はまだありません」。
    func testPriceHistoryChartAndEmptyState() {
        let app = launch(fake: "estimates")
        requireExists(app, "item-12").tap()
        let disclosure = requireExists(app, "historyDisclosure")
        XCTAssertTrue(disclosure.label.contains("価格の推移"), disclosure.label)
        XCTAssertFalse(element(app, "priceHistoryChart").exists, "開くまではグラフを出さない")
        disclosure.tap()
        XCTAssertEqual(requireExists(app, "priceHistoryChart").label, "価格の推移 10/1〜10/3 最安 ¥3,000 最高 ¥3,200")
        let mercari = requireExists(app, "historySite-1")
        XCTAssertEqual(mercari.label, "メルカリ")
        XCTAssertEqual(mercari.value as? String, "非表示")
        mercari.tap()
        XCTAssertEqual(element(app, "historySite-1").value as? String, "表示中")
        XCTAssertFalse(element(app, "historySite-2").exists, "推移の無いサイトは凡例に出さない")

        requireExists(app, "closeSheet").tap()
        requireExists(app, "item-11").tap()
        requireExists(app, "historyDisclosure").tap()
        XCTAssertEqual(requireExists(app, "historyEmpty").label, "推移はまだありません")
        XCTAssertFalse(element(app, "priceHistoryChart").exists)
    }

    // MARK: - フェーズ4-3・4-1(docs/phase4-spec.md の AC-IOS-OFF-UI-01・AC-IOS-ALI-UI-01)
    // `WISHLIST_USE_FAKE=official`(WishlistFixtures.makeServiceWithOfficial。キャッシュも officialItems・officialGenres):
    // 商品 12 グリスは監視中(予約受付中・根拠「予約受付中」「予約する」・10/4 確認)、11 は監視しない。ジャンル 1 に別名グループ 1 つ。
    // 追加の accessibilityIdentifier: 詳細シートの `officialStatus`(ラベル = summary)・`officialEvidence`・`officialChange`・`officialLastAttempt`、
    // 編集の `watchOfficialToggle`(値は "1" / "0")・`watchOfficialHint`、ジャンル編集の `aliasLine-<N>`(1 始まりのテキスト欄)・
    // `addAliasLine`(別名グループを追加)・`removeAliasLine-<N>`、保存は既存の「保存」ボタン。

    /// AC-IOS-OFF-UI-01: 監視中の商品の詳細シートに「公式」の行と根拠が出る。監視していない商品には出ない。編集にトグルがある。
    func testOfficialStatusInDetailSheetAndEditToggle() {
        let app = launch(fake: "official")
        requireExists(app, "item-12").tap()
        XCTAssertEqual(requireExists(app, "officialStatus").label, "公式: 予約受付中(10/4 確認)")
        XCTAssertEqual(requireExists(app, "officialEvidence").label, "根拠: 予約受付中・予約する")
        XCTAssertFalse(element(app, "officialChange").exists, "変化が無ければ出さない")
        requireExists(app, "closeSheet").tap()

        requireExists(app, "item-11").tap()
        _ = requireExists(app, "detailSheet")
        XCTAssertFalse(element(app, "officialStatus").exists, "監視していない商品には出さない")
        requireExists(app, "closeSheet").tap()

        requireExists(app, "item-12").press(forDuration: 0.8)
        requireExists(app, "menuEdit").tap()
        let toggle = requireExists(app, "watchOfficialToggle")
        XCTAssertEqual(toggle.value as? String, "1")
        XCTAssertTrue(toggle.isEnabled)
    }

    /// AC-IOS-ALI-UI-01: 設定のジャンル編集で、別名グループを 1 行ずつ編集・追加して保存でき、開き直すと残っている。
    func testGenreAliasesCanBeEditedInSettings() {
        let app = launch(fake: "official")
        requireExists(app, "settingsButton").tap()
        app.buttons["S.H.Figuartsを編集"].firstMatch.tap()
        // 5-3: ジャンル編集に色・絵柄・手がかりが増え、別名グループは下のほうになったので、スクロールして出す
        revealByScrolling(app, "aliasLine-1")
        XCTAssertEqual(requireExists(app, "aliasLine-1").value as? String, "S.H.Figuarts, SHフィギュアーツ")
        revealByScrolling(app, "addAliasLine")
        requireExists(app, "addAliasLine").tap()
        revealByScrolling(app, "aliasLine-2")
        let line2 = requireExists(app, "aliasLine-2")
        line2.tap()
        line2.typeText("真骨彫, 真骨彫製法")
        app.buttons["保存"].firstMatch.tap()
        XCTAssertTrue(waitForDisappearance(element(app, "aliasLine-1")), "保存したら閉じる")

        app.buttons["S.H.Figuartsを編集"].firstMatch.tap()
        revealByScrolling(app, "aliasLine-2")
        XCTAssertEqual(requireExists(app, "aliasLine-1").value as? String, "S.H.Figuarts, SHフィギュアーツ")
        XCTAssertEqual(requireExists(app, "aliasLine-2").value as? String, "真骨彫, 真骨彫製法")
    }

    /// 別名グループの行を消せる(番号は詰める)。保存して開き直すと、消した行は無い。
    func testGenreAliasLineCanBeRemoved() {
        let app = launch(fake: "official")
        requireExists(app, "settingsButton").tap()
        app.buttons["S.H.Figuartsを編集"].firstMatch.tap()
        revealByScrolling(app, "addAliasLine")
        requireExists(app, "addAliasLine").tap()
        revealByScrolling(app, "aliasLine-2")
        let line2 = requireExists(app, "aliasLine-2")
        line2.tap()
        line2.typeText("真骨彫, 真骨彫製法")
        revealByScrolling(app, "removeAliasLine-1")
        requireExists(app, "removeAliasLine-1").tap()
        XCTAssertEqual(requireExists(app, "aliasLine-1").value as? String, "真骨彫, 真骨彫製法", "消すと番号が詰める")
        XCTAssertTrue(waitForDisappearance(element(app, "aliasLine-2")))
        app.buttons["保存"].firstMatch.tap()
        XCTAssertTrue(waitForDisappearance(element(app, "aliasLine-1")), "保存したら閉じる")
        app.buttons["S.H.Figuartsを編集"].firstMatch.tap()
        revealByScrolling(app, "aliasLine-1")
        XCTAssertEqual(requireExists(app, "aliasLine-1").value as? String, "真骨彫, 真骨彫製法")
        XCTAssertFalse(element(app, "aliasLine-2").exists)
    }

    private func waitForDisappearance(_ target: XCUIElement) -> Bool {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate(format: "exists == false"), object: target)
        return XCTWaiter().wait(for: [expectation], timeout: Self.timeout) == .completed
    }
}
