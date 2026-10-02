import XCTest

/// P6-26: タイプバランス画面(ADR-0505。受け入れ条件は ADR-0501「P6-26 の受け入れ条件」)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ。ADR-0500 §5・§7)。タイプバランスのモックの挙動は環境変数
/// `POKECALC_MOCK_BALANCE`(`error` / `coverage-error`)で切り替える。入力は構築(端末内)から選ぶので、各テストは構築ビルダーで構築を作ってから開く。
///
/// 数値は、モックの固定の事実(ADR-0505 §8。`MockBalanceServiceTests` が単体で固定している)に頼る箇所だけで検査する。メンバー i(0 始まり)・タイプ t(normal = 0):
/// 防御は (i + t) % 6 で ×4 弱点 / ×2 弱点 / ×1 等倍 / ×1/2 耐性 / ×1/4 耐性 / ×0 無効 → メンバー 0 の normal は「×4 弱点」・メンバー 1 の normal は「×2 弱点」。
/// 攻撃範囲は技を持つメンバーだけ [×0, ×1/2, ×1, ×2][(i + t) % 4]・技の無いメンバーは「攻撃技なし」→ メンバー 0(技あり)の normal は「×0 無効」・メンバー 1(技なし)は「攻撃技なし」。
/// それ以外は「導線・要素の有無・並び・選択状態・失敗が他に波及しないこと」を見る(ADR-0501「XCUITest で確かめること」と同じ粒度)。
/// 構築は「テストモンいち(技あり)」と「テストモンに(技なし)」の 2 体(`BalanceUITestSupport`)。
///
/// 識別子(ADR-0505 §10。implementer はこの名前で付ける)。`<t>` はタイプの ID(normal … fairy)・`<i>` はメンバーの位置(0 始まり)・`<n>` は構築の位置(0 始まり。保存順):
///   ルート: openBalanceScreen / 画面: balanceScreen
///   構築: balanceTeamList / balanceTeam-<n>(構築名とメンバー数を label に。選択中は isSelected)/ balanceTeamEmptyMessage(構築が無い案内)/ balanceTeamLoadError(構築を読めない)
///   状態: balanceSelectPrompt(未選択の案内)/ balanceLoading / balanceNoMembers(メンバー 0 体の案内)/ balanceRetry(もう一度解析する)
///   防御相性: balanceDefenseRegion / balanceDefenseMember-<i>(名前・タイプ。label に名前)/ balanceDefenseCell-<i>-<t>(label に「ノーマル」のようなタイプ名と「×4 弱点」の語)/
///             balanceAbilityNote-<i>-<t>(特性が変えた欄の添え書き)/ balanceSummaryRegion / balanceSummary-<t>(チームの集計の 1 行。label に集計の文言)/ balanceDefenseError
///   攻撃範囲: balanceCoverageRegion / balanceCoverageMember-<i> / balanceCoverageCell-<i>-<t>(label に防御タイプ名と「×0 無効」「攻撃技なし」)/
///             balanceCoverageTeam-<t>(チームの行。label に最大倍率と「有効 n体・抜群 n体」)/ balanceCoverageNoMoves(技が 1 つも無い構築の案内)/ balanceCoverageError
///
/// P6-26 の spec 時点では View が未実装のため、このテストは失敗してよい。
@MainActor
final class BalanceScreenUITests: XCTestCase {
    private typealias S = BalanceUITestSupport
    private static let existenceTimeout = S.existenceTimeout
    private static let maxScrolls = 14

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 補助

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement { S.element(app, identifier) }

