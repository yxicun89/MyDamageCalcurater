import XCTest

@testable import PokeCalcCore

/// 計算画面の「詳細」: 急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク・攻撃側の特性
/// (issue #274。ADR-0501「issue #274」)。
///
/// 既存の `CalcViewModelTests` / `CalcViewModelTeamIndividualTests` は変えない。ここでは
/// (1) 既定のままなら要求がこれまでと同じ、(2) 各条件が要求の決まった場所にだけ写る、
/// (3) 値が変わる操作ごとに計算がちょうど1回(変わらない操作は0回)、(4) 種族・技・入れ替え・構築での
/// 引き継ぎとリセットの規則、を固定する。計算結果の数値には依存しない(stub はエコー応答)。
@MainActor
final class CalcViewModelConditionsTests: XCTestCase {

    /// 契約 `RankBlock` の範囲(openapi: minimum -6 / maximum 6)。
    private let maxRank = 6

    // MARK: - 補助

    private func loadedViewModel(
        _ stub: StubPokeCalcService, store: StubTeamStore? = nil
    ) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub, teamStore: store)
        await viewModel.load()
        return viewModel
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    private func bulkCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.bulkRequests.count
    }

    /// `operation` の前後で calcBulk が何回呼ばれたか。
    private func calcCount(
        _ stub: StubPokeCalcService, during operation: () async -> Void
    ) async -> Int {
        let before = await bulkCount(stub)
        await operation()
        let after = await bulkCount(stub)
        return after - before
    }

    // MARK: - 1. 既定は今の要求と同じ

    func testDefaultsReproduceTheRequestSentBeforeThisFeature() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        XCTAssertFalse(viewModel.isCritical)
        XCTAssertFalse(viewModel.isAttackerBurned)
        XCTAssertEqual(viewModel.weather, .none)
        XCTAssertEqual(viewModel.terrain, .none)
        XCTAssertEqual(viewModel.defenderScreens, Screens())
        XCTAssertEqual(viewModel.attackerRanks, RankBlock())
        XCTAssertNil(viewModel.attackerAbilityId, "プリセット経路の特性はこれまでどおり送らない")
        XCTAssertEqual(viewModel.attackerAbilityOptions, StubMaster.alpha.abilities,
                       "特性の選択肢は攻撃側の species(key:) の abilities")
        // 既定の技(alphaOnlyMove)は物理なので A。
        XCTAssertEqual(viewModel.attackerRankStat, .atk)
        XCTAssertEqual(viewModel.attackerRank, 0)
        XCTAssertEqual(viewModel.attackerRankText, "A ±0")

        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, 1, "起動時の計算は1回のまま")
        let request = try XCTUnwrap(requests.first)
        XCTAssertFalse(request.critical)
        XCTAssertEqual(request.field, FieldState(), "既定の場は「何もない場」(要求では省略と同じ)")
        XCTAssertEqual(request.attacker.status, .none)
        XCTAssertEqual(request.attacker.ranks, RankBlock())
        XCTAssertNil(request.attacker.abilityId)
    }

    // MARK: - 2. 各条件が要求の決まった場所にだけ写る(値が変わるたびに計算1回)

    func testCriticalMapsToOptionsCriticalWithOneCalcPerChange() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        var calls = await calcCount(stub) { await viewModel.setCritical(true) }
        XCTAssertEqual(calls, 1)
        XCTAssertTrue(viewModel.isCritical)
        var request = try await lastRequest(stub)
        XCTAssertTrue(request.critical)
        XCTAssertEqual(request.field, FieldState(), "急所は場を変えない")
        XCTAssertEqual(request.attacker.status, .none)

        calls = await calcCount(stub) { await viewModel.setCritical(true) }
        XCTAssertEqual(calls, 0, "値が変わらない操作は計算しない")

        calls = await calcCount(stub) { await viewModel.setCritical(false) }
        XCTAssertEqual(calls, 1)
        request = try await lastRequest(stub)
        XCTAssertFalse(request.critical)
    }

    func testBurnMapsToAttackerStatusWithOneCalcPerChange() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        var calls = await calcCount(stub) { await viewModel.setAttackerBurned(true) }
        XCTAssertEqual(calls, 1)
        XCTAssertTrue(viewModel.isAttackerBurned)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.status, .burn)
        XCTAssertFalse(request.critical)
        XCTAssertEqual(request.field, FieldState())

        calls = await calcCount(stub) { await viewModel.setAttackerBurned(true) }
        XCTAssertEqual(calls, 0)

        calls = await calcCount(stub) { await viewModel.setAttackerBurned(false) }
        XCTAssertEqual(calls, 1)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.status, .none)
    }

    func testEveryWeatherMapsToFieldWeatherOnly() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        // 既定の none から始め、none 以外 → 最後に none へ戻す(全5値を1回ずつ要求に載せる)。
        let sequence = Weather.allCases.filter { $0 != .none } + [.none]
        for weather in sequence {
            let calls = await calcCount(stub) { await viewModel.selectWeather(weather) }
            XCTAssertEqual(calls, 1, "\(weather)")
            XCTAssertEqual(viewModel.weather, weather)
            let request = try await lastRequest(stub)
            XCTAssertEqual(request.field, FieldState(weather: weather), "\(weather) は天候だけを変える")
            XCTAssertFalse(request.critical)
        }
        let calls = await calcCount(stub) { await viewModel.selectWeather(.none) }
        XCTAssertEqual(calls, 0, "選択中の天候を押し直しても計算しない")
    }

    func testEveryTerrainMapsToFieldTerrainOnly() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        let sequence = Terrain.allCases.filter { $0 != .none } + [.none]
        for terrain in sequence {
            let calls = await calcCount(stub) { await viewModel.selectTerrain(terrain) }
            XCTAssertEqual(calls, 1, "\(terrain)")
            XCTAssertEqual(viewModel.terrain, terrain)
            let request = try await lastRequest(stub)
            XCTAssertEqual(request.field, FieldState(terrain: terrain), "\(terrain) はフィールドだけを変える")
        }
        let calls = await calcCount(stub) { await viewModel.selectTerrain(.none) }
        XCTAssertEqual(calls, 0)
    }

    func testDefenderScreensMapToDefenderScreensOnlyAndCombine() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        var expected = Screens()
        for kind in ScreenKind.allCases {
            let calls = await calcCount(stub) { await viewModel.setDefenderScreen(kind, isOn: true) }
            XCTAssertEqual(calls, 1, "\(kind)")
            expected = expected.setting(kind, to: true)
            XCTAssertEqual(viewModel.defenderScreens, expected)
            let request = try await lastRequest(stub)
            XCTAssertEqual(request.field.defenderScreens, expected, "壁は重ねて張れる(\(kind))")
            XCTAssertEqual(request.field.attackerScreens, Screens(), "攻撃側の壁は画面から変えない")
            XCTAssertEqual(request.field.weather, .none)
            XCTAssertEqual(request.field.terrain, .none)
        }
        XCTAssertEqual(expected, Screens(reflect: true, lightScreen: true, auroraVeil: true))

        var calls = await calcCount(stub) { await viewModel.setDefenderScreen(.reflect, isOn: true) }
        XCTAssertEqual(calls, 0, "張ってある壁を張っても計算しない")

        calls = await calcCount(stub) { await viewModel.setDefenderScreen(.lightScreen, isOn: false) }
        XCTAssertEqual(calls, 1)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.field.defenderScreens, Screens(reflect: true, lightScreen: false, auroraVeil: true))
    }

    // MARK: - 3. ランク(境界 ±6 と技の分類)

    func testRankClampsToContractRangeAndIgnoresNoOpChanges() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        var calls = await calcCount(stub) { await viewModel.setAttackerRank(1) }
        XCTAssertEqual(calls, 1)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 1), "物理技なので atk だけ。他は 0")
        XCTAssertEqual(viewModel.attackerRankText, "A +1")

        calls = await calcCount(stub) { await viewModel.setAttackerRank(maxRank) }
        XCTAssertEqual(calls, 1)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks.atk, maxRank)
        XCTAssertEqual(viewModel.attackerRankText, "A +6")

        calls = await calcCount(stub) { await viewModel.setAttackerRank(maxRank + 1) }
        XCTAssertEqual(calls, 0, "+6 を超える値は +6 に丸め、変化が無いので計算しない")
        XCTAssertEqual(viewModel.attackerRank, maxRank)

        calls = await calcCount(stub) { await viewModel.setAttackerRank(-maxRank) }
        XCTAssertEqual(calls, 1)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: -maxRank))
        XCTAssertEqual(viewModel.attackerRankText, "A -6")

        calls = await calcCount(stub) { await viewModel.setAttackerRank(-100) }
        XCTAssertEqual(calls, 0, "-6 を下回る値は -6 に丸める")

        calls = await calcCount(stub) { await viewModel.setAttackerRank(0) }
        XCTAssertEqual(calls, 1)
        calls = await calcCount(stub) { await viewModel.setAttackerRank(100) }
        XCTAssertEqual(calls, 1, "範囲外でも丸めた値が変われば計算1回")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: maxRank))
    }

    func testRankStepperFollowsMoveCategoryAndKeepsEachStatsRank() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.selectedMove?.category, .physical)

        await viewModel.setAttackerRank(2)
        XCTAssertEqual(viewModel.attackerRanks, RankBlock(atk: 2))

        // 特殊技に変えると、ステッパーは C(spa)を編集する。A のランクは消さずに持ち続ける。
        let calls = await calcCount(stub) { await viewModel.selectMove(id: StubMaster.specialMove.id) }
        XCTAssertEqual(calls, 1, "技の変更は従来どおり計算1回(ランクの付け替えで余計に計算しない)")
        XCTAssertEqual(viewModel.attackerRankStat, .spa)
        XCTAssertEqual(viewModel.attackerRank, 0)
        XCTAssertEqual(viewModel.attackerRankText, "C ±0")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 2, spa: 0),
                       "atk/spa は両方そのまま送る(engine は技の分類の関連ステータスだけを使う)")

        await viewModel.setAttackerRank(-1)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 2, spa: -1))
        XCTAssertEqual(viewModel.attackerRankText, "C -1")

        // 物理技に戻すと A の +2 がまた見える。
        await viewModel.selectMove(id: StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(viewModel.attackerRankStat, .atk)
        XCTAssertEqual(viewModel.attackerRank, 2)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 2, spa: -1))
    }

    func testStatusMoveEditsAttackRank() async {
        // 変化技の関連ステータスは atk(`AttackerPreset.relevantStat(for:)`)。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectMove(id: StubMaster.statusMove.id)
        XCTAssertEqual(viewModel.attackerRankStat, .atk)
        await viewModel.setAttackerRank(1)
        XCTAssertEqual(viewModel.attackerRanks, RankBlock(atk: 1))
    }

    // MARK: - 4. 特性

    func testAbilityPickerAcceptsOnlyUnspecifiedOrSpeciesAbilities() async throws {
        let stub = StubMaster.makeService(species: [StubMaster.abilityXAndY, StubMaster.beta])
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.attackerAbilityOptions, [StubMaster.abilityX, StubMaster.abilityY])
        XCTAssertNil(viewModel.attackerAbilityId)

        var calls = await calcCount(stub) { await viewModel.selectAttackerAbility(id: StubMaster.abilityY.id) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.attackerAbilityId, StubMaster.abilityY.id)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.abilityId, StubMaster.abilityY.id)

        calls = await calcCount(stub) { await viewModel.selectAttackerAbility(id: StubMaster.ability.id) }
        XCTAssertEqual(calls, 0, "攻撃側が持たない特性は無視する")
        XCTAssertEqual(viewModel.attackerAbilityId, StubMaster.abilityY.id)

        calls = await calcCount(stub) { await viewModel.selectAttackerAbility(id: StubMaster.abilityY.id) }
        XCTAssertEqual(calls, 0, "選択中の特性を選び直しても計算しない")

        calls = await calcCount(stub) { await viewModel.selectAttackerAbility(id: nil) }
        XCTAssertEqual(calls, 1)
        XCTAssertNil(viewModel.attackerAbilityId)
        request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.abilityId, "「指定なし」は送らない")
    }

    func testAttackerSpeciesChangeKeepsAbilityOnlyWhenNewSpeciesHasIt() async throws {
        let stub = StubMaster.makeService(
            species: [StubMaster.abilityXAndY, StubMaster.beta, StubMaster.abilityYOnly, StubMaster.abilityXOnly]
        )
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttackerAbility(id: StubMaster.abilityY.id)

        // 新しい攻撃側も特性Yを持つ → そのまま。
        var calls = await calcCount(stub) { await viewModel.selectAttacker(speciesKey: StubMaster.abilityYOnly.key) }
        XCTAssertEqual(calls, 1, "種族の変更は従来どおり計算1回")
        XCTAssertEqual(viewModel.attackerAbilityOptions, [StubMaster.abilityY])
        XCTAssertEqual(viewModel.attackerAbilityId, StubMaster.abilityY.id)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.abilityId, StubMaster.abilityY.id)

        // 新しい攻撃側が特性Yを持たない → 指定なしに戻す(旧種族の特性を送らない)。
        calls = await calcCount(stub) { await viewModel.selectAttacker(speciesKey: StubMaster.abilityXOnly.key) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.attackerAbilityOptions, [StubMaster.abilityX])
        XCTAssertNil(viewModel.attackerAbilityId)
        request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.abilityId)
    }

    // MARK: - 5. 引き継ぎ(条件は画面の状態。種族・技・入れ替え・プリセット・持ち物で消えない)

    func testSwapKeepsConditionsAndDropsAbilityTheNewAttackerLacks() async throws {
        let stub = StubMaster.makeService(species: [StubMaster.abilityXOnly, StubMaster.abilityYOnly])
        let viewModel = await loadedViewModel(stub)
        await viewModel.setCritical(true)
        await viewModel.setAttackerBurned(true)
        await viewModel.selectWeather(.rain)
        await viewModel.selectTerrain(.grassy)
        await viewModel.setDefenderScreen(.reflect, isOn: true)
        await viewModel.setAttackerRank(3)
        await viewModel.selectAttackerAbility(id: StubMaster.abilityX.id)

        let calls = await calcCount(stub) { await viewModel.swapSides() }
        XCTAssertEqual(calls, 1, "入れ替えは従来どおり計算1回")
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.abilityYOnly.key)
        XCTAssertEqual(viewModel.attackerAbilityOptions, [StubMaster.abilityY])
        XCTAssertNil(viewModel.attackerAbilityId, "入れ替え後の攻撃側が持たない特性は外す")

        let request = try await lastRequest(stub)
        XCTAssertTrue(request.critical)
        XCTAssertEqual(request.attacker.status, .burn)
        XCTAssertEqual(request.field, FieldState(weather: .rain, terrain: .grassy, defenderScreens: Screens(reflect: true)))
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 3), "ランク・やけども入れ替えで消さない(規則6と同じ扱い)")
        XCTAssertNil(request.attacker.abilityId)
    }

    func testConditionsPersistAcrossDefenderPresetItemAndComparisonChanges() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setCritical(true)
        await viewModel.setAttackerBurned(true)
        await viewModel.selectWeather(.sun)
        await viewModel.selectTerrain(.electric)
        await viewModel.setDefenderScreen(.auroraVeil, isOn: true)
        await viewModel.setAttackerRank(-2)
        await viewModel.selectAttackerAbility(id: StubMaster.ability.id)
        let expectedField = FieldState(weather: .sun, terrain: .electric, defenderScreens: Screens(auroraVeil: true))

        let operations: [(String, () async -> Void)] = [
            ("防御側", { await viewModel.selectDefender(speciesKey: StubMaster.gamma.key) }),
            ("プリセット", { await viewModel.selectAttackerPreset(.aFull) }),
            ("持ち物", { await viewModel.selectAttackerItem(id: StubMaster.itemA.id) }),
            ("比較", { await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemB.id) }),
        ]
        for (name, operation) in operations {
            let calls = await calcCount(stub) { await operation() }
            XCTAssertEqual(calls, 1, name)
            let request = try await lastRequest(stub)
            XCTAssertTrue(request.critical, name)
            XCTAssertEqual(request.attacker.status, .burn, name)
            XCTAssertEqual(request.field, expectedField, name)
            XCTAssertEqual(request.attacker.ranks, RankBlock(atk: -2), name)
            XCTAssertEqual(request.attacker.abilityId, StubMaster.ability.id,
                           "プリセット同士の切り替えでは、利用者が選んだ特性を残す(\(name))")
        }
    }

    // MARK: - 6. 構築から呼んだ個体

    func testTeamIndividualKeepsSavedAbilityAndScreenConditionsApply() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.setAttackerBurned(true)
        await viewModel.setAttackerRank(1) // 既定の物理技なので atk
        await viewModel.selectWeather(.snow)

        let calls = await calcCount(stub) {
            await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id)
        }
        XCTAssertEqual(calls, 1, "構築からの呼び出しは従来どおり計算1回")
        XCTAssertEqual(viewModel.attackerAbilityId, StubTeams.namedMember.abilityId, "保存された特性を選択状態にする")
        XCTAssertEqual(viewModel.attackerAbilityOptions, StubMaster.alpha.abilities)
        // 個体の技は特殊技なので、ステッパーは C を指す。
        XCTAssertEqual(viewModel.attackerRankStat, .spa)

        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP, "個体の SP はそのまま")
        XCTAssertEqual(request.attacker.abilityId, StubTeams.namedMember.abilityId)
        XCTAssertEqual(request.attacker.status, .burn, "構築は状態異常を持たないので、画面のやけどを使う")
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 1), "構築はランクを持たないので、画面のランクを使う")
        XCTAssertEqual(request.field, FieldState(weather: .snow))

        // 利用者が特性を変えると上書きする(構築の個体の選択は外れない)。
        await viewModel.selectAttackerAbility(id: nil)
        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection)
        request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.abilityId)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)

        // 同じ個体を呼び直すと保存された特性に戻る。
        await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.abilityId, StubTeams.namedMember.abilityId)
    }

    func testPresetAfterTeamDropsTeamAbilityButKeepsOtherConditions() async throws {
        // 既存の `testSelectingPresetAfterTeamClearsTeamSelection`(特性 nil)と同じ規則を、条件ありで確かめる。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.setCritical(true)
        await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id)
        await viewModel.selectAttackerPreset(.aMax)

        XCTAssertNil(viewModel.attackerAbilityId, "構築から来た特性はプリセットに戻すと外す")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.abilityId)
        XCTAssertTrue(request.critical)
    }

    // MARK: - 7. 世代(古い計算は最新の条件変更に追い越される)

    func testConditionChangeCancelsThePreviousInFlightCalcAndAccumulates() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let baseline = await bulkCount(stub)
        await stub.setBulkMode(.manual)

        let older = viewModel.scheduleLatest { await $0.setCritical(true) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.selectWeather(.rain) }
        try await stub.waitForBulkCancellation(at: baseline)
        try await stub.waitForBulkRequests(count: baseline + 2)
        await stub.resolveBulkWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        XCTAssertTrue(older.isCancelled)
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, baseline + 2, "条件1つの変更につき1回")
        let last = try XCTUnwrap(requests.last)
        XCTAssertTrue(last.critical, "先の変更(急所)は状態として残り、次の要求に載る")
        XCTAssertEqual(last.field.weather, .rain)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }
}
