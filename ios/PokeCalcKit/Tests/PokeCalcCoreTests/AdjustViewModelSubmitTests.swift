import XCTest

@testable import PokeCalcCore

/// AJ7: 送信の結果・エラー・取り消し・未対応の印(ADR-0502 §5・AC5〜AC7)。
@MainActor
final class AdjustViewModelSubmitTests: XCTestCase {
    private typealias Support = AdjustTestSupport

    /// 自分 = beta(物理技)・相手 = gamma・倒せる最小・2発。
    private func minKoFixture() async -> AdjustTestSupport.Fixture {
        let fixture = await Support.makeFixture()
        await Support.selectOwnAndOpponent(fixture.viewModel)
        fixture.viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        fixture.viewModel.selectMode(.minKo)
        fixture.viewModel.selectHits(2)
        return fixture
    }

    // MARK: - 両方そろって結果

    func testBothSucceedProducesOutcome() async {
        let fixture = await minKoFixture()
        await fixture.viewModel.submit()
        XCTAssertEqual(fixture.viewModel.outcome, AdjustOutcome(
            indices: StubAdjustFixtures.indicesResult(firepower: true),
            modeResult: .ko(StubAdjustFixtures.koResult, hits: 2),
            unsupportedNotice: nil
        ))
        XCTAssertNil(fixture.viewModel.alertMessage)
        XCTAssertFalse(fixture.viewModel.isLoading)
    }

