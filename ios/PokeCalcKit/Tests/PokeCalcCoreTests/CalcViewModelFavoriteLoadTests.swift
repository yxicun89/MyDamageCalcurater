import XCTest

@testable import PokeCalcCore

/// 計算画面でお気に入りを読み込む(ADR-0513。`CalcViewModel.loadFavorite(_:side:)`)。
///
/// 約束: 読み込みは計算ちょうど1回(全部読めなかったときは0回)・`species(key:)` は新しい種族につき1回・
/// 古い応答は捨てる・読めなかった分は黙って落とさず `favoriteLoadNotice` で案内・失敗しても画面のエラーにしない・
/// 規則(ランク・やけど・天候などの画面条件を戻さない)は構築から呼ぶ処理(`selectTeamIndividual`)に揃える。
/// 架空のマスタ(`StubMaster` / `StubMegaMaster` / `StubBulkMaster`)だけを使う。
@MainActor
final class CalcViewModelFavoriteLoadTests: XCTestCase {

    // MARK: - 補助

    private let sp = StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32)

    private func makeStub() -> StubPokeCalcService {
        StubMaster.makeService(
            species: [
                StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.abilityXAndY,
                StubMegaMaster.megaAlpha, StubMegaMaster.megaMissingStone,
            ],
            items: StubMegaMaster.items + [StubMaster.itemA, StubMaster.itemB])
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func favorite(
        _ id: String = "fav-1", label: String? = nil, species: SpeciesDetail = StubMaster.gamma,
        nature: String? = nil, ability: String? = nil, item: String? = nil,
        ranks: RankBlock = RankBlock(), status: StatusCondition = .none
    ) -> Favorite {
        favorite(id, label: label, key: species.key, nature: nature, ability: ability, item: item, ranks: ranks, status: status)
    }

    private func favorite(
        _ id: String, label: String?, key: String, nature: String?, ability: String?, item: String?,
        ranks: RankBlock = RankBlock(), status: StatusCondition = .none
    ) -> Favorite {
        Favorite(
            id: id, label: label,
            individual: Individual(
                speciesKey: key, natureId: nature ?? StubMaster.spaUpNature.id, sp: sp,
                abilityId: ability, itemId: item, ranks: ranks, status: status),
            createdAt: Date(timeIntervalSince1970: 0), updatedAt: Date(timeIntervalSince1970: 0))
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    private func bulkCount(_ stub: StubPokeCalcService) async -> Int { await stub.bulkRequests.count }
    private func speciesCount(_ stub: StubPokeCalcService) async -> Int { await stub.speciesRequests.count }

    // MARK: - 攻撃側: 個体の読み込み

    func testAttackerLoadSetsIndividualAndCalculatesExactlyOnce() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)
        let speciesBefore = await speciesCount(stub)
        let saved = favorite(
            label: "テストこたい", species: StubMaster.gamma, nature: StubMaster.spaUpNature.id,
            ability: StubMaster.ability.id, item: StubMaster.itemA.id)

        await viewModel.loadFavorite(saved, side: .attacker)

        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter - bulkBefore, 1, "計算はちょうど1回")
        let speciesAfter = await speciesCount(stub)
        XCTAssertEqual(speciesAfter - speciesBefore, 1, "新しい種族の species(key:) は1回(詳細を重ねて読まない)")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.gamma.key)
        XCTAssertEqual(request.attacker.natureId, StubMaster.spaUpNature.id, "プリセットに丸め直さない")
        XCTAssertEqual(request.attacker.sp, sp)
        XCTAssertEqual(request.attacker.abilityId, StubMaster.ability.id)
        XCTAssertEqual(request.attacker.itemId, StubMaster.itemA.id)
        XCTAssertNil(request.attacker.moveId, "技は要求本体の moveId が正")
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.beta.key, "防御側は変えない")

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemA.id)
        XCTAssertEqual(viewModel.attackerAbilityId, StubMaster.ability.id)
        XCTAssertEqual(viewModel.attackerAbilityOptions, StubMaster.gamma.abilities, "特性の選択肢を新しい種族に整合させる")
        XCTAssertNil(viewModel.attackerPreset, "お気に入りの個体はプリセットのどれでもない")
        let selection = try XCTUnwrap(viewModel.attackerBuildSource.teamSelection)
        XCTAssertEqual(selection.teamID, FavoriteLoad.sourceTeamID)
        XCTAssertEqual(selection.memberID, saved.id)
        XCTAssertEqual(selection.displayName, "テストこたい", "ラベルがあればラベル")
        XCTAssertEqual(selection.individual.natureId, StubMaster.spaUpNature.id)
        XCTAssertEqual(selection.individual.sp, sp)
        XCTAssertNil(viewModel.favoriteLoadNotice, "全部読めたら案内なし")
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }

    func testAttackerDisplayNameFallsBackToSpeciesName() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadFavorite(favorite(species: StubMaster.gamma), side: .attacker)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.displayName, StubMaster.gamma.nameJa)
    }

    func testAttackerLoadKeepsCurrentMoveWhenInNewLearnset() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectMove(id: StubMaster.specialMove.id)  // alpha の learnset にあり、gamma の learnset にもある

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma), side: .attacker)

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key, "前提: 読み込めている")
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id, "技は変えない(新しい learnset にあるなら残す)")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
    }

    func testAttackerLoadFallsBackToFirstDamagingMoveWhenCurrentMoveNotLearned() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id, "前提: alpha の既定の技(beta の learnset には無い)")

        await viewModel.loadFavorite(favorite(species: StubMaster.beta), side: .attacker)

        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id, "既定の規則: learnset の最初のダメージ技")
        XCTAssertNil(viewModel.error)
    }

    func testAttackerLoadDoesNotRestoreSavedRanksStatusOrTouchScreenConditions() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setCritical(true)
        await viewModel.setAttackerBurned(true)
        await viewModel.setAttackerRank(2)
        let ranksBefore = viewModel.attackerRanks
        let saved = favorite(
            species: StubMaster.gamma, ranks: RankBlock(atk: -3, def: 0, spa: 0, spd: 0, spe: 0), status: .paralysis)

        await viewModel.loadFavorite(saved, side: .attacker)

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key, "前提: 読み込めている")
        // 構築から呼ぶ処理と同じ: ランク・やけど・急所は画面の計算条件で、読み込みでは変えない・お気に入りの値も持ち込まない。
        XCTAssertEqual(viewModel.attackerRanks, ranksBefore)
        XCTAssertTrue(viewModel.isAttackerBurned)
        XCTAssertTrue(viewModel.isCritical)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.ranks, ranksBefore)
        XCTAssertEqual(request.attacker.status, .burn)
        XCTAssertTrue(request.critical)
    }

    func testAttackerLoadSpeciesOutsideFirstPageIsSelectable() async throws {
        // 先頭ページ(検索結果)に無い種族でも、species(key:) で引けるなら読み込める(issue #68 と同じ考え方)。
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let hidden = favorite(species: StubBulkMaster.hiddenSpecies)

        await viewModel.loadFavorite(hidden, side: .attacker)

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubBulkMaster.hiddenSpecies.key)
        XCTAssertEqual(viewModel.attackerSpecies?.nameJa, StubBulkMaster.hiddenSpecies.nameJa, "カードに名前が出せる")
        XCTAssertNil(viewModel.favoriteLoadNotice)
    }

    // MARK: - 攻撃側: マスタに無い要素

    func testAttackerLoadWithUnknownSpeciesChangesNothingAndNotes() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)
        let rowsBefore = viewModel.rows
        let sourceBefore = viewModel.attackerBuildSource

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma, nature: nil).withSpeciesKey("stub-gone-000"), side: .attacker)

        XCTAssertEqual(viewModel.favoriteLoadNotice, .speciesMissing)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key, "何も変えない")
        XCTAssertEqual(viewModel.attackerBuildSource, sourceBefore)
        XCTAssertEqual(viewModel.rows, rowsBefore, "結果の表示は消さない")
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter, bulkBefore, "全部読めなかったときは計算しない")
        XCTAssertNil(viewModel.error, "画面のエラーにしない")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testAttackerLoadWhenSpeciesLookupFailsChangesNothingAndNotesUnavailable() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)
        let rowsBefore = viewModel.rows
        await stub.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト: 通信できない"))

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma), side: .attacker)

        XCTAssertEqual(viewModel.favoriteLoadNotice, .unavailable)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.rows, rowsBefore)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter, bulkBefore)
        XCTAssertNil(viewModel.error, "お気に入りを引けなくても計算画面のエラーにしない(絶対ルール5)")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testUnknownNatureKeepsPresetAndSetsTheRestWithNotice() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let presetBefore = viewModel.attackerPreset
        let saved = favorite(
            species: StubMaster.gamma, nature: "stub-nature-gone", ability: StubMaster.ability.id, item: StubMaster.itemA.id)

        await viewModel.loadFavorite(saved, side: .attacker)

        XCTAssertEqual(viewModel.favoriteLoadNotice, .partial([.nature]))
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.attackerPreset, presetBefore, "性格が無いので出どころはいまのプリセットのまま")
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemA.id)
        XCTAssertEqual(viewModel.attackerAbilityId, StubMaster.ability.id)
        let request = try await lastRequest(stub)
        XCTAssertNotEqual(request.attacker.sp, sp, "保存の SP は使わない(性格と組)")
        XCTAssertNotEqual(request.attacker.natureId, "stub-nature-gone")
    }

    func testUnknownItemAndAbilityAreDroppedWithNotice() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let saved = favorite(species: StubMaster.gamma, ability: "stub-ability-gone", item: "stub-item-gone")

        await viewModel.loadFavorite(saved, side: .attacker)

        XCTAssertEqual(viewModel.favoriteLoadNotice, .partial([.item, .ability]))
        XCTAssertNil(viewModel.attackerItemId)
        XCTAssertNil(viewModel.attackerAbilityId)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.individual.sp, sp, "性格と SP は読めた")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.itemId)
        XCTAssertNil(request.attacker.abilityId)
    }

    // MARK: - 攻撃側: 持ち物の役割・メガ固定(ADR-0509)

    func testMegaSpeciesFavoriteFixesItemToStoneAndNotesWhenSavedItemDiffers() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        await viewModel.loadFavorite(favorite(species: StubMegaMaster.megaAlpha, item: StubMaster.itemA.id), side: .attacker)

        XCTAssertEqual(viewModel.attackerItemId, StubMegaMaster.stone.id)
        XCTAssertTrue(viewModel.attackerItemLock.disablesItemField)
        XCTAssertEqual(viewModel.favoriteLoadNotice, .partial([.item]))
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.itemId, StubMegaMaster.stone.id)
    }

    func testMegaSpeciesFavoriteWithoutSavedItemFixesStoneWithoutNotice() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        await viewModel.loadFavorite(favorite(species: StubMegaMaster.megaAlpha), side: .attacker)

        XCTAssertEqual(viewModel.attackerItemId, StubMegaMaster.stone.id)
        XCTAssertNil(viewModel.favoriteLoadNotice)
    }

    func testMegaStoneOnNonMegaSpeciesIsDropped() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma, item: StubMegaMaster.stone.id), side: .attacker)

        XCTAssertNil(viewModel.attackerItemId)
        XCTAssertEqual(viewModel.favoriteLoadNotice, .partial([.item]))
    }

    func testRoleViolatingItemIsKeptAndStillSelectable() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma, item: StubMegaMaster.defenseOnly.id), side: .attacker)

        XCTAssertEqual(viewModel.attackerItemId, StubMegaMaster.defenseOnly.id, "役割に反するだけなら残す(選択済みの値は選択肢に残る)")
        XCTAssertTrue(viewModel.attackerItemOptions.contains { $0.id == StubMegaMaster.defenseOnly.id })
        XCTAssertNil(viewModel.favoriteLoadNotice)
    }

    func testLeavingMegaAfterFavoriteLoadClearsFixedItemLikeSpeciesChange() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadFavorite(favorite(species: StubMegaMaster.megaAlpha), side: .attacker)
        XCTAssertEqual(viewModel.attackerItemId, StubMegaMaster.stone.id, "前提: メガでストーン固定")

        await viewModel.loadFavorite(favorite("fav-2", species: StubMaster.gamma), side: .attacker)

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertNil(viewModel.attackerItemId, "メガから非メガへ変わる持ち物の扱いは種族変更と同じ(ADR-0509 §4)")
        XCTAssertFalse(viewModel.attackerItemLock.disablesItemField)
    }

    // MARK: - 防御側

    func testDefenderLoadSetsSpeciesAndAbilityAndCalculatesExactlyOnce() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let before = try await lastRequest(stub)
        let bulkBefore = await bulkCount(stub)
        let speciesBefore = await speciesCount(stub)
        // 攻撃側として保存した個体(性格・SP・持ち物つき)を防御側に読んでも、使うのは種族と特性だけ。
        let saved = favorite(
            species: StubMaster.abilityXAndY, nature: StubMaster.atkUpNature.id, ability: StubMaster.abilityY.id,
            item: StubMaster.itemA.id)

        await viewModel.loadFavorite(saved, side: .defender)

        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter - bulkBefore, 1, "計算はちょうど1回")
        let speciesAfter = await speciesCount(stub)
        XCTAssertEqual(speciesAfter - speciesBefore, 1, "防御側の species(key:) は1回")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.abilityXAndY.key)
        XCTAssertEqual(request.defenderAbilityId, StubMaster.abilityY.id)
        XCTAssertEqual(request.attacker, before.attacker, "攻撃側は変えない(性格・SP・持ち物を持ち込まない)")
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.abilityXAndY.key)
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityY.id)
        XCTAssertEqual(viewModel.defenderAbilityOptions, StubMaster.abilityXAndY.abilities, "特性の選択肢を読み込みで整合させる")
        XCTAssertEqual(viewModel.defenderSpecies?.key, StubMaster.abilityXAndY.key)
        XCTAssertNil(viewModel.favoriteLoadNotice, "防御側で使わない項目は案内しない")
        XCTAssertNil(viewModel.error)
    }

    func testDefenderLoadThenViewLoadingAbilityOptionsDoesNotReadOrCalculateAgain() async throws {
        // View は防御側が変わるたび `.task(id:)` で loadDefenderAbilityOptions() を呼ぶ(P6-19)。
        // 読み込みで選択肢が整合済みなら、重ねて species(key:) を読まず・計算もしない。
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadFavorite(favorite(species: StubMaster.abilityXAndY, ability: StubMaster.abilityX.id), side: .defender)
        let bulkBefore = await bulkCount(stub)
        let speciesBefore = await speciesCount(stub)

        await viewModel.loadDefenderAbilityOptions()

        let speciesAfter = await speciesCount(stub)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(speciesAfter, speciesBefore)
        XCTAssertEqual(bulkAfter, bulkBefore)
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityX.id)
    }

    func testDefenderAbilityNotInSpeciesIsDroppedWithNotice() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma, ability: StubMaster.abilityX.id), side: .defender)

        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key)
        XCTAssertNil(viewModel.defenderAbilityId)
        XCTAssertEqual(viewModel.favoriteLoadNotice, .partial([.ability]))
    }

    func testDefenderLoadWithoutSavedAbilityResetsPreviousAbility() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)
        await viewModel.loadDefenderAbilityOptions()
        await viewModel.selectDefenderAbility(id: StubMaster.abilityX.id)
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityX.id)

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma), side: .defender)

        XCTAssertNil(viewModel.defenderAbilityId, "旧種族の特性を残さない(指定なし)")
        XCTAssertEqual(viewModel.defenderAbilityOptions, StubMaster.gamma.abilities)
        XCTAssertNil(viewModel.favoriteLoadNotice)
    }

    func testDefenderLoadWithUnknownSpeciesChangesNothingAndNotes() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma).withSpeciesKey("stub-gone-000"), side: .defender)

        XCTAssertEqual(viewModel.favoriteLoadNotice, .speciesMissing)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter, bulkBefore)
        XCTAssertNil(viewModel.error)
    }

    func testDefenderLoadKeepsDefenderRanksLikeSelectDefender() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setDefenderRank(1)
        let ranksBefore = viewModel.defenderRanks

        await viewModel.loadFavorite(favorite(species: StubMaster.gamma), side: .defender)

        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key, "前提: 読み込めている")
        XCTAssertEqual(viewModel.defenderRanks, ranksBefore, "リセット規則は selectDefender に揃える(ランクは画面の計算条件)")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, ranksBefore)
    }

    func testMegaDefenderFavoriteLocksItemVariantsInTheSameSingleCalculation() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)

        await viewModel.loadFavorite(favorite(species: StubMegaMaster.megaAlpha), side: .defender)

        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter - bulkBefore, 1, "メガ固定を反映するための2回目の計算をしない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemVariants, [StubMegaMaster.stone.id], "防御側のメガはストーン1件(ADR-0509 §4)")
    }

    // MARK: - 古い応答の破棄・失敗

    func testStaleAttackerLoadDoesNotOverwriteNewerOne() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)
        let baseline = await speciesCount(stub)
        await stub.setSpeciesMode(.manual)
        // 古い方(unknown item 付き = 案内が出る内容)と、新しい方(全部読める)。
        let older = favorite("old", species: StubMaster.gamma, item: "stub-item-gone")
        let newer = favorite("new", species: StubMaster.beta, nature: StubMaster.atkUpNature.id)

        let loadingOlder = Task { await viewModel.loadFavorite(older, side: .attacker) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let loadingNewer = Task { await viewModel.loadFavorite(newer, side: .attacker) }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        let betaDetail = try await stub.lookupSpecies(key: StubMaster.beta.key)
        let gammaDetail = try await stub.lookupSpecies(key: StubMaster.gamma.key)
        await stub.resolveSpecies(at: baseline + 1, with: .success(betaDetail))
        await loadingNewer.value
        await stub.resolveSpecies(at: baseline, with: .success(gammaDetail))  // 古い応答が後から届く
        await loadingOlder.value

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, "new", "古い応答で出どころを上書きしない")
        XCTAssertNil(viewModel.favoriteLoadNotice, "古い読み込みの案内を出さない")
        XCTAssertNil(viewModel.error)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter - bulkBefore, 1, "古い読み込みは計算しない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.beta.key)
    }

    func testStaleFailureOfOlderLoadDoesNotShowNoticeOrError() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let baseline = await speciesCount(stub)
        await stub.setSpeciesMode(.manual)

        let loadingOlder = Task { await viewModel.loadFavorite(self.favorite("old", species: StubMaster.gamma), side: .attacker) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let loadingNewer = Task { await viewModel.loadFavorite(self.favorite("new", species: StubMaster.beta), side: .attacker) }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        let betaDetail = try await stub.lookupSpecies(key: StubMaster.beta.key)
        await stub.resolveSpecies(at: baseline + 1, with: .success(betaDetail))
        await loadingNewer.value
        await stub.resolveSpecies(at: baseline, with: .failure(PokeCalcError(code: "not_found", message: "テスト")))
        await loadingOlder.value

        XCTAssertNil(viewModel.favoriteLoadNotice, "古い読み込みの失敗の案内を出さない")
        XCTAssertNil(viewModel.error)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }

    func testStaleDefenderLoadDoesNotOverwriteNewerOne() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)
        let baseline = await speciesCount(stub)
        await stub.setSpeciesMode(.manual)

        let loadingOlder = Task { await viewModel.loadFavorite(self.favorite("old", species: StubMaster.abilityXAndY, ability: StubMaster.abilityX.id), side: .defender) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let loadingNewer = Task { await viewModel.loadFavorite(self.favorite("new", species: StubMaster.gamma), side: .defender) }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        let gammaDetail = try await stub.lookupSpecies(key: StubMaster.gamma.key)
        let xyDetail = try await stub.lookupSpecies(key: StubMaster.abilityXAndY.key)
        await stub.resolveSpecies(at: baseline + 1, with: .success(gammaDetail))
        await loadingNewer.value
        await stub.resolveSpecies(at: baseline, with: .success(xyDetail))
        await loadingOlder.value

        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key)
        XCTAssertNil(viewModel.defenderAbilityId, "古い防御側の特性を入れない")
        XCTAssertEqual(viewModel.defenderAbilityOptions, StubMaster.gamma.abilities)
        let bulkAfter = await bulkCount(stub)
        XCTAssertEqual(bulkAfter - bulkBefore, 1)
    }

    func testLoadFavoriteWhileCalculatingSupersedesInFlightCalculation() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let bulkBefore = await bulkCount(stub)
        await stub.setBulkMode(.manual)

        let selecting = Task { await viewModel.selectMove(id: StubMaster.specialMove.id) }
        try await stub.waitForBulkRequests(count: bulkBefore + 1)
        let loading = Task { await viewModel.loadFavorite(self.favorite(species: StubMaster.gamma), side: .attacker) }
        try await stub.waitForBulkRequests(count: bulkBefore + 2)
        await stub.resolveBulkWithEcho(at: bulkBefore + 1)
        await loading.value
        await stub.resolveBulk(at: bulkBefore, with: .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト: 古い計算の失敗")))
        await selecting.value

        XCTAssertNil(viewModel.error, "追い越された古い計算の失敗を出さない")
        XCTAssertFalse(viewModel.rows.isEmpty)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - 案内の寿命

    func testNoticeIsClearedByTheNextInput() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadFavorite(favorite(species: StubMaster.gamma, item: "stub-item-gone"), side: .attacker)
        XCTAssertNotNil(viewModel.favoriteLoadNotice)

        await viewModel.setCritical(true)

        XCTAssertNil(viewModel.favoriteLoadNotice, "次の入力操作で古い案内を消す")
    }

    func testNewLoadClearsPreviousNotice() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadFavorite(favorite("a", species: StubMaster.gamma, item: "stub-item-gone"), side: .attacker)
        XCTAssertEqual(viewModel.favoriteLoadNotice, .partial([.item]))

        await viewModel.loadFavorite(favorite("b", species: StubMaster.beta), side: .attacker)

        XCTAssertNil(viewModel.favoriteLoadNotice)
    }

    func testDismissClearsNotice() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.loadFavorite(favorite(species: StubMaster.gamma).withSpeciesKey("stub-gone-000"), side: .attacker)
        XCTAssertEqual(viewModel.favoriteLoadNotice, .speciesMissing)

        viewModel.dismissFavoriteLoadNotice()

        XCTAssertNil(viewModel.favoriteLoadNotice)
    }

    // MARK: - 既存の経路は変えない

    func testInitialLoadAndPresetPathAreUnchangedWithoutFavorites() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        XCTAssertNil(viewModel.favoriteLoadNotice)
        XCTAssertNotNil(viewModel.attackerPreset)
        let bulkCount = await bulkCount(stub)
        XCTAssertEqual(bulkCount, 1, "起動時の一括計算はこれまでどおり1回")
    }
}

private extension Favorite {
    /// マスタに無い種族のお気に入りを作る(他の項目はそのまま)。
    func withSpeciesKey(_ key: String) -> Favorite {
        var copy = self
        copy.individual.speciesKey = key
        return copy
    }
}
