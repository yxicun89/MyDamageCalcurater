import XCTest

@testable import PokeCalcCore

/// `JudgeViewModel.submit()` の非同期(P6-25。ADR-0504 §4・§6): 送信ボタンのみ・同期で loading・古い応答の破棄・cancel・失敗・
/// 候補の取り違えがない・送信時点の入力から結果を作る・判定の失敗は他に波及しない(絶対ルール5)。
@MainActor
final class JudgeViewModelAsyncTests: XCTestCase {
    private typealias H = JudgeHarness

    private func loadedRows(_ viewModel: JudgeViewModel, file: StaticString = #filePath, line: UInt = #line) -> [JudgeMatchupDisplay] {
        guard case .loaded(let display) = viewModel.resultState else {
            XCTFail("loaded ではない: \(viewModel.resultState)", file: file, line: line)
            return []
        }
        return display.rows
    }

    // MARK: - 基本

    func testSubmitTurnsLoadingSynchronouslyThenLoaded() async {
        let viewModel = await H.makeFilled()
        viewModel.submit()
        XCTAssertEqual(viewModel.resultState, .loading, "同期で判定中にする(古い結果を出し続けない)")
        await viewModel.settle()
        XCTAssertEqual(loadedRows(viewModel).count, 1)
    }

    func testTheServiceReceivesExactlyTheBuiltRequest() async throws {
        let service = StubJudgeService()
        let viewModel = await H.makeFilled(service: service)
        viewModel.setSP(.spe, 20, for: .attacker)
        viewModel.setTrickRoom(true)
        let expected = try H.request(viewModel)
        viewModel.submit()
        await viewModel.settle()
        let requests = await service.requests
        XCTAssertEqual(requests, [expected])
    }

    func testRowsFollowTheCandidatesWithTheirOwnValues() async {
        let service = StubJudgeService()
        let viewModel = await H.makeFilled(service: service)
        viewModel.addCandidate()
        await viewModel.setSpecies(H.gamma, for: .candidate(1))
        viewModel.setMove(StubMaster.physicalMove, for: .candidate(1))
        viewModel.submit()
        await viewModel.settle()
        let rows = loadedRows(viewModel)
        XCTAssertEqual(rows.map(\.nameJa), [StubMaster.beta.nameJa, StubMaster.gamma.nameJa], "種族名は入力から index で引く")
        // StubJudge.matchup は index ごとに違う値(defenderSpeed = 100 + 10 × index)。
        XCTAssertEqual(rows.map(\.speedText), ["素早さ 200 対 100", "素早さ 200 対 110"])
    }

    /// 応答の行を `defenderIndex` の順に並べ、入力の候補と取り違えない(サーバーは同じ順で返す契約だが、画面は index で引く)。
    func testRowsAreMatchedByDefenderIndexEvenIfTheServiceReturnsThemOutOfOrder() async {
        let service = StubJudgeService()
        await service.setResponder { request, _ in
            .success(JudgeResponse(matchups: request.defenders.indices.reversed().map { StubJudge.matchup($0) }))
        }
        let viewModel = await H.makeFilled(service: service)
        viewModel.addCandidate()
        await viewModel.setSpecies(H.gamma, for: .candidate(1))
        viewModel.setMove(StubMaster.physicalMove, for: .candidate(1))
        viewModel.submit()
        await viewModel.settle()
        let rows = loadedRows(viewModel)
        XCTAssertEqual(rows.map(\.defenderIndex), [0, 1])
        XCTAssertEqual(rows.map(\.nameJa), [StubMaster.beta.nameJa, StubMaster.gamma.nameJa])
    }

    /// 結果は送信時点の入力から作る。判定中に入力を変えても、その結果の名前は変わらない(別の候補の名前を貼らない)。
    func testResultNamesComeFromTheSnapshotAtSubmitTime() async throws {
        let service = StubJudgeService()
        await service.hold()
        let viewModel = await H.makeFilled(service: service)
        viewModel.submit()
        try await service.waitForCalls(count: 1)
        await viewModel.setSpecies(H.gamma, for: .candidate(0))
        await service.release(at: 0)
        await viewModel.settle()
        XCTAssertEqual(loadedRows(viewModel).map(\.nameJa), [StubMaster.beta.nameJa])
    }

    // MARK: - 古い応答・cancel

