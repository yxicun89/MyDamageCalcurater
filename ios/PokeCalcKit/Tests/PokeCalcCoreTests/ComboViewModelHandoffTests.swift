import XCTest

@testable import PokeCalcCore

/// I-ios-21(ADR-0542 §3〜§5・ADR-0501「素早さ画面から連続技へ」AC-h3〜AC-h5): 連続技の節が素早さ画面の値を受け取る
/// `ComboViewModel.applyHandoff(_:)`。
///
/// 規則:
/// - 節を広げる(候補を読む)。手がメインだけなら計算はしない(開いただけでは計算しない = 既存 AC-i1 と同じ)。
/// - 「自分の素早さ」(SP・性格補正・ランク)は `ComboHandoffPlan.selfSpeedInput` の値で**丸ごと上書き**する
///   (前の引き継ぎ・手で入れた値を残さない)。SP は攻撃側の残りの SP の上限に丸める(既存の `setSelfSpeedSP` と同じ)。
/// - それ以外の節の入力(追加の手・相手の振り・相手の技・相手の素早さの切り替え・素早さの場)は触らない。
/// - 計算中の条件(手が 2 つ以上 / 相手の技あり)なら、上書きした素早さで 1 回計算し直す。
/// - スカーフを指定していたら「反映できない」の 1 行を出す。自分の素早さの入力を変えたら消す。次の引き継ぎがスカーフなしでも消す。
/// - 受け取るたびに `appliedHandoffCount` を 1 進める(View はこれでフォーカスを節へ移し、読み上げを 1 回出す)。
@MainActor
final class ComboViewModelHandoffTests: XCTestCase {

    private func makeViewModel(
        combo: StubComboService = StubComboService(), context: ComboContext? = ComboStub.context()
    ) async -> (ComboViewModel, StubComboService) {
        let master = StubPokeCalcService(
            species: [ComboStub.attacker, ComboStub.defender], moves: ComboStub.allMoves, items: [], natures: ComboStub.natures)
        await master.setMoveBatchMode(.immediate)
        let viewModel = ComboViewModel(comboService: combo, master: master)
        await viewModel.updateContext(context)
        return (viewModel, combo)
    }

    private func handoff(speed: SpeedInvestment? = SpeedInvestment(sp: 20, nature: .plus), rank: Int = 1, scarf: Bool = false)
        -> SpeedComboHandoff
    {
        SpeedComboHandoff(
            attackerPokemonID: ComboStub.attacker.key, defenderPokemonID: ComboStub.defender.key, attackerSpeed: speed,
            attackerRank: rank, attackerScarf: scarf)
    }

    private func lastRequest(_ combo: StubComboService) async throws -> ComboRequest {
        let requests = await combo.requests
        return try XCTUnwrap(requests.last, "calcCombo が呼ばれていない")
    }

    // MARK: - AC-h3: 開いて自分の素早さを上書きする

    func testOpensTheSectionAndFillsSelfSpeedWithoutCalculating() async {
        let (viewModel, combo) = await makeViewModel()
        XCTAssertFalse(viewModel.isExpanded, "前提: 閉じている")

        await viewModel.applyHandoff(handoff())

        XCTAssertTrue(viewModel.isExpanded, "節を広げる")
        XCTAssertFalse(viewModel.extraCandidates.isEmpty, "広げたので追加の手の候補を読む")
        XCTAssertEqual(viewModel.selfSpeed, ComboSelfSpeedInput(sp: 20, nature: .plus, rank: 1))
        XCTAssertEqual(viewModel.entries, [.main])
        let count = await combo.requests.count
        XCTAssertEqual(count, 0, "手がメインだけなら開いただけでは計算しない")
        XCTAssertEqual(viewModel.state, .idle)
    }

    func testOverwritesPreviousSelfSpeedInputs() async {
        let (viewModel, _) = await makeViewModel()
        await viewModel.open()
        await viewModel.setSelfSpeedSP(5)
        await viewModel.setSelfSpeedNature(.minus)
        await viewModel.setSelfSpeedRank(-2)

        await viewModel.applyHandoff(handoff(speed: nil, rank: 0))

        XCTAssertEqual(
            viewModel.selfSpeed, ComboSelfSpeedInput(sp: nil, nature: .keep, rank: 0),
            "今の振りの無い引き継ぎは SP・性格を未入力に戻す(前の値を残さない。Web と同じ)")
    }

    func testSPIsClampedToTheAttackersRemainingRoom() async {
        let context = ComboStub.context(attacker: ComboStub.individual(sp: StatBlock(hp: 32, atk: 32, def: 0, spa: 0, spd: 0, spe: 0)))
        let (viewModel, _) = await makeViewModel(context: context)
        await viewModel.applyHandoff(handoff(speed: SpeedInvestment(sp: 32, nature: .neutral), rank: 0))
        XCTAssertEqual(viewModel.selfSpeed.sp, 2, "66 − 64 = 2 に丸める(送れない SP を作らない)")
    }

    func testRecalculatesOnceWithTheHandedOverSpeedWhenAlreadyCalculating() async throws {
        let (viewModel, combo) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        let before = await combo.requests.count
        XCTAssertGreaterThan(before, 0, "前提: 2 手で計算している")

        await viewModel.applyHandoff(handoff(speed: SpeedInvestment(sp: 20, nature: .plus), rank: 1))

        let after = await combo.requests.count
        XCTAssertEqual(after, before + 1, "上書きした素早さで 1 回だけ計算し直す")
        let request = try await lastRequest(combo)
        XCTAssertEqual(request.attacker.sp.spe, 20)
        XCTAssertEqual(request.attacker.ranks.spe, 1)
        XCTAssertEqual(request.attacker.natureId, ComboStub.speUpSpaDown.id, "A上C下に素早さ上がる → S上C下(既存の置き換え規則)")
    }

