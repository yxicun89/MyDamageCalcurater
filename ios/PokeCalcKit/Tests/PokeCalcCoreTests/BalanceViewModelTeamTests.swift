import XCTest

@testable import PokeCalcCore

/// `BalanceViewModel` の入力 = 構築から選ぶ(P6-26。ADR-0505 §4・§5。iOS の強み)。
/// 構築は `StubBalance` の架空データ(`StubTeamStore`)。ストアは読むだけで、保存・削除はしない。
@MainActor
final class BalanceViewModelTeamTests: XCTestCase {
    private typealias H = BalanceHarness
    private typealias B = StubBalance

    // MARK: - 一覧

    func testTeamOptionsListEveryTeamInStoreOrderWithMemberCounts() async {
        let viewModel = await H.makeLoaded()
        XCTAssertEqual(viewModel.teamOptions.map(\.id), B.allTeams.map(\.id), "メンバー 0 体の構築も出す(選ぶと案内になる)")
        XCTAssertEqual(viewModel.teamOptions.map(\.name), B.allTeams.map(\.name))
        XCTAssertEqual(viewModel.teamOptions.map(\.memberCount), [2, 4, 1, 0, 6])
        XCTAssertFalse(viewModel.teamLoadFailed)
    }

    func testWithoutAStoreTheTeamOptionsAreEmptyAndNothingFails() async {
        let viewModel = H.makeViewModel(store: nil)
        await viewModel.load()
        XCTAssertEqual(viewModel.teamOptions, [])
        XCTAssertFalse(viewModel.teamLoadFailed, "ストアが無いのは失敗ではない(案内は「まだ構築がありません」)")
    }

