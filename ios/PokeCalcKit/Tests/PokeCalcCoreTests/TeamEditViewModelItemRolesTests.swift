import XCTest

@testable import PokeCalcCore

/// 構築の編集の持ち物の役割とメガ固定・保存データの補正(ADR-0509 §3・§4。Web ADR-0320 PR-B と同じ方針)。
///
/// - 選択肢 = `.any`(どちらかの役割を持つ持ち物。構築の時点では攻守が決まらない)。いまの持ち物は残す。
/// - メガ種族に変えたらストーンに固定、メガ以外に変えたら未選択に戻す。固定中は `setMemberItem` を無視する。
/// - 開いた時点でメガ種族に別の持ち物(null を含む)があればストーンに直し、通知を出す(保存はしない)。
@MainActor
final class TeamEditViewModelItemRolesTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private func loadedViewModel(member: TeamMember) async -> (TeamEditViewModel, StubTeamStore) {
        let store = StubTeamStore()
        let team = Team(id: "team-mega", name: "テストメガチーム", members: [member])
        let viewModel = TeamEditViewModel(store: store, service: Mega.makeService(), team: team)
        await viewModel.load()
        return (viewModel, store)
    }

    private func member(_ viewModel: TeamEditViewModel, _ id: String) throws -> TeamMember {
        try XCTUnwrap(viewModel.team.members.first(where: { $0.id == id }))
    }

    func testOptionsAreNotFilteredByRoleExceptMegaStones() async {
        let plain = TeamMember(id: "m-plain", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")
        let (viewModel, _) = await loadedViewModel(member: plain)

        XCTAssertEqual(
            viewModel.itemOptions(forMember: plain.id).map(\.id), [Mega.attackOnly.id, Mega.defenseOnly.id, Mega.both.id, Mega.noRole.id],
            "構築は役割で絞らずメガストーンだけ外す(ADR-0326 §2。計算に効かない持ち物も記録できる)")
    }

    func testSavedItemWithoutRoleIsKeptAsOption() async {
        let legacy = TeamMember(id: "m-legacy", speciesKey: StubMaster.alpha.key, itemId: Mega.noRole.id, natureId: "stub-nature-neutral")
        let (viewModel, _) = await loadedViewModel(member: legacy)

        XCTAssertEqual(
            viewModel.itemOptions(forMember: legacy.id).map(\.id),
            [Mega.attackOnly.id, Mega.defenseOnly.id, Mega.both.id, Mega.noRole.id], "保存済みの値は黙って消さない(§5)")
        XCTAssertNil(viewModel.itemNotice(forMember: legacy.id))
    }

    func testChangingSpeciesToMegaLocksAndBackUnlocks() async throws {
        let plain = TeamMember(id: "m-1", speciesKey: StubMaster.alpha.key, itemId: Mega.both.id, natureId: "stub-nature-neutral")
        let (viewModel, _) = await loadedViewModel(member: plain)

        await viewModel.setMemberSpecies(id: plain.id, speciesKey: Mega.megaAlpha.key)
        XCTAssertEqual(viewModel.itemLock(forMember: plain.id), .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(try member(viewModel, plain.id).itemId, Mega.stone.id)
        XCTAssertEqual(viewModel.itemLabel(for: Mega.stone.id), Mega.megaAlphaStoneName)

        viewModel.setMemberItem(id: plain.id, itemId: Mega.both.id)
        XCTAssertEqual(try member(viewModel, plain.id).itemId, Mega.stone.id, "固定中は持ち物を変えられない")

        await viewModel.setMemberSpecies(id: plain.id, speciesKey: StubMaster.beta.key)
        XCTAssertEqual(viewModel.itemLock(forMember: plain.id), .none)
        XCTAssertNil(try member(viewModel, plain.id).itemId, "メガストーンを残さない")
    }

    /// 開いた時点でメガ種族に別の持ち物 → ストーンに直し、通知する。保存はしない(下書きの変更)。
    func testLoadCorrectsSavedMegaMemberWithOtherItem() async throws {
        let saved = TeamMember(id: "m-saved", speciesKey: Mega.megaAlpha.key, itemId: Mega.both.id, natureId: "stub-nature-neutral")
        let (viewModel, store) = await loadedViewModel(member: saved)

        XCTAssertEqual(try member(viewModel, saved.id).itemId, Mega.stone.id)
        XCTAssertEqual(
            viewModel.itemNotice(forMember: saved.id),
            "メガシンカのため持ち物を\(Mega.megaAlphaStoneName)に直しました。保存すると反映されます")
        let saves = await store.saveCalls.count
        XCTAssertEqual(saves, 0, "自動では保存しない")
    }

    func testLoadClearsSavedMegaMemberWhenStoneIsMissing() async throws {
        let saved = TeamMember(id: "m-missing", speciesKey: Mega.megaMissingStone.key, itemId: Mega.both.id, natureId: "stub-nature-neutral")
        let (viewModel, _) = await loadedViewModel(member: saved)

        XCTAssertNil(try member(viewModel, saved.id).itemId)
        XCTAssertEqual(viewModel.itemLock(forMember: saved.id), .missing)
        XCTAssertEqual(
            viewModel.itemNotice(forMember: saved.id),
            "メガシンカのメガストーンがマスタに無いため、持ち物を空にしました。保存すると反映されます")
    }

    func testLoadDoesNotTouchCorrectMegaMemberOrNonMegaWithStone() async throws {
        let correct = TeamMember(id: "m-ok", speciesKey: Mega.megaAlpha.key, itemId: Mega.stone.id, natureId: "stub-nature-neutral")
        let (viewModel, _) = await loadedViewModel(member: correct)
        XCTAssertEqual(try member(viewModel, correct.id).itemId, Mega.stone.id)
        XCTAssertNil(viewModel.itemNotice(forMember: correct.id))

        let nonMega = TeamMember(id: "m-non", speciesKey: StubMaster.alpha.key, itemId: Mega.stone.id, natureId: "stub-nature-neutral")
        let (other, _) = await loadedViewModel(member: nonMega)
        XCTAssertEqual(try member(other, nonMega.id).itemId, Mega.stone.id, "非メガのストーンは直さない(API が検査しない)")
        XCTAssertNil(other.itemNotice(forMember: nonMega.id))
        XCTAssertEqual(other.itemLabel(for: Mega.stone.id), "メガストーン", "知らないストーンでも英語名は出さない")
    }

    func testNoticeDisappearsWhenSpeciesChanges() async {
        let saved = TeamMember(id: "m-saved", speciesKey: Mega.megaAlpha.key, itemId: Mega.both.id, natureId: "stub-nature-neutral")
        let (viewModel, _) = await loadedViewModel(member: saved)

        await viewModel.setMemberSpecies(id: saved.id, speciesKey: StubMaster.gamma.key)

        XCTAssertNil(viewModel.itemNotice(forMember: saved.id))
    }
}
