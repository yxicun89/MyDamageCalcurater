import XCTest

@testable import PokeCalcCore

// G-02(ADR-0501「G-02」): 「3 体だけ入れた構築を保存したら画面が全部消えた」の調査用の回帰テスト。
// F-08(ADR-0522)の「6 体・0 体・旧 JSON の往復」と重ならない観点だけを、実物の `LocalTeamStore`
// (UserDefaults の専用 suite)で保存 → 再読込 → 一覧 → 再編集 → 他機能の構築選択まで通して固定する:
// 枠の途中が空(1・3・5 体目だけ)・技 0 個のメンバー・タイプバランスで読んで分析する経路。
@MainActor
final class TeamSparseSaveTests: XCTestCase {
    private nonisolated(unsafe) var suiteName = ""
    private nonisolated(unsafe) var defaults = UserDefaults()
    private var store: LocalTeamStore!

    override func setUpWithError() throws {
        suiteName = "pokecalc-tests-team-sparse-\(UUID().uuidString)"
        defaults = try XCTUnwrap(UserDefaults(suiteName: suiteName))
        store = LocalTeamStore(defaults: defaults)
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suiteName)
    }

    /// 1・3・5 体目だけ埋めた編集画面(技は 0 個のまま)を作って保存する。
    private func saveSparseTeam(teamID: String = "team-sparse") async throws -> TeamEditViewModel {
        let viewModel = TeamEditViewModel(
            store: store, service: StubMaster.makeService(),
            team: Team(id: teamID, name: TeamNaming.defaultName))
        await viewModel.load()
        await viewModel.selectSpecies(slot: 0, speciesKey: StubMaster.alpha.key)
        await viewModel.selectSpecies(slot: 2, speciesKey: StubMaster.beta.key)
        await viewModel.selectSpecies(slot: 4, speciesKey: StubMaster.gamma.key)
        let saved = await viewModel.save()
        XCTAssertTrue(saved)
        XCTAssertNil(viewModel.error)
        return viewModel
    }

    func testSparseTeamWithNoMovesSurvivesStoreRoundTrip() async throws {
        _ = try await saveSparseTeam()
        let teams = try await store.list()
        XCTAssertEqual(teams.count, 1)
        XCTAssertEqual(teams[0].members.map(\.speciesKey), [StubMaster.alpha.key, StubMaster.beta.key, StubMaster.gamma.key])
        XCTAssertTrue(teams[0].members.allSatisfy { $0.moveIds.isEmpty }, "技 0 個のまま保存される")
        let fetched = try await store.get(id: "team-sparse")
        XCTAssertEqual(fetched, teams[0])
    }

    func testSparseTeamShowsInListAfterSave() async throws {
        _ = try await saveSparseTeam()
        let list = TeamListViewModel(store: store, service: StubMaster.makeService())
        await list.load()
        XCTAssertNil(list.error)
        XCTAssertEqual(list.teams.count, 1)
        XCTAssertEqual(list.teams[0].members.count, 3)
        XCTAssertFalse(list.displayName(for: "team-sparse").isEmpty)
    }

    func testReopeningSparseTeamKeepsThreeMembersInSixSlots() async throws {
        _ = try await saveSparseTeam()
        let stored = try await store.get(id: "team-sparse")
        let saved = try XCTUnwrap(stored)
        let reopened = TeamEditViewModel(store: store, service: StubMaster.makeService(), team: saved)
        await reopened.load()
        XCTAssertNil(reopened.error)
        XCTAssertEqual(reopened.team.members.count, 3)
        XCTAssertEqual(reopened.slotIDs.compactMap { $0 }.count, 3)
        XCTAssertEqual(reopened.slotIDs.count, TeamLimits.maxMembers)
        XCTAssertFalse(reopened.hasUnsavedChanges)
        // もう一度保存しても 3 体のまま(再保存で消えない)。
        let again = await reopened.save()
        XCTAssertTrue(again)
        let teams = try await store.list()
        XCTAssertEqual(teams.first?.members.count, 3)
    }

    func testSavingTwiceFromTheSameDraftKeepsOneTeam() async throws {
        let viewModel = try await saveSparseTeam()
        await viewModel.selectSpecies(slot: 1, speciesKey: StubMaster.beta.key)
        let saved = await viewModel.save()
        XCTAssertTrue(saved)
        let teams = try await store.list()
        XCTAssertEqual(teams.count, 1, "同じ id の再保存は追加でなく更新")
        XCTAssertEqual(teams[0].members.count, 4)
    }

    func testSparseTeamAppearsInOtherScreensTeamPicker() async throws {
        _ = try await saveSparseTeam()
        let teams = try await store.list()
        let groups = TeamIndividualOptions.groups(from: teams, species: [])
        XCTAssertEqual(groups.count, 1)
        XCTAssertEqual(groups[0].members.count, 3)
    }

    func testBalanceLoadsSparseTeamAndSendsEmptyMoveIds() async throws {
        _ = try await saveSparseTeam()
        let stored = try await store.get(id: "team-sparse")
        let team = try XCTUnwrap(stored)
        let balance = StubBalanceService()
        let viewModel = BalanceViewModel(balance: balance, master: StubMaster.makeService(), refreshDebounce: .zero)
        await viewModel.load()
        await viewModel.loadTeam(team)
        for _ in 0..<10000 where viewModel.isLoadingAnalysis || viewModel.isLoadingCoverage {
            try? await Task.sleep(for: .milliseconds(1))
        }
        XCTAssertNil(viewModel.masterError)
        XCTAssertEqual(viewModel.members.count, 3)
        let last = await balance.analyzeRequests.last
        XCTAssertEqual(last?.count, 3)
        XCTAssertTrue(last?.allSatisfy { $0.moveIds.isEmpty } ?? false, "技 0 個でも要求は 3 体分出る")
        XCTAssertNotNil(viewModel.analysis)
        XCTAssertNil(viewModel.analysisError)
    }
}
