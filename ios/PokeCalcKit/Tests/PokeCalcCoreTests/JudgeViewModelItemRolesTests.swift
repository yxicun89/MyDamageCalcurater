import XCTest

@testable import PokeCalcCore

/// 判定画面の持ち物の役割とメガ固定(ADR-0509 §3・§4)。
/// 判定は自分の技で候補を撃ち、候補の技で撃ち返されるので、どの個体も `.any`。
@MainActor
final class JudgeViewModelItemRolesTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private func loadedViewModel() async -> JudgeViewModel {
        let viewModel = JudgeHarness.makeViewModel(master: Mega.makeService())
        await viewModel.load()
        return viewModel
    }

    func testOptionsIncludeEitherRole() async {
        let viewModel = await loadedViewModel()

        XCTAssertEqual(viewModel.selectableItemOptions.map(\.id), [Mega.attackOnly.id, Mega.defenseOnly.id, Mega.both.id])
    }

    func testMegaCandidateLocksStoneAndUnlocksOnChange() async throws {
        let viewModel = await loadedViewModel()
        await viewModel.setSpecies(JudgeHarness.alpha, for: .attacker)
        viewModel.setMove(StubMaster.physicalMove, for: .attacker)
        viewModel.setItem(Mega.both.id, for: .candidate(0))

        await viewModel.setSpecies(SpeciesSummary(detail: Mega.megaAlpha), for: .candidate(0))
        viewModel.setMove(StubMaster.specialMove, for: .candidate(0))

        XCTAssertEqual(viewModel.itemLock(for: .candidate(0)), .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(viewModel.draft(for: .candidate(0))?.itemId, Mega.stone.id)
        viewModel.setItem(Mega.both.id, for: .candidate(0))
        XCTAssertEqual(viewModel.draft(for: .candidate(0))?.itemId, Mega.stone.id, "固定中は変えられない")
        let request = try JudgeHarness.request(viewModel)
        XCTAssertEqual(request.defenders.first?.individual.itemId, Mega.stone.id)
        XCTAssertEqual(viewModel.itemLabel(for: Mega.stone.id), Mega.megaAlphaStoneName)

        await viewModel.setSpecies(JudgeHarness.gamma, for: .candidate(0))
        XCTAssertEqual(viewModel.itemLock(for: .candidate(0)), .none)
        XCTAssertNil(viewModel.draft(for: .candidate(0))?.itemId, "メガストーンを残さない")
    }

    func testMegaAttackerLocksStone() async throws {
        let viewModel = await loadedViewModel()

        await viewModel.setSpecies(SpeciesSummary(detail: Mega.megaAlpha), for: .attacker)

        XCTAssertEqual(viewModel.itemLock(for: .attacker), .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(viewModel.draft(for: .attacker)?.itemId, Mega.stone.id)
    }
}
