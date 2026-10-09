import XCTest

@testable import PokeCalcCore

/// 計算履歴の行から計算画面へ入力を復元する(ADR-0519。`CalcViewModel.loadHistoryCalc(_:)`)。
///
/// 約束: 計算はちょうど1回・復元できる入力(攻撃側の個体・技・防御側の種族/特性/ランク・天候/フィールド/壁/急所)を要求に反映・
/// 防御側の性格/SP/持ち物は画面が表せないので使わない・マスタに無い種族/技は計算の失敗として表示し入力を書き換えない・
/// 古い応答は捨てる。架空のマスタ(`StubMaster`)だけを使う。
@MainActor
final class CalcViewModelHistoryLoadTests: XCTestCase {
    private let sp = StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32)

    private func makeStub() -> StubPokeCalcService {
        StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.abilityXAndY],
            items: [StubMaster.itemA, StubMaster.itemB])
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func calc(
        attacker: SpeciesDetail = StubMaster.gamma, defender: SpeciesDetail = StubMaster.beta,
        move: String = StubMaster.specialMove.id, attackerNature: String = StubMaster.spaUpNature.id,
        attackerAbility: String? = nil, attackerItem: String? = nil, attackerRanks: RankBlock = RankBlock(),
        attackerStatus: StatusCondition = .none, defenderAbility: String? = nil,
        defenderRanks: RankBlock = RankBlock(), field: FieldState = FieldState(), critical: Bool = false
    ) -> CalcHistoryCalc {
        CalcHistoryCalc(
            format: .single,
            attacker: Individual(
                speciesKey: attacker.key, natureId: attackerNature, sp: sp, abilityId: attackerAbility,
                itemId: attackerItem, ranks: attackerRanks, status: attackerStatus),
            defender: Individual(
                speciesKey: defender.key, natureId: StubMaster.neutralNature.id,
                sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0), abilityId: defenderAbility,
                itemId: StubMaster.itemB.id, ranks: defenderRanks),
            moveId: move, field: field, critical: critical)
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    private func bulkCount(_ stub: StubPokeCalcService) async -> Int { await stub.bulkRequests.count }

    func testRestoresInputsAndCalculatesExactlyOnce() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let before = await bulkCount(stub)
        let field = FieldState(weather: .rain, terrain: .grassy, defenderScreens: Screens(reflect: true))
        let history = calc(
            attackerAbility: StubMaster.ability.id, attackerItem: StubMaster.itemA.id,
            attackerRanks: RankBlock(spa: 2), attackerStatus: .burn, defenderAbility: StubMaster.ability.id,
            defenderRanks: RankBlock(spd: -1), field: field, critical: true)

        await viewModel.loadHistoryCalc(history)

        let after = await bulkCount(stub)
        XCTAssertEqual(after - before, 1, "計算はちょうど1回")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.gamma.key)
        XCTAssertEqual(request.attacker.natureId, StubMaster.spaUpNature.id, "プリセットに丸め直さない")
        XCTAssertEqual(request.attacker.sp, sp)
        XCTAssertEqual(request.attacker.abilityId, StubMaster.ability.id)
        XCTAssertEqual(request.attacker.itemId, StubMaster.itemA.id)
        XCTAssertEqual(request.attacker.ranks, RankBlock(spa: 2))
        XCTAssertEqual(request.attacker.status, .burn)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.defenderAbilityId, StubMaster.ability.id)
        XCTAssertEqual(request.defenderRanks, RankBlock(spd: -1))
        XCTAssertEqual(request.field.weather, .rain)
        XCTAssertEqual(request.field.terrain, .grassy)
        XCTAssertEqual(request.field.defenderScreens, Screens(reflect: true))
        XCTAssertTrue(request.critical)
        XCTAssertEqual(request.itemVariants, [], "防御側の持ち物の比較は使わない(履歴の防御側の持ち物は復元しない)")

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
        XCTAssertNil(viewModel.attackerPreset)
        let selection = try XCTUnwrap(viewModel.attackerBuildSource.teamSelection)
        XCTAssertEqual(selection.teamID, CalcHistory.sourceTeamID)
        XCTAssertEqual(selection.displayName, StubMaster.gamma.nameJa)
        XCTAssertNil(viewModel.favoriteLoadNotice)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertFalse(viewModel.rows.isEmpty)
        XCTAssertTrue(viewModel.isCritical)
        XCTAssertTrue(viewModel.isAttackerBurned)
        XCTAssertEqual(viewModel.weather, .rain)
    }

    func testSameSpeciesOnBothSidesReadsSpeciesOnce() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.speciesRequests.count
        await viewModel.loadHistoryCalc(calc(attacker: StubMaster.beta, defender: StubMaster.beta))
        let count = await stub.speciesRequests.count
        XCTAssertEqual(count - baseline, 1)
    }

    func testRestoreResetsPreviousConditionsAndComparisons() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setCritical(true)
        await viewModel.selectWeather(.sun)
        await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemA.id)
        await viewModel.loadHistoryCalc(calc())
        let request = try await lastRequest(stub)
        XCTAssertFalse(request.critical, "履歴の条件で置き換える(前の条件を残さない)")
        XCTAssertEqual(request.field.weather, .none)
        XCTAssertEqual(request.itemVariants, [])
        XCTAssertTrue(viewModel.comparedDefenderItemIds.isEmpty)
    }

    func testAbilitiesNotInSpeciesAreDropped() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadHistoryCalc(calc(attackerAbility: "stub-ability-gone", defenderAbility: "stub-ability-gone"))
        let request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.abilityId)
        XCTAssertNil(request.defenderAbilityId)
        XCTAssertNil(viewModel.error)
    }

    func testMoveNotInLearnsetIsStillRestored() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        // gamma の learnset は physical・special。変化技は覚えないが、履歴の技は黙って別の技に置き換えない。
        // 計算画面は変化技を計算しない(F-01。ADR-0518 §1・§3)ので、入力は履歴どおりに戻し、要求は送らず結果は出さない。
        let before = await bulkCount(stub)
        await viewModel.loadHistoryCalc(calc(move: StubMaster.statusMove.id))
        XCTAssertEqual(viewModel.moveId, StubMaster.statusMove.id)
        let after = await bulkCount(stub)
        XCTAssertEqual(after, before, "変化技は計算の要求を送らない")
        XCTAssertTrue(viewModel.rows.isEmpty, "古い結果を残さない")
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 失敗(計算の失敗として表示し、入力は書き換えない)

    func testUnknownMoveShowsErrorAndKeepsInputs() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let attackerBefore = viewModel.attackerSpeciesKey
        let moveBefore = viewModel.moveId
        let bulkBefore = await bulkCount(stub)
        await viewModel.loadHistoryCalc(calc(move: "stub-move-gone"))
        XCTAssertNotNil(viewModel.error)
        XCTAssertTrue(viewModel.rows.isEmpty)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertEqual(viewModel.attackerSpeciesKey, attackerBefore)
        XCTAssertEqual(viewModel.moveId, moveBefore)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter, bulkBefore, "復元できないときは計算しない")
    }

    func testUnknownSpeciesShowsErrorAndKeepsInputs() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let attackerBefore = viewModel.attackerSpeciesKey
        let bulkBefore = await bulkCount(stub)
        let history = CalcHistoryCalc(
            format: .single,
            attacker: Individual(speciesKey: "9999-000", natureId: StubMaster.neutralNature.id, sp: sp),
            defender: Individual(speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id, sp: sp),
            moveId: StubMaster.specialMove.id)
        await viewModel.loadHistoryCalc(history)
        XCTAssertNotNil(viewModel.error)
        XCTAssertEqual(viewModel.attackerSpeciesKey, attackerBefore)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter, bulkBefore)
    }

    func testServerRejectionOfRestoredRequestIsShownAsCalcFailure() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await stub.setBulkResponder { _ in
            .failure(PokeCalcError(code: "unknown_move", message: "技がマスタに無い"))
        }
        await viewModel.loadHistoryCalc(calc())
        XCTAssertEqual(viewModel.error, .service(code: "unknown_move", message: "技がマスタに無い"))
        XCTAssertTrue(viewModel.rows.isEmpty)
    }

    // MARK: - 古い応答の破棄

    func testStaleRestoreDoesNotOverwriteNewerOne() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.speciesRequests.count
        let bulkBefore = await bulkCount(stub)
        await stub.setSpeciesMode(.manual)

        let older = Task {
            await viewModel.loadHistoryCalc(self.calc(attacker: StubMaster.gamma, defender: StubMaster.gamma))
        }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let newer = Task {
            await viewModel.loadHistoryCalc(self.calc(attacker: StubMaster.beta, defender: StubMaster.beta))
        }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        let betaDetail = try await stub.lookupSpecies(key: StubMaster.beta.key)
        let gammaDetail = try await stub.lookupSpecies(key: StubMaster.gamma.key)
        await stub.resolveSpecies(at: baseline + 1, with: .success(betaDetail))
        await newer.value
        await stub.resolveSpecies(at: baseline, with: .success(gammaDetail))
        await older.value

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter - bulkBefore, 1, "古い復元は計算しない")
    }
}
