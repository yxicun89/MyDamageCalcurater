import XCTest

@testable import PokeCalcCore

/// `SpeedMatchupViewModel` の状態遷移(I-speed-4c。ADR-0610 §4): 入力 → 読み込み中 → 結果 / 失敗、トリックルーム、
/// 失敗の独立(matchup・reach・ポケモン一覧)、キャンセル・古い応答の破棄・debounce。
/// 方式(削除済みの旧 `SpeedViewModel` のテストから引き継いだ): debounce は `.zero`(または終わらないほど長い値)を注入し、
/// スタブの hold/release で順序を作る(待ち時間に依存しない)。
@MainActor
final class SpeedMatchupViewModelStateTests: XCTestCase {
    private let pokemonA = StubSpeed.pokemonA.pokemonId
    private let pokemonB = StubSpeed.pokemonB.pokemonId
    private let pokemonD = StubSpeed.pokemonD.pokemonId

    private func loaded(
        _ stub: StubSpeedService = StubSpeedService(), debounce: Duration = .zero
    ) async -> (SpeedMatchupViewModel, StubSpeedService) {
        let viewModel = SpeedMatchupViewModel(service: stub, debounce: debounce)
        await viewModel.load()
        return (viewModel, stub)
    }

    private func selectBoth(_ viewModel: SpeedMatchupViewModel) {
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.selectOpponentPokemon(id: pokemonD)
    }

    // MARK: - 入力 → 読み込み中 → 結果

    func testSelectingBothPokemonLoadsTheMatchup() async {
        let (viewModel, stub) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        XCTAssertEqual(viewModel.matchupState, .idle, "相手が未選択なら送らない")
        viewModel.selectOpponentPokemon(id: pokemonD)
        XCTAssertEqual(viewModel.matchupState, .loading, "送れる入力が揃った直後は読み込み中")
        await viewModel.settle()

        let calls = await stub.matchupCalls
        XCTAssertEqual(calls, [try XCTUnwrap(viewModel.matchupRequest)], "要求は 1 回だけ。内容は matchupRequest のとおり")
        guard case .loaded(let matchup) = viewModel.matchupState else { return XCTFail("読み込めていない: \(viewModel.matchupState)") }
        XCTAssertEqual(matchup.lines.count, 4, "既定の想定 4 つぶんの行")
    }

    func testMatchupCardsAreBuiltFromTheLoadedResponse() async {
        let stub = StubSpeedService()
        let natures = [
            SpeedNatureLine(nature: .minus, movesFirst: nil, tie: nil),
            SpeedNatureLine(nature: .neutral, movesFirst: SpeedSPRange(min: 31, max: 32), tie: nil),
            SpeedNatureLine(nature: .plus, movesFirst: SpeedSPRange(min: 18, max: 32), tie: SpeedSPRange(min: 17, max: 17)),
        ]
        await stub.setMatchupResponder { request, _ in
            .success(StubSpeed.matchup(lines: [StubSpeed.matchupLine(.preset(.uninvested), speed: 150, natures: natures)], trickRoom: request.trickRoom))
        }
        let (viewModel, _) = await loaded(stub)
        selectBoth(viewModel)
        await viewModel.settle()
        XCTAssertEqual(viewModel.matchupCards.count, 1)
        XCTAssertEqual(viewModel.matchupCards.first?.headline, "準速で抜ける")
        XCTAssertEqual(viewModel.matchupCards.first?.rows.map(\.text), ["S31 以上で抜ける", "S18 以上で抜ける"])
    }

    func testCardsAreEmptyWhileLoadingAndAfterAFailure() async {
        let stub = StubSpeedService()
        let (viewModel, _) = await loaded(stub)
        await stub.hold([.matchup])
        selectBoth(viewModel)
        XCTAssertEqual(viewModel.matchupCards, [], "読み込み中はカードを出さない(読み込み中の表示に切り替える)")
        try? await stub.waitForCalls(.matchup, count: 1)
        await stub.release(.matchup, at: 0)
        await viewModel.settle()
        XCTAssertFalse(viewModel.matchupCards.isEmpty)
    }

    func testReachLoadsOnlyWhenTheCurrentInvestmentIsSet() async {
        let (viewModel, stub) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        await viewModel.settle()
        XCTAssertEqual(viewModel.reachState, .idle)
        viewModel.applyCurrentPreset(.max)
        XCTAssertEqual(viewModel.reachState, .loading)
        await viewModel.settle()

        let calls = await stub.reachCalls
        XCTAssertEqual(calls, [try XCTUnwrap(viewModel.reachRequest)])
        guard case .loaded(let reach) = viewModel.reachState else { return XCTFail("読み込めていない: \(viewModel.reachState)") }
        XCTAssertEqual(reach.lines.count, 6)
        XCTAssertEqual(viewModel.reachRows.count, 6)
        let matchupCalls = await stub.matchupCalls
        XCTAssertEqual(matchupCalls, [], "相手が未選択なので matchup は呼ばない。reach は相手なしで出る")
    }

