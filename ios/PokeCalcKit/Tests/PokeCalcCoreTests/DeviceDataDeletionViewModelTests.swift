import XCTest

@testable import PokeCalcCore

/// P6-7: 「この端末のデータを削除」の状態遷移(ADR-0501「P6-7」3章)。
/// record と team は独立に呼ぶ・partial は上限付きで繰り返す・片方失敗でももう片方は進める・
/// キャンセルを尊重する・両方 completed になってから完了。
@MainActor
final class DeviceDataDeletionViewModelTests: XCTestCase {
    private static let maxRequests = 3
    private let unavailable = PokeCalcError(code: "store_unavailable", message: "テスト")

    private func makeViewModel(_ stub: StubDeviceDataService) -> DeviceDataDeletionViewModel {
        DeviceDataDeletionViewModel(service: stub, maxRequestsPerTarget: Self.maxRequests)
    }

    private func runConfirmed(_ viewModel: DeviceDataDeletionViewModel) async {
        viewModel.requestDeletion()
        await viewModel.confirmDeletion()
    }

    // MARK: - 確認

    func testRequestDeletionOnlyAsksForConfirmationWithoutCalling() async {
        let stub = StubDeviceDataService()
        let viewModel = makeViewModel(stub)
        XCTAssertEqual(viewModel.phase, .idle)
        XCTAssertNil(viewModel.statusMessage)
        viewModel.requestDeletion()
        XCTAssertEqual(viewModel.phase, .confirming)
        XCTAssertNil(viewModel.statusMessage)
        let calls = await stub.calls
        XCTAssertTrue(calls.isEmpty, "確認の前に通信しない")
    }

    func testCancelConfirmationReturnsToIdleWithoutCalling() async {
        let stub = StubDeviceDataService()
        let viewModel = makeViewModel(stub)
        viewModel.requestDeletion()
        viewModel.cancelConfirmation()
        XCTAssertEqual(viewModel.phase, .idle)
        await viewModel.confirmDeletion()
        let calls = await stub.calls
        XCTAssertTrue(calls.isEmpty, "取り消した後の confirm は何もしない")
    }

    func testConfirmWithoutRequestDoesNothing() async {
        let stub = StubDeviceDataService()
        let viewModel = makeViewModel(stub)
        await viewModel.confirmDeletion()
        XCTAssertEqual(viewModel.phase, .idle)
        let calls = await stub.calls
        XCTAssertTrue(calls.isEmpty, "確認なしで削除しない")
    }

    // MARK: - 完了

    func testBothCompletedShowsCompletedOnlyAfterBoth() async {
        let stub = StubDeviceDataService()
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        XCTAssertEqual(viewModel.phase, .finished)
        XCTAssertEqual(viewModel.recordOutcome, .completed)
        XCTAssertEqual(viewModel.teamOutcome, .completed)
        XCTAssertTrue(viewModel.isAllCompleted)
        XCTAssertFalse(viewModel.canRetry)
        XCTAssertEqual(viewModel.statusMessage, DeviceDataText.completed)
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual([record, team], [1, 1])
    }

    func testPartialIsRepeatedUntilCompleted() async {
        let stub = StubDeviceDataService(record: [.progress(.partial), .progress(.partial), .progress(.completed)])
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual([record, team], [3, 1])
        XCTAssertEqual(viewModel.statusMessage, DeviceDataText.completed)
    }

    func testStatusIsDeletingThenPartialNoticeWhileInFlight() async {
        let stub = StubDeviceDataService(record: [.progress(.partial), .progress(.completed)])
        let viewModel = makeViewModel(stub)
        let observed = LockedBox<[String?]>([])
        await stub.setHook { target, nth in
            guard target == .record else { return }
            let message = await MainActor.run { viewModel.statusMessage }
            observed.append(message)
        }
        await runConfirmed(viewModel)
        XCTAssertEqual(observed.value, [DeviceDataText.deleting, DeviceDataText.partialNotice])
    }

    func testConfirmWhileDeletingIsIgnored() async {
        let stub = StubDeviceDataService()
        let viewModel = makeViewModel(stub)
        await stub.setHook { target, nth in
            guard target == .record, nth == 1 else { return }
            await MainActor.run { viewModel.requestDeletion() }
            await viewModel.confirmDeletion()
        }
        await runConfirmed(viewModel)
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual([record, team], [1, 1], "削除中の二重起動で要求が増えない")
        XCTAssertEqual(viewModel.phase, .finished)
    }

    // MARK: - partial の上限

    func testEndlessPartialStopsAtLimitAndOtherTargetStillRuns() async {
        let stub = StubDeviceDataService(record: Array(repeating: .progress(.partial), count: 100))
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual(record, Self.maxRequests, "上限回数で止まる(無限ループしない)")
        XCTAssertEqual(team, 1)
        XCTAssertEqual(viewModel.recordOutcome, .incomplete)
        XCTAssertEqual(viewModel.teamOutcome, .completed)
        XCTAssertEqual(viewModel.phase, .finished)
        XCTAssertEqual(viewModel.statusMessage, DeviceDataText.partialNotice)
        XCTAssertTrue(viewModel.canRetry)
        XCTAssertFalse(viewModel.isAllCompleted)
    }