    /// 連打しても、最後に送った要求の応答だけを表示する(cancel を無視するサービスの古い応答も捨てる)。
    func testOnlyTheLatestSubmitIsShownEvenIfAnOlderResponseArrivesLast() async throws {
        let service = StubJudgeService()
        await service.hold(ignoringCancellation: true)
        let viewModel = await H.makeFilled(service: service)
        viewModel.submit()
        try await service.waitForCalls(count: 1)
        viewModel.addCandidate()
        await viewModel.setSpecies(H.gamma, for: .candidate(1))
        viewModel.setMove(StubMaster.physicalMove, for: .candidate(1))
        viewModel.submit()
        try await service.waitForCalls(count: 2)

        await service.release(at: 1)
        await viewModel.settle()
        XCTAssertEqual(loadedRows(viewModel).count, 2, "新しい要求(候補2件)の結果")
        await service.release(at: 0)
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(loadedRows(viewModel).count, 2, "遅れて届いた古い応答(候補1件)は反映しない")
    }

    func testANewSubmitCancelsTheRunningRequestWithoutShowingItAsAFailure() async throws {
        let service = StubJudgeService()
        await service.hold()
        let viewModel = await H.makeFilled(service: service)
        viewModel.submit()
        try await service.waitForCalls(count: 1)
        viewModel.submit()
        try await service.waitForCalls(count: 2)
        try await service.waitForCancellation(at: 0)
        XCTAssertEqual(viewModel.resultState, .loading, "先行の cancel を失敗として出さない")
        await service.release(at: 1)
        await viewModel.settle()
        XCTAssertEqual(loadedRows(viewModel).count, 1)
        let cancelled = await service.cancelled
        XCTAssertEqual(cancelled, [0])
    }

    func testCancelPendingWorkCancelsTheRunningRequestAndIsNotAFailure() async throws {
        let service = StubJudgeService()
        await service.hold()
        let viewModel = await H.makeFilled(service: service)
        viewModel.submit()
        try await service.waitForCalls(count: 1)
        viewModel.cancelPendingWork()
        try await service.waitForCancellation(at: 0)
        await viewModel.settle()
        if case .failed = viewModel.resultState { XCTFail("画面を離れた cancel を失敗にしない") }
        if case .loaded = viewModel.resultState { XCTFail("cancel した要求の結果は出さない") }
    }

    // MARK: - 失敗

    func testFailureShowsAJapaneseMessageFromTheCodeAndKeepsTheInput() async {
        let service = StubJudgeService()
        await service.setResponder { _, _ in .failure(StubJudge.failure) }
        let viewModel = await H.makeFilled(service: service)
        let before = (viewModel.attacker, viewModel.candidates, viewModel.speedField)
        viewModel.submit()
        await viewModel.settle()
        guard case .failed(let failure) = viewModel.resultState else { return XCTFail("failed ではない: \(viewModel.resultState)") }
        XCTAssertEqual(failure.code, "upstream_unavailable")
        XCTAssertEqual(failure.message, "判定に必要なサービスに接続できません")
        XCTAssertFalse(failure.message.contains("english"), "サーバーの英語 message は出さない")
        XCTAssertNil(failure.candidateNumber)
        XCTAssertNil(failure.candidateHint)
        XCTAssertEqual(viewModel.attacker, before.0, "エラーでも入力は消さない(直して送り直せる)")
        XCTAssertEqual(viewModel.candidates, before.1)
        XCTAssertEqual(viewModel.speedField, before.2)
    }

    /// 失敗した候補はサーバーの message の `defenders[<index>]` から読み、「相手候補 N」で示す(ADR-0703 §3)。英語は出さない。
    func testFailureNamesTheFailedCandidateFromTheServerMessage() async {
        let service = StubJudgeService()
        await service.setResponder { _, _ in
            .failure(PokeCalcError(code: "unknown_move", message: "unknown move for defenders[1]: stub"))
        }
        let viewModel = await H.makeFilled(service: service)
        viewModel.addCandidate()
        await viewModel.setSpecies(H.gamma, for: .candidate(1))
        viewModel.setMove(StubMaster.physicalMove, for: .candidate(1))
        viewModel.submit()
        await viewModel.settle()
        guard case .failed(let failure) = viewModel.resultState else { return XCTFail("failed ではない") }
        XCTAssertEqual(failure.code, "unknown_move")
        XCTAssertEqual(failure.candidateNumber, 2, "defenders[1] → 相手候補2")
        XCTAssertEqual(failure.candidateHint, "相手候補2の入力で失敗しました")
        XCTAssertEqual(failure.message, "この技の ID はマスタにありません")
    }

