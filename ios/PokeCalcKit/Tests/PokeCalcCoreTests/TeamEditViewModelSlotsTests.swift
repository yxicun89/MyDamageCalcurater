import XCTest

@testable import PokeCalcCore

// F-08(ADR-0522): 構築編集は 6 つの枠が最初から並ぶ。枠の入れ替え・外す・未保存の判定・明示保存。
// 枠は `slotIDs`(長さ 6。メンバー id か nil)で持ち、`team.members` は枠の順に詰めた並びを保つ
// (保存は種族の決まった枠だけを枠の順に詰める。空き枠の位置は保存しない)。
@MainActor
final class TeamEditViewModelSlotsTests: XCTestCase {
    private func makeViewModel(
        existing: Int, store: StubTeamStore = StubTeamStore(), now: Date = Date(timeIntervalSince1970: 1_800_000_000)
    ) async -> TeamEditViewModel {
        let members = (0..<existing).map {
            TeamMember(id: "m\($0)", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")
        }
        let viewModel = TeamEditViewModel(
            store: store, service: StubMaster.makeService(),
            team: Team(id: "team-1", name: TeamNaming.defaultName, members: members), now: { now })
        await viewModel.load()
        return viewModel
    }

    // MARK: 6 枠

    func testAlwaysSixSlotsFilledFromTheFront() async {
        let viewModel = await makeViewModel(existing: 2)
        XCTAssertEqual(viewModel.slotIDs, ["m0", "m1", nil, nil, nil, nil])
        XCTAssertEqual(viewModel.slotIDs.count, TeamLimits.maxMembers)
        XCTAssertEqual(viewModel.member(atSlot: 0)?.id, "m0")
        XCTAssertNil(viewModel.member(atSlot: 2))
        XCTAssertNil(viewModel.member(atSlot: 6), "範囲外は nil")
    }

    func testNewTeamHasSixEmptySlots() async {
        let viewModel = await makeViewModel(existing: 0)
        XCTAssertEqual(viewModel.slotIDs, Array(repeating: nil, count: 6))
    }

    func testSelectingSpeciesOnEmptySlotFillsThatSlot() async {
        let viewModel = await makeViewModel(existing: 1)
        await viewModel.selectSpecies(slot: 3, speciesKey: StubMaster.beta.key)
        let member = viewModel.member(atSlot: 3)
        XCTAssertEqual(member?.speciesKey, StubMaster.beta.key)
        XCTAssertNil(viewModel.member(atSlot: 1), "他の空き枠は埋まらない")
        XCTAssertEqual(viewModel.team.members.map(\.speciesKey), [StubMaster.alpha.key, StubMaster.beta.key], "members は枠の順")
        guard let id = member?.id else { return XCTFail("枠が埋まっていない") }
        XCTAssertFalse(viewModel.moveOptionsByMember[id]?.isEmpty ?? true, "種族を選ぶと技の候補が出る")
        XCTAssertFalse(viewModel.abilityOptionsByMember[id]?.isEmpty ?? true)
        XCTAssertEqual(member?.natureId, viewModel.natureOptions.first?.id)
    }

    func testSelectingSpeciesOnFilledSlotChangesSpeciesKeepingTheMember() async {
        let viewModel = await makeViewModel(existing: 1)
        await viewModel.selectSpecies(slot: 0, speciesKey: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.member(atSlot: 0)?.id, "m0")
        XCTAssertEqual(viewModel.member(atSlot: 0)?.speciesKey, StubMaster.gamma.key)
    }

    func testSelectingOutOfRangeSlotDoesNothing() async {
        let viewModel = await makeViewModel(existing: 1)
        await viewModel.selectSpecies(slot: 6, speciesKey: StubMaster.beta.key)
        XCTAssertEqual(viewModel.team.members.count, 1)
    }

    // MARK: 外す

    func testRemovingASlotEmptiesItWithoutMovingOthers() async {
        let viewModel = await makeViewModel(existing: 3)
        viewModel.removeSlot(1)
        XCTAssertEqual(viewModel.slotIDs, ["m0", nil, "m2", nil, nil, nil])
        XCTAssertEqual(viewModel.team.members.map(\.id), ["m0", "m2"])
        XCTAssertNil(viewModel.moveOptionsByMember["m1"])
    }

    func testRemovingAnEmptySlotIsHarmless() async {
        let viewModel = await makeViewModel(existing: 1)
        viewModel.removeSlot(4)
        XCTAssertEqual(viewModel.team.members.map(\.id), ["m0"])
        XCTAssertFalse(viewModel.hasUnsavedChanges)
    }

    // MARK: 上へ・下へ

    func testMoveDownSwapsWithNeighbour() async {
        let viewModel = await makeViewModel(existing: 3)
        XCTAssertTrue(viewModel.moveSlot(0, by: 1))
        XCTAssertEqual(viewModel.slotIDs, ["m1", "m0", "m2", nil, nil, nil])
        XCTAssertEqual(viewModel.team.members.map(\.id), ["m1", "m0", "m2"])
    }

    func testMoveUpSwapsWithNeighbour() async {
        let viewModel = await makeViewModel(existing: 3)
        XCTAssertTrue(viewModel.moveSlot(2, by: -1))
        XCTAssertEqual(viewModel.team.members.map(\.id), ["m0", "m2", "m1"])
    }

    func testMoveSwapsWithAnEmptySlotToo() async {
        let viewModel = await makeViewModel(existing: 1)
        XCTAssertTrue(viewModel.moveSlot(0, by: 1))
        XCTAssertEqual(viewModel.slotIDs, [nil, "m0", nil, nil, nil, nil])
        XCTAssertEqual(viewModel.team.members.map(\.id), ["m0"])
    }

    func testFirstCannotMoveUpAndLastCannotMoveDown() async {
        let viewModel = await makeViewModel(existing: 2)
        XCTAssertFalse(viewModel.canMoveSlot(0, by: -1))
        XCTAssertFalse(viewModel.canMoveSlot(5, by: 1))
        XCTAssertTrue(viewModel.canMoveSlot(0, by: 1))
        XCTAssertFalse(viewModel.moveSlot(0, by: -1))
        XCTAssertFalse(viewModel.moveSlot(5, by: 1))
        XCTAssertEqual(viewModel.slotIDs, ["m0", "m1", nil, nil, nil, nil])
    }

    func testEmptySlotCannotBeMoved() async {
        let viewModel = await makeViewModel(existing: 1)
        XCTAssertFalse(viewModel.canMoveSlot(3, by: 1), "空の枠には上下の操作が無い")
    }

    // MARK: 未保存

    func testNoUnsavedChangesRightAfterOpening() async {
        let viewModel = await makeViewModel(existing: 2)
        XCTAssertFalse(viewModel.hasUnsavedChanges)
    }

    func testEditsMakeUnsavedChangesAndSaveClearsThem() async {
        let store = StubTeamStore()
        let viewModel = await makeViewModel(existing: 2, store: store)
        viewModel.setMemberNature(id: "m0", natureId: "stub-nature-spa-up")
        XCTAssertTrue(viewModel.hasUnsavedChanges)
        let ok = await viewModel.save()
        XCTAssertTrue(ok)
        XCTAssertFalse(viewModel.hasUnsavedChanges)
        XCTAssertTrue(viewModel.didSave)
    }

    func testEditingAfterSaveClearsTheSavedFlagAndMarksUnsaved() async {
        let viewModel = await makeViewModel(existing: 1)
        _ = await viewModel.save()
        viewModel.removeSlot(0)
        XCTAssertTrue(viewModel.hasUnsavedChanges)
        XCTAssertFalse(viewModel.didSave)
    }

    func testSwapAndSwapBackIsNotUnsaved() async {
        let viewModel = await makeViewModel(existing: 2)
        _ = viewModel.moveSlot(0, by: 1)
        XCTAssertTrue(viewModel.hasUnsavedChanges)
        _ = viewModel.moveSlot(1, by: -1)
        XCTAssertFalse(viewModel.hasUnsavedChanges, "元に戻せば未保存ではない")
    }

    func testFillingAnEmptySlotIsUnsaved() async {
        let viewModel = await makeViewModel(existing: 0)
        await viewModel.selectSpecies(slot: 0, speciesKey: StubMaster.alpha.key)
        XCTAssertTrue(viewModel.hasUnsavedChanges)
    }

    func testFailedSaveKeepsTheDraftUnsaved() async {
        let store = StubTeamStore()
        await store.setSaveError(PokeCalcError(code: "test_save_failure", message: "テスト: 保存に失敗"))
        let viewModel = await makeViewModel(existing: 1, store: store)
        viewModel.setMemberNature(id: "m0", natureId: "stub-nature-spa-up")
        let ok = await viewModel.save()
        XCTAssertFalse(ok)
        XCTAssertTrue(viewModel.hasUnsavedChanges)
        XCTAssertFalse(viewModel.didSave)
    }

    // MARK: 保存

    func testSaveStampsUpdatedAtAndKeepsTheName() async {
        let store = StubTeamStore()
        let now = Date(timeIntervalSince1970: 1_800_000_123)
        let viewModel = await makeViewModel(existing: 1, store: store, now: now)
        _ = await viewModel.save()
        let saved = await store.saveCalls.last
        XCTAssertEqual(saved?.updatedAt, now)
        XCTAssertEqual(saved?.name, TeamNaming.defaultName)
    }

    func testSaveWritesOnlyFilledSlotsInSlotOrder() async {
        let store = StubTeamStore()
        let viewModel = await makeViewModel(existing: 2, store: store)
        viewModel.removeSlot(0)
        await viewModel.selectSpecies(slot: 4, speciesKey: StubMaster.beta.key)
        _ = await viewModel.save()
        let saved = await store.saveCalls.last
        XCTAssertEqual(saved?.members.count, 2)
        XCTAssertEqual(saved?.members.first?.id, "m1")
        XCTAssertEqual(saved?.members.last?.speciesKey, StubMaster.beta.key)
    }

    func testSavingAnOldNamedTeamKeepsItsName() async {
        let store = StubTeamStore()
        let viewModel = TeamEditViewModel(
            store: store, service: StubMaster.makeService(), team: Team(id: "t", name: "テストむかしのなまえ"))
        await viewModel.load()
        _ = await viewModel.save()
        let saved = await store.saveCalls.last
        XCTAssertEqual(saved?.name, "テストむかしのなまえ", "旧データの名前は保存で消さない(互換)")
    }

    // MARK: 取り込み・追加との整合

    func testAddMemberFillsTheFirstEmptySlot() async {
        let viewModel = await makeViewModel(existing: 3)
        viewModel.removeSlot(1)
        let ok = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        XCTAssertTrue(ok)
        XCTAssertEqual(viewModel.member(atSlot: 1)?.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.team.members.count, 3)
    }

    func testImportFillsEmptySlotsInOrder() async {
        let viewModel = await makeViewModel(existing: 1)
        let imported = TeamMember(id: "n1", speciesKey: StubMaster.beta.key, natureId: "stub-nature-neutral")
        _ = await viewModel.importMembers([imported])
        XCTAssertEqual(viewModel.slotIDs, ["m0", "n1", nil, nil, nil, nil])
    }
}