    func testTurningTheCurrentInvestmentOffClearsTheReachAndCancelsIt() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.applyCurrentPreset(.max)
        await viewModel.settle()
        XCTAssertNotEqual(viewModel.reachState, .idle)
        viewModel.setCurrentEnabled(false)
        XCTAssertEqual(viewModel.reachState, .idle, "今の振りを外すと「これ以上」は消える")
        XCTAssertEqual(viewModel.reachRows, [])
    }

    func testDeselectingAPokemonGoesBackToIdleWithoutAFailure() async {
        let (viewModel, _) = await loaded()
        selectBoth(viewModel)
        await viewModel.settle()
        viewModel.selectOpponentPokemon(id: nil)
        XCTAssertEqual(viewModel.matchupState, .idle)
        XCTAssertFalse(viewModel.matchupState.isFailed)
        XCTAssertEqual(viewModel.matchupCards, [])
    }

    /// 要求が変わらない入力(同じ値の再設定)では送り直さない。
    func testSettingTheSameValueDoesNotSendAgain() async {
        let (viewModel, stub) = await loaded()
        selectBoth(viewModel)
        await viewModel.settle()
        viewModel.setScarf(false)
        viewModel.setRank(0)
        viewModel.setTrickRoom(false)
        viewModel.setPokemonQuery("テスト")
        await viewModel.settle()
        let count = await stub.matchupCalls.count
        XCTAssertEqual(count, 1)
    }

    // MARK: - どの入力がどの要求を取り直すか

    /// (matchup の呼び出し回数, reach の呼び出し回数)。
    private func callCounts(_ stub: StubSpeedService) async -> (matchup: Int, reach: Int) {
        (await stub.matchupCalls.count, await stub.reachCalls.count)
    }

    /// 自分・相手・今の振りを入れて落ち着かせ、その時点の回数を返す(以降は増分で比べる)。
    private func settledWithEverything() async -> (SpeedMatchupViewModel, StubSpeedService, (matchup: Int, reach: Int)) {
        let (viewModel, stub) = await loaded()
        selectBoth(viewModel)
        viewModel.applyCurrentPreset(.max)
        await viewModel.settle()
        let base = await callCounts(stub)
        XCTAssertGreaterThanOrEqual(base.matchup, 1)
        XCTAssertGreaterThanOrEqual(base.reach, 1)
        return (viewModel, stub, base)
    }

    func testOpponentPokemonRefetchesTheMatchupOnly() async {
        let (viewModel, stub, base) = await settledWithEverything()
        viewModel.selectOpponentPokemon(id: pokemonB)
        await viewModel.settle()
        let now = await callCounts(stub)
        XCTAssertEqual(now.matchup, base.matchup + 1)
        XCTAssertEqual(now.reach, base.reach, "「これ以上」は自分の素早さだけで決まる。相手を替えても取り直さない")
    }

    func testOwnConditionsRefetchBoth() async {
        let (viewModel, stub, base) = await settledWithEverything()
        let changes: [() -> Void] = [
            { viewModel.setScarf(true) }, { viewModel.setRank(1) }, { viewModel.setTailwind(true) },
            { viewModel.setParalysis(true) }, { viewModel.setCurrentSP(20) },
        ]
        for change in changes {
            change()
            await viewModel.settle()
        }
        let now = await callCounts(stub)
        XCTAssertEqual(now.matchup, base.matchup + changes.count)
        XCTAssertEqual(now.reach, base.reach + changes.count)
    }

    func testOpponentTailwindRefetchesBothButOpponentParalysisOnlyTheMatchup() async {
        let (viewModel, stub, base) = await settledWithEverything()
        viewModel.setOpponentTailwind(true)
        await viewModel.settle()
        var now = await callCounts(stub)
        XCTAssertEqual(now.matchup, base.matchup + 1)
        XCTAssertEqual(now.reach, base.reach + 1, "相手側の追い風は reach の tableTailwind")
        let lastReach = await stub.reachCalls.last
        XCTAssertEqual(lastReach?.tableTailwind, true)

        viewModel.setOpponentParalysis(true)
        await viewModel.settle()
        now = await callCounts(stub)
        XCTAssertEqual(now.matchup, base.matchup + 2)
        XCTAssertEqual(now.reach, base.reach + 1, "reach には相手のまひの欄が無い(契約 0.7.0)")
    }

    func testAssumptionChipsRefetchTheMatchupOnly() async {
        let (viewModel, stub, base) = await settledWithEverything()
        viewModel.togglePreset(.maxPlus1)
        await viewModel.settle()
        let now = await callCounts(stub)
        XCTAssertEqual(now.matchup, base.matchup + 1)
        XCTAssertEqual(now.reach, base.reach)
        let last = await stub.matchupCalls.last
        XCTAssertEqual(last?.opponent.assumptions?.count, 5)
    }

    // MARK: - トリックルーム

    func testTrickRoomRefetchesTheMatchupAndSendsTheFlag() async {
        let (viewModel, stub) = await loaded()
        selectBoth(viewModel)
        await viewModel.settle()
        viewModel.setTrickRoom(true)
        XCTAssertEqual(viewModel.matchupState, .loading)
        await viewModel.settle()
        let calls = await stub.matchupCalls
        XCTAssertEqual(calls.map(\.trickRoom), [false, true])
        guard case .loaded(let matchup) = viewModel.matchupState else { return XCTFail("読み込めていない") }
        XCTAssertTrue(matchup.trickRoom)
    }

    /// reach の応答はトリックルームに依存しない(契約)。取り直さず、言い換えだけが切り替わる。
    func testTrickRoomDoesNotRefetchTheReachButChangesItsWording() async {
        let stub = StubSpeedService()
        await stub.setReachResponder { _, _ in
            .success(
                StubSpeed.reach(
                    speed: 170,
                    lines: SpeedPresetID.allCases.map {
                        StubSpeed.reachLine($0, slower: SpeedBaseSpeedRange(min: 1, max: 102), faster: SpeedBaseSpeedRange(min: 104, max: 255))
                    }))
        }
        let (viewModel, _) = await loaded(stub)
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.applyCurrentPreset(.max)
        await viewModel.settle()
        XCTAssertEqual(viewModel.reachRows[safe: 2]?.slowerText, "最速 102 族まで抜ける")

        viewModel.setTrickRoom(true)
        await viewModel.settle()
        let reachCalls = await stub.reachCalls.count
        XCTAssertEqual(reachCalls, 1, "トリックルームでは reach を取り直さない")
        XCTAssertEqual(viewModel.reachRows[safe: 2]?.slowerText, "最速 102 族までは後に動く")
        XCTAssertEqual(viewModel.reachRows[safe: 2]?.fasterText, "最速 104 族以上には先に動ける")
        XCTAssertEqual(viewModel.reachState.isFailed, false)
    }

    // MARK: - 失敗(独立・復帰)

    func testMatchupFailureShowsTheJapaneseMessageAndKeepsTheReach() async {
        let stub = StubSpeedService()
        await stub.setMatchupResponder { _, _ in .failure(PokeCalcError(code: "upstream_unavailable", message: "english")) }
        let (viewModel, _) = await loaded(stub)
        selectBoth(viewModel)
        viewModel.applyCurrentPreset(.max)
        await viewModel.settle()

        XCTAssertEqual(viewModel.matchupState, .failed(SpeedFailure(code: "upstream_unavailable")))
        if case .failed(let failure) = viewModel.matchupState {
            XCTAssertEqual(failure.message, ServiceFailureKind.serverFault.message(feature: "素早さ"), "gateway の 503 はサーバーの障害として伝える(接続できないとは別)。サーバーの英語は出さない")
        }
        XCTAssertEqual(viewModel.matchupCards, [])
        guard case .loaded = viewModel.reachState else { return XCTFail("matchup の失敗が reach を消してはいけない: \(viewModel.reachState)") }
        XCTAssertEqual(viewModel.pokemonState, .loaded(StubSpeed.pokemonList.pokemon), "ポケモン一覧も消えない")
    }

    func testReachFailureKeepsTheMatchup() async {
        let stub = StubSpeedService()
        await stub.setReachResponder { _, _ in .failure(StubSpeed.failure) }
        let (viewModel, _) = await loaded(stub)
        selectBoth(viewModel)
        viewModel.applyCurrentPreset(.max)
        await viewModel.settle()
        XCTAssertEqual(viewModel.reachState, .failed(SpeedFailure(code: "master_unavailable")))
        guard case .loaded = viewModel.matchupState else { return XCTFail("reach の失敗が matchup を消してはいけない") }
        XCTAssertFalse(viewModel.matchupCards.isEmpty)
        XCTAssertEqual(viewModel.reachRows, [])
    }

    func testTheNextInputRecoversFromAFailure() async {
        let stub = StubSpeedService()
        await stub.setMatchupResponder { request, index in
            index == 0 ? .failure(StubSpeed.failure) : .success(StubSpeed.matchup(for: request, index: index))
        }
        let (viewModel, _) = await loaded(stub)
        selectBoth(viewModel)
        await viewModel.settle()
        XCTAssertTrue(viewModel.matchupState.isFailed)
        viewModel.setScarf(true)
        XCTAssertEqual(viewModel.matchupState, .loading, "失敗のあとの入力で読み込み中に戻る")
        await viewModel.settle()
        guard case .loaded = viewModel.matchupState else { return XCTFail("復帰できていない") }
    }

    func testTransportFailureAlsoMeansTheServiceIsUnavailable() async {
        let stub = StubSpeedService()
        await stub.setMatchupResponder { _, _ in .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "x")) }
        let (viewModel, _) = await loaded(stub)
        selectBoth(viewModel)
        await viewModel.settle()
        if case .failed(let failure) = viewModel.matchupState {
            XCTAssertEqual(failure.message, ServiceFailureKind.offline.message(feature: "素早さ"))
        } else {
            XCTFail("失敗になっていない")
        }
    }

    // MARK: - キャンセルと最新世代のみ反映

    func testNewInputCancelsTheInFlightMatchupRequest() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.matchup])
        selectBoth(viewModel)
        try await stub.waitForCalls(.matchup, count: 1)
        viewModel.setScarf(true)
        try await stub.waitForCancellation(.matchup, at: 0)
        try await stub.waitForCalls(.matchup, count: 2)
        await stub.release(.matchup, at: 1)
        await viewModel.settle()

        let cancelled = await stub.cancelledIndices(.matchup)
        XCTAssertEqual(cancelled, [0], "送信済みの古い要求だけが cancel される")
        guard case .loaded(let matchup) = viewModel.matchupState else { return XCTFail("読み込めていない") }
        XCTAssertEqual(matchup.lines.first?.opponentSpeed, 200, "最新(2 番目)の要求の応答だけを反映する")
    }

    /// サービスが cancel を無視して古い応答を返しても、画面には出さない(世代で守る)。
    func testStaleMatchupResponseIsDiscardedEvenIfTheServiceIgnoresCancellation() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.matchup], ignoringCancellation: true)
        selectBoth(viewModel)
        try await stub.waitForCalls(.matchup, count: 1)
        viewModel.setScarf(true)
        try await stub.waitForCalls(.matchup, count: 2)

        await stub.release(.matchup, at: 0)
        for _ in 0..<20 { await Task.yield() }
        XCTAssertEqual(viewModel.matchupState, .loading, "古い応答では更新しない")

        await stub.release(.matchup, at: 1)
        await viewModel.settle()
        guard case .loaded(let matchup) = viewModel.matchupState else { return XCTFail("読み込めていない") }
        XCTAssertEqual(matchup.lines.first?.opponentSpeed, 200)
    }

    func testDeselectingWhileInFlightDiscardsTheLateResponse() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.matchup], ignoringCancellation: true)
        selectBoth(viewModel)
        try await stub.waitForCalls(.matchup, count: 1)
        viewModel.selectOpponentPokemon(id: nil)
        await stub.release(.matchup, at: 0)
        for _ in 0..<20 { await Task.yield() }
        XCTAssertEqual(viewModel.matchupState, .idle, "入力が無くなったあとに古い応答が来ても結果を出さない")
    }

    // MARK: - debounce

    /// 素早い入力(SP の連打)では、確定した値の要求だけを出す。
    func testRapidCurrentSPInputSendsOnlyTheFinalValue() async {
        let (viewModel, stub) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.setCurrentEnabled(true)
        await viewModel.settle()
        for value in 1...5 { viewModel.setCurrentSP(value) }
        await viewModel.settle()
        let calls = await stub.reachCalls
        XCTAssertEqual(calls.last, viewModel.reachRequest)
        XCTAssertLessThan(calls.count, 6, "中間値の要求をすべて出さない")
        XCTAssertEqual(calls.last.map { call -> Int? in if case .custom(_, let sp, _, _, _, _, _) = call.input { return sp } else { return nil } }, 5)
    }

    func testPendingWorkIsCancelledBeforeItSendsAnything() async {
        let (viewModel, stub) = await loaded(debounce: .seconds(30))
        selectBoth(viewModel)
        let duringDebounce = await stub.matchupCalls
        XCTAssertEqual(duringDebounce, [], "debounce 中は要求を出さない")
        XCTAssertEqual(viewModel.matchupState, .loading)

        viewModel.cancelPendingWork()
        await viewModel.settle()
        let afterCancel = await stub.matchupCalls
        XCTAssertEqual(afterCancel, [], "画面を離れたら待機中の要求は出さない")
        XCTAssertFalse(viewModel.matchupState.isFailed, "cancel は失敗として表示しない")
    }

    func testSpeedInputDebounceIsTheSharedInterval() {
        XCTAssertEqual(SpeedInput.debounceInterval, CalcInput.debounceInterval)
    }
}
