import XCTest

@testable import PokeCalcCore

// `ComboViewModel`(CB-8。ADR-0535・ADR-0501「連続技」AC-i1〜AC-i10)。
//
// 形(実装者はこの名前で作る):
// - `@MainActor @Observable public final class ComboViewModel`
//   `init(comboService: any ComboCalcService, master: any PokeCalcService, speedService: (any SpeedService)? = nil)`
// - 状態: `context: ComboContext?`・`isVisible: Bool`(context がある)・`isExpanded: Bool`・`entries: [ComboEntry]`(表示する手)・
//   `extraCandidates: [Move]`・`opponentCandidates: [Move]`(どちらもタイプ順)・`selectedDefender: ComboDefenderOption?`・
//   `defenderWasReset: Bool`・`opponentMoveId: String?`・`opponentSpeedFromDefender: Bool`・`speedField: ComboSpeedField`・
//   `selfSpeed: ComboSelfSpeedInput`・`canAddStep: Bool`・`canClose: Bool`・`state: ComboViewState`・`speedExampleLines: [String]`
// - `ComboEntry`: `.main` / `.extra(key: String, moveId: String, critical: Bool)`(Identifiable。id はメイン "main"・追加の手は key)
// - `ComboViewState`: `.idle` / `.loading` / `.loaded(ComboResultDisplay)` / `.failed(message: String)`
// - 操作(すべて async。入力が変わったら必要なときだけ計算する): `updateContext(_:)`・`open()`・`close()`・`addStep(moveId:)`・
//   `moveStep(at:by:)`・`removeStep(at:)`・`setExtraCritical(key:isOn:)`・`selectDefender(key:)`・`selectOpponentMove(id:)`・
//   `setOpponentSpeedFromDefender(_:)`・`setSpeedField(_:)`・`setSelfSpeedSP(_:)`・`setSelfSpeedNature(_:)`・`setSelfSpeedRank(_:)`・`retry()`
@MainActor
final class ComboViewModelTests: XCTestCase {

    // MARK: - 補助

    private func makeMaster() async -> StubPokeCalcService {
        let master = StubPokeCalcService(
            species: [ComboStub.attacker, ComboStub.defender], moves: ComboStub.allMoves, items: [], natures: ComboStub.natures)
        await master.setMoveBatchMode(.immediate)
        return master
    }

    private func makeViewModel(
        combo: StubComboService = StubComboService(), speed: StubSpeedService? = nil, context: ComboContext? = ComboStub.context()
    ) async -> (ComboViewModel, StubComboService) {
        let master = await makeMaster()
        let viewModel = ComboViewModel(comboService: combo, master: master, speedService: speed)
        await viewModel.updateContext(context)
        return (viewModel, combo)
    }

    private func lastRequest(_ combo: StubComboService) async throws -> ComboRequest {
        let requests = await combo.requests
        return try XCTUnwrap(requests.last, "calcCombo が呼ばれていない")
    }

    private func loadedDisplay(_ viewModel: ComboViewModel, file: StaticString = #filePath, line: UInt = #line) -> ComboResultDisplay? {
        guard case .loaded(let display) = viewModel.state else {
            XCTFail("結果が出ていない: \(viewModel.state)", file: file, line: line)
            return nil
        }
        return display
    }

    private func extraKey(_ viewModel: ComboViewModel, at index: Int) throws -> String {
        guard case .extra(let key, _, _) = try XCTUnwrap(viewModel.entries[safe: index]) else {
            XCTFail("位置 \(index) は追加の手ではない")
            throw XCTSkip("位置 \(index) は追加の手ではない")
        }
        return key
    }

    private func moveIds(_ viewModel: ComboViewModel) -> [String] {
        viewModel.entries.map { entry in
            switch entry {
            case .main: return "main"
            case .extra(_, let moveId, _): return moveId
            }
        }
    }

    // MARK: - AC-i1: 表示と開閉

    func testHiddenWithoutContext() async {
        let (viewModel, combo) = await makeViewModel(context: nil)
        XCTAssertFalse(viewModel.isVisible, "計算画面に結果が無いときは節を出さない")
        await viewModel.open()
        let count = await combo.requests.count
        XCTAssertEqual(count, 0)
    }

