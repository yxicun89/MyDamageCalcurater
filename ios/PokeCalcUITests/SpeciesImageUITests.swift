import XCTest

/// P8-1c: ポケモン画像の表示(ADR-0508)。モックの画像は `POKECALC_MOCK_IMAGES=1`(小さな架空 PNG の data URL。
/// 9001-000・9003-000 だけ。9002-000 は画像なし)。画像の中身は検査しない。画像の枠は identifier `speciesImage-<key>`
/// (読み込みに成功したときだけ存在。装飾なので label は付けない)で見る。
/// 既定(画像なし)で従来どおり動くこと(AC-X)は、既存の全 XCUITest が無変更で通ること自体と、
/// `testNoImageFramesByDefault` で確かめる。実装前は identifier が無いので画像ありのテストは失敗してよい。
@MainActor
final class SpeciesImageUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(images: Bool, contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if images { app.launchEnvironment["POKECALC_MOCK_IMAGES"] = "1" }
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        let open = app.buttons["openCalcScreen"]
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func imageFrames(_ app: XCUIApplication) -> XCUIElementQuery {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", "speciesImage-"))
    }

    private func openDefenderSearch(_ app: XCUIApplication) {
        let picker = element(app, "defenderSpeciesPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
    }

    /// AC-X: 既定(画像なし)では画像の枠が1つも無く、画面は従来どおり使える(エンブレム+名前)。
    func testNoImageFramesByDefault() {
        let app = launch(images: false)
        XCTAssertTrue(element(app, "attackerSpeciesPicker").exists)
        // 画像の読み込みは非同期なので、少し待っても出ないことを見る。
        XCTAssertFalse(imageFrames(app).firstMatch.waitForExistence(timeout: 1.5))
        openDefenderSearch(app)
        XCTAssertTrue(element(app, "speciesSearchResult-9001-000").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(imageFrames(app).firstMatch.exists)
    }

    /// 画像あり: 計算画面の攻撃側ヘッダー(既定は 9001-000)に画像の枠が出る。名前のテキストは変わらない。
    func testCalcHeaderShowsImageFrameWhenManifestHasKey() {
        let app = launch(images: true)
        XCTAssertTrue(element(app, "speciesImage-9001-000").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "attackerSpeciesPicker").exists, "ヘッダーのボタンはそのまま")
    }

    /// 画像あり: 種族検索の行。manifest にあるキーだけ枠が出て、無いキー(9002-000)の行もエンブレムのまま出る(壊れない)。
    func testSearchRowsShowImageFrameOnlyForKeysInManifest() {
        let app = launch(images: true)
        openDefenderSearch(app)
        for key in ["9001-000", "9002-000", "9003-000"] {
            XCTAssertTrue(element(app, "speciesSearchResult-\(key)").waitForExistence(timeout: Self.existenceTimeout), key)
        }
        XCTAssertTrue(element(app, "speciesImage-9001-000").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "speciesImage-9003-000").exists)
        XCTAssertFalse(element(app, "speciesImage-9002-000").exists, "manifest に無いキーは画像なし")
    }

    /// 画像ありでも選択の挙動は変わらない(行をタップして防御側が変わる)。
    func testSelectingRowStillWorksWithImages() {
        let app = launch(images: true)
        openDefenderSearch(app)
        let row = element(app, "speciesSearchResult-9003-000")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        row.tap()
        XCTAssertTrue(element(app, "defenderCard").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "シートが閉じる")
    }

    /// AX5(最大の文字サイズ)でも画像の枠が画面の横幅に収まり、ヘッダーのボタンが操作できる。
    func testImageFrameFitsScreenAtAX5() {
        let app = launch(images: true, contentSizeCategory: Self.ax5ContentSizeCategory)
        let frame = element(app, "speciesImage-9001-000")
        XCTAssertTrue(frame.waitForExistence(timeout: Self.existenceTimeout))
        let screenWidth = app.windows.firstMatch.frame.width
        XCTAssertGreaterThanOrEqual(frame.frame.minX, -1)
        XCTAssertLessThanOrEqual(frame.frame.maxX, screenWidth + 1, "画像の枠が横にはみ出さない")
        XCTAssertGreaterThan(frame.frame.width, 0)
        XCTAssertLessThanOrEqual(frame.frame.width, screenWidth / 2, "画像は文字サイズに引きずられて巨大化しない")
        XCTAssertTrue(element(app, "attackerSpeciesPicker").isHittable)
    }
}