    private func wait(_ element: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) {
            app.swipeUp()
        }
    }

    private func tap(_ app: XCUIApplication, _ identifier: String) {
        let target = element(app, identifier)
        wait(target, "\(identifier) が無い")
        scrollUntilHittable(app, target)
        target.tap()
    }

    private func label(_ app: XCUIApplication, _ identifier: String) -> String {
        let target = element(app, identifier)
        wait(target, "\(identifier) が無い")
        return target.label
    }

    private func expectSelected(_ target: XCUIElement, _ selected: Bool, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        let predicate = NSPredicate(format: "isSelected == %@", NSNumber(value: selected))
        let result = XCTWaiter().wait(for: [XCTNSPredicateExpectation(predicate: predicate, object: target)], timeout: Self.existenceTimeout)
        XCTAssertEqual(result, .completed, message, file: file, line: line)
    }

    /// 構築を 1 つ作ってタイプバランス画面を開く(メンバーは技あり・技なしの 2 体)。
    private func launchWithTeam(scenario: String? = nil, members: [S.Member] = [S.armedMember, S.unarmedMember]) -> XCUIApplication {
        let app = S.launch(scenario: scenario)
        S.createTeam(app, name: "テストこうちくP6-26", members: members)
        S.openBalanceScreen(app)
        return app
    }

    // MARK: - 導線

    /// ルートから開ける。構築が無い間は「まだ構築がありません」の案内だけで、解析の結果は出ない。
    func testOpenBalanceScreenFromRootWithoutTeamsShowsGuidance() {
        let app = S.launch()
        S.openBalanceScreen(app)
        wait(element(app, "balanceTeamEmptyMessage"), "構築が無い案内が出る")
        XCTAssertFalse(element(app, "balanceDefenseRegion").exists)
        XCTAssertFalse(element(app, "balanceCoverageRegion").exists)
    }

    func testOpensAtLaunchWithTheEnvironmentVariable() {
        let app = S.launch(openAtLaunch: true)
        wait(element(app, "balanceScreen"), "起動時に balanceScreen が開く")
    }

    /// 構築があるのに未選択の間は、構築の一覧と案内だけ(結果の領域は出ない)。
    func testTeamsAreListedAndNothingIsAnalyzedUntilOneIsSelected() {
        let app = launchWithTeam()
        wait(element(app, "balanceTeam-0"), "構築の行が出る")
        XCTAssertTrue(label(app, "balanceTeam-0").contains("テストこうちくP6-26"), "構築名が出る")
        XCTAssertTrue(label(app, "balanceTeam-0").contains("2体"), "メンバー数が出る")
        wait(element(app, "balanceSelectPrompt"), "未選択の案内")
        XCTAssertFalse(element(app, "balanceDefenseRegion").exists)
        XCTAssertFalse(element(app, "balanceCoverageRegion").exists)
    }

    // MARK: - 防御相性

    /// 構築を選ぶと防御相性が出る。値はサーバー(モック)の応答のまま。メンバーごとに違う値で、色だけでなく語(「弱点」)・タイプ名も出る。
    func testSelectingATeamShowsDefenseWithTheServerValuesPerMember() {
        let app = launchWithTeam()
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceDefenseRegion"), "防御相性が出る")
        expectSelected(element(app, "balanceTeam-0"), true, "選んだ構築は選択状態")
        XCTAssertFalse(element(app, "balanceSelectPrompt").exists, "選んだら案内は消える")
        wait(element(app, "balanceDefenseCell-0-normal"), "メンバー 0 の normal の欄")
        let first = label(app, "balanceDefenseCell-0-normal")
        XCTAssertTrue(first.contains("×4") && first.contains("弱点"), "メンバー 0 の normal は ×4 弱点: \(first)")
        XCTAssertTrue(first.contains("ノーマル"), "タイプ名も label に出る(色だけにしない): \(first)")
        let second = label(app, "balanceDefenseCell-1-normal")
        XCTAssertTrue(second.contains("×2") && second.contains("弱点"), "メンバー 1 の normal は ×2 弱点(メンバーごとに違う): \(second)")
        let third = label(app, "balanceDefenseCell-0-water")
        XCTAssertTrue(third.contains("×1") && third.contains("等倍"), "メンバー 0 の water は (0 + 2) % 6 = ×1 等倍: \(third)")
    }

    /// メンバーの行は構築の順(名前はマスタの種族名)。
    func testMemberRowsFollowTheTeamOrderWithSpeciesNames() {
        let app = launchWithTeam()
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceDefenseMember-0"), "メンバー 0 の行")
        XCTAssertTrue(label(app, "balanceDefenseMember-0").contains("テストモンいち"))
        XCTAssertTrue(label(app, "balanceDefenseMember-1").contains("テストモンに"))
    }

    /// チームの集計はサーバーの数のまま(メンバー 0 の normal = quad_weak・メンバー 1 の normal = weak → 弱点 2・うち×4 1)。
    func testTeamSummaryShowsTheServerCounts() {
        let app = launchWithTeam()
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceSummary-normal"), "チームの集計の行")
        let text = label(app, "balanceSummary-normal")
        XCTAssertTrue(text.contains("弱点 2") && text.contains("うち×4 1"), "弱点 2(うち×4 1): \(text)")
    }

    // MARK: - 攻撃範囲

    /// 技を持つメンバーは倍率、技の無いメンバーは「攻撃技なし」。チームの行は有効・抜群の人数(技を持つメンバーだけ数える)。
    func testCoverageShowsMultipliersAndNoAttackMoveForAnUnarmedMember() {
        let app = launchWithTeam()
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceCoverageRegion"), "攻撃範囲が出る")
        let armed = label(app, "balanceCoverageCell-0-normal")
        XCTAssertTrue(armed.contains("×0") && armed.contains("無効"), "メンバー 0(技あり)の normal は ×0 無効: \(armed)")
        let electric = label(app, "balanceCoverageCell-0-electric")
        XCTAssertTrue(electric.contains("×2") && electric.contains("抜群"), "メンバー 0 の electric は (0 + 3) % 4 = ×2 抜群: \(electric)")
        let unarmed = label(app, "balanceCoverageCell-1-normal")
        XCTAssertTrue(unarmed.contains("攻撃技なし"), "技の無いメンバーは攻撃技なし: \(unarmed)")
        let team = label(app, "balanceCoverageTeam-normal")
        XCTAssertTrue(team.contains("有効 0体") && team.contains("抜群 0体"), "技を持つメンバーだけ数える(normal は ×0 だけ): \(team)")
        let teamElectric = label(app, "balanceCoverageTeam-electric")
        XCTAssertTrue(teamElectric.contains("有効 1体") && teamElectric.contains("抜群 1体"), "electric は ×2 のメンバーが 1 体: \(teamElectric)")
    }

    /// 技を持つメンバーが 1 体もいない構築は、攻撃範囲を出さず案内する(防御相性は出る)。
    func testTeamWithoutAnyMoveShowsCoverageGuidanceButStillShowsDefense() {
        let app = launchWithTeam(members: [S.unarmedMember])
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceDefenseRegion"), "防御相性は出る")
        wait(element(app, "balanceCoverageNoMoves"), "攻撃範囲の案内が出る")
        XCTAssertFalse(element(app, "balanceCoverageCell-0-normal").exists, "攻撃範囲の表は出ない")
    }

    // MARK: - 構築の切り替え・メンバー 0 体

    /// 別の構築を選ぶと、結果は新しい構築のものに替わる(メンバー数が違う 2 つの構築で見分ける)。
    func testSelectingAnotherTeamReplacesTheResults() {
        let app = S.launch()
        S.createTeam(app, name: "テストこうちくひとり", members: [S.armedMember])
        S.createTeam(app, name: "テストこうちくふたり", members: [S.armedMember, S.unarmedMember])
        S.openBalanceScreen(app)
        tap(app, "balanceTeam-1")
        wait(element(app, "balanceDefenseMember-1"), "2 体の構築はメンバー 1 の行がある")
        tap(app, "balanceTeam-0")
        expectSelected(element(app, "balanceTeam-0"), true, "選び直した構築が選択状態")
        expectSelected(element(app, "balanceTeam-1"), false, "前の構築は選択を外れる")
        XCTAssertTrue(element(app, "balanceDefenseMember-1").waitForNonExistence(timeout: Self.existenceTimeout), "1 体の構築にメンバー 1 の行は無い")
        wait(element(app, "balanceDefenseMember-0"), "メンバー 0 の行はある")
    }

    /// メンバーが 0 体の構築は、案内だけ(結果の領域は出ない)。
    func testTeamWithNoMembersShowsGuidanceOnly() {
        let app = launchWithTeam(members: [])
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceNoMembers"), "ポケモンがいない案内")
        XCTAssertFalse(element(app, "balanceDefenseRegion").exists)
        XCTAssertFalse(element(app, "balanceCoverageRegion").exists)
    }

    // MARK: - 失敗(絶対ルール 5: balance の失敗は計算・構築に影響しない)

    /// 失敗は日本語(サーバーの英語 message は出さない)。再試行の入口が出る。構築の一覧は残り、計算画面は開く。
    func testBalanceFailureShowsJapaneseMessagesKeepsTheTeamListAndDoesNotBreakTheCalcScreen() {
        let app = launchWithTeam(scenario: "error")
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceDefenseError"), "防御相性の失敗が出る")
        wait(element(app, "balanceCoverageError"), "攻撃範囲の失敗が出る")
        XCTAssertTrue(label(app, "balanceDefenseError").contains("サーバーのマスタを読み込めません"), "code から日本語にする")
        XCTAssertFalse(label(app, "balanceDefenseError").contains("mock"), "サーバーの英語 message は出さない")
        wait(element(app, "balanceRetry"), "もう一度解析する入口")
        XCTAssertTrue(element(app, "balanceTeam-0").exists, "失敗しても構築の一覧は残る")

        app.navigationBars.buttons.element(boundBy: 0).tap()
        let openCalc = app.buttons["openCalcScreen"]
        wait(openCalc, "ルートの計算への入口")
        openCalc.tap()
        wait(element(app, "calcScreen"), "balance が失敗していても計算画面は開く")
    }

    /// 攻撃範囲だけ失敗しても、防御相性は出たまま(2 つの解析は独立)。
    func testCoverageFailureKeepsTheDefenseResult() {
        let app = launchWithTeam(scenario: "coverage-error")
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceCoverageError"), "攻撃範囲の失敗")
        XCTAssertTrue(label(app, "balanceCoverageError").contains("サーバーでエラーが起きました"))
        wait(element(app, "balanceDefenseCell-0-normal"), "防御相性は出る")
        XCTAssertFalse(element(app, "balanceDefenseError").exists)
    }

    // MARK: - 表示の不変条件

    /// 総合点・ランキング・おすすめの語を画面に出さない(type-balance-design.md §1。この範囲では解析結果の表示だけ)。
    func testNoOverallScoreWordsOnTheResultScreen() {
        let app = launchWithTeam()
        tap(app, "balanceTeam-0")
        wait(element(app, "balanceCoverageRegion"), "結果が出る")
        for word in ["総合", "スコア", "ランキング", "評価"] {
            XCTAssertFalse(app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", word)).firstMatch.exists, "\(word) が出ている")
        }
    }
}
