import XCTest

@testable import PokeCalcCore

/// `FavoritesViewModel`(一覧・外す。ADR-0511)。名前の解決先は `StubPokeCalcService.species(key:)`。
@MainActor
final class FavoritesViewModelTests: XCTestCase {
    private func resolver(count: Int = 6) -> StubPokeCalcService {
        StubPokeCalcService(species: (0..<count).map(StubBulkMaster.pageSpecies), moves: [], items: [], natures: [])
    }

    private func key(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).key }
    private func name(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).nameJa }

    private func fav(_ id: String, _ index: Int, label: String? = nil) -> Favorite {
        StubFavoritesService.favorite(id, key: key(index), label: label)
    }

    private func makeViewModel(
        _ service: (any FavoritesService)?, master: StubPokeCalcService? = nil
    ) -> FavoritesViewModel {
        FavoritesViewModel(service: service, resolver: master ?? resolver())
    }

    private static let transportError = PokeCalcError(code: PokeCalcError.Code.transport, message: "test")
    private static let notFoundError = PokeCalcError(code: "not_found", message: "test")

    // MARK: - 読み込み

    func testLoadResolvesNamesKeepingServerOrder() async {
        let stub = StubFavoritesService(list: [.result([fav("30", 2, label: "HB特化"), fav("20", 0), fav("10", 1)])])
        let viewModel = makeViewModel(stub)
        XCTAssertEqual(viewModel.loadState, .idle)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.id), ["30", "20", "10"], "サーバーの順を保つ(解決が終わった順ではない)")
        XCTAssertEqual(viewModel.items.map(\.speciesName), [name(2), name(0), name(1)])
    }

    func testRowTitleAndSubtitle() {
        let withLabel = FavoriteRow(favorite: StubFavoritesService.favorite("1", label: "HB特化"), speciesName: "テストモンいち")
        XCTAssertEqual(withLabel.title, "HB特化")
        XCTAssertEqual(withLabel.subtitle, "テストモンいち")
        let withoutLabel = FavoriteRow(favorite: StubFavoritesService.favorite("2"), speciesName: "テストモンいち")
        XCTAssertEqual(withoutLabel.title, "テストモンいち")
        XCTAssertNil(withoutLabel.subtitle)
        let unknownNoLabel = FavoriteRow(favorite: StubFavoritesService.favorite("3"), speciesName: nil)
        XCTAssertEqual(unknownNoLabel.title, FavoritesLabels.unknownSpecies)
        XCTAssertNil(unknownNoLabel.subtitle)
        let unknownWithLabel = FavoriteRow(favorite: StubFavoritesService.favorite("4", label: "名前"), speciesName: nil)
        XCTAssertEqual(unknownWithLabel.title, "名前")
        XCTAssertEqual(unknownWithLabel.subtitle, FavoritesLabels.unknownSpecies)
    }

    func testUnresolvableSpeciesKeepsRowAndDoesNotFailLoad() async {
        let stub = StubFavoritesService(list: [.result([fav("2", 0), StubFavoritesService.favorite("1", key: "9999-000")])])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.id), ["2", "1"])
        XCTAssertEqual(viewModel.items.map(\.speciesName), [name(0), nil], "引けない種族は行を残して名前だけ nil")
    }

    func testMasterFailureDoesNotFailLoad() async {
        let master = resolver()
        await master.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "test"))
        let viewModel = makeViewModel(StubFavoritesService(list: [.result([fav("1", 0)])]), master: master)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded, "名前が引けなくてもお気に入り自体は出す")
        XCTAssertEqual(viewModel.items.map(\.id), ["1"])
        XCTAssertEqual(viewModel.items.first?.speciesName, nil)
    }

    func testEmptyListIsLoadedNotFailure() async {
        let viewModel = makeViewModel(StubFavoritesService(list: [.result([])]))
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items, [])
    }

    func testLoadFailureSetsFailedStateWithoutThrowingAndRetryRecovers() async {
        let stub = StubFavoritesService(list: [.failure(Self.transportError), .result([fav("1", 0)])])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.transport))
        XCTAssertEqual(viewModel.items, [])
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.id), ["1"])
    }

    func testStoreUnavailableMapsToStoreUnavailableState() async {
        let stub = StubFavoritesService(list: [.failure(PokeCalcError(code: "store_unavailable", message: "x"))])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.storeUnavailable))
    }

    func testReloadFailureKeepsPreviousItems() async {
        let stub = StubFavoritesService(list: [.result([fav("1", 0)]), .failure(Self.transportError)])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.transport))
        XCTAssertEqual(viewModel.items.map(\.id), ["1"], "再読み込みの失敗で表示中の一覧を消さない")
    }

    func testNilServiceDoesNothing() async {
        let viewModel = makeViewModel(nil)
        await viewModel.load()
        await viewModel.remove(id: "1")
        XCTAssertEqual(viewModel.loadState, .idle)
        XCTAssertEqual(viewModel.items, [])
        XCTAssertNil(viewModel.actionError)
    }

    func testIsAtLimitUsesRequestLimits() async {
        let underLimit = (1..<RequestLimits.maxFavorites).map { StubFavoritesService.favorite("\($0)") }
        let viewModel = makeViewModel(StubFavoritesService(list: [.result(underLimit)]))
        await viewModel.load()
        XCTAssertFalse(viewModel.isAtLimit)

        let atLimit = (1...RequestLimits.maxFavorites).map { StubFavoritesService.favorite("\($0)") }
        let full = makeViewModel(StubFavoritesService(list: [.result(atLimit)]))
        await full.load()
        XCTAssertEqual(full.items.count, RequestLimits.maxFavorites)
        XCTAssertTrue(full.isAtLimit)
    }

    // MARK: - 古い応答の破棄・キャンセル

    func testStaleLoadResponseIsDiscarded() async throws {
        let stub = StubFavoritesService(list: [.manual, .result([fav("2", 1)])])
        let viewModel = makeViewModel(stub)
        let first = Task { await viewModel.load() }
        try await stub.waitForList(calls: 1)
        await viewModel.load()
        XCTAssertEqual(viewModel.items.map(\.id), ["2"])
        // 古い要求(0番目)が後から届いても、新しい結果を上書きしない。
        await stub.resolveList(at: 0, with: .success([fav("1", 0)]))
        await first.value
        XCTAssertEqual(viewModel.items.map(\.id), ["2"], "古い応答は捨てる")
        XCTAssertEqual(viewModel.loadState, .loaded)
    }

    func testStaleLoadFailureDoesNotOverwriteNewerSuccess() async throws {
        let stub = StubFavoritesService(list: [.manual, .result([fav("2", 1)])])
        let viewModel = makeViewModel(stub)
        let first = Task { await viewModel.load() }
        try await stub.waitForList(calls: 1)
        await viewModel.load()
        await stub.resolveList(at: 0, with: .failure(Self.transportError))
        await first.value
        XCTAssertEqual(viewModel.loadState, .loaded, "古い失敗で .failed にしない")
        XCTAssertEqual(viewModel.items.map(\.id), ["2"])
    }

    func testCancelledLoadChangesNothingAndIsNotAFailure() async throws {
        let stub = StubFavoritesService(list: [.result([fav("1", 0)]), .manual])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        let task = Task { await viewModel.load() }
        try await stub.waitForList(calls: 2)
        task.cancel()
        await task.value
        XCTAssertEqual(viewModel.items.map(\.id), ["1"], "キャンセルで一覧を変えない")
        if case .failed = viewModel.loadState { XCTFail("キャンセルは失敗ではない") }
    }

    // MARK: - 外す

    func testRemoveSuccessDropsItemAndKeepsOrderOfOthers() async {
        let stub = StubFavoritesService(list: [.result([fav("3", 0), fav("2", 1), fav("1", 2)])])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        await viewModel.remove(id: "2")
        XCTAssertEqual(viewModel.items.map(\.id), ["3", "1"])
        XCTAssertNil(viewModel.actionError)
        XCTAssertTrue(viewModel.removingIDs.isEmpty)
        let removed = await stub.removeRequests
        XCTAssertEqual(removed, ["2"], "削除の要求は1回(一覧の再取得はしない)")
        let listCalls = await stub.listCalls
        XCTAssertEqual(listCalls, 1)
    }

    func testRemoveNotFoundTreatedAsAlreadyRemoved() async {
        let stub = StubFavoritesService(list: [.result([fav("2", 0), fav("1", 1)])], remove: [.failure(Self.notFoundError)])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        await viewModel.remove(id: "2")
        XCTAssertEqual(viewModel.items.map(\.id), ["1"], "404 は「すでに無い」なので一覧から除く")
        XCTAssertNil(viewModel.actionError, "利用者に失敗を見せない")
    }

    func testRemoveFailureKeepsItemAndSetsActionErrorThenDismiss() async {
        for (code, expected) in [
            (PokeCalcError.Code.transport, RecordScreenError.transport),
            ("store_unavailable", .storeUnavailable),
        ] {
            let stub = StubFavoritesService(
                list: [.result([fav("2", 0), fav("1", 1)])], remove: [.failure(PokeCalcError(code: code, message: "x"))])
            let viewModel = makeViewModel(stub)
            await viewModel.load()
            await viewModel.remove(id: "2")
            XCTAssertEqual(viewModel.items.map(\.id), ["2", "1"], "失敗では一覧を保つ(\(code))")
            XCTAssertEqual(viewModel.actionError, expected)
            XCTAssertEqual(viewModel.loadState, .loaded, "外す失敗は一覧の読み込み状態を壊さない")
            XCTAssertTrue(viewModel.removingIDs.isEmpty)
            viewModel.dismissActionError()
            XCTAssertNil(viewModel.actionError)
        }
    }

    func testSuccessfulRemoveClearsPreviousActionError() async {
        let stub = StubFavoritesService(
            list: [.result([fav("2", 0), fav("1", 1)])], remove: [.failure(Self.transportError), .result(())])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        await viewModel.remove(id: "2")
        XCTAssertNotNil(viewModel.actionError)
        await viewModel.remove(id: "2")
        XCTAssertNil(viewModel.actionError)
        XCTAssertEqual(viewModel.items.map(\.id), ["1"])
    }

    func testRemovingIDIsTrackedAndDuplicateTapSendsOneRequest() async throws {
        let stub = StubFavoritesService(list: [.result([fav("2", 0), fav("1", 1)])], remove: [.manual])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        let first = Task { await viewModel.remove(id: "2") }
        try await stub.waitForRemove(calls: 1)
        XCTAssertEqual(viewModel.removingIDs, ["2"])
        await viewModel.remove(id: "2")  // 二重タップ
        let requests = await stub.removeRequests
        XCTAssertEqual(requests, ["2"], "外している最中の同じ ID は再送しない")
        await stub.resolveRemove(at: 0, with: .success(()))
        await first.value
        XCTAssertTrue(viewModel.removingIDs.isEmpty)
        XCTAssertEqual(viewModel.items.map(\.id), ["1"])
    }

    func testRemovedItemIsNotResurrectedByAnInFlightLoad() async throws {
        let stub = StubFavoritesService(list: [.result([fav("2", 0), fav("1", 1)]), .manual])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        let reload = Task { await viewModel.load() }
        try await stub.waitForList(calls: 2)
        await viewModel.remove(id: "2")
        // 削除前に始まった読み込みの応答(まだ "2" を含む)が後から届く。
        await stub.resolveList(at: 1, with: .success([fav("2", 0), fav("1", 1)]))
        await reload.value
        XCTAssertEqual(viewModel.items.map(\.id), ["1"], "外した項目が古い応答で復活しない")
    }

    // MARK: - 件数上限の定数

    func testLimitConstantsMatchContractValues() {
        XCTAssertEqual(RequestLimits.maxFavorites, 100)
        XCTAssertEqual(RequestLimits.maxFavoriteLabelLength, 30)
        XCTAssertEqual(Favorites.maxConcurrentResolutions, 4)
    }
}
