import XCTest

@testable import PokeCalcCore

/// I-ios-21(ADR-0542 §2・ADR-0501「素早さ画面から連続技へ」AC-h2): 素早さ画面の「この相手で連続技を計算」で、
/// 計算画面の攻撃側 = 自分・防御側 = 相手の種族を入れる `CalcViewModel.openFromSpeed(_:)`。
///
/// 上書きの規則(引き継ぐ項目と触らない項目):
/// - 攻撃側・防御側の種族だけを入れる。種族が今と違う側は、ピッカーで選んだのと同じ規則(`selectAttacker` / `selectDefender`)で変わる
///   (技の選び直し・持ち物のメガの固定・防御側の特性を「指定なし」に戻す、は既存の規則のまま)。
/// - 種族が今と同じ側は何も変えない(持ち物・特性・技・構築の個体をそのまま残す)。
/// - 場・急所・やけど・攻撃側/防御側のランク・攻撃側の SP と性格補正(攻撃・特攻)・比較する持ち物は触らない。
/// - 計算は変わる側があるときだけ **ちょうど 1 回**。両側とも同じなら計算しない。
/// - データに無い種族は、その側を変えない(黙って飛ばす。エラーにしない。Web と同じ)。先頭ページの外の種族は詳細を引いて入れる。
/// - 画面を開いた直後(初期の読み込みの途中)に届いても、読み込みが終わってから入れる(読み込みに上書きされない)。
@MainActor
final class CalcViewModelSpeedHandoffTests: XCTestCase {

    private func makeStub() -> StubPokeCalcService {
        StubMaster.makeService(species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly])
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func handoff(attacker: String, defender: String) -> SpeedComboHandoff {
        SpeedComboHandoff(
            attackerPokemonID: attacker, defenderPokemonID: defender, attackerSpeed: SpeedInvestment(sp: 32, nature: .plus),
            attackerRank: 1, attackerScarf: true)
    }