    /// 構築を読めない失敗は `teamLoadFailed`。balance は呼ばない・`load()` は throw しない(絶対ルール 5)。
    func testStoreListFailureSetsTeamLoadFailedAndCallsNothing() async {
        let store = StubTeamStore(teams: B.allTeams)
        await store.setListError(PokeCalcError(code: "team_store_unavailable", message: "stub"))
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service, store: store)
        XCTAssertTrue(viewModel.teamLoadFailed)
        XCTAssertEqual(viewModel.teamOptions, [])
        let analyzeCalls = await service.callCount(.analyze)
        let coverageCalls = await service.callCount(.coverage)
        XCTAssertEqual(analyzeCalls + coverageCalls, 0)
    }

    func testInitialStateIsIdleWithNoSelection() async {
        let viewModel = await H.makeLoaded()
        XCTAssertNil(viewModel.selectedTeamID)
        XCTAssertEqual(viewModel.defenseState, .idle)
        XCTAssertEqual(viewModel.coverageState, .idle)
    }

    // MARK: - 選ぶ

    /// `selectTeam` は同期で選択と `.loading` を反映する(2 つとも。技のある構築)。
    func testSelectingATeamSetsLoadingSynchronously() async {
        let service = StubBalanceService()
        await service.hold(.analyze)
        await service.hold(.coverage)
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamMixed.id)
        XCTAssertEqual(viewModel.selectedTeamID, B.teamMixed.id)
        XCTAssertEqual(viewModel.defenseState, .loading)
        XCTAssertEqual(viewModel.coverageState, .loading)
        viewModel.cancelPendingWork()
    }

    func testSelectingAnUnknownTeamDoesNothing() async {
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: "no-such-team")
        await viewModel.settle()
        XCTAssertNil(viewModel.selectedTeamID)
        XCTAssertEqual(viewModel.defenseState, .idle)
        let analyzeCalls = await service.callCount(.analyze)
        XCTAssertEqual(analyzeCalls, 0)
    }

    /// 選ぶと、構築から作った要求(`BalanceRequestBuilder`)が analyze・coverage に 1 回ずつ送られる。
    func testSelectingATeamSendsTheBuiltRequestsOnceEach() async {
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamFour.id)
        await viewModel.settle()
        let expected = BalanceRequestBuilder.make(from: B.teamFour)
        let analyze = await service.analyzeRequests
        let coverage = await service.coverageRequests
        XCTAssertEqual(analyze, [expected.analyze].compactMap { $0 })
        XCTAssertEqual(coverage, [expected.coverage].compactMap { $0 })
        XCTAssertNotNil(expected.analyze)
        XCTAssertNotNil(expected.coverage)
    }

    /// 名前の引き当て: ニックネーム → 種族名(マスタ)→ speciesKey(空白だけのニックネームは無いものとして扱う。マスタに無い種族は speciesKey)。
    /// 特性名は種族の特性候補から引く(特性が空文字のメンバーは nil)。同じ種族の重複も別々の行。
    func testDisplayNamesFollowNicknameThenSpeciesNameThenKey() async throws {
        let viewModel = await H.makeLoaded()
        viewModel.selectTeam(id: B.teamFour.id)
        await viewModel.settle()
        let defense = try XCTUnwrap(H.defenseDisplay(viewModel))
        XCTAssertEqual(
            defense.members.map(\.name), ["テストバランスニックA", StubMaster.beta.nameJa, StubMaster.alpha.nameJa, "9999-000"])
        XCTAssertEqual(defense.members.map(\.abilityName), [StubMaster.ability.nameJa, nil, nil, nil])
        let coverage = try XCTUnwrap(H.coverageDisplay(viewModel))
        XCTAssertEqual(coverage.members.map(\.name), defense.members.map(\.name), "防御と攻撃範囲で同じ名前")
    }

    /// 技を持つメンバーがいなければ coverage は呼ばない(`.skipped`)。analyze は呼ぶ。
    func testTeamWithoutAnyMoveSkipsCoverageButAnalyzes() async {
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamNoMoves.id)
        XCTAssertEqual(viewModel.coverageState, .skipped, "同期で決まる(呼ばないので待たない)")
        await viewModel.settle()
        XCTAssertNotNil(H.defenseDisplay(viewModel))
        XCTAssertEqual(viewModel.coverageState, .skipped)
        let analyzeCalls = await service.callCount(.analyze)
        let coverageCalls = await service.callCount(.coverage)
        XCTAssertEqual(analyzeCalls, 1)
        XCTAssertEqual(coverageCalls, 0)
    }

    /// メンバーが 0 体の構築: どちらも呼ばず `.skipped`(画面は「ポケモンがいません」)。
    func testEmptyTeamSkipsBothAndCallsNothing() async {
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service)
        viewModel.selectTeam(id: B.teamEmpty.id)
        XCTAssertEqual(viewModel.selectedTeamID, B.teamEmpty.id)
        XCTAssertEqual(viewModel.defenseState, .skipped)
        XCTAssertEqual(viewModel.coverageState, .skipped)
        await viewModel.settle()
        let analyzeCalls = await service.callCount(.analyze)
        let coverageCalls = await service.callCount(.coverage)
        XCTAssertEqual(analyzeCalls + coverageCalls, 0)
    }

    /// 6 体ちょうどはそのまま送る。壊れた保存データの 7 体は先頭 6 体だけ送る(契約の上限を超えない)。
    func testSixMembersAreSentAndSevenAreCappedAtSix() async {
        let service = StubBalanceService()
        let store = StubTeamStore(teams: [B.teamSix, B.teamSeven])
        let viewModel = await H.makeLoaded(service: service, store: store)
        viewModel.selectTeam(id: B.teamSix.id)
        await viewModel.settle()
        viewModel.selectTeam(id: B.teamSeven.id)
        await viewModel.settle()
        let analyze = await service.analyzeRequests
        XCTAssertEqual(analyze.map { $0.members.count }, [6, 6])
        if case .loaded(let display) = viewModel.defenseState {
            XCTAssertEqual(display.members.count, 6)
        } else {
            XCTFail("loaded ではない: \(viewModel.defenseState)")
        }
    }

    /// 別の構築を選ぶと、結果は新しい構築のものに置き換わる(前の構築の行を引きずらない)。
    func testSelectingAnotherTeamReplacesTheResults() async throws {
        let viewModel = await H.makeLoaded()
        viewModel.selectTeam(id: B.teamFour.id)
        await viewModel.settle()
        XCTAssertEqual(try XCTUnwrap(H.defenseDisplay(viewModel)).members.count, 4)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        XCTAssertEqual(viewModel.selectedTeamID, B.teamMixed.id)
        XCTAssertEqual(try XCTUnwrap(H.defenseDisplay(viewModel)).members.count, 2)
        XCTAssertEqual(try XCTUnwrap(H.coverageDisplay(viewModel)).members.count, 2)
    }

    /// 構築ありから「技なし」「0 体」の構築へ切り替えたとき、前の構築の攻撃範囲を残さない。
    func testSwitchingToATeamWithoutMovesClearsThePreviousCoverage() async {
        let viewModel = await H.makeLoaded()
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        XCTAssertNotNil(H.coverageDisplay(viewModel))
        viewModel.selectTeam(id: B.teamNoMoves.id)
        XCTAssertEqual(viewModel.coverageState, .skipped)
        viewModel.selectTeam(id: B.teamEmpty.id)
        XCTAssertEqual(viewModel.defenseState, .skipped)
    }

    // MARK: - 読み直す

    /// `reanalyze()` は構築の一覧を読み直し、選択中の構築の最新の内容で解析し直す。
    func testReanalyzeRereadsTheStoreAndUsesTheLatestMembers() async {
        let store = StubTeamStore(teams: [B.teamNoMoves])
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service, store: store)
        viewModel.selectTeam(id: B.teamNoMoves.id)
        await viewModel.settle()
        var edited = B.teamNoMoves
        edited.members.append(B.memberA)
        await store.seed([edited])
        await viewModel.reanalyze()
        await viewModel.settle()
        XCTAssertEqual(viewModel.teamOptions.map(\.memberCount), [2])
        let analyze = await service.analyzeRequests
        XCTAssertEqual(analyze.map { $0.members.count }, [1, 2])
        let coverage = await service.coverageRequests
        XCTAssertEqual(coverage.count, 1, "技を持つメンバーが増えたので coverage も呼ぶ")
        XCTAssertEqual(viewModel.selectedTeamID, B.teamNoMoves.id)
    }

    /// 選択中の構築が消えていたら、選択を外して `.idle` に戻す(存在しない構築を送らない)。
    func testReanalyzeAfterTheSelectedTeamWasDeletedClearsTheSelection() async {
        let store = StubTeamStore(teams: [B.teamMixed])
        let service = StubBalanceService()
        let viewModel = await H.makeLoaded(service: service, store: store)
        viewModel.selectTeam(id: B.teamMixed.id)
        await viewModel.settle()
        await store.seed([])
        await viewModel.reanalyze()
        XCTAssertNil(viewModel.selectedTeamID)
        XCTAssertEqual(viewModel.defenseState, .idle)
        XCTAssertEqual(viewModel.coverageState, .idle)
        let analyzeCalls = await service.callCount(.analyze)
        XCTAssertEqual(analyzeCalls, 1, "消えた構築では呼ばない")
    }

    /// 構築のストアには書かない(読むだけ)。
    func testTheStoreIsNeverWritten() async {
        let store = StubTeamStore(teams: B.allTeams)
        let viewModel = await H.makeLoaded(store: store)
        viewModel.selectTeam(id: B.teamFour.id)
        await viewModel.settle()
        await viewModel.reanalyze()
        let saves = await store.saveCalls
        let deletes = await store.deleteCalls
        XCTAssertTrue(saves.isEmpty)
        XCTAssertTrue(deletes.isEmpty)
    }
}
