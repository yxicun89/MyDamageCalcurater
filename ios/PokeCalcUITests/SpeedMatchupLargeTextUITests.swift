import XCTest

/// I-speed-4c: 新しい素早さ画面(SpeedMatchupUITests と同じ識別子)の最大の文字サイズ(AX5)。
/// LargeTextLayoutUITests の素早さ表前提の 3 本(`testSpeedScreenNoHorizontalOverflowAtAX5` /
/// `testSpeedScreenResultAndTableNoHorizontalOverflowAtAX5` / `testSpeedPokemonSheetNoHorizontalOverflowAtAX5`)の置き換え。
/// 見るのは「横にはみ出さないこと」と「固定高にしないこと」(文言が長くなるぶん高さが伸びる)だけ。
/// モックの数値・文言には依存しない(SpeedMatchupUITests が見る)。
///
/// 文字サイズは起動引数 `-UIPreferredContentSizeCategoryName` で固定する(LargeTextLayoutUITests と同じ)。
/// 入力側と出力側の 2 本に分け、各々が既定の 300 秒に収まるようにする(識別子ごとに探し直さず、上から 1 周だけ走査する)。
@MainActor
final class SpeedMatchupLargeTextUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrolls = 30
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let defaultContentSizeCategory = "UICTContentSizeCategoryL"
    private static let overflowTolerance: CGFloat = 1
    private static let mockSelf = "9001-000"
    private static let mockOpponent = "9004-000"

    /// 入力側。長い日本語(「追い風(相手側)」「こだわりスカーフ」「トリックルーム」「今の振りを入れる」など)を持つもの。
    private static let inputIdentifiers = [
        "speedScreen", "speedPokemonButton", "speedOpponentPokemonButton", "speedScarf", "speedSelfTailwind", "speedSelfParalysis",
        "speedOpponentTailwind", "speedOpponentParalysis", "speedTrickRoom", "speedCurrentToggle",
        "speedAssumption-uninvested", "speedAssumption-max-scarf", "speedAssumption-max-plus2",
    ]
    /// 出力側(トリックルーム + 今の振りで、文言が最も長くなる状態)。
    private static let outputIdentifiers = [
        "speedCard-uninvested", "speedCardHeadline-uninvested", "speedCardOpponentSpeed-uninvested",
        "speedCardNature-uninvested-minus", "speedCardNature-uninvested-neutral", "speedCardOrder-uninvested",
        "speedReach", "speedReachRow-uninvested", "speedReachSlower-uninvested", "speedReachTie-uninvested", "speedReachFaster-uninvested",
    ]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 補助

    private func launchSpeedScreen(contentSizeCategory: String) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        app.launch()
        let open = element(app, "openSpeedScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openSpeedScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout), "speedScreen が開かない")
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) {
            app.swipeUp(velocity: .slow)
        }
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) {
            app.swipeDown(velocity: .slow)
        }
    }

    private func tap(_ app: XCUIApplication, _ identifier: String) {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い")
        scrollUntilHittable(app, target)
        target.tap()
    }

    private func pick(_ app: XCUIApplication, button: String, pokemonID: String) {
        tap(app, button)
        XCTAssertTrue(element(app, "speedPokemonSheet").waitForExistence(timeout: Self.existenceTimeout), "ポケモンのシートが開かない")
        let row = element(app, "speedPokemonRow-\(pokemonID)")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout), "speedPokemonRow-\(pokemonID) が無い")
        row.tap()
        XCTAssertTrue(element(app, "speedPokemonSheet").waitForNonExistence(timeout: Self.existenceTimeout), "シートが閉じない")
    }

    /// 同じ identifier の要素をすべて集める(コンテナが子の identifier を飲む既知の挙動に備えて 1 件に決め打たない)。
    private func matches(_ app: XCUIApplication, _ identifier: String) -> [XCUIElement] {
        let query = app.descendants(matching: .any).matching(identifier: identifier)
        return (0..<query.count).map { query.element(boundBy: $0) }
    }

    /// 画面の一番上へ戻す(自分のポケモンのボタンが見えるまで速く下へ送る)。シートの中では使わない。
    private func scrollToTop(_ app: XCUIApplication) {
        let top = element(app, "speedPokemonButton")
        for _ in 0..<Self.maxScrolls where !(top.exists && top.isHittable) {
            app.swipeDown()
        }
    }

    /// 上端から 1 歩ずつ swipeUp で進め、見えている identifier を全部検査して未確認の集合から消す(1 周だけ走査する)。
    /// 見えた要素ごとに、画面の外へ左右にはみ出していないことを確かめる。最後まで見えなかった identifier は失敗にする。
    private func assertNoHorizontalOverflow(
        _ app: XCUIApplication, identifiers: [String], startFromTop: Bool = true,
        file: StaticString = #filePath, line: UInt = #line
    ) {
        let windowFrame = app.windows.firstMatch.frame
        XCTAssertGreaterThan(windowFrame.width, 0, "ウィンドウの frame が取得できない", file: file, line: line)
        if startFromTop { scrollToTop(app) }
        var pending = identifiers
        var offenders: [String] = []
        for step in 0...Self.maxScrolls {
            for identifier in pending where element(app, identifier).exists && element(app, identifier).isHittable {
                for (index, found) in matches(app, identifier).enumerated() where found.exists {
                    let frame = found.frame
                    if frame.width == 0 && frame.height == 0 { continue }
                    if frame.minX < -Self.overflowTolerance || frame.maxX > windowFrame.maxX + Self.overflowTolerance {
                        offenders.append("\(identifier)[\(index)]: frame=\(frame) window=\(windowFrame) label=\(found.label)")
                    }
                }
                pending.removeAll { $0 == identifier }
            }
            if pending.isEmpty || step == Self.maxScrolls { break }
            app.swipeUp(velocity: .slow)
        }
        XCTAssertTrue(pending.isEmpty, "画面に出なかった要素: \(pending)", file: file, line: line)
        XCTAssertTrue(offenders.isEmpty, "横にはみ出している要素があります:\n" + offenders.joined(separator: "\n"), file: file, line: line)
    }

    private func openFullState(_ app: XCUIApplication) {
        pick(app, button: "speedPokemonButton", pokemonID: Self.mockSelf)
        pick(app, button: "speedOpponentPokemonButton", pokemonID: Self.mockOpponent)
        tap(app, "speedCurrentToggle")
        tap(app, "speedCurrentPreset-max")
        tap(app, "speedTrickRoom")
        XCTAssertTrue(element(app, "speedCardNature-uninvested-minus").waitForExistence(timeout: Self.existenceTimeout), "トリックルーム中の下降の行が出ない")
        XCTAssertTrue(element(app, "speedReachRow-uninvested").waitForExistence(timeout: Self.existenceTimeout), "「これ以上」が出ない")
    }

    // MARK: - AX5

    /// AX5 で、入力欄とピッカーのシートが横にはみ出さない。
    func testSpeedMatchupInputNoHorizontalOverflowAtAX5() {
        let app = launchSpeedScreen(contentSizeCategory: Self.ax5ContentSizeCategory)
        assertNoHorizontalOverflow(app, identifiers: Self.inputIdentifiers)

        // ピッカーのシート(自分・相手で共通)。シートの中は最初から見えているので、上へ戻す操作はしない(閉じてしまうため)。
        tap(app, "speedOpponentPokemonButton")
        XCTAssertTrue(element(app, "speedPokemonSheet").waitForExistence(timeout: Self.existenceTimeout), "シートが開かない")
        assertNoHorizontalOverflow(
            app, identifiers: ["speedPokemonSheet", "speedPokemonSearchField", "speedPokemonRow-\(Self.mockOpponent)"],
            startFromTop: false)
    }

    /// AX5 で、カードと「これ以上」が横にはみ出さず、固定高にならない(文言が長いと高さが伸びる)。
    func testSpeedMatchupOutputNoHorizontalOverflowAndGrowsTallerAtAX5() {
        // 既定の文字サイズでの「最低ラインのカード」の高さ(固定高でないことの比較の基準)。
        let normalApp = launchSpeedScreen(contentSizeCategory: Self.defaultContentSizeCategory)
        pick(normalApp, button: "speedPokemonButton", pokemonID: Self.mockSelf)
        pick(normalApp, button: "speedOpponentPokemonButton", pokemonID: Self.mockOpponent)
        let normalCard = element(normalApp, "speedCard-uninvested")
        XCTAssertTrue(normalCard.waitForExistence(timeout: Self.existenceTimeout), "カードが出ない")
        let normalHeight = normalCard.frame.height
        normalApp.terminate()

        let app = launchSpeedScreen(contentSizeCategory: Self.ax5ContentSizeCategory)
        openFullState(app)
        assertNoHorizontalOverflow(app, identifiers: Self.outputIdentifiers)

        let ax5Card = element(app, "speedCard-uninvested")
        scrollUntilHittable(app, ax5Card)
        XCTAssertGreaterThan(ax5Card.frame.height, normalHeight, "AX5 ではカードの高さが伸びる(固定高にしない)")
    }
}
