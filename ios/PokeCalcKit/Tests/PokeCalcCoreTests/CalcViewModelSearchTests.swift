import XCTest

@testable import PokeCalcCore

/// ダメージ計算画面の種族・技の検索(issue #68。ADR-0501「issue #68 の受け入れ条件」)。
///
/// 壊れていたこと: `load()` が `searchSpecies(query: "", limit: 200)` の結果を**マスタ全件**として扱い、
/// 201件目以降の種族・技が恒久的に選べなかった(learnset が先頭200件の外の技しか持たない種族は
/// `moveUnavailable` で計算不能)。
///
/// サービスは `StubBulkMaster.makeService()`(先頭ページ `MasterSearch.pageLimit` 件 + その外に
/// `mixedSpecies` / `hiddenSpecies` / `hiddenMove`)。
@MainActor
final class CalcViewModelSearchTests: XCTestCase {

    /// デバウンスを実際に働かせる必要があるテストだけが使う待ち時間(値そのものに意味は無い。
    /// 「打ち直しが前の検索を追い越す」のを確実にするために0より大きければよい)。
    private let observableDebounce: Duration = .milliseconds(50)

    // MARK: - 補助

    private func loadedViewModel(_ stub: StubPokeCalcService, debounce: Duration = .zero) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub, searchDebounce: debounce)
        await viewModel.load()
        return viewModel
    }

    private func searchSpecies(_ viewModel: CalcViewModel, _ query: String) async {
        viewModel.setSpeciesQuery(query)
        await viewModel.runSpeciesSearch()
    }

    private func searchMoves(_ viewModel: CalcViewModel, _ query: String) async {
        viewModel.setMoveQuery(query)
        await viewModel.runMoveSearch()
    }

    private func errorCode(_ error: CalcScreenError?) -> String? {
        guard case .service(let code, _) = error else { return nil }
        return code
    }

    // MARK: - 1. load() が読むのは「全件」ではなく「先頭ページ」

    func testLoadFetchesOnlyTheFirstPageAndMarksItAsReachingTheLimit() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        let speciesCalls = await stub.speciesSearchCalls
        XCTAssertEqual(speciesCalls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)],
                       "起動時の種族取得は空クエリの先頭ページ1回だけ")
        XCTAssertEqual(viewModel.speciesOptions.count, MasterSearch.pageLimit)
        XCTAssertFalse(viewModel.speciesOptions.contains(where: { $0.key == StubBulkMaster.hiddenSpecies.key }),
                       "先頭ページの外の種族は空クエリでは返らない(これが issue #68 の前提)")
        XCTAssertTrue(viewModel.speciesSearchReachedLimit, "上限に達した = まだ他にあるかもしれない、と画面に伝える")

        let moveCalls = await stub.moveSearchCalls
        XCTAssertEqual(moveCalls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)])
        XCTAssertTrue(viewModel.moveSearchReachedLimit)
        XCTAssertNil(viewModel.error)
    }

    func testDefaultDebounceIntervalIsNotZero() {
        XCTAssertGreaterThan(MasterSearch.debounceInterval, .zero,
                             "既定では1キーストロークごとにそのまま API を叩かない(#113 が基盤に置き換えるまでの素朴なデバウンス)")
    }

    func testSearchLabelsAreDistinctAndNotEmpty() {
        let labels = [MasterSearchLabels.prompt, MasterSearchLabels.truncated, MasterSearchLabels.noMatch]
        XCTAssertEqual(Set(labels).count, labels.count, "案内文言は3種類とも別の文言")
        XCTAssertFalse(labels.contains(where: \.isEmpty))
    }

    // MARK: - 2. 検索語が q として渡り、結果が絞られる

    func testSpeciesQueryIsSentAsTheSearchQueryAndNarrowsTheResults() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)

        let calls = await stub.speciesSearchCalls
        XCTAssertEqual(calls.last, StubPokeCalcService.SearchCall(query: StubBulkMaster.hiddenSpeciesQuery, limit: MasterSearch.pageLimit))
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key])
        XCTAssertFalse(viewModel.speciesSearchReachedLimit)
        XCTAssertFalse(viewModel.isSearchingSpecies)
    }

    func testSetSpeciesQueryReportsWhetherASearchIsNeeded() async {
        let viewModel = await loadedViewModel(StubBulkMaster.makeService())
        XCTAssertTrue(viewModel.setSpeciesQuery(StubBulkMaster.hiddenSpeciesQuery), "語が変わったら検索が要る")
        XCTAssertEqual(viewModel.speciesQuery, StubBulkMaster.hiddenSpeciesQuery, "文字の反映は同期(TextField の Binding 用)")
        XCTAssertFalse(viewModel.setSpeciesQuery(StubBulkMaster.hiddenSpeciesQuery), "同じ語なら検索しない")
    }

    // MARK: - 3. 先頭ページの外の種族を選べる(issue #68 の再現手順の解消)

    func testDefenderFoundOnlyBySearchCanBeSelectedAndCalculated() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let callsBefore = await stub.bulkRequests.count

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        await viewModel.selectDefender(speciesKey: StubBulkMaster.hiddenSpecies.key)

        XCTAssertEqual(viewModel.defenderSpeciesKey, StubBulkMaster.hiddenSpecies.key)
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, callsBefore + 1, "選択のたびに計算はちょうど1回")
        XCTAssertEqual(requests.last?.defenderSpeciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 4. 選択中の種族は検索結果が変わっても名前を引ける

    func testSelectedSpeciesStillResolvesAfterTheSearchResultsChange() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        await viewModel.selectDefender(speciesKey: StubBulkMaster.hiddenSpecies.key)
        // 検索語を変えると、選択中の種族は検索結果からいなくなる
        await searchSpecies(viewModel, StubBulkMaster.pageSpeciesQuery)

        XCTAssertFalse(viewModel.speciesOptions.contains(where: { $0.key == StubBulkMaster.hiddenSpecies.key }))
        XCTAssertEqual(viewModel.defenderSpecies?.nameJa, StubBulkMaster.hiddenSpecies.nameJa,
                       "カードのヘッダーは検索結果ではなく「一度でも見た種族」から引く")
        XCTAssertEqual(viewModel.attackerSpecies?.key, viewModel.attackerSpeciesKey)
        XCTAssertEqual(viewModel.speciesSummary(forKey: StubBulkMaster.hiddenSpecies.key)?.nameJa,
                       StubBulkMaster.hiddenSpecies.nameJa)
        XCTAssertNil(viewModel.speciesSummary(forKey: "stub-species-never-seen"))
    }

    // MARK: - 5. 空クエリは先頭ページに戻る(取り直さない)

    func testClearingTheQueryRestoresTheFirstPageWithoutAnotherRequest() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let firstPage = viewModel.speciesOptions.map(\.key)

        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        let callsAfterSearch = await stub.speciesSearchCalls.count
        await searchSpecies(viewModel, "")

        XCTAssertEqual(viewModel.speciesOptions.map(\.key), firstPage, "空欄では起動時の先頭ページに戻る")
        let callsAfterClear = await stub.speciesSearchCalls.count
        XCTAssertEqual(callsAfterClear, callsAfterSearch, "空欄で「全件(上限まで)」を取り直さない")
    }

    // MARK: - 6. 知らない key は無視する(既存の規則を弱めない)

    func testSelectingSpeciesThatWasNeverSeenIsStillIgnored() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let before = (key: viewModel.attackerSpeciesKey, calls: await stub.bulkRequests.count)

        await viewModel.selectAttacker(speciesKey: "stub-species-never-seen")

        XCTAssertEqual(viewModel.attackerSpeciesKey, before.key)
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before.calls, "選べない種族では計算しない")
    }

    // MARK: - 7. 検索応答の追い越し(世代の保護を検索にも広げる)

    func testStaleSpeciesSearchResponseDoesNotOverwriteTheNewerOne() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await stub.setSpeciesSearchMode(.manual)

        viewModel.setSpeciesQuery(StubBulkMaster.pageSpeciesQuery)
        let older = Task { await viewModel.runSpeciesSearch() }
        try await stub.waitForSpeciesSearchCalls(count: 2)

        viewModel.setSpeciesQuery(StubBulkMaster.hiddenSpeciesQuery)
        let newer = Task { await viewModel.runSpeciesSearch() }
        try await stub.waitForSpeciesSearchCalls(count: 3)

        // 新しい検索の応答が先に届く
        let newerResult = await stub.matchedSpecies(query: StubBulkMaster.hiddenSpeciesQuery, limit: MasterSearch.pageLimit)
        await stub.resolveSpeciesSearch(at: 2, with: .success(newerResult))
        await newer.value
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key])

        // 古い検索の応答が後から届いても上書きしない
        let olderResult = await stub.matchedSpecies(query: StubBulkMaster.pageSpeciesQuery, limit: MasterSearch.pageLimit)
        await stub.resolveSpeciesSearch(at: 1, with: .success(olderResult))
        await older.value
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key],
                       "追い越された古い検索の応答は反映しない")
        XCTAssertFalse(viewModel.isSearchingSpecies)
    }

    func testStaleSpeciesSearchFailureDoesNotReplaceTheNewerResult() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await stub.setSpeciesSearchMode(.manual)

        viewModel.setSpeciesQuery(StubBulkMaster.pageSpeciesQuery)
        let older = Task { await viewModel.runSpeciesSearch() }
        try await stub.waitForSpeciesSearchCalls(count: 2)
        viewModel.setSpeciesQuery(StubBulkMaster.hiddenSpeciesQuery)
        let newer = Task { await viewModel.runSpeciesSearch() }
        try await stub.waitForSpeciesSearchCalls(count: 3)

        let newerResult = await stub.matchedSpecies(query: StubBulkMaster.hiddenSpeciesQuery, limit: MasterSearch.pageLimit)
        await stub.resolveSpeciesSearch(at: 2, with: .success(newerResult))
        await newer.value
        await stub.resolveSpeciesSearch(at: 1, with: .failure(PokeCalcError(code: "upstream_unavailable", message: "テスト: 古い検索の失敗")))
        await older.value

        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key])
        XCTAssertNil(viewModel.error, "追い越された古い検索の失敗でエラーを立てない")
    }

    // MARK: - 8. 打ち直しでは最後の1回だけ要求が飛ぶ(デバウンス)

    func testRapidQueryChangesIssueOnlyTheLatestSearch() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, debounce: observableDebounce)
        let callsAfterLoad = await stub.speciesSearchCalls.count

        viewModel.setSpeciesQuery(StubBulkMaster.pageSpeciesQuery)
        let first = Task { await viewModel.runSpeciesSearch() }
        viewModel.setSpeciesQuery(StubBulkMaster.hiddenSpeciesQuery)
        let second = Task { await viewModel.runSpeciesSearch() }
        await first.value
        await second.value

        let calls = await stub.speciesSearchCalls
        XCTAssertEqual(calls.count, callsAfterLoad + 1, "デバウンス中に打ち直した分の要求は飛ばない")
        XCTAssertEqual(calls.last?.query, StubBulkMaster.hiddenSpeciesQuery)
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubBulkMaster.hiddenSpecies.key])
    }

    // MARK: - 9. 技: 先頭ページの外の技は検索してから選べる

    func testMoveOptionsAreTheLatestMoveSearchIntersectedWithTheLearnset() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        // 起動時: 攻撃側 = 先頭ページの0番目。learnset は同じ index の技1つだけ
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubBulkMaster.pageMove(0).id])

        // learnset と重ならない語で検索しても、選択中の技・計算の入力は壊れない(候補が絞られるだけ)
        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)

        XCTAssertEqual(viewModel.moveOptions, [], "learnset と重ならない検索結果は候補にしない")
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(0).id, "検索は選択中の技を変えない")
        XCTAssertEqual(viewModel.selectedMove?.id, StubBulkMaster.pageMove(0).id, "選択中の技は検索結果ではなく辞書から引く")
        XCTAssertNil(viewModel.error)
    }

    func testMoveOutsideTheFirstPageBecomesSelectableAfterSearching() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        // 先頭ページの外の技しか覚えない種族に変えると、いまの技候補では解決できない(部分的な穴。ADR 6章)
        await searchSpecies(viewModel, StubBulkMaster.hiddenSpeciesQuery)
        await viewModel.selectAttacker(speciesKey: StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.moveOptions, [])
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable)

        // 技名で検索すれば候補に出る(learnset の ID 集合と突き合わせる)
        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubBulkMaster.hiddenMove.id])

        let callsBefore = await stub.bulkRequests.count
        await viewModel.selectMove(id: StubBulkMaster.hiddenMove.id)

        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id)
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, callsBefore + 1)
        XCTAssertEqual(requests.last?.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertEqual(requests.last?.attacker.speciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertNil(viewModel.error, "計算が通ったら learnset 由来のエラーは消える")
    }

    func testMoveQueryIsSentAsTheSearchQuery() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)

        let calls = await stub.moveSearchCalls
        XCTAssertEqual(calls.last, StubPokeCalcService.SearchCall(query: StubBulkMaster.hiddenMoveQuery, limit: MasterSearch.pageLimit))
        XCTAssertFalse(viewModel.moveSearchReachedLimit)
        XCTAssertFalse(viewModel.isSearchingMoves)
    }

    func testSelectingMoveOutsideTheCurrentOptionsIsStillIgnored() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let before = await stub.bulkRequests.count

        // 攻撃側(先頭ページの0番目)が覚えない技は、検索で見つけても選べない
        await searchMoves(viewModel, StubBulkMaster.hiddenMoveQuery)
        await viewModel.selectMove(id: StubBulkMaster.hiddenMove.id)

        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(0).id)
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before, "learnset に無い技では計算しない")
    }
}
