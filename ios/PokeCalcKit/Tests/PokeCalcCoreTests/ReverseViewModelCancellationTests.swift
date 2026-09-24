import XCTest

@testable import PokeCalcCore

/// 入力変更時の古い計算要求の抑止・キャンセル(issue #113。ADR-0501「issue #113 の受け入れ条件(iOS 側)」)。
///
/// 固定すること:
/// - 観測欄の文字入力は trailing debounce で、確定値の計算だけが始まる(中間値の要求を出さない)。
/// - `ReverseViewModel` は入力の Task を1つだけ保持し、新しい入力・画面破棄で先行 Task を cancel する。
/// - `CancellationError` は画面 `error` にしない(結果も消さない)。
/// - 同期の文字反映・検証(`setObservationText`)は debounce の影響を受けない。
///
/// 待ち時間に依存しないため、debounce は `.zero`(または「終わらないほど長い値」)を注入する
/// (ADR-0501「issue #68」3章の `searchDebounce` と同じ手法)。
@MainActor
final class ReverseViewModelCancellationTests: XCTestCase {

    // MARK: - 補助

    private func makeStub() async -> StubPokeCalcService {
        let stub = StubMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(_ stub: StubPokeCalcService, calcDebounce: Duration = .zero) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub, searchDebounce: .zero, calcDebounce: calcDebounce)
        await viewModel.load()
        return viewModel
    }

    private func firstObservationID(_ viewModel: ReverseViewModel) throws -> Int {
        try XCTUnwrap(viewModel.observations.first?.id, "観測の行が無い")
    }

    /// 観測欄への1回分のキー入力(View と同じ順序: 同期の反映・検証 → 送る観測が変わったときだけ
    /// debounce つきの計算を予約する)。予約した Task を返す(予約しなかったら nil)。
    private func typeObservation(_ viewModel: ReverseViewModel, id: Int, text: String) -> Task<Void, Never>? {
        guard viewModel.setObservationText(id: id, text: text) else { return nil }
        return viewModel.scheduleRecalculationAfterObservationEdit()
    }

    private func reverseCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.reverseRequests.count
    }

    // MARK: - 契約の値

    func testObservationDebounceIntervalIsTwoHundredMilliseconds() {
        XCTAssertEqual(CalcInput.debounceInterval, .milliseconds(200), "issue #113 が決めた 200ms の契約")
    }

    // MARK: - debounce(素早い `4` → `45`)

    func testRapidObservationEditsCalculateOnlyTheFinalValueOnce() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)

        let intermediate = try XCTUnwrap(typeObservation(viewModel, id: id, text: "4"), "中間値でも計算を予約する")
        let final = try XCTUnwrap(typeObservation(viewModel, id: id, text: "45"), "確定値でも計算を予約する")
        await intermediate.value
        await final.value

        XCTAssertTrue(intermediate.isCancelled, "先行の計算 Task は新しい入力で cancel すること")
        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.count, 1, "中間値 4 の計算は始めず、確定値 45 だけ計算する")
        XCTAssertEqual(requests.last?.observations, [.percent(45)])
        XCTAssertNotNil(viewModel.result)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    func testSynchronousTextUpdateIsNotDelayedByTheDebounce() async throws {
        let stub = await makeStub()
        // 実際の 200ms ではなく「十分長い待ち時間」を入れる(テストが壁時計に依存しないようにするため。
        // 200ms という契約値そのものは `testObservationDebounceIntervalIsTwoHundredMilliseconds` が見る)。
        let viewModel = await loadedViewModel(stub, calcDebounce: .seconds(30))
        let id = try firstObservationID(viewModel)

        let pending = try XCTUnwrap(typeObservation(viewModel, id: id, text: "45"))
        XCTAssertEqual(viewModel.observations.first?.text, "45", "TextField の表示は待たずに反映する")
        XCTAssertEqual(viewModel.observations.first?.observation, .percent(45), "検証も待たずに行う")
        XCTAssertNil(viewModel.observations.first?.error)
        let duringDebounce = await reverseCount(stub)
        XCTAssertEqual(duringDebounce, 0, "debounce 中は計算を始めない")

        // 画面破棄(`.onDisappear`)相当。待機中の計算は始まらずに終わる。
        viewModel.cancelPendingWork()
        await pending.value
        XCTAssertTrue(pending.isCancelled)
        let afterCancel = await reverseCount(stub)
        XCTAssertEqual(afterCancel, 0, "debounce 中に画面を離れたら要求を出さない")
    }

    // MARK: - 送信済みの要求の cancel

    func testNewObservationEditCancelsTheInFlightReverseRequest() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseMode(.manual)

        let older = try XCTUnwrap(typeObservation(viewModel, id: id, text: "4"))
        try await stub.waitForReverseRequests(count: 1)
        let newer = try XCTUnwrap(typeObservation(viewModel, id: id, text: "45"))
        try await stub.waitForReverseCancellation(at: 0)
        try await stub.waitForReverseRequests(count: 2)
        await stub.resolveReverseWithEcho(at: 1)
        await newer.value
        await older.value

        XCTAssertTrue(older.isCancelled)
        let cancelled = await stub.cancelledReverseRequests
        XCTAssertEqual(cancelled, [0], "送信済みの古い要求だけが cancel されること")
        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.last?.observations, [.percent(45)])
        XCTAssertNil(viewModel.error, "意図した cancel を画面 error にしない")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNotNil(viewModel.result)
    }

    func testCancelPendingWorkCancelsTheInFlightRequestWithoutShowingError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        // 先に1回成功させて結果を持たせる(cancel で結果が消えないことを見るため)。
        await viewModel.editObservation(id: id, text: "12")
        let previousCandidates = try XCTUnwrap(viewModel.result?.candidates.map(\.spRangeText))

        await stub.setReverseMode(.manual)
        let pending = try XCTUnwrap(typeObservation(viewModel, id: id, text: "45"))
        try await stub.waitForReverseRequests(count: 2)
        XCTAssertTrue(viewModel.isLoading)

        viewModel.cancelPendingWork()
        try await stub.waitForReverseCancellation(at: 1)
        await pending.value

        XCTAssertTrue(pending.isCancelled)
        XCTAssertNil(viewModel.error, "CancellationError は画面 error に変換しない")
        XCTAssertFalse(viewModel.isLoading, "cancel したら読み込み中を解く")
        XCTAssertEqual(viewModel.result?.candidates.map(\.spRangeText), previousCandidates,
                       "cancel は結果を消さない(表示中の最後の結果を残す)")
    }

    // MARK: - 確定操作(select・toggle・行削除)

    func testConfirmedSelectionIsNotDebounced() async throws {
        let stub = await makeStub()
        // 確定操作が debounce に巻き込まれていたら、この待ち合わせ(1ms ポーリング)では終わらない。
        let viewModel = await loadedViewModel(stub, calcDebounce: .seconds(30))
        let id = try firstObservationID(viewModel)
        await viewModel.editObservation(id: id, text: "12")
        let baseline = await reverseCount(stub)
        // 待ち合わせに失敗しても 30 秒眠る Task を残さない。
        defer { viewModel.cancelPendingWork() }

        let task = viewModel.scheduleLatest { await $0.selectMyItem(id: StubMaster.itemA.id) }
        try await stub.waitForReverseRequests(count: baseline + 1)
        await task.value

        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.last?.known.itemId, StubMaster.itemA.id)
        XCTAssertNil(viewModel.error)
    }

    func testScheduleLatestCancelsThePreviousInFlightRequest() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await viewModel.editObservation(id: id, text: "12")
        let baseline = await reverseCount(stub)
        await stub.setReverseMode(.manual)

        let older = viewModel.scheduleLatest { await $0.selectMyItem(id: StubMaster.itemA.id) }
        try await stub.waitForReverseRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.selectMyItem(id: StubMaster.itemB.id) }
        try await stub.waitForReverseCancellation(at: baseline)
        try await stub.waitForReverseRequests(count: baseline + 2)
        await stub.resolveReverseWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        XCTAssertTrue(older.isCancelled, "確定操作でも先行 Task を1つだけ保持して cancel する")
        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.last?.known.itemId, StubMaster.itemB.id)
        XCTAssertEqual(viewModel.myItemId, StubMaster.itemB.id)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - master 取得(species(key:))中の cancel(critic 指摘: A5 未達だった箇所)

    /// critic 指摘(issue #113 A5 未達): `catch` を分岐しているのが `reverse` を包む `recalculateIfPossible`
    /// だけでは足りない。`selectMySpecies` は `species(key:)`(learnset の読み直し)→ `reverse` の順で
    /// 待つので、`species(key:)` の応答待ち中に画面を離れても `CancellationError` を `error` にしないこと
    /// (`reloadAttackingMovesAndRecalculate` の catch。ADR-0501「issue #113」5章)。
    func testCancelPendingWorkDuringSpeciesLookupDoesNotTurnCancellationIntoAScreenError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        // 先に1回成功させて結果を持たせる(cancel で結果が消えないことを見るため)。
        await viewModel.editObservation(id: id, text: "12")
        let previousCandidates = try XCTUnwrap(viewModel.result?.candidates.map(\.spRangeText))

        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        // 与えたダメージ(側=defender)なので、自分の種族変更は `species(key:)` を経由する(`reverse` の手前)。
        let pending = viewModel.scheduleLatest { await $0.selectMySpecies(key: StubMaster.gamma.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading)

        // 画面破棄(`.onDisappear`)相当。`reverse` に届く前(species 取得中)の cancel。
        viewModel.cancelPendingWork()
        try await stub.waitForSpeciesCancellation(at: baseline)
        await pending.value

        XCTAssertTrue(pending.isCancelled)
        XCTAssertNil(viewModel.error, "species(key:) 中の CancellationError を画面 error に変換しない")
        XCTAssertFalse(viewModel.isLoading, "cancel したら読み込み中を解く")
        XCTAssertEqual(viewModel.result?.candidates.map(\.spRangeText), previousCandidates, "cancel は結果を消さない")
    }
}
