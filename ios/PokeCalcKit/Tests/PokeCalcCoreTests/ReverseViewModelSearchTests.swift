import XCTest

@testable import PokeCalcCore

/// 逆算画面の種族・技の検索(issue #68。ADR-0501「issue #68 の受け入れ条件」)。
///
/// `CalcViewModelSearchTests` と同じ規則を逆算画面にも当てる(検索の API は3画面で同じ名前)。
/// 用語: 「自分」= 既知側、「相手」= 逆算する側。与えたダメージ(`side: .defender`)では
/// 攻撃側 = 自分なので、技の候補は自分の learnset から作る。
@MainActor
final class ReverseViewModelSearchTests: XCTestCase {

    // MARK: - 補助

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub, searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func searchSpecies(_ viewModel: ReverseViewModel, _ query: String) async {
        viewModel.setSpeciesQuery(query)
        await viewModel.runSpeciesSearch()
    }

    private func searchMoves(_ viewModel: ReverseViewModel, _ query: String) async {
        viewModel.setMoveQuery(query)
        await viewModel.runMoveSearch()
    }

    private func errorCode(_ error: CalcScreenError?) -> String? {
        guard case .service(let code, _) = error else { return nil }
        return code
    }

    // MARK: - load() が読むのは先頭ページ

    func testLoadFetchesOnlyTheFirstPageOfSpeciesAndMoves() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)

        let speciesCalls = await stub.speciesSearchCalls
        XCTAssertEqual(speciesCalls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)])
        XCTAssertEqual(viewModel.speciesOptions.count, MasterSearch.pageLimit)
        XCTAssertTrue(viewModel.speciesSearchReachedLimit)
        XCTAssertFalse(viewModel.speciesOptions.contains(where: { $0.key == StubBulkMaster.hiddenSpecies.key }))

        let moveCalls = await stub.moveSearchCalls
        XCTAssertEqual(moveCalls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)])
        XCTAssertTrue(viewModel.moveSearchReachedLimit)
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 先頭ページの外の種族を選べる

    func testOpponentSpeciesFoundOnlyBySearchCanBeSelected() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key])
        await viewModel.selectOpponentSpecies(key: StubBulkMaster.hiddenSpecies.key)

        XCTAssertEqual(viewModel.opponentSpeciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.opponentSpecies?.nameJa, StubBulkMaster.hiddenSpecies.nameJa)
        XCTAssertNil(viewModel.error)
    }

    func testSelectedSpeciesNamesStillResolveAfterTheSearchResultsChange() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        await viewModel.selectOpponentSpecies(key: StubBulkMaster.hiddenSpecies.key)
        await searchSpecies(viewModel, StubBulkMaster.pageSpeciesQuery)

        XCTAssertEqual(viewModel.opponentSpecies?.nameJa, StubBulkMaster.hiddenSpecies.nameJa,
                       "検索結果から外れても、相手のカードの名前は引けること")
        XCTAssertEqual(viewModel.mySpecies?.key, viewModel.mySpeciesKey)
        XCTAssertEqual(viewModel.speciesSummary(forKey: StubBulkMaster.hiddenSpecies.key)?.nameJa,
                       StubBulkMaster.hiddenSpecies.nameJa)
    }

    func testSelectingSpeciesThatWasNeverSeenIsStillIgnored() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)
        let myKey = viewModel.mySpeciesKey
        let opponentKey = viewModel.opponentSpeciesKey

        await viewModel.selectMySpecies(key: "stub-species-never-seen")
        await viewModel.selectOpponentSpecies(key: "stub-species-never-seen")

        XCTAssertEqual(viewModel.mySpeciesKey, myKey)
        XCTAssertEqual(viewModel.opponentSpeciesKey, opponentKey)
    }

    // MARK: - 空クエリ

    func testClearingTheQueryRestoresTheFirstPageWithoutAnotherRequest() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)
        let firstPage = viewModel.speciesOptions.map(\.key)

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        let callsAfterSearch = await stub.speciesSearchCalls.count
        await searchSpecies(viewModel, "")

        XCTAssertEqual(viewModel.speciesOptions.map(\.key), firstPage)
        let callsAfterClear = await stub.speciesSearchCalls.count
        XCTAssertEqual(callsAfterClear, callsAfterSearch)
    }

    // MARK: - 技

    func testMoveOutsideTheFirstPageBecomesSelectableAfterSearching() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)

        // 与えたダメージ(自分が攻撃側)で、自分を先頭ページの外の種族にする
        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        await viewModel.selectMySpecies(key: StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.moveOptions, [])
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable)

        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubBulkMaster.hiddenMove.id])

        await viewModel.selectMove(id: StubBulkMaster.hiddenMove.id)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertNil(viewModel.error, "技が選べたら learnset 由来のエラーは消える")
    }

    func testSearchingMovesDoesNotChangeTheSelectedMove() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)
        let selected = viewModel.moveId
        XCTAssertEqual(selected, StubBulkMaster.pageMove(0).id)

        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)

        XCTAssertEqual(viewModel.moveOptions, [], "learnset と重ならない検索結果は候補にしない")
        XCTAssertEqual(viewModel.moveId, selected)
        XCTAssertNil(viewModel.error, "候補が絞られただけでエラーにしない")
    }

    // MARK: - 技の検索応答の追い越し

    func testStaleMoveSearchResponseDoesNotOverwriteTheNewerOne() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)
        await stub.setMoveSearchMode(.manual)

        // 古い検索: learnset と重ならない語(反映されると候補が空になる)
        viewModel.setMoveQuery(StubBulkMaster.hiddenMoveQuery)
        let older = Task { await viewModel.runMoveSearch() }
        try await stub.waitForMoveSearchCalls(count: 2)

        // 新しい検索: 攻撃側が覚える技に当たる語
        let learnedQuery = StubBulkMaster.pageMove(0).nameJa
        viewModel.setMoveQuery(learnedQuery)
        let newer = Task { await viewModel.runMoveSearch() }
        try await stub.waitForMoveSearchCalls(count: 3)

        let newerResult = await stub.matchedMoves(query: learnedQuery, limit: MasterSearch.pageLimit)
        await stub.resolveMoveSearch(at: 2, with: .success(newerResult))
        await newer.value
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubBulkMaster.pageMove(0).id])

        let olderResult = await stub.matchedMoves(query: StubBulkMaster.hiddenMoveQuery, limit: MasterSearch.pageLimit)
        await stub.resolveMoveSearch(at: 1, with: .success(olderResult))
        await older.value

        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubBulkMaster.pageMove(0).id],
                       "追い越された古い技検索の応答は反映しない")
        XCTAssertFalse(viewModel.isSearchingMoves)
    }
}
