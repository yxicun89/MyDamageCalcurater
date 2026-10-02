import XCTest

@testable import PokeCalcCore

/// `BalanceViewModel` の非同期・失敗(P6-26。ADR-0505 §4): 2 つの呼び出しは独立・最新の世代の応答だけ反映・cancel・失敗は日本語で他に波及しない。
@MainActor
final class BalanceViewModelAsyncTests: XCTestCase {
    private typealias H = BalanceHarness
    private typealias B = StubBalance

    // MARK: - 独立(Web の ADR-0303 §9 と同じ。1 つの遅延・失敗が他方の表示を消さない)

    /// analyze が終わっていれば、coverage が保留中でも防御相性は出る(coverage は `.loading` のまま)。
    func testDefenseIsShownWhileCoverageIsStillLoading() async throws {
        let service = StubBalanceService()
        await service.hold(.coverage)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        try await service.waitForCalls(.coverage, count: 1)
        for _ in 0..<2000 {
            if case .loaded = viewModel.defenseState { break }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTAssertNotNil(H.defenseDisplay(viewModel))
        XCTAssertEqual(viewModel.coverageState, .loading)
        await service.release(.coverage, at: 0)
        await viewModel.settle()
        XCTAssertNotNil(H.coverageDisplay(viewModel))
    }

    /// analyze が失敗しても coverage は出る。失敗は `code` から日本語にする(サーバーの英語 message は state に入らない)。
    func testAnalyzeFailureDoesNotHideCoverage() async {
        let service = StubBalanceService()
        await service.setAnalyzeResponder { _, _ in .failure(B.failure) }
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        XCTAssertEqual(viewModel.defenseState, .failed(BalanceFailure(code: "overloaded")))
        if case .failed(let failure) = viewModel.defenseState {
            XCTAssertEqual(failure.message, BalanceLabelsTests.expectedErrorMessages["overloaded"])
        }
        XCTAssertNotNil(H.coverageDisplay(viewModel), "防御相性が失敗しても攻撃範囲は出る")
    }

    func testCoverageFailureDoesNotHideDefense() async {
        let service = StubBalanceService()
        await service.setCoverageResponder { _, _ in .failure(B.otherFailure) }
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        XCTAssertEqual(viewModel.coverageState, .failed(BalanceFailure(code: "internal_error")))
        XCTAssertNotNil(H.defenseDisplay(viewModel), "攻撃範囲が失敗しても防御相性は出る")
    }

    /// 失敗しても構築の一覧・選択は残り、同じ構築を選び直す(または reanalyze)と再試行できる。
    func testRetryAfterFailureSucceeds() async {
        let service = StubBalanceService()
        await service.setAnalyzeResponder { request, index in index == 0 ? .failure(B.failure) : .success(B.defense(for: request)) }
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        XCTAssertEqual(viewModel.defenseState, .failed(BalanceFailure(code: "overloaded")))
        XCTAssertEqual(viewModel.teamOptions.count, B.allTeams.count, "失敗しても構築の一覧は残る(絶対ルール 5)")
        XCTAssertEqual(viewModel.selectedTeamID, B.teamMixed.id)
        await viewModel.reanalyze()
        await viewModel.settle()
        XCTAssertNotNil(H.defenseDisplay(viewModel))
    }

    /// 再解析の間は `.loading` に戻る(古い結果を新しい結果のように見せない)。
    func testReanalyzeShowsLoadingAgain() async throws {
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        await service.hold(.analyze)
        let task = Task { await viewModel.reanalyze() }
        try await service.waitForCalls(.analyze, count: 2)
        XCTAssertEqual(viewModel.defenseState, .loading)
        await service.release(.analyze, at: 1)
        await task.value
        await viewModel.settle()
        XCTAssertNotNil(H.defenseDisplay(viewModel))
    }

    // MARK: - 世代・cancel

    /// 新しい選択が先行の呼び出しを cancel する(cancel を受け取るサービスの場合)。
    func testANewSelectionCancelsThePreviousCalls() async throws {
        let service = StubBalanceService()
        await service.hold(.analyze)
        await service.hold(.coverage)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        try await service.waitForCalls(.analyze, count: 1)
        try await service.waitForCalls(.coverage, count: 1)
        viewModel.selectTeam(id: B.teamFour.id)
        try await service.waitForCancellation(.analyze, at: 0)
        try await service.waitForCancellation(.coverage, at: 0)
        viewModel.cancelPendingWork()
    }

    /// cancel を無視するサービスの古い応答が後から届いても、最新の選択の結果を上書きしない。
    func testStaleResponsesAreDroppedEvenIfTheServiceIgnoresCancellation() async throws {
        let service = StubBalanceService()
        await service.hold(.analyze, ignoringCancellation: true)
        await service.hold(.coverage, ignoringCancellation: true)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        try await service.waitForCalls(.analyze, count: 1)
        viewModel.selectTeam(id: B.teamFour.id)
        try await service.waitForCalls(.analyze, count: 2)
        try await service.waitForCalls(.coverage, count: 2)
        await service.release(.analyze, at: 1)
        await service.release(.coverage, at: 1)
        await viewModel.settle()
        XCTAssertEqual(try XCTUnwrap(H.defenseDisplay(viewModel)).members.count, 4)
        // 古い応答(2 体の構築)を今さら返しても、4 体の結果のまま。
        await service.release(.analyze, at: 0)
        await service.release(.coverage, at: 0)
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(try XCTUnwrap(H.defenseDisplay(viewModel)).members.count, 4)
        XCTAssertEqual(try XCTUnwrap(H.coverageDisplay(viewModel)).members.count, 4)
        XCTAssertEqual(viewModel.selectedTeamID, B.teamFour.id)
    }

    /// 古い応答の失敗も、最新の結果を消さない。
    func testStaleFailureDoesNotOverwriteTheLatestResult() async throws {
        let service = StubBalanceService()
        await service.setAnalyzeResponder { request, index in index == 0 ? .failure(B.failure) : .success(B.defense(for: request)) }
        await service.hold(.analyze, ignoringCancellation: true)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        try await service.waitForCalls(.analyze, count: 1)
        viewModel.selectTeam(id: B.teamFour.id)
        try await service.waitForCalls(.analyze, count: 2)
        await service.release(.analyze, at: 1)
        await viewModel.settle()
        await service.release(.analyze, at: 0)
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertNotNil(H.defenseDisplay(viewModel), "古い呼び出しの失敗で、最新の成功を消さない")
    }

    /// 画面を離れるとき: 進行中は cancel して `.idle` に戻す(失敗にしない。`CancellationError` は失敗ではない)。完了済みの結果はそのまま。
    func testCancelPendingWorkStopsLoadingWithoutFailing() async throws {
        let service = StubBalanceService()
        await service.hold(.coverage)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        try await service.waitForCalls(.coverage, count: 1)
        for _ in 0..<2000 {
            if case .loaded = viewModel.defenseState { break }
            try await Task.sleep(for: .milliseconds(1))
        }
        viewModel.cancelPendingWork()
        XCTAssertNotNil(H.defenseDisplay(viewModel), "完了済みの防御相性は残る")
        XCTAssertEqual(viewModel.coverageState, .idle, "進行中だった攻撃範囲は .idle に戻る(失敗にしない)")
        try await service.waitForCancellation(.coverage, at: 0)
        try await Task.sleep(for: .milliseconds(30))
        XCTAssertEqual(viewModel.coverageState, .idle)
    }

    /// サービスが `CancellationError` を投げても失敗として出さない。
    func testCancellationErrorIsNotShownAsAFailure() async throws {
        let service = StubBalanceService()
        await service.hold(.analyze)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        try await service.waitForCalls(.analyze, count: 1)
        viewModel.selectTeam(id: B.teamNoMoves.id)
        try await service.waitForCancellation(.analyze, at: 0)
        await service.release(.analyze, at: 1)
        await viewModel.settle()
        if case .failed = viewModel.defenseState { XCTFail("キャンセルが失敗として出た") }
        XCTAssertNotNil(H.defenseDisplay(viewModel))
    }

    // MARK: - 他に影響しない(絶対ルール 5)

    /// balance の失敗は構築の読み込み・マスタ(名前の引き当て)に影響しない。逆に、マスタが失敗しても解析は止まらない(名前が ID に落ちるだけ)。
    func testMasterFailureOnlyDegradesNamesNotTheAnalysis() async throws {
        let master = StubMaster.makeService(species: [])
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service, master: master)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        let defense = try XCTUnwrap(H.defenseDisplay(viewModel))
        XCTAssertEqual(
            defense.members.map(\.name), ["テストバランスニックA", StubMaster.beta.key], "マスタに種族が無くても解析は成功し、名前はニックネーム → speciesKey に落ちる")
        XCTAssertEqual(defense.members.map(\.abilityName), [StubMaster.ability.id, nil], "特性名も引けなければ ID")
        XCTAssertNotNil(H.coverageDisplay(viewModel))
    }

    /// 結果は応答の値のまま(分類・倍率を書き換えない)。サービスが返した特殊な応答を、そのまま表示用に写す。
    func testResultsCarryTheServiceResponseThrough() async throws {
        let service = StubBalanceService()
        await service.setAnalyzeResponder { request, _ in
            var response = B.defense(for: request)
            response.members[0].defense[0] = BalanceDefenseEntry(attackType: .normal, multiplier: "5/4", category: .weak)
            return .success(response)
        }
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        XCTAssertEqual(try XCTUnwrap(H.defenseDisplay(viewModel)).members[safe: 0]?.cells[safe: 0]?.text, "×5/4 弱点")
    }
}
