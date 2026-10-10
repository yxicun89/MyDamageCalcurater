import XCTest

@testable import PokeCalcCore

/// F-11(ADR-0525): 調整の「目標」方式の ViewModel。目標の追加・削除・上限、種類の切り替え、送信前の検査の順、
/// 要求の組み立て(省略可の欄を送らない・メガのストーン)、結果、機能なしの扱い、古い応答の無視。
@MainActor
final class AdjustViewModelGoalsTests: XCTestCase {
    private typealias Support = AdjustGoalsTestSupport
    private typealias Mega = StubMegaMaster

    // MARK: - モード

    func testGoalsModeNeedsServiceAndSelectModeLeavesIt() async {
        let without = await Support.makeFixture(withGoalsService: false)
        without.viewModel.selectGoalsMode()
        XCTAssertFalse(without.viewModel.goalsAvailable)
        XCTAssertFalse(without.viewModel.isGoalsMode, "サービスが無ければ選べない")

        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        XCTAssertTrue(viewModel.goalsAvailable)
        XCTAssertFalse(viewModel.isGoalsMode, "既定は従来のモード")
        viewModel.selectMode(.minKo)
        viewModel.selectGoalsMode()
        XCTAssertTrue(viewModel.isGoalsMode)
        XCTAssertFalse(viewModel.needsOpponent, "目標方式では従来の相手の領域を出さない")
        XCTAssertFalse(viewModel.opponentAttacks)
        viewModel.selectMode(.bulk)
        XCTAssertFalse(viewModel.isGoalsMode, "従来のモードを選ぶと目標方式を外れる")
        XCTAssertEqual(viewModel.mode, .bulk)
    }

    // MARK: - 目標の追加・削除・上限

    func testAddRemoveAndLimit() async {
        let viewModel = await Support.makeFixture().viewModel
        var ids: [Int] = []
        for _ in 0..<RequestLimits.maxAdjustGoals { ids.append(viewModel.addGoal()!) }
        XCTAssertEqual(Set(ids).count, RequestLimits.maxAdjustGoals, "id は重ならない")
        XCTAssertFalse(viewModel.canAddGoal)
        XCTAssertNil(viewModel.addGoal(), "上限では足せない")
        XCTAssertEqual(viewModel.goalDrafts.count, 6)

        viewModel.removeGoal(id: ids[1])
        XCTAssertEqual(viewModel.goalDrafts.map(\.id), [ids[0]] + ids[2...])
        XCTAssertTrue(viewModel.canAddGoal)
        let next = viewModel.addGoal()
        XCTAssertNotNil(next)
        XCTAssertFalse(ids.contains(next!), "外した id を再利用しない")
    }

    func testNewGoalDefaultsToOutspeedFastestOneHitCertain() async {
        let viewModel = await Support.makeFixture().viewModel
        XCTAssertTrue(viewModel.goalDrafts.isEmpty, "最初は目標なし")
        let id = viewModel.addGoal()!
        let added = viewModel.goalDrafts[0]
        XCTAssertEqual(added.id, id)
        XCTAssertEqual(added.kind, .outspeed)
        XCTAssertEqual(added.speedPreset, .fastest)
        XCTAssertEqual(added.attackerPreset, .aFull, "耐えるの既定は特化(Web の x_full)")
        XCTAssertEqual(added.defenderPreset, .none)
        XCTAssertEqual(added.hits, 1)
        XCTAssertEqual(added.thresholdPercent, 100)
    }

    // MARK: - 種類の切り替え・選択の制約

