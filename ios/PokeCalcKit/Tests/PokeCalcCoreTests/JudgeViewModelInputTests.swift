import XCTest

@testable import PokeCalcCore

/// `JudgeViewModel` の入力の操作(P6-25。ADR-0504 §4・§5): 初期値・収める範囲・対象の取り違えがない・候補の増減・種族選択。
@MainActor
final class JudgeViewModelInputTests: XCTestCase {
    private typealias H = JudgeHarness

    // MARK: - 初期値

    func testInitialState() {
        let viewModel = H.makeViewModel()
        XCTAssertEqual(viewModel.attacker, JudgeDraft())
        XCTAssertEqual(viewModel.candidates, [JudgeDraft()], "候補は空の1件から始まる")
        XCTAssertEqual(viewModel.speedField, JudgeSpeedField())
        XCTAssertEqual(viewModel.resultState, .idle)
        XCTAssertNil(viewModel.validationError)
        XCTAssertNil(viewModel.masterFailure)
        XCTAssertTrue(viewModel.canAddCandidate)
        XCTAssertFalse(viewModel.canRemoveCandidate, "最低1件は残す")
    }

    func testDraftDefaults() {
        let draft = JudgeDraft()
        XCTAssertNil(draft.speciesKey)
        XCTAssertNil(draft.natureId)
        XCTAssertEqual(draft.sp, zeroSP)
        XCTAssertEqual(draft.ranks, RankBlock())
        XCTAssertNil(draft.abilityId)
        XCTAssertNil(draft.itemId)
        XCTAssertNil(draft.moveId)
    }

    func testDraftForTargetReturnsTheTargetsDraftOrNil() {
        let viewModel = H.makeViewModel()
        XCTAssertEqual(viewModel.draft(for: .attacker), JudgeDraft())
        XCTAssertEqual(viewModel.draft(for: .candidate(0)), JudgeDraft())
        XCTAssertNil(viewModel.draft(for: .candidate(1)))
        XCTAssertNil(viewModel.draft(for: .candidate(-1)))
    }

    // MARK: - 収める範囲(能力ポイントは各 0〜32・ランクは ±6。送信前検査は合計だけを見る)

    func testSetSPClampsEachStatToTheContractRangeAndLeavesTheOthers() {
        let viewModel = H.makeViewModel()
        viewModel.setSP(.spe, 99, for: .attacker)
        XCTAssertEqual(viewModel.attacker.sp.spe, SPLimits.maxPerStat)
        viewModel.setSP(.spe, -5, for: .attacker)
        XCTAssertEqual(viewModel.attacker.sp.spe, 0)
        viewModel.setSP(.hp, 7, for: .attacker)
        viewModel.setSP(.atk, 8, for: .attacker)
        viewModel.setSP(.def, 9, for: .attacker)
        viewModel.setSP(.spa, 10, for: .attacker)
        viewModel.setSP(.spd, 11, for: .attacker)
        XCTAssertEqual(viewModel.attacker.sp, StatBlock(hp: 7, atk: 8, def: 9, spa: 10, spd: 11, spe: 0), "6項目がそれぞれ独立")
    }

    func testSetRankClampsToTheContractRangeAndIgnoresHP() {
        let viewModel = H.makeViewModel()
        viewModel.setRank(.atk, 9, for: .attacker)
        XCTAssertEqual(viewModel.attacker.ranks.atk, RankLimits.max)
        viewModel.setRank(.atk, -9, for: .attacker)
        XCTAssertEqual(viewModel.attacker.ranks.atk, RankLimits.min)
        viewModel.setRank(.def, 1, for: .attacker)
        viewModel.setRank(.spa, 2, for: .attacker)
        viewModel.setRank(.spd, 3, for: .attacker)
        viewModel.setRank(.spe, 4, for: .attacker)
        XCTAssertEqual(viewModel.attacker.ranks, RankBlock(atk: RankLimits.min, def: 1, spa: 2, spd: 3, spe: 4))
        let before = viewModel.attacker.ranks
        viewModel.setRank(.hp, 3, for: .attacker)
        XCTAssertEqual(viewModel.attacker.ranks, before, "HP のランクは無い")
    }

    // MARK: - 対象

    func testSettersChangeOnlyTheirOwnTarget() {
        let viewModel = H.makeViewModel()
        viewModel.addCandidate()
        viewModel.setNature("n-self", for: .attacker)
        viewModel.setNature("n-first", for: .candidate(0))
        viewModel.setNature("n-second", for: .candidate(1))
        viewModel.setSP(.atk, 10, for: .candidate(1))
        viewModel.setRank(.spe, 2, for: .candidate(0))
        viewModel.setAbility("a-first", for: .candidate(0))
        viewModel.setItem("i-second", for: .candidate(1))
        XCTAssertEqual(viewModel.attacker, JudgeDraft(natureId: "n-self"))
        XCTAssertEqual(viewModel.candidates[0], JudgeDraft(natureId: "n-first", ranks: RankBlock(spe: 2), abilityId: "a-first"))
        XCTAssertEqual(
            viewModel.candidates[safe: 1],
            JudgeDraft(natureId: "n-second", sp: StatBlock(hp: 0, atk: 10, def: 0, spa: 0, spd: 0, spe: 0), itemId: "i-second"))
    }

