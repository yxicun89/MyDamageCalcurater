import XCTest

@testable import PokeCalcCore

/// ナビゲーション案 B(ADR-0531 §2): タブ・区切りの画面は破棄しないので、離れても進行中の入力・送信を取り消さない。
/// 取り消さなくても、最新の入力の結果だけが残る(古い応答で上書きしない)ことを ViewModel で確かめる。
@MainActor
final class RetainedScreenWorkTests: XCTestCase {
    func testOnlyPushedScreensCancelOnDisappear() {
        XCTAssertFalse(ScreenLifetime.retained.cancelsPendingWorkOnDisappear, "タブ・区切りの画面は離れても取り消さない")
        XCTAssertTrue(ScreenLifetime.pushed.cancelsPendingWorkOnDisappear, "積んだ画面は戻ると破棄されるので取り消す")
    }

    /// 入力を変えた直後にタブを離れても(取り消さない)、計算は完了し、結果は新しい入力のものになる。
    func testCalcInputChangedJustBeforeLeavingCompletesWithLatestInput() async throws {
        let stub = StubMaster.makeService()
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        let baseline = await stub.bulkRequests.count
        await stub.setBulkMode(.manual)

        let pending = viewModel.scheduleLatest { await $0.selectAttackerItem(id: StubMaster.itemB.id) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading)
        // ここでタブを離れる(保持する画面なので cancelPendingWork は呼ばない)。
        await stub.resolveBulkWithEcho(at: baseline)
        await pending.value

        XCTAssertFalse(pending.isCancelled, "離れても取り消さない")
        let cancelled = await stub.cancelledBulkRequests
        XCTAssertEqual(cancelled, [], "送信済みの要求を取り消さない")
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.last?.attacker.itemId, StubMaster.itemB.id, "新しい入力で計算した")
        XCTAssertFalse(viewModel.rows.isEmpty, "結果が出る")
        XCTAssertFalse(viewModel.isLoading, "計算中のままにしない")
        XCTAssertNil(viewModel.error)
    }

    /// 取り消さないときも、続けて入力を変えたら古い応答は捨て、新しい入力の結果だけを出す(LatestTaskRunner と世代)。
    func testCalcKeepsOnlyLatestResultWithoutCancelPendingWork() async throws {
        let stub = StubMaster.makeService()
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        let baseline = await stub.bulkRequests.count
        await stub.setBulkMode(.manual)

        let older = viewModel.scheduleLatest { await $0.selectAttackerItem(id: StubMaster.itemA.id) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.selectAttackerItem(id: StubMaster.itemB.id) }
        try await stub.waitForBulkRequests(count: baseline + 2)
        await stub.resolveBulkWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.last?.attacker.itemId, StubMaster.itemB.id)
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemB.id, "入力は新しいほう")
        XCTAssertFalse(viewModel.rows.isEmpty)
        XCTAssertFalse(viewModel.isLoading)
    }
}
