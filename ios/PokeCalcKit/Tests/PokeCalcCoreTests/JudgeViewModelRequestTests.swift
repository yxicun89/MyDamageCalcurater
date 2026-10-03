import XCTest

@testable import PokeCalcCore

/// `JudgeViewModel.makeRequest()`(P6-25。ADR-0504 §5): 入力 → 要求。省略の規則は Web(ADR-0705 §6)と同じ。送信前の検査は契約の範囲と同じ(ADR-0705 §7)。
@MainActor
final class JudgeViewModelRequestTests: XCTestCase {
    private typealias H = JudgeHarness

    // MARK: - 組み立て

    func testFilledInputBuildsTheMinimalRequest() async throws {
        let viewModel = await H.makeFilled()
        let request = try H.request(viewModel)
        XCTAssertEqual(request.format, .single, "iOS の画面は single 固定(ユーザー決定でダブルは対象外)")
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id, "load() が入れる既定の性格")
        XCTAssertEqual(request.attacker.sp, zeroSP)
        XCTAssertEqual(request.moveId, StubMaster.physicalMove.id, "自分の技は要求直下")
        XCTAssertEqual(request.defenders.count, 1)
        XCTAssertEqual(request.defenders[0].individual.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.defenders[0].moveId, StubMaster.specialMove.id, "候補の技は候補の欄")
        XCTAssertNil(request.attacker.ranks, "ランクが5項目すべて 0 なら送らない")
        XCTAssertNil(request.attacker.abilityId)
        XCTAssertNil(request.attacker.itemId, "未選択の特性・持ち物は欄ごと送らない")
        XCTAssertNil(request.defenders[0].individual.ranks)
        XCTAssertNil(request.speedField, "場の効果が3つとも false なら送らない")
    }

    func testRanksAreSentOnlyWhenAnyIsNonZeroAndThenAllFiveAreSent() async throws {
        let viewModel = await H.makeFilled()
        viewModel.setRank(.spe, 1, for: .attacker)
        var request = try H.request(viewModel)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 0, def: 0, spa: 0, spd: 0, spe: 1), "1つでも 0 でなければ5項目すべて")
        XCTAssertNil(request.defenders[0].individual.ranks, "ランクは体ごと")
        viewModel.setRank(.spe, 0, for: .attacker)
        request = try H.request(viewModel)
        XCTAssertNil(request.attacker.ranks, "全部 0 に戻せばまた送らない")
        viewModel.setRank(.def, -2, for: .candidate(0))
        request = try H.request(viewModel)
        XCTAssertEqual(request.defenders[0].individual.ranks, RankBlock(atk: 0, def: -2, spa: 0, spd: 0, spe: 0))
    }

    func testAbilityAndItemAreSentOnlyWhenSelected() async throws {
        let viewModel = await H.makeFilled()
        viewModel.setAbility(StubMaster.ability.id, for: .attacker)
        viewModel.setItem(StubMaster.itemB.id, for: .candidate(0))
        let request = try H.request(viewModel)
        XCTAssertEqual(request.attacker.abilityId, StubMaster.ability.id)
        XCTAssertNil(request.attacker.itemId)
        XCTAssertNil(request.defenders[0].individual.abilityId)
        XCTAssertEqual(request.defenders[0].individual.itemId, StubMaster.itemB.id)
    }

    func testSpeedFieldIsSentOnlyWhenAnyIsTrueAndThenAllThreeAreSent() async throws {
        let viewModel = await H.makeFilled()
        viewModel.setDefenderTailwind(true)
        var request = try H.request(viewModel)
        XCTAssertEqual(request.speedField, JudgeSpeedField(trickRoom: false, attackerTailwind: false, defenderTailwind: true))
        viewModel.setTrickRoom(true)
        request = try H.request(viewModel)
        XCTAssertEqual(request.speedField, JudgeSpeedField(trickRoom: true, attackerTailwind: false, defenderTailwind: true))
        viewModel.setTrickRoom(false)
        viewModel.setDefenderTailwind(false)
        request = try H.request(viewModel)
        XCTAssertNil(request.speedField)
    }

    func testSPValuesArePassedThrough() async throws {
        let viewModel = await H.makeFilled()
        viewModel.setSP(.hp, 1, for: .attacker)
        viewModel.setSP(.spe, 32, for: .attacker)
        viewModel.setSP(.atk, 32, for: .candidate(0))
        let request = try H.request(viewModel)
        XCTAssertEqual(request.attacker.sp, StatBlock(hp: 1, atk: 0, def: 0, spa: 0, spd: 0, spe: 32))
        XCTAssertEqual(request.defenders[0].individual.sp, StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0))
    }

    func testDefendersKeepTheCandidateOrderEachWithItsOwnMove() async throws {
        let viewModel = await H.makeFilled()
        viewModel.addCandidate()
        await viewModel.setSpecies(H.gamma, for: .candidate(1))
        viewModel.setMove(StubMaster.alphaOnlyMove, for: .candidate(1))
        let request = try H.request(viewModel)
        XCTAssertEqual(request.defenders.map(\.individual.speciesKey), [StubMaster.beta.key, StubMaster.gamma.key])
        XCTAssertEqual(request.defenders.map(\.moveId), [StubMaster.specialMove.id, StubMaster.alphaOnlyMove.id], "技は候補ごとに別")
    }

    func testTheSameSpeciesTwiceIsNotMerged() async throws {
        let viewModel = await H.makeFilled()
        viewModel.addCandidate()
        await viewModel.setSpecies(H.beta, for: .candidate(1))
        viewModel.setMove(StubMaster.specialMove, for: .candidate(1))
        XCTAssertEqual(try H.request(viewModel).defenders.count, 2, "同じ候補が重複していても取りまとめない(ADR-0703 §6)")
    }

    // MARK: - 検査(必須)

    func testFreshInputIsMissingRequiredFieldsOfTheAttackerFirst() {
        let viewModel = H.makeViewModel()
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.attacker))
    }

    func testEachRequiredFieldOfTheAttackerIsChecked() async {
        for clear in [
            { (vm: JudgeViewModel) in vm.setNature(nil, for: .attacker) },
            { (vm: JudgeViewModel) in vm.setMove(nil, for: .attacker) },
        ] {
            let viewModel = await H.makeFilled()
            clear(viewModel)
            XCTAssertEqual(H.violation(viewModel), .requiredMissing(.attacker))
        }
        let noSpecies = H.makeViewModel()
        await noSpecies.load()
        noSpecies.setMove(StubMaster.physicalMove, for: .attacker)
        XCTAssertEqual(H.violation(noSpecies), .requiredMissing(.attacker), "種族が未選択")
    }

    func testCandidateRequiredFieldsAreCheckedAndNameTheCandidate() async {
        let viewModel = await H.makeFilled()
        viewModel.addCandidate()
        await viewModel.setSpecies(H.gamma, for: .candidate(1))
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.candidate(1)), "候補2は技が未選択")
        viewModel.setMove(StubMaster.specialMove, for: .candidate(1))
        XCTAssertNoThrow(try H.request(viewModel))
        viewModel.setNature(nil, for: .candidate(1))
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.candidate(1)), "性格が未選択")
    }

    func testViolationOrderIsAttackerThenCandidatesInIndexOrder() async {
        let viewModel = await H.makeFilled()
        viewModel.addCandidate()
        viewModel.addCandidate()
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.candidate(1)), "候補を index 昇順に見て最初の1件")
        viewModel.setMove(nil, for: .attacker)
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.attacker), "自分が先")
    }

    // MARK: - 検査(能力ポイントの合計・技 ID の長さ)

    func testSPTotalAtTheLimitPassesAndOverTheLimitIsRejected() async throws {
        let viewModel = await H.makeFilled()
        viewModel.setSP(.atk, 32, for: .attacker)
        viewModel.setSP(.def, 32, for: .attacker)
        viewModel.setSP(.spe, SPLimits.maxTotal - 64, for: .attacker)
        XCTAssertNoThrow(try H.request(viewModel), "合計 \(SPLimits.maxTotal) ちょうどは通る")
        viewModel.setSP(.spe, SPLimits.maxTotal - 64 + 1, for: .attacker)
        XCTAssertEqual(H.violation(viewModel), .spTotalExceeded(.attacker))
    }

    func testCandidateSPTotalIsCheckedPerCandidate() async {
        let viewModel = await H.makeFilled()
        viewModel.setSP(.atk, 32, for: .candidate(0))
        viewModel.setSP(.def, 32, for: .candidate(0))
        viewModel.setSP(.spe, 3, for: .candidate(0))
        XCTAssertEqual(H.violation(viewModel), .spTotalExceeded(.candidate(0)))
    }

    func testRequiredIsCheckedBeforeTheSPTotalWithinOneIndividual() async {
        let viewModel = await H.makeFilled()
        viewModel.setSP(.atk, 32, for: .attacker)
        viewModel.setSP(.def, 32, for: .attacker)
        viewModel.setSP(.spe, 3, for: .attacker)
        viewModel.setMove(nil, for: .attacker)
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.attacker))
    }

    func testMoveIdLongerThanTheContractMaximumIsRejected() async throws {
        let viewModel = await H.makeFilled()
        let atLimit = Move(
            id: String(repeating: "a", count: RequestLimits.maxJudgeMoveIdLength), nameJa: "テストわざながい", type: .normal,
            category: .physical, power: 40)
        viewModel.setMove(atLimit, for: .attacker)
        XCTAssertNoThrow(try H.request(viewModel), "最大長ちょうどは通る")
        let over = Move(
            id: String(repeating: "a", count: RequestLimits.maxJudgeMoveIdLength + 1), nameJa: "テストわざながすぎ", type: .normal,
            category: .physical, power: 40)
        viewModel.setMove(over, for: .attacker)
        XCTAssertEqual(H.violation(viewModel), .moveIdInvalid(.attacker))
        viewModel.setMove(StubMaster.physicalMove, for: .attacker)
        viewModel.setMove(over, for: .candidate(0))
        XCTAssertEqual(H.violation(viewModel), .moveIdInvalid(.candidate(0)))
    }

    // MARK: - submit の検査(違反なら呼ばない)

    func testSubmitWithAViolationDoesNotCallTheServiceAndShowsTheReason() async {
        let service = StubJudgeService()
        let viewModel = await H.makeFilled(service: service)
        viewModel.setMove(nil, for: .candidate(0))
        viewModel.submit()
        await viewModel.settle()
        XCTAssertEqual(viewModel.validationError, .requiredMissing(.candidate(0)))
        XCTAssertEqual(viewModel.validationError?.message, "相手候補1: ポケモン・性格・技をすべて選んでください")
        XCTAssertEqual(viewModel.resultState, .idle, "呼ばないので結果は変わらない")
        let requests = await service.requests
        XCTAssertTrue(requests.isEmpty)
    }

    func testAViolationDoesNotEraseAnExistingResultAndALaterValidSubmitClearsTheReason() async {
        let service = StubJudgeService()
        let viewModel = await H.makeFilled(service: service)
        viewModel.submit()
        await viewModel.settle()
        guard case .loaded = viewModel.resultState else { return XCTFail("結果が出ていない: \(viewModel.resultState)") }
        let shown = viewModel.resultState

        viewModel.setMove(nil, for: .attacker)
        viewModel.submit()
        XCTAssertNotNil(viewModel.validationError)
        XCTAssertEqual(viewModel.resultState, shown, "違反では直前の結果を消さない")

        viewModel.setMove(StubMaster.physicalMove, for: .attacker)
        viewModel.submit()
        XCTAssertNil(viewModel.validationError, "送れたら理由は消える")
        await viewModel.settle()
    }
}
