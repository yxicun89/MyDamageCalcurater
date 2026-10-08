import XCTest

/// 計算画面でお気に入りを読み込む(ADR-0513。ADR-0501「お気に入りの読み込みの受け入れ条件」)。
/// モックの挙動は環境変数 `POKECALC_MOCK_FAVORITES` で切り替える。
/// - `loadable`: 304「読み込める」9002-000(攻撃上昇・特性/持ち物がマスタにある)・303「一部だけ」9004-000(特性と持ち物がマスタに無い)・
///   302「種族なし」9999-000(マスタに無い)・301 ラベルなし 9003-000。
/// - 未設定=空 / `fail` / `unavailable`。
/// モックのマスタの既定: 攻撃側 9001「テストモンいち」・防御側 9002「テストモンに」。
/// 識別子と架空の key を直接書く(UI テストは App のターゲットに依存しない)。実装前は identifier が無いので失敗してよい。
/// 新しい XCUITest は iPhone 17e と iPhone 18 Pro の両方で通すこと(機種で画面の高さが違う。
/// スクロールは前方に進めて届かなければ下へ戻る)。
@MainActor
final class FavoriteLoadUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let minTapSize: CGFloat = 36
    private static let maxScrollAttempts = 14

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 補助

    private func launchCalc(favorites: String? = "loadable") -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let favorites { app.launchEnvironment["POKECALC_MOCK_FAVORITES"] = favorites }
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

    /// 前方(swipeUp)に進めて届かなければ、通り越した場合のために下(swipeDown)へ戻る。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, container: String = "calcScreen") {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)")
        for _ in 0..<Self.maxScrollAttempts where !(target.exists && target.isHittable) {
            element(app, container).swipeUp()
        }
        for _ in 0..<(Self.maxScrollAttempts * 2) where !(target.exists && target.isHittable) {
            element(app, container).swipeDown()
        }
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)")
    }

    private func openSheet(_ app: XCUIApplication, side: String) {
        let entry = element(app, "\(side)FavoriteSourceButton")
        scrollUntilHittable(app, entry)
        entry.tap()
        XCTAssertTrue(element(app, "favoriteLoadSheet").waitForExistence(timeout: Self.existenceTimeout), "シートが開かない")
    }

    private func tapRow(_ app: XCUIApplication, _ id: String) {
        let row = element(app, "favoriteLoadRow-\(id)")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout), "行が無い: \(id)")
        scrollUntilHittable(app, row, container: "favoriteLoadSheet")
        row.tap()
    }

    private func waitUntilGone(_ target: XCUIElement) {
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    private func waitForLabel(_ target: XCUIElement, equalTo text: String, file: StaticString = #filePath, line: UInt = #line) {
        let predicate = NSPredicate(format: "label == %@", text)
        expectation(for: predicate, evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertEqual(target.label, text, file: file, line: line)
    }

    // MARK: - 入口とシート

    /// 攻撃側・防御側の入口が計算画面にあり、タップ範囲が 36pt 以上。既存の入口(構築から選ぶ・お気に入りに追加)は残る。
    func testEntriesExistWithMinimumTapSizeAndExistingEntriesRemain() {
        let app = launchCalc()
        for side in ["attacker", "defender"] {
            let entry = element(app, "\(side)FavoriteSourceButton")
            scrollUntilHittable(app, entry)
            XCTAssertGreaterThanOrEqual(entry.frame.height, Self.minTapSize, "\(side) 入口の高さ")
            XCTAssertGreaterThanOrEqual(entry.frame.width, Self.minTapSize)
        }
        XCTAssertTrue(element(app, "attackerTeamSourceButton").exists, "既存の「構築から選ぶ」は残る")
        XCTAssertTrue(element(app, "pinAttackerFavoriteButton").exists, "既存の「お気に入りに追加」は残る")
    }

    /// 攻撃側のシート: 注記(技は変わらない)・4件の行(サーバーの順)・閉じるで戻れる。
    func testAttackerSheetShowsNoteAndRowsInServerOrderAndCloses() {
        let app = launchCalc()
        openSheet(app, side: "attacker")

        let note = element(app, "favoriteLoadNote")
        XCTAssertTrue(note.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(note.label.contains("技は変わりません"), "label: \(note.label)")
        let ids = ["304", "303", "302", "301"]
        for id in ids {
            XCTAssertTrue(element(app, "favoriteLoadRow-\(id)").waitForExistence(timeout: Self.existenceTimeout), "行が無い: \(id)")
        }
        let first = element(app, "favoriteLoadRow-304")
        XCTAssertTrue(first.label.contains("読み込める"), "label: \(first.label)")
        XCTAssertTrue(first.label.contains("テストモンに"), "ラベルがあれば種族名も添える: \(first.label)")
        XCTAssertGreaterThanOrEqual(first.frame.height, Self.minTapSize)
        XCTAssertLessThan(first.frame.minY, element(app, "favoriteLoadRow-303").frame.minY, "サーバーの順")
        XCTAssertTrue(element(app, "favoriteLoadRow-301").label.contains("テストモンさん"), "ラベルなしは種族名")

        let close = element(app, "favoriteLoadClose")
        XCTAssertTrue(close.isHittable)
        XCTAssertGreaterThanOrEqual(close.frame.height, Self.minTapSize)
        close.tap()
        waitUntilGone(element(app, "favoriteLoadSheet"))
        XCTAssertTrue(element(app, "calcScreen").exists)
    }

    /// 防御側のシートの注記は「種族と特性だけ」。
    func testDefenderSheetNoteSaysOnlySpeciesAndAbility() {
        let app = launchCalc()
        openSheet(app, side: "defender")
        let note = element(app, "favoriteLoadNote")
        XCTAssertTrue(note.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(note.label.contains("種族と特性だけ"), "label: \(note.label)")
        XCTAssertFalse(note.label.contains("技は変わりません"), "攻撃側の注記と混ざらない")
    }

    // MARK: - 読み込み

    /// 攻撃側に読み込む: シートが閉じ、攻撃側の種族が変わり、プリセットの選択が外れ、案内は出ない。
    func testLoadingIntoAttackerChangesAttackerAndClearsPreset() {
        let app = launchCalc()
        let picker = element(app, "attackerSpeciesPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(picker.label, "テストモンいち")
        let preset = app.buttons["attackerPreset-none"]
        XCTAssertTrue(preset.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(preset.isSelected, "前提: 既定のプリセットが選ばれている")

        openSheet(app, side: "attacker")
        tapRow(app, "304")

        waitUntilGone(element(app, "favoriteLoadSheet"))
        waitForLabel(element(app, "attackerSpeciesPicker"), equalTo: "テストモンに")
        let deselected = NSPredicate(format: "isSelected == false")
        expectation(for: deselected, evaluatedWith: preset, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertEqual(element(app, "defenderSpeciesPicker").label, "テストモンに", "防御側は変わらない(モックの既定は 9002)")
        XCTAssertFalse(element(app, "favoriteLoadNotice").exists, "全部読めたら案内は出ない")
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout), "計算結果が出る")
    }

    /// 防御側に読み込む: 防御側の種族だけが変わる(攻撃側・プリセットは変わらない)。
    func testLoadingIntoDefenderChangesOnlyDefender() {
        let app = launchCalc()
        let preset = app.buttons["attackerPreset-none"]
        XCTAssertTrue(preset.waitForExistence(timeout: Self.existenceTimeout))

        openSheet(app, side: "defender")
        tapRow(app, "301")

        waitUntilGone(element(app, "favoriteLoadSheet"))
        waitForLabel(element(app, "defenderSpeciesPicker"), equalTo: "テストモンさん")
        XCTAssertEqual(element(app, "attackerSpeciesPicker").label, "テストモンいち", "攻撃側は変えない")
        XCTAssertTrue(preset.isSelected, "防御側の読み込みでプリセットは外れない")
        XCTAssertFalse(element(app, "favoriteLoadNotice").exists)
    }

    // MARK: - 読めなかった分の案内

    /// 持ち物と特性がマスタに無いお気に入り: 種族は読み込み、読めなかった項目を案内する。
    func testPartialLoadSetsSpeciesAndNotesDroppedItems() {
        let app = launchCalc()
        openSheet(app, side: "attacker")
        tapRow(app, "303")

        waitUntilGone(element(app, "favoriteLoadSheet"))
        waitForLabel(element(app, "attackerSpeciesPicker"), equalTo: "テストモンよん")
        let notice = element(app, "favoriteLoadNotice")
        scrollUntilHittable(app, notice)
        XCTAssertTrue(notice.label.contains("持ち物"), "label: \(notice.label)")
        XCTAssertTrue(notice.label.contains("特性"), "label: \(notice.label)")
        XCTAssertTrue(notice.label.contains("読み込めた分だけ設定しました"), "label: \(notice.label)")
    }

    /// 防御側は特性だけを使うので、特性が読めなければ案内に特性だけが出る(持ち物は使わないので出ない)。
    func testDefenderPartialLoadNotesAbilityOnly() {
        let app = launchCalc()
        openSheet(app, side: "defender")
        tapRow(app, "303")

        waitUntilGone(element(app, "favoriteLoadSheet"))
        waitForLabel(element(app, "defenderSpeciesPicker"), equalTo: "テストモンよん")
        let notice = element(app, "favoriteLoadNotice")
        scrollUntilHittable(app, notice)
        XCTAssertTrue(notice.label.contains("特性"), "label: \(notice.label)")
        XCTAssertFalse(notice.label.contains("持ち物"), "防御側で使わない項目は案内しない: \(notice.label)")
    }

    /// マスタに無い種族: 何も変えず、理由を案内する。計算結果は消えない。
    func testUnknownSpeciesChangesNothingAndNotes() {
        let app = launchCalc()
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout))
        openSheet(app, side: "attacker")
        tapRow(app, "302")

        waitUntilGone(element(app, "favoriteLoadSheet"))
        let notice = element(app, "favoriteLoadNotice")
        scrollUntilHittable(app, notice)
        XCTAssertTrue(notice.label.contains("マスタに無い"), "label: \(notice.label)")
        XCTAssertTrue(notice.label.contains("何も変えていません"), "label: \(notice.label)")
        XCTAssertEqual(element(app, "attackerSpeciesPicker").label, "テストモンいち", "何も変えない")
        XCTAssertTrue(element(app, "calcResultRow-none@-").exists, "結果の表示は消さない")
        XCTAssertFalse(element(app, "calcErrorMessage").exists, "画面のエラーにしない")
    }

    // MARK: - 取得の失敗・空(計算画面を壊さない。絶対ルール5)

    /// 空のストア: シートに空の案内。計算はそのまま使える。
    func testEmptyStoreShowsEmptyMessage() {
        let app = launchCalc(favorites: nil)
        openSheet(app, side: "attacker")
        let empty = element(app, "favoriteLoadEmpty")
        XCTAssertTrue(empty.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(empty.label.contains("お気に入りはまだありません"), "label: \(empty.label)")
        XCTAssertFalse(element(app, "favoriteLoadError").exists)
    }

    /// 通信失敗: シートの中で案内と再読み込み。閉じれば計算画面は無傷で、攻守入れ替えなど他の操作も動く。
    func testFetchFailureIsContainedInSheetAndCalcStillWorks() {
        let app = launchCalc(favorites: "fail")
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout))
        openSheet(app, side: "attacker")

        let error = element(app, "favoriteLoadError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(error.label.contains("通信に失敗しました"), "label: \(error.label)")
        let retry = element(app, "favoriteLoadRetry")
        XCTAssertTrue(retry.isHittable)
        XCTAssertGreaterThanOrEqual(retry.frame.height, Self.minTapSize)
        retry.tap()
        XCTAssertTrue(element(app, "favoriteLoadError").waitForExistence(timeout: Self.existenceTimeout), "再読み込みしてもまだ失敗(クラッシュしない)")

        element(app, "favoriteLoadClose").tap()
        waitUntilGone(element(app, "favoriteLoadSheet"))
        XCTAssertFalse(element(app, "calcErrorMessage").exists, "計算画面のエラーにしない")
        XCTAssertTrue(element(app, "calcResultRow-none@-").exists)
        let swap = element(app, "swapSidesButton")
        scrollUntilHittable(app, swap)
        swap.tap()
        waitForLabel(element(app, "attackerSpeciesPicker"), equalTo: "テストモンに")
    }

    /// 503(保存先の障害): 「計算はそのまま使えます」と案内する。
    func testUnavailableStoreExplainsCalcStillWorks() {
        let app = launchCalc(favorites: "unavailable")
        openSheet(app, side: "defender")
        let error = element(app, "favoriteLoadError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(error.label.contains("計算はそのまま使えます"), "label: \(error.label)")
    }

    /// 読み込んだあとに別の入力(攻守入れ替え)をすると、古い案内は消える。
    func testNoticeClearsOnNextInput() {
        let app = launchCalc()
        openSheet(app, side: "attacker")
        tapRow(app, "302")
        waitUntilGone(element(app, "favoriteLoadSheet"))
        let notice = element(app, "favoriteLoadNotice")
        scrollUntilHittable(app, notice)

        let swap = element(app, "swapSidesButton")
        scrollUntilHittable(app, swap)
        swap.tap()

        waitUntilGone(element(app, "favoriteLoadNotice"))
    }
}
