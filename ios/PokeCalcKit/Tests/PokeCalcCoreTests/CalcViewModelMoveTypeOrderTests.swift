import XCTest

@testable import PokeCalcCore

/// 計算画面の技の並び(G-01。タイプ順だけ)。選択中の技・結果・要求は並びで変わらない。
@MainActor
final class CalcViewModelMoveTypeOrderTests: XCTestCase {
    private func loaded() async -> (CalcViewModel, StubPokeCalcService) {
        let stub = StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.abilityXAndY],
            items: [StubMaster.itemA, StubMaster.itemB])
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return (viewModel, stub)
    }

    func testDisplayedOptionsAreTypeOrderedSameMovesAsMoveOptions() async {
        let (viewModel, _) = await loaded()
        XCTAssertEqual(viewModel.displayedMoveOptions, MoveSort.byType(viewModel.moveOptions))
        XCTAssertEqual(Set(viewModel.displayedMoveOptions.map(\.id)), Set(viewModel.moveOptions.map(\.id)))
        XCTAssertFalse(viewModel.displayedMoveOptions.isEmpty)
    }

    func testDefaultMoveSelectionStaysLearnsetFirstAndReadingOrderDoesNotRecalculate() async {
        let (viewModel, stub) = await loaded()
        XCTAssertEqual(viewModel.moveId, viewModel.moveOptions.first?.id, "既定の技は learnset の先頭のまま")
        let before = await stub.bulkRequests.count
        _ = viewModel.displayedMoveOptions
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before, "表示順の取得だけで再計算しない")
    }
}