    func testOperationsOnAnOutOfRangeCandidateAreIgnored() {
        let viewModel = H.makeViewModel()
        viewModel.setNature("x", for: .candidate(5))
        viewModel.setSP(.hp, 3, for: .candidate(-1))
        viewModel.setMove(StubMaster.physicalMove, for: .candidate(1))
        XCTAssertEqual(viewModel.candidates, [JudgeDraft()])
        XCTAssertEqual(viewModel.attacker, JudgeDraft())
    }

    func testSetAbilityItemAndMoveCanBeClearedWithNil() {
        let viewModel = H.makeViewModel()
        viewModel.setAbility("a", for: .attacker)
        viewModel.setItem("i", for: .attacker)
        viewModel.setMove(StubMaster.physicalMove, for: .attacker)
        XCTAssertEqual(viewModel.attacker.abilityId, "a")
        XCTAssertEqual(viewModel.attacker.itemId, "i")
        XCTAssertEqual(viewModel.attacker.moveId, StubMaster.physicalMove.id)
        XCTAssertEqual(viewModel.move(forID: StubMaster.physicalMove.id), StubMaster.physicalMove, "選んだ技は名前を引ける")
        viewModel.setAbility(nil, for: .attacker)
        viewModel.setItem(nil, for: .attacker)
        viewModel.setMove(nil, for: .attacker)
        XCTAssertNil(viewModel.attacker.abilityId)
        XCTAssertNil(viewModel.attacker.itemId)
        XCTAssertNil(viewModel.attacker.moveId)
    }

    func testSpeedFieldSetters() {
        let viewModel = H.makeViewModel()
        viewModel.setTrickRoom(true)
        XCTAssertEqual(viewModel.speedField, JudgeSpeedField(trickRoom: true))
        viewModel.setAttackerTailwind(true)
        viewModel.setDefenderTailwind(true)
        XCTAssertEqual(viewModel.speedField, JudgeSpeedField(trickRoom: true, attackerTailwind: true, defenderTailwind: true))
        viewModel.setTrickRoom(false)
        viewModel.setAttackerTailwind(false)
        viewModel.setDefenderTailwind(false)
        XCTAssertTrue(viewModel.speedField.isDefault)
    }

    // MARK: - 相手候補の増減

    func testAddCandidateUpToTheContractLimit() {
        let viewModel = H.makeViewModel()
        for _ in 1..<RequestLimits.maxJudgeDefenders {
            XCTAssertTrue(viewModel.addCandidate())
        }
        XCTAssertEqual(viewModel.candidates.count, RequestLimits.maxJudgeDefenders)
        XCTAssertFalse(viewModel.canAddCandidate)
        XCTAssertFalse(viewModel.addCandidate(), "上限なら足さない")
        XCTAssertEqual(viewModel.candidates.count, RequestLimits.maxJudgeDefenders)
    }

    func testRemoveCandidateKeepsTheOrderAndAtLeastOne() {
        let viewModel = H.makeViewModel()
        viewModel.addCandidate()
        viewModel.addCandidate()
        viewModel.setNature("first", for: .candidate(0))
        viewModel.setNature("second", for: .candidate(1))
        viewModel.setNature("third", for: .candidate(2))
        XCTAssertTrue(viewModel.canRemoveCandidate)
        XCTAssertTrue(viewModel.removeCandidate(at: 1))
        XCTAssertEqual(viewModel.candidates.map(\.natureId), ["first", "third"], "残りの順を保つ")
        XCTAssertFalse(viewModel.removeCandidate(at: 5), "範囲外は何もしない")
        XCTAssertEqual(viewModel.candidates.count, 2)
        XCTAssertTrue(viewModel.removeCandidate(at: 0))
        XCTAssertEqual(viewModel.candidates.map(\.natureId), ["third"])
        XCTAssertFalse(viewModel.canRemoveCandidate)
        XCTAssertFalse(viewModel.removeCandidate(at: 0), "最後の1件は消せない")
        XCTAssertEqual(viewModel.candidates.count, RequestLimits.minJudgeDefenders)
    }

    func testRemovingACandidateMovesItsNameAndAbilitiesWithIt() async {
        let viewModel = H.makeViewModel()
        viewModel.addCandidate()
        await viewModel.setSpecies(H.alpha, for: .candidate(0))
        await viewModel.setSpecies(H.beta, for: .candidate(1))
        viewModel.removeCandidate(at: 0)
        XCTAssertEqual(viewModel.speciesName(for: .candidate(0)), H.beta.nameJa, "名前は候補について行く(位置に貼り付かない)")
        XCTAssertEqual(viewModel.abilityOptions(for: .candidate(0)), StubMaster.beta.abilities)
    }

