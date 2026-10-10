import XCTest

@testable import PokeCalcCore

// F-08(ADR-0522): 構築一覧。名前なしで新しい構築を作る・表示名「構築 N」・取り込みで新しい構築を作る・
// 削除の 2 段階(確認中の構築 id)・一覧のアイコン用の種族の解決。
@MainActor
final class TeamListViewModelRebuildTests: XCTestCase {
    private let fixedNow = Date(timeIntervalSince1970: 1_800_000_000)

    private func make(store: StubTeamStore = StubTeamStore(), service: (any PokeCalcService)? = nil) -> TeamListViewModel {
        let now = fixedNow
        return TeamListViewModel(store: store, service: service, now: { now })
    }

    func testCreateTeamWithoutNameUsesTheDefaultNameAndNoMembers() async {
        let store = StubTeamStore()
        let viewModel = make(store: store)
        let created = await viewModel.createTeam()
        XCTAssertEqual(created?.name, TeamNaming.defaultName)
        XCTAssertEqual(created?.members, [])
        XCTAssertEqual(created?.updatedAt, fixedNow)
        XCTAssertEqual(viewModel.teams.count, 1)
        let saves = await store.saveCalls.count
        XCTAssertEqual(saves, 1)
    }

    func testDisplayNamesFollowTheListOrder() async {
        let viewModel = make()
        _ = await viewModel.createTeam()
        _ = await viewModel.createTeam()
        let ids = viewModel.teams.map(\.id)
        XCTAssertEqual(viewModel.displayName(for: ids[0]), "構築 1")
        XCTAssertEqual(viewModel.displayName(for: ids[1]), "構築 2")
        XCTAssertEqual(viewModel.displayName(for: "unknown"), TeamNaming.untitled(1), "未知の id でも落ちない")
    }

    func testLegacyNamedTeamShowsItsSavedName() async {
        let store = StubTeamStore(teams: [Team(id: "old", name: "テストむかしのなまえ")])
        let viewModel = make(store: store)
        await viewModel.load()
        XCTAssertEqual(viewModel.displayName(for: "old"), "テストむかしのなまえ")
    }

    func testCreateTeamFromImportedMembers() async {
        let store = StubTeamStore()
        let viewModel = make(store: store)
        let members = [TeamMember(id: "n1", speciesKey: "9001-000", natureId: "stub-nature-neutral")]
        let created = await viewModel.createTeam(members: members)
        XCTAssertEqual(created?.members.map(\.id), ["n1"])
        XCTAssertEqual(created?.name, TeamNaming.defaultName)
        XCTAssertEqual(viewModel.teams.count, 1)
    }

    func testCreateTeamFromImportRejectsMoreThanSix() async {
        let viewModel = make()
        let members = (0..<7).map { TeamMember(id: "n\($0)", speciesKey: "9001-000", natureId: "x") }
        let created = await viewModel.createTeam(members: members)
        XCTAssertNil(created)
        XCTAssertNotNil(viewModel.error)
    }

    // MARK: 削除の 2 段階

    func testDeleteNeedsConfirmationAndCancelKeepsTheTeam() async {
        let store = StubTeamStore(teams: [Team(id: "t1", name: TeamNaming.defaultName)])
        let viewModel = make(store: store)
        await viewModel.load()
        viewModel.requestDelete(id: "t1")
        XCTAssertEqual(viewModel.pendingDeleteID, "t1")
        let before = await store.deleteCalls
        XCTAssertEqual(before, [], "確認の前は削除しない")
        viewModel.cancelDelete()
        XCTAssertNil(viewModel.pendingDeleteID)
        XCTAssertEqual(viewModel.teams.count, 1)
    }

    func testConfirmDeleteRemovesTheTeam() async {
        let store = StubTeamStore(teams: [Team(id: "t1", name: TeamNaming.defaultName)])
        let viewModel = make(store: store)
        await viewModel.load()
        viewModel.requestDelete(id: "t1")
        await viewModel.confirmDelete()
        XCTAssertNil(viewModel.pendingDeleteID)
        XCTAssertEqual(viewModel.teams, [])
    }

    func testConfirmDeleteWithoutRequestDoesNothing() async {
        let store = StubTeamStore(teams: [Team(id: "t1", name: TeamNaming.defaultName)])
        let viewModel = make(store: store)
        await viewModel.load()
        await viewModel.confirmDelete()
        XCTAssertEqual(viewModel.teams.count, 1)
    }

    // MARK: アイコン用の種族

    func testLoadResolvesSpeciesForIconsAndToleratesFailure() async {
        let service = StubMaster.makeService()
        let viewModel = make(service: service)
        await viewModel.load()
        XCTAssertNotNil(viewModel.speciesSummary(forKey: StubMaster.alpha.key))
        XCTAssertNil(viewModel.speciesSummary(forKey: "unknown"))

        let failing = StubMaster.makeService()
        await failing.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "offline"))
        let other = make(service: failing)
        await other.load()
        XCTAssertNil(other.error, "アイコン用の種族が引けなくても一覧は読める")
    }
}