    func testRetryAfterIncompleteGetsFreshBudgetAndSkipsCompletedTarget() async {
        let stub = StubDeviceDataService(record: Array(repeating: .progress(.partial), count: Self.maxRequests))
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        XCTAssertEqual(viewModel.recordOutcome, .incomplete)
        await viewModel.retry()
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual(record, Self.maxRequests + 1)
        XCTAssertEqual(team, 1, "completed の team は再送しない")
        XCTAssertEqual(viewModel.statusMessage, DeviceDataText.completed)
    }

    // MARK: - 失敗(独立・結果を分ける)

    func testRecordFailureDoesNotStopTeamAndResultsAreSeparated() async {
        let stub = StubDeviceDataService(record: [.failure(unavailable)])
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual([record, team], [1, 1], "失敗は自動で再送しない。team は進める")
        XCTAssertEqual(viewModel.recordOutcome, .failed(code: "store_unavailable"))
        XCTAssertEqual(viewModel.teamOutcome, .completed)
        XCTAssertFalse(viewModel.isAllCompleted)
        XCTAssertTrue(viewModel.canRetry)
        let message = viewModel.statusMessage ?? ""
        XCTAssertTrue(message.contains(DeviceDataText.failure), message)
        XCTAssertTrue(message.contains(DeviceDataText.partlyDeleted(label: DeviceDataText.teamLabel)), message)
        XCTAssertFalse(message.contains(DeviceDataText.completed), "片方だけで完了を出さない")
    }

    func testTeamFailureDoesNotStopRecordAndResultsAreSeparated() async {
        let stub = StubDeviceDataService(team: [.failure(unavailable)])
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        XCTAssertEqual(viewModel.recordOutcome, .completed)
        XCTAssertEqual(viewModel.teamOutcome, .failed(code: "store_unavailable"))
        let message = viewModel.statusMessage ?? ""
        XCTAssertTrue(message.contains(DeviceDataText.failure), message)
        XCTAssertTrue(message.contains(DeviceDataText.partlyDeleted(label: DeviceDataText.recordLabel)), message)
        XCTAssertFalse(message.contains(DeviceDataText.completed))
    }

    func testBothFailShowsFailureOnly() async {
        let stub = StubDeviceDataService(record: [.failure(unavailable)], team: [.failure(unavailable)])
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        XCTAssertEqual(viewModel.statusMessage, DeviceDataText.failure)
        XCTAssertTrue(viewModel.canRetry)
    }

    func testTransportErrorCodeIsCarried() async {
        let transport = PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト")
        let stub = StubDeviceDataService(record: [.failure(transport)])
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        XCTAssertEqual(viewModel.recordOutcome, .failed(code: PokeCalcError.Code.transport))
    }

    func testRetryCallsOnlyFailedTargetAndThenCompletes() async {
        let stub = StubDeviceDataService(record: [.failure(unavailable)])
        let viewModel = makeViewModel(stub)
        await runConfirmed(viewModel)
        await viewModel.retry()
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual([record, team], [2, 1])
        XCTAssertEqual(viewModel.phase, .finished)
        XCTAssertTrue(viewModel.isAllCompleted)
        XCTAssertEqual(viewModel.statusMessage, DeviceDataText.completed)
    }

    func testRetryWhenNothingToRetryDoesNothing() async {
        let stub = StubDeviceDataService()
        let viewModel = makeViewModel(stub)
        await viewModel.retry()
        await runConfirmed(viewModel)
        await viewModel.retry()
        let record = await stub.callCount(.record)
        let team = await stub.callCount(.team)
        XCTAssertEqual([record, team], [1, 1], "idle・全完了のときの retry は通信しない")
    }

    // MARK: - キャンセル

    func testCancellationStopsWithoutFailureAndWithoutCallingTheOtherTarget() async throws {
        let stub = StubDeviceDataService(record: [.hang])
        let viewModel = makeViewModel(stub)
        viewModel.requestDeletion()
        let task = Task { await viewModel.confirmDeletion() }
        try await waitUntilCalled(stub, .record)
        task.cancel()
        await task.value
        let team = await stub.callCount(.team)
        XCTAssertEqual(team, 0, "キャンセル後に続きの要求を送らない")
        XCTAssertEqual(viewModel.phase, .idle)
        XCTAssertEqual(viewModel.recordOutcome, .pending, "キャンセルは失敗にしない")
        XCTAssertNil(viewModel.statusMessage)
    }

    private func waitUntilCalled(_ stub: StubDeviceDataService, _ target: DeviceDataTarget) async throws {
        for _ in 0..<5000 {
            if await stub.callCount(target) > 0 { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("要求が送られない")
    }
}

/// フック(別 executor)から値を溜めるための最小の箱。
final class LockedBox<Value: Sendable>: @unchecked Sendable {
    private let lock = NSLock()
    private var stored: Value
    init(_ value: Value) { stored = value }
    var value: Value { lock.withLock { stored } }
}

extension LockedBox where Value == [String?] {
    func append(_ element: String?) { lock.withLock { stored.append(element) } }
}
