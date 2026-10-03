import XCTest

@testable import PokeCalcCore

/// `FavoritePinViewModel`(計算画面の「お気に入りに追加」。ADR-0509)。
@MainActor
final class FavoritePinViewModelTests: XCTestCase {
    private let individual = Individual(
        speciesKey: "9001-000", natureId: "test-nature-neutral",
        sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0), abilityId: "test-ability-1")

    func testIsAvailableOnlyWithService() {
        XCTAssertTrue(FavoritePinViewModel(service: StubFavoritesService()).isAvailable)
        XCTAssertFalse(FavoritePinViewModel(service: nil).isAvailable, "サービスが無ければ計算画面は追加ボタンを出さない")
    }

    func testPinCreatedSendsIndividualAsIsWithoutLabel() async {
        let stub = StubFavoritesService(add: [.result(.created(StubFavoritesService.favorite("1")))])
        let viewModel = FavoritePinViewModel(service: stub)
        XCTAssertEqual(viewModel.status, .idle)
        await viewModel.pin(individual)
        XCTAssertEqual(viewModel.status, .pinned)
        let requests = await stub.addRequests
        XCTAssertEqual(requests.count, 1)
        XCTAssertEqual(requests.first?.individual, individual)
        XCTAssertNil(requests.first?.label, "計算画面からはラベル無しで追加する")
    }

    func testPinAlreadyPinnedIsADistinctSuccess() async {
        let stub = StubFavoritesService(add: [.result(.alreadyPinned(StubFavoritesService.favorite("1")))])
        let viewModel = FavoritePinViewModel(service: stub)
        await viewModel.pin(individual)
        XCTAssertEqual(viewModel.status, .alreadyPinned)
    }

    func testPinFailureSetsFailedWithMappedError() async {
        let cases: [(PokeCalcError, RecordScreenError)] = [
            (PokeCalcError(code: PokeCalcError.Code.transport, message: "x"), .transport),
            (PokeCalcError(code: "store_unavailable", message: "x"), .storeUnavailable),
            (PokeCalcError(code: "upstream_unavailable", message: "x"), .storeUnavailable),
            (PokeCalcError(code: "invalid_input", message: "x"), .invalidInput),
        ]
        for (error, expected) in cases {
            let viewModel = FavoritePinViewModel(service: StubFavoritesService(add: [.failure(error)]))
            await viewModel.pin(individual)
            XCTAssertEqual(viewModel.status, .failed(expected), error.code)
        }
    }

    func testRetryAfterFailureCanSucceed() async {
        let stub = StubFavoritesService(
            add: [
                .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "x")),
                .result(.created(StubFavoritesService.favorite("1"))),
            ])
        let viewModel = FavoritePinViewModel(service: stub)
        await viewModel.pin(individual)
        XCTAssertEqual(viewModel.status, .failed(.transport))
        await viewModel.pin(individual)
        XCTAssertEqual(viewModel.status, .pinned)
    }

    func testPinWhileSavingIsIgnored() async throws {
        let stub = StubFavoritesService(add: [.manual])
        let viewModel = FavoritePinViewModel(service: stub)
        let first = Task { await viewModel.pin(individual) }
        try await stub.waitForAdd(calls: 1)
        XCTAssertEqual(viewModel.status, .saving)
        await viewModel.pin(individual)
        let requests = await stub.addRequests
        XCTAssertEqual(requests.count, 1, "保存中の再呼び出しは要求を増やさない")
        await stub.resolveAdd(at: 0, with: .success(.created(StubFavoritesService.favorite("1"))))
        await first.value
        XCTAssertEqual(viewModel.status, .pinned)
    }

    func testCancelledPinReturnsToIdleNotFailed() async throws {
        let stub = StubFavoritesService(add: [.manual])
        let viewModel = FavoritePinViewModel(service: stub)
        let task = Task { await viewModel.pin(individual) }
        try await stub.waitForAdd(calls: 1)
        task.cancel()
        await task.value
        XCTAssertEqual(viewModel.status, .idle)
    }

    func testResetReturnsToIdleExceptWhileSaving() async throws {
        let stub = StubFavoritesService(add: [.result(.created(StubFavoritesService.favorite("1"))), .manual])
        let viewModel = FavoritePinViewModel(service: stub)
        await viewModel.pin(individual)
        viewModel.reset()
        XCTAssertEqual(viewModel.status, .idle)

        let task = Task { await viewModel.pin(individual) }
        try await stub.waitForAdd(calls: 2)
        viewModel.reset()
        XCTAssertEqual(viewModel.status, .saving, "保存中は変えない")
        await stub.resolveAdd(at: 1, with: .success(.created(StubFavoritesService.favorite("2"))))
        await task.value
    }

    func testNilServicePinDoesNothing() async {
        let viewModel = FavoritePinViewModel(service: nil)
        await viewModel.pin(individual)
        XCTAssertEqual(viewModel.status, .idle)
    }
}
