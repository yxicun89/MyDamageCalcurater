import XCTest

@testable import PokeCalcCore

/// 構築編集画面: 保存済みメンバーの技(先頭ページの外)の名前を `move(id:)`(openapi `getMove`)で解決する
/// (issue #68 の残り。ADR-0501「issue #68 の残り: getMove による選択中の技の解決」5章)。
///
/// 壊れていたこと: 技スロットの表示名は「直近の技検索の結果 ∩ learnset」(`moveOptionsByMember`)から引いて
/// いたため、先頭ページの外の技は ID のまま出る。さらに検索語を変えると、先頭ページの技まで ID に化ける。
/// `move(forID:)` は一度でも見た技(先頭ページ・検索結果・`move(id:)` の応答)の辞書から引く。
@MainActor
final class TeamEditViewModelMoveLookupTests: XCTestCase {

    private let memberID = "member-lookup-1"

    // MARK: - 補助

    private func loadedViewModel(_ stub: StubPokeCalcService, moveIds: [String]) async -> TeamEditViewModel {
        let member = TeamMember(
            id: memberID, speciesKey: StubBulkMaster.mixedSpecies.key, moveIds: moveIds, natureId: StubMaster.neutralNature.id
        )
        let team = Team(id: "team-lookup-1", name: "テストチーム技解決", members: [member])
        let viewModel = TeamEditViewModel(store: StubTeamStore(), service: stub, team: team, searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func searchMoves(_ viewModel: TeamEditViewModel, _ query: String) async {
        viewModel.setMoveQuery(query)
        await viewModel.runMoveSearch()
    }

    // MARK: - 保存済みの技の名前を解決する

    func testSavedMoveOutsideTheFirstPageIsResolvedByIDOnLoad() async throws {
        let stub = StubBulkMaster.makeService()
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub, moveIds: [StubBulkMaster.pageMove(0).id, StubBulkMaster.hiddenMove.id])

        XCTAssertEqual(viewModel.move(forID: StubBulkMaster.hiddenMove.id)?.nameJa, StubBulkMaster.hiddenMove.nameJa,
                       "先頭ページの外の保存済みの技も、ID ではなく名前で出せる")
        XCTAssertEqual(viewModel.move(forID: StubBulkMaster.pageMove(0).id)?.nameJa, StubBulkMaster.pageMove(0).nameJa)
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id], "先頭ページで既に見た技は引き直さない")
        XCTAssertEqual(viewModel.team.members.first?.moveIds,
                       [StubBulkMaster.pageMove(0).id, StubBulkMaster.hiddenMove.id], "読み込みで moveIds を変えない")
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    func testNoLookupWhenEverySavedMoveIsAlreadyKnown() async throws {
        let stub = StubBulkMaster.makeService()
        await stub.setMoveLookupMode(.immediate)

        _ = await loadedViewModel(stub, moveIds: [StubBulkMaster.pageMove(0).id])

        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [], "learnset 全件ではなく、保存済みの技のうち未知のものだけを解決する")
    }

    // MARK: - 失敗したら ID のまま(今日の振る舞い)

    func testLookupNotFoundLeavesTheIDUnresolvedWithoutAScreenError() async throws {
        let stub = StubBulkMaster.makeService()
        // 既定の `.notFound`

        let viewModel = await loadedViewModel(stub, moveIds: [StubBulkMaster.hiddenMove.id])

        XCTAssertNil(viewModel.move(forID: StubBulkMaster.hiddenMove.id), "解決できなければ nil(View は ID をそのまま出す)")
        XCTAssertNil(viewModel.error, "補助の解決の失敗で編集画面を止めない")
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [StubBulkMaster.hiddenMove.id], "解決できない技も消さない")
        XCTAssertEqual(viewModel.abilityOptionsByMember[memberID], StubBulkMaster.mixedSpecies.abilities,
                       "技の解決に失敗しても、メンバーの読み込み自体は終わっている")
    }

    func testLookupTransportFailureLeavesTheIDUnresolvedWithoutAScreenError() async throws {
        let stub = StubBulkMaster.makeService()
        await stub.setMoveLookupMode(.immediate)
        await stub.setMoveLookupError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト: 通信できない"))

        let viewModel = await loadedViewModel(stub, moveIds: [StubBulkMaster.hiddenMove.id])

        XCTAssertNil(viewModel.move(forID: StubBulkMaster.hiddenMove.id))
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - 検索語を変えても技の名前が消えない

    func testKnownMoveNamesSurviveANarrowingMoveSearch() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, moveIds: [StubBulkMaster.pageMove(0).id])

        // learnset の先頭ページの技と重ならない語で検索 → 候補からは消える
        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)
        XCTAssertEqual(viewModel.moveOptionsByMember[memberID]?.map(\.id), [StubBulkMaster.hiddenMove.id])

        XCTAssertEqual(viewModel.move(forID: StubBulkMaster.pageMove(0).id)?.nameJa, StubBulkMaster.pageMove(0).nameJa,
                       "技スロットの名前は検索結果ではなく一度でも見た技から引く")
        XCTAssertEqual(viewModel.move(forID: StubBulkMaster.hiddenMove.id)?.nameJa, StubBulkMaster.hiddenMove.nameJa,
                       "検索で見た技も辞書に入る")
        XCTAssertNil(viewModel.move(forID: "stub-move-never-seen"))
    }

    // MARK: - 持ち物の一覧が上限に達したら黙って切り捨てない

    func testItemOptionsReachingThePageLimitAreFlagged() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubBulkMaster.pageSpecies(0), items: StubMoveLookupMaster.pageItemList)
        let viewModel = await loadedViewModel(stub, moveIds: [])

        XCTAssertEqual(viewModel.itemOptions.count, MasterSearch.pageLimit)
        XCTAssertTrue(viewModel.itemOptionsReachedLimit)
    }

    func testItemOptionsBelowThePageLimitAreNotFlagged() async throws {
        let viewModel = await loadedViewModel(StubBulkMaster.makeService(), moveIds: [])
        XCTAssertFalse(viewModel.itemOptionsReachedLimit)
    }
}
