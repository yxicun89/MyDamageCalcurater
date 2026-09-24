import Foundation
import XCTest

@testable import PokeCalcCore

/// 逆算画面の件数上限(issue #110。ADR-0501「issue #110 の受け入れ条件(iOS 側)」A2〜A7)。
///
/// 契約(`api/openapi.yaml`)は `ReverseRequest.observations` が 16 件以下、
/// `ReverseRequest.itemCandidates` が 64 件以下 + 重複なし。超えた要求を**送らせない**のが画面の責任。
/// 判断: 観測は「追加操作を拒否 + 理由表示」(ADR 4章)、持ち物候補は「上限到達中の ON 操作を拒否。
/// OFF は常に通る」(ADR 5章。Web の決定的な切り捨てとはあえて変えている)。
@MainActor
final class ReverseViewModelLimitTests: XCTestCase {

    // MARK: - 補助

    /// 上限(63件)を実際に踏める数の架空の持ち物。マスタの並び順がそのまま候補の順になる。
    private static func items(_ count: Int) -> [Item] {
        (0..<count).map { Item(id: String(format: "stub-item-%03d", $0), nameJa: "テストどうぐ\($0)") }
    }

    /// 上限を踏むのに十分な余裕(+6)を持たせた持ち物マスタ。
    private static var overflowingItemCount: Int { RequestLimits.maxItemCandidates + 6 }

    private func makeStub(itemCount: Int = 2) async -> StubPokeCalcService {
        let stub = StubMaster.makeService(items: Self.items(itemCount), natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func reverseCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.reverseRequests.count
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> ReverseRequest {
        let requests = await stub.reverseRequests
        return try XCTUnwrap(requests.last, "reverse が呼ばれていない")
    }

    /// 1行目に有効な観測を入れて「計算できる状態」にする(以後のトグルは計算を1回ずつ呼ぶ)。
    @discardableResult
    private func enterFirstObservation(_ viewModel: ReverseViewModel) async throws -> Int {
        let id = try XCTUnwrap(viewModel.observations.first?.id, "観測の行が無い")
        await viewModel.editObservation(id: id, text: "12")
        return id
    }

    /// 観測の行を上限ちょうどまで増やす。
    private func fillObservationRowsToLimit(_ viewModel: ReverseViewModel) {
        while viewModel.observations.count < RequestLimits.maxObservations {
            let before = viewModel.observations.count
            viewModel.addObservation()
            guard viewModel.observations.count == before + 1 else {
                return XCTFail("上限(\(RequestLimits.maxObservations))前に追加が止まった(\(before) 行)")
            }
        }
    }

    // MARK: - 観測の件数(A2〜A4)

    /// A2: 上限ちょうどまでは追加でき、達したら `observationsReachedLimit` が立つ。
    func testObservationRowsCanGrowUpToTheLimit() async throws {
        let viewModel = await loadedViewModel(await makeStub())
        XCTAssertEqual(viewModel.observations.count, 1)
        XCTAssertFalse(viewModel.observationsReachedLimit, "1行では上限ではない")

        fillObservationRowsToLimit(viewModel)

        XCTAssertEqual(viewModel.observations.count, RequestLimits.maxObservations)
        XCTAssertTrue(viewModel.observationsReachedLimit)
        XCTAssertEqual(Set(viewModel.observations.map(\.id)).count, RequestLimits.maxObservations, "行の id は一意")
    }

    /// A2: 上限に達したあとの `addObservation()` は何もしない(行も要求も動かさない)。
    func testAddObservationBeyondTheLimitIsIgnored() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        try await enterFirstObservation(viewModel)
        fillObservationRowsToLimit(viewModel)
        let idsBefore = viewModel.observations.map(\.id)
        let requestsBefore = await reverseCount(stub)

        viewModel.addObservation()
        viewModel.addObservation()

        XCTAssertEqual(viewModel.observations.count, RequestLimits.maxObservations, "上限を超えて増えない")
        XCTAssertEqual(viewModel.observations.map(\.id), idsBefore, "既存の行を作り直さない")
        XCTAssertTrue(viewModel.observationsReachedLimit)
        let requestsAfter = await reverseCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore, "拒否した操作で逆算し直さない")
    }

