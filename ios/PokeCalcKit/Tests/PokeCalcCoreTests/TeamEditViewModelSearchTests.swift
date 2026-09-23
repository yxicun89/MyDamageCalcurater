import XCTest

@testable import PokeCalcCore

/// 構築編集画面の種族・技の検索(issue #68。ADR-0501「issue #68 の受け入れ条件」)。
///
/// 計算・逆算画面と同じ検索の API を使う。構築特有の確認は2つ:
/// - 保存済みメンバーの種族(先頭ページの外)が `species(key:)` 経由で名前を引けること。
/// - 種族変更で `moveIds` を絞るときに、**learnset の ID 集合**で判定すること
///   (解決できないだけの合法な技を黙って消さない。issue #68 の learnset 側の症状)。
@MainActor
final class TeamEditViewModelSearchTests: XCTestCase {

    private let memberID = "member-search-1"

    // MARK: - 補助

    private func loadedViewModel(
        _ stub: StubPokeCalcService,
        member: TeamMember
    ) async -> TeamEditViewModel {
        let team = Team(id: "team-search-1", name: "テストチーム検索", members: [member])
        let viewModel = TeamEditViewModel(store: StubTeamStore(), service: stub, team: team, searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func member(speciesKey: String, moveIds: [String] = []) -> TeamMember {
        TeamMember(id: memberID, speciesKey: speciesKey, moveIds: moveIds, natureId: StubMaster.neutralNature.id)
    }

    private func searchSpecies(_ viewModel: TeamEditViewModel, _ query: String) async {
        viewModel.setSpeciesQuery(query)
        await viewModel.runSpeciesSearch()
    }

    private func searchMoves(_ viewModel: TeamEditViewModel, _ query: String) async {
        viewModel.setMoveQuery(query)
        await viewModel.runMoveSearch()
    }

    // MARK: - load() が読むのは先頭ページ

    func testLoadFetchesOnlyTheFirstPageOfSpecies() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, member: member(speciesKey: StubBulkMaster.pageSpecies(0).key))

        let calls = await stub.speciesSearchCalls
        XCTAssertEqual(calls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)])
        XCTAssertEqual(viewModel.speciesOptions.count, MasterSearch.pageLimit)
        XCTAssertTrue(viewModel.speciesSearchReachedLimit)
        XCTAssertNil(viewModel.error)
    }

    /// 保存済みの構築のメンバーは、先頭ページの外の種族でも名前を引ける
    /// (`species(key:)` は key 指定なので検索しなくても引ける。その応答も辞書に入れる)。
    func testSavedMemberSpeciesOutsideTheFirstPageStillResolves() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, member: member(speciesKey: StubBulkMaster.hiddenSpecies.key))

        XCTAssertFalse(viewModel.speciesOptions.contains(where: { $0.key == StubBulkMaster.hiddenSpecies.key }))
        XCTAssertEqual(viewModel.speciesSummary(forKey: StubBulkMaster.hiddenSpecies.key)?.nameJa,
                       StubBulkMaster.hiddenSpecies.nameJa)
    }

    // MARK: - 先頭ページの外の種族をメンバーに追加できる

    func testAddingMemberWithSpeciesFoundOnlyBySearch() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, member: member(speciesKey: StubBulkMaster.pageSpecies(0).key))

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key])
        let added = await viewModel.addMember(speciesKey: StubBulkMaster.hiddenSpecies.key)

        XCTAssertTrue(added)
        XCTAssertEqual(viewModel.team.members.map(\.speciesKey).last, StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.speciesSummary(forKey: StubBulkMaster.hiddenSpecies.key)?.nameJa,
                       StubBulkMaster.hiddenSpecies.nameJa)
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 技: 先頭ページの外の技は検索してから技スロットに入れられる

    func testMoveOutsideTheFirstPageBecomesAddableAfterSearching() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, member: member(speciesKey: StubBulkMaster.hiddenSpecies.key))

        XCTAssertEqual(viewModel.moveOptionsByMember[memberID], [], "先頭ページの技と learnset が重ならない")

        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)
        XCTAssertEqual(viewModel.moveOptionsByMember[memberID]?.map(\.id), [StubBulkMaster.hiddenMove.id])

        XCTAssertTrue(viewModel.addMove(id: memberID, moveId: StubBulkMaster.hiddenMove.id))
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [StubBulkMaster.hiddenMove.id])
        XCTAssertNil(viewModel.memberErrors[memberID])
    }

    func testMoveOptionsAreIntersectedWithEachMembersLearnset() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, member: member(speciesKey: StubBulkMaster.pageSpecies(0).key))
        let secondSpecies = StubBulkMaster.pageSpecies(1)
        await viewModel.addMember(speciesKey: secondSpecies.key)
        let secondMemberID = try XCTUnwrap(viewModel.team.members.last?.id)

        // 詰め物の技すべてに当たる語で検索しても、メンバーごとの候補は各自の learnset の分だけ
        await searchMoves(viewModel, StubBulkMaster.pageMove(0).nameJa)
        XCTAssertEqual(viewModel.moveOptionsByMember[memberID]?.map(\.id), [StubBulkMaster.pageMove(0).id])
        XCTAssertEqual(viewModel.moveOptionsByMember[secondMemberID], [],
                       "2体目の learnset には この技が無い")
    }

    // MARK: - 種族変更時の moveIds の絞り込みは learnset の ID 集合で行う

    func testSpeciesChangeKeepsLearnableMovesThatTheCurrentSearchCannotResolve() async throws {
        let stub = StubBulkMaster.makeService()
        let saved = member(
            speciesKey: StubBulkMaster.mixedSpecies.key,
            moveIds: [StubBulkMaster.pageMove(0).id, StubBulkMaster.hiddenMove.id]
        )
        let viewModel = await loadedViewModel(stub, member: saved)

        // 先頭ページの技だけが実体化できる。保存済みの moveIds は読み込みでは落とさない
        XCTAssertEqual(viewModel.moveOptionsByMember[memberID]?.map(\.id), [StubBulkMaster.pageMove(0).id])
        XCTAssertEqual(viewModel.team.members.first?.moveIds,
                       [StubBulkMaster.pageMove(0).id, StubBulkMaster.hiddenMove.id])

        // 新しい種族の learnset は先頭ページの外の技だけ
        await viewModel.setMemberSpecies(id: memberID, speciesKey: StubBulkMaster.hiddenSpecies.key)

        XCTAssertEqual(viewModel.team.members.first?.speciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [StubBulkMaster.hiddenMove.id],
                       "新しい learnset に無い技だけを落とす。実体化できないだけの合法な技は残す")
    }
}
