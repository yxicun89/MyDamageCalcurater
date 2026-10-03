import XCTest

@testable import PokeCalcCore

/// AJ7: 調整画面の ViewModel — 起動・入力・モードごとの要求の組み立て(ADR-0502 §2・§5、AC2・AC3)。
@MainActor
final class AdjustViewModelTests: XCTestCase {
    private typealias Support = AdjustTestSupport

    // MARK: - 起動(調整 API を呼ばない)

    func testLoadReadsMasterAndDoesNotCallAdjust() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        XCTAssertEqual(viewModel.natureOptions, StubMaster.reverseNatures)
        XCTAssertFalse(viewModel.speciesOptions.isEmpty, "種族の先頭ページを読む")
        XCTAssertEqual(viewModel.itemOptions, [StubMaster.itemA, StubMaster.itemB])
        XCTAssertNil(viewModel.ownSpeciesKey, "既定は未選択(Web と同じ)")
        XCTAssertNil(viewModel.ownNatureId)
        XCTAssertNil(viewModel.opponentSpeciesKey)
        XCTAssertEqual(viewModel.mode, .indices)
        XCTAssertEqual(viewModel.bulkFocus, .both)
        XCTAssertEqual(viewModel.offenseCategory, .physical)
        XCTAssertEqual(viewModel.hits, 1)
        XCTAssertEqual(viewModel.thresholdPercent, 100)
        XCTAssertFalse(viewModel.useGoal)
        for stat in StatKey.allCases {
            XCTAssertEqual(viewModel.ceiling(for: stat), 32, "上限の既定は 32(\(stat))")
            XCTAssertEqual(viewModel.fixedSPText(for: stat), "", "固定 SP の既定は空(= 0)")
        }
        XCTAssertNil(viewModel.outcome)
        XCTAssertNil(viewModel.alertMessage)
        XCTAssertNil(viewModel.learners)
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [], "開いただけでは調整 API を呼ばない")
    }

    func testOptionsMatchContractAndWeb() {
        XCTAssertEqual(AdjustViewModel.hitsOptions, Array(1...10))
        XCTAssertEqual(AdjustViewModel.thresholdOptions, [100, 90, 75, 50])
        XCTAssertEqual(AdjustViewModel.stabModifier, 6144)
        XCTAssertEqual(AdjustViewModel.neutralModifier, 4096)
    }

    // MARK: - 自分の選択

    func testSelectOwnSpeciesLoadsDamageMovesAndAbilities() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        XCTAssertEqual(viewModel.ownSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.ownSpecies?.nameJa, StubMaster.beta.nameJa)
        XCTAssertEqual(viewModel.ownMoveOptions, [StubMaster.specialMove, StubMaster.physicalMove],
                       "learnset の順のダメージ技だけ(変化技を除く)")
        XCTAssertEqual(viewModel.ownAbilityOptions, [StubMaster.ability])
        let batches = await fixture.master.moveBatchRequests
        XCTAssertEqual(batches.last, StubMaster.beta.learnset, "技は moves(ids:) で learnset を引く")
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [])
    }

    func testSelectOwnMoveFollowsCategoryAndRejectsUnknown() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        viewModel.selectOwnMove(id: StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.ownMoveId, StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.offenseCategory, .special, "自分の技を選ぶと攻撃の分類を合わせる")
        viewModel.selectOwnMove(id: StubMaster.statusMove.id)
        XCTAssertEqual(viewModel.ownMoveId, StubMaster.specialMove.id, "選択肢に無い技(変化技)は受け付けない")
        viewModel.selectOwnMove(id: nil)
        XCTAssertNil(viewModel.ownMoveId)
    }

    func testChangingOwnSpeciesDropsMoveNotInNewLearnset() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        await viewModel.selectOwnSpecies(key: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.ownMoveId, StubMaster.physicalMove.id, "新しい learnset にもある技は残す")
        await viewModel.selectOwnSpecies(key: StubMaster.statusOnly.key)
        XCTAssertNil(viewModel.ownMoveId, "新しい learnset に無い技は外す")
        XCTAssertEqual(viewModel.ownMoveOptions, [])
    }

    func testFixedSPTotalSumsValidFields() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        viewModel.setFixedSPText("4", for: .hp)
        viewModel.setFixedSPText("32", for: .atk)
        viewModel.setFixedSPText("abc", for: .def)
        XCTAssertEqual(viewModel.fixedSPText(for: .hp), "4")
        XCTAssertEqual(viewModel.fixedSPText(for: .def), "abc", "入力の文字はそのまま保つ")
        XCTAssertEqual(viewModel.fixedSPTotal, 36, "読めない欄は合計に足さない")
    }

    func testSettersRejectOutOfRangeChoices() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        viewModel.setCeiling(20, for: .def)
        XCTAssertEqual(viewModel.ceiling(for: .def), 20)
        viewModel.setCeiling(33, for: .def)
        viewModel.setCeiling(-1, for: .def)
        XCTAssertEqual(viewModel.ceiling(for: .def), 20, "上限は 0...32 だけ")
        viewModel.selectHits(3)
        viewModel.selectHits(11)
        viewModel.selectHits(0)
        XCTAssertEqual(viewModel.hits, 3, "発数は 1...maxAdjustHits だけ")
        viewModel.selectThreshold(75)
        viewModel.selectThreshold(60)
        XCTAssertEqual(viewModel.thresholdPercent, 75, "確率は選択肢の値だけ")
        viewModel.selectOffenseCategory(.special)
        viewModel.selectOffenseCategory(.status)
        XCTAssertEqual(viewModel.offenseCategory, .special, "変化は選べない")
    }

    func testNeedsOpponentAndOpponentAttacksFollowModeAndGoal() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        let expectations: [(AdjustMode, Bool, needs: Bool, attacks: Bool)] = [
            (.indices, false, false, false), (.indices, true, false, false),
            (.bulk, false, false, false), (.bulk, true, true, true),
            (.offense, false, false, false), (.offense, true, true, false),
            (.minKo, false, true, false), (.minSurvive, false, true, true),
        ]
        for (mode, useGoal, needs, attacks) in expectations {
            viewModel.selectMode(mode)
            viewModel.setUseGoal(useGoal)
            XCTAssertEqual(viewModel.needsOpponent, needs, "\(mode) goal=\(useGoal)")
            XCTAssertEqual(viewModel.opponentAttacks, attacks, "\(mode) goal=\(useGoal)")
        }
    }

    func testInputChangesNeverCallAdjustAndModeSwitchKeepsInputs() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        viewModel.setFixedSPText("10", for: .hp)
        viewModel.setMinSpeedText("100")
        for mode in AdjustMode.allCases { viewModel.selectMode(mode) }
        viewModel.setUseGoal(true)
        viewModel.selectHits(2)
        XCTAssertEqual(viewModel.fixedSPText(for: .hp), "10", "モードを切り替えても入力は消さない")
        XCTAssertEqual(viewModel.minSpeedText, "100")
        XCTAssertEqual(viewModel.opponentSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.ownMoveId, StubMaster.physicalMove.id)
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [], "入力の変更では調整 API を呼ばない")
    }

    // MARK: - 要求の組み立て: indices

    func testIndicesModeSendsOnlyIndicesWithStabModifier() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnBeta(viewModel)
        viewModel.selectOwnMove(id: StubMaster.specialMove.id)
        viewModel.selectOwnAbility(id: StubMaster.ability.id)
        viewModel.selectOwnItem(id: StubMaster.itemA.id)
        viewModel.setFixedSPText("4", for: .hp)
        viewModel.setFixedSPText("32", for: .spa)

        await viewModel.submit()

        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [.indices(AdjustIndicesRequest(
            individual: Individual(
                speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id,
                sp: StatBlock(hp: 4, atk: 0, def: 0, spa: 32, spd: 0, spe: 0),
                abilityId: StubMaster.ability.id, itemId: StubMaster.itemA.id
            ),
            moveId: StubMaster.specialMove.id,
            modifier: 6144,
            damageModifier: nil
        ))], "ほのおの種族のほのお技はタイプ一致(×1.5 = 6144)。damageModifier は送らない")
        XCTAssertNotNil(viewModel.outcome)
        XCTAssertNil(viewModel.outcome?.modeResult)
    }

    func testIndicesWithoutStabOrMoveOmitsModifier() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnBeta(viewModel)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        await viewModel.submit()
        viewModel.selectOwnMove(id: nil)
        await viewModel.submit()

        let requests = await fixture.adjust.calls(of: .indices)
        guard requests.count == 2, case .indices(let nonStab) = requests[0], case .indices(let noMove) = requests[1] else {
            return XCTFail("indices が2回呼ばれていない: \(requests)")
        }
        XCTAssertEqual(nonStab.moveId, StubMaster.physicalMove.id)
        XCTAssertNil(nonStab.modifier, "タイプ一致でなければ 4096 なので送らない")
        XCTAssertNil(noMove.moveId)
        XCTAssertNil(noMove.modifier)
        XCTAssertEqual(noMove.individual.sp, Support.zero, "空欄の固定 SP は 0")
    }

    // MARK: - 要求の組み立て: 最小 SP

    func testMinKoSendsIndicesAndKOWithDefenderPreset() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        viewModel.selectMode(.minKo)
        viewModel.selectOpponentDefenderPreset(.full)
        viewModel.selectHits(2)
        viewModel.selectThreshold(90)
        viewModel.setFixedSPText("8", for: .spe)

        await viewModel.submit()

        let indices = await fixture.adjust.calls(of: .indices)
        XCTAssertEqual(indices.count, 1, "indices とモードの操作の両方を呼ぶ")
        let ko = await fixture.adjust.calls(of: .ko)
        XCTAssertEqual(ko, [.ko(AdjustSearchRequest(
            format: .single,
            attacker: Individual(
                speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id,
                sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 8)
            ),
            defender: Individual(
                speciesKey: StubMaster.gamma.key, natureId: StubMaster.defUpNature.id,
                sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0)
            ),
            moveId: StubMaster.physicalMove.id, hits: 2, thresholdPercent: 90
        ))], "相手は防御側プリセット(HB特化 = +def/-atk・H32・B32)。自分の技の分類で B / D が決まる")
        let all = await fixture.adjust.calls
        XCTAssertEqual(all.filter { $0.kind == .survive || $0.kind == .allocation }, [])
    }

    func testMinKoOmitsThresholdWhenCertain() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectOwnMove(id: StubMaster.specialMove.id)
        viewModel.selectMode(.minKo)
        await viewModel.submit()
        let ko = await fixture.adjust.calls(of: .ko)
        guard case .ko(let request)? = ko.first else { return XCTFail("ko が呼ばれていない") }
        XCTAssertNil(request.thresholdPercent, "確定(100%)は契約の既定なので送らない")
        XCTAssertEqual(request.defender.natureId, StubMaster.neutralNature.id, "既定の相手の調整は無振り(無補正)")
        XCTAssertEqual(request.defender.sp, Support.zero)
    }

    func testMinSurviveSendsAttackerPresetAndOpponentMove() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectMode(.minSurvive)
        viewModel.selectOpponentMove(id: StubMaster.specialMove.id)
        viewModel.selectOpponentAttackerPreset(.aFull)
        viewModel.setFixedSPText("4", for: .atk)

        await viewModel.submit()

        let survive = await fixture.adjust.calls(of: .survive)
        XCTAssertEqual(survive, [.survive(AdjustSearchRequest(
            format: .single,
            attacker: Individual(
                speciesKey: StubMaster.gamma.key, natureId: StubMaster.spaUpNature.id,
                sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0)
            ),
            defender: Individual(
                speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id,
                sp: StatBlock(hp: 0, atk: 4, def: 0, spa: 0, spd: 0, spe: 0)
            ),
            moveId: StubMaster.specialMove.id, hits: 1, thresholdPercent: nil
        ))], "相手は攻撃側プリセット(特殊技なので C 特化 = +spa/-atk)")
    }

    func testOpponentMoveOptionsAreOpponentDamageMoves() async {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await viewModel.selectOpponentSpecies(key: StubMaster.beta.key)
        XCTAssertEqual(viewModel.opponentMoveOptions, [StubMaster.specialMove, StubMaster.physicalMove])
        viewModel.selectOpponentMove(id: StubMaster.statusMove.id)
        XCTAssertNil(viewModel.opponentMoveId, "変化技は選べない")
    }

    // MARK: - 要求の組み立て: 配分

    func testBulkWithoutGoalSendsCeilingsForHBDOnly() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnBeta(viewModel)
        viewModel.selectMode(.bulk)
        viewModel.selectBulkFocus(.physical)
        viewModel.setCeiling(20, for: .def)
        viewModel.setFixedSPText("4", for: .spe)

        await viewModel.submit()

        let allocation = await fixture.adjust.calls(of: .allocation)
        XCTAssertEqual(allocation, [.allocation(AdjustAllocationRequest(
            selfIndividual: Individual(
                speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id,
                sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 4)
            ),
            ceiling: AdjustCeiling(hp: 32, def: 20, spd: 32),
            mode: .bulk, focus: .physical, offenseCategory: nil, minSpeed: 0, goal: nil
        ))], "耐久側は H・B・D の上限だけを送り、素早さは見ない")
        let indices = await fixture.adjust.calls(of: .indices)
        XCTAssertEqual(indices.count, 1)
        guard case .allocation(_, let mode, let goalRequested, _)? = viewModel.outcome?.modeResult else {
            return XCTFail("配分の結果が無い")
        }
        XCTAssertEqual(mode, .bulk)
        XCTAssertFalse(goalRequested)
    }

    func testBulkWithGoalSendsAttackerPresetGoal() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectMode(.bulk)
        viewModel.setUseGoal(true)
        viewModel.selectOpponentMove(id: StubMaster.physicalMove.id)
        viewModel.selectHits(2)
        viewModel.selectThreshold(75)

        await viewModel.submit()

        let allocation = await fixture.adjust.calls(of: .allocation)
        guard case .allocation(let request)? = allocation.first else { return XCTFail("allocation が呼ばれていない") }
        XCTAssertEqual(request.goal, AdjustAllocGoal(
            format: .single,
            opponent: Individual(speciesKey: StubMaster.gamma.key, natureId: StubMaster.neutralNature.id, sp: Support.zero),
            moveId: StubMaster.physicalMove.id, hits: 2, thresholdPercent: 75
        ), "bulk の目標の相手は攻撃側(既定の無振り)、技は相手の技")
        guard case .allocation(_, _, let goalRequested, _)? = viewModel.outcome?.modeResult else {
            return XCTFail("配分の結果が無い")
        }
        XCTAssertTrue(goalRequested)
    }

    func testOffenseSendsCategoryCeilingsMinSpeedAndDefenderGoal() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnAndOpponent(viewModel)
        viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        viewModel.selectMode(.offense)
        viewModel.setCeiling(20, for: .spe)
        viewModel.setMinSpeedText("120")
        viewModel.setUseGoal(true)
        viewModel.selectOpponentDefenderPreset(.max)

        await viewModel.submit()

        let allocation = await fixture.adjust.calls(of: .allocation)
        XCTAssertEqual(allocation, [.allocation(AdjustAllocationRequest(
            selfIndividual: Individual(speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id, sp: Support.zero),
            ceiling: AdjustCeiling(atk: 32, spe: 20),
            mode: .offense, focus: nil, offenseCategory: .physical, minSpeed: 120,
            goal: AdjustAllocGoal(
                format: .single,
                opponent: Individual(
                    speciesKey: StubMaster.gamma.key, natureId: StubMaster.neutralNature.id,
                    sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0)
                ),
                moveId: StubMaster.physicalMove.id, hits: 1, thresholdPercent: nil
            )
        ))], "攻撃側は A(物理)と S の上限、目標の相手は防御側プリセット(HB振り)、技は自分の技")
    }

    func testOffenseSpecialWithoutGoalUsesSpaCeilingAndEmptyMinSpeedIsZero() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnBeta(viewModel)
        viewModel.selectMode(.offense)
        viewModel.selectOffenseCategory(.special)

        await viewModel.submit()

        let allocation = await fixture.adjust.calls(of: .allocation)
        guard case .allocation(let request)? = allocation.first else { return XCTFail("allocation が呼ばれていない") }
        XCTAssertEqual(request.ceiling, AdjustCeiling(spa: 32, spe: 32))
        XCTAssertEqual(request.offenseCategory, .special)
        XCTAssertEqual(request.minSpeed, 0, "空欄の素早さの目標は 0(目標なし)")
        XCTAssertNil(request.goal)
        XCTAssertNil(request.focus)
    }

    func testSpeedTargetOfResultIsFixedAtSubmitTime() async throws {
        let fixture = await Support.makeFixture()
        let viewModel = fixture.viewModel
        await Support.selectOwnBeta(viewModel)
        viewModel.selectMode(.offense)
        viewModel.setMinSpeedText("120")
        await viewModel.submit()
        guard case .allocation(_, _, _, let requested)? = viewModel.outcome?.modeResult else {
            return XCTFail("配分の結果が無い")
        }
        XCTAssertTrue(requested)

        viewModel.setMinSpeedText("")
        guard case .allocation(_, _, _, let afterEdit)? = viewModel.outcome?.modeResult else {
            return XCTFail("入力を書き換えても結果は残る")
        }
        XCTAssertTrue(afterEdit, "送信後に素早さの目標を書き換えても、結果の表示(送信時の値)は変わらない")

        await viewModel.submit()
        guard case .allocation(_, _, _, let resent)? = viewModel.outcome?.modeResult else {
            return XCTFail("配分の結果が無い")
        }
        XCTAssertFalse(resent, "送り直すと新しい入力(空 = 目標なし)で決まる")
    }

    func testBulkNeverRequestsSpeedTarget() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.bulk)
        fixture.viewModel.setMinSpeedText("120")
        await fixture.viewModel.submit()
        guard case .allocation(_, _, _, let requested)? = fixture.viewModel.outcome?.modeResult else {
            return XCTFail("配分の結果が無い")
        }
        XCTAssertFalse(requested, "耐久側は素早さの目標を見ない")
    }
}
