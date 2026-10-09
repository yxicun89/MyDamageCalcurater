import XCTest

@testable import PokeCalcCore

/// `CalcHistoryViewModel`(計算履歴の一覧。ADR-0230・ADR-0519)。
/// 先頭ページ(limit=20)→「もっと見る」(`nextCursor` をそのまま渡す)・`nextCursor` が null なら終わり・
/// 失敗しても取得済みの行を壊さない・古い応答を捨てる・400 は先頭から読み直す。架空のマスタだけを使う。
@MainActor
final class CalcHistoryViewModelTests: XCTestCase {
    private func resolver() -> StubPokeCalcService {
        StubPokeCalcService(
            species: (0..<4).map(StubBulkMaster.pageSpecies),
            moves: [Move(id: "test-move-a", nameJa: "テストわざ", type: .fire, category: .special, power: 60)],
            items: [], natures: [])
    }

    private func speciesKey(_ index: Int) -> String { StubBulkMaster.pageSpecies(index).key }

    private func entry(_ index: Int, attacker: String? = nil, defender: String? = nil, move: String = "test-move-a")
        -> CalcHistoryEntry
    {
        StubCalcHistoryService.entry(
            index, attacker: attacker ?? speciesKey(0), defender: defender ?? speciesKey(1), move: move)
    }

    private func page(_ indexes: Range<Int>, next: String?) -> CalcHistoryPage {
        CalcHistoryPage(items: indexes.map { entry($0) }, nextCursor: next)
    }

    private static let transportError = PokeCalcError(code: PokeCalcError.Code.transport, message: "test")
    private static let unavailableError = PokeCalcError(code: "store_unavailable", message: "test")

    private func setUpMoves(_ stub: StubPokeCalcService) async {
        await stub.setMoveLookupMode(.immediate)
    }

    func testPageSizeIsTwenty() {
        XCTAssertEqual(CalcHistory.pageSize, 20)
    }

    // MARK: - 先頭ページ

