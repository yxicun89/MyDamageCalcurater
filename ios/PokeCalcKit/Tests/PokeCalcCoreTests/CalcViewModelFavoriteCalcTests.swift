import XCTest

@testable import PokeCalcCore

/// F-09(ADR-0524): お気に入りの `calc`(計算の入力)の保存と復元。
/// 保存 = `attackerFavoritePin()` が今の計算入力を `CalcHistoryCalc`(= `CalcRequest` の写像)にする。
/// 復元 = `loadFavoriteCalc(_:)` が `loadHistoryCalc` と同じ規則で入力を戻して計算を1回出す。架空のマスタだけを使う。
@MainActor
final class CalcViewModelFavoriteCalcTests: XCTestCase {
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

    private func favorite(_ calc: CalcHistoryCalc?, id: String = "77", label: String? = "テスト見出し") -> Favorite {
        Favorite(
            id: id, label: label, individual: calc?.attacker
                ?? Individual(speciesKey: "9001-000", natureId: "x", sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)),
            createdAt: Date(timeIntervalSince1970: 1), updatedAt: Date(timeIntervalSince1970: 1), calc: calc)
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    // MARK: - 保存(今の入力 → calc)

    func testPinTargetBuildsCalcFromCurrentInputs() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let target = try viewModel.attackerFavoritePin()
        let calc = try XCTUnwrap(target.calc)
        let request = try await lastRequest(stub)

