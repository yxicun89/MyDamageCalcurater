import XCTest

@testable import PokeCalcCore

// TeamListViewModel(構築一覧画面の状態。P6-2c・ADR-0501「P6-2c」)。
//
// 決定した形:
// - `@MainActor @Observable final class TeamListViewModel`。`init(store: any TeamStore)`。
// - `teams: [Team]`(private(set))・`isLoading: Bool`(private(set))・`error: TeamScreenError?`(private(set))。
// - `func load() async`: `store.list()` を呼んで `teams` を更新する。`CalcViewModel.load()` と違い
//   **毎回呼び直せる**(一覧画面は他の画面(編集・削除)から戻るたびに最新化する必要があるため、
//   `didLoad` のような一度きりガードは付けない。単発の `list()` 呼び出しだけなので世代トークンも持たない
//   ―― 同時に複数の `load()` が飛ぶ操作(連打)は画面側で起きにくく、CalcViewModel のような
//   「入力のたびに前の要求を追い越す」状況が無いため。over-engineering を避ける)。
// - `func createTeam(name:) async -> Team?`: 前後空白を落として空なら `store.save` を呼ばず nil を返し、
//   `error` に `TeamScreenError.service(code: PokeCalcError.Code.teamNameEmpty, message:)` を立てる。
//   空でなければ新しい `Team(name:)` を `store.save` し、成功したら `load()` して返す(失敗したら nil)。
// - `func deleteTeam(id:) async`: `store.delete(id:)` を呼び、成功でも失敗でも `load()` して最新化する
//   (失敗時は `error` を立てる)。
@MainActor
final class TeamListViewModelTests: XCTestCase {

    private func team(_ id: String, name: String = "テストチーム") -> Team {
        Team(id: id, name: name)
    }

    // MARK: - load

    func testLoadPopulatesTeamsFromStore() async {
        let store = StubTeamStore(teams: [team("team-1"), team("team-2")])
        let viewModel = TeamListViewModel(store: store)
        await viewModel.load()
        XCTAssertEqual(viewModel.teams.map(\.id), ["team-1", "team-2"])
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    func testLoadCanBeCalledAgainToRefresh() async {
        let store = StubTeamStore(teams: [team("team-1")])
        let viewModel = TeamListViewModel(store: store)
        await viewModel.load()
        XCTAssertEqual(viewModel.teams.map(\.id), ["team-1"])

        await store.seed([team("team-1"), team("team-2")])
        await viewModel.load()
        XCTAssertEqual(viewModel.teams.map(\.id), ["team-1", "team-2"], "2回目の load() も反映される")
    }

    func testLoadFailureSetsError() async {
        let store = StubTeamStore()
        await store.setListError(PokeCalcError(code: "test_list_failure", message: "テスト: 一覧の取得に失敗"))
        let viewModel = TeamListViewModel(store: store)
        await viewModel.load()
        XCTAssertEqual(viewModel.teams, [])
        XCTAssertNotNil(viewModel.error)
    }

    // MARK: - createTeam

    func testCreateTeamSavesAndReloadsList() async {
        let store = StubTeamStore()
        let viewModel = TeamListViewModel(store: store)
        await viewModel.load()

        let created = await viewModel.createTeam(name: "テストあたらしいチーム")
        XCTAssertEqual(created?.name, "テストあたらしいチーム")
        XCTAssertEqual(created?.members, [])
        XCTAssertEqual(viewModel.teams.map(\.name), ["テストあたらしいチーム"])
        let saved = await store.saveCalls
        XCTAssertEqual(saved.count, 1)
    }

    /// 前後の空白だけの名前は保存せず、`teamNameEmpty` のエラーを立てる。
    func testCreateTeamRejectsBlankName() async {
        let store = StubTeamStore()
        let viewModel = TeamListViewModel(store: store)

        let created = await viewModel.createTeam(name: "   ")
        XCTAssertNil(created)
        let saveCallCount = await store.saveCalls.count
        XCTAssertEqual(saveCallCount, 0, "store.save を呼ばない")
        guard case .service(let code, _) = viewModel.error else {
            return XCTFail("teamNameEmpty の service エラーになる: \(String(describing: viewModel.error))")
        }
        XCTAssertEqual(code, PokeCalcError.Code.teamNameEmpty)
    }

    func testCreateTeamFailureSetsErrorAndReturnsNil() async {
        let store = StubTeamStore()
        await store.setSaveError(PokeCalcError(code: "test_save_failure", message: "テスト: 保存に失敗"))
        let viewModel = TeamListViewModel(store: store)

        let created = await viewModel.createTeam(name: "テストチーム")
        XCTAssertNil(created)
        XCTAssertNotNil(viewModel.error)
    }

    // MARK: - deleteTeam

    func testDeleteTeamRemovesFromListAndReloads() async {
        let store = StubTeamStore(teams: [team("team-1"), team("team-2")])
        let viewModel = TeamListViewModel(store: store)
        await viewModel.load()

        await viewModel.deleteTeam(id: "team-1")
        XCTAssertEqual(viewModel.teams.map(\.id), ["team-2"])
        let deleteCalls = await store.deleteCalls
        XCTAssertEqual(deleteCalls, ["team-1"])
    }

    func testDeleteTeamFailureSetsError() async {
        let store = StubTeamStore(teams: [team("team-1")])
        let viewModel = TeamListViewModel(store: store)
        await viewModel.load()
        await store.setListError(PokeCalcError(code: "test_delete_reload_failure", message: "テスト"))

        await viewModel.deleteTeam(id: "team-1")
        XCTAssertNotNil(viewModel.error, "delete 後の再読み込みに失敗したらエラーを立てる")
    }
}
