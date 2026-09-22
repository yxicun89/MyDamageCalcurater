import XCTest

@testable import PokeCalcCore

/// 「構築から個体を呼び出す」の純粋な部分(P6-2d。ADR-0501「P6-2d」1章・6章)。
///
/// - `BuildSource`(`.preset` / `.team` の直和。自分側の SP・性格の出どころ)
/// - `TeamIndividualOptions.groups(from:species:)`(構築 → 画面のピッカーの選択肢)
/// - `TeamLoadLabels`(画面に出す文言。`DisplayLabels` と同じく表示専用の語彙を Core に1か所)
///
/// ViewModel を通さずに確かめる(View も API も要らない値の変換だけ)。
final class TeamIndividualSelectionTests: XCTestCase {

    // MARK: - 補助

    /// `StubMaster` の種族詳細から、マスタ検索が返す要約を作る。
    private func summaries(_ details: [SpeciesDetail]) -> [SpeciesSummary] {
        details.map {
            SpeciesSummary(key: $0.key, dexNo: $0.dexNo, form: $0.form, nameJa: $0.nameJa, types: $0.types)
        }
    }

    private func selection(memberID: String) -> TeamIndividualSelection {
        TeamIndividualSelection(
            teamID: StubTeams.teamAlpha.id, memberID: memberID, displayName: "テストひょうじめい",
            individual: TeamMemberConverter.makeIndividual(
                from: StubTeams.namedMember, moves: [StubMaster.specialMove]
            )
        )
    }

    // MARK: - BuildSource(排他の直和)

    func testPresetSourceExposesPresetAndNoTeamSelection() {
        let source: AttackerBuildSource = .preset(.aMax)
        XCTAssertEqual(source.preset, .aMax)
        XCTAssertNil(source.teamSelection, "プリセットを選んでいる間は構築の選択を持たない")
    }

    func testTeamSourceExposesSelectionAndNoPreset() {
        let picked = selection(memberID: StubTeams.namedMember.id)
        let source: AttackerBuildSource = .team(picked)
        XCTAssertNil(source.preset, "構築の個体を呼んでいる間はどのプリセットも選ばれていない")
        XCTAssertEqual(source.teamSelection, picked)
    }

    func testKnownDefenderBuildSourceIsTheSameShape() {
        // 逆算の「受けたダメージ」(自分が防御側)も同じ直和を使う(ADR-0501「P6-2d」4章: 両側に入れる)。
        let presetSource: KnownDefenderBuildSource = .preset(.full)
        XCTAssertEqual(presetSource.preset, .full)
        XCTAssertNil(presetSource.teamSelection)

        let picked = selection(memberID: StubTeams.namedMember.id)
        let teamSource: KnownDefenderBuildSource = .team(picked)
        XCTAssertNil(teamSource.preset)
        XCTAssertEqual(teamSource.teamSelection, picked)
    }

    // MARK: - ピッカーの選択肢

    func testGroupsKeepStoreOrderAndSkipTeamsWithoutMembers() {
        let groups = TeamIndividualOptions.groups(
            from: StubTeams.all, species: summaries([StubMaster.alpha, StubMaster.beta])
        )
        XCTAssertEqual(groups.map(\.id), [StubTeams.teamAlpha.id, StubTeams.teamBeta.id],
                       "メンバーが0体の構築は選べるものが無いので出さない")
        XCTAssertEqual(groups.map(\.name), [StubTeams.teamAlpha.name, StubTeams.teamBeta.name])
        XCTAssertEqual(groups.first?.members.map(\.id),
                       [StubTeams.namedMember.id, StubTeams.noMoveMember.id],
                       "メンバーは構築の並び順のまま")
        XCTAssertEqual(groups.first?.members.map(\.speciesKey),
                       [StubTeams.namedMember.speciesKey, StubTeams.noMoveMember.speciesKey])
    }

    func testMemberDisplayNamePrefersNickname() {
        let groups = TeamIndividualOptions.groups(
            from: [StubTeams.teamAlpha], species: summaries([StubMaster.alpha])
        )
        XCTAssertEqual(groups.first?.members.first?.displayName, StubTeams.namedMember.nickname)
    }

    func testMemberDisplayNameFallsBackToSpeciesNameWhenNoNickname() {
        let groups = TeamIndividualOptions.groups(
            from: [StubTeams.teamAlpha], species: summaries([StubMaster.alpha])
        )
        XCTAssertEqual(groups.first?.members.last?.displayName, StubMaster.alpha.nameJa)
    }

    func testMemberDisplayNameFallsBackToSpeciesKeyWhenMasterIsMissing() {
        // 種族マスタがまだ読めていない・その種族が一覧に無いときでも名前を空にしない。
        let groups = TeamIndividualOptions.groups(from: [StubTeams.teamAlpha], species: [])
        XCTAssertEqual(groups.first?.members.last?.displayName, StubTeams.noMoveMember.speciesKey)
    }

    func testBlankNicknameIsTreatedAsAbsent() {
        var member = StubTeams.namedMember
        member.nickname = "   "
        let team = Team(id: StubTeams.teamAlpha.id, name: StubTeams.teamAlpha.name, members: [member])
        let groups = TeamIndividualOptions.groups(from: [team], species: summaries([StubMaster.alpha]))
        XCTAssertEqual(groups.first?.members.first?.displayName, StubMaster.alpha.nameJa)
    }

    func testNoTeamsGivesNoGroups() {
        XCTAssertTrue(TeamIndividualOptions.groups(from: [], species: summaries([StubMaster.alpha])).isEmpty)
        XCTAssertTrue(TeamIndividualOptions.groups(from: [StubTeams.emptyTeam], species: []).isEmpty)
    }

    // MARK: - 文言

    func testLabelsAreNotEmpty() {
        // 文言そのものは View に直書きせず Core に1か所置く(ADR-0501「P6-2d」6章)。
        XCTAssertFalse(TeamLoadLabels.entryTitle.isEmpty)
        XCTAssertFalse(TeamLoadLabels.empty.isEmpty)
        XCTAssertNotEqual(TeamLoadLabels.entryTitle, TeamLoadLabels.empty)
    }
}