    func testLoadRequestsFirstPageWithoutCursorAndShowsRowsNewestFirstWithNames() async {
        let master = resolver()
        await setUpMoves(master)
        let stub = StubCalcHistoryService([.page(page(0..<3, next: "cursor-1"))])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: master)
        XCTAssertEqual(viewModel.loadState, .idle)
        await viewModel.load()
        let calls = await stub.calls
        XCTAssertEqual(calls, [.init(limit: 20, cursor: nil)])
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.rows.count, 3)
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [10, 11, 12], "サーバーの順を並べ替えない")
        let first = viewModel.rows[0]
        XCTAssertEqual(first.attackerName, StubBulkMaster.pageSpecies(0).nameJa)
        XCTAssertEqual(first.defenderName, StubBulkMaster.pageSpecies(1).nameJa)
        XCTAssertEqual(first.moveName, "テストわざ")
        XCTAssertEqual(first.percentText, "10.0\u{301C}12.0%")
        XCTAssertTrue(viewModel.hasMore)
        XCTAssertFalse(viewModel.isEmpty)
    }

    func testNilNextCursorMeansNoMore() async {
        let stub = StubCalcHistoryService([.page(page(0..<2, next: nil))])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        XCTAssertFalse(viewModel.hasMore)
        await viewModel.loadMore()
        let calls = await stub.calls
        XCTAssertEqual(calls.count, 1, "続きが無ければ呼ばない")
    }

    func testEmptyIsLoadedNotFailure() async {
        let viewModel = CalcHistoryViewModel(service: StubCalcHistoryService([.page(page(0..<0, next: nil))]), resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertTrue(viewModel.isEmpty)
        XCTAssertFalse(viewModel.hasMore)
    }

    func testUnresolvableNamesBecomeUnknownButRowStays() async {
        let stub = StubCalcHistoryService([
            .page(CalcHistoryPage(items: [entry(0, attacker: "9999-000", move: "test-move-gone")], nextCursor: nil))
        ])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        let row = viewModel.rows[0]
        XCTAssertNil(row.attackerName)
        XCTAssertEqual(row.title, "\(FavoritesLabels.unknownSpecies) → \(StubBulkMaster.pageSpecies(1).nameJa)")
        XCTAssertEqual(row.moveText, CalcHistoryLabels.unknownMove)
    }

    func testNilServiceDoesNothing() async {
        let viewModel = CalcHistoryViewModel(service: nil, resolver: resolver())
        await viewModel.load()
        await viewModel.loadMore()
        XCTAssertEqual(viewModel.loadState, .idle)
        XCTAssertTrue(viewModel.rows.isEmpty)
    }

    // MARK: - 続き

    func testLoadMorePassesNextCursorUnchangedAndAppends() async {
        let stub = StubCalcHistoryService([
            .page(page(0..<2, next: "opaque_Cursor-1==")), .page(page(2..<4, next: nil)),
        ])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        let idsBefore = viewModel.rows.map(\.id)
        await viewModel.loadMore()
        let calls = await stub.calls
        XCTAssertEqual(calls, [.init(limit: 20, cursor: nil), .init(limit: 20, cursor: "opaque_Cursor-1==")])
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [10, 11, 12, 13], "末尾に足す")
        XCTAssertEqual(Array(viewModel.rows.map(\.id).prefix(2)), idsBefore, "ページを足しても既存の行の id は変わらない")
        XCTAssertEqual(Set(viewModel.rows.map(\.id)).count, 4, "id は重ならない")
        XCTAssertFalse(viewModel.hasMore)
        XCTAssertFalse(viewModel.isLoadingMore)
    }

    func testLoadMoreFailureKeepsRowsAndRetryRecovers() async {
        let stub = StubCalcHistoryService([
            .page(page(0..<2, next: "c1")), .failure(Self.unavailableError), .page(page(2..<3, next: nil)),
        ])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        await viewModel.loadMore()
        XCTAssertEqual(viewModel.loadMoreError, .storeUnavailable)
        XCTAssertEqual(viewModel.rows.count, 2, "取得済みの行は消さない")
        XCTAssertEqual(viewModel.loadState, .loaded, "先頭ページの状態は壊さない")
        XCTAssertTrue(viewModel.hasMore, "同じカーソルで再試行できる")
        await viewModel.loadMore()
        XCTAssertNil(viewModel.loadMoreError)
        XCTAssertEqual(viewModel.rows.count, 3)
        let calls = await stub.calls
        XCTAssertEqual(calls.map(\.cursor), [nil, "c1", "c1"])
    }

    func testLoadMoreInvalidInputReloadsFromTop() async {
        let badRequest = PokeCalcError(code: "invalid_input", message: "x")
        let stub = StubCalcHistoryService([
            .page(page(0..<2, next: "c1")), .failure(badRequest), .page(page(5..<7, next: nil)),
        ])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        await viewModel.loadMore()
        let calls = await stub.calls
        XCTAssertEqual(calls.map(\.cursor), [nil, "c1", nil], "400 は先頭から読み直す")
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [15, 16], "先頭ページで置き換える")
        XCTAssertNil(viewModel.loadMoreError)
        XCTAssertEqual(viewModel.loadState, .loaded)
    }

    func testSecondLoadMoreWhileLoadingIsIgnored() async throws {
        let stub = StubCalcHistoryService([.page(page(0..<2, next: "c1")), .manual])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        let first = Task { await viewModel.loadMore() }
        try await stub.waitForCalls(count: 2)
        XCTAssertTrue(viewModel.isLoadingMore)
        await viewModel.loadMore()
        let calls = await stub.calls
        XCTAssertEqual(calls.count, 2, "読み込み中の「もっと見る」は要求を重ねない")
        await stub.resolve(at: 1, with: .success(page(2..<3, next: nil)))
        await first.value
        XCTAssertEqual(viewModel.rows.count, 3)
    }

    // MARK: - 失敗・再読み込み

    func testFailureSetsFailedStateAndRetryRecovers() async {
        let stub = StubCalcHistoryService([.failure(Self.unavailableError), .page(page(0..<1, next: nil))])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.storeUnavailable))
        XCTAssertTrue(viewModel.rows.isEmpty)
        XCTAssertFalse(viewModel.isEmpty, "失敗を「空」と見せない")
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .loaded)
        XCTAssertEqual(viewModel.rows.count, 1)
    }

    func testReloadReplacesRowsAndResetsPaging() async {
        let stub = StubCalcHistoryService([
            .page(page(0..<2, next: "c1")), .page(page(2..<3, next: nil)), .page(page(10..<11, next: "c9")),
        ])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        await viewModel.loadMore()
        XCTAssertEqual(viewModel.rows.count, 3)
        await viewModel.load()
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [20], "読み直しは先頭から置き換える")
        XCTAssertTrue(viewModel.hasMore)
        let calls = await stub.calls
        XCTAssertEqual(calls.last, .init(limit: 20, cursor: nil))
    }

    func testReloadFailureKeepsPreviousRows() async {
        let stub = StubCalcHistoryService([.page(page(0..<2, next: nil)), .failure(Self.transportError)])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        await viewModel.load()
        XCTAssertEqual(viewModel.loadState, .failed(.transport))
        XCTAssertEqual(viewModel.rows.count, 2, "再読み込みの失敗で取得済みの行を消さない")
    }

    // MARK: - 古い応答の破棄

    func testStaleLoadResponseIsDiscarded() async throws {
        let stub = StubCalcHistoryService([.manual, .manual])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        let older = Task { await viewModel.load() }
        try await stub.waitForCalls(count: 1)
        let newer = Task { await viewModel.load() }
        try await stub.waitForCalls(count: 2)
        await stub.resolve(at: 1, with: .success(page(5..<6, next: nil)))
        await newer.value
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [15])
        await stub.resolve(at: 0, with: .success(page(0..<3, next: "old")))
        await older.value
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [15], "古い応答で上書きしない")
        XCTAssertFalse(viewModel.hasMore)
        XCTAssertEqual(viewModel.loadState, .loaded)
    }

    func testLoadMoreResponseAfterReloadIsDiscarded() async throws {
        let stub = StubCalcHistoryService([.page(page(0..<2, next: "c1")), .manual, .page(page(7..<8, next: nil))])
        let viewModel = CalcHistoryViewModel(service: stub, resolver: resolver())
        await viewModel.load()
        let more = Task { await viewModel.loadMore() }
        try await stub.waitForCalls(count: 2)
        await viewModel.load()
        await stub.resolve(at: 1, with: .success(page(2..<4, next: "c2")))
        await more.value
        XCTAssertEqual(viewModel.rows.map(\.entry.minPercent), [17], "読み直しの後に届いた続きの応答は捨てる")
        XCTAssertFalse(viewModel.hasMore)
        XCTAssertFalse(viewModel.isLoadingMore)
    }

    // MARK: - 文言

    func testLabelsAndOccurredText() {
        XCTAssertEqual(CalcHistoryLabels.sectionTitle, "計算履歴")
        XCTAssertEqual(CalcHistoryLabels.loadMoreButton, "もっと見る")
        let row = CalcHistoryRow(
            id: "0-0", entry: entry(0), attackerName: "あ", defenderName: nil, moveName: nil)
        XCTAssertEqual(row.title, "あ → \(FavoritesLabels.unknownSpecies)")
        let calendar = Calendar(identifier: .gregorian)
        let now = entry(0).occurredAt
        XCTAssertEqual(row.occurredText(now: now, calendar: calendar), "今日")
        XCTAssertEqual(row.occurredText(now: now.addingTimeInterval(86_400 * 3), calendar: calendar), "3日前")
    }
}
