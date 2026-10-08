import XCTest

@testable import PokeCalcCore

/// お気に入り → 計算画面の個体の写像(ADR-0513 §3。純粋関数 `FavoriteLoad.plan`)。
/// 攻撃側は種族・性格・SP・特性・持ち物、防御側は特性だけ。マスタに無い要素は黙って落とさず `dropped` に残す。
/// 架空のマスタ(`StubMaster` / `StubMegaMaster`)だけを使う。
final class FavoriteLoadPlanTests: XCTestCase {

    private let sp = StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32)
    private let natures = [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature]
    private let items = StubMegaMaster.items + [StubMaster.itemA, StubMaster.itemB]

    private func favorite(
        species: SpeciesDetail = StubMaster.alpha, nature: String? = nil, ability: String? = nil, item: String? = nil
    ) -> Favorite {
        Favorite(
            id: "fav-1", label: nil,
            individual: Individual(
                speciesKey: species.key, natureId: nature ?? StubMaster.atkUpNature.id, sp: sp,
                abilityId: ability, itemId: item),
            createdAt: Date(timeIntervalSince1970: 0), updatedAt: Date(timeIntervalSince1970: 0))
    }

    private func attackerPlan(
        _ favorite: Favorite, species: SpeciesDetail = StubMaster.alpha, items: [Item]? = nil
    ) -> FavoriteLoadPlan {
        FavoriteLoad.plan(
            for: favorite, side: .attacker, species: species, natures: natures, items: items ?? self.items)
    }

    private func defenderPlan(_ favorite: Favorite, species: SpeciesDetail = StubMaster.alpha) -> FavoriteLoadPlan {
        FavoriteLoad.plan(for: favorite, side: .defender, species: species, natures: natures, items: items)
    }

    // MARK: - 攻撃側

    func testAttackerKeepsNatureSPAbilityAndItemWhenAllInMaster() {
        let plan = attackerPlan(favorite(ability: StubMaster.ability.id, item: StubMaster.itemA.id))
        XCTAssertEqual(plan.natureId, StubMaster.atkUpNature.id)
        XCTAssertEqual(plan.sp, sp, "SP は丸めずそのまま")
        XCTAssertEqual(plan.abilityId, StubMaster.ability.id)
        XCTAssertEqual(plan.itemId, StubMaster.itemA.id)
        XCTAssertEqual(plan.dropped, [])
    }

    func testAttackerWithoutAbilityAndItemDropsNothing() {
        let plan = attackerPlan(favorite())
        XCTAssertNil(plan.abilityId)
        XCTAssertNil(plan.itemId)
        XCTAssertEqual(plan.dropped, [], "保存に無いものは落としたことにしない")
    }

    func testUnknownNatureDropsNatureAndSPTogether() {
        let plan = attackerPlan(favorite(nature: "stub-nature-gone", ability: StubMaster.ability.id, item: StubMaster.itemA.id))
        XCTAssertNil(plan.natureId)
        XCTAssertNil(plan.sp, "性格が無いなら SP も使わない(性格補正と組で意味を持つ)")
        XCTAssertEqual(plan.abilityId, StubMaster.ability.id, "他の項目は読める分だけ読む")
        XCTAssertEqual(plan.itemId, StubMaster.itemA.id)
        XCTAssertEqual(plan.dropped, [.nature])
    }

    func testUnknownItemIsDropped() {
        let plan = attackerPlan(favorite(item: "stub-item-gone"))
        XCTAssertNil(plan.itemId)
        XCTAssertEqual(plan.dropped, [.item])
        XCTAssertEqual(plan.natureId, StubMaster.atkUpNature.id)
    }

    func testAbilityNotInSpeciesIsDropped() {
        let plan = attackerPlan(favorite(ability: StubMaster.abilityX.id), species: StubMaster.alpha)
        XCTAssertNil(plan.abilityId, "種族の特性に無い ID は選択肢に無いので設定しない")
        XCTAssertEqual(plan.dropped, [.ability])
    }

    func testAbilityOfSpeciesWithSeveralAbilitiesIsKept() {
        let plan = attackerPlan(
            favorite(species: StubMaster.abilityXAndY, ability: StubMaster.abilityY.id), species: StubMaster.abilityXAndY)
        XCTAssertEqual(plan.abilityId, StubMaster.abilityY.id)
        XCTAssertEqual(plan.dropped, [])
    }

    func testDroppedItemsFollowDeclarationOrder() {
        let plan = attackerPlan(favorite(nature: "stub-nature-gone", ability: "stub-ability-gone", item: "stub-item-gone"))
        XCTAssertEqual(plan.dropped, [.nature, .item, .ability])
    }

    // MARK: - 持ち物の役割・メガ固定(ADR-0509)

    func testItemThatOnlyViolatesRoleIsKeptWithoutNotice() {
        // 役割(ItemRoleFilter)は選択肢を絞るだけ。選択済みの値は残す(ADR-0509 §2。攻撃側に defender 専用の持ち物)。
        let plan = attackerPlan(favorite(item: StubMegaMaster.defenseOnly.id))
        XCTAssertEqual(plan.itemId, StubMegaMaster.defenseOnly.id)
        XCTAssertEqual(plan.dropped, [])
    }

    func testMegaStoneOnNonMegaSpeciesIsDropped() {
        let plan = attackerPlan(favorite(item: StubMegaMaster.stone.id), species: StubMaster.alpha)
        XCTAssertNil(plan.itemId, "非メガ種族はメガストーンを持てない")
        XCTAssertEqual(plan.dropped, [.item])
    }

    func testMegaSpeciesWithoutSavedItemGetsStoneSilently() {
        let plan = attackerPlan(favorite(species: StubMegaMaster.megaAlpha), species: StubMegaMaster.megaAlpha)
        XCTAssertEqual(plan.itemId, StubMegaMaster.stone.id, "メガ種族は持ち物をストーンに固定する")
        XCTAssertEqual(plan.dropped, [], "保存の持ち物が無いなら固定しても案内しない")
    }

    func testMegaSpeciesWithOtherSavedItemIsFixedToStoneAndNoted() {
        let plan = attackerPlan(
            favorite(species: StubMegaMaster.megaAlpha, item: StubMaster.itemA.id), species: StubMegaMaster.megaAlpha)
        XCTAssertEqual(plan.itemId, StubMegaMaster.stone.id)
        XCTAssertEqual(plan.dropped, [.item], "保存の持ち物を使えなかったことは案内する")
    }

    func testMegaSpeciesWithItsOwnStoneIsUnchanged() {
        let plan = attackerPlan(
            favorite(species: StubMegaMaster.megaAlpha, item: StubMegaMaster.stone.id), species: StubMegaMaster.megaAlpha)
        XCTAssertEqual(plan.itemId, StubMegaMaster.stone.id)
        XCTAssertEqual(plan.dropped, [])
    }

    func testMegaSpeciesWhoseStoneIsMissingFromMasterClearsItem() {
        let withSaved = attackerPlan(
            favorite(species: StubMegaMaster.megaMissingStone, item: StubMaster.itemA.id), species: StubMegaMaster.megaMissingStone)
        XCTAssertNil(withSaved.itemId, "ストーンをマスタから引けないメガは持ち物なし(MegaItemLock.missing)")
        XCTAssertEqual(withSaved.dropped, [.item])

        let without = attackerPlan(favorite(species: StubMegaMaster.megaMissingStone), species: StubMegaMaster.megaMissingStone)
        XCTAssertNil(without.itemId)
        XCTAssertEqual(without.dropped, [])
    }

    // MARK: - 防御側(種族と特性だけ)

    func testDefenderUsesAbilityOnly() {
        let plan = defenderPlan(
            favorite(species: StubMaster.abilityXAndY, ability: StubMaster.abilityX.id, item: StubMaster.itemA.id),
            species: StubMaster.abilityXAndY)
        XCTAssertEqual(plan.abilityId, StubMaster.abilityX.id)
        XCTAssertNil(plan.natureId, "防御側は性格を使わない")
        XCTAssertNil(plan.sp, "防御側は SP を使わない")
        XCTAssertNil(plan.itemId, "防御側は持ち物を使わない(攻撃側のお気に入りを読んでも)")
        XCTAssertEqual(plan.dropped, [], "使わない項目を落としたことにはしない")
    }

    func testDefenderIgnoresUnknownNatureAndItemWithoutNotice() {
        let plan = defenderPlan(favorite(nature: "stub-nature-gone", item: "stub-item-gone"))
        XCTAssertEqual(plan.dropped, [])
    }

    func testDefenderAbilityNotInSpeciesIsDropped() {
        let plan = defenderPlan(favorite(ability: StubMaster.abilityY.id), species: StubMaster.alpha)
        XCTAssertNil(plan.abilityId)
        XCTAssertEqual(plan.dropped, [.ability])
    }

    func testDefenderWithoutSavedAbilityIsUnspecified() {
        let plan = defenderPlan(favorite())
        XCTAssertNil(plan.abilityId, "指定なし(サーバーが種族の特性をすべて試す)")
        XCTAssertEqual(plan.dropped, [])
    }

    // MARK: - 純粋(同じ入力で同じ結果)

    func testPlanIsDeterministic() {
        let input = favorite(ability: StubMaster.ability.id, item: StubMaster.itemA.id)
        XCTAssertEqual(attackerPlan(input), attackerPlan(input))
    }
}
