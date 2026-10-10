import XCTest

@testable import PokeCalcCore

/// `SpeedViewModel` の非同期の振る舞い(P6-24。ADR-0503 §5): キャンセル・古い応答の破棄・失敗の独立・debounce。
/// 方式は `ReverseViewModelCancellationTests`(issue #113)と同じ: 待ち時間に依存しないよう、
/// debounce は `.zero`(または「終わらないほど長い値」)を注入し、スタブの hold/release で順序を作る。
@MainActor
final class SpeedViewModelAsyncTests: XCTestCase {
    private let pokemonA = StubSpeed.pokemonA.pokemonId

    private func loaded(
        _ stub: StubSpeedService = StubSpeedService(), debounce: Duration = .zero
    ) async -> (SpeedViewModel, StubSpeedService) {
        let viewModel = SpeedViewModel(service: stub, debounce: debounce)
        await viewModel.load()
        return (viewModel, stub)
    }

    // MARK: - debounce

    func testSpeedInputDebounceMatchesTheCalcInputInterval() {
        XCTAssertEqual(SpeedInput.debounceInterval, CalcInput.debounceInterval)
    }

    /// 素早い入力(`3` → `30` → `301`)では、確定した値の要求だけを1回出す。
    func testRapidRawInputSendsOnlyTheFinalValueOnce() async {
        let (viewModel, stub) = await loaded()
        viewModel.setMode(.raw)
        viewModel.setRawValueText("3")
        viewModel.setRawValueText("30")
        viewModel.setRawValueText("301")
        await viewModel.settle()
        let calls = await stub.positionCalls
        XCTAssertEqual(calls, [SpeedPositionRequest(input: .raw(value: 301, pokemonId: nil))], "中間値の要求は出さない")
        XCTAssertEqual(viewModel.rawValueText, "301", "文字の反映は待たない")
    }

    func testPendingWorkIsCancelledBeforeItSendsAnything() async {
        let (viewModel, stub) = await loaded(debounce: .seconds(30))
        viewModel.selectPokemon(id: pokemonA)
        let duringDebounce = await stub.positionCalls
        XCTAssertEqual(duringDebounce, [], "debounce 中は要求を出さない")
        XCTAssertEqual(viewModel.positionState, .loading)

        viewModel.cancelPendingWork()
        await viewModel.settle()
        let afterCancel = await stub.positionCalls
        XCTAssertEqual(afterCancel, [], "画面を離れたら待機中の要求は出さない")
        XCTAssertFalse(viewModel.positionState.isFailed, "cancel は失敗として表示しない")
    }

    // MARK: - キャンセルと最新世代のみ反映