    /// A3: 1行削れば上限は解け、また追加できる。
    func testRemovingAnObservationBelowTheLimitAllowsAddingAgain() async throws {
        let viewModel = await loadedViewModel(await makeStub())
        fillObservationRowsToLimit(viewModel)
        let removedID = try XCTUnwrap(viewModel.observations.last?.id)

        await viewModel.removeObservation(id: removedID)
        XCTAssertEqual(viewModel.observations.count, RequestLimits.maxObservations - 1)
        XCTAssertFalse(viewModel.observationsReachedLimit)

        viewModel.addObservation()
        XCTAssertEqual(viewModel.observations.count, RequestLimits.maxObservations)
        XCTAssertTrue(viewModel.observationsReachedLimit)
    }

    /// A4: 全行に有効な値を入れても、送る観測は契約の上限を超えない。
    func testRequestNeverExceedsTheObservationLimit() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        fillObservationRowsToLimit(viewModel)

        for (index, id) in viewModel.observations.map(\.id).enumerated() {
            await viewModel.editObservation(id: id, text: "\(index + 1)")
        }

        let request = try await lastRequest(stub)
        XCTAssertEqual(request.observations.count, RequestLimits.maxObservations, "契約の上限を超えない")
        XCTAssertEqual(request.observations.first, .percent(1), "行の順で送る(既存の規則を変えない)")
    }

    // MARK: - 相手の持ち物候補の件数(A5〜A7)

    /// A5: 「持ち物なし」(null)を含めて契約の上限ちょうどまで選べる。
    func testSelectingUpToTheItemCandidateLimitKeepsTheRequestWithinTheContract() async throws {
        let stub = await makeStub(itemCount: Self.overflowingItemCount)
        let viewModel = await loadedViewModel(stub)
        try await enterFirstObservation(viewModel)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = Array(ids.prefix(RequestLimits.maxSelectableItemCandidates))

        for id in selectable {
            await viewModel.toggleOpponentItemCandidate(itemId: id)
        }

        XCTAssertEqual(viewModel.opponentItemCandidateIds, selectable, "マスタの順(既存の規則)")
        XCTAssertTrue(viewModel.opponentItemCandidatesReachedLimit)

        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates.count, RequestLimits.maxItemCandidates, "null 1件 + 選んだ \(selectable.count) 件")
        XCTAssertNil(try XCTUnwrap(request.itemCandidates.first), "先頭は「持ち物なし」")
        XCTAssertEqual(request.itemCandidates.filter { $0 == nil }.count, 1, "null は1件だけ(uniqueItems)")
        XCTAssertEqual(Set(request.itemCandidates.compactMap { $0 }).count, selectable.count, "ID の重複なし")
    }

    /// A6: 上限到達中の ON 操作は拒否する。選択も要求も動かさない(黙って選ばれたことにしない)。
    func testTogglingOnBeyondTheItemCandidateLimitIsRejectedWithoutRequesting() async throws {
        let stub = await makeStub(itemCount: Self.overflowingItemCount)
        let viewModel = await loadedViewModel(stub)
        try await enterFirstObservation(viewModel)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = Array(ids.prefix(RequestLimits.maxSelectableItemCandidates))
        for id in selectable {
            await viewModel.toggleOpponentItemCandidate(itemId: id)
        }
        let requestsBefore = await reverseCount(stub)
        let rejected = ids[RequestLimits.maxSelectableItemCandidates]

        await viewModel.toggleOpponentItemCandidate(itemId: rejected)

        XCTAssertFalse(viewModel.opponentItemCandidateIds.contains(rejected), "上限を超えて選べない")
        XCTAssertEqual(viewModel.opponentItemCandidateIds, selectable, "選択は1つも変わらない")
        XCTAssertTrue(viewModel.opponentItemCandidatesReachedLimit)
        let requestsAfter = await reverseCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore, "拒否した操作で逆算し直さない")
    }

    /// A6: 上限に達していても選択解除はできる(外せなくなると詰む)。解除すれば上限は解け、計算も走る。
    func testDeselectingIsAlwaysAllowedEvenAtTheLimit() async throws {
        let stub = await makeStub(itemCount: Self.overflowingItemCount)
        let viewModel = await loadedViewModel(stub)
        try await enterFirstObservation(viewModel)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = Array(ids.prefix(RequestLimits.maxSelectableItemCandidates))
        for id in selectable {
            await viewModel.toggleOpponentItemCandidate(itemId: id)
        }
        let requestsBefore = await reverseCount(stub)

        await viewModel.toggleOpponentItemCandidate(itemId: selectable[0])

        XCTAssertEqual(viewModel.opponentItemCandidateIds, Array(selectable.dropFirst()))
        XCTAssertFalse(viewModel.opponentItemCandidatesReachedLimit, "1つ外せば上限は解ける")
        let requestsAfter = await reverseCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore + 1, "解除はトグル1回で計算1回(既存の規則)")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates.count, RequestLimits.maxItemCandidates - 1)
    }

    /// A7(回帰): 上限に達していないときの振る舞いは変えない。
    func testTogglingBelowTheLimitStillWorksAsBefore() async throws {
        let stub = await makeStub(itemCount: 4)
        let viewModel = await loadedViewModel(stub)
        try await enterFirstObservation(viewModel)
        let ids = viewModel.itemOptions.map(\.id)
        let requestsBefore = await reverseCount(stub)

        await viewModel.toggleOpponentItemCandidate(itemId: ids[1])
        await viewModel.toggleOpponentItemCandidate(itemId: ids[0])

        XCTAssertEqual(viewModel.opponentItemCandidateIds, [ids[0], ids[1]], "トグル順ではなくマスタの順")
        XCTAssertFalse(viewModel.opponentItemCandidatesReachedLimit)
        let requestsAfter = await reverseCount(stub)
        XCTAssertEqual(requestsAfter, requestsBefore + 2, "トグル1回で計算1回")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates, [nil, ids[0], ids[1]])

        // 観測の追加も従来どおり(上限に達していない限り必ず1行増える)。
        let rowsBefore = viewModel.observations.count
        viewModel.addObservation()
        XCTAssertEqual(viewModel.observations.count, rowsBefore + 1)
        XCTAssertEqual(viewModel.observations.last?.text, "")
        XCTAssertFalse(viewModel.observationsReachedLimit)
    }

    // MARK: - guard の位置の回帰(critic 指摘)

    /// critic 指摘: 上限到達中の ON 拒否の guard は `beginInput()` より前でなければならない
    /// (ADR-0501「issue #110」8章)。後ろに動かすと、拒否操作でも世代(`latestRequestToken`)が
    /// 進んでしまい、拒否の直前から進行中だった逆算の応答が「追い越された」ことにされ、
    /// `isLoading` が解けないまま残る。
    func testRejectedToggleDoesNotAdvanceGenerationOfAnInFlightReverse() async throws {
        let stub = await makeStub(itemCount: Self.overflowingItemCount)
        let viewModel = await loadedViewModel(stub)
        try await enterFirstObservation(viewModel)
        let ids = viewModel.itemOptions.map(\.id)
        let selectable = Array(ids.prefix(RequestLimits.maxSelectableItemCandidates))
        for id in selectable {
            await viewModel.toggleOpponentItemCandidate(itemId: id)
        }
        XCTAssertTrue(viewModel.opponentItemCandidatesReachedLimit)
        let rejected = ids[RequestLimits.maxSelectableItemCandidates]

        await stub.setReverseMode(.manual)
        let baseline = await reverseCount(stub)
        let inFlight = Task { await viewModel.selectMyItem(id: ids[0]) }
        try await stub.waitForReverseRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading)

        // 上限到達中の ON 操作(拒否される)。この操作で世代を進めてはいけない。
        await viewModel.toggleOpponentItemCandidate(itemId: rejected)
        XCTAssertFalse(viewModel.opponentItemCandidateIds.contains(rejected), "上限を超えて選べない")
        XCTAssertEqual(viewModel.opponentItemCandidateIds, selectable, "拒否操作で選択は変わらない")

        // 拒否操作の直前から進行中だった逆算の応答は、ちゃんと反映されること
        // (guard が `beginInput()` より前にあれば世代は進んでいないはず)。
        await stub.resolveReverseWithEcho(at: baseline)
        await inFlight.value

        XCTAssertFalse(viewModel.isLoading, "拒否操作が世代を進めて、進行中の逆算を追い越したことにしてはいけない")
        XCTAssertNil(viewModel.error)
        XCTAssertNotNil(viewModel.result)
    }
}
