import XCTest

/// フェーズ2の XCUITest(docs/phase2-ios-spec.md の AC-IOS-UI-*)。シミュレータでだけ動く(Xcode が要る)。
///
/// 起動の取り決め:
/// - `WISHLIST_USE_FAKE=1`: 通信しない。WishlistFixtures を入れた FakeWishlistService と InMemoryWishlistCache、設定済みの接続設定で起動する。
/// - `WISHLIST_USE_FAKE=offline`: 保存済み(SwiftData のキャッシュに WishlistFixtures)があり、Service は通信できない状態で起動する(機内モード相当)。
/// - 環境変数なし: 本物の設定を使う。トークン未設定なら設定画面から始まる。
///
/// accessibilityIdentifier の取り決め(View はこれを付ける):
/// `itemGrid`(グリッド)/ `item-<id>`(商品)/ `genreChip-all`・`genreChip-<id>` / `addButton` / `settingsButton` /
/// `menuEdit`・`menuDelete`(長押しメニュー)/ `deleteConfirm`(削除確認の「削除する」)/
/// `detailSheet`・`closeSheet`・`queryEditButton`(ラベルは現在の検索ワード)・`queryField`・`querySaveButton`・`summaryText`・`siteRow-<siteID>` /
/// `registerSheet` / `settingsScreen`・`connectionURLField`・`connectionTokenField`。
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

    /// AC-IOS-UI-03: タップで詳細シートが出て、サイト行がジャンルの順に並ぶ。サマリは「まだ価格情報はありません」。
    func testTapOpensDetailSheetWithSiteRowsInGenreOrder() {
        let app = launch(fake: "1")
        requireExists(app, "item-12").tap()
        _ = requireExists(app, "detailSheet")
        let mercari = requireExists(app, "siteRow-1")
        let amazon = requireExists(app, "siteRow-2")
        XCTAssertLessThan(mercari.frame.minY, amazon.frame.minY, "S.H.Figuarts のサイトはメルカリ → Amazon の順")
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
        _ = requireExists(app, "menuEdit")
        _ = requireExists(app, "menuDelete")
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

    /// AC-IOS-UI-09: トークン未設定で起動したら設定画面から始める(API を呼ばない)。
    func testLaunchWithoutTokenStartsOnSettings() {
        let app = launch(fake: nil)
        _ = requireExists(app, "settingsScreen")
        _ = requireExists(app, "connectionURLField")
        _ = requireExists(app, "connectionTokenField")
    }

    private func waitForDisappearance(_ target: XCUIElement) -> Bool {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate(format: "exists == false"), object: target)
        return XCTWaiter().wait(for: [expectation], timeout: Self.timeout) == .completed
    }
}