    func testFailureCandidateIsReadOnlyFromARealDefendersIndex() {
        XCTAssertEqual(JudgeFailure(code: "invalid_request", serverMessage: "defenders[0].moveId is required", candidateCount: 3).candidateNumber, 1)
        XCTAssertEqual(JudgeFailure(code: "unknown_species", serverMessage: "defenders[2]", candidateCount: 3).candidateNumber, 3)
        XCTAssertNil(
            JudgeFailure(code: "unknown_species", serverMessage: "defenders[3]", candidateCount: 3).candidateNumber, "候補数以上の index は信じない")
        XCTAssertNil(
            JudgeFailure(code: "invalid_request", serverMessage: "attacker.moveId is invalid", candidateCount: 3).candidateNumber,
            "自分(attacker)の失敗に候補の番号を付けない(ADR-0706 §4)")
        XCTAssertNil(JudgeFailure(code: "internal_error", serverMessage: "", candidateCount: 3).candidateNumber)
        XCTAssertNil(JudgeFailure(code: "invalid_request", serverMessage: "defenders[x]", candidateCount: 3).candidateNumber)
    }

    func testTransportAndDecodeFailuresReadAsUnavailable() async {
        for code in [PokeCalcError.Code.transport, PokeCalcError.Code.decode, PokeCalcError.Code.unexpectedStatus] {
            let service = StubJudgeService()
            await service.setResponder { _, _ in .failure(PokeCalcError(code: code, message: "boom")) }
            let viewModel = await H.makeFilled(service: service)
            viewModel.submit()
            await viewModel.settle()
            guard case .failed(let failure) = viewModel.resultState else { return XCTFail("failed ではない: \(code)") }
            XCTAssertEqual(failure.message, "判定の API に接続できません", code)
        }
    }

    func testAfterAFailureAnotherSubmitCanSucceed() async {
        let service = StubJudgeService()
        await service.setResponder { request, index in
            index == 0 ? .failure(StubJudge.failure) : .success(StubJudge.echo(for: request))
        }
        let viewModel = await H.makeFilled(service: service)
        viewModel.submit()
        await viewModel.settle()
        guard case .failed = viewModel.resultState else { return XCTFail("1回目は失敗") }
        viewModel.submit()
        await viewModel.settle()
        XCTAssertEqual(loadedRows(viewModel).count, 1)
    }

    // MARK: - 絶対ルール5: 判定とマスタ・構築は互いに巻き込まない

    func testAJudgeFailureDoesNotTouchTheMasterData() async {
        let service = StubJudgeService()
        await service.setResponder { _, _ in .failure(StubJudge.failure) }
        let viewModel = await H.makeFilled(service: service, store: StubTeams.makeStore())
        let natures = viewModel.natureOptions
        let items = viewModel.itemOptions
        let teams = viewModel.teamOptions
        viewModel.submit()
        await viewModel.settle()
        XCTAssertNil(viewModel.masterFailure)
        XCTAssertEqual(viewModel.natureOptions, natures)
        XCTAssertEqual(viewModel.itemOptions, items)
        XCTAssertEqual(viewModel.teamOptions, teams)
    }

    /// マスタ(pokedex)が読めなくても、手元の入力が揃っていれば判定は送れる(判定の成否はマスタの読み込みと独立)。
    func testMasterFailureDoesNotStopTheJudgeFromBeingSent() async {
        let service = StubJudgeService()
        let master = H.makeMaster()
        await master.setMasterError(PokeCalcError(code: "upstream_unavailable", message: "x"))
        let viewModel = H.makeViewModel(service: service, master: master)
        await viewModel.load()
        XCTAssertNotNil(viewModel.masterFailure)
        await viewModel.setSpecies(H.alpha, for: .attacker)
        viewModel.setNature("stub-nature-neutral", for: .attacker)
        viewModel.setMove(StubMaster.physicalMove, for: .attacker)
        await viewModel.setSpecies(H.beta, for: .candidate(0))
        viewModel.setNature("stub-nature-neutral", for: .candidate(0))
        viewModel.setMove(StubMaster.specialMove, for: .candidate(0))
        viewModel.submit()
        await viewModel.settle()
        XCTAssertEqual(loadedRows(viewModel).count, 1)
    }

    func testLoadNeverThrowsEvenWhenEverythingFails() async {
        let master = H.makeMaster()
        await master.setMasterError(PokeCalcError(code: "upstream_unavailable", message: "x"))
        let store = StubTeams.makeStore()
        await store.setListError(PokeCalcError(code: "client_team_store_corrupted", message: "x"))
        let viewModel = H.makeViewModel(master: master, store: store)
        await viewModel.load()
        XCTAssertEqual(viewModel.masterFailure?.code, "upstream_unavailable")
        XCTAssertEqual(viewModel.teamOptions, [])
        XCTAssertEqual(viewModel.resultState, .idle)
    }
}