    func testSurviveOutcomeCarriesHits() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnAndOpponent(fixture.viewModel)
        fixture.viewModel.selectMode(.minSurvive)
        fixture.viewModel.selectOpponentMove(id: StubMaster.physicalMove.id)
        fixture.viewModel.selectHits(3)
        await fixture.viewModel.submit()
        XCTAssertEqual(fixture.viewModel.outcome?.modeResult, .survive(StubAdjustFixtures.surviveResult, hits: 3))
    }

    func testIsLoadingWhileBothCallsArePending() async throws {
        let fixture = await minKoFixture()
        await fixture.adjust.setMode(.manual)
        let task = fixture.viewModel.scheduleSubmit()
        try await fixture.adjust.waitForCalls(count: 2)
        XCTAssertTrue(fixture.viewModel.isLoading)
        XCTAssertNil(fixture.viewModel.outcome)

        await fixture.adjust.resolveWithDefault(at: 0)
        await Task.yield()
        XCTAssertNil(fixture.viewModel.outcome, "片方だけでは結果を出さない")
        await fixture.adjust.resolveWithDefault(at: 1)
        await task.value
        XCTAssertNotNil(fixture.viewModel.outcome)
        XCTAssertFalse(fixture.viewModel.isLoading)
    }

    // MARK: - エラー(どちらかが失敗したら結果を出さない。日本語・サーバーの message を出さない・入力を消さない)

    func testModeOperationFailureShowsJapaneseErrorAndNoOutcome() async {
        let fixture = await minKoFixture()
        await fixture.viewModel.submit()
        XCTAssertNotNil(fixture.viewModel.outcome)

        let serverMessage = "engine: hits out of range"
        await fixture.adjust.setError(PokeCalcError(code: "invalid_input", message: serverMessage), for: .ko)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.outcome, "片方の結果で判断させない(前の結果も残さない)")
        XCTAssertEqual(fixture.viewModel.alertMessage, AdjustText.errorMessages["invalid_input"])
        XCTAssertFalse(fixture.viewModel.alertMessage?.contains(serverMessage) ?? false)
        XCTAssertFalse(fixture.viewModel.isLoading)
    }

    func testIndicesFailureShowsErrorAndKeepsInputs() async {
        let fixture = await minKoFixture()
        fixture.viewModel.setFixedSPText("4", for: .hp)
        await fixture.adjust.setError(PokeCalcError(code: "master_unavailable", message: "db down"), for: .indices)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.outcome)
        XCTAssertEqual(fixture.viewModel.alertMessage, AdjustText.errorMessages["master_unavailable"])
        XCTAssertEqual(fixture.viewModel.fixedSPText(for: .hp), "4", "入力は消さない")
        XCTAssertEqual(fixture.viewModel.ownSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(fixture.viewModel.opponentSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(fixture.viewModel.mode, .minKo)
    }

    func testTransportFailureShowsUnavailableAndUnknownCodeShowsFallback() async {
        let fixture = await minKoFixture()
        await fixture.adjust.setError(PokeCalcError(code: PokeCalcError.Code.transport, message: "URLError"), for: .indices)
        await fixture.viewModel.submit()
        XCTAssertEqual(fixture.viewModel.alertMessage, AdjustText.unavailable)

        await fixture.adjust.setError(nil, for: .indices)
        await fixture.adjust.setError(PokeCalcError(code: "brand_new_code", message: "x"), for: .ko)
        await fixture.viewModel.submit()
        XCTAssertEqual(fixture.viewModel.alertMessage, AdjustText.errorFallback)
    }

    func testSuccessAfterErrorClearsMessage() async {
        let fixture = await minKoFixture()
        await fixture.adjust.setError(PokeCalcError(code: "invalid_input", message: "x"), for: .ko)
        await fixture.viewModel.submit()
        XCTAssertNotNil(fixture.viewModel.alertMessage)
        await fixture.adjust.setError(nil, for: .ko)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.alertMessage)
        XCTAssertNotNil(fixture.viewModel.outcome)
    }

    // MARK: - 取り消し(送り直し・画面を閉じる)

    /// 取り消しに応じず、最初の minKo だけ遅れて成功を返すサービス(取り消しに失敗した古い応答の再現)。
    private actor LateFirstKOService: AdjustService {
        static let lateResult = AdjustKOResult(stat: .atk, searchLimit: 32, feasible: true, sp: 99, chancePercent: 100)
        static let freshResult = AdjustKOResult(stat: .atk, searchLimit: 32, feasible: true, sp: 12, chancePercent: 100)
        private var koCalls = 0
        private var gate: CheckedContinuation<Void, Never>?

        var isParked: Bool { gate != nil }
        func release() {
            gate?.resume()
            gate = nil
        }

        func adjustIndices(_ request: AdjustIndicesRequest) async throws -> AdjustIndicesResult {
            StubAdjustFixtures.indicesResult(firepower: request.moveId != nil)
        }

        func adjustMinSpToKo(_ request: AdjustSearchRequest) async throws -> AdjustKOResult {
            koCalls += 1
            guard koCalls == 1 else { return Self.freshResult }
            await withCheckedContinuation { gate = $0 }
            return Self.lateResult
        }

        func adjustMinSpToSurvive(_ request: AdjustSearchRequest) async throws -> AdjustSurviveResult {
            StubAdjustFixtures.surviveResult
        }

        func adjustAllocation(_ request: AdjustAllocationRequest) async throws -> AdjustAllocationResult {
            StubAdjustFixtures.allocationResult
        }

        func moveLearners(moveId: String, limit: Int, offset: Int) async throws -> [SpeciesSummary] { [] }
    }

    func testLateSuccessOfStaleSubmitDoesNotOverwriteNewerOutcome() async throws {
        let master = StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly],
            natures: StubMaster.reverseNatures)
        await master.setMoveBatchMode(.immediate)
        let adjust = LateFirstKOService()
        let viewModel = AdjustViewModel(service: master, adjust: adjust, searchDebounce: .zero)
        await viewModel.load()
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        viewModel.selectMode(.minKo)

        let first = viewModel.scheduleSubmit()
        for _ in 0..<10000 {
            if await adjust.isParked { break }
            try await Task.sleep(for: .milliseconds(1))
        }
        let parked = await adjust.isParked
        XCTAssertTrue(parked, "最初の送信が保留されている")

        await viewModel.submit()
        XCTAssertEqual(viewModel.outcome?.modeResult, .ko(LateFirstKOService.freshResult, hits: 1))

        await adjust.release()
        await first.value
        XCTAssertEqual(
            viewModel.outcome?.modeResult, .ko(LateFirstKOService.freshResult, hits: 1), "遅れて成功した古い応答で上書きしない")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.alertMessage)
    }

    func testResubmitClearsPreviousOutcomeAtStart() async throws {
        let fixture = await minKoFixture()
        await fixture.viewModel.submit()
        XCTAssertNotNil(fixture.viewModel.outcome)

        await fixture.adjust.setMode(.manual)
        _ = fixture.viewModel.scheduleSubmit()
        try await fixture.adjust.waitForCalls(count: 4)
        XCTAssertNil(fixture.viewModel.outcome, "送り直しを始めたら前の結果を消す(計算中に古い結果を見せない)")
        XCTAssertTrue(fixture.viewModel.isLoading)
        fixture.viewModel.cancelPendingWork()
    }

    func testAlertSerialIncreasesForRepeatedSameMessage() async {
        let fixture = await minKoFixture()
        await fixture.adjust.setError(PokeCalcError(code: "invalid_input", message: "x"), for: .ko)
        await fixture.viewModel.submit()
        let first = fixture.viewModel.alertSerial
        let message = fixture.viewModel.alertMessage
        await fixture.viewModel.submit()
        XCTAssertEqual(fixture.viewModel.alertMessage, message, "同じ文が続く")
        XCTAssertGreaterThan(fixture.viewModel.alertSerial, first, "文を出すたびに番号が進み、View が読み直せる")
    }

    func testResubmitCancelsPreviousAndAppliesOnlyLatest() async throws {
        let fixture = await minKoFixture()
        await fixture.adjust.setMode(.manual)
        _ = fixture.viewModel.scheduleSubmit()
        try await fixture.adjust.waitForCalls(count: 2)

        fixture.viewModel.selectHits(3)
        let second = fixture.viewModel.scheduleSubmit()
        try await fixture.adjust.waitForCancellation(at: 0)
        try await fixture.adjust.waitForCancellation(at: 1)
        try await fixture.adjust.waitForCalls(count: 4)

        await fixture.adjust.resolveWithDefault(at: 2)
        await fixture.adjust.resolveWithDefault(at: 3)
        await second.value
        XCTAssertEqual(fixture.viewModel.outcome?.modeResult, .ko(StubAdjustFixtures.koResult, hits: 3), "新しい送信の結果だけを出す")
        XCTAssertNil(fixture.viewModel.alertMessage, "取り消しはエラーとして出さない")
        XCTAssertFalse(fixture.viewModel.isLoading)
    }

    func testCancelPendingWorkCancelsInFlightCallsWithoutError() async throws {
        let fixture = await minKoFixture()
        await fixture.adjust.setMode(.manual)
        let task = fixture.viewModel.scheduleSubmit()
        try await fixture.adjust.waitForCalls(count: 2)

        fixture.viewModel.cancelPendingWork()
        try await fixture.adjust.waitForCancellation(at: 0)
        try await fixture.adjust.waitForCancellation(at: 1)
        await task.value
        XCTAssertNil(fixture.viewModel.outcome)
        XCTAssertNil(fixture.viewModel.alertMessage)
        XCTAssertFalse(fixture.viewModel.isLoading, "取り消したら計算中を解く")
    }

    // MARK: - 未対応の印

    func testUnsupportedMarksOfModeResultBecomeOneNotice() async {
        let fixture = await minKoFixture()
        let mark = UnsupportedMark(target: .move, reason: .multiHit, id: StubMaster.physicalMove.id)
        var ko = StubAdjustFixtures.koResult
        ko.unsupported = [mark]
        await fixture.adjust.setKOResult(ko)
        await fixture.viewModel.submit()

        let notice = try? XCTUnwrap(fixture.viewModel.outcome?.unsupportedNotice)
        XCTAssertEqual(notice, UnsupportedNoticeText.summary([mark], names: UnsupportedMarkNames(moveNames: [
            StubMaster.physicalMove.id: StubMaster.physicalMove.nameJa,
        ])), "印の名前はマスタから引く(技名)")
        XCTAssertEqual(fixture.viewModel.outcome?.modeResult, .ko(ko, hits: 2), "数値は印の有無で変えない")
    }

    func testAllocationUnsupportedMarksAreShown() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.bulk)
        let mark = UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: "test-unknown-item")
        var result = StubAdjustFixtures.allocationResult
        result.unsupported = [mark]
        await fixture.adjust.setAllocationResult(result)
        await fixture.viewModel.submit()
        XCTAssertEqual(
            fixture.viewModel.outcome?.unsupportedNotice,
            UnsupportedNoticeText.summary([mark], names: UnsupportedMarkNames()), "名前が引けない ID は ID のまま")
    }

    func testIndicesModeHasNoNotice() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        await fixture.viewModel.submit()
        XCTAssertNotNil(fixture.viewModel.outcome)
        XCTAssertNil(fixture.viewModel.outcome?.unsupportedNotice)
    }
}
