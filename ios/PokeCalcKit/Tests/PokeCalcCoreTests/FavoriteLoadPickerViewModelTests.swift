import XCTest

@testable import PokeCalcCore

/// 「お気に入りから呼ぶ」シートの ViewModel(ADR-0513 §6)。開いたとき1回取得・失敗・空・古い応答の破棄。
/// 取得の失敗はこのシートの中の案内だけで、計算画面には影響しない(絶対ルール5)。
@MainActor
final class FavoriteLoadPickerViewModelTests: XCTestCase {
    private func resolver() -> StubPokeCalcService { StubMaster.makeService() }

    private func fav(_ id: String, _ species: SpeciesDetail = StubMaster.alpha, label: String? = nil) -> Favorite {
        StubFavoritesService.favorite(id, key: species.key, label: label)
    }

    private func makeViewModel(_ service: (any FavoritesService)?) -> FavoriteLoadPickerViewModel {
        FavoriteLoadPickerViewModel(service: service, resolver: resolver())
    }

    private static let transportError = PokeCalcError(code: PokeCalcError.Code.transport, message: "test")

    func testIsAvailableOnlyWithService() {
        XCTAssertTrue(makeViewModel(StubFavoritesService()).isAvailable)
        XCTAssertFalse(makeViewModel(nil).isAvailable, "サービスが無ければ入口を出さない(FavoritePinViewModel と同じ流儀)")
    }

    func testLoadCallsFavoritesExactlyOnceAndKeepsServerOrderWithNames() async {
        let stub = StubFavoritesService(list: [
            .result([fav("30", StubMaster.beta, label: "HB特化"), fav("20", StubMaster.alpha), fav("10", StubMaster.gamma)])
        ])
        let viewModel = makeViewModel(stub)
        XCTAssertEqual(viewModel.loadState, .idle)

        await viewModel.load()

        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.rows.map(\.id), ["30", "20", "10"], "サーバーの順のまま")
        XCTAssertEqual(viewModel.rows.map(\.title), ["HB特化", StubMaster.alpha.nameJa, StubMaster.gamma.nameJa])
        XCTAssertFalse(viewModel.isEmpty)
        let calls = await stub.listCalls
        XCTAssertEqual(calls, 1, "取得はシートを開いたとき1回")
    }

    func testRowKeepsTheFavoriteSoTheCalcCanLoadIt() async {
        let favorite = fav("1", StubMaster.beta, label: "ラベル")
        let viewModel = makeViewModel(StubFavoritesService(list: [.result([favorite])]))
        await viewModel.load()
        XCTAssertEqual(viewModel.rows.first?.favorite, favorite, "行から個体をそのまま読み込める")
    }

    func testUnknownSpeciesRowIsKeptSoTheLoadCanExplain() async {
        let unknown = StubFavoritesService.favorite("9", key: "stub-gone-000")
        let viewModel = makeViewModel(StubFavoritesService(list: [.result([fav("2"), unknown])]))
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.rows.map(\.id), ["2", "9"], "マスタに無い種族の行も残す(選ぶと案内が出る)")
        XCTAssertEqual(viewModel.rows.last?.title, FavoritesLabels.unknownSpecies)
    }

    func testEmptyStoreIsLoadedAndEmpty() async {
        let viewModel = makeViewModel(StubFavoritesService(list: [.result([])]))
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertTrue(viewModel.rows.isEmpty)
        XCTAssertTrue(viewModel.isEmpty, "0件は空の案内(失敗ではない)")
    }

    func testIsNotEmptyBeforeLoadAndWhileFailed() async {
        let viewModel = makeViewModel(StubFavoritesService(list: [.failure(Self.transportError)]))
        XCTAssertFalse(viewModel.isEmpty, "読み込み前は空と言わない")
        await viewModel.load()
        XCTAssertFalse(viewModel.isEmpty, "失敗は空ではない")
    }

    func testFailureKindsMapToRecordScreenErrorWithoutBreakingRows() async {
        for (error, expected) in [
            (Self.transportError, RecordScreenError.transport),
            (PokeCalcError(code: "store_unavailable", message: "test"), .storeUnavailable),
            (PokeCalcError(code: PokeCalcError.Code.decode, message: "test"), .unexpectedResponse),
        ] {
            let viewModel = makeViewModel(StubFavoritesService(list: [.failure(error)]))
            await viewModel.load()
            XCTAssertEqual(viewModel.loadState, .failed(expected), "\(error.code)")
            XCTAssertTrue(viewModel.rows.isEmpty)
        }
    }

    func testRetryAfterFailureLoadsAndClearsFailure() async {
        let stub = StubFavoritesService(list: [.failure(Self.transportError), .result([fav("1")])])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.rows.map(\.id), ["1"])
        let calls = await stub.listCalls
        XCTAssertEqual(calls, 2, "再読み込みは新しい1回の取得")
    }

    func testWithoutServiceLoadDoesNothing() async {
        let viewModel = makeViewModel(nil)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .idle)
        XCTAssertTrue(viewModel.rows.isEmpty)
    }

    func testStaleResponseIsDiscarded() async throws {
        let stub = StubFavoritesService(list: [.manual, .result([fav("2")])])
        let viewModel = makeViewModel(stub)
        let first = Task { await viewModel.load() }
        try await stub.waitForList(calls: 1)
        await viewModel.load()
        XCTAssertEqual(viewModel.rows.map(\.id), ["2"])

        await stub.resolveList(at: 0, with: .success([fav("1")]))
        await first.value

        XCTAssertEqual(viewModel.rows.map(\.id), ["2"], "古い応答は捨てる")
        XCTAssertEqual(viewModel.loadState, .loaded)
    }

    func testStaleFailureDoesNotOverwriteNewerSuccess() async throws {
        let stub = StubFavoritesService(list: [.manual, .result([fav("2")])])
        let viewModel = makeViewModel(stub)
        let first = Task { await viewModel.load() }
        try await stub.waitForList(calls: 1)
        await viewModel.load()
        await stub.resolveList(at: 0, with: .failure(Self.transportError))
        await first.value
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.rows.map(\.id), ["2"])
    }

    func testCancelledLoadChangesNothingAndIsNotAFailure() async throws {
        let stub = StubFavoritesService(list: [.result([fav("1")]), .manual])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        let task = Task { await viewModel.load() }
        try await stub.waitForList(calls: 2)
        task.cancel()
        await task.value
        XCTAssertEqual(viewModel.rows.map(\.id), ["1"], "シートを閉じてキャンセルされても行を変えない")
        if case .failed = viewModel.loadState { XCTFail("キャンセルは失敗ではない") }
    }

    func testPickerNeverMutatesTheStore() async {
        let stub = StubFavoritesService(list: [.result([fav("1")])])
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        let adds = await stub.addRequests
        let removes = await stub.removeRequests
        XCTAssertTrue(adds.isEmpty)
        XCTAssertTrue(removes.isEmpty, "読み込みの入口は追加・削除をしない")
    }
}