    func testCollapsedWithOnlyTheMainStepAndNoRequest() async {
        let (viewModel, combo) = await makeViewModel()
        XCTAssertTrue(viewModel.isVisible)
        XCTAssertFalse(viewModel.isExpanded, "手がメインだけのときは「次の技」ボタンだけ")
        XCTAssertEqual(viewModel.entries, [.main])
        XCTAssertEqual(viewModel.state, .idle)
        let count = await combo.requests.count
        XCTAssertEqual(count, 0, "手が 1 つで相手の技も無ければ計算しない")
    }

    func testOpenLoadsCandidatesAndCloseCollapses() async {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        XCTAssertTrue(viewModel.isExpanded)
        XCTAssertEqual(
            viewModel.extraCandidates,
            MoveSort.byType([ComboStub.attackMove, ComboStub.boostMove, ComboStub.priorityMove, ComboStub.closeMove]),
            "覚える技を moves(ids:) で引き、攻撃技とランクを変える変化技だけ(マスタに無い技は省く)。タイプ順")
        XCTAssertEqual(
            viewModel.opponentCandidates, MoveSort.byType([ComboStub.specialMove, ComboStub.slowDownMove]),
            "相手の技は防御側の覚える攻撃技だけ")
        XCTAssertTrue(viewModel.canAddStep)
        XCTAssertTrue(viewModel.canClose, "手がメインだけで相手の技も無いときだけ閉じられる")
        let count = await combo.requests.count
        XCTAssertEqual(count, 0, "開いただけでは計算しない")
        viewModel.close()
        XCTAssertFalse(viewModel.isExpanded)
    }

    // MARK: - AC-i2〜i4: 手と要求

