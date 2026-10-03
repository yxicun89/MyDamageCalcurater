import XCTest

@testable import PokeCalcCore

/// 構築から呼び出す(P6-25。ADR-0504 §7。iOS の強み): 自分側と各候補側を、構築のメンバー1体の内容で埋める。
/// 規則は計算・逆算の「構築から選ぶ」と同じ(`TeamMemberConverter` の技の選び方・スナップショット・無効な ID は何もしない)。
/// 構築は `StubTeams`(架空)。構築のストアは読むだけで、保存・削除はしない。
@MainActor
final class JudgeViewModelTeamTests: XCTestCase {
    private typealias H = JudgeHarness

    private func makeLoaded(
        service: StubJudgeService = StubJudgeService(), store: StubTeamStore = StubTeams.makeStore()
    ) async -> JudgeViewModel {
        let viewModel = H.makeViewModel(service: service, store: store)
        await viewModel.load()
        return viewModel
    }

    // MARK: - 一覧

    func testTeamOptionsListTeamsWithMembersAndSkipEmptyTeams() async {
        let viewModel = await makeLoaded()
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamAlpha.id, StubTeams.teamBeta.id], "メンバー0体の構築は出さない")
        XCTAssertEqual(
            viewModel.teamOptions.first?.members.map(\.displayName), [StubTeams.namedMember.nickname ?? "", StubMaster.alpha.nameJa],
            "ニックネーム → 無ければ種族名")
    }

    func testWithoutAStoreTheTeamOptionsAreEmpty() async {
        let viewModel = H.makeViewModel(store: nil)
        await viewModel.load()
        XCTAssertEqual(viewModel.teamOptions, [])
        await viewModel.loadTeams()
        XCTAssertEqual(viewModel.teamOptions, [])
    }

    func testLoadTeamsRereadsTheStore() async {
        let store = StubTeams.makeStore(teams: [])
        let viewModel = await makeLoaded(store: store)
        XCTAssertEqual(viewModel.teamOptions, [])
        await store.seed([StubTeams.teamAlpha])
        await viewModel.loadTeams()
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamAlpha.id])
    }

    // MARK: - 呼び出す

    func testApplyingAMemberToTheAttackerFillsEveryFieldFromTheMember() async {
        let viewModel = await makeLoaded()
        viewModel.setRank(.atk, 3, for: .attacker)
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id, to: .attacker)
        let draft = viewModel.attacker
        XCTAssertEqual(draft.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(draft.natureId, StubMaster.spaUpNature.id)
        XCTAssertEqual(draft.sp, StubTeams.customSP)
        XCTAssertEqual(draft.abilityId, StubMaster.ability.id)
        XCTAssertEqual(draft.itemId, StubMaster.itemB.id)
        XCTAssertEqual(draft.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(draft.ranks, RankBlock(), "ランクは構築に無いので 0 に戻す(前の呼び出しの値を引きずらない)")
        XCTAssertEqual(viewModel.speciesName(for: .attacker), StubMaster.alpha.nameJa)
        XCTAssertEqual(viewModel.abilityOptions(for: .attacker), StubMaster.alpha.abilities)
    }

    func testApplyingAMemberToACandidateTouchesOnlyThatCandidate() async {
        let viewModel = await makeLoaded()
        viewModel.addCandidate()
        viewModel.setNature("keep", for: .attacker)
        let attackerBefore = viewModel.attacker
        let firstBefore = viewModel.candidates[0]
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id, to: .candidate(1))
        XCTAssertEqual(viewModel.attacker, attackerBefore)
        XCTAssertEqual(viewModel.candidates[0], firstBefore)
        XCTAssertEqual(viewModel.candidates[safe: 1]?.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.candidates[safe: 1]?.moveId, StubMaster.specialMove.id, "候補の技も構築のメンバーの技から")
        XCTAssertEqual(viewModel.candidates[safe: 1]?.sp, StubTeams.customSP)
        XCTAssertEqual(viewModel.speciesName(for: .candidate(1)), StubMaster.alpha.nameJa)
    }

    func testTheMoveIsTheFirstDamagingMoveOtherwiseTheFirstOtherwiseNone() async {
        let mixed = TeamMember(
            id: "stub-member-mixed", speciesKey: StubMaster.alpha.key,
            moveIds: [StubMaster.statusMove.id, StubMaster.physicalMove.id], natureId: StubMaster.neutralNature.id)
        let team = Team(id: "stub-team-mixed", name: "テストこうちくミックス", members: [mixed])
        let store = StubTeams.makeStore(teams: [team, StubTeams.teamAlpha, StubTeams.teamBeta])
        let viewModel = await makeLoaded(store: store)

        await viewModel.applyTeamMember(teamID: team.id, memberID: mixed.id, to: .attacker)
        XCTAssertEqual(viewModel.attacker.moveId, StubMaster.physicalMove.id, "最初のダメージ技(変化技は飛ばす)")

        await viewModel.applyTeamMember(teamID: StubTeams.teamBeta.id, memberID: StubTeams.statusMoveMember.id, to: .attacker)
        XCTAssertEqual(viewModel.attacker.moveId, StubMaster.statusMove.id, "ダメージ技が無ければ最初の技")

        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.noMoveMember.id, to: .attacker)
        XCTAssertNil(viewModel.attacker.moveId, "技が無ければ未選択(候補は技が必須なので、画面で選んでもらう)")
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.attacker))
    }

    func testUnknownIDsChangeNothing() async {
        let viewModel = await makeLoaded()
        await viewModel.applyTeamMember(teamID: "no-such-team", memberID: StubTeams.namedMember.id, to: .attacker)
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: "no-such-member", to: .attacker)
        await viewModel.applyTeamMember(teamID: StubTeams.emptyTeam.id, memberID: "x", to: .attacker)
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id, to: .candidate(9))
        XCTAssertEqual(viewModel.attacker.speciesKey, nil)
        XCTAssertEqual(viewModel.candidates.count, 1)
        XCTAssertNil(viewModel.candidates[0].speciesKey)
    }

    /// 呼び出したあとも各欄は手で直せる。呼び出しは判定を送らず、構築のストアも書き換えない。
    func testAppliedValuesCanBeEditedAndApplyingNeverSendsOrSaves() async {
        let service = StubJudgeService()
        let store = StubTeams.makeStore()
        let viewModel = await makeLoaded(service: service, store: store)
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id, to: .attacker)
        viewModel.setSP(.spe, 32, for: .attacker)
        viewModel.setItem(nil, for: .attacker)
        XCTAssertEqual(viewModel.attacker.sp.spe, 32)
        XCTAssertNil(viewModel.attacker.itemId)
        XCTAssertEqual(viewModel.attacker.speciesKey, StubMaster.alpha.key, "他の欄は呼び出した内容のまま")
        let requests = await service.requests
        let saves = await store.saveCalls
        let deletes = await store.deleteCalls
        XCTAssertTrue(requests.isEmpty)
        XCTAssertTrue(saves.isEmpty)
        XCTAssertTrue(deletes.isEmpty)
    }

    /// 構築から埋めた自分・候補は、そのまま判定の要求になる(技は候補ごと)。
    func testAFullTeamCallBuildsARequest() async throws {
        let viewModel = await makeLoaded()
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id, to: .attacker)
        await viewModel.applyTeamMember(teamID: StubTeams.teamBeta.id, memberID: StubTeams.statusMoveMember.id, to: .candidate(0))
        let request = try H.request(viewModel)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.defenders[0].moveId, StubMaster.statusMove.id)
    }
}
