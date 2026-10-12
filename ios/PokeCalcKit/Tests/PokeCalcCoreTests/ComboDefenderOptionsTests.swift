import XCTest

@testable import PokeCalcCore

/// 連続技の「相手の振り」= 計算画面の一括計算の行(AC-i5。UC-5: 別の入力欄を作らない)。
final class ComboDefenderOptionsTests: XCTestCase {

    private let hbSP = StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0)
    private let itemA = Item(id: "stub-item-a", nameJa: "テストどうぐA")

    private func row(
        preset: DefenderPreset, label: String, itemId: String? = nil, natureId: String?, nature: NatureModifier = NatureModifier(),
        abilityId: String? = nil
    ) -> BulkCalcRow {
        BulkCalcRow(
            preset: preset, presetLabel: label, itemId: itemId,
            defender: BulkDefender(sp: hbSP, nature: nature, natureId: natureId, stats: StatBlock(hp: 1, atk: 1, def: 1, spa: 1, spd: 1, spe: 1)),
            result: ComboStub.calcResult(), abilityId: abilityId, abilityIds: abilityId.map { [$0] } ?? [])
    }

    func testEachRowBecomesAnIndividualWithTheRowsValues() {
        let rows = [
            row(preset: .none, label: "テスト無振り", natureId: ComboStub.neutral.id),
            row(preset: .hbFull, label: "テストHB特化", itemId: itemA.id, natureId: "test-nature-def-up",
                nature: NatureModifier(plus: .def, minus: .atk)),
        ]
        let options = ComboDefenderOptions.make(
            rows: rows, rowIDs: ["none@-", "hb_full@stub-item-a"], defenderSpeciesKey: "9002-000",
            defenderRanks: RankBlock(def: 1), items: [itemA], abilityNames: [:], natures: ComboStub.natures)
        XCTAssertEqual(options.map(\.key), ["none@-", "hb_full@stub-item-a"], "キーは結果の行の ID と同じ")
        XCTAssertEqual(options.map(\.label), ["テスト無振り", "テストHB特化 / テストどうぐA"], "持ち物があれば名前を添える")
        let second = options[1].individual
        XCTAssertEqual(second.speciesKey, "9002-000")
        XCTAssertEqual(second.natureId, "test-nature-def-up", "行の natureId をそのまま使う")
        XCTAssertEqual(second.sp, hbSP)
        XCTAssertEqual(second.itemId, itemA.id)
        XCTAssertEqual(second.ranks, RankBlock(def: 1), "防御側のランクは計算画面の値")
        XCTAssertNil(options[0].individual.itemId)
    }

    func testMissingNatureIdIsResolvedFromTheModifierOrTheRowIsSkipped() {
        let rows = [
            row(preset: .none, label: "テスト無振り", natureId: nil),
            row(preset: .hbBoost, label: "テストH振りB補正", natureId: nil, nature: NatureModifier(plus: .def, minus: .hp)),
        ]
        let options = ComboDefenderOptions.make(
            rows: rows, rowIDs: ["none@-", "hb_boost@-"], defenderSpeciesKey: "9002-000", defenderRanks: RankBlock(),
            items: [], abilityNames: [:], natures: ComboStub.natures)
        XCTAssertEqual(options.map(\.key), ["none@-"], "性格を引けない行は選択肢にしない(送れない要求を作らない)")
        XCTAssertEqual(options.first?.individual.natureId, ComboStub.neutral.id, "補正が一致する性格の ID 昇順の最初(ADR-0200 §2)")
    }

    func testAbilityNameIsAddedOnlyWhenRowsSplitByAbility() {
        let names = ["stub-ability-x": "テストとくせいX", "stub-ability-y": "テストとくせいY"]
        let split = ComboDefenderOptions.make(
            rows: [
                row(preset: .none, label: "テスト無振り", natureId: ComboStub.neutral.id, abilityId: "stub-ability-x"),
                row(preset: .none, label: "テスト無振り", natureId: ComboStub.neutral.id, abilityId: "stub-ability-y"),
            ],
            rowIDs: ["none@-@stub-ability-x", "none@-@stub-ability-y"], defenderSpeciesKey: "9002-000", defenderRanks: RankBlock(),
            items: [], abilityNames: names, natures: ComboStub.natures)
        XCTAssertEqual(split.map(\.label), ["テスト無振り / テストとくせいX", "テスト無振り / テストとくせいY"])
        XCTAssertEqual(split.map(\.individual.abilityId), ["stub-ability-x", "stub-ability-y"])

        let single = ComboDefenderOptions.make(
            rows: [row(preset: .none, label: "テスト無振り", natureId: ComboStub.neutral.id, abilityId: "stub-ability-x")],
            rowIDs: ["none@-"], defenderSpeciesKey: "9002-000", defenderRanks: RankBlock(),
            items: [], abilityNames: names, natures: ComboStub.natures)
        XCTAssertEqual(single.map(\.label), ["テスト無振り"], "特性で分かれていなければ特性名は添えない")
        XCTAssertEqual(single.first?.individual.abilityId, "stub-ability-x", "特性は送る(行の計算に使った特性)")
    }
}

