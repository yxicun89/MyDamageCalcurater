import Foundation
import XCTest

@testable import PokeCalcCore

/// 計算画面の比較する持ち物の件数上限(issue #110。ADR-0501「issue #110 の受け入れ条件(iOS 側)」A8)。
///
/// 契約(`api/openapi.yaml`)は `BulkCalcRequest.itemVariants` が 64 件以下 + 重複なし。
/// 逆算の `itemCandidates` と**同じ**規則にする(ADR 6章: iOS は `itemVariants` を実際に使っている)。
@MainActor
final class CalcViewModelLimitTests: XCTestCase {

    // MARK: - 補助

    private static func items(_ count: Int) -> [Item] {
        (0..<count).map { Item(id: String(format: "stub-item-%03d", $0), nameJa: "テストどうぐ\($0)") }
    }

    private static var overflowingItemCount: Int { RequestLimits.maxItemVariants + 6 }

    private func loadedViewModel(itemCount: Int) async -> (StubPokeCalcService, CalcViewModel) {
        let stub = StubMaster.makeService(items: Self.items(itemCount))
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return (stub, viewModel)
    }

    private func bulkCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.bulkRequests.count
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    /// 上限ちょうど(= 選べる最大件数)まで選ぶ。選んだ ID をマスタ順で返す。
    @discardableResult
    private func selectUpToLimit(_ viewModel: CalcViewModel, ids: [String]) async -> [String] {
        let selectable = Array(ids.prefix(RequestLimits.maxSelectableItemVariants))
        for id in selectable {
            await viewModel.toggleDefenderItemComparison(itemId: id)
        }
        return selectable
    }

    // MARK: - A8

    /// 「持ち物なし」(null)を含めて契約の上限ちょうどまで選べる。
    func testSelectingUpToTheItemVariantLimitKeepsTheRequestWithinTheContract() async throws {
        let (stub, viewModel) = await loadedViewModel(itemCount: Self.overflowingItemCount)
        let ids = viewModel.itemOptions.map(\.id)

        let selectable = await selectUpToLimit(viewModel, ids: ids)

        XCTAssertEqual(viewModel.comparedDefenderItemIds, selectable, "マスタの順(既存の規則)")
        XCTAssertTrue(viewModel.comparedDefenderItemsReachedLimit)

        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemVariants.count, RequestLimits.maxItemVariants, "null 1件 + 選んだ \(selectable.count) 件")
        XCTAssertNil(try XCTUnwrap(request.itemVariants.first), "先頭は「持ち物なし」")
        XCTAssertEqual(request.itemVariants.filter { $0 == nil }.count, 1, "null は1件だけ(uniqueItems)")
        XCTAssertEqual(Set(request.itemVariants.compactMap { $0 }).count, selectable.count, "ID の重複なし")
    }

    /// 上限到達中の ON 操作は拒否する。選択も要求も動かさない。
    func testTogglingOnBeyondTheItemVariantLimitIsRejectedWithoutRequesting() async throws {
        let (stub, viewModel) = await loadedViewModel(itemCount: Self.overflowingItemCount)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = await selectUpToLimit(viewModel, ids: ids)
        let requestsBefore = await bulkCount(stub)
        let rejected = ids[RequestLimits.maxSelectableItemVariants]

        await viewModel.toggleDefenderItemComparison(itemId: rejected)

        XCTAssertFalse(viewModel.comparedDefenderItemIds.contains(rejected), "上限を超えて選べない")
        XCTAssertEqual(viewModel.comparedDefenderItemIds, selectable, "選択は1つも変わらない")
        XCTAssertTrue(viewModel.comparedDefenderItemsReachedLimit)
        let requestsAfter = await bulkCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore, "拒否した操作で計算し直さない")
    }

    /// 上限に達していても選択解除はできる。解除すれば上限は解け、計算も1回走る。
    func testDeselectingIsAlwaysAllowedEvenAtTheLimit() async throws {
        let (stub, viewModel) = await loadedViewModel(itemCount: Self.overflowingItemCount)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = await selectUpToLimit(viewModel, ids: ids)
        let requestsBefore = await bulkCount(stub)

        await viewModel.toggleDefenderItemComparison(itemId: selectable[0])

        XCTAssertEqual(viewModel.comparedDefenderItemIds, Array(selectable.dropFirst()))
        XCTAssertFalse(viewModel.comparedDefenderItemsReachedLimit, "1つ外せば上限は解ける")
        let requestsAfter = await bulkCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore + 1, "解除はトグル1回で計算1回(既存の規則)")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemVariants.count, RequestLimits.maxItemVariants - 1)
    }

    /// 回帰: 上限に達していないときの振る舞いは変えない。
    func testTogglingBelowTheLimitStillWorksAsBefore() async throws {
        let (stub, viewModel) = await loadedViewModel(itemCount: 4)
        let ids = viewModel.itemOptions.map(\.id)
        let requestsBefore = await bulkCount(stub)

        await viewModel.toggleDefenderItemComparison(itemId: ids[1])
        await viewModel.toggleDefenderItemComparison(itemId: ids[0])

        XCTAssertEqual(viewModel.comparedDefenderItemIds, [ids[0], ids[1]], "トグル順ではなくマスタの順")
        XCTAssertFalse(viewModel.comparedDefenderItemsReachedLimit)
        let requestsAfter = await bulkCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore + 2, "トグル1回で計算1回")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemVariants, [nil, ids[0], ids[1]])
    }

    // MARK: - guard の位置の回帰(critic 指摘)

    /// critic 指摘: 上限到達中の ON 拒否の guard は `beginInput()` より前でなければならない
    /// (ADR-0501「issue #110」8章)。後ろに動かすと、拒否操作でも世代(`latestRequestToken`)が
    /// 進んでしまい、拒否の直前から進行中だった計算の応答が「追い越された」ことにされ、
    /// `isLoading` が解けないまま残る(`CalcViewModelCancellationTests` と同じ道具立て)。
    func testRejectedToggleDoesNotAdvanceGenerationOfAnInFlightCalc() async throws {
        let (stub, viewModel) = await loadedViewModel(itemCount: Self.overflowingItemCount)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = await selectUpToLimit(viewModel, ids: ids)
        let rejected = ids[RequestLimits.maxSelectableItemVariants]
        XCTAssertTrue(viewModel.comparedDefenderItemsReachedLimit)

        await stub.setBulkMode(.manual)
        let baseline = await bulkCount(stub)
        let inFlight = Task { await viewModel.selectAttackerItem(id: ids[0]) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading)

        // 上限到達中の ON 操作(拒否される)。この操作で世代を進めてはいけない。
        await viewModel.toggleDefenderItemComparison(itemId: rejected)
        XCTAssertFalse(viewModel.comparedDefenderItemIds.contains(rejected), "上限を超えて選べない")
        XCTAssertEqual(viewModel.comparedDefenderItemIds, selectable, "拒否操作で選択は変わらない")

        // 拒否操作の直前から進行中だった計算の応答は、ちゃんと反映されること
        // (guard が `beginInput()` より前にあれば世代は進んでいないはず)。
        await stub.resolveBulkWithEcho(at: baseline)
        await inFlight.value

        XCTAssertFalse(viewModel.isLoading, "拒否操作が世代を進めて、進行中の計算を追い越したことにしてはいけない")
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }
}