    // MARK: - 種族

    func testSetSpeciesStoresKeyAndNameAndFetchesAbilityCandidates() async {
        let master = H.makeMaster()
        let viewModel = H.makeViewModel(master: master)
        await viewModel.setSpecies(H.alpha, for: .attacker)
        XCTAssertEqual(viewModel.attacker.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.speciesName(for: .attacker), StubMaster.alpha.nameJa)
        XCTAssertEqual(viewModel.abilityOptions(for: .attacker), StubMaster.alpha.abilities)
        let requested = await master.speciesRequests
        XCTAssertEqual(requested, [StubMaster.alpha.key], "特性の候補は species(key:) の応答から")
        XCTAssertNil(viewModel.speciesName(for: .candidate(0)), "選んでいない候補は名前なし")
        XCTAssertEqual(viewModel.abilityOptions(for: .candidate(0)), [])
    }

    func testChangingSpeciesDropsAnAbilityThatTheNewSpeciesLacks() async {
        let viewModel = H.makeViewModel()
        await viewModel.setSpecies(SpeciesSummary(detail: StubMaster.abilityXOnly), for: .attacker)
        viewModel.setAbility(StubMaster.abilityX.id, for: .attacker)
        await viewModel.setSpecies(SpeciesSummary(detail: StubMaster.abilityYOnly), for: .attacker)
        XCTAssertNil(viewModel.attacker.abilityId, "新しい種族の候補に無い特性は外す(送ると calc-svc が拒否しうる)")
        XCTAssertEqual(viewModel.abilityOptions(for: .attacker), [StubMaster.abilityY])
    }

    func testChangingSpeciesKeepsAnAbilityThatTheNewSpeciesAlsoHas() async {
        let viewModel = H.makeViewModel()
        await viewModel.setSpecies(SpeciesSummary(detail: StubMaster.abilityXOnly), for: .attacker)
        viewModel.setAbility(StubMaster.abilityX.id, for: .attacker)
        await viewModel.setSpecies(SpeciesSummary(detail: StubMaster.abilityXAndY), for: .attacker)
        XCTAssertEqual(viewModel.attacker.abilityId, StubMaster.abilityX.id)
    }

    /// 特性の候補が取れなくても入力は止めない(特性は任意)。種族の選択は反映される。
    func testSpeciesDetailFailureStillKeepsTheSelection() async {
        let master = H.makeMaster()
        let viewModel = H.makeViewModel(master: master)
        await master.setMasterError(PokeCalcError(code: "upstream_unavailable", message: "x"))
        await viewModel.setSpecies(H.alpha, for: .attacker)
        XCTAssertEqual(viewModel.attacker.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.speciesName(for: .attacker), StubMaster.alpha.nameJa)
        XCTAssertEqual(viewModel.abilityOptions(for: .attacker), [])
    }

    /// 種族を続けて選んだとき、古い種族の応答が遅れて届いても、最新の選択の特性候補を壊さない。
    func testStaleSpeciesDetailDoesNotOverwriteANewerSelection() async throws {
        let master = H.makeMaster()
        await master.setSpeciesMode(.manual)
        let viewModel = H.makeViewModel(master: master)
        let first = Task { await viewModel.setSpecies(H.alpha, for: .attacker) }
        try await master.waitForSpeciesRequests(count: 1)
        let second = Task { await viewModel.setSpecies(H.beta, for: .attacker) }
        try await master.waitForSpeciesRequests(count: 2)
        let betaDetail = try await master.lookupSpecies(key: StubMaster.beta.key)
        let alphaDetail = try await master.lookupSpecies(key: StubMaster.alpha.key)
        await master.resolveSpecies(at: 1, with: .success(betaDetail))
        await second.value
        await master.resolveSpecies(at: 0, with: .success(alphaDetail))
        await first.value
        XCTAssertEqual(viewModel.attacker.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.speciesName(for: .attacker), StubMaster.beta.nameJa)
        XCTAssertEqual(viewModel.abilityOptions(for: .attacker), StubMaster.beta.abilities)
    }

    // MARK: - 送らない(入力のたびに判定を呼ばない。ADR-0705 §7)

    func testEditingInputsNeverCallsTheJudgeService() async {
        let service = StubJudgeService()
        let viewModel = await H.makeFilled(service: service)
        viewModel.setSP(.spe, 20, for: .attacker)
        viewModel.setRank(.atk, 2, for: .candidate(0))
        viewModel.setTrickRoom(true)
        viewModel.addCandidate()
        await viewModel.settle()
        let requests = await service.requests
        XCTAssertTrue(requests.isEmpty, "判定は『判定する』を押したときだけ。debounce もしない")
        XCTAssertEqual(viewModel.resultState, .idle)
    }
}