    func testKindChangeKeepsOpponentAndResetsPresetsAndMoves() async {
        let viewModel = await Support.makeFixture().viewModel
        await AdjustTestSupport.selectOwnBeta(viewModel)
        let id = await Support.addGoal(viewModel, kind: .survive, opponent: StubMaster.gamma.key)
        viewModel.selectGoalOpponentMove(id: id, moveId: StubMaster.physicalMove.id)
        viewModel.selectGoalAttackerPreset(id: id, .none)
        viewModel.selectGoalHits(id: id, 3)
        viewModel.selectGoalThreshold(id: id, 75)

        viewModel.setGoalKind(id: id, .ko)
        let draft = viewModel.goalDrafts[0]
        XCTAssertEqual(draft.kind, .ko)
        XCTAssertEqual(draft.opponentSpeciesKey, StubMaster.gamma.key, "相手のポケモンは保つ")
        XCTAssertNil(draft.opponentMoveId, "技は新しい種類の既定(未選択)に戻る")
        XCTAssertEqual(draft.attackerPreset, .aFull)
        XCTAssertEqual(draft.hits, 1)
        XCTAssertEqual(draft.thresholdPercent, 100)
    }

    func testMoveChoicesAreLimitedToLearnsetsAndHitsToOptions() async {
        let viewModel = await Support.makeFixture().viewModel
        await AdjustTestSupport.selectOwnBeta(viewModel)
        let id = await Support.addGoal(viewModel, kind: .survive, opponent: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.goalDrafts[0].opponentMoves.map(\.id), [StubMaster.physicalMove.id, StubMaster.specialMove.id],
                       "相手の技はダメージ技だけ(learnset の順)")
        viewModel.selectGoalOpponentMove(id: id, moveId: "stub-move-unknown")
        XCTAssertNil(viewModel.goalDrafts[0].opponentMoveId)
        viewModel.selectGoalOwnMove(id: id, moveId: StubMaster.alphaOnlyMove.id)
        XCTAssertNil(viewModel.goalDrafts[0].ownMoveId, "自分の learnset に無い技は選べない")
        viewModel.selectGoalOwnMove(id: id, moveId: StubMaster.physicalMove.id)
        XCTAssertEqual(viewModel.goalDrafts[0].ownMoveId, StubMaster.physicalMove.id)
        viewModel.selectGoalHits(id: id, 11)
        XCTAssertEqual(viewModel.goalDrafts[0].hits, 1)
        viewModel.selectGoalThreshold(id: id, 60)
        XCTAssertEqual(viewModel.goalDrafts[0].thresholdPercent, 100)
    }

    func testChangingOpponentDropsMoveNotLearnedByNewOpponent() async {
        let viewModel = await Support.makeFixture().viewModel
        await AdjustTestSupport.selectOwnBeta(viewModel)
        let id = await Support.addGoal(viewModel, kind: .survive, opponent: StubMaster.gamma.key)
        viewModel.selectGoalOpponentMove(id: id, moveId: StubMaster.physicalMove.id)
        await viewModel.selectGoalOpponent(id: id, key: StubMaster.alpha.key)
        XCTAssertNil(viewModel.goalDrafts[0].opponentMoveId, "新しい相手が覚えない技の選択は外す")
        XCTAssertEqual(viewModel.goalDrafts[0].opponentMoves.map(\.id), [StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id],
                       "alpha の learnset のダメージ技")
    }

    // MARK: - 送信前の検査(API を呼ばない)

    private func assertRejected(
        _ fixture: Support.Fixture, _ message: String, file: StaticString = #filePath, line: UInt = #line
    ) async {
        XCTAssertEqual(fixture.viewModel.alertMessage, message, file: file, line: line)
        let goalCalls = await fixture.goals.calls
        let adjustCalls = await fixture.adjust.calls
        XCTAssertEqual(goalCalls.count, 0, "検査に違反したら goals を呼ばない", file: file, line: line)
        XCTAssertEqual(adjustCalls.count, 0, "検査に違反したら indices も呼ばない", file: file, line: line)
        XCTAssertNil(fixture.viewModel.outcome, file: file, line: line)
    }

    func testValidationOrderOwnThenFixedSPThenNoGoalsThenPerGoal() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        viewModel.selectGoalsMode()
        await viewModel.submit()
        await assertRejected(fixture, AdjustText.ownRequired)

        await AdjustTestSupport.selectOwnBeta(viewModel)
        viewModel.setFixedSPText("33", for: .hp)
        await viewModel.submit()
        await assertRejected(fixture, AdjustText.spRangeInvalid)

