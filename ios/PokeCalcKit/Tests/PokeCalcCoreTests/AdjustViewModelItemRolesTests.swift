import XCTest

@testable import PokeCalcCore

/// 調整画面の持ち物の役割とメガ固定(ADR-0509 §3・§4)。
/// モードを選んだ後に変えられるので、自分の持ち物は `.any`(どちらかの役割を持つ持ち物)。
@MainActor
final class AdjustViewModelItemRolesTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private func loadedViewModel() async -> AdjustViewModel {
        let master = Mega.makeService(natures: StubMaster.reverseNatures)
        await master.setMoveBatchMode(.immediate)
        let viewModel = AdjustViewModel(service: master, adjust: StubAdjustService(), searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    func testOwnItemOptionsIncludeEitherRole() async {
        let viewModel = await loadedViewModel()

        XCTAssertEqual(viewModel.ownItemOptions.map(\.id), [Mega.attackOnly.id, Mega.defenseOnly.id, Mega.both.id])
        viewModel.selectOwnItem(id: Mega.noRole.id)
        XCTAssertNil(viewModel.ownItemId, "役割の無い持ち物は選べない")
    }

    func testMegaOwnSpeciesLocksStoneAndUnlocksOnChange() async {
        let viewModel = await loadedViewModel()
        viewModel.selectOwnItem(id: Mega.both.id)

        await viewModel.selectOwnSpecies(key: Mega.megaAlpha.key)

        XCTAssertEqual(viewModel.ownItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(viewModel.ownItemId, Mega.stone.id)
        viewModel.selectOwnItem(id: Mega.both.id)
        XCTAssertEqual(viewModel.ownItemId, Mega.stone.id, "固定中は変えられない")
        XCTAssertEqual(viewModel.itemLabel(for: Mega.stone.id), Mega.megaAlphaStoneName)

        await viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        XCTAssertEqual(viewModel.ownItemLock, .none)
        XCTAssertNil(viewModel.ownItemId, "メガストーンを残さない")
    }
}