/// `CalcViewModel.comboContext`: 連続技へ渡す計算画面の値(AC-i4・AC-i5)。
@MainActor
final class CalcViewModelComboContextTests: XCTestCase {

    private func loaded(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    func testContextMirrorsTheLastBulkRequestAndRows() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loaded(stub)
        let context = try XCTUnwrap(viewModel.comboContext, "結果の行があれば連続技の材料が揃う")
        let requests = await stub.bulkRequests
        let request = try XCTUnwrap(requests.last)
        XCTAssertEqual(context.attacker, request.attacker, "攻撃側は計算に使った個体そのまま(ランク・やけど・特性・持ち物込み)")
        XCTAssertEqual(context.mainMove, viewModel.selectedMove)
        XCTAssertEqual(context.mainCritical, request.critical)
        XCTAssertEqual(context.field, request.field)
        XCTAssertEqual(context.defenderSpeciesKey, request.defenderSpeciesKey)
        XCTAssertEqual(context.defenders.map(\.key), viewModel.rows.map(\.id), "防御側の振りは結果の行と同じ並び・同じ ID")
        XCTAssertEqual(context.attackerLearnset, StubMaster.alpha.learnset)
        XCTAssertEqual(context.natures, [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature])
    }

    func testContextFollowsCalcScreenInputs() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loaded(stub)
        await viewModel.setCritical(true)
        await viewModel.setAttackerBurned(true)
        await viewModel.selectWeather(.rain)
        await viewModel.setDefenderRank(2)
        let context = try XCTUnwrap(viewModel.comboContext)
        XCTAssertTrue(context.mainCritical, "メインの急所は計算画面の「急所」")
        XCTAssertEqual(context.attacker.status, .burn)
        XCTAssertEqual(context.field.weather, .rain)
        XCTAssertEqual(context.defenders.first?.individual.ranks.def, 2, "防御側のランクは計算画面の値")
    }

    func testContextIsNilWhenTheCalcFailed() async {
        let stub = StubMaster.makeService()
        await stub.setBulkResponder { _ in .failure(PokeCalcError(code: "internal_error", message: "boom")) }
        let viewModel = await loaded(stub)
        XCTAssertNil(viewModel.comboContext, "単発の計算が失敗中は連続技を出さない(Web の ADR-0359 追記と同じ)")
    }

    func testContextIsNilWhenNoDamagingMove() async {
        let stub = StubMaster.makeService(species: [StubMaster.statusOnly, StubMaster.beta])
        let viewModel = await loaded(stub)
        XCTAssertNil(viewModel.comboContext, "メインの技が無ければ連続技は出さない")
    }
}