    func testReplacesBothSidesAndCalculatesExactlyOnce() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key, "前提: 既定の攻撃側")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key, "前提: 既定の防御側")
        let before = await stub.bulkRequests.count

        await viewModel.openFromSpeed(handoff(attacker: StubMaster.gamma.key, defender: StubMaster.alpha.key))

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key, "攻撃側 = 素早さ画面の自分")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.alpha.key, "防御側 = 素早さ画面の相手")
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, before + 1, "両側を入れても計算は 1 回")
        let last = try XCTUnwrap(requests.last)
        XCTAssertEqual(last.attacker.speciesKey, StubMaster.gamma.key)
        XCTAssertEqual(last.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.rows.isEmpty, "結果の行が出る(連続技の節の文脈になる)")
    }

    /// 既存の計算の入力を壊さない: 種族以外は触らない。
    func testKeepsConditionsRanksItemAndComparedItems() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setCritical(true)
        await viewModel.selectWeather(.rain)
        await viewModel.selectTerrain(.grassy)
        await viewModel.setDefenderScreen(.reflect, isOn: true)
        await viewModel.setAttackerBurned(true)
        await viewModel.setAttackerRank(2)
        await viewModel.setDefenderRank(-1)
        await viewModel.selectAttackerItem(id: StubMaster.itemA.id)
        await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemB.id)
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemA.id, "前提: 攻撃側の持ち物")
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [StubMaster.itemB.id], "前提: 比較する持ち物")
        let ranksBefore = viewModel.attackerRanks
        let defenderRanksBefore = viewModel.defenderRanks

        await viewModel.openFromSpeed(handoff(attacker: StubMaster.gamma.key, defender: StubMaster.alpha.key))

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key, "種族は入る")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertTrue(viewModel.isCritical)
        XCTAssertEqual(viewModel.weather, .rain)
        XCTAssertEqual(viewModel.terrain, .grassy)
        XCTAssertEqual(viewModel.defenderScreens, Screens(reflect: true))
        XCTAssertTrue(viewModel.isAttackerBurned)
        XCTAssertEqual(viewModel.attackerRanks, ranksBefore, "攻撃側のランクは触らない(素早さのランクは連続技の節の入力)")
        XCTAssertEqual(viewModel.defenderRanks, defenderRanksBefore)
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemA.id, "メガでない持ち物は種族を変えても残る(selectAttacker と同じ)")
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [StubMaster.itemB.id])
        let requests = await stub.bulkRequests
        let last = try XCTUnwrap(requests.last)
        XCTAssertTrue(last.critical)
        XCTAssertEqual(last.field.weather, .rain)
    }

    /// 種族が今と同じ側は何も変えない(技・特性を選び直さない)。
    func testSameAttackerKeepsMoveAndAbility() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttackerAbility(id: StubMaster.ability.id)
        let moveBefore = viewModel.moveId
        let abilityBefore = viewModel.attackerAbilityId
        XCTAssertEqual(abilityBefore, StubMaster.ability.id, "前提: 特性を選んである")
        let speciesRequestsBefore = await stub.speciesRequests.count
        let before = await stub.bulkRequests.count

        await viewModel.openFromSpeed(handoff(attacker: StubMaster.alpha.key, defender: StubMaster.gamma.key))

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.moveId, moveBefore, "攻撃側が同じなら技を選び直さない")
        XCTAssertEqual(viewModel.attackerAbilityId, abilityBefore, "攻撃側が同じなら特性を戻さない")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key)
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before + 1)
        let speciesRequestsAfter = await stub.speciesRequests
        XCTAssertFalse(
            speciesRequestsAfter.dropFirst(speciesRequestsBefore).contains(StubMaster.alpha.key),
            "変えない側の詳細は読み直さない")
    }

    func testSameDefenderKeepsDefenderAbility() async throws {
        let stub = StubMaster.makeService(species: [StubMaster.alpha, StubMaster.abilityXAndY, StubMaster.gamma])
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.abilityXAndY.key, "前提: 既定の防御側")
        await viewModel.loadDefenderAbilityOptions()
        await viewModel.selectDefenderAbility(id: StubMaster.abilityY.id)
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityY.id, "前提: 防御側の特性を選んである")

        await viewModel.openFromSpeed(handoff(attacker: StubMaster.gamma.key, defender: StubMaster.abilityXAndY.key))

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityY.id, "防御側が同じなら特性を「指定なし」に戻さない")
    }

    func testBothSameDoesNotCalculate() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let before = await stub.bulkRequests.count
        await viewModel.openFromSpeed(handoff(attacker: StubMaster.alpha.key, defender: StubMaster.beta.key))
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before, "変わる側が無ければ計算し直さない")
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
    }

    /// データに無い種族の側は変えない(エラーにしない。計算画面の他の入力は壊さない)。
    func testUnknownSpeciesLeavesThatSideAsIs() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.openFromSpeed(handoff(attacker: "9999-000", defender: StubMaster.gamma.key))
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key, "引けない自分は入れない")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key, "引ける相手は入れる")
        XCTAssertNil(viewModel.error, "引けない種族を画面のエラーにしない")
        XCTAssertFalse(viewModel.rows.isEmpty)

        await viewModel.openFromSpeed(handoff(attacker: "9999-000", defender: "9998-000"))
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key)
        XCTAssertNil(viewModel.error)
    }

    /// 先頭ページ(空の検索)の外の種族も、詳細を引いて入れる(素早さ画面は全ポケモンから選べる)。
    func testSpeciesOutsideTheFirstPageIsResolved() async throws {
        let stub = StubBulkMaster.makeService()
        await stub.setMoveLookupMode(.immediate)  // 先頭ページの外の技は個別に引ける設定(既定の .notFound だと selectAttacker でも同じ誤りになる)
        let viewModel = await loadedViewModel(stub)
        await viewModel.openFromSpeed(
            handoff(attacker: StubBulkMaster.hiddenSpecies.key, defender: StubBulkMaster.mixedSpecies.key))
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.attackerSpecies?.nameJa, StubBulkMaster.hiddenSpecies.nameJa, "カードに名前が出る(辞書に入る)")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubBulkMaster.mixedSpecies.key)
        XCTAssertEqual(viewModel.defenderSpecies?.nameJa, StubBulkMaster.mixedSpecies.nameJa)
        XCTAssertNil(viewModel.error)
    }

    /// 計算画面を初めて開くのと同時に届いても、初期の読み込み(既定の攻撃側・防御側)に上書きされない。
    func testArrivingDuringTheInitialLoadIsAppliedAfterIt() async throws {
        let stub = makeStub()
        let viewModel = CalcViewModel(service: stub)
        let sent = handoff(attacker: StubMaster.gamma.key, defender: StubMaster.alpha.key)
        // 画面の `.task`(初期の読み込み)と引き継ぎの `.task` が同時に走るのと同じ形。
        let initial = Task { await viewModel.load() }
        let opened = Task { await viewModel.openFromSpeed(sent) }
        await initial.value
        await opened.value
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertFalse(viewModel.natureOptions.isEmpty, "初期の読み込み(性格の一覧など)は終わっている")
        XCTAssertNil(viewModel.error)
        let requests = await stub.bulkRequests
        let last = try XCTUnwrap(requests.last)
        XCTAssertEqual(last.attacker.speciesKey, StubMaster.gamma.key, "最後の計算は引き継いだ値")
    }

    func testBeforeAnyLoadItLoadsFirst() async throws {
        let stub = makeStub()
        let viewModel = CalcViewModel(service: stub)
        await viewModel.openFromSpeed(handoff(attacker: StubMaster.gamma.key, defender: StubMaster.alpha.key))
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertFalse(viewModel.natureOptions.isEmpty)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }

    /// 取り消された引き継ぎは、片側だけ書き換えない・計算しない(タブを離れて画面の .task が取り消されたとき。ADR-0542 §2)。
    func testCancelledHandoffChangesNeitherSideAndDoesNotCalculate() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let before = await stub.bulkRequests.count
        let task = Task { await viewModel.openFromSpeed(handoff(attacker: StubMaster.gamma.key, defender: StubMaster.alpha.key)) }
        task.cancel()
        await task.value
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before)
    }

    /// 取り消されても、続けて呼び直せば最後まで入る(取り消しが状態を壊さない)。
    func testHandoffAfterACancelledOneStillApplies() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let sent = handoff(attacker: StubMaster.gamma.key, defender: StubMaster.alpha.key)
        let task = Task { await viewModel.openFromSpeed(sent) }
        task.cancel()
        await task.value
        await viewModel.openFromSpeed(sent)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertNil(viewModel.error)
    }
}
