import XCTest

@testable import PokeCalcCore

/// 計算画面の入力 Task の保持と cancel(issue #113。ADR-0501「issue #113 の受け入れ条件(iOS 側)」)。
///
/// 計算画面には「1文字ごとに計算へ渡る自由入力」が無い(種族・技・持ち物はすべて選択と
/// トグルの確定操作)。そのため debounce は持たせず、**最新の Task を1つ保持して先行 Task を
/// cancel する**ところだけを逆算画面とそろえる(ADR の判断2)。
@MainActor
final class CalcViewModelCancellationTests: XCTestCase {

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    func testScheduleLatestCancelsThePreviousInFlightCalc() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.bulkRequests.count
        await stub.setBulkMode(.manual)

        let older = viewModel.scheduleLatest { await $0.selectAttackerItem(id: StubMaster.itemA.id) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.selectAttackerItem(id: StubMaster.itemB.id) }
        try await stub.waitForBulkCancellation(at: baseline)
        try await stub.waitForBulkRequests(count: baseline + 2)
        await stub.resolveBulkWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        XCTAssertTrue(older.isCancelled, "先行の計算 Task を cancel すること")
        let cancelled = await stub.cancelledBulkRequests
        XCTAssertEqual(cancelled, [baseline], "送信済みの古い要求だけが cancel されること")
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.last?.attacker.itemId, StubMaster.itemB.id)
        XCTAssertFalse(viewModel.rows.isEmpty)
        XCTAssertNil(viewModel.error, "意図した cancel を画面 error にしない")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testCancelPendingWorkDoesNotTurnCancellationIntoAScreenError() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.bulkRequests.count
        let previousRows = viewModel.rows.map(\.presetLabel)
        XCTAssertFalse(previousRows.isEmpty, "起動時の計算結果が要る")
        await stub.setBulkMode(.manual)

        let pending = viewModel.scheduleLatest { await $0.selectAttackerItem(id: StubMaster.itemA.id) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading)

        // 画面破棄(`.onDisappear`)相当。
        viewModel.cancelPendingWork()
        try await stub.waitForBulkCancellation(at: baseline)
        await pending.value

        XCTAssertTrue(pending.isCancelled)
        XCTAssertNil(viewModel.error, "CancellationError は画面 error に変換しない")
        XCTAssertFalse(viewModel.isLoading, "cancel したら読み込み中を解く")
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), previousRows, "cancel は結果を消さない")
    }

    /// critic 指摘(issue #113 A5 未達): `catch` を分岐しているのが `calcBulk` を包む `performCalc` だけでは
    /// 足りない。`selectAttacker` は `species(key:)`(learnset の読み直し)→ `calcBulk` の順で待つので、
    /// `species(key:)` の応答待ち中に画面を離れても `CancellationError` を `error` にしないこと
    /// (`applyAttackerChangeAndRecalculate` の catch。ADR-0501「issue #113」5章)。
    func testCancelPendingWorkDuringSpeciesLookupDoesNotTurnCancellationIntoAScreenError() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let previousRows = viewModel.rows.map(\.presetLabel)
        XCTAssertFalse(previousRows.isEmpty, "起動時の計算結果が要る")
        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        // 種族の変更は `species(key:)`(learnset の読み直し)を経由する(`calcBulk` の手前)。
        let pending = viewModel.scheduleLatest { await $0.selectAttacker(speciesKey: StubMaster.gamma.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading)

        // 画面破棄(`.onDisappear`)相当。`calcBulk` に届く前(species 取得中)の cancel。
        viewModel.cancelPendingWork()
        try await stub.waitForSpeciesCancellation(at: baseline)
        await pending.value

        XCTAssertTrue(pending.isCancelled)
        XCTAssertNil(viewModel.error, "species(key:) 中の CancellationError を画面 error に変換しない")
        XCTAssertFalse(viewModel.isLoading, "cancel したら読み込み中を解く")
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), previousRows, "cancel は結果を消さない")
    }
}
