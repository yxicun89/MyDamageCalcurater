import XCTest

@testable import PokeCalcCore

/// AJ7: 技を覚えるポケモン(機能 1。ADR-0502 §7・AC8)。1ページ `RequestLimits.moveLearnersPageSize` 件を
/// `offset` で読み、ちょうど1ページ分返ったら「続きを読み込む」を出す。
@MainActor
final class AdjustViewModelLearnersTests: XCTestCase {
    private typealias Support = AdjustTestSupport
    private let pageSize = RequestLimits.moveLearnersPageSize

    private func fixtureWithMoves() async -> AdjustTestSupport.Fixture {
        let fixture = await Support.makeFixture()
        await Support.selectOwnAndOpponent(fixture.viewModel)
        fixture.viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        fixture.viewModel.selectOpponentMove(id: StubMaster.specialMove.id)
        return fixture
    }

    func testPageSizeIsContractDefault() {
        XCTAssertEqual(RequestLimits.moveLearnersPageSize, 50)
    }

    func testOpenWithoutMoveDoesNothing() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        await fixture.viewModel.openLearners(.own)
        await fixture.viewModel.openLearners(.opponent)
        XCTAssertNil(fixture.viewModel.learners)
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [])
    }

    func testOpenOwnLoadsFirstPage() async {
        let fixture = await fixtureWithMoves()
        let page = Support.learners(0..<3)
        await fixture.adjust.setLearnersResponder { _, _, _ in page }
        await fixture.viewModel.openLearners(.own)

        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [.learners(moveId: StubMaster.physicalMove.id, limit: pageSize, offset: 0)],
                       "逆引きだけを呼ぶ(調整 API は呼ばない)")
        XCTAssertEqual(fixture.viewModel.learners, AdjustLearnersState(
            side: .own, moveId: StubMaster.physicalMove.id, moveName: StubMaster.physicalMove.nameJa,
            species: page, canLoadMore: false, isLoading: false, errorMessage: nil
        ))
    }

    func testOpenOpponentUsesOpponentMove() async {
        let fixture = await fixtureWithMoves()
        await fixture.viewModel.openLearners(.opponent)
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [.learners(moveId: StubMaster.specialMove.id, limit: pageSize, offset: 0)])
        XCTAssertEqual(fixture.viewModel.learners?.side, .opponent)
        XCTAssertEqual(fixture.viewModel.learners?.moveName, StubMaster.specialMove.nameJa)
    }

    func testEmptyResultHasNoErrorAndNoMore() async {
        let fixture = await fixtureWithMoves()
        await fixture.viewModel.openLearners(.own)
        XCTAssertEqual(fixture.viewModel.learners?.species, [])
        XCTAssertEqual(fixture.viewModel.learners?.canLoadMore, false)
        XCTAssertNil(fixture.viewModel.learners?.errorMessage, "一致なしはエラーではない(画面は learnersEmpty を出す)")
    }

    func testFullPageOffersMoreAndLoadMoreAppendsNextOffset() async {
        let fixture = await fixtureWithMoves()
        let pageSize = self.pageSize
        await fixture.adjust.setLearnersResponder { _, _, offset in
            offset == 0 ? Support.learners(0..<pageSize) : Support.learners(pageSize..<(pageSize + 10))
        }
        await fixture.viewModel.openLearners(.own)
        XCTAssertEqual(fixture.viewModel.learners?.species.count, pageSize)
        XCTAssertEqual(fixture.viewModel.learners?.canLoadMore, true, "ちょうど1ページ分なら続きがありうる")

        await fixture.viewModel.loadMoreLearners()
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls.last, .learners(moveId: StubMaster.physicalMove.id, limit: pageSize, offset: pageSize))
        XCTAssertEqual(fixture.viewModel.learners?.species, Support.learners(0..<(pageSize + 10)), "読んだ順に連結する")
        XCTAssertEqual(fixture.viewModel.learners?.canLoadMore, false)
    }

    func testLoadMoreWithoutMoreDoesNothing() async {
        let fixture = await fixtureWithMoves()
        await fixture.viewModel.openLearners(.own)
        await fixture.viewModel.loadMoreLearners()
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls.count, 1)
    }

    func testFirstPageErrorShowsJapaneseMessage() async {
        let fixture = await fixtureWithMoves()
        await fixture.adjust.setError(PokeCalcError(code: "not_found", message: "move missing"), for: .learners)
        await fixture.viewModel.openLearners(.own)
        XCTAssertEqual(fixture.viewModel.learners?.errorMessage, AdjustText.errorMessages["not_found"])
        XCTAssertEqual(fixture.viewModel.learners?.species, [])
        XCTAssertNil(fixture.viewModel.alertMessage, "一覧のエラーは一覧の中に出す(調整のエラー欄を使わない)")
    }

    func testLoadMoreErrorKeepsListAndAllowsRetry() async {
        let fixture = await fixtureWithMoves()
        let pageSize = self.pageSize
        await fixture.adjust.setLearnersResponder { _, _, offset in
            offset == 0 ? Support.learners(0..<pageSize) : Support.learners(pageSize..<(pageSize + 1))
        }
        await fixture.viewModel.openLearners(.own)
        await fixture.adjust.setError(PokeCalcError(code: PokeCalcError.Code.transport, message: "offline"), for: .learners)
        await fixture.viewModel.loadMoreLearners()
        XCTAssertEqual(fixture.viewModel.learners?.species.count, pageSize, "続きの失敗で読み込んだ一覧を消さない")
        XCTAssertEqual(fixture.viewModel.learners?.errorMessage, AdjustText.unavailable)
        XCTAssertEqual(fixture.viewModel.learners?.canLoadMore, true, "もう一度押せる")

        await fixture.adjust.setError(nil, for: .learners)
        await fixture.viewModel.loadMoreLearners()
        XCTAssertNil(fixture.viewModel.learners?.errorMessage)
        XCTAssertEqual(fixture.viewModel.learners?.species.count, pageSize + 1)
    }

    func testReopenWithOtherMoveCancelsPreviousAndReplacesList() async throws {
        let fixture = await fixtureWithMoves()
        await fixture.adjust.setMode(.manual)
        _ = fixture.viewModel.scheduleOpenLearners(.own)
        try await fixture.adjust.waitForCalls(count: 1)
        let second = fixture.viewModel.scheduleOpenLearners(.opponent)
        try await fixture.adjust.waitForCancellation(at: 0)
        try await fixture.adjust.waitForCalls(count: 2)
        await fixture.adjust.resolve(at: 1, with: .success(.learners(Support.learners(0..<2))))
        await second.value
        XCTAssertEqual(fixture.viewModel.learners?.side, .opponent)
        XCTAssertEqual(fixture.viewModel.learners?.species, Support.learners(0..<2))
        XCTAssertNil(fixture.viewModel.learners?.errorMessage, "取り消しはエラーとして出さない")
    }

    func testCloseAndCancelPendingWork() async throws {
        let fixture = await fixtureWithMoves()
        await fixture.adjust.setMode(.manual)
        let task = fixture.viewModel.scheduleOpenLearners(.own)
        try await fixture.adjust.waitForCalls(count: 1)
        XCTAssertEqual(fixture.viewModel.learners?.isLoading, true)
        fixture.viewModel.cancelPendingWork()
        try await fixture.adjust.waitForCancellation(at: 0)
        await task.value
        XCTAssertNil(fixture.viewModel.learners?.errorMessage)

        fixture.viewModel.closeLearners()
        XCTAssertNil(fixture.viewModel.learners)
    }

    func testLearnersDoNotTouchAdjustOutcome() async {
        let fixture = await fixtureWithMoves()
        await fixture.viewModel.submit()
        let outcome = fixture.viewModel.outcome
        XCTAssertNotNil(outcome)
        await fixture.viewModel.openLearners(.own)
        XCTAssertEqual(fixture.viewModel.outcome, outcome)
        let indices = await fixture.adjust.calls(of: .indices)
        XCTAssertEqual(indices.count, 1, "一覧を開いても調整 API は呼ばない")
    }
}
