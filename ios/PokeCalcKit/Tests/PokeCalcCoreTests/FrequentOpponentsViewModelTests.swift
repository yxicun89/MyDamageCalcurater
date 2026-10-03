import XCTest

@testable import PokeCalcCore

/// P6-23: `FrequentOpponentsViewModel`(ADR-0501「P6-23」4〜8章)。
/// 名前の解決先は `StubPokeCalcService.species(key:)`(`speciesRequests` で呼び出しを数える)。
@MainActor
final class FrequentOpponentsViewModelTests: XCTestCase {
    private func resolver(count: Int = 12) -> StubPokeCalcService {
        StubPokeCalcService(
            species: (0..<count).map(StubBulkMaster.pageSpecies), moves: [], items: [], natures: [])
    }

    private func key(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).key }
    private func name(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).nameJa }

    private func makeViewModel(
        service: (any FrequentOpponentsService)?, resolver: StubPokeCalcService, limit: Int = FrequentOpponents.defaultLimit,
        maxConcurrent: Int = FrequentOpponents.maxConcurrentResolutions
    ) -> FrequentOpponentsViewModel {
        FrequentOpponentsViewModel(service: service, resolver: resolver, limit: limit, maxConcurrentResolutions: maxConcurrent)
    }

    // MARK: - 定数・文言

    func testConstantsAndLabels() {
        XCTAssertEqual(FrequentOpponents.defaultLimit, 10)
        XCTAssertEqual(FrequentOpponents.limitRange, 1...50)
        XCTAssertTrue(FrequentOpponents.limitRange.contains(FrequentOpponents.defaultLimit))
        XCTAssertEqual(FrequentOpponents.maxConcurrentResolutions, 4)
        XCTAssertEqual(FrequentOpponentsLabels.sectionTitle, "よく使う相手")
    }

    // MARK: - 解決・順序・省略

    func testRefreshRequestsDefaultLimitAndResolvesNamesInServerOrder() async throws {
        let stub = StubFrequentOpponentsService(keys: [key(2), key(0), key(1)])
        let master = resolver()
        let viewModel = makeViewModel(service: stub, resolver: master)
        await viewModel.refresh()

        let limits = await stub.limits
        XCTAssertEqual(limits, [FrequentOpponents.defaultLimit])
        XCTAssertEqual(viewModel.items.map(\.key), [key(2), key(0), key(1)], "サーバーの順を保つ(解決が終わった順ではない)")
        XCTAssertEqual(viewModel.items.map(\.nameJa), [name(2), name(0), name(1)])
        XCTAssertFalse(viewModel.isLoading)
        let species = await master.speciesRequests
        XCTAssertEqual(species.sorted(), [key(0), key(1), key(2)].sorted(), "名前の解決は species(key:) を1件ずつ")
        let searches = await master.speciesSearchCalls
        XCTAssertEqual(searches.count, 0, "検索 API は呼ばない")
    }

    func testCustomLimitIsPassedToService() async throws {
        let stub = StubFrequentOpponentsService(keys: [])
        await makeViewModel(service: stub, resolver: resolver(), limit: 3).refresh()
        let limits = await stub.limits
        XCTAssertEqual(limits, [3])
    }

    func testUnresolvableKeysAreSilentlyOmittedKeepingOrder() async throws {
        let stub = StubFrequentOpponentsService(keys: [key(0), "9999-000", key(1)])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items.map(\.key), [key(0), key(1)], "not_found の key は黙って省く")
    }

    func testResolutionFailuresOfAnyKindAreOmittedWithoutFailingTheRest() async throws {
        let stub = StubFrequentOpponentsService(keys: [key(0), key(1)])
        let master = resolver()
        await master.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト"))
        let viewModel = makeViewModel(service: stub, resolver: master)
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items, [], "全件解決できなければ何も出さない(エラーも出さない)")
    }

    func testAtMostLimitItemsAreResolvedEvenIfServerReturnsMore() async throws {
        let stub = StubFrequentOpponentsService(keys: (0..<12).map(key))
        let master = resolver()
        let viewModel = makeViewModel(service: stub, resolver: master, limit: 5)
        await viewModel.refresh()
        let requested = await master.speciesRequests
        XCTAssertEqual(requested.count, 5, "件数の上限(limit)を超えて species(key:) を呼ばない")
        XCTAssertEqual(viewModel.items.map(\.key), (0..<5).map(key))
    }

    func testDuplicateKeysAreResolvedOnceAndShownOnce() async throws {
        let stub = StubFrequentOpponentsService(keys: [key(0), key(0), key(1)])
        let master = resolver()
        let viewModel = makeViewModel(service: stub, resolver: master)
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items.map(\.key), [key(0), key(1)])
        let requested = await master.speciesRequests
        XCTAssertEqual(requested.count, 2)
    }

    // MARK: - 失敗・空・service なし(絶対ルール5)

    func testServiceFailureShowsNothingAndDoesNotThrow() async throws {
        for code in ["store_unavailable", "upstream_unavailable", "invalid_input", PokeCalcError.Code.transport] {
            let stub = StubFrequentOpponentsService([.failure(PokeCalcError(code: code, message: "テスト"))])
            let master = resolver()
            let viewModel = makeViewModel(service: stub, resolver: master)
            await viewModel.refresh()
            XCTAssertEqual(viewModel.items, [], code)
            XCTAssertFalse(viewModel.isLoading, code)
            let requested = await master.speciesRequests
            XCTAssertEqual(requested, [], "\(code): 取得に失敗したら species(key:) を呼ばない")
        }
    }

    func testEmptyArrayShowsNothing() async throws {
        let master = resolver()
        let viewModel = makeViewModel(service: StubFrequentOpponentsService(keys: []), resolver: master)
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items, [])
        let requested = await master.speciesRequests
        XCTAssertEqual(requested, [])
    }

    func testNilServiceDoesNothing() async throws {
        let master = resolver()
        let viewModel = makeViewModel(service: nil, resolver: master)
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items, [])
        XCTAssertFalse(viewModel.isLoading)
        let requested = await master.speciesRequests
        XCTAssertEqual(requested, [])
    }

    func testFailureAfterSuccessClearsOldItems() async throws {
        let stub = StubFrequentOpponentsService([
            .result([StubFrequentOpponentsService.opponent(key(0))]),
            .failure(PokeCalcError(code: "store_unavailable", message: "テスト")),
        ])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items.map(\.key), [key(0)])
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items, [], "取得に失敗したら古い候補を出し続けない")
    }

    // MARK: - 取得のタイミング(開くたびに再取得)

    func testEachRefreshFetchesAgainAndReplacesItems() async throws {
        let stub = StubFrequentOpponentsService([
            .result([StubFrequentOpponentsService.opponent(key(0))]),
            .result([StubFrequentOpponentsService.opponent(key(1)), StubFrequentOpponentsService.opponent(key(0))]),
        ])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        await viewModel.refresh()
        await viewModel.refresh()
        let limits = await stub.limits
        XCTAssertEqual(limits.count, 2, "シートを開くたびに取得し直す(計算のたびにスコアが変わるため)")
        XCTAssertEqual(viewModel.items.map(\.key), [key(1), key(0)])
    }

    func testPreviousItemsStayVisibleWhileRefreshing() async throws {
        let stub = StubFrequentOpponentsService([.result([StubFrequentOpponentsService.opponent(key(0))]), .manual])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        await viewModel.refresh()
        let task = Task { await viewModel.refresh() }
        try await stub.waitForCalls(count: 2)
        XCTAssertTrue(viewModel.isLoading)
        XCTAssertEqual(viewModel.items.map(\.key), [key(0)], "再取得中は前回の候補を出したまま(ちらつかせない)")
        await stub.resolve(at: 1, with: .success([StubFrequentOpponentsService.opponent(key(1))]))
        await task.value
        XCTAssertEqual(viewModel.items.map(\.key), [key(1)])
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - 古い応答の破棄・キャンセル

    func testStaleServiceResponseIsDiscarded() async throws {
        let stub = StubFrequentOpponentsService([.manual, .manual])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        let first = Task { await viewModel.refresh() }
        try await stub.waitForCalls(count: 1)
        let second = Task { await viewModel.refresh() }
        try await stub.waitForCalls(count: 2)

        await stub.resolve(at: 1, with: .success([StubFrequentOpponentsService.opponent(key(1))]))
        await second.value
        XCTAssertEqual(viewModel.items.map(\.key), [key(1)])

        // 古い呼び出し(0 番目)は新しい呼び出しに無効にされている。cancel 済みなら resolve は XCTFail になるので、
        // cancel されたことを待つ。
        try await stub.waitForCancellation(at: 0)
        await first.value
        XCTAssertEqual(viewModel.items.map(\.key), [key(1)], "古い応答が新しい結果を上書きしない")
    }

    func testStaleNameResolutionIsDiscarded() async throws {
        let stub = StubFrequentOpponentsService([
            .result([StubFrequentOpponentsService.opponent(key(0))]),
            .result([StubFrequentOpponentsService.opponent(key(1))]),
        ])
        let master = resolver()
        await master.setSpeciesMode(.manual)
        let viewModel = makeViewModel(service: stub, resolver: master)
        let first = Task { await viewModel.refresh() }
        try await master.waitForSpeciesRequests(count: 1)
        let second = Task { await viewModel.refresh() }
        try await master.waitForSpeciesRequests(count: 2)

        let detail1 = try await master.lookupSpecies(key: key(1))
        await master.resolveSpecies(at: 1, with: .success(detail1))
        await second.value
        XCTAssertEqual(viewModel.items.map(\.key), [key(1)])
        try await master.waitForSpeciesCancellation(at: 0)
        await first.value
        XCTAssertEqual(viewModel.items.map(\.key), [key(1)], "古い refresh の名前解決は結果を反映しない")
    }

    func testCancellingRefreshWhileFetchingLeavesItemsUntouchedAndStopsLoading() async throws {
        let stub = StubFrequentOpponentsService([.result([StubFrequentOpponentsService.opponent(key(0))]), .manual])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        await viewModel.refresh()
        let task = Task { await viewModel.refresh() }
        try await stub.waitForCalls(count: 2)
        task.cancel()
        await task.value
        XCTAssertEqual(viewModel.items.map(\.key), [key(0)], "キャンセルでは候補を変えない")
        XCTAssertFalse(viewModel.isLoading)
        try await stub.waitForCancellation(at: 1)
    }

    func testCancellingRefreshWhileResolvingCancelsSpeciesRequests() async throws {
        let stub = StubFrequentOpponentsService(keys: [key(0), key(1)])
        let master = resolver()
        await master.setSpeciesMode(.manual)
        let viewModel = makeViewModel(service: stub, resolver: master)
        let task = Task { await viewModel.refresh() }
        try await master.waitForSpeciesRequests(count: 2)
        task.cancel()
        try await master.waitForSpeciesCancellation(at: 0)
        try await master.waitForSpeciesCancellation(at: 1)
        await task.value
        XCTAssertEqual(viewModel.items, [])
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - 同時実行数の上限

    func testNameResolutionRespectsConcurrencyCap() async throws {
        let stub = StubFrequentOpponentsService(keys: (0..<10).map(key))
        let master = resolver()
        await master.setSpeciesMode(.manual)
        let viewModel = makeViewModel(service: stub, resolver: master, maxConcurrent: 3)
        let task = Task { await viewModel.refresh() }

        try await master.waitForSpeciesRequests(count: 3)
        try await Task.sleep(for: .milliseconds(50))
        var requested = await master.speciesRequests
        XCTAssertEqual(requested.count, 3, "同時に飛ばす species(key:) は上限(3)まで")

        // 1件返すと次の1件が始まる(常に上限以内)。
        let first = try await master.lookupSpecies(key: requested[0])
        await master.resolveSpecies(at: 0, with: .success(first))
        try await master.waitForSpeciesRequests(count: 4)
        try await Task.sleep(for: .milliseconds(50))
        requested = await master.speciesRequests
        XCTAssertEqual(requested.count, 4)

        // 残りをすべて返して終える。
        var index = 1
        while index < 10 {
            try await master.waitForSpeciesRequests(count: index + 1)
            let detail = try await master.lookupSpecies(key: await master.speciesRequests[index])
            await master.resolveSpecies(at: index, with: .success(detail))
            index += 1
        }
        await task.value
        XCTAssertEqual(viewModel.items.map(\.key), (0..<10).map(key), "上限があっても順序はサーバーの順")
    }

    // MARK: - 検索中は出さない

    func testVisibleItemsOnlyForEmptyQuery() async throws {
        let stub = StubFrequentOpponentsService(keys: [key(0), key(1)])
        let viewModel = makeViewModel(service: stub, resolver: resolver())
        await viewModel.refresh()
        XCTAssertEqual(viewModel.visibleItems(forQuery: "").map(\.key), [key(0), key(1)])
        XCTAssertEqual(viewModel.visibleItems(forQuery: "  ").map(\.key), [key(0), key(1)], "空白だけは空クエリ扱い")
        XCTAssertEqual(viewModel.visibleItems(forQuery: "テ"), [], "検索中は出さない")
        XCTAssertEqual(viewModel.visibleItems(forQuery: " テ "), [])
    }

    // MARK: - 既存の呼び出し回数を変えない

    func testExistingViewModelsNeverCallFrequentOpponents() async throws {
        let frequent = StubFrequentOpponentsService(keys: [key(0)])
        let master = StubMaster.makeService()
        let calc = CalcViewModel(service: master, searchDebounce: .zero)
        await calc.load()
        let reverse = ReverseViewModel(service: master, searchDebounce: .zero)
        await reverse.load()
        let calls = await frequent.limits
        XCTAssertEqual(calls, [], "計算・逆算の ViewModel は FrequentOpponentsService に依存しない")
    }

    func testRefreshDoesNotTouchSearchOrOtherMasterCalls() async throws {
        let master = StubMaster.makeService()
        let viewModel = FrequentOpponentsViewModel(
            service: StubFrequentOpponentsService(keys: [StubMaster.alpha.key]), resolver: master)
        await viewModel.refresh()
        let searches = await master.speciesSearchCalls
        let moves = await master.moveSearchCalls
        let requests = await master.speciesRequests
        XCTAssertEqual(searches.count, 0)
        XCTAssertEqual(moves.count, 0)
        XCTAssertEqual(requests, [StubMaster.alpha.key])
    }
}