    func testNewInputCancelsTheInFlightPositionRequest() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.position])
        viewModel.selectPokemon(id: pokemonA)
        try await stub.waitForCalls(.position, count: 1)
        viewModel.setScarf(true)
        try await stub.waitForCancellation(.position, at: 0)
        try await stub.waitForCalls(.position, count: 2)
        await stub.release(.position, at: 1)
        await viewModel.settle()

        let cancelled = await stub.cancelledIndices(.position)
        XCTAssertEqual(cancelled, [0], "送信済みの古い要求だけが cancel される")
        XCTAssertEqual(viewModel.positionState, .loaded(StubSpeed.position(speed: 200)), "最新の要求の応答だけを反映する")
    }

    /// サービスが cancel を無視して古い応答を返しても、画面には出さない(世代で守る)。
    func testStalePositionResponseIsDiscardedEvenIfTheServiceIgnoresCancellation() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.position], ignoringCancellation: true)
        viewModel.selectPokemon(id: pokemonA)
        try await stub.waitForCalls(.position, count: 1)
        viewModel.setScarf(true)
        try await stub.waitForCalls(.position, count: 2)

        // 古い要求(0)が先に返る。新しい要求(1)はまだ。
        await stub.release(.position, at: 0)
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(viewModel.positionState, .loading, "古い応答(speed 100)は表示しない")

        await stub.release(.position, at: 1)
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionState, .loaded(StubSpeed.position(speed: 200)))
    }

    func testStaleTableResponseIsDiscardedEvenIfTheServiceIgnoresCancellation() async throws {
        let (viewModel, stub) = await loaded()
        await stub.setTableResponder { _, _, index in
            .success(StubSpeed.table(tiers: [SpeedTier(speed: 1000 + index, entries: [StubSpeed.entry(StubSpeed.pokemonA, .max)])]))
        }
        await stub.hold([.table], ignoringCancellation: true)
        viewModel.toggleFilterPreset(.max)
        try await stub.waitForCalls(.table, count: 2)
        viewModel.toggleFilterPreset(.maxPlus1)
        try await stub.waitForCalls(.table, count: 3)

        await stub.release(.table, at: 1)
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(viewModel.tableState, .loading, "古い表(1 番目の要求)は表示しない")
        await stub.release(.table, at: 2)
        await viewModel.settle()
        guard case .loaded(let table) = viewModel.tableState else { return XCTFail("読み込めていない") }
        XCTAssertEqual(table.tiers.map(\.speed), [1002])
    }

    func testClearingTheInputCancelsTheRequestAndGoesIdle() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.position])
        viewModel.selectPokemon(id: pokemonA)
        try await stub.waitForCalls(.position, count: 1)
        viewModel.selectPokemon(id: nil)
        try await stub.waitForCancellation(.position, at: 0)
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionState, .idle, "送れる入力が無くなったら idle に戻し、古い応答は出さない")
        let calls = await stub.positionCalls
        XCTAssertEqual(calls.count, 1)
    }

    /// 画面を離れたら送信済みの要求も cancel し、結果を反映しない。
    func testCancelPendingWorkCancelsTheInFlightRequest() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.position])
        viewModel.selectPokemon(id: pokemonA)
        try await stub.waitForCalls(.position, count: 1)
        viewModel.cancelPendingWork()
        try await stub.waitForCancellation(.position, at: 0)
        await viewModel.settle()
        XCTAssertFalse(viewModel.positionState.isFailed, "意図した cancel を画面のエラーにしない")
    }

    /// サービスが `CancellationError` を投げても失敗として表示しない(`CalcViewModel` と同じ)。
    func testCancellationErrorFromTheServiceIsNotShownAsFailure() async {
        let stub = StubSpeedService()
        let (viewModel, _) = await loaded(stub)
        await stub.hold([.position])
        viewModel.selectPokemon(id: pokemonA)
        viewModel.cancelPendingWork()
        await viewModel.settle()
        XCTAssertFalse(viewModel.positionState.isFailed)
        XCTAssertFalse(viewModel.tableState.isFailed)
    }

    // MARK: - 失敗の表示と独立

    func testTableFailureShowsJapaneseMessageAndDoesNotAffectPokemonOrPosition() async {
        let stub = StubSpeedService()
        await stub.setTableResponder { _, _, _ in .failure(PokeCalcError(code: "master_unavailable", message: "english")) }
        let (viewModel, _) = await loaded(stub)
        XCTAssertEqual(viewModel.tableState, .failed(SpeedFailure(code: "master_unavailable")))
        XCTAssertEqual(viewModel.tableRows, [])
        XCTAssertEqual(viewModel.pokemonState, .loaded(StubSpeed.pokemonList.pokemon), "ポケモン一覧は表の失敗に巻き込まれない")

        viewModel.setMode(.raw)
        viewModel.setRawValueText("301")
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionState, .loaded(StubSpeed.position(speed: 100)), "位置は表の失敗に巻き込まれない")
        if case .failed(let failure) = viewModel.tableState {
            XCTAssertEqual(failure.message, "ポケモンのデータを読み込めません", "サーバーの英語 message は出さない")
        }
    }

    func testPokemonFailureStillAllowsRawInputAndKeepsTheTable() async {
        let stub = StubSpeedService()
        await stub.setPokemonResult(.failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "offline")))
        let (viewModel, _) = await loaded(stub)
        XCTAssertEqual(viewModel.pokemonState, .failed(SpeedFailure(code: PokeCalcError.Code.transport)))
        XCTAssertEqual(SpeedFailure(code: PokeCalcError.Code.transport).message, "素早さのサーバーに接続できません")
        XCTAssertEqual(viewModel.tableState, .loaded(StubSpeed.table()))
        XCTAssertEqual(viewModel.filteredPokemon, [], "一覧が無いのでピッカーは空")

        viewModel.setMode(.raw)
        viewModel.setRawValueText("301")
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionState, .loaded(StubSpeed.position(speed: 100)), "raw はポケモンを選ばずに使える")
    }

    func testPositionFailureKeepsTheTableAndRecoversOnNextInput() async {
        let stub = StubSpeedService()
        await stub.setPositionResponder { _, index in
            index == 0 ? .failure(PokeCalcError(code: "unknown_pokemon", message: "english")) : .success(StubSpeed.position(speed: 250))
        }
        let (viewModel, _) = await loaded(stub)
        viewModel.selectPokemon(id: pokemonA)
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionState, .failed(SpeedFailure(code: "unknown_pokemon")))
        XCTAssertEqual(viewModel.tableState, .loaded(StubSpeed.table()))
        XCTAssertEqual(speedsOf(viewModel.tableRows), [300, 250, 200, 100], "位置が失敗しても表はそのまま出す")

        viewModel.setScarf(true)
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionState, .loaded(StubSpeed.position(speed: 250)), "次の入力で失敗の表示は消える")
    }

    func testReloadingTheTableAfterAFailureRecovers() async {
        let stub = StubSpeedService()
        await stub.setTableResponder { _, _, index in
            index == 0 ? .failure(StubSpeed.failure) : .success(StubSpeed.table())
        }
        let (viewModel, _) = await loaded(stub)
        XCTAssertTrue(viewModel.tableState.isFailed)
        viewModel.setTrickRoom(true)
        await viewModel.settle()
        XCTAssertEqual(viewModel.tableState, .loaded(StubSpeed.table()))
    }

    /// 絶対ルール5: speed の失敗は計算・構築に影響しない。`SpeedViewModel` は `PokeCalcService` を知らず、
    /// 失敗は投げ直さずに状態として持つ(`load()` は throw しない)。
    func testLoadNeverThrowsAndSpeedFailuresStayInsideTheViewModel() async {
        let stub = StubSpeedService()
        await stub.setPokemonResult(.failure(StubSpeed.failure))
        await stub.setTableResponder { _, _, _ in .failure(StubSpeed.failure) }
        let (viewModel, _) = await loaded(stub)
        XCTAssertTrue(viewModel.pokemonState.isFailed)
        XCTAssertTrue(viewModel.tableState.isFailed)
        XCTAssertEqual(viewModel.positionState, .idle)
    }

    private func speedsOf(_ rows: [SpeedTableRow]) -> [Int] {
        rows.compactMap { if case .tier(let tier) = $0 { return tier.speed } else { return nil } }
    }
}

extension SpeedLoadState {
    fileprivate var isFailed: Bool {
        if case .failed = self { return true }
        return false
    }
}

extension SpeedPositionState {
    fileprivate var isFailed: Bool {
        if case .failed = self { return true }
        return false
    }
}