    /// 節の他の入力は触らない(未入力は触らない・既存の入力を壊さない)。
    func testKeepsOtherComboInputs() async throws {
        let (viewModel, _) = await makeViewModel()
        await viewModel.open()
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        await viewModel.selectDefender(key: ComboStub.defenderB.key)
        await viewModel.selectOpponentMove(id: ComboStub.specialMove.id)
        await viewModel.setOpponentSpeedFromDefender(true)
        await viewModel.setSpeedField(ComboSpeedField(trickRoom: true))
        let entries = viewModel.entries

        await viewModel.applyHandoff(handoff())

        XCTAssertEqual(viewModel.selfSpeed, ComboSelfSpeedInput(sp: 20, nature: .plus, rank: 1), "自分の素早さだけを上書きする")
        XCTAssertEqual(viewModel.entries, entries, "手は変えない")
        XCTAssertEqual(viewModel.selectedDefender?.key, ComboStub.defenderB.key, "相手の振りは変えない")
        XCTAssertEqual(viewModel.opponentMoveId, ComboStub.specialMove.id, "相手の技は変えない")
        XCTAssertTrue(viewModel.opponentSpeedFromDefender, "相手の素早さの切り替えは変えない")
        XCTAssertEqual(viewModel.speedField, ComboSpeedField(trickRoom: true), "素早さの場は引き継がない・変えない")
    }

    // MARK: - AC-h4: スカーフの 1 行

    func testScarfNoticeShowsOnlyAfterAScarfHandoff() async {
        let (viewModel, _) = await makeViewModel()
        XCTAssertFalse(viewModel.handoffScarfNoticeVisible)
        await viewModel.applyHandoff(handoff(scarf: false))
        XCTAssertFalse(viewModel.handoffScarfNoticeVisible)
        await viewModel.applyHandoff(handoff(scarf: true))
        XCTAssertTrue(viewModel.handoffScarfNoticeVisible)
        await viewModel.applyHandoff(handoff(scarf: false))
        XCTAssertFalse(viewModel.handoffScarfNoticeVisible, "次の引き継ぎがスカーフなしなら消す")
    }

    /// 自分の素早さの入力を変えたら消す(Web と同じ「入力を触ると消す」)。
    func testScarfNoticeClearsWhenSelfSpeedIsEdited() async {
        let (viewModel, _) = await makeViewModel()
        await viewModel.applyHandoff(handoff(scarf: true))
        await viewModel.setSelfSpeedRank(2)
        XCTAssertFalse(viewModel.handoffScarfNoticeVisible, "ランクを変えたら消す")

        await viewModel.applyHandoff(handoff(scarf: true))
        await viewModel.setSelfSpeedSP(3)
        XCTAssertFalse(viewModel.handoffScarfNoticeVisible, "SP を変えたら消す")

        await viewModel.applyHandoff(handoff(scarf: true))
        await viewModel.setSelfSpeedNature(.neutral)
        XCTAssertFalse(viewModel.handoffScarfNoticeVisible, "性格補正を変えたら消す")
    }

    func testScarfNoticeStaysWhenOtherInputsChange() async {
        let (viewModel, _) = await makeViewModel()
        await viewModel.applyHandoff(handoff(scarf: true))
        await viewModel.addStep(moveId: ComboStub.boostMove.id)
        await viewModel.setSpeedField(ComboSpeedField(trickRoom: true))
        XCTAssertTrue(viewModel.handoffScarfNoticeVisible, "素早さ以外の入力では消さない")
    }

    // MARK: - AC-h5: フォーカスと読み上げの合図

    func testAppliedHandoffCountAdvancesOncePerHandoff() async {
        let (viewModel, _) = await makeViewModel()
        XCTAssertEqual(viewModel.appliedHandoffCount, 0)
        await viewModel.applyHandoff(handoff())
        XCTAssertEqual(viewModel.appliedHandoffCount, 1)
        await viewModel.applyHandoff(handoff())
        XCTAssertEqual(viewModel.appliedHandoffCount, 2, "同じ値でも受け取るたびに進む")
        await viewModel.setSelfSpeedRank(3)
        await viewModel.open()
        XCTAssertEqual(viewModel.appliedHandoffCount, 2, "利用者の操作では進まない")
    }

    /// 計算画面に結果が無い(節を出せない)ときは計算しない。値は持っておき、結果が出たら節がその値で開いている。
    func testWithoutContextNothingIsSentAndTheInputIsKept() async throws {
        let (viewModel, combo) = await makeViewModel(context: nil)
        await viewModel.applyHandoff(handoff(speed: SpeedInvestment(sp: 20, nature: .plus), rank: 1))
        XCTAssertFalse(viewModel.isVisible)
        let count = await combo.requests.count
        XCTAssertEqual(count, 0)
        XCTAssertEqual(viewModel.selfSpeed, ComboSelfSpeedInput(sp: 20, nature: .plus, rank: 1))

        await viewModel.updateContext(ComboStub.context())
        XCTAssertTrue(viewModel.isVisible)
        XCTAssertTrue(viewModel.isExpanded, "結果が出たら開いた状態で出る")
    }
}