    func testAddingAStepCalculatesWithTheContractShape() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        XCTAssertEqual(moveIds(viewModel), ["main", ComboStub.boostMove.id])
        XCTAssertFalse(viewModel.canClose, "手が 2 つ以上なら閉じない")
        let request = try await lastRequest(combo)
        let context = ComboStub.context()
        XCTAssertEqual(request.format, .single)
        XCTAssertEqual(request.attacker, context.attacker, "攻撃側は計算画面の値そのまま")
        XCTAssertEqual(request.defender, ComboStub.defenderA.individual, "既定の相手の振りは先頭の行")
        XCTAssertEqual(request.field, context.field)
        XCTAssertEqual(request.steps, [ComboStepInput(moveId: ComboStub.attackMove.id), ComboStepInput(moveId: ComboStub.boostMove.id)])
        XCTAssertNil(request.opponentMoveId)
        XCTAssertEqual(request.opponentSpeed, .unknown)
        XCTAssertEqual(request.speedField, ComboSpeedField())
        XCTAssertNotNil(loadedDisplay(viewModel))
    }

    func testCriticalOfMainComesFromTheCalcScreenAndExtraCriticalIsPerStep() async throws {
        let (viewModel, combo) = await makeViewModel(context: ComboStub.context(mainCritical: true))
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.priorityMove.id)
        var request = try await lastRequest(combo)
        XCTAssertEqual(request.steps.map(\.critical), [true, false], "メインの急所は計算画面の「急所」")

        await viewModel.setExtraCritical(key: try extraKey(viewModel, at: 1), isOn: true)
        request = try await lastRequest(combo)
        XCTAssertEqual(request.steps.map(\.critical), [true, true])
    }

    func testAddStepRejectsNonCandidatesAndTheLimit() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.plainStatusMove.id)
        await viewModel.addStep(moveId: "stub-combo-missing")
        XCTAssertEqual(viewModel.entries, [.main], "候補に無い技(ランクの変わらない変化技・マスタに無い技)は足さない")

        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        await viewModel.addStep(moveId: ComboStub.priorityMove.id)
        XCTAssertEqual(viewModel.entries.count, RequestLimits.maxComboSteps)
        XCTAssertFalse(viewModel.canAddStep, "上限では「追加の技」を出さず案内を出す")
        let before = await combo.requests.count
        await viewModel.addStep(moveId: ComboStub.closeMove.id)
        XCTAssertEqual(viewModel.entries.count, RequestLimits.maxComboSteps)
        let after = await combo.requests.count
        XCTAssertEqual(after, before, "上限を超える追加は計算もしない")
    }

    func testMoveAndRemoveSteps() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        await viewModel.addStep(moveId: ComboStub.priorityMove.id)

        await viewModel.moveStep(at: 1, by: -1)
        XCTAssertEqual(moveIds(viewModel), [ComboStub.boostMove.id, "main", ComboStub.priorityMove.id], "メインより前にも置ける")
        let request = try await lastRequest(combo)
        XCTAssertEqual(request.steps.map(\.moveId), [ComboStub.boostMove.id, ComboStub.attackMove.id, ComboStub.priorityMove.id])

        let count = await combo.requests.count
        await viewModel.moveStep(at: 0, by: -1)
        await viewModel.moveStep(at: 2, by: 1)
        XCTAssertEqual(moveIds(viewModel), [ComboStub.boostMove.id, "main", ComboStub.priorityMove.id], "端より外へは動かない")
        let unchanged = await combo.requests.count
        XCTAssertEqual(unchanged, count, "動かない操作では計算しない")

        await viewModel.removeStep(at: 1)
        XCTAssertEqual(moveIds(viewModel), [ComboStub.boostMove.id, "main", ComboStub.priorityMove.id], "メインは外せない")
        await viewModel.removeStep(at: 0)
        XCTAssertEqual(moveIds(viewModel), ["main", ComboStub.priorityMove.id])
        let last = try await lastRequest(combo)
        XCTAssertEqual(last.steps.map(\.moveId), [ComboStub.attackMove.id, ComboStub.priorityMove.id])
    }

    func testExtraStepsTheAttackerNoLongerLearnsAreHiddenButKept() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        let withoutBoost = ComboStub.attacker.learnset.filter { $0 != ComboStub.boostMove.id }
        await viewModel.updateContext(ComboStub.context(attackerLearnset: withoutBoost))
        XCTAssertEqual(viewModel.entries, [.main], "覚えなくなった追加の手は表示と要求から外す")
        XCTAssertEqual(viewModel.state, .idle, "手が 1 つに戻れば計算しない")
        XCTAssertFalse(viewModel.extraCandidates.contains(ComboStub.boostMove), "候補も攻撃側に合わせて引き直す")

        await viewModel.updateContext(ComboStub.context())
        XCTAssertEqual(moveIds(viewModel), ["main", ComboStub.boostMove.id], "状態は消していないので、また覚えれば戻る(Web と同じ)")
        let request = try await lastRequest(combo)
        XCTAssertEqual(request.steps.count, 2)
    }

    func testOpponentMoveAloneTriggersTheCalc() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.selectOpponentMove(id: ComboStub.boostMove.id)
        XCTAssertNil(viewModel.opponentMoveId, "変化技は相手の技にできない")

        await viewModel.selectOpponentMove(id: ComboStub.specialMove.id)
        XCTAssertFalse(viewModel.canClose, "相手の技を選んでいる間は閉じない")
        let request = try await lastRequest(combo)
        XCTAssertEqual(request.steps.map(\.moveId), [ComboStub.attackMove.id])
        XCTAssertEqual(request.opponentMoveId, ComboStub.specialMove.id)

        await viewModel.selectOpponentMove(id: nil)
        XCTAssertEqual(viewModel.state, .idle, "「選ばない」に戻すと手が 1 つなので計算しない")
    }

    func testOpponentSpeedAndSpeedFieldAreSentOnlyWhenSet() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        await viewModel.setOpponentSpeedFromDefender(true)
        var request = try await lastRequest(combo)
        XCTAssertEqual(request.opponentSpeed, .fromDefender)

        await viewModel.setSpeedField(ComboSpeedField(trickRoom: true, selfTailwind: false, opponentTailwind: true))
        request = try await lastRequest(combo)
        XCTAssertEqual(request.speedField, ComboSpeedField(trickRoom: true, selfTailwind: false, opponentTailwind: true))
    }

    // MARK: - AC-i5: 相手の振り

    func testDefenderSelectionAndResetNotice() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        await viewModel.selectDefender(key: ComboStub.defenderB.key)
        XCTAssertEqual(viewModel.selectedDefender, ComboStub.defenderB)
        var request = try await lastRequest(combo)
        XCTAssertEqual(request.defender, ComboStub.defenderB.individual)

        await viewModel.selectDefender(key: "no-such-row")
        XCTAssertEqual(viewModel.selectedDefender, ComboStub.defenderB, "無い振りは選ばない")

        await viewModel.updateContext(ComboStub.context(defenders: [ComboStub.defenderA]))
        XCTAssertEqual(viewModel.selectedDefender, ComboStub.defenderA, "選んでいた振りが無くなれば先頭に戻す")
        XCTAssertTrue(viewModel.defenderWasReset, "戻したことを知らせる")
        request = try await lastRequest(combo)
        XCTAssertEqual(request.defender, ComboStub.defenderA.individual)

        await viewModel.selectDefender(key: ComboStub.defenderA.key)
        XCTAssertFalse(viewModel.defenderWasReset, "選び直したら案内を消す")
    }

    func testCalcScreenChangesRecalculateAndSameContextDoesNot() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        let before = await combo.requests.count

        await viewModel.updateContext(ComboStub.context())
        let same = await combo.requests.count
        XCTAssertEqual(same, before, "同じ入力なら引き直さない")

        await viewModel.updateContext(ComboStub.context(field: FieldState(weather: .rain)))
        let request = try await lastRequest(combo)
        XCTAssertEqual(request.field.weather, .rain, "計算画面の場が変われば引き直す(UC-5)")
    }

    // MARK: - AC-i6: 自分の素早さ

    func testSelfSpeedInputsOverlayTheAttacker() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        await viewModel.setSelfSpeedSP(10)
        var request = try await lastRequest(combo)
        XCTAssertEqual(request.attacker.sp.spe, 10)

        await viewModel.setSelfSpeedRank(2)
        request = try await lastRequest(combo)
        XCTAssertEqual(request.attacker.ranks.spe, 2)

        await viewModel.setSelfSpeedNature(.plus)
        request = try await lastRequest(combo)
        XCTAssertEqual(request.attacker.natureId, ComboStub.speUpSpaDown.id, "いじっぱり型に素早さ↑ → ようき型")
        XCTAssertEqual(viewModel.selfSpeed, ComboSelfSpeedInput(sp: 10, nature: .plus, rank: 2))

        await viewModel.setSelfSpeedSP(nil)
        request = try await lastRequest(combo)
        XCTAssertEqual(request.attacker.sp.spe, ComboStub.context().attacker.sp.spe, "未入力に戻すと個体の値")

        await viewModel.setSelfSpeedSP(99)
        XCTAssertEqual(viewModel.selfSpeed.sp, 32, "入力時に上限へ丸める")
    }

    func testUnresolvableSelfSpeedNatureShowsANoticeWithoutSending() async throws {
        var context = ComboStub.context()
        context.natures = [ComboStub.atkUp]
        let (viewModel, combo) = await makeViewModel(context: context)
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        let before = await combo.requests.count

        await viewModel.setSelfSpeedNature(.plus)
        XCTAssertEqual(viewModel.state, .failed(message: ComboText.selfSpeedNatureUnavailable))
        let after = await combo.requests.count
        XCTAssertEqual(after, before, "送れない要求は送らない")
    }

    // MARK: - AC-i10: 失敗・もう一度・古い応答

    func testFailureShowsMessageInsideTheSectionAndRetrySendsTheSameRequest() async throws {
        let combo = StubComboService()
        let failure = PokeCalcError(code: PokeCalcError.Code.transport, message: "offline")
        await combo.setResponder { _, index in index == 0 ? .failure(failure) : .success(ComboStub.result()) }
        let (viewModel, _) = await makeViewModel(combo: combo)
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        XCTAssertEqual(viewModel.state, .failed(message: ComboText.failureMessage(for: failure)))
        await viewModel.retry()
        let requests = await combo.requests
        XCTAssertEqual(requests.count, 2)
        XCTAssertEqual(requests[1], requests[0], "もう一度は同じ要求を送る")
        XCTAssertNotNil(loadedDisplay(viewModel))
    }

    func testStaleResponseIsDiscarded() async throws {
        let combo = StubComboService()
        await combo.setResponder { _, index in .success(ComboStub.result(ko: ComboStub.ko(display: index == 0 ? 10.0 : 20.0))) }
        let (viewModel, _) = await makeViewModel(combo: combo)
        await viewModel.open()
        await combo.hold()

        let first = Task { await viewModel.addStep(moveId: ComboStub.boostMove.id) }
        try await combo.waitForRequests(count: 1)
        XCTAssertEqual(viewModel.state, .loading)
        let second = Task { await viewModel.addStep(moveId: ComboStub.priorityMove.id) }
        try await combo.waitForRequests(count: 2)

        await combo.release(at: 1)
        await second.value
        XCTAssertEqual(loadedDisplay(viewModel)?.koText, "2 手で倒す 20.0%")
        await combo.release(at: 0)
        await first.value
        XCTAssertEqual(loadedDisplay(viewModel)?.koText, "2 手で倒す 20.0%", "古い要求の応答は捨てる")
    }

    // MARK: - AC-i8: 素早さの基本値の例

    nonisolated private static let exampleReach = StubSpeed.reach(speed: 198, lines: [
        StubSpeed.reachLine(.neutralMax, slower: SpeedBaseSpeedRange(min: 1, max: 117), faster: SpeedBaseSpeedRange(min: 120, max: 255)),
    ])

    func testSpeedExampleIsFetchedWithTheLastSelfSpeed() async throws {
        let speed = StubSpeedService()
        await speed.setReachResponder { _, _ in .success(Self.exampleReach) }
        let (viewModel, _) = await makeViewModel(speed: speed)
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)

        let calls = await speed.reachCalls
        XCTAssertEqual(calls.last, SpeedPositionRequest(input: .raw(value: 198, pokemonId: nil)), "最後の手の自分の素早さで raw")
        XCTAssertEqual(viewModel.speedExampleLines, ComboText.speedExampleLines(Self.exampleReach))
    }

    func testSpeedExampleIsNotShownInTrickRoomOrWhenOpponentSpeedIsKnownOrOnFailure() async throws {
        let speed = StubSpeedService()
        await speed.setReachResponder { _, _ in .success(Self.exampleReach) }
        let (viewModel, _) = await makeViewModel(speed: speed)
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        XCTAssertFalse(viewModel.speedExampleLines.isEmpty)

        await viewModel.setSpeedField(ComboSpeedField(trickRoom: true))
        XCTAssertEqual(viewModel.speedExampleLines, [], "トリックルーム中は出さない")
        await viewModel.setSpeedField(ComboSpeedField())

        let combo2 = StubComboService()
        await combo2.setResponder { _, _ in
            .success(ComboStub.result(
                speed: ComboSpeedSummary(firstSelfSpeed: 132, lastSelfSpeed: 198, opponentSpeed: 169, opponentSpeedKnown: true)))
        }
        let (known, _) = await makeViewModel(combo: combo2, speed: speed)
        await known.open()
        await known.addStep(moveId: ComboStub.boostMove.id)
        XCTAssertEqual(known.speedExampleLines, [], "相手の素早さが分かるときは出さない")

        let failing = StubSpeedService()
        await failing.setReachResponder { _, _ in .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "x")) }
        let (failed, _) = await makeViewModel(speed: failing)
        await failed.open()
        await failed.addStep(moveId: ComboStub.boostMove.id)
        XCTAssertEqual(failed.speedExampleLines, [], "失敗したら何も出さない")
        XCTAssertNotNil(loadedDisplay(failed), "例の失敗で連続技の結果は壊れない")

        let (noService, _) = await makeViewModel(speed: nil)
        await noService.open()
        await noService.addStep(moveId: ComboStub.boostMove.id)
        XCTAssertEqual(noService.speedExampleLines, [])
    }

    // MARK: - 候補の読み込みの取り消し

    func testCancelledCandidateLoadIsReloadedByTheNextContextWithTheSameLearnset() async throws {
        let master = await makeMaster()
        await master.setMoveBatchMode(.manual)
        let viewModel = ComboViewModel(comboService: StubComboService(), master: master, speedService: nil)
        await viewModel.updateContext(ComboStub.context())
        let opening = Task { await viewModel.open() }
        try await master.waitForMoveBatchRequests(count: 1)
        opening.cancel()
        await opening.value
        XCTAssertEqual(viewModel.extraCandidates, [], "取り消された読み込みは候補を書き換えない")

        await master.setMoveBatchMode(.immediate)
        await viewModel.updateContext(ComboStub.context(field: FieldState(weather: .rain)))
        XCTAssertFalse(viewModel.extraCandidates.isEmpty, "覚える技が同じでも、未読なら読み直す")
        XCTAssertFalse(viewModel.opponentCandidates.isEmpty)
    }
}