        viewModel.setFixedSPText("", for: .hp)
        await viewModel.submit()
        await assertRejected(fixture, "目標を追加してください")

        let first = await Support.addGoal(viewModel, kind: .survive)
        await Support.addGoal(viewModel, kind: .ko, opponent: StubMaster.gamma.key)
        await viewModel.submit()
        await assertRejected(fixture, "目標 1: 相手のポケモンを選んでください")

        await viewModel.selectGoalOpponent(id: first, key: StubMaster.gamma.key)
        await viewModel.submit()
        await assertRejected(fixture, "目標 1: 相手の技を選んでください")

        viewModel.selectGoalOpponentMove(id: first, moveId: StubMaster.physicalMove.id)
        await viewModel.submit()
        await assertRejected(fixture, "目標 2: 自分の技を選んでください")
    }

    func testMissingSpeedNatureIsRejectedWithGoalNumber() async {
        let fixture = await Support.makeFixture(natures: StubMaster.reverseNatures)
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        await viewModel.submit()
        await assertRejected(fixture, "目標 1: 相手の振り方に合う性格がマスタにありません")
    }

    // MARK: - 要求の組み立て

    func testRequestOmitsOptionalFieldsAndKeepsGoalOrder() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        viewModel.setFixedSPText("4", for: .hp)
        viewModel.selectOwnMove(id: StubMaster.specialMove.id)
        let speed = await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        let survive = await Support.addGoal(viewModel, kind: .survive, opponent: StubMaster.gamma.key)
        let ko = await Support.addGoal(viewModel, kind: .ko, opponent: StubMaster.gamma.key)
        viewModel.selectGoalSpeedPreset(id: speed, .neutralMax)
        viewModel.selectGoalOpponentMove(id: survive, moveId: StubMaster.physicalMove.id)
        viewModel.selectGoalHits(id: survive, 2)
        viewModel.selectGoalThreshold(id: survive, 90)
        viewModel.selectGoalOwnMove(id: ko, moveId: StubMaster.specialMove.id)
        viewModel.selectGoalDefenderPreset(id: ko, .max)

        await viewModel.submit()

        let calls = await fixture.goals.calls
        let request = try XCTUnwrap(calls.first)
        XCTAssertEqual(calls.count, 1)
        XCTAssertEqual(request.format, .single)
        XCTAssertNil(request.ceiling, "ceiling は送らない(各能力 32)")
        XCTAssertEqual(request.selfIndividual.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.selfIndividual.sp.hp, 4, "固定 SP が下限")
        XCTAssertEqual(request.goals.map(\.kind), [.outspeed, .survive, .ko])

        let outspeed = request.goals[0]
        XCTAssertNil(outspeed.moveId, "先に使う技を選ばなければ送らない")
        XCTAssertNil(outspeed.hits)
        XCTAssertNil(outspeed.thresholdPercent)
        XCTAssertEqual(outspeed.opponent.sp.spe, SPLimits.maxPerStat)
        XCTAssertEqual(outspeed.opponent.natureId, StubMaster.neutralNature.id, "準速は無補正")
        XCTAssertEqual(outspeed.opponent.sp.hp, 0)

        let surviveGoal = request.goals[1]
        XCTAssertEqual(surviveGoal.moveId, StubMaster.physicalMove.id)
        XCTAssertEqual(surviveGoal.hits, 2)
        XCTAssertEqual(surviveGoal.thresholdPercent, 90)
        XCTAssertEqual(surviveGoal.opponent.sp.atk, SPLimits.maxPerStat, "既定は A 特化(物理技)")
        XCTAssertEqual(surviveGoal.opponent.natureId, StubMaster.atkUpNature.id)

        let koGoal = request.goals[2]
        XCTAssertEqual(koGoal.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(koGoal.hits, 1)
        XCTAssertNil(koGoal.thresholdPercent, "100(確定)は契約の既定なので送らない")
        XCTAssertEqual(koGoal.opponent.sp.hp, SPLimits.maxPerStat)
        XCTAssertEqual(koGoal.opponent.sp.spd, SPLimits.maxPerStat, "特殊技に対する HD 振り")
    }

    func testBoostMoveIsSentOnlyWhenChosen() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        let id = await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        viewModel.selectGoalBoostMove(id: id, moveId: StubMaster.physicalMove.id)
        viewModel.selectGoalSpeedPreset(id: id, .fastest)
        await viewModel.submit()
        let goalCalls = await fixture.goals.calls
        let request = try XCTUnwrap(goalCalls.first)
        XCTAssertEqual(request.goals[0].moveId, StubMaster.physicalMove.id)
        XCTAssertEqual(request.goals[0].opponent.natureId, Support.speUpNature.id, "最速は素早さ上昇の性格")
    }

    func testMegaOpponentCarriesStoneAndMegaOwnKeepsLockedStone() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await viewModel.selectOwnSpecies(key: Mega.megaAlpha.key)
        viewModel.selectOwnNature(id: StubMaster.neutralNature.id)
        viewModel.selectGoalsMode()
        await Support.addGoal(viewModel, kind: .outspeed, opponent: Mega.megaAlpha.key)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: Mega.megaMissingStone.key)
        await viewModel.submit()
        let goalCalls = await fixture.goals.calls
        let request = try XCTUnwrap(goalCalls.first)
        XCTAssertEqual(request.selfIndividual.itemId, Mega.stone.id, "自分がメガならストーン")
        XCTAssertEqual(request.goals[0].opponent.itemId, Mega.stone.id, "相手がメガならストーン")
        XCTAssertNil(request.goals[1].opponent.itemId)
        XCTAssertNil(request.goals[2].opponent.itemId, "ストーンを引けなければ送らない")
    }

    // MARK: - 結果・並行呼び出し

    func testSubmitCallsIndicesAndGoalsAndBuildsOutcomeWithSnapshots() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        viewModel.selectOwnMove(id: StubMaster.specialMove.id)
        let speed = await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        viewModel.selectGoalBoostMove(id: speed, moveId: StubMaster.physicalMove.id)
        let survive = await Support.addGoal(viewModel, kind: .survive, opponent: StubMaster.gamma.key)
        viewModel.selectGoalOpponentMove(id: survive, moveId: StubMaster.specialMove.id)
        viewModel.selectGoalHits(id: survive, 2)

        await viewModel.submit()

        let indicesCalls = await fixture.adjust.calls(of: .indices)
        XCTAssertEqual(indicesCalls.count, 1, "今の振り方の指数も同時に出す")
        XCTAssertNil(viewModel.alertMessage)
        XCTAssertFalse(viewModel.isLoading)
        let outcome = try XCTUnwrap(viewModel.outcome)
        guard case .goals(let presentation) = outcome.modeResult else { return XCTFail("目標の結果が無い") }
        XCTAssertEqual(presentation.result.goals.count, 2)
        XCTAssertEqual(presentation.snapshots, [
            AdjustGoalSnapshot(
                kind: .outspeed, opponentName: "\(StubMaster.gamma.nameJa)(最速)", boostMoveName: StubMaster.physicalMove.nameJa),
            AdjustGoalSnapshot(
                kind: .survive, opponentName: "\(StubMaster.gamma.nameJa)(C特化)",
                moveName: StubMaster.specialMove.nameJa, hits: 2),
        ])
        XCTAssertEqual(outcome.indices, StubAdjustFixtures.indicesResult(firepower: true))
    }

    func testResultSentenceUsesSnapshotNotLaterInput() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        await viewModel.submit()
        await viewModel.selectGoalOpponent(id: viewModel.goalDrafts[0].id, key: StubMaster.alpha.key)
        guard case .goals(let presentation) = viewModel.outcome?.modeResult else { return XCTFail("結果が無い") }
        XCTAssertEqual(presentation.snapshots[0].opponentName, "\(StubMaster.gamma.nameJa)(最速)", "送信後に相手を変えても結果の名前は変わらない")
    }

    func testUnsupportedMarkProducesNoticeFromGoalsResult() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        await Support.addGoal(viewModel, kind: .ko, opponent: StubMaster.gamma.key)
        viewModel.selectGoalOwnMove(id: viewModel.goalDrafts[0].id, moveId: StubMaster.physicalMove.id)
        var result = StubAdjustGoalsFixtures.result(for: AdjustGoalsRequest(
            format: .single, selfIndividual: Individual(speciesKey: "x", natureId: "y", sp: zeroSP),
            goals: [AdjustGoalInput(kind: .ko, opponent: Individual(speciesKey: "x", natureId: "y", sp: zeroSP))]))
        result.unsupported = [UnsupportedMark(target: .move, reason: .multiHit, id: StubMaster.physicalMove.id)]
        await fixture.goals.setResult(result)
        await viewModel.submit()
        XCTAssertNotNil(viewModel.outcome?.unsupportedNotice)
    }

    // MARK: - エラー・機能なし

    func testTransientErrorShowsMessageAndKeepsInputAndGoalsMode() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        await fixture.goals.setError(PokeCalcError(code: "master_unavailable", message: "internal english"))
        await viewModel.submit()
        XCTAssertEqual(viewModel.alertMessage, AdjustText.errorMessages["master_unavailable"])
        XCTAssertNil(viewModel.outcome)
        XCTAssertTrue(viewModel.isGoalsMode, "一時的な失敗では目標方式のまま")
        XCTAssertTrue(viewModel.goalsAvailable)
        XCTAssertEqual(viewModel.goalDrafts.count, 1, "入力は消さない")

        await fixture.goals.setError(PokeCalcError(code: PokeCalcError.Code.transport, message: "x"))
        await viewModel.submit()
        XCTAssertEqual(viewModel.alertMessage, AdjustText.unavailable)
        XCTAssertTrue(viewModel.goalsAvailable, "通信失敗は機能なしではない")
    }

    func testNotFoundMeansFeatureUnavailableAndFallsBackToClassicAdjust() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        await fixture.goals.setError(PokeCalcError(code: "not_found", message: "route"))
        await viewModel.submit()
        XCTAssertEqual(viewModel.alertMessage, AdjustText.goalsUnavailable)
        XCTAssertFalse(viewModel.goalsAvailable)
        XCTAssertFalse(viewModel.isGoalsMode, "従来の調整に戻る")
        XCTAssertEqual(viewModel.goalDrafts.count, 1, "入力は消さない")
        viewModel.selectGoalsMode()
        XCTAssertFalse(viewModel.isGoalsMode, "機能なしと分かったら選べない")

        // 従来の調整はそのまま使える(絶対ルール5)。
        viewModel.selectMode(.indices)
        await viewModel.submit()
        XCTAssertNil(viewModel.alertMessage)
        XCTAssertNotNil(viewModel.outcome?.indices)
    }

    // MARK: - 取り消し・古い応答

    func testStaleResponseDoesNotOverwriteLatest() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        await Support.addGoal(viewModel, kind: .outspeed, opponent: StubMaster.gamma.key)
        await fixture.goals.setMode(.manual)

        let first = viewModel.scheduleSubmit()
        try await fixture.goals.waitForCalls(count: 1)
        let second = viewModel.scheduleSubmit()
        try await fixture.goals.waitForCalls(count: 2)

        var latest = StubAdjustGoalsFixtures.result(for: await fixture.goals.calls[1])
        latest.remaining = 7
        await fixture.goals.resolve(at: 1, with: latest)
        await second.value
        guard case .goals(let shown) = viewModel.outcome?.modeResult else { return XCTFail("結果が無い") }
        XCTAssertEqual(shown.result.remaining, 7)

        var stale = latest
        stale.remaining = 99
        await fixture.goals.resolve(at: 0, with: stale)
        await first.value
        guard case .goals(let after) = viewModel.outcome?.modeResult else { return XCTFail("結果が消えた") }
        XCTAssertEqual(after.result.remaining, 7, "古い応答で上書きしない")
        XCTAssertFalse(viewModel.isLoading)
    }
}
