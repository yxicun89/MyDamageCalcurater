import XCTest

@testable import PokeCalcCore

/// 計算画面の技の並び(F-02)。選択中の技・結果・要求は並びで変わらない。
@MainActor
final class CalcViewModelMoveSortTests: XCTestCase {
    private func makeDefaults() -> UserDefaults {
        let name = "CalcViewModelMoveSortTests-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: name)!
        addTeardownBlock { defaults.removePersistentDomain(forName: name) }
        return defaults
    }

    private func loaded(_ store: MoveSortStore? = nil) async -> (CalcViewModel, StubPokeCalcService) {
        let stub = StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.abilityXAndY],
            items: [StubMaster.itemA, StubMaster.itemB])
        let viewModel = CalcViewModel(service: stub, moveSortStore: store)
        await viewModel.load()
        return (viewModel, stub)
    }

    func testDefaultIsLearnsetAndDisplayedOptionsEqualMoveOptions() async {
        let (viewModel, _) = await loaded()
        XCTAssertEqual(viewModel.moveSortOrder, .learnset)
        XCTAssertEqual(viewModel.displayedMoveOptions, viewModel.moveOptions)
    }

    func testChangingOrderReordersOnlyDisplayAndKeepsSelectionAndDoesNotRecalculate() async {
        let (viewModel, stub) = await loaded()
        let selected = viewModel.moveId
        let requests = await stub.bulkRequests.count
        viewModel.moveSortOrder = .kana
        XCTAssertEqual(viewModel.displayedMoveOptions, MoveSort.sorted(viewModel.moveOptions, by: .kana))
        XCTAssertEqual(Set(viewModel.displayedMoveOptions.map(\.id)), Set(viewModel.moveOptions.map(\.id)))
        XCTAssertEqual(viewModel.moveId, selected, "選択中の技は並びを変えても保つ")
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, requests, "並びの切り替えだけで再計算しない")
    }

    func testChangeIsPersistedAndRestoredByNextViewModel() async {
        let defaults = makeDefaults()
        let (first, _) = await loaded(MoveSortStore(defaults: defaults))
        first.moveSortOrder = .type
        let (second, _) = await loaded(MoveSortStore(defaults: defaults))
        XCTAssertEqual(second.moveSortOrder, .type)
    }

    func testDefaultMoveSelectionStaysLearnsetFirstEvenWithKana() async {
        let defaults = makeDefaults()
        MoveSortStore(defaults: defaults).save(.kana)
        let (kana, _) = await loaded(MoveSortStore(defaults: defaults))
        let (plain, _) = await loaded()
        XCTAssertEqual(kana.moveId, plain.moveId, "既定の技は learnset の先頭のまま(五十音順でも変えない)")
    }
}