        XCTAssertEqual(calc.format, .single)
        XCTAssertEqual(calc.attacker, request.attacker, "攻撃側は今の計算要求の個体そのまま")
        XCTAssertEqual(target.individual, calc.attacker, "individual = calc.attacker(Web と同じ)")
        XCTAssertEqual(calc.moveId, request.moveId)
        XCTAssertEqual(calc.defender.speciesKey, request.defenderSpeciesKey)
        XCTAssertEqual(calc.defender.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0), "防御側は無振り")
        XCTAssertEqual(calc.defender.natureId, StubMaster.neutralNature.id, "防御側は無補正の性格")
        XCTAssertEqual(calc.field, FieldState())
        XCTAssertFalse(calc.critical)
        XCTAssertNil(calc.attacker.teraType)
    }

    func testPinTargetLabelIsAttackerArrowDefenderWithMove() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let target = try viewModel.attackerFavoritePin()
        let attackerName = try XCTUnwrap(viewModel.attackerSpecies?.nameJa)
        let defenderName = try XCTUnwrap(viewModel.defenderSpecies?.nameJa)
        let moveName = try XCTUnwrap(viewModel.selectedMove?.nameJa)
        XCTAssertEqual(target.label, "\(attackerName)→\(defenderName)(\(moveName))")
        XCTAssertEqual(FavoriteCalcLabel.text(attacker: "A", defender: "B", move: "C"), "A→B(C)")
    }

    func testPinTargetCarriesConditionsAndAttackerBuild() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setCritical(true)
        await viewModel.selectWeather(.rain)
        await viewModel.selectTerrain(.grassy)
        await viewModel.setDefenderScreen(.reflect, isOn: true)
        await viewModel.setAttackerRank(2)
        await viewModel.setAttackerBurned(true)
        await viewModel.setDefenderRank(-1)
        let calc = try XCTUnwrap(try viewModel.attackerFavoritePin().calc)
        XCTAssertTrue(calc.critical)
        XCTAssertEqual(calc.field.weather, .rain)
        XCTAssertEqual(calc.field.terrain, .grassy)
        XCTAssertEqual(calc.field.defenderScreens, Screens(reflect: true))
        XCTAssertEqual(calc.field.attackerScreens, Screens())
        XCTAssertEqual(calc.attacker.status, .burn)
        XCTAssertNotEqual(calc.attacker.ranks, RankBlock())
        XCTAssertNotEqual(calc.defender.ranks, RankBlock())
    }

    func testPinTargetThrowsWhenAttackerCannotBeBuilt() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerSPText("99", for: .atk)
        XCTAssertThrowsError(try viewModel.attackerFavoritePin(), "SP 不正のときは従来どおり追加できない")
    }

    func testSaveThenRestoreOnFreshViewModelReproducesRequest() async throws {
        let stub = makeStub()
        let first = await loadedViewModel(stub)
        await first.selectDefender(speciesKey: StubMaster.gamma.key)
        await first.selectWeather(.sun)
        await first.setCritical(true)
        await first.setDefenderScreen(.lightScreen, isOn: true)
        await first.setAttackerRank(1)
        let original = try await lastRequest(stub)
        let target = try first.attackerFavoritePin()

        let stub2 = makeStub()
        let second = await loadedViewModel(stub2)
        await second.loadFavoriteCalc(favorite(target.calc, label: target.label))
        let restored = try await lastRequest(stub2)

        XCTAssertEqual(restored.attacker.speciesKey, original.attacker.speciesKey)
        XCTAssertEqual(restored.attacker.natureId, original.attacker.natureId)
        XCTAssertEqual(restored.attacker.sp, original.attacker.sp)
        XCTAssertEqual(restored.attacker.ranks, original.attacker.ranks)
        XCTAssertEqual(restored.attacker.status, original.attacker.status)
        XCTAssertEqual(restored.moveId, original.moveId)
        XCTAssertEqual(restored.defenderSpeciesKey, original.defenderSpeciesKey)
        XCTAssertEqual(restored.field, original.field)
        XCTAssertEqual(restored.critical, original.critical)
        XCTAssertEqual(restored.defenderRanks, original.defenderRanks)
    }

    // MARK: - 復元

    func testLoadFavoriteCalcRestoresAndCalculatesExactlyOnceWithFavoriteSource() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let before = await stub.bulkRequests.count
        let calc = CalcHistoryCalc(
            format: .single,
            attacker: Individual(
                speciesKey: StubMaster.gamma.key, natureId: StubMaster.spaUpNature.id,
                sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 32, spd: 2, spe: 32)),
            defender: Individual(
                speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id,
                sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)),
            moveId: StubMaster.specialMove.id, field: FieldState(weather: .rain), critical: true)
        await viewModel.loadFavoriteCalc(favorite(calc))
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after - before, 1, "計算はちょうど1回")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.gamma.key)
        XCTAssertEqual(request.attacker.natureId, StubMaster.spaUpNature.id)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.field.weather, .rain)
        XCTAssertTrue(request.critical)
        let selection = try XCTUnwrap(viewModel.attackerBuildSource.teamSelection)
        XCTAssertEqual(selection.teamID, FavoriteLoad.sourceTeamID)
        XCTAssertEqual(selection.memberID, "77")
        XCTAssertNil(viewModel.error)
    }

    func testLoadFavoriteWithoutCalcDoesNothing() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let before = await stub.bulkRequests.count
        let attackerBefore = viewModel.attackerSpeciesKey
        await viewModel.loadFavoriteCalc(favorite(nil))
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before, "calc の無い旧お気に入りは従来の読み込み導線(loadFavorite)で扱う")
        XCTAssertEqual(viewModel.attackerSpeciesKey, attackerBefore)
    }

    func testUnknownMoveIsShownAsCalcFailureAndKeepsInputs() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let attackerBefore = viewModel.attackerSpeciesKey
        let moveBefore = viewModel.moveId
        let before = await stub.bulkRequests.count
        let calc = CalcHistoryCalc(
            format: .single,
            attacker: Individual(speciesKey: StubMaster.gamma.key, natureId: StubMaster.neutralNature.id,
                                 sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)),
            defender: Individual(speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id,
                                 sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)),
            moveId: "stub-move-gone")
        await viewModel.loadFavoriteCalc(favorite(calc))
        XCTAssertNotNil(viewModel.error)
        XCTAssertEqual(viewModel.attackerSpeciesKey, attackerBefore)
        XCTAssertEqual(viewModel.moveId, moveBefore)
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before)
    }

    func testServerRejectionIsShownAsCalcFailure() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await stub.setBulkResponder { _ in .failure(PokeCalcError(code: "unknown_move", message: "技がマスタに無い")) }
        let calc = try XCTUnwrap(try viewModel.attackerFavoritePin().calc)
        await viewModel.loadFavoriteCalc(favorite(calc))
        XCTAssertEqual(viewModel.error, .service(code: "unknown_move", message: "技がマスタに無い"))
    }
}
