import XCTest

@testable import PokeCalcCore

// P6-20: 取り込んだメンバーを構築編集の ViewModel へ追加する(`TeamEditViewModel.importMembers`。
// ADR-0501「P6-20」5章)。保存は伴わない(`addMember` と同じ。保存は画面の「保存」)。
@MainActor
final class TeamEditViewModelImportTests: XCTestCase {
    private func loadedViewModel(
        existing: Int = 1, service: StubPokeCalcService = StubMaster.makeService(), store: StubTeamStore = StubTeamStore()
    ) async -> TeamEditViewModel {
        let members = (0..<existing).map {
            TeamMember(id: "existing-\($0)", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")
        }
        let viewModel = TeamEditViewModel(store: store, service: service, team: Team(id: "team-1", name: "テストチーム", members: members))
        await viewModel.load()
        return viewModel
    }

    private func imported(_ id: String, species: String = "9102-000", moves: [String] = ["stub-move-physical"]) -> TeamMember {
        TeamMember(
            id: id, speciesKey: species, moveIds: moves, itemId: "stub-item-a", abilityId: "stub-ability",
            natureId: "stub-nature-spa-up", sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 2), teraType: .fire
        )
    }

    func testImportAppendsMembersWithoutSavingAndPreparesTheirOptions() async {
        let store = StubTeamStore()
        let viewModel = await loadedViewModel(store: store)
        let before = viewModel.team.members
        let count = await viewModel.importMembers([imported("n1"), imported("n2", species: "9103-000")])
        XCTAssertEqual(count, 2)
        XCTAssertEqual(viewModel.team.members.map(\.id), before.map(\.id) + ["n1", "n2"], "末尾に追加し、既存は変えない")
        XCTAssertEqual(viewModel.team.members.last?.speciesKey, "9103-000")
        XCTAssertEqual(viewModel.team.members.dropFirst().first, imported("n1"), "取り込んだ値をそのまま持つ")
        XCTAssertNil(viewModel.teamError)
        XCTAssertEqual(viewModel.abilityOptionsByMember["n1"]?.map(\.id), [StubMaster.ability.id])
        XCTAssertFalse(viewModel.moveOptionsByMember["n1"]?.isEmpty ?? true)
        let saves = await store.saveCalls
        XCTAssertEqual(saves, [], "取り込みだけでは保存しない")

        let saved = await viewModel.save()
        XCTAssertTrue(saved)
        let afterSave = await store.saveCalls
        XCTAssertEqual(afterSave.count, 1)
        XCTAssertEqual(afterSave.first?.members.map(\.id), before.map(\.id) + ["n1", "n2"])
    }

    func testImportStopsAtTheMemberLimit() async {
        let viewModel = await loadedViewModel(existing: TeamLimits.maxMembers - 1)
        let count = await viewModel.importMembers([imported("n1"), imported("n2"), imported("n3")])
        XCTAssertEqual(count, 1)
        XCTAssertEqual(viewModel.team.members.count, TeamLimits.maxMembers)
        XCTAssertEqual(viewModel.team.members.last?.id, "n1", "先頭から入れられるだけ入れる")
        XCTAssertEqual(viewModel.teamError, .tooManyMembers)
    }

    func testImportIntoAFullTeamAddsNothing() async {
        let viewModel = await loadedViewModel(existing: TeamLimits.maxMembers)
        let count = await viewModel.importMembers([imported("n1")])
        XCTAssertEqual(count, 0)
        XCTAssertEqual(viewModel.team.members.count, TeamLimits.maxMembers)
        XCTAssertEqual(viewModel.teamError, .tooManyMembers)
    }

    func testImportingNothingChangesNothing() async {
        let viewModel = await loadedViewModel()
        let before = viewModel.team
        let count = await viewModel.importMembers([])
        XCTAssertEqual(count, 0)
        XCTAssertEqual(viewModel.team, before)
        XCTAssertNil(viewModel.teamError)
    }

    /// 通信に失敗しても、追加したメンバーも既存のメンバーも消えない(選択肢が足りないだけ)。
    func testCommunicationFailureKeepsAddedAndExistingMembers() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(service: stub)
        let before = viewModel.team.members
        await stub.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "down"))
        let count = await viewModel.importMembers([imported("n1")])
        XCTAssertEqual(count, 1)
        XCTAssertEqual(viewModel.team.members.map(\.id), before.map(\.id) + ["n1"])
        XCTAssertNotNil(viewModel.error)
    }
}
