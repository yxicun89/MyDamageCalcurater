import XCTest

@testable import PokeCalcCore

/// 計算画面の持ち物の役割とメガ固定(ADR-0509 §3・§4・§6)。
///
/// - 攻撃側の持ち物 = `.attacker`、「持ち物の候補も比較」= `.defender`。`itemOptions`(全件)は残す。
/// - メガの攻撃側は持ち物をストーンに固定(要求の `attacker.itemId` = ストーン)、メガ以外に変えたら未選択に戻す。
/// - 防御側のメガは詳細を読んだ後に分かる(L1・L2)。固定中は `itemVariants = [ストーン]`、比較のトグルは無視する。
/// - 表示名はストーンの `nameJa` ではなく「{基本種名}のメガストーン」。
@MainActor
final class CalcViewModelItemRolesTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    private func calcCount(_ stub: StubPokeCalcService, during operation: () async -> Void) async -> Int {
        let before = await stub.bulkRequests.count
        await operation()
        return await stub.bulkRequests.count - before
    }

    // MARK: - 役割で絞る

    func testItemOptionsAreFilteredByRole() async {
        let viewModel = await loadedViewModel(Mega.makeService())

        XCTAssertEqual(viewModel.attackerItemOptions.map(\.id), [Mega.attackOnly.id, Mega.both.id])
        XCTAssertEqual(viewModel.defenderCompareItemOptions.map(\.id), [Mega.defenseOnly.id, Mega.both.id])
        XCTAssertEqual(viewModel.itemOptions, Mega.items, "全件(名前の引き当て・固定の検索用)は従来どおり")
    }

    /// `roles` が無い(古いサーバー・モック)ときは絞らない。
    func testLegacyItemsWithoutRolesAreNotFiltered() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())

        XCTAssertEqual(viewModel.attackerItemOptions, [StubMaster.itemA, StubMaster.itemB])
        XCTAssertEqual(viewModel.defenderCompareItemOptions, [StubMaster.itemA, StubMaster.itemB])
    }

    func testSelectingItemOutsideAttackerOptionsIsIgnored() async {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)

        let calcs = await calcCount(stub) { await viewModel.selectAttackerItem(id: Mega.defenseOnly.id) }

        XCTAssertEqual(calcs, 0, "攻撃側で意味の無い持ち物は選べない")
        XCTAssertNil(viewModel.attackerItemId)
    }

    func testTogglingItemOutsideDefenderOptionsIsIgnored() async {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)

        let calcs = await calcCount(stub) { await viewModel.toggleDefenderItemComparison(itemId: Mega.attackOnly.id) }

        XCTAssertEqual(calcs, 0)
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [])
    }

    // MARK: - 攻撃側のメガ固定

    func testSelectingMegaAttackerLocksStone() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttackerItem(id: Mega.both.id)

        await viewModel.selectAttacker(speciesKey: Mega.megaAlpha.key)

        XCTAssertEqual(viewModel.attackerItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(viewModel.attackerItemId, Mega.stone.id)
        XCTAssertEqual(viewModel.itemLabel(for: viewModel.attackerItemId), Mega.megaAlphaStoneName)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.itemId, Mega.stone.id, "メガ種族は requiredItemId を持って計算する")
    }

    func testSelectAttackerItemIsIgnoredWhileLocked() async {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttacker(speciesKey: Mega.megaAlpha.key)

        let calcs = await calcCount(stub) {
            await viewModel.selectAttackerItem(id: Mega.both.id)
            await viewModel.selectAttackerItem(id: nil)
        }

        XCTAssertEqual(calcs, 0)
        XCTAssertEqual(viewModel.attackerItemId, Mega.stone.id)
    }

    func testChangingToNonMegaUnlocksAndClearsItem() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttacker(speciesKey: Mega.megaAlpha.key)

        await viewModel.selectAttacker(speciesKey: StubMaster.gamma.key)

        XCTAssertEqual(viewModel.attackerItemLock, .none)
        XCTAssertNil(viewModel.attackerItemId, "メガストーンを残さない")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.itemId)
    }

    func testMegaWithoutBaseNameShowsGenericStoneName() async {
        let viewModel = await loadedViewModel(Mega.makeService())

        await viewModel.selectAttacker(speciesKey: Mega.megaNoBaseName.key)

        XCTAssertEqual(viewModel.attackerItemLock, .locked(itemId: Mega.otherStone.id, displayName: "メガストーン"))
        XCTAssertEqual(viewModel.itemLabel(for: Mega.otherStone.id), "メガストーン")
    }

    func testMegaWithMissingStoneSendsNoItem() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttackerItem(id: Mega.both.id)

        await viewModel.selectAttacker(speciesKey: Mega.megaMissingStone.key)

        XCTAssertEqual(viewModel.attackerItemLock, .missing)
        XCTAssertNil(viewModel.attackerItemId, "黙って別の持ち物にしない")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.itemId)
    }

    // MARK: - 防御側のメガ固定(L1・L2)

    /// 比較が無いときは防御側の変更で species(key:) を読まない約束(ADR-0501「P6-19」)を保ち、
    /// View が呼ぶ `loadDefenderAbilityOptions()` で固定が分かったら1回だけ計算し直す。
    func testDefenderMegaLockIsAppliedAfterDetailLoad() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        let speciesBefore = await stub.speciesRequests.count
        await viewModel.selectDefender(speciesKey: Mega.megaAlpha.key)
        let speciesAfterSelect = await stub.speciesRequests.count
        XCTAssertEqual(speciesAfterSelect, speciesBefore, "比較が無ければ防御側の変更では読まない(既存の約束)")

        let calcs = await calcCount(stub) { await viewModel.loadDefenderAbilityOptions() }

        XCTAssertEqual(viewModel.defenderItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(calcs, 1, "固定が変わったときだけ1回計算し直す")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemVariants, [Mega.stone.id], "メガの防御側はストーン1件(null も混ぜない)")
        XCTAssertEqual(viewModel.rows.map(\.itemLabel), [Mega.megaAlphaStoneName], "行にストーンの nameJa を出さない")
    }

    /// 比較のトグルがあるときは、送れない持ち物(メガ + 別の持ち物)を送らないよう要求の前に詳細を読む(L1)。
    func testDefenderMegaWithComparisonReadsDetailBeforeRequest() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.toggleDefenderItemComparison(itemId: Mega.both.id)
        let bulkBefore = await stub.bulkRequests.count

        await viewModel.selectDefender(speciesKey: Mega.megaAlpha.key)

        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests.last, Mega.megaAlpha.key)
        let sent = await stub.bulkRequests.dropFirst(bulkBefore)
        XCTAssertFalse(sent.isEmpty)
        for request in sent {
            XCTAssertEqual(request.itemVariants, [Mega.stone.id], "メガの防御側にストーン以外を送らない")
        }
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [Mega.both.id], "トグルは画面の設定として残す(固定中は使わない)")
    }

    func testToggleIsIgnoredWhileDefenderLocked() async {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: Mega.megaAlpha.key)
        await viewModel.loadDefenderAbilityOptions()

        let calcs = await calcCount(stub) { await viewModel.toggleDefenderItemComparison(itemId: Mega.both.id) }

        XCTAssertEqual(calcs, 0)
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [])
    }

    /// 防御側をメガ以外に戻したら固定を外し、トグルが要求に戻る。
    func testDefenderUnlockRestoresComparison() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.toggleDefenderItemComparison(itemId: Mega.both.id)
        await viewModel.selectDefender(speciesKey: Mega.megaAlpha.key)

        await viewModel.selectDefender(speciesKey: StubMaster.gamma.key)

        XCTAssertEqual(viewModel.defenderItemLock, .none)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemVariants, [nil, Mega.both.id])
    }

    // MARK: - 攻守入れ替え

    /// 入れ替えで攻撃側がメガでなくなったら未選択に戻し、防御側に来たメガは(読み済みなので)すぐ固定する。
    func testSwapMovesLockWithSpecies() async throws {
        let stub = Mega.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttacker(speciesKey: Mega.megaAlpha.key)

        await viewModel.swapSides()

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.attackerItemLock, .none)
        XCTAssertNil(viewModel.attackerItemId)
        XCTAssertEqual(viewModel.defenderItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        let request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.itemId)
        XCTAssertEqual(request.itemVariants, [Mega.stone.id])
    }
}
