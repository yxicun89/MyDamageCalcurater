import XCTest

@testable import PokeCalcCore

/// `OpponentHistoryViewModel`(よく計算する相手の一覧画面。ADR-0511)と `OpponentHistoryLabels`。
/// データ源は P6-23 の `FrequentOpponentsService`(新しい API は足さない)。
@MainActor
final class OpponentHistoryViewModelTests: XCTestCase {
    private func resolver(count: Int = 6) -> StubPokeCalcService {
        StubPokeCalcService(species: (0..<count).map(StubBulkMaster.pageSpecies), moves: [], items: [], natures: [])
    }

    private func key(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).key }
    private func name(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).nameJa }

    private func opponent(_ key: String, count: Int = 1, at seconds: TimeInterval = 1_790_000_000) -> FrequentOpponent {
        FrequentOpponent(speciesKey: key, score: Double(count), count: count, lastCalculatedAt: Date(timeIntervalSince1970: seconds))
    }

    private static let transportError = PokeCalcError(code: PokeCalcError.Code.transport, message: "test")

    func testConstants() {
        XCTAssertEqual(OpponentHistory.listLimit, 50, "openapi limit の最大")
        XCTAssertEqual(OpponentHistory.listLimit, FrequentOpponents.limitRange.upperBound)
    }

    // MARK: - 読み込み

    func testLoadRequestsListLimitAndKeepsServerOrderWithCountAndTime() async {
        let stub = StubFrequentOpponentsService([
            .result([opponent(key(2), count: 6, at: 1_790_000_100), opponent(key(0), count: 3, at: 1_790_000_000)])
        ])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        XCTAssertEqual(viewModel.loadState, .idle)
        await viewModel.load()
        let limits = await stub.limits
        XCTAssertEqual(limits, [OpponentHistory.listLimit])
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(2), key(0)], "サーバーの順を保つ")
        XCTAssertEqual(viewModel.items.map(\.speciesName), [name(2), name(0)])
        XCTAssertEqual(viewModel.items.map(\.count), [6, 3])
        XCTAssertEqual(
            viewModel.items.map(\.lastCalculatedAt),
            [Date(timeIntervalSince1970: 1_790_000_100), Date(timeIntervalSince1970: 1_790_000_000)])
    }

    func testCustomLimitIsPassed() async {
        let stub = StubFrequentOpponentsService(keys: [])
        await OpponentHistoryViewModel(service: stub, resolver: resolver(), limit: 5).load()
        let limits = await stub.limits
        XCTAssertEqual(limits, [5])
    }

    func testUnresolvableSpeciesIsKeptAsUnknownUnlikeThePicker() async {
        let stub = StubFrequentOpponentsService(keys: [key(0), "9999-000", key(1)])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(0), "9999-000", key(1)], "件数・時刻の情報があるので行を残す")
        XCTAssertEqual(viewModel.items.map(\.speciesName), [name(0), nil, name(1)])
        XCTAssertEqual(viewModel.items.map(\.title), [name(0), FavoritesLabels.unknownSpecies, name(1)])
    }

    func testDuplicateKeysKeepFirstOnly() async {
        let stub = StubFrequentOpponentsService([.result([opponent(key(0), count: 5), opponent(key(1)), opponent(key(0), count: 1)])])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(0), key(1)])
        XCTAssertEqual(viewModel.items.first?.count, 5)
    }

    func testEmptyIsLoadedNotFailure() async {
        let viewModel = OpponentHistoryViewModel(service: StubFrequentOpponentsService(keys: []), resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items, [])
    }

    func testFailureSetsFailedStateAndRetryRecovers() async {
        let stub = StubFrequentOpponentsService([.failure(Self.transportError), .result([opponent(key(0))])])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.transport))
        XCTAssertEqual(viewModel.items, [])
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(0)])
    }

    func testServiceUnavailableAndBadRequestMapToStates() async {
        let stub = StubFrequentOpponentsService([
            .failure(PokeCalcError(code: "upstream_unavailable", message: "x")),
            .failure(PokeCalcError(code: "invalid_input", message: "x")),
        ])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.storeUnavailable))
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.invalidInput))
    }

    func testReloadFailureKeepsPreviousItems() async {
        let stub = StubFrequentOpponentsService([.result([opponent(key(0))]), .failure(Self.transportError)])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.transport))
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(0)])
    }

    func testMasterFailureDoesNotFailLoad() async {
        let master = resolver()
        await master.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "test"))
        let viewModel = OpponentHistoryViewModel(service: StubFrequentOpponentsService(keys: [key(0)]), resolver: master)
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(0)])
        XCTAssertEqual(viewModel.items.first?.speciesName, nil)
    }

    func testNilServiceDoesNothing() async {
        let viewModel = OpponentHistoryViewModel(service: nil, resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .idle)
        XCTAssertEqual(viewModel.items, [])
    }

    func testStaleResponseIsDiscarded() async throws {
        let stub = StubFrequentOpponentsService([.manual, .result([opponent(key(1))])])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        let first = Task { await viewModel.load() }
        try await stub.waitForCalls(count: 1)
        await viewModel.load()
        await stub.resolve(at: 0, with: .success([opponent(key(0))]))
        await first.value
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(1)], "古い応答は捨てる")
        XCTAssertEqual(viewModel.loadState, .loaded)
    }

    func testStaleFailureDoesNotOverwriteNewerSuccess() async throws {
        let stub = StubFrequentOpponentsService([.manual, .result([opponent(key(1))])])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        let first = Task { await viewModel.load() }
        try await stub.waitForCalls(count: 1)
        await viewModel.load()
        await stub.resolve(at: 0, with: .failure(Self.transportError))
        await first.value
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(1)])
    }

    func testCancelledLoadChangesNothingAndIsNotAFailure() async throws {
        let stub = StubFrequentOpponentsService([.result([opponent(key(0))]), .manual])
        let viewModel = OpponentHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        let task = Task { await viewModel.load() }
        try await stub.waitForCalls(count: 2)
        task.cancel()
        await task.value
        XCTAssertEqual(viewModel.items.map(\.speciesKey), [key(0)])
        if case .failed = viewModel.loadState { XCTFail("キャンセルは失敗ではない") }
    }

    // MARK: - 文言

    private var tokyo: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "Asia/Tokyo")!
        return calendar
    }

    func testCountText() {
        XCTAssertEqual(OpponentHistoryLabels.countText(1), "1回")
        XCTAssertEqual(OpponentHistoryLabels.countText(12), "12回")
    }

    func testLastCalculatedTextUsesCalendarDays() {
        // 2026-10-04 09:00 JST を「いま」にする。
        let now = tokyo.date(from: DateComponents(year: 2026, month: 10, day: 4, hour: 9))!
        func text(_ components: DateComponents) -> String {
            OpponentHistoryLabels.lastCalculatedText(tokyo.date(from: components)!, now: now, calendar: tokyo)
        }
        XCTAssertEqual(text(.init(year: 2026, month: 10, day: 4, hour: 0, minute: 1)), "最後: 今日")
        XCTAssertEqual(text(.init(year: 2026, month: 10, day: 3, hour: 23, minute: 59)), "最後: 昨日", "時刻の差ではなく暦日の差")
        XCTAssertEqual(text(.init(year: 2026, month: 10, day: 1, hour: 12)), "最後: 3日前")
        XCTAssertEqual(text(.init(year: 2026, month: 9, day: 4, hour: 12)), "最後: 30日前")
        XCTAssertEqual(text(.init(year: 2026, month: 10, day: 5, hour: 12)), "最後: 今日", "未来の時刻は今日にする")
    }
}
